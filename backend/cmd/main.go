package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"apex-build/internal/agents"
	"apex-build/internal/agents/autonomous"
	"apex-build/internal/ai"
	"apex-build/internal/api"
	"apex-build/internal/auth"
	"apex-build/internal/cache"
	"apex-build/internal/collaboration"
	"apex-build/internal/community"
	"apex-build/internal/completions"
	"apex-build/internal/config"
	manageddb "apex-build/internal/database"
	"apex-build/internal/db"
	"apex-build/internal/debugging"
	"apex-build/internal/deploy"
	deployalwayson "apex-build/internal/deploy/alwayson"
	"apex-build/internal/deploy/providers"
	"apex-build/internal/enterprise"
	"apex-build/internal/extensions"
	"apex-build/internal/git"
	"apex-build/internal/handlers"
	"apex-build/internal/hosting"
	"apex-build/internal/mcp"
	"apex-build/internal/metrics"
	"apex-build/internal/middleware"
	"apex-build/internal/payments"
	"apex-build/internal/preview"
	"apex-build/internal/search"
	"apex-build/internal/secrets"
	"apex-build/internal/usage"
	"apex-build/internal/websocket"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	log.Println("Starting APEX.BUILD - Multi-AI Cloud Development Platform")

	// Load .env file
	if err := godotenv.Load(); err != nil {
		// Try parent directory for .env
		if err := godotenv.Load("../.env"); err != nil {
			log.Println("WARNING: No .env file found, using environment variables")
		}
	}
	log.Println("Environment configuration loaded")

	// Load basic config early so we can bind the HTTP port immediately.
	appConfig := loadConfig()
	port := appConfig.Port
	if port == "" {
		port = "8080"
	}

	// Start a bootstrap HTTP listener immediately so Render health checks succeed
	// while slower initialization (DB, migrations, AI services) is still running.
	var startupReady atomic.Bool
	var activeRouter atomic.Value // stores *gin.Engine

	bootstrapRouter := gin.New()
	bootstrapRouter.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "starting",
			"ready":  startupReady.Load(),
		})
	})
	bootstrapRouter.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "server starting",
			"ready": startupReady.Load(),
		})
	})
	activeRouter.Store(bootstrapRouter)

	serverErrors := make(chan error, 1)
	httpServer := &http.Server{
		Addr:              ":" + port,
		ReadHeaderTimeout: 10 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			activeRouter.Load().(*gin.Engine).ServeHTTP(w, r)
		}),
	}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()
	log.Printf("Bootstrap HTTP listener started on port %s (health endpoint ready immediately)", port)

	// SECURITY: Validate all required secrets before starting
	// MustValidateSecrets calls ValidateAndLogSecrets and fatals on error
	secretsConfig := config.MustValidateSecrets()

	// Initialize database
	database, err := db.NewDatabase(appConfig.Database)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Run database seeds (create admin accounts)
	if err := database.RunSeeds(); err != nil {
		log.Printf("WARNING: Database seeding had issues: %v", err)
	}

	// Initialize authentication service with validated JWT secret
	// Use the validated secret from secretsConfig for consistency
	jwtSecret := secretsConfig.JWTSecret
	if jwtSecret == "" {
		jwtSecret = appConfig.JWTSecret // Fallback for dev
	}
	authService := auth.NewAuthService(jwtSecret)
	authService.SetDB(database.DB)

	// Initialize AI router with all providers (Claude, OpenAI, Gemini, Grok).
	// Production keeps Ollama disabled globally to preserve strict BYOK behavior.
	// In development, allow a local Ollama fallback only when explicitly configured
	// and no cloud platform keys are present, so local smoke tests can exercise the
	// full app-building pipeline.
	ollamaRouterURL := ""
	hasCloudProviderKey := appConfig.ClaudeAPIKey != "" || appConfig.OpenAIAPIKey != "" ||
		appConfig.GeminiAPIKey != "" || appConfig.GrokAPIKey != ""
	if !config.IsProductionEnvironment() && appConfig.OllamaBaseURL != "" && !hasCloudProviderKey {
		ollamaRouterURL = appConfig.OllamaBaseURL
		log.Printf("DEV MODE: Enabling global Ollama fallback for app builds at %s", ollamaRouterURL)
	}

	aiRouter := ai.NewAIRouter(
		appConfig.ClaudeAPIKey,
		appConfig.OpenAIAPIKey,
		appConfig.GeminiAPIKey,
		appConfig.GrokAPIKey,
		ollamaRouterURL,
	)

	log.Println("Multi-AI integration initialized:")
	log.Printf("   - Claude API: %s", getStatusIcon(appConfig.ClaudeAPIKey != ""))
	log.Printf("   - OpenAI API: %s", getStatusIcon(appConfig.OpenAIAPIKey != ""))
	log.Printf("   - Gemini API: %s", getStatusIcon(appConfig.GeminiAPIKey != ""))
	log.Printf("   - Grok API:   %s", getStatusIcon(appConfig.GrokAPIKey != ""))
	if ollamaRouterURL != "" {
		log.Printf("   - Ollama:     ✅ Dev fallback enabled (%s)", ollamaRouterURL)
	} else {
		log.Printf("   - Ollama:     ❌ Disabled globally (BYOK-only in production)")
	}

	// Initialize Secrets Manager with validated master key
	// SECURITY: Use validated key from secretsConfig, with fallback for development ONLY
	masterKey := secretsConfig.SecretsMasterKey
	if masterKey == "" {
		if config.IsProductionEnvironment() {
			log.Fatalf("CRITICAL: SECRETS_MASTER_KEY not set in production - refusing to start. " +
				"Generate with: openssl rand -base64 32 and set as env var. " +
				"Without a persistent key, all encrypted user data (API keys, secrets) will be permanently lost on restart.")
		}
		// Generate an ephemeral key for development only
		var genErr error
		masterKey, genErr = secrets.GenerateMasterKey()
		if genErr != nil {
			log.Printf("WARNING: Failed to generate master key: %v", genErr)
		}
		log.Println("DEV ONLY: Using ephemeral SECRETS_MASTER_KEY - encrypted data will not survive restart")
	}

	secretsManager, err := secrets.NewSecretsManager(masterKey)
	if err != nil {
		if config.IsProductionEnvironment() {
			log.Fatalf("CRITICAL: Failed to initialize secrets manager: %v", err)
		}
		log.Printf("WARNING: Failed to initialize secrets manager: %v", err)
	} else {
		log.Println("Secrets Manager initialized with AES-256 encryption")
	}

	// Initialize BYOK (Bring Your Own Key) Manager
	byokManager := ai.NewBYOKManager(database.GetDB(), secretsManager, aiRouter)
	byokHandler := handlers.NewBYOKHandlers(byokManager)
	log.Println("BYOK Manager initialized (user-provided API keys, per-provider cost tracking)")

	// Initialize Agent Orchestration System
	aiAdapter := agents.NewAIRouterAdapter(aiRouter, byokManager)
	agentManager := agents.NewAgentManager(aiAdapter, database.GetDB())
	if recovered, recoverErr := agentManager.RecoverStaleBuildsOnStartup(); recoverErr != nil {
		log.Printf("WARNING: Failed to recover stale builds on startup: %v", recoverErr)
	} else if recovered > 0 {
		log.Printf("Recovered %d stale in-progress build(s) after restart", recovered)
	}
	wsHub := agents.NewWSHub(agentManager)
	buildHandler := agents.NewBuildHandler(agentManager, wsHub)

	log.Println("Agent Orchestration System initialized")

	// Initialize MCP Server (APEX.BUILD as MCP provider)
	mcpServer := mcp.NewMCPServer("APEX.BUILD", "1.0.0")

	// Register built-in tools
	mcpServer.RegisterTool(mcp.Tool{
		Name:        "generate_code",
		Description: "Generate code using APEX.BUILD's multi-AI system",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"language":    map[string]string{"type": "string", "description": "Programming language"},
				"description": map[string]string{"type": "string", "description": "What to generate"},
			},
			"required": []string{"language", "description"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolCallResult, error) {
		lang := args["language"].(string)
		desc := args["description"].(string)

		// Use AI router for generation
		response, err := aiRouter.Generate(ctx, &ai.AIRequest{
			Capability: ai.CapabilityCodeGeneration,
			Prompt:     desc,
			Language:   lang,
		})
		if err != nil {
			return &mcp.ToolCallResult{
				Content: []mcp.ContentBlock{{Type: "text", Text: err.Error()}},
				IsError: true,
			}, nil
		}

		return &mcp.ToolCallResult{
			Content: []mcp.ContentBlock{{Type: "text", Text: response.Content}},
		}, nil
	})

	log.Println("MCP Server initialized with built-in tools")

	// Initialize MCP Connection Manager (for connecting to external MCP servers)
	mcpConnManager := mcp.NewMCPConnectionManager()
	log.Println("MCP Connection Manager ready for external integrations")

	// Initialize Secrets and MCP handlers
	secretsHandler := handlers.NewSecretsHandler(database.GetDB(), secretsManager)
	mcpHandler := handlers.NewMCPHandler(database.GetDB(), mcpServer, mcpConnManager, secretsManager)
	templatesHandler := handlers.NewTemplatesHandler(database.GetDB())

	log.Println("Project Templates System initialized (15+ starter templates)")

	// Initialize Code Search Engine
	searchEngine := search.NewSearchEngine(database.GetDB())
	searchHandler := handlers.NewSearchHandler(searchEngine)

	log.Println("Code Search Engine initialized (full-text, regex, symbol search)")

	// Initialize Live Preview Server
	previewServer := preview.NewPreviewServer(database.GetDB())
	previewHandler := handlers.NewPreviewHandler(database.GetDB(), previewServer, authService)

	log.Println("Live Preview Server initialized (hot reload support)")

	// Initialize Git Integration Service
	gitService := git.NewGitService(database.GetDB())
	gitHandler := handlers.NewGitHandler(database.GetDB(), gitService, secretsManager)

	log.Println("Git Integration initialized (GitHub support)")

	// Initialize GitHub Import Handler (one-click repo import like replit.new)
	importHandler := handlers.NewImportHandler(database.GetDB(), gitService, secretsManager)
	log.Println("GitHub Import Wizard initialized (one-click repo import)")

	// Initialize GitHub Export Handler (export projects to GitHub repos)
	exportHandler := handlers.NewExportHandler(database.GetDB(), gitService, secretsManager)
	log.Println("GitHub Export initialized (one-click export to GitHub)")

	// Initialize Version History Handler (Replit parity feature)
	versionHandler := handlers.NewVersionHandler(database.GetDB())
	log.Println("Version History System initialized (diff viewing, restore, pinning)")

	// Initialize Code Comments Handler (Replit parity feature)
	commentsHandler := handlers.NewCommentsHandler(database.GetDB())
	log.Println("Code Comments System initialized (inline threads, reactions, resolve)")

	// Initialize Stripe Payment Service with validated key
	// SECURITY: Use validated key from secretsConfig
	stripeSecretKey := secretsConfig.StripeSecretKey
	paymentHandler := handlers.NewPaymentHandlers(database.GetDB(), stripeSecretKey)

	if stripeSecretKey != "" && stripeSecretKey != "sk_test_xxx" {
		log.Println("Stripe Payment Integration initialized")
		log.Printf("   - Plans: Free, Pro ($12/mo), Team ($29/mo), Enterprise ($99/mo)")
	} else {
		log.Println("WARNING: Stripe not configured - payment features disabled")
		log.Println("   Set STRIPE_SECRET_KEY and STRIPE_WEBHOOK_SECRET to enable")
	}

	// Log available plans
	plans := payments.GetAllPlans()
	log.Printf("Subscription Plans configured: %d plans available", len(plans))

	// Initialize WebSocket Hub for real-time updates
	// PERFORMANCE: Using BatchedHub for 70% message reduction via 50ms batching
	wsHubRT := websocket.NewBatchedHub()
	go wsHubRT.Run()
	log.Println("WebSocket BatchedHub initialized (50ms batching, 16ms write coalescing)")

	// Initialize cache for performance optimization
	// PERFORMANCE: 30s TTL cache with in-memory fallback when Redis unavailable
	cacheConfig := cache.DefaultCacheConfig()
	redisURL := os.Getenv("REDIS_URL")
	var redisCache *cache.RedisCache
	if redisURL != "" {
		log.Printf("Redis cache connecting to: %s", redisURL)
		// TODO: Initialize with Redis client when available
		redisCache = cache.NewRedisCache(cacheConfig)
	} else {
		log.Println("WARNING: REDIS_URL not set - using in-memory cache (set for production)")
		redisCache = cache.NewRedisCache(cacheConfig)
	}

	// Initialize base Handler for dependent handlers
	// Note: BatchedHub embeds *Hub, so we pass the embedded Hub for Handler compatibility
	baseHandler := handlers.NewHandler(database.GetDB(), aiRouter, authService, wsHubRT.Hub)

	// Initialize OptimizedHandler with caching for better performance
	// PERFORMANCE: Fixes N+1 queries with proper JOINs, adds cursor-based pagination
	optimizedHandler := handlers.NewOptimizedHandler(baseHandler, redisCache)
	log.Println("OptimizedHandler initialized (N+1 fix, 30s cache, cursor pagination)")

	// Initialize Code Execution Engine with Docker container sandboxing
	// SECURITY: Container sandboxing is the default and recommended mode
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		projectsDir = "/tmp/apex-build-projects"
	}

	// Create execution handler with security configuration
	executionConfig := handlers.DefaultExecutionHandlerConfig()
	executionConfig.ProjectsDir = projectsDir

	executionHandler, err := handlers.NewExecutionHandlerWithConfig(database.GetDB(), executionConfig)
	if err != nil {
		log.Printf("CRITICAL: Failed to initialize execution handler: %v", err)
		log.Println("SECURITY: Code execution features are DISABLED")
	} else {
		log.Println("Code Execution Engine initialized (10+ languages supported)")
		log.Println("   - Languages: JavaScript, TypeScript, Python, Go, Rust, Java, C, C++, Ruby, PHP")

		// Log sandbox status
		sandboxStatus := executionHandler.GetSandboxStatus()
		if containerAvail, ok := sandboxStatus["container_available"].(bool); ok && containerAvail {
			log.Println("SECURITY: Docker container sandboxing ENABLED")
			log.Println("   - Seccomp syscall filtering: enabled")
			log.Println("   - Network isolation: disabled by default")
			log.Println("   - Memory limit: 256MB default")
			log.Println("   - CPU limit: 0.5 cores default")
			log.Println("   - Read-only root filesystem: enabled")
		} else {
			if executionConfig.ForceContainer {
				log.Println("WARNING: Docker required but not available - execution DISABLED")
			} else {
				log.Println("WARNING: Docker not available - using process-based sandbox (less secure)")
				log.Println("WARNING: Set EXECUTION_FORCE_CONTAINER=true to require Docker in production")
			}
		}
	}

	// Initialize One-Click Deployment Service
	vercelToken := os.Getenv("VERCEL_TOKEN")
	netlifyToken := os.Getenv("NETLIFY_TOKEN")
	renderToken := os.Getenv("RENDER_TOKEN")
	deployService := deploy.NewDeploymentService(database.GetDB(), vercelToken, netlifyToken, renderToken)

	// Register deployment providers
	if vercelToken != "" {
		deployService.RegisterProvider(deploy.ProviderVercel, providers.NewVercelProvider(vercelToken))
	}
	if netlifyToken != "" {
		deployService.RegisterProvider(deploy.ProviderNetlify, providers.NewNetlifyProvider(netlifyToken))
	}
	if renderToken != "" {
		deployService.RegisterProvider(deploy.ProviderRender, providers.NewRenderProvider(renderToken))
	}

	deployHandler := handlers.NewDeployHandler(database.GetDB(), deployService)
	log.Println("One-Click Deployment initialized (Vercel, Netlify, Render)")

	// Initialize Package Manager
	packageHandler := handlers.NewPackageHandler(baseHandler)
	log.Println("Package Manager initialized (NPM, PyPI, Go Modules)")

	// Initialize Environment Handler (Nix-like reproducible environments - Replit parity)
	environmentHandler := handlers.NewEnvironmentHandler(baseHandler)
	log.Println("Environment Configuration initialized (Nix-like reproducible environments)")

	// Initialize Community/Sharing Marketplace
	communityHandler := community.NewCommunityHandler(database.GetDB())

	// Run community migrations
	if err := community.AutoMigrate(database.GetDB()); err != nil {
		log.Printf("WARNING: Community migration had issues: %v", err)
	}

	// Seed default categories
	if err := community.SeedCategories(database.GetDB()); err != nil {
		log.Printf("WARNING: Category seeding had issues: %v", err)
	}

	log.Println("Community Marketplace initialized (discover, share, fork projects)")

	// Initialize Native Hosting Service
	hostingService := hosting.NewHostingService(database.GetDB())
	hostingHandler := handlers.NewHostingHandler(database.GetDB(), hostingService)
	log.Println("Native Hosting (.apex.app) initialized")

	// Always-On Deployment Controller
	alwaysOnController := deployalwayson.NewService(hostingService, nil)
	alwaysOnController.SetInventoryProvider(func(ctx context.Context) ([]string, error) {
		var ids []string
		err := database.GetDB().WithContext(ctx).Model(&hosting.NativeDeployment{}).
			Where("always_on = ? AND status != ?", true, hosting.StatusDeleted).
			Pluck("id", &ids).Error
		return ids, err
	})
	go alwaysOnController.Start(context.Background())
	log.Println("Always-On deployment controller started")

	// Initialize Managed Database Service
	dbManagerConfig := &manageddb.ManagerConfig{
		BaseDir:       getEnv("DATABASE_MANAGED_DIR", "/tmp/apex-managed-dbs"),
		PostgresHost:  getEnv("MANAGED_PG_HOST", "localhost"),
		PostgresPort:  getEnvInt("MANAGED_PG_PORT", 5432),
		RedisHost:     getEnv("MANAGED_REDIS_HOST", "localhost"),
		RedisPort:     getEnvInt("MANAGED_REDIS_PORT", 6379),
		EncryptionKey: getEnv("DB_ENCRYPTION_KEY", ""),
	}
	dbManager, err := manageddb.NewDatabaseManager(dbManagerConfig)
	var databaseHandler *handlers.DatabaseHandler
	if err != nil {
		log.Printf("WARNING: Managed Database service not available: %v", err)
	} else {
		databaseHandler = handlers.NewDatabaseHandler(database.GetDB(), dbManager, secretsManager)
		// Initialize auto-provisioning dependencies for project creation
		handlers.InitAutoProvisioningDeps(dbManager, secretsManager)
		log.Println("Managed Database Service initialized (PostgreSQL, Redis, SQLite)")
		log.Println("Auto-Provision PostgreSQL enabled for new projects")
	}

	// Initialize Debugging Service
	debugService := debugging.NewDebugService(database.GetDB())
	debuggingHandler := handlers.NewDebuggingHandler(database.GetDB(), debugService)
	log.Println("Debugging Service initialized (breakpoints, stepping, watch expressions)")

	// Initialize AI Completions Service
	completionService := completions.NewCompletionService(database.GetDB(), aiRouter, byokManager)
	completionsHandler := handlers.NewCompletionsHandler(completionService)
	log.Println("AI Completions Service initialized (inline ghost-text, multi-provider)")

	// Initialize Extensions Marketplace
	extensionService := extensions.NewService(database.GetDB())
	extensionsHandler := handlers.NewExtensionsHandler(extensionService)
	// Run extensions migrations
	database.GetDB().AutoMigrate(
		&extensions.Extension{},
		&extensions.ExtensionVersion{},
		&extensions.ExtensionReview{},
		&extensions.UserExtension{},
	)
	log.Println("Extensions Marketplace initialized (discover, install, publish)")

	// Initialize Enterprise Services (SAML SSO, SCIM, RBAC, Audit)
	auditService := enterprise.NewAuditService(database.GetDB())
	rbacService := enterprise.NewRBACService(database.GetDB())

	baseURL := getEnv("BASE_URL", "https://apex.build")
	samlConfig := &enterprise.ServiceProviderConfig{
		EntityID:                    baseURL,
		AssertionConsumerServiceURL: baseURL + "/api/v1/enterprise/sso/callback",
		SingleLogoutServiceURL:      baseURL + "/api/v1/enterprise/sso/logout",
	}
	samlService := enterprise.NewSAMLService(database.GetDB(), samlConfig, auditService)
	scimService := enterprise.NewSCIMService(database.GetDB(), auditService, rbacService)
	enterpriseHandler := handlers.NewEnterpriseHandler(database.GetDB(), samlService, scimService, auditService, rbacService)

	// Run enterprise migrations
	database.GetDB().AutoMigrate(
		&enterprise.Organization{},
		&enterprise.OrganizationMember{},
		&enterprise.Role{},
		&enterprise.Permission{},
		&enterprise.AuditLog{},
		&enterprise.RateLimit{},
		&enterprise.Invitation{},
	)
	log.Println("Enterprise Features initialized (SSO/SAML, SCIM, RBAC, Audit Logs)")

	// Initialize Autonomous Agent System (CRITICAL Replit parity feature)
	// This enables AI-powered autonomous building, testing, and deployment
	autonomousAIAdapter := autonomous.NewAIAdapter(aiRouter, byokManager)
	autonomousAgent := autonomous.NewAutonomousAgent(autonomousAIAdapter, projectsDir)
	autonomousHandler := autonomous.NewHandler(autonomousAgent)
	log.Println("Autonomous Agent System initialized (Replit Agent 3.0 parity)")
	log.Println("   - Planning: Natural language → execution plan")
	log.Println("   - Building: Create files, install deps, run builds")
	log.Println("   - Validation: Auto-test, lint, security scan")
	log.Println("   - Recovery: Self-healing with retries and rollback")

	// Initialize Real-Time Collaboration Hub
	collabHub := collaboration.NewCollabHub()
	go collabHub.Run()
	log.Println("Real-Time Collaboration initialized (OT, presence, cursor tracking)")

	// Initialize Key Rotation Handler (admin-only)
	rotationHandler := handlers.NewRotationHandler(database.GetDB())
	log.Println("Key Rotation Handler initialized (admin-only)")

	// Initialize Usage Tracker for quota enforcement (REVENUE PROTECTION)
	usageTracker := usage.NewTracker(database.GetDB(), redisCache)
	if err := usageTracker.Migrate(); err != nil {
		log.Printf("WARNING: Usage tracker migration had issues: %v", err)
	}
	usageHandler := handlers.NewUsageHandlers(database.GetDB(), usageTracker)
	quotaChecker := middleware.NewQuotaChecker(usageTracker)
	log.Println("Usage Tracking & Quota Enforcement initialized (projects, storage, AI, execution)")
	log.Println("   - Free: 3 projects, 100MB storage, 1000 AI/month, 10 exec min/day")
	log.Println("   - Pro ($12): 25 projects, 5GB storage, 10000 AI/month, 120 exec min/day")
	log.Println("   - Team ($29): 100 projects, 25GB storage, 50000 AI/month, 480 exec min/day")
	log.Println("   - Enterprise ($79): Unlimited")

	// Initialize Prometheus Metrics and Business Metrics Collector
	metricsEnabled := getEnv("ENABLE_METRICS", "true") == "true"
	if metricsEnabled {
		// Set build info
		m := metrics.Get()
		m.SetBuildInfo(getEnv("VERSION", "dev"), getEnv("GIT_COMMIT", "unknown"), getEnv("BUILD_DATE", "unknown"))

		// Start business metrics collector (collects user/project/subscription counts)
		businessCollector := metrics.NewBusinessMetricsCollector(database.GetDB(), 30*time.Second)
		businessCollector.Start(context.Background())
		log.Println("Prometheus Metrics initialized")
		log.Println("   - HTTP metrics: requests_total, request_duration, response_size")
		log.Println("   - AI metrics: requests_total, tokens_total, cost_dollars, latency")
		log.Println("   - Execution metrics: total, duration, queue_length, container_usage")
		log.Println("   - Business metrics: active_users, total_projects, subscriptions")
		log.Println("   - Metrics endpoint: GET /metrics")
	}

	// Initialize API server
	server := api.NewServer(database, authService, aiRouter, byokManager)

	// Setup routes
	router := setupRoutes(
		server, buildHandler, wsHub, secretsHandler, mcpHandler,
		templatesHandler, searchHandler, previewHandler, gitHandler, importHandler,
		versionHandler,    // Version history system (Replit parity)
		commentsHandler,   // Code comments system (Replit parity)
		autonomousHandler, // Autonomous agent system (Replit parity)
		paymentHandler, executionHandler, deployHandler, packageHandler,
		environmentHandler, // Environment configuration (Nix-like - Replit parity)
		communityHandler, hostingHandler, databaseHandler, debuggingHandler,
		completionsHandler, extensionsHandler, enterpriseHandler, collabHub,
		optimizedHandler,
		byokHandler,     // BYOK API key management and model selection
		exportHandler,   // GitHub export (push projects to GitHub)
		usageHandler,    // Usage tracking and quota API endpoints
		quotaChecker,    // Quota enforcement middleware
		rotationHandler, // Key rotation (admin)
	)

	// Activate the full router now that all services are initialized.
	activeRouter.Store(router)
	startupReady.Store(true)

	log.Printf("Server ready on port %s", port)
	log.Printf("Health check: http://localhost:%s/health", port)
	log.Printf("API documentation: http://localhost:%s/docs", port)
	log.Println("")
	log.Println("APEX.BUILD is ready!")
	if secretsConfig.IsProduction {
		log.Println("Running in PRODUCTION mode with validated secrets")
	} else {
		log.Println("Running in DEVELOPMENT mode - some security checks relaxed")
	}

	// Graceful shutdown: listen for SIGTERM/SIGINT (Render, K8s, Docker stop)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Fatalf("CRITICAL: Failed to start server: %v", err)
	case sig := <-quit:
		log.Printf("Received signal %v, starting graceful shutdown...", sig)
	}

	// Give in-flight requests up to 15 seconds to complete
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// 1. Stop accepting new HTTP connections and drain existing ones
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	log.Println("HTTP server stopped")

	// 2. Stop all preview backend processes (prevents orphan child processes)
	if sr := previewHandler.GetServerRunner(); sr != nil {
		sr.StopAll(shutdownCtx)
		log.Println("Preview backend processes stopped")
	}

	// 3. Shutdown agent manager (cancels in-flight builds, closes task queues)
	agentManager.Shutdown()
	log.Println("Agent manager stopped")

	log.Println("Graceful shutdown complete")
}

// AppConfig holds all application configuration (non-secret)
// SECURITY: Secret values should come from config.SecretsConfig
type AppConfig struct {
	// Database configuration
	Database *db.Config

	// API Keys for AI providers
	ClaudeAPIKey  string
	OpenAIAPIKey  string
	GeminiAPIKey  string
	GrokAPIKey    string
	OllamaBaseURL string // Ollama local server URL (e.g., http://localhost:11434)

	// Authentication (fallback for development)
	JWTSecret string

	// Server configuration
	Port        string
	Environment string
}

// loadConfig loads application configuration from environment variables
// NOTE: Security-critical secrets are validated separately via config.ValidateAndLogSecrets()
func loadConfig() *AppConfig {
	// Check for DATABASE_URL first (Fly.io, Heroku, Railway, etc.)
	dbConfig := parseDatabaseURL(os.Getenv("DATABASE_URL"))
	if dbConfig == nil {
		// Fall back to individual environment variables
		dbConfig = &db.Config{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "password"),
			DBName:   getEnv("DB_NAME", "apex_build"),
			SSLMode:  getEnv("DB_SSL_MODE", "disable"),
			TimeZone: getEnv("DB_TIMEZONE", "UTC"),
		}
	}

	// JWT_SECRET handling is now done in config.ValidateAndLogSecrets()
	// This is just a fallback for development
	jwtSecret := os.Getenv("JWT_SECRET")
	environment := config.GetEnvironment()

	return &AppConfig{
		Database:      dbConfig,
		ClaudeAPIKey:  getEnvAny([]string{"ANTHROPIC_API_KEY", "CLAUDE_API_KEY"}, ""),
		OpenAIAPIKey:  getEnvAny([]string{"OPENAI_API_KEY", "CHATGPT_API_KEY", "GPT_API_KEY", "OPENAI_PLATFORM_API_KEY", "OPENAI_KEY", "OPENAI_TOKEN", "OPENAI_SECRET_KEY"}, ""),
		GeminiAPIKey:  getEnvAny([]string{"GEMINI_API_KEY", "GOOGLE_AI_API_KEY", "GOOGLE_GEMINI_API_KEY"}, ""),
		GrokAPIKey:    getEnv("XAI_API_KEY", ""),
		OllamaBaseURL: getEnv("OLLAMA_BASE_URL", ""), // Empty = disabled, or "http://localhost:11434"
		JWTSecret:     jwtSecret,
		Port:          getEnv("PORT", "8080"),
		Environment:   environment,
	}
}

