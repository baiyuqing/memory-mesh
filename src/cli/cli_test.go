package cli

import (
	"bytes"
	"fmt"
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

func TestRootHelp(t *testing.T) {
	for _, flag := range []string{"--help", "-h", "help"} {
		var buf bytes.Buffer
		err := run([]string{flag}, &buf)
		if err != nil {
			t.Fatalf("root %s returned error: %v", flag, err)
		}
		out := buf.String()
		for _, want := range []string{"Usage:", "blocks", "compose"} {
			if !strings.Contains(out, want) {
				t.Errorf("root %s output missing %q", flag, want)
			}
		}
	}
}

func TestRootNoArgs(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{}, &buf)
	if err != nil {
		t.Fatalf("root no-args returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "Usage:") {
		t.Error("expected usage output for no args")
	}
}

func TestBlocksHelp(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"blocks", "--help"}, &buf)
	if err != nil {
		t.Fatalf("blocks --help returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "list") {
		t.Error("blocks help missing 'list' subcommand")
	}
}

func TestBlocksNoArgs(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"blocks"}, &buf)
	if err == nil {
		t.Fatal("expected error for blocks with no subcommand")
	}
	if !strings.Contains(buf.String(), "Usage:") {
		t.Error("expected usage in output")
	}
}

func TestBlocksUnknownSubcommand(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"blocks", "foobar"}, &buf)
	if err == nil {
		t.Fatal("expected error for unknown blocks subcommand")
	}
	if !strings.Contains(err.Error(), "unknown blocks subcommand") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestComposeHelp(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"compose", "--help"}, &buf)
	if err != nil {
		t.Fatalf("compose --help returned error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"validate", "auto-wire", "topology", "--file"} {
		if !strings.Contains(out, want) {
			t.Errorf("compose help missing %q", want)
		}
	}
}

func TestComposeNoArgs(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"compose"}, &buf)
	if err == nil {
		t.Fatal("expected error for compose with no subcommand")
	}
	if !strings.Contains(buf.String(), "Usage:") {
		t.Error("expected usage in output")
	}
}

func TestComposeSubcommandHelp(t *testing.T) {
	for _, sub := range []string{"validate", "auto-wire", "topology"} {
		var buf bytes.Buffer
		err := run([]string{"compose", sub, "--help"}, &buf)
		if err != nil {
			t.Fatalf("compose %s --help returned error: %v", sub, err)
		}
		out := buf.String()
		if !strings.Contains(out, "--file") {
			t.Errorf("compose %s --help missing --file flag", sub)
		}
		if !strings.Contains(out, sub) {
			t.Errorf("compose %s --help missing subcommand name", sub)
		}
	}
}

func TestComposeMissingFile(t *testing.T) {
	for _, sub := range []string{"validate", "auto-wire", "topology"} {
		var buf bytes.Buffer
		err := run([]string{"compose", sub}, &buf)
		if err == nil {
			t.Fatalf("compose %s without --file should error", sub)
		}
		if !strings.Contains(err.Error(), "--file is required") {
			t.Errorf("compose %s: unexpected error: %v", sub, err)
		}
		if !strings.Contains(buf.String(), "--file is required") {
			t.Errorf("compose %s: output should mention --file requirement", sub)
		}
	}
}

func TestUnknownCommand(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"foobar"}, &buf)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "Usage:") {
		t.Error("expected usage in output")
	}
}

// --- Golden output snapshot tests ---
// These tests protect the format stability of CLI output: column names,
// ordering, success text, wire labels, and topology structure.

func TestGolden_BlocksList(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"blocks", "list"}, &buf); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")

	// Header line: exact column names and order
	if len(lines) < 1 {
		t.Fatal("no output")
	}
	for _, col := range []string{"CATEGORY", "NAME", "KIND", "DESCRIPTION"} {
		if !strings.Contains(lines[0], col) {
			t.Errorf("header missing column %q: %s", col, lines[0])
		}
	}

	// Expect exactly 4 data rows + 1 header = 5 lines
	if len(lines) != 5 {
		t.Errorf("expected 5 lines (1 header + 4 blocks), got %d", len(lines))
	}

	// Rows sorted by category then kind: datastore, gateway, security, storage
	wantOrder := []struct{ category, name, kind string }{
		{"datastore", "PostgreSQL", "datastore.postgresql"},
		{"gateway", "PgBouncer", "gateway.pgbouncer"},
		{"security", "Password Rotation", "security.password-rotation"},
		{"storage", "Local PV", "storage.local-pv"},
	}
	for i, want := range wantOrder {
		line := lines[i+1]
		if !strings.Contains(line, want.category) {
			t.Errorf("line %d: expected category %q, got: %s", i+1, want.category, line)
		}
		if !strings.Contains(line, want.name) {
			t.Errorf("line %d: expected name %q, got: %s", i+1, want.name, line)
		}
		if !strings.Contains(line, want.kind) {
			t.Errorf("line %d: expected kind %q, got: %s", i+1, want.kind, line)
		}
	}
}

