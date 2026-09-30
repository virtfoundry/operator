# Design: Instance sshKeyRefs → cloud-init (core#116 / operator)

**Status:** Approved (session 2026-09-30)  
**Issue:** https://github.com/virtfoundry/core/issues/116  
**Epic:** A — CRD-first

## Goal

Wire `Instance.spec.sshKeyRefs` so the operator injects SSH public keys into guest cloud-init at boot. Core maps `ssh_key_id` → CR refs and allows the operator deploy path when a key is present.

## Non-goals (v1)

- Do not put one-time passwords on the Instance CR (password stays hypervisor path).
- Do not move cloud-init into a Secret (operator#16 remains separate).
- Do not re-inject keys after first boot / watch SSHKey for live updates (cloud-init is once).
- Do not change Linux fail-closed guest auth in core API (`ssh_key_id` or `cloud_init_password`).

## Approach

1. **Operator:** resolve `sshKeyRefs` → `SSHKey.spec.publicKey` in the Instance namespace; build `#cloud-config` with `ubuntu` + `ssh_authorized_keys`; append Template `cloudInitUserData` as extra fragment (same shape as core `BuildLinuxUserData`).
2. **Core:** `PlatformVM.SSHKeyRefs` → `InstanceToUnstructured`; `deployVMViaOperator` sets refs from `SSHKeyID` via `SanitizeCRName(key.Name)`; `canDeployViaOperator` allows `SSHKeyID`, still blocks password.

## Safety

- Missing SSHKey CR or empty `publicKey` → reconcile error (no silent boot without keys when refs are set).
- RBAC: get/list/watch `sshkeys` only (no secrets).
- Unit tests with fake client before merge.

## Follow-ups

- Password / Secret userdata (operator#16)
- Anti dual-writer (core#131)
