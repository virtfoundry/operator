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

// SecurityGroupRuleSpec defines an ingress or egress rule.
type SecurityGroupRuleSpec struct {
	// Direction is ingress or egress.
	Direction string `json:"direction"`

	// Protocol is tcp, udp, or icmp.
	Protocol string `json:"protocol"`

	// PortFrom start of port range (inclusive).
	// +optional
	PortFrom int `json:"portFrom,omitempty"`

	// PortTo end of port range (inclusive).
	// +optional
	PortTo int `json:"portTo,omitempty"`

	// CIDR source or destination CIDR block.
	CIDR string `json:"cidr"`
}

// SecurityGroupSpec is the desired state of a SecurityGroup.
type SecurityGroupSpec struct {
	// Name display label shown in UI/REST.
	Name string `json:"name"`

	// Description human-readable summary.
	// +optional
	Description string `json:"description,omitempty"`

	// VPCRef names the parent VPC CR.
	// +optional
	VPCRef *LocalObjectRef `json:"vpcRef,omitempty"`

	// Rules firewall rules applied to attached instances.
	// +optional
	Rules []SecurityGroupRuleSpec `json:"rules,omitempty"`
}

// SecurityGroupStatus is the observed state of a SecurityGroup.
type SecurityGroupStatus struct {
	// Phase is Pending|Ready|Failed|Terminating.
	// +optional
	Phase string `json:"phase,omitempty"`

	// NetworkPolicyName is the reconciled NetworkPolicy name.
	// +optional
	NetworkPolicyName string `json:"networkPolicyName,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=vf-sg

// SecurityGroup is the Schema for the securitygroups API.
type SecurityGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecurityGroupSpec   `json:"spec,omitempty"`
	Status SecurityGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SecurityGroupList contains a list of SecurityGroup.
type SecurityGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecurityGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecurityGroup{}, &SecurityGroupList{})
}
