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

func TestAssertInstanceTenantNamespace(t *testing.T) {
	t.Parallel()
	if err := assertInstanceTenantNamespace("virtfoundry-tenant-acme"); err != nil {
		t.Fatalf("accepted tenant ns rejected: %v", err)
	}
	for _, ns := range []string{namespaceDefault, operatorNamespace, "virtfoundry-tenant-", "argocd", "other"} {
		if err := assertInstanceTenantNamespace(ns); err == nil {
			t.Fatalf("expected reject for %q", ns)
		}
	}
}

func TestValidateOfferingSpec(t *testing.T) {
	t.Parallel()
	if err := validateOfferingSpec(1, 64); err != nil {
		t.Fatalf("min bounds rejected: %v", err)
	}
	if err := validateOfferingSpec(256, 1048576); err != nil {
		t.Fatalf("max bounds rejected: %v", err)
	}
	cases := []struct {
		cpu int
		mem int64
	}{
		{0, 512},
		{-1, 512},
		{257, 512},
		{1, 63},
		{1, 0},
		{1, 1048577},
	}
	for _, tc := range cases {
		if err := validateOfferingSpec(tc.cpu, tc.mem); err == nil {
			t.Fatalf("expected reject cpu=%d mem=%d", tc.cpu, tc.mem)
		}
	}
}

func TestVMResourceRequirements_NoPanicOnBadInput(t *testing.T) {
	t.Parallel()
	if _, err := vmResourceRequirements(-1, 1, false); err == nil {
		t.Fatal("expected error for negative memory")
	}
	if _, err := vmResourceRequirements(512, -4, true); err == nil {
		t.Fatal("expected error for negative cpu")
	}
	reqs, err := vmResourceRequirements(1024, 2, true)
	if err != nil {
		t.Fatalf("valid input: %v", err)
	}
	if reqs.Requests.Memory().IsZero() {
		t.Fatal("expected memory request")
	}
}
