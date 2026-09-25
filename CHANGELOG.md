# Changelog

All notable changes to the **VirtFoundry operator** are documented here.

Format based on [Keep a Changelog](https://keepachangelog.com/). Versioning aligned with [virtfoundry/helm-charts](https://github.com/virtfoundry/helm-charts/blob/main/docs/project/versioning.md).

## [Unreleased]

## [0.7.2] - 2026-09-25

Security release: tenant isolation, ContainerDisk image allowlist, CR admission bounds, least-privilege RBAC.

### Added

- Tenant reconcile ensures PSA (`privileged`), default-deny NetworkPolicy
  (`virtfoundry-default-deny`), ResourceQuota, and LimitRange in tenant namespaces
- Instance Multus/VPC NIC wiring from `spec.nics` → Network status NAD
- ContainerDisk image allowlist on Instance reconcile (`quay.io/containerdisks/`,
  `quay.io/kubevirt/` by default; override via
  `VIRTFOUNDRY_ALLOWED_CONTAINER_IMAGE_PREFIXES` / chart `imageAllowlist.prefixes`)

### Changed

- **Breaking:** Instance no longer attaches KubeVirt pod network (masquerade) by
  default. Set `spec.nics` or annotate `virtfoundry.io/allow-pod-network=true`
- Instance reconcile refuses namespaces that are not labelled VirtFoundry tenants
- Instance reconcile refuses Template images outside the ContainerDisk allowlist
  (HTTP(S) URLs must use the ISO/CDI path)
## [0.7.1] - 2026-09-04

### Changed

- Chart and default image tag aligned with VirtFoundry `0.7.1` (no functional operator changes)

[0.7.2]: https://github.com/virtfoundry/operator/compare/v0.7.1...v0.7.2

[0.7.1]: https://github.com/virtfoundry/operator/releases/tag/v0.7.1

## [0.7.0] - 2026-09-02

### Changed

- Chart and default image tag aligned with VirtFoundry `0.7.0` (no functional operator changes)

[0.7.0]: https://github.com/virtfoundry/operator/releases/tag/v0.7.0

## [0.6.0] - 2026-09-01

### Added

- Initial public release aligned with VirtFoundry `0.6.0`
- `virtfoundry.io/v1alpha1` CRDs (Tenant, Instance, VPC, Network, Disk, IAM, …)
- **Tenant** controller — tenant namespace reconciliation
- **Instance** controller — KubeVirt VM status sync to Instance CR status
- Helm chart `charts/virtfoundry-operator` (`ghcr.io/virtfoundry/operator:0.6.0`)

### Known gaps (0.7+)

- Full infra controllers (VPC, Network, Disk, Instance create/delete)
- CI image publish + digest write-back to homelab Argo values

[0.6.0]: https://github.com/virtfoundry/operator/releases/tag/v0.6.0
