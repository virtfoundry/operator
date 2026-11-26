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
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

// LoadBalancerReconciler turns a LoadBalancer CR into a type=LoadBalancer
// Service plus manual EndpointSlices that point at the guest IPs of its TargetGroups.
type LoadBalancerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=virtfoundry.io,resources=loadbalancers;targetgroups,verbs=get;list;watch
// +kubebuilder:rbac:groups=virtfoundry.io,resources=loadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch;create;update;patch;delete

// Reconcile converges the Service and Endpoints of one LoadBalancer.
func (r *LoadBalancerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lb := &virtfoundryv1alpha1.LoadBalancer{}
	if err := r.Get(ctx, req.NamespacedName, lb); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !lb.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil // Service and Endpoints are owned: GC removes them.
	}

	svcName := lbServiceName(lb)
	tgList := &virtfoundryv1alpha1.TargetGroupList{}
	if err := r.List(ctx, tgList, client.InNamespace(lb.Namespace)); err != nil {
		return ctrl.Result{}, err
	}
	groups := make(map[string]*virtfoundryv1alpha1.TargetGroup, len(tgList.Items))
	for i := range tgList.Items {
		groups[tgList.Items[i].Name] = &tgList.Items[i]
	}

	ports := lbServicePorts(lb, groups)
	if len(ports) == 0 {
		// A Service needs at least one port: wait for the first usable listener.
		if err := r.deleteOwned(ctx, lb, svcName); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, r.setStatus(ctx, lb, virtfoundryv1alpha1.PhasePending, svcName, "", "NoListeners", "no listener with an existing target group")
	}

	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: svcName, Namespace: lb.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		if svc.Labels == nil {
			svc.Labels = map[string]string{}
		}
		svc.Labels[labelManagedBy] = managedByOperator
		svc.Labels[lbLabel] = lb.Name
		svc.Spec.Type = corev1.ServiceTypeLoadBalancer
		svc.Spec.Ports = ports // ClusterIP and the rest of the spec are left to Kubernetes.
		return controllerutil.SetControllerReference(lb, svc, r.Scheme)
	}); err != nil {
		return r.fail(ctx, lb, svcName, err)
	}

	if err := r.syncSlices(ctx, lb, svcName, lbEndpointSlices(lb, svcName, groups)); err != nil {
		return r.fail(ctx, lb, svcName, err)
	}

	vip := serviceVIP(svc)
	if vip == "" {
		return ctrl.Result{}, r.setStatus(ctx, lb, virtfoundryv1alpha1.PhasePending, svcName, "", "VIPPending", "waiting for the load balancer address")
	}
	return ctrl.Result{}, r.setStatus(ctx, lb, virtfoundryv1alpha1.PhaseReady, svcName, vip, "Reconciled", "")
}

const (
	lbLabel        = "virtfoundry.io/load-balancer"
	conditionReady = "Ready"
)

func lbServiceName(lb *virtfoundryv1alpha1.LoadBalancer) string { return "lb-" + lb.Name }

func lbPortName(l virtfoundryv1alpha1.LoadBalancerListener) string {
	return fmt.Sprintf("%s-%d", l.Protocol, l.Port)
}

func lbProtocol(p string) corev1.Protocol {
	if p == "udp" {
		return corev1.ProtocolUDP
	}
	return corev1.ProtocolTCP
}

// lbServicePorts returns one Service port per listener whose target group exists,
// sorted so reconciles are stable.
func lbServicePorts(lb *virtfoundryv1alpha1.LoadBalancer, groups map[string]*virtfoundryv1alpha1.TargetGroup) []corev1.ServicePort {
	var out []corev1.ServicePort
	for _, l := range lb.Spec.Listeners {
		tg, ok := groups[l.TargetGroupRef.Name]
		if !ok {
			continue
		}
		out = append(out, corev1.ServicePort{
			Name:       lbPortName(l),
			Port:       l.Port,
			TargetPort: intstr.FromInt32(tg.Spec.Port),
			Protocol:   lbProtocol(l.Protocol),
		})
	}
	slices.SortFunc(out, func(a, b corev1.ServicePort) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// lbEndpointSlices returns one EndpointSlice per listener, so a listener only
// forwards to its own target group instead of every group's addresses.
func lbEndpointSlices(lb *virtfoundryv1alpha1.LoadBalancer, svcName string, groups map[string]*virtfoundryv1alpha1.TargetGroup) []*discoveryv1.EndpointSlice {
	var out []*discoveryv1.EndpointSlice
	for _, l := range lb.Spec.Listeners {
		tg, ok := groups[l.TargetGroupRef.Name]
		if !ok || len(tg.Spec.Targets) == 0 {
			continue
		}
		var addrs []string
		for _, t := range tg.Spec.Targets {
			if !slices.Contains(addrs, t.IP) {
				addrs = append(addrs, t.IP)
			}
		}
		slices.Sort(addrs)
		slice := &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{Name: svcName + "-" + lbPortName(l), Namespace: lb.Namespace},
			Endpoints:  []discoveryv1.Endpoint{{Addresses: addrs}},
			Ports: []discoveryv1.EndpointPort{{
				Name:     ptr(lbPortName(l)),
				Port:     ptr(tg.Spec.Port),
				Protocol: ptr(lbProtocol(l.Protocol)),
			}},
		}
		if strings.Contains(addrs[0], ":") {
			slice.AddressType = discoveryv1.AddressTypeIPv6
		} else {
			slice.AddressType = discoveryv1.AddressTypeIPv4
		}
		out = append(out, slice)
	}
	return out
}

func ptr[T any](v T) *T { return &v }

// syncSlices makes the owned EndpointSlices of svcName match want and deletes the rest.
func (r *LoadBalancerReconciler) syncSlices(ctx context.Context, lb *virtfoundryv1alpha1.LoadBalancer, svcName string, want []*discoveryv1.EndpointSlice) error {
	keep := map[string]struct{}{}
	for _, w := range want {
		keep[w.Name] = struct{}{}
		slice := &discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: w.Name, Namespace: w.Namespace}}
		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, slice, func() error {
			slice.Labels = map[string]string{
				discoveryv1.LabelServiceName: svcName,
				labelManagedBy:               managedByOperator,
				lbLabel:                      lb.Name,
			}
			slice.AddressType, slice.Endpoints, slice.Ports = w.AddressType, w.Endpoints, w.Ports
			return controllerutil.SetControllerReference(lb, slice, r.Scheme)
		}); err != nil {
			return err
		}
	}
	return r.deleteSlices(ctx, lb, keep)
}

