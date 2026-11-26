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
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

const (
	tgWeb  = "tg-web"
	svcWeb = "lb-web"
)

func lbTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := virtfoundryv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	return s
}

func lbFixture() (*virtfoundryv1alpha1.LoadBalancer, []client.Object) {
	const ns = "virtfoundry-tenant-default"
	lb := &virtfoundryv1alpha1.LoadBalancer{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: ns, UID: "lb-uid"},
		Spec: virtfoundryv1alpha1.LoadBalancerSpec{Listeners: []virtfoundryv1alpha1.LoadBalancerListener{
			{ID: "l1", Protocol: "tcp", Port: 80, TargetGroupRef: virtfoundryv1alpha1.LocalObjectRef{Name: tgWeb}},
			{ID: "l2", Protocol: "udp", Port: 53, TargetGroupRef: virtfoundryv1alpha1.LocalObjectRef{Name: "tg-missing"}},
		}},
	}
	tg := &virtfoundryv1alpha1.TargetGroup{
		ObjectMeta: metav1.ObjectMeta{Name: tgWeb, Namespace: ns},
		Spec: virtfoundryv1alpha1.TargetGroupSpec{Protocol: "tcp", Port: 8080, Targets: []virtfoundryv1alpha1.TargetGroupTarget{
			{ID: "t1", IP: "10.0.0.5"}, {ID: "t2", IP: "10.0.0.4"}, {ID: "t3", IP: "10.0.0.5"},
		}},
	}
	return lb, []client.Object{lb, tg}
}

func TestLoadBalancerReconcile_ServiceEndpointsAndStatus(t *testing.T) {
	scheme := lbTestScheme(t)
	lb, objs := lbFixture()
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&virtfoundryv1alpha1.LoadBalancer{}).WithObjects(objs...).Build()
	r := &LoadBalancerReconciler{Client: c, Scheme: scheme}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: lb.Namespace, Name: lb.Name}}
	ctx := context.Background()

	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	svc := &corev1.Service{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: lb.Namespace, Name: svcWeb}, svc); err != nil {
		t.Fatal(err)
	}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer || len(svc.Spec.Ports) != 1 {
		t.Fatalf("service spec=%+v", svc.Spec)
	}
	if p := svc.Spec.Ports[0]; p.Name != "tcp-80" || p.Port != 80 || p.TargetPort.IntValue() != 8080 {
		t.Fatalf("port=%+v", p)
	}
	if !metav1.IsControlledBy(svc, lb) {
		t.Fatal("service not owned by the LoadBalancer")
	}
	slices := &discoveryv1.EndpointSliceList{}
	if err := c.List(ctx, slices, client.InNamespace(lb.Namespace)); err != nil {
		t.Fatal(err)
	}
	if len(slices.Items) != 1 {
		t.Fatalf("slices=%d", len(slices.Items))
	}
	sl := slices.Items[0]
	if sl.Labels[discoveryv1.LabelServiceName] != svcWeb || !metav1.IsControlledBy(&sl, lb) {
		t.Fatalf("slice labels/owner wrong: %+v", sl.ObjectMeta)
	}
	if len(sl.Endpoints) != 1 || len(sl.Endpoints[0].Addresses) != 2 || sl.Endpoints[0].Addresses[0] != "10.0.0.4" || *sl.Ports[0].Port != 8080 {
		t.Fatalf("endpoints=%+v ports=%+v", sl.Endpoints, sl.Ports)
	}

	got := &virtfoundryv1alpha1.LoadBalancer{}
	if err := c.Get(ctx, req.NamespacedName, got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase != virtfoundryv1alpha1.PhasePending || got.Status.VIP != "" {
		t.Fatalf("before VIP: %+v", got.Status)
	}

	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "10.0.50.7"}}
	if err := c.Status().Update(ctx, svc); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(ctx, req.NamespacedName, got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase != virtfoundryv1alpha1.PhaseReady || got.Status.VIP != "10.0.50.7" {
		t.Fatalf("after VIP: %+v", got.Status)
	}
}

func TestLoadBalancerReconcile_NoListenersRemovesOwnedService(t *testing.T) {
	scheme := lbTestScheme(t)
	lb, objs := lbFixture()
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&virtfoundryv1alpha1.LoadBalancer{}).WithObjects(objs...).Build()
	r := &LoadBalancerReconciler{Client: c, Scheme: scheme}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: lb.Namespace, Name: lb.Name}}
	ctx := context.Background()
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}

	cur := &virtfoundryv1alpha1.LoadBalancer{}
	if err := c.Get(ctx, req.NamespacedName, cur); err != nil {
		t.Fatal(err)
	}
	cur.Spec.Listeners = nil
	if err := c.Update(ctx, cur); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	err := c.Get(ctx, types.NamespacedName{Namespace: lb.Namespace, Name: svcWeb}, &corev1.Service{})
	if err == nil {
		t.Fatal("owned service should be removed when no listener is usable")
	}
	slices := &discoveryv1.EndpointSliceList{}
	if err := c.List(ctx, slices, client.InNamespace(lb.Namespace)); err != nil || len(slices.Items) != 0 {
		t.Fatalf("slices=%d err=%v", len(slices.Items), err)
	}
}

func TestLoadBalancerReconcile_NeverDeletesForeignService(t *testing.T) {
	scheme := lbTestScheme(t)
	lb, objs := lbFixture()
	lb.Spec.Listeners = nil
	foreign := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: svcWeb, Namespace: lb.Namespace}}
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&virtfoundryv1alpha1.LoadBalancer{}).WithObjects(append(objs, foreign)...).Build()
	r := &LoadBalancerReconciler{Client: c, Scheme: scheme}
	if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: lb.Namespace, Name: lb.Name}}); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: lb.Namespace, Name: svcWeb}, &corev1.Service{}); err != nil {
		t.Fatalf("foreign service deleted: %v", err)
	}
}

func TestLbsForTargetGroup(t *testing.T) {
	scheme := lbTestScheme(t)
	lb, objs := lbFixture()
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	r := &LoadBalancerReconciler{Client: c, Scheme: scheme}
	tg := &virtfoundryv1alpha1.TargetGroup{ObjectMeta: metav1.ObjectMeta{Name: tgWeb, Namespace: lb.Namespace}}
	if got := r.lbsForTargetGroup(context.Background(), tg); len(got) != 1 || got[0].Name != "web" {
		t.Fatalf("requests=%v", got)
	}
	tg.Name = "unrelated"
	if got := r.lbsForTargetGroup(context.Background(), tg); len(got) != 0 {
		t.Fatalf("requests=%v", got)
	}
}
