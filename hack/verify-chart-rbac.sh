#!/usr/bin/env bash
# Fails if the rendered operator ClusterRole regains cluster-wide Secret access,
# or if the namespace deletion guard stops rendering on capable clusters.
set -euo pipefail

CHART_DIR="${CHART_DIR:-charts/virtfoundry-operator}"
VAP_API="admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy"

if ! command -v helm >/dev/null 2>&1; then
  echo "helm is required to verify the rendered chart RBAC" >&2
  exit 1
fi

# Comments are dropped so documentation mentioning secrets does not trip the check.
rbac="$(helm template virtfoundry-operator "$CHART_DIR" -s templates/rbac.yaml | sed 's/[[:space:]]*#.*$//')"

if grep -qw "secrets" <<<"$rbac"; then
  echo "FAIL: rendered ClusterRole grants access to secrets" >&2
  grep -n -B2 -w "secrets" <<<"$rbac" >&2
  exit 1
fi
echo "OK: rendered ClusterRole has no secrets rule"

guard="$(helm template virtfoundry-operator "$CHART_DIR" \
  -s templates/namespace-guard.yaml --api-versions "$VAP_API")"

if ! grep -q "kind: ValidatingAdmissionPolicy$" <<<"$guard"; then
  echo "FAIL: namespace deletion guard is not rendered on clusters serving $VAP_API" >&2
  exit 1
fi
echo "OK: namespace deletion guard renders on clusters serving ValidatingAdmissionPolicy"
