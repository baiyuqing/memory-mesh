package reconciler

import (
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"
	"github.com/baiyuqing/ottoplus/src/core/compiler"
	"github.com/baiyuqing/ottoplus/src/core/testfixture"
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
