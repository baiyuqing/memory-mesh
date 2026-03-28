package blocks

import (
	"context"
	"fmt"

	"github.com/baiyuqing/ottoplus/src/core/block"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResolvedCredential holds the username and password extracted from a
// credential reference's backing Secret.
type ResolvedCredential struct {
	Username string
	Password string
}

// ResolveCredential decodes a credential JSON string, reads the
// referenced Kubernetes Secret, and validates that the expected keys
// exist and are non-empty. This is the single entry point for any
// block that consumes credentials — it replaces the decode → read →
// validate pattern that would otherwise be duplicated in each consumer.
func ResolveCredential(ctx context.Context, c client.Client, credJSON string) (*ResolvedCredential, error) {
	credRef, err := block.DecodeCredentialRef(credJSON)
	if err != nil {
		return nil, fmt.Errorf("credential decode failed: %w", err)
	}

	var secret corev1.Secret
	if err := c.Get(ctx, types.NamespacedName{Name: credRef.SecretName, Namespace: credRef.SecretNamespace}, &secret); err != nil {
		return nil, fmt.Errorf("credential Secret %q in namespace %q not found: %w", credRef.SecretName, credRef.SecretNamespace, err)
	}

	usernameBytes, ok := secret.Data[credRef.UsernameKey]
	if !ok || len(usernameBytes) == 0 {
		return nil, fmt.Errorf("credential Secret %q missing key %q", credRef.SecretName, credRef.UsernameKey)
	}

	passwordBytes, ok := secret.Data[credRef.PasswordKey]
	if !ok || len(passwordBytes) == 0 {
		return nil, fmt.Errorf("credential Secret %q missing key %q", credRef.SecretName, credRef.PasswordKey)
	}

	return &ResolvedCredential{
		Username: string(usernameBytes),
		Password: string(passwordBytes),
	}, nil
}
