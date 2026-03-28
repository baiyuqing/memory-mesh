// Package mtls implements the security.mtls block, provisioning self-signed
// mTLS certificates (CA, server, client) stored as Kubernetes Secrets.
package mtls

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/baiyuqing/ottoplus/src/core/block"
	blocks "github.com/baiyuqing/ottoplus/src/operator/blocks"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Block implements BlockRuntime for mTLS certificate provisioning.
type Block struct{}

func (b *Block) Descriptor() block.Descriptor {
	return block.Descriptor{
		Kind:        "security.mtls",
		Category:    block.CategorySecurity,
		Version:     "1.0.0",
		Description: "Self-signed mTLS certificate provisioner. Generates CA, server, and client certificates stored as Kubernetes Secrets.",
		Ports: []block.Port{
			{Name: "tls-cert", PortType: "tls-cert", Direction: block.PortOutput},
		},
		Parameters: []block.ParameterSpec{
			{Name: "commonName", Type: "string", Default: "ottoplus.local", Required: true, Description: "Common name for the CA certificate."},
			{Name: "validityDays", Type: "int", Default: "365", Description: "Certificate validity in days."},
			{Name: "keySize", Type: "int", Default: "2048", Description: "RSA key size in bits."},
			{Name: "organization", Type: "string", Default: "ottoplus", Description: "Organization name for certificates."},
		},
		Requires: []string{},
		Provides: []string{"tls-cert"},
	}
}

func (b *Block) ValidateParameters(_ context.Context, params map[string]string) error {
	if v, ok := params["keySize"]; ok && v != "" {
		size, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("keySize must be an integer: %w", err)
		}
		if size != 2048 && size != 4096 {
			return fmt.Errorf("keySize must be 2048 or 4096, got %d", size)
		}
	}
	return nil
}

func (b *Block) Reconcile(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (blocks.ReconcileResult, error) {
	params := req.BlockRef.Parameters
	commonName := paramOrDefault(params, "commonName", "ottoplus.local")
	validityStr := paramOrDefault(params, "validityDays", "365")
	keySizeStr := paramOrDefault(params, "keySize", "2048")
	organization := paramOrDefault(params, "organization", "ottoplus")

	validityDays, _ := strconv.Atoi(validityStr)
	keySize, _ := strconv.Atoi(keySizeStr)

	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	labels := blockLabels(fullName, req.ClusterName, req.BlockRef.Name)

	// Idempotent: only generate certificates if the CA secret does not exist.
	caSecretName := fullName + "-ca"
	existingCA := &corev1.Secret{}
	err := c.Get(ctx, types.NamespacedName{Name: caSecretName, Namespace: req.ClusterNamespace}, existingCA)
	if err != nil && !errors.IsNotFound(err) {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	if errors.IsNotFound(err) {
		caCert, caKey, err := generateCACertificate(commonName, organization, keySize, validityDays)
		if err != nil {
			return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, fmt.Errorf("generating CA certificate: %w", err)
		}

		serverCert, serverKey, err := generateSignedCertificate(commonName, organization, keySize, validityDays, caCert, caKey, false)
		if err != nil {
			return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, fmt.Errorf("generating server certificate: %w", err)
		}

		clientCert, clientKey, err := generateSignedCertificate("client."+commonName, organization, keySize, validityDays, caCert, caKey, true)
		if err != nil {
			return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, fmt.Errorf("generating client certificate: %w", err)
		}

		caCertPEM := encodeCertPEM(caCert)
		caKeyPEM := encodeKeyPEM(caKey)
		serverCertPEM := encodeCertPEM(serverCert)
		serverKeyPEM := encodeKeyPEM(serverKey)
		clientCertPEM := encodeCertPEM(clientCert)
		clientKeyPEM := encodeKeyPEM(clientKey)

		if err := createSecret(ctx, c, req.ClusterNamespace, fullName+"-ca", labels, map[string][]byte{
			"ca.crt": caCertPEM,
			"ca.key": caKeyPEM,
		}); err != nil {
			return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
		}

		if err := createSecret(ctx, c, req.ClusterNamespace, fullName+"-server", labels, map[string][]byte{
			"tls.crt": serverCertPEM,
			"tls.key": serverKeyPEM,
			"ca.crt":  caCertPEM,
		}); err != nil {
			return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
		}

		if err := createSecret(ctx, c, req.ClusterNamespace, fullName+"-client", labels, map[string][]byte{
			"tls.crt": clientCertPEM,
			"tls.key": clientKeyPEM,
			"ca.crt":  caCertPEM,
		}); err != nil {
			return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
		}
	}

	if err := reconcileConfigMap(ctx, c, req.ClusterNamespace, fullName, labels, map[string]string{
		"commonName":   commonName,
		"organization": organization,
		"keySize":      keySizeStr,
		"validityDays": validityStr,
	}); err != nil {
		return blocks.ReconcileResult{Phase: block.PhaseFailed, Message: err.Error()}, err
	}

	tlsCertOutput, _ := json.Marshal(map[string]string{
		"caSecret":     fullName + "-ca",
		"serverSecret": fullName + "-server",
		"clientSecret": fullName + "-client",
	})

	return blocks.ReconcileResult{
		Phase:   block.PhaseReady,
		Message: "mTLS certificates reconciled",
		Outputs: map[string]string{
			"tls-cert": string(tlsCertOutput),
		},
	}, nil
}

func (b *Block) Delete(ctx context.Context, c client.Client, req blocks.ReconcileRequest) error {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	ns := req.ClusterNamespace
	_ = c.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-ca", Namespace: ns}})
	_ = c.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-server", Namespace: ns}})
	_ = c.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-client", Namespace: ns}})
	_ = c.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: fullName + "-config", Namespace: ns}})
	return nil
}

func (b *Block) HealthCheck(ctx context.Context, c client.Client, req blocks.ReconcileRequest) (block.Phase, error) {
	fullName := fmt.Sprintf("%s-%s", req.ClusterName, req.BlockRef.Name)
	var secret corev1.Secret
	if err := c.Get(ctx, types.NamespacedName{Name: fullName + "-ca", Namespace: req.ClusterNamespace}, &secret); err != nil {
		if errors.IsNotFound(err) {
			return block.PhasePending, nil
		}
		return block.PhaseFailed, err
	}
	return block.PhaseReady, nil
}

func generateCACertificate(commonName, organization string, keySize, validityDays int) (*x509.Certificate, *rsa.PrivateKey, error) {
	caKey, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return nil, nil, fmt.Errorf("generating CA key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("generating serial number: %w", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{organization},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Duration(validityDays) * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("creating CA certificate: %w", err)
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing CA certificate: %w", err)
	}

	return caCert, caKey, nil
}

func generateSignedCertificate(commonName, organization string, keySize, validityDays int, caCert *x509.Certificate, caKey *rsa.PrivateKey, isClient bool) (*x509.Certificate, *rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return nil, nil, fmt.Errorf("generating key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("generating serial number: %w", err)
	}

	extKeyUsage := x509.ExtKeyUsageServerAuth
	if isClient {
		extKeyUsage = x509.ExtKeyUsageClientAuth
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{organization},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Duration(validityDays) * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{extKeyUsage},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("creating certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing certificate: %w", err)
	}

	return cert, key, nil
}

func encodeCertPEM(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
}

func encodeKeyPEM(key *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

func createSecret(ctx context.Context, c client.Client, namespace, name string, labels map[string]string, data map[string][]byte) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
		Type:       corev1.SecretTypeOpaque,
		Data:       data,
	}
	return c.Create(ctx, secret)
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

func blockLabels(fullName, clusterName, blockName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "mtls",
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
