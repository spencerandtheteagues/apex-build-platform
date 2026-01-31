package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"apex-build/internal/auth"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAuthTestRouter(authService *auth.AuthService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	return router
}

func generateTestToken(authService *auth.AuthService, user *models.User) string {
	tokens, _ := authService.GenerateTokens(user)
	return tokens.AccessToken
}

func TestRequireAuth(t *testing.T) {
	authService := auth.NewAuthService("test-secret-key-for-auth-middleware")

	testUser := &models.User{
		ID:               1,
		Username:         "testuser",
		Email:            "test@example.com",
		SubscriptionType: "pro",
	}

	validToken := generateTestToken(authService, testUser)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectedCode   string
		checkContext   bool
	}{
		{
			name:           "valid token",
			authHeader:     "Bearer " + validToken,
			expectedStatus: http.StatusOK,
			checkContext:   true,
		},
		{
			name:           "missing auth header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "AUTH_HEADER_MISSING",
		},
		{
			name:           "invalid auth header format - no bearer",
			authHeader:     validToken,
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "INVALID_AUTH_HEADER",
		},
		{
			name:           "invalid auth header format - wrong prefix",
			authHeader:     "Token " + validToken,
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "INVALID_AUTH_HEADER",
		},
		{
			name:           "empty token after bearer",
			authHeader:     "Bearer ",
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "INVALID_AUTH_HEADER",
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalid.token.here",
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "INVALID_TOKEN",
		},
		{
			name:           "malformed token",
			authHeader:     "Bearer not-even-a-jwt",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupAuthTestRouter(authService)
			router.Use(RequireAuth(authService))
			router.GET("/protected", func(c *gin.Context) {
				userID, _ := GetUserID(c)
				username, _ := GetUsername(c)
				email, _ := GetUserEmail(c)
				c.JSON(http.StatusOK, gin.H{
					"user_id":  userID,
					"username": username,
					"email":    email,
				})
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/protected", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				assert.Contains(t, w.Body.String(), tt.expectedCode)
			}

			if tt.checkContext && w.Code == http.StatusOK {
				assert.Contains(t, w.Body.String(), `"user_id":1`)
				assert.Contains(t, w.Body.String(), `"username":"testuser"`)
				assert.Contains(t, w.Body.String(), `"email":"test@example.com"`)
			}
		})
	}
}

func TestRequireRole(t *testing.T) {
	authService := auth.NewAuthService("test-secret-key")

	tests := []struct {
		name           string
		userRole       string
		requiredRole   string
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "matching role",
			userRole:       "admin",
			requiredRole:   "admin",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "non-matching role",
			userRole:       "user",
			requiredRole:   "admin",
			expectedStatus: http.StatusForbidden,
			expectedCode:   "INSUFFICIENT_PERMISSIONS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupAuthTestRouter(authService)

			// Middleware to set role in context (simulating RequireAuth)
			router.Use(func(c *gin.Context) {
				c.Set("role", tt.userRole)
				c.Next()
			})
			router.Use(RequireRole(tt.requiredRole))
			router.GET("/admin", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/admin", nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				assert.Contains(t, w.Body.String(), tt.expectedCode)
			}
		})
	}
}

