// Copyright 2025 Saga Data AS. All rights reserved.
// Use of this source code is governed by the Mozilla Public License, v. 2.0.

package sagadata

import (
	"io"

	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

const (
	// ProviderName is the name used when registering and when passing --cloud-provider=sagadata.
	ProviderName = "sagadata"
)

// cloud implements cloudprovider.Interface with a minimal "do nothing" implementation.
type cloud struct{}

// newCloud returns a new cloudprovider.Interface for Saga Data.
// For the minimal version, config is ignored.
func newCloud(config io.Reader) (cloudprovider.Interface, error) {
	return &cloud{}, nil
}

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		return newCloud(config)
	})
}

// Initialize provides the cloud with a kubernetes client builder. No-op for the minimal implementation.
func (c *cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	klog.Info("Sagadata cloud provider initialized (minimal implementation)")
}

// LoadBalancer returns a balancer interface. Not supported in the minimal implementation.
func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

// Instances returns an instances interface. Not supported in the minimal implementation.
func (c *cloud) Instances() (cloudprovider.Instances, bool) {
	return nil, false
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

// HasClusterID returns true if a ClusterID is required. Minimal implementation returns false.
func (c *cloud) HasClusterID() bool {
	return false
}