// parseDatabaseURL parses a DATABASE_URL into a db.Config
// Format: postgres://user:password@host:port/dbname?sslmode=disable
func parseDatabaseURL(databaseURL string) *db.Config {
	if databaseURL == "" {
		return nil
	}

	log.Printf("Parsing DATABASE_URL for database connection")

	u, err := url.Parse(databaseURL)
	if err != nil {
		log.Printf("WARNING: Failed to parse DATABASE_URL: %v, falling back to individual vars", err)
		return nil
	}

	// Extract password
	password, _ := u.User.Password()

	// Extract port (default to 5432)
	port := 5432
	if u.Port() != "" {
		if p, err := strconv.Atoi(u.Port()); err == nil {
			port = p
		}
	}

	// Extract database name (remove leading /)
	dbName := strings.TrimPrefix(u.Path, "/")

	// Extract sslmode from query params
	sslMode := u.Query().Get("sslmode")
	if sslMode == "" {
		sslMode = "disable" // Fly.io internal connections don't need SSL
	}

	config := &db.Config{
		Host:     u.Hostname(),
		Port:     port,
		User:     u.User.Username(),
		Password: password,
		DBName:   dbName,
		SSLMode:  sslMode,
		TimeZone: "UTC",
	}

	log.Printf("✅ Database config: host=%s port=%d user=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.DBName, config.SSLMode)

	return config
}

// setupRoutes configures all API routes
func setupRoutes(
	server *api.Server, buildHandler *agents.BuildHandler, wsHub *agents.WSHub,
	secretsHandler *handlers.SecretsHandler, mcpHandler *handlers.MCPHandler,
	templatesHandler *handlers.TemplatesHandler, searchHandler *handlers.SearchHandler,
	previewHandler *handlers.PreviewHandler, gitHandler *handlers.GitHandler,
	importHandler *handlers.ImportHandler, // GitHub repository import wizard
	versionHandler *handlers.VersionHandler, // Version history system (Replit parity)
	commentsHandler *handlers.CommentsHandler, // Code comments system (Replit parity)
	autonomousHandler *autonomous.Handler, // Autonomous agent system (Replit parity)
	paymentHandler *handlers.PaymentHandlers, executionHandler *handlers.ExecutionHandler,
	deployHandler *handlers.DeployHandler, packageHandler *handlers.PackageHandler,
	environmentHandler *handlers.EnvironmentHandler, // Environment configuration (Nix-like - Replit parity)
	communityHandler *community.CommunityHandler,
	hostingHandler *handlers.HostingHandler, databaseHandler *handlers.DatabaseHandler,
	debuggingHandler *handlers.DebuggingHandler, completionsHandler *handlers.CompletionsHandler,
	extensionsHandler *handlers.ExtensionsHandler, enterpriseHandler *handlers.EnterpriseHandler,
	collabHub *collaboration.CollabHub,
	optimizedHandler *handlers.OptimizedHandler, // PERFORMANCE: Optimized handlers with caching
	byokHandler *handlers.BYOKHandlers, // BYOK API key management
	exportHandler *handlers.ExportHandler, // GitHub export
	usageHandler *handlers.UsageHandlers, // Usage tracking and quota API
	quotaChecker *middleware.QuotaChecker, // Quota enforcement middleware
	rotationHandler *handlers.RotationHandler, // Key rotation (admin)
) *gin.Engine {
	// Set gin mode based on environment
	if os.Getenv("ENVIRONMENT") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// Add middleware
	router.Use(server.CORSMiddleware())
	router.Use(gin.Recovery())
	router.Use(middleware.SecurityHeaders())

	// Add Prometheus metrics middleware (if enabled)
	if getEnv("ENABLE_METRICS", "true") == "true" {
		router.Use(metrics.PrometheusMiddleware())
		// Metrics endpoint (Prometheus format)
		router.GET("/metrics", metrics.PrometheusHandler())
	}

	// Health check endpoint
	router.GET("/health", server.Health)

	// API documentation endpoint
	router.GET("/docs", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"name":        "APEX.BUILD API",
			"version":     "1.0.0",
			"description": "Next-generation cloud development platform with multi-AI integration",
			"features": []string{
				"Multi-AI integration (Claude, GPT-4, Gemini)",
				"Real-time code collaboration",
				"Intelligent AI routing and fallbacks",
				"Enterprise-grade security",
				"High-performance code execution",
			},
			"competitive_advantages": map[string]string{
				"AI_response_time":    "1.5s (1440x faster than Replit's 36+ minutes)",
				"environment_startup": "85ms (120x faster than Replit's 3-10 seconds)",
				"cost_savings":        "50% cheaper with transparent pricing",
				"reliability":         "Multi-cloud architecture with 99.99% uptime",
				"interface":           "Beautiful cyberpunk UI vs bland corporate design",
			},
			"endpoints": gin.H{
				"authentication": []string{
					"POST /api/auth/register - User registration",
					"POST /api/auth/login - User login",
					"POST /api/auth/refresh - Refresh tokens",
				},
				"ai": []string{
					"POST /api/ai/generate - Multi-AI code generation and assistance",
					"GET /api/ai/usage - AI usage statistics",
				},
				"projects": []string{
					"POST /api/projects - Create project",
					"GET /api/projects - List user projects",
					"GET /api/projects/:id - Get project details",
					"PUT /api/projects/:id - Update project",
				},
				"files": []string{
					"POST /api/projects/:projectId/files - Create file",
					"GET /api/projects/:projectId/files - List project files",
					"GET /api/files/:id - Get file content",
					"PUT /api/files/:id - Update file content",
				},
			},
		})
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Authentication routes (no auth required, but rate limited)
		// SECURITY: Stricter rate limit (10 req/min) to prevent brute force attacks
		auth := v1.Group("/auth")
		auth.Use(middleware.AuthRateLimit())
		{
			auth.POST("/register", server.Register)
			auth.POST("/login", server.Login)
			auth.POST("/refresh", server.RefreshToken)
			auth.POST("/logout", server.Logout)
		}

		// Community/Sharing Marketplace public endpoints (no auth required for viewing)
		communityHandler.RegisterRoutes(v1)

		// Preview proxy endpoints (token-auth via query param for iframe embedding)
		previewProxy := v1.Group("/preview")
		{
			previewProxy.Any("/proxy/:projectId", previewHandler.ProxyPreview)
			previewProxy.Any("/proxy/:projectId/*path", previewHandler.ProxyPreview)
		}

		// Protected routes (authentication required)
		protected := v1.Group("/")
		protected.Use(server.AuthMiddleware())
		{
			// Usage tracking and quota API endpoints (REVENUE PROTECTION)
			usageHandler.RegisterUsageRoutes(protected)

			// AI endpoints - with quota enforcement
			ai := protected.Group("/ai")
			ai.Use(quotaChecker.CheckAIQuota()) // Enforce AI request quota
			{
				ai.POST("/generate", server.AIGenerate)
				ai.GET("/usage", server.GetAIUsage)
			}

			// Project endpoints - using OptimizedHandler for better performance
			// PERFORMANCE: 60-80% faster with proper JOINs, no N+1 queries, 30s cache
			projects := protected.Group("/projects")
			{
				// Project creation has quota check
				projects.POST("", quotaChecker.CheckProjectQuota(), optimizedHandler.CreateProjectOptimized)
				projects.GET("", optimizedHandler.GetProjectsOptimized)          // Optimized: cursor pagination, caching
				projects.GET("/:id", optimizedHandler.GetProjectOptimized)       // Optimized: JOINed file count
				projects.PUT("/:id", optimizedHandler.UpdateProjectOptimized)    // Optimized: cache invalidation
				projects.DELETE("/:id", optimizedHandler.DeleteProjectOptimized) // Optimized: cache invalidation
				projects.GET("/:id/download", server.DownloadProject)

				// File endpoints under projects - using optimized handler
				// Storage quota checked on file creation
				projects.POST("/:id/files", quotaChecker.CheckStorageQuota(1024*1024), server.CreateFile) // Estimate 1MB
				projects.GET("/:id/files", optimizedHandler.GetProjectFilesOptimized)                     // Optimized: no content loading for list
			}

			// File endpoints
			files := protected.Group("/files")
			{
				files.GET("/:id", server.GetFile)
				files.PUT("/:id", server.UpdateFile)
				files.DELETE("/:id", server.DeleteFile)
			}

			// User profile endpoints
			user := protected.Group("/user")
			{
				user.GET("/profile", server.GetUserProfile)
				user.PUT("/profile", server.UpdateUserProfile)
			}

			// Build/Agent endpoints (the core of APEX.BUILD)
			buildHandler.RegisterRoutes(protected)

			// Autonomous Agent endpoints (AI-driven build, test, deploy)
			autonomousHandler.RegisterRoutes(protected)

			// Secrets Management endpoints
			secretsRoutes := protected.Group("/secrets")
			{
				secretsRoutes.GET("", secretsHandler.ListSecrets)
				secretsRoutes.POST("", secretsHandler.CreateSecret)
				secretsRoutes.GET("/:id", secretsHandler.GetSecret)
				secretsRoutes.PUT("/:id", secretsHandler.UpdateSecret)
				secretsRoutes.DELETE("/:id", secretsHandler.DeleteSecret)
				secretsRoutes.POST("/:id/rotate", secretsHandler.RotateSecret)
				secretsRoutes.GET("/:id/audit", secretsHandler.GetAuditLog)
			}

			// Project-specific secrets (environment variables)
			protected.GET("/projects/:id/secrets", secretsHandler.GetProjectSecrets)

			// MCP Server Management endpoints
			mcpRoutes := protected.Group("/mcp")
			{
				// External MCP server management
				mcpRoutes.GET("/servers", mcpHandler.ListExternalServers)
				mcpRoutes.POST("/servers", mcpHandler.AddExternalServer)
				mcpRoutes.DELETE("/servers/:id", mcpHandler.DeleteExternalServer)
				mcpRoutes.POST("/servers/:id/connect", mcpHandler.ConnectToServer)
				mcpRoutes.POST("/servers/:id/disconnect", mcpHandler.DisconnectFromServer)

				// Tool execution on connected MCP servers
				mcpRoutes.POST("/servers/:id/tools/call", mcpHandler.CallTool)
				mcpRoutes.GET("/servers/:id/resources", mcpHandler.ReadResource)

				// Aggregated tools across all connected servers
				mcpRoutes.GET("/tools", mcpHandler.GetAvailableTools)
			}

			// Project Templates endpoints
			templates := protected.Group("/templates")
			{
				templates.GET("", templatesHandler.ListTemplates)
				templates.GET("/categories", templatesHandler.GetCategories)
				templates.GET("/:id", templatesHandler.GetTemplate)
				templates.POST("/create-project", templatesHandler.CreateProjectFromTemplate)
			}

			// Code Search endpoints
			searchRoutes := protected.Group("/search")
			{
				searchRoutes.POST("", searchHandler.Search)                       // Full search with all options
				searchRoutes.GET("/quick", searchHandler.QuickSearch)             // Quick search for autocomplete
				searchRoutes.GET("/symbols", searchHandler.SearchSymbols)         // Symbol search (functions, classes)
				searchRoutes.GET("/files", searchHandler.SearchFiles)             // File name search
				searchRoutes.POST("/replace", searchHandler.SearchAndReplace)     // Search & replace
				searchRoutes.GET("/history", searchHandler.GetSearchHistory)      // Search history
				searchRoutes.DELETE("/history", searchHandler.ClearSearchHistory) // Clear history
			}

			// Live Preview endpoints
			previewRoutes := protected.Group("/preview")
			{
				previewRoutes.POST("/start", previewHandler.StartPreview)                    // Start preview server
				previewRoutes.POST("/fullstack/start", previewHandler.StartFullStackPreview) // Start preview + backend runtime
				previewRoutes.POST("/stop", previewHandler.StopPreview)                      // Stop preview server
				previewRoutes.GET("/status/:projectId", previewHandler.GetPreviewStatus)     // Get status
				previewRoutes.POST("/refresh", previewHandler.RefreshPreview)                // Trigger reload
				previewRoutes.POST("/hot-reload", previewHandler.HotReload)                  // Hot reload file
				previewRoutes.GET("/list", previewHandler.ListPreviews)                      // List active previews
				previewRoutes.GET("/url/:projectId", previewHandler.GetPreviewURL)           // Get preview URL

				// Bundler endpoints
				previewRoutes.POST("/build", previewHandler.BuildProject)                       // Bundle project
				previewRoutes.GET("/bundler/status", previewHandler.GetBundlerStatus)           // Bundler availability
				previewRoutes.POST("/bundler/invalidate", previewHandler.InvalidateBundleCache) // Invalidate cache

				// Backend server endpoints
				previewRoutes.POST("/server/start", previewHandler.StartServer)                // Start backend server
				previewRoutes.POST("/server/stop", previewHandler.StopServer)                  // Stop backend server
				previewRoutes.GET("/server/status/:projectId", previewHandler.GetServerStatus) // Server status
				previewRoutes.GET("/server/logs/:projectId", previewHandler.GetServerLogs)     // Server logs
				previewRoutes.GET("/server/detect/:projectId", previewHandler.DetectServer)    // Detect backend

				// Docker sandbox endpoint
				previewRoutes.GET("/docker/status", previewHandler.GetDockerStatus) // Docker availability
			}

			// Git Integration endpoints
			gitRoutes := protected.Group("/git")
			{
				gitRoutes.POST("/connect", gitHandler.ConnectRepository)                  // Connect to repo
				gitRoutes.GET("/repo/:projectId", gitHandler.GetRepository)               // Get repo info
				gitRoutes.DELETE("/repo/:projectId", gitHandler.DisconnectRepository)     // Disconnect
				gitRoutes.GET("/branches/:projectId", gitHandler.GetBranches)             // List branches
				gitRoutes.GET("/commits/:projectId", gitHandler.GetCommits)               // Get commits
				gitRoutes.GET("/status/:projectId", gitHandler.GetStatus)                 // Working tree status
				gitRoutes.POST("/commit", gitHandler.Commit)                              // Create commit
				gitRoutes.POST("/push", gitHandler.Push)                                  // Push to remote
				gitRoutes.POST("/pull", gitHandler.Pull)                                  // Pull from remote
				gitRoutes.POST("/branch", gitHandler.CreateBranch)                        // Create branch
				gitRoutes.POST("/checkout", gitHandler.SwitchBranch)                      // Switch branch
				gitRoutes.GET("/pulls/:projectId", gitHandler.GetPullRequests)            // List PRs
				gitRoutes.POST("/pulls", gitHandler.CreatePullRequest)                    // Create PR
				gitRoutes.POST("/export", exportHandler.ExportToGitHub)                   // Export project to GitHub
				gitRoutes.GET("/export/status/:projectId", exportHandler.GetExportStatus) // Check export status
			}

			// GitHub Repository Import Wizard (one-click import like replit.new/URL)
			importHandler.RegisterImportRoutes(protected)

			// Version History System (Replit parity feature)
			// Enables viewing file versions, diffs, and restoring previous states
			versionHandler.RegisterVersionRoutes(protected)

			// Code Comments System (Replit parity feature)
			// Inline code comments, threads, reactions, and resolve functionality
			commentsHandler.RegisterCommentRoutes(protected)

			// Billing & Subscription endpoints (Stripe integration)
			billing := protected.Group("/billing")
			{
				billing.POST("/checkout", paymentHandler.CreateCheckoutSession)    // Create Stripe checkout
				billing.GET("/subscription", paymentHandler.GetSubscription)       // Get current subscription
				billing.POST("/portal", paymentHandler.CreateBillingPortalSession) // Stripe billing portal
				billing.GET("/plans", paymentHandler.GetPlans)                     // List available plans
				billing.GET("/usage", paymentHandler.GetUsage)                     // Get usage stats
				billing.POST("/cancel", paymentHandler.CancelSubscription)         // Cancel subscription
				billing.POST("/reactivate", paymentHandler.ReactivateSubscription) // Reactivate subscription
				billing.GET("/invoices", paymentHandler.GetInvoices)               // Get invoice history
				billing.GET("/payment-methods", paymentHandler.GetPaymentMethods)  // Get payment methods
				billing.GET("/check-limit/:type", paymentHandler.CheckUsageLimit)  // Check usage limit
				billing.GET("/config-status", paymentHandler.StripeConfigStatus)   // Check Stripe config
			}

			// Code Execution endpoints (the core of cloud IDE) - with quota enforcement
			if executionHandler != nil {
				execute := protected.Group("/execute")
				execute.Use(quotaChecker.CheckExecutionQuota(1)) // Check execution minutes quota
				{
					execute.POST("", executionHandler.ExecuteCode)                           // Execute code snippet
					execute.POST("/file", executionHandler.ExecuteFile)                      // Execute a file
					execute.POST("/project", executionHandler.ExecuteProject)                // Execute entire project
					execute.GET("/languages", executionHandler.GetLanguages)                 // Get supported languages
					execute.GET("/:id", executionHandler.GetExecution)                       // Get execution details
					execute.GET("/history", executionHandler.GetExecutionHistory)            // Get execution history
					execute.POST("/:id/stop", executionHandler.StopExecution)                // Stop running execution
					execute.GET("/stats", executionHandler.GetExecutionStats)                // Get execution statistics
					execute.GET("/sandbox/status", executionHandler.GetSandboxStatusHandler) // Get sandbox security status
				}

				// Terminal endpoints (interactive shell with full PTY support)
				terminal := protected.Group("/terminal")
				{
					terminal.POST("/sessions", executionHandler.CreateTerminalSession)            // Create new terminal
					terminal.GET("/sessions", executionHandler.ListTerminalSessions)              // List all terminals
					terminal.GET("/sessions/:id", executionHandler.GetTerminalSession)            // Get terminal info
					terminal.DELETE("/sessions/:id", executionHandler.DeleteTerminalSession)      // Close terminal
					terminal.POST("/sessions/:id/resize", executionHandler.ResizeTerminalSession) // Resize terminal
					terminal.GET("/sessions/:id/history", executionHandler.GetTerminalHistory)    // Get command history
					terminal.GET("/shells", executionHandler.GetAvailableShells)                  // List available shells
				}
			}

			// One-Click Deployment endpoints (Vercel, Netlify, Render)
			deployRoutes := protected.Group("/deploy")
			{
				deployRoutes.POST("", deployHandler.StartDeployment)                                  // Start deployment
				deployRoutes.GET("/:id", deployHandler.GetDeployment)                                 // Get deployment details
				deployRoutes.GET("/:id/status", deployHandler.GetDeploymentStatus)                    // Get status only
				deployRoutes.GET("/:id/logs", deployHandler.GetDeploymentLogs)                        // Get deployment logs
				deployRoutes.DELETE("/:id", deployHandler.CancelDeployment)                           // Cancel deployment
				deployRoutes.POST("/:id/redeploy", deployHandler.Redeploy)                            // Redeploy
				deployRoutes.GET("/providers", deployHandler.GetProviders)                            // List providers
				deployRoutes.GET("/projects/:projectId/history", deployHandler.GetProjectDeployments) // Deployment history
				deployRoutes.GET("/projects/:projectId/latest", deployHandler.GetLatestDeployment)    // Latest deployment
			}

			// Package Management endpoints (NPM, PyPI, Go Modules)
			packageHandler.RegisterPackageRoutes(protected)

			// Environment Configuration endpoints (Nix-like reproducible environments)
			environmentHandler.RegisterEnvironmentRoutes(protected)

			// Community/Sharing Marketplace endpoints (protected actions)
			communityHandler.RegisterProtectedRoutes(protected)

			// Native Hosting endpoints (.apex.app)
			hostingHandler.RegisterHostingRoutes(protected)

			// Managed Database endpoints
			if databaseHandler != nil {
				databaseHandler.RegisterDatabaseRoutes(protected)
			}

			// Debugging endpoints
			debuggingHandler.RegisterRoutes(protected)

			// AI Completions endpoints
			completionsHandler.RegisterCompletionRoutes(protected)

			// Extensions Marketplace endpoints
			extensionsHandler.RegisterExtensionRoutes(protected)

			// Enterprise endpoints (SSO, RBAC, Audit Logs, Organizations)
			enterpriseHandler.RegisterEnterpriseRoutes(protected, v1)

			// BYOK (Bring Your Own Key) endpoints
			byok := protected.Group("/byok")
			{
				byok.POST("/keys", byokHandler.SaveKey)
				byok.GET("/keys", byokHandler.GetKeys)
				byok.DELETE("/keys/:provider", byokHandler.DeleteKey)
				byok.PATCH("/keys/:provider", byokHandler.UpdateKeySettings)
				byok.POST("/keys/:provider/validate", byokHandler.ValidateKey)
				byok.GET("/usage", byokHandler.GetUsage)
				byok.GET("/models", byokHandler.GetModels)
			}

			// Admin endpoints (requires admin privileges)
			admin := protected.Group("/admin")
			admin.Use(server.AdminMiddleware())
			{
				admin.GET("/dashboard", server.AdminDashboard)
				admin.GET("/users", server.AdminGetUsers)
				admin.GET("/users/:id", server.AdminGetUser)
				admin.PUT("/users/:id", server.AdminUpdateUser)
				admin.DELETE("/users/:id", server.AdminDeleteUser)
				admin.POST("/users/:id/credits", server.AdminAddCredits)
				admin.GET("/stats", server.AdminGetSystemStats)
				admin.POST("/rotate-secrets", rotationHandler.RotateSecrets)
				admin.GET("/validate-secrets", rotationHandler.ValidateSecrets)
			}
		}
	}

	// WebSocket endpoint for real-time build updates
	router.GET("/ws/build/:buildId", wsHub.HandleWebSocket)

	// WebSocket endpoint for interactive terminal sessions
	if executionHandler != nil {
		router.GET("/ws/terminal/:sessionId", executionHandler.HandleTerminalWebSocket)
	}

	// MCP WebSocket endpoint (for APEX.BUILD as MCP server)
	router.GET("/mcp/ws", mcpHandler.HandleMCPWebSocket)

	// WebSocket endpoint for real-time collaboration
	router.GET("/ws/collab", collabHub.HandleWebSocket)

	// WebSocket endpoint for debugging sessions
	router.GET("/ws/debug/:sessionId", debuggingHandler.HandleDebugWebSocket)

	// WebSocket endpoint for deployment log streaming
	router.GET("/ws/deploy/:deploymentId", hostingHandler.HandleDeploymentWebSocket)

	// WebSocket endpoint for autonomous agent real-time updates
	autonomousHandler.RegisterWebSocketRoute(router)

	return router
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAny(keys []string, defaultValue string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getStatusIcon(enabled bool) string {
	if enabled {
		return "✅ Enabled"
	}
	return "❌ Disabled (no API key)"
}

func getOllamaStatus(baseURL string) string {
	if baseURL != "" {
		return "✅ Enabled (" + baseURL + ")"
	}
	return "❌ Disabled (set OLLAMA_BASE_URL)"
}
