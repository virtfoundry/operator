# Design: NetworkReconciler → Multus NAD (operator#36)

**Status:** Approved (session 2026-09-30)  
**Issue:** https://github.com/virtfoundry/operator/issues/36  
**Epic:** A — CRD-first

## Goal

Reconcile `Network` CRs (`networkType: isolated`) into Multus `NetworkAttachmentDefinition` objects and write `status.nadName` / `status.nadNamespace` so Instance `resolveVMNetworks` can attach real Multus NICs.

## Non-goals (v1)

- Do not reconcile `networkType: shared` / public (`vf-pub0`) — stays API/Helm.
- Do not install Multus, bridges, or node CNI plugins.
- Do not change `allow-pod-network` fail-closed behavior.
- Do not delete foreign NADs (other `managed-by`).
- Do not introduce whereabouts / macvlan.

## Approach

Port the existing core NAD template from `core/internal/platform/k8s/network.go` (`createBridgeNAD`: CNI `bridge` + IPAM `host-local` + `spec.cidr`, bridge default `virtfoundry-br0`) into the operator. Same JSON shape already used in homelab — lowest cluster risk.

## Behavior

1. Watch `Network` in all namespaces.
2. If `spec.networkType != isolated` → set condition `Skipped` / leave phase unchanged (or `Ready` only if status already has NAD from elsewhere); **no NAD create**.
3. If isolated and `spec.cidr` missing/invalid → `phase=Failed`, requeue.
4. CreateOrUpdate NAD named from Network (stable: Network.Name) in Network.Namespace.
5. Labels: `app.kubernetes.io/managed-by=virtfoundry-operator`, owner ref → Network.
6. Patch status: `nadName`, `nadNamespace`, `phase=Ready`.
7. If NAD exists with foreign managed-by → do not overwrite; Failed + event.

## RBAC / chart

- Grant `network-attachment-definitions` get/list/watch/create/update/patch.
- Grant `networks/status` update/patch.
- Update `hack/verify-chart-rbac.sh` allowlist (today forbids NAD).

## Safety

- Homelab smoke: create Network in `virtfoundry-tenant-smoke` only; never mutate public NAD.
- Unit tests with fake client before any cluster apply.

## Follow-ups (out of this design)

- sshKeyRefs vivos (core#116)
- Core stop `CreateNetworkAttachment` when Network status already Ready
- Anti dual-writer (core#131)