func TestRequireAnyRole(t *testing.T) {
	authService := auth.NewAuthService("test-secret-key")

	tests := []struct {
		name           string
		userRole       string
		requiredRoles  []string
		expectedStatus int
	}{
		{
			name:           "has first required role",
			userRole:       "admin",
			requiredRoles:  []string{"admin", "moderator"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "has second required role",
			userRole:       "moderator",
			requiredRoles:  []string{"admin", "moderator"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "has none of the required roles",
			userRole:       "user",
			requiredRoles:  []string{"admin", "moderator"},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "single role match",
			userRole:       "admin",
			requiredRoles:  []string{"admin"},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupAuthTestRouter(authService)

			router.Use(func(c *gin.Context) {
				c.Set("role", tt.userRole)
				c.Next()
			})
			router.Use(RequireAnyRole(tt.requiredRoles...))
			router.GET("/endpoint", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/endpoint", nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestOptionalAuth(t *testing.T) {
	authService := auth.NewAuthService("test-secret-key")

	testUser := &models.User{
		ID:               1,
		Username:         "testuser",
		Email:            "test@example.com",
		SubscriptionType: "free",
	}
	validToken := generateTestToken(authService, testUser)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectUserID   bool
	}{
		{
			name:           "valid token - user authenticated",
			authHeader:     "Bearer " + validToken,
			expectedStatus: http.StatusOK,
			expectUserID:   true,
		},
		{
			name:           "no token - still proceeds",
			authHeader:     "",
			expectedStatus: http.StatusOK,
			expectUserID:   false,
		},
		{
			name:           "invalid token - still proceeds",
			authHeader:     "Bearer invalid-token",
			expectedStatus: http.StatusOK,
			expectUserID:   false,
		},
		{
			name:           "invalid format - still proceeds",
			authHeader:     "Token " + validToken,
			expectedStatus: http.StatusOK,
			expectUserID:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupAuthTestRouter(authService)
			router.Use(OptionalAuth(authService))
			router.GET("/public", func(c *gin.Context) {
				userID, exists := GetUserID(c)
				c.JSON(http.StatusOK, gin.H{
					"authenticated": exists,
					"user_id":       userID,
				})
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/public", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectUserID {
				assert.Contains(t, w.Body.String(), `"authenticated":true`)
				assert.Contains(t, w.Body.String(), `"user_id":1`)
			} else {
				assert.Contains(t, w.Body.String(), `"authenticated":false`)
			}
		})
	}
}

func TestGetUserID(t *testing.T) {
	t.Run("user ID exists in context", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/test", func(c *gin.Context) {
			c.Set("user_id", uint(42))
			userID, exists := GetUserID(c)
			assert.True(t, exists)
			assert.Equal(t, uint(42), userID)
			c.JSON(http.StatusOK, gin.H{"user_id": userID})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("user ID does not exist in context", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/test", func(c *gin.Context) {
			userID, exists := GetUserID(c)
			assert.False(t, exists)
			assert.Equal(t, uint(0), userID)
			c.JSON(http.StatusOK, gin.H{"exists": exists})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGetUsername(t *testing.T) {
	t.Run("username exists in context", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/test", func(c *gin.Context) {
			c.Set("username", "testuser")
			username, exists := GetUsername(c)
			assert.True(t, exists)
			assert.Equal(t, "testuser", username)
			c.JSON(http.StatusOK, gin.H{"username": username})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("username does not exist in context", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/test", func(c *gin.Context) {
			username, exists := GetUsername(c)
			assert.False(t, exists)
			assert.Empty(t, username)
			c.JSON(http.StatusOK, gin.H{"exists": exists})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGetUserEmail(t *testing.T) {
	t.Run("email exists in context", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/test", func(c *gin.Context) {
			c.Set("email", "test@example.com")
			email, exists := GetUserEmail(c)
			assert.True(t, exists)
			assert.Equal(t, "test@example.com", email)
			c.JSON(http.StatusOK, gin.H{"email": email})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGetUserRole(t *testing.T) {
	t.Run("role exists in context", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/test", func(c *gin.Context) {
			c.Set("role", "admin")
			role, exists := GetUserRole(c)
			assert.True(t, exists)
			assert.Equal(t, "admin", role)
			c.JSON(http.StatusOK, gin.H{"role": role})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestIsAuthenticated(t *testing.T) {
	tests := []struct {
		name        string
		setupCtx    func(*gin.Context)
		expectedVal bool
	}{
		{
			name: "authenticated flag set to true",
			setupCtx: func(c *gin.Context) {
				c.Set("authenticated", true)
			},
			expectedVal: true,
		},
		{
			name: "authenticated flag set to false",
			setupCtx: func(c *gin.Context) {
				c.Set("authenticated", false)
			},
			expectedVal: false,
		},
		{
			name: "user_id exists (from RequireAuth)",
			setupCtx: func(c *gin.Context) {
				c.Set("user_id", uint(1))
			},
			expectedVal: true,
		},
		{
			name:        "nothing set",
			setupCtx:    func(c *gin.Context) {},
			expectedVal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.GET("/test", func(c *gin.Context) {
				tt.setupCtx(c)
				isAuth := IsAuthenticated(c)
				assert.Equal(t, tt.expectedVal, isAuth)
				c.JSON(http.StatusOK, gin.H{"authenticated": isAuth})
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name        string
		authHeader  string
		expectToken string
		expectError bool
	}{
		{
			name:        "valid bearer token",
			authHeader:  "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expectToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expectError: false,
		},
		{
			name:        "no bearer prefix",
			authHeader:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expectToken: "",
			expectError: true,
		},
		{
			name:        "wrong prefix",
			authHeader:  "Token eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expectToken: "",
			expectError: true,
		},
		{
			name:        "empty token after bearer",
			authHeader:  "Bearer ",
			expectToken: "",
			expectError: true,
		},
		{
			name:        "bearer only",
			authHeader:  "Bearer",
			expectToken: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := extractBearerToken(tt.authHeader)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectToken, token)
			}
		})
	}
}

func TestRequireRoleNoRoleInContext(t *testing.T) {
	authService := auth.NewAuthService("test-secret-key")

	router := setupAuthTestRouter(authService)
	// Don't set role in context
	router.Use(RequireRole("admin"))
	router.GET("/admin", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "ROLE_NOT_FOUND")
}

func TestExpiredToken(t *testing.T) {
	// This test would require creating an expired token
	// For now, we test with an invalid token which triggers the invalid path
	authService := auth.NewAuthService("test-secret-key")

	router := setupAuthTestRouter(authService)
	router.Use(RequireAuth(authService))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Using a clearly expired or invalid token
	expiredToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjEwMDAwMDAwMDB9.invalid"

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+expiredToken)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// Integration test with full auth flow
func TestFullAuthFlow(t *testing.T) {
	authService := auth.NewAuthService("integration-test-secret")

	// Create a user
	user := &models.User{
		ID:               100,
		Username:         "integrationuser",
		Email:            "integration@test.com",
		SubscriptionType: "pro",
	}

	// Generate tokens
	tokens, err := authService.GenerateTokens(user)
	require.NoError(t, err)

	// Set up router with full middleware chain
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(RequireAuth(authService))
	router.Use(RequireRole("pro"))

	router.GET("/pro-feature", func(c *gin.Context) {
		userID, _ := GetUserID(c)
		username, _ := GetUsername(c)
		role, _ := GetUserRole(c)

		c.JSON(http.StatusOK, gin.H{
			"user_id":  userID,
			"username": username,
			"role":     role,
			"feature":  "pro-only",
		})
	})

	t.Run("full flow with valid token and role", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/pro-feature", nil)
		req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"user_id":100`)
		assert.Contains(t, w.Body.String(), `"username":"integrationuser"`)
		assert.Contains(t, w.Body.String(), `"role":"pro"`)
	})
}

// Benchmark tests
func BenchmarkRequireAuth(b *testing.B) {
	authService := auth.NewAuthService("benchmark-secret")
	user := &models.User{
		ID:               1,
		Username:         "benchuser",
		Email:            "bench@test.com",
		SubscriptionType: "pro",
	}
	tokens, _ := authService.GenerateTokens(user)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequireAuth(authService))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkOptionalAuth(b *testing.B) {
	authService := auth.NewAuthService("benchmark-secret")
	user := &models.User{
		ID:               1,
		Username:         "benchuser",
		Email:            "bench@test.com",
		SubscriptionType: "pro",
	}
	tokens, _ := authService.GenerateTokens(user)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(OptionalAuth(authService))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
		router.ServeHTTP(w, req)
	}
}
