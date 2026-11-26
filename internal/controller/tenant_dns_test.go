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

import "testing"

func TestNodeLocalDNSOpensOnlyPort53ToAnyPeer(t *testing.T) {
	if len(defaultDenyNetworkPolicySpec(false).Egress[0].To) == 0 {
		t.Fatal("by default DNS must stay restricted to kube-dns")
	}
	rule := defaultDenyNetworkPolicySpec(true).Egress[0]
	if rule.To != nil || len(rule.Ports) != 2 || rule.Ports[0].Port.IntValue() != 53 || rule.Ports[1].Port.IntValue() != 53 {
		t.Fatalf("unexpected DNS rule: %+v", rule)
	}
}
