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

// TargetGroupTarget is one backend registered in a TargetGroup.
type TargetGroupTarget struct {
	// ID is the stable REST identifier of the target.
	ID string `json:"id"`

	// VMID is the REST identifier of the backing VM.
	// +optional
	VMID string `json:"vmID,omitempty"`

	// IP is the backend address the load balancer forwards to.
	// +kubebuilder:validation:MinLength=1
	IP string `json:"ip"`
}

// TargetGroupSpec is the desired state of a TargetGroup.
type TargetGroupSpec struct {
	// Name display label shown in UI/REST.
	Name string `json:"name"`

	// Protocol is tcp or udp.
	// +kubebuilder:validation:Enum=tcp;udp
	// +kubebuilder:default=tcp
	Protocol string `json:"protocol,omitempty"`

	// Port is the backend port every target listens on.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// Targets are the registered backends.
	// +optional
	Targets []TargetGroupTarget `json:"targets,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=vf-tg

// TargetGroup is the Schema for the targetgroups API.
type TargetGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec TargetGroupSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// TargetGroupList contains a list of TargetGroup.
type TargetGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TargetGroup `json:"items"`
}

// LoadBalancerListener forwards a front port to a TargetGroup.
type LoadBalancerListener struct {
	// ID is the stable REST identifier of the listener.
	ID string `json:"id"`

	// Protocol is tcp or udp.
	// +kubebuilder:validation:Enum=tcp;udp
	// +kubebuilder:default=tcp
	Protocol string `json:"protocol,omitempty"`

	// Port is the front port exposed on the VIP.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// TargetGroupRef names the TargetGroup CR in the same namespace.
	TargetGroupRef LocalObjectRef `json:"targetGroupRef"`
}

// LoadBalancerSpec is the desired state of a LoadBalancer.
type LoadBalancerSpec struct {
	// Name display label shown in UI/REST.
	Name string `json:"name"`

	// Description human-readable summary.
	// +optional
	Description string `json:"description,omitempty"`

	// Listeners map front ports to target groups.
	// +optional
	Listeners []LoadBalancerListener `json:"listeners,omitempty"`
}

// LoadBalancerStatus is the observed state of a LoadBalancer.
type LoadBalancerStatus struct {
	// Phase is Pending|Ready|Failed.
	// +optional
	Phase string `json:"phase,omitempty"`

	// ServiceName is the LoadBalancer Service the operator reconciles.
	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	// VIP is the address (or hostname) assigned to the Service.
	// +optional
	VIP string `json:"vip,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=vf-lb
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="VIP",type=string,JSONPath=`.status.vip`

// LoadBalancer is the Schema for the loadbalancers API.
type LoadBalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoadBalancerSpec   `json:"spec,omitempty"`
	Status LoadBalancerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LoadBalancerList contains a list of LoadBalancer.
type LoadBalancerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoadBalancer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TargetGroup{}, &TargetGroupList{}, &LoadBalancer{}, &LoadBalancerList{})
}
