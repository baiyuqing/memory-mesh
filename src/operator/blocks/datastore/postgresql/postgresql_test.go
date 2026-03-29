package postgresql

import (
	"context"
	"strings"
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"
	"github.com/baiyuqing/ottoplus/src/core/testfixture"
	blocks "github.com/baiyuqing/ottoplus/src/operator/blocks"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcile_OutputsCredential(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	b := &Block{}

	result, err := b.Reconcile(context.Background(), c, blocks.ReconcileRequest{
		ClusterName:      "test-cluster",
		ClusterNamespace: "default",
		BlockRef: block.BlockRef{
			Kind: "datastore.postgresql",
			Name: "db",
			Parameters: map[string]string{
				"version":  "16",
				"replicas": "1",
			},
		},
		ResolvedInputs: map[string]string{},
	})
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// Verify dsn is still present (backward compat).
	if result.Outputs["dsn"] == "" {
		t.Error("expected dsn output to be present")
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

	expectedSecretName := "test-cluster-db-credentials"
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
	if !cred.DevOnly {
		t.Error("credential.devOnly: expected true for dev-only trust auth")
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
	if string(secret.Data["username"]) != "postgres" {
		t.Errorf("secret username: got %q, want %q", string(secret.Data["username"]), "postgres")
	}
	if string(secret.Data["password"]) != "postgres-dev" {
		t.Errorf("secret password: got %q, want %q", string(secret.Data["password"]), "postgres-dev")
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

func TestReconcile_DevOnlyDSN(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	b := &Block{}

	result, err := b.Reconcile(context.Background(), c, blocks.ReconcileRequest{
		ClusterName:      "test-cluster",
		ClusterNamespace: "default",
		BlockRef: block.BlockRef{
			Kind: "datastore.postgresql",
			Name: "db",
		},
		ResolvedInputs: map[string]string{},
	})
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// DSN must be present (backward compat) and use trust auth (no password in URL).
	dsn := result.Outputs["dsn"]
	if dsn == "" {
		t.Fatal("expected dsn output to be present")
	}
	if strings.Contains(dsn, ":") && strings.Contains(dsn, "@") {
		// Check there is no password between :// user part and @
		// Format: postgresql://postgres@host — no colon-password before @
		userPart := strings.SplitN(strings.TrimPrefix(dsn, "postgresql://"), "@", 2)[0]
		if strings.Contains(userPart, ":") {
			t.Errorf("dsn should not contain a password (dev-only trust auth), got: %s", dsn)
		}
	}

	// Reconcile message must mention dev-only status with specific keywords.
	for _, keyword := range testfixture.PostgreSQLDevOnlyKeywords {
		if !strings.Contains(result.Message, keyword) {
			t.Errorf("reconcile message missing %q keyword, got: %s", keyword, result.Message)
		}
	}

	// Credential must be marked DevOnly.
	cred, err := block.DecodeCredentialRef(result.Outputs["credential"])
	if err != nil {
		t.Fatalf("credential decode failed: %v", err)
	}
	if !cred.DevOnly {
		t.Error("credential.devOnly: expected true for dev-only trust auth")
	}
}

func TestReconcile_TrustAuthEnv(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	b := &Block{}

	_, err := b.Reconcile(context.Background(), c, blocks.ReconcileRequest{
		ClusterName:      "test-cluster",
		ClusterNamespace: "default",
		BlockRef: block.BlockRef{
			Kind: "datastore.postgresql",
			Name: "db",
		},
		ResolvedInputs: map[string]string{},
	})
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// Verify the StatefulSet uses trust auth.
	var sts appsv1.StatefulSet
	err = c.Get(context.Background(), types.NamespacedName{
		Name:      "test-cluster-db",
		Namespace: "default",
	}, &sts)
	if err != nil {
		t.Fatalf("expected StatefulSet to exist: %v", err)
	}

	containers := sts.Spec.Template.Spec.Containers
	if len(containers) == 0 {
		t.Fatal("expected at least one container")
	}

	foundTrustAuth := false
	for _, env := range containers[0].Env {
		if env.Name == "POSTGRES_HOST_AUTH_METHOD" && env.Value == "trust" {
			foundTrustAuth = true
			break
		}
	}
	if !foundTrustAuth {
		t.Error("expected POSTGRES_HOST_AUTH_METHOD=trust env var (dev-only trust auth)")
	}
}
