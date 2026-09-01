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

// UserSpec is the desired state of a User.
type UserSpec struct {
	// Username login identifier.
	Username string `json:"username"`

	// Email contact address.
	// +optional
	Email string `json:"email,omitempty"`

	// RoleRef names the Role CR granting permissions.
	RoleRef LocalObjectRef `json:"roleRef"`

	// TenantRef scopes the user to a tenant; nil for platform users.
	// +optional
	TenantRef *LocalObjectRef `json:"tenantRef,omitempty"`

	// State desired account state (e.g. enabled, disabled).
	// +optional
	State string `json:"state,omitempty"`

	// SecretRef points at the Secret holding password_hash.
	SecretRef SecretKeyRef `json:"secretRef"`
}

// UserStatus is the observed state of a User.
type UserStatus struct {
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
// +kubebuilder:resource:scope=Cluster,shortName=vf-user

// User is the Schema for the users API.
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec,omitempty"`
	Status UserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UserList contains a list of User.
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}
