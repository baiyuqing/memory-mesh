package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/testfixture"
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
	// Must report correct wire count for standard composition
	if !strings.Contains(out, fmt.Sprintf("%d wires", standardSpec.wireCount)) {
		t.Errorf("expected %d wires, got: %s", standardSpec.wireCount, out)
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
	if !strings.Contains(out, fmt.Sprintf("%d wires", sampleSpec.wireCount)) {
		t.Errorf("expected %d wires, got: %s", sampleSpec.wireCount, out)
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
	if !strings.Contains(out, fmt.Sprintf("%d blocks", sampleSpec.blockCount)) {
		t.Errorf("expected %d blocks, got: %s", sampleSpec.blockCount, out)
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

func TestBlocksListFormatJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"blocks", "list", "--format", "json"}, &buf); err != nil {
		t.Fatal(err)
	}
	var entries []blockEntry
	if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, buf.String())
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 blocks, got %d", len(entries))
	}
	// Verify sorted order and all fields present
	wantOrder := []blockEntry{
		{Category: "datastore", Name: "PostgreSQL", Kind: "datastore.postgresql", Description: testfixture.BlockDescription(t, "datastore.postgresql")},
		{Category: "gateway", Name: "PgBouncer", Kind: "gateway.pgbouncer", Description: testfixture.BlockDescription(t, "gateway.pgbouncer")},
		{Category: "security", Name: "Password Rotation", Kind: "security.password-rotation", Description: testfixture.BlockDescription(t, "security.password-rotation")},
		{Category: "storage", Name: "Local PV", Kind: "storage.local-pv", Description: testfixture.BlockDescription(t, "storage.local-pv")},
	}
	for i, want := range wantOrder {
		got := entries[i]
		if got != want {
			t.Errorf("entry %d mismatch.\nwant: %+v\ngot:  %+v", i, want, got)
		}
	}
}

func TestBlocksListFormatTable(t *testing.T) {
	// --format table should produce the same output as no flag
	var defBuf, tableBuf bytes.Buffer
	if err := run([]string{"blocks", "list"}, &defBuf); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"blocks", "list", "--format", "table"}, &tableBuf); err != nil {
		t.Fatal(err)
	}
	if defBuf.String() != tableBuf.String() {
		t.Errorf("--format table output differs from default.\ndefault:\n%s\ntable:\n%s", defBuf.String(), tableBuf.String())
	}
}

func TestBlocksListFormatInvalid(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"blocks", "list", "--format", "yaml"}, &buf)
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBlocksListFormatMissingValue(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"blocks", "list", "--format"}, &buf)
	if err == nil {
		t.Fatal("expected error for --format without value")
	}
}

func TestBlocksListUnexpectedArg(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"blocks", "list", "garbage"}, &buf)
	if err == nil {
		t.Fatal("expected error for unexpected positional argument")
	}
	if !strings.Contains(err.Error(), "unexpected argument") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBlocksListHelp(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"blocks", "list", "--help"}, &buf)
	if err != nil {
		t.Fatalf("blocks list --help returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "--format") {
		t.Error("blocks list --help missing --format flag")
	}
}

func TestComposeValidateFormatJSON(t *testing.T) {
	path := sampleCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "validate", "--file", path, "--format", "json"}, &buf); err != nil {
		t.Fatal(err)
	}
	var result validateResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if !result.Valid {
		t.Error("expected valid=true")
	}
	if result.BlockCount != sampleSpec.blockCount {
		t.Errorf("expected blockCount=%d, got %d", sampleSpec.blockCount, result.BlockCount)
	}
	if result.File != path {
		t.Errorf("expected file=%q, got %q", path, result.File)
	}
}

func TestComposeAutoWireFormatJSON(t *testing.T) {
	path := sampleCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "auto-wire", "--file", path, "--format", "json"}, &buf); err != nil {
		t.Fatal(err)
	}
	var result autoWireResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	assertJSONCounts(t, sampleSpec, result.BlockCount, result.WireCount)
	if len(result.Wires) != sampleSpec.wireCount {
		t.Fatalf("expected %d wires, got %d", sampleSpec.wireCount, len(result.Wires))
	}
	wantWires := []wireEntry{
		{FromBlock: "storage", FromPort: "pvc-spec", ToBlock: "db", ToPort: "storage"},
		{FromBlock: "db", FromPort: "dsn", ToBlock: "pooler", ToPort: "upstream-dsn"},
		{FromBlock: "db", FromPort: "credential", ToBlock: "pooler", ToPort: "upstream-credential"},
	}
	for i, want := range wantWires {
		if result.Wires[i] != want {
			t.Errorf("wire %d mismatch.\nwant: %+v\ngot:  %+v", i, want, result.Wires[i])
		}
	}
}

