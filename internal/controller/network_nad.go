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
	"fmt"
	"strings"
)

const (
	networkTypeIsolated = "isolated"
	networkTypeShared   = "shared"

	defaultIsolatedBridge = "virtfoundry-br0"

	networkPhaseReady  = "Ready"
	networkPhaseFailed = "Failed"

	nadAPIVersion = "k8s.cni.cncf.io/v1"
	nadKind       = "NetworkAttachmentDefinition"
)

// buildIsolatedNADConfig mirrors core/internal/platform/k8s/network.go createBridgeNAD
// (bridge CNI + host-local IPAM). Shared/public NADs are out of scope for v1.
func buildIsolatedNADConfig(name, bridge, cidr string) (string, error) {
	if strings.TrimSpace(cidr) == "" {
		return "", fmt.Errorf("network cidr is required")
	}
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("network attachment name is required")
	}
	if bridge == "" {
		bridge = defaultIsolatedBridge
	}
	ipam := fmt.Sprintf(`{
    "type": "host-local",
    "subnet": %q,
    "routes": [{ "dst": "0.0.0.0/0" }]
  }`, cidr)
	return fmt.Sprintf(`{
  "cniVersion": "0.3.1",
  "name": %q,
  "type": "bridge",
  "bridge": %q,
  "ipam": %s
}`, name, bridge, ipam), nil
}
