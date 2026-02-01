package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// FixedWindowRateLimiter provides thread-safe rate limiting using a fixed window algorithm
// This is separate from the token bucket RateLimiter in middleware.go and provides
// accurate request counts for rate limit headers
type FixedWindowRateLimiter struct {
	requests    sync.Map      // map[string]*fixedWindowEntry
	limit       int64         // max requests per window
	windowSecs  int64         // window duration in seconds
	cleanupStop chan struct{} // channel to stop cleanup goroutine
}

// fixedWindowEntry tracks request count for a single client
type fixedWindowEntry struct {
	count       int64 // atomic counter for requests in current window
	windowStart int64 // atomic unix timestamp of window start
}

// Global fixed window rate limiter instance (initialized lazily)
var (
	headerRateLimiter     *FixedWindowRateLimiter
	headerRateLimiterOnce sync.Once
)

// NewFixedWindowRateLimiter creates a new fixed window rate limiter with the specified limit and window
func NewFixedWindowRateLimiter(limit int64, windowSecs int64) *FixedWindowRateLimiter {
	rl := &FixedWindowRateLimiter{
		limit:       limit,
		windowSecs:  windowSecs,
		cleanupStop: make(chan struct{}),
	}
	// Start background cleanup goroutine
	go rl.cleanupExpiredEntries()
	return rl
}

// getHeaderRateLimiter returns the singleton rate limiter for headers (1000 req/hour)
func getHeaderRateLimiter() *FixedWindowRateLimiter {
	headerRateLimiterOnce.Do(func() {
		headerRateLimiter = NewFixedWindowRateLimiter(1000, 3600) // 1000 requests per hour
	})
	return headerRateLimiter
}

// Allow checks if a request from the given key should be allowed
// Returns: allowed (bool), remaining count, seconds until reset
func (rl *FixedWindowRateLimiter) Allow(key string) (bool, int64, int64) {
	now := time.Now().Unix()

	// Get or create entry for this key
	entryI, loaded := rl.requests.LoadOrStore(key, &fixedWindowEntry{
		count:       1,
		windowStart: now,
	})
	entry := entryI.(*fixedWindowEntry)

	if !loaded {
		// New entry was created with count=1
		resetIn := rl.windowSecs
		return true, rl.limit - 1, resetIn
	}

	// Check if we need to reset the window using compare-and-swap for thread safety
	for {
		windowStart := atomic.LoadInt64(&entry.windowStart)
		if now-windowStart >= rl.windowSecs {
			// Window has expired, try to reset using CAS
			// Only one goroutine will succeed in resetting
			if atomic.CompareAndSwapInt64(&entry.windowStart, windowStart, now) {
				// We won the race, reset the counter
				atomic.StoreInt64(&entry.count, 1)
				return true, rl.limit - 1, rl.windowSecs
			}
			// Another goroutine reset the window, retry the check
			continue
		}
		// Window is still valid, break out of the loop
		break
	}

	// Increment counter and check limit
	windowStart := atomic.LoadInt64(&entry.windowStart)
	newCount := atomic.AddInt64(&entry.count, 1)
	remaining := rl.limit - newCount
	resetIn := rl.windowSecs - (now - windowStart)

	if remaining < 0 {
		remaining = 0
	}
	if resetIn < 0 {
		resetIn = 0
	}

	if newCount > rl.limit {
		// Rate limit exceeded - decrement the counter we just added
		// so we don't count rejected requests
		atomic.AddInt64(&entry.count, -1)
		return false, 0, resetIn
	}

	return true, remaining, resetIn
}

// cleanupExpiredEntries removes entries that haven't been used in 2x the window duration
func (rl *FixedWindowRateLimiter) cleanupExpiredEntries() {
	ticker := time.NewTicker(time.Duration(rl.windowSecs) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now().Unix()
			expireThreshold := now - (rl.windowSecs * 2) // Keep entries for 2 windows

			rl.requests.Range(func(key, value interface{}) bool {
				entry := value.(*fixedWindowEntry)
				windowStart := atomic.LoadInt64(&entry.windowStart)
				if windowStart < expireThreshold {
					rl.requests.Delete(key)
				}
				return true
			})
		case <-rl.cleanupStop:
			return
		}
	}
}

// StopCleanup stops the cleanup goroutine (call when shutting down)
func (rl *FixedWindowRateLimiter) StopCleanup() {
	close(rl.cleanupStop)
}

// getClientIPForRateLimit extracts the client IP address, handling proxies
func getClientIPForRateLimit(c *gin.Context) string {
	// Check X-Forwarded-For header first (for reverse proxies)
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain (original client)
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip := c.ClientIP()
	if ip == "" {
		ip = c.Request.RemoteAddr
		// Strip port if present
		if idx := strings.LastIndex(ip, ":"); idx != -1 {
			ip = ip[:idx]
		}
	}
	return ip
}

