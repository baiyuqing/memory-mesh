package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"
)

// setupTestServer creates an API server with the Phase 1 block set
// registered in a real Registry.
func setupTestServer(t *testing.T) *Server {
	t.Helper()
	registry := block.NewRegistry()

	for _, b := range []block.Block{
		&fakeBlock{descriptor: block.Descriptor{
			Kind:     "storage.local-pv",
			Category: block.CategoryStorage,
			Version:  "1.0.0",
			Ports:    []block.Port{{Name: "pvc-spec", PortType: "pvc-spec", Direction: block.PortOutput}},
			Parameters: []block.ParameterSpec{
				{Name: "size", Type: "string", Default: "1Gi"},
			},
		}},
		&fakeBlock{descriptor: block.Descriptor{
			Kind:     "datastore.postgresql",
			Category: block.CategoryDatastore,
			Version:  "1.0.0",
			Ports: []block.Port{
				{Name: "storage", PortType: "pvc-spec", Direction: block.PortInput, Required: true},
				{Name: "dsn", PortType: "dsn", Direction: block.PortOutput},
				{Name: "metrics", PortType: "metrics-endpoint", Direction: block.PortOutput},
			},
		}},
		&fakeBlock{descriptor: block.Descriptor{
			Kind:     "gateway.pgbouncer",
			Category: block.CategoryGateway,
			Version:  "1.0.0",
			Ports: []block.Port{
				{Name: "upstream-dsn", PortType: "dsn", Direction: block.PortInput, Required: true},
				{Name: "dsn", PortType: "dsn", Direction: block.PortOutput},
			},
		}},
	} {
		if err := registry.Register(b); err != nil {
			t.Fatal(err)
		}
	}
	return NewServer(registry)
}

func TestHealthz(t *testing.T) {
	srv := setupTestServer(t)
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %s", body["status"])
	}
}

func TestListBlocks(t *testing.T) {
	srv := setupTestServer(t)
	req := httptest.NewRequest("GET", "/v1/blocks", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body BlockListResponse
	json.NewDecoder(w.Body).Decode(&body)
	if len(body.Blocks) != 3 {
		t.Errorf("expected 3 blocks, got %d", len(body.Blocks))
	}
}

func TestListBlocksByCategory(t *testing.T) {
	srv := setupTestServer(t)
	req := httptest.NewRequest("GET", "/v1/blocks?category=datastore", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body BlockListResponse
	json.NewDecoder(w.Body).Decode(&body)
	if len(body.Blocks) != 1 {
		t.Errorf("expected 1 datastore block, got %d", len(body.Blocks))
	}
	if len(body.Blocks) > 0 && body.Blocks[0].Kind != "datastore.postgresql" {
		t.Errorf("expected datastore.postgresql, got %s", body.Blocks[0].Kind)
	}
}

func TestGetBlock(t *testing.T) {
	srv := setupTestServer(t)
	req := httptest.NewRequest("GET", "/v1/blocks/datastore.postgresql", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var desc block.Descriptor
	json.NewDecoder(w.Body).Decode(&desc)
	if desc.Kind != "datastore.postgresql" {
		t.Errorf("expected kind datastore.postgresql, got %s", desc.Kind)
	}
}

func TestGetBlockNotFound(t *testing.T) {
	srv := setupTestServer(t)
	req := httptest.NewRequest("GET", "/v1/blocks/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestValidateComposition_Valid(t *testing.T) {
	srv := setupTestServer(t)

	comp := block.Composition{
		Blocks: []block.BlockRef{
			{Kind: "storage.local-pv", Name: "storage"},
			{Kind: "datastore.postgresql", Name: "db", Inputs: map[string]string{"storage": "storage/pvc-spec"}},
			{Kind: "gateway.pgbouncer", Name: "pooler", Inputs: map[string]string{"upstream-dsn": "db/dsn"}},
		},
	}
	body, _ := json.Marshal(ValidateRequest{Composition: comp})
	req := httptest.NewRequest("POST", "/v1/compositions/validate", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var resp ValidateResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.IsValid {
		t.Errorf("expected valid composition, got errors: %v", resp.Errors)
	}
}

func TestValidateComposition_InvalidKind(t *testing.T) {
	srv := setupTestServer(t)

	comp := block.Composition{
		Blocks: []block.BlockRef{
			{Kind: "nonexistent.block", Name: "bad"},
		},
	}
	body, _ := json.Marshal(ValidateRequest{Composition: comp})
	req := httptest.NewRequest("POST", "/v1/compositions/validate", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
	var resp ValidateResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.IsValid {
		t.Error("expected invalid composition")
	}
}

func TestTopology_CorrectOrder(t *testing.T) {
	srv := setupTestServer(t)

	comp := block.Composition{
		Blocks: []block.BlockRef{
			{Kind: "gateway.pgbouncer", Name: "pooler", Inputs: map[string]string{"upstream-dsn": "db/dsn"}},
			{Kind: "storage.local-pv", Name: "storage"},
			{Kind: "datastore.postgresql", Name: "db", Inputs: map[string]string{"storage": "storage/pvc-spec"}},
		},
	}
	body, _ := json.Marshal(AutoWireRequest{Composition: comp})
	req := httptest.NewRequest("POST", "/v1/compositions/topology", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var resp TopologyResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(resp.Nodes))
	}

	// Verify topological order: storage before db, db before pooler.
	posMap := make(map[string]int)
	for i, n := range resp.Nodes {
		posMap[n.Name] = i
	}
	if posMap["storage"] >= posMap["db"] {
		t.Errorf("storage (pos %d) should come before db (pos %d)", posMap["storage"], posMap["db"])
	}
	if posMap["db"] >= posMap["pooler"] {
		t.Errorf("db (pos %d) should come before pooler (pos %d)", posMap["db"], posMap["pooler"])
	}
}

// fakeBlock is a minimal Block implementation for API tests.
type fakeBlock struct {
	descriptor block.Descriptor
}

func (f *fakeBlock) Descriptor() block.Descriptor                                     { return f.descriptor }
func (f *fakeBlock) ValidateParameters(_ context.Context, _ map[string]string) error { return nil }
