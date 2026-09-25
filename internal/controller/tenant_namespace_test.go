package controller

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

func tenantFixture(name, slug string, uid types.UID) *virtfoundryv1alpha1.Tenant {
	return &virtfoundryv1alpha1.Tenant{
		ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid},
		Spec:       virtfoundryv1alpha1.TenantSpec{Name: name, Slug: slug},
	}
}

func ownedNamespaceFixture(tenant *virtfoundryv1alpha1.Tenant) *corev1.Namespace {
	controller := true
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   tenantNamespaceName(tenant.Spec.Slug),
			Labels: tenantNamespaceLabels(tenant),
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: virtfoundryv1alpha1.GroupVersion.String(),
				Kind:       "Tenant",
				Name:       tenant.Name,
				UID:        tenant.UID,
				Controller: &controller,
			}},
		},
	}
}

func TestValidateTenantNamespaceNameRejectsProtectedNamespaces(t *testing.T) {
	for name := range protectedNamespaces {
		if err := validateTenantNamespaceName(name); err == nil {
			t.Fatalf("validateTenantNamespaceName(%q) = nil, want refusal", name)
		}
	}
}

func TestValidateTenantNamespaceNameRejectsForeignPrefix(t *testing.T) {
	for _, name := range []string{"argocd", "virtfoundry-system-extra", "tenant-acme", "virtfoundry-tenant-"} {
		if err := validateTenantNamespaceName(name); err == nil {
			t.Fatalf("validateTenantNamespaceName(%q) = nil, want refusal", name)
		}
	}
}

func TestValidateTenantNamespaceNameAcceptsTenantNamespace(t *testing.T) {
	if err := validateTenantNamespaceName(tenantNamespaceName("acme")); err != nil {
		t.Fatalf("validateTenantNamespaceName: %v", err)
	}
}

func TestAssertTenantNamespaceOwnedAcceptsOwnNamespace(t *testing.T) {
	tenant := tenantFixture("acme", "acme", "uid-acme")
	if err := assertTenantNamespaceOwned(ownedNamespaceFixture(tenant), tenant); err != nil {
		t.Fatalf("assertTenantNamespaceOwned: %v", err)
	}
}

func TestAssertTenantNamespaceOwnedAdoptsLegacyNamespace(t *testing.T) {
	// Namespaces created before ownerRefs were stamped carry only the labels.
	tenant := tenantFixture("acme", "acme", "uid-acme")
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: tenantNamespaceName("acme"),
			Labels: map[string]string{
				labelPartOf: partOfVirtFoundry,
				labelTenant: "acme",
			},
		},
	}
	if err := assertTenantNamespaceOwned(ns, tenant); err != nil {
		t.Fatalf("assertTenantNamespaceOwned: %v", err)
	}
}

func TestAssertTenantNamespaceOwnedRefusesProtectedNamespace(t *testing.T) {
	tenant := tenantFixture("acme", "acme", "uid-acme")
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "kube-system",
			Labels: tenantNamespaceLabels(tenant),
		},
	}
	err := assertTenantNamespaceOwned(ns, tenant)
	if !errors.Is(err, errNamespaceNotOwned) {
		t.Fatalf("assertTenantNamespaceOwned(kube-system) = %v, want errNamespaceNotOwned", err)
	}
}

func TestAssertTenantNamespaceOwnedRefusesUnlabelledNamespace(t *testing.T) {
	tenant := tenantFixture("acme", "acme", "uid-acme")
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: tenantNamespaceName("acme")},
	}
	err := assertTenantNamespaceOwned(ns, tenant)
	if !errors.Is(err, errNamespaceNotOwned) {
		t.Fatalf("assertTenantNamespaceOwned(unlabelled) = %v, want errNamespaceNotOwned", err)
	}
}

func TestAssertTenantNamespaceOwnedRefusesForeignTenantLabel(t *testing.T) {
	tenant := tenantFixture("acme", "acme", "uid-acme")
	ns := ownedNamespaceFixture(tenant)
	ns.Labels[labelTenant] = "globex"
	err := assertTenantNamespaceOwned(ns, tenant)
	if !errors.Is(err, errNamespaceNotOwned) {
		t.Fatalf("assertTenantNamespaceOwned(foreign label) = %v, want errNamespaceNotOwned", err)
	}
}

func TestAssertTenantNamespaceOwnedRefusesNamespaceOfAnotherTenant(t *testing.T) {
	// Two Tenants may declare the same slug; only the owner may delete it.
	owner := tenantFixture("acme", "acme", "uid-acme")
	squatter := tenantFixture("acme-copy", "acme", "uid-acme-copy")
	err := assertTenantNamespaceOwned(ownedNamespaceFixture(owner), squatter)
	if !errors.Is(err, errNamespaceNotOwned) {
		t.Fatalf("assertTenantNamespaceOwned(other tenant) = %v, want errNamespaceNotOwned", err)
	}
}

func TestAssertTenantNamespaceOwnedRefusesNamespaceOfAnotherSlug(t *testing.T) {
	globex := tenantFixture("globex", "globex", "uid-globex")
	acmeNamespace := ownedNamespaceFixture(tenantFixture("acme", "acme", "uid-acme"))
	err := assertTenantNamespaceOwned(acmeNamespace, globex)
	if !errors.Is(err, errNamespaceNotOwned) {
		t.Fatalf("assertTenantNamespaceOwned(other slug) = %v, want errNamespaceNotOwned", err)
	}
}
