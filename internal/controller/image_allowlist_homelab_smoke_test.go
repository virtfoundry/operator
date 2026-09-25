//go:build homelab_smoke

package controller

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	virtfoundryv1alpha1 "github.com/virtfoundry/operator/api/v1alpha1"
)

func TestHomelab_ImageAllowlistAgainstLiveTemplates(t *testing.T) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(os.Getenv("HOME"), "Documents/homelab/kubespray/inventory/homelab-cluster/artifacts/admin.conf")
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatal(err)
	}
	scheme := runtime.NewScheme()
	_ = virtfoundryv1alpha1.AddToScheme(scheme)
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	r := &InstanceReconciler{Client: c}
	ctx := context.Background()

	// Positive: catalog ubuntu
	instOK := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allowlist-smoke-ok",
			Namespace: "virtfoundry-tenant-default",
			Annotations: map[string]string{annotationAllowPodNetwork: annotationTruthy},
		},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			TemplateRef: &virtfoundryv1alpha1.LocalObjectRef{Name: "ubuntu-2204"},
		},
	}
	in, err := r.resolveVMBuildInput(ctx, instOK)
	if err != nil {
		t.Fatalf("catalog image should resolve: %v", err)
	}
	if !strings.HasPrefix(in.image, "quay.io/containerdisks/") {
		t.Fatalf("unexpected image %q", in.image)
	}

	// Negative: create ephemeral evil Template then resolve
	evil := &virtfoundryv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sec22-evil-smoke",
			Namespace: operatorNamespace,
		},
		Spec: virtfoundryv1alpha1.TemplateSpec{
			Image:      "evil.example.com/pwn:latest",
			SourceType: "container",
			OSType:     osTypeLinux,
		},
	}
	_ = c.Delete(ctx, evil)
	if err := c.Create(ctx, evil); err != nil {
		t.Fatalf("create evil template: %v", err)
	}
	t.Cleanup(func() { _ = c.Delete(context.Background(), evil) })

	instBad := &virtfoundryv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allowlist-smoke-bad",
			Namespace: "virtfoundry-tenant-default",
			Annotations: map[string]string{annotationAllowPodNetwork: annotationTruthy},
		},
		Spec: virtfoundryv1alpha1.InstanceSpec{
			TemplateRef: &virtfoundryv1alpha1.LocalObjectRef{Name: "sec22-evil-smoke"},
		},
	}
	_, err = r.resolveVMBuildInput(ctx, instBad)
	if err == nil || !strings.Contains(err.Error(), "allowlist") {
		t.Fatalf("expected allowlist reject, got %v", err)
	}
}
