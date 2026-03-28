package reconciler

import (
	"os"
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"
	"github.com/baiyuqing/ottoplus/src/core/compiler"
	"github.com/baiyuqing/ottoplus/src/core/testfixture"

	"gopkg.in/yaml.v3"
)

// TestOperatorCompiler_ShorthandPostgreSQL proves that the operator's input (ClusterSpec)
// processed through the unified compiler produces valid, predictable
// results. This is the consistency proof required by Phase 2.

func TestOperatorCompiler_ShorthandPostgreSQL(t *testing.T) {
	registry := testfixture.NewPhase1Registry()
	spec := compiler.ClusterSpec{
		Engine:   "postgresql",
		Replicas: 3,
		Version:  "15",
		Storage:  "10Gi",
	}

	result, errs := compiler.Compile(spec, registry)
	if result == nil {
		t.Fatalf("expected result, got errors: %v", errs)
	}

	if len(result.Composition.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result.Composition.Blocks))
	}
	if result.Composition.Blocks[0].Kind != "storage.local-pv" {
		t.Errorf("expected first block storage.local-pv, got %s", result.Composition.Blocks[0].Kind)
	}
	if result.Composition.Blocks[1].Kind != "datastore.postgresql" {
		t.Errorf("expected second block datastore.postgresql, got %s", result.Composition.Blocks[1].Kind)
	}
	if result.Composition.Blocks[1].Parameters["replicas"] != "3" {
		t.Errorf("expected replicas 3, got %s", result.Composition.Blocks[1].Parameters["replicas"])
	}

	// Verify topo sort: storage before engine.
	if len(result.Sorted) != 2 {
		t.Fatalf("expected 2 sorted, got %d", len(result.Sorted))
	}
	if result.Sorted[0].Kind != "storage.local-pv" {
		t.Errorf("expected storage first in topo order, got %s", result.Sorted[0].Kind)
	}
}

func TestOperatorCompiler_ExplicitWithInlineInputs(t *testing.T) {
	registry := testfixture.NewPhase1Registry()
	spec := compiler.ClusterSpec{
		Blocks: &compiler.BlocksSpec{
			Composition: testfixture.StandardComposition(),
		},
	}

	result, errs := compiler.Compile(spec, registry)
	if result == nil {
		t.Fatalf("expected result, got errors: %v", errs)
	}

	// Wires: 2 inline inputs + 1 auto-wired credential (db→pooler).
	if len(result.Composition.Wires) != 3 {
		t.Fatalf("expected 3 wires, got %d", len(result.Composition.Wires))
	}

	// Topo order must be: storage -> db -> pooler (regardless of input order).
	posMap := make(map[string]int)
	for i, ref := range result.Sorted {
		posMap[ref.Name] = i
	}
	if posMap["storage"] >= posMap["db"] {
		t.Errorf("storage should come before db")
	}
	if posMap["db"] >= posMap["pooler"] {
		t.Errorf("db should come before pooler")
	}
}

func TestOperatorAndAPIGetSameResult(t *testing.T) {
	// The key consistency proof: given the same explicit composition,
	// the operator path (Compile with ClusterSpec) and the API path
	// (CompileComposition with bare Composition) produce identical
	// wires and topo order.
	registry := testfixture.NewPhase1Registry()
	blocks := testfixture.StandardComposition()

	// Operator path.
	operatorResult, _ := compiler.Compile(compiler.ClusterSpec{
		Blocks: &compiler.BlocksSpec{Composition: blocks},
	}, registry)

	// API path.
	apiResult, _ := compiler.CompileComposition(block.Composition{Blocks: blocks}, registry)

	if operatorResult == nil || apiResult == nil {
		t.Fatal("both paths should succeed")
	}

	// Same wires.
	if len(operatorResult.Composition.Wires) != len(apiResult.Composition.Wires) {
		t.Errorf("wire count: operator=%d, api=%d",
			len(operatorResult.Composition.Wires), len(apiResult.Composition.Wires))
	}

	// Same topo order.
	for i := range operatorResult.Sorted {
		if operatorResult.Sorted[i].Name != apiResult.Sorted[i].Name {
			t.Errorf("sorted[%d]: operator=%s, api=%s",
				i, operatorResult.Sorted[i].Name, apiResult.Sorted[i].Name)
		}
	}
}

