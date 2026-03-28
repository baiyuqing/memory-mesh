// Package redis implements the engine.redis block, managing Redis
// instances as Kubernetes StatefulSets.
package redis

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

// Block implements BlockRuntime for Redis.
type Block struct{}

func (b *Block) Descriptor() block.Descriptor {
	return block.Descriptor{
		Kind:        "datastore.redis",
		Category:    block.CategoryDatastore,
		Version:     "1.0.0",
		Description: "Redis in-memory data store managed as a Kubernetes StatefulSet.",
		Ports: []block.Port{
			{Name: "dsn", PortType: "dsn", Direction: block.PortOutput},
			{Name: "metrics", PortType: "metrics-endpoint", Direction: block.PortOutput},
			{Name: "storage", PortType: "pvc-spec", Direction: block.PortInput, Required: false},
		},
		Parameters: []block.ParameterSpec{
			{Name: "version", Type: "string", Default: "7.2", Required: true, Description: "Redis major version."},
			{Name: "replicas", Type: "int", Default: "1", Required: true, Description: "Number of replicas."},
			{Name: "maxMemory", Type: "string", Default: "256mb", Description: "maxmemory setting."},
			{Name: "maxMemoryPolicy", Type: "string", Default: "allkeys-lru", Description: "Eviction policy."},
			{Name: "persistence", Type: "string", Default: "none", Description: "Persistence mode: none, rdb, aof, rdb+aof."},
		},
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
	validPolicies := map[string]bool{
		"noeviction": true, "allkeys-lru": true, "volatile-lru": true,
		"allkeys-random": true, "volatile-random": true, "volatile-ttl": true,
	}
	if policy, ok := params["maxMemoryPolicy"]; ok && !validPolicies[policy] {
		return fmt.Errorf("invalid maxMemoryPolicy: %s", policy)
	}
	validPersistence := map[string]bool{"none": true, "rdb": true, "aof": true, "rdb+aof": true}
	if p, ok := params["persistence"]; ok && !validPersistence[p] {
		return fmt.Errorf("invalid persistence mode: %s", p)
	}
	return nil
}

func (b *Block) Reconcile(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (blocks.ReconcileResult, error) {
	params := req.BlockRef.Parameters
	version := paramOrDefault(params, "version", "7.2")
	replicaStr := paramOrDefault(params, "replicas", "1")
	maxMem := paramOrDefault(params, "maxMemory", "256mb")
	maxMemPolicy := paramOrDefault(params, "maxMemoryPolicy", "allkeys-lru")
	persistence := paramOrDefault(params, "persistence", "none")

	replicas, _ := strconv.Atoi(replicaStr)
	replicaCount := int32(replicas)

	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	labels := map[string]string{
		"app.kubernetes.io/name":       "redis",
		"app.kubernetes.io/instance":   fullName,
		"app.kubernetes.io/part-of":    "ottoplus",
		"app.kubernetes.io/managed-by": "ottoplus-operator",
		"ottoplus.io/cluster":          req.ClusterName,
		"ottoplus.io/block":            req.BlockRef.Name,
	}

	redisConf := buildRedisConf(maxMem, maxMemPolicy, persistence)
	if err := b.reconcileConfigMap(ctx, c, req.ClusterNamespace, fullName, labels, redisConf); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	hasPVC := hasStorageInput(req.ResolvedInputs)
	if err := b.reconcileStatefulSet(ctx, c, req.ClusterNamespace, fullName, labels, version, replicaCount, persistence, hasPVC, req.ResolvedInputs); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcileService(ctx, c, req.ClusterNamespace, fullName, labels); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcileHeadlessService(ctx, c, req.ClusterNamespace, fullName, labels); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	dsn := fmt.Sprintf("redis://%s.%s.svc:6379", fullName, req.ClusterNamespace)

	return blocks.ReconcileResult{
		Phase:   block.PhaseReady,
		Message: "Redis StatefulSet reconciled",
		Outputs: map[string]string{
			"dsn":     dsn,
			"metrics": fmt.Sprintf("http://%s.%s.svc:9121/metrics", fullName, req.ClusterNamespace),
		},
	}, nil
}

func (b *Block) Delete(ctx context.Context, c client.Client, req blocks.ReconcileRequest) error {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	ns := req.ClusterNamespace
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

func buildRedisConf(maxMem, maxMemPolicy, persistence string) string {
	conf := fmt.Sprintf("bind 0.0.0.0\nprotected-mode no\nport 6379\nmaxmemory %s\nmaxmemory-policy %s\n", maxMem, maxMemPolicy)
	switch persistence {
	case "rdb":
		conf += "save 900 1\nsave 300 10\nsave 60 10000\n"
	case "aof":
		conf += "appendonly yes\nappendfsync everysec\n"
	case "rdb+aof":
		conf += "save 900 1\nsave 300 10\nsave 60 10000\nappendonly yes\nappendfsync everysec\n"
	default:
		conf += "save \"\"\nappendonly no\n"
	}
	return conf
}

func hasStorageInput(inputs map[string]string) bool {
	_, ok := inputs["pvc-spec"]
	return ok
}

func (b *Block) reconcileConfigMap(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, config string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-config", Namespace: namespace, Labels: labels},
		Data:       map[string]string{"redis.conf": config},
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

func (b *Block) reconcileStatefulSet(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, version string, replicas int32, persistence string, hasPVC bool, inputs map[string]string) error {
	image := fmt.Sprintf("redis:%s", version)

	container := corev1.Container{
		Name:  "redis",
		Image: image,
		Command: []string{
			"redis-server",
			"/etc/redis/redis.conf",
		},
		Ports: []corev1.ContainerPort{
			{Name: "redis", ContainerPort: 6379, Protocol: corev1.ProtocolTCP},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "config", MountPath: "/etc/redis", ReadOnly: true},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(6379),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       5,
		},
	}

	volumes := []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name + "-config"},
				},
			},
		},
	}

	if hasPVC && persistence != "none" {
		container.VolumeMounts = append(container.VolumeMounts,
			corev1.VolumeMount{Name: "data", MountPath: "/data"},
		)
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: name + "-headless",
			Replicas:    &replicas,
			Selector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/instance": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
					Volumes:    volumes,
				},
			},
		},
	}

	if hasPVC && persistence != "none" {
		storageSize := "1Gi"
		storageClass := "local-path"
		if pvcSpec, ok := inputs["pvc-spec"]; ok {
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
		qty := resource.MustParse(storageSize)
		sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					StorageClassName: &storageClass,
					Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: qty}},
				},
			},
		}
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
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app.kubernetes.io/instance": name},
			Ports:    []corev1.ServicePort{{Name: "redis", Port: 6379, TargetPort: intstr.FromInt(6379), Protocol: corev1.ProtocolTCP}},
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

func (b *Block) reconcileHeadlessService(ctx context.Context, c client.Client, namespace, name string, labels map[string]string) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-headless", Namespace: namespace, Labels: labels},
		Spec: corev1.ServiceSpec{
			Selector:  map[string]string{"app.kubernetes.io/instance": name},
			Ports:     []corev1.ServicePort{{Name: "redis", Port: 6379, TargetPort: intstr.FromInt(6379), Protocol: corev1.ProtocolTCP}},
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
