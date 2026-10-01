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
	"fmt"
	"os"
	"strings"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

// Offering / guest resource bounds — keep in sync with:
// - api/v1alpha1 OfferingSpec kubebuilder Minimum/Maximum markers
// - charts/.../templates/cr-admission.yaml ValidatingAdmissionPolicy
const (
	offeringCPUMin      = 1
	offeringCPUMax      = 256
	offeringMemoryMiMin = 64
	offeringMemoryMiMax = 1048576 // 1 TiB

	// envAllowDedicatedCPU opts into KubeVirt DedicatedCPUPlacement when set to
	// a truthy value (chart crAdmission.allowDedicatedCPU). Default deny —
	// privileged host CPU pinning is gated at VAP + reconciler (issue #26).
	envAllowDedicatedCPU = "VIRTFOUNDRY_ALLOW_DEDICATED_CPU"
)

// assertInstanceTenantNamespace rejects Instances outside virtfoundry-tenant-*.
// Admission (VAP) is the primary gate; this is defense in depth when VAP is off.
func assertInstanceTenantNamespace(namespace string) error {
	if !strings.HasPrefix(namespace, tenantNamespacePrefix) {
		return fmt.Errorf("instance must live in a %s* namespace, got %q", tenantNamespacePrefix, namespace)
	}
	if namespace == tenantNamespacePrefix {
		return fmt.Errorf("instance namespace %q has an empty tenant slug", namespace)
	}
	return nil
}

// validateOfferingSpec rejects out-of-bounds CPU/memory before they reach
// KubeVirt (negative CPU cast to uint32, MustParse panics, etc.).
func validateOfferingSpec(cpu int, memoryMi int64) error {
	if cpu < offeringCPUMin || cpu > offeringCPUMax {
		return fmt.Errorf("offering cpu %d out of range [%d, %d]", cpu, offeringCPUMin, offeringCPUMax)
	}
	if memoryMi < offeringMemoryMiMin || memoryMi > offeringMemoryMiMax {
		return fmt.Errorf("offering memoryMi %d out of range [%d, %d]",
			memoryMi, offeringMemoryMiMin, offeringMemoryMiMax)
	}
	return nil
}

func validateGuestResources(cpu int, memoryMi int64) error {
	if cpu < offeringCPUMin || cpu > offeringCPUMax {
		return fmt.Errorf("guest cpu %d out of range [%d, %d]", cpu, offeringCPUMin, offeringCPUMax)
	}
	if memoryMi < offeringMemoryMiMin || memoryMi > offeringMemoryMiMax {
		return fmt.Errorf("guest memoryMi %d out of range [%d, %d]",
			memoryMi, offeringMemoryMiMin, offeringMemoryMiMax)
	}
	return nil
}

// validateResolvedOffering applies bounds when an Offering CR was loaded.
func validateResolvedOffering(off *virtfoundryv1alpha1.Offering) error {
	return validateOfferingSpec(off.Spec.CPU, off.Spec.MemoryMi)
}

func dedicatedCPUAllowed() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(envAllowDedicatedCPU)))
	return v == "1" || v == "true" || v == "yes"
}

// validateDedicatedCPU refuses host CPU pinning unless explicitly enabled
// (chart crAdmission.allowDedicatedCPU / VIRTFOUNDRY_ALLOW_DEDICATED_CPU).
// Admission (VAP) is the primary gate; this is defense in depth when VAP is off.
func validateDedicatedCPU(requested bool) error {
	if !requested {
		return nil
	}
	if dedicatedCPUAllowed() {
		return nil
	}
	return fmt.Errorf("dedicatedCPU is disabled (set chart crAdmission.allowDedicatedCPU / %s=true to enable host CPU pinning)",
		envAllowDedicatedCPU)
}
