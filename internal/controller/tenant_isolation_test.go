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
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
)

func TestDefaultDenyNetworkPolicySpec_HasDNSAndPolicyTypes(t *testing.T) {
	spec := defaultDenyNetworkPolicySpec()
	if len(spec.PolicyTypes) != 2 {
		t.Fatalf("expected ingress+egress policy types, got %#v", spec.PolicyTypes)
	}
	hasIngress, hasEgress := false, false
	for _, pt := range spec.PolicyTypes {
		if pt == networkingv1.PolicyTypeIngress {
			hasIngress = true
		}
		if pt == networkingv1.PolicyTypeEgress {
			hasEgress = true
		}
	}
	if !hasIngress || !hasEgress {
		t.Fatalf("missing policy types: %#v", spec.PolicyTypes)
	}
	if len(spec.Egress) < 3 {
		t.Fatalf("expected DNS + intra + platform egress rules, got %d", len(spec.Egress))
	}
	// First egress rule is DNS to kube-system.
	dns := spec.Egress[0]
	if len(dns.Ports) != 2 {
		t.Fatalf("expected UDP+TCP DNS ports, got %#v", dns.Ports)
	}
	if dns.To[0].NamespaceSelector == nil || dns.To[0].NamespaceSelector.MatchLabels[labelK8sName] != "kube-system" {
		t.Fatalf("DNS rule should target kube-system, got %#v", dns.To)
	}
}

func TestTenantPSALabels_Privileged(t *testing.T) {
	labels := tenantPSALabels()
	if labels[labelPSAEnforce] != psaPrivileged {
		t.Fatalf("enforce: got %q", labels[labelPSAEnforce])
	}
	if labels[labelPSAAudit] != psaPrivileged || labels[labelPSAWarn] != psaPrivileged {
		t.Fatalf("audit/warn: %#v", labels)
	}
}
