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
	"sigs.k8s.io/controller-runtime/pkg/client"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

// assertInstanceInTenantNamespace refuses to reconcile Instances outside a
// VirtFoundry-labeled tenant namespace (virtfoundry-tenant-* with
// virtfoundry.io/tenant set).
func (r *InstanceReconciler) assertInstanceInTenantNamespace(
	ctx context.Context,
	inst *virtfoundryv1alpha1.Instance,
) error {
	nsName := inst.Namespace
	if !strings.HasPrefix(nsName, tenantNamespacePrefix) {
		return fmt.Errorf(
			"instance namespace %q is not a VirtFoundry tenant namespace (want prefix %q)",
			nsName, tenantNamespacePrefix,
		)
	}

	ns := &corev1.Namespace{}
	if err := r.Get(ctx, client.ObjectKey{Name: nsName}, ns); err != nil {
		return fmt.Errorf("get namespace %q: %w", nsName, err)
	}
	if ns.Labels[labelPartOf] != partOfVirtFoundry {
		return fmt.Errorf(
			"namespace %q is missing label %s=%s",
			nsName, labelPartOf, partOfVirtFoundry,
		)
	}
	if strings.TrimSpace(ns.Labels[labelTenant]) == "" {
		return fmt.Errorf(
			"namespace %q is missing label %s (not a managed tenant namespace)",
			nsName, labelTenant,
		)
	}
	return nil
}
