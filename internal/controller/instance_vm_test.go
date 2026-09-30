package controller

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		image:      catalogUbuntuImage,
		osType:     osTypeLinux,
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
	if vm.Spec.Template.Spec.Volumes[0].ContainerDisk.Image != catalogUbuntuImage {
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

func TestResolveVMBuildInput_FailsWithoutTemplate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)
	r := &InstanceReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}

	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testVMName,
			Namespace: testTenantNS,
			Annotations: map[string]string{
				annotationAllowPodNetwork: annotationTruthy,
			},
		},
		Spec: virtfoundryv1alpha1.InstanceSpec{DisplayName: testVMName},
	}

	in, err := r.resolveVMBuildInput(context.Background(), inst)
	if err == nil {
		t.Fatal("expected error when templateRef is missing")
	}
	if !strings.Contains(err.Error(), "templateRef") {
		t.Fatalf("error should mention templateRef, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Cirros") {
		t.Fatalf("error should mention Cirros default refusal, got: %v", err)
	}
	if in.image == cirrosDemoContainerDisk || strings.Contains(in.image, "cirros") {
		t.Fatalf("must not default image to Cirros, got %q", in.image)
	}
}

func TestResolveVMBuildInput_FailsWithEmptyTemplateRefName(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)
	r := &InstanceReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}

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
			TemplateRef: &virtfoundryv1alpha1.LocalObjectRef{Name: "  "},
		},
	}

	_, err := r.resolveVMBuildInput(context.Background(), inst)
	if err == nil || !strings.Contains(err.Error(), "templateRef") {
		t.Fatalf("expected templateRef required error, got %v", err)
	}
}

func TestResolveVMBuildInput_FailsWhenTemplateImageEmpty(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)
	tmpl := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{Name: "blank", Namespace: operatorNamespace},
		Spec: virtfoundryv1alpha1.TemplateSpec{
			SourceType: sourceTypeContainer,
			OSType:     osTypeLinux,
		},
	}
	r := &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(tmpl).Build(),
	}
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
			TemplateRef: &virtfoundryv1alpha1.LocalObjectRef{Name: "blank"},
		},
	}

	_, err := r.resolveVMBuildInput(context.Background(), inst)
	if err == nil || !strings.Contains(err.Error(), "empty spec.image") {
		t.Fatalf("expected empty image error, got %v", err)
	}
}

func TestResolveVMBuildInput_UsesExplicitCirrosTemplate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)
	tmpl := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{Name: "cirros", Namespace: operatorNamespace},
		Spec: virtfoundryv1alpha1.TemplateSpec{
			Image:      cirrosDemoContainerDisk,
			SourceType: sourceTypeContainer,
			OSType:     osTypeLinux,
		},
	}
	r := &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(tmpl).Build(),
	}
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
			TemplateRef: &virtfoundryv1alpha1.LocalObjectRef{Name: "cirros"},
		},
	}

	in, err := r.resolveVMBuildInput(context.Background(), inst)
	if err != nil {
		t.Fatal(err)
	}
	if in.image != cirrosDemoContainerDisk {
		t.Fatalf("image=%q", in.image)
	}
}

func TestMergeManagedVolumes_UpdatesImageDropsOrphanPreservesForeign(t *testing.T) {
	existing := []kubevirtv1.Volume{
		{
			Name: volumeContainerDisk,
			VolumeSource: kubevirtv1.VolumeSource{
				ContainerDisk: &kubevirtv1.ContainerDiskSource{Image: "old-image"},
			},
		},
		{
			Name: volumeCloudInitDisk,
			VolumeSource: kubevirtv1.VolumeSource{
				CloudInitNoCloud: &kubevirtv1.CloudInitNoCloudSource{UserData: "old"},
			},
		},
		{Name: "datadisk"}, // non-managed — keep
	}
	desired := linuxVolumes(catalogUbuntuImage, "#cloud-config\npackages: [nginx]\n")

	got := mergeManagedVolumes(existing, desired)
	if len(got) != 3 {
		t.Fatalf("expected 3 volumes (2 managed + foreign), got %#v", got)
	}
	byName := map[string]kubevirtv1.Volume{}
	for _, v := range got {
		byName[v.Name] = v
	}
	if byName[volumeContainerDisk].ContainerDisk == nil || byName[volumeContainerDisk].ContainerDisk.Image != catalogUbuntuImage {
		t.Fatalf("containerdisk not updated: %#v", byName[volumeContainerDisk])
	}
	if byName[volumeCloudInitDisk].CloudInitNoCloud == nil ||
		!strings.Contains(byName[volumeCloudInitDisk].CloudInitNoCloud.UserData, "nginx") {
		t.Fatalf("cloudinit not updated: %#v", byName[volumeCloudInitDisk])
	}
	if _, ok := byName["datadisk"]; !ok {
		t.Fatal("foreign volume datadisk was dropped")
	}
}

