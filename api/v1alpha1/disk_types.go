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

// DiskSpec is the desired state of a Disk.
type DiskSpec struct {
	// Name display label shown in UI/REST.
	Name string `json:"name"`

	// SizeGi disk size in gibibytes.
	SizeGi int `json:"sizeGi"`

	// StorageClass for the backing PVC.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`

	// InstanceRef names the attached Instance CR.
	// +optional
	InstanceRef *LocalObjectRef `json:"instanceRef,omitempty"`
}

// DiskStatus is the observed state of a Disk.
type DiskStatus struct {
	// Phase is Pending|Ready|Failed|Terminating.
	// +optional
	Phase string `json:"phase,omitempty"`

	// PVCName is the reconciled PersistentVolumeClaim name.
	// +optional
	PVCName string `json:"pvcName,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=vf-disk

// Disk is the Schema for the disks API.
type Disk struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DiskSpec   `json:"spec,omitempty"`
	Status DiskStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DiskList contains a list of Disk.
type DiskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Disk `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Disk{}, &DiskList{})
}
