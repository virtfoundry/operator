#!/usr/bin/env bash
# Real E2E on a THROWAWAY Kind cluster: KubeVirt (software emulation) + Multus + CDI,
# the charts from helm-charts, the operator image built from this checkout and the
# released core image from the chart defaults.
#
# Flow: Tenant CR -> namespace; API: VPC + isolated network (NAD), volume, VM deploy,
# VM Running, Instance powerState Halted -> stopped, Running -> back, attach volume, delete.
#
# Env: HELM_CHARTS_DIR (checkout of virtfoundry/helm-charts, required)
#      OPERATOR_IMG    (image already loaded into Kind, default virtfoundry-operator:e2e)
# Safety: refuses any API server that is not on localhost.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

: "${HELM_CHARTS_DIR:?set HELM_CHARTS_DIR to a helm-charts checkout}"
OPERATOR_IMG="${OPERATOR_IMG:-virtfoundry-operator:e2e}"
KUBEVIRT_VERSION="${KUBEVIRT_VERSION:-v1.10.0}"
CDI_VERSION="${CDI_VERSION:-v1.66.1}"
NS=virtfoundry-system
ROOT_PW="e2e-root-password-0123"
JWT="$(printf 'e2e-jwt-secret-%.0s' 1 2 3)0123456789"
API_PORT="${API_PORT:-18080}"
API="http://127.0.0.1:${API_PORT}/api/v1"

server="$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}')"
case "$server" in
  https://127.0.0.1:*|https://localhost:*|https://\[::1\]:*) ;;
  *) echo "REFUSING: API server $server is not local." >&2; exit 2 ;;
esac

ok()   { echo "OK: $*"; }
fail() { echo "FAIL: $*" >&2; diagnostics; exit 1; }
diagnostics() {
  set +e
  echo "::group::diagnostics"
  kubectl get pods -A -o wide
  kubectl get tenants,networks,instances -A
  kubectl get vm,vmi,dv,pvc,net-attach-def -A
  kubectl -n "$NS" logs deploy/virtfoundry-operator --tail=80
  kubectl -n "$NS" logs deploy/virtfoundry-api --tail=80
  echo "::endgroup::"
}

# wait_for "<description>" <timeout-seconds> <command...>
wait_for() {
  local what=$1 timeout=$2; shift 2
  local end=$((SECONDS + timeout))
  until "$@" >/dev/null 2>&1; do
    [ "$SECONDS" -lt "$end" ] || fail "timeout waiting for $what"
    sleep 5
  done
  ok "$what"
}

H="$HELM_CHARTS_DIR"

echo "==> Platform: Multus, KubeVirt $KUBEVIRT_VERSION, CDI $CDI_VERSION"
"$H/scripts/setup/multus.sh"
kubectl create -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml"
kubectl create -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-cr.yaml"
kubectl -n kubevirt patch kubevirt kubevirt --type merge -p \
  '{"spec":{"configuration":{"developerConfiguration":{"useEmulation":true}}}}'
kubectl -n kubevirt wait --for=condition=Available kubevirt/kubevirt --timeout=600s
CDI_VERSION="$CDI_VERSION" "$H/scripts/setup/cdi.sh"

echo "==> VirtFoundry charts (operator image: $OPERATOR_IMG)"
helm install virtfoundry-crds "$H/charts/virtfoundry-crds" -n "$NS" --create-namespace
helm install virtfoundry-operator "$H/charts/virtfoundry-operator" -n "$NS" \
  --set image.repository="${OPERATOR_IMG%:*}" --set image.tag="${OPERATOR_IMG##*:}" \
  --set image.digest="" --set image.pullPolicy=IfNotPresent --wait --timeout 5m
helm install virtfoundry "$H/charts/virtfoundry" -n "$NS" -f "$H/charts/virtfoundry/values-kind.yaml" \
  --set-string secrets.rootPassword="$ROOT_PW" --set-string secrets.jwtSecret="$JWT" \
  --wait --timeout 10m
ok "charts installed"

echo "==> Tenant CR"
kubectl apply -f - <<'YAML'
apiVersion: virtfoundry.io/v1alpha1
kind: Tenant
metadata: { name: e2e }
spec: { name: E2E, slug: e2e }
YAML
wait_for "Tenant e2e Ready" 180 \
  test "$(kubectl get tenants.virtfoundry.io e2e -o jsonpath='{.status.phase}')" = Ready
