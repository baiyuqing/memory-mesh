// Package reconciler implements the top-level composition reconciler
// that orchestrates block reconciliation for Cluster CRs.
package reconciler

import (
	"strconv"

	"github.com/baiyuqing/ottoplus/src/core/block"
)

// ClusterSpec mirrors the CRD spec fields needed for expansion.
// This avoids importing generated CRD types during early development.
type ClusterSpec struct {
	// Shorthand fields
	Engine   string            `json:"engine,omitempty"`
	Replicas int               `json:"replicas,omitempty"`
	Version  string            `json:"version,omitempty"`
	Storage  string            `json:"storage,omitempty"`
	Backup   *BackupSpec       `json:"backup,omitempty"`
	Config   map[string]string `json:"config,omitempty"`

	// Explicit block composition
	Blocks *BlocksSpec `json:"blocks,omitempty"`
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

// ExpandToComposition converts a ClusterSpec into a Composition.
// If spec.Blocks is set, it is used directly. Otherwise, a default
// composition is synthesized from the shorthand fields.
func ExpandToComposition(spec ClusterSpec) (block.Composition, []error) {
	if spec.Blocks != nil {
		comp := block.Composition{
			Blocks: spec.Blocks.Composition,
			Wires:  spec.Blocks.Wires,
		}
		errs := comp.NormalizeInputs()
		return comp, errs
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
			Kind: "compute.s3-backup",
			Name: "default-backup",
			Parameters: map[string]string{
				"schedule": spec.Backup.Schedule,
				"bucket":   spec.Backup.Destination,
			},
		})
	}

	return comp, nil
}
