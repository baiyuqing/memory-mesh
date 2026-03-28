// Package api implements the control plane HTTP API server.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/baiyuqing/ottoplus/src/core/block"
)

// Server is the control plane API server.
type Server struct {
	registry *block.Registry
	mux      *http.ServeMux
}

// NewServer creates an API server with the given block registry.
func NewServer(registry *block.Registry) *Server {
	s := &Server{
		registry: registry,
		mux:      http.NewServeMux(),
	}
	s.routes()
	return s
}

// Handler returns the HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)
	s.mux.HandleFunc("GET /v1/blocks", s.handleListBlocks)
	s.mux.HandleFunc("GET /v1/blocks/{kind}", s.handleGetBlock)
	s.mux.HandleFunc("POST /v1/compositions/validate", s.handleValidateComposition)
	s.mux.HandleFunc("POST /v1/compositions/auto-wire", s.handleAutoWire)
	s.mux.HandleFunc("POST /v1/compositions/topology", s.handleTopology)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
