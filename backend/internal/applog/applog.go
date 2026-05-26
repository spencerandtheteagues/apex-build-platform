// Package applog provides structured logging using the standard log/slog package.
// In production (LOG_FORMAT=json), logs are emitted as JSON for easy parsing in
// Render, Datadog, Logtail, etc. In development, human-readable text is used.
package applog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"
)

// Init configures the global slog logger based on environment variables.
// Call once from main() before any other initialization.
func Init() {
	level := parseLevel(os.Getenv("LOG_LEVEL"))
	format := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT")))

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ─── Build lifecycle ──────────────────────────────────────────────────────────

func BuildStarted(buildID string, userID uint, mode, powerMode, description string) {
	slog.Info("build_started",
		"event", "build_started",
		"build_id", buildID,
		"user_id", userID,
		"mode", mode,
		"power_mode", powerMode,
		"description_len", len(description),
	)
}

func BuildPhaseStarted(buildID, phase string, agentCount int) {
	slog.Info("build_phase_started",
		"event", "build_phase_started",
		"build_id", buildID,
		"phase", phase,
		"agent_count", agentCount,
	)
}

func BuildPhaseComplete(buildID, phase string) {
	slog.Info("build_phase_complete",
		"event", "build_phase_complete",
		"build_id", buildID,
		"phase", phase,
	)
}

func BuildCompleted(buildID string, userID uint, fileCount int, totalCostUSD float64, durationMs int64) {
	slog.Info("build_completed",
		"event", "build_completed",
		"build_id", buildID,
		"user_id", userID,
		"file_count", fileCount,
		"cost_usd", totalCostUSD,
		"duration_ms", durationMs,
	)
}

func BuildFailed(buildID string, userID uint, reason string, durationMs int64) {
	slog.Error("build_failed",
		"event", "build_failed",
		"build_id", buildID,
		"user_id", userID,
		"reason", reason,
		"duration_ms", durationMs,
	)
}

// ─── WebSocket ────────────────────────────────────────────────────────────────

func WSConnected(buildID string, userID uint) {
	slog.Info("ws_connected",
		"event", "ws_connected",
		"build_id", buildID,
		"user_id", userID,
	)
}

func WSDisconnected(buildID string, userID uint, reason string) {
	slog.Info("ws_disconnected",
		"event", "ws_disconnected",
		"build_id", buildID,
		"user_id", userID,
		"reason", reason,
	)
}

func WSRejected(buildID string, reason string) {
	slog.Warn("ws_rejected",
		"event", "ws_rejected",
		"build_id", buildID,
		"reason", reason,
	)
}

// ─── AI provider ─────────────────────────────────────────────────────────────

func AICallStarted(buildID, taskID, provider, model, capability string, promptLen int) {
	slog.Debug("ai_call_started",
		"event", "ai_call_started",
		"build_id", buildID,
		"task_id", taskID,
		"provider", provider,
		"model", model,
		"capability", capability,
		"prompt_len", promptLen,
	)
}

func AICallSucceeded(buildID, taskID, provider, model string, inputTokens, outputTokens int, costUSD float64, durationMs int64) {
	slog.Info("ai_call_succeeded",
		"event", "ai_call_succeeded",
		"build_id", buildID,
		"task_id", taskID,
		"provider", provider,
		"model", model,
		"input_tokens", inputTokens,
		"output_tokens", outputTokens,
		"cost_usd", costUSD,
		"duration_ms", durationMs,
	)
}

func AICallFailed(buildID, taskID, provider, model string, err error) {
	slog.Error("ai_call_failed",
		"event", "ai_call_failed",
		"build_id", buildID,
		"task_id", taskID,
		"provider", provider,
		"model", model,
		"error", err.Error(),
	)
}

func AIProviderFallback(buildID, taskID, fromProvider, toProvider, reason string) {
	slog.Warn("ai_provider_fallback",
		"event", "ai_provider_fallback",
		"build_id", buildID,
		"task_id", taskID,
		"from_provider", fromProvider,
		"to_provider", toProvider,
		"reason", reason,
	)
}

// ─── Auth ─────────────────────────────────────────────────────────────────────

func AuthFailed(endpoint, reason, clientIP string) {
	slog.Warn("auth_failed",
		"event", "auth_failed",
		"endpoint", endpoint,
		"reason", reason,
		"client_ip", clientIP,
	)
}

// ─── Generic helpers ──────────────────────────────────────────────────────────

func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}

