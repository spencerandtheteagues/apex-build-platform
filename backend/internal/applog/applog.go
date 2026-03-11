// Package applog provides structured logging using the standard log/slog package.
// In production (LOG_FORMAT=json), logs are emitted as JSON for easy parsing in
// Render, Datadog, Logtail, etc. In development, human-readable text is used.
package applog

import (
	"context"
	"log/slog"
	"os"
	"strings"
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
