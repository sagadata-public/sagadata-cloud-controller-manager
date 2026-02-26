// Copyright 2025 Saga Data AS. All rights reserved.
// Use of this source code is governed by the Mozilla Public License, v. 2.0.

package sagadata

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	sagadata "github.com/sagadata-public/sagadata-go"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	// AnnotationLoadBalancerName is written back to the Service so that
	// cluster admins can see which Saga Data load balancer backs it.
	AnnotationLoadBalancerName = "sagadata.no/loadbalancer-name"
)

type loadBalancers struct {
	client     *sagadata.ClientWithResponses
	kubeClient kubernetes.Interface
	region     sagadata.Region
	network    string
}

func lbName(svc *v1.Service) string {
	raw := strings.ReplaceAll(string(svc.UID), "-", "")
	b, err := hex.DecodeString(raw)
	if err != nil {
		return "kube-svc-" + string(svc.UID)
	}
	var n big.Int
	n.SetBytes(b)
	return "kube-svc-" + n.Text(36)
}

func (lb *loadBalancers) lbByName(ctx context.Context, name string) (*sagadata.Loadbalancer, error) {
	page := 1
	for {
		resp, err := lb.client.ListLoadbalancersWithResponse(ctx, &sagadata.ListLoadbalancersParams{
			Page: &page,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list load balancers: %w", err)
		}
		if resp.JSON200 == nil {
			return nil, fmt.Errorf("unexpected response: %s", resp.HTTPResponse.Status)
		}
		for idx := range resp.JSON200.Loadbalancers {
			if resp.JSON200.Loadbalancers[idx].Name == name {
				return &resp.JSON200.Loadbalancers[idx], nil
			}
		}
		if page*resp.JSON200.PerPage >= resp.JSON200.TotalCount {
			break
		}
		page++
	}
	return nil, nil
}

func buildPorts(svc *v1.Service, nodes []*v1.Node) []sagadata.LoadbalancerPort {
	var targets []string
	for _, n := range nodes {
		for _, addr := range n.Status.Addresses {
			if addr.Type == v1.NodeInternalIP {
				targets = append(targets, addr.Address)
				break
			}
		}
	}

	var ports []sagadata.LoadbalancerPort
	for _, sp := range svc.Spec.Ports {
		ports = append(ports, sagadata.LoadbalancerPort{
			Port:       int(sp.Port),
			TargetPort: int(sp.NodePort),
			Targets:    targets,
		})
	}
	return ports
}

// GetLoadBalancer returns the status of the load balancer for the given service,
// or (nil, false, nil) if no matching LB exists.
func (lb *loadBalancers) GetLoadBalancer(ctx context.Context, clusterName string, svc *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	found, err := lb.lbByName(ctx, lbName(svc))
	if err != nil {
		return nil, false, err
	}
	if found == nil {
		return nil, false, nil
	}
	if found.ExternalIp != nil {
		return &v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{{IP: *found.ExternalIp}},
		}, true, nil
	}
	return &v1.LoadBalancerStatus{}, true, nil
}

// GetLoadBalancerName returns the deterministic name for the LB backing this service.
func (lb *loadBalancers) GetLoadBalancerName(ctx context.Context, clusterName string, svc *v1.Service) string {
	return lbName(svc)
}

