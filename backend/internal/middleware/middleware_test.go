package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewIPRateLimiter(t *testing.T) {
	tests := []struct {
		name      string
		rateLimit rate.Limit
		burst     int
	}{
		{
			name:      "standard rate limit",
			rateLimit: rate.Limit(100),
			burst:     10,
		},
		{
			name:      "high rate limit",
			rateLimit: rate.Limit(1000),
			burst:     50,
		},
		{
			name:      "low rate limit",
			rateLimit: rate.Limit(1),
			burst:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewIPRateLimiter(tt.rateLimit, tt.burst)

			require.NotNil(t, limiter)
			assert.Equal(t, tt.rateLimit, limiter.rate)
			assert.Equal(t, tt.burst, limiter.burst)
			assert.NotNil(t, limiter.limiters)
		})
	}
}

func TestIPRateLimiter_GetLimiter(t *testing.T) {
	limiter := NewIPRateLimiter(rate.Limit(10), 5)

	t.Run("creates new limiter for new IP", func(t *testing.T) {
		l1 := limiter.GetLimiter("192.168.1.1")
		require.NotNil(t, l1)

		// Should get the same limiter for the same IP
		l2 := limiter.GetLimiter("192.168.1.1")
		assert.Equal(t, l1, l2)
	})

	t.Run("creates different limiters for different IPs", func(t *testing.T) {
		l1 := limiter.GetLimiter("192.168.1.1")
		l2 := limiter.GetLimiter("192.168.1.2")
		l3 := limiter.GetLimiter("10.0.0.1")

		assert.NotNil(t, l1)
		assert.NotNil(t, l2)
		assert.NotNil(t, l3)
	})

	t.Run("concurrent access is safe", func(t *testing.T) {
		var wg sync.WaitGroup
		ips := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4", "5.5.5.5"}

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				ip := ips[idx%len(ips)]
				l := limiter.GetLimiter(ip)
				assert.NotNil(t, l)
			}(i)
		}

		wg.Wait()
	})
}

func TestRateLimitMiddleware(t *testing.T) {
	// Reset global rate limiter for tests
	globalRateLimiter = nil
	InitRateLimiter(60, 5) // 60 per minute = 1 per second, burst of 5

	tests := []struct {
		name           string
		requestCount   int
		expectedStatus int
		expectBlocked  bool
	}{
		{
			name:           "single request passes",
			requestCount:   1,
			expectedStatus: http.StatusOK,
			expectBlocked:  false,
		},
		{
			name:           "burst requests pass",
			requestCount:   5,
			expectedStatus: http.StatusOK,
			expectBlocked:  false,
		},
		{
			name:           "exceeding burst gets blocked",
			requestCount:   10,
			expectedStatus: http.StatusTooManyRequests,
			expectBlocked:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset rate limiter for each test
			globalRateLimiter = nil
			InitRateLimiter(60, 5)

			router := gin.New()
			router.Use(RateLimit())
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			var lastStatus int
			blocked := false

			for i := 0; i < tt.requestCount; i++ {
				w := httptest.NewRecorder()
				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-For", "192.168.1.1")
				router.ServeHTTP(w, req)
				lastStatus = w.Code

				if w.Code == http.StatusTooManyRequests {
					blocked = true
					break
				}
			}

			if tt.expectBlocked {
				assert.True(t, blocked)
				assert.Equal(t, http.StatusTooManyRequests, lastStatus)
			} else {
				assert.Equal(t, tt.expectedStatus, lastStatus)
			}
		})
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		requestID := c.GetString("request_id")
		c.JSON(http.StatusOK, gin.H{"request_id": requestID})
	})

	t.Run("generates request ID when not provided", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
	})

	t.Run("uses provided request ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-ID", "custom-request-id-123")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "custom-request-id-123", w.Header().Get("X-Request-ID"))
	})
}

func TestCORSMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(CORS())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	allowedOrigins := []string{
		"http://localhost:3000",
		"http://localhost:5173",
		"http://127.0.0.1:3000",
		"https://apex.build",
	}

	t.Run("allows configured origins", func(t *testing.T) {
		for _, origin := range allowedOrigins {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set("Origin", origin)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, origin, w.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("blocks unconfigured origins", func(t *testing.T) {
		blockedOrigins := []string{
			"http://malicious.com",
			"https://hacker.site",
			"http://localhost:4000",
		}

		for _, origin := range blockedOrigins {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set("Origin", origin)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code) // Request still succeeds
			assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("handles preflight OPTIONS requests", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("OPTIONS", "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("sets required CORS headers", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		router.ServeHTTP(w, req)

		assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	})
}

func TestSecurityMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(Security())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	t.Run("sets security headers", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
		assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
		assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
		assert.NotEmpty(t, w.Header().Get("Content-Security-Policy"))
		assert.NotEmpty(t, w.Header().Get("Strict-Transport-Security"))
	})
}

func TestRecoveryMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(Recovery())
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})
	router.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	t.Run("recovers from panic", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/panic", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Internal server error")
	})

	t.Run("does not affect normal requests", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ok", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestAPIKeyAuthMiddleware(t *testing.T) {
	validAPIKeys := map[string]string{
		"valid-api-key-1":   "service-a",
		"valid-api-key-2":   "service-b",
		"webhook-api-key":   "webhook-service",
	}

	router := gin.New()
	router.Use(APIKeyAuth(validAPIKeys))
	router.GET("/test", func(c *gin.Context) {
		serviceName := c.GetString("service_name")
		c.JSON(http.StatusOK, gin.H{"service": serviceName})
	})

	tests := []struct {
		name           string
		apiKey         string
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "valid API key 1",
			apiKey:         "valid-api-key-1",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid API key 2",
			apiKey:         "valid-api-key-2",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing API key",
			apiKey:         "",
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "API_KEY_MISSING",
		},
		{
			name:           "invalid API key",
			apiKey:         "invalid-key",
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "INVALID_API_KEY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			if tt.apiKey != "" {
				req.Header.Set("X-API-Key", tt.apiKey)
			}
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				assert.Contains(t, w.Body.String(), tt.expectedCode)
			}
		})
	}
}

func TestMaintenanceMiddleware(t *testing.T) {
	t.Run("maintenance mode enabled", func(t *testing.T) {
		router := gin.New()
		router.Use(Maintenance(true, "System under maintenance"))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
		router.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "healthy"})
		})

		// Regular endpoint should be blocked
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		assert.Contains(t, w.Body.String(), "System under maintenance")

		// Health check should still work
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("maintenance mode disabled", func(t *testing.T) {
		router := gin.New()
		router.Use(Maintenance(false, ""))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestTimeoutMiddleware(t *testing.T) {
	t.Run("request completes within timeout", func(t *testing.T) {
		router := gin.New()
		router.Use(Timeout(time.Second * 5))
		router.GET("/fast", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/fast", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// Note: Testing timeout behavior is complex due to goroutine handling
	// The actual timeout test would require more sophisticated setup
}

func TestGenerateRequestID(t *testing.T) {
	t.Run("generates unique IDs", func(t *testing.T) {
		ids := make(map[string]bool)

		for i := 0; i < 100; i++ {
			id := generateRequestID()
			assert.NotEmpty(t, id)
			assert.False(t, ids[id], "Duplicate ID generated: %s", id)
			ids[id] = true
		}
	})

	t.Run("ID format is consistent", func(t *testing.T) {
		id := generateRequestID()
		assert.Contains(t, id, "-")
	})
}

func TestErrorResponse(t *testing.T) {
	t.Run("error response structure", func(t *testing.T) {
		resp := ErrorResponse{
			Error:     "Test error",
			Code:      "TEST_ERROR",
			Timestamp: time.Now().UTC(),
			RequestID: "test-123",
			Details: map[string]interface{}{
				"key": "value",
			},
		}

		assert.Equal(t, "Test error", resp.Error)
		assert.Equal(t, "TEST_ERROR", resp.Code)
		assert.Equal(t, "test-123", resp.RequestID)
		assert.NotNil(t, resp.Details)
		assert.Equal(t, "value", resp.Details["key"])
	})
}

// Benchmarks
func BenchmarkRateLimiter_GetLimiter(b *testing.B) {
	limiter := NewIPRateLimiter(rate.Limit(1000), 50)
	ips := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4", "5.5.5.5"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.GetLimiter(ips[i%len(ips)])
	}
}

func BenchmarkRateLimitMiddleware(b *testing.B) {
	globalRateLimiter = nil
	InitRateLimiter(10000, 100)

	router := gin.New()
	router.Use(RateLimit())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkCORSMiddleware(b *testing.B) {
	router := gin.New()
	router.Use(CORS())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		router.ServeHTTP(w, req)
	}
}
