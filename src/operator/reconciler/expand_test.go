package reconciler

import (
	"context"
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"
)

func TestExpandToComposition_ShorthandPostgreSQL(t *testing.T) {
	spec := ClusterSpec{
		Engine:   "postgresql",
		Replicas: 3,
		Version:  "15",
		Storage:  "10Gi",
	}

	comp, errs := ExpandToComposition(spec)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if len(comp.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(comp.Blocks))
	}

	// First block should be storage.
	if comp.Blocks[0].Kind != "storage.local-pv" {
		t.Errorf("expected first block kind storage.local-pv, got %s", comp.Blocks[0].Kind)
	}
	if comp.Blocks[0].Parameters["size"] != "10Gi" {
		t.Errorf("expected storage size 10Gi, got %s", comp.Blocks[0].Parameters["size"])
	}

	// Second block should be the engine.
	if comp.Blocks[1].Kind != "datastore.postgresql" {
		t.Errorf("expected second block kind datastore.postgresql, got %s", comp.Blocks[1].Kind)
	}
	if comp.Blocks[1].Parameters["version"] != "15" {
		t.Errorf("expected version 15, got %s", comp.Blocks[1].Parameters["version"])
	}
	if comp.Blocks[1].Parameters["replicas"] != "3" {
		t.Errorf("expected replicas 3, got %s", comp.Blocks[1].Parameters["replicas"])
	}
}

func TestExpandToComposition_ShorthandDefaults(t *testing.T) {
	spec := ClusterSpec{
		Engine: "postgresql",
	}

	comp, errs := ExpandToComposition(spec)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if comp.Blocks[1].Parameters["version"] != "16" {
		t.Errorf("expected default version 16, got %s", comp.Blocks[1].Parameters["version"])
	}
	if comp.Blocks[1].Parameters["replicas"] != "1" {
		t.Errorf("expected default replicas 1, got %s", comp.Blocks[1].Parameters["replicas"])
	}
	if comp.Blocks[0].Parameters["size"] != "1Gi" {
		t.Errorf("expected default storage 1Gi, got %s", comp.Blocks[0].Parameters["size"])
	}
}

func TestExpandToComposition_WithBackup(t *testing.T) {
	spec := ClusterSpec{
		Engine: "postgresql",
		Backup: &BackupSpec{
			Enabled:     true,
			Schedule:    "0 2 * * *",
			Destination: "s3://backups/mydb",
		},
	}

	comp, errs := ExpandToComposition(spec)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if len(comp.Blocks) != 3 {
		t.Fatalf("expected 3 blocks (storage + engine + backup), got %d", len(comp.Blocks))
	}
	if comp.Blocks[2].Kind != "integration.s3-backup" {
		t.Errorf("expected backup block kind integration.s3-backup, got %s", comp.Blocks[2].Kind)
	}
	if comp.Blocks[2].Parameters["schedule"] != "0 2 * * *" {
		t.Errorf("expected schedule 0 2 * * *, got %s", comp.Blocks[2].Parameters["schedule"])
	}
}

func TestExpandToComposition_ExplicitBlocks(t *testing.T) {
	spec := ClusterSpec{
		Blocks: &BlocksSpec{
			Composition: []block.BlockRef{
				{Kind: "storage.local-pv", Name: "storage", Parameters: map[string]string{"size": "5Gi"}},
				{
					Kind:       "datastore.postgresql",
					Name:       "db",
					Parameters: map[string]string{"version": "16", "replicas": "2"},
					Inputs:     map[string]string{"storage": "storage/pvc-spec"},
				},
				{
					Kind:   "gateway.pgbouncer",
					Name:   "pooler",
					Inputs: map[string]string{"upstream-dsn": "db/dsn"},
				},
			},
		},
	}

	comp, errs := ExpandToComposition(spec)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// Explicit blocks should be used directly.
	if len(comp.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(comp.Blocks))
	}

	// NormalizeInputs should have expanded inline inputs to wires.
	if len(comp.Wires) != 2 {
		t.Fatalf("expected 2 wires from inline inputs, got %d", len(comp.Wires))
	}

	// Verify the wires are correct.
	wireMap := make(map[string]block.Wire)
	for _, w := range comp.Wires {
		wireMap[w.ToBlock+":"+w.ToPort] = w
	}

	storageWire, ok := wireMap["db:storage"]
	if !ok {
		t.Fatal("expected wire to db:storage")
	}
	if storageWire.FromBlock != "storage" || storageWire.FromPort != "pvc-spec" {
		t.Errorf("expected wire from storage/pvc-spec, got %s/%s", storageWire.FromBlock, storageWire.FromPort)
	}

	dsnWire, ok := wireMap["pooler:upstream-dsn"]
	if !ok {
		t.Fatal("expected wire to pooler:upstream-dsn")
	}
	if dsnWire.FromBlock != "db" || dsnWire.FromPort != "dsn" {
		t.Errorf("expected wire from db/dsn, got %s/%s", dsnWire.FromBlock, dsnWire.FromPort)
	}
}

