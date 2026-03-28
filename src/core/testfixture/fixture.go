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
			Kind:     "storage.local-pv",
			Category: block.CategoryStorage,
			Version:  "1.0.0",
			Ports:    []block.Port{{Name: "pvc-spec", PortType: "pvc-spec", Direction: block.PortOutput}},
			Parameters: []block.ParameterSpec{
				{Name: "size", Type: "string", Default: "1Gi"},
			},
		}},
		&FakeBlock{Desc: block.Descriptor{
			Kind:     "datastore.postgresql",
			Category: block.CategoryDatastore,
			Version:  "1.0.0",
			Ports: []block.Port{
				{Name: "storage", PortType: "pvc-spec", Direction: block.PortInput, Required: true},
				{Name: "dsn", PortType: "dsn", Direction: block.PortOutput},
				{Name: "credential", PortType: "credential", Direction: block.PortOutput},
				{Name: "metrics", PortType: "metrics-endpoint", Direction: block.PortOutput},
			},
		}},
		&FakeBlock{Desc: block.Descriptor{
			Kind:     "gateway.pgbouncer",
			Category: block.CategoryGateway,
			Version:  "1.0.0",
			Ports: []block.Port{
				{Name: "upstream-dsn", PortType: "dsn", Direction: block.PortInput, Required: true},
				{Name: "upstream-credential", PortType: "credential", Direction: block.PortInput},
				{Name: "dsn", PortType: "dsn", Direction: block.PortOutput},
			},
		}},
		&FakeBlock{Desc: block.Descriptor{
			Kind:     "security.password-rotation",
			Category: block.CategorySecurity,
			Version:  "1.0.0",
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
