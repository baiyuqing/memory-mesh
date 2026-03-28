package examples_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"
	"github.com/baiyuqing/ottoplus/src/core/compiler"
	"github.com/baiyuqing/ottoplus/src/core/testfixture"

	"gopkg.in/yaml.v3"
)

// clusterYAML mirrors the subset of the Cluster CRD we need to parse.
type clusterYAML struct {
	Spec struct {
		Blocks struct {
			Composition []block.BlockRef `yaml:"composition"`
		} `yaml:"blocks"`
	} `yaml:"spec"`
}

// compositionJSON mirrors the JSON composition file format.
type compositionJSON struct {
	Composition struct {
		Blocks []block.BlockRef `json:"blocks"`
	} `json:"composition"`
}

// TestStandardCompositionJSON_Compiles proves the 4-block JSON example
// file passes the full compilation pipeline.
func TestStandardCompositionJSON_Compiles(t *testing.T) {
	blocks := loadCompositionJSON(t)
	registry := testfixture.NewPhase1Registry()

	result, errs := compiler.CompileComposition(block.Composition{Blocks: blocks}, registry)
	if result == nil {
		t.Fatalf("compilation failed: %v", errs)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(result.Sorted) != 4 {
		t.Fatalf("expected 4 sorted blocks, got %d", len(result.Sorted))
	}
}

// TestStandardClusterYAML_Compiles proves the 4-block YAML example
// file passes the full compilation pipeline.
func TestStandardClusterYAML_Compiles(t *testing.T) {
	blocks := loadClusterYAML(t)
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
	if len(result.Sorted) != 4 {
		t.Fatalf("expected 4 sorted blocks, got %d", len(result.Sorted))
	}
}

// TestStandardCompositionJSON_TopoOrder verifies the dependency order
// matches the standard credential path.
func TestStandardCompositionJSON_TopoOrder(t *testing.T) {
	blocks := loadCompositionJSON(t)
	registry := testfixture.NewPhase1Registry()

	result, _ := compiler.CompileComposition(block.Composition{Blocks: blocks}, registry)
	if result == nil {
		t.Fatal("compilation failed")
	}

	assertCredentialPathOrder(t, result.Sorted)
}

// TestStandardClusterYAML_TopoOrder verifies the dependency order
// matches the standard credential path.
func TestStandardClusterYAML_TopoOrder(t *testing.T) {
	blocks := loadClusterYAML(t)
	registry := testfixture.NewPhase1Registry()

	result, _ := compiler.Compile(compiler.ClusterSpec{
		Blocks: &compiler.BlocksSpec{Composition: blocks},
	}, registry)
	if result == nil {
		t.Fatal("compilation failed")
	}

	assertCredentialPathOrder(t, result.Sorted)
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
		{"json", loadCompositionJSON(t)},
		{"yaml", loadClusterYAML(t)},
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

// --- helpers ---

func loadCompositionJSON(t *testing.T) []block.BlockRef {
	t.Helper()
	data, err := os.ReadFile("standard-composition.json")
	if err != nil {
		t.Fatalf("read standard-composition.json: %v", err)
	}
	var doc compositionJSON
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse standard-composition.json: %v", err)
	}
	return doc.Composition.Blocks
}

func loadClusterYAML(t *testing.T) []block.BlockRef {
	t.Helper()
	data, err := os.ReadFile("standard-cluster.yaml")
	if err != nil {
		t.Fatalf("read standard-cluster.yaml: %v", err)
	}
	var doc clusterYAML
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse standard-cluster.yaml: %v", err)
	}
	return doc.Spec.Blocks.Composition
}

func assertCredentialPathOrder(t *testing.T, sorted []block.BlockRef) {
	t.Helper()
	expectedOrder := []string{"storage", "db", "rotator", "pooler"}
	if len(sorted) != len(expectedOrder) {
		t.Fatalf("expected %d sorted blocks, got %d", len(expectedOrder), len(sorted))
	}
	posMap := make(map[string]int)
	for i, ref := range sorted {
		posMap[ref.Name] = i
	}
	if posMap["storage"] >= posMap["db"] {
		t.Error("storage should come before db")
	}
	if posMap["db"] >= posMap["rotator"] {
		t.Error("db should come before rotator")
	}
	if posMap["rotator"] >= posMap["pooler"] {
		t.Error("rotator should come before pooler")
	}
}
