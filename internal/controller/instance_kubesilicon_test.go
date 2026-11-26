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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

func TestEnsureSiliconVMCreatesAndFollowsPowerState(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := virtfoundryv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	const ns, name = "virtfoundry-tenant-default", "silicon-vm"
	tmpl := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{Name: "ubuntu", Namespace: ns},
		Spec:       virtfoundryv1alpha1.TemplateSpec{DisplayName: "u", Image: "x", SourceType: sourceTypeContainer},
	}
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, UID: "uid-1"},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			DisplayName: name,
			TemplateRef: &virtfoundryv1alpha1.LocalObjectRef{Name: "ubuntu"},
		},
	}
	r := &InstanceReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(tmpl).Build(), Scheme: scheme}

	if _, err := r.ensureSiliconVM(ctx, inst, name); err == nil {
		t.Fatal("a template without the disk annotation must fail")
	}
	tmpl.Annotations = map[string]string{annotationKubeSiliconDisk: "ubuntu.img"}
	if err := r.Update(ctx, tmpl); err != nil {
		t.Fatal(err)
	}
	for _, power := range []string{powerStateRunning, powerStateHalted} {
		inst.Spec.PowerState = power
		if _, err := r.ensureSiliconVM(ctx, inst, name); err != nil {
			t.Fatal(err)
		}
		got := newSiliconVM(ns, name)
		if err := r.Get(ctx, client.ObjectKeyFromObject(got), got); err != nil {
			t.Fatal(err)
		}
		running, _, _ := unstructured.NestedBool(got.Object, "spec", "running")
		disk, _, _ := unstructured.NestedString(got.Object, "spec", "disk", "source")
		if running != (power == powerStateRunning) || disk != "ubuntu.img" || len(got.GetOwnerReferences()) != 1 {
			t.Fatalf("%s: running=%v disk=%q owners=%v", power, running, disk, got.GetOwnerReferences())
		}
	}
}
