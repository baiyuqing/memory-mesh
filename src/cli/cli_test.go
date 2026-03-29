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
	for _, name := range []string{"Local PV", "PostgreSQL", "PgBouncer", "Password Rotation"} {
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
		{"storage.local-pv", "Local PV"},
		{"datastore.postgresql", "PostgreSQL"},
		{"gateway.pgbouncer", "PgBouncer"},
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

func TestComposeAutoWire_StandardComposition(t *testing.T) {
	path := standardCompositionPath(t)
	var buf bytes.Buffer
	err := run([]string{"compose", "auto-wire", "--file", path}, &buf)
	if err != nil {
		t.Fatalf("compose auto-wire: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ok") {
		t.Errorf("expected ok output, got: %s", out)
	}
	// Must contain wire table headers
	for _, header := range []string{"FROM BLOCK", "PORT", "TO BLOCK"} {
		if !strings.Contains(out, header) {
			t.Errorf("output missing %s header", header)
		}
	}
	// Must report 4 wires for standard composition
	if !strings.Contains(out, "4 wires") {
		t.Errorf("expected 4 wires, got: %s", out)
	}
}

func TestComposeAutoWire_SampleComposition(t *testing.T) {
	path := sampleCompositionPath(t)
	var buf bytes.Buffer
	err := run([]string{"compose", "auto-wire", "--file", path}, &buf)
	if err != nil {
		t.Fatalf("compose auto-wire: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ok") {
		t.Errorf("expected ok output, got: %s", out)
	}
	if !strings.Contains(out, "3 wires") {
		t.Errorf("expected 3 wires, got: %s", out)
	}
}

func TestComposeTopology_StandardComposition(t *testing.T) {
	path := standardCompositionPath(t)
	var buf bytes.Buffer
	err := run([]string{"compose", "topology", "--file", path}, &buf)
	if err != nil {
		t.Fatalf("compose topology: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ok") {
		t.Errorf("expected ok output, got: %s", out)
	}
	if !strings.Contains(out, "Topological order") {
		t.Errorf("expected topological order section, got: %s", out)
	}
	// storage must come before db in topological order
	storageIdx := strings.Index(out, "storage (storage.local-pv)")
	dbIdx := strings.Index(out, "db (datastore.postgresql)")
	if storageIdx < 0 || dbIdx < 0 {
		t.Errorf("missing expected blocks in output: %s", out)
	} else if storageIdx > dbIdx {
		t.Errorf("storage should come before db in topological order")
	}
}

func TestComposeTopology_SampleComposition(t *testing.T) {
	path := sampleCompositionPath(t)
	var buf bytes.Buffer
	err := run([]string{"compose", "topology", "--file", path}, &buf)
	if err != nil {
		t.Fatalf("compose topology: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ok") {
		t.Errorf("expected ok output, got: %s", out)
	}
	if !strings.Contains(out, "3 blocks") {
		t.Errorf("expected 3 blocks, got: %s", out)
	}
}

func TestComposeAutoWire_FileNotFound(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"compose", "auto-wire", "--file", "/nonexistent/path.json"}, &buf)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "cannot read file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestComposeTopology_FileNotFound(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"compose", "topology", "--file", "/nonexistent/path.json"}, &buf)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "cannot read file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestComposeUnknownSubcommand(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"compose", "foobar", "--file", "x.json"}, &buf)
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), "unknown compose subcommand") {
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