func TestComposeTopologyFormatJSON(t *testing.T) {
	path := sampleCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "topology", "--file", path, "--format", "json"}, &buf); err != nil {
		t.Fatal(err)
	}
	var result topologyResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if result.BlockCount != sampleSpec.blockCount {
		t.Errorf("expected blockCount=%d, got %d", sampleSpec.blockCount, result.BlockCount)
	}
	if len(result.Order) != sampleSpec.blockCount {
		t.Fatalf("expected %d order entries, got %d", sampleSpec.blockCount, len(result.Order))
	}
	wantOrder := []topoBlockEntry{
		{Index: 1, Name: "storage", Kind: "storage.local-pv"},
		{Index: 2, Name: "db", Kind: "datastore.postgresql"},
		{Index: 3, Name: "pooler", Kind: "gateway.pgbouncer"},
	}
	for i, want := range wantOrder {
		if result.Order[i] != want {
			t.Errorf("order %d mismatch.\nwant: %+v\ngot:  %+v", i, want, result.Order[i])
		}
	}
	if result.WireCount != sampleSpec.wireCount {
		t.Errorf("expected wireCount=%d, got %d", sampleSpec.wireCount, result.WireCount)
	}
}

func TestComposeSubcommandHelpIncludesFormat(t *testing.T) {
	for _, sub := range []string{"validate", "auto-wire", "topology"} {
		var buf bytes.Buffer
		err := run([]string{"compose", sub, "--help"}, &buf)
		if err != nil {
			t.Fatalf("compose %s --help returned error: %v", sub, err)
		}
		if !strings.Contains(buf.String(), "--format") {
			t.Errorf("compose %s --help missing --format flag", sub)
		}
	}
}

func TestComposeUnexpectedArg(t *testing.T) {
	path := sampleCompositionPath(t)
	for _, sub := range []string{"validate", "auto-wire", "topology"} {
		var buf bytes.Buffer
		err := run([]string{"compose", sub, "--file", path, "garbage"}, &buf)
		if err == nil {
			t.Fatalf("compose %s with trailing arg should error", sub)
		}
		if !strings.Contains(err.Error(), "unexpected argument") {
			t.Errorf("compose %s: unexpected error: %v", sub, err)
		}
	}
}

func TestComposeFormatInvalid(t *testing.T) {
	path := sampleCompositionPath(t)
	for _, sub := range []string{"validate", "auto-wire", "topology"} {
		var buf bytes.Buffer
		err := run([]string{"compose", sub, "--file", path, "--format", "yaml"}, &buf)
		if err == nil {
			t.Fatalf("compose %s --format yaml should error", sub)
		}
		if !strings.Contains(err.Error(), "unsupported format") {
			t.Errorf("compose %s: unexpected error: %v", sub, err)
		}
	}
}

// --- Golden output snapshot tests ---
// These tests protect the exact format stability of CLI output.
// Any change to column names, spacing, ordering, success text, wire labels,
// or topology structure will break these tests — that is intentional.

