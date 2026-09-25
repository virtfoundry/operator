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

func TestValidateContainerDiskImage_AllowsDefaults(t *testing.T) {
	t.Parallel()
	for _, img := range []string{
		"quay.io/kubevirt/cirros-container-disk-demo",
		"quay.io/containerdisks/ubuntu:22.04",
		"quay.io/containerdisks/fedora:40",
		defaultContainerImg,
	} {
		if err := validateContainerDiskImage(img, nil); err != nil {
			t.Fatalf("%q: %v", img, err)
		}
	}
}

func TestValidateContainerDiskImage_RejectsUnlisted(t *testing.T) {
	t.Parallel()
	for _, img := range []string{
		"evil.example.com/pwn:latest",
		"docker.io/library/nginx:latest",
		"ghcr.io/attacker/malware:1",
		"http://evil.example.com/disk.qcow2",
		"https://mirror.example.com/ubuntu.iso",
		"",
		"   ",
	} {
		if err := validateContainerDiskImage(img, nil); err == nil {
			t.Fatalf("expected reject for %q", img)
		}
	}
}

func TestValidateContainerDiskImage_CustomAllowlist(t *testing.T) {
	t.Parallel()
	allowed := []string{"registry.homelab/vf/", "quay.io/containerdisks/"}
	if err := validateContainerDiskImage("registry.homelab/vf/cirros:1", allowed); err != nil {
		t.Fatal(err)
	}
	if err := validateContainerDiskImage("quay.io/kubevirt/cirros-container-disk-demo", allowed); err == nil {
		t.Fatal("expected reject when kubevirt prefix not in custom list")
	}
}

func TestParseImageAllowlistEnv(t *testing.T) {
	t.Parallel()
	got := parseImageAllowlist(" quay.io/a/ , ,quay.io/b/ ")
	want := []string{"quay.io/a/", "quay.io/b/"}
	if len(got) != len(want) {
		t.Fatalf("got %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

func TestResolveVMBuildInput_RejectsDisallowedTemplateImage(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)
	tmpl := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{Name: "evil", Namespace: "virtfoundry-system"},
		Spec: virtfoundryv1alpha1.TemplateSpec{
			Image:      "evil.example.com/pwn:latest",
			SourceType: "container",
			OSType:     "linux",
		},
	}
	r := &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(tmpl).Build(),
	}
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vm1",
			Namespace: testTenantNS,
			Annotations: map[string]string{
				annotationAllowPodNetwork: annotationTruthy,
			},
		},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			TemplateRef: &virtfoundryv1alpha1.LocalObjectRef{Name: "evil"},
		},
	}
	_, err := r.resolveVMBuildInput(context.Background(), inst)
	if err == nil || !strings.Contains(err.Error(), "allowlist") {
		t.Fatalf("expected allowlist error, got %v", err)
	}
}

func TestResolveVMBuildInput_AllowsCatalogImage(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)
	tmpl := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{Name: "ubuntu-2204", Namespace: "virtfoundry-system"},
		Spec: virtfoundryv1alpha1.TemplateSpec{
			Image:      "quay.io/containerdisks/ubuntu:22.04",
			SourceType: "container",
			OSType:     "linux",
		},
	}
	r := &InstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(tmpl).Build(),
	}
	inst := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vm1",
			Namespace: testTenantNS,
			Annotations: map[string]string{
				annotationAllowPodNetwork: annotationTruthy,
			},
		},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			TemplateRef: &virtfoundryv1alpha1.LocalObjectRef{Name: "ubuntu-2204"},
		},
	}
	in, err := r.resolveVMBuildInput(context.Background(), inst)
	if err != nil {
		t.Fatal(err)
	}
	if in.image != "quay.io/containerdisks/ubuntu:22.04" {
		t.Fatalf("image=%q", in.image)
	}
}
