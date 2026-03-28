package reconciler

import (
	"context"
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"
	"github.com/baiyuqing/ottoplus/src/core/compiler"
)

// TestOperatorUsesCompiler proves that the operator's input (ClusterSpec)
// processed through the unified compiler produces valid, predictable
// results. This is the consistency proof required by Phase 2.

func TestOperatorCompiler_ShorthandPostgreSQL(t *testing.T) {
	registry := testRegistry(t)
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
	registry := testRegistry(t)
	spec := compiler.ClusterSpec{
		Blocks: &compiler.BlocksSpec{
			Composition: []block.BlockRef{
				{Kind: "gateway.pgbouncer", Name: "pooler", Inputs: map[string]string{"upstream-dsn": "db/dsn"}},
				{Kind: "datastore.postgresql", Name: "db", Inputs: map[string]string{"storage": "storage/pvc-spec"}},
				{Kind: "storage.local-pv", Name: "storage"},
			},
		},
	}

	result, errs := compiler.Compile(spec, registry)
	if result == nil {
		t.Fatalf("expected result, got errors: %v", errs)
	}

	// Wires should have been created from inline inputs.
	if len(result.Composition.Wires) != 2 {
		t.Fatalf("expected 2 wires, got %d", len(result.Composition.Wires))
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
	registry := testRegistry(t)

	blocks := []block.BlockRef{
		{Kind: "storage.local-pv", Name: "storage"},
		{Kind: "datastore.postgresql", Name: "db", Inputs: map[string]string{"storage": "storage/pvc-spec"}},
		{Kind: "gateway.pgbouncer", Name: "pooler", Inputs: map[string]string{"upstream-dsn": "db/dsn"}},
	}

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
	registry := testRegistry(t)
	// Register s3-backup so the shorthand path can reference it.
	registry.Register(&fakeBlock{descriptor: block.Descriptor{
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

// testRegistry builds the Phase 1 block set for testing.
func testRegistry(t *testing.T) *block.Registry {
	t.Helper()
	r := block.NewRegistry()
	for _, b := range []block.Block{
		&fakeBlock{descriptor: block.Descriptor{
			Kind:     "storage.local-pv",
			Category: block.CategoryStorage,
			Ports:    []block.Port{{Name: "pvc-spec", PortType: "pvc-spec", Direction: block.PortOutput}},
		}},
		&fakeBlock{descriptor: block.Descriptor{
			Kind:     "datastore.postgresql",
			Category: block.CategoryDatastore,
			Ports: []block.Port{
				{Name: "storage", PortType: "pvc-spec", Direction: block.PortInput, Required: true},
				{Name: "dsn", PortType: "dsn", Direction: block.PortOutput},
				{Name: "metrics", PortType: "metrics-endpoint", Direction: block.PortOutput},
			},
		}},
		&fakeBlock{descriptor: block.Descriptor{
			Kind:     "gateway.pgbouncer",
			Category: block.CategoryGateway,
			Ports: []block.Port{
				{Name: "upstream-dsn", PortType: "dsn", Direction: block.PortInput, Required: true},
				{Name: "dsn", PortType: "dsn", Direction: block.PortOutput},
			},
		}},
	} {
		if err := r.Register(b); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

type fakeBlock struct {
	descriptor block.Descriptor
}

func (f *fakeBlock) Descriptor() block.Descriptor                                     { return f.descriptor }
func (f *fakeBlock) ValidateParameters(_ context.Context, _ map[string]string) error { return nil }
