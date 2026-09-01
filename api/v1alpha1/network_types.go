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

// NetworkSpec is the desired state of a Network.
type NetworkSpec struct {
	// Name display label shown in UI/REST.
	Name string `json:"name"`

	// NetworkType is isolated or shared.
	// +kubebuilder:validation:Enum=isolated;shared
	NetworkType string `json:"networkType"`

	// CIDR IPv4 subnet range.
	CIDR string `json:"cidr"`

	// Gateway default gateway address.
	// +optional
	Gateway string `json:"gateway,omitempty"`

	// VPCRef names the parent VPC CR.
	// +optional
	VPCRef *LocalObjectRef `json:"vpcRef,omitempty"`

	// Import optional CloudStack/other import identity.
	// +optional
	Import *ImportMeta `json:"import,omitempty"`
}

// NetworkStatus is the observed state of a Network.
type NetworkStatus struct {
	// Phase is Pending|Ready|Failed|Terminating.
	// +optional
	Phase string `json:"phase,omitempty"`

	// NADNamespace is the Multus NAD namespace.
	// +optional
	NADNamespace string `json:"nadNamespace,omitempty"`

	// NADName is the Multus NetworkAttachmentDefinition name.
	// +optional
	NADName string `json:"nadName,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=vf-network

// Network is the Schema for the networks API.
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec,omitempty"`
	Status NetworkStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkList contains a list of Network.
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Network `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Network{}, &NetworkList{})
}
