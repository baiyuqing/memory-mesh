// Package blocks defines the runtime interface that bridges domain-level
// block definitions to Kubernetes resource management.
package blocks

import (
	"context"

	"github.com/baiyuqing/ottoplus/src/core/block"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReconcileRequest carries all context a block runtime needs to
// reconcile itself within a Kubernetes cluster.
type ReconcileRequest struct {
	ClusterName      string
	ClusterNamespace string
	BlockRef         block.BlockRef
	// ResolvedInputs contains the concrete values produced by upstream
	// blocks' output ports, keyed by port name. For example, if an
	// engine block outputs a "dsn" port, the proxy block receives it
	// here as {"dsn": "postgresql://..."}.
	ResolvedInputs map[string]string
}

// ReconcileResult is returned after a block reconciliation.
type ReconcileResult struct {
	Phase   block.Phase
	Message string
	// Outputs are the resolved output port values that downstream
	// blocks can consume.
	Outputs map[string]string
}

// BlockRuntime is the infrastructure-aware counterpart to block.Block.
// Each block implementation registers a BlockRuntime that knows how to
// create, update, and delete Kubernetes resources for that block kind.
type BlockRuntime interface {
	block.Block

	// Reconcile creates or updates the Kubernetes resources for this
	// block. It is called by the composition reconciler in dependency
	// order. The method must be idempotent.
	Reconcile(ctx context.Context, c client.Client, request ReconcileRequest) (ReconcileResult, error)

	// Delete removes all Kubernetes resources owned by this block
	// instance. Called during composition teardown.
	Delete(ctx context.Context, c client.Client, request ReconcileRequest) error

	// HealthCheck returns the current phase of this block's resources.
	HealthCheck(ctx context.Context, c client.Client, request ReconcileRequest) (block.Phase, error)
}

// RuntimeRegistry holds BlockRuntime implementations keyed by block kind.
type RuntimeRegistry struct {
	runtimes map[string]BlockRuntime
}

// NewRuntimeRegistry creates an empty runtime registry.
func NewRuntimeRegistry() *RuntimeRegistry {
	return &RuntimeRegistry{runtimes: make(map[string]BlockRuntime)}
}

// Register adds a BlockRuntime to the registry.
func (r *RuntimeRegistry) Register(rt BlockRuntime) {
	r.runtimes[rt.Descriptor().Kind] = rt
}

// Get retrieves a BlockRuntime by block kind.
func (r *RuntimeRegistry) Get(kind string) (BlockRuntime, bool) {
	rt, ok := r.runtimes[kind]
	return rt, ok
}
