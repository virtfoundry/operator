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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

func TestAssertInstanceInTenantNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = virtfoundryv1alpha1.AddToScheme(scheme)

	goodNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: tenantNamespaceName(testAcmeSlug),
			Labels: map[string]string{
				labelPartOf: partOfVirtFoundry,
				labelTenant: testAcmeSlug,
			},
		},
	}
	badNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "virtfoundry-tenant-orphan",
			Labels: map[string]string{
				labelPartOf: partOfVirtFoundry,
			},
		},
	}

	r := &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(goodNS, badNS).Build(),
	}

	t.Run("accepts labeled tenant namespace", func(t *testing.T) {
		inst := &virtfoundryv1alpha1.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: tenantNamespaceName(testAcmeSlug)},
		}
		if err := r.assertInstanceInTenantNamespace(context.Background(), inst); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("rejects missing tenant label", func(t *testing.T) {
		inst := &virtfoundryv1alpha1.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: "virtfoundry-tenant-orphan"},
		}
		err := r.assertInstanceInTenantNamespace(context.Background(), inst)
		if err == nil || !strings.Contains(err.Error(), labelTenant) {
			t.Fatalf("expected tenant label error, got %v", err)
		}
	})

	t.Run("rejects non-tenant namespace name", func(t *testing.T) {
		inst := &virtfoundryv1alpha1.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: namespaceDefault},
		}
		err := r.assertInstanceInTenantNamespace(context.Background(), inst)
		if err == nil || !strings.Contains(err.Error(), tenantNamespacePrefix) {
			t.Fatalf("expected prefix error, got %v", err)
		}
	})
}
