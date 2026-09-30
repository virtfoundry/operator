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
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

const (
	testSSHKeyName   = "laptop"
	testSSHPublicKey = "ssh-ed25519 AAAA laptop"
	testTemplateName = "ubuntu"
)

func TestMergeCloudInitWithSSHKeys(t *testing.T) {
	got, err := mergeCloudInitWithSSHKeys("#cloud-config\npackages:\n  - curl\n", []string{
		"ssh-ed25519 AAAA demo@vf",
		"  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "ssh_authorized_keys:") {
		t.Fatalf("missing ssh_authorized_keys: %s", got)
	}
	if !strings.Contains(got, "ssh-ed25519 AAAA demo@vf") {
		t.Fatalf("missing public key: %s", got)
	}
	if !strings.Contains(got, "packages:") {
		t.Fatalf("expected template fragment preserved: %s", got)
	}
	if !strings.Contains(got, "lock_passwd: true") {
		t.Fatalf("expected password locked: %s", got)
	}
}

func TestMergeCloudInitWithSSHKeys_RejectsEmpty(t *testing.T) {
	if _, err := mergeCloudInitWithSSHKeys("", nil); err == nil {
		t.Fatal("expected error for empty keys")
	}
	if _, err := mergeCloudInitWithSSHKeys("", []string{"  "}); err == nil {
		t.Fatal("expected error for whitespace-only keys")
	}
}

func TestResolveSSHPublicKeys(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)

	key := &virtfoundryv1alpha1.SSHKey{
		ObjectMeta: metav1.ObjectMeta{Name: testSSHKeyName, Namespace: testTenantNS},
		Spec:       virtfoundryv1alpha1.SSHKeySpec{PublicKey: testSSHPublicKey},
	}
	r := &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(key).Build(),
	}
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: testVMName, Namespace: testTenantNS},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			SSHKeyRefs: []virtfoundryv1alpha1.LocalObjectRef{{Name: testSSHKeyName}},
		},
	}
	got, err := r.resolveSSHPublicKeys(context.Background(), inst)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != testSSHPublicKey {
		t.Fatalf("got %#v", got)
	}
}

func TestResolveSSHPublicKeys_MissingCR(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)
	r := &InstanceReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: testVMName, Namespace: testTenantNS},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			SSHKeyRefs: []virtfoundryv1alpha1.LocalObjectRef{{Name: "missing"}},
		},
	}
	if _, err := r.resolveSSHPublicKeys(context.Background(), inst); err == nil {
		t.Fatal("expected error for missing SSHKey")
	}
}

func TestResolveVMBuildInput_MergesSSHKeyRefs(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)

	tmpl := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{Name: testTemplateName, Namespace: testTenantNS},
		Spec: virtfoundryv1alpha1.TemplateSpec{
			Image:             catalogUbuntuImage,
			OSType:            osTypeLinux,
			SourceType:        sourceTypeContainer,
			CloudInitUserData: testTemplateCloudInit,
		},
	}
	key := &virtfoundryv1alpha1.SSHKey{
		ObjectMeta: metav1.ObjectMeta{Name: testSSHKeyName, Namespace: testTenantNS},
		Spec:       virtfoundryv1alpha1.SSHKeySpec{PublicKey: testSSHPublicKey},
	}
	r := &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(tmpl, key).Build(),
	}
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testVMName,
			Namespace: testTenantNS,
			Annotations: map[string]string{
				annotationAllowPodNetwork: annotationTruthy,
			},
		},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			DisplayName: testVMName,
			TemplateRef: &virtfoundryv1alpha1.LocalObjectRef{Name: testTemplateName},
			SSHKeyRefs:  []virtfoundryv1alpha1.LocalObjectRef{{Name: testSSHKeyName}},
		},
	}

	in, err := r.resolveVMBuildInput(context.Background(), inst)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(in.cloudInit, testSSHPublicKey) {
		t.Fatalf("cloud-init missing key: %s", in.cloudInit)
	}
	if !strings.Contains(in.cloudInit, "timezone: UTC") {
		t.Fatalf("cloud-init missing template fragment: %s", in.cloudInit)
	}
}

