// Package healthdashboard implements the observability.health-dashboard block,
// deploying a lightweight health dashboard that aggregates metrics, logs,
// and events into a single pane of glass.
package healthdashboard

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

// Block implements BlockRuntime for the health dashboard.
type Block struct{}

func (b *Block) Descriptor() block.Descriptor {
	return block.Descriptor{
		Kind:        "observability.health-dashboard",
		Category:    block.CategoryObservability,
		Version:     "1.0.0",
		Description: "Lightweight health dashboard aggregating metrics, logs, and events.",
		Ports: []block.Port{
			{Name: "metrics", PortType: "metrics-endpoint", Direction: block.PortInput},
			{Name: "logs", PortType: "log-endpoint", Direction: block.PortInput},
			{Name: "events", PortType: "event-stream", Direction: block.PortInput},
			{Name: "dashboard-url", PortType: "dashboard-url", Direction: block.PortOutput},
			{Name: "http-endpoint", PortType: "http-endpoint", Direction: block.PortOutput},
		},
		Parameters: []block.ParameterSpec{
			{Name: "refreshInterval", Type: "int", Default: "30", Description: "Dashboard refresh interval in seconds."},
			{Name: "title", Type: "string", Default: "OttoPlus Health", Description: "Dashboard title."},
			{Name: "port", Type: "int", Default: "8080", Description: "Dashboard HTTP port."},
		},
		Requires: []string{},
		Provides: []string{"dashboard-url", "http-endpoint"},
	}
}

func (b *Block) ValidateParameters(_ context.Context, params map[string]string) error {
	if raw, ok := params["refreshInterval"]; ok {
		interval, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("refreshInterval must be an integer: %w", err)
		}
		if interval <= 0 {
			return fmt.Errorf("refreshInterval must be positive, got %d", interval)
		}
	}
	if raw, ok := params["port"]; ok {
		port, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("port must be an integer: %w", err)
		}
		if port < 1 || port > 65535 {
			return fmt.Errorf("port must be between 1 and 65535, got %d", port)
		}
	}
	return nil
}

// Reconcile creates or updates the Deployment, Service, and ConfigMap for
// the health dashboard. All input ports are optional; the dashboard degrades
// gracefully when sources are unavailable.
func (b *Block) Reconcile(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (blocks.ReconcileResult, error) {
	params := req.BlockRef.Parameters
	refreshInterval := paramOrDefault(params, "refreshInterval", "30")
	title := paramOrDefault(params, "title", "OttoPlus Health")
	portStr := paramOrDefault(params, "port", "8080")
	port := int32(8080)
	if parsed, err := strconv.Atoi(portStr); err == nil {
		port = int32(parsed)
	}

	metricsEndpoint := req.ResolvedInputs["metrics"]
	logsEndpoint := req.ResolvedInputs["logs"]
	eventsEndpoint := req.ResolvedInputs["events"]

	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	labels := blockLabels(fullName, req.ClusterName, req.BlockRef.Name)

	dashboardConfig := fmt.Sprintf(
		`{"title":%q,"refreshInterval":%s,"metricsEndpoint":%q,"logsEndpoint":%q,"eventsEndpoint":%q,"port":%d}`,
		title, refreshInterval, metricsEndpoint, logsEndpoint, eventsEndpoint, port,
	)

	if err := reconcileConfigMap(ctx, c, req.ClusterNamespace, fullName, labels, map[string]string{"dashboard.json": dashboardConfig}); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcileDeployment(ctx, c, req.ClusterNamespace, fullName, labels, port); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := reconcileClusterIPService(ctx, c, req.ClusterNamespace, fullName, labels, "http", port); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	dashboardURL := fmt.Sprintf("http://%s.%s.svc:%d/dashboard", fullName, req.ClusterNamespace, port)
	httpEndpoint := fmt.Sprintf(`{"host":"%s.%s.svc","port":%d}`, fullName, req.ClusterNamespace, port)

	return blocks.ReconcileResult{
		Phase:   block.PhaseReady,
		Message: "Health dashboard reconciled",
		Outputs: map[string]string{
			"dashboard-url": dashboardURL,
			"http-endpoint": httpEndpoint,
		},
	}, nil
}

func (b *Block) Delete(ctx context.Context, c client.Client, req blocks.ReconcileRequest) error {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	ns := req.ClusterNamespace
	_ = c.Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: fullName, Namespace: ns}})
	_ = c.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: fullName, Namespace: ns}})
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

func (b *Block) reconcileDeployment(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, port int32) error {
	replicas := int32(1)
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
							Name:  "health-dashboard",
							Image: "ottoplus/health-dashboard:latest",
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: port, Protocol: corev1.ProtocolTCP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "config", MountPath: "/etc/dashboard", ReadOnly: true},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt32(port),
									},
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
