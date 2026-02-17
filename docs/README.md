# Sagadata Cloud Controller Manager — Documentation

This document describes the Sagadata Kubernetes Cloud Controller Manager (CCM): what it is, how it is built, and how to use and extend it.

---

## 1. Overview

### 1.1 Purpose

The **Sagadata Cloud Controller Manager** is an [out-of-tree](https://kubernetes.io/docs/tasks/administer-cluster/developing-cloud-controller-manager/#out-of-tree) Kubernetes Cloud Controller Manager for [Saga Data](https://www.sagadata.no/). It implements the standard [`cloudprovider.Interface`](https://github.com/kubernetes/cloud-provider/blob/master/cloud.go) from [k8s.io/cloud-provider](https://github.com/kubernetes/cloud-provider), so it can be used as the `--cloud-provider` backend for a Kubernetes control plane.

The CCM embeds cloud-specific control logic (or placeholders for it) and runs the shared controllers (Node, Route, Service) provided by Kubernetes. The current version is a **minimal implementation**: it registers the provider and runs the framework but does not yet implement Node, Route, or LoadBalancer behaviour. It is intended as a starting point to add real integration with the [Saga Data API](https://developers.sagadata.no/).

### 1.2 Relationship to other projects

| Project | Role |
|--------|------|
| **sagadata-cloud-controller-manager** (this repo) | Kubernetes integration: implements `cloudprovider.Interface` and runs as a CCM binary. |
| [terraform-provider-sagadata](https://github.com/sagadata-public/terraform-provider-sagadata) | Terraform integration: manages Saga Data resources via Terraform. Same cloud, different interface. |
| [Saga Data API](https://developers.sagadata.no/) | Backend for instances, networking, etc. Can be used by both the CCM and the Terraform provider. |

This CCM does not depend on the Terraform provider. Future versions may share client code (e.g. [sagadata-go](https://github.com/sagadata-public/sagadata-go)) with the Terraform provider.

### 1.3 License

The project is licensed under the **Mozilla Public License Version 2.0** (MPL 2.0). See [LICENSE.txt](../LICENSE.txt) in the repository root. This aligns with the Terraform provider for Saga Data.

---

## 2. Architecture

### 2.1 Kubernetes Cloud Controller Manager

The [Cloud Controller Manager](https://kubernetes.io/docs/concepts/architecture/cloud-controller/) is a control-plane component that:

- Runs **cloud-specific** logic (node lifecycle, routes, load balancers).
- Uses a **pluggable** cloud backend: the binary is built once, and the backend is selected by `--cloud-provider=<name>` and must implement `cloudprovider.Interface`.

Shared controllers (Node, Route, Service) live in Kubernetes / k8s.io/cloud-provider; the cloud provider supplies the actual cloud API behaviour via the interface.

### 2.2 Out-of-tree model

We do **not** vendor or copy `k8s.io/kubernetes/cmd/cloud-controller-manager`. Instead we:

1. Depend on **k8s.io/cloud-provider** (and related packages) as a library.
2. Implement **cloudprovider.Interface** in our own package (`pkg/cloudprovider/sagadata`).
3. **Register** our provider in an `init()` so that `cloudprovider.InitCloudProvider("sagadata", configFile)` can instantiate it.
4. Build our own **binary** that uses the shared CCM app and our cloud initializer.

This follows the [official “Developing Cloud Controller Manager”](https://kubernetes.io/docs/tasks/administer-cluster/developing-cloud-controller-manager/#out-of-tree) and the [k8s.io/cloud-provider sample](https://github.com/kubernetes/cloud-provider/blob/master/sample/basic_main.go).

### 2.3 High-level flow

```
┌─────────────────────────────────────────────────────────────────┐
│  cloud-controller-manager binary (our main.go)                   │
│  - Parses flags (--cloud-provider, --kubeconfig, etc.)            │
│  - Builds CCM options, default --cloud-provider=sagadata        │
│  - Calls app.NewCloudControllerManagerCommand(..., cloudInit)     │
└──────────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│  k8s.io/cloud-provider/app                                       │
│  - Runs shared controllers (node, route, service)                 │
│  - Calls cloudInitializer(config) to get cloudprovider.Interface  │
└──────────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│  cloudInitializer                                                │
│  - cloudprovider.InitCloudProvider(name, configFile)             │
│  - Looks up "sagadata" in the registry (set by our init())        │
│  - Calls our factory with config io.Reader → returns *cloud      │
└──────────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│  pkg/cloudprovider/sagadata (*cloud)                             │
│  - Implements cloudprovider.Interface                            │
│  - Initialize(), ProviderName(), HasClusterID()                 │
│  - LoadBalancer() (nil, false), Instances() (nil, false), etc.   │
└─────────────────────────────────────────────────────────────────┘
```

---

## 3. Project structure

```
sagadata-cloud-controller-manager/
├── cmd/
│   └── cloud-controller-manager/
│       └── main.go              # CCM entrypoint: options, cloud initializer, run
├── pkg/
│   └── cloudprovider/
│       └── sagadata/
│           ├── doc.go           # Package comment
│           ├── cloud.go          # cloud type, newCloud(), init(), Interface impl
│           └── cloud_test.go     # Unit tests + interface compile check
├── docs/
│   └── README.md                # This documentation
├── go.mod
├── go.sum
├── LICENSE.txt
├── README.md
└── .gitignore
```

| Path | Purpose |
|------|--------|
| `cmd/cloud-controller-manager/main.go` | Builds CCM options, sets default cloud provider to `sagadata`, defines `cloudInitializer` that calls `cloudprovider.InitCloudProvider`, and runs the CCM command. Imports the sagadata package so its `init()` runs and the provider is registered. |
| `pkg/cloudprovider/sagadata/cloud.go` | Defines `ProviderName`, the `cloud` struct, `newCloud(config)`, and `init()` that registers the provider. Implements all methods of `cloudprovider.Interface`; optional sub-interfaces return `(nil, false)`. |
| `pkg/cloudprovider/sagadata/cloud_test.go` | Tests that `newCloud(nil)` works, `ProviderName()` and `HasClusterID()` are correct, and that no optional interface is reported as supported. Includes `var _ cloudprovider.Interface = (*cloud)(nil)` for compile-time verification. |

---

## 4. The cloudprovider.Interface contract

Our `cloud` type implements [cloudprovider.Interface](https://github.com/kubernetes/cloud-provider/blob/master/cloud.go). Summary:

| Method | Our implementation |
|--------|---------------------|
| `Initialize(clientBuilder, stop)` | No-op; logs that the minimal provider is initialized. |
| `LoadBalancer()` | Returns `(nil, false)` — Service controller will not use a custom LB. |
| `Instances()` | Returns `(nil, false)` — legacy node info not supported. |
| `InstancesV2()` | Returns `(nil, false)` — node metadata not supported yet. |
| `Zones()` | Returns `(nil, false)`. |
| `Clusters()` | Returns `(nil, false)`. |
| `Routes()` | Returns `(nil, false)` — Route controller will not program routes. |
| `ProviderName()` | Returns `"sagadata"`. |
| `HasClusterID()` | Returns `false` — cluster can run without a ClusterID if `--allow-untagged-cloud` is set. |

Returning `(nil, false)` for an optional sub-interface means “this provider does not support that feature”. The shared controllers check the boolean and skip or no-op accordingly. No need to implement the actual `LoadBalancer`, `Instances`, or `Routes` types until you add real behaviour.

---

## 5. Building

### 5.1 Requirements

- **Go 1.22 or later** (the module may require a higher toolchain; see `go.mod`).
- No need for a Kubernetes cluster to build.

### 5.2 Build command

From the repository root:

```bash
go build -o cloud-controller-manager ./cmd/cloud-controller-manager
```

To resolve and tidy dependencies after changing `go.mod` or imports:

```bash
go mod tidy
```

### 5.3 Dependencies

The main dependencies are:

- `k8s.io/cloud-provider` — interface, app, options, names, InitCloudProvider, RegisterCloudProvider.
- `k8s.io/component-base` — cli, flags, metrics.
- `k8s.io/apimachinery` — e.g. `util/wait`.
- `k8s.io/klog/v2` — logging.

All other dependencies are transitive. See `go.mod` and `go.sum` for exact versions.

---

## 6. Running

### 6.1 Defaults

The binary defaults `--cloud-provider` to **sagadata**, so you can omit that flag. You still need to pass at least a kubeconfig for a real cluster.

### 6.2 Minimal run (no cluster ID)

For the minimal implementation, cloud-config is optional. To run without a cluster ID (e.g. for testing):

```bash
./cloud-controller-manager \
  --cloud-provider=sagadata \
  --kubeconfig=/path/to/kubeconfig \
  --allow-untagged-cloud
```

### 6.3 Full example

```bash
./cloud-controller-manager \
  --cloud-provider=sagadata \
  --kubeconfig=~/.kube/config \
  --cloud-config=/path/to/cloud-config \   # optional
  --allow-untagged-cloud
```

### 6.4 Key flags

| Flag | Description |
|------|-------------|
| `--cloud-provider` | Cloud provider name; default is `sagadata`. |
| `--kubeconfig` | Path to kubeconfig for API server and auth. |
| `--cloud-config` | Optional path to provider config file (passed as `io.Reader` to our factory). |
| `--allow-untagged-cloud` | Allow running when the cluster has no ClusterID (our minimal impl has `HasClusterID() == false`; this avoids a fatal when that’s the case). |

All other CCM flags (auth, leader election, bind address, etc.) are the same as for any Kubernetes CCM. See:

```bash
./cloud-controller-manager --help
```

### 6.5 What runs

When the CCM starts:

1. Our package’s `init()` has already registered the provider name `"sagadata"`.
2. The app calls `cloudInitializer`, which calls `cloudprovider.InitCloudProvider("sagadata", cloudConfigFile)`.
3. Our factory is invoked and returns a `*cloud` instance.
4. The shared Node, Route, and Service controllers start. With our current implementation they get no Instances, Routes, or LoadBalancer, so they effectively no-op or skip cloud-specific work.
5. You should see a log line like: `Sagadata cloud provider initialized (minimal implementation)`.

---

## 7. Testing

### 7.1 Unit tests

The package `pkg/cloudprovider/sagadata` has a test file `cloud_test.go` that:

- Instantiates the provider with `newCloud(nil)`.
- Asserts `ProviderName()` is `"sagadata"` and `HasClusterID()` is `false`.
- Asserts that `LoadBalancer()`, `Instances()`, `InstancesV2()`, `Zones()`, `Clusters()`, and `Routes()` all return `(_, false)`.
- Uses `var _ cloudprovider.Interface = (*cloud)(nil)` to ensure the type implements the full interface at compile time.

Run the tests:

```bash
go test ./pkg/cloudprovider/sagadata/... -v
```

On environments where the Go toolchain produces runnable binaries (e.g. Linux), this proves the provider behaves as intended. On some macOS setups you may see a dyld/LC_UUID error when executing the test binary; in that case run the same command on Linux or in CI.

### 7.2 Proving it works end-to-end

1. **Build** — `go build -o cloud-controller-manager ./cmd/cloud-controller-manager` succeeds.
2. **Help** — `./cloud-controller-manager --help` prints CCM flags.
3. **Unit tests** — `go test ./pkg/cloudprovider/sagadata/...` passes (on a supported platform).
4. **Live run** — Run the CCM against a cluster (e.g. kind or minikube) with `--kubeconfig` and `--allow-untagged-cloud`; process stays up and logs show provider registration and initialization.

---

## 8. Configuration

### 8.1 Cloud config file

If you pass `--cloud-config=/path/to/file`, the CCM opens that file and passes an `io.Reader` to our `newCloud(config)` function. The minimal implementation ignores `config`. A future version can parse it (e.g. INI or YAML) for:

- API endpoint or region
- Cluster ID
- Credentials path or similar

Format is up to you; Kubernetes does not define it.

### 8.2 Cluster ID

We currently return `HasClusterID() == false`. The CCM framework may require a cluster ID unless `--allow-untagged-cloud` is set. For production you may later:

- Set `HasClusterID() == true` and require a cluster ID in cloud-config or environment, or
- Keep `false` and rely on `--allow-untagged-cloud` where appropriate.

---

## 9. Extending the provider

To add real behaviour:

1. **InstancesV2** — Implement `InstancesV2()` to return a type that implements `InstanceExists`, `InstanceShutdown`, and `InstanceMetadata`. The node controller uses this to sync nodes with the cloud. You will need to call the Saga Data API (or a shared client such as sagadata-go) to resolve node names or provider IDs to instance metadata.
2. **Routes** — If your cluster needs cloud-managed routes, implement `Routes()` and the `ListRoutes`, `CreateRoute`, `DeleteRoute` methods using the cloud API.
3. **LoadBalancer** — To support `LoadBalancer` Services, implement `LoadBalancer()` and the Get/Ensure/Update/Delete methods; integrate with Saga Data load balancer or networking APIs.
4. **Cloud config** — In `newCloud(config)`, read from `config` (e.g. decode INI/YAML) and pass cluster ID, API URL, or credentials into the `cloud` struct.

Keep the same registration pattern: `init()` calling `cloudprovider.RegisterCloudProvider(ProviderName, factory)` and the factory returning a type that implements `cloudprovider.Interface`.

---

## 10. References

- [Cloud Controller Manager (concepts)](https://kubernetes.io/docs/concepts/architecture/cloud-controller/)
- [Developing Cloud Controller Manager (out-of-tree)](https://kubernetes.io/docs/tasks/administer-cluster/developing-cloud-controller-manager/#out-of-tree)
- [k8s.io/cloud-provider](https://github.com/kubernetes/cloud-provider) — `cloud.go` (interface), `plugins.go` (RegisterCloudProvider, InitCloudProvider), [sample/basic_main.go](https://github.com/kubernetes/cloud-provider/blob/master/sample/basic_main.go)
- [Saga Data](https://www.sagadata.no/) — [API documentation](https://developers.sagadata.no/)
- [Terraform provider for Saga Data](https://github.com/sagadata-public/terraform-provider-sagadata) — same cloud, Terraform interface; MPL 2.0
