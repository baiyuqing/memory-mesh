package passwordrotation

import (
	"context"
	"strings"
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"
	blocks "github.com/baiyuqing/ottoplus/src/operator/blocks"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcile_OutputsCredential(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	b := &Block{}

	result, err := b.Reconcile(context.Background(), c, blocks.ReconcileRequest{
		ClusterName:      "test-cluster",
		ClusterNamespace: "default",
		BlockRef: block.BlockRef{
			Kind: "security.password-rotation",
			Name: "rotator",
			Parameters: map[string]string{
				"rotationSchedule": "0 0 */7 * *",
				"passwordLength":   "32",
			},
		},
		ResolvedInputs: map[string]string{
			"dsn": "postgresql://postgres@test-cluster-db.default.svc:5432/postgres",
		},
	})
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// Verify credential output exists and is decodable.
	credJSON, ok := result.Outputs["credential"]
	if !ok || credJSON == "" {
		t.Fatal("expected credential output to be present")
	}

	cred, err := block.DecodeCredentialRef(credJSON)
	if err != nil {
		t.Fatalf("credential decode failed: %v", err)
	}

	expectedSecretName := "test-cluster-rotator-creds"
	if cred.SecretName != expectedSecretName {
		t.Errorf("credential.secretName: got %q, want %q", cred.SecretName, expectedSecretName)
	}
	if cred.SecretNamespace != "default" {
		t.Errorf("credential.secretNamespace: got %q, want %q", cred.SecretNamespace, "default")
	}
	if cred.UsernameKey != "username" {
		t.Errorf("credential.usernameKey: got %q, want %q", cred.UsernameKey, "username")
	}
	if cred.PasswordKey != "password" {
		t.Errorf("credential.passwordKey: got %q, want %q", cred.PasswordKey, "password")
	}
	if cred.DevOnly {
		t.Error("credential.devOnly: expected false for password-rotation (real rotation)")
	}

	// Verify the Secret was actually created in the fake cluster.
	var secret corev1.Secret
	err = c.Get(context.Background(), types.NamespacedName{
		Name:      expectedSecretName,
		Namespace: "default",
	}, &secret)
	if err != nil {
		t.Fatalf("expected credential Secret to exist: %v", err)
	}
	if string(secret.Data["username"]) != "dbadmin" {
		t.Errorf("secret username: got %q, want %q", string(secret.Data["username"]), "dbadmin")
	}
	if string(secret.Data["password"]) == "" {
		t.Error("expected secret password to be non-empty")
	}
}

func TestReconcile_CustomSecretName(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	b := &Block{}

	result, err := b.Reconcile(context.Background(), c, blocks.ReconcileRequest{
		ClusterName:      "test-cluster",
		ClusterNamespace: "default",
		BlockRef: block.BlockRef{
			Kind: "security.password-rotation",
			Name: "rotator",
			Parameters: map[string]string{
				"secretName": "my-custom-secret",
			},
		},
		ResolvedInputs: map[string]string{},
	})
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	cred, err := block.DecodeCredentialRef(result.Outputs["credential"])
	if err != nil {
		t.Fatalf("credential decode failed: %v", err)
	}
	if cred.SecretName != "my-custom-secret" {
		t.Errorf("credential.secretName: got %q, want %q", cred.SecretName, "my-custom-secret")
	}
}

func TestReconcile_StubMessageIsHonest(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	b := &Block{}

	result, err := b.Reconcile(context.Background(), c, blocks.ReconcileRequest{
		ClusterName:      "test-cluster",
		ClusterNamespace: "default",
		BlockRef: block.BlockRef{
			Kind: "security.password-rotation",
			Name: "rotator",
		},
		ResolvedInputs: map[string]string{},
	})
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// Reconcile message must not claim rotation is fully functional.
	if !strings.Contains(result.Message, "stub") {
		t.Errorf("reconcile message should mention stub status, got: %s", result.Message)
	}

	// Verify the rotation script in the ConfigMap is honest.
	var cm corev1.ConfigMap
	err = c.Get(context.Background(), types.NamespacedName{
		Name:      "test-cluster-rotator-config",
		Namespace: "default",
	}, &cm)
	if err != nil {
		t.Fatalf("expected ConfigMap to exist: %v", err)
	}
	script := cm.Data["rotate.sh"]
	if strings.Contains(script, "Password rotation complete") {
		t.Error("rotation script should not claim 'Password rotation complete' — it is a stub")
	}
	if strings.Contains(script, "TODO") {
		t.Error("rotation script should not contain TODO comments")
	}
	if !strings.Contains(script, "stub") {
		t.Error("rotation script should mention stub status")
	}
}

func TestDescriptor_HasCredentialPort(t *testing.T) {
	b := &Block{}
	desc := b.Descriptor()

	found := false
	for _, p := range desc.Ports {
		if p.Name == "credential" && p.PortType == "credential" && p.Direction == block.PortOutput {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Descriptor to have a credential output port")
	}
}