// SecurityHeaders adds comprehensive security headers to all responses
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Prevent page from being displayed in frames (clickjacking protection)
		c.Header("X-Frame-Options", "DENY")

		// Enable XSS filtering (legacy support)
		c.Header("X-XSS-Protection", "1; mode=block")

		// Strict Transport Security (force HTTPS)
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		// Content Security Policy
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdnjs.cloudflare.com https://unpkg.com; " +
			"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://cdnjs.cloudflare.com; " +
			"font-src 'self' https://fonts.gstatic.com; " +
			"img-src 'self' data: https:; " +
			"connect-src 'self' wss: ws:; " +
			"worker-src 'self' blob:; " +
			"child-src 'self';"
		c.Header("Content-Security-Policy", csp)

		// Referrer Policy
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions Policy (Feature Policy successor)
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		// Prevent caching of sensitive content
		if c.Request.URL.Path == "/api/v1/auth/login" ||
		   c.Request.URL.Path == "/api/v1/auth/register" {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
			c.Header("Pragma", "no-cache")
		}

		c.Next()
	}
}

// CSRFProtection implements CSRF protection middleware
func CSRFProtection() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip CSRF for GET requests and certain endpoints
		if c.Request.Method == "GET" ||
		   c.Request.URL.Path == "/health" ||
		   c.Request.URL.Path == "/api/v1/auth/login" {
			c.Next()
			return
		}

		// Check for CSRF token in header
		csrfToken := c.GetHeader("X-CSRF-Token")
		if csrfToken == "" {
			c.JSON(403, gin.H{
				"error": "CSRF token required",
				"code":  "CSRF_TOKEN_MISSING",
			})
			c.Abort()
			return
		}

		// Validate CSRF token (implement your validation logic)
		if !validateCSRFToken(csrfToken) {
			c.JSON(403, gin.H{
				"error": "Invalid CSRF token",
				"code":  "CSRF_TOKEN_INVALID",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// validateCSRFToken validates the CSRF token using HMAC
// Tokens are generated as: base64(timestamp:hmac(timestamp, secret))
func validateCSRFToken(token string) bool {
	if len(token) < 32 {
		return false
	}

	// Decode the token
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return false
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return false
	}

	timestamp, signature := parts[0], parts[1]

	// Check if token is expired (1 hour validity)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}

	if time.Now().Unix()-ts > 3600 {
		return false // Token expired
	}

	// Verify HMAC signature
	secret := getCSRFSecret()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSig))
}

// GenerateCSRFToken creates a new CSRF token
func GenerateCSRFToken() string {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	secret := getCSRFSecret()

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	signature := hex.EncodeToString(mac.Sum(nil))

	token := timestamp + ":" + signature
	return base64.StdEncoding.EncodeToString([]byte(token))
}

// getCSRFSecret returns the CSRF secret from environment
// SECURITY: Requires CSRF_SECRET or JWT_SECRET to be set
func getCSRFSecret() string {
	secret := os.Getenv("CSRF_SECRET")
	if secret == "" {
		secret = os.Getenv("JWT_SECRET") // Fallback to JWT secret
	}
	if secret == "" {
		// In production, this should never happen as JWT_SECRET is required
		// Log warning for development environments
		log.Println("⚠️  WARNING: CSRF_SECRET not set - CSRF protection may be weak")
		// Generate a runtime secret (will change on restart, but better than hardcoded)
		secret = "runtime-csrf-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return secret
}

// RateLimitHeaders adds rate limiting headers and enforces rate limits
// Uses a fixed window algorithm with 1000 requests per hour per IP
func RateLimitHeaders() gin.HandlerFunc {
	limiter := getHeaderRateLimiter()

	return func(c *gin.Context) {
		// Get client identifier (IP address)
		clientKey := getClientIPForRateLimit(c)

		// Check rate limit
		allowed, remaining, resetIn := limiter.Allow(clientKey)

		// Always set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.FormatInt(limiter.limit, 10))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetIn, 10))

		if !allowed {
			// Rate limit exceeded - return 429 Too Many Requests
			c.Header("Retry-After", strconv.FormatInt(resetIn, 10))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"code":        "RATE_LIMIT_EXCEEDED",
				"limit":       limiter.limit,
				"reset_in":    resetIn,
				"retry_after": resetIn,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitHeadersWithConfig creates a rate limiter with custom configuration
func RateLimitHeadersWithConfig(limit int64, windowSecs int64) gin.HandlerFunc {
	limiter := NewFixedWindowRateLimiter(limit, windowSecs)

	return func(c *gin.Context) {
		// Get client identifier (IP address)
		clientKey := getClientIPForRateLimit(c)

		// Check rate limit
		allowed, remaining, resetIn := limiter.Allow(clientKey)

		// Always set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.FormatInt(limiter.limit, 10))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetIn, 10))

		if !allowed {
			// Rate limit exceeded - return 429 Too Many Requests
			c.Header("Retry-After", strconv.FormatInt(resetIn, 10))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"code":        "RATE_LIMIT_EXCEEDED",
				"limit":       limiter.limit,
				"reset_in":    resetIn,
				"retry_after": resetIn,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Note: RequestID and generateRequestID functions are available in middleware.go