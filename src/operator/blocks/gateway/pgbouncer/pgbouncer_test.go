package pgbouncer

import (
	"context"
	"strings"
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"
	blocks "github.com/baiyuqing/ottoplus/src/operator/blocks"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcile_WithCredential(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	// Pre-create the upstream credential Secret that CredentialRef points to.
	upstreamSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-pg-credentials",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"username": []byte("pguser"),
			"password": []byte("pgpass123"),
		},
	}

	credRef := block.CredentialRef{
		SecretName:      "test-cluster-pg-credentials",
		SecretNamespace: "default",
		UsernameKey:     "username",
		PasswordKey:     "password",
		DevOnly:         true,
	}
	credJSON, _ := credRef.Encode()

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(upstreamSecret).Build()
	b := &Block{}

	result, err := b.Reconcile(context.Background(), c, blocks.ReconcileRequest{
		ClusterName:      "test-cluster",
		ClusterNamespace: "default",
		BlockRef: block.BlockRef{
			Kind: "gateway.pgbouncer",
			Name: "bouncer",
		},
		ResolvedInputs: map[string]string{
			"upstream-dsn":        "postgresql://postgres@test-cluster-pg.default.svc:5432/postgres",
			"upstream-credential": credJSON,
		},
	})
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.Phase != block.PhaseReady {
		t.Fatalf("expected PhaseReady, got %v", result.Phase)
	}

	// 1. Verify userlist Secret was created with correct content.
	var userlistSecret corev1.Secret
	err = c.Get(context.Background(), types.NamespacedName{
		Name:      "test-cluster-bouncer-userlist",
		Namespace: "default",
	}, &userlistSecret)
	if err != nil {
		t.Fatalf("expected userlist Secret to exist: %v", err)
	}
	userlist := string(userlistSecret.Data["userlist.txt"])
	if !strings.Contains(userlist, `"pguser"`) || !strings.Contains(userlist, `"pgpass123"`) {
		t.Errorf("userlist content unexpected: %q", userlist)
	}

	// 2. Verify ConfigMap uses auth_type=md5, not auth_type=any.
	var cm corev1.ConfigMap
	err = c.Get(context.Background(), types.NamespacedName{
		Name:      "test-cluster-bouncer-config",
		Namespace: "default",
	}, &cm)
	if err != nil {
		t.Fatalf("expected ConfigMap to exist: %v", err)
	}
	ini := cm.Data["pgbouncer.ini"]
	if !strings.Contains(ini, "auth_type = plain") {
		t.Errorf("expected auth_type = plain in pgbouncer.ini, got:\n%s", ini)
	}
	if strings.Contains(ini, "auth_type = any") {
		t.Error("expected auth_type = any to be replaced when credential is provided")
	}
	if !strings.Contains(ini, "auth_file = /etc/pgbouncer/userlist.txt") {
		t.Errorf("expected auth_file directive in pgbouncer.ini, got:\n%s", ini)
	}

	// 3. Verify Deployment has userlist volume mount.
	var deploy appsv1.Deployment
	err = c.Get(context.Background(), types.NamespacedName{
		Name:      "test-cluster-bouncer",
		Namespace: "default",
	}, &deploy)
	if err != nil {
		t.Fatalf("expected Deployment to exist: %v", err)
	}
	foundVolume := false
	for _, v := range deploy.Spec.Template.Spec.Volumes {
		if v.Name == "userlist" && v.Secret != nil && v.Secret.SecretName == "test-cluster-bouncer-userlist" {
			foundVolume = true
			break
		}
	}
	if !foundVolume {
		t.Error("expected Deployment to have userlist volume")
	}
	foundMount := false
	for _, vm := range deploy.Spec.Template.Spec.Containers[0].VolumeMounts {
		if vm.Name == "userlist" && vm.MountPath == "/etc/pgbouncer" {
			foundMount = true
			break
		}
	}
	if !foundMount {
		t.Error("expected Deployment container to have userlist volume mount")
	}
}

