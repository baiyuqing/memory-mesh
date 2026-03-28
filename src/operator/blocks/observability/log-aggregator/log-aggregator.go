// Package logaggregator implements the observability.log-aggregator block,
// deploying Loki as a StatefulSet and Promtail as a DaemonSet for
// centralized log aggregation.
package logaggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/baiyuqing/ottoplus/src/core/block"
	blocks "github.com/baiyuqing/ottoplus/src/operator/blocks"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Block implements BlockRuntime for the log aggregator.
type Block struct{}

func (b *Block) Descriptor() block.Descriptor {
	return block.Descriptor{
		Kind:        "observability.log-aggregator",
		Category:    block.CategoryObservability,
		Version:     "1.0.0",
		Description: "Centralized log aggregation with Loki and Promtail.",
		Ports: []block.Port{
			{Name: "storage", PortType: "pvc-spec", Direction: block.PortInput},
			{Name: "log-endpoint", PortType: "log-endpoint", Direction: block.PortOutput},
		},
		Parameters: []block.ParameterSpec{
			{Name: "retentionDays", Type: "int", Default: "7", Description: "Log retention period in days."},
			{Name: "lokiVersion", Type: "string", Default: "2.9.4", Description: "Loki container image version."},
			{Name: "promtailVersion", Type: "string", Default: "2.9.4", Description: "Promtail container image version."},
		},
		Requires:  []string{},
		Provides:  []string{"log-endpoint"},
	}
}

// ValidateParameters checks that retentionDays is a positive integer.
func (b *Block) ValidateParameters(_ context.Context, params map[string]string) error {
	if v, ok := params["retentionDays"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("retentionDays must be an integer: %w", err)
		}
		if n < 1 {
			return fmt.Errorf("retentionDays must be positive, got %d", n)
		}
	}
	return nil
}

