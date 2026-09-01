# VirtFoundry Operator

Kubernetes operator for VirtFoundry private cloud (`virtfoundry.io` CRDs).

Canonical desired state lives in Custom Resources. The REST API and UI in
[virtfoundry/core](https://github.com/virtfoundry/core) are optional clients of
that API (adoption / GitOps-friendly layer).

## Develop

```bash
make generate manifests
make test
make build
```

## Install (kind)

```bash
kind create cluster --name virtfoundry-op
make install
make deploy IMG=virtfoundry-operator:dev
# or for local iterate:
make run
kubectl apply -f config/samples/virtfoundry_v1alpha1_tenant.yaml
kubectl get vf-tenant
```

## License

Apache-2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE).

## Governance

[GOVERNANCE.md](GOVERNANCE.md) · [MAINTAINERS.md](MAINTAINERS.md) · [CONTRIBUTING.md](CONTRIBUTING.md) · [SECURITY.md](SECURITY.md) · [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
