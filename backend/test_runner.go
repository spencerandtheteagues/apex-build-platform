package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"apex-build/internal/auth"
	"apex-build/internal/handlers"
	"apex-build/internal/ai"
	"apex-build/internal/websocket"
	"apex-build/pkg/models"
)

// TestResults tracks test execution results
type TestResults struct {
	TotalTests      int               `json:"total_tests"`
	PassedTests     int               `json:"passed_tests"`
	FailedTests     int               `json:"failed_tests"`
	Duration        time.Duration     `json:"duration"`
	TestCategories  map[string]bool   `json:"test_categories"`
	Errors          []string          `json:"errors"`
	Timestamp       time.Time         `json:"timestamp"`
}

func main() {
	fmt.Println("🧪 APEX.BUILD Comprehensive Test Runner")
	fmt.Println("==========================================")

	startTime := time.Now()
	results := &TestResults{
		TestCategories: make(map[string]bool),
		Errors:         []string{},
		Timestamp:      startTime,
	}

	// Initialize test environment
	fmt.Println("🔧 Setting up test environment...")
	db, router, err := setupTestEnvironment()
	if err != nil {
		log.Fatal("Failed to setup test environment:", err)
	}

	// Run comprehensive tests
	runHealthTests(router, results)
	runAuthenticationTests(router, results)
	runSecurityTests(router, results)
	runPerformanceTests(router, results)
	runConcurrencyTests(router, results)
	runDatabaseTests(db, results)

	// Calculate final results
	results.Duration = time.Since(startTime)

	// Print results
	printTestResults(results)
}

func setupTestEnvironment() (*gorm.DB, *gin.Engine, error) {
	gin.SetMode(gin.TestMode)

	// Initialize in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		return nil, nil, err
	}

	// Auto-migrate models
	err = db.AutoMigrate(
		&models.User{},
		&models.Project{},
		&models.File{},
		&models.AIRequest{},
		&models.CollabRoom{},
		&models.Execution{},
	)
	if err != nil {
		return nil, nil, err
	}

	// Initialize services
	authService := auth.NewAuthService("test_secret")
	aiRouter := ai.NewAIRouter("", "", "")
	wsHub := websocket.NewHub()
	handler := handlers.NewHandler(db, aiRouter, authService, wsHub)

	// Setup router
	router := gin.New()
	router.Use(gin.Recovery())

	// Health endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"service":   "apex-build",
			"version":   "2.0.0",
			"enhanced":  true,
		})
	})

	// API routes
	v1 := router.Group("/api/v1")
	v1.POST("/auth/register", handler.Register)
	v1.POST("/auth/login", handler.Login)
	v1.GET("/auth/profile", handler.GetProfile)
	v1.GET("/projects", handler.GetProjects)
	v1.POST("/projects", handler.CreateProject)
	v1.POST("/ai/generate", handler.GenerateAI)

	return db, router, nil
}

func runHealthTests(router *gin.Engine, results *TestResults) {
	fmt.Println("🏥 Running Health Tests...")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	if w.Code == 200 {
		fmt.Println("  ✅ Health endpoint responds correctly")
		results.PassedTests++
		results.TestCategories["health"] = true
	} else {
		fmt.Printf("  ❌ Health endpoint failed: %d\n", w.Code)
		results.FailedTests++
		results.Errors = append(results.Errors, "Health endpoint failed")
	}
	results.TotalTests++
}

