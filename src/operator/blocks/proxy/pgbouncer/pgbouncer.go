// Package pgbouncer implements the proxy.pgbouncer block, deploying a
// PgBouncer connection pooler in front of a PostgreSQL engine block.
package pgbouncer

import (
	"context"
	"fmt"
	"strconv"

	"github.com/baiyuqing/ottoplus/src/core/block"
	blocks "github.com/baiyuqing/ottoplus/src/operator/blocks"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Block implements BlockRuntime for PgBouncer.
type Block struct{}

func (b *Block) Descriptor() block.Descriptor {
	return block.Descriptor{
		Kind:        "proxy.pgbouncer",
		Category:    block.CategoryProxy,
		Version:     "1.0.0",
		Description: "PgBouncer connection pooler for PostgreSQL.",
		Ports: []block.Port{
			{Name: "upstream-dsn", PortType: "dsn", Direction: block.PortInput, Required: true},
			{Name: "dsn", PortType: "dsn", Direction: block.PortOutput},
			{Name: "metrics", PortType: "metrics-endpoint", Direction: block.PortOutput},
		},
		Parameters: []block.ParameterSpec{
			{Name: "poolMode", Type: "string", Default: "transaction", Description: "Pool mode: session, transaction, or statement."},
			{Name: "maxClientConnections", Type: "int", Default: "500", Description: "Maximum client connections."},
			{Name: "defaultPoolSize", Type: "int", Default: "20", Description: "Default pool size per user/database pair."},
		},
		Requires: []string{"engine.postgresql"},
		Provides: []string{"dsn", "metrics-endpoint"},
	}
}

func (b *Block) ValidateParameters(_ context.Context, params map[string]string) error {
	validModes := map[string]bool{"session": true, "transaction": true, "statement": true}
	if mode, ok := params["poolMode"]; ok && !validModes[mode] {
		return fmt.Errorf("invalid poolMode %q, must be session/transaction/statement", mode)
	}
	return nil
}

func (b *Block) Reconcile(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (blocks.ReconcileResult, error) {
	params := req.BlockRef.Parameters
	poolMode := paramOrDefault(params, "poolMode", "transaction")
	maxClient := paramOrDefault(params, "maxClientConnections", "500")
	poolSize := paramOrDefault(params, "defaultPoolSize", "20")
	upstreamDSN := req.ResolvedInputs["upstream-dsn"]

	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	labels := blockLabels(fullName, req.ClusterName, req.BlockRef.Name)

	pgbouncerINI := fmt.Sprintf(
		"[databases]\n* = %s\n\n[pgbouncer]\nlisten_addr = 0.0.0.0\nlisten_port = 6432\nauth_type = any\npool_mode = %s\nmax_client_conn = %s\ndefault_pool_size = %s\nadmin_users = pgbouncer\nstats_users = pgbouncer\n",
		upstreamDSN, poolMode, maxClient, poolSize,
	)

	if err := reconcileConfigMap(ctx, c, req.ClusterNamespace, fullName, labels, map[string]string{"pgbouncer.ini": pgbouncerINI}); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	replicas := int32(2)
	if err := b.reconcileDeployment(ctx, c, req.ClusterNamespace, fullName, labels, replicas); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := reconcileClusterIPService(ctx, c, req.ClusterNamespace, fullName, labels, "pgbouncer", 6432); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	dsn := fmt.Sprintf("postgresql://pgbouncer@%s.%s.svc:6432/postgres", fullName, req.ClusterNamespace)

	return blocks.ReconcileResult{
		Phase:   block.PhaseReady,
		Message: "PgBouncer Deployment reconciled",
		Outputs: map[string]string{
			"dsn":     dsn,
			"metrics": fmt.Sprintf("http://%s.%s.svc:9127/metrics", fullName, req.ClusterNamespace),
		},
	}, nil
}

func (b *Block) Delete(ctx context.Context, c client.Client, req blocks.ReconcileRequest) error {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	ns := req.ClusterNamespace
	_ = c.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: fullName, Namespace: ns}})
	_ = c.Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: fullName, Namespace: ns}})
	_ = c.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-config", Namespace: ns}})
	return nil
}

func (b *Block) HealthCheck(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (block.Phase, error) {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	var deploy appsv1.Deployment
	if err := c.Get(ctx, types.NamespacedName{Name: fullName, Namespace: req.ClusterNamespace}, &deploy); err != nil {
		if errors.IsNotFound(err) {
			return block.PhasePending, nil
		}
		return block.PhaseFailed, err
	}
	if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
		return block.PhaseReady, nil
	}
	return block.PhaseProvisioning, nil
}

func (b *Block) reconcileDeployment(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, replicas int32) error {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/instance": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "pgbouncer",
							Image: "bitnami/pgbouncer:1.22.0",
							Ports: []corev1.ContainerPort{
								{Name: "pgbouncer", ContainerPort: 6432, Protocol: corev1.ProtocolTCP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "config", MountPath: "/bitnami/pgbouncer/conf", ReadOnly: true},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(6432)},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: name + "-config"},
								},
							},
						},
					},
				},
			},
		},
	}
	existing := &appsv1.Deployment{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, deploy)
	}
	if err != nil {
		return err
	}
	existing.Spec.Replicas = deploy.Spec.Replicas
	existing.Spec.Template = deploy.Spec.Template
	existing.Labels = labels
	return c.Update(ctx, existing)
}

// Shared helpers to reduce duplication across blocks.

func blockLabels(fullName, clusterName, blockName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/instance":   fullName,
		"app.kubernetes.io/part-of":    "ottoplus",
		"app.kubernetes.io/managed-by": "ottoplus-operator",
		"ottoplus.io/cluster":          clusterName,
		"ottoplus.io/block":            blockName,
	}
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

func reconcileClusterIPService(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, portName string, port int32) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app.kubernetes.io/instance": name},
			Ports:    []corev1.ServicePort{{Name: portName, Port: port, TargetPort: intstr.FromInt32(port), Protocol: corev1.ProtocolTCP}},
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
	existing := &corev1.Service{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, existing)
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

func paramOrDefault(params map[string]string, key, defaultValue string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return defaultValue
}

func atoi(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}
