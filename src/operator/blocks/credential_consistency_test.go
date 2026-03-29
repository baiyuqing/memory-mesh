package blocks_test

import (
	"context"
	"strings"
	"testing"

	"github.com/baiyuqing/ottoplus/src/core/block"
	blocks "github.com/baiyuqing/ottoplus/src/operator/blocks"

	postgresql "github.com/baiyuqing/ottoplus/src/operator/blocks/datastore/postgresql"
	passwordrotation "github.com/baiyuqing/ottoplus/src/operator/blocks/security/password-rotation"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestCredentialProducers_ConsistentFormat proves that all credential
// producers in the codebase output JSON that DecodeCredentialRef can
// consume. If a new producer is added without using CredentialRef, this
// test should be extended.
func TestCredentialProducers_ConsistentFormat(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	ctx := context.Background()

	producers := []struct {
		name    string
		block   blocks.BlockRuntime
		request blocks.ReconcileRequest
	}{
		{
			name:  "datastore.postgresql",
			block: &postgresql.Block{},
			request: blocks.ReconcileRequest{
				ClusterName:      "consistency-test",
				ClusterNamespace: "default",
				BlockRef: block.BlockRef{
					Kind: "datastore.postgresql",
					Name: "pg",
					Parameters: map[string]string{
						"version":  "16",
						"replicas": "1",
					},
				},
				ResolvedInputs: map[string]string{},
			},
		},
		{
			name:  "security.password-rotation",
			block: &passwordrotation.Block{},
			request: blocks.ReconcileRequest{
				ClusterName:      "consistency-test",
				ClusterNamespace: "default",
				BlockRef: block.BlockRef{
					Kind: "security.password-rotation",
					Name: "rotator",
					Parameters: map[string]string{
						"rotationSchedule": "0 0 */7 * *",
						"passwordLength":   "32",
					},
				},
				ResolvedInputs: map[string]string{
					"dsn": "postgresql://postgres@consistency-test-pg.default.svc:5432/postgres",
				},
			},
		},
	}

	decoded := make([]block.CredentialRef, 0, len(producers))

	for _, p := range producers {
		t.Run(p.name, func(t *testing.T) {
			result, err := p.block.Reconcile(ctx, c, p.request)
			if err != nil {
				t.Fatalf("Reconcile failed: %v", err)
			}

			credJSON, ok := result.Outputs["credential"]
			if !ok || credJSON == "" {
				t.Fatal("expected credential output to be present")
			}

			cred, err := block.DecodeCredentialRef(credJSON)
			if err != nil {
				t.Fatalf("DecodeCredentialRef failed — producer %q emits incompatible credential format: %v", p.name, err)
			}

			// All decoded credentials must have the required fields populated.
			if cred.SecretName == "" {
				t.Error("decoded credential has empty SecretName")
			}
			if cred.SecretNamespace == "" {
				t.Error("decoded credential has empty SecretNamespace")
			}
			if cred.UsernameKey == "" {
				t.Error("decoded credential has empty UsernameKey")
			}
			if cred.PasswordKey == "" {
				t.Error("decoded credential has empty PasswordKey")
			}

			decoded = append(decoded, cred)
		})
	}

	// Cross-check: all producers use the same key conventions.
	if len(decoded) >= 2 {
		for i := 1; i < len(decoded); i++ {
			if decoded[i].UsernameKey != decoded[0].UsernameKey {
				t.Errorf("credential key mismatch: %q uses usernameKey=%q but %q uses %q",
					producers[0].name, decoded[0].UsernameKey,
					producers[i].name, decoded[i].UsernameKey)
			}
			if decoded[i].PasswordKey != decoded[0].PasswordKey {
				t.Errorf("credential key mismatch: %q uses passwordKey=%q but %q uses %q",
					producers[0].name, decoded[0].PasswordKey,
					producers[i].name, decoded[i].PasswordKey)
			}
		}
	}
}

// TestCredentialContract_DevOnlyFlag verifies that each credential producer
// sets the DevOnly flag correctly:
//   - postgresql: DevOnly=true (trust auth, no real password)
//   - password-rotation: DevOnly=false (real rotation, real credential)
//
// This prevents DevOnly from silently disappearing or flipping.
func TestCredentialContract_DevOnlyFlag(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	ctx := context.Background()

	tests := []struct {
		name        string
		block       blocks.BlockRuntime
		request     blocks.ReconcileRequest
		wantDevOnly bool
	}{
		{
			name:  "datastore.postgresql must be DevOnly=true",
			block: &postgresql.Block{},
			request: blocks.ReconcileRequest{
				ClusterName:      "devonly-test",
				ClusterNamespace: "default",
				BlockRef: block.BlockRef{
					Kind: "datastore.postgresql",
					Name: "pg",
					Parameters: map[string]string{
						"version":  "16",
						"replicas": "1",
					},
				},
				ResolvedInputs: map[string]string{},
			},
			wantDevOnly: true,
		},
		{
			name:  "security.password-rotation must be DevOnly=false",
			block: &passwordrotation.Block{},
			request: blocks.ReconcileRequest{
				ClusterName:      "devonly-test",
				ClusterNamespace: "default",
				BlockRef: block.BlockRef{
					Kind: "security.password-rotation",
					Name: "rotator",
					Parameters: map[string]string{
						"rotationSchedule": "0 0 */7 * *",
						"passwordLength":   "32",
					},
				},
				ResolvedInputs: map[string]string{
					"dsn": "postgresql://postgres@devonly-test-pg.default.svc:5432/postgres",
				},
			},
			wantDevOnly: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.block.Reconcile(ctx, c, tt.request)
			if err != nil {
				t.Fatalf("Reconcile failed: %v", err)
			}
			cred, err := block.DecodeCredentialRef(result.Outputs["credential"])
			if err != nil {
				t.Fatalf("DecodeCredentialRef failed: %v", err)
			}
			if cred.DevOnly != tt.wantDevOnly {
				t.Errorf("DevOnly: got %v, want %v", cred.DevOnly, tt.wantDevOnly)
			}
		})
	}
}

