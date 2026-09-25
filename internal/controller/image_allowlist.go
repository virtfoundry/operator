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
	"os"
	"strings"
)

// envAllowedContainerImagePrefixes overrides the built-in ContainerDisk allowlist
// (comma-separated image reference prefixes). Empty / unset → defaults.
const envAllowedContainerImagePrefixes = "VIRTFOUNDRY_ALLOWED_CONTAINER_IMAGE_PREFIXES"

// defaultContainerImagePrefixes are the registries VirtFoundry ships in the
// catalog (cirros demo + quay containerdisks). Operators with a private mirror
// must set VIRTFOUNDRY_ALLOWED_CONTAINER_IMAGE_PREFIXES explicitly — configuring
// the env replaces the built-in list (same model as core ISO allowlist).
var defaultContainerImagePrefixes = []string{
	"quay.io/containerdisks/",
	"quay.io/kubevirt/",
}

func effectiveContainerImagePrefixes(override []string) []string {
	if len(override) > 0 {
		return override
	}
	if env := strings.TrimSpace(os.Getenv(envAllowedContainerImagePrefixes)); env != "" {
		return parseImageAllowlist(env)
	}
	return defaultContainerImagePrefixes
}

func parseImageAllowlist(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// validateContainerDiskImage rejects ContainerDisk refs that are not on the
// allowlist (issue #22). HTTP(S) URLs must use the ISO/CDI path — never land
// as containerDisk.image.
func validateContainerDiskImage(image string, override []string) error {
	image = strings.TrimSpace(image)
	if image == "" {
		return fmt.Errorf("container disk image is empty")
	}
	lower := strings.ToLower(image)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return fmt.Errorf("container disk image %q looks like an HTTP(S) URL; use an ISO template / CDI import instead", image)
	}
	allowed := effectiveContainerImagePrefixes(override)
	for _, prefix := range allowed {
		if prefix != "" && strings.HasPrefix(image, prefix) {
			return nil
		}
	}
	return fmt.Errorf("container disk image %q is not on the allowlist (allowed prefixes: %s)",
		image, strings.Join(allowed, ", "))
}
