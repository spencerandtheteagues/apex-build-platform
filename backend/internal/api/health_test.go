package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"apex-build/internal/ai"
	"apex-build/internal/auth"
	"apex-build/internal/cache"
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

func TestFeatureReadinessAPIV1RouteShape(t *testing.T) {
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
	registry.SetPhase(startup.PhaseReady)
	server.SetReadinessRegistry(registry)

	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.GET("/health/features", server.FeatureReadiness)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health/features", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var payload struct {
		Status string `json:"status"`
		Ready  bool   `json:"ready"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "healthy", payload.Status)
	require.True(t, payload.Ready)
}

func TestPlatformTruthIncludesBackendOwnedPlansAndStack(t *testing.T) {
	t.Helper()

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
	registry.SetPhase(startup.PhaseReady)
	server.SetReadinessRegistry(registry)

	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.GET("/platform/truth", server.PlatformTruth)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/platform/truth", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var payload struct {
		Stack struct {
			BackendGo string `json:"backend_go"`
			Node      string `json:"node"`
			Frontend  string `json:"frontend"`
		} `json:"stack"`
		Plans []struct {
			Name              string `json:"name"`
			MonthlyPriceCents int64  `json:"monthly_price_cents"`
		} `json:"plans"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "1.26+", payload.Stack.BackendGo)
	require.Equal(t, "20+", payload.Stack.Node)
	require.Contains(t, payload.Stack.Frontend, "Vite 4")

	planPrices := map[string]int64{}
	for _, plan := range payload.Plans {
		planPrices[plan.Name] = plan.MonthlyPriceCents
	}
	require.Equal(t, int64(2400), planPrices["Builder"])
	require.Equal(t, int64(7900), planPrices["Pro"])
	require.Equal(t, int64(14900), planPrices["Team"])
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
	services, ok := payload["services"].([]any)
	require.True(t, ok)
	require.Len(t, services, 2)
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

func TestFeatureReadinessReflectsRuntimeRedisFallbackEvenAfterHealthyStartup(t *testing.T) {
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
	registry.MarkReady("redis_cache", startup.TierOptional, "Redis cache connected", map[string]any{"backend": "redis"})
	registry.SetPhase(startup.PhaseReady)
	server.SetReadinessRegistry(registry)
	server.SetCacheStatusProvider(func() cache.Status {
		return cache.Status{
			Backend:         "memory",
			RedisConfigured: true,
			RedisConnected:  false,
			FallbackReason:  "redis ping failed: maintenance window",
		}
	})

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

func TestFeatureReadinessIncludesRedisAllowlistRemediation(t *testing.T) {
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
	registry.SetPhase(startup.PhaseReady)
	server.SetReadinessRegistry(registry)
	server.SetCacheStatusProvider(func() cache.Status {
		return cache.Status{
			Backend:         "memory",
			RedisConfigured: true,
			RedisConnected:  false,
			FallbackReason:  "redis ping failed: AUTH failed: Client IP address is not in the allowlist.",
			RecommendedFix:  "On Render, point REDIS_URL at the apex-redis internal connection string instead of an external allowlisted Redis URL.",
		}
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/health/features", nil)

	server.FeatureReadiness(context)

	require.Equal(t, http.StatusOK, recorder.Code)

	var payload struct {
		Services []struct {
			Name    string         `json:"name"`
			Details map[string]any `json:"details"`
		} `json:"services"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))

	var redisDetails map[string]any
	for _, service := range payload.Services {
		if service.Name == "redis_cache" {
			redisDetails = service.Details
			break
		}
	}
	require.NotNil(t, redisDetails)
	require.Equal(t,
		"On Render, point REDIS_URL at the apex-redis internal connection string instead of an external allowlisted Redis URL.",
		redisDetails["recommended_fix"],
	)
}

func TestDeepHealthReportsUnavailableWhenPrimaryDatabasePingFails(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := gormDB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	server := NewServer(
		&db.Database{DB: gormDB},
		auth.NewAuthService("test-jwt-secret-with-sufficient-length-1234567890"),
		ai.NewAIRouter("", "", ""),
		nil,
	)

	registry := startup.NewRegistry()
	registry.MarkReady("primary_database", startup.TierCritical, "Database connected", nil)
	registry.MarkReady("auth_service", startup.TierCritical, "Auth initialized", nil)
	registry.SetPhase(startup.PhaseReady)
	server.SetReadinessRegistry(registry)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/ready", nil)

	server.DeepHealth(context)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "unhealthy", payload["status"])
	require.Equal(t, "unavailable", payload["database"])
}
