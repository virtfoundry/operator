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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kubevirtv1 "kubevirt.io/api/core/v1"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

const instanceRequeue = 30 * time.Second

// InstanceReconciler ensures KubeVirt VMs from Instance spec and syncs status.
type InstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=virtfoundry.io,resources=instances,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=virtfoundry.io,resources=instances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=virtfoundry.io,resources=instances/finalizers,verbs=update
// +kubebuilder:rbac:groups=virtfoundry.io,resources=offerings,verbs=get;list;watch
// +kubebuilder:rbac:groups=virtfoundry.io,resources=templates,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines;virtualmachineinstances,verbs=get;list;watch;create;update;patch;delete

func (r *InstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	inst := &virtfoundryv1alpha1.Instance{}
	if err := r.Get(ctx, req.NamespacedName, inst); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	kvName := kubeVirtVMName(inst.Name, inst.Status.KubeVirtName)

	if !inst.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(inst, instanceFinalizer) {
			if err := r.deleteVirtualMachine(ctx, inst.Namespace, kvName); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(inst, instanceFinalizer)
			if err := r.Update(ctx, inst); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(inst, instanceFinalizer) {
		controllerutil.AddFinalizer(inst, instanceFinalizer)
		if err := r.Update(ctx, inst); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if err := r.ensureVirtualMachine(ctx, inst, kvName); err != nil {
		inst.Status.KubeVirtName = kvName
		inst.Status.Phase = virtfoundryv1alpha1.PhaseFailed
		inst.Status.ErrorMessage = err.Error()
		if statusErr := r.Status().Update(ctx, inst); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		logger.Error(err, "ensure VirtualMachine")
		return ctrl.Result{RequeueAfter: instanceRequeue}, nil
	}

	vm := &kubevirtv1.VirtualMachine{}
	err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: kvName}, vm)
	if apierrors.IsNotFound(err) {
		inst.Status.KubeVirtName = kvName
		if inst.Status.Phase == "" {
			inst.Status.Phase = virtfoundryv1alpha1.PhasePending
		}
		if err := r.Status().Update(ctx, inst); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: instanceRequeue}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	phase := instancePhaseFromVM(vm)
	ip := ""
	errMsg := vmErrorMessage(vm)

	vmi := &kubevirtv1.VirtualMachineInstance{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: kvName}, vmi); err == nil {
		phase = instancePhaseFromVMI(vmi, vm)
		ip = preferGuestIP(vmi.Status.Interfaces)
	}

	inst.Status.KubeVirtName = kvName
	inst.Status.Phase = phase
	inst.Status.IP = ip
	inst.Status.ErrorMessage = errMsg

	if err := r.Status().Update(ctx, inst); err != nil {
		return ctrl.Result{}, err
	}

	logger.V(1).Info("synced instance", "phase", phase, "ip", ip)
	return ctrl.Result{RequeueAfter: instanceRequeue}, nil
}

func (r *InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&virtfoundryv1alpha1.Instance{}).
		Owns(&kubevirtv1.VirtualMachine{}).
		Watches(
			&kubevirtv1.VirtualMachine{},
			handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{
						Namespace: obj.GetNamespace(),
						Name:      obj.GetName(),
					},
				}}
			}),
		).
		Watches(
			&kubevirtv1.VirtualMachineInstance{},
			handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{
						Namespace: obj.GetNamespace(),
						Name:      obj.GetName(),
					},
				}}
			}),
		).
		Named("instance").
		Complete(r)
}
