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

// TemplateSpec is the desired state of a Template.
type TemplateSpec struct {
	// DisplayName shown in UI/REST.
	DisplayName string `json:"displayName"`

	// Image source URL or registry reference.
	Image string `json:"image"`

	// SourceType describes how the image is imported (e.g. iso, qcow2).
	SourceType string `json:"sourceType"`

	// OSType operating system identifier.
	// +optional
	OSType string `json:"osType,omitempty"`

	// CloudInitUserData cloud-init user-data payload.
	// +optional
	CloudInitUserData string `json:"cloudInitUserData,omitempty"`

	// ISOSizeGi expected ISO size in gibibytes.
	// +optional
	ISOSizeGi int `json:"isoSizeGi,omitempty"`

	// BootDiskSizeGi boot disk size in gibibytes.
	// +optional
	BootDiskSizeGi int `json:"bootDiskSizeGi,omitempty"`

	// StorageClass for template volumes.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`

	// Import optional CloudStack/other import identity.
	// +optional
	Import *ImportMeta `json:"import,omitempty"`
}

// TemplateStatus is the observed state of a Template.
type TemplateStatus struct {
	// Phase is Pending|Ready|Failed|Terminating.
	// +optional
	Phase string `json:"phase,omitempty"`

	// ImportState tracks CDI/PVC import progress.
	// +optional
	ImportState string `json:"importState,omitempty"`

	// ISOVolumeName is the PVC holding the ISO image.
	// +optional
	ISOVolumeName string `json:"isoVolumeName,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=vf-template

// Template is the Schema for the templates API.
type Template struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TemplateSpec   `json:"spec,omitempty"`
	Status TemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TemplateList contains a list of Template.
type TemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Template `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Template{}, &TemplateList{})
}
