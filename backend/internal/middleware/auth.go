// APEX.BUILD Authentication Middleware
// Production-ready JWT authentication middleware for Gin

package middleware

import (
	"net/http"
	"strings"

	"apex-build/internal/auth"

	"github.com/gin-gonic/gin"
)

// RequireAuth middleware validates JWT tokens
func RequireAuth(authService *auth.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header is required",
				"code":  "AUTH_HEADER_MISSING",
			})
			c.Abort()
			return
		}

		// Extract Bearer token
		token, err := extractBearerToken(authHeader)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
				"code":  "INVALID_AUTH_HEADER",
			})
			c.Abort()
			return
		}

		// Validate token
		claims, err := authService.ValidateToken(token)
		if err != nil {
			var code string
			switch err {
			case auth.ErrTokenExpired:
				code = "TOKEN_EXPIRED"
			case auth.ErrInvalidToken:
				code = "INVALID_TOKEN"
			default:
				code = "TOKEN_VALIDATION_FAILED"
			}

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
				"code":  code,
			})
			c.Abort()
			return
		}

		// Store user information in context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Set("token_claims", claims)

		c.Next()
	}
}

// RequireRole middleware checks if user has required role
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User role not found in context",
				"code":  "ROLE_NOT_FOUND",
			})
			c.Abort()
			return
		}

		if userRole.(string) != role {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient permissions",
				"code":  "INSUFFICIENT_PERMISSIONS",
				"required_role": role,
				"user_role": userRole,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAnyRole middleware checks if user has any of the required roles
func RequireAnyRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User role not found in context",
				"code":  "ROLE_NOT_FOUND",
			})
			c.Abort()
			return
		}

		userRoleStr := userRole.(string)
		for _, role := range roles {
			if userRoleStr == role {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error": "Insufficient permissions",
			"code":  "INSUFFICIENT_PERMISSIONS",
			"required_roles": roles,
			"user_role": userRoleStr,
		})
		c.Abort()
	}
}

// OptionalAuth middleware validates token if present, but doesn't require it
func OptionalAuth(authService *auth.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// No token provided, continue without authentication
			c.Next()
			return
		}

		token, err := extractBearerToken(authHeader)
		if err != nil {
			// Invalid auth header format, continue without authentication
			c.Next()
			return
		}

		claims, err := authService.ValidateToken(token)
		if err != nil {
			// Invalid token, continue without authentication
			c.Next()
			return
		}

		// Valid token, store user information
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Set("token_claims", claims)
		c.Set("authenticated", true)

		c.Next()
	}
}

// extractBearerToken extracts the token from Bearer authorization header
func extractBearerToken(authHeader string) (string, error) {
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return "", gin.Error{
			Err:  nil,
			Type: gin.ErrorTypePublic,
			Meta: "Invalid authorization header format. Expected 'Bearer <token>'",
		}
	}

	token := strings.TrimPrefix(authHeader, bearerPrefix)
	if token == "" {
		return "", gin.Error{
			Err:  nil,
			Type: gin.ErrorTypePublic,
			Meta: "Token cannot be empty",
		}
	}

	return token, nil
}

// GetUserID helper function to extract user ID from context
func GetUserID(c *gin.Context) (uint, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	return userID.(uint), true
}

// GetUsername helper function to extract username from context
func GetUsername(c *gin.Context) (string, bool) {
	username, exists := c.Get("username")
	if !exists {
		return "", false
	}
	return username.(string), true
}

// GetUserEmail helper function to extract email from context
func GetUserEmail(c *gin.Context) (string, bool) {
	email, exists := c.Get("email")
	if !exists {
		return "", false
	}
	return email.(string), true
}

// GetUserRole helper function to extract role from context
func GetUserRole(c *gin.Context) (string, bool) {
	role, exists := c.Get("role")
	if !exists {
		return "", false
	}
	return role.(string), true
}

// IsAuthenticated checks if request is authenticated
func IsAuthenticated(c *gin.Context) bool {
	authenticated, exists := c.Get("authenticated")
	if !exists {
		// Check if user_id exists (from required auth)
		_, exists = c.Get("user_id")
		return exists
	}
	return authenticated.(bool)
}