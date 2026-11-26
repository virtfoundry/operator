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
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

// Experimental backend for development on Apple Silicon, where KubeVirt cannot
// run: Instances become KubeSilicon SiliconVMs (github.com/Matheus-Thurler/KubeSilicon).
// Guests are NATed on the Mac: spec.nics is ignored, so there is NO tenant network
// isolation. The operator runs out of cluster; the chart grants no RBAC for it.
const (
	InstanceBackendKubeVirt    = "kubevirt"
	InstanceBackendKubeSilicon = "kubesilicon"

	// annotationKubeSiliconDisk on a Template is the arm64 cloud image (http(s) URL or
	// a file under ~/.kubesilicon/images). Template.spec.image is an amd64 containerDisk.
	annotationKubeSiliconDisk = "virtfoundry.io/kubesilicon-disk"
)

var siliconVMGVK = schema.GroupVersionKind{Group: "kubesilicon.io", Version: "v1", Kind: "SiliconVM"}

func newSiliconVM(namespace, name string) *unstructured.Unstructured {
	vm := &unstructured.Unstructured{}
	vm.SetGroupVersionKind(siliconVMGVK)
	vm.SetNamespace(namespace)
	vm.SetName(name)
	return vm
}

func (r *InstanceReconciler) siliconSpec(ctx context.Context, inst *virtfoundryv1alpha1.Instance) (map[string]any, error) {
	cpu, memoryMi := 1, int64(512)
	if ref := inst.Spec.OfferingRef; ref != nil && ref.Name != "" {
		off := &virtfoundryv1alpha1.Offering{}
		if err := r.Get(ctx, client.ObjectKey{Name: ref.Name}, off); err != nil {
			return nil, fmt.Errorf("offering %q: %w", ref.Name, err)
		}
		if err := validateResolvedOffering(off); err != nil {
			return nil, fmt.Errorf("offering %q: %w", off.Name, err)
		}
		cpu, memoryMi = off.Spec.CPU, off.Spec.MemoryMi
	}
	if inst.Spec.TemplateRef == nil || strings.TrimSpace(inst.Spec.TemplateRef.Name) == "" {
		return nil, fmt.Errorf("spec.templateRef is required")
	}
	tmpl, err := r.resolveTemplate(ctx, inst, inst.Spec.TemplateRef.Name)
	if err != nil {
		return nil, err
	}
	disk := strings.TrimSpace(tmpl.Annotations[annotationKubeSiliconDisk])
	if disk == "" {
		return nil, fmt.Errorf("template %q has no %s annotation (arm64 cloud image)", tmpl.Name, annotationKubeSiliconDisk)
	}
	userData, err := r.resolveCloudInitUserData(ctx, inst, tmpl)
	if err != nil {
		return nil, err
	}
	keys, err := r.resolveSSHPublicKeys(ctx, inst)
	if err != nil {
		return nil, err
	}
	if len(keys) > 0 {
		if userData, err = mergeCloudInitWithSSHKeys(userData, keys); err != nil {
			return nil, err
		}
	}
	spec := map[string]any{
		"running": instancePowerState(inst) == powerStateRunning,
		"cpu":     int64(cpu),
		"memory":  fmt.Sprintf("%dMi", memoryMi),
		"disk":    map[string]any{"source": disk},
	}
	if userData != "" {
		spec["cloudInit"] = map[string]any{"userData": userData}
	}
	return spec, nil
}

func (r *InstanceReconciler) ensureSiliconVM(ctx context.Context, inst *virtfoundryv1alpha1.Instance, name string) (*unstructured.Unstructured, error) {
	spec, err := r.siliconSpec(ctx, inst)
	if err != nil {
		return nil, err
	}
	vm := newSiliconVM(inst.Namespace, name)
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, vm, func() error {
		// Only the power state changes after creation, as with the KubeVirt backend
		// where size and image are create-time only.
		if vm.GetResourceVersion() == "" {
			vm.Object["spec"] = spec
		} else if err := unstructured.SetNestedField(vm.Object, spec["running"], "spec", "running"); err != nil {
			return err
		}
		return controllerutil.SetControllerReference(inst, vm, r.Scheme)
	})
	return vm, err
}

func instancePhaseFromSiliconVM(phase string) string {
	switch phase {
	case "Running":
		return instancePhaseRunning
	case "Stopped":
		return instancePhaseStopped
	case "Provisioning", "Starting", "Restarting":
		return instancePhaseStarting
	case "Failed", "Error":
		return instancePhaseError
	default:
		return instancePhasePending
	}
}

func (r *InstanceReconciler) reconcileSiliconVM(ctx context.Context, inst *virtfoundryv1alpha1.Instance, name string) (ctrl.Result, error) {
	inst.Status.KubeVirtName = name
	vm, err := r.ensureSiliconVM(ctx, inst, name)
	if err != nil {
		logf.FromContext(ctx).Error(err, "ensure SiliconVM")
		inst.Status.Phase = virtfoundryv1alpha1.PhaseFailed
		inst.Status.ErrorMessage = err.Error()
	} else {
		phase, _, _ := unstructured.NestedString(vm.Object, "status", "phase")
		inst.Status.Phase = instancePhaseFromSiliconVM(phase)
		inst.Status.IP, _, _ = unstructured.NestedString(vm.Object, "status", "ip")
		inst.Status.ErrorMessage = ""
		if inst.Status.Phase == instancePhaseError {
			inst.Status.ErrorMessage, _, _ = unstructured.NestedString(vm.Object, "status", "message")
		}
	}
	if err := r.Status().Update(ctx, inst); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: instanceRequeue}, nil
}

func (r *InstanceReconciler) deleteSiliconVM(ctx context.Context, namespace, name string) error {
	err := r.Delete(ctx, newSiliconVM(namespace, name))
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