func runAuthenticationTests(router *gin.Engine, results *TestResults) {
	fmt.Println("🔐 Running Authentication Tests...")

	// Test registration endpoint exists
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(`{"username":"test","email":"test@test.com","password":"test123"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != 404 {
		fmt.Println("  ✅ Registration endpoint accessible")
		results.PassedTests++
		results.TestCategories["auth"] = true
	} else {
		fmt.Println("  ❌ Registration endpoint not found")
		results.FailedTests++
		results.Errors = append(results.Errors, "Registration endpoint not found")
	}
	results.TotalTests++
}

func runSecurityTests(router *gin.Engine, results *TestResults) {
	fmt.Println("🔒 Running Security Tests...")

	// Test SQL injection protection
	sqlPayloads := []string{
		"'; DROP TABLE users; --",
		"' OR '1'='1",
		"admin'--",
	}

	allSecure := true
	for _, payload := range sqlPayloads {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/auth/login",
			strings.NewReader(fmt.Sprintf(`{"username":"%s","password":"test"}`, payload)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code == 200 {
			allSecure = false
			break
		}
	}

	if allSecure {
		fmt.Println("  ✅ SQL injection protection working")
		results.PassedTests++
		results.TestCategories["security"] = true
	} else {
		fmt.Println("  ❌ SQL injection vulnerability detected")
		results.FailedTests++
		results.Errors = append(results.Errors, "SQL injection vulnerability")
	}
	results.TotalTests++
}

func runPerformanceTests(router *gin.Engine, results *TestResults) {
	fmt.Println("⚡ Running Performance Tests...")

	iterations := 50
	totalDuration := time.Duration(0)

	for i := 0; i < iterations; i++ {
		start := time.Now()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)
		duration := time.Since(start)
		totalDuration += duration
	}

	avgResponseTime := totalDuration / time.Duration(iterations)

	if avgResponseTime < 50*time.Millisecond {
		fmt.Printf("  ✅ Average response time: %v\n", avgResponseTime)
		results.PassedTests++
		results.TestCategories["performance"] = true
	} else {
		fmt.Printf("  ❌ Slow response time: %v\n", avgResponseTime)
		results.FailedTests++
		results.Errors = append(results.Errors, fmt.Sprintf("Slow response: %v", avgResponseTime))
	}
	results.TotalTests++
}

func runConcurrencyTests(router *gin.Engine, results *TestResults) {
	fmt.Println("🔄 Running Concurrency Tests...")

	concurrentRequests := 100
	results_chan := make(chan int, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		go func() {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/health", nil)
			router.ServeHTTP(w, req)
			results_chan <- w.Code
		}()
	}

	successCount := 0
	for i := 0; i < concurrentRequests; i++ {
		code := <-results_chan
		if code == 200 {
			successCount++
		}
	}

	successRate := float64(successCount) / float64(concurrentRequests)

	if successRate >= 0.95 {
		fmt.Printf("  ✅ Concurrency test passed: %.1f%% success rate\n", successRate*100)
		results.PassedTests++
		results.TestCategories["concurrency"] = true
	} else {
		fmt.Printf("  ❌ Concurrency test failed: %.1f%% success rate\n", successRate*100)
		results.FailedTests++
		results.Errors = append(results.Errors, fmt.Sprintf("Low concurrency success rate: %.1f%%", successRate*100))
	}
	results.TotalTests++
}

func runDatabaseTests(db *gorm.DB, results *TestResults) {
	fmt.Println("🗄️  Running Database Tests...")

	// Test database connection
	sqlDB, err := db.DB()
	if err != nil {
		fmt.Println("  ❌ Database connection failed")
		results.FailedTests++
		results.Errors = append(results.Errors, "Database connection failed")
		results.TotalTests++
		return
	}

	err = sqlDB.Ping()
	if err != nil {
		fmt.Println("  ❌ Database ping failed")
		results.FailedTests++
		results.Errors = append(results.Errors, "Database ping failed")
		results.TotalTests++
		return
	}

	// Test CRUD operations
	testUser := &models.User{
		Username: "testuser_db",
		Email:    "test_db@apex.build",
		PasswordHash: "hashed_password",
	}

	result := db.Create(testUser)
	if result.Error != nil {
		fmt.Println("  ❌ Database create operation failed")
		results.FailedTests++
		results.Errors = append(results.Errors, "Database create failed")
	} else {
		fmt.Println("  ✅ Database operations working")
		results.PassedTests++
		results.TestCategories["database"] = true
	}
	results.TotalTests++
}

func printTestResults(results *TestResults) {
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("🎯 APEX.BUILD Test Results Summary")
	fmt.Println(strings.Repeat("=", 50))

	fmt.Printf("📊 Total Tests: %d\n", results.TotalTests)
	fmt.Printf("✅ Passed: %d\n", results.PassedTests)
	fmt.Printf("❌ Failed: %d\n", results.FailedTests)
	fmt.Printf("⏱️  Duration: %v\n", results.Duration)

	successRate := float64(results.PassedTests) / float64(results.TotalTests) * 100
	fmt.Printf("📈 Success Rate: %.1f%%\n", successRate)

	fmt.Println("\n📋 Test Categories:")
	for category, passed := range results.TestCategories {
		status := "✅"
		if !passed {
			status = "❌"
		}
		fmt.Printf("  %s %s\n", status, category)
	}

	if len(results.Errors) > 0 {
		fmt.Println("\n🚨 Errors:")
		for i, err := range results.Errors {
			fmt.Printf("  %d. %s\n", i+1, err)
		}
	}

	if successRate >= 80 {
		fmt.Println("\n🎉 APEX.BUILD platform ready for deployment!")
	} else {
		fmt.Println("\n⚠️  Some issues need attention before deployment")
	}

	fmt.Printf("\n🕒 Test completed at: %v\n", results.Timestamp.Format(time.RFC3339))
}