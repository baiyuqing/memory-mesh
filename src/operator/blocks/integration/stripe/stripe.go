// Package stripe implements the integration.stripe block, deploying a
// Stripe webhook receiver that persists billing events to a database.
package stripe

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/baiyuqing/ottoplus/src/core/block"
	blocks "github.com/baiyuqing/ottoplus/src/operator/blocks"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Block implements BlockRuntime for Stripe integration.
type Block struct{}

func (b *Block) Descriptor() block.Descriptor {
	return block.Descriptor{
		Kind:        "integration.stripe",
		Category:    block.CategoryIntegration,
		Version:     "1.0.0",
		Description: "Stripe payment API integration. Deploys a webhook receiver and provides billing event streams.",
		Ports: []block.Port{
			{Name: "webhook-endpoint", PortType: "http-endpoint", Direction: block.PortOutput},
			{Name: "billing-events", PortType: "event-stream", Direction: block.PortOutput},
			{Name: "dsn", PortType: "dsn", Direction: block.PortInput, Required: true},
		},
		Parameters: []block.ParameterSpec{
			{Name: "apiKeySecret", Type: "string", Required: true, Description: "K8s Secret name with STRIPE_API_KEY."},
			{Name: "webhookSecret", Type: "string", Required: true, Description: "K8s Secret name with STRIPE_WEBHOOK_SECRET."},
			{Name: "webhookPath", Type: "string", Default: "/webhooks/stripe", Description: "Webhook receiver path."},
			{Name: "replicas", Type: "int", Default: "2", Description: "Webhook receiver replicas."},
			{Name: "events", Type: "string", Default: "invoice.paid,invoice.payment_failed,customer.subscription.updated,customer.subscription.deleted", Description: "Stripe event types to handle."},
		},
		Provides: []string{"http-endpoint", "event-stream"},
	}
}

func (b *Block) ValidateParameters(_ context.Context, params map[string]string) error {
	if params["apiKeySecret"] == "" {
		return fmt.Errorf("parameter 'apiKeySecret' is required")
	}
	if params["webhookSecret"] == "" {
		return fmt.Errorf("parameter 'webhookSecret' is required")
	}
	if v, ok := params["replicas"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("replicas must be an integer: %w", err)
		}
		if n < 1 || n > 10 {
			return fmt.Errorf("replicas must be between 1 and 10, got %d", n)
		}
	}
	return nil
}

func (b *Block) Reconcile(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (blocks.ReconcileResult, error) {
	params := req.BlockRef.Parameters
	apiKeySecret := params["apiKeySecret"]
	webhookSecret := params["webhookSecret"]
	webhookPath := paramOrDefault(params, "webhookPath", "/webhooks/stripe")
	replicaStr := paramOrDefault(params, "replicas", "2")
	events := paramOrDefault(params, "events", "invoice.paid,invoice.payment_failed,customer.subscription.updated,customer.subscription.deleted")

	replicas, _ := strconv.Atoi(replicaStr)
	replicaCount := int32(replicas)

	dsn := req.ResolvedInputs["dsn"]
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	labels := map[string]string{
		"app.kubernetes.io/name":       "stripe-webhook",
		"app.kubernetes.io/instance":   fullName,
		"app.kubernetes.io/part-of":    "ottoplus",
		"app.kubernetes.io/managed-by": "ottoplus-operator",
		"ottoplus.io/cluster":          req.ClusterName,
		"ottoplus.io/block":            req.BlockRef.Name,
	}

	// Validate secrets exist
	for _, secretName := range []string{apiKeySecret, webhookSecret} {
		var secret corev1.Secret
		if err := c.Get(ctx, types.NamespacedName{Name: secretName, Namespace: req.ClusterNamespace}, &secret); err != nil {
			if errors.IsNotFound(err) {
				return blocks.ReconcileResult{
					Phase:   block.PhaseFailed,
					Message: fmt.Sprintf("secret %q not found", secretName),
				}, nil
			}
			return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
		}
	}

	if err := b.reconcileConfigMap(ctx, c, req.ClusterNamespace, fullName, labels, webhookPath, events); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcileMigrationJob(ctx, c, req.ClusterNamespace, fullName, labels, dsn); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcileDeployment(ctx, c, req.ClusterNamespace, fullName, labels, replicaCount, apiKeySecret, webhookSecret, dsn); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcileService(ctx, c, req.ClusterNamespace, fullName, labels); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	webhookURL := fmt.Sprintf("http://%s.%s.svc:8080%s", fullName, req.ClusterNamespace, webhookPath)

	return blocks.ReconcileResult{
		Phase:   block.PhaseReady,
		Message: "Stripe webhook receiver reconciled",
		Outputs: map[string]string{
			"webhook-endpoint": webhookURL,
			"billing-events":   fmt.Sprintf("db://%s/stripe_events", fullName),
		},
	}, nil
}

func (b *Block) Delete(ctx context.Context, c client.Client, req blocks.ReconcileRequest) error {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	ns := req.ClusterNamespace
	_ = c.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: fullName, Namespace: ns}})
	_ = c.Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: fullName, Namespace: ns}})
	_ = c.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-migrate", Namespace: ns}})
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

func (b *Block) reconcileConfigMap(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, webhookPath, events string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-config", Namespace: namespace, Labels: labels},
		Data: map[string]string{
			"WEBHOOK_PATH":    webhookPath,
			"STRIPE_EVENTS":   events,
			"LISTEN_ADDR":     ":8080",
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

func (b *Block) reconcileMigrationJob(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, dsn string) error {
	jobName := name + "-migrate"

	// Check if job already completed
	var existing batchv1.Job
	err := c.Get(ctx, types.NamespacedName{Name: jobName, Namespace: namespace}, &existing)
	if err == nil {
		if existing.Status.Succeeded > 0 {
			return nil
		}
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	migrationSQL := strings.Join([]string{
		"CREATE TABLE IF NOT EXISTS stripe_events (",
		"  id TEXT PRIMARY KEY,",
		"  type TEXT NOT NULL,",
		"  data JSONB NOT NULL,",
		"  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),",
		"  processed_at TIMESTAMPTZ",
		");",
		"CREATE INDEX IF NOT EXISTS idx_stripe_events_type ON stripe_events(type);",
		"CREATE INDEX IF NOT EXISTS idx_stripe_events_created_at ON stripe_events(created_at);",
	}, "\n")

	backoffLimit := int32(3)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: namespace, Labels: labels},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:    "migrate",
							Image:   "postgres:16",
							Command: []string{"psql", dsn, "-c", migrationSQL},
						},
					},
				},
			},
		},
	}
	return c.Create(ctx, job)
}

func (b *Block) reconcileDeployment(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, replicas int32, apiKeySecret, webhookSecret, dsn string) error {
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
							Name:  "webhook-receiver",
							Image: "ottoplus/stripe-webhook:latest",
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
							},
							EnvFrom: []corev1.EnvFromSource{
								{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: name + "-config"}}},
							},
							Env: []corev1.EnvVar{
								{Name: "DATABASE_URL", Value: dsn},
								{
									Name: "STRIPE_API_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: apiKeySecret},
											Key:                  "STRIPE_API_KEY",
										},
									},
								},
								{
									Name: "STRIPE_WEBHOOK_SECRET",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: webhookSecret},
											Key:                  "STRIPE_WEBHOOK_SECRET",
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
	existing.Spec.Replicas = deploy.Spec.Replicas
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

func paramOrDefault(params map[string]string, key, defaultValue string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return defaultValue
}
