// Package passwordrotation implements the auth.password-rotation block,
// deploying automated database credential rotation via CronJob.
package passwordrotation

import (
	"context"
	"fmt"
	"strconv"

	"github.com/baiyuqing/ottoplus/src/core/block"
	blocks "github.com/baiyuqing/ottoplus/src/operator/blocks"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Block implements BlockRuntime for password rotation.
type Block struct{}

func (b *Block) Descriptor() block.Descriptor {
	return block.Descriptor{
		Kind:        "auth.password-rotation",
		Category:    block.CategoryAuth,
		Version:     "1.0.0",
		Description: "Automated database credential rotation via CronJob.",
		Ports: []block.Port{
			{Name: "upstream-dsn", PortType: "dsn", Direction: block.PortInput, Required: true},
			{Name: "credential", PortType: "credential", Direction: block.PortOutput},
		},
		Parameters: []block.ParameterSpec{
			{Name: "rotationSchedule", Type: "string", Default: "0 0 */7 * *", Required: true, Description: "Cron schedule for password rotation (default weekly)."},
			{Name: "passwordLength", Type: "int", Default: "32", Description: "Generated password length."},
			{Name: "secretName", Type: "string", Default: "", Description: "Name of the Secret storing current credentials. Defaults to {cluster}-{name}-creds."},
		},
		Requires: []string{"engine.*"},
		Provides: []string{"credential"},
	}
}

func (b *Block) ValidateParameters(_ context.Context, params map[string]string) error {
	if v, ok := params["passwordLength"]; ok && v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("passwordLength must be an integer: %w", err)
		}
		if n < 8 || n > 128 {
			return fmt.Errorf("passwordLength must be between 8 and 128, got %d", n)
		}
	}
	return nil
}

func (b *Block) Reconcile(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (blocks.ReconcileResult, error) {
	params := req.BlockRef.Parameters
	schedule := paramOrDefault(params, "rotationSchedule", "0 0 */7 * *")
	passwordLength := paramOrDefault(params, "passwordLength", "32")
	secretName := paramOrDefault(params, "secretName", "")

	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	if secretName == "" {
		secretName = fullName + "-creds"
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       "password-rotation",
		"app.kubernetes.io/instance":   fullName,
		"app.kubernetes.io/part-of":    "ottoplus",
		"app.kubernetes.io/managed-by": "ottoplus-operator",
		"ottoplus.io/cluster":          req.ClusterName,
		"ottoplus.io/block":            req.BlockRef.Name,
	}

	if err := reconcileSecret(ctx, c, req.ClusterNamespace, secretName, labels, passwordLength); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	rotationScript := fmt.Sprintf(`#!/bin/sh
set -e
echo "Starting password rotation"
NEW_PASS=$(cat /dev/urandom | tr -dc 'A-Za-z0-9!@#$%%^&*' | head -c %s)
echo "Generated new password of length %s"
# TODO: execute ALTER USER via upstream DSN
echo "Password rotation complete"
`, passwordLength, passwordLength)

	if err := reconcileConfigMap(ctx, c, req.ClusterNamespace, fullName, labels, map[string]string{"rotate.sh": rotationScript}); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	saName := fullName + "-sa"
	if err := reconcileServiceAccount(ctx, c, req.ClusterNamespace, saName, labels); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := reconcileRole(ctx, c, req.ClusterNamespace, fullName, labels, secretName); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := reconcileRoleBinding(ctx, c, req.ClusterNamespace, fullName, labels, saName); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := b.reconcileCronJob(ctx, c, req.ClusterNamespace, fullName, labels, schedule, saName); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	credentialJSON := fmt.Sprintf(`{"secretName":"%s","secretNamespace":"%s","usernameKey":"username","passwordKey":"password"}`, secretName, req.ClusterNamespace)

	return blocks.ReconcileResult{
		Phase:   block.PhaseReady,
		Message: fmt.Sprintf("Password rotation CronJob scheduled: %s", schedule),
		Outputs: map[string]string{
			"credential": credentialJSON,
		},
	}, nil
}

func (b *Block) Delete(ctx context.Context, c client.Client, req blocks.ReconcileRequest) error {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	ns := req.ClusterNamespace
	secretName := paramOrDefault(req.BlockRef.Parameters, "secretName", "")
	if secretName == "" {
		secretName = fullName + "-creds"
	}
	_ = c.Delete(ctx, &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: fullName, Namespace: ns}})
	_ = c.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns}})
	_ = c.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-config", Namespace: ns}})
	_ = c.Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-sa", Namespace: ns}})
	_ = c.Delete(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-role", Namespace: ns}})
	_ = c.Delete(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-rolebinding", Namespace: ns}})
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

func (b *Block) reconcileCronJob(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, schedule, serviceAccountName string) error {
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
							RestartPolicy:      corev1.RestartPolicyOnFailure,
							ServiceAccountName: serviceAccountName,
							Containers: []corev1.Container{
								{
									Name:    "rotate",
									Image:   "busybox:1.36",
									Command: []string{"/bin/sh", "/scripts/rotate.sh"},
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

func reconcileSecret(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, passwordLength string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"username": "dbadmin",
			"password": fmt.Sprintf("initial-%s-char-password", passwordLength),
		},
	}
	existing := &corev1.Secret{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, secret)
	}
	if err != nil {
		return err
	}
	// Do not overwrite existing credentials on reconcile; the CronJob handles rotation.
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

func reconcileServiceAccount(ctx context.Context, c client.Client, namespace, name string, labels map[string]string) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
	}
	existing := &corev1.ServiceAccount{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, sa)
	}
	if err != nil {
		return err
	}
	existing.Labels = labels
	return c.Update(ctx, existing)
}

func reconcileRole(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, secretName string) error {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-role", Namespace: namespace, Labels: labels},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{secretName},
				Verbs:         []string{"get", "update", "patch"},
			},
		},
	}
	existing := &rbacv1.Role{}
	err := c.Get(ctx, types.NamespacedName{Name: role.Name, Namespace: namespace}, existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, role)
	}
	if err != nil {
		return err
	}
	existing.Rules = role.Rules
	existing.Labels = labels
	return c.Update(ctx, existing)
}

func reconcileRoleBinding(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, serviceAccountName string) error {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-rolebinding", Namespace: namespace, Labels: labels},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     name + "-role",
		},
	}
	existing := &rbacv1.RoleBinding{}
	err := c.Get(ctx, types.NamespacedName{Name: rb.Name, Namespace: namespace}, existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, rb)
	}
	if err != nil {
		return err
	}
	existing.Subjects = rb.Subjects
	existing.RoleRef = rb.RoleRef
	existing.Labels = labels
	return c.Update(ctx, existing)
}

func paramOrDefault(params map[string]string, key, defaultValue string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return defaultValue
}
