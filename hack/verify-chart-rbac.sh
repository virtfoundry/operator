#!/usr/bin/env bash
# Fails if the rendered operator ClusterRole drifts from Tenant+Instance needs
# (kubebuilder config/rbac/role.yaml), regains cluster-wide Secret access, or if
# the namespace deletion guard stops rendering on capable clusters.
#
# Keep in sync with virtfoundry/helm-charts scripts/ci/verify-operator-chart-rbac.sh.
set -euo pipefail

CHART_DIR="${CHART_DIR:-charts/virtfoundry-operator}"
VAP_API="admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy"

# API groups / resource names that belong to future controllers, not Tenant+Instance.
FORBIDDEN_PATTERNS=(
  'secrets'
  'networkpolicies'
  'persistentvolumeclaims'
  'volumesnapshots'
  'network-attachment-definitions'
  'virtualmachinesnapshots'
  'virtualmachinerestores'
  'datavolumes'
  'users'
  'roles'
  'apikeys'
  'vpcs'
  'networks'
  'securitygroups'
  'disks'
  'disksnapshots'
  'instancesnapshots'
  'sshkeys'
  'ipaddresses'
)

REQUIRED_SNIPPETS=(
  'resources: \["tenants"\]'
  'resources: \["instances"\]'
  'resources: \["offerings", "templates"\]'
  'resources: \["namespaces"\]'
  'resources: \["virtualmachines", "virtualmachineinstances"\]'
)

if ! command -v helm >/dev/null 2>&1; then
  echo "helm is required to verify the rendered chart RBAC" >&2
  exit 1
fi

# Comments are dropped so documentation mentioning secrets does not trip the check.
rbac="$(helm template virtfoundry-operator "$CHART_DIR" -s templates/rbac.yaml | sed 's/[[:space:]]*#.*$//')"

for pat in "${FORBIDDEN_PATTERNS[@]}"; do
  if grep -qw "$pat" <<<"$rbac"; then
    echo "FAIL: rendered ClusterRole still grants access to '$pat' (Tenant+Instance only)" >&2
    grep -n -B2 -w "$pat" <<<"$rbac" >&2
    exit 1
  fi
done
echo "OK: rendered ClusterRole has no future-controller / sprawl rules"

for snip in "${REQUIRED_SNIPPETS[@]}"; do
  if ! grep -Eq "$snip" <<<"$rbac"; then
    echo "FAIL: rendered ClusterRole missing required rule matching /$snip/" >&2
    exit 1
  fi
done
echo "OK: rendered ClusterRole covers Tenant + Instance (+ KubeVirt VMs/VMIs)"

# The ClusterRole cannot be scoped by resourceNames (tenant namespaces are
# virtfoundry-tenant-{slug}), so at least keep `update` off namespaces.
namespace_verbs="$(grep -A1 'resources: \["namespaces"\]' <<<"$rbac" | grep 'verbs:' || true)"
[[ -n "$namespace_verbs" ]] || { echo "FAIL: could not find a namespaces rule" >&2; exit 1; }
if grep -qw "update" <<<"$namespace_verbs"; then
  echo "FAIL: rendered ClusterRole grants update on namespaces (verbs: $namespace_verbs)" >&2
  exit 1
fi
echo "OK: rendered ClusterRole cannot update arbitrary namespaces"

guard="$(helm template virtfoundry-operator "$CHART_DIR" \
  -s templates/namespace-guard.yaml --api-versions "$VAP_API")"

if ! grep -q "kind: ValidatingAdmissionPolicy$" <<<"$guard"; then
  echo "FAIL: namespace deletion guard is not rendered on clusters serving $VAP_API" >&2
  exit 1
fi
echo "OK: namespace deletion guard renders on clusters serving ValidatingAdmissionPolicy"
