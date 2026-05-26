package applog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	gormlogger "gorm.io/gorm/logger"
)

const defaultDBSlowQueryThreshold = 200 * time.Millisecond

var (
	sqlWhitespaceRE = regexp.MustCompile(`\s+`)
	sqlNumberRE     = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
)

type gormOperationLogger struct {
	delegate      gormlogger.Interface
	mode          string
	slowThreshold time.Duration
	maxSQLChars   int
}

// NewGormOperationLogger wraps a GORM logger and emits redacted db.query
// operation events for query-level debugging. By default every query is logged;
// set APEX_DB_QUERY_LOG_MODE to off, error, or slow to reduce volume.
func NewGormOperationLogger(delegate gormlogger.Interface) gormlogger.Interface {
	if delegate == nil {
		delegate = gormlogger.Default.LogMode(gormlogger.Silent)
	}
	return &gormOperationLogger{
		delegate:      delegate,
		mode:          dbQueryLogMode(),
		slowThreshold: dbSlowQueryThreshold(),
		maxSQLChars:   dbQueryLogMaxChars(),
	}
}

func (l *gormOperationLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	next := *l
	if l.delegate != nil {
		next.delegate = l.delegate.LogMode(level)
	}
	return &next
}

func (l *gormOperationLogger) Info(ctx context.Context, msg string, args ...any) {
	if l.delegate != nil {
		l.delegate.Info(ctx, msg, args...)
	}
}

func (l *gormOperationLogger) Warn(ctx context.Context, msg string, args ...any) {
	if l.delegate != nil {
		l.delegate.Warn(ctx, msg, args...)
	}
}

func (l *gormOperationLogger) Error(ctx context.Context, msg string, args ...any) {
	if l.delegate != nil {
		l.delegate.Error(ctx, msg, args...)
	}
}

func (l *gormOperationLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.delegate != nil {
		l.delegate.Trace(ctx, begin, fc, err)
	}

	elapsed := time.Since(begin)
	if !l.shouldLog(elapsed, err) {
		return
	}

	sqlText, rows := fc()
	sanitizedSQL := sanitizeSQLForLog(sqlText, l.maxSQLChars)
	status := "success"
	if err != nil {
		status = "failed"
	} else if elapsed >= l.slowThreshold {
		status = "slow"
	}

	fields := map[string]any{
		"status":            status,
		"duration_ms":       elapsed.Milliseconds(),
		"slow":              elapsed >= l.slowThreshold,
		"slow_threshold_ms": l.slowThreshold.Milliseconds(),
		"rows_affected":     rows,
		"sql_verb":          sqlVerb(sanitizedSQL),
		"sql_sanitized":     sanitizedSQL,
		"sql_fingerprint":   sqlFingerprint(sanitizedSQL),
	}
	if requestID := contextString(ctx, "request_id", "requestID", "x-request-id"); requestID != "" {
		fields["request_id"] = requestID
	}
	if operationID := contextString(ctx, "operation_id", "operationID", "x-apex-operation-id"); operationID != "" {
		fields["operation_id"] = operationID
	}
	if err != nil {
		fields["error"] = err.Error()
	}
	Operation("db.query", fields)
}

func (l *gormOperationLogger) shouldLog(elapsed time.Duration, err error) bool {
	switch l.mode {
	case "off":
		return false
	case "error":
		return err != nil
	case "slow":
		return err != nil || elapsed >= l.slowThreshold
	default:
		return true
	}
}

func dbQueryLogMode() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("APEX_DB_QUERY_LOG_MODE"))) {
	case "off", "none", "false", "disabled":
		return "off"
	case "error", "errors", "failed", "failure":
		return "error"
	case "slow", "slow_error", "slow-and-error", "slow_and_error":
		return "slow"
	case "", "all", "true", "debug":
		return "all"
	default:
		return "all"
	}
}

func dbSlowQueryThreshold() time.Duration {
	for _, key := range []string{"APEX_DB_SLOW_QUERY_THRESHOLD", "DB_SLOW_QUERY_THRESHOLD"} {
		raw := strings.TrimSpace(os.Getenv(key))
		if raw == "" {
			continue
		}
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			return parsed
		}
		if ms, err := strconv.Atoi(raw); err == nil && ms > 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return defaultDBSlowQueryThreshold
}

func dbQueryLogMaxChars() int {
	raw := strings.TrimSpace(os.Getenv("APEX_DB_QUERY_LOG_SQL_MAX_CHARS"))
	if raw == "" {
		return 2000
	}
	maxChars, err := strconv.Atoi(raw)
	if err != nil || maxChars < 200 {
		return 2000
	}
	return maxChars
}

func sanitizeSQLForLog(sqlText string, maxChars int) string {
	sanitized := redactSQLStringLiterals(sqlText)
	sanitized = sqlNumberRE.ReplaceAllString(sanitized, "?")
	sanitized = strings.TrimSpace(sqlWhitespaceRE.ReplaceAllString(sanitized, " "))
	if maxChars > 0 && len(sanitized) > maxChars {
		return sanitized[:maxChars] + "... [truncated]"
	}
	return sanitized
}

func redactSQLStringLiterals(sqlText string) string {
	var b strings.Builder
	b.Grow(len(sqlText))
	inString := false
	for i := 0; i < len(sqlText); i++ {
		ch := sqlText[i]
		if !inString {
			if ch == '\'' {
				b.WriteString("'?")
				inString = true
				continue
			}
			b.WriteByte(ch)
			continue
		}

		if ch == '\'' {
			if i+1 < len(sqlText) && sqlText[i+1] == '\'' {
				i++
				continue
			}
			b.WriteByte('\'')
			inString = false
		}
	}
	if inString {
		b.WriteByte('\'')
	}
	return b.String()
}

func sqlVerb(sqlText string) string {
	fields := strings.Fields(sqlText)
	if len(fields) == 0 {
		return "unknown"
	}
	return strings.ToUpper(strings.Trim(fields[0], "(;"))
}

func sqlFingerprint(sqlText string) string {
	sum := sha256.Sum256([]byte(sqlText))
	return hex.EncodeToString(sum[:8])
}

func contextString(ctx context.Context, keys ...string) string {
	if ctx == nil {
		return ""
	}
	for _, key := range keys {
		if value := strings.TrimSpace(fmt.Sprint(ctx.Value(key))); value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}
