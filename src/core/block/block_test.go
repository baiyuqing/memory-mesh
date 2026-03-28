package block_test

import (
	"context"
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"
)

// stubBlock is a minimal block for testing.
type stubBlock struct {
	desc block.Descriptor
}

func (s *stubBlock) Descriptor() block.Descriptor {
	return s.desc
}

func (s *stubBlock) ValidateParameters(_ context.Context, params map[string]string) error {
	return nil
}

func newStorageBlock() *stubBlock {
	return &stubBlock{desc: block.Descriptor{
		Kind:     "storage.local-pv",
		Category: block.CategoryStorage,
		Version:  "1.0.0",
		Ports: []block.Port{
			{Name: "pvc-spec", PortType: "pvc-spec", Direction: block.PortOutput},
		},
		Provides: []string{"pvc-spec"},
	}}
}

func newEngineBlock() *stubBlock {
	return &stubBlock{desc: block.Descriptor{
		Kind:     "engine.postgresql",
		Category: block.CategoryEngine,
		Version:  "1.0.0",
		Ports: []block.Port{
			{Name: "storage", PortType: "pvc-spec", Direction: block.PortInput, Required: true},
			{Name: "dsn", PortType: "dsn", Direction: block.PortOutput},
			{Name: "metrics", PortType: "metrics-endpoint", Direction: block.PortOutput},
		},
		Requires: []string{"storage.*"},
		Provides: []string{"dsn", "metrics-endpoint"},
	}}
}

func newProxyBlock() *stubBlock {
	return &stubBlock{desc: block.Descriptor{
		Kind:     "proxy.pgbouncer",
		Category: block.CategoryProxy,
		Version:  "1.0.0",
		Ports: []block.Port{
			{Name: "upstream-dsn", PortType: "dsn", Direction: block.PortInput, Required: true},
			{Name: "dsn", PortType: "dsn", Direction: block.PortOutput},
		},
		Requires: []string{"engine.*"},
		Provides: []string{"dsn"},
	}}
}

func setupRegistry() *block.Registry {
	r := block.NewRegistry()
	r.Register(newStorageBlock())
	r.Register(newEngineBlock())
	r.Register(newProxyBlock())
	return r
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := block.NewRegistry()

	b := newStorageBlock()
	if err := r.Register(b); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := r.Get("storage.local-pv")
	if !ok {
		t.Fatal("block not found")
	}
	if got.Descriptor().Kind != "storage.local-pv" {
		t.Fatalf("expected storage.local-pv, got %s", got.Descriptor().Kind)
	}
}

func TestRegistryDuplicateRegister(t *testing.T) {
	r := block.NewRegistry()
	r.Register(newStorageBlock())
	err := r.Register(newStorageBlock())
	if err == nil {
		t.Fatal("expected error on duplicate register")
	}
}

func TestRegistryListByCategory(t *testing.T) {
	r := setupRegistry()

	engines := r.ListByCategory(block.CategoryEngine)
	if len(engines) != 1 {
		t.Fatalf("expected 1 engine, got %d", len(engines))
	}
	if engines[0].Kind != "engine.postgresql" {
		t.Fatalf("expected engine.postgresql, got %s", engines[0].Kind)
	}
}
