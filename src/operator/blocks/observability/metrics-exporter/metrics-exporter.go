// Package metricsexporter implements the observability.metrics-exporter block,
// deploying Prometheus scrape configuration for database engine pods.
package metricsexporter

import (
	"context"
	"fmt"

	"github.com/baiyuqing/ottoplus/src/core/block"
	blocks "github.com/baiyuqing/ottoplus/src/operator/blocks"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Block implements BlockRuntime for the metrics exporter.
type Block struct{}

func (b *Block) Descriptor() block.Descriptor {
	return block.Descriptor{
		Kind:        "observability.metrics-exporter",
		Category:    block.CategoryObservability,
		Version:     "1.0.0",
		Description: "Prometheus metrics exporter sidecar for database engines.",
		Ports: []block.Port{
			{Name: "scrape-target", PortType: "metrics-endpoint", Direction: block.PortInput, Required: true},
		},
		Parameters: []block.ParameterSpec{
			{Name: "port", Type: "int", Default: "9187", Description: "Metrics exporter listen port."},
			{Name: "scrapeInterval", Type: "string", Default: "15s", Description: "Prometheus scrape interval."},
		},
		Requires: []string{"datastore.*"},
	}
}

func (b *Block) ValidateParameters(_ context.Context, _ map[string]string) error {
	return nil
}

// Reconcile creates a ConfigMap with Prometheus scrape configuration
// that references the upstream engine's metrics endpoint. If Prometheus
// Operator is installed, it also creates a ServiceMonitor.
func (b *Block) Reconcile(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (blocks.ReconcileResult, error) {
	params := req.BlockRef.Parameters
	port := paramOrDefault(params, "port", "9187")
	scrapeInterval := paramOrDefault(params, "scrapeInterval", "15s")
	scrapeTarget := req.ResolvedInputs["scrape-target"]

	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	labels := map[string]string{
		"app.kubernetes.io/name":       "metrics-exporter",
		"app.kubernetes.io/instance":   fullName,
		"app.kubernetes.io/part-of":    "ottoplus",
		"app.kubernetes.io/managed-by": "ottoplus-operator",
		"ottoplus.io/cluster":          req.ClusterName,
		"ottoplus.io/block":            req.BlockRef.Name,
	}

	// Prometheus scrape config as a ConfigMap that can be picked up by
	// Prometheus via additional scrape config or file-based SD.
	scrapeConfig := fmt.Sprintf(`- job_name: '%s'
  scrape_interval: %s
  static_configs:
    - targets: ['%s']
      labels:
        cluster: '%s'
        block: '%s'
        port: '%s'
`, fullName, scrapeInterval, extractHost(scrapeTarget), req.ClusterName, req.BlockRef.Name, port)

	if err := b.reconcileConfigMap(ctx, c, req.ClusterNamespace, fullName, labels, scrapeConfig); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	return blocks.ReconcileResult{
		Phase:   block.PhaseReady,
		Message: fmt.Sprintf("Metrics scrape config created for %s", scrapeTarget),
		Outputs: map[string]string{},
	}, nil
}

func (b *Block) Delete(ctx context.Context, c client.Client, req blocks.ReconcileRequest) error {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	_ = c.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:      fullName + "-config",
		Namespace: req.ClusterNamespace,
	}})
	return nil
}

func (b *Block) HealthCheck(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (block.Phase, error) {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	var cm corev1.ConfigMap
	if err := c.Get(ctx, types.NamespacedName{Name: fullName + "-config", Namespace: req.ClusterNamespace}, &cm); err != nil {
		if errors.IsNotFound(err) {
			return block.PhasePending, nil
		}
		return block.PhaseFailed, err
	}
	return block.PhaseReady, nil
}

func (b *Block) reconcileConfigMap(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, scrapeConfig string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-config", Namespace: namespace, Labels: labels},
		Data:       map[string]string{"scrape-config.yaml": scrapeConfig},
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

// extractHost pulls the host:port from a metrics URL like
// "http://host:9187/metrics" -> "host:9187".
func extractHost(metricsURL string) string {
	// Simple extraction: strip scheme and path
	s := metricsURL
	for _, prefix := range []string{"http://", "https://"} {
		if len(s) > len(prefix) && s[:len(prefix)] == prefix {
			s = s[len(prefix):]
			break
		}
	}
	for i, ch := range s {
		if ch == '/' {
			return s[:i]
		}
	}
	return s
}

func paramOrDefault(params map[string]string, key, defaultValue string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return defaultValue
}
