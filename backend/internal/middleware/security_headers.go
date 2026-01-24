package middleware

import (
	"github.com/gin-gonic/gin"
)

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

// validateCSRFToken validates the CSRF token
func validateCSRFToken(token string) bool {
	// TODO: Implement proper CSRF token validation
	// For now, accept any non-empty token
	return len(token) > 0
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