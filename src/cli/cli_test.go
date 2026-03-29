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
	if result.BlockCount != 3 {
		t.Errorf("expected blockCount=3, got %d", result.BlockCount)
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
	if result.BlockCount != 3 {
		t.Errorf("expected blockCount=3, got %d", result.BlockCount)
	}
	if result.WireCount != 3 {
		t.Errorf("expected wireCount=3, got %d", result.WireCount)
	}
	if len(result.Wires) != 3 {
		t.Fatalf("expected 3 wires, got %d", len(result.Wires))
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
	if result.BlockCount != 3 {
		t.Errorf("expected blockCount=3, got %d", result.BlockCount)
	}
	if len(result.Order) != 3 {
		t.Fatalf("expected 3 order entries, got %d", len(result.Order))
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
	if result.WireCount != 3 {
		t.Errorf("expected wireCount=3, got %d", result.WireCount)
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

func TestCredentialPath_SampleComposition_AutoWire(t *testing.T) {
	path := sampleCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "auto-wire", "--file", path}, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// Sample: 3 blocks, 3 wires. Credential wire is auto-wired from db to pooler.
	if !strings.Contains(out, "3 blocks, 3 wires") {
		t.Errorf("expected 3 blocks / 3 wires summary, got: %s", out)
	}
	// The credential wire must go from db to pooler.
	for _, wire := range []string{
		"db                    credential        pooler                upstream-credential",
		"db                    dsn               pooler                upstream-dsn",
		"storage               pvc-spec          db                    storage",
	} {
		if !strings.Contains(out, wire) {
			t.Errorf("missing expected wire %q in output:\n%s", wire, out)
		}
	}
}

func TestCredentialPath_StandardComposition_AutoWire(t *testing.T) {
	path := standardCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "auto-wire", "--file", path}, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// Standard: 4 blocks, 4 wires. Credential wire goes from rotator to pooler.
	if !strings.Contains(out, "4 blocks, 4 wires") {
		t.Errorf("expected 4 blocks / 4 wires summary, got: %s", out)
	}
	// Key wires that must not drift (order may vary due to map iteration).
	for _, wire := range []string{
		"rotator               credential        pooler                upstream-credential",
		"db                    dsn               rotator               upstream-dsn",
	} {
		if !strings.Contains(out, wire) {
			t.Errorf("missing expected credential-path wire %q in output:\n%s", wire, out)
		}
	}
	// Storage wire must remain.
	if !strings.Contains(out, "storage               pvc-spec          db                    storage") {
		t.Errorf("missing storage wire in output:\n%s", out)
	}
}

func TestCredentialPath_StandardComposition_Topology(t *testing.T) {
	path := standardCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "topology", "--file", path}, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// 4 blocks in topological order.
	if !strings.Contains(out, "4 blocks") {
		t.Errorf("expected 4 blocks, got: %s", out)
	}
	// Rotator must come after db and before pooler in topological order.
	dbIdx := strings.Index(out, "db (datastore.postgresql)")
	rotatorIdx := strings.Index(out, "rotator (security.password-rotation)")
	poolerIdx := strings.Index(out, "pooler (gateway.pgbouncer)")
	if dbIdx < 0 || rotatorIdx < 0 || poolerIdx < 0 {
		t.Fatalf("missing expected blocks in topology output:\n%s", out)
	}
	if dbIdx > rotatorIdx {
		t.Errorf("db must come before rotator in topological order")
	}
	if rotatorIdx > poolerIdx {
		t.Errorf("rotator must come before pooler in topological order")
	}

	// Credential wire must appear in topology wires section.
	if !strings.Contains(out, "rotator/credential -> pooler/upstream-credential") {
		t.Errorf("missing credential wire in topology:\n%s", out)
	}
	if !strings.Contains(out, "Wires (4)") {
		t.Errorf("expected 4 wires in topology, got:\n%s", out)
	}
}

func TestCredentialPath_StandardComposition_TopologyJSON(t *testing.T) {
	path := standardCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "topology", "--file", path, "--format", "json"}, &buf); err != nil {
		t.Fatal(err)
	}
	var result topologyResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if result.BlockCount != 4 {
		t.Errorf("expected blockCount=4, got %d", result.BlockCount)
	}
	if result.WireCount != 4 {
		t.Errorf("expected wireCount=4, got %d", result.WireCount)
	}

	// Verify the credential wire exists in JSON output.
	foundCredWire := false
	for _, w := range result.Wires {
		if w.FromBlock == "rotator" && w.FromPort == "credential" &&
			w.ToBlock == "pooler" && w.ToPort == "upstream-credential" {
			foundCredWire = true
			break
		}
	}
	if !foundCredWire {
		t.Errorf("missing credential wire (rotator/credential -> pooler/upstream-credential) in JSON topology")
	}

	// Verify topological order: db before rotator before pooler.
	orderMap := make(map[string]int)
	for _, entry := range result.Order {
		orderMap[entry.Name] = entry.Index
	}
	if orderMap["db"] >= orderMap["rotator"] {
		t.Errorf("db (index %d) must come before rotator (index %d)", orderMap["db"], orderMap["rotator"])
	}
	if orderMap["rotator"] >= orderMap["pooler"] {
		t.Errorf("rotator (index %d) must come before pooler (index %d)", orderMap["rotator"], orderMap["pooler"])
	}
}

func TestCredentialPath_StandardComposition_AutoWireJSON(t *testing.T) {
	path := standardCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "auto-wire", "--file", path, "--format", "json"}, &buf); err != nil {
		t.Fatal(err)
	}
	var result autoWireResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if result.BlockCount != 4 {
		t.Errorf("expected blockCount=4, got %d", result.BlockCount)
	}
	if result.WireCount != 4 {
		t.Errorf("expected wireCount=4, got %d", result.WireCount)
	}

	// Verify credential-path wires exist in JSON output.
	wantWires := []wireEntry{
		{FromBlock: "rotator", FromPort: "credential", ToBlock: "pooler", ToPort: "upstream-credential"},
		{FromBlock: "db", FromPort: "dsn", ToBlock: "rotator", ToPort: "upstream-dsn"},
		{FromBlock: "db", FromPort: "dsn", ToBlock: "pooler", ToPort: "upstream-dsn"},
		{FromBlock: "storage", FromPort: "pvc-spec", ToBlock: "db", ToPort: "storage"},
	}
	for _, want := range wantWires {
		found := false
		for _, got := range result.Wires {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing wire %+v in JSON auto-wire output", want)
		}
	}
}

func TestCredentialPath_SampleComposition_ValidateJSON(t *testing.T) {
	path := sampleCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "validate", "--file", path, "--format", "json"}, &buf); err != nil {
		t.Fatal(err)
	}
	var result validateResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if result.BlockCount != 3 {
		t.Errorf("expected blockCount=3, got %d", result.BlockCount)
	}
}

func TestCredentialPath_StandardComposition_ValidateJSON(t *testing.T) {
	path := standardCompositionPath(t)
	var buf bytes.Buffer
	if err := run([]string{"compose", "validate", "--file", path, "--format", "json"}, &buf); err != nil {
		t.Fatal(err)
	}
	var result validateResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if result.BlockCount != 4 {
		t.Errorf("expected blockCount=4, got %d", result.BlockCount)
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