// deleteSlices removes EndpointSlices controlled by lb whose name is not in keep.
func (r *LoadBalancerReconciler) deleteSlices(ctx context.Context, lb *virtfoundryv1alpha1.LoadBalancer, keep map[string]struct{}) error {
	list := &discoveryv1.EndpointSliceList{}
	if err := r.List(ctx, list, client.InNamespace(lb.Namespace), client.MatchingLabels{lbLabel: lb.Name}); err != nil {
		return err
	}
	for i := range list.Items {
		s := &list.Items[i]
		if _, ok := keep[s.Name]; ok || !metav1.IsControlledBy(s, lb) {
			continue
		}
		if err := r.Delete(ctx, s); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func serviceVIP(svc *corev1.Service) string {
	for _, ing := range svc.Status.LoadBalancer.Ingress {
		if ing.IP != "" {
			return ing.IP
		}
		if ing.Hostname != "" {
			return ing.Hostname
		}
	}
	return ""
}

func (r *LoadBalancerReconciler) deleteOwned(ctx context.Context, lb *virtfoundryv1alpha1.LoadBalancer, name string) error {
	svc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Namespace: lb.Namespace, Name: name}, svc)
	switch {
	case apierrors.IsNotFound(err):
	case err != nil:
		return err
	case metav1.IsControlledBy(svc, lb): // never delete what another actor owns
		if err := r.Delete(ctx, svc); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return r.deleteSlices(ctx, lb, nil)
}

func (r *LoadBalancerReconciler) fail(ctx context.Context, lb *virtfoundryv1alpha1.LoadBalancer, svcName string, cause error) (ctrl.Result, error) {
	if err := r.setStatus(ctx, lb, virtfoundryv1alpha1.PhaseFailed, svcName, "", "ReconcileError", cause.Error()); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, cause
}

func (r *LoadBalancerReconciler) setStatus(ctx context.Context, lb *virtfoundryv1alpha1.LoadBalancer, phase, svcName, vip, reason, msg string) error {
	status := metav1.ConditionTrue
	if phase != virtfoundryv1alpha1.PhaseReady {
		status = metav1.ConditionFalse
	}
	lb.Status.Phase = phase
	lb.Status.ServiceName = svcName
	lb.Status.VIP = vip
	lb.Status.Conditions = []metav1.Condition{{
		Type:               conditionReady,
		Status:             status,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: lb.Generation,
		LastTransitionTime: metav1.Now(),
	}}
	return r.Status().Update(ctx, lb)
}

// lbsForTargetGroup maps a TargetGroup event to the LoadBalancers that use it.
func (r *LoadBalancerReconciler) lbsForTargetGroup(ctx context.Context, obj client.Object) []reconcile.Request {
	list := &virtfoundryv1alpha1.LoadBalancerList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		return nil
	}
	var out []reconcile.Request
	for _, lb := range list.Items {
		for _, l := range lb.Spec.Listeners {
			if l.TargetGroupRef.Name == obj.GetName() {
				out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: lb.Namespace, Name: lb.Name}})
				break
			}
		}
	}
	return out
}

// SetupWithManager registers the LoadBalancer controller.
func (r *LoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&virtfoundryv1alpha1.LoadBalancer{}).
		Owns(&corev1.Service{}).
		Watches(&virtfoundryv1alpha1.TargetGroup{}, handler.EnqueueRequestsFromMapFunc(r.lbsForTargetGroup)).
		Named("loadbalancer").
		Complete(r)
}
