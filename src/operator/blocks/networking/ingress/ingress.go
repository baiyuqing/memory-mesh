// Package ingress implements the networking.ingress block, creating a
// Kubernetes Ingress resource for external HTTP access with optional TLS.
package ingress

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/baiyuqing/ottoplus/src/core/block"
	blocks "github.com/baiyuqing/ottoplus/src/operator/blocks"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Block implements BlockRuntime for Kubernetes Ingress.
type Block struct{}

func (b *Block) Descriptor() block.Descriptor {
	return block.Descriptor{
		Kind:        "networking.ingress",
		Category:    block.CategoryNetworking,
		Version:     "1.0.0",
		Description: "Kubernetes Ingress for external HTTP access with optional TLS.",
		Ports: []block.Port{
			{Name: "upstream-http", PortType: "http-endpoint", Direction: block.PortInput, Required: true},
			{Name: "tls-cert", PortType: "tls-cert", Direction: block.PortInput},
			{Name: "ingress-url", PortType: "ingress-url", Direction: block.PortOutput},
		},
		Parameters: []block.ParameterSpec{
			{Name: "host", Type: "string", Required: true, Description: "Hostname for the Ingress rule."},
			{Name: "path", Type: "string", Default: "/", Description: "URL path prefix."},
			{Name: "ingressClassName", Type: "string", Default: "nginx", Description: "Ingress class name."},
			{Name: "tlsEnabled", Type: "string", Default: "false", Description: "Enable TLS termination."},
		},
		Requires: []string{"engine.*"},
		Provides: []string{"ingress-url"},
	}
}

func (b *Block) ValidateParameters(_ context.Context, params map[string]string) error {
	if params["host"] == "" {
		return fmt.Errorf("parameter 'host' is required")
	}
	return nil
}

// httpEndpoint represents the JSON structure of an http-endpoint port value.
type httpEndpoint struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

func (b *Block) Reconcile(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (blocks.ReconcileResult, error) {
	params := req.BlockRef.Parameters
	host := params["host"]
	path := paramOrDefault(params, "path", "/")
	ingressClassName := paramOrDefault(params, "ingressClassName", "nginx")
	tlsEnabled := paramOrDefault(params, "tlsEnabled", "false")

	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	labels := blockLabels(fullName, req.ClusterName, req.BlockRef.Name)

	// Parse upstream http-endpoint
	var upstream httpEndpoint
	if raw, ok := req.ResolvedInputs["upstream-http"]; ok {
		if err := json.Unmarshal([]byte(raw), &upstream); err != nil {
			return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: fmt.Sprintf("invalid upstream-http: %v", err)}, nil
		}
	}

	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: fullName, Namespace: req.ClusterNamespace, Labels: labels},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressClassName,
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     path,
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: upstream.Host,
											Port: networkingv1.ServiceBackendPort{
												Number: 8080,
											},
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

	if tlsEnabled == "true" {
		tlsSecretName := fullName + "-tls"
		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{Hosts: []string{host}, SecretName: tlsSecretName},
		}
	}

	existing := &networkingv1.Ingress{}
	err := c.Get(ctx, types.NamespacedName{Name: fullName, Namespace: req.ClusterNamespace}, existing)
	if errors.IsNotFound(err) {
		if err := c.Create(ctx, ingress); err != nil {
			return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
		}
	} else if err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	} else {
		existing.Spec = ingress.Spec
		existing.Labels = labels
		if err := c.Update(ctx, existing); err != nil {
			return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
		}
	}

	scheme := "http"
	if tlsEnabled == "true" {
		scheme = "https"
	}
	ingressURL := fmt.Sprintf("%s://%s%s", scheme, host, path)

	return blocks.ReconcileResult{
		Phase:   block.PhaseReady,
		Message: "Ingress reconciled",
		Outputs: map[string]string{
			"ingress-url": ingressURL,
		},
	}, nil
}

func (b *Block) Delete(ctx context.Context, c client.Client, req blocks.ReconcileRequest) error {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	return client.IgnoreNotFound(c.Delete(ctx, &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: fullName, Namespace: req.ClusterNamespace},
	}))
}

func (b *Block) HealthCheck(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (block.Phase, error) {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	var ingress networkingv1.Ingress
	if err := c.Get(ctx, types.NamespacedName{Name: fullName, Namespace: req.ClusterNamespace}, &ingress); err != nil {
		if errors.IsNotFound(err) {
			return block.PhasePending, nil
		}
		return block.PhaseFailed, err
	}
	return block.PhaseReady, nil
}

func blockLabels(fullName, clusterName, blockName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "ingress",
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
