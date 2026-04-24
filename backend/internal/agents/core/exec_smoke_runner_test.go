package core

import (
	"context"
	"testing"
	"time"
)

func TestExecSmokeRunner_SuccessfulCommand(t *testing.T) {
	r := NewExecSmokeRunner("")
	output, code, err := r.RunSmokeTest(context.Background(), "echo hello", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if output != "hello" {
		t.Fatalf("output = %q, want %q", output, "hello")
	}
}

func TestExecSmokeRunner_FailingCommand_CapturesExitCode(t *testing.T) {
	r := NewExecSmokeRunner("")
	_, code, err := r.RunSmokeTest(context.Background(), "false", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected exec error: %v", err)
	}
	if code == 0 {
		t.Fatal("expected non-zero exit code from 'false'")
	}
}

func TestExecSmokeRunner_Timeout_ReturnsCtxError(t *testing.T) {
	r := NewExecSmokeRunner("")
	// Sleep longer than the timeout
	_, code, err := r.RunSmokeTest(context.Background(), "sleep 60", 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected context deadline error, got nil")
	}
	if code != -1 {
		t.Fatalf("expected exit code -1 on timeout, got %d", code)
	}
}

func TestExecSmokeRunner_EmptyCommand_Noop(t *testing.T) {
	r := NewExecSmokeRunner("")
	output, code, err := r.RunSmokeTest(context.Background(), "", 5*time.Second)
	if err != nil {
		t.Fatalf("empty command should not error: %v", err)
	}
	if code != 0 {
		t.Fatalf("empty command should return exit 0, got %d", code)
	}
	if output != "" {
		t.Fatalf("empty command should produce no output, got %q", output)
	}
}

func TestExecSmokeRunner_StderrCaptured(t *testing.T) {
	r := NewExecSmokeRunner("")
	output, code, err := r.RunSmokeTest(context.Background(), "echo stderr_test >&2; exit 1", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if output == "" {
		t.Fatal("expected stderr to be captured in output")
	}
}

func TestExecSmokeRunner_CancelledContext_Stops(t *testing.T) {
	r := NewExecSmokeRunner("")
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		r.RunSmokeTest(ctx, "sleep 60", 30*time.Second) //nolint:errcheck
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("command did not stop after context cancellation")
	}
}
