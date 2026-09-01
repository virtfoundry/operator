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
	"strings"

	corev1 "k8s.io/api/core/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

func kubeVirtVMName(instanceName, statusName string) string {
	if statusName != "" {
		return statusName
	}
	return instanceName
}

func instancePhaseFromVM(vm *kubevirtv1.VirtualMachine) string {
	switch vm.Status.PrintableStatus {
	case kubevirtv1.VirtualMachineStatusRunning:
		return "Running"
	case kubevirtv1.VirtualMachineStatusStopped:
		return "Stopped"
	case kubevirtv1.VirtualMachineStatusStarting, kubevirtv1.VirtualMachineStatusProvisioning:
		return "Starting"
	case kubevirtv1.VirtualMachineStatusStopping, kubevirtv1.VirtualMachineStatusTerminating:
		return "Stopping"
	case kubevirtv1.VirtualMachineStatusCrashLoopBackOff,
		kubevirtv1.VirtualMachineStatusUnknown,
		kubevirtv1.VirtualMachineStatusUnschedulable,
		kubevirtv1.VirtualMachineStatusErrImagePull,
		kubevirtv1.VirtualMachineStatusImagePullBackOff,
		kubevirtv1.VirtualMachineStatusPvcNotFound,
		kubevirtv1.VirtualMachineStatusDataVolumeError:
		return "Error"
	}
	if vm.Spec.RunStrategy != nil {
		switch *vm.Spec.RunStrategy {
		case kubevirtv1.RunStrategyHalted, kubevirtv1.RunStrategyManual:
			return "Stopped"
		case kubevirtv1.RunStrategyAlways, kubevirtv1.RunStrategyRerunOnFailure:
			if vm.Status.Ready {
				return "Running"
			}
			return "Starting"
		}
	}
	return "Pending"
}

func instancePhaseFromVMI(vmi *kubevirtv1.VirtualMachineInstance, vm *kubevirtv1.VirtualMachine) string {
	for _, cond := range vmi.Status.Conditions {
		if cond.Type == kubevirtv1.VirtualMachineInstanceConditionType(corev1.PodScheduled) && cond.Status == corev1.ConditionFalse {
			if cond.Reason == "Unschedulable" {
				return "Error"
			}
		}
	}
	switch vmi.Status.Phase {
	case kubevirtv1.Pending, kubevirtv1.Scheduling, kubevirtv1.Scheduled:
		return "Starting"
	case kubevirtv1.Running:
		return "Running"
	case kubevirtv1.Succeeded:
		return "Stopped"
	case kubevirtv1.Failed:
		return "Error"
	case kubevirtv1.Unknown:
		return instancePhaseFromVM(vm)
	}
	return instancePhaseFromVM(vm)
}

func preferGuestIP(ifaces []kubevirtv1.VirtualMachineInstanceNetworkInterface) string {
	for _, iface := range ifaces {
		if iface.Name == "public" && iface.IP != "" {
			return iface.IP
		}
	}
	for _, iface := range ifaces {
		if strings.HasPrefix(iface.IP, "10.0.50.") {
			return iface.IP
		}
	}
	for _, iface := range ifaces {
		if iface.IP != "" && iface.Name != "default" {
			return iface.IP
		}
	}
	for _, iface := range ifaces {
		if iface.IP != "" {
			return iface.IP
		}
	}
	return ""
}

func vmErrorMessage(vm *kubevirtv1.VirtualMachine) string {
	if vm.Status.PrintableStatus == kubevirtv1.VirtualMachineStatusCrashLoopBackOff {
		return "VM in crash loop"
	}
	if vm.Status.PrintableStatus == kubevirtv1.VirtualMachineStatusUnschedulable {
		return "VM unschedulable"
	}
	return ""
}
