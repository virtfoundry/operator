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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

const tenantFinalizer = "virtfoundry.io/finalizer"

// TenantReconciler reconciles a Tenant object.
type TenantReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=virtfoundry.io,resources=tenants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=virtfoundry.io,resources=tenants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=virtfoundry.io,resources=tenants/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete

// Reconcile ensures Namespace virtfoundry-tenant-{slug} exists for the Tenant.
func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	tenant := &virtfoundryv1alpha1.Tenant{}
	if err := r.Get(ctx, req.NamespacedName, tenant); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	nsName := fmt.Sprintf("virtfoundry-tenant-%s", tenant.Spec.Slug)

	if !tenant.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(tenant, tenantFinalizer) {
			ns := &corev1.Namespace{}
			err := r.Get(ctx, client.ObjectKey{Name: nsName}, ns)
			if err == nil {
				if err := r.Delete(ctx, ns); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{Requeue: true}, nil
			}
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(tenant, tenantFinalizer)
			if err := r.Update(ctx, tenant); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(tenant, tenantFinalizer) {
		controllerutil.AddFinalizer(tenant, tenantFinalizer)
		if err := r.Update(ctx, tenant); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, ns, func() error {
		if ns.Labels == nil {
			ns.Labels = map[string]string{}
		}
		ns.Labels["app.kubernetes.io/part-of"] = "virtfoundry"
		ns.Labels["virtfoundry.io/tenant"] = tenant.Spec.Slug
		return nil
	})
	if err != nil {
		logger.Error(err, "failed to ensure namespace")
		tenant.Status.Phase = "Failed"
		_ = r.Status().Update(ctx, tenant)
		return ctrl.Result{}, err
	}

	tenant.Status.Phase = "Ready"
	tenant.Status.Namespace = nsName
	if err := r.Status().Update(ctx, tenant); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&virtfoundryv1alpha1.Tenant{}).
		Named("tenant").
		Complete(r)
}
