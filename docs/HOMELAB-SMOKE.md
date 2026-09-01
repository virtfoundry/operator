# Homelab smoke — VirtFoundry operator (local)

CRDs + RBAC installed via Helm in `virtfoundry-system`.
Operator currently runs **from your laptop** against the homelab kubeconfig
(Deployment `replicas=0` until the image is on the nodes).

## Env

```bash
export KUBECONFIG=/Users/matheusthurler/Documents/homelab/kubespray/inventory/homelab-cluster/artifacts/admin.conf
```

## Check CRDs

```bash
kubectl api-resources --api-group=virtfoundry.io
kubectl get crd | grep virtfoundry.io
```

## Tenant smoke (already applied: `homelab-smoke`)

```bash
kubectl get vf-tenant
kubectl get vf-tenant homelab-smoke -o yaml
kubectl get ns virtfoundry-tenant-smoke
```

## Create your own Tenant

```bash
cat <<'EOF' | kubectl apply -f -
apiVersion: virtfoundry.io/v1alpha1
kind: Tenant
metadata:
  name: my-test
  labels:
    app.kubernetes.io/part-of: virtfoundry
spec:
  name: My Test
  slug: my-test
EOF

kubectl get vf-tenant my-test -w
# expect status.phase=Ready and Namespace virtfoundry-tenant-my-test
```

## Other CRDs (no controller yet — create/store only)

```bash
kubectl get vf-vpc,vf-network,vf-instance,vf-disk -A
# create YAML against virtfoundry.io/v1alpha1 — objects persist; status stays empty until controllers land
```

## Helm release

```bash
helm -n virtfoundry-system status virtfoundry-operator
helm -n virtfoundry-system get values virtfoundry-operator
```

Chart paths:
- `operator/charts/virtfoundry-operator` (source)
- `helm-charts/charts/virtfoundry-operator` (copy for Argo)

## Operator process (laptop)

If reconcile stops, restart:

```bash
export KUBECONFIG=/Users/matheusthurler/Documents/homelab/kubespray/inventory/homelab-cluster/artifacts/admin.conf
cd /Users/matheusthurler/Documents/github/virtfoundry/operator
go run ./cmd/main.go --health-probe-bind-address=:18081 --metrics-bind-address=0 --leader-elect=false
```

## In-cluster Deployment later

Needs image on nodes (`docker.io/virtfoundry/operator:0.1.0-dev` already built locally) via SSH sideload as `matheus`, import-pod, or GHCR push — then:

```bash
helm upgrade virtfoundry-operator ./charts/virtfoundry-operator \
  -n virtfoundry-system -f charts/virtfoundry-operator/values-homelab.yaml \
  --set replicas=1
```

## Cleanup smoke Tenant

```bash
kubectl delete vf-tenant homelab-smoke
# Namespace virtfoundry-tenant-smoke should go away after finalizer
```
