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
	"fmt"
	"maps"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kubevirtv1 "kubevirt.io/api/core/v1"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

const (
	instanceFinalizer   = "virtfoundry.io/finalizer"
	instanceManagedBy   = "virtfoundry"
	powerStateRunning   = "Running"
	powerStateHalted    = "Halted"
	osTypeLinux         = "linux"
	sourceTypeContainer = "container"
	catalogUbuntuImage  = "quay.io/containerdisks/ubuntu:22.04"

	// cirrosDemoContainerDisk is an allowlisted demo image for explicit Template
	// CRs only. Instances never fall back to it when templateRef is missing
	// (issue #17 — Cirros default login is public).
	cirrosDemoContainerDisk = "quay.io/kubevirt/cirros-container-disk-demo"

	// annotationAllowPodNetwork opts an Instance into the KubeVirt pod network
	// (masquerade). Production path does not attach it by default — guests must
	// use Multus/VPC networks via spec.nics, or set this annotation to "true".
	// BREAKING CHANGE vs operator ≤0.7: Instances without NICs or this annotation
	// fail reconcile instead of landing on the cluster CNI.
	annotationAllowPodNetwork = "virtfoundry.io/allow-pod-network"
	annotationTruthy          = "true"

	volumeContainerDisk = "containerdisk"
	volumeCloudInitDisk = "cloudinitdisk"
)

// managedVolumeNames are Instance-owned volume names rewritten on every
// CreateOrUpdate. Other volumes (future PVC disks, etc.) are left alone.
var managedVolumeNames = map[string]struct{}{
	volumeContainerDisk: {},
	volumeCloudInitDisk: {},
}

type vmBuildInput struct {
	cpu          int
	memoryMi     int64
	image        string
	osType       string
	dedicatedCPU bool
	cloudInit    string
	powerState   string
	interfaces   []kubevirtv1.Interface
	networks     []kubevirtv1.Network
}

func instancePowerState(inst *virtfoundryv1alpha1.Instance) string {
	switch strings.TrimSpace(inst.Spec.PowerState) {
	case powerStateHalted:
		return powerStateHalted
	default:
		return powerStateRunning
	}
}

func (r *InstanceReconciler) resolveVMBuildInput(ctx context.Context, inst *virtfoundryv1alpha1.Instance) (vmBuildInput, error) {
	in := vmBuildInput{
		cpu:        1,
		memoryMi:   512,
		osType:     osTypeLinux,
		powerState: instancePowerState(inst),
	}

	if inst.Spec.OfferingRef != nil && inst.Spec.OfferingRef.Name != "" {
		off := &virtfoundryv1alpha1.Offering{}
		if err := r.Get(ctx, client.ObjectKey{Name: inst.Spec.OfferingRef.Name}, off); err != nil {
			return in, fmt.Errorf("offering %q: %w", inst.Spec.OfferingRef.Name, err)
		}
		if err := validateResolvedOffering(off); err != nil {
			return in, fmt.Errorf("offering %q: %w", off.Name, err)
		}
		in.cpu = off.Spec.CPU
		in.memoryMi = off.Spec.MemoryMi
		in.dedicatedCPU = off.Spec.DedicatedCPU || inst.Spec.DedicatedCPU
	} else {
		in.dedicatedCPU = inst.Spec.DedicatedCPU
	}

	// Issue #17: never default ContainerDisk to Cirros. TemplateRef is required.
	if inst.Spec.TemplateRef == nil || strings.TrimSpace(inst.Spec.TemplateRef.Name) == "" {
		return in, fmt.Errorf(
			"spec.templateRef is required: refuse to reconcile Instance %q without a Template (no default Cirros image)",
			inst.Name,
		)
	}

	tmpl, err := r.resolveTemplate(ctx, inst, inst.Spec.TemplateRef.Name)
	if err != nil {
		return in, err
	}
	if strings.EqualFold(tmpl.Spec.SourceType, "iso") {
		return in, fmt.Errorf("iso templates are not reconciled by the operator yet (template %q)", tmpl.Name)
	}
	if strings.TrimSpace(tmpl.Spec.Image) == "" {
		return in, fmt.Errorf("template %q has empty spec.image", tmpl.Name)
	}
	in.image = tmpl.Spec.Image
	if tmpl.Spec.OSType != "" {
		in.osType = tmpl.Spec.OSType
	}
	// Resolve cloud-init: Instance secret/string wins; else Template secret/legacy.
	// Never log the resolved body (operator#16).
	cloudInit, err := r.resolveCloudInitUserData(ctx, inst, tmpl)
	if err != nil {
		return in, err
	}
	in.cloudInit = cloudInit

	// Defense in depth for #22/#26: never copy an unlisted Template.spec.image
	// into ContainerDisk. Chart crAdmission also denies bad Template CRs at
	// admission when ValidatingAdmissionPolicy is available.
	if err := validateContainerDiskImage(in.image, r.AllowedContainerImagePrefixes); err != nil {
		return in, err
	}

	pubKeys, err := r.resolveSSHPublicKeys(ctx, inst)
	if err != nil {
		return in, err
	}
	if len(pubKeys) > 0 {
		merged, err := mergeCloudInitWithSSHKeys(in.cloudInit, pubKeys)
		if err != nil {
			return in, err
		}
		in.cloudInit = merged
	}

	ifaces, networks, err := r.resolveVMNetworks(ctx, inst)
	if err != nil {
		return in, err
	}
	in.interfaces = ifaces
	in.networks = networks

	return in, nil
}

