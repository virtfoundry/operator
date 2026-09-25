# VirtFoundry Operator

Kubernetes operator for VirtFoundry private cloud (`virtfoundry.io` CRDs).

Canonical desired state lives in Custom Resources. The REST API and UI in
[virtfoundry/core](https://github.com/virtfoundry/core) are optional clients of
that API (adoption / GitOps-friendly layer).

## Controllers (v1alpha1)

| Kind | Reconciler | Notes |
|------|------------|-------|
| Tenant | Namespace + isolation + status | Creates `virtfoundry-tenant-{slug}` with PSA, default-deny NetworkPolicy, ResourceQuota, LimitRange |
| Instance | KubeVirt VM + status sync | Writes VM/VMI status; Multus/VPC NICs by default (pod network opt-in only) |

Other kinds (VPC, Network, Disk, Instance create/delete) are defined as CRDs; controllers are planned per [core design spec](https://github.com/virtfoundry/core/blob/main/docs/superpowers/specs/2026-09-01-crd-operator-design.md).

### Tenant namespace safety

`spec.slug` must be unique across Tenants. The controller indexes `.spec.slug`
and marks colliding Tenants `Failed` (terminal) so they never share or wipe a
namespace. Admission-time uniqueness is deferred to validating webhooks
(follow-up #26).

The reconciler only writes to `virtfoundry-tenant-{slug}` namespaces that carry
`virtfoundry.io/tenant={slug}` and either no controller ownerRef (adopted once)
or an ownerRef pointing at that Tenant. Deletes require label **and** a matching
controller ownerRef. Anything else — system namespaces, unlabelled namespaces,
another Tenant's namespace, label-only legacy namespaces — is refused, and the
Tenant reports `status.phase: Failed` (or drops its finalizer on delete) instead
of adopting or wiping it.

The chart adds a matching cluster-side guard: `namespaceGuard.enabled` (default
`true`) installs a ValidatingAdmissionPolicy that denies the operator
ServiceAccount any Namespace `DELETE` outside that set. It renders only on
clusters serving `admissionregistration.k8s.io/v1` policies (Kubernetes >= 1.30).

### CR admission (Instance / Offering)

`crAdmission.enabled` (default `true`) installs a ValidatingAdmissionPolicy that:

- Rejects `Instance` CREATE/UPDATE outside `virtfoundry-tenant-*`
- Rejects `Offering` with CPU outside `1..256` or `memoryMi` outside `64..1048576`

Offering bounds are also in the CRD OpenAPI schema. The Instance reconciler
refuses out-of-namespace Instances and out-of-bounds Offerings with
`status.phase: Failed` (defense in depth when VAP is unavailable). Quantity
parsing never uses `resource.MustParse` on guest CPU/memory.

**Not in this slice (tracked in [#26](https://github.com/virtfoundry/operator/issues/26)):**

- Validating webhooks + cert-manager + Helm `:9443` (including admission-time slug uniqueness)
- Template image allowlist / privileged KubeVirt feature rejection

The manager no longer starts an empty webhook TLS server.

### Tenant isolation defaults

On every successful Tenant reconcile the operator also ensures:

| Object | Name | Purpose |
|--------|------|---------|
| PSA labels | `pod-security.kubernetes.io/{enforce,audit,warn}=privileged` | Required for KubeVirt virt-launcher |
| NetworkPolicy | `virtfoundry-default-deny` | Default-deny ingress/egress; allow DNS to kube-system, same-namespace, `kubevirt`/`cdi` namespaces, and TCP 80/443 for containerDisk/CDI pulls |
| ResourceQuota | `virtfoundry-default` | Caps pods, CPU/memory requests+limits, PVCs, Services |
| LimitRange | `virtfoundry-default` | Sensible container default request/limit/max |

Instance reconcile only runs in namespaces labelled `virtfoundry.io/tenant` under
the `virtfoundry-tenant-*` prefix. Guest VMs do **not** get the KubeVirt pod
network (masquerade) by default — attach Multus/VPC networks via `spec.nics`, or
opt in with annotation `virtfoundry.io/allow-pod-network=true` (breaking change
vs ≤0.7). Image allowlists and `dedicatedCPU` Offering gates are follow-ups.

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
