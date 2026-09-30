/*
Copyright 2026 The VirtFoundry Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
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
	if m["name"] != "net-a" {
		t.Fatalf("name=%v", m["name"])
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
	if _, err := buildIsolatedNADConfig("n", "virtfoundry-br0", "   "); err == nil {
		t.Fatal("expected error for whitespace cidr")
	}
}

func TestBuildIsolatedNADConfig_DefaultBridge(t *testing.T) {
	cfg, err := buildIsolatedNADConfig("n", "", "10.1.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cfg, `"bridge": "virtfoundry-br0"`) {
		t.Fatalf("expected default bridge, got %s", cfg)
	}
}

func networkTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := virtfoundryv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

func TestNetworkReconcile_IsolatedWritesStatus(t *testing.T) {
	scheme := networkTestScheme(t)
	net := &virtfoundryv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: "vpc-net", Namespace: "virtfoundry-tenant-smoke", ResourceVersion: "1"},
		Spec: virtfoundryv1alpha1.NetworkSpec{
			Name:        "vpc-net",
			NetworkType: networkTypeIsolated,
			CIDR:        "10.20.0.0/24",
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&virtfoundryv1alpha1.Network{}).
		WithObjects(net).Build()
	r := &NetworkReconciler{Client: c, Scheme: scheme}

	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: net.Name, Namespace: net.Namespace},
	}); err != nil {
		t.Fatal(err)
	}

	got := &virtfoundryv1alpha1.Network{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(net), got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase != networkPhaseReady {
		t.Fatalf("phase=%q", got.Status.Phase)
	}
	if got.Status.NADName != net.Name || got.Status.NADNamespace != net.Namespace {
		t.Fatalf("status nad=%s/%s", got.Status.NADNamespace, got.Status.NADName)
	}

	nad := &unstructured.Unstructured{}
	nad.SetGroupVersionKind(nadGVK)
	if err := c.Get(context.Background(), types.NamespacedName{Name: net.Name, Namespace: net.Namespace}, nad); err != nil {
		t.Fatalf("expected NAD: %v", err)
	}
	cfg, _, _ := unstructured.NestedString(nad.Object, "spec", "config")
	if !strings.Contains(cfg, "host-local") || !strings.Contains(cfg, "10.20.0.0/24") {
		t.Fatalf("unexpected NAD config: %s", cfg)
	}
	if mb, _, _ := unstructured.NestedString(nad.Object, "metadata", "labels", labelManagedBy); mb != managedByOperator {
		t.Fatalf("managed-by=%q", mb)
	}
}

func TestNetworkReconcile_SharedNoOp(t *testing.T) {
	scheme := networkTestScheme(t)
	net := &virtfoundryv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: "public", Namespace: "virtfoundry-system", ResourceVersion: "1"},
		Spec: virtfoundryv1alpha1.NetworkSpec{
			Name:        "public",
			NetworkType: networkTypeShared,
			CIDR:        "10.0.50.0/24",
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&virtfoundryv1alpha1.Network{}).
		WithObjects(net).Build()
	r := &NetworkReconciler{Client: c, Scheme: scheme}

	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: net.Name, Namespace: net.Namespace},
	}); err != nil {
		t.Fatal(err)
	}

	nad := &unstructured.Unstructured{}
	nad.SetGroupVersionKind(nadGVK)
	err := c.Get(context.Background(), types.NamespacedName{Name: net.Name, Namespace: net.Namespace}, nad)
	if err == nil {
		t.Fatal("expected no NAD for shared Network")
	}
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestNetworkReconcile_ForeignNADRefuse(t *testing.T) {
	scheme := networkTestScheme(t)
	net := &virtfoundryv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: "taken", Namespace: "ns1", ResourceVersion: "1"},
		Spec: virtfoundryv1alpha1.NetworkSpec{
			Name:        "taken",
			NetworkType: networkTypeIsolated,
			CIDR:        "10.30.0.0/24",
		},
	}
	foreign := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": nadAPIVersion,
		"kind":       nadKind,
		"metadata": map[string]any{
			"name":      "taken",
			"namespace": "ns1",
			"labels": map[string]any{
				labelManagedBy: "someone-else",
			},
		},
		"spec": map[string]any{"config": `{"cniVersion":"0.3.1","type":"bridge"}`},
	}}
	foreign.SetGroupVersionKind(nadGVK)

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&virtfoundryv1alpha1.Network{}).
		WithObjects(net, foreign).Build()
	r := &NetworkReconciler{Client: c, Scheme: scheme}

	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: net.Name, Namespace: net.Namespace},
	})
	if err == nil || !strings.Contains(err.Error(), "refusing overwrite") {
		t.Fatalf("expected refuse overwrite, got %v", err)
	}

	got := &virtfoundryv1alpha1.Network{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(net), got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase != networkPhaseFailed {
		t.Fatalf("phase=%q", got.Status.Phase)
	}
}
