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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

const (
	testCloudInitSecretName = "guest-cloud-init"
	testCloudInitPassword   = "super-secret-guest-password"
	testCloudInitFromSecret = "#cloud-config\nchpasswd:\n  list: |\n    ubuntu:" + testCloudInitPassword + "\n"
)

func TestResolveCloudInitUserData_TemplateSecretPrefersOverLegacy(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testCloudInitSecretName, Namespace: testTenantNS},
		Data:       map[string][]byte{cloudInitSecretKeyDefault: []byte(testCloudInitFromSecret)},
	}
	tmpl := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{Name: testTemplateName, Namespace: testTenantNS},
		Spec: virtfoundryv1alpha1.TemplateSpec{
			Image:             catalogUbuntuImage,
			SourceType:        sourceTypeContainer,
			CloudInitUserData: testTemplateCloudInit,
			CloudInitSecretRef: &virtfoundryv1alpha1.SecretKeyRef{
				Name: testCloudInitSecretName,
			},
		},
	}
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: testVMName, Namespace: testTenantNS},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			TemplateRef: &virtfoundryv1alpha1.LocalObjectRef{Name: testTemplateName},
		},
	}
	r := &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(sec, tmpl).Build(),
	}

	got, err := r.resolveCloudInitUserData(context.Background(), inst, tmpl)
	if err != nil {
		t.Fatal(err)
	}
	if got != testCloudInitFromSecret {
		t.Fatalf("expected secret userdata, got %q", got)
	}
	if strings.Contains(got, "timezone: UTC") {
		t.Fatal("legacy Template string should not win when secretRef is set")
	}
}

func TestResolveCloudInitUserData_InstanceSecretOverridesTemplate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	instSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-ci", Namespace: testTenantNS},
		Data:       map[string][]byte{cloudInitSecretKeyDefault: []byte("#cloud-config\npackages:\n  - nginx\n")},
	}
	tmplSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testCloudInitSecretName, Namespace: testTenantNS},
		Data:       map[string][]byte{cloudInitSecretKeyDefault: []byte(testCloudInitFromSecret)},
	}
	tmpl := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{Name: testTemplateName, Namespace: testTenantNS},
		Spec: virtfoundryv1alpha1.TemplateSpec{
			Image:      catalogUbuntuImage,
			SourceType: sourceTypeContainer,
			CloudInitSecretRef: &virtfoundryv1alpha1.SecretKeyRef{
				Name: testCloudInitSecretName,
			},
		},
	}
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: testVMName, Namespace: testTenantNS},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			TemplateRef: &virtfoundryv1alpha1.LocalObjectRef{Name: testTemplateName},
			CloudInitSecretRef: &virtfoundryv1alpha1.SecretKeyRef{
				Name: "inst-ci",
			},
		},
	}
	r := &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(instSecret, tmplSecret, tmpl).Build(),
	}

	got, err := r.resolveCloudInitUserData(context.Background(), inst, tmpl)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "nginx") {
		t.Fatalf("expected Instance secret, got %q", got)
	}
	if strings.Contains(got, testCloudInitPassword) {
		t.Fatal("Template secret must not leak when Instance secretRef wins")
	}
}

func TestResolveCloudInitUserData_LegacyStringStillWorks(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)

	tmpl := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{Name: testTemplateName, Namespace: testTenantNS},
		Spec: virtfoundryv1alpha1.TemplateSpec{
			Image:             catalogUbuntuImage,
			SourceType:        sourceTypeContainer,
			CloudInitUserData: testTemplateCloudInit,
		},
	}
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: testVMName, Namespace: testTenantNS},
		Spec:       virtfoundryv1alpha1.InstanceSpec{},
	}
	r := &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}

	got, err := r.resolveCloudInitUserData(context.Background(), inst, tmpl)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "timezone: UTC") {
		t.Fatalf("expected legacy Template userdata, got %q", got)
	}
}

func TestResolveCloudInitUserData_ErrorsOmitSecretBody(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testCloudInitSecretName, Namespace: testTenantNS},
		Data:       map[string][]byte{"other": []byte(testCloudInitFromSecret)},
	}
	tmpl := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{Name: testTemplateName, Namespace: testTenantNS},
		Spec: virtfoundryv1alpha1.TemplateSpec{
			CloudInitSecretRef: &virtfoundryv1alpha1.SecretKeyRef{Name: testCloudInitSecretName},
		},
	}
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: testVMName, Namespace: testTenantNS},
	}
	r := &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(sec).Build(),
	}

	_, err := r.resolveCloudInitUserData(context.Background(), inst, tmpl)
	if err == nil {
		t.Fatal("expected missing-key error")
	}
	if strings.Contains(err.Error(), testCloudInitPassword) {
		t.Fatalf("error must not dump secret body: %v", err)
	}
	if !strings.Contains(err.Error(), "missing key") {
		t.Fatalf("expected missing key message, got %v", err)
	}
}

func TestResolveVMBuildInput_UsesTemplateCloudInitSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testCloudInitSecretName, Namespace: testTenantNS},
		Data:       map[string][]byte{cloudInitSecretKeyDefault: []byte(testCloudInitFromSecret)},
	}
	tmpl := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{Name: testTemplateName, Namespace: testTenantNS},
		Spec: virtfoundryv1alpha1.TemplateSpec{
			Image:      catalogUbuntuImage,
			OSType:     osTypeLinux,
			SourceType: sourceTypeContainer,
			CloudInitSecretRef: &virtfoundryv1alpha1.SecretKeyRef{
				Name: testCloudInitSecretName,
			},
		},
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
			DisplayName: "secret-ci",
			TemplateRef: &virtfoundryv1alpha1.LocalObjectRef{Name: testTemplateName},
		},
	}
	r := &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(sec, tmpl).Build(),
	}

	in, err := r.resolveVMBuildInput(context.Background(), inst)
	if err != nil {
		t.Fatal(err)
	}
	if in.cloudInit != testCloudInitFromSecret {
		t.Fatalf("expected secret cloud-init on VM build input")
	}
}

func TestResolveCloudInitUserData_CustomKey(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testCloudInitSecretName, Namespace: testTenantNS},
		Data:       map[string][]byte{"userdata": []byte("#cloud-config\ntimezone: Brazil/East\n")},
	}
	tmpl := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{Name: testTemplateName, Namespace: testTenantNS},
		Spec: virtfoundryv1alpha1.TemplateSpec{
			CloudInitSecretRef: &virtfoundryv1alpha1.SecretKeyRef{
				Name: testCloudInitSecretName,
				Key:  "userdata",
			},
		},
	}
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: testVMName, Namespace: testTenantNS},
	}
	r := &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(sec).Build(),
	}

	got, err := r.resolveCloudInitUserData(context.Background(), inst, tmpl)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Brazil/East") {
		t.Fatalf("expected custom key payload, got %q", got)
	}
}
