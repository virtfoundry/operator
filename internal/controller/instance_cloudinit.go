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

func normalizeSSHPublicKeys(keys []string) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k != "" {
			out = append(out, k)
		}
	}
	return out
}

// mergeCloudInitWithSSHKeys builds Linux #cloud-config with ssh_authorized_keys
// and appends optional Template userdata (core BuildLinuxUserData shape).
func mergeCloudInitWithSSHKeys(baseUserData string, publicKeys []string) (string, error) {
	keys := normalizeSSHPublicKeys(publicKeys)
	if len(keys) == 0 {
		return "", fmt.Errorf("sshKeyRefs resolved to no public keys")
	}

	var b strings.Builder
	b.WriteString("#cloud-config\n")
	b.WriteString("ssh_pwauth: false\n")
	b.WriteString("users:\n")
	b.WriteString("  - name: ubuntu\n")
	b.WriteString("    sudo: ALL=(ALL) NOPASSWD:ALL\n")
	b.WriteString("    shell: /bin/bash\n")
	b.WriteString("    lock_passwd: true\n")
	b.WriteString("    ssh_authorized_keys:\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "      - %s\n", k)
	}
	b.WriteString("runcmd:\n")
	b.WriteString("  - [ systemctl, enable, --now, getty@tty1 ]\n")

	if extra := strings.TrimSpace(baseUserData); extra != "" {
		b.WriteString("\n")
		b.WriteString(extra)
		if !strings.HasSuffix(extra, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}
