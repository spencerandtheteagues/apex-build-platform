package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"apex-build/internal/ai"
	"apex-build/internal/auth"
	"apex-build/internal/db"
	"apex-build/internal/startup"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDeepHealthIncludesReadyFlag(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	server := NewServer(
		&db.Database{DB: gormDB},
		auth.NewAuthService("test-jwt-secret-with-sufficient-length-1234567890"),
		ai.NewAIRouter("", "", ""),
		nil,
	)
	registry := startup.NewRegistry()
	registry.MarkReady("primary_database", startup.TierCritical, "Database connected", nil)
	registry.MarkReady("auth_service", startup.TierCritical, "Auth initialized", nil)
	registry.MarkDegraded("payments", startup.TierOptional, "Stripe not configured", map[string]any{"enabled": false})
	registry.SetPhase(startup.PhaseReady)
	server.SetReadinessRegistry(registry)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/ready", nil)

	server.DeepHealth(context)

	require.Equal(t, http.StatusOK, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "healthy", payload["status"])
	require.Equal(t, true, payload["ready"])
	require.Equal(t, "degraded", payload["feature_readiness_status"])
}

func TestFeatureReadinessReturnsDegradedSummaryWithHealthyCriticalServices(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	server := NewServer(
		&db.Database{DB: gormDB},
		auth.NewAuthService("test-jwt-secret-with-sufficient-length-1234567890"),
		ai.NewAIRouter("", "", ""),
		nil,
	)

	registry := startup.NewRegistry()
	registry.MarkReady("primary_database", startup.TierCritical, "Database connected", nil)
	registry.MarkReady("auth_service", startup.TierCritical, "Auth initialized", nil)
	registry.MarkDegraded("redis_cache", startup.TierOptional, "Using in-memory cache fallback", map[string]any{"backend": "memory"})
	registry.SetPhase(startup.PhaseReady)
	server.SetReadinessRegistry(registry)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/health/features", nil)

	server.FeatureReadiness(context)

	require.Equal(t, http.StatusOK, recorder.Code)

	var payload struct {
		Status string        `json:"status"`
		Ready  bool          `json:"ready"`
		Phase  startup.Phase `json:"phase"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "degraded", payload.Status)
	require.True(t, payload.Ready)
	require.Equal(t, startup.PhaseReady, payload.Phase)
}

func TestDeepHealthStaysHealthyWhenOnlyOptionalServicesAreDegraded(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	registry := startup.NewRegistry()
	registry.Register("database", startup.TierCritical, "Waiting for database", nil)
	registry.Register("payments", startup.TierOptional, "Waiting for payments", nil)
	registry.MarkReady("database", startup.TierCritical, "Database connected", nil)
	registry.MarkDegraded("payments", startup.TierOptional, "Stripe not configured", nil)
	registry.SetPhase(startup.PhaseReady)

	server := NewServer(
		&db.Database{DB: gormDB},
		auth.NewAuthService("test-jwt-secret-with-sufficient-length-1234567890"),
		ai.NewAIRouter("", "", ""),
		nil,
	)
	server.SetReadinessRegistry(registry)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/ready", nil)

	server.DeepHealth(context)

	require.Equal(t, http.StatusOK, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "healthy", payload["status"])
	require.Equal(t, "degraded", payload["feature_readiness_status"])
	require.Equal(t, true, payload["ready"])
}

func TestFeatureReadinessEndpointShowsLaunchBlockers(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	registry := startup.NewRegistry()
	registry.Register("http_routes", startup.TierCritical, "Waiting for router activation", nil)

	server := NewServer(
		&db.Database{DB: gormDB},
		auth.NewAuthService("test-jwt-secret-with-sufficient-length-1234567890"),
		ai.NewAIRouter("", "", ""),
		nil,
	)
	server.SetReadinessRegistry(registry)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/health/features", nil)

	server.FeatureReadiness(context)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "starting", payload["status"])
	require.Equal(t, false, payload["ready"])
	require.Len(t, payload["services"], 1)
}

func TestFeatureReadinessReportsOptionalDegradation(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	server := NewServer(
		&db.Database{DB: gormDB},
		auth.NewAuthService("test-jwt-secret-with-sufficient-length-1234567890"),
		ai.NewAIRouter("", "", ""),
		nil,
	)

	registry := startup.NewRegistry()
	registry.MarkReady("bootstrap_http", startup.TierCritical, "Bootstrap HTTP listener ready", nil)
	registry.MarkReady("secrets_validation", startup.TierCritical, "Secrets validated", nil)
	registry.MarkReady("primary_database", startup.TierCritical, "Database connected", nil)
	registry.MarkReady("auth_service", startup.TierCritical, "Auth service ready", nil)
	registry.MarkReady("secrets_manager", startup.TierCritical, "Secrets manager ready", nil)
	registry.MarkReady("http_routes", startup.TierCritical, "HTTP routes active", nil)
	registry.MarkDegraded("redis_cache", startup.TierOptional, "Using in-memory fallback", nil)
	registry.SetPhase(startup.PhaseReady)
	server.SetReadinessRegistry(registry)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/health/features", nil)

	server.FeatureReadiness(context)

	require.Equal(t, http.StatusOK, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "degraded", payload["status"])
	require.Equal(t, true, payload["ready"])
	require.Contains(t, payload["degraded_features"], "redis_cache")
}
