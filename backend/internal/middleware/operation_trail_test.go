package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestOperationTrailLogsEveryHTTPRequestWithOutcome(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(original) })

	router := gin.New()
	router.Use(RequestID())
	router.Use(OperationTrail())
	router.GET("/api/v1/build/:buildId/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/build/build-123/status?token=secret", nil)
	req.Header.Set("X-Request-ID", "req-http")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected operation log line")
	}
	if strings.Contains(line, "secret") {
		t.Fatalf("operation trail leaked query secret: %s", line)
	}

	var event map[string]any
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		t.Fatalf("operation trail log is not valid JSON: %v\n%s", err, line)
	}
	if event["event"] != "platform_operation" || event["operation"] != "http.request" || event["status"] != "success" {
		t.Fatalf("unexpected event fields: %+v", event)
	}
	if event["request_id"] != "req-http" || event["method"] != http.MethodGet || event["route"] != "/api/v1/build/:buildId/status" || event["build_id"] != "build-123" {
		t.Fatalf("missing request correlation fields: %+v", event)
	}
	if event["http_status"] != float64(http.StatusOK) {
		t.Fatalf("http_status = %v, want 200", event["http_status"])
	}
}
