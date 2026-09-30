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
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

// resolveSSHPublicKeys loads SSHKey CRs named by Instance.spec.sshKeyRefs.
func (r *InstanceReconciler) resolveSSHPublicKeys(
	ctx context.Context,
	inst *virtfoundryv1alpha1.Instance,
) ([]string, error) {
	if len(inst.Spec.SSHKeyRefs) == 0 {
		return nil, nil
	}

	keys := make([]string, 0, len(inst.Spec.SSHKeyRefs))
	for i, ref := range inst.Spec.SSHKeyRefs {
		name := strings.TrimSpace(ref.Name)
		if name == "" {
			return nil, fmt.Errorf("spec.sshKeyRefs[%d]: name is required", i)
		}
		ssh := &virtfoundryv1alpha1.SSHKey{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: inst.Namespace, Name: name}, ssh); err != nil {
			return nil, fmt.Errorf("spec.sshKeyRefs[%d]: sshkey %q: %w", i, name, err)
		}
		pub := strings.TrimSpace(ssh.Spec.PublicKey)
		if pub == "" {
			return nil, fmt.Errorf("spec.sshKeyRefs[%d]: sshkey %q has empty spec.publicKey", i, name)
		}
		keys = append(keys, pub)
	}
	return keys, nil
}
