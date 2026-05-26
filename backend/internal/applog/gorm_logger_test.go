package applog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	gormlogger "gorm.io/gorm/logger"
)

func TestGormOperationLoggerTraceEmitsRedactedStructuredQuery(t *testing.T) {
	t.Setenv("APEX_DB_QUERY_LOG_MODE", "all")

	var buf bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(original) })

	logger := NewGormOperationLogger(gormlogger.Default.LogMode(gormlogger.Silent))
	logger.Trace(context.Background(), time.Now().Add(-25*time.Millisecond), func() (string, int64) {
		return "SELECT * FROM users WHERE email = 'secret@example.com' AND id = 42 AND api_key = 'sk-never-log-this'", 1
	}, nil)

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected db.query operation log")
	}
	if strings.Contains(line, "secret@example.com") || strings.Contains(line, "sk-never-log-this") || strings.Contains(line, " 42") {
		t.Fatalf("db query log leaked interpolated SQL values: %s", line)
	}

	var event map[string]any
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		t.Fatalf("operation log is not valid JSON: %v\n%s", err, line)
	}
	if event["event"] != "platform_operation" || event["operation"] != "db.query" {
		t.Fatalf("unexpected operation event: %+v", event)
	}
	if event["status"] != "success" || event["sql_verb"] != "SELECT" || event["rows_affected"] != float64(1) {
		t.Fatalf("missing query fields: %+v", event)
	}
	sqlText, _ := event["sql_sanitized"].(string)
	if !strings.Contains(sqlText, "email = '?'") || !strings.Contains(sqlText, "id = ?") {
		t.Fatalf("SQL was not sanitized as expected: %q", sqlText)
	}
}

func TestGormOperationLoggerTraceEmitsFailures(t *testing.T) {
	t.Setenv("APEX_DB_QUERY_LOG_MODE", "error")

	var buf bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(original) })

	logger := NewGormOperationLogger(gormlogger.Default.LogMode(gormlogger.Silent))
	logger.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "UPDATE users SET password = 'super-secret' WHERE id = 7", 0
	}, errors.New("boom"))

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected failed db.query operation log")
	}
	if strings.Contains(line, "super-secret") || strings.Contains(line, " id = 7") {
		t.Fatalf("db query failure log leaked interpolated SQL values: %s", line)
	}
	var event map[string]any
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		t.Fatalf("operation log is not valid JSON: %v\n%s", err, line)
	}
	if event["status"] != "failed" || event["error"] != "boom" || event["sql_verb"] != "UPDATE" {
		t.Fatalf("missing failure fields: %+v", event)
	}
}
