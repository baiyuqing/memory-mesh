// Package localpv implements the storage.local-pv block, providing
// PersistentVolumeClaim specs for database engine StatefulSets.
package localpv

import (
	"context"
	"fmt"

	"github.com/baiyuqing/ottoplus/src/core/block"
	blocks "github.com/baiyuqing/ottoplus/src/operator/blocks"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Block implements BlockRuntime for local PersistentVolume storage.
type Block struct{}

func (b *Block) Descriptor() block.Descriptor {
	return block.Descriptor{
		Kind:        "storage.local-pv",
		Category:    block.CategoryStorage,
		Version:     "1.0.0",
		Description: "Local PersistentVolume storage for database data. Ephemeral — data does not survive node loss.",
		Ports: []block.Port{
			{Name: "pvc-spec", PortType: "pvc-spec", Direction: block.PortOutput},
		},
		Parameters: []block.ParameterSpec{
			{Name: "size", Type: "string", Default: "1Gi", Required: true, Description: "Storage size."},
			{Name: "storageClass", Type: "string", Default: "local-path", Description: "Kubernetes StorageClass name."},
		},
		Provides: []string{"pvc-spec"},
	}
}

func (b *Block) ValidateParameters(_ context.Context, params map[string]string) error {
	size := paramOrDefault(params, "size", "1Gi")
	if size == "" {
		return fmt.Errorf("parameter 'size' is required")
	}
	return nil
}

// Reconcile for local-pv is a no-op — it doesn't create standalone K8s
// resources. Instead, it outputs PVC spec parameters that the engine
// block embeds into its StatefulSet volumeClaimTemplates.
func (b *Block) Reconcile(_ context.Context, _ client.Client, req blocks.ReconcileRequest) (blocks.ReconcileResult, error) {
	size := paramOrDefault(req.BlockRef.Parameters, "size", "1Gi")
	storageClass := paramOrDefault(req.BlockRef.Parameters, "storageClass", "local-path")

	return blocks.ReconcileResult{
		Phase:   block.PhaseReady,
		Message: "PVC spec ready",
		Outputs: map[string]string{
			"pvc-spec":      fmt.Sprintf(`{"size":"%s","storageClass":"%s"}`, size, storageClass),
			"size":          size,
			"storageClass":  storageClass,
		},
	}, nil
}

func (b *Block) Delete(_ context.Context, _ client.Client, _ blocks.ReconcileRequest) error {
	return nil
}

func (b *Block) HealthCheck(_ context.Context, _ client.Client, _ blocks.ReconcileRequest) (block.Phase, error) {
	return block.PhaseReady, nil
}

func paramOrDefault(params map[string]string, key, defaultValue string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return defaultValue
}
