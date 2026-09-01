# Security Policy

## Supported versions

| Version | Supported |
|---------|-----------|
| main branch | yes |
| tagged releases | yes |

## Reporting a vulnerability

**Do not open public GitHub issues for security vulnerabilities.**

Please report security issues via a private GitHub security advisory on this repository or by contacting maintainers listed on the [virtfoundry organization profile](https://github.com/virtfoundry).

Include:

- Description of the issue and impact
- Steps to reproduce
- Affected component (CRDs, controllers, Helm chart)
- Suggested fix (if any)

We aim to acknowledge reports within 7 days.

## Secure deployment notes

- Platform state lives in `virtfoundry.io` CRs; credential hashes live only in Kubernetes Secrets referenced by `secretRef`
- Never put password or API-key hashes in CR `spec`
- Run the operator ServiceAccount with least-privilege RBAC (review chart / `config/rbac`)
- Prefer image digests over floating tags in production overlays