func TestApplyDesiredVMOnUpdate_ConvergesNicsAndImage(t *testing.T) {
	runAlways := kubevirtv1.RunStrategyAlways
	runHalted := kubevirtv1.RunStrategyHalted
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testVMName,
			Namespace:         testTenantNS,
			CreationTimestamp: metav1.Now(),
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: &runAlways,
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							Disks: linuxDisks(),
							Interfaces: []kubevirtv1.Interface{{
								Name: "old0",
								InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
									Bridge: &kubevirtv1.InterfaceBridge{},
								},
							}},
						},
					},
					Networks: []kubevirtv1.Network{{
						Name: "old0",
						NetworkSource: kubevirtv1.NetworkSource{
							Multus: &kubevirtv1.MultusNetwork{NetworkName: "old-nad"},
						},
					}},
					Volumes: append(linuxVolumes("old-image", "old-ci"), kubevirtv1.Volume{Name: "datadisk"}),
				},
			},
		},
	}
	desired := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"app.kubernetes.io/managed-by": instanceManagedBy},
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: &runHalted,
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							Disks: linuxDisks(),
							Interfaces: []kubevirtv1.Interface{{
								Name: "eth0",
								InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
									Bridge: &kubevirtv1.InterfaceBridge{},
								},
							}},
						},
					},
					Networks: []kubevirtv1.Network{{
						Name: "eth0",
						NetworkSource: kubevirtv1.NetworkSource{
							Multus: &kubevirtv1.MultusNetwork{NetworkName: "new-nad"},
						},
					}},
					Volumes: linuxVolumes(catalogUbuntuImage, "#cloud-config\n"),
				},
			},
		},
	}

	applyDesiredVMOnUpdate(vm, desired)

	if vm.Spec.RunStrategy == nil || *vm.Spec.RunStrategy != kubevirtv1.RunStrategyHalted {
		t.Fatalf("RunStrategy not converged: %#v", vm.Spec.RunStrategy)
	}
	if len(vm.Spec.Template.Spec.Networks) != 1 ||
		vm.Spec.Template.Spec.Networks[0].Name != "eth0" ||
		vm.Spec.Template.Spec.Networks[0].Multus == nil ||
		vm.Spec.Template.Spec.Networks[0].Multus.NetworkName != "new-nad" {
		t.Fatalf("networks not converged: %#v", vm.Spec.Template.Spec.Networks)
	}
	if len(vm.Spec.Template.Spec.Domain.Devices.Interfaces) != 1 ||
		vm.Spec.Template.Spec.Domain.Devices.Interfaces[0].Name != "eth0" {
		t.Fatalf("interfaces not converged: %#v", vm.Spec.Template.Spec.Domain.Devices.Interfaces)
	}
	foundData := false
	for _, v := range vm.Spec.Template.Spec.Volumes {
		switch v.Name {
		case volumeContainerDisk:
			if v.ContainerDisk == nil || v.ContainerDisk.Image != catalogUbuntuImage {
				t.Fatalf("image not converged: %#v", v)
			}
		case "datadisk":
			foundData = true
		case "old0":
			t.Fatal("orphan managed network volume should not appear in volumes")
		}
	}
	if !foundData {
		t.Fatal("foreign datadisk volume was orphaned/dropped")
	}
}

func TestEnsureVirtualMachine_UpdatesNicsOnExistingVM(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)
	_ = kubevirtv1.AddToScheme(scheme)

	netA := &virtfoundryv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: "net-a", Namespace: testTenantNS},
		Status:     virtfoundryv1alpha1.NetworkStatus{NADName: "net-a-nad", NADNamespace: testTenantNS},
	}
	netB := &virtfoundryv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: "net-b", Namespace: testTenantNS},
		Status:     virtfoundryv1alpha1.NetworkStatus{NADName: "net-b-nad", NADNamespace: testTenantNS},
	}
	tmpl := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{Name: testTemplateName, Namespace: operatorNamespace},
		Spec: virtfoundryv1alpha1.TemplateSpec{
			Image: catalogUbuntuImage, SourceType: sourceTypeContainer, OSType: osTypeLinux,
		},
	}
	runAlways := kubevirtv1.RunStrategyAlways
	existing := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testVMName,
			Namespace:         testTenantNS,
			CreationTimestamp: metav1.Now(),
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: &runAlways,
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							Disks: linuxDisks(),
							Interfaces: []kubevirtv1.Interface{{
								Name: "eth0",
								InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
									Bridge: &kubevirtv1.InterfaceBridge{},
								},
							}},
						},
					},
					Networks: []kubevirtv1.Network{{
						Name: "eth0",
						NetworkSource: kubevirtv1.NetworkSource{
							Multus: &kubevirtv1.MultusNetwork{NetworkName: "net-a-nad"},
						},
					}},
					Volumes: linuxVolumes(catalogUbuntuImage, "#cloud-config\n"),
				},
			},
		},
	}

	r := &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(netA, netB, tmpl, existing).Build(),
	}
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: testVMName, Namespace: testTenantNS},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			DisplayName: testVMName,
			TemplateRef: &virtfoundryv1alpha1.LocalObjectRef{Name: testTemplateName},
			Nics: []virtfoundryv1alpha1.InstanceNicSpec{{
				Name:       "eth1",
				NetworkRef: virtfoundryv1alpha1.LocalObjectRef{Name: "net-b"},
			}},
		},
	}

	if err := r.ensureVirtualMachine(context.Background(), inst, testVMName); err != nil {
		t.Fatal(err)
	}

	got := &kubevirtv1.VirtualMachine{}
	if err := r.Get(context.Background(), client.ObjectKey{Namespace: testTenantNS, Name: testVMName}, got); err != nil {
		t.Fatal(err)
	}
	if len(got.Spec.Template.Spec.Networks) != 1 ||
		got.Spec.Template.Spec.Networks[0].Name != "eth1" ||
		got.Spec.Template.Spec.Networks[0].Multus == nil ||
		got.Spec.Template.Spec.Networks[0].Multus.NetworkName != "net-b-nad" {
		t.Fatalf("NIC not converged on existing VM: %#v", got.Spec.Template.Spec.Networks)
	}
	if len(got.Spec.Template.Spec.Domain.Devices.Interfaces) != 1 ||
		got.Spec.Template.Spec.Domain.Devices.Interfaces[0].Name != "eth1" {
		t.Fatalf("iface not converged: %#v", got.Spec.Template.Spec.Domain.Devices.Interfaces)
	}
}