wait_for "tenant namespace exists" 60 kubectl get ns virtfoundry-tenant-e2e

echo "==> API login"
kubectl -n "$NS" port-forward svc/virtfoundry-api "$API_PORT":8080 >/tmp/pf.log 2>&1 &
trap 'kill %1 2>/dev/null || true' EXIT
wait_for "API reachable" 60 curl -fsS "http://127.0.0.1:${API_PORT}/health"
TOK=$(curl -fsS -X POST "$API/auth/login" -H 'Content-Type: application/json' \
  -d "{\"username\":\"root\",\"password\":\"$ROOT_PW\"}" | python3 -c 'import sys,json; print(json.load(sys.stdin)["token"])')
api() { # api METHOD PATH [JSON]
  local m=$1 p=$2; shift 2
  curl -fsS -X "$m" "$API$p" -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' "${@:+-d}" "$@"
}
jget() { python3 -c "import sys,json; d=json.load(sys.stdin); print($1)"; }

echo "==> VPC + isolated network"
VPC=$(api POST /vpcs '{"name":"e2e-vpc","cidr":"10.77.0.0/16"}')
NET_ID=$(echo "$VPC" | jget 'd["default_network"]["id"]')
[ -n "$NET_ID" ] || fail "VPC creation returned no default network"
wait_for "Network CR Ready with a NAD" 180 bash -c \
  "kubectl get networks.virtfoundry.io -A -o jsonpath='{range .items[*]}{.status.phase}{\" \"}{.status.nadName}{\"\n\"}{end}' | grep -q '^Ready .'"

echo "==> Volume"
VOL_ID=$(api POST /volumes '{"name":"e2e-vol","size_gi":1}' | jget 'd["volume"]["id"]')
[ -n "$VOL_ID" ] || fail "volume creation returned no id"

echo "==> VM deploy"
TPL=$(api GET /vm-templates | python3 -c 'import sys,json; d=json.load(sys.stdin); t=[x for x in (d.get("templates") or d.get("vm_templates") or d) if "cirros" in (x.get("name","")+x.get("image","")).lower()]; print(t[0]["id"] if t else "")')
[ -n "$TPL" ] || fail "no cirros template seeded"
api POST /vms "{\"name\":\"e2e-vm\",\"template_id\":\"$TPL\",\"service_offering_id\":\"small\",\"network_ids\":[\"$NET_ID\"]}" >/dev/null
wait_for "Instance e2e-vm Ready" 600 \
  test "$(kubectl get instances.virtfoundry.io -A -o jsonpath='{.items[?(@.metadata.name=="e2e-vm")].status.phase}')" = Ready
wait_for "VMI Running" 600 \
  test "$(kubectl get vmi -A -o jsonpath='{.items[?(@.metadata.name=="e2e-vm")].status.phase}')" = Running

INS_NS=$(kubectl get instances.virtfoundry.io -A -o jsonpath='{.items[?(@.metadata.name=="e2e-vm")].metadata.namespace}')

echo "==> powerState Halted / Running"
kubectl -n "$INS_NS" patch instances.virtfoundry.io e2e-vm --type merge -p '{"spec":{"powerState":"Halted"}}'
wait_for "VMI gone after Halted" 300 bash -c "! kubectl -n $INS_NS get vmi e2e-vm"
kubectl -n "$INS_NS" patch instances.virtfoundry.io e2e-vm --type merge -p '{"spec":{"powerState":"Running"}}'
wait_for "VMI Running again" 600 \
  test "$(kubectl -n "$INS_NS" get vmi e2e-vm -o jsonpath='{.status.phase}')" = Running

echo "==> Volume attach"
api POST /vms/e2e-vm/volumes "{\"volume_id\":\"$VOL_ID\"}" >/dev/null && ok "volume attached"

echo "==> Cleanup"
api POST /vms/delete '{"name":"e2e-vm"}' >/dev/null
wait_for "Instance deleted" 300 bash -c "! kubectl -n $INS_NS get instances.virtfoundry.io e2e-vm"
wait_for "VM deleted" 300 bash -c "! kubectl -n $INS_NS get vm e2e-vm"
kubectl delete tenants.virtfoundry.io e2e --wait=false
wait_for "tenant namespace removed" 300 bash -c "! kubectl get ns virtfoundry-tenant-e2e"
echo "E2E PASSED"
