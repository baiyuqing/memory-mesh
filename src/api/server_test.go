package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"
	"github.com/baiyuqing/ottoplus/src/core/testfixture"
)

// setupTestServer creates an API server with the canonical Phase 1
// block set from testfixture.
func setupTestServer(t *testing.T) *Server {
	t.Helper()
	return NewServer(testfixture.NewPhase1Registry())
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
	if len(body.Blocks) != 4 {
		t.Errorf("expected 4 blocks, got %d", len(body.Blocks))
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

	comp := block.Composition{Blocks: testfixture.StandardComposition()}
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

	// Use the standard composition but in a different order to prove
	// topo sort works regardless of input order.
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

// loadStandardCompositionFile reads the 4-block standard example file
// and returns the blocks for use in API tests.
func loadStandardCompositionFile(t *testing.T) []block.BlockRef {
	t.Helper()
	data, err := os.ReadFile("../../deploy/examples/standard-composition.json")
	if err != nil {
		t.Fatalf("read standard-composition.json: %v", err)
	}
	var doc struct {
		Composition struct {
			Blocks []block.BlockRef `json:"blocks"`
		} `json:"composition"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse standard-composition.json: %v", err)
	}
	return doc.Composition.Blocks
}

func TestValidateComposition_StandardPath(t *testing.T) {
	srv := setupTestServer(t)

	comp := block.Composition{Blocks: loadStandardCompositionFile(t)}
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

func TestTopology_StandardPath(t *testing.T) {
	srv := setupTestServer(t)

	comp := block.Composition{Blocks: loadStandardCompositionFile(t)}
	body, _ := json.Marshal(AutoWireRequest{Composition: comp})
	req := httptest.NewRequest("POST", "/v1/compositions/topology", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var resp TopologyResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(resp.Nodes))
	}

	// Verify topological order: storage -> db -> rotator -> pooler.
	posMap := make(map[string]int)
	for i, n := range resp.Nodes {
		posMap[n.Name] = i
	}
	if posMap["storage"] >= posMap["db"] {
		t.Errorf("storage (pos %d) should come before db (pos %d)", posMap["storage"], posMap["db"])
	}
	if posMap["db"] >= posMap["rotator"] {
		t.Errorf("db (pos %d) should come before rotator (pos %d)", posMap["db"], posMap["rotator"])
	}
	if posMap["rotator"] >= posMap["pooler"] {
		t.Errorf("rotator (pos %d) should come before pooler (pos %d)", posMap["rotator"], posMap["pooler"])
	}
}
