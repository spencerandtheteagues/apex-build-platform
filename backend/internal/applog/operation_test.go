package applog

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestOperationRedactsSensitiveFieldsAndKeepsCorrelation(t *testing.T) {
	var buf bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(original) })

	Operation("build.task.enqueue", map[string]any{
		"operation_id": "op-123",
		"request_id":   "req-123",
		"build_id":     "build-123",
		"task_id":      "task-123",
		"user_id":      uint(42),
		"provider":     "claude",
		"password":     "never-log-this",
		"api_key":      "sk-never-log-this",
		"metadata": map[string]any{
			"refresh_token": "also-secret",
			"phase":         "planning",
		},
	})

	line := strings.TrimSpace(buf.String())
	if strings.Contains(line, "never-log-this") || strings.Contains(line, "also-secret") {
		t.Fatalf("operation log leaked secret material: %s", line)
	}

	var event map[string]any
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		t.Fatalf("operation log is not valid JSON: %v\n%s", err, line)
	}
	if event["event"] != "platform_operation" {
		t.Fatalf("event = %v, want platform_operation", event["event"])
	}
	if event["operation"] != "build.task.enqueue" {
		t.Fatalf("operation = %v, want build.task.enqueue", event["operation"])
	}
	if event["operation_id"] != "op-123" || event["request_id"] != "req-123" || event["build_id"] != "build-123" || event["task_id"] != "task-123" {
		t.Fatalf("missing correlation fields: %+v", event)
	}
	if event["password"] != "[redacted]" || event["api_key"] != "[redacted]" {
		t.Fatalf("top-level secrets were not redacted: %+v", event)
	}
	metadata, ok := event["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata missing or wrong type: %+v", event["metadata"])
	}
	if metadata["refresh_token"] != "[redacted]" || metadata["phase"] != "planning" {
		t.Fatalf("nested metadata redaction failed: %+v", metadata)
	}
}

func TestOperationFinishReportsSuccessAndFailureDurations(t *testing.T) {
	var buf bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(original) })

	finish := StartOperation("ai.provider.generate", map[string]any{"operation_id": "op-ai", "provider": "claude"})
	finish("failed", errTestOperationFailure{}, map[string]any{"duration_ms": int64(12), "model": "claude-sonnet-4-6"})

	line := strings.TrimSpace(buf.String())
	var event map[string]any
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		t.Fatalf("operation log is not valid JSON: %v\n%s", err, line)
	}
	if event["status"] != "failed" || event["error"] != "test operation failed" {
		t.Fatalf("failure fields missing: %+v", event)
	}
	if event["duration_ms"] != float64(12) {
		t.Fatalf("duration_ms = %v, want 12", event["duration_ms"])
	}
	startedAt, ok := event["operation_started_at"].(string)
	if !ok {
		t.Fatalf("operation_started_at missing: %+v", event)
	}
	if _, err := time.Parse(time.RFC3339Nano, startedAt); err != nil {
		t.Fatalf("operation_started_at not RFC3339Nano: %+v", event)
	}
}

type errTestOperationFailure struct{}

func (errTestOperationFailure) Error() string { return "test operation failed" }
