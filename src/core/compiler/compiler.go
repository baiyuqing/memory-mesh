// Package compiler provides the single entry point for processing
// block compositions. All consumers (API, operator, future UI) call
// into this package instead of assembling their own pipeline.
//
// The compilation pipeline runs these steps in order:
//  1. Expand shorthand ClusterSpec to Composition (if applicable)
//  2. NormalizeInputs — convert inline inputs to explicit wires
//  3. AutoWire — fill in unambiguous port matches
//  4. Validate — check block kinds, port types, required ports
//  5. TopologicalSort — order blocks by dependency
package compiler

import (
	"fmt"
	"strconv"

	"github.com/baiyuqing/ottoplus/src/core/block"
)

// ClusterSpec mirrors the CRD spec fields needed for expansion.
type ClusterSpec struct {
	Engine   string            `json:"engine,omitempty"`
	Replicas int               `json:"replicas,omitempty"`
	Version  string            `json:"version,omitempty"`
	Storage  string            `json:"storage,omitempty"`
	Backup   *BackupSpec       `json:"backup,omitempty"`
	Config   map[string]string `json:"config,omitempty"`
	Blocks   *BlocksSpec       `json:"blocks,omitempty"`
}

// BackupSpec holds shorthand backup config.
type BackupSpec struct {
	Enabled     bool   `json:"enabled"`
	Schedule    string `json:"schedule,omitempty"`
	Destination string `json:"destination,omitempty"`
}

// BlocksSpec holds explicit block composition from the CRD.
type BlocksSpec struct {
	Composition []block.BlockRef `json:"composition"`
	Wires       []block.Wire     `json:"wires,omitempty"`
}

// CompileResult holds the fully processed composition ready for
// execution or display.
type CompileResult struct {
	Composition block.Composition
	Sorted      []block.BlockRef
}

// Compile is the single entry point for turning a ClusterSpec into
// a fully validated, topologically sorted composition. Both the API
// and operator must use this function (or CompileComposition) instead
// of assembling their own pipeline.
func Compile(spec ClusterSpec, registry *block.Registry) (*CompileResult, []error) {
	comp, errs := expandToComposition(spec)
	if len(errs) > 0 {
		return nil, errs
	}
	return CompileComposition(comp, registry)
}

// CompileComposition processes an already-formed Composition through
// the normalize / auto-wire / validate / topo-sort pipeline. Use this
// when the caller already has a Composition (e.g. API endpoints that
// accept composition JSON directly).
func CompileComposition(comp block.Composition, registry *block.Registry) (*CompileResult, []error) {
	var allErrors []error

	if normErrs := comp.NormalizeInputs(); len(normErrs) > 0 {
		allErrors = append(allErrors, normErrs...)
	}

	if wireErrs := comp.AutoWire(registry); len(wireErrs) > 0 {
		allErrors = append(allErrors, wireErrs...)
	}

	if valErrs := comp.Validate(registry); len(valErrs) > 0 {
		allErrors = append(allErrors, valErrs...)
		return nil, allErrors
	}

	sorted, err := comp.TopologicalSort()
	if err != nil {
		allErrors = append(allErrors, err)
		return nil, allErrors
	}

	return &CompileResult{
		Composition: comp,
		Sorted:      sorted,
	}, allErrors
}

// expandToComposition converts a ClusterSpec into a Composition.
// If spec.Blocks is set, it is used directly. Otherwise, a default
// composition is synthesized from the shorthand fields.
func expandToComposition(spec ClusterSpec) (block.Composition, []error) {
	if spec.Blocks != nil {
		comp := block.Composition{
			Blocks: spec.Blocks.Composition,
			Wires:  spec.Blocks.Wires,
		}
		return comp, nil
	}

	replicas := spec.Replicas
	if replicas < 1 {
		replicas = 1
	}
	version := spec.Version
	if version == "" {
		version = "16"
	}
	storage := spec.Storage
	if storage == "" {
		storage = "1Gi"
	}

	if spec.Engine == "" {
		return block.Composition{}, []error{fmt.Errorf("engine is required when not using explicit blocks")}
	}

	engineKind := "datastore." + spec.Engine

	comp := block.Composition{
		Blocks: []block.BlockRef{
			{
				Kind: "storage.local-pv",
				Name: "default-storage",
				Parameters: map[string]string{
					"size": storage,
				},
			},
			{
				Kind: engineKind,
				Name: "default-engine",
				Parameters: map[string]string{
					"version":  version,
					"replicas": strconv.Itoa(replicas),
				},
			},
		},
	}

	if spec.Config != nil {
		for k, v := range spec.Config {
			comp.Blocks[1].Parameters[k] = v
		}
	}

	if spec.Backup != nil && spec.Backup.Enabled {
		comp.Blocks = append(comp.Blocks, block.BlockRef{
			Kind: "integration.s3-backup",
			Name: "default-backup",
			Parameters: map[string]string{
				"schedule": spec.Backup.Schedule,
				"bucket":   spec.Backup.Destination,
			},
		})
	}

	return comp, nil
}
