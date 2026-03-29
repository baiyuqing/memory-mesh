package block

import "testing"

func TestCredentialRef_RoundTrip(t *testing.T) {
	original := CredentialRef{
		SecretName:      "mydb-credentials",
		SecretNamespace: "default",
		UsernameKey:     "username",
		PasswordKey:     "password",
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeCredentialRef(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.SecretName != original.SecretName {
		t.Errorf("SecretName: got %q, want %q", decoded.SecretName, original.SecretName)
	}
	if decoded.SecretNamespace != original.SecretNamespace {
		t.Errorf("SecretNamespace: got %q, want %q", decoded.SecretNamespace, original.SecretNamespace)
	}
	if decoded.UsernameKey != original.UsernameKey {
		t.Errorf("UsernameKey: got %q, want %q", decoded.UsernameKey, original.UsernameKey)
	}
	if decoded.PasswordKey != original.PasswordKey {
		t.Errorf("PasswordKey: got %q, want %q", decoded.PasswordKey, original.PasswordKey)
	}
}

func TestCredentialRef_RoundTripWithDevOnly(t *testing.T) {
	original := CredentialRef{
		SecretName:      "mydb-credentials",
		SecretNamespace: "default",
		UsernameKey:     "username",
		PasswordKey:     "password",
		DevOnly:         true,
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeCredentialRef(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if !decoded.DevOnly {
		t.Error("DevOnly: got false, want true")
	}
}

func TestCredentialRef_DevOnlyOmittedWhenFalse(t *testing.T) {
	cr := CredentialRef{
		SecretName:      "test",
		SecretNamespace: "ns",
		UsernameKey:     "user",
		PasswordKey:     "pass",
	}

	encoded, err := cr.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// devOnly should not appear in JSON when false.
	if contains(encoded, "devOnly") {
		t.Errorf("expected devOnly to be omitted when false, got: %s", encoded)
	}
}

// TestCredentialRef_JSONFieldNames verifies the exact JSON field names in the
// wire format. If a json struct tag is renamed, this test catches it before
// any cross-block or cross-layer integration silently breaks.
func TestCredentialRef_JSONFieldNames(t *testing.T) {
	cr := CredentialRef{
		SecretName:      "s",
		SecretNamespace: "ns",
		UsernameKey:     "u",
		PasswordKey:     "p",
		DevOnly:         true,
	}
	encoded, err := cr.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	for _, field := range []string{
		`"secretName"`,
		`"secretNamespace"`,
		`"usernameKey"`,
		`"passwordKey"`,
		`"devOnly"`,
	} {
		if !contains(encoded, field) {
			t.Errorf("JSON wire format missing field %s, got: %s", field, encoded)
		}
	}
}

func TestDecodeCredentialRef_InvalidJSON(t *testing.T) {
	_, err := DecodeCredentialRef("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDecodeCredentialRef_MissingFields(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"missing secretName", `{"secretNamespace":"ns","usernameKey":"u","passwordKey":"p"}`},
		{"missing secretNamespace", `{"secretName":"s","usernameKey":"u","passwordKey":"p"}`},
		{"missing usernameKey", `{"secretName":"s","secretNamespace":"ns","passwordKey":"p"}`},
		{"missing passwordKey", `{"secretName":"s","secretNamespace":"ns","usernameKey":"u"}`},
		{"all empty", `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeCredentialRef(tt.input)
			if err == nil {
				t.Error("expected error for incomplete credential")
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
