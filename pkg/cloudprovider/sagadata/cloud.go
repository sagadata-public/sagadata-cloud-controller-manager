// Copyright 2025 Saga Data AS. All rights reserved.
// Use of this source code is governed by the Mozilla Public License, v. 2.0.

package sagadata

import (
	"fmt"
	"io"
	"os"

	sagadata "github.com/sagadata-public/sagadata-go"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

const (
	// ProviderName is the name used when registering and when passing --cloud-provider=sagadata.
	ProviderName = "sagadata"
)

// cloud implements cloudprovider.Interface
type cloud struct {
	instances cloudprovider.Instances
}

// newCloud returns a new cloudprovider.Interface for Saga Data.
func newCloud(config io.Reader) (cloudprovider.Interface, error) {
	endpoint := os.Getenv("ENDPOINT")
	if endpoint == "" {
		return nil, fmt.Errorf("ENDPOINT environment variable not set")
	}

	tokenFile := os.Getenv("TOKEN_FILE")
	if tokenFile == "" {
		return nil, fmt.Errorf("TOKEN_FILE environment variable not set")
	}

	client, err := sagadata.NewSagaDataClient(sagadata.ClientConfig{
		Endpoint:  endpoint,
		TokenFile: tokenFile,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create sagadata client: %w", err)
	}

	c := &cloud{}
	i, err := NewInstances(client)
	if err != nil {
		return nil, err
	}
	c.instances = i
	return c, nil
}

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		return newCloud(config)
	})
}

// Initialize provides the cloud with a kubernetes client builder.
func (c *cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	klog.Info("Sagadata cloud provider initialized")
}

// LoadBalancer returns a balancer interface. Not supported in the minimal implementation.
func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

// Instances returns an instances interface. Not supported in the minimal implementation.
func (c *cloud) Instances() (cloudprovider.Instances, bool) {
	return c.instances, true
}

// InstancesV2 returns an instances interface. Not supported in the minimal implementation.
func (c *cloud) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return nil, false
}

// Zones returns a zones interface. Not supported in the minimal implementation.
func (c *cloud) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

// Clusters returns a clusters interface. Not supported in the minimal implementation.
func (c *cloud) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// Routes returns a routes interface. Not supported in the minimal implementation.
func (c *cloud) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (c *cloud) ProviderName() string {
	return ProviderName
}

// HasClusterID returns true if a ClusterID is required.
func (c *cloud) HasClusterID() bool {
	return true
}
