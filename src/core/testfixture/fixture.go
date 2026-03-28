// Package testfixture provides the canonical block set and test helpers
// for compiler, API, and reconciler tests. All three layers import from
// here instead of defining their own fakeBlock and registry builders.
//
// This ensures that "same input → same result" tests are literally
// using the same input.
package testfixture

import (
	"context"

	"github.com/baiyuqing/ottoplus/src/core/block"
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
				{Name: "dsn", PortType: "dsn", Direction: block.PortOutput},
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