func (r *InstanceReconciler) resolveTemplate(ctx context.Context, inst *virtfoundryv1alpha1.Instance, name string) (*virtfoundryv1alpha1.Template, error) {
	namespaces := []string{inst.Namespace, operatorNamespace}
	for _, ns := range namespaces {
		tmpl := &virtfoundryv1alpha1.Template{}
		err := r.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, tmpl)
		if err == nil {
			return tmpl, nil
		}
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("template %q not found in %s or %s", name, inst.Namespace, operatorNamespace)
}

// resolveVMNetworks builds guest NICs. Production default: Multus/VPC only.
// Pod masquerade requires an explicit opt-in annotation.
func (r *InstanceReconciler) resolveVMNetworks(
	ctx context.Context,
	inst *virtfoundryv1alpha1.Instance,
) ([]kubevirtv1.Interface, []kubevirtv1.Network, error) {
	if allowPodNetwork(inst) {
		return podNetworkAttachment()
	}

	if len(inst.Spec.Nics) == 0 {
		return nil, nil, fmt.Errorf(
			"no guest NICs configured: set spec.nics to attach Multus/VPC networks, or annotate %s=true to opt into the pod network",
			annotationAllowPodNetwork,
		)
	}

	ifaces := make([]kubevirtv1.Interface, 0, len(inst.Spec.Nics))
	networks := make([]kubevirtv1.Network, 0, len(inst.Spec.Nics))
	for i, nic := range inst.Spec.Nics {
		name := strings.TrimSpace(nic.Name)
		if name == "" {
			name = fmt.Sprintf("nic%d", i)
		}
		if nic.NetworkRef.Name == "" {
			return nil, nil, fmt.Errorf("spec.nics[%d]: networkRef.name is required", i)
		}

		net := &virtfoundryv1alpha1.Network{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: inst.Namespace, Name: nic.NetworkRef.Name}, net); err != nil {
			return nil, nil, fmt.Errorf("spec.nics[%d]: network %q: %w", i, nic.NetworkRef.Name, err)
		}
		if net.Status.NADName == "" {
			return nil, nil, fmt.Errorf(
				"spec.nics[%d]: network %q has no Multus NAD in status yet (NADName empty)",
				i, nic.NetworkRef.Name,
			)
		}

		nadRef := net.Status.NADName
		if net.Status.NADNamespace != "" && net.Status.NADNamespace != inst.Namespace {
			nadRef = net.Status.NADNamespace + "/" + net.Status.NADName
		}

		ifaces = append(ifaces, kubevirtv1.Interface{
			Name: name,
			InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
				Bridge: &kubevirtv1.InterfaceBridge{},
			},
		})
		networks = append(networks, kubevirtv1.Network{
			Name: name,
			NetworkSource: kubevirtv1.NetworkSource{
				Multus: &kubevirtv1.MultusNetwork{NetworkName: nadRef},
			},
		})
	}
	return ifaces, networks, nil
}

func allowPodNetwork(inst *virtfoundryv1alpha1.Instance) bool {
	if inst.Annotations == nil {
		return false
	}
	v := strings.TrimSpace(strings.ToLower(inst.Annotations[annotationAllowPodNetwork]))
	return v == annotationTruthy || v == "1" || v == "yes"
}

func podNetworkAttachment() ([]kubevirtv1.Interface, []kubevirtv1.Network, error) {
	ifaces := []kubevirtv1.Interface{{
		Name: podNetworkName,
		InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
			Masquerade: &kubevirtv1.InterfaceMasquerade{},
		},
	}}
	networks := []kubevirtv1.Network{{
		Name: podNetworkName,
		NetworkSource: kubevirtv1.NetworkSource{
			Pod: &kubevirtv1.PodNetwork{},
		},
	}}
	return ifaces, networks, nil
}

