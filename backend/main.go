//go:build ignore
// +build ignore

// APEX.BUILD Production Server
// Real cloud development platform to beat Replit

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"apex-build/internal/ai"
	"apex-build/internal/auth"
	"apex-build/internal/handlers"
	"apex-build/internal/middleware"
	"apex-build/internal/websocket"
	"apex-build/pkg/models"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// Initialize database
	db, err := initDB()
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	// Initialize AI services
	aiRouter, err := initAI()
	if err != nil {
		log.Fatal("Failed to initialize AI services:", err)
	}

	// Initialize authentication service
	authService := auth.NewAuthService(
		os.Getenv("JWT_SECRET"),
	)
	authService.SetDB(db)

	// Initialize WebSocket hub
	wsHub := websocket.NewHub()
	go wsHub.Run()

	// Initialize handlers
	handler := handlers.NewHandler(db, aiRouter, authService, wsHub)

	// Setup router
	router := setupRouter(handler)

	// Start server
	srv := &http.Server{
		Addr:         ":" + getPort(),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Printf("🚀 APEX.BUILD server starting on port %s", getPort())
		log.Println("⚡ Multi-AI cloud development platform")
		log.Println("🎯 Ready to compete with Replit!")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server:", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down APEX.BUILD server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("✅ APEX.BUILD server shut down gracefully")
}

func initDB() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgresql://postgres:apex_build_2024@localhost:5432/apex_build?sslmode=disable"
	}

	// Configure GORM logger
	logLevel := logger.Silent
	if os.Getenv("ENV") == "development" {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

func runMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.CollabRoom{},
		&models.Project{},
		&models.File{},
		&models.AIRequest{},
		&models.Execution{},
		&models.CursorPosition{},
		&models.ChatMessage{},
	)
}

func initAI() (*ai.AIRouter, error) {
	// Get API keys from environment
	claudeKey := os.Getenv("ANTHROPIC_API_KEY")
	openAIKey := os.Getenv("OPENAI_API_KEY")
	if openAIKey == "" {
		openAIKey = os.Getenv("CHATGPT_API_KEY")
	}
	if openAIKey == "" {
		openAIKey = os.Getenv("GPT_API_KEY")
	}
	if openAIKey == "" {
		openAIKey = os.Getenv("OPENAI_PLATFORM_API_KEY")
	}
	if openAIKey == "" {
		openAIKey = os.Getenv("OPENAI_KEY")
	}
	if openAIKey == "" {
		openAIKey = os.Getenv("OPENAI_TOKEN")
	}
	if openAIKey == "" {
		openAIKey = os.Getenv("OPENAI_SECRET_KEY")
	}
	geminiKey := os.Getenv("GOOGLE_AI_API_KEY")
	grokKey := os.Getenv("XAI_API_KEY")
	if grokKey == "" {
		grokKey = os.Getenv("GROK_API_KEY")
	}

	// Check for placeholder values
	if claudeKey == "sk-ant-api03-your-claude-key-here" {
		claudeKey = ""
	}
	if openAIKey == "sk-your-openai-key-here" {
		openAIKey = ""
	}
	if geminiKey == "your-gemini-key-here" {
		geminiKey = ""
	}
	if grokKey == "your-grok-key-here" {
		grokKey = ""
	}

	// Log which clients will be initialized
	if claudeKey != "" {
		log.Println("✅ Claude API key configured")
	} else {
		log.Println("⚠️  Claude API key not configured")
	}

	if openAIKey != "" {
		log.Println("✅ OpenAI API key configured")
	} else {
		log.Println("⚠️  OpenAI API key not configured")
	}

	if geminiKey != "" {
		log.Println("✅ Gemini API key configured")
	} else {
		log.Println("⚠️  Gemini API key not configured")
	}

	if grokKey != "" {
		log.Println("✅ Grok (xAI) API key configured")
	} else {
		log.Println("⚠️  Grok (xAI) API key not configured")
	}

	// Ollama URL for local/remote inference (free, no API key needed)
	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = os.Getenv("OLLAMA_HOST")
	}
	if ollamaURL != "" {
		log.Printf("✅ Ollama configured at %s", ollamaURL)
	} else {
		log.Println("⚠️  Ollama not configured (set OLLAMA_URL to enable free local AI)")
	}

	if claudeKey == "" && openAIKey == "" && geminiKey == "" && grokKey == "" && ollamaURL == "" {
		log.Println("⚠️  No AI clients configured - some features will use mock responses")
	}

	// Initialize AI router with available keys (Grok passed as first extra key, Ollama URL as second)
	return ai.NewAIRouter(claudeKey, openAIKey, geminiKey, grokKey, ollamaURL), nil
}

func setupRouter(handler *handlers.Handler) *gin.Engine {
	// Set Gin mode
	if os.Getenv("ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.ErrorHandler())
	router.Use(middleware.RateLimit())

	// CORS configuration
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://localhost:3000", "http://localhost:5173", "http://localhost:3001", "http://127.0.0.1:3000", "http://127.0.0.1:3001"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"}
	config.AllowCredentials = true
	router.Use(cors.New(config))

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"service":   "apex-build",
			"version":   "1.0.0",
			"timestamp": time.Now().UTC(),
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
			auth.POST("/logout", middleware.RequireAuth(handler.AuthService), handler.Logout)
		}

		// Protected routes
		protected := v1.Group("")
		protected.Use(middleware.RequireAuth(handler.AuthService))
		{
			// User routes
			protected.GET("/user/profile", handler.GetProfile)
			protected.PUT("/user/profile", handler.UpdateProfile)

			// Project routes
			protected.GET("/projects", handler.GetProjects)
			protected.POST("/projects", handler.CreateProject)
			protected.GET("/projects/:id", handler.GetProject)
			protected.PUT("/projects/:id", handler.UpdateProject)
			protected.DELETE("/projects/:id", handler.DeleteProject)

			// File routes
			protected.GET("/projects/:id/files", handler.GetFiles)
			protected.POST("/projects/:id/files", handler.CreateFile)
			protected.GET("/files/:id", handler.GetFile)
			protected.PUT("/files/:id", handler.UpdateFile)
			protected.DELETE("/files/:id", handler.DeleteFile)

			// AI routes
			protected.POST("/ai/generate", handler.GenerateAI)
			protected.GET("/ai/usage", handler.GetAIUsage)
			protected.GET("/ai/history", handler.GetAIHistory)
			protected.POST("/ai/rate/:id", handler.RateAIResponse)

			// Code execution routes
			protected.POST("/execute", handler.ExecuteCode)
			protected.GET("/execute/:id", handler.GetExecution)
			protected.GET("/execute/history", handler.GetExecutionHistory)
			protected.POST("/execute/:id/stop", handler.StopExecution)

			// Collaboration routes
			protected.POST("/collab/join/:project_id", handler.JoinCollabRoom)
			protected.POST("/collab/leave/:room_id", handler.LeaveCollabRoom)
			protected.GET("/collab/users/:room_id", handler.GetCollabUsers)

			// System routes
			protected.GET("/system/info", handler.GetSystemInfo)
		}

		// WebSocket endpoint
		v1.GET("/ws", handler.HandleWebSocket)
	}

	// Static file serving for uploads
	router.Static("/uploads", "./uploads")

	return router
}

func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return port
}
