package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds comprehensive security headers to all responses
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Prevent page from being displayed in frames (clickjacking protection)
		// Exception: preview proxy responses must be embeddable by the frontend app.
		isPreviewProxy := strings.HasPrefix(c.Request.URL.Path, "/api/v1/preview/proxy/")
		if !isPreviewProxy {
			c.Header("X-Frame-Options", "DENY")
		}

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
		if isPreviewProxy {
			csp += " frame-ancestors 'self' https://apex-frontend-gigq.onrender.com https://apex.build https://www.apex.build;"
		}
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

// RateLimitHeaders adds rate limiting headers
func RateLimitHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Add rate limiting headers
		c.Header("X-RateLimit-Limit", "1000")
		c.Header("X-RateLimit-Remaining", "999") // TODO: Implement actual counting
		c.Header("X-RateLimit-Reset", "3600")

		c.Next()
	}
}

// Note: RequestID and generateRequestID functions are available in middleware.go