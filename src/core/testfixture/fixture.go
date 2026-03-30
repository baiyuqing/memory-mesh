// Package testfixture provides the canonical block set and test helpers
// for compiler, API, and reconciler tests. All three layers import from
// here instead of defining their own fakeBlock and registry builders.
//
// This ensures that "same input → same result" tests are literally
// using the same input.
package testfixture

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"

	"gopkg.in/yaml.v3"
)

// FakeBlock is a minimal Block implementation for tests. It carries
// only a Descriptor and always passes parameter validation.
type FakeBlock struct {
	Desc block.Descriptor
}

func (f *FakeBlock) Descriptor() block.Descriptor                                     { return f.Desc }
func (f *FakeBlock) ValidateParameters(_ context.Context, _ map[string]string) error { return nil }

// Phase1Blocks returns the canonical Phase 1 block set used across all
// test layers. Any test that needs "the standard 3 blocks" should call
// this instead of building its own list.
func Phase1Blocks() []block.Block {
	return []block.Block{
		&FakeBlock{Desc: block.Descriptor{
			Kind:        "storage.local-pv",
			Category:    block.CategoryStorage,
			Version:     "1.0.0",
			Description: "Local PersistentVolume storage for database data. Ephemeral — data does not survive node loss.",
			Ports:       []block.Port{{Name: "pvc-spec", PortType: "pvc-spec", Direction: block.PortOutput}},
			Parameters: []block.ParameterSpec{
				{Name: "size", Type: "string", Default: "1Gi"},
			},
		}},
		&FakeBlock{Desc: block.Descriptor{
			Kind:        "datastore.postgresql",
			Category:    block.CategoryDatastore,
			Version:     "1.0.0",
			Description: "PostgreSQL database engine managed as a Kubernetes StatefulSet.",
			Ports: []block.Port{
				{Name: "storage", PortType: "pvc-spec", Direction: block.PortInput, Required: true},
				{Name: "dsn", PortType: "dsn", Direction: block.PortOutput},
				{Name: "credential", PortType: "credential", Direction: block.PortOutput},
				{Name: "metrics", PortType: "metrics-endpoint", Direction: block.PortOutput},
			},
		}},
		&FakeBlock{Desc: block.Descriptor{
			Kind:        "gateway.pgbouncer",
			Category:    block.CategoryGateway,
			Version:     "1.0.0",
			Description: "PgBouncer connection pooler for PostgreSQL.",
			Ports: []block.Port{
				{Name: "upstream-dsn", PortType: "dsn", Direction: block.PortInput, Required: true},
				{Name: "upstream-credential", PortType: "credential", Direction: block.PortInput},
				{Name: "dsn", PortType: "dsn", Direction: block.PortOutput},
			},
		}},
		&FakeBlock{Desc: block.Descriptor{
			Kind:        "security.password-rotation",
			Category:    block.CategorySecurity,
			Version:     "1.0.0",
			Description: "Credential rotation scaffold via CronJob (stub — generates and stores passwords in Secret, does not yet execute ALTER USER on upstream DB).",
			Ports: []block.Port{
				{Name: "upstream-dsn", PortType: "dsn", Direction: block.PortInput, Required: true},
				{Name: "credential", PortType: "credential", Direction: block.PortOutput},
			},
		}},
	}
}

// NewPhase1Registry builds a Registry pre-loaded with the Phase 1 block
// set. This is the single canonical registry builder for all test layers.
func NewPhase1Registry() *block.Registry {
	r := block.NewRegistry()
	for _, b := range Phase1Blocks() {
		_ = r.Register(b)
	}
	return r
}

// StandardComposition returns the canonical 3-block explicit composition
// used for cross-layer consistency proofs.
func StandardComposition() []block.BlockRef {
	return []block.BlockRef{
		{Kind: "storage.local-pv", Name: "storage"},
		{Kind: "datastore.postgresql", Name: "db", Inputs: map[string]string{"storage": "storage/pvc-spec"}},
		{Kind: "gateway.pgbouncer", Name: "pooler", Inputs: map[string]string{"upstream-dsn": "db/dsn"}},
	}
}

// CredentialPathComposition returns the 4-block credential path:
// local-pv -> postgresql -> password-rotation -> pgbouncer.
// pgbouncer explicitly wires both upstream-dsn and upstream-credential.
func CredentialPathComposition() []block.BlockRef {
	return []block.BlockRef{
		{Kind: "storage.local-pv", Name: "storage", Parameters: map[string]string{"size": "5Gi"}},
		{Kind: "datastore.postgresql", Name: "db", Parameters: map[string]string{"version": "16", "replicas": "3"}, Inputs: map[string]string{"storage": "storage/pvc-spec"}},
		{Kind: "security.password-rotation", Name: "rotator", Inputs: map[string]string{"upstream-dsn": "db/dsn"}},
		{Kind: "gateway.pgbouncer", Name: "pooler", Inputs: map[string]string{
			"upstream-dsn":        "db/dsn",
			"upstream-credential": "rotator/credential",
		}},
	}
}

// Sample path summary constants (deploy/examples/sample-composition.json).
const (
	SampleBlockCount = 3
	SampleWireCount  = 3
)

// Standard path summary constants (deploy/examples/standard-composition.json).
const (
	StandardBlockCount = 4
	StandardWireCount  = 4
)

// SampleTopoOrder is the expected topological order for the sample path.
var SampleTopoOrder = []string{"storage", "db", "pooler"}

// StandardTopoOrder is the expected topological order for the standard path.
var StandardTopoOrder = []string{"storage", "db", "rotator", "pooler"}

// SampleCredentialWire is the credential wire in the sample path:
// db produces the credential directly to pooler.
var SampleCredentialWire = block.Wire{
	FromBlock: "db", FromPort: "credential", ToBlock: "pooler", ToPort: "upstream-credential",
}

// StandardCredentialWire is the credential wire in the standard path:
// rotator produces the credential to pooler (not db directly).
var StandardCredentialWire = block.Wire{
	FromBlock: "rotator", FromPort: "credential", ToBlock: "pooler", ToPort: "upstream-credential",
}

// WireLabel returns "fromBlock/fromPort -> toBlock/toPort" for use in
// text output assertions.
func WireLabel(w block.Wire) string {
	return w.FromBlock + "/" + w.FromPort + " -> " + w.ToBlock + "/" + w.ToPort
}

// SampleExpectedWires lists the wires produced by the sample (3-block) path.
var SampleExpectedWires = []block.Wire{
	{FromBlock: "storage", FromPort: "pvc-spec", ToBlock: "db", ToPort: "storage"},
	{FromBlock: "db", FromPort: "dsn", ToBlock: "pooler", ToPort: "upstream-dsn"},
	SampleCredentialWire,
}

// StandardExpectedWires lists the wires produced by the standard (4-block credential) path.
var StandardExpectedWires = []block.Wire{
	{FromBlock: "storage", FromPort: "pvc-spec", ToBlock: "db", ToPort: "storage"},
	{FromBlock: "db", FromPort: "dsn", ToBlock: "rotator", ToPort: "upstream-dsn"},
	{FromBlock: "db", FromPort: "dsn", ToBlock: "pooler", ToPort: "upstream-dsn"},
	StandardCredentialWire,
}

// blockKindMap maps block name to kind from Phase1Blocks.
var blockKindMap = func() map[string]string {
	m := make(map[string]string)
	for _, b := range Phase1Blocks() {
		d := b.Descriptor()
		// Use the short name (last segment after the dot is the category prefix).
		// Names in compositions use arbitrary aliases, so we map kind to itself
		// and also map common aliases used in sample/standard compositions.
		m[d.Kind] = d.Kind
	}
	// Common composition aliases → kind.
	m["storage"] = "storage.local-pv"
	m["db"] = "datastore.postgresql"
	m["pooler"] = "gateway.pgbouncer"
	m["rotator"] = "security.password-rotation"
	return m
}()

// TopoLabels converts an ordered list of block names into "name (kind)"
// labels for topology text output assertions.
func TopoLabels(order []string) []string {
	labels := make([]string, len(order))
	for i, name := range order {
		kind, ok := blockKindMap[name]
		if !ok {
			kind = name // fallback
		}
		labels[i] = name + " (" + kind + ")"
	}
	return labels
}

// SampleTopoLabels are the "name (kind)" labels for sample path topology output.
var SampleTopoLabels = TopoLabels(SampleTopoOrder)

// StandardTopoLabels are the "name (kind)" labels for standard path topology output.
var StandardTopoLabels = TopoLabels(StandardTopoOrder)

