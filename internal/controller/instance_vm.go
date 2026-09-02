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
	defaultContainerImg = "quay.io/kubevirt/cirros-container-disk-demo"
)

type vmBuildInput struct {
	cpu          int
	memoryMi     int64
	image        string
	osType       string
	dedicatedCPU bool
	cloudInit    string
	powerState   string
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
		image:      defaultContainerImg,
		osType:     "linux",
		powerState: instancePowerState(inst),
	}

	if inst.Spec.OfferingRef != nil && inst.Spec.OfferingRef.Name != "" {
		off := &virtfoundryv1alpha1.Offering{}
		if err := r.Get(ctx, client.ObjectKey{Name: inst.Spec.OfferingRef.Name}, off); err != nil {
			return in, fmt.Errorf("offering %q: %w", inst.Spec.OfferingRef.Name, err)
		}
		if off.Spec.CPU > 0 {
			in.cpu = off.Spec.CPU
		}
		if off.Spec.MemoryMi > 0 {
			in.memoryMi = off.Spec.MemoryMi
		}
		in.dedicatedCPU = off.Spec.DedicatedCPU || inst.Spec.DedicatedCPU
	} else {
		in.dedicatedCPU = inst.Spec.DedicatedCPU
	}

	if inst.Spec.TemplateRef != nil && inst.Spec.TemplateRef.Name != "" {
		tmpl, err := r.resolveTemplate(ctx, inst, inst.Spec.TemplateRef.Name)
		if err != nil {
			return in, err
		}
		if strings.EqualFold(tmpl.Spec.SourceType, "iso") {
			return in, fmt.Errorf("iso templates are not reconciled by the operator yet (template %q)", tmpl.Name)
		}
		if tmpl.Spec.Image != "" {
			in.image = tmpl.Spec.Image
		}
		if tmpl.Spec.OSType != "" {
			in.osType = tmpl.Spec.OSType
		}
		in.cloudInit = tmpl.Spec.CloudInitUserData
	}

	return in, nil
}

func (r *InstanceReconciler) resolveTemplate(ctx context.Context, inst *virtfoundryv1alpha1.Instance, name string) (*virtfoundryv1alpha1.Template, error) {
	namespaces := []string{inst.Namespace, "virtfoundry-system"}
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
	return nil, fmt.Errorf("template %q not found in %s or virtfoundry-system", name, inst.Namespace)
}

func (r *InstanceReconciler) ensureVirtualMachine(ctx context.Context, inst *virtfoundryv1alpha1.Instance, kvName string) error {
	input, err := r.resolveVMBuildInput(ctx, inst)
	if err != nil {
		return err
	}

	desired := buildVirtualMachine(inst, kvName, input)
	vm := &kubevirtv1.VirtualMachine{}
	vm.Name = kvName
	vm.Namespace = inst.Namespace

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, vm, func() error {
		if vm.CreationTimestamp.IsZero() {
			vm.Labels = desired.Labels
			vm.Spec = desired.Spec
		} else if desired.Spec.RunStrategy != nil {
			vm.Spec.RunStrategy = desired.Spec.RunStrategy
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

func (r *InstanceReconciler) deleteVirtualMachine(ctx context.Context, namespace, name string) error {
	vm := &kubevirtv1.VirtualMachine{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, vm); err != nil {
		return client.IgnoreNotFound(err)
	}
	return r.Delete(ctx, vm)
}

func buildVirtualMachine(inst *virtfoundryv1alpha1.Instance, kvName string, in vmBuildInput) *kubevirtv1.VirtualMachine {
	runStrategy := kubevirtv1.RunStrategyAlways
	if in.powerState == powerStateHalted {
		runStrategy = kubevirtv1.RunStrategyHalted
	}

	ifaces := []kubevirtv1.Interface{{
		Name: "default",
		InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
			Masquerade: &kubevirtv1.InterfaceMasquerade{},
		},
	}}
	networks := []kubevirtv1.Network{{
		Name: "default",
		NetworkSource: kubevirtv1.NetworkSource{
			Pod: &kubevirtv1.PodNetwork{},
		},
	}}

	vmiSpec := kubevirtv1.VirtualMachineInstanceSpec{
		Domain: kubevirtv1.DomainSpec{
			CPU:       guestCPUSpec(in.cpu, in.dedicatedCPU),
			Devices:   kubevirtv1.Devices{Disks: linuxDisks(), Interfaces: ifaces},
			Resources: vmResourceRequirements(in.memoryMi, in.cpu, in.dedicatedCPU),
		},
		Volumes:  linuxVolumes(in.image, in.cloudInit),
		Networks: networks,
	}

	return &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kvName,
			Namespace: inst.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": instanceManagedBy,
				"app.kubernetes.io/part-of":      "virtfoundry",
				"virtfoundry.io/instance":        inst.Name,
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
	}
}

func linuxDisks() []kubevirtv1.Disk {
	return []kubevirtv1.Disk{
		{Name: "containerdisk", DiskDevice: kubevirtv1.DiskDevice{Disk: &kubevirtv1.DiskTarget{Bus: "virtio"}}},
		{Name: "cloudinitdisk", DiskDevice: kubevirtv1.DiskDevice{Disk: &kubevirtv1.DiskTarget{Bus: "virtio"}}},
	}
}

func linuxVolumes(image, cloudInit string) []kubevirtv1.Volume {
	userData := cloudInit
	if userData == "" {
		userData = "#cloud-config\n"
	}
	return []kubevirtv1.Volume{
		{
			Name: "containerdisk",
			VolumeSource: kubevirtv1.VolumeSource{
				ContainerDisk: &kubevirtv1.ContainerDiskSource{Image: image},
			},
		},
		{
			Name: "cloudinitdisk",
			VolumeSource: kubevirtv1.VolumeSource{
				CloudInitNoCloud: &kubevirtv1.CloudInitNoCloudSource{UserData: userData},
			},
		},
	}
}

func guestCPUSpec(cores int, dedicated bool) *kubevirtv1.CPU {
	cpu := &kubevirtv1.CPU{Cores: uint32(cores)}
	if dedicated {
		cpu.DedicatedCPUPlacement = true
	}
	return cpu
}

func vmResourceRequirements(memMi int64, cpu int, dedicated bool) kubevirtv1.ResourceRequirements {
	mem := resource.MustParse(fmt.Sprintf("%dMi", memMi))
	reqs := corev1.ResourceList{corev1.ResourceMemory: mem}
	limits := corev1.ResourceList{corev1.ResourceMemory: mem}
	if dedicated {
		cpuQty := resource.MustParse(fmt.Sprintf("%d", cpu))
		reqs[corev1.ResourceCPU] = cpuQty
		limits[corev1.ResourceCPU] = cpuQty
	}
	return kubevirtv1.ResourceRequirements{Requests: reqs, Limits: limits}
}
