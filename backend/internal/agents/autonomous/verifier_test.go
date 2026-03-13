package autonomous

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type verifierTestAI struct {
	calls int
}

func (v *verifierTestAI) Generate(_ context.Context, _ string, _ AIOptions) (string, error) {
	v.calls++
	return `FILE: package.json
SEARCH: "build": "node -e \"console.error('boom'); process.exit(1)\""
REPLACE: "build": "node -e \"process.exit(0)\""
---`, nil
}

func (v *verifierTestAI) Analyze(_ context.Context, _ string, _ string, _ AIOptions) (string, error) {
	return "", nil
}

func writeVerifierFixture(t *testing.T, dir string) {
	t.Helper()

	const pkg = `{
  "name": "verifier-fixture",
  "version": "1.0.0",
  "private": true,
  "scripts": {
    "build": "node -e \"console.error('boom'); process.exit(1)\"",
    "test": "node -e \"process.exit(0)\""
  }
}
`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
}

func TestBuildVerifierVerifyZeroRetriesDisablesAutoFixLoop(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	writeVerifierFixture(t, workDir)

	ai := &verifierTestAI{}
	verifier := NewBuildVerifier(ai, workDir)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := verifier.Verify(ctx, 0)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if result.Passed {
		t.Fatalf("expected verification to fail without retries")
	}
	if len(result.FixesApplied) != 0 {
		t.Fatalf("expected zero applied fixes without retries, got %d", len(result.FixesApplied))
	}
	if ai.calls != 0 {
		t.Fatalf("expected no AI fix attempts when retries are disabled, got %d calls", ai.calls)
	}
}