// Reconcile creates or updates all Kubernetes resources for the log
// aggregator: Loki StatefulSet, Promtail DaemonSet, Services, ConfigMaps,
// and a ServiceAccount for Promtail.
func (b *Block) Reconcile(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (blocks.ReconcileResult, error) {
	params := req.BlockRef.Parameters
	retentionDays := paramOrDefault(params, "retentionDays", "7")
	lokiVersion := paramOrDefault(params, "lokiVersion", "2.9.4")
	promtailVersion := paramOrDefault(params, "promtailVersion", "2.9.4")

	storageSize := "10Gi"
	storageClass := "local-path"
	if pvcSpec, ok := req.ResolvedInputs["storage"]; ok {
		var spec map[string]string
		if err := json.Unmarshal([]byte(pvcSpec), &spec); err == nil {
			if s, ok := spec["size"]; ok {
				storageSize = s
			}
			if sc, ok := spec["storageClass"]; ok {
				storageClass = sc
			}
		}
	}

	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	labels := blockLabels(fullName, req.ClusterName, req.BlockRef.Name)

	if err := b.reconcileLokiConfigMap(ctx, c, req.ClusterNamespace, fullName, labels, retentionDays); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcilePromtailConfigMap(ctx, c, req.ClusterNamespace, fullName, labels); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcileServiceAccount(ctx, c, req.ClusterNamespace, fullName, labels); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcileLokiStatefulSet(ctx, c, req.ClusterNamespace, fullName, labels, lokiVersion, storageSize, storageClass); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcilePromtailDaemonSet(ctx, c, req.ClusterNamespace, fullName, labels, promtailVersion); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcileLokiService(ctx, c, req.ClusterNamespace, fullName, labels); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	logEndpoint := fmt.Sprintf("http://%s-loki.%s.svc:3100/loki/api/v1/push", fullName, req.ClusterNamespace)

	return blocks.ReconcileResult{
		Phase:   block.PhaseReady,
		Message: "Loki StatefulSet and Promtail DaemonSet reconciled",
		Outputs: map[string]string{
			"log-endpoint": logEndpoint,
		},
	}, nil
}

// Delete removes all Kubernetes resources owned by this block instance.
func (b *Block) Delete(ctx context.Context, c client.Client, req blocks.ReconcileRequest) error {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	ns := req.ClusterNamespace

	_ = c.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-loki", Namespace: ns}})
	_ = c.Delete(ctx, &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-promtail", Namespace: ns}})
	_ = c.Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-loki", Namespace: ns}})
	_ = c.Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-promtail", Namespace: ns}})
	_ = c.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-promtail-config", Namespace: ns}})
	_ = c.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-loki-config", Namespace: ns}})
	return nil
}

// HealthCheck returns the current phase by checking the Loki StatefulSet.
func (b *Block) HealthCheck(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (block.Phase, error) {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	var sts appsv1.StatefulSet
	if err := c.Get(ctx, types.NamespacedName{Name: fullName + "-loki", Namespace: req.ClusterNamespace}, &sts); err != nil {
		if errors.IsNotFound(err) {
			return block.PhasePending, nil
		}
		return block.PhaseFailed, err
	}
	if sts.Status.ReadyReplicas == *sts.Spec.Replicas {
		return block.PhaseReady, nil
	}
	return block.PhaseProvisioning, nil
}

func (b *Block) reconcileLokiConfigMap(ctx context.Context, c client.Client, namespace, fullName string, labels map[string]string, retentionDays string) error {
	lokiConfig := fmt.Sprintf(`auth_enabled: false

server:
  http_listen_port: 3100

ingester:
  lifecycler:
    ring:
      kvstore:
        store: inmemory
      replication_factor: 1
  chunk_idle_period: 5m
  chunk_retain_period: 30s

schema_config:
  configs:
    - from: "2020-01-01"
      store: boltdb-shipper
      object_store: filesystem
      schema: v11
      index:
        prefix: index_
        period: 24h

storage_config:
  boltdb_shipper:
    active_index_directory: /loki/index
    cache_location: /loki/boltdb-cache
    shared_store: filesystem
  filesystem:
    directory: /loki/chunks

limits_config:
  retention_period: %sd

compactor:
  working_directory: /loki/compactor
  shared_store: filesystem
  retention_enabled: true
`, retentionDays)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fullName + "-loki-config",
			Namespace: namespace,
			Labels:    labels,
		},
		Data: map[string]string{"loki.yaml": lokiConfig},
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

func (b *Block) reconcilePromtailConfigMap(ctx context.Context, c client.Client, namespace, fullName string, labels map[string]string) error {
	promtailConfig := fmt.Sprintf(`server:
  http_listen_port: 9080

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://%s-loki.%s.svc:3100/loki/api/v1/push

scrape_configs:
  - job_name: kubernetes-pods
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_ottoplus_io_cluster]
        target_label: cluster
      - source_labels: [__meta_kubernetes_namespace]
        target_label: namespace
      - source_labels: [__meta_kubernetes_pod_name]
        target_label: pod
    pipeline_stages:
      - docker: {}
`, fullName, namespace)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fullName + "-promtail-config",
			Namespace: namespace,
			Labels:    labels,
		},
		Data: map[string]string{"promtail.yaml": promtailConfig},
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

func (b *Block) reconcileServiceAccount(ctx context.Context, c client.Client, namespace, fullName string, labels map[string]string) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fullName + "-promtail",
			Namespace: namespace,
			Labels:    labels,
		},
	}
	existing := &corev1.ServiceAccount{}
	err := c.Get(ctx, types.NamespacedName{Name: sa.Name, Namespace: namespace}, existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, sa)
	}
	if err != nil {
		return err
	}
	existing.Labels = labels
	return c.Update(ctx, existing)
}

