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

// InstanceNicSpec defines a network interface on an Instance.
type InstanceNicSpec struct {
	// Name interface identifier within the instance.
	Name string `json:"name"`

	// NetworkRef names the Network CR to attach.
	NetworkRef LocalObjectRef `json:"networkRef"`
}

// InstanceSpec is the desired state of an Instance.
type InstanceSpec struct {
	// DisplayName shown in UI/REST.
	DisplayName string `json:"displayName"`

	// OfferingRef names the ServiceOffering CR.
	// +optional
	OfferingRef *LocalObjectRef `json:"offeringRef,omitempty"`

	// TemplateRef names the Template CR.
	// +optional
	TemplateRef *LocalObjectRef `json:"templateRef,omitempty"`

	// Nics network interfaces to attach.
	// +optional
	Nics []InstanceNicSpec `json:"nics,omitempty"`

	// SSHKeyRefs names SSHKey CRs injected at boot.
	// +optional
	SSHKeyRefs []LocalObjectRef `json:"sshKeyRefs,omitempty"`

	// DedicatedCPU pins vCPU threads to host cores.
	// +optional
	DedicatedCPU bool `json:"dedicatedCPU,omitempty"`

	// Import optional CloudStack/other import identity.
	// +optional
	Import *ImportMeta `json:"import,omitempty"`
}

// InstanceStatus is the observed state of an Instance.
type InstanceStatus struct {
	// Phase is Pending|Ready|Failed|Terminating.
	// +optional
	Phase string `json:"phase,omitempty"`

	// KubeVirtName is the reconciled VirtualMachine name.
	// +optional
	KubeVirtName string `json:"kubevirtName,omitempty"`

	// IP primary guest IP address.
	// +optional
	IP string `json:"ip,omitempty"`

	// ErrorMessage last reconcile error message.
	// +optional
	ErrorMessage string `json:"errorMessage,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=vf-instance
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="IP",type=string,JSONPath=`.status.ip`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Instance is the Schema for the instances API.
type Instance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceSpec   `json:"spec,omitempty"`
	Status InstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InstanceList contains a list of Instance.
type InstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Instance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Instance{}, &InstanceList{})
}