// examplesDir returns the absolute path to deploy/examples/ relative
// to this source file's location.
func examplesDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "deploy", "examples")
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

// compositionJSON mirrors the JSON composition file format.
type compositionJSON struct {
	Composition struct {
		Blocks []block.BlockRef `json:"blocks"`
	} `json:"composition"`
}

// LoadSampleCompositionJSON reads deploy/examples/sample-composition.json
// and returns the 3-block sample path blocks.
func LoadSampleCompositionJSON(t *testing.T) []block.BlockRef {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(examplesDir(), "sample-composition.json"))
	if err != nil {
		t.Fatalf("read sample-composition.json: %v", err)
	}
	var doc compositionJSON
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse sample-composition.json: %v", err)
	}
	return doc.Composition.Blocks
}

// LoadStandardCompositionJSON reads deploy/examples/standard-composition.json
// and returns the 4-block standard path blocks.
func LoadStandardCompositionJSON(t *testing.T) []block.BlockRef {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(examplesDir(), "standard-composition.json"))
	if err != nil {
		t.Fatalf("read standard-composition.json: %v", err)
	}
	var doc compositionJSON
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse standard-composition.json: %v", err)
	}
	return doc.Composition.Blocks
}

// LoadStandardClusterYAML reads deploy/examples/standard-cluster.yaml
// and returns the 4-block standard path blocks.
func LoadStandardClusterYAML(t *testing.T) []block.BlockRef {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(examplesDir(), "standard-cluster.yaml"))
	if err != nil {
		t.Fatalf("read standard-cluster.yaml: %v", err)
	}
	var doc clusterYAML
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse standard-cluster.yaml: %v", err)
	}
	return doc.Spec.Blocks.Composition
}

// assertTopoOrder verifies that sorted blocks follow the expected order.
func assertTopoOrder(t *testing.T, sorted []block.BlockRef, order []string) {
	t.Helper()
	if len(sorted) != len(order) {
		t.Fatalf("expected %d sorted blocks, got %d", len(order), len(sorted))
	}
	posMap := make(map[string]int)
	for i, ref := range sorted {
		posMap[ref.Name] = i
	}
	for i := 0; i < len(order)-1; i++ {
		a, b := order[i], order[i+1]
		if posMap[a] >= posMap[b] {
			t.Errorf("%s (pos %d) should come before %s (pos %d)", a, posMap[a], b, posMap[b])
		}
	}
}

// AssertCredentialPathOrder verifies the 4-block standard credential
// path topo order: storage -> db -> rotator -> pooler.
func AssertCredentialPathOrder(t *testing.T, sorted []block.BlockRef) {
	t.Helper()
	assertTopoOrder(t, sorted, StandardTopoOrder)
}

// AssertSampleTopoOrder verifies the 3-block sample path topo order:
// storage -> db -> pooler.
func AssertSampleTopoOrder(t *testing.T, sorted []block.BlockRef) {
	t.Helper()
	assertTopoOrder(t, sorted, SampleTopoOrder)
}

// assertWiresExist verifies that all expected wires exist in the actual wire set.
func assertWiresExist(t *testing.T, actual []block.Wire, expected []block.Wire) {
	t.Helper()
	wireSet := make(map[string]bool)
	for _, w := range actual {
		key := w.FromBlock + "/" + w.FromPort + "->" + w.ToBlock + "/" + w.ToPort
		wireSet[key] = true
	}
	for _, ew := range expected {
		key := ew.FromBlock + "/" + ew.FromPort + "->" + ew.ToBlock + "/" + ew.ToPort
		if !wireSet[key] {
			t.Errorf("expected wire %q not found in %v", key, wireSet)
		}
	}
}

// AssertCredentialPathWires verifies the key wires of the standard
// credential path exist: storage→db, db→rotator, rotator→pooler.
func AssertCredentialPathWires(t *testing.T, wires []block.Wire) {
	t.Helper()
	assertWiresExist(t, wires, StandardExpectedWires)
}

// AssertSampleWires verifies the key wires of the sample path exist:
// storage→db, db→pooler (dsn + credential).
func AssertSampleWires(t *testing.T, wires []block.Wire) {
	t.Helper()
	assertWiresExist(t, wires, SampleExpectedWires)
}
