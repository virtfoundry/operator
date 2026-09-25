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

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

const (
	isolationNetworkPolicyName = "virtfoundry-default-deny"
	isolationResourceQuotaName = "virtfoundry-default"
	isolationLimitRangeName    = "virtfoundry-default"

	// PSA privileged is required for KubeVirt virt-launcher (devices, capabilities).
	labelPSAEnforce = "pod-security.kubernetes.io/enforce"
	labelPSAAudit   = "pod-security.kubernetes.io/audit"
	labelPSAWarn    = "pod-security.kubernetes.io/warn"
	psaPrivileged   = "privileged"

	labelK8sName = "kubernetes.io/metadata.name"
	labelK8sApp  = "k8s-app"
)

// ensureTenantIsolation stamps PSA labels and ensures default-deny NetworkPolicy,
// ResourceQuota, and LimitRange in an owned tenant namespace.
//
// NetworkPolicy choices (documented for operators):
//   - Default deny ingress and egress for all pods in the tenant namespace.
//   - Allow DNS (UDP/TCP 53) to kube-system CoreDNS/kube-dns so guests and
//     virt-launcher can resolve cluster DNS.
//   - Allow all traffic within the tenant namespace (CDI importer ↔ DV pods,
//     virt-launcher sidecars).
//   - Allow egress to kubevirt and cdi namespaces (common install names) so
//     live-migration / CDI hooks that talk cluster-CNI can reach controllers.
//   - Allow egress TCP 80/443 cluster-wide so containerDisk and CDI can pull
//     images until a platform image allowlist / registry mirror lands (#13).
//   - Guest pod-network attachment is intentionally NOT granted by this policy
//     alone; Instance reconcile refuses masquerade unless opted in (see
//     instance_vm.go). Multus/VPC NICs bypass the cluster CNI for guest traffic.
func (r *TenantReconciler) ensureTenantIsolation(
	ctx context.Context,
	tenant *virtfoundryv1alpha1.Tenant,
	nsName string,
) error {
	if err := r.ensureDefaultDenyNetworkPolicy(ctx, tenant, nsName); err != nil {
		return err
	}
	if err := r.ensureResourceQuota(ctx, tenant, nsName); err != nil {
		return err
	}
	return r.ensureLimitRange(ctx, tenant, nsName)
}

func tenantPSALabels() map[string]string {
	return map[string]string{
		labelPSAEnforce: psaPrivileged,
		labelPSAAudit:   psaPrivileged,
		labelPSAWarn:    psaPrivileged,
	}
}

func (r *TenantReconciler) ensureDefaultDenyNetworkPolicy(
	ctx context.Context,
	tenant *virtfoundryv1alpha1.Tenant,
	nsName string,
) error {
	np := &networkingv1.NetworkPolicy{}
	np.Name = isolationNetworkPolicyName
	np.Namespace = nsName

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, np, func() error {
		np.Labels = isolationObjectLabels(tenant)
		np.Spec = defaultDenyNetworkPolicySpec()
		return controllerutil.SetControllerReference(tenant, np, r.Scheme)
	})
	return err
}

func defaultDenyNetworkPolicySpec() networkingv1.NetworkPolicySpec {
	udp := corev1.ProtocolUDP
	tcp := corev1.ProtocolTCP
	dnsPort := intstr.FromInt32(53)
	httpPort := intstr.FromInt32(80)
	httpsPort := intstr.FromInt32(443)

	return networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{},
		PolicyTypes: []networkingv1.PolicyType{
			networkingv1.PolicyTypeIngress,
			networkingv1.PolicyTypeEgress,
		},
		// Empty Ingress deny-all; same-namespace peers are allowed explicitly.
		Ingress: []networkingv1.NetworkPolicyIngressRule{{
			From: []networkingv1.NetworkPolicyPeer{{
				PodSelector: &metav1.LabelSelector{},
			}},
		}},
		Egress: []networkingv1.NetworkPolicyEgressRule{
			{
				// DNS to kube-system CoreDNS / kube-dns.
				To: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{labelK8sName: "kube-system"},
					},
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{labelK8sApp: "kube-dns"},
					},
				}},
				Ports: []networkingv1.NetworkPolicyPort{
					{Protocol: &udp, Port: &dnsPort},
					{Protocol: &tcp, Port: &dnsPort},
				},
			},
			{
				// Intra-tenant traffic.
				To: []networkingv1.NetworkPolicyPeer{{
					PodSelector: &metav1.LabelSelector{},
				}},
			},
			{
				// KubeVirt control-plane namespace (default install name).
				To: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{labelK8sName: "kubevirt"},
					},
				}},
			},
			{
				// CDI control-plane namespace (default install name).
				To: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{labelK8sName: "cdi"},
					},
				}},
			},
			{
				// Container image pulls (containerDisk / CDI importer).
				Ports: []networkingv1.NetworkPolicyPort{
					{Protocol: &tcp, Port: &httpPort},
					{Protocol: &tcp, Port: &httpsPort},
				},
			},
		},
	}
}

func (r *TenantReconciler) ensureResourceQuota(
	ctx context.Context,
	tenant *virtfoundryv1alpha1.Tenant,
	nsName string,
) error {
	rq := &corev1.ResourceQuota{}
	rq.Name = isolationResourceQuotaName
	rq.Namespace = nsName

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, rq, func() error {
		rq.Labels = isolationObjectLabels(tenant)
		rq.Spec = corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceRequestsCPU:            resource.MustParse("20"),
				corev1.ResourceRequestsMemory:         resource.MustParse("40Gi"),
				corev1.ResourceLimitsCPU:              resource.MustParse("40"),
				corev1.ResourceLimitsMemory:           resource.MustParse("80Gi"),
				corev1.ResourcePods:                   resource.MustParse("50"),
				corev1.ResourcePersistentVolumeClaims: resource.MustParse("20"),
				corev1.ResourceServices:               resource.MustParse("20"),
			},
		}
		return controllerutil.SetControllerReference(tenant, rq, r.Scheme)
	})
	return err
}

func (r *TenantReconciler) ensureLimitRange(
	ctx context.Context,
	tenant *virtfoundryv1alpha1.Tenant,
	nsName string,
) error {
	lr := &corev1.LimitRange{}
	lr.Name = isolationLimitRangeName
	lr.Namespace = nsName

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, lr, func() error {
		lr.Labels = isolationObjectLabels(tenant)
		lr.Spec = corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{{
				Type: corev1.LimitTypeContainer,
				Default: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				DefaultRequest: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Max: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("8"),
					corev1.ResourceMemory: resource.MustParse("16Gi"),
				},
			}},
		}
		return controllerutil.SetControllerReference(tenant, lr, r.Scheme)
	})
	return err
}

func isolationObjectLabels(tenant *virtfoundryv1alpha1.Tenant) map[string]string {
	return map[string]string{
		labelPartOf:    partOfVirtFoundry,
		labelManagedBy: managedByOperator,
		labelTenant:    tenant.Spec.Slug,
	}
}
