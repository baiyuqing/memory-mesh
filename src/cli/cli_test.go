package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBlocksList(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"blocks", "list"}, &buf); err != nil {
		t.Fatalf("blocks list: %v", err)
	}
	out := buf.String()

	// Must contain headers
	for _, header := range []string{"CATEGORY", "NAME", "KIND"} {
		if !strings.Contains(out, header) {
			t.Errorf("output missing %s header", header)
		}
	}

	// Must list the registered block kinds
	for _, kind := range []string{"storage.local-pv", "datastore.postgresql", "gateway.pgbouncer", "security.password-rotation"} {
		if !strings.Contains(out, kind) {
			t.Errorf("output missing block kind %q", kind)
		}
	}

	// Must contain display names
	for _, name := range []string{"Local Pv", "Postgresql", "Pgbouncer", "Password Rotation"} {
		if !strings.Contains(out, name) {
			t.Errorf("output missing display name %q", name)
		}
	}

	// Must contain categories
	for _, cat := range []string{"storage", "datastore", "gateway", "security"} {
		if !strings.Contains(out, cat) {
			t.Errorf("output missing category %q", cat)
		}
	}
}

func TestDisplayName(t *testing.T) {
	tests := []struct {
		kind string
		want string
	}{
		{"storage.local-pv", "Local Pv"},
		{"datastore.postgresql", "Postgresql"},
		{"gateway.pgbouncer", "Pgbouncer"},
		{"security.password-rotation", "Password Rotation"},
	}
	for _, tt := range tests {
		got := displayName(tt.kind)
		if got != tt.want {
			t.Errorf("displayName(%q) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestComposeValidate_StandardComposition(t *testing.T) {
	path := standardCompositionPath(t)
	var buf bytes.Buffer
	err := run([]string{"compose", "validate", "--file", path}, &buf)
	if err != nil {
		t.Fatalf("compose validate: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ok") {
		t.Errorf("expected ok output, got: %s", out)
	}
}

func TestComposeValidate_SampleComposition(t *testing.T) {
	path := sampleCompositionPath(t)
	var buf bytes.Buffer
	err := run([]string{"compose", "validate", "--file", path}, &buf)
	if err != nil {
		t.Fatalf("compose validate: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ok") {
		t.Errorf("expected ok output, got: %s", out)
	}
}

func TestComposeValidate_FileNotFound(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"compose", "validate", "--file", "/nonexistent/path.json"}, &buf)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "cannot read file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestComposeValidate_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.json")
	if err := os.WriteFile(path, []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := run([]string{"compose", "validate", "--file", path}, &buf)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestComposeValidate_MissingFile(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"compose", "validate"}, &buf)
	if err == nil {
		t.Fatal("expected error when --file is missing")
	}
	if !strings.Contains(err.Error(), "--file is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUnknownCommand(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"foobar"}, &buf)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

// helpers to locate example files relative to the repo root.
func repoRoot(t *testing.T) string {
	t.Helper()
	// Walk up from this test file to find the repo root (contains go.mod).
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("cannot find repo root (go.mod)")
		}
		dir = parent
	}
}

func standardCompositionPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "deploy", "examples", "standard-composition.json")
}

func sampleCompositionPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "deploy", "examples", "sample-composition.json")
}
