# VirtFoundry Operator

Kubernetes operator for VirtFoundry private cloud (`virtfoundry.io` CRDs).

Canonical desired state lives in Custom Resources. The REST API and UI in
[virtfoundry/core](https://github.com/virtfoundry/core) are optional clients of
that API (adoption / GitOps-friendly layer).

## Controllers (v1alpha1)

| Kind | Reconciler | Notes |
|------|------------|-------|
| Tenant | Namespace + status | Creates `virtfoundry-tenant-{slug}` namespace |
| Instance | KubeVirt status sync | Writes `status.phase`, `status.ip`, `status.kubevirtName` from VM/VMI |

Other kinds (VPC, Network, Disk, Instance create/delete) are defined as CRDs; controllers are planned per [core design spec](https://github.com/virtfoundry/core/blob/main/docs/superpowers/specs/2026-09-01-crd-operator-design.md).

### Tenant namespace safety

The Tenant reconciler only writes to `virtfoundry-tenant-{slug}` namespaces that
carry `virtfoundry.io/tenant={slug}` and either no controller ownerRef (adopted
once) or an ownerRef pointing at that Tenant. Anything else — system namespaces,
unlabelled namespaces, another Tenant's namespace — is refused, and the Tenant
reports `status.phase: Failed` instead of adopting or deleting it.

The chart adds a matching cluster-side guard: `namespaceGuard.enabled` (default
`true`) installs a ValidatingAdmissionPolicy that denies the operator
ServiceAccount any Namespace `DELETE` outside that set. It renders only on
clusters serving `admissionregistration.k8s.io/v1` policies (Kubernetes >= 1.30).

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
