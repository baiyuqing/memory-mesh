// Package slacknotifier implements the integration.slack-notifier block,
// deploying a Slack webhook notification relay for cluster events and alerts.
package slacknotifier

import (
	"context"
	"fmt"

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

// Block implements BlockRuntime for the Slack notifier integration.
type Block struct{}

func (b *Block) Descriptor() block.Descriptor {
	return block.Descriptor{
		Kind:        "integration.slack-notifier",
		Category:    block.CategoryIntegration,
		Version:     "1.0.0",
		Description: "Slack webhook notifier for cluster events and alerts.",
		Ports: []block.Port{
			{Name: "event-stream", PortType: "event-stream", Direction: block.PortInput},
			{Name: "metrics", PortType: "metrics-endpoint", Direction: block.PortInput},
			{Name: "http-endpoint", PortType: "http-endpoint", Direction: block.PortOutput},
		},
		Parameters: []block.ParameterSpec{
			{Name: "webhookSecretName", Type: "string", Required: true, Description: "K8s Secret containing the Slack webhook URL."},
			{Name: "channel", Type: "string", Default: "#alerts", Description: "Slack channel for notifications."},
			{Name: "severity", Type: "string", Default: "warning", Description: "Minimum severity level: info, warning, critical."},
			{Name: "throttleSeconds", Type: "int", Default: "60", Description: "Minimum seconds between notifications."},
		},
		Provides: []string{"http-endpoint"},
	}
}

func (b *Block) ValidateParameters(_ context.Context, params map[string]string) error {
	if params["webhookSecretName"] == "" {
		return fmt.Errorf("parameter 'webhookSecretName' is required")
	}
	if v, ok := params["severity"]; ok {
		switch v {
		case "info", "warning", "critical":
		default:
			return fmt.Errorf("severity must be one of: info, warning, critical; got %q", v)
		}
	}
	return nil
}

func (b *Block) Reconcile(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (blocks.ReconcileResult, error) {
	params := req.BlockRef.Parameters
	webhookSecretName := params["webhookSecretName"]
	channel := paramOrDefault(params, "channel", "#alerts")
	severity := paramOrDefault(params, "severity", "warning")
	throttle := paramOrDefault(params, "throttleSeconds", "60")

	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	labels := blockLabels(fullName, req.ClusterName, req.BlockRef.Name)

	// Build config data including optional resolved inputs
	configData := map[string]string{
		"CHANNEL":          channel,
		"SEVERITY":         severity,
		"THROTTLE_SECONDS": throttle,
	}
	if v, ok := req.ResolvedInputs["event-stream"]; ok {
		configData["EVENT_STREAM_URL"] = v
	}
	if v, ok := req.ResolvedInputs["metrics"]; ok {
		configData["METRICS_URL"] = v
	}

	if err := b.reconcileConfigMap(ctx, c, req.ClusterNamespace, fullName, labels, configData); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcileDeployment(ctx, c, req.ClusterNamespace, fullName, labels, webhookSecretName); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcileService(ctx, c, req.ClusterNamespace, fullName, labels); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	return blocks.ReconcileResult{
		Phase:   block.PhaseReady,
		Message: "Slack notifier reconciled",
		Outputs: map[string]string{
			"http-endpoint": fmt.Sprintf(`{"host":"%s.%s.svc","port":"8080"}`, fullName, req.ClusterNamespace),
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

func (b *Block) reconcileConfigMap(ctx context.Context, c client.Client, namespace, name string, labels, data map[string]string) error {
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

func (b *Block) reconcileDeployment(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, webhookSecretName string) error {
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
							Name:  "slack-notifier",
							Image: "ottoplus/slack-notifier:latest",
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
							},
							EnvFrom: []corev1.EnvFromSource{
								{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: name + "-config"}}},
							},
							Env: []corev1.EnvVar{
								{
									Name: "SLACK_WEBHOOK_URL",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: webhookSecretName},
											Key:                  "SLACK_WEBHOOK_URL",
										},
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromInt(8080)},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
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
	existing.Spec.Template = deploy.Spec.Template
	existing.Labels = labels
	return c.Update(ctx, existing)
}

func (b *Block) reconcileService(ctx context.Context, c client.Client, namespace, name string, labels map[string]string) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app.kubernetes.io/instance": name},
			Ports:    []corev1.ServicePort{{Name: "http", Port: 8080, TargetPort: intstr.FromInt(8080), Protocol: corev1.ProtocolTCP}},
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

func blockLabels(fullName, clusterName, blockName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "slack-notifier",
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
