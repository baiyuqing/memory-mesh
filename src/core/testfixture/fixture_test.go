package testfixture_test

import (
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"
	"github.com/baiyuqing/ottoplus/src/core/compiler"
	"github.com/baiyuqing/ottoplus/src/core/testfixture"
)

// TestSampleSummaryTruth verifies that the sample path summary constants
// (SampleBlockCount, SampleWireCount, SampleTopoOrder, SampleTopoLabels)
// match the actual compilation result of StandardComposition().
func TestSampleSummaryTruth(t *testing.T) {
	registry := testfixture.NewPhase1Registry()
	result, errs := compiler.CompileComposition(
		block.Composition{Blocks: testfixture.StandardComposition()}, registry)
	if result == nil {
		t.Fatalf("compilation failed: %v", errs)
	}

	if len(result.Composition.Blocks) != testfixture.SampleBlockCount {
		t.Errorf("SampleBlockCount=%d but StandardComposition() compiles to %d blocks",
			testfixture.SampleBlockCount, len(result.Composition.Blocks))
	}
	if len(result.Composition.Wires) != testfixture.SampleWireCount {
		t.Errorf("SampleWireCount=%d but StandardComposition() compiles to %d wires",
			testfixture.SampleWireCount, len(result.Composition.Wires))
	}
	if len(result.Sorted) != len(testfixture.SampleTopoOrder) {
		t.Fatalf("SampleTopoOrder has %d entries but got %d sorted blocks",
			len(testfixture.SampleTopoOrder), len(result.Sorted))
	}
	for i, ref := range result.Sorted {
		if ref.Name != testfixture.SampleTopoOrder[i] {
			t.Errorf("SampleTopoOrder[%d]=%q but sorted[%d].Name=%q",
				i, testfixture.SampleTopoOrder[i], i, ref.Name)
		}
	}

	labels := testfixture.TopoLabels(testfixture.SampleTopoOrder)
	for i, want := range testfixture.SampleTopoLabels {
		if labels[i] != want {
			t.Errorf("SampleTopoLabels[%d]=%q but TopoLabels() produces %q", i, want, labels[i])
		}
	}

	// Verify SampleExpectedWires matches the actual compilation result.
	testfixture.AssertSampleWires(t, result.Composition.Wires)
	if len(testfixture.SampleExpectedWires) != testfixture.SampleWireCount {
		t.Errorf("len(SampleExpectedWires)=%d but SampleWireCount=%d",
			len(testfixture.SampleExpectedWires), testfixture.SampleWireCount)
	}

	// Verify SampleCredentialWire is present in the compiled wires.
	found := false
	for _, w := range result.Composition.Wires {
		if w == testfixture.SampleCredentialWire {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("SampleCredentialWire %v not found in compiled wires", testfixture.SampleCredentialWire)
	}
}

// TestStandardSummaryTruth verifies that the standard path summary constants
// (StandardBlockCount, StandardWireCount, StandardTopoOrder, StandardTopoLabels)
// match the actual compilation result of CredentialPathComposition().
func TestStandardSummaryTruth(t *testing.T) {
	registry := testfixture.NewPhase1Registry()
	result, errs := compiler.CompileComposition(
		block.Composition{Blocks: testfixture.CredentialPathComposition()}, registry)
	if result == nil {
		t.Fatalf("compilation failed: %v", errs)
	}

	if len(result.Composition.Blocks) != testfixture.StandardBlockCount {
		t.Errorf("StandardBlockCount=%d but CredentialPathComposition() compiles to %d blocks",
			testfixture.StandardBlockCount, len(result.Composition.Blocks))
	}
	if len(result.Composition.Wires) != testfixture.StandardWireCount {
		t.Errorf("StandardWireCount=%d but CredentialPathComposition() compiles to %d wires",
			testfixture.StandardWireCount, len(result.Composition.Wires))
	}
	if len(result.Sorted) != len(testfixture.StandardTopoOrder) {
		t.Fatalf("StandardTopoOrder has %d entries but got %d sorted blocks",
			len(testfixture.StandardTopoOrder), len(result.Sorted))
	}
	for i, ref := range result.Sorted {
		if ref.Name != testfixture.StandardTopoOrder[i] {
			t.Errorf("StandardTopoOrder[%d]=%q but sorted[%d].Name=%q",
				i, testfixture.StandardTopoOrder[i], i, ref.Name)
		}
	}

	labels := testfixture.TopoLabels(testfixture.StandardTopoOrder)
	for i, want := range testfixture.StandardTopoLabels {
		if labels[i] != want {
			t.Errorf("StandardTopoLabels[%d]=%q but TopoLabels() produces %q", i, want, labels[i])
		}
	}

	// Verify StandardExpectedWires matches the actual compilation result.
	testfixture.AssertCredentialPathWires(t, result.Composition.Wires)
	if len(testfixture.StandardExpectedWires) != testfixture.StandardWireCount {
		t.Errorf("len(StandardExpectedWires)=%d but StandardWireCount=%d",
			len(testfixture.StandardExpectedWires), testfixture.StandardWireCount)
	}

	// Verify StandardCredentialWire is present in the compiled wires.
	found := false
	for _, w := range result.Composition.Wires {
		if w == testfixture.StandardCredentialWire {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("StandardCredentialWire %v not found in compiled wires", testfixture.StandardCredentialWire)
	}
}

// TestSampleExampleFileMatchesSummaryTruth verifies that
// deploy/examples/sample-composition.json produces the same block/wire
// counts as the sample summary constants.
func TestSampleExampleFileMatchesSummaryTruth(t *testing.T) {
	registry := testfixture.NewPhase1Registry()
	blocks := testfixture.LoadSampleCompositionJSON(t)
	result, errs := compiler.CompileComposition(
		block.Composition{Blocks: blocks}, registry)
	if result == nil {
		t.Fatalf("compilation failed: %v", errs)
	}

	if len(result.Composition.Blocks) != testfixture.SampleBlockCount {
		t.Errorf("sample-composition.json has %d blocks, SampleBlockCount=%d",
			len(result.Composition.Blocks), testfixture.SampleBlockCount)
	}
	if len(result.Composition.Wires) != testfixture.SampleWireCount {
		t.Errorf("sample-composition.json has %d wires, SampleWireCount=%d",
			len(result.Composition.Wires), testfixture.SampleWireCount)
	}
}

// TestStandardExampleFileMatchesSummaryTruth verifies that
// deploy/examples/standard-composition.json produces the same block/wire
// counts as the standard summary constants.
func TestStandardExampleFileMatchesSummaryTruth(t *testing.T) {
	registry := testfixture.NewPhase1Registry()
	blocks := testfixture.LoadStandardCompositionJSON(t)
	result, errs := compiler.CompileComposition(
		block.Composition{Blocks: blocks}, registry)
	if result == nil {
		t.Fatalf("compilation failed: %v", errs)
	}

	if len(result.Composition.Blocks) != testfixture.StandardBlockCount {
		t.Errorf("standard-composition.json has %d blocks, StandardBlockCount=%d",
			len(result.Composition.Blocks), testfixture.StandardBlockCount)
	}
	if len(result.Composition.Wires) != testfixture.StandardWireCount {
		t.Errorf("standard-composition.json has %d wires, StandardWireCount=%d",
			len(result.Composition.Wires), testfixture.StandardWireCount)
	}
}
