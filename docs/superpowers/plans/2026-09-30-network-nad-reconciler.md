# NetworkReconciler → Multus NAD Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement operator NetworkReconciler that creates Multus NADs for `isolated` Networks and fills `status.nadName`/`status.nadNamespace` ([operator#36](https://github.com/virtfoundry/operator/issues/36)).

**Architecture:** Port core’s bridge+host-local NAD template into the operator. Register a new controller beside Tenant/Instance. Only `networkType: isolated`; skip `shared`. Label-owned CreateOrUpdate; never overwrite foreign NADs; never touch public `vf-pub0`.

**Tech Stack:** Kubebuilder/controller-runtime, Multus `k8s.cni.cncf.io/v1` NetworkAttachmentDefinition (unstructured or typed client), Go tests with envtest/fake client.

**Design:** `docs/superpowers/specs/2026-09-30-network-nad-reconciler-design.md`

## Global Constraints

- CNI JSON must match `core/internal/platform/k8s/network.go` `createBridgeNAD` (cniVersion `0.3.1`, type `bridge`, ipam `host-local`).
- Default bridge name: `virtfoundry-br0` (same as core branding).
- `app.kubernetes.io/managed-by=virtfoundry-operator` on NADs.
- No floating cluster changes without unit tests green.
- Homelab smoke only in `virtfoundry-tenant-smoke` (or equivalent smoke tenant).

## File map

| File | Role |
|------|------|
| `internal/controller/network_nad.go` | Build CNI config JSON + Apply NAD |
| `internal/controller/network_controller.go` | Reconcile Network → NAD + status |
| `internal/controller/network_controller_test.go` | Unit tests |
| `cmd/main.go` | Register NetworkReconciler |
| `config/rbac/role.yaml` + chart RBAC | NAD + networks/status verbs |
| `hack/verify-chart-rbac.sh` | Allow NAD API group |
| `charts/virtfoundry-operator/...` | Mirror RBAC if generated separately |

---

### Task 1: Failing tests for NAD builder + reconcile happy path

**Files:**
- Create: `internal/controller/network_nad.go` (stubs ok until Task 2)
- Create: `internal/controller/network_controller_test.go`
- Test: same

**Interfaces:**
- Produces: `buildIsolatedNADConfig(name, bridge, cidr string) (string, error)`
- Produces: `NetworkReconciler` with `Reconcile(ctx, req) (ctrl.Result, error)` (registered later)

- [ ] **Step 1: Write failing tests**

```go
package controller

import (
	"encoding/json"
	"testing"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildIsolatedNADConfig_MatchesCoreShape(t *testing.T) {
	cfg, err := buildIsolatedNADConfig("net-a", "virtfoundry-br0", "10.10.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(cfg), &m); err != nil {
		t.Fatal(err)
	}
	if m["type"] != "bridge" {
		t.Fatalf("type=%v", m["type"])
	}
	if m["bridge"] != "virtfoundry-br0" {
		t.Fatalf("bridge=%v", m["bridge"])
	}
	ipam, _ := m["ipam"].(map[string]any)
	if ipam["type"] != "host-local" || ipam["subnet"] != "10.10.0.0/24" {
		t.Fatalf("ipam=%v", ipam)
	}
}

func TestBuildIsolatedNADConfig_RejectsEmptyCIDR(t *testing.T) {
	if _, err := buildIsolatedNADConfig("n", "virtfoundry-br0", ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestNetworkReconcile_IsolatedWritesStatus(t *testing.T) {
	// Use existing envtest/fake pattern from tenant_controller_test.go.
	// Create Network{Type:isolated, CIDR:"10.20.0.0/24"} in ns "t1".
	// Reconcile once.
	// Assert NAD exists in t1 named after Network; status.NADName/NADNamespace set; Phase Ready.
	t.Fatal("TODO implement with fake client once builder exists")
}
```

- [ ] **Step 2: Run tests — expect fail**

```bash
cd /Users/matheusthurler/Documents/github/virtfoundry/operator
go test ./internal/controller/ -count=1 -run 'TestBuildIsolatedNADConfig|TestNetworkReconcile_Isolated'
```

Expected: compile error / FAIL (functions missing).

- [ ] **Step 3: Commit**

```bash
git add internal/controller/network_controller_test.go
git commit -m "test(operator): add failing Network NAD reconcile tests"
```

---

### Task 2: Implement NAD builder + reconcile (isolated only)

**Files:**
- Create: `internal/controller/network_nad.go`
- Create: `internal/controller/network_controller.go`
- Modify: `cmd/main.go` (register)
- Test: `internal/controller/network_controller_test.go`

**Interfaces:**
- Consumes: `virtfoundryv1alpha1.Network`, Multus GVR `k8s.cni.cncf.io/v1/network-attachment-definitions`
- Produces: working `NetworkReconciler.SetupWithManager`

- [ ] **Step 1: Implement `buildIsolatedNADConfig`** mirroring core:

```go
func buildIsolatedNADConfig(name, bridge, cidr string) (string, error) {
	if strings.TrimSpace(cidr) == "" {
		return "", fmt.Errorf("network cidr is required")
	}
	if bridge == "" {
		bridge = "virtfoundry-br0"
	}
	ipam := fmt.Sprintf(`{
    "type": "host-local",
    "subnet": %q,
    "routes": [{ "dst": "0.0.0.0/0" }]
  }`, cidr)
	return fmt.Sprintf(`{
  "cniVersion": "0.3.1",
  "name": %q,
  "type": "bridge",
  "bridge": %q,
  "ipam": %s
}`, name, bridge, ipam), nil
}
```

- [ ] **Step 2: Implement Reconcile sketch**

```go
// Pseudo — follow tenant_controller patterns for status patch + events.
// 1. Get Network
// 2. if networkType != "isolated" → return nil (no-op)
// 3. build config from Name, Bridge default, Spec.CIDR
// 4. Get existing NAD; if exists && managed-by != virtfoundry-operator → Failed
// 5. CreateOrUpdate unstructured NAD with ownerRef + labels
// 6. Patch status NADName=Name, NADNamespace=Namespace, Phase=Ready
```

- [ ] **Step 3: Register in `cmd/main.go` next to InstanceReconciler**

```go
if err = (&controller.NetworkReconciler{
	Client: mgr.GetClient(),
	Scheme: mgr.GetScheme(),
}).SetupWithManager(mgr); err != nil {
	setupLog.Error(err, "unable to create controller", "controller", "Network")
	os.Exit(1)
}
```

- [ ] **Step 4: Finish envtest for IsolatedWritesStatus + SharedNoOp + ForeignNADFail**

- [ ] **Step 5: Run tests**

```bash
go test ./internal/controller/ -count=1 -run 'TestBuildIsolated|TestNetworkReconcile'
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/controller/network_nad.go internal/controller/network_controller.go internal/controller/network_controller_test.go cmd/main.go
git commit -m "feat(operator): reconcile isolated Network to Multus NAD"
```

---

### Task 3: RBAC + chart verify allowlist

**Files:**
- Modify: `config/rbac/role.yaml` (or kubebuilder markers on reconciler + `make manifests`)
- Modify: `hack/verify-chart-rbac.sh`
- Modify: `charts/virtfoundry-operator/templates/` RBAC as needed

**Interfaces:**
- Consumes: Task 2 controller needing NAD verbs
- Produces: chart that passes `hack/verify-chart-rbac.sh`

- [ ] **Step 1: Add kubebuilder RBAC markers on NetworkReconciler**

```go
//+kubebuilder:rbac:groups=virtfoundry.io,resources=networks,verbs=get;list;watch
//+kubebuilder:rbac:groups=virtfoundry.io,resources=networks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch;create;update;patch
```

- [ ] **Step 2: Regenerate / sync chart RBAC** (`make manifests` or manual mirror used by this repo)

- [ ] **Step 3: Update `hack/verify-chart-rbac.sh`** — remove NAD from forbidden list (or move to allowed)

- [ ] **Step 4: Run**

```bash
./hack/verify-chart-rbac.sh
go test ./internal/controller/ -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git commit -am "fix(rbac): allow Multus NAD for NetworkReconciler"
```

---

### Task 4: Homelab smoke (read-mostly, smoke tenant only)

**Files:** none required (kubectl only)

- [ ] **Step 1: Confirm operator image path** — PR → merge → Argo digest (do **not** kubectl set image)

- [ ] **Step 2: After deploy, in smoke tenant ns only:**

```bash
export KUBECONFIG="$HOME/Documents/homelab/kubespray/inventory/homelab-cluster/artifacts/admin.conf"
NS=virtfoundry-tenant-smoke   # adjust if slug differs
kubectl -n "$NS" apply -f - <<'EOF'
apiVersion: virtfoundry.io/v1alpha1
kind: Network
metadata:
  name: epic-a-nad-smoke
spec:
  name: epic-a-nad-smoke
  networkType: isolated
  cidr: 10.233.200.0/24
EOF
kubectl -n "$NS" wait --for=jsonpath='{.status.phase}'=Ready network/epic-a-nad-smoke --timeout=120s
kubectl -n "$NS" get network epic-a-nad-smoke -o yaml | rg 'nadName|nadNamespace|phase'
kubectl -n "$NS" get net-attach-def epic-a-nad-smoke -o yaml | rg 'bridge|host-local|managed-by'
```

- [ ] **Step 3: Cleanup smoke Network + NAD**

```bash
kubectl -n "$NS" delete network epic-a-nad-smoke --wait=true
```

- [ ] **Step 4: Verify public/shared NADs untouched** (`kubectl get net-attach-def -A | rg vf-pub` unchanged)

---

### Task 5: PR

- [ ] **Step 1: Branch `feat/36-network-nad-reconciler` from `main`**
- [ ] **Step 2: `gh pr create` linking operator#36**
- [ ] **Step 3: Wait CI green — do not merge until Matheus says so unless already authorized**

---

## Spec coverage self-check

| Design item | Task |
|-------------|------|
| isolated bridge+host-local | 1–2 |
| status NAD fields | 2 |
| skip shared | 2 test SharedNoOp |
| foreign NAD no overwrite | 2 test ForeignNADFail |
| RBAC + chart guard | 3 |
| smoke tenant only | 4 |

## Out of scope (next plans)

- sshKeyRefs (core#116)
- core#131 anti dual-writer
- core stop creating NAD when operator status Ready