func TestGolden_ComposeValidate(t *testing.T) {
	path := sampleCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "validate", "--file", path}, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// Must match format: "ok  <path> (3 blocks)"
	if !strings.Contains(out, "ok") {
		t.Error("missing 'ok' prefix")
	}
	if !strings.Contains(out, "(3 blocks)") {
		t.Errorf("expected '(3 blocks)', got: %s", out)
	}
}

func TestGolden_ComposeAutoWire(t *testing.T) {
	path := sampleCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "auto-wire", "--file", path}, &buf); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")

	// Line 1: "ok  <path> (3 blocks, 3 wires)"
	if !strings.Contains(lines[0], "(3 blocks, 3 wires)") {
		t.Errorf("summary line mismatch: %s", lines[0])
	}

	// Line 2: empty
	// Line 3: header with exact column names
	headerLine := ""
	for _, l := range lines {
		if strings.Contains(l, "FROM BLOCK") {
			headerLine = l
			break
		}
	}
	if headerLine == "" {
		t.Fatal("missing wire table header")
	}
	for _, col := range []string{"FROM BLOCK", "PORT", "TO BLOCK", "PORT"} {
		if !strings.Contains(headerLine, col) {
			t.Errorf("header missing %q", col)
		}
	}

	// Exact wire rows (3 wires for onboarding sample)
	out := buf.String()
	wantWires := []struct{ from, fromPort, to, toPort string }{
		{"storage", "pvc-spec", "db", "storage"},
		{"db", "dsn", "pooler", "upstream-dsn"},
		{"db", "credential", "pooler", "upstream-credential"},
	}
	for _, w := range wantWires {
		found := false
		for _, l := range lines {
			if strings.Contains(l, w.from) && strings.Contains(l, w.fromPort) &&
				strings.Contains(l, w.to) && strings.Contains(l, w.toPort) &&
				l != headerLine {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing wire: %s/%s -> %s/%s in output:\n%s", w.from, w.fromPort, w.to, w.toPort, out)
		}
	}
}

func TestGolden_ComposeTopology(t *testing.T) {
	path := sampleCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "topology", "--file", path}, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")

	// Line 1: "ok  <path> (3 blocks)"
	if !strings.Contains(lines[0], "(3 blocks)") {
		t.Errorf("summary line mismatch: %s", lines[0])
	}

	// "Topological order:" section header
	if !strings.Contains(out, "Topological order:") {
		t.Error("missing 'Topological order:' header")
	}

	// Exact order: storage -> db -> pooler with kind annotations
	wantOrder := []struct {
		index int
		name  string
		kind  string
	}{
		{1, "storage", "storage.local-pv"},
		{2, "db", "datastore.postgresql"},
		{3, "pooler", "gateway.pgbouncer"},
	}
	for _, w := range wantOrder {
		expected := fmt.Sprintf("%d. %s (%s)", w.index, w.name, w.kind)
		if !strings.Contains(out, expected) {
			t.Errorf("missing topo entry %q in output:\n%s", expected, out)
		}
	}

	// "Wires (3):" section
	if !strings.Contains(out, "Wires (3):") {
		t.Error("missing 'Wires (3):' header")
	}

	// Wire format: "block/port -> block/port"
	wantWires := []string{
		"storage/pvc-spec -> db/storage",
		"db/dsn -> pooler/upstream-dsn",
		"db/credential -> pooler/upstream-credential",
	}
	for _, w := range wantWires {
		if !strings.Contains(out, w) {
			t.Errorf("missing wire %q in output:\n%s", w, out)
		}
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