func (r *InstanceReconciler) ensureVirtualMachine(ctx context.Context, inst *virtfoundryv1alpha1.Instance, kvName string) error {
	input, err := r.resolveVMBuildInput(ctx, inst)
	if err != nil {
		return err
	}

	desired, err := buildVirtualMachine(inst, kvName, input)
	if err != nil {
		return err
	}
	vm := &kubevirtv1.VirtualMachine{}
	vm.Name = kvName
	vm.Namespace = inst.Namespace

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, vm, func() error {
		if vm.CreationTimestamp.IsZero() {
			vm.Labels = desired.Labels
			vm.Spec = desired.Spec
		} else {
			// Update path (issue #37): converge RunStrategy, NICs, and
			// managed ContainerDisk/cloud-init volumes. Offering CPU/memory
			// and DedicatedCPU remain create-time only (documented gap).
			applyDesiredVMOnUpdate(vm, desired)
		}
		if r.Scheme != nil {
			if err := controllerutil.SetControllerReference(inst, vm, r.Scheme); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// applyDesiredVMOnUpdate mutates an existing VirtualMachine toward desired
// Instance-owned fields without replacing the whole Spec (avoids wiping
// unknown/future volumes) and without leaving orphan managed volumes.
func applyDesiredVMOnUpdate(vm, desired *kubevirtv1.VirtualMachine) {
	if desired.Spec.RunStrategy != nil {
		vm.Spec.RunStrategy = desired.Spec.RunStrategy
	}
	if desired.Labels != nil {
		if vm.Labels == nil {
			vm.Labels = map[string]string{}
		}
		maps.Copy(vm.Labels, desired.Labels)
	}
	if desired.Spec.Template == nil {
		return
	}
	if vm.Spec.Template == nil {
		vm.Spec.Template = desired.Spec.Template.DeepCopy()
		return
	}

	desiredSpec := desired.Spec.Template.Spec
	vm.Spec.Template.Spec.Networks = desiredSpec.Networks
	vm.Spec.Template.Spec.Domain.Devices.Interfaces = desiredSpec.Domain.Devices.Interfaces
	vm.Spec.Template.Spec.Domain.Devices.Disks = mergeManagedDisks(
		vm.Spec.Template.Spec.Domain.Devices.Disks,
		desiredSpec.Domain.Devices.Disks,
	)
	vm.Spec.Template.Spec.Volumes = mergeManagedVolumes(
		vm.Spec.Template.Spec.Volumes,
		desiredSpec.Volumes,
	)
}

// mergeManagedVolumes replaces Instance-owned volumes by name, drops managed
// orphans no longer in desired, and preserves any non-managed volumes.
func mergeManagedVolumes(existing, desired []kubevirtv1.Volume) []kubevirtv1.Volume {
	desiredByName := make(map[string]kubevirtv1.Volume, len(desired))
	for _, v := range desired {
		desiredByName[v.Name] = v
	}

	out := make([]kubevirtv1.Volume, 0, len(existing)+len(desired))
	seen := make(map[string]struct{}, len(desired))
	for _, v := range existing {
		if _, managed := managedVolumeNames[v.Name]; !managed {
			out = append(out, v)
			continue
		}
		if d, ok := desiredByName[v.Name]; ok {
			out = append(out, d)
			seen[v.Name] = struct{}{}
		}
		// else: drop orphan managed volume (e.g. renamed away)
	}
	for _, v := range desired {
		if _, ok := seen[v.Name]; ok {
			continue
		}
		if _, managed := managedVolumeNames[v.Name]; managed {
			out = append(out, v)
		}
	}
	return out
}

// mergeManagedDisks mirrors mergeManagedVolumes for Domain.Devices.Disks.
func mergeManagedDisks(existing, desired []kubevirtv1.Disk) []kubevirtv1.Disk {
	desiredByName := make(map[string]kubevirtv1.Disk, len(desired))
	for _, d := range desired {
		desiredByName[d.Name] = d
	}

	out := make([]kubevirtv1.Disk, 0, len(existing)+len(desired))
	seen := make(map[string]struct{}, len(desired))
	for _, d := range existing {
		if _, managed := managedVolumeNames[d.Name]; !managed {
			out = append(out, d)
			continue
		}
		if want, ok := desiredByName[d.Name]; ok {
			out = append(out, want)
			seen[d.Name] = struct{}{}
		}
	}
	for _, d := range desired {
		if _, ok := seen[d.Name]; ok {
			continue
		}
		if _, managed := managedVolumeNames[d.Name]; managed {
			out = append(out, d)
		}
	}
	return out
}

func (r *InstanceReconciler) deleteVirtualMachine(ctx context.Context, namespace, name string) error {
	vm := &kubevirtv1.VirtualMachine{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, vm); err != nil {
		return client.IgnoreNotFound(err)
	}
	return r.Delete(ctx, vm)
}

func buildVirtualMachine(inst *virtfoundryv1alpha1.Instance, kvName string, in vmBuildInput) (*kubevirtv1.VirtualMachine, error) {
	runStrategy := kubevirtv1.RunStrategyAlways
	if in.powerState == powerStateHalted {
		runStrategy = kubevirtv1.RunStrategyHalted
	}

	cpu, err := guestCPUSpec(in.cpu, in.dedicatedCPU)
	if err != nil {
		return nil, err
	}
	resources, err := vmResourceRequirements(in.memoryMi, in.cpu, in.dedicatedCPU)
	if err != nil {
		return nil, err
	}

	vmiSpec := kubevirtv1.VirtualMachineInstanceSpec{
		Domain: kubevirtv1.DomainSpec{
			CPU: cpu,
			Devices: kubevirtv1.Devices{
				Disks:      linuxDisks(),
				Interfaces: in.interfaces,
			},
			Resources: resources,
		},
		Volumes:  linuxVolumes(in.image, in.cloudInit),
		Networks: in.networks,
	}

	return &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kvName,
			Namespace: inst.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": instanceManagedBy,
				"app.kubernetes.io/part-of":    "virtfoundry",
				"virtfoundry.io/instance":      inst.Name,
			},
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: &runStrategy,
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"kubevirt.io/domain": kvName,
						"virtfoundry.io/vm":  kvName,
					},
				},
				Spec: vmiSpec,
			},
		},
	}, nil
}

