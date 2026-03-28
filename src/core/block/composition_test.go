package block_test

import (
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"
)

func TestCompositionValidateValid(t *testing.T) {
	r := setupRegistry()
	comp := block.Composition{
		Blocks: []block.BlockRef{
			{Kind: "storage.local-pv", Name: "storage"},
			{Kind: "engine.postgresql", Name: "db"},
		},
		Wires: []block.Wire{
			{FromBlock: "storage", FromPort: "pvc-spec", ToBlock: "db", ToPort: "storage"},
		},
	}

	errs := comp.Validate(r)
	if len(errs) > 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestCompositionValidateUnknownBlock(t *testing.T) {
	r := setupRegistry()
	comp := block.Composition{
		Blocks: []block.BlockRef{
			{Kind: "engine.oracle", Name: "db"},
		},
	}

	errs := comp.Validate(r)
	if len(errs) == 0 {
		t.Fatal("expected error for unknown block kind")
	}
}

func TestCompositionValidateDuplicateName(t *testing.T) {
	r := setupRegistry()
	comp := block.Composition{
		Blocks: []block.BlockRef{
			{Kind: "storage.local-pv", Name: "same"},
			{Kind: "engine.postgresql", Name: "same"},
		},
	}

	errs := comp.Validate(r)
	found := false
	for _, e := range errs {
		if e.Error() == `duplicate block name "same"` {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected duplicate name error, got: %v", errs)
	}
}

func TestCompositionValidatePortTypeMismatch(t *testing.T) {
	r := setupRegistry()
	comp := block.Composition{
		Blocks: []block.BlockRef{
			{Kind: "storage.local-pv", Name: "storage"},
			{Kind: "proxy.pgbouncer", Name: "proxy"},
		},
		Wires: []block.Wire{
			{FromBlock: "storage", FromPort: "pvc-spec", ToBlock: "proxy", ToPort: "upstream-dsn"},
		},
	}

	errs := comp.Validate(r)
	found := false
	for _, e := range errs {
		if e != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("expected port type mismatch error")
	}
}

func TestCompositionValidateRequiredPortNotWired(t *testing.T) {
	r := setupRegistry()
	comp := block.Composition{
		Blocks: []block.BlockRef{
			{Kind: "engine.postgresql", Name: "db"},
		},
	}

	errs := comp.Validate(r)
	found := false
	for _, e := range errs {
		if e.Error() == `block "db" required input port "storage" is not wired` {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected required port error, got: %v", errs)
	}
}

func TestAutoWireSimple(t *testing.T) {
	r := setupRegistry()
	comp := block.Composition{
		Blocks: []block.BlockRef{
			{Kind: "storage.local-pv", Name: "storage"},
			{Kind: "engine.postgresql", Name: "db"},
			{Kind: "proxy.pgbouncer", Name: "proxy"},
		},
	}

	errs := comp.AutoWire(r)
	if len(errs) > 0 {
		t.Fatalf("unexpected auto-wire errors: %v", errs)
	}

	if len(comp.Wires) != 2 {
		t.Fatalf("expected 2 auto-wires, got %d: %+v", len(comp.Wires), comp.Wires)
	}

	// After auto-wire, validation should pass
	validErrs := comp.Validate(r)
	if len(validErrs) > 0 {
		t.Fatalf("validation failed after auto-wire: %v", validErrs)
	}
}

func TestTopologicalSort(t *testing.T) {
	r := setupRegistry()
	comp := block.Composition{
		Blocks: []block.BlockRef{
			{Kind: "proxy.pgbouncer", Name: "proxy"},
			{Kind: "engine.postgresql", Name: "db"},
			{Kind: "storage.local-pv", Name: "storage"},
		},
	}
	comp.AutoWire(r)

	sorted, err := comp.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sorted) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(sorted))
	}

	// storage must come before db, db must come before proxy
	nameOrder := make(map[string]int)
	for i, ref := range sorted {
		nameOrder[ref.Name] = i
	}

	if nameOrder["storage"] >= nameOrder["db"] {
		t.Fatalf("storage should come before db, got order: %v", nameOrder)
	}
	if nameOrder["db"] >= nameOrder["proxy"] {
		t.Fatalf("db should come before proxy, got order: %v", nameOrder)
	}
}

func TestTopologicalSortCircularDependency(t *testing.T) {
	comp := block.Composition{
		Blocks: []block.BlockRef{
			{Kind: "a", Name: "a"},
			{Kind: "b", Name: "b"},
		},
		Wires: []block.Wire{
			{FromBlock: "a", FromPort: "x", ToBlock: "b", ToPort: "y"},
			{FromBlock: "b", FromPort: "x", ToBlock: "a", ToPort: "y"},
		},
	}

	_, err := comp.TopologicalSort()
	if err == nil {
		t.Fatal("expected circular dependency error")
	}
}
