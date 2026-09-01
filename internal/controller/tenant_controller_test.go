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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

var _ = Describe("Tenant Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("when creating a Tenant", func() {
		const tenantSlug = "acme"

		It("creates namespace virtfoundry-tenant-{slug} and sets Ready", func() {
			ctx := context.Background()
			key := types.NamespacedName{Name: tenantSlug}

			tenant := &virtfoundryv1alpha1.Tenant{
				ObjectMeta: metav1.ObjectMeta{Name: tenantSlug},
				Spec: virtfoundryv1alpha1.TenantSpec{
					Name: "Acme Corp",
					Slug: tenantSlug,
				},
			}
			Expect(k8sClient.Create(ctx, tenant)).To(Succeed())

			r := &TenantReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}

			By("adding finalizer")
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())

			By("ensuring namespace and status")
			_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				ns := &corev1.Namespace{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "virtfoundry-tenant-acme"}, ns)).To(Succeed())
				g.Expect(ns.Labels["app.kubernetes.io/part-of"]).To(Equal("virtfoundry"))
				g.Expect(ns.Labels["virtfoundry.io/tenant"]).To(Equal(tenantSlug))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				got := &virtfoundryv1alpha1.Tenant{}
				g.Expect(k8sClient.Get(ctx, key, got)).To(Succeed())
				g.Expect(got.Status.Phase).To(Equal("Ready"))
				g.Expect(got.Status.Namespace).To(Equal("virtfoundry-tenant-acme"))
				g.Expect(got.Finalizers).To(ContainElement("virtfoundry.io/finalizer"))
			}, timeout, interval).Should(Succeed())
		})
	})
})
