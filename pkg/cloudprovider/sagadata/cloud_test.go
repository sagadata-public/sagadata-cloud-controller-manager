// Copyright 2025 Saga Data AS. All rights reserved.
// Use of this source code is governed by the Mozilla Public License, v. 2.0.

package sagadata

import (
	"testing"

	cloudprovider "k8s.io/cloud-provider"
)

func TestProviderRegistered(t *testing.T) {
	cloud, err := newCloud(nil)
	if err != nil {
		t.Fatalf("newCloud: %v", err)
	}
	if cloud.ProviderName() != ProviderName {
		t.Errorf("ProviderName() = %q, want %q", cloud.ProviderName(), ProviderName)
	}
	if !cloud.HasClusterID() {
		t.Error("HasClusterID() = false, want true")
	}
	if _, ok := cloud.LoadBalancer(); ok {
		t.Error("LoadBalancer() supported, want unsupported")
	}
	if _, ok := cloud.Instances(); ok {
		t.Error("Instances() supported, want unsupported")
	}
	if _, ok := cloud.InstancesV2(); ok {
		t.Error("InstancesV2() supported, want unsupported")
	}
	if _, ok := cloud.Zones(); ok {
		t.Error("Zones() supported, want unsupported")
	}
	if _, ok := cloud.Clusters(); ok {
		t.Error("Clusters() supported, want unsupported")
	}
	if _, ok := cloud.Routes(); ok {
		t.Error("Routes() supported, want unsupported")
	}
}

// Ensure our cloud type satisfies cloudprovider.Interface at compile time.
var _ cloudprovider.Interface = (*cloud)(nil)
