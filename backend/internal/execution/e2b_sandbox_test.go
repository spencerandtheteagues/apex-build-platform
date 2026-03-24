package execution

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultE2BHelperCommandUsesOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), "runner.mjs")
	if err := os.WriteFile(override, []byte("export {};\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("E2B_RUNNER_PATH", override)

	command, err := defaultE2BHelperCommand()
	if err != nil {
		t.Fatalf("defaultE2BHelperCommand() error = %v", err)
	}
	if len(command) != 2 {
		t.Fatalf("defaultE2BHelperCommand() len = %d, want 2", len(command))
	}
	if command[0] != "node" {
		t.Fatalf("defaultE2BHelperCommand()[0] = %q, want %q", command[0], "node")
	}
	if command[1] != override {
		t.Fatalf("defaultE2BHelperCommand()[1] = %q, want %q", command[1], override)
	}
}

func TestE2BSandboxExecuteWithHelper(t *testing.T) {
	helperPath := filepath.Join(t.TempDir(), "fake-e2b.sh")
	helperScript := `#!/bin/sh
request="$(cat)"
if printf '%s' "$request" | grep -q '"action":"create"'; then
  printf '%s' '{"sandboxId":"sbx-test"}'
  exit 0
fi
if printf '%s' "$request" | grep -q '"action":"write"'; then
  printf '%s' '{"ok":true}'
  exit 0
fi
if printf '%s' "$request" | grep -q '"action":"run"'; then
  printf '%s' '{"exitCode":0,"stdout":"42\n","stderr":""}'
  exit 0
fi
if printf '%s' "$request" | grep -q '"action":"kill"'; then
  printf '%s' '{"killed":true}'
  exit 0
fi
printf '%s' "unexpected request" >&2
exit 1
`
	if err := os.WriteFile(helperPath, []byte(helperScript), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	sandbox := &E2BSandbox{
		apiKey:        "e2b_test_key",
		helperCommand: []string{helperPath},
		executions:    make(map[string]*e2bExecution),
	}

	result, err := sandbox.Execute(context.Background(), "javascript", "console.log(6 * 7)", "")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("result.Status = %q, want %q", result.Status, "completed")
	}
	if got := strings.TrimSpace(result.Output); got != "42" {
		t.Fatalf("strings.TrimSpace(result.Output) = %q, want %q", got, "42")
	}
	if result.ExitCode != 0 {
		t.Fatalf("result.ExitCode = %d, want %d", result.ExitCode, 0)
	}
}

func TestE2BSandboxExecutePropagatesCommandFailure(t *testing.T) {
	helperPath := filepath.Join(t.TempDir(), "fake-e2b.sh")
	helperScript := `#!/bin/sh
request="$(cat)"
if printf '%s' "$request" | grep -q '"action":"create"'; then
  printf '%s' '{"sandboxId":"sbx-test"}'
  exit 0
fi
if printf '%s' "$request" | grep -q '"action":"write"'; then
  printf '%s' '{"ok":true}'
  exit 0
fi
if printf '%s' "$request" | grep -q '"action":"run"'; then
  printf '%s' '{"exitCode":2,"stdout":"","stderr":"boom"}'
  exit 0
fi
if printf '%s' "$request" | grep -q '"action":"kill"'; then
  printf '%s' '{"killed":true}'
  exit 0
fi
printf '%s' "unexpected request" >&2
exit 1
`
	if err := os.WriteFile(helperPath, []byte(helperScript), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	sandbox := &E2BSandbox{
		apiKey:        "e2b_test_key",
		helperCommand: []string{helperPath},
		executions:    make(map[string]*e2bExecution),
	}

	result, err := sandbox.Execute(context.Background(), "javascript", "throw new Error('boom')", "")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("result.Status = %q, want %q", result.Status, "failed")
	}
	if result.ExitCode != 2 {
		t.Fatalf("result.ExitCode = %d, want %d", result.ExitCode, 2)
	}
	if result.ErrorOutput != "boom" {
		t.Fatalf("result.ErrorOutput = %q, want %q", result.ErrorOutput, "boom")
	}
}