func linuxDisks() []kubevirtv1.Disk {
	return []kubevirtv1.Disk{
		{Name: volumeContainerDisk, DiskDevice: kubevirtv1.DiskDevice{Disk: &kubevirtv1.DiskTarget{Bus: "virtio"}}},
		{Name: volumeCloudInitDisk, DiskDevice: kubevirtv1.DiskDevice{Disk: &kubevirtv1.DiskTarget{Bus: "virtio"}}},
	}
}

func linuxVolumes(image, cloudInit string) []kubevirtv1.Volume {
	userData := cloudInit
	if userData == "" {
		userData = "#cloud-config\n"
	}
	return []kubevirtv1.Volume{
		{
			Name: volumeContainerDisk,
			VolumeSource: kubevirtv1.VolumeSource{
				ContainerDisk: &kubevirtv1.ContainerDiskSource{Image: image},
			},
		},
		{
			Name: volumeCloudInitDisk,
			VolumeSource: kubevirtv1.VolumeSource{
				CloudInitNoCloud: &kubevirtv1.CloudInitNoCloudSource{UserData: userData},
			},
		},
	}
}

func guestCPUSpec(cores int, dedicated bool) (*kubevirtv1.CPU, error) {
	if err := validateGuestResources(cores, 64); err != nil {
		return nil, err
	}
	cpu := &kubevirtv1.CPU{Cores: uint32(cores)}
	if dedicated {
		cpu.DedicatedCPUPlacement = true
	}
	return cpu, nil
}

func vmResourceRequirements(memMi int64, cpu int, dedicated bool) (kubevirtv1.ResourceRequirements, error) {
	if err := validateGuestResources(cpu, memMi); err != nil {
		return kubevirtv1.ResourceRequirements{}, err
	}
	mem, err := resource.ParseQuantity(fmt.Sprintf("%dMi", memMi))
	if err != nil {
		return kubevirtv1.ResourceRequirements{}, fmt.Errorf("parse memory %dMi: %w", memMi, err)
	}
	reqs := corev1.ResourceList{corev1.ResourceMemory: mem}
	limits := corev1.ResourceList{corev1.ResourceMemory: mem}
	if dedicated {
		cpuQty, err := resource.ParseQuantity(fmt.Sprintf("%d", cpu))
		if err != nil {
			return kubevirtv1.ResourceRequirements{}, fmt.Errorf("parse cpu %d: %w", cpu, err)
		}
		reqs[corev1.ResourceCPU] = cpuQty
		limits[corev1.ResourceCPU] = cpuQty
	}
	return kubevirtv1.ResourceRequirements{Requests: reqs, Limits: limits}, nil
}
