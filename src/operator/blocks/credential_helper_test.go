package blocks_test

import (
	"context"
	"strings"
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"
	blocks "github.com/baiyuqing/ottoplus/src/operator/blocks"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolveCredential_Success(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "db-creds", Namespace: "default"},
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("s3cret"),
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	credRef := block.CredentialRef{
		SecretName:      "db-creds",
		SecretNamespace: "default",
		UsernameKey:     "username",
		PasswordKey:     "password",
	}
	credJSON, _ := credRef.Encode()

	resolved, err := blocks.ResolveCredential(context.Background(), c, credJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Username != "admin" {
		t.Errorf("username: got %q, want %q", resolved.Username, "admin")
	}
	if resolved.Password != "s3cret" {
		t.Errorf("password: got %q, want %q", resolved.Password, "s3cret")
	}
}

func TestResolveCredential_SecretNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	credRef := block.CredentialRef{
		SecretName:      "nonexistent",
		SecretNamespace: "default",
		UsernameKey:     "username",
		PasswordKey:     "password",
	}
	credJSON, _ := credRef.Encode()

	_, err := blocks.ResolveCredential(context.Background(), c, credJSON)
	if err == nil {
		t.Fatal("expected error for missing Secret")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestResolveCredential_MissingUsernameKey(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "db-creds", Namespace: "default"},
		Data: map[string][]byte{
			"password": []byte("s3cret"),
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	credRef := block.CredentialRef{
		SecretName:      "db-creds",
		SecretNamespace: "default",
		UsernameKey:     "username",
		PasswordKey:     "password",
	}
	credJSON, _ := credRef.Encode()

	_, err := blocks.ResolveCredential(context.Background(), c, credJSON)
	if err == nil {
		t.Fatal("expected error for missing username key")
	}
	if !strings.Contains(err.Error(), "missing key") {
		t.Errorf("expected 'missing key' in error, got: %v", err)
	}
}

func TestResolveCredential_MissingPasswordKey(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "db-creds", Namespace: "default"},
		Data: map[string][]byte{
			"username": []byte("admin"),
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	credRef := block.CredentialRef{
		SecretName:      "db-creds",
		SecretNamespace: "default",
		UsernameKey:     "username",
		PasswordKey:     "password",
	}
	credJSON, _ := credRef.Encode()

	_, err := blocks.ResolveCredential(context.Background(), c, credJSON)
	if err == nil {
		t.Fatal("expected error for missing password key")
	}
	if !strings.Contains(err.Error(), "missing key") {
		t.Errorf("expected 'missing key' in error, got: %v", err)
	}
}

func TestResolveCredential_EmptyValue(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "db-creds", Namespace: "default"},
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte(""),
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	credRef := block.CredentialRef{
		SecretName:      "db-creds",
		SecretNamespace: "default",
		UsernameKey:     "username",
		PasswordKey:     "password",
	}
	credJSON, _ := credRef.Encode()

	_, err := blocks.ResolveCredential(context.Background(), c, credJSON)
	if err == nil {
		t.Fatal("expected error for empty password value")
	}
	if !strings.Contains(err.Error(), "missing key") {
		t.Errorf("expected 'missing key' in error, got: %v", err)
	}
}
