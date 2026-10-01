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
- Helm ClusterRole is scoped to the **running** controllers (Tenant + Instance + Network today: tenants/instances/networks CRs, offerings/templates/sshkeys read, namespaces for tenants, KubeVirt VMs/VMIs monitor, Secrets read, RoleBindings + bind on the kubevirt-mutate ClusterRole). It is not absolute least privilege cluster-wide — Namespace `delete` remains cluster-scoped in RBAC and is constrained by reconciler ownership guards plus chart ValidatingAdmissionPolicies (`namespaceGuard`, `kubevirtGuard`). Expand RBAC only when a new controller lands; keep `charts/virtfoundry-operator/templates/rbac.yaml` aligned with `config/rbac/role.yaml`
- Pin operator image by digest in production overlays
- Prefer clusters with ValidatingAdmissionPolicy (Kubernetes >= 1.30) so
  `namespaceGuard`, `kubevirtGuard`, and `crAdmission` chart policies render
- KubeVirt VM/VMI mutate (issue #28): manager ClusterRole is **monitor-only**
  (`get/list/watch`). Mutate verbs live on `*-kubevirt-mutate` and are granted
  per `virtfoundry-tenant-*` via Tenant-reconciled RoleBinding (`bind` on that
  ClusterRole avoids the privilege-escalation chicken-egg). Stolen SA without a
  tenant RoleBinding cannot mutate VMs; `kubevirtGuard` still denies mutate
  outside tenant namespaces. Residual: `rolebindings` create + `bind` remain
  cluster-scoped (a stolen SA could mint bindings outside tenants — VAP covers
  the mutate path; RoleBinding-guard VAP is follow-up)
- Offering CPU/memory are bounded in the CRD schema; Instances must live in
  `virtfoundry-tenant-*` namespaces; Template `sourceType: container` images
  must match the ContainerDisk allowlist at admission (`crAdmission`) and
  again in the Instance reconciler
- `dedicatedCPU` / KubeVirt `DedicatedCPUPlacement` requires a platform-owned
  Offering (`metadata.labels["virtfoundry.io/platform-owned"]="true"`). Without
  that label the reconciler ignores Offering/Instance `dedicatedCPU` flags
  (issue #23)
- Residual admission hardening (issue #26): validating webhooks + cert-manager
  + Helm `:9443`, admission-time Tenant slug uniqueness, broader privileged
  KubeVirt feature rejection at admission
- Operator logs use zap `Development: false` by default; never log cloud-init bodies
