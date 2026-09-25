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

package controller

import (
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

const (
	// tenantNamespacePrefix is the only namespace name prefix this operator may write to.
	tenantNamespacePrefix = "virtfoundry-tenant-"

	labelPartOf    = "app.kubernetes.io/part-of"
	labelManagedBy = "app.kubernetes.io/managed-by"
	labelTenant    = "virtfoundry.io/tenant"

	partOfVirtFoundry = "virtfoundry"
	managedByOperator = "virtfoundry-operator"

	// operatorNamespace holds the operator itself and never belongs to a Tenant.
	operatorNamespace = "virtfoundry-system"
)

// protectedNamespaces are never mutated or deleted by this operator. Namespace
// names are derived from a Tenant slug, so a bug in that derivation is the only
// way one of these could be reached — this map makes that failure mode safe.
var protectedNamespaces = map[string]struct{}{
	"default":         {},
	"kube-node-lease": {},
	"kube-public":     {},
	"kube-system":     {},
	operatorNamespace: {},
}

// errNamespaceNotOwned is returned whenever the operator declines to write to a
// namespace because it cannot prove the namespace belongs to the Tenant.
var errNamespaceNotOwned = errors.New("namespace is not owned by this Tenant")

// errSlugConflict is returned when another live Tenant already claims this slug.
// Two Tenants sharing a slug would share (and on delete, wipe) one namespace.
var errSlugConflict = errors.New("tenant slug is already in use")

// tenantSlugIndexKey is the field indexer key for Tenant.spec.slug.
const tenantSlugIndexKey = "spec.slug"

func tenantNamespaceName(slug string) string {
	return tenantNamespacePrefix + slug
}

func tenantNamespaceLabels(tenant *virtfoundryv1alpha1.Tenant) map[string]string {
	labels := map[string]string{
		labelPartOf:    partOfVirtFoundry,
		labelManagedBy: managedByOperator,
		labelTenant:    tenant.Spec.Slug,
	}
	for k, v := range tenantPSALabels() {
		labels[k] = v
	}
	return labels
}

// validateTenantNamespaceName rejects any name outside the tenant namespace
// space. It runs before create as well, where there is no object to inspect yet.
func validateTenantNamespaceName(name string) error {
	if _, protected := protectedNamespaces[name]; protected {
		return fmt.Errorf("%w: %q is a protected namespace", errNamespaceNotOwned, name)
	}
	if !strings.HasPrefix(name, tenantNamespacePrefix) {
		return fmt.Errorf("%w: %q does not start with %q", errNamespaceNotOwned, name, tenantNamespacePrefix)
	}
	if name == tenantNamespacePrefix {
		return fmt.Errorf("%w: %q has an empty tenant slug", errNamespaceNotOwned, name)
	}
	return nil
}

// assertTenantNamespaceOwned is the single guard in front of every mutating call
// the controller makes against an existing Namespace. It must pass before the
// operator patches or deletes anything.
func assertTenantNamespaceOwned(ns *corev1.Namespace, tenant *virtfoundryv1alpha1.Tenant) error {
	want := tenantNamespaceName(tenant.Spec.Slug)
	if err := validateTenantNamespaceName(want); err != nil {
		return err
	}
	if err := validateTenantNamespaceName(ns.Name); err != nil {
		return err
	}
	if ns.Name != want {
		return fmt.Errorf("%w: %q is not the namespace of Tenant %q (%q)",
			errNamespaceNotOwned, ns.Name, tenant.Name, want)
	}
	if ns.Labels[labelPartOf] != partOfVirtFoundry {
		return fmt.Errorf("%w: %q is missing label %s=%s",
			errNamespaceNotOwned, ns.Name, labelPartOf, partOfVirtFoundry)
	}
	if ns.Labels[labelTenant] != tenant.Spec.Slug {
		return fmt.Errorf("%w: %q carries label %s=%q, want %q",
			errNamespaceNotOwned, ns.Name, labelTenant, ns.Labels[labelTenant], tenant.Spec.Slug)
	}
	// Two Tenants must not share a slug (assertSlugUnique), but a race or a
	// legacy object can still leave a foreign ownerRef. Only the controlling
	// Tenant may manage the namespace; namespaces created before ownerRefs were
	// stamped have no controller ref and may be adopted once by the slug owner.
	if owner := metav1.GetControllerOf(ns); owner != nil && owner.UID != tenant.UID {
		return fmt.Errorf("%w: %q is controlled by %s %q",
			errNamespaceNotOwned, ns.Name, owner.Kind, owner.Name)
	}
	return nil
}

// assertTenantNamespaceDeletable is stricter than assertTenantNamespaceOwned:
// delete requires both the tenant label and a matching controller ownerRef.
// Label-only legacy namespaces are left alone rather than wiped.
func assertTenantNamespaceDeletable(ns *corev1.Namespace, tenant *virtfoundryv1alpha1.Tenant) error {
	if err := assertTenantNamespaceOwned(ns, tenant); err != nil {
		return err
	}
	owner := metav1.GetControllerOf(ns)
	if owner == nil {
		return fmt.Errorf("%w: %q has no controller ownerRef; refusing delete",
			errNamespaceNotOwned, ns.Name)
	}
	if owner.UID != tenant.UID {
		return fmt.Errorf("%w: %q is controlled by %s %q",
			errNamespaceNotOwned, ns.Name, owner.Kind, owner.Name)
	}
	return nil
}
