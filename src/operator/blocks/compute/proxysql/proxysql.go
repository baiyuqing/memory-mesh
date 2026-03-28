// Package proxysql implements the compute.proxysql block, deploying a
// ProxySQL connection pooler in front of a MySQL datastore block.
package proxysql

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

// Block implements BlockRuntime for ProxySQL.
type Block struct{}

func (b *Block) Descriptor() block.Descriptor {
	return block.Descriptor{
		Kind:        "compute.proxysql",
		Category:    block.CategoryCompute,
		Version:     "1.0.0",
		Description: "ProxySQL connection pooler and query router for MySQL.",
		Ports: []block.Port{
			{Name: "upstream-dsn", PortType: "dsn", Direction: block.PortInput, Required: true},
			{Name: "dsn", PortType: "dsn", Direction: block.PortOutput},
			{Name: "metrics", PortType: "metrics-endpoint", Direction: block.PortOutput},
		},
		Parameters: []block.ParameterSpec{
			{Name: "maxConnections", Type: "int", Default: "1024", Description: "Maximum frontend connections."},
			{Name: "defaultHostgroup", Type: "int", Default: "0", Description: "Default hostgroup for queries."},
			{Name: "monitorUsername", Type: "string", Default: "monitor", Description: "Username for backend health checks."},
			{Name: "multiplexing", Type: "string", Default: "true", Description: "Enable connection multiplexing."},
		},
		Requires: []string{"datastore.mysql"},
		Provides: []string{"dsn", "metrics-endpoint"},
	}
}

func (b *Block) ValidateParameters(_ context.Context, _ map[string]string) error {
	return nil
}

func (b *Block) Reconcile(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (blocks.ReconcileResult, error) {
	params := req.BlockRef.Parameters
	maxConn := paramOrDefault(params, "maxConnections", "1024")
	hostgroup := paramOrDefault(params, "defaultHostgroup", "0")
	monUser := paramOrDefault(params, "monitorUsername", "monitor")
	multiplex := paramOrDefault(params, "multiplexing", "true")
	upstreamDSN := req.ResolvedInputs["upstream-dsn"]

	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	labels := blockLabels(fullName, req.ClusterName, req.BlockRef.Name)

	proxysqlCnf := fmt.Sprintf(`datadir="/var/lib/proxysql"

admin_variables=
{
    admin_credentials="admin:admin"
    mysql_ifaces="0.0.0.0:6032"
}

mysql_variables=
{
    threads=4
    max_connections=%s
    default_query_delay=0
    default_query_timeout=36000000
    interfaces="0.0.0.0:6033"
    monitor_username="%s"
    monitor_password="monitor"
    multiplexing=%s
}

mysql_servers=
(
    { address="%s", port=3306, hostgroup=%s, max_connections=100 }
)

mysql_users=
(
    { username="root", password="", default_hostgroup=%s }
)
`, maxConn, monUser, multiplex, extractHost(upstreamDSN), hostgroup, hostgroup)

	if err := reconcileConfigMap(ctx, c, req.ClusterNamespace, fullName, labels, map[string]string{"proxysql.cnf": proxysqlCnf}); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	replicas := int32(2)
	if err := b.reconcileDeployment(ctx, c, req.ClusterNamespace, fullName, labels, replicas); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if err := reconcileClusterIPService(ctx, c, req.ClusterNamespace, fullName, labels, "proxysql", 6033); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	dsn := fmt.Sprintf("mysql://root@%s.%s.svc:6033/mysql", fullName, req.ClusterNamespace)

	return blocks.ReconcileResult{
		Phase:   block.PhaseReady,
		Message: "ProxySQL Deployment reconciled",
		Outputs: map[string]string{
			"dsn":     dsn,
			"metrics": fmt.Sprintf("http://%s.%s.svc:6070/metrics", fullName, req.ClusterNamespace),
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
					Containers: []corev1.Container{{
						Name:  "proxysql",
						Image: "proxysql/proxysql:2.6.3",
						Ports: []corev1.ContainerPort{
							{Name: "mysql", ContainerPort: 6033, Protocol: corev1.ProtocolTCP},
							{Name: "admin", ContainerPort: 6032, Protocol: corev1.ProtocolTCP},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "config", MountPath: "/etc/proxysql.cnf", SubPath: "proxysql.cnf", ReadOnly: true},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(6033)},
							},
							InitialDelaySeconds: 5,
							PeriodSeconds:       5,
						},
					}},
					Volumes: []corev1.Volume{{
						Name: "config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: name + "-config"},
							},
						},
					}},
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
		"app.kubernetes.io/name":       "proxysql",
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

func extractHost(dsn string) string {
	// Parse mysql://user@host:port/db -> host
	s := dsn
	for _, prefix := range []string{"mysql://", "postgresql://"} {
		if len(s) > len(prefix) && s[:len(prefix)] == prefix {
			s = s[len(prefix):]
			break
		}
	}
	if idx := findByte(s, '@'); idx >= 0 {
		s = s[idx+1:]
	}
	if idx := findByte(s, ':'); idx >= 0 {
		return s[:idx]
	}
	if idx := findByte(s, '/'); idx >= 0 {
		return s[:idx]
	}
	return s
}

func findByte(s string, b byte) int {
	for i := range s {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func paramOrDefault(params map[string]string, key, defaultValue string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return defaultValue
}
