package blocks_test

import (
	"context"
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
