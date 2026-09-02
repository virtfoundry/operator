package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

func TestBuildVirtualMachine_Running(t *testing.T) {
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "virtfoundry-tenant-default"},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			DisplayName: "demo",
			PowerState:  powerStateRunning,
		},
	}
	vm := buildVirtualMachine(inst, "demo", vmBuildInput{
		cpu:        1,
		memoryMi:   1024,
		image:      "quay.io/containerdisks/ubuntu:22.04",
		osType:     "linux",
		powerState: powerStateRunning,
	})

	if vm.Name != "demo" {
		t.Fatalf("name: got %q", vm.Name)
	}
	if vm.Spec.RunStrategy == nil || *vm.Spec.RunStrategy != kubevirtv1.RunStrategyAlways {
		t.Fatalf("expected RunStrategyAlways, got %#v", vm.Spec.RunStrategy)
	}
	if vm.Spec.Template.Spec.Volumes[0].ContainerDisk.Image != "quay.io/containerdisks/ubuntu:22.04" {
		t.Fatalf("unexpected image: %#v", vm.Spec.Template.Spec.Volumes[0].ContainerDisk)
	}
}

func TestBuildVirtualMachine_Halted(t *testing.T) {
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "ns"},
	}
	vm := buildVirtualMachine(inst, "demo", vmBuildInput{powerState: powerStateHalted})
	if vm.Spec.RunStrategy == nil || *vm.Spec.RunStrategy != kubevirtv1.RunStrategyHalted {
		t.Fatalf("expected RunStrategyHalted, got %#v", vm.Spec.RunStrategy)
	}
}

func TestInstancePowerState_DefaultsRunning(t *testing.T) {
	inst := &virtfoundryv1alpha1.Instance{}
	if got := instancePowerState(inst); got != powerStateRunning {
		t.Fatalf("got %q", got)
	}
}