func (b *Block) reconcileLokiStatefulSet(ctx context.Context, c client.Client, namespace, fullName string, labels map[string]string, lokiVersion, storageSize, storageClass string) error {
	image := fmt.Sprintf("grafana/loki:%s", lokiVersion)
	qty := resource.MustParse(storageSize)
	replicas := int32(1)

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fullName + "-loki",
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: fullName + "-loki",
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/instance":  fullName + "-loki",
					"app.kubernetes.io/component": "loki",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":       "log-aggregator",
						"app.kubernetes.io/instance":   fullName + "-loki",
						"app.kubernetes.io/component":  "loki",
						"app.kubernetes.io/part-of":    "ottoplus",
						"app.kubernetes.io/managed-by": "ottoplus-operator",
						"ottoplus.io/cluster":          labels["ottoplus.io/cluster"],
						"ottoplus.io/block":            labels["ottoplus.io/block"],
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "loki",
							Image: image,
							Args:  []string{"-config.file=/etc/loki/loki.yaml"},
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: 3100, Protocol: corev1.ProtocolTCP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "data", MountPath: "/loki"},
								{Name: "config", MountPath: "/etc/loki", ReadOnly: true},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromInt(3100),
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       5,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: fullName + "-loki-config",
									},
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						StorageClassName: &storageClass,
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: qty,
							},
						},
					},
				},
			},
		},
	}

	existing := &appsv1.StatefulSet{}
	err := c.Get(ctx, types.NamespacedName{Name: sts.Name, Namespace: namespace}, existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, sts)
	}
	if err != nil {
		return err
	}
	existing.Spec.Template = sts.Spec.Template
	existing.Labels = labels
	return c.Update(ctx, existing)
}

func (b *Block) reconcilePromtailDaemonSet(ctx context.Context, c client.Client, namespace, fullName string, labels map[string]string, promtailVersion string) error {
	image := fmt.Sprintf("grafana/promtail:%s", promtailVersion)
	hostPathType := corev1.HostPathDirectory

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fullName + "-promtail",
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/instance":  fullName + "-promtail",
					"app.kubernetes.io/component": "promtail",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":       "log-aggregator",
						"app.kubernetes.io/instance":   fullName + "-promtail",
						"app.kubernetes.io/component":  "promtail",
						"app.kubernetes.io/part-of":    "ottoplus",
						"app.kubernetes.io/managed-by": "ottoplus-operator",
						"ottoplus.io/cluster":          labels["ottoplus.io/cluster"],
						"ottoplus.io/block":            labels["ottoplus.io/block"],
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: fullName + "-promtail",
					Containers: []corev1.Container{
						{
							Name:  "promtail",
							Image: image,
							Args:  []string{"-config.file=/etc/promtail/promtail.yaml"},
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: 9080, Protocol: corev1.ProtocolTCP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "config", MountPath: "/etc/promtail", ReadOnly: true},
								{Name: "varlog", MountPath: "/var/log", ReadOnly: true},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: fullName + "-promtail-config",
									},
								},
							},
						},
						{
							Name: "varlog",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/log",
									Type: &hostPathType,
								},
							},
						},
					},
				},
			},
		},
	}

	existing := &appsv1.DaemonSet{}
	err := c.Get(ctx, types.NamespacedName{Name: ds.Name, Namespace: namespace}, existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, ds)
	}
	if err != nil {
		return err
	}
	existing.Spec.Template = ds.Spec.Template
	existing.Labels = labels
	return c.Update(ctx, existing)
}

func (b *Block) reconcileLokiService(ctx context.Context, c client.Client, namespace, fullName string, labels map[string]string) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fullName + "-loki",
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app.kubernetes.io/instance":  fullName + "-loki",
				"app.kubernetes.io/component": "loki",
			},
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 3100, TargetPort: intstr.FromInt(3100), Protocol: corev1.ProtocolTCP},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	existing := &corev1.Service{}
	err := c.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: namespace}, existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, svc)
	}
	if err != nil {
		return err
	}
	existing.Spec.Ports = svc.Spec.Ports
	existing.Spec.Selector = svc.Spec.Selector
	return c.Update(ctx, existing)
}

func blockLabels(fullName, clusterName, blockName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "log-aggregator",
		"app.kubernetes.io/instance":   fullName,
		"app.kubernetes.io/part-of":    "ottoplus",
		"app.kubernetes.io/managed-by": "ottoplus-operator",
		"ottoplus.io/cluster":          clusterName,
		"ottoplus.io/block":            blockName,
	}
}

func paramOrDefault(params map[string]string, key, defaultValue string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return defaultValue
}
