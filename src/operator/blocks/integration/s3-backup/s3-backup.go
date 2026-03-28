// Package s3backup implements the integration.s3-backup block, deploying scheduled
// database backups to S3-compatible storage via CronJob.
package s3backup

import (
	"context"
	"fmt"
	"strconv"

	"github.com/baiyuqing/ottoplus/src/core/block"
	blocks "github.com/baiyuqing/ottoplus/src/operator/blocks"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Block implements BlockRuntime for S3 backups.
type Block struct{}

func (b *Block) Descriptor() block.Descriptor {
	return block.Descriptor{
		Kind:        "integration.s3-backup",
		Category:    block.CategoryIntegration,
		Version:     "1.0.0",
		Description: "Scheduled database backups to S3-compatible storage.",
		Ports: []block.Port{
			{Name: "source-dsn", PortType: "dsn", Direction: block.PortInput, Required: true},
		},
		Parameters: []block.ParameterSpec{
			{Name: "schedule", Type: "string", Default: "0 2 * * *", Required: true, Description: "Cron expression for backup schedule."},
			{Name: "bucket", Type: "string", Required: true, Description: "S3 bucket path (e.g. s3://backups/cluster)."},
			{Name: "retentionDays", Type: "int", Default: "7", Description: "Days to retain backups."},
			{Name: "endpoint", Type: "string", Default: "http://localstack.localstack.svc:4566", Description: "S3 endpoint URL."},
		},
		Requires: []string{"datastore.*"},
	}
}

func (b *Block) ValidateParameters(_ context.Context, params map[string]string) error {
	if params["bucket"] == "" {
		return fmt.Errorf("parameter 'bucket' is required")
	}
	if v, ok := params["retentionDays"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("retentionDays must be an integer: %w", err)
		}
		if n < 1 {
			return fmt.Errorf("retentionDays must be >= 1, got %d", n)
		}
	}
	return nil
}

func (b *Block) Reconcile(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (blocks.ReconcileResult, error) {
	params := req.BlockRef.Parameters
	schedule := paramOrDefault(params, "schedule", "0 2 * * *")
	bucket := params["bucket"]
	retentionDays := paramOrDefault(params, "retentionDays", "7")
	endpoint := paramOrDefault(params, "endpoint", "http://localstack.localstack.svc:4566")
	sourceDSN := req.ResolvedInputs["source-dsn"]

	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	labels := map[string]string{
		"app.kubernetes.io/name":       "s3-backup",
		"app.kubernetes.io/instance":   fullName,
		"app.kubernetes.io/part-of":    "ottoplus",
		"app.kubernetes.io/managed-by": "ottoplus-operator",
		"ottoplus.io/cluster":          req.ClusterName,
		"ottoplus.io/block":            req.BlockRef.Name,
	}

	backupScript := fmt.Sprintf(`#!/bin/sh
set -e
TIMESTAMP=$(date +%%Y%%m%%d-%%H%%M%%S)
DUMP_FILE="/tmp/backup-${TIMESTAMP}.sql.gz"
echo "Starting backup at ${TIMESTAMP}"
pg_dump "%s" | gzip > "${DUMP_FILE}"
aws --endpoint-url "%s" s3 cp "${DUMP_FILE}" "%s/backup-${TIMESTAMP}.sql.gz"
echo "Backup uploaded to %s/backup-${TIMESTAMP}.sql.gz"
# Cleanup old backups
aws --endpoint-url "%s" s3 ls "%s/" | while read -r line; do
  FILE_DATE=$(echo "$line" | awk '{print $1}')
  if [ "$(( ($(date +%%s) - $(date -d "$FILE_DATE" +%%s)) / 86400 ))" -gt %s ]; then
    FILE_NAME=$(echo "$line" | awk '{print $4}')
    aws --endpoint-url "%s" s3 rm "%s/${FILE_NAME}"
    echo "Deleted old backup: ${FILE_NAME}"
  fi
done
echo "Backup complete"
`, sourceDSN, endpoint, bucket, bucket, endpoint, bucket, retentionDays, endpoint, bucket)

	if err := reconcileConfigMap(ctx, c, req.ClusterNamespace, fullName, labels, map[string]string{"backup.sh": backupScript}); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcileCronJob(ctx, c, req.ClusterNamespace, fullName, labels, schedule); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	return blocks.ReconcileResult{
		Phase:   block.PhaseReady,
		Message: fmt.Sprintf("Backup CronJob scheduled: %s", schedule),
		Outputs: map[string]string{},
	}, nil
}

func (b *Block) Delete(ctx context.Context, c client.Client, req blocks.ReconcileRequest) error {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	ns := req.ClusterNamespace
	_ = c.Delete(ctx, &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: fullName, Namespace: ns}})
	_ = c.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-config", Namespace: ns}})
	return nil
}

func (b *Block) HealthCheck(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (block.Phase, error) {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	var cj batchv1.CronJob
	if err := c.Get(ctx, types.NamespacedName{Name: fullName, Namespace: req.ClusterNamespace}, &cj); err != nil {
		if errors.IsNotFound(err) {
			return block.PhasePending, nil
		}
		return block.PhaseFailed, err
	}
	return block.PhaseReady, nil
}

func (b *Block) reconcileCronJob(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, schedule string) error {
	defaultMode := int32(0755)
	backoffLimit := int32(2)
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
		Spec: batchv1.CronJobSpec{
			Schedule:          schedule,
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					BackoffLimit: &backoffLimit,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: labels},
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{
								{
									Name:    "backup",
									Image:   "postgres:16",
									Command: []string{"/bin/sh", "/scripts/backup.sh"},
									VolumeMounts: []corev1.VolumeMount{
										{Name: "scripts", MountPath: "/scripts", ReadOnly: true},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "scripts",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{Name: name + "-config"},
											DefaultMode:          &defaultMode,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	existing := &batchv1.CronJob{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, cj)
	}
	if err != nil {
		return err
	}
	existing.Spec.Schedule = cj.Spec.Schedule
	existing.Spec.JobTemplate = cj.Spec.JobTemplate
	existing.Labels = labels
	return c.Update(ctx, existing)
}

func reconcileConfigMap(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, data map[string]string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-config", Namespace: namespace, Labels: labels},
		Data:       data,
	}
	existing := &corev1.ConfigMap{}
	err := c.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: namespace}, existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, cm)
	}
	if err != nil {
		return err
	}
	existing.Data = cm.Data
	existing.Labels = labels
	return c.Update(ctx, existing)
}

func paramOrDefault(params map[string]string, key, defaultValue string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return defaultValue
}
