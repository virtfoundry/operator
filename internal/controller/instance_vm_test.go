package controller

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kubevirtv1 "kubevirt.io/api/core/v1"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

const (
	testVMName   = "demo"
	testTenantNS = "virtfoundry-tenant-default"
	testVPCNet   = "vpc-net"
)

func TestBuildVirtualMachine_RunningWithPodNetworkOptIn(t *testing.T) {
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testVMName,
			Namespace: testTenantNS,
			Annotations: map[string]string{
				annotationAllowPodNetwork: annotationTruthy,
			},
		},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			DisplayName: testVMName,
			PowerState:  powerStateRunning,
		},
	}
	ifaces, networks, err := podNetworkAttachment()
	if err != nil {
		t.Fatal(err)
	}
	vm, err := buildVirtualMachine(inst, testVMName, vmBuildInput{
		cpu:        1,
		memoryMi:   1024,
		image:      "quay.io/containerdisks/ubuntu:22.04",
		osType:     "linux",
		powerState: powerStateRunning,
		interfaces: ifaces,
		networks:   networks,
	})
	if err != nil {
		t.Fatalf("buildVirtualMachine: %v", err)
	}

	if vm.Name != testVMName {
		t.Fatalf("name: got %q", vm.Name)
	}
	if vm.Spec.RunStrategy == nil || *vm.Spec.RunStrategy != kubevirtv1.RunStrategyAlways {
		t.Fatalf("expected RunStrategyAlways, got %#v", vm.Spec.RunStrategy)
	}
	if vm.Spec.Template.Spec.Volumes[0].ContainerDisk.Image != "quay.io/containerdisks/ubuntu:22.04" {
		t.Fatalf("unexpected image: %#v", vm.Spec.Template.Spec.Volumes[0].ContainerDisk)
	}
	if len(vm.Spec.Template.Spec.Networks) != 1 || vm.Spec.Template.Spec.Networks[0].Pod == nil {
		t.Fatalf("expected pod network, got %#v", vm.Spec.Template.Spec.Networks)
	}
}

func TestBuildVirtualMachine_Halted(t *testing.T) {
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: testVMName, Namespace: tenantNamespaceName(testAcmeSlug)},
	}
	vm, err := buildVirtualMachine(inst, testVMName, vmBuildInput{
		cpu:        1,
		memoryMi:   512,
		powerState: powerStateHalted,
	})
	if err != nil {
		t.Fatalf("buildVirtualMachine: %v", err)
	}
	if vm.Spec.RunStrategy == nil || *vm.Spec.RunStrategy != kubevirtv1.RunStrategyHalted {
		t.Fatalf("expected RunStrategyHalted, got %#v", vm.Spec.RunStrategy)
	}
}

func TestBuildVirtualMachine_RejectsInvalidResources(t *testing.T) {
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: testVMName, Namespace: tenantNamespaceName(testAcmeSlug)},
	}
	if _, err := buildVirtualMachine(inst, testVMName, vmBuildInput{cpu: -1, memoryMi: 512}); err == nil {
		t.Fatal("expected error for negative cpu")
	}
	if _, err := buildVirtualMachine(inst, testVMName, vmBuildInput{cpu: 1, memoryMi: 0}); err == nil {
		t.Fatal("expected error for zero memory")
	}
}

func TestInstancePowerState_DefaultsRunning(t *testing.T) {
	inst := &virtfoundryv1alpha1.Instance{}
	if got := instancePowerState(inst); got != powerStateRunning {
		t.Fatalf("got %q", got)
	}
}

func TestResolveVMNetworks_FailsWithoutNicsOrOptIn(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)
	r := &InstanceReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}

	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: testVMName, Namespace: testTenantNS},
		Spec:       virtfoundryv1alpha1.InstanceSpec{DisplayName: testVMName},
	}
	_, _, err := r.resolveVMNetworks(context.Background(), inst)
	if err == nil {
		t.Fatal("expected error when no NICs and no opt-in")
	}
	if !strings.Contains(err.Error(), annotationAllowPodNetwork) {
		t.Fatalf("error should mention opt-in annotation, got: %v", err)
	}
}

func TestResolveVMNetworks_MultusFromNetworkStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)

	net := &virtfoundryv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: testVPCNet, Namespace: testTenantNS},
		Spec:       virtfoundryv1alpha1.NetworkSpec{Name: testVPCNet, NetworkType: "isolated", CIDR: "10.0.0.0/24"},
		Status: virtfoundryv1alpha1.NetworkStatus{
			NADNamespace: testTenantNS,
			NADName:      testVPCNet + "-nad",
		},
	}
	r := &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(net).Build(),
	}

	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: testVMName, Namespace: testTenantNS},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			DisplayName: testVMName,
			Nics: []virtfoundryv1alpha1.InstanceNicSpec{{
				Name:       "eth0",
				NetworkRef: virtfoundryv1alpha1.LocalObjectRef{Name: testVPCNet},
			}},
		},
	}

	ifaces, networks, err := r.resolveVMNetworks(context.Background(), inst)
	if err != nil {
		t.Fatal(err)
	}
	if len(ifaces) != 1 || ifaces[0].Bridge == nil {
		t.Fatalf("expected bridge iface, got %#v", ifaces)
	}
	wantNAD := testVPCNet + "-nad"
	if len(networks) != 1 || networks[0].Multus == nil || networks[0].Multus.NetworkName != wantNAD {
		t.Fatalf("expected multus nad %q, got %#v", wantNAD, networks)
	}
}

func TestAllowPodNetwork(t *testing.T) {
	inst := &virtfoundryv1alpha1.Instance{}
	if allowPodNetwork(inst) {
		t.Fatal("expected false without annotation")
	}
	inst.Annotations = map[string]string{annotationAllowPodNetwork: annotationTruthy}
	if !allowPodNetwork(inst) {
		t.Fatal("expected true")
	}
}
