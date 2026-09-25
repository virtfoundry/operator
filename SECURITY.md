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
- Helm ClusterRole is scoped to the **running** controllers (Tenant + Instance today: tenants/instances CRs, offerings/templates read, namespaces for tenants, KubeVirt VMs/VMIs). It is not absolute least privilege cluster-wide — Namespace `delete` remains cluster-scoped in RBAC and is constrained by the reconciler ownership guard plus the chart's ValidatingAdmissionPolicy. Expand RBAC only when a new controller lands; keep `charts/virtfoundry-operator/templates/rbac.yaml` aligned with `config/rbac/role.yaml`
- Pin operator image by digest in production overlays