func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

func WithContext(ctx context.Context) *slog.Logger {
	return slog.Default()
}

// Operation emits one redacted, structured platform-operation event. Use it for
// high-volume operational telemetry where every start, handoff, success, and
// failure needs the same correlation shape in production logs.
func Operation(operation string, fields map[string]any) {
	operation = strings.TrimSpace(operation)
	if operation == "" {
		operation = "unknown"
	}

	safe := redactMap(fields)
	if strings.TrimSpace(fmt.Sprint(safe["operation_id"])) == "" {
		safe["operation_id"] = NewOperationID()
	}

	attrs := []slog.Attr{
		slog.String("event", "platform_operation"),
		slog.String("operation", operation),
	}
	keys := make([]string, 0, len(safe))
	for key := range safe {
		if key == "event" || key == "operation" || strings.TrimSpace(key) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		attrs = append(attrs, slog.Any(key, safe[key]))
	}

	ctx := context.Background()
	switch operationLogLevel(safe) {
	case slog.LevelError:
		slog.LogAttrs(ctx, slog.LevelError, "platform_operation", attrs...)
	case slog.LevelWarn:
		slog.LogAttrs(ctx, slog.LevelWarn, "platform_operation", attrs...)
	default:
		slog.LogAttrs(ctx, slog.LevelInfo, "platform_operation", attrs...)
	}
}

// StartOperation captures a start timestamp and returns a finish function that
// writes the terminal event with duration and optional error details.
func StartOperation(operation string, fields map[string]any) func(status string, err error, extra map[string]any) {
	startedAt := time.Now().UTC()
	base := cloneMap(fields)
	if strings.TrimSpace(fmt.Sprint(base["operation_id"])) == "" {
		base["operation_id"] = NewOperationID()
	}
	base["operation_started_at"] = startedAt.Format(time.RFC3339Nano)

	return func(status string, err error, extra map[string]any) {
		merged := cloneMap(base)
		for key, value := range extra {
			merged[key] = value
		}
		if strings.TrimSpace(status) == "" {
			if err != nil {
				status = "failed"
			} else {
				status = "success"
			}
		}
		merged["status"] = status
		if _, ok := merged["duration_ms"]; !ok {
			merged["duration_ms"] = time.Since(startedAt).Milliseconds()
		}
		merged["operation_finished_at"] = time.Now().UTC().Format(time.RFC3339Nano)
		if err != nil {
			merged["error"] = err.Error()
		}
		Operation(operation, merged)
	}
}

func NewOperationID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		return "op_" + hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("op_%d", time.Now().UnixNano())
}

func operationLogLevel(fields map[string]any) slog.Level {
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(fields["status"])))
	if status == "failed" || status == "error" || strings.TrimSpace(fmt.Sprint(fields["error"])) != "" {
		return slog.LevelError
	}
	if status == "blocked" || status == "cancelled" || status == "degraded" || status == "warning" {
		return slog.LevelWarn
	}
	if code, ok := numericStatus(fields["http_status"]); ok && code >= 400 {
		if code >= 500 {
			return slog.LevelError
		}
		return slog.LevelWarn
	}
	return slog.LevelInfo
}

func numericStatus(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func redactMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = redactValue(key, value)
	}
	return out
}

func redactValue(key string, value any) any {
	if isSensitiveLogKey(key) {
		return "[redacted]"
	}
	switch v := value.(type) {
	case map[string]any:
		return redactMap(v)
	case map[string]string:
		out := make(map[string]any, len(v))
		for nestedKey, nestedValue := range v {
			out[nestedKey] = redactValue(nestedKey, nestedValue)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = redactValue(key, item)
		}
		return out
	default:
		return value
	}
}

func isSensitiveLogKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
	if normalized == "" {
		return false
	}
	sensitiveFragments := []string{
		"password",
		"passwd",
		"secret",
		"token",
		"api_key",
		"apikey",
		"authorization",
		"cookie",
		"csrf",
		"credential",
		"session",
	}
	for _, fragment := range sensitiveFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}
