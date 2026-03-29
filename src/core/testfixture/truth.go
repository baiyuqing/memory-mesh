// truth.go centralises the keyword assertions for stub/dev-only semantics.
// Tests that verify reconcile messages or descriptions should import these
// lists instead of hardcoding keywords locally.
package testfixture

import "testing"

// PostgreSQLDevOnlyKeywords are the keywords that must appear in the
// postgresql reconcile message to prove it is still dev-only.
var PostgreSQLDevOnlyKeywords = []string{
	"dev-only",
	"trust auth",
	"no password enforcement",
	"credential port",
}

// PasswordRotationStubKeywords are the keywords that must appear in the
// password-rotation reconcile message to prove it is still a stub.
var PasswordRotationStubKeywords = []string{
	"stub",
	"Secret scaffold only",
	"ALTER USER not yet implemented",
}

// ProductionReadyBannedPhrases are phrases that must NOT appear in
// stub/dev-only reconcile messages. If any of these appear, the message
// is misleadingly claiming production readiness.
var ProductionReadyBannedPhrases = []string{
	"rotation complete",
	"fully operational",
	"production ready",
}

// BlockDescription returns the canonical description for a block kind
// from Phase1Blocks. It fails the test if the kind is not found.
func BlockDescription(t *testing.T, kind string) string {
	t.Helper()
	for _, b := range Phase1Blocks() {
		d := b.Descriptor()
		if d.Kind == kind {
			return d.Description
		}
	}
	t.Fatalf("kind %q not found in Phase1Blocks()", kind)
	return ""
}