func TestGolden_BlocksList(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"blocks", "list"}, &buf); err != nil {
		t.Fatal(err)
	}
	want := "" +
		"CATEGORY        NAME                    KIND                        DESCRIPTION\n" +
		"datastore       PostgreSQL              datastore.postgresql        PostgreSQL database engine managed as...\n" +
		"gateway         PgBouncer               gateway.pgbouncer           PgBouncer connection pooler for Postg...\n" +
		"security        Password Rotation       security.password-rotation  Credential rotation scaffold via Cron...\n" +
		"storage         Local PV                storage.local-pv            Local PersistentVolume storage for da...\n"
	if got := buf.String(); got != want {
		t.Errorf("blocks list output mismatch.\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestGolden_BlocksListJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"blocks", "list", "--format", "json"}, &buf); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`[
  {
    "category": "datastore",
    "name": "PostgreSQL",
    "kind": "datastore.postgresql",
    "description": "%s"
  },
  {
    "category": "gateway",
    "name": "PgBouncer",
    "kind": "gateway.pgbouncer",
    "description": "%s"
  },
  {
    "category": "security",
    "name": "Password Rotation",
    "kind": "security.password-rotation",
    "description": "%s"
  },
  {
    "category": "storage",
    "name": "Local PV",
    "kind": "storage.local-pv",
    "description": "%s"
  }
]
`,
		testfixture.BlockDescription(t, "datastore.postgresql"),
		testfixture.BlockDescription(t, "gateway.pgbouncer"),
		testfixture.BlockDescription(t, "security.password-rotation"),
		testfixture.BlockDescription(t, "storage.local-pv"),
	)
	if got := buf.String(); got != want {
		t.Errorf("blocks list --format json output mismatch.\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestGolden_ComposeValidate(t *testing.T) {
	path := sampleCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "validate", "--file", path}, &buf); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("ok  %s (3 blocks)\n", path)
	if got := buf.String(); got != want {
		t.Errorf("compose validate output mismatch.\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestGolden_ComposeAutoWire(t *testing.T) {
	path := sampleCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "auto-wire", "--file", path}, &buf); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("ok  %s (3 blocks, 3 wires)\n", path) +
		"\n" +
		"FROM BLOCK            PORT              TO BLOCK              PORT            \n" +
		"storage               pvc-spec          db                    storage         \n" +
		"db                    dsn               pooler                upstream-dsn    \n" +
		"db                    credential        pooler                upstream-credential\n"
	if got := buf.String(); got != want {
		t.Errorf("compose auto-wire output mismatch.\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestGolden_ComposeTopology(t *testing.T) {
	path := sampleCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "topology", "--file", path}, &buf); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("ok  %s (3 blocks)\n", path) +
		"\n" +
		"Topological order:\n" +
		"  1. storage (storage.local-pv)\n" +
		"  2. db (datastore.postgresql)\n" +
		"  3. pooler (gateway.pgbouncer)\n" +
		"\n" +
		"Wires (3):\n" +
		"  storage/pvc-spec -> db/storage\n" +
		"  db/dsn -> pooler/upstream-dsn\n" +
		"  db/credential -> pooler/upstream-credential\n"
	if got := buf.String(); got != want {
		t.Errorf("compose topology output mismatch.\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestGolden_ComposeValidateJSON(t *testing.T) {
	path := sampleCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "validate", "--file", path, "--format", "json"}, &buf); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`{
  "file": %q,
  "valid": true,
  "blockCount": 3
}
`, path)
	if got := buf.String(); got != want {
		t.Errorf("compose validate --format json output mismatch.\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestGolden_ComposeAutoWireJSON(t *testing.T) {
	path := sampleCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "auto-wire", "--file", path, "--format", "json"}, &buf); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`{
  "file": %q,
  "blockCount": 3,
  "wireCount": 3,
  "wires": [
    {
      "fromBlock": "storage",
      "fromPort": "pvc-spec",
      "toBlock": "db",
      "toPort": "storage"
    },
    {
      "fromBlock": "db",
      "fromPort": "dsn",
      "toBlock": "pooler",
      "toPort": "upstream-dsn"
    },
    {
      "fromBlock": "db",
      "fromPort": "credential",
      "toBlock": "pooler",
      "toPort": "upstream-credential"
    }
  ]
}
`, path)
	if got := buf.String(); got != want {
		t.Errorf("compose auto-wire --format json output mismatch.\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestGolden_ComposeTopologyJSON(t *testing.T) {
	path := sampleCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "topology", "--file", path, "--format", "json"}, &buf); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`{
  "file": %q,
  "blockCount": 3,
  "order": [
    {
      "index": 1,
      "name": "storage",
      "kind": "storage.local-pv"
    },
    {
      "index": 2,
      "name": "db",
      "kind": "datastore.postgresql"
    },
    {
      "index": 3,
      "name": "pooler",
      "kind": "gateway.pgbouncer"
    }
  ],
  "wireCount": 3,
  "wires": [
    {
      "fromBlock": "storage",
      "fromPort": "pvc-spec",
      "toBlock": "db",
      "toPort": "storage"
    },
    {
      "fromBlock": "db",
      "fromPort": "dsn",
      "toBlock": "pooler",
      "toPort": "upstream-dsn"
    },
    {
      "fromBlock": "db",
      "fromPort": "credential",
      "toBlock": "pooler",
      "toPort": "upstream-credential"
    }
  ]
}
`, path)
	if got := buf.String(); got != want {
		t.Errorf("compose topology --format json output mismatch.\nwant:\n%s\ngot:\n%s", want, got)
	}
}

// --- Credential-path wire regression tests ---
// These tests protect the credential wiring in sample and standard compositions.
// If a credential wire drifts, disappears, or is mis-routed, these tests catch it.

// compositionSpec defines the expected shape of a composition for reuse
// across table and JSON test variants.
type compositionSpec struct {
	name       string
	path       func(t *testing.T) string
	blockCount int
	wireCount  int
	// tableWires are wire lines expected in table output (exact column-formatted strings).
	tableWires []string
	// jsonWires are wire entries expected in JSON output.
	jsonWires []wireEntry
	// topoOrder is the expected block ordering (earlier entries must appear first).
	topoOrder []string
	// topoLabels are "name (kind)" strings expected in topology text output.
	topoLabels []string
}

var sampleSpec = compositionSpec{
	name:       "sample",
	path:       sampleCompositionPath,
	blockCount: testfixture.SampleBlockCount,
	wireCount:  testfixture.SampleWireCount,
	tableWires: []string{
		"db                    credential        pooler                upstream-credential",
		"db                    dsn               pooler                upstream-dsn",
		"storage               pvc-spec          db                    storage",
	},
	jsonWires: []wireEntry{
		{FromBlock: "db", FromPort: "credential", ToBlock: "pooler", ToPort: "upstream-credential"},
		{FromBlock: "db", FromPort: "dsn", ToBlock: "pooler", ToPort: "upstream-dsn"},
		{FromBlock: "storage", FromPort: "pvc-spec", ToBlock: "db", ToPort: "storage"},
	},
	topoOrder:  testfixture.SampleTopoOrder,
	topoLabels: []string{"storage (storage.local-pv)", "db (datastore.postgresql)", "pooler (gateway.pgbouncer)"},
}

var standardSpec = compositionSpec{
	name:       "standard",
	path:       standardCompositionPath,
	blockCount: testfixture.StandardBlockCount,
	wireCount:  testfixture.StandardWireCount,
	tableWires: []string{
		"rotator               credential        pooler                upstream-credential",
		"db                    dsn               rotator               upstream-dsn",
		"storage               pvc-spec          db                    storage",
	},
	jsonWires: []wireEntry{
		{FromBlock: "rotator", FromPort: "credential", ToBlock: "pooler", ToPort: "upstream-credential"},
		{FromBlock: "db", FromPort: "dsn", ToBlock: "rotator", ToPort: "upstream-dsn"},
		{FromBlock: "db", FromPort: "dsn", ToBlock: "pooler", ToPort: "upstream-dsn"},
		{FromBlock: "storage", FromPort: "pvc-spec", ToBlock: "db", ToPort: "storage"},
	},
	topoOrder:  []string{"db", "rotator", "pooler"},
	topoLabels: []string{"db (datastore.postgresql)", "rotator (security.password-rotation)", "pooler (gateway.pgbouncer)"},
}

// assertTableWires checks that the table output contains the expected summary and wires.
func assertTableWires(t *testing.T, spec compositionSpec, out string) {
	t.Helper()
	summary := fmt.Sprintf("%d blocks, %d wires", spec.blockCount, spec.wireCount)
	if !strings.Contains(out, summary) {
		t.Errorf("expected %s summary, got: %s", summary, out)
	}
	for _, wire := range spec.tableWires {
		if !strings.Contains(out, wire) {
			t.Errorf("missing expected wire %q in output:\n%s", wire, out)
		}
	}
}

// assertJSONWires checks that JSON result contains the expected wires.
func assertJSONWires(t *testing.T, spec compositionSpec, wires []wireEntry) {
	t.Helper()
	for _, want := range spec.jsonWires {
		found := false
		for _, got := range wires {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing wire %+v in JSON output", want)
		}
	}
}

// assertJSONCounts checks blockCount and wireCount in a JSON result.
func assertJSONCounts(t *testing.T, spec compositionSpec, gotBlocks, gotWires int) {
	t.Helper()
	if gotBlocks != spec.blockCount {
		t.Errorf("expected blockCount=%d, got %d", spec.blockCount, gotBlocks)
	}
	if gotWires != spec.wireCount {
		t.Errorf("expected wireCount=%d, got %d", spec.wireCount, gotWires)
	}
}

// assertTopoOrder checks topological ordering in text or JSON output.
func assertTopoOrder(t *testing.T, spec compositionSpec, out string) {
	t.Helper()
	for i := 0; i < len(spec.topoLabels)-1; i++ {
		a := strings.Index(out, spec.topoLabels[i])
		b := strings.Index(out, spec.topoLabels[i+1])
		if a < 0 || b < 0 {
			t.Fatalf("missing expected block label %q or %q in output:\n%s", spec.topoLabels[i], spec.topoLabels[i+1], out)
		}
		if a > b {
			t.Errorf("%s must come before %s in topological order", spec.topoLabels[i], spec.topoLabels[i+1])
		}
	}
}

// assertJSONTopoOrder checks topological ordering in JSON result.
func assertJSONTopoOrder(t *testing.T, spec compositionSpec, order []topoBlockEntry) {
	t.Helper()
	orderMap := make(map[string]int)
	for _, entry := range order {
		orderMap[entry.Name] = entry.Index
	}
	for i := 0; i < len(spec.topoOrder)-1; i++ {
		a, b := spec.topoOrder[i], spec.topoOrder[i+1]
		if orderMap[a] >= orderMap[b] {
			t.Errorf("%s (index %d) must come before %s (index %d)", a, orderMap[a], b, orderMap[b])
		}
	}
}

// runComposeSpec runs a compose subcommand against the spec's composition file
// and returns the output. It fails the test on any run error.
func runComposeSpec(t *testing.T, spec compositionSpec, subcmd string, extraArgs ...string) string {
	t.Helper()
	args := []string{"compose", subcmd, "--file", spec.path(t)}
	args = append(args, extraArgs...)
	var buf bytes.Buffer
	if err := run(args, &buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// assertAutoWireTable runs auto-wire (table) and checks summary + wires.
func assertAutoWireTable(t *testing.T, spec compositionSpec) {
	t.Helper()
	out := runComposeSpec(t, spec, "auto-wire")
	assertTableWires(t, spec, out)
}

// assertAutoWireJSON runs auto-wire (JSON) and checks counts + wires.
func assertAutoWireJSON(t *testing.T, spec compositionSpec) {
	t.Helper()
	out := runComposeSpec(t, spec, "auto-wire", "--format", "json")
	var result autoWireResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	assertJSONCounts(t, spec, result.BlockCount, result.WireCount)
	assertJSONWires(t, spec, result.Wires)
}

// assertTopologyJSON runs topology (JSON) and checks counts, wires, and ordering.
func assertTopologyJSON(t *testing.T, spec compositionSpec) {
	t.Helper()
	out := runComposeSpec(t, spec, "topology", "--format", "json")
	var result topologyResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	assertJSONCounts(t, spec, result.BlockCount, result.WireCount)
	assertJSONWires(t, spec, result.Wires)
	assertJSONTopoOrder(t, spec, result.Order)
}

// assertValidateJSON runs validate (JSON) and checks the block count.
func assertValidateJSON(t *testing.T, spec compositionSpec) {
	t.Helper()
	out := runComposeSpec(t, spec, "validate", "--format", "json")
	var result validateResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if result.BlockCount != spec.blockCount {
		t.Errorf("expected blockCount=%d, got %d", spec.blockCount, result.BlockCount)
	}
}

func TestCredentialPath_SampleComposition_AutoWire(t *testing.T) {
	assertAutoWireTable(t, sampleSpec)
}

func TestCredentialPath_StandardComposition_AutoWire(t *testing.T) {
	assertAutoWireTable(t, standardSpec)
}

func TestCredentialPath_StandardComposition_Topology(t *testing.T) {
	out := runComposeSpec(t, standardSpec, "topology")
	if !strings.Contains(out, fmt.Sprintf("%d blocks", standardSpec.blockCount)) {
		t.Errorf("expected %d blocks, got: %s", standardSpec.blockCount, out)
	}
	if !strings.Contains(out, fmt.Sprintf("Wires (%d)", standardSpec.wireCount)) {
		t.Errorf("expected %d wires in topology, got:\n%s", standardSpec.wireCount, out)
	}
	assertTopoOrder(t, standardSpec, out)
	if !strings.Contains(out, "rotator/credential -> pooler/upstream-credential") {
		t.Errorf("missing credential wire in topology:\n%s", out)
	}
}

func TestCredentialPath_StandardComposition_TopologyJSON(t *testing.T) {
	assertTopologyJSON(t, standardSpec)
}

func TestCredentialPath_StandardComposition_AutoWireJSON(t *testing.T) {
	assertAutoWireJSON(t, standardSpec)
}

func TestCredentialPath_SampleComposition_ValidateJSON(t *testing.T) {
	assertValidateJSON(t, sampleSpec)
}

func TestCredentialPath_StandardComposition_ValidateJSON(t *testing.T) {
	assertValidateJSON(t, standardSpec)
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
