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
			{Kind: "datastore.postgresql", Name: "db"},
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
			{Kind: "datastore.oracle", Name: "db"},
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
			{Kind: "datastore.postgresql", Name: "same"},
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
			{Kind: "gateway.pgbouncer", Name: "proxy"},
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
			{Kind: "datastore.postgresql", Name: "db"},
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
			{Kind: "datastore.postgresql", Name: "db"},
			{Kind: "gateway.pgbouncer", Name: "proxy"},
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
			{Kind: "gateway.pgbouncer", Name: "proxy"},
			{Kind: "datastore.postgresql", Name: "db"},
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

func TestNormalizeInputsBasic(t *testing.T) {
	comp := block.Composition{
		Blocks: []block.BlockRef{
			{Kind: "storage.local-pv", Name: "storage"},
			{Kind: "datastore.postgresql", Name: "db", Inputs: map[string]string{
				"storage": "storage/pvc-spec",
			}},
		},
	}

	errs := comp.NormalizeInputs()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(comp.Wires) != 1 {
		t.Fatalf("expected 1 wire, got %d", len(comp.Wires))
	}
	w := comp.Wires[0]
	if w.FromBlock != "storage" || w.FromPort != "pvc-spec" || w.ToBlock != "db" || w.ToPort != "storage" {
		t.Fatalf("unexpected wire: %+v", w)
	}
}

func TestNormalizeInputsMalformed(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"no slash", "storage"},
		{"empty block", "/pvc-spec"},
		{"empty port", "storage/"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			comp := block.Composition{
				Blocks: []block.BlockRef{
					{Kind: "datastore.postgresql", Name: "db", Inputs: map[string]string{
						"storage": tc.value,
					}},
				},
			}
			errs := comp.NormalizeInputs()
			if len(errs) == 0 {
				t.Fatalf("expected error for malformed input %q", tc.value)
			}
		})
	}
}

func TestNormalizeInputsConflict(t *testing.T) {
	comp := block.Composition{
		Blocks: []block.BlockRef{
			{Kind: "storage.local-pv", Name: "s1"},
			{Kind: "storage.local-pv", Name: "s2"},
			{Kind: "datastore.postgresql", Name: "db", Inputs: map[string]string{
				"storage": "s2/pvc-spec",
			}},
		},
		Wires: []block.Wire{
			{FromBlock: "s1", FromPort: "pvc-spec", ToBlock: "db", ToPort: "storage"},
		},
	}

	errs := comp.NormalizeInputs()
	if len(errs) == 0 {
		t.Fatal("expected conflict error")
	}
}

func TestNormalizeInputsHarmlessDuplicate(t *testing.T) {
	comp := block.Composition{
		Blocks: []block.BlockRef{
			{Kind: "storage.local-pv", Name: "storage"},
			{Kind: "datastore.postgresql", Name: "db", Inputs: map[string]string{
				"storage": "storage/pvc-spec",
			}},
		},
		Wires: []block.Wire{
			{FromBlock: "storage", FromPort: "pvc-spec", ToBlock: "db", ToPort: "storage"},
		},
	}

	errs := comp.NormalizeInputs()
	if len(errs) > 0 {
		t.Fatalf("expected no errors for identical duplicate, got: %v", errs)
	}
	if len(comp.Wires) != 1 {
		t.Fatalf("expected 1 wire (deduped), got %d", len(comp.Wires))
	}
}

func TestNormalizeInputsWithValidate(t *testing.T) {
	r := setupRegistry()
	comp := block.Composition{
		Blocks: []block.BlockRef{
			{Kind: "storage.local-pv", Name: "storage"},
			{Kind: "datastore.postgresql", Name: "db", Inputs: map[string]string{
				"storage": "storage/pvc-spec",
			}},
			{Kind: "gateway.pgbouncer", Name: "proxy", Inputs: map[string]string{
				"upstream-dsn": "db/dsn",
			}},
		},
	}

	normErrs := comp.NormalizeInputs()
	if len(normErrs) > 0 {
		t.Fatalf("normalize errors: %v", normErrs)
	}

	validErrs := comp.Validate(r)
	if len(validErrs) > 0 {
		t.Fatalf("validation errors: %v", validErrs)
	}
}

func TestNormalizeInputsTopologicalSort(t *testing.T) {
	r := setupRegistry()
	comp := block.Composition{
		Blocks: []block.BlockRef{
			{Kind: "gateway.pgbouncer", Name: "proxy", Inputs: map[string]string{
				"upstream-dsn": "db/dsn",
			}},
			{Kind: "datastore.postgresql", Name: "db", Inputs: map[string]string{
				"storage": "storage/pvc-spec",
			}},
			{Kind: "storage.local-pv", Name: "storage"},
		},
	}

	comp.NormalizeInputs()
	comp.AutoWire(r)

	sorted, err := comp.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nameOrder := make(map[string]int)
	for i, ref := range sorted {
		nameOrder[ref.Name] = i
	}

	if nameOrder["storage"] >= nameOrder["db"] {
		t.Fatalf("storage should come before db, got: %v", nameOrder)
	}
	if nameOrder["db"] >= nameOrder["proxy"] {
		t.Fatalf("db should come before proxy, got: %v", nameOrder)
	}
}

func TestNormalizeInputsCoexistWithWires(t *testing.T) {
	comp := block.Composition{
		Blocks: []block.BlockRef{
			{Kind: "storage.local-pv", Name: "storage"},
			{Kind: "datastore.postgresql", Name: "db", Inputs: map[string]string{
				"storage": "storage/pvc-spec",
			}},
			{Kind: "gateway.pgbouncer", Name: "proxy"},
		},
		Wires: []block.Wire{
			{FromBlock: "db", FromPort: "dsn", ToBlock: "proxy", ToPort: "upstream-dsn"},
		},
	}

	errs := comp.NormalizeInputs()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(comp.Wires) != 2 {
		t.Fatalf("expected 2 wires (1 explicit + 1 from inputs), got %d", len(comp.Wires))
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
