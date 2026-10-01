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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

const (
	testOfferingSmall      = "small"
	testTemplateUbuntu2204 = "ubuntu-2204"
)

func TestIsPlatformOwnedOffering(t *testing.T) {
	t.Parallel()
	if isPlatformOwnedOffering(nil) {
		t.Fatal("nil offering")
	}
	off := &virtfoundryv1alpha1.Offering{}
	if isPlatformOwnedOffering(off) {
		t.Fatal("missing label")
	}
	off.Labels = map[string]string{labelPlatformOwned: "false"}
	if isPlatformOwnedOffering(off) {
		t.Fatal("false label")
	}
	off.Labels[labelPlatformOwned] = annotationTruthy
	if !isPlatformOwnedOffering(off) {
		t.Fatal("expected platform-owned")
	}
}

func TestResolveDedicatedCPU(t *testing.T) {
	t.Parallel()
	inst := &virtfoundryv1alpha1.Instance{Spec: virtfoundryv1alpha1.InstanceSpec{DedicatedCPU: true}}
	if resolveDedicatedCPU(inst, nil) {
		t.Fatal("instance-only request must be ignored")
	}

	tenantOff := &virtfoundryv1alpha1.Offering{
		ObjectMeta: metav1.ObjectMeta{Name: testOfferingSmall},
		Spec:       virtfoundryv1alpha1.OfferingSpec{DedicatedCPU: true, CPU: 1, MemoryMi: 512},
	}
	if resolveDedicatedCPU(inst, tenantOff) {
		t.Fatal("non-platform offering dedicatedCPU must be ignored")
	}

	platformOff := &virtfoundryv1alpha1.Offering{
		ObjectMeta: metav1.ObjectMeta{
			Name:   testOfferingSmall,
			Labels: map[string]string{labelPlatformOwned: annotationTruthy},
		},
		Spec: virtfoundryv1alpha1.OfferingSpec{CPU: 2, MemoryMi: 1024},
	}
	if resolveDedicatedCPU(&virtfoundryv1alpha1.Instance{}, platformOff) {
		t.Fatal("platform offering without dedicatedCPU flag")
	}
	if !resolveDedicatedCPU(inst, platformOff) {
		t.Fatal("platform offering + instance dedicatedCPU should apply")
	}
	platformOff.Spec.DedicatedCPU = true
	if !resolveDedicatedCPU(&virtfoundryv1alpha1.Instance{}, platformOff) {
		t.Fatal("platform offering.spec.dedicatedCPU should apply")
	}
}

func TestResolveVMBuildInput_DedicatedCPUGate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)

	tmpl := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{Name: testTemplateUbuntu2204, Namespace: operatorNamespace},
		Spec: virtfoundryv1alpha1.TemplateSpec{
			Image:      catalogUbuntuImage,
			SourceType: sourceTypeContainer,
			OSType:     osTypeLinux,
		},
	}
	tenantOff := &virtfoundryv1alpha1.Offering{
		ObjectMeta: metav1.ObjectMeta{Name: "tenant-large"},
		Spec: virtfoundryv1alpha1.OfferingSpec{
			DisplayName:  "Tenant Large",
			CPU:          4,
			MemoryMi:     8192,
			DedicatedCPU: true,
		},
	}
	platformOff := &virtfoundryv1alpha1.Offering{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "platform-large",
			Labels: map[string]string{labelPlatformOwned: annotationTruthy},
		},
		Spec: virtfoundryv1alpha1.OfferingSpec{
			DisplayName:  "Platform Large",
			CPU:          4,
			MemoryMi:     8192,
			DedicatedCPU: true,
		},
	}

	r := &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(tmpl, tenantOff, platformOff).Build(),
	}

	base := virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testVMName,
			Namespace: testTenantNS,
			Annotations: map[string]string{
				annotationAllowPodNetwork: annotationTruthy,
			},
		},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			DisplayName: testVMName,
			TemplateRef: &virtfoundryv1alpha1.LocalObjectRef{Name: testTemplateUbuntu2204},
		},
	}

	t.Run("ignores tenant offering dedicatedCPU", func(t *testing.T) {
		inst := base.DeepCopy()
		inst.Spec.OfferingRef = &virtfoundryv1alpha1.LocalObjectRef{Name: "tenant-large"}
		inst.Spec.DedicatedCPU = true
		in, err := r.resolveVMBuildInput(context.Background(), inst)
		if err != nil {
			t.Fatal(err)
		}
		if in.dedicatedCPU {
			t.Fatal("expected dedicatedCPU ignored without platform-owned label")
		}
		if in.cpu != 4 || in.memoryMi != 8192 {
			t.Fatalf("cpu/mem still from offering: cpu=%d mem=%d", in.cpu, in.memoryMi)
		}
	})

	t.Run("applies platform offering dedicatedCPU", func(t *testing.T) {
		inst := base.DeepCopy()
		inst.Spec.OfferingRef = &virtfoundryv1alpha1.LocalObjectRef{Name: "platform-large"}
		in, err := r.resolveVMBuildInput(context.Background(), inst)
		if err != nil {
			t.Fatal(err)
		}
		if !in.dedicatedCPU {
			t.Fatal("expected dedicatedCPU from platform-owned offering")
		}
	})

	t.Run("ignores instance dedicatedCPU without offering", func(t *testing.T) {
		inst := base.DeepCopy()
		inst.Spec.DedicatedCPU = true
		in, err := r.resolveVMBuildInput(context.Background(), inst)
		if err != nil {
			t.Fatal(err)
		}
		if in.dedicatedCPU {
			t.Fatal("expected instance-only dedicatedCPU ignored")
		}
	})
}
