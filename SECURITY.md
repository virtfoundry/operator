# Security Policy

## Supported versions

| Version | Supported |
|---------|-----------|
| `main` branch | yes |
| tagged releases | yes |

## Reporting a vulnerability

**Do not open public GitHub issues for security vulnerabilities.**

Report via a **private GitHub security advisory** on this repository or [virtfoundry/core](https://github.com/virtfoundry/core/security/advisories). Primary contact: **Matheus Thurler** ([@Matheus-Thurler](https://github.com/Matheus-Thurler)) — see [MAINTAINERS.md](MAINTAINERS.md).

Include affected component (CRDs, controllers, Helm chart), impact, reproduction steps, and suggested fix if any.

We aim to acknowledge within **7 days**.

## Secure deployment

- Credential hashes belong only in Kubernetes Secrets (`secretRef`), never in CR `spec`
- Cloud-init user-data with guest passwords belongs in a Secret via
  `Template.spec.cloudInitSecretRef` / `Instance.spec.cloudInitSecretRef`
  (default key `userData`). Inline `cloudInitUserData` remains for migration only
  and is deprecated on Template
- Operator ClusterRole may **get/list/watch** Secrets (read-only) to resolve
  cloud-init refs; mutate verbs on Secrets stay forbidden
- Helm ClusterRole is scoped to the **running** controllers (Tenant + Instance + Network today: tenants/instances/networks CRs, offerings/templates/sshkeys read, namespaces for tenants, KubeVirt VMs/VMIs, Secrets read). It is not absolute least privilege cluster-wide — Namespace `delete` and KubeVirt VM/VMI mutate remain cluster-scoped in RBAC and are constrained by reconciler ownership guards plus chart ValidatingAdmissionPolicies (`namespaceGuard`, `kubevirtGuard`). Expand RBAC only when a new controller lands; keep `charts/virtfoundry-operator/templates/rbac.yaml` aligned with `config/rbac/role.yaml`
- Pin operator image by digest in production overlays
- Prefer clusters with ValidatingAdmissionPolicy (Kubernetes >= 1.30) so
  `namespaceGuard`, `kubevirtGuard`, and `crAdmission` chart policies render
- KubeVirt VM/VMI mutate (issue #28): stolen operator SA is denied CREATE/UPDATE/DELETE
  outside `virtfoundry-tenant-*` by `kubevirtGuard`. Residual: Role/RoleBinding
  per tenant namespace (RBAC privilege-escalation rules block the operator from
  minting mutate Roles without already holding those verbs)
- Offering CPU/memory are bounded in the CRD schema; Instances must live in
  `virtfoundry-tenant-*` namespaces; Template `sourceType: container` images
  must match the ContainerDisk allowlist at admission (`crAdmission`) and
  again in the Instance reconciler
- Residual admission hardening (issue #26): validating webhooks + cert-manager
  + Helm `:9443`, admission-time Tenant slug uniqueness, privileged KubeVirt
  feature rejection
- Operator logs use zap `Development: false` by default; never log cloud-init bodies
