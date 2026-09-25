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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
		const (
			tenantSlug = "acme"
			tenantNS   = "virtfoundry-tenant-acme"
		)

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
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tenantNS}, ns)).To(Succeed())
				g.Expect(ns.Labels["app.kubernetes.io/part-of"]).To(Equal("virtfoundry"))
				g.Expect(ns.Labels["virtfoundry.io/tenant"]).To(Equal(tenantSlug))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				got := &virtfoundryv1alpha1.Tenant{}
				g.Expect(k8sClient.Get(ctx, key, got)).To(Succeed())
				g.Expect(got.Status.Phase).To(Equal("Ready"))
				g.Expect(got.Status.Namespace).To(Equal(tenantNS))
				g.Expect(got.Finalizers).To(ContainElement("virtfoundry.io/finalizer"))
			}, timeout, interval).Should(Succeed())
		})

		It("stamps the namespace with an ownerRef so it is traceable to the Tenant", func() {
			ctx := context.Background()
			tenant := &virtfoundryv1alpha1.Tenant{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tenantSlug}, tenant)).To(Succeed())

			ns := &corev1.Namespace{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tenantNS}, ns)).To(Succeed())
			Expect(ns.Labels[labelManagedBy]).To(Equal(managedByOperator))

			owner := metav1.GetControllerOf(ns)
			Expect(owner).NotTo(BeNil())
			Expect(owner.Kind).To(Equal("Tenant"))
			Expect(owner.UID).To(Equal(tenant.UID))
		})

		It("deletes the namespace it owns when the Tenant is deleted", func() {
			ctx := context.Background()
			tenant := &virtfoundryv1alpha1.Tenant{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tenantSlug}, tenant)).To(Succeed())
			Expect(k8sClient.Delete(ctx, tenant)).To(Succeed())

			r := &TenantReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: tenantSlug}})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.RequeueAfter).To(BeNumerically(">", 0))

			// envtest has no namespace controller, so the namespace stays
			// Terminating; the deletion timestamp proves the delete was issued.
			Eventually(func(g Gomega) {
				ns := &corev1.Namespace{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tenantNS}, ns)).To(Succeed())
				g.Expect(ns.DeletionTimestamp.IsZero()).To(BeFalse())
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("when a namespace of the same name already exists", func() {
		const tenantSlug = "squatter"
		const nsName = "virtfoundry-tenant-squatter"

		It("refuses to adopt a namespace that is not labelled as ours", func() {
			ctx := context.Background()
			key := types.NamespacedName{Name: tenantSlug}

			foreign := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}
			Expect(k8sClient.Create(ctx, foreign)).To(Succeed())

			tenant := &virtfoundryv1alpha1.Tenant{
				ObjectMeta: metav1.ObjectMeta{Name: tenantSlug},
				Spec: virtfoundryv1alpha1.TenantSpec{
					Name: "Squatter",
					Slug: tenantSlug,
				},
			}
			Expect(k8sClient.Create(ctx, tenant)).To(Succeed())

			r := &TenantReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())

			_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, errNamespaceNotOwned)).To(BeTrue())
			Expect(errors.Is(err, reconcile.TerminalError(nil))).To(BeTrue())

			By("leaving the foreign namespace untouched")
			ns := &corev1.Namespace{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nsName}, ns)).To(Succeed())
			Expect(ns.Labels).NotTo(HaveKey(labelTenant))
			Expect(ns.OwnerReferences).To(BeEmpty())

			Eventually(func(g Gomega) {
				got := &virtfoundryv1alpha1.Tenant{}
				g.Expect(k8sClient.Get(ctx, key, got)).To(Succeed())
				g.Expect(got.Status.Phase).To(Equal("Failed"))
			}, timeout, interval).Should(Succeed())
		})

		It("never deletes that namespace, and still releases the Tenant", func() {
			ctx := context.Background()
			key := types.NamespacedName{Name: tenantSlug}

			tenant := &virtfoundryv1alpha1.Tenant{}
			Expect(k8sClient.Get(ctx, key, tenant)).To(Succeed())
			Expect(k8sClient.Delete(ctx, tenant)).To(Succeed())

			r := &TenantReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())

			By("keeping the foreign namespace alive")
			ns := &corev1.Namespace{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nsName}, ns)).To(Succeed())
			Expect(ns.DeletionTimestamp.IsZero()).To(BeTrue())

			By("dropping the finalizer so the Tenant is not wedged")
			Eventually(func() bool {
				got := &virtfoundryv1alpha1.Tenant{}
				return apierrors.IsNotFound(k8sClient.Get(ctx, key, got))
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("when the Tenant namespace was never created", func() {
		const tenantSlug = "ghost"

		It("removes the finalizer without touching any namespace", func() {
			ctx := context.Background()
			key := types.NamespacedName{Name: tenantSlug}

			tenant := &virtfoundryv1alpha1.Tenant{
				ObjectMeta: metav1.ObjectMeta{Name: tenantSlug},
				Spec: virtfoundryv1alpha1.TenantSpec{
					Name: "Ghost",
					Slug: tenantSlug,
				},
			}
			Expect(k8sClient.Create(ctx, tenant)).To(Succeed())

			r := &TenantReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, tenant)).To(Succeed())

			_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				got := &virtfoundryv1alpha1.Tenant{}
				return apierrors.IsNotFound(k8sClient.Get(ctx, key, got))
			}, timeout, interval).Should(BeTrue())

			ns := &corev1.Namespace{}
			Expect(apierrors.IsNotFound(
				k8sClient.Get(ctx, types.NamespacedName{Name: "virtfoundry-tenant-ghost"}, ns),
			)).To(BeTrue())
		})
	})
})
