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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

var nadGVK = schema.GroupVersionKind{
	Group:   "k8s.cni.cncf.io",
	Version: "v1",
	Kind:    nadKind,
}

// NetworkReconciler ensures isolated Network CRs own a Multus NAD and status refs.
type NetworkReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=virtfoundry.io,resources=networks,verbs=get;list;watch
// +kubebuilder:rbac:groups=virtfoundry.io,resources=networks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch;create;update;patch

// Reconcile creates/updates a Multus NAD for isolated Networks and writes status.
func (r *NetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	net := &virtfoundryv1alpha1.Network{}
	if err := r.Get(ctx, req.NamespacedName, net); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if net.Spec.NetworkType != networkTypeIsolated {
		logger.Info("Skipping Network reconcile (not isolated)", "networkType", net.Spec.NetworkType)
		return ctrl.Result{}, nil
	}

	cfg, err := buildIsolatedNADConfig(net.Name, defaultIsolatedBridge, net.Spec.CIDR)
	if err != nil {
		logger.Error(err, "Invalid Network for NAD")
		return r.markFailed(ctx, net, err)
	}

	if err := r.ensureNAD(ctx, net, cfg); err != nil {
		logger.Error(err, "Failed to ensure Multus NAD")
		return r.markFailed(ctx, net, err)
	}

	return r.markReady(ctx, net)
}

func (r *NetworkReconciler) ensureNAD(ctx context.Context, net *virtfoundryv1alpha1.Network, config string) error {
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(nadGVK)
	err := r.Get(ctx, types.NamespacedName{Namespace: net.Namespace, Name: net.Name}, existing)
	switch {
	case apierrors.IsNotFound(err):
		nad := newNADObject(net, config)
		if err := controllerutil.SetControllerReference(net, nad, r.Scheme); err != nil {
			return err
		}
		return r.Create(ctx, nad)
	case err != nil:
		return err
	}

	if mb, _, _ := unstructured.NestedString(existing.Object, "metadata", "labels", labelManagedBy); mb != "" && mb != managedByOperator {
		return fmt.Errorf("NAD %s/%s managed by %q, refusing overwrite", net.Namespace, net.Name, mb)
	}

	desired := newNADObject(net, config)
	desired.SetResourceVersion(existing.GetResourceVersion())
	if err := controllerutil.SetControllerReference(net, desired, r.Scheme); err != nil {
		return err
	}
	return r.Update(ctx, desired)
}

func newNADObject(net *virtfoundryv1alpha1.Network, config string) *unstructured.Unstructured {
	nad := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": nadAPIVersion,
		"kind":       nadKind,
		"metadata": map[string]any{
			"name":      net.Name,
			"namespace": net.Namespace,
			"labels": map[string]any{
				labelManagedBy: managedByOperator,
			},
		},
		"spec": map[string]any{
			"config": config,
		},
	}}
	nad.SetGroupVersionKind(nadGVK)
	return nad
}

func (r *NetworkReconciler) markReady(ctx context.Context, net *virtfoundryv1alpha1.Network) (ctrl.Result, error) {
	net.Status.Phase = networkPhaseReady
	net.Status.NADName = net.Name
	net.Status.NADNamespace = net.Namespace
	if err := r.Status().Update(ctx, net); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *NetworkReconciler) markFailed(ctx context.Context, net *virtfoundryv1alpha1.Network, cause error) (ctrl.Result, error) {
	net.Status.Phase = networkPhaseFailed
	net.Status.Conditions = []metav1.Condition{{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "ReconcileError",
		Message:            cause.Error(),
		LastTransitionTime: metav1.Now(),
	}}
	if err := r.Status().Update(ctx, net); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, cause
}

// SetupWithManager registers the Network controller.
func (r *NetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&virtfoundryv1alpha1.Network{}).
		Named("network").
		Complete(r)
}