func TestExpandToComposition_TopologicalOrder(t *testing.T) {
	// This test verifies the full pipeline: expand -> auto-wire -> topo sort
	// produces the correct dependency order: local-pv -> postgresql -> pgbouncer.
	registry := block.NewRegistry()

	// Register minimal blocks that match the descriptors.
	registry.Register(&fakeBlock{descriptor: block.Descriptor{
		Kind:     "storage.local-pv",
		Category: block.CategoryStorage,
		Ports:    []block.Port{{Name: "pvc-spec", PortType: "pvc-spec", Direction: block.PortOutput}},
	}})
	registry.Register(&fakeBlock{descriptor: block.Descriptor{
		Kind:     "datastore.postgresql",
		Category: block.CategoryDatastore,
		Ports: []block.Port{
			{Name: "storage", PortType: "pvc-spec", Direction: block.PortInput, Required: true},
			{Name: "dsn", PortType: "dsn", Direction: block.PortOutput},
			{Name: "metrics", PortType: "metrics-endpoint", Direction: block.PortOutput},
		},
	}})
	registry.Register(&fakeBlock{descriptor: block.Descriptor{
		Kind:     "gateway.pgbouncer",
		Category: block.CategoryGateway,
		Ports: []block.Port{
			{Name: "upstream-dsn", PortType: "dsn", Direction: block.PortInput, Required: true},
			{Name: "dsn", PortType: "dsn", Direction: block.PortOutput},
		},
	}})

	spec := ClusterSpec{
		Blocks: &BlocksSpec{
			Composition: []block.BlockRef{
				{Kind: "gateway.pgbouncer", Name: "pooler", Inputs: map[string]string{"upstream-dsn": "db/dsn"}},
				{Kind: "datastore.postgresql", Name: "db", Inputs: map[string]string{"storage": "storage/pvc-spec"}},
				{Kind: "storage.local-pv", Name: "storage"},
			},
		},
	}

	comp, errs := ExpandToComposition(spec)
	if len(errs) > 0 {
		t.Fatalf("expand errors: %v", errs)
	}

	if validErrs := comp.Validate(registry); len(validErrs) > 0 {
		t.Fatalf("validation errors: %v", validErrs)
	}

	sorted, err := comp.TopologicalSort()
	if err != nil {
		t.Fatalf("topo sort error: %v", err)
	}

	if len(sorted) != 3 {
		t.Fatalf("expected 3 sorted blocks, got %d", len(sorted))
	}

	// The dependency chain is: storage -> db -> pooler.
	// storage must come before db, and db must come before pooler.
	nameOrder := make(map[string]int)
	for i, ref := range sorted {
		nameOrder[ref.Name] = i
	}

	if nameOrder["storage"] >= nameOrder["db"] {
		t.Errorf("storage (pos %d) must come before db (pos %d)", nameOrder["storage"], nameOrder["db"])
	}
	if nameOrder["db"] >= nameOrder["pooler"] {
		t.Errorf("db (pos %d) must come before pooler (pos %d)", nameOrder["db"], nameOrder["pooler"])
	}
}

// fakeBlock is a minimal Block implementation for testing.
type fakeBlock struct {
	descriptor block.Descriptor
}

func (f *fakeBlock) Descriptor() block.Descriptor                                     { return f.descriptor }
func (f *fakeBlock) ValidateParameters(_ context.Context, _ map[string]string) error { return nil }