func TestReconcile_WithoutCredential_Fallback(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	b := &Block{}

	result, err := b.Reconcile(context.Background(), c, blocks.ReconcileRequest{
		ClusterName:      "test-cluster",
		ClusterNamespace: "default",
		BlockRef: block.BlockRef{
			Kind: "gateway.pgbouncer",
			Name: "bouncer",
		},
		ResolvedInputs: map[string]string{
			"upstream-dsn": "postgresql://postgres@test-cluster-pg.default.svc:5432/postgres",
		},
	})
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.Phase != block.PhaseReady {
		t.Fatalf("expected PhaseReady, got %v", result.Phase)
	}

	// Verify ConfigMap uses auth_type=any (DEV-ONLY fallback).
	var cm corev1.ConfigMap
	err = c.Get(context.Background(), types.NamespacedName{
		Name:      "test-cluster-bouncer-config",
		Namespace: "default",
	}, &cm)
	if err != nil {
		t.Fatalf("expected ConfigMap to exist: %v", err)
	}
	ini := cm.Data["pgbouncer.ini"]
	if !strings.Contains(ini, "auth_type = any") {
		t.Errorf("expected auth_type = any in fallback mode, got:\n%s", ini)
	}
	if strings.Contains(ini, "auth_file") {
		t.Error("expected no auth_file directive in fallback mode")
	}

	// Verify no userlist Secret was created.
	var secret corev1.Secret
	err = c.Get(context.Background(), types.NamespacedName{
		Name:      "test-cluster-bouncer-userlist",
		Namespace: "default",
	}, &secret)
	if err == nil {
		t.Error("expected no userlist Secret in fallback mode")
	}

	// Verify Deployment does NOT have userlist volume.
	var deploy appsv1.Deployment
	err = c.Get(context.Background(), types.NamespacedName{
		Name:      "test-cluster-bouncer",
		Namespace: "default",
	}, &deploy)
	if err != nil {
		t.Fatalf("expected Deployment to exist: %v", err)
	}
	for _, v := range deploy.Spec.Template.Spec.Volumes {
		if v.Name == "userlist" {
			t.Error("expected Deployment to NOT have userlist volume in fallback mode")
		}
	}

	// Verify dsn output is still present.
	if result.Outputs["dsn"] == "" {
		t.Error("expected dsn output in fallback mode")
	}
}

func TestReconcile_WithCredential_MissingKey(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	// Secret exists but is missing the "password" key.
	upstreamSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-pg-credentials",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"username": []byte("pguser"),
		},
	}

	credRef := block.CredentialRef{
		SecretName:      "test-cluster-pg-credentials",
		SecretNamespace: "default",
		UsernameKey:     "username",
		PasswordKey:     "password",
	}
	credJSON, _ := credRef.Encode()

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(upstreamSecret).Build()
	b := &Block{}

	result, err := b.Reconcile(context.Background(), c, blocks.ReconcileRequest{
		ClusterName:      "test-cluster",
		ClusterNamespace: "default",
		BlockRef: block.BlockRef{
			Kind: "gateway.pgbouncer",
			Name: "bouncer",
		},
		ResolvedInputs: map[string]string{
			"upstream-dsn":        "postgresql://postgres@test-cluster-pg.default.svc:5432/postgres",
			"upstream-credential": credJSON,
		},
	})
	if err == nil {
		t.Fatal("expected Reconcile to fail when credential Secret is missing a key")
	}
	if result.Phase != block.PhaseFailed {
		t.Errorf("expected PhaseFailed, got %v", result.Phase)
	}
	if !strings.Contains(result.Message, "missing key") {
		t.Errorf("expected error message about missing key, got: %s", result.Message)
	}
}

func TestDescriptor_HasUpstreamCredentialPort(t *testing.T) {
	b := &Block{}
	desc := b.Descriptor()

	found := false
	for _, p := range desc.Ports {
		if p.Name == "upstream-credential" && p.PortType == "credential" && p.Direction == block.PortInput {
			found = true
			if p.Required {
				t.Error("upstream-credential should not be required")
			}
			break
		}
	}
	if !found {
		t.Error("expected Descriptor to have an upstream-credential input port")
	}
}