// EnsureLoadBalancer creates or updates a load balancer for the given service.
func (lb *loadBalancers) EnsureLoadBalancer(ctx context.Context, clusterName string, svc *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	name := lbName(svc)
	ports := buildPorts(svc, nodes)

	if err := lb.annotateService(ctx, svc, name); err != nil {
		klog.Warningf("failed to annotate service %s/%s with load balancer name: %v", svc.Namespace, svc.Name, err)
	}

	found, err := lb.lbByName(ctx, name)
	if err != nil {
		return nil, err
	}

	var result *sagadata.Loadbalancer

	if found == nil {
		createBody := sagadata.CreateLoadbalancerJSONRequestBody{
			Name:    name,
			Region:  lb.region,
			Network: lb.network,
			Ports:   ports,
		}
		if bodyJSON, err := json.Marshal(createBody); err == nil {
			klog.Infof("creating load balancer %q, request body: %s", name, string(bodyJSON))
		}
		resp, err := lb.client.CreateLoadbalancerWithResponse(ctx, createBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create load balancer: %w", err)
		}
		if resp.JSON201 == nil {
			return nil, fmt.Errorf("unexpected response creating load balancer: %s, body: %s", resp.HTTPResponse.Status, string(resp.Body))
		}
		result = &resp.JSON201.Loadbalancer
	} else {
		updateBody := sagadata.UpdateLoadbalancerJSONRequestBody{
			Ports: &ports,
		}
		if bodyJSON, err := json.Marshal(updateBody); err == nil {
			klog.Infof("updating load balancer %q (%s), request body: %s", name, found.Id, string(bodyJSON))
		}
		resp, err := lb.client.UpdateLoadbalancerWithResponse(ctx, found.Id, updateBody)
		if err != nil {
			return nil, fmt.Errorf("failed to update load balancer: %w", err)
		}
		if resp.JSON200 == nil {
			return nil, fmt.Errorf("unexpected response updating load balancer: %s, body: %s", resp.HTTPResponse.Status, string(resp.Body))
		}
		result = &resp.JSON200.Loadbalancer
	}

	if result.ExternalIp != nil {
		return &v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{{IP: *result.ExternalIp}},
		}, nil
	}

	// The LB was created/updated but has no external IP yet (still provisioning).
	// Poll until it becomes available so the service controller can set the ingress.
	klog.Infof("waiting for load balancer %q (%s) to get an external IP", name, result.Id)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}

		resp, err := lb.client.GetLoadbalancerWithResponse(ctx, result.Id)
		if err != nil {
			return nil, fmt.Errorf("failed to get load balancer: %w", err)
		}
		if resp.JSON200 == nil {
			return nil, fmt.Errorf("unexpected response getting load balancer: %s, body: %s", resp.HTTPResponse.Status, string(resp.Body))
		}
		cur := &resp.JSON200.Loadbalancer
		klog.Infof("load balancer %q status=%s externalIp=%v", name, cur.Status, cur.ExternalIp)

		if cur.Status == sagadata.LoadbalancerStatusError {
			return nil, fmt.Errorf("load balancer %q entered error state", name)
		}
		if cur.ExternalIp != nil {
			return &v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{{IP: *cur.ExternalIp}},
			}, nil
		}
	}
}

// UpdateLoadBalancer updates the load balancer targets for the given service.
func (lb *loadBalancers) UpdateLoadBalancer(ctx context.Context, clusterName string, svc *v1.Service, nodes []*v1.Node) error {
	name := lbName(svc)
	found, err := lb.lbByName(ctx, name)
	if err != nil {
		return err
	}
	if found == nil {
		return fmt.Errorf("load balancer %q not found", name)
	}

	ports := buildPorts(svc, nodes)
	updateBody := sagadata.UpdateLoadbalancerJSONRequestBody{
		Ports: &ports,
	}
	if bodyJSON, err := json.Marshal(updateBody); err == nil {
		klog.Infof("UpdateLoadBalancer %q (%s), request body: %s", name, found.Id, string(bodyJSON))
	}
	resp, err := lb.client.UpdateLoadbalancerWithResponse(ctx, found.Id, updateBody)
	if err != nil {
		return fmt.Errorf("failed to update load balancer: %w", err)
	}
	if resp.JSON200 == nil {
		return fmt.Errorf("unexpected response updating load balancer: %s, body: %s", resp.HTTPResponse.Status, string(resp.Body))
	}
	return nil
}

// EnsureLoadBalancerDeleted deletes the load balancer for the given service if it exists.
func (lb *loadBalancers) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, svc *v1.Service) error {
	name := lbName(svc)
	found, err := lb.lbByName(ctx, name)
	if err != nil {
		return err
	}
	if found == nil {
		return nil
	}

	klog.Infof("deleting load balancer %q (%s)", name, found.Id)
	resp, err := lb.client.DeleteLoadbalancerWithResponse(ctx, found.Id)
	if err != nil {
		return fmt.Errorf("failed to delete load balancer: %w", err)
	}
	if resp.JSONDefault != nil {
		return fmt.Errorf("error deleting load balancer: %s", resp.HTTPResponse.Status)
	}
	return nil
}

// annotateService patches the Service with the LB name annotation if not already set.
func (lb *loadBalancers) annotateService(ctx context.Context, svc *v1.Service, name string) error {
	if svc.Annotations[AnnotationLoadBalancerName] == name {
		return nil
	}
	patch := fmt.Sprintf(`{"metadata":{"annotations":{%q:%q}}}`, AnnotationLoadBalancerName, name)
	_, err := lb.kubeClient.CoreV1().Services(svc.Namespace).Patch(
		ctx, svc.Name, types.MergePatchType, []byte(patch), metav1.PatchOptions{},
	)
	return err
}
