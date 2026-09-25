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
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

const (
	tenantFinalizer = "virtfoundry.io/finalizer"

	// namespaceDeletionPoll is how often deletion of a tenant namespace is polled
	// while the API server drains its contents.
	namespaceDeletionPoll = 5 * time.Second
)

// TenantReconciler reconciles a Tenant object.
type TenantReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Namespace names are derived from a Tenant slug, so RBAC cannot scope these
// verbs any further: resourceNames does not support prefixes and namespaces are
// cluster-scoped. Ownership is therefore enforced by assertTenantNamespaceOwned
// below, and by the optional ValidatingAdmissionPolicy shipped with the chart.
//
// +kubebuilder:rbac:groups=virtfoundry.io,resources=tenants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=virtfoundry.io,resources=tenants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=virtfoundry.io,resources=tenants/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;patch;delete

// Reconcile ensures Namespace virtfoundry-tenant-{slug} exists for the Tenant.
func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	tenant := &virtfoundryv1alpha1.Tenant{}
	if err := r.Get(ctx, req.NamespacedName, tenant); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !tenant.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, tenant)
	}

	if !controllerutil.ContainsFinalizer(tenant, tenantFinalizer) {
		controllerutil.AddFinalizer(tenant, tenantFinalizer)
		if err := r.Update(ctx, tenant); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return r.reconcileNamespace(ctx, tenant)
}

// reconcileNamespace creates the tenant namespace, or adopts an existing one
// only when it can be proven to belong to this Tenant.
func (r *TenantReconciler) reconcileNamespace(
	ctx context.Context,
	tenant *virtfoundryv1alpha1.Tenant,
) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	nsName := tenantNamespaceName(tenant.Spec.Slug)

	if err := validateTenantNamespaceName(nsName); err != nil {
		logger.Error(err, "Refused to manage Namespace for Tenant", "namespace", nsName)
		return r.markFailed(ctx, tenant, err)
	}

	if err := r.assertSlugUnique(ctx, tenant); err != nil {
		logger.Error(err, "Refused Tenant with colliding slug", "slug", tenant.Spec.Slug, "tenant", tenant.Name)
		return r.markFailed(ctx, tenant, err)
	}

	ns := &corev1.Namespace{}
	err := r.Get(ctx, client.ObjectKey{Name: nsName}, ns)
	switch {
	case apierrors.IsNotFound(err):
		if err := r.createNamespace(ctx, tenant, nsName); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				logger.Error(err, "Failed to create Namespace", "namespace", nsName)
				return r.markFailed(ctx, tenant, err)
			}
			// Concurrent create: re-fetch and require ownership before adopting.
			if getErr := r.Get(ctx, client.ObjectKey{Name: nsName}, ns); getErr != nil {
				return ctrl.Result{}, getErr
			}
			if ownErr := assertTenantNamespaceOwned(ns, tenant); ownErr != nil {
				logger.Error(ownErr, "Refused to adopt Namespace for Tenant", "namespace", nsName, "tenant", tenant.Name)
				return r.markFailed(ctx, tenant, ownErr)
			}
			if err := r.stampNamespace(ctx, tenant, ns); err != nil {
				logger.Error(err, "Failed to update Namespace", "namespace", nsName)
				return ctrl.Result{}, err
			}
		} else {
			logger.Info("Created Namespace for Tenant", "namespace", nsName, "tenant", tenant.Name)
		}
	case err != nil:
		return ctrl.Result{}, err
	default:
		if err := assertTenantNamespaceOwned(ns, tenant); err != nil {
			// Adopting a foreign namespace would later let this Tenant delete it.
			logger.Error(err, "Refused to adopt Namespace for Tenant", "namespace", nsName, "tenant", tenant.Name)
			return r.markFailed(ctx, tenant, err)
		}
		if err := r.stampNamespace(ctx, tenant, ns); err != nil {
			logger.Error(err, "Failed to update Namespace", "namespace", nsName)
			return ctrl.Result{}, err
		}
	}

	tenant.Status.Phase = "Ready"
	tenant.Status.Namespace = nsName
	if err := r.Status().Update(ctx, tenant); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *TenantReconciler) createNamespace(
	ctx context.Context,
	tenant *virtfoundryv1alpha1.Tenant,
	nsName string,
) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   nsName,
			Labels: tenantNamespaceLabels(tenant),
		},
	}
	if err := controllerutil.SetControllerReference(tenant, ns, r.Scheme); err != nil {
		return err
	}
	return r.Create(ctx, ns)
}

