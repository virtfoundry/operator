# Instance CR smoke test (0.7 slice)

Validates operator creates a KubeVirt VM from an `Instance` CR without the API calling hypervisor.

## Prerequisites

- Homelab or kind with KubeVirt, operator, and VirtFoundry API/UI (catalog seeded).
- Tenant namespace `virtfoundry-tenant-default` exists (default tenant).
- Cluster-scoped `Offering` `small` and catalog `Template` `cirros` in `virtfoundry-system`.

## Apply

```bash
export KUBECONFIG=/path/to/kubeconfig

kubectl apply -f config/samples/virtfoundry_v1alpha1_instance.yaml

kubectl get vf-instance -n virtfoundry-tenant-default
kubectl get vm -n virtfoundry-tenant-default
kubectl describe vf-instance cr-smoke-demo -n virtfoundry-tenant-default
```

Expect:

- `VirtualMachine/cr-smoke-demo` created by operator
- `status.phase` → `Running` (may take 1–2 min)
- UI lists the VM once API store lists the Instance CR (or apply CR via API SaveVM path)

## Delete

```bash
kubectl delete -f config/samples/virtfoundry_v1alpha1_instance.yaml
kubectl get vm -n virtfoundry-tenant-default  # VM should be gone
```

## Notes

- ISO templates return `status.phase=Failed` until CDI controller lands.
- Multus NICs from `spec.nics` are not wired in this slice (pod network only).
