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

// APIKeySpec is the desired state of an APIKey.
type APIKeySpec struct {
	// UserRef names the owning User CR.
	UserRef LocalObjectRef `json:"userRef"`

	// Name display label for the key.
	Name string `json:"name"`

	// Prefix first characters of the key shown in UI.
	Prefix string `json:"prefix"`

	// Scopes granted to this key.
	// +optional
	Scopes []string `json:"scopes,omitempty"`

	// ExpiresAt optional expiration timestamp.
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`

	// SecretRef points at the Secret holding secret_hash.
	SecretRef SecretKeyRef `json:"secretRef"`
}

// APIKeyStatus is the observed state of an APIKey.
type APIKeyStatus struct {
	// Phase is Pending|Ready|Failed|Terminating.
	// +optional
	Phase string `json:"phase,omitempty"`

	// LastUsedAt last successful authentication time.
	// +optional
	LastUsedAt *metav1.Time `json:"lastUsedAt,omitempty"`

	// RevokedAt revocation timestamp when the key is disabled.
	// +optional
	RevokedAt *metav1.Time `json:"revokedAt,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=vf-apikey

// APIKey is the Schema for the apikeys API.
type APIKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   APIKeySpec   `json:"spec,omitempty"`
	Status APIKeyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// APIKeyList contains a list of APIKey.
type APIKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []APIKey `json:"items"`
}

func init() {
	SchemeBuilder.Register(&APIKey{}, &APIKeyList{})
}
