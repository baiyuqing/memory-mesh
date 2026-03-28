package api

import (
	"net/http"

	"github.com/baiyuqing/ottoplus/src/core/block"
)

// BlockListResponse is the response for GET /v1/blocks.
type BlockListResponse struct {
	Blocks []block.Descriptor `json:"blocks"`
}

// handleListBlocks returns all registered block descriptors.
// Supports optional ?category= query parameter for filtering.
//
//	GET /v1/blocks
//	GET /v1/blocks?category=engine
func (s *Server) handleListBlocks(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

	var descriptors []block.Descriptor
	if category != "" {
		descriptors = s.registry.ListByCategory(block.Category(category))
	} else {
		descriptors = s.registry.List()
	}

	writeJSON(w, http.StatusOK, BlockListResponse{Blocks: descriptors})
}

// handleGetBlock returns a single block descriptor by kind.
//
//	GET /v1/blocks/{kind}
func (s *Server) handleGetBlock(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	b, ok := s.registry.Get(kind)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "block not found: " + kind,
		})
		return
	}
	writeJSON(w, http.StatusOK, b.Descriptor())
}
