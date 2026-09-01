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

// InstanceSnapshotSpec is the desired state of an InstanceSnapshot.
type InstanceSnapshotSpec struct {
	// Name display label shown in UI/REST.
	Name string `json:"name"`

	// InstanceRef names the source Instance CR.
	InstanceRef LocalObjectRef `json:"instanceRef"`
}

// InstanceSnapshotStatus is the observed state of an InstanceSnapshot.
type InstanceSnapshotStatus struct {
	// Phase is Pending|Ready|Failed|Terminating.
	// +optional
	Phase string `json:"phase,omitempty"`

	// KubeVirtSnapshotName is the reconciled VirtualMachineSnapshot name.
	// +optional
	KubeVirtSnapshotName string `json:"kubevirtSnapshotName,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=vf-isnap

// InstanceSnapshot is the Schema for the instancesnapshots API.
type InstanceSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceSnapshotSpec   `json:"spec,omitempty"`
	Status InstanceSnapshotStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InstanceSnapshotList contains a list of InstanceSnapshot.
type InstanceSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InstanceSnapshot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InstanceSnapshot{}, &InstanceSnapshotList{})
}
