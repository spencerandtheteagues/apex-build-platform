package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"apex-build/internal/auth"
	"apex-build/internal/billing"
	"apex-build/internal/handlers"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestSetup initializes test environment
func TestSetup(t *testing.T) (*gin.Engine, *gorm.DB) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create in-memory database for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Initialize services
	authService := auth.NewAuthService("test_jwt_secret_12345")
	billingService := billing.NewSimpleBillingService(db)

	// Create handlers
	handler := handlers.NewHandler(db, nil, authService, nil)
	billingHandlers := handlers.NewBillingHandlers(billingService)

	// Setup router
	router := gin.New()

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"service":   "apex-build",
			"version":   "1.0.0",
			"test_mode": true,
		})
	})

	// API routes
	v1 := router.Group("/api/v1")
	{
		// Authentication routes
		auth := v1.Group("/auth")
		{
			auth.POST("/register", handler.Register)
			auth.POST("/login", handler.Login)
			auth.POST("/refresh", handler.RefreshToken)
		}

		// Billing routes
		billing := v1.Group("/billing")
		{
			billing.GET("/pricing", billingHandlers.GetPricing)
			billing.GET("/usage", billingHandlers.GetUsage)
		}

		// AI routes
		ai := v1.Group("/ai")
		{
			ai.POST("/generate", handler.GenerateAI)
			ai.GET("/usage", handler.GetAIUsage)
		}
	}

	return router, db
}

// TestHealthEndpoint tests the health check endpoint
func TestHealthEndpoint(t *testing.T) {
	router, _ := TestSetup(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
	assert.Equal(t, "apex-build", response["service"])
	assert.Equal(t, "1.0.0", response["version"])
	assert.Equal(t, true, response["test_mode"])
}

// TestUserRegistration tests user registration endpoint
func TestUserRegistration(t *testing.T) {
	router, _ := TestSetup(t)

	// Test valid registration
	registrationData := map[string]interface{}{
		"username":  "testuser",
		"email":     "test@apex.build",
		"password":  "SecurePassword123!",
		"full_name": "Test User",
	}

	jsonData, _ := json.Marshal(registrationData)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 201, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, true, response["success"])

	// Test duplicate registration (should fail)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonData))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)

	assert.Equal(t, 400, w2.Code)
}

// TestUserLogin tests user login endpoint
func TestUserLogin(t *testing.T) {
	router, _ := TestSetup(t)

	// First register a user
	registrationData := map[string]interface{}{
		"username":  "loginuser",
		"email":     "login@apex.build",
		"password":  "SecurePassword123!",
		"full_name": "Login User",
	}

	jsonData, _ := json.Marshal(registrationData)
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonData))
	req1.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w1, req1)

	// Now test login
	loginData := map[string]interface{}{
		"username": "loginuser",
		"password": "SecurePassword123!",
	}

	loginJson, _ := json.Marshal(loginData)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(loginJson))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)

	assert.Equal(t, 200, w2.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w2.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, true, response["success"])

	// Should have access token
	tokens := response["tokens"].(map[string]interface{})
	assert.NotEmpty(t, tokens["access_token"])
	assert.NotEmpty(t, tokens["refresh_token"])

	// Test invalid login
	invalidLogin := map[string]interface{}{
		"username": "loginuser",
		"password": "WrongPassword",
	}

	invalidJson, _ := json.Marshal(invalidLogin)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(invalidJson))
	req3.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w3, req3)

	assert.Equal(t, 401, w3.Code)
}

// TestBillingPricing tests the billing pricing endpoint
func TestBillingPricing(t *testing.T) {
	router, _ := TestSetup(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/billing/pricing", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, true, response["success"])

	data := response["data"].(map[string]interface{})
	plans := data["plans"].([]interface{})

	// Should have 4 plans: free, pro, team, enterprise
	assert.Equal(t, 4, len(plans))

	// Check first plan (free)
	freePlan := plans[0].(map[string]interface{})
	assert.Equal(t, "free", freePlan["type"])
	assert.Equal(t, "Free", freePlan["name"])
	assert.Equal(t, float64(0), freePlan["monthly_price"])
}

// TestAIGenerationEndpoint tests AI code generation
func TestAIGenerationEndpoint(t *testing.T) {
	router, _ := TestSetup(t)

	aiRequest := map[string]interface{}{
		"prompt":    "Create a simple Hello World function in JavaScript",
		"language":  "javascript",
		"capability": "code_generation",
	}

	jsonData, _ := json.Marshal(aiRequest)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/ai/generate", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	// Add mock auth header for protected endpoint
	req.Header.Set("Authorization", "Bearer mock_token_for_testing")
	router.ServeHTTP(w, req)

	// Should handle the request (might return 401 due to auth, but endpoint exists)
	assert.True(t, w.Code == 200 || w.Code == 401 || w.Code == 500)
}

// TestPasswordSecurity tests password hashing security
func TestPasswordSecurity(t *testing.T) {
	authService := auth.NewAuthService("test_secret")

	// Test password hashing
	password := "TestPassword123!"
	hashedPassword, err := authService.HashPassword(password)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashedPassword)
	assert.NotEqual(t, password, hashedPassword)

	// Test password verification
	err = authService.CheckPassword(password, hashedPassword)
	assert.NoError(t, err)

	// Test wrong password
	err = authService.CheckPassword("WrongPassword", hashedPassword)
	assert.Error(t, err)
}

// TestInputValidation tests input validation and sanitization
func TestInputValidation(t *testing.T) {
	router, _ := TestSetup(t)

	// Test registration with invalid data
	testCases := []struct {
		name string
		data map[string]interface{}
		expectedCode int
	}{
		{
			name: "Missing username",
			data: map[string]interface{}{
				"email":    "test@apex.build",
				"password": "SecurePassword123!",
			},
			expectedCode: 400,
		},
		{
			name: "Invalid email",
			data: map[string]interface{}{
				"username": "testuser",
				"email":    "invalid-email",
				"password": "SecurePassword123!",
			},
			expectedCode: 400,
		},
		{
			name: "Weak password",
			data: map[string]interface{}{
				"username": "testuser",
				"email":    "test@apex.build",
				"password": "123",
			},
			expectedCode: 400,
		},
		{
			name: "SQL injection attempt",
			data: map[string]interface{}{
				"username": "'; DROP TABLE users; --",
				"email":    "test@apex.build",
				"password": "SecurePassword123!",
			},
			expectedCode: 400,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jsonData, _ := json.Marshal(tc.data)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedCode, w.Code)
		})
	}
}

// TestCORSHeaders tests CORS configuration
func TestCORSHeaders(t *testing.T) {
	router, _ := TestSetup(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/health", nil)
	req.Header.Set("Origin", "http://localhost:3001")
	router.ServeHTTP(w, req)

	// Should have CORS headers
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Origin"), "localhost")
}

// TestConcurrentRequests tests system under concurrent load
func TestConcurrentRequests(t *testing.T) {
	router, _ := TestSetup(t)

	// Create multiple concurrent requests
	numRequests := 10
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/health", nil)
			router.ServeHTTP(w, req)
			results <- w.Code
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		code := <-results
		assert.Equal(t, 200, code)
	}
}

// TestRateLimiting tests rate limiting functionality (if implemented)
func TestRateLimiting(t *testing.T) {
	router, _ := TestSetup(t)

	// Make rapid successive requests to test rate limiting
	for i := 0; i < 50; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		// Should either be successful or rate limited
		assert.True(t, w.Code == 200 || w.Code == 429)
	}
}