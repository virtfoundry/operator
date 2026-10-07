# Changelog

All notable changes to the **VirtFoundry operator** are documented here.

Format based on [Keep a Changelog](https://keepachangelog.com/). Versioning aligned with [virtfoundry/helm-charts](https://github.com/virtfoundry/helm-charts/blob/main/docs/project/versioning.md).

## [Unreleased]

## [0.11.0] - 2026-10-07

### Changed

- **Breaking:** the `virtfoundry-operator` chart no longer ships `crds/`. Install the CRDs with the `virtfoundry-crds` chart from [helm-charts](https://github.com/virtfoundry/helm-charts) first (`helm.sh/resource-policy: keep`, upgraded by `helm upgrade`). `config/crd/bases` stays the source of truth and the CRD chart syncs from it. Existing clusters adopt their CRDs first: see [CRDs and upgrades](https://virtfoundry.github.io/helm-charts/docs/guide/crds/).
- Chart / image pins aligned with core **0.11.0**.

## [0.10.0] - 2026-10-06

### Added

- CRD validation allows `ghcr.io/virtfoundry/` containerDisk images (VKS node image).

### Security

- CodeQL, Scorecard, Dependabot (grouped monthly) and dependency review.

### Changed

- Chart / image pins aligned with core **0.10.0**.

## [0.9.0] - 2026-10-01

### Added

- NetworkReconciler → Multus NAD for isolated networks.
- Instance `sshKeyRefs` injected into cloud-init; `cloudInitUserData` + `cloudInitSecretRef`.
- Instance CreateOrUpdate converges NICs and TemplateRef/image (+ cloud-init) on existing VMs (not only powerState).
- CI pins operator digest to argo-homelab after image push.

### Security

- VAP admits Template container images; deny kubevirt mutate outside tenant ns.
- KubeVirt mutate split to per-tenant RoleBinding (`*-kubevirt-mutate`); ClusterRole monitor-only.
- Gate `dedicatedCPU` via platform-owned Offering label (coordinates core Offering stamp).
- Fail reconcile without template instead of cirros default.
- CI fails when Helm ClusterRole drifts from kubebuilder markers.

### Changed

- Release alignment with core/helm-charts **0.9.0**.

## [0.8.0] - 2026-09-28

### Changed

- Release alignment with core/helm-charts **0.8.0** (no operator functional change).

## [0.7.3] - 2026-09-25

### Changed

- Release alignment with core/helm-charts **0.7.3** (no operator functional change).

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

## [0.7.0] - 2026-09-02

First tagged operator release (no separate `v0.6.0` tag was ever published).

### Added

- `virtfoundry.io/v1alpha1` CRDs (Tenant, Instance, VPC, Network, Disk, IAM, …)
- **Tenant** controller — tenant namespace reconciliation
- **Instance** controller — KubeVirt VM status sync to Instance CR status
- Helm chart packaging via [virtfoundry/helm-charts](https://github.com/virtfoundry/helm-charts) (`ghcr.io/virtfoundry/operator:0.7.0`)

### Known gaps (post-0.7.0)

- Full infra controllers (VPC, Network, Disk, Instance create/delete)
- CI image publish + digest write-back to homelab Argo values

[0.7.3]: https://github.com/virtfoundry/operator/compare/v0.7.2...v0.7.3
[0.7.2]: https://github.com/virtfoundry/operator/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/virtfoundry/operator/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/virtfoundry/operator/releases/tag/v0.7.0