func TestEnsureVirtualMachine_UpdatesImageWithoutOrphanVolumes(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)
	_ = kubevirtv1.AddToScheme(scheme)

	oldTmpl := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{Name: "old-ubuntu", Namespace: operatorNamespace},
		Spec: virtfoundryv1alpha1.TemplateSpec{
			Image: "quay.io/containerdisks/ubuntu:20.04", SourceType: sourceTypeContainer, OSType: osTypeLinux,
		},
	}
	newTmpl := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{Name: "new-ubuntu", Namespace: operatorNamespace},
		Spec: virtfoundryv1alpha1.TemplateSpec{
			Image: catalogUbuntuImage, SourceType: sourceTypeContainer, OSType: osTypeLinux,
			CloudInitUserData: "#cloud-config\npackages:\n  - curl\n",
		},
	}
	runAlways := kubevirtv1.RunStrategyAlways
	existing := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testVMName,
			Namespace:         testTenantNS,
			CreationTimestamp: metav1.Now(),
			Annotations:       map[string]string{annotationAllowPodNetwork: annotationTruthy},
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: &runAlways,
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							Disks:      linuxDisks(),
							Interfaces: mustPodIfaces(t),
						},
					},
					Networks: mustPodNetworks(t),
					Volumes: append(
						linuxVolumes("quay.io/containerdisks/ubuntu:20.04", "stale"),
						kubevirtv1.Volume{Name: "datadisk"},
					),
				},
			},
		},
	}

	r := &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(oldTmpl, newTmpl, existing).Build(),
	}
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:        testVMName,
			Namespace:   testTenantNS,
			Annotations: map[string]string{annotationAllowPodNetwork: annotationTruthy},
		},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			DisplayName: testVMName,
			TemplateRef: &virtfoundryv1alpha1.LocalObjectRef{Name: "new-ubuntu"},
		},
	}

	if err := r.ensureVirtualMachine(context.Background(), inst, testVMName); err != nil {
		t.Fatal(err)
	}

	got := &kubevirtv1.VirtualMachine{}
	if err := r.Get(context.Background(), client.ObjectKey{Namespace: testTenantNS, Name: testVMName}, got); err != nil {
		t.Fatal(err)
	}

	byName := map[string]kubevirtv1.Volume{}
	for _, v := range got.Spec.Template.Spec.Volumes {
		byName[v.Name] = v
	}
	if len(byName) != 3 {
		t.Fatalf("expected 3 volumes (no orphan managed), got %#v", got.Spec.Template.Spec.Volumes)
	}
	if byName[volumeContainerDisk].ContainerDisk == nil ||
		byName[volumeContainerDisk].ContainerDisk.Image != catalogUbuntuImage {
		t.Fatalf("image not updated: %#v", byName[volumeContainerDisk])
	}
	if byName[volumeCloudInitDisk].CloudInitNoCloud == nil ||
		!strings.Contains(byName[volumeCloudInitDisk].CloudInitNoCloud.UserData, "curl") {
		t.Fatalf("cloud-init not updated from TemplateRef: %#v", byName[volumeCloudInitDisk])
	}
	if _, ok := byName["datadisk"]; !ok {
		t.Fatal("foreign datadisk orphaned")
	}
}

func mustPodIfaces(t *testing.T) []kubevirtv1.Interface {
	t.Helper()
	ifaces, _, err := podNetworkAttachment()
	if err != nil {
		t.Fatal(err)
	}
	return ifaces
}

func mustPodNetworks(t *testing.T) []kubevirtv1.Network {
	t.Helper()
	_, networks, err := podNetworkAttachment()
	if err != nil {
		t.Fatal(err)
	}
	return networks
}
