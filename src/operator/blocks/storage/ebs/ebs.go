// Package ebs implements the storage.ebs block, provisioning AWS EBS
// volumes via a Kubernetes StorageClass.
package ebs

import (
	"context"
	"fmt"

	"github.com/baiyuqing/ottoplus/src/core/block"
	blocks "github.com/baiyuqing/ottoplus/src/operator/blocks"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Block implements BlockRuntime for AWS EBS storage.
type Block struct{}

func (b *Block) Descriptor() block.Descriptor {
	return block.Descriptor{
		Kind:        "storage.ebs",
		Category:    block.CategoryStorage,
		Version:     "1.0.0",
		Description: "AWS EBS storage provisioner. Uses StorageClass with EBS CSI driver (simulated via LocalStack for local dev).",
		Ports: []block.Port{
			{Name: "pvc-spec", PortType: "pvc-spec", Direction: block.PortOutput},
		},
		Parameters: []block.ParameterSpec{
			{Name: "size", Type: "string", Default: "10Gi", Required: true, Description: "Volume size."},
			{Name: "volumeType", Type: "string", Default: "gp3", Description: "EBS volume type."},
			{Name: "iops", Type: "int", Default: "3000", Description: "Provisioned IOPS."},
			{Name: "encrypted", Type: "string", Default: "true", Description: "Enable encryption."},
			{Name: "storageClass", Type: "string", Default: "ebs-sc", Description: "StorageClass name."},
		},
		Provides: []string{"pvc-spec"},
	}
}

func (b *Block) ValidateParameters(_ context.Context, params map[string]string) error {
	validTypes := map[string]bool{"gp2": true, "gp3": true, "io1": true, "io2": true}
	if vt, ok := params["volumeType"]; ok && !validTypes[vt] {
		return fmt.Errorf("invalid volumeType %q, must be gp2/gp3/io1/io2", vt)
	}
	return nil
}

func (b *Block) Reconcile(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (blocks.ReconcileResult, error) {
	params := req.BlockRef.Parameters
	size := paramOrDefault(params, "size", "10Gi")
	volumeType := paramOrDefault(params, "volumeType", "gp3")
	iops := paramOrDefault(params, "iops", "3000")
	encrypted := paramOrDefault(params, "encrypted", "true")
	storageClassName := paramOrDefault(params, "storageClass", "ebs-sc")

	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)

	if err := b.reconcileStorageClass(ctx, c, fullName, storageClassName, volumeType, iops, encrypted); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	return blocks.ReconcileResult{
		Phase:   block.PhaseReady,
		Message: "EBS StorageClass ready",
		Outputs: map[string]string{
			"pvc-spec":     fmt.Sprintf(`{"size":"%s","storageClass":"%s"}`, size, storageClassName),
			"size":         size,
			"storageClass": storageClassName,
		},
	}, nil
}

func (b *Block) Delete(ctx context.Context, c client.Client, req blocks.ReconcileRequest) error {
	storageClassName := paramOrDefault(req.BlockRef.Parameters, "storageClass", "ebs-sc")
	_ = c.Delete(ctx, &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: storageClassName}})
	return nil
}

func (b *Block) HealthCheck(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (block.Phase, error) {
	storageClassName := paramOrDefault(req.BlockRef.Parameters, "storageClass", "ebs-sc")
	var sc storagev1.StorageClass
	if err := c.Get(ctx, types.NamespacedName{Name: storageClassName}, &sc); err != nil {
		if errors.IsNotFound(err) {
			return block.PhasePending, nil
		}
		return block.PhaseFailed, err
	}
	return block.PhaseReady, nil
}

func (b *Block) reconcileStorageClass(ctx context.Context, c client.Client, fullName, name, volumeType, iops, encrypted string) error {
	provisioner := "ebs.csi.aws.com"
	reclaimPolicy := corev1.PersistentVolumeReclaimDelete
	bindingMode := storagev1.VolumeBindingWaitForFirstConsumer

	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/part-of":    "ottoplus",
				"app.kubernetes.io/managed-by": "ottoplus-operator",
				"ottoplus.io/block":            fullName,
			},
		},
		Provisioner:       provisioner,
		ReclaimPolicy:     &reclaimPolicy,
		VolumeBindingMode: &bindingMode,
		Parameters: map[string]string{
			"type":      volumeType,
			"iops":      iops,
			"encrypted": encrypted,
		},
	}

	existing := &storagev1.StorageClass{}
	err := c.Get(ctx, types.NamespacedName{Name: name}, existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, sc)
	}
	if err != nil {
		return err
	}
	existing.Parameters = sc.Parameters
	existing.Labels = sc.Labels
	return c.Update(ctx, existing)
}

func paramOrDefault(params map[string]string, key, defaultValue string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return defaultValue
}
