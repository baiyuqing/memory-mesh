// Package postgresql implements the engine.postgresql block, managing
// PostgreSQL instances as Kubernetes StatefulSets.
package postgresql

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

// Block implements BlockRuntime for PostgreSQL.
type Block struct{}

func (b *Block) Descriptor() block.Descriptor {
	return block.Descriptor{
		Kind:        "engine.postgresql",
		Category:    block.CategoryEngine,
		Version:     "1.0.0",
		Description: "PostgreSQL database engine managed as a Kubernetes StatefulSet.",
		Ports: []block.Port{
			{Name: "dsn", PortType: "dsn", Direction: block.PortOutput},
			{Name: "metrics", PortType: "metrics-endpoint", Direction: block.PortOutput},
			{Name: "storage", PortType: "pvc-spec", Direction: block.PortInput, Required: true},
		},
		Parameters: []block.ParameterSpec{
			{Name: "version", Type: "string", Default: "16", Required: true, Description: "PostgreSQL major version."},
			{Name: "replicas", Type: "int", Default: "1", Required: true, Description: "Number of replicas."},
			{Name: "maxConnections", Type: "int", Default: "100", Description: "max_connections parameter."},
			{Name: "sharedBuffers", Type: "string", Default: "128MB", Description: "shared_buffers parameter."},
		},
		Requires: []string{"storage.*"},
		Provides: []string{"dsn", "metrics-endpoint"},
	}
}

func (b *Block) ValidateParameters(_ context.Context, params map[string]string) error {
	if v, ok := params["replicas"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("replicas must be an integer: %w", err)
		}
		if n < 1 || n > 7 {
			return fmt.Errorf("replicas must be between 1 and 7, got %d", n)
		}
	}
	return nil
}

func (b *Block) Reconcile(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (blocks.ReconcileResult, error) {
	params := req.BlockRef.Parameters
	version := paramOrDefault(params, "version", "16")
	replicaStr := paramOrDefault(params, "replicas", "1")
	maxConn := paramOrDefault(params, "maxConnections", "100")
	sharedBuf := paramOrDefault(params, "sharedBuffers", "128MB")

	replicas, _ := strconv.Atoi(replicaStr)
	replicaCount := int32(replicas)

	storageSize := "1Gi"
	storageClass := "local-path"
	if pvcSpec, ok := req.ResolvedInputs["pvc-spec"]; ok {
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
	labels := map[string]string{
		"app.kubernetes.io/name":       "postgresql",
		"app.kubernetes.io/instance":   fullName,
		"app.kubernetes.io/part-of":    "ottoplus",
		"app.kubernetes.io/managed-by": "ottoplus-operator",
		"ottoplus.io/cluster":          req.ClusterName,
		"ottoplus.io/block":            req.BlockRef.Name,
	}

	configData := fmt.Sprintf(
		"max_connections = %s\nshared_buffers = %s\nlisten_addresses = '*'\n",
		maxConn, sharedBuf,
	)
	if err := b.reconcileConfigMap(ctx, c, req.ClusterNamespace, fullName, labels, configData); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcileStatefulSet(ctx, c, req.ClusterNamespace, fullName, labels, version, replicaCount, storageSize, storageClass); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcileService(ctx, c, req.ClusterNamespace, fullName, labels); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcileHeadlessService(ctx, c, req.ClusterNamespace, fullName, labels); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	dsn := fmt.Sprintf("postgresql://postgres@%s.%s.svc:5432/postgres", fullName, req.ClusterNamespace)

	return blocks.ReconcileResult{
		Phase:   block.PhaseReady,
		Message: "PostgreSQL StatefulSet reconciled",
		Outputs: map[string]string{
			"dsn":     dsn,
			"metrics": fmt.Sprintf("http://%s.%s.svc:9187/metrics", fullName, req.ClusterNamespace),
		},
	}, nil
}

func (b *Block) Delete(ctx context.Context, c client.Client, req blocks.ReconcileRequest) error {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	ns := req.ClusterNamespace

	// Delete in reverse order: service, statefulset, configmap
	_ = c.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: fullName, Namespace: ns}})
	_ = c.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-headless", Namespace: ns}})
	_ = c.Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: fullName, Namespace: ns}})
	_ = c.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-config", Namespace: ns}})
	return nil
}

func (b *Block) HealthCheck(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (block.Phase, error) {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	var sts appsv1.StatefulSet
	if err := c.Get(ctx, types.NamespacedName{Name: fullName, Namespace: req.ClusterNamespace}, &sts); err != nil {
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

func (b *Block) reconcileConfigMap(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, config string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-config",
			Namespace: namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"postgresql.conf": config,
		},
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

func (b *Block) reconcileStatefulSet(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, version string, replicas int32, storageSize, storageClass string) error {
	image := fmt.Sprintf("postgres:%s", version)
	qty := resource.MustParse(storageSize)

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: name + "-headless",
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/instance": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "postgresql",
							Image: image,
							Ports: []corev1.ContainerPort{
								{Name: "postgresql", ContainerPort: 5432, Protocol: corev1.ProtocolTCP},
							},
							Env: []corev1.EnvVar{
								{Name: "POSTGRES_HOST_AUTH_METHOD", Value: "trust"},
								{Name: "PGDATA", Value: "/var/lib/postgresql/data/pgdata"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "data", MountPath: "/var/lib/postgresql/data"},
								{Name: "config", MountPath: "/etc/postgresql", ReadOnly: true},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(5432),
									},
								},
								InitialDelaySeconds: 10,
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
										Name: name + "-config",
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
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, sts)
	}
	if err != nil {
		return err
	}
	existing.Spec.Replicas = sts.Spec.Replicas
	existing.Spec.Template = sts.Spec.Template
	existing.Labels = labels
	return c.Update(ctx, existing)
}

func (b *Block) reconcileService(ctx context.Context, c client.Client, namespace, name string, labels map[string]string) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app.kubernetes.io/instance": name,
			},
			Ports: []corev1.ServicePort{
				{Name: "postgresql", Port: 5432, TargetPort: intstr.FromInt(5432), Protocol: corev1.ProtocolTCP},
			},
			Type: corev1.ServiceTypeClusterIP,
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

func (b *Block) reconcileHeadlessService(ctx context.Context, c client.Client, namespace, name string, labels map[string]string) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-headless",
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app.kubernetes.io/instance": name,
			},
			Ports: []corev1.ServicePort{
				{Name: "postgresql", Port: 5432, TargetPort: intstr.FromInt(5432), Protocol: corev1.ProtocolTCP},
			},
			ClusterIP: corev1.ClusterIPNone,
		},
	}
	existing := &corev1.Service{}
	err := c.Get(ctx, types.NamespacedName{Name: name + "-headless", Namespace: namespace}, existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, svc)
	}
	return err
}

func paramOrDefault(params map[string]string, key, defaultValue string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return defaultValue
}
