package examples_test

import (
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"
	"github.com/baiyuqing/ottoplus/src/core/compiler"
	"github.com/baiyuqing/ottoplus/src/core/testfixture"
)

// TestStandardCompositionJSON_Compiles proves the 4-block JSON example
// file passes the full compilation pipeline.
func TestStandardCompositionJSON_Compiles(t *testing.T) {
	blocks := testfixture.LoadStandardCompositionJSON(t)
	registry := testfixture.NewPhase1Registry()

	result, errs := compiler.CompileComposition(block.Composition{Blocks: blocks}, registry)
	if result == nil {
		t.Fatalf("compilation failed: %v", errs)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(result.Sorted) != testfixture.StandardBlockCount {
		t.Fatalf("expected %d sorted blocks, got %d", testfixture.StandardBlockCount, len(result.Sorted))
	}
}

// TestStandardClusterYAML_Compiles proves the 4-block YAML example
// file passes the full compilation pipeline.
func TestStandardClusterYAML_Compiles(t *testing.T) {
	blocks := testfixture.LoadStandardClusterYAML(t)
	registry := testfixture.NewPhase1Registry()

	result, errs := compiler.Compile(compiler.ClusterSpec{
		Blocks: &compiler.BlocksSpec{Composition: blocks},
	}, registry)
	if result == nil {
		t.Fatalf("compilation failed: %v", errs)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(result.Sorted) != testfixture.StandardBlockCount {
		t.Fatalf("expected %d sorted blocks, got %d", testfixture.StandardBlockCount, len(result.Sorted))
	}
}

// TestStandardCompositionJSON_TopoOrder verifies the dependency order
// matches the standard credential path.
func TestStandardCompositionJSON_TopoOrder(t *testing.T) {
	blocks := testfixture.LoadStandardCompositionJSON(t)
	registry := testfixture.NewPhase1Registry()

	result, _ := compiler.CompileComposition(block.Composition{Blocks: blocks}, registry)
	if result == nil {
		t.Fatal("compilation failed")
	}

	testfixture.AssertCredentialPathOrder(t, result.Sorted)
}

// TestStandardClusterYAML_TopoOrder verifies the dependency order
// matches the standard credential path.
func TestStandardClusterYAML_TopoOrder(t *testing.T) {
	blocks := testfixture.LoadStandardClusterYAML(t)
	registry := testfixture.NewPhase1Registry()

	result, _ := compiler.Compile(compiler.ClusterSpec{
		Blocks: &compiler.BlocksSpec{Composition: blocks},
	}, registry)
	if result == nil {
		t.Fatal("compilation failed")
	}

	testfixture.AssertCredentialPathOrder(t, result.Sorted)
}

// TestStandardExamples_MatchCredentialPathFixture ensures the example
// files don't drift from testfixture.CredentialPathComposition().
// If either file is edited without updating the fixture (or vice versa),
// this test breaks.
func TestStandardExamples_MatchCredentialPathFixture(t *testing.T) {
	canonical := testfixture.CredentialPathComposition()

	for _, tc := range []struct {
		name   string
		blocks []block.BlockRef
	}{
		{"json", testfixture.LoadStandardCompositionJSON(t)},
		{"yaml", testfixture.LoadStandardClusterYAML(t)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.blocks) != len(canonical) {
				t.Fatalf("block count: got %d, want %d", len(tc.blocks), len(canonical))
			}
			for i, got := range tc.blocks {
				want := canonical[i]
				if got.Kind != want.Kind {
					t.Errorf("block[%d].Kind: got %q, want %q", i, got.Kind, want.Kind)
				}
				if got.Name != want.Name {
					t.Errorf("block[%d].Name: got %q, want %q", i, got.Name, want.Name)
				}
				for k, wantV := range want.Parameters {
					if gotV, ok := got.Parameters[k]; !ok || gotV != wantV {
						t.Errorf("block[%d].Parameters[%q]: got %q, want %q", i, k, gotV, wantV)
					}
				}
				for k := range got.Parameters {
					if _, ok := want.Parameters[k]; !ok {
						t.Errorf("block[%d].Parameters has unexpected key %q", i, k)
					}
				}
				for k, wantV := range want.Inputs {
					if gotV, ok := got.Inputs[k]; !ok || gotV != wantV {
						t.Errorf("block[%d].Inputs[%q]: got %q, want %q", i, k, gotV, wantV)
					}
				}
				for k := range got.Inputs {
					if _, ok := want.Inputs[k]; !ok {
						t.Errorf("block[%d].Inputs has unexpected key %q", i, k)
					}
				}
			}
		})
	}
}

