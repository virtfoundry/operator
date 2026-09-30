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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

// Default Secret data key for cloud-init user-data (operator#16).
const cloudInitSecretKeyDefault = "userData"

// resolveCloudInitUserData prefers Secret refs over legacy inline strings.
// Precedence: Instance secretRef → Instance string → Template secretRef → Template string.
// Errors never include Secret body bytes.
func (r *InstanceReconciler) resolveCloudInitUserData(
	ctx context.Context,
	inst *virtfoundryv1alpha1.Instance,
	tmpl *virtfoundryv1alpha1.Template,
) (string, error) {
	if ref := inst.Spec.CloudInitSecretRef; ref != nil && strings.TrimSpace(ref.Name) != "" {
		return r.readCloudInitSecret(ctx, inst.Namespace, *ref)
	}
	if strings.TrimSpace(inst.Spec.CloudInitUserData) != "" {
		return inst.Spec.CloudInitUserData, nil
	}
	if ref := tmpl.Spec.CloudInitSecretRef; ref != nil && strings.TrimSpace(ref.Name) != "" {
		return r.readCloudInitSecretForTemplate(ctx, inst, tmpl, *ref)
	}
	// Legacy Template.spec.cloudInitUserData (deprecated).
	return tmpl.Spec.CloudInitUserData, nil
}

// readCloudInitSecretForTemplate loads the Secret from the Instance (tenant)
// namespace first, then the Template namespace (catalog Templates in
// virtfoundry-system may share a Secret name that exists per-tenant).
func (r *InstanceReconciler) readCloudInitSecretForTemplate(
	ctx context.Context,
	inst *virtfoundryv1alpha1.Instance,
	tmpl *virtfoundryv1alpha1.Template,
	ref virtfoundryv1alpha1.SecretKeyRef,
) (string, error) {
	namespaces := []string{inst.Namespace}
	if tmpl.Namespace != "" && tmpl.Namespace != inst.Namespace {
		namespaces = append(namespaces, tmpl.Namespace)
	}
	var lastNotFound error
	for _, ns := range namespaces {
		data, err := r.readCloudInitSecret(ctx, ns, ref)
		if err == nil {
			return data, nil
		}
		if apierrors.IsNotFound(err) {
			lastNotFound = err
			continue
		}
		// Missing key / empty value: do not try another namespace.
		return "", err
	}
	if lastNotFound != nil {
		return "", lastNotFound
	}
	return "", fmt.Errorf("cloudInitSecretRef %q: secret not found", strings.TrimSpace(ref.Name))
}

func (r *InstanceReconciler) readCloudInitSecret(
	ctx context.Context,
	namespace string,
	ref virtfoundryv1alpha1.SecretKeyRef,
) (string, error) {
	name := strings.TrimSpace(ref.Name)
	if name == "" {
		return "", fmt.Errorf("cloudInitSecretRef.name is required")
	}
	key := strings.TrimSpace(ref.Key)
	if key == "" {
		key = cloudInitSecretKeyDefault
	}

	sec := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, sec); err != nil {
		// Preserve NotFound for callers; never attach Secret body.
		return "", fmt.Errorf("cloudInitSecretRef secret %q in namespace %q: %w", name, namespace, err)
	}

	raw, ok := sec.Data[key]
	if !ok {
		return "", fmt.Errorf("cloudInitSecretRef secret %q in namespace %q: missing key %q", name, namespace, key)
	}
	if len(raw) == 0 {
		return "", fmt.Errorf("cloudInitSecretRef secret %q in namespace %q: key %q is empty", name, namespace, key)
	}
	// Return as string for existing NoCloud UserData path; callers must not log it.
	return string(raw), nil
}
