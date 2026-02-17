# Sagadata Cloud Controller Manager

Kubernetes [Cloud Controller Manager](https://kubernetes.io/docs/concepts/architecture/cloud-controller/) (CCM) for [Saga Data](https://www.sagadata.no/). This is an out-of-tree implementation that satisfies the `cloudprovider.Interface` from [k8s.io/cloud-provider](https://github.com/kubernetes/cloud-provider).

**Full documentation:** [docs/README.md](docs/README.md) (architecture, project structure, interface contract, building, running, testing, extending).

The current version is a **minimal implementation**: it registers the provider and runs the CCM framework but does not yet implement Node, Route, or LoadBalancer logic. You can extend it with real cloud integration (e.g. using the [Saga Data API](https://developers.sagadata.no/)).

This project is licensed under the [Mozilla Public License 2.0](LICENSE.txt) (MPL 2.0), in line with the [Terraform provider for Saga Data](https://github.com/sagadata-public/terraform-provider-sagadata).

## Requirements

- [Go](https://go.dev/doc/install) 1.22 or later
- A Kubernetes cluster (for running the CCM; the minimal CCM can also be started with `--help` to verify the binary)

## Building

From the repository root:

```bash
go build -o cloud-controller-manager ./cmd/cloud-controller-manager
```

## Running

The binary defaults to `--cloud-provider=sagadata`. You still need to pass kubeconfig and (optionally) cloud-config:

```bash
./cloud-controller-manager \
  --cloud-provider=sagadata \
  --kubeconfig=/path/to/kubeconfig \
  --cloud-config=/path/to/cloud-config  # optional
```

For the minimal implementation, cloud-config can be omitted. To allow running without a cluster ID (e.g. for testing), use:

```bash
./cloud-controller-manager --cloud-provider=sagadata --allow-untagged-cloud ...
```

See `./cloud-controller-manager --help` for all options.

## License

This repository is licensed under the Mozilla Public License Version 2.0 (see [LICENSE.txt](LICENSE.txt)).
