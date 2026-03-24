// APEX.BUILD Middleware
// Production-ready middleware for error handling, rate limiting, etc.

package middleware

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"apex-build/internal/origins"

	"github.com/gin-gonic/gin"
	goredis "github.com/go-redis/redis/v8"
	"golang.org/x/time/rate"
)

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error     string                 `json:"error"`
	Code      string                 `json:"code"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	RequestID string                 `json:"request_id,omitempty"`
}

// ErrorHandler middleware for consistent error handling
func ErrorHandler() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(param gin.LogFormatterParams) string {
			return fmt.Sprintf("[APEX.BUILD] %s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
				param.ClientIP,
				param.TimeStamp.Format(time.RFC3339),
				param.Method,
				param.Path,
				param.Request.Proto,
				param.StatusCode,
				param.Latency,
				param.Request.UserAgent(),
				param.ErrorMessage,
			)
		},
		Output:    gin.DefaultWriter,
		SkipPaths: []string{"/health"},
	})
}

// Recovery middleware with custom error handling
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Log the panic
		log.Printf("[PANIC RECOVERY] RequestID: %s, Error: %v\nStack: %s",
			requestID, recovered, debug.Stack())

		// Return standardized error response
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:     "Internal server error",
			Code:      "INTERNAL_SERVER_ERROR",
			Timestamp: time.Now().UTC(),
			RequestID: requestID,
		})
	})
}

// RateLimiter represents a rate limiter for a specific client
type RateLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter manages rate limiters for different IP addresses
type IPRateLimiter struct {
	limiters map[string]*RateLimiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
	cleanup  time.Duration
	stopCh   chan struct{} // Channel to signal cleanup goroutine to stop
	scope    string
	window   time.Duration
	limit    int
	shared   sharedRateLimitStore
}

type sharedRateLimitStore interface {
	Allow(ctx context.Context, scope string, identifier string, limit int, window time.Duration) (bool, error)
	Close() error
}

type redisSharedRateLimitStore struct {
	client goredis.UniversalClient
}

// NewIPRateLimiter creates a new IP-based rate limiter
func NewIPRateLimiter(rateLimit rate.Limit, burst int) *IPRateLimiter {
	return NewScopedIPRateLimiter(rateLimit, burst, "api")
}

func NewScopedIPRateLimiter(rateLimit rate.Limit, burst int, scope string) *IPRateLimiter {
	limiter := &IPRateLimiter{
		limiters: make(map[string]*RateLimiter),
		rate:     rateLimit,
		burst:    burst,
		cleanup:  time.Minute * 10, // Clean up old limiters every 10 minutes
		stopCh:   make(chan struct{}),
		scope:    strings.TrimSpace(scope),
		window:   time.Minute,
		limit:    requestsPerMinuteFromRate(rateLimit, burst),
		shared:   newSharedRateLimitStoreFromEnv(),
	}

	// Start cleanup goroutine
	go limiter.cleanupRoutine()

	return limiter
}

// Stop stops the cleanup goroutine
func (irl *IPRateLimiter) Stop() {
	if irl.shared != nil {
		_ = irl.shared.Close()
	}
	close(irl.stopCh)
}

// GetLimiter returns the rate limiter for a given IP
func (irl *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	irl.mu.Lock()
	defer irl.mu.Unlock()

	limiter, exists := irl.limiters[ip]
	if !exists {
		limiter = &RateLimiter{
			limiter:  rate.NewLimiter(irl.rate, irl.burst),
			lastSeen: time.Now(),
		}
		irl.limiters[ip] = limiter
	} else {
		limiter.lastSeen = time.Now()
	}

	return limiter.limiter
}

func (irl *IPRateLimiter) Allow(ip string) bool {
	if irl == nil {
		return true
	}
	if irl.shared != nil && irl.limit > 0 {
		allowed, err := irl.shared.Allow(context.Background(), irl.scope, ip, irl.limit, irl.window)
		if err != nil {
			log.Printf("WARNING: shared rate limit fallback for scope %s: %v", irl.scope, err)
		} else if !allowed {
			return false
		}
	}

	return irl.GetLimiter(ip).Allow()
}

// cleanupRoutine removes old rate limiters to prevent memory leaks
func (irl *IPRateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(irl.cleanup)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			irl.mu.Lock()
			cutoff := time.Now().Add(-time.Hour) // Remove limiters not seen for 1 hour

			for ip, limiter := range irl.limiters {
				if limiter.lastSeen.Before(cutoff) {
					delete(irl.limiters, ip)
				}
			}
			irl.mu.Unlock()
		case <-irl.stopCh:
			return
		}
	}
}

// Global rate limiter instance
var globalRateLimiter *IPRateLimiter

// InitRateLimiter initializes the global rate limiter
func InitRateLimiter(requestsPerMinute int, burst int) {
	rateLimit := rate.Limit(requestsPerMinute) / 60 // Convert per minute to per second
	globalRateLimiter = NewScopedIPRateLimiter(rateLimit, burst, "api")
}

// RateLimit middleware for rate limiting by IP
func RateLimit() gin.HandlerFunc {
	// Initialize with default values if not already initialized
	if globalRateLimiter == nil {
		InitRateLimiter(1000, 50) // Default: 1000 requests per minute, burst of 50
	}

	return func(c *gin.Context) {
		// Get client IP
		clientIP := c.ClientIP()

		// Check if request is allowed
		if !globalRateLimiter.Allow(clientIP) {
			c.JSON(http.StatusTooManyRequests, ErrorResponse{
				Error: "Rate limit exceeded",
				Code:  "RATE_LIMIT_EXCEEDED",
				Details: map[string]interface{}{
					"retry_after": "60s",
					"limit":       "1000 requests per minute",
				},
				Timestamp: time.Now().UTC(),
				RequestID: c.GetHeader("X-Request-ID"),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequestID middleware adds a unique request ID to each request
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

// CORS middleware for handling cross-origin requests
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if origins.IsAllowedOrigin(origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With, X-Request-ID")
		c.Header("Access-Control-Expose-Headers", "X-Request-ID")
		c.Header("Access-Control-Max-Age", "86400") // 24 hours

		// Handle preflight OPTIONS requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Security middleware adds security headers
func Security() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Security headers
		c.Header("X-Content-Type-Options", "nosniff")
		isPreviewProxy := strings.HasPrefix(c.Request.URL.Path, "/api/v1/preview/proxy/")
		if !isPreviewProxy {
			c.Header("X-Frame-Options", "DENY")
		}
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", contentSecurityPolicy(c.Request.URL.Path))
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		c.Next()
	}
}

// Timeout middleware adds request timeout
func Timeout(duration time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set timeout for the context
		ctx, cancel := context.WithTimeout(c.Request.Context(), duration)
		defer cancel()

		// Replace request context with timeout context
		c.Request = c.Request.WithContext(ctx)

		// Channel to track completion
		finished := make(chan bool, 1)

		go func() {
			c.Next()
			finished <- true
		}()

		select {
		case <-finished:
			// Request completed normally
			return
		case <-ctx.Done():
			// Request timed out
			c.JSON(http.StatusRequestTimeout, ErrorResponse{
				Error:     "Request timeout",
				Code:      "REQUEST_TIMEOUT",
				Timestamp: time.Now().UTC(),
				RequestID: c.GetHeader("X-Request-ID"),
			})
			c.Abort()
		}
	}
}

// Logging middleware with structured logging
func Logger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[APEX.BUILD] %s - %s \"%s %s\" %d %s %s\n",
			param.TimeStamp.Format(time.RFC3339),
			param.ClientIP,
			param.Method,
			param.Path,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
		)
	})
}

// generateRequestID generates a unique request ID using timestamp + random bytes
func generateRequestID() string {
	// Add 4 random bytes to ensure uniqueness even in tight loops
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(randomBytes))
}

// APIKeyAuth middleware for API key authentication (for webhook endpoints)
func APIKeyAuth(validAPIKeys map[string]string) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:     "API key is required",
				Code:      "API_KEY_MISSING",
				Timestamp: time.Now().UTC(),
				RequestID: c.GetHeader("X-Request-ID"),
			})
			c.Abort()
			return
		}

		// Check if API key is valid
		serviceName, exists := validAPIKeys[apiKey]
		if !exists {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:     "Invalid API key",
				Code:      "INVALID_API_KEY",
				Timestamp: time.Now().UTC(),
				RequestID: c.GetHeader("X-Request-ID"),
			})
			c.Abort()
			return
		}

		// Store service name in context
		c.Set("service_name", serviceName)
		c.Next()
	}
}

// AuthRateLimiter is a stricter rate limiter specifically for auth endpoints
// SECURITY: Prevents brute force attacks on login/register endpoints
var authRateLimiter *IPRateLimiter

// InitAuthRateLimiter initializes the auth-specific rate limiter
func InitAuthRateLimiter() {
	// 10 requests per minute for auth endpoints (much stricter than general)
	authRateLimiter = NewScopedIPRateLimiter(rate.Limit(10)/60, 5, "auth")
}

// AuthRateLimit middleware for strict rate limiting on auth endpoints
// SECURITY: 10 requests/minute with burst of 5 to prevent credential stuffing
func AuthRateLimit() gin.HandlerFunc {
	if authRateLimiter == nil {
		InitAuthRateLimiter()
	}

	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		if !authRateLimiter.Allow(clientIP) {
			log.Printf("⚠️  Auth rate limit exceeded for IP: %s on path: %s", clientIP, c.Request.URL.Path)
			c.JSON(http.StatusTooManyRequests, ErrorResponse{
				Error: "Too many authentication attempts. Please try again later.",
				Code:  "AUTH_RATE_LIMIT_EXCEEDED",
				Details: map[string]interface{}{
					"retry_after": "60s",
					"limit":       "10 requests per minute",
				},
				Timestamp: time.Now().UTC(),
				RequestID: c.GetHeader("X-Request-ID"),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func requestsPerMinuteFromRate(rateLimit rate.Limit, burst int) int {
	perMinute := int(math.Ceil(float64(rateLimit) * 60))
	if perMinute < 1 {
		perMinute = 1
	}
	if perMinute < burst {
		perMinute = burst
	}
	return perMinute
}

func newSharedRateLimitStoreFromEnv() sharedRateLimitStore {
	redisURL := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if redisURL == "" {
		return nil
	}

	opts, err := goredis.ParseURL(redisURL)
	if err != nil {
		log.Printf("WARNING: shared rate limiter disabled - invalid REDIS_URL: %v", err)
		return nil
	}

	client := goredis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("WARNING: shared rate limiter disabled - redis ping failed: %v", err)
		_ = client.Close()
		return nil
	}

	return &redisSharedRateLimitStore{client: client}
}

func rateLimitIdentifier(scope string, identifier string, windowStart time.Time) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(identifier)))
	return fmt.Sprintf("ratelimit:%s:%d:%x", scope, windowStart.UTC().Unix(), sum[:8])
}

func (s *redisSharedRateLimitStore) Allow(ctx context.Context, scope string, identifier string, limit int, window time.Duration) (bool, error) {
	if s == nil || s.client == nil || limit <= 0 {
		return true, nil
	}

	now := time.Now().UTC()
	windowStart := now.Truncate(window)
	key := rateLimitIdentifier(scope, identifier, windowStart)
	count, err := s.client.Incr(ctx, key).Result()
	if err != nil {
		return true, err
	}

	if count == 1 {
		ttl := time.Until(windowStart.Add(window)) + time.Second
		if ttl < time.Second {
			ttl = time.Second
		}
		if err := s.client.Expire(ctx, key, ttl).Err(); err != nil {
			return true, err
		}
	}

	return count <= int64(limit), nil
}

func (s *redisSharedRateLimitStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

// Maintenance middleware for maintenance mode
func Maintenance(enabled bool, message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if enabled {
			// Allow health checks during maintenance
			if c.Request.URL.Path == "/health" {
				c.Next()
				return
			}

			c.JSON(http.StatusServiceUnavailable, ErrorResponse{
				Error: message,
				Code:  "SERVICE_UNAVAILABLE",
				Details: map[string]interface{}{
					"maintenance_mode": true,
				},
				Timestamp: time.Now().UTC(),
				RequestID: c.GetHeader("X-Request-ID"),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
