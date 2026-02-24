// Copyright 2025 Saga Data AS. All rights reserved.
// Use of this source code is governed by the Mozilla Public License, v. 2.0.

package sagadata

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	cloudprovider "k8s.io/cloud-provider"
)

func TestProviderRegistered(t *testing.T) {
	srv := httptest.NewServer(http.NewServeMux())
	t.Cleanup(srv.Close)

	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("test-token"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ENDPOINT", srv.URL)
	t.Setenv("TOKEN_FILE", tokenFile)
	t.Setenv("REGION", "NORD-NO-KRS-1")
	t.Setenv("NETWORK", "test-network")

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
	if _, ok := cloud.LoadBalancer(); !ok {
		t.Error("LoadBalancer() not supported, want supported")
	}
	if _, ok := cloud.Instances(); !ok {
		t.Error("Instances() not supported, want supported")
	}
	if _, ok := cloud.InstancesV2(); !ok {
		t.Error("InstancesV2() not supported, want supported")
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
