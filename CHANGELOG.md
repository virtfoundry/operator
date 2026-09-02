# Changelog

All notable changes to the **VirtFoundry operator** are documented here.

Format based on [Keep a Changelog](https://keepachangelog.com/). Versioning aligned with [virtfoundry/helm-charts](https://github.com/virtfoundry/helm-charts/blob/main/docs/project/versioning.md).

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
