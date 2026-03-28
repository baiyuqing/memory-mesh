package compiler

import (
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"
	"github.com/baiyuqing/ottoplus/src/core/testfixture"
)

func TestCompile_Shorthand(t *testing.T) {
	registry := testfixture.NewPhase1Registry()
	spec := ClusterSpec{
		Engine:   "postgresql",
		Replicas: 3,
		Version:  "15",
		Storage:  "10Gi",
	}

	result, errs := Compile(spec, registry)
	if result == nil {
		t.Fatalf("expected result, got errors: %v", errs)
	}

	// Should have 2 blocks: storage + engine.
	if len(result.Composition.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result.Composition.Blocks))
	}

	// Sorted should put storage before engine (auto-wired dependency).
	if len(result.Sorted) != 2 {
		t.Fatalf("expected 2 sorted blocks, got %d", len(result.Sorted))
	}

	posMap := make(map[string]int)
	for i, ref := range result.Sorted {
		posMap[ref.Name] = i
	}
	if posMap["default-storage"] >= posMap["default-engine"] {
		t.Errorf("storage should come before engine in topo order")
	}

	// Wires should have been auto-generated.
	if len(result.Composition.Wires) == 0 {
		t.Error("expected auto-wired connections, got none")
	}
}

func TestCompile_ExplicitComposition(t *testing.T) {
	registry := testfixture.NewPhase1Registry()
	spec := ClusterSpec{
		Blocks: &BlocksSpec{
			Composition: testfixture.StandardComposition(),
		},
	}

	result, errs := Compile(spec, registry)
	if result == nil {
		t.Fatalf("expected result, got errors: %v", errs)
	}

	if len(result.Composition.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(result.Composition.Blocks))
	}

	// Topo order: storage -> db -> pooler.
	if len(result.Sorted) != 3 {
		t.Fatalf("expected 3 sorted, got %d", len(result.Sorted))
	}
	posMap := make(map[string]int)
	for i, ref := range result.Sorted {
		posMap[ref.Name] = i
	}
	if posMap["storage"] >= posMap["db"] {
		t.Errorf("storage (pos %d) should come before db (pos %d)", posMap["storage"], posMap["db"])
	}
	if posMap["db"] >= posMap["pooler"] {
		t.Errorf("db (pos %d) should come before pooler (pos %d)", posMap["db"], posMap["pooler"])
	}

	// Wires: inline inputs should have been normalized.
	if len(result.Composition.Wires) != 2 {
		t.Errorf("expected 2 wires from inline inputs, got %d", len(result.Composition.Wires))
	}
}

func TestCompileComposition_MatchesCompile(t *testing.T) {
	// Prove that CompileComposition and Compile produce identical results
	// for the same explicit composition input.
	registry := testfixture.NewPhase1Registry()
	blocks := testfixture.StandardComposition()

	// Via Compile (ClusterSpec with explicit blocks).
	specResult, specErrs := Compile(ClusterSpec{
		Blocks: &BlocksSpec{Composition: blocks},
	}, registry)

	// Via CompileComposition (direct composition).
	compResult, compErrs := CompileComposition(block.Composition{Blocks: blocks}, registry)

	if specResult == nil {
		t.Fatalf("Compile failed: %v", specErrs)
	}
	if compResult == nil {
		t.Fatalf("CompileComposition failed: %v", compErrs)
	}

	// Same number of wires.
	if len(specResult.Composition.Wires) != len(compResult.Composition.Wires) {
		t.Errorf("wire count mismatch: Compile=%d, CompileComposition=%d",
			len(specResult.Composition.Wires), len(compResult.Composition.Wires))
	}

	// Same topo order.
	if len(specResult.Sorted) != len(compResult.Sorted) {
		t.Fatalf("sorted count mismatch: %d vs %d", len(specResult.Sorted), len(compResult.Sorted))
	}
	for i := range specResult.Sorted {
		if specResult.Sorted[i].Name != compResult.Sorted[i].Name {
			t.Errorf("sorted[%d] mismatch: Compile=%s, CompileComposition=%s",
				i, specResult.Sorted[i].Name, compResult.Sorted[i].Name)
		}
	}
}

func TestCompile_InvalidKind(t *testing.T) {
	registry := testfixture.NewPhase1Registry()
	spec := ClusterSpec{
		Blocks: &BlocksSpec{
			Composition: []block.BlockRef{
				{Kind: "nonexistent.block", Name: "bad"},
			},
		},
	}

	result, errs := Compile(spec, registry)
	if result != nil {
		t.Error("expected nil result for invalid composition")
	}
	if len(errs) == 0 {
		t.Error("expected errors for invalid block kind")
	}
}

func TestCompile_MissingEngine(t *testing.T) {
	registry := testfixture.NewPhase1Registry()
	spec := ClusterSpec{
		Replicas: 1,
	}

	result, errs := Compile(spec, registry)
	if result != nil {
		t.Error("expected nil result when engine is missing")
	}
	if len(errs) == 0 {
		t.Error("expected error for missing engine")
	}
}
