package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSMiddlewareAllowsLoopbackDevelopmentOrigins(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	gin.SetMode(gin.TestMode)

	server := &Server{}
	router := gin.New()
	router.Use(server.CORSMiddleware())
	router.OPTIONS("/api/v1/build/preflight", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/build/preflight", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5180")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected preflight 204, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:5180" {
		t.Fatalf("expected loopback origin to be allowed, got %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected credentials header true, got %q", got)
	}
}