func TestOperatorCompiler_WithBackup(t *testing.T) {
	registry := testfixture.NewPhase1Registry()
	// Register s3-backup so the shorthand path can reference it.
	registry.Register(&testfixture.FakeBlock{Desc: block.Descriptor{
		Kind:     "integration.s3-backup",
		Category: block.CategoryIntegration,
		Ports: []block.Port{
			{Name: "source-dsn", PortType: "dsn", Direction: block.PortInput},
		},
	}})

	spec := compiler.ClusterSpec{
		Engine: "postgresql",
		Backup: &compiler.BackupSpec{
			Enabled:     true,
			Schedule:    "0 2 * * *",
			Destination: "s3://backups/mydb",
		},
	}

	result, errs := compiler.Compile(spec, registry)
	if result == nil {
		t.Fatalf("expected result, got errors: %v", errs)
	}
	if len(result.Composition.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(result.Composition.Blocks))
	}
	if result.Composition.Blocks[2].Kind != "integration.s3-backup" {
		t.Errorf("expected backup block, got %s", result.Composition.Blocks[2].Kind)
	}
}

// clusterYAML mirrors the subset of the Cluster CRD needed to extract
// the composition from the standard example file.
type clusterYAML struct {
	Spec struct {
		Blocks struct {
			Composition []block.BlockRef `yaml:"composition"`
		} `yaml:"blocks"`
	} `yaml:"spec"`
}

func loadStandardClusterFile(t *testing.T) []block.BlockRef {
	t.Helper()
	data, err := os.ReadFile("../../../deploy/examples/standard-cluster.yaml")
	if err != nil {
		t.Fatalf("read standard-cluster.yaml: %v", err)
	}
	var doc clusterYAML
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse standard-cluster.yaml: %v", err)
	}
	return doc.Spec.Blocks.Composition
}

func TestOperatorCompiler_StandardPath(t *testing.T) {
	registry := testfixture.NewPhase1Registry()
	blocks := loadStandardClusterFile(t)

	result, errs := compiler.Compile(compiler.ClusterSpec{
		Blocks: &compiler.BlocksSpec{Composition: blocks},
	}, registry)
	if result == nil {
		t.Fatalf("compilation failed: %v", errs)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(result.Sorted) != 4 {
		t.Fatalf("expected 4 sorted blocks, got %d", len(result.Sorted))
	}

	// Verify topological order: storage -> db -> rotator -> pooler.
	posMap := make(map[string]int)
	for i, ref := range result.Sorted {
		posMap[ref.Name] = i
	}
	if posMap["storage"] >= posMap["db"] {
		t.Errorf("storage (pos %d) should come before db (pos %d)", posMap["storage"], posMap["db"])
	}
	if posMap["db"] >= posMap["rotator"] {
		t.Errorf("db (pos %d) should come before rotator (pos %d)", posMap["db"], posMap["rotator"])
	}
	if posMap["rotator"] >= posMap["pooler"] {
		t.Errorf("rotator (pos %d) should come before pooler (pos %d)", posMap["rotator"], posMap["pooler"])
	}

	// Verify key wires exist: storage->db, db->rotator, rotator->pooler.
	wireSet := make(map[string]bool)
	for _, w := range result.Composition.Wires {
		key := w.FromBlock + "/" + w.FromPort + "->" + w.ToBlock + "/" + w.ToPort
		wireSet[key] = true
	}
	expectedWires := []string{
		"storage/pvc-spec->db/storage",
		"db/dsn->rotator/upstream-dsn",
		"rotator/credential->pooler/upstream-credential",
	}
	for _, ew := range expectedWires {
		if !wireSet[ew] {
			t.Errorf("expected wire %q not found in %v", ew, wireSet)
		}
	}
}