// TestCredentialContract_PostgreSQLDSNCompat verifies that postgresql
// continues to output both "dsn" (backward compat, dev-only) and
// "credential" ports. The dsn path must not silently disappear.
func TestCredentialContract_PostgreSQLDSNCompat(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	b := &postgresql.Block{}
	result, err := b.Reconcile(context.Background(), c, blocks.ReconcileRequest{
		ClusterName:      "compat-test",
		ClusterNamespace: "default",
		BlockRef: block.BlockRef{
			Kind: "datastore.postgresql",
			Name: "pg",
			Parameters: map[string]string{
				"version":  "16",
				"replicas": "1",
			},
		},
		ResolvedInputs: map[string]string{},
	})
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// Both outputs must be present.
	if result.Outputs["dsn"] == "" {
		t.Error("expected dsn output to be present (backward compat)")
	}
	if result.Outputs["credential"] == "" {
		t.Error("expected credential output to be present (production path)")
	}

	// Credential must be decodable.
	cred, err := block.DecodeCredentialRef(result.Outputs["credential"])
	if err != nil {
		t.Fatalf("credential output is not decodable: %v", err)
	}

	// DSN must not contain a password (trust auth, dev-only).
	// Format: postgresql://user:password@host or postgresql://user@host (no password).
	dsn := result.Outputs["dsn"]
	atIdx := strings.Index(dsn, "@")
	if atIdx > 0 {
		// Extract the userinfo part (after "://" and before "@").
		userinfo := dsn[strings.Index(dsn, "://")+3 : atIdx]
		if strings.Contains(userinfo, ":") {
			t.Errorf("dsn should not contain a password (dev-only trust auth), got: %s", dsn)
		}
	}

	// Credential must be marked dev-only.
	if !cred.DevOnly {
		t.Error("postgresql credential must be DevOnly=true")
	}
}