const (
	testTemplateCloudInit = "#cloud-config\ntimezone: UTC\n"
	testInstanceCloudInit = "#cloud-config\npackages:\n  - nginx\n"
)

func newCloudInitPrecedenceFixtures(t *testing.T) *InstanceReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)

	tmpl := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{Name: testTemplateName, Namespace: testTenantNS},
		Spec: virtfoundryv1alpha1.TemplateSpec{
			Image:             catalogUbuntuImage,
			OSType:            osTypeLinux,
			SourceType:        sourceTypeContainer,
			CloudInitUserData: testTemplateCloudInit,
		},
	}
	key := &virtfoundryv1alpha1.SSHKey{
		ObjectMeta: metav1.ObjectMeta{Name: testSSHKeyName, Namespace: testTenantNS},
		Spec:       virtfoundryv1alpha1.SSHKeySpec{PublicKey: testSSHPublicKey},
	}
	return &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(tmpl, key).Build(),
	}
}

func TestResolveVMBuildInput_InstanceCloudInitOverridesTemplate(t *testing.T) {
	r := newCloudInitPrecedenceFixtures(t)
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testVMName,
			Namespace: testTenantNS,
			Annotations: map[string]string{
				annotationAllowPodNetwork: annotationTruthy,
			},
		},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			DisplayName:       testVMName,
			TemplateRef:       &virtfoundryv1alpha1.LocalObjectRef{Name: testTemplateName},
			CloudInitUserData: testInstanceCloudInit,
		},
	}

	in, err := r.resolveVMBuildInput(context.Background(), inst)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(in.cloudInit, "nginx") {
		t.Fatalf("expected Instance userdata, got: %s", in.cloudInit)
	}
	if strings.Contains(in.cloudInit, "timezone: UTC") {
		t.Fatalf("Template userdata should not win when Instance sets cloudInitUserData: %s", in.cloudInit)
	}
}

func TestResolveVMBuildInput_EmptyInstanceCloudInitFallsBackToTemplate(t *testing.T) {
	r := newCloudInitPrecedenceFixtures(t)
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testVMName,
			Namespace: testTenantNS,
			Annotations: map[string]string{
				annotationAllowPodNetwork: annotationTruthy,
			},
		},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			DisplayName:       testVMName,
			TemplateRef:       &virtfoundryv1alpha1.LocalObjectRef{Name: testTemplateName},
			CloudInitUserData: "   ",
		},
	}

	in, err := r.resolveVMBuildInput(context.Background(), inst)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(in.cloudInit, "timezone: UTC") {
		t.Fatalf("expected Template userdata fallback, got: %s", in.cloudInit)
	}
	if strings.Contains(in.cloudInit, "nginx") {
		t.Fatalf("unexpected Instance userdata: %s", in.cloudInit)
	}
}

func TestResolveVMBuildInput_InstanceCloudInitStillMergesSSHKeys(t *testing.T) {
	r := newCloudInitPrecedenceFixtures(t)
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testVMName,
			Namespace: testTenantNS,
			Annotations: map[string]string{
				annotationAllowPodNetwork: annotationTruthy,
			},
		},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			DisplayName:       testVMName,
			TemplateRef:       &virtfoundryv1alpha1.LocalObjectRef{Name: testTemplateName},
			CloudInitUserData: testInstanceCloudInit,
			SSHKeyRefs:        []virtfoundryv1alpha1.LocalObjectRef{{Name: testSSHKeyName}},
		},
	}

	in, err := r.resolveVMBuildInput(context.Background(), inst)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(in.cloudInit, "nginx") {
		t.Fatalf("expected Instance userdata base, got: %s", in.cloudInit)
	}
	if !strings.Contains(in.cloudInit, testSSHPublicKey) {
		t.Fatalf("cloud-init missing merged SSH key: %s", in.cloudInit)
	}
	if strings.Contains(in.cloudInit, "timezone: UTC") {
		t.Fatalf("Template userdata should not appear after Instance override: %s", in.cloudInit)
	}
}
