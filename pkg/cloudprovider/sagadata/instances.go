package sagadata

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	sagadata "github.com/sagadata-public/sagadata-go"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

const providerIDPrefix = "sagadata://"

type instances struct {
	client *sagadata.ClientWithResponses
}

// parseProviderID extracts the instance ID from a provider ID of the form
// "sagadata://<instance-id>". It returns an error if the provider ID is empty
// or does not have the expected prefix.
func parseProviderID(providerID string) (string, error) {
	if providerID == "" {
		return "", fmt.Errorf("provider ID is empty")
	}
	if !strings.HasPrefix(providerID, providerIDPrefix) {
		return "", fmt.Errorf("provider ID %q missing %q prefix", providerID, providerIDPrefix)
	}
	instanceID := strings.TrimPrefix(providerID, providerIDPrefix)
	return instanceID, nil
}

// NodeAddresses returns the addresses of the specified instance.
func (i *instances) NodeAddresses(ctx context.Context, name types.NodeName) ([]v1.NodeAddress, error) {
	inst, err := i.instanceByNodeName(ctx, name)
	if err != nil {
		return nil, err
	}
	return i.nodeAddresses(ctx, inst)
}

// NodeAddressesByProviderID returns the addresses of the specified instance.
// The instance is specified using the providerID of the node. The
// ProviderID is a unique identifier of the node. This will not be called
// from the node whose nodeaddresses are being queried. i.e. local metadata
// services cannot be used in this method to obtain nodeaddresses
func (i *instances) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	instanceID, err := parseProviderID(providerID)
	if err != nil {
		return nil, err
	}

	resp, err := i.client.GetInstanceWithResponse(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, cloudprovider.InstanceNotFound
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %s", resp.Status())
	}

	return i.nodeAddresses(ctx, &resp.JSON200.Instance)
}

// instanceByNodeName lists all instances and returns the one whose hostname
// matches the given Kubernetes node name.
func (i *instances) instanceByNodeName(ctx context.Context, name types.NodeName) (*sagadata.Instance, error) {
	nodeName := string(name)
	page := 1
	for {
		resp, err := i.client.ListInstancesPaginatedWithResponse(ctx, &sagadata.ListInstancesPaginatedParams{
			Page: &page,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list instances: %w", err)
		}
		if resp.JSON200 == nil {
			return nil, fmt.Errorf("unexpected response: %s", resp.Status())
		}
		for idx := range resp.JSON200.Instances {
			inst := &resp.JSON200.Instances[idx]
			if inst.Hostname == nodeName || inst.Name == nodeName {
				return inst, nil
			}
		}
		if page*resp.JSON200.PerPage >= resp.JSON200.TotalCount {
			break
		}
		page++
	}
	return nil, cloudprovider.InstanceNotFound
}

func (i *instances) nodeAddresses(ctx context.Context, inst *sagadata.Instance) ([]v1.NodeAddress, error) {
	var addresses []v1.NodeAddress

	addresses = append(addresses, v1.NodeAddress{
		Type:    v1.NodeHostName,
		Address: inst.Hostname,
	})

	if inst.PrivateIp != nil {
		addresses = append(addresses, v1.NodeAddress{
			Type:    v1.NodeInternalIP,
			Address: *inst.PrivateIp,
		})
	}

	if inst.PublicIp != nil {
		addresses = append(addresses, v1.NodeAddress{
			Type:    v1.NodeExternalIP,
			Address: *inst.PublicIp,
		})
	}

	if inst.FloatingIp != nil {
		resp, err := i.client.GetFloatingIPWithResponse(ctx, inst.FloatingIp.Id)
		if err != nil {
			klog.Warningf("failed to get floating IP %s for instance %s: %v", inst.FloatingIp.Id, inst.Id, err)
		} else if resp.JSON200 != nil && resp.JSON200.FloatingIp.IpAddress != nil {
			addrType := v1.NodeExternalIP
			if !resp.JSON200.FloatingIp.IsPublic {
				addrType = v1.NodeInternalIP
			}
			addresses = append(addresses, v1.NodeAddress{
				Type:    addrType,
				Address: *resp.JSON200.FloatingIp.IpAddress,
			})
		}
	}

	klog.Infof("instance %s (%s): resolved %d address(es):", inst.Id, inst.Hostname, len(addresses))
	for _, a := range addresses {
		klog.Infof("  %s = %s", a.Type, a.Address)
	}

	return addresses, nil
}

// InstanceID returns the cloud provider ID of the node with the specified NodeName.
// Note that if the instance does not exist, we must return ("", cloudprovider.InstanceNotFound)
// cloudprovider.InstanceNotFound should NOT be returned for instances that exist but are stopped/sleeping
func (i *instances) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	inst, err := i.instanceByNodeName(ctx, nodeName)
	if err != nil {
		return "", err
	}
	klog.Infof("InstanceID: nodeName=%q instanceID=%q", nodeName, inst.Id)
	return inst.Id, nil
}

// InstanceType returns the type of the specified instance.
func (i *instances) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	inst, err := i.instanceByNodeName(ctx, name)
	if err != nil {
		return "", err
	}
	return inst.Type, nil
}

// InstanceTypeByProviderID returns the type of the specified instance.
func (i *instances) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	instanceID, err := parseProviderID(providerID)
	if err != nil {
		return "", err
	}

	resp, err := i.client.GetInstanceWithResponse(ctx, instanceID)
	if err != nil {
		return "", fmt.Errorf("failed to get instance: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return "", cloudprovider.InstanceNotFound
	}

	if resp.JSON200 == nil {
		return "", fmt.Errorf("unexpected response: %s", resp.Status())
	}

	return resp.JSON200.Instance.Type, nil
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all instances
// expected format for the key is standard ssh-keygen format: <protocol> <blob>
func (i *instances) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	return cloudprovider.NotImplemented
}

// CurrentNodeName returns the name of the node we are currently running on
// On most clouds (e.g. GCE) this is the hostname, so we provide the hostname
func (i *instances) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	return "", cloudprovider.NotImplemented
}

// InstanceExistsByProviderID returns true if the instance for the given provider exists.
// If false is returned with no error, the instance will be immediately deleted by the cloud controller manager.
// This method should still return true for instances that exist but are stopped/sleeping.
func (i *instances) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	instanceID, err := parseProviderID(providerID)
	klog.Infof("InstanceExistsByProviderID: instanceID=%q", instanceID)
	if err != nil {
		return false, err
	}

	resp, err := i.client.GetInstanceWithResponse(ctx, instanceID)
	if err != nil {
		return false, fmt.Errorf("failed to get instance: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return false, nil
	}

	if resp.JSON200 == nil {
		return false, fmt.Errorf("unexpected response: %s", resp.Status())
	}

	return true, nil
}

// InstanceShutdownByProviderID returns true if the instance is shutdown in cloudprovider
func (i *instances) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	instanceID, err := parseProviderID(providerID)
	if err != nil {
		return false, err
	}

	resp, err := i.client.GetInstanceWithResponse(ctx, instanceID)
	if err != nil {
		return false, fmt.Errorf("failed to get instance: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return false, cloudprovider.InstanceNotFound
	}

	if resp.JSON200 == nil {
		return false, fmt.Errorf("unexpected response: %s", resp.Status())
	}

	return resp.JSON200.Instance.Status == sagadata.InstanceStatusStopped, nil
}

func NewInstances(client *sagadata.ClientWithResponses) (cloudprovider.Instances, error) {
	return &instances{client: client}, nil
}
