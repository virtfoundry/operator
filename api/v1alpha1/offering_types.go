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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OfferingSpec is the desired state of an Offering.
type OfferingSpec struct {
	// DisplayName shown in UI/REST.
	DisplayName string `json:"displayName"`

	// CPU cores allocated to instances using this offering.
	CPU int `json:"cpu"`

	// MemoryMi is memory in mebibytes.
	MemoryMi int64 `json:"memoryMi"`

	// DedicatedCPU pins vCPU threads to host cores.
	// +optional
	DedicatedCPU bool `json:"dedicatedCPU,omitempty"`

	// StorageTags comma-separated storage tag filter.
	// +optional
	StorageTags string `json:"storageTags,omitempty"`

	// Import optional CloudStack/other import identity.
	// +optional
	Import *ImportMeta `json:"import,omitempty"`
}

// OfferingStatus is the observed state of an Offering.
type OfferingStatus struct {
	// Phase is Pending|Ready|Failed|Terminating.
	// +optional
	Phase string `json:"phase,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=vf-offering

// Offering is the Schema for the offerings API.
type Offering struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OfferingSpec   `json:"spec,omitempty"`
	Status OfferingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OfferingList contains a list of Offering.
type OfferingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Offering `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Offering{}, &OfferingList{})
}