// stampNamespace keeps labels and the ownerRef current on an owned namespace.
func (r *TenantReconciler) stampNamespace(
	ctx context.Context,
	tenant *virtfoundryv1alpha1.Tenant,
	ns *corev1.Namespace,
) error {
	patch := client.MergeFrom(ns.DeepCopy())
	if ns.Labels == nil {
		ns.Labels = map[string]string{}
	}
	maps.Copy(ns.Labels, tenantNamespaceLabels(tenant))
	if err := controllerutil.SetControllerReference(tenant, ns, r.Scheme); err != nil {
		return err
	}
	return r.Patch(ctx, ns, patch)
}

// assertSlugUnique rejects a Tenant whose slug is already claimed by another
// live Tenant. Uses the field index when registered; falls back to a full list
// so unit tests without a manager still work.
func (r *TenantReconciler) assertSlugUnique(ctx context.Context, tenant *virtfoundryv1alpha1.Tenant) error {
	if tenant.Spec.Slug == "" {
		return fmt.Errorf("%w: empty slug", errSlugConflict)
	}

	list := &virtfoundryv1alpha1.TenantList{}
	err := r.List(ctx, list, client.MatchingFields{tenantSlugIndexKey: tenant.Spec.Slug})
	if err != nil {
		if listErr := r.List(ctx, list); listErr != nil {
			return listErr
		}
	}

	for i := range list.Items {
		other := &list.Items[i]
		if other.Name == tenant.Name {
			continue
		}
		if other.Spec.Slug != tenant.Spec.Slug {
			continue
		}
		if !other.DeletionTimestamp.IsZero() {
			continue
		}
		return fmt.Errorf("%w: slug %q is already used by Tenant %q",
			errSlugConflict, tenant.Spec.Slug, other.Name)
	}
	return nil
}

// reconcileDelete removes the tenant namespace, but only the one this operator
// created for this Tenant. Anything else is left untouched.
func (r *TenantReconciler) reconcileDelete(
	ctx context.Context,
	tenant *virtfoundryv1alpha1.Tenant,
) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	if !controllerutil.ContainsFinalizer(tenant, tenantFinalizer) {
		return ctrl.Result{}, nil
	}

	nsName := tenantNamespaceName(tenant.Spec.Slug)
	ns := &corev1.Namespace{}
	err := r.Get(ctx, client.ObjectKey{Name: nsName}, ns)
	switch {
	case apierrors.IsNotFound(err):
		// Nothing left to clean up.
	case err != nil:
		return ctrl.Result{}, err
	default:
		if guardErr := assertTenantNamespaceDeletable(ns, tenant); guardErr != nil {
			// Never delete a namespace we cannot prove we own. Drop the finalizer
			// so the Tenant is not wedged on a namespace that is not ours.
			logger.Error(guardErr, "Refused to delete Namespace for Tenant",
				"namespace", nsName, "tenant", tenant.Name)
			break
		}
		if ns.DeletionTimestamp.IsZero() {
			// The UID precondition makes the ownership check above non-racy: if the
			// namespace was recreated since the Get, the delete is rejected.
			err := r.Delete(ctx, ns, client.Preconditions{UID: &ns.UID})
			if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
				return ctrl.Result{}, err
			}
			logger.Info("Deleted Namespace for Tenant", "namespace", nsName, "tenant", tenant.Name)
		}
		return ctrl.Result{RequeueAfter: namespaceDeletionPoll}, nil
	}

	controllerutil.RemoveFinalizer(tenant, tenantFinalizer)
	if err := r.Update(ctx, tenant); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// markFailed records the failure on the Tenant status. An ownership or slug
// refusal is returned as a terminal error: retrying cannot change the outcome;
// an operator has to rename the slug or clean up the namespace.
func (r *TenantReconciler) markFailed(
	ctx context.Context,
	tenant *virtfoundryv1alpha1.Tenant,
	cause error,
) (ctrl.Result, error) {
	tenant.Status.Phase = "Failed"
	if err := r.Status().Update(ctx, tenant); err != nil {
		return ctrl.Result{}, err
	}
	if errors.Is(cause, errNamespaceNotOwned) || errors.Is(cause, errSlugConflict) {
		return ctrl.Result{}, reconcile.TerminalError(cause)
	}
	return ctrl.Result{}, cause
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&virtfoundryv1alpha1.Tenant{},
		tenantSlugIndexKey,
		func(obj client.Object) []string {
			tenant, ok := obj.(*virtfoundryv1alpha1.Tenant)
			if !ok || tenant.Spec.Slug == "" {
				return nil
			}
			return []string{tenant.Spec.Slug}
		},
	); err != nil {
		return fmt.Errorf("index Tenant by %s: %w", tenantSlugIndexKey, err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&virtfoundryv1alpha1.Tenant{}).
		Named("tenant").
		Complete(r)
}
