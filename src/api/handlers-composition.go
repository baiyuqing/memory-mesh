package api

import (
	"net/http"

	"github.com/baiyuqing/ottoplus/src/core/block"
)

// ValidateRequest is the request body for POST /v1/compositions/validate.
type ValidateRequest struct {
	Composition block.Composition `json:"composition"`
}

// ValidateResponse is the response for POST /v1/compositions/validate.
type ValidateResponse struct {
	IsValid bool     `json:"isValid"`
	Errors  []string `json:"errors,omitempty"`
}

// handleValidateComposition validates a composition against the registry.
//
//	POST /v1/compositions/validate
func (s *Server) handleValidateComposition(w http.ResponseWriter, r *http.Request) {
	var req ValidateRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	normErrs := req.Composition.NormalizeInputs()
	errs := req.Composition.Validate(s.registry)
	errs = append(normErrs, errs...)

	resp := ValidateResponse{IsValid: len(errs) == 0}
	for _, e := range errs {
		resp.Errors = append(resp.Errors, e.Error())
	}

	status := http.StatusOK
	if !resp.IsValid {
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, resp)
}

// AutoWireRequest is the request body for POST /v1/compositions/auto-wire.
type AutoWireRequest struct {
	Composition block.Composition `json:"composition"`
}

// AutoWireResponse is the response for POST /v1/compositions/auto-wire.
type AutoWireResponse struct {
	Composition block.Composition `json:"composition"`
	Warnings    []string          `json:"warnings,omitempty"`
}

// handleAutoWire auto-wires a composition and returns the result with
// inferred wires filled in.
//
//	POST /v1/compositions/auto-wire
func (s *Server) handleAutoWire(w http.ResponseWriter, r *http.Request) {
	var req AutoWireRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	comp := req.Composition
	normErrs := comp.NormalizeInputs()
	errs := comp.AutoWire(s.registry)
	errs = append(normErrs, errs...)

	resp := AutoWireResponse{Composition: comp}
	for _, e := range errs {
		resp.Warnings = append(resp.Warnings, e.Error())
	}

	writeJSON(w, http.StatusOK, resp)
}

// TopologyNode represents a block in the topologically sorted graph.
type TopologyNode struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Category string   `json:"category"`
	DependsOn []string `json:"dependsOn"`
}

// TopologyResponse is the response for POST /v1/compositions/topology.
type TopologyResponse struct {
	Nodes []TopologyNode `json:"nodes"`
	Wires []block.Wire   `json:"wires"`
	Error string         `json:"error,omitempty"`
}

// handleTopology returns the topological sort order and dependency graph
// for a composition. This is the primary data source for visual editors.
//
//	POST /v1/compositions/topology
func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	var req AutoWireRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	comp := req.Composition
	comp.NormalizeInputs()
	comp.AutoWire(s.registry)

	sorted, err := comp.TopologicalSort()
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, TopologyResponse{Error: err.Error()})
		return
	}

	// Build dependency map from wires
	deps := make(map[string][]string)
	for _, w := range comp.Wires {
		deps[w.ToBlock] = append(deps[w.ToBlock], w.FromBlock)
	}

	nodes := make([]TopologyNode, 0, len(sorted))
	for _, ref := range sorted {
		category := ""
		if b, ok := s.registry.Get(ref.Kind); ok {
			category = string(b.Descriptor().Category)
		}
		nodes = append(nodes, TopologyNode{
			Name:      ref.Name,
			Kind:      ref.Kind,
			Category:  category,
			DependsOn: deps[ref.Name],
		})
	}

	writeJSON(w, http.StatusOK, TopologyResponse{
		Nodes: nodes,
		Wires: comp.Wires,
	})
}
