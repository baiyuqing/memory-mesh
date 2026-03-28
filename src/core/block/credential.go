package block

import (
	"encoding/json"
	"fmt"
)

// CredentialRef is the unified representation of a credential reference.
// Blocks that produce credentials output this as JSON on their
// "credential" port. Blocks that consume credentials decode it to
// locate the Kubernetes Secret holding the actual values.
//
// This replaces ad-hoc JSON string construction in individual blocks.
type CredentialRef struct {
	// SecretName is the name of the Kubernetes Secret.
	SecretName string `json:"secretName"`
	// SecretNamespace is the namespace of the Secret.
	SecretNamespace string `json:"secretNamespace"`
	// UsernameKey is the key within the Secret's Data map that holds the username.
	UsernameKey string `json:"usernameKey"`
	// PasswordKey is the key within the Secret's Data map that holds the password.
	PasswordKey string `json:"passwordKey"`
	// DevOnly marks this credential as using an insecure dev-only default
	// (e.g. trust auth, hardcoded password). Consumers can check this flag
	// to warn or block in stricter environments.
	DevOnly bool `json:"devOnly,omitempty"`
}

// Encode serializes a CredentialRef to a JSON string suitable for
// passing through block output ports.
func (cr CredentialRef) Encode() (string, error) {
	b, err := json.Marshal(cr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DecodeCredentialRef parses a JSON string from a block output port
// back into a CredentialRef. It returns an error if any required field
// (secretName, secretNamespace, usernameKey, passwordKey) is missing.
func DecodeCredentialRef(s string) (CredentialRef, error) {
	var cr CredentialRef
	if err := json.Unmarshal([]byte(s), &cr); err != nil {
		return CredentialRef{}, err
	}
	if cr.SecretName == "" {
		return CredentialRef{}, fmt.Errorf("credential: secretName is required")
	}
	if cr.SecretNamespace == "" {
		return CredentialRef{}, fmt.Errorf("credential: secretNamespace is required")
	}
	if cr.UsernameKey == "" {
		return CredentialRef{}, fmt.Errorf("credential: usernameKey is required")
	}
	if cr.PasswordKey == "" {
		return CredentialRef{}, fmt.Errorf("credential: passwordKey is required")
	}
	return cr, nil
}
