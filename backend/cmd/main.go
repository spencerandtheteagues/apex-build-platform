package main

import (
	"context"
	"log"
	"net"
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
	"apex-build/internal/applog"
	"apex-build/internal/auth"
	"apex-build/internal/budget"
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
	"apex-build/internal/email"
	"apex-build/internal/enterprise"
	"apex-build/internal/extensions"
	"apex-build/internal/git"
	"apex-build/internal/handlers"
	"apex-build/internal/hosting"
	"apex-build/internal/mcp"
	"apex-build/internal/metrics"
	"apex-build/internal/middleware"
	"apex-build/internal/mobile"
	"apex-build/internal/payments"
	"apex-build/internal/preview"
	"apex-build/internal/search"
	"apex-build/internal/secrets"
	"apex-build/internal/spend"
	"apex-build/internal/startup"
	"apex-build/internal/storage"
	"apex-build/internal/usage"
	"apex-build/internal/websocket"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	log.Println("Starting APEX.BUILD - Multi-AI Cloud Development Platform")

	// Load environment files from common local run locations.
	// `godotenv.Load` does not overwrite variables that are already exported.
	envPaths := []string{
		".env",
		".env.local",
		".env.docker.local",
		"backend/.env.docker.local",
		"../.env",
		"../.env.local",
		"../backend/.env.docker.local",
	}
	loadedEnv := false
	for _, path := range envPaths {
		if err := godotenv.Load(path); err == nil {
			loadedEnv = true
		}
	}
	if !loadedEnv {
		log.Println("WARNING: No .env file found, using environment variables")
	}
	log.Println("Environment configuration loaded")

	// Init structured logger (JSON in production, text in dev)
	applog.Init()

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
	startupRegistry := newStartupRegistry()

	bootstrapRouter := gin.New()
	bootstrapRouter.GET("/health", func(c *gin.Context) {
		summary := startupRegistry.Snapshot()
		c.JSON(http.StatusOK, gin.H{
			"status":  summary.Status,
			"ready":   startupReady.Load() && summary.Ready,
			"phase":   summary.Phase,
			"startup": summary,
		})
	})
	bootstrapRouter.GET("/ready", func(c *gin.Context) {
		summary := startupRegistry.Snapshot()
		statusCode := featureReadinessHTTPStatus(summary)
		c.JSON(statusCode, summary)
	})
	bootstrapRouter.GET("/health/features", func(c *gin.Context) {
		summary := startupRegistry.Snapshot()
		statusCode := featureReadinessHTTPStatus(summary)
		c.JSON(statusCode, summary)
	})
	bootstrapRouter.NoRoute(func(c *gin.Context) {
		summary := startupRegistry.Snapshot()
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "server starting",
			"ready":   startupReady.Load() && summary.Ready,
			"phase":   summary.Phase,
			"startup": summary,
		})
	})
	activeRouter.Store(bootstrapRouter)

	serverErrors := make(chan error, 1)
	httpServer := &http.Server{
		Addr:              ":" + port,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second, // Long for SSE/streaming build responses
		IdleTimeout:       60 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			activeRouter.Load().(*gin.Engine).ServeHTTP(w, r)
		}),
	}
	listener, err := net.Listen("tcp", httpServer.Addr)
	if err != nil {
		startupRegistry.MarkFailed("bootstrap_http", startup.TierCritical, "Failed to bind bootstrap HTTP listener", map[string]any{
			"error": err.Error(),
		})
		startupRegistry.SetPhase(startup.PhaseFailed)
		log.Fatalf("CRITICAL: Failed to bind HTTP listener on %s: %v", httpServer.Addr, err)
	}
	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()
	startupRegistry.MarkReady("bootstrap_http", startup.TierCritical, "Bootstrap health listener accepting requests", map[string]any{
		"addr": listener.Addr().String(),
	})
	log.Printf("Bootstrap HTTP listener started on port %s (health endpoint ready immediately)", port)

	// SECURITY: Validate all required secrets before starting.
	secretsConfig, err := config.ValidateAndLogSecrets()
	if err != nil {
		startupRegistry.MarkFailed("secrets_validation", startup.TierCritical, "Required secrets validation failed", map[string]any{
			"error": err.Error(),
		})
		startupRegistry.SetPhase(startup.PhaseFailed)
		log.Fatalf("FATAL: Cannot start server — secrets validation failed: %v", err)
	}
	startupRegistry.MarkReady("secrets_validation", startup.TierCritical, "Required secrets validated", map[string]any{
		"environment": secretsConfig.Environment,
	})

	// Initialize database
	database, err := db.NewDatabase(appConfig.Database)
	if err != nil {
		startupRegistry.MarkFailed("primary_database", startup.TierCritical, "Failed to connect to primary database", map[string]any{
			"error": err.Error(),
		})
		startupRegistry.SetPhase(startup.PhaseFailed)
		log.Fatalf("CRITICAL: Failed to connect to database: %v", err)
	}
	startupRegistry.MarkReady("primary_database", startup.TierCritical, "Primary database connected", nil)
	defer database.Close()

	explicitSeedPasswords := db.HasExplicitSeedPasswords()
	seedRuntimeAccounts := !(config.IsProductionEnvironment() || config.IsStagingEnvironment()) ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("ENABLE_RUNTIME_SEEDS")), "true") ||
		explicitSeedPasswords
	if seedRuntimeAccounts {
		if err := database.RunSeeds(); err != nil {
			startupRegistry.MarkDegraded("database_seeding", startup.TierOptional, "Database seeds completed with warnings", map[string]any{
				"error":                   err.Error(),
				"explicit_seed_passwords": explicitSeedPasswords,
			})
			log.Printf("WARNING: Database seeding had issues: %v", err)
		} else {
			startupRegistry.MarkReady("database_seeding", startup.TierOptional, "Database seeds completed", map[string]any{
				"explicit_seed_passwords": explicitSeedPasswords,
			})
		}
	} else {
		startupRegistry.MarkReady("database_seeding", startup.TierOptional, "Runtime database seeds skipped for this environment", map[string]any{
			"enabled":                 false,
			"explicit_seed_passwords": explicitSeedPasswords,
		})
	}

	// One-shot admin promotion: ADMIN_PROMOTE_EMAIL may be a comma-separated list.
	// Grants is_admin + is_super_admin to each email on startup.
	// Clear the env var after the deploy so it doesn't re-fire unnecessarily.
	for _, promoteEmail := range strings.Split(os.Getenv("ADMIN_PROMOTE_EMAIL"), ",") {
		promoteEmail = strings.TrimSpace(promoteEmail)
		if promoteEmail == "" {
			continue
		}
		res := database.DB.Exec(
			`UPDATE users SET is_admin = true, is_super_admin = true WHERE email = ?`,
			promoteEmail,
		)
		if res.Error != nil {
			log.Printf("WARNING: admin promotion for %s failed: %v", promoteEmail, res.Error)
		} else {
			log.Printf("ADMIN_PROMOTE: granted is_admin+is_super_admin to %s (%d row(s) updated)", promoteEmail, res.RowsAffected)
		}
	}

	// Initialize authentication service with validated JWT secret
	// Use the validated secret from secretsConfig for consistency
	jwtSecret := secretsConfig.JWTSecret
	if jwtSecret == "" {
		jwtSecret = appConfig.JWTSecret // Fallback for dev
	}
	authService := auth.NewAuthService(jwtSecret)
	authService.SetDB(database.DB)
	startupRegistry.MarkReady("auth_service", startup.TierCritical, "Authentication service initialized", nil)

	// Initialize AI router with all providers (Claude, OpenAI, Gemini, Grok, Ollama).
	// Ollama is enabled whenever OLLAMA_BASE_URL is set — in any environment.
	// In development with Ollama-only mode (OLLAMA_BASE_URL set, no cloud keys), cloud
	// keys from the OS environment are suppressed so only the local/cloud Ollama model
	// is used. This prevents ambient shell exports from leaking into the router.
	ollamaRouterURL := appConfig.OllamaBaseURL
	ollamaAPIKey := appConfig.OllamaAPIKey
	claudeKey := appConfig.ClaudeAPIKey
	openaiKey := appConfig.OpenAIAPIKey
	geminiKey := appConfig.GeminiAPIKey
	grokKey := appConfig.GrokAPIKey

	// Ollama Cloud / managed mode: if OLLAMA_API_KEY is set (even in production),
	// enable the Ollama provider with the provided key and base URL.
	ollamaCloudEnabled := appConfig.OllamaAPIKey != ""
	if ollamaCloudEnabled {
		if appConfig.OllamaBaseURL == "" {
			appConfig.OllamaBaseURL = "https://ollama.com/v1"
		}
		log.Printf("OLLAMA CLOUD: Enabling Ollama provider at %s", appConfig.OllamaBaseURL)
	}

	if !config.IsProductionEnvironment() && appConfig.OllamaBaseURL != "" {
		ollamaRouterURL = appConfig.OllamaBaseURL
		// Suppress cloud keys so the router sees only Ollama and the single-provider
		// Ollama profile activates (isLocalDevSingleOllamaProfile → true).
		claudeKey = ""
		openaiKey = ""
		geminiKey = ""
		grokKey = ""
		log.Printf("DEV MODE: Ollama-only mode — cloud API keys suppressed, using %s", ollamaRouterURL)
	} else if ollamaCloudEnabled {
		// In production with OLLAMA_API_KEY set, add Ollama alongside other providers.
		ollamaRouterURL = appConfig.OllamaBaseURL
		log.Printf("PROD MODE: Ollama enabled at %s", ollamaRouterURL)
	}

	aiRouter := ai.NewAIRouter(
		claudeKey,
		openaiKey,
		geminiKey,
		grokKey,
		ollamaRouterURL,
		ollamaAPIKey,
	)

	// If Ollama Cloud BYOK is enabled, upgrade the Ollama client to cloud mode
	if ollamaCloudEnabled && ollamaRouterURL != "" {
		if ollamaClient, ok := aiRouter.GetClient(ai.ProviderOllama); ok {
			if cloudClient, ok := ollamaClient.(*ai.OllamaClient); ok {
				cloudClient.SetAPIKey(appConfig.OllamaAPIKey)
				log.Println("OLLAMA CLOUD: API key configured for BYOK")
			}
		}
	}

	log.Println("Multi-AI integration initialized:")
	log.Printf("   - Claude API: %s", getStatusIcon(claudeKey != ""))
	log.Printf("   - OpenAI API: %s", getStatusIcon(openaiKey != ""))
	log.Printf("   - Gemini API: %s", getStatusIcon(geminiKey != ""))
	log.Printf("   - Grok API:   %s", getStatusIcon(grokKey != ""))
	if ollamaRouterURL != "" {
		log.Printf("   - Ollama:     ✅ Enabled (%s, key=%s)", ollamaRouterURL, getStatusIcon(ollamaAPIKey != ""))
	} else {
		log.Printf("   - Ollama:     ❌ Disabled (set OLLAMA_BASE_URL or OLLAMA_API_KEY to enable)")
	}
	platformAIProviders := 0
	if claudeKey != "" {
		platformAIProviders++
	}
	if openaiKey != "" {
		platformAIProviders++
	}
	if geminiKey != "" {
		platformAIProviders++
	}
	if grokKey != "" {
		platformAIProviders++
	}
	aiDetails := map[string]any{
		"platform_provider_count": platformAIProviders,
		"ollama_enabled":          ollamaRouterURL != "",
		"byok_available":          true,
	}
	if platformAIProviders == 0 && ollamaRouterURL == "" {
		startupRegistry.MarkDegraded("ai_platform", startup.TierOptional, "No platform AI providers configured; BYOK-only mode", aiDetails)
	} else {
		startupRegistry.MarkReady("ai_platform", startup.TierOptional, "AI router initialized", aiDetails)
	}

	// Initialize Secrets Manager with validated master key
	// SECURITY: Use validated key from secretsConfig, with fallback for development ONLY
	masterKey := secretsConfig.SecretsMasterKey
	if masterKey == "" {
		if config.IsProductionEnvironment() {
			startupRegistry.MarkFailed("secrets_manager", startup.TierCritical, "SECRETS_MASTER_KEY missing in production", nil)
			startupRegistry.SetPhase(startup.PhaseFailed)
			log.Fatalf("CRITICAL: SECRETS_MASTER_KEY not set in production - refusing to start. " +
				"Generate with: openssl rand -base64 32 and set as env var. " +
				"Without a persistent key, all encrypted user data (API keys, secrets) will be permanently lost on restart.")
		}
		// Generate an ephemeral key for development only
		var genErr error
		masterKey, genErr = secrets.GenerateMasterKey()
		if genErr != nil {
			startupRegistry.MarkFailed("secrets_manager", startup.TierCritical, "Failed to generate development master key", map[string]any{
				"error": genErr.Error(),
			})
			startupRegistry.SetPhase(startup.PhaseFailed)
			log.Fatalf("CRITICAL: Failed to generate development master key: %v", genErr)
		}
		log.Println("DEV ONLY: Using ephemeral SECRETS_MASTER_KEY - encrypted data will not survive restart")
	}

	secretsManager, err := secrets.NewSecretsManager(masterKey)
	if err != nil {
		startupRegistry.MarkFailed("secrets_manager", startup.TierCritical, "Failed to initialize secrets manager", map[string]any{
			"error": err.Error(),
		})
		startupRegistry.SetPhase(startup.PhaseFailed)
		log.Fatalf("CRITICAL: Failed to initialize secrets manager: %v", err)
	}
	startupRegistry.MarkReady("secrets_manager", startup.TierCritical, "Secrets manager initialized", map[string]any{
		"persistent_key": secretsConfig.SecretsMasterKey != "",
	})
	log.Println("Secrets Manager initialized with AES-256 encryption")

	// Initialize BYOK (Bring Your Own Key) Manager
	byokManager := ai.NewBYOKManager(database.GetDB(), secretsManager, aiRouter)
	byokHandler := handlers.NewBYOKHandlers(byokManager)
	log.Println("BYOK Manager initialized (user-provided API keys, per-provider cost tracking)")
	startupRegistry.MarkReady("byok", startup.TierOptional, "BYOK manager initialized", nil)

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
	startupRegistry.MarkReady("agent_orchestration", startup.TierOptional, "Agent orchestration services initialized", nil)

	// Initialize Spend Tracker (S2: Real-Time Spend Dashboard)
	spendTracker := spend.NewSpendTracker(database.GetDB())
	spendHandler := handlers.NewSpendHandler(spendTracker)
	if err := database.GetDB().AutoMigrate(&spend.SpendEvent{}, &budget.BudgetCap{}); err != nil {
		log.Printf("WARNING: Spend/budget migrations completed with warnings: %v", err)
		startupRegistry.MarkDegraded("spend_tracking", startup.TierOptional, "Spend tracking migrations completed with warnings", map[string]any{
			"error": err.Error(),
		})
	} else {
		startupRegistry.MarkReady("spend_tracking", startup.TierOptional, "Spend tracking schema initialized", nil)
	}
	log.Println("Spend Tracker initialized (real-time cost tracking, per-agent attribution)")

	// Initialize Budget Enforcer (S1: Hard Budget Caps)
	budgetEnforcer := budget.NewBudgetEnforcer(database.GetDB(), spendTracker)
	budgetHandler := handlers.NewBudgetHandler(budgetEnforcer)
	budgetMiddleware := middleware.BudgetCheck(budgetEnforcer)
	log.Println("Budget Enforcer initialized (daily/monthly/per-build caps, instant stop)")
	startupRegistry.MarkReady("budget_enforcement", startup.TierOptional, "Budget enforcement initialized", nil)

	// Wire budget enforcer into agent manager so each AI Generate call is
	// pre-authorized with a real estimated cost rather than 0.
	agentManager.SetBudgetEnforcer(budgetEnforcer)

	// Initialize Protected Paths Handler (A3)
	protectedPathsHandler := handlers.NewProtectedPathsHandler(database.GetDB())
	startupRegistry.MarkReady("protected_paths", startup.TierOptional, "Protected paths handler initialized", nil)

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
		lang, ok := args["language"].(string)
		if !ok || strings.TrimSpace(lang) == "" {
			return &mcp.ToolCallResult{
				Content: []mcp.ContentBlock{{Type: "text", Text: "language parameter required"}},
				IsError: true,
			}, nil
		}
		desc, ok := args["description"].(string)
		if !ok || strings.TrimSpace(desc) == "" {
			return &mcp.ToolCallResult{
				Content: []mcp.ContentBlock{{Type: "text", Text: "description parameter required"}},
				IsError: true,
			}, nil
		}

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
	startupRegistry.MarkReady("project_secrets", startup.TierOptional, "Project secrets handlers initialized", nil)
	startupRegistry.MarkReady("mcp", startup.TierOptional, "MCP handlers initialized", nil)
	startupRegistry.MarkReady("project_templates", startup.TierOptional, "Project templates initialized", nil)

	log.Println("Project Templates System initialized (15+ starter templates)")

	// Initialize Code Search Engine
	searchEngine := search.NewSearchEngine(database.GetDB())
	searchHandler := handlers.NewSearchHandler(searchEngine, database.GetDB())
	startupRegistry.MarkReady("code_search", startup.TierOptional, "Code search initialized", nil)

	log.Println("Code Search Engine initialized (full-text, regex, symbol search)")

	// Initialize Live Preview Server
	previewFactoryConfig := preview.DefaultFactoryConfig()
	previewFactoryConfig.ForceContainerMode = strings.EqualFold(strings.TrimSpace(os.Getenv("ENVIRONMENT")), "production") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("PREVIEW_FORCE_CONTAINER")), "true")

	var previewHandler *handlers.PreviewHandler
	previewFactory, previewFactoryErr := preview.NewPreviewServerFactory(database.GetDB(), previewFactoryConfig)
	if previewFactoryErr != nil {
		log.Printf("WARNING: Preview container factory unavailable: %v", previewFactoryErr)
		previewHandler = handlers.NewPreviewHandler(database.GetDB(), preview.NewPreviewServer(database.GetDB()), authService)
	} else {
		previewHandler = handlers.NewPreviewHandlerWithFactory(database.GetDB(), previewFactory, authService)
	}

	// Wire preview verification gate into the agent manager.
	// The bridge converts between agents.VerifiableFile and preview.VerifiableFile
	// so neither package needs to import the other.
	// Runtime Vite boot proof defaults on in production when Chrome is available.
	// Operators can still force it on/off with APEX_PREVIEW_RUNTIME_VERIFY.
	chromePath := preview.FindChrome()
	chromeLaunchOK := false
	if chromePath != "" {
		if err := preview.SmokeTestChrome(context.Background(), chromePath); err != nil {
			log.Printf("WARNING: Chrome/Chromium found at %s but failed headless smoke test: %v", chromePath, err)
			chromePath = ""
		} else {
			chromeLaunchOK = true
		}
	}
	runtimeVerifyEnabled, runtimeVerifyMode := resolvePreviewRuntimeVerify(chromePath)
	var pvVerifier *preview.Verifier
	if runtimeVerifyEnabled {
		pvVerifier = preview.NewVerifierWithRuntime(previewHandler.GetServerRunner())
		log.Println("Preview verification gate enabled (runtime boot proof ON)")
	} else {
		pvVerifier = preview.NewVerifier(previewHandler.GetServerRunner())
		log.Printf("Preview verification gate enabled (static checks only; mode=%s)", runtimeVerifyMode)
	}
	agentManager.SetPreviewVerifier(&previewVerifierBridge{verifier: pvVerifier})

	// Surface runtime/browser verify capability in /health/features.
	pvRuntimeDetails := map[string]any{
		"enabled":          runtimeVerifyEnabled,
		"browser_proof":    runtimeVerifyEnabled && chromePath != "",
		"canary_probes":    runtimeVerifyEnabled && chromePath != "" && preview.CanaryProbesEnabled(),
		"chrome_available": chromePath != "",
		"chrome_launch_ok": chromeLaunchOK,
		"mode":             runtimeVerifyMode,
	}
	if runtimeVerifyEnabled {
		if chromePath != "" {
			startupRegistry.MarkReady("preview_runtime_verify", startup.TierOptional,
				"Runtime Vite boot proof enabled (browser: yes)",
				pvRuntimeDetails)
		} else {
			startupRegistry.MarkDegraded("preview_runtime_verify", startup.TierOptional,
				"Runtime Vite boot proof enabled but Chrome was not found on PATH",
				pvRuntimeDetails)
		}
	} else {
		message := "Runtime Vite boot proof disabled by default outside production (set APEX_PREVIEW_RUNTIME_VERIFY=true)"
		switch runtimeVerifyMode {
		case "explicit_disabled":
			message = "Runtime Vite boot proof explicitly disabled (APEX_PREVIEW_RUNTIME_VERIFY=false)"
		case "production_no_chrome":
			message = "Runtime Vite boot proof unavailable in production because Chrome was not found on PATH"
		}
		startupRegistry.MarkDegraded("preview_runtime_verify", startup.TierOptional,
			message, pvRuntimeDetails)
	}

	log.Println("Live Preview Server initialized")
	previewFeatureStatus := previewHandler.FeatureStatus()
	if previewFactoryErr != nil {
		previewFeatureStatus["factory_error"] = previewFactoryErr.Error()
	}
	if sandboxRequired, _ := previewFeatureStatus["sandbox_required"].(bool); sandboxRequired {
		if sandboxReady, _ := previewFeatureStatus["sandbox_ready"].(bool); sandboxReady {
			log.Println("SECURITY: Live preview sandbox enforcement enabled")
		} else {
			log.Println("WARNING: Live preview sandbox enforcement enabled, but Docker preview is unavailable")
		}
	}
	if bundlerStatus, ok := previewFeatureStatus["bundler"].(map[string]interface{}); ok {
		if sandboxRequired, _ := previewFeatureStatus["sandbox_required"].(bool); sandboxRequired {
			if sandboxReady, _ := previewFeatureStatus["sandbox_ready"].(bool); !sandboxReady {
				startupRegistry.MarkDegraded("preview_service", startup.TierOptional, "Live preview sandbox required but unavailable", previewFeatureStatus)
			} else if available, ok := bundlerStatus["available"].(bool); ok && !available {
				startupRegistry.MarkDegraded("preview_service", startup.TierOptional, "Preview service initialized with bundler unavailable", previewFeatureStatus)
			} else {
				startupRegistry.MarkReady("preview_service", startup.TierOptional, "Live preview service initialized", previewFeatureStatus)
			}
		} else if available, ok := bundlerStatus["available"].(bool); ok && !available {
			startupRegistry.MarkDegraded("preview_service", startup.TierOptional, "Preview service initialized with bundler unavailable", previewFeatureStatus)
		} else {
			startupRegistry.MarkReady("preview_service", startup.TierOptional, "Live preview service initialized", previewFeatureStatus)
		}
	} else {
		startupRegistry.MarkReady("preview_service", startup.TierOptional, "Live preview service initialized", previewFeatureStatus)
	}

	// Initialize Git Integration Service
	gitService := git.NewGitService(database.GetDB())
	gitHandler := handlers.NewGitHandler(database.GetDB(), gitService, secretsManager)

	log.Println("Git Integration initialized (GitHub support)")
	startupRegistry.MarkReady("git_integration", startup.TierOptional, "Git integration initialized", nil)

	// Initialize GitHub Import Handler (one-click repo import like replit.new)
	importHandler := handlers.NewImportHandler(database.GetDB(), gitService, secretsManager)
	log.Println("GitHub Import Wizard initialized (one-click repo import)")
	startupRegistry.MarkReady("github_import", startup.TierOptional, "GitHub import initialized", nil)

	// Initialize GitHub Export Handler (export projects to GitHub repos)
	exportHandler := handlers.NewExportHandler(database.GetDB(), gitService, secretsManager)
	log.Println("GitHub Export initialized (one-click export to GitHub)")
	startupRegistry.MarkReady("github_export", startup.TierOptional, "GitHub export initialized", nil)

	// Initialize Version History Handler (Replit parity feature)
	versionHandler := handlers.NewVersionHandler(database.GetDB())
	log.Println("Version History System initialized (diff viewing, restore, pinning)")
	startupRegistry.MarkReady("version_history", startup.TierOptional, "Version history initialized", nil)

	// Initialize Code Comments Handler (Replit parity feature)
	commentsHandler := handlers.NewCommentsHandler(database.GetDB())
	log.Println("Code Comments System initialized (inline threads, reactions, resolve)")
	startupRegistry.MarkReady("code_comments", startup.TierOptional, "Code comments initialized", nil)

	// Initialize Stripe Payment Service with validated key
	// SECURITY: Use validated key from secretsConfig
	stripeSecretKey := secretsConfig.StripeSecretKey
	paymentHandler := handlers.NewPaymentHandlers(database.GetDB(), stripeSecretKey)

	if stripeSecretKey != "" && stripeSecretKey != "sk_test_xxx" {
		log.Println("Stripe Payment Integration initialized")
		log.Printf("   - Plans: Free, Builder ($19/mo), Pro ($49/mo), Team ($99/mo), Enterprise (contact sales)")
		startupRegistry.MarkReady("payments", startup.TierOptional, "Stripe payment integration initialized", map[string]any{
			"enabled": true,
		})
	} else {
		log.Println("WARNING: Stripe not configured - payment features disabled")
		log.Println("   Set STRIPE_SECRET_KEY and STRIPE_WEBHOOK_SECRET to enable")
		startupRegistry.MarkDegraded("payments", startup.TierOptional, "Stripe not configured; payment features disabled", map[string]any{
			"enabled": false,
		})
	}

	// Log available plans
	plans := payments.GetAllPlans()
	log.Printf("Subscription Plans configured: %d plans available", len(plans))

	// Initialize WebSocket Hub for real-time updates
	// PERFORMANCE: Using BatchedHub for 70% message reduction via 50ms batching
	wsHubRT := websocket.NewBatchedHub()
	go wsHubRT.Run()
	log.Println("WebSocket BatchedHub initialized (50ms batching, 16ms write coalescing)")
	startupRegistry.MarkReady("realtime_updates", startup.TierOptional, "Realtime websocket hub initialized", nil)

	// Initialize cache for performance optimization
	// PERFORMANCE: 30s TTL cache with in-memory fallback when Redis unavailable
	cacheConfig := cache.DefaultCacheConfig()
	redisURL := os.Getenv("REDIS_URL")
	var redisCache *cache.RedisCache
	if redisURL != "" {
		log.Printf("Redis cache connecting to: %s", redisURL)
		redisCache = cache.NewRedisCacheFromURL(redisURL, cacheConfig)
		log.Println("Redis cache initialized (falls back to in-memory on connection failure)")
	} else {
		log.Println("WARNING: REDIS_URL not set - using in-memory cache (set for production)")
		redisCache = cache.NewRedisCache(cacheConfig)
	}
	if redisCache != nil {
		spendTracker.SetCache(redisCache)
	}
	cacheStatus := redisCache.Status()
	cacheDetails := map[string]any{
		"backend":              cacheStatus.Backend,
		"redis_url_configured": redisURL != "",
		"redis_connected":      cacheStatus.RedisConnected,
	}
	if cacheStatus.FallbackReason != "" {
		cacheDetails["fallback_reason"] = cacheStatus.FallbackReason
	}
	if cacheStatus.RecommendedFix != "" {
		cacheDetails["recommended_fix"] = cacheStatus.RecommendedFix
		log.Printf("Redis cache remediation: %s", cacheStatus.RecommendedFix)
	}
	if redisCache.HasRedisBackend() {
		startupRegistry.MarkReady("redis_cache", startup.TierOptional, "Redis cache backend connected", map[string]any{
			"backend":         "redis",
			"redis_connected": cacheStatus.RedisConnected,
		})
	} else {
		startupRegistry.MarkDegraded("redis_cache", startup.TierOptional, "Using in-memory cache fallback", cacheDetails)
	}

	// Initialize base Handler for dependent handlers
	// Note: BatchedHub embeds *Hub, so we pass the embedded Hub for Handler compatibility
	baseHandler := handlers.NewHandler(database.GetDB(), aiRouter, authService, wsHubRT.Hub)
	baseHandler.SpendTracker = spendTracker

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
	executionTier := startup.TierOptional

	executionHandler, err := handlers.NewExecutionHandlerWithConfig(database.GetDB(), executionConfig)
	if err != nil {
		log.Printf("WARNING: Failed to initialize execution handler: %v", err)
		log.Println("SECURITY: Code execution features are DISABLED")
		startupRegistry.MarkDegraded("code_execution", executionTier, "Code execution disabled", map[string]any{
			"error": err.Error(),
		})
	} else {
		log.Println("Code Execution Engine initialized (10+ languages supported)")
		log.Println("   - Languages: JavaScript, TypeScript, Python, Go, Rust, Java, C, C++, Ruby, PHP")

		// Log sandbox status
		sandboxStatus := executionHandler.GetSandboxStatus()

		// Check for E2B first (highest security)
		if e2bAvail, ok := sandboxStatus["e2b_available"].(bool); ok && e2bAvail {
			log.Println("EXECUTION: Using E2B managed sandboxes (Docker-free remote execution)")
			log.Println("   - Managed microVMs via E2B API")
			log.Println("   - Full isolation and security")
			log.Println("   - No local Docker required")
			startupRegistry.MarkReady("code_execution", executionTier, "E2B sandbox enabled", sandboxStatus)
		} else if containerAvail, ok := sandboxStatus["container_available"].(bool); ok && containerAvail {
			log.Println("SECURITY: Docker container sandboxing ENABLED")
			log.Println("   - Seccomp syscall filtering: enabled")
			log.Println("   - Network isolation: enabled by default")
			log.Println("   - Memory limit: 256MB default")
			log.Println("   - CPU limit: 0.5 cores default")
			log.Println("   - Read-only root filesystem: enabled")
			startupRegistry.MarkReady("code_execution", executionTier, "Container sandbox enabled", sandboxStatus)
		} else {
			if executionConfig.ForceContainer {
				log.Println("WARNING: Docker required but not available - execution DISABLED")
				startupRegistry.MarkDegraded("code_execution", executionTier, "Code execution unavailable because the required container sandbox is missing", sandboxStatus)
			} else {
				log.Println("WARNING: Docker not available - using process-based sandbox (less secure)")
				log.Println("WARNING: Set EXECUTION_FORCE_CONTAINER=true to require Docker in production")
				log.Println("WARNING: Set E2B_API_KEY=xxx to enable managed sandboxes without Docker")
				startupRegistry.MarkDegraded("code_execution", executionTier, "Execution running without container sandbox", sandboxStatus)
			}
		}
	}

	// Initialize One-Click Deployment Service
	vercelToken := os.Getenv("VERCEL_TOKEN")
	netlifyToken := os.Getenv("NETLIFY_TOKEN")
	renderToken := os.Getenv("RENDER_TOKEN")
	railwayToken := os.Getenv("RAILWAY_TOKEN")
	railwayWorkspace := os.Getenv("RAILWAY_WORKSPACE")
	cloudflarePagesToken := os.Getenv("CLOUDFLARE_API_TOKEN")
	cloudflareAccountID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	neonToken := os.Getenv("NEON_API_KEY")
	neonOrgID := os.Getenv("NEON_ORG_ID")
	deployService := deploy.NewDeploymentService(database.GetDB(), vercelToken, netlifyToken, renderToken, railwayToken, cloudflarePagesToken)

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
	if railwayToken != "" && providers.BinaryAvailable("railway") {
		deployService.RegisterProvider(deploy.ProviderRailway, providers.NewRailwayProvider(railwayToken, railwayWorkspace))
	}
	if cloudflarePagesToken != "" && cloudflareAccountID != "" && providers.BinaryAvailable("wrangler") {
		deployService.RegisterProvider(deploy.ProviderCloudflarePages, providers.NewCloudflarePagesProvider(cloudflarePagesToken, cloudflareAccountID))
	}
	if neonToken != "" {
		deployService.RegisterDatabaseProvisioner(deploy.DatabaseProviderNeon, deploy.NewNeonDatabaseProvisioner(neonToken, neonOrgID))
	}

	deployHandler := handlers.NewDeployHandler(database.GetDB(), deployService)
	log.Println("One-Click Deployment initialized (Vercel, Netlify, Render, Railway, Cloudflare Pages, Neon orchestration)")
	availableDeployProviders := make([]string, 0, 5)
	if vercelToken != "" {
		availableDeployProviders = append(availableDeployProviders, string(deploy.ProviderVercel))
	}
	if netlifyToken != "" {
		availableDeployProviders = append(availableDeployProviders, string(deploy.ProviderNetlify))
	}
	if renderToken != "" {
		availableDeployProviders = append(availableDeployProviders, string(deploy.ProviderRender))
	}
	if railwayToken != "" && providers.BinaryAvailable("railway") {
		availableDeployProviders = append(availableDeployProviders, string(deploy.ProviderRailway))
	}
	if cloudflarePagesToken != "" && cloudflareAccountID != "" && providers.BinaryAvailable("wrangler") {
		availableDeployProviders = append(availableDeployProviders, string(deploy.ProviderCloudflarePages))
	}
	if len(availableDeployProviders) == 0 {
		startupRegistry.MarkDegraded("deployment_providers", startup.TierOptional, "No deployment providers configured", map[string]any{
			"providers": availableDeployProviders,
		})
	} else {
		startupRegistry.MarkReady("deployment_providers", startup.TierOptional, "Deployment providers registered", map[string]any{
			"providers": availableDeployProviders,
		})
	}
	if neonToken == "" {
		startupRegistry.MarkDegraded("deployment_databases", startup.TierOptional, "Neon orchestration is not configured", map[string]any{
			"providers": []string{},
		})
	} else {
		startupRegistry.MarkReady("deployment_databases", startup.TierOptional, "Managed deployment database orchestration initialized", map[string]any{
			"providers": []string{string(deploy.DatabaseProviderNeon)},
			"org_id":    neonOrgID != "",
		})
	}

	// Initialize Package Manager
	packageHandler := handlers.NewPackageHandler(baseHandler)
	log.Println("Package Manager initialized (NPM, PyPI, Go Modules)")
	startupRegistry.MarkReady("package_management", startup.TierOptional, "Package management initialized", nil)

	// Initialize Environment Handler (Nix-like reproducible environments - Replit parity)
	environmentHandler := handlers.NewEnvironmentHandler(baseHandler)
	log.Println("Environment Configuration initialized (Nix-like reproducible environments)")
	startupRegistry.MarkReady("environment_configs", startup.TierOptional, "Environment configuration initialized", nil)

	// Initialize Community/Sharing Marketplace
	communityHandler := community.NewCommunityHandler(database.GetDB())
	communityHealthy := true

	// Run community migrations
	if err := community.AutoMigrate(database.GetDB()); err != nil {
		communityHealthy = false
		log.Printf("WARNING: Community migration had issues: %v", err)
	}

	// Seed default categories
	if err := community.SeedCategories(database.GetDB()); err != nil {
		communityHealthy = false
		log.Printf("WARNING: Category seeding had issues: %v", err)
	}
	if communityHealthy {
		startupRegistry.MarkReady("community_marketplace", startup.TierOptional, "Community marketplace initialized", nil)
	} else {
		startupRegistry.MarkDegraded("community_marketplace", startup.TierOptional, "Community marketplace initialized with degraded setup", nil)
	}

	log.Println("Community Marketplace initialized (discover, share, fork projects)")

	// Initialize Native Hosting Service
	hostingService := hosting.NewHostingService(database.GetDB())
	hostingHandler := handlers.NewHostingHandler(database.GetDB(), hostingService)
	log.Println("Native Hosting (.apex.app) initialized")
	startupRegistry.MarkReady("native_hosting", startup.TierOptional, "Native hosting service initialized", nil)

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
	startupRegistry.MarkReady("always_on_controller", startup.TierOptional, "Always-on deployment controller started", nil)

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
		startupRegistry.MarkDegraded("managed_databases", startup.TierOptional, "Managed database service unavailable", map[string]any{
			"error": err.Error(),
		})
	} else {
		databaseHandler = handlers.NewDatabaseHandler(database.GetDB(), dbManager, secretsManager)
		// Initialize auto-provisioning dependencies for project creation
		handlers.InitAutoProvisioningDeps(dbManager, secretsManager)
		log.Println("Managed Database Service initialized (PostgreSQL, Redis, SQLite)")
		log.Println("Auto-Provision PostgreSQL enabled for new projects")
		startupRegistry.MarkReady("managed_databases", startup.TierOptional, "Managed database service initialized", nil)
	}

	// Initialize Debugging Service
	debugService := debugging.NewDebugService(database.GetDB())
	debuggingHandler := handlers.NewDebuggingHandler(database.GetDB(), debugService)
	log.Println("Debugging Service initialized (breakpoints, stepping, watch expressions)")
	startupRegistry.MarkReady("debugging_service", startup.TierOptional, "Debugging service initialized", nil)

	// Initialize AI Completions Service
	completionService := completions.NewCompletionService(database.GetDB(), aiRouter, byokManager)
	completionsHandler := handlers.NewCompletionsHandler(completionService)
	log.Println("AI Completions Service initialized (inline ghost-text, multi-provider)")
	startupRegistry.MarkReady("completions_service", startup.TierOptional, "AI completions service initialized", nil)

	// Initialize Extensions Marketplace
	extensionService := extensions.NewService(database.GetDB())
	extensionsHandler := handlers.NewExtensionsHandler(extensionService)
	// Run extensions migrations
	if err := database.GetDB().AutoMigrate(
		&extensions.Extension{},
		&extensions.ExtensionVersion{},
		&extensions.ExtensionReview{},
		&extensions.UserExtension{},
	); err != nil {
		startupRegistry.MarkDegraded("extensions_marketplace", startup.TierOptional, "Extensions migrations completed with warnings", map[string]any{
			"error": err.Error(),
		})
	} else {
		startupRegistry.MarkReady("extensions_marketplace", startup.TierOptional, "Extensions marketplace initialized", nil)
	}
	log.Println("Extensions Marketplace initialized (discover, install, publish)")

	// Initialize Enterprise Services (SAML SSO, SCIM, RBAC, Audit)
	auditService := enterprise.NewAuditService(database.GetDB())
	rbacService := enterprise.NewRBACService(database.GetDB())

	baseURL := getEnv("BASE_URL", "https://apex-build.dev")
	samlConfig := &enterprise.ServiceProviderConfig{
		EntityID:                    baseURL,
		AssertionConsumerServiceURL: baseURL + "/api/v1/enterprise/sso/callback",
		SingleLogoutServiceURL:      baseURL + "/api/v1/enterprise/sso/logout",
	}
	samlService := enterprise.NewSAMLService(database.GetDB(), samlConfig, auditService)
	scimService := enterprise.NewSCIMService(database.GetDB(), auditService, rbacService)
	enterpriseHandler := handlers.NewEnterpriseHandler(database.GetDB(), samlService, scimService, auditService, rbacService)

	// Run enterprise migrations
	if err := database.GetDB().AutoMigrate(
		&enterprise.Organization{},
		&enterprise.OrganizationMember{},
		&enterprise.Role{},
		&enterprise.Permission{},
		&enterprise.AuditLog{},
		&enterprise.RateLimit{},
		&enterprise.Invitation{},
	); err != nil {
		startupRegistry.MarkDegraded("enterprise_features", startup.TierOptional, "Enterprise migrations completed with warnings", map[string]any{
			"error": err.Error(),
		})
	} else {
		startupRegistry.MarkReady("enterprise_features", startup.TierOptional, "Enterprise features initialized", nil)
	}
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
	startupRegistry.MarkReady("autonomous_agent", startup.TierOptional, "Autonomous agent system initialized", nil)

	// Initialize Real-Time Collaboration Hub
	collabHub := collaboration.NewCollabHub()
	collabAccessor := collaboration.NewDatabaseAdapter(database.GetDB())
	collabHub.SetAccessResolver(collabAccessor.ResolveProjectAccess)
	collabHub.SetFileStore(collabAccessor)
	go collabHub.Run()
	log.Println("Real-Time Collaboration initialized (OT, presence, cursor tracking)")
	startupRegistry.MarkReady("collaboration", startup.TierOptional, "Real-time collaboration hub started", nil)
	collaborationHandler := handlers.NewCollaborationHandler(collabHub, collabAccessor.ResolveProjectAccess)

	// Initialize Key Rotation Handler (admin-only)
	rotationHandler := handlers.NewRotationHandler(database.GetDB())
	log.Println("Key Rotation Handler initialized (admin-only)")
	startupRegistry.MarkReady("admin_controls", startup.TierOptional, "Admin controls initialized", nil)

	// Initialize Usage Tracker for quota enforcement (REVENUE PROTECTION)
	usageTracker := usage.NewTracker(database.GetDB(), redisCache)
	if err := usageTracker.Migrate(); err != nil {
		startupRegistry.MarkDegraded("usage_tracking", startup.TierOptional, "Usage tracker migration completed with warnings", map[string]any{
			"error": err.Error(),
		})
		log.Printf("WARNING: Usage tracker migration had issues: %v", err)
	} else {
		startupRegistry.MarkReady("usage_tracking", startup.TierOptional, "Usage tracking initialized", nil)
	}
	usageHandler := handlers.NewUsageHandlers(database.GetDB(), usageTracker)
	quotaChecker := middleware.NewQuotaChecker(usageTracker)
	completionService.SetUsageTracker(usageTracker)
	if executionHandler != nil {
		executionHandler.SetUsageTracker(usageTracker)
	}
	log.Println("Usage Tracking & Quota Enforcement initialized (projects, storage, AI, execution)")
	log.Println("   - Free: 3 projects, 100MB storage, 1000 AI/month, 10 exec min/day")
	log.Println("   - Builder ($19/mo): unlimited projects, 5GB storage, credit-based AI, 240 exec min/day")
	log.Println("   - Pro ($49/mo): unlimited projects, 20GB storage, credit-based AI, 720 exec min/day")
	log.Println("   - Team ($99/mo): unlimited projects, 100GB storage, credit-based AI, 1440 exec min/day")
	log.Println("   - Enterprise: unlimited")

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
		startupRegistry.MarkReady("metrics", startup.TierOptional, "Prometheus metrics initialized", nil)
	} else {
		startupRegistry.MarkDegraded("metrics", startup.TierOptional, "Prometheus metrics disabled by configuration", nil)
	}

	// Initialize API server
	server := api.NewServer(database, authService, aiRouter, byokManager)
	server.SetReadinessRegistry(startupRegistry)
	server.SetUsageTracker(usageTracker)
	server.SetCacheStatusProvider(redisCache.Status)
	server.SetMobileBuildService(mobile.NewMobileBuildService(
		mobile.LoadFeatureFlagsFromEnv(),
		nil,
		mobile.NewGormMobileBuildStore(database.GetDB()),
	))

	// Initialize Email Service (SMTP transactional email for verification codes etc.)
	emailSvc := email.NewService()
	server.SetEmailService(emailSvc)

	// Initialize Storage Provider (R2 or local fallback)
	storageProvider := storage.NewFromEnv()
	server.SetStorageProvider(storageProvider)

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
		completionsHandler, extensionsHandler, enterpriseHandler, collabHub, collaborationHandler,
		optimizedHandler,
		byokHandler,           // BYOK API key management and model selection
		exportHandler,         // GitHub export (push projects to GitHub)
		usageHandler,          // Usage tracking and quota API endpoints
		quotaChecker,          // Quota enforcement middleware
		rotationHandler,       // Key rotation (admin)
		spendHandler,          // Spend tracking dashboard
		budgetHandler,         // Budget caps enforcement
		budgetMiddleware,      // Budget enforcement middleware
		protectedPathsHandler, // Protected paths management
	)

	// Activate the full router now that all services are initialized.
	activeRouter.Store(router)
	startupRegistry.MarkReady("http_routes", startup.TierCritical, "Application router activated", map[string]any{
		"port": port,
	})
	startupRegistry.SetPhase(startup.PhaseReady)
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
		startupRegistry.MarkFailed("http_routes", startup.TierCritical, "HTTP server stopped unexpectedly", map[string]any{
			"error": err.Error(),
		})
		startupRegistry.SetPhase(startup.PhaseFailed)
		log.Fatalf("CRITICAL: Failed to start server: %v", err)
	case sig := <-quit:
		startupRegistry.SetPhase(startup.PhaseShuttingDown)
		log.Printf("Received signal %v, starting graceful shutdown...", sig)
	}

	// Give in-flight requests time to complete (default 30s, configurable)
	shutdownTimeout := 30 * time.Second
	if t := os.Getenv("GRACEFUL_SHUTDOWN_TIMEOUT"); t != "" {
		if d, err := time.ParseDuration(t); err == nil {
			shutdownTimeout = d
		}
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
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

func newStartupRegistry() *startup.Registry {
	registry := startup.NewRegistry()
	registry.Register("bootstrap_http", startup.TierCritical, "Waiting for bootstrap health listener", nil)
	registry.Register("secrets_validation", startup.TierCritical, "Waiting for secrets validation", nil)
	registry.Register("primary_database", startup.TierCritical, "Waiting for database connection", nil)
	registry.Register("auth_service", startup.TierCritical, "Waiting for authentication service", nil)
	registry.Register("secrets_manager", startup.TierCritical, "Waiting for secrets manager", nil)
	registry.Register("http_routes", startup.TierCritical, "Waiting for application router activation", nil)
	registry.Register("database_seeding", startup.TierOptional, "Waiting for database seeding", nil)
	registry.Register("ai_platform", startup.TierOptional, "Waiting for AI router initialization", nil)
	registry.Register("spend_tracking", startup.TierOptional, "Waiting for spend tracking initialization", nil)
	registry.Register("budget_enforcement", startup.TierOptional, "Waiting for budget enforcement initialization", nil)
	registry.Register("protected_paths", startup.TierOptional, "Waiting for protected paths initialization", nil)
	registry.Register("project_secrets", startup.TierOptional, "Waiting for project secrets handlers", nil)
	registry.Register("mcp", startup.TierOptional, "Waiting for MCP handlers", nil)
	registry.Register("project_templates", startup.TierOptional, "Waiting for project templates", nil)
	registry.Register("code_search", startup.TierOptional, "Waiting for code search initialization", nil)
	registry.Register("byok", startup.TierOptional, "Waiting for BYOK manager", nil)
	registry.Register("agent_orchestration", startup.TierOptional, "Waiting for agent orchestration services", nil)
	registry.Register("preview_service", startup.TierOptional, "Waiting for live preview service", nil)
	registry.Register("git_integration", startup.TierOptional, "Waiting for Git integration", nil)
	registry.Register("github_import", startup.TierOptional, "Waiting for GitHub import initialization", nil)
	registry.Register("github_export", startup.TierOptional, "Waiting for GitHub export initialization", nil)
	registry.Register("version_history", startup.TierOptional, "Waiting for version history initialization", nil)
	registry.Register("code_comments", startup.TierOptional, "Waiting for code comments initialization", nil)
	registry.Register("payments", startup.TierOptional, "Waiting for payment provider setup", nil)
	registry.Register("realtime_updates", startup.TierOptional, "Waiting for realtime websocket hub", nil)
	registry.Register("redis_cache", startup.TierOptional, "Waiting for cache backend", nil)
	registry.Register("code_execution", startup.TierOptional, "Waiting for code execution sandbox", nil)
	registry.Register("deployment_providers", startup.TierOptional, "Waiting for deployment provider registration", nil)
	registry.Register("deployment_databases", startup.TierOptional, "Waiting for deployment database orchestration", nil)
	registry.Register("package_management", startup.TierOptional, "Waiting for package management initialization", nil)
	registry.Register("environment_configs", startup.TierOptional, "Waiting for environment configuration initialization", nil)
	registry.Register("community_marketplace", startup.TierOptional, "Waiting for community marketplace initialization", nil)
	registry.Register("native_hosting", startup.TierOptional, "Waiting for native hosting service", nil)
	registry.Register("always_on_controller", startup.TierOptional, "Waiting for always-on controller", nil)
	registry.Register("managed_databases", startup.TierOptional, "Waiting for managed database service", nil)
	registry.Register("debugging_service", startup.TierOptional, "Waiting for debugging service", nil)
	registry.Register("completions_service", startup.TierOptional, "Waiting for completions service", nil)
	registry.Register("extensions_marketplace", startup.TierOptional, "Waiting for extensions marketplace", nil)
	registry.Register("enterprise_features", startup.TierOptional, "Waiting for enterprise features", nil)
	registry.Register("autonomous_agent", startup.TierOptional, "Waiting for autonomous agent system", nil)
	registry.Register("collaboration", startup.TierOptional, "Waiting for collaboration hub", nil)
	registry.Register("admin_controls", startup.TierOptional, "Waiting for admin controls initialization", nil)
	registry.Register("usage_tracking", startup.TierOptional, "Waiting for usage tracker", nil)
	registry.Register("metrics", startup.TierOptional, "Waiting for metrics subsystem", nil)
	return registry
}

func featureReadinessHTTPStatus(summary startup.Summary) int {
	if summary.Ready {
		return http.StatusOK
	}
	return http.StatusServiceUnavailable
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
	OllamaBaseURL string // Ollama server URL; local (http://localhost:11434) or cloud (https://ollama.com/v1)
	OllamaAPIKey  string // Ollama Pro cloud API key; leave empty for local installs

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
		OllamaBaseURL: getEnv("OLLAMA_BASE_URL", ""),
		OllamaAPIKey:  getEnv("OLLAMA_API_KEY", ""),
		JWTSecret:     jwtSecret,
		Port:          getEnv("PORT", "8080"),
		Environment:   environment,
	}
}

func resolvePreviewRuntimeVerify(chromePath string) (bool, string) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("APEX_PREVIEW_RUNTIME_VERIFY"))) {
	case "1", "true", "yes", "on":
		return true, "explicit_enabled"
	case "0", "false", "no", "off":
		return false, "explicit_disabled"
	}

	if config.IsProductionEnvironment() {
		if chromePath != "" {
			return true, "production_default"
		}
		return false, "production_no_chrome"
	}

	return false, "non_production_default"
}

func configureTrustedClientIP(router *gin.Engine) {
	if router == nil {
		return
	}

	router.ForwardedByClientIP = true
	router.RemoteIPHeaders = []string{"CF-Connecting-IP", "X-Forwarded-For", "X-Real-IP"}
	if platform := strings.TrimSpace(os.Getenv("GIN_TRUSTED_PLATFORM")); platform != "" {
		router.TrustedPlatform = platform
	} else {
		router.TrustedPlatform = "CF-Connecting-IP"
	}

	raw := strings.TrimSpace(firstNonEmpty(
		os.Getenv("GIN_TRUSTED_PROXIES"),
		os.Getenv("TRUSTED_PROXIES"),
	))
	proxies := splitCSV(raw)
	if len(proxies) == 0 && config.IsProductionEnvironment() {
		proxies = defaultProductionTrustedProxies()
	}
	if len(proxies) == 0 {
		return
	}
	if err := router.SetTrustedProxies(proxies); err != nil {
		log.Printf("WARNING: invalid trusted proxy configuration: %v", err)
	}
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func defaultProductionTrustedProxies() []string {
	return []string{
		"127.0.0.1",
		"::1",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"173.245.48.0/20",
		"103.21.244.0/22",
		"103.22.200.0/22",
		"103.31.4.0/22",
		"141.101.64.0/18",
		"108.162.192.0/18",
		"190.93.240.0/20",
		"188.114.96.0/20",
		"197.234.240.0/22",
		"198.41.128.0/17",
		"162.158.0.0/15",
		"104.16.0.0/13",
		"104.24.0.0/14",
		"172.64.0.0/13",
		"131.0.72.0/22",
		"2400:cb00::/32",
		"2606:4700::/32",
		"2803:f800::/32",
		"2405:b500::/32",
		"2405:8100::/32",
		"2a06:98c0::/29",
		"2c0f:f248::/32",
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
	collabHub *collaboration.CollabHub, collaborationHandler *handlers.CollaborationHandler,
	optimizedHandler *handlers.OptimizedHandler, // PERFORMANCE: Optimized handlers with caching
	byokHandler *handlers.BYOKHandlers, // BYOK API key management
	exportHandler *handlers.ExportHandler, // GitHub export
	usageHandler *handlers.UsageHandlers, // Usage tracking and quota API
	quotaChecker *middleware.QuotaChecker, // Quota enforcement middleware
	rotationHandler *handlers.RotationHandler, // Key rotation (admin)
	spendHandler *handlers.SpendHandler, // Spend tracking dashboard
	budgetHandler *handlers.BudgetHandler, // Budget caps enforcement
	budgetMiddleware gin.HandlerFunc, // Budget enforcement middleware
	protectedPathsHandler *handlers.ProtectedPathsHandler, // Protected paths management
) *gin.Engine {
	// Set gin mode based on environment
	if os.Getenv("ENVIRONMENT") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	configureTrustedClientIP(router)

	// Add middleware
	router.Use(server.CORSMiddleware())
	router.Use(gin.Recovery())
	router.Use(middleware.SecurityHeaders())

	// Add Prometheus metrics middleware (if enabled)
	if getEnv("ENABLE_METRICS", "true") == "true" {
		router.Use(metrics.PrometheusMiddleware())
		// Metrics endpoint (Prometheus format) — requires METRICS_AUTH_TOKEN bearer token
		metricsToken := getEnv("METRICS_AUTH_TOKEN", "")
		router.GET("/metrics", func(c *gin.Context) {
			if metricsToken != "" {
				auth := c.GetHeader("Authorization")
				if auth != "Bearer "+metricsToken {
					c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
					return
				}
			}
			metrics.PrometheusHandler()(c)
		})
	}

	// Health check endpoints
	router.GET("/health", server.Health)
	router.GET("/health/deep", server.DeepHealth)
	router.GET("/health/features", server.FeatureReadiness)
	router.GET("/ready", server.DeepHealth) // Kubernetes readiness probe

	// API documentation endpoint
	router.GET("/docs", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"name":        "APEX.BUILD API",
			"version":     "1.0.0",
			"description": "Next-generation cloud development platform with multi-AI integration",
			"features": []string{
				"Multi-AI integration (Claude, OpenAI, Gemini)",
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
		// Frontend API clients are rooted at /api/v1 in production. Keep the
		// root health endpoints for infrastructure probes, but mirror the
		// readiness endpoints here so app-shell health checks do not 404.
		v1.GET("/health", server.Health)
		v1.GET("/health/deep", server.DeepHealth)
		v1.GET("/health/features", server.FeatureReadiness)

		// Authentication routes (no auth required, but rate limited)
		// SECURITY: Stricter rate limit (10 req/min) to prevent brute force attacks
		auth := v1.Group("/auth")
		auth.Use(middleware.AuthRateLimit())
		{
			auth.POST("/register", server.Register)
			auth.POST("/login", server.Login)
			auth.POST("/refresh", server.RefreshToken)
			auth.POST("/logout", server.Logout)
			// Email verification (authenticated path requires Bearer/cookie; unauthenticated path uses body email)
			auth.POST("/verify-email", server.VerifyEmail)
			auth.POST("/resend-verification", server.ResendVerification)
		}

		// Community/Sharing Marketplace public endpoints (no auth required for viewing)
		communityHandler.RegisterRoutes(v1)

		// Preview proxy endpoints (token-auth via query param for iframe embedding)
		previewProxy := v1.Group("/preview")
		{
			previewProxy.Any("/proxy/:projectId", previewHandler.ProxyPreview)
			previewProxy.Any("/proxy/:projectId/*path", previewHandler.ProxyPreview)
			// Backend proxy: routes fetch() calls from the preview frontend to the running backend
			previewProxy.Any("/backend-proxy/:projectId", previewHandler.ProxyBackend)
			previewProxy.Any("/backend-proxy/:projectId/*path", previewHandler.ProxyBackend)
		}

		// Stripe webhook — must be unauthenticated (Stripe sends this, not users)
		// Raw body is required for signature verification — do NOT add body parsers here
		v1.POST("/billing/webhook", paymentHandler.HandleWebhook)

		// Build canary poll endpoint. Authenticated build/detail/preview routes
		// remain protected; this route accepts only per-build read-only tokens.
		buildHandler.RegisterPublicRoutes(v1)

		// CSRF token endpoint — public GET, issues a time-limited HMAC token.
		// The frontend fetches this once and attaches it as X-CSRF-Token on all
		// state-mutating requests to the protected group.
		v1.GET("/csrf-token", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"token": middleware.GenerateCSRFToken()})
		})

		// Protected routes (authentication required)
		protected := v1.Group("/")
		protected.Use(server.AuthMiddleware())
		protected.Use(middleware.CSRFProtection())
		{
			// Usage tracking and quota API endpoints (REVENUE PROTECTION)
			usageHandler.RegisterUsageRoutes(protected)

			// AI endpoints - with quota and budget enforcement
			ai := protected.Group("/ai")
			ai.Use(quotaChecker.CheckAIQuota()) // Enforce AI request quota
			ai.Use(budgetMiddleware)            // Enforce budget caps
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
				projects.GET("/:id/mobile/validation", server.GetProjectMobileValidation)
				projects.GET("/:id/mobile/scorecard", server.GetProjectMobileScorecard)
				projects.GET("/:id/mobile/builds", server.ListProjectMobileBuilds)
				projects.POST("/:id/mobile/builds", server.CreateProjectMobileBuild)
				projects.GET("/:id/mobile/builds/:buildId", server.GetProjectMobileBuild)
				projects.GET("/:id/mobile/builds/:buildId/logs", server.GetProjectMobileBuildLogs)
				projects.GET("/:id/mobile/builds/:buildId/artifacts", server.GetProjectMobileBuildArtifacts)

				// File endpoints under projects - using optimized handler
				// Storage quota checked on file creation
				projects.POST("/:id/files", quotaChecker.CheckStorageQuota(1024*1024), server.CreateFile) // Estimate 1MB
				projects.GET("/:id/files", optimizedHandler.GetProjectFilesOptimized)                     // Optimized: no content loading for list

				// Asset upload endpoints — users upload images, CSVs, PDFs etc for AI agents to use
				projects.POST("/:id/assets", server.UploadAsset)
				projects.GET("/:id/assets", server.ListAssets)
				projects.DELETE("/:id/assets/:assetId", server.DeleteAsset)

				// Protected Paths (A3)
				protectedPathsHandler.RegisterProtectedPathsRoutes(projects)
			}

			// Asset serving endpoint (for local storage)
			assets := v1.Group("/assets")
			{
				assets.GET("/raw/*key", server.ServeAsset)
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
			buildHandler.RegisterCleanupRoutes(protected)

			// Autonomous Agent endpoints (AI-driven build, test, deploy)
			autonomousHandler.RegisterRoutes(protected)

			// Secrets Management endpoints
			if secretsHandler != nil {
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
				protected.GET("/projects/:id/mobile/credentials", secretsHandler.ListProjectMobileCredentials)
				protected.POST("/projects/:id/mobile/credentials", secretsHandler.CreateProjectMobileCredential)
				protected.DELETE("/projects/:id/mobile/credentials/:type", secretsHandler.DeleteProjectMobileCredential)
			} else {
				registerUnavailableRoutes(protected, "/secrets", "Secrets management is currently unavailable")
				protected.GET("/projects/:id/secrets", featureUnavailableHandler("Secrets management is currently unavailable"))
				protected.GET("/projects/:id/mobile/credentials", featureUnavailableHandler("Secrets management is currently unavailable"))
				protected.POST("/projects/:id/mobile/credentials", featureUnavailableHandler("Secrets management is currently unavailable"))
				protected.DELETE("/projects/:id/mobile/credentials/:type", featureUnavailableHandler("Secrets management is currently unavailable"))
			}

			// MCP Server Management endpoints
			if mcpHandler != nil {
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
			} else {
				registerUnavailableRoutes(protected, "/mcp", "MCP integrations are currently unavailable")
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
				billing.POST("/change-plan", paymentHandler.ChangePlan)            // Upgrade or downgrade plan
				billing.POST("/cancel", paymentHandler.CancelSubscription)         // Cancel subscription
				billing.POST("/reactivate", paymentHandler.ReactivateSubscription) // Reactivate subscription
				billing.GET("/invoices", paymentHandler.GetInvoices)               // Get invoice history
				billing.GET("/payment-methods", paymentHandler.GetPaymentMethods)  // Get payment methods
				billing.GET("/check-limit/:type", paymentHandler.CheckUsageLimit)  // Check usage limit
				billing.GET("/config-status", paymentHandler.StripeConfigStatus)   // Check Stripe config
				billing.POST("/credits/purchase", paymentHandler.PurchaseCredits)  // Buy AI credits (one-time)
				billing.GET("/credits/balance", paymentHandler.GetCreditBalance)   // Get current credit balance
				billing.GET("/credits/ledger", paymentHandler.GetCreditLedger)     // Paginated credit history
			}

			// Code Execution endpoints (the core of cloud IDE) - with quota + budget enforcement
			if executionHandler != nil {
				execute := protected.Group("/execute")
				execute.Use(quotaChecker.CheckExecutionQuota(1)) // Check execution minutes quota
				execute.Use(budgetMiddleware)                    // Enforce budget caps
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
			} else {
				registerUnavailableRoutes(protected, "/execute", "Code execution is currently unavailable")
				registerUnavailableRoutes(protected, "/terminal", "Terminal sessions are currently unavailable")
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
			} else {
				registerUnavailableRoutes(protected, "/projects/:id/databases", "Managed databases are currently unavailable")
			}

			// Debugging endpoints
			debuggingHandler.RegisterRoutes(protected)

			// AI Completions endpoints
			completionsHandler.RegisterCompletionRoutes(protected, quotaChecker.CheckAIQuota(), budgetMiddleware)

			// Extensions Marketplace endpoints
			extensionsHandler.RegisterExtensionRoutes(protected)

			// Enterprise endpoints (SSO, RBAC, Audit Logs, Organizations)
			enterpriseHandler.RegisterEnterpriseRoutes(protected, v1)

			// BYOK (Bring Your Own Key) endpoints
			if byokHandler != nil {
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
			} else {
				registerUnavailableRoutes(protected, "/byok", "BYOK key management is currently unavailable")
			}

			// Spend Tracking endpoints (S2: Real-Time Spend Dashboard)
			spendHandler.RegisterRoutes(protected)

			// Budget Caps endpoints (S1: Hard Budget Caps + Instant Stop)
			budgetHandler.RegisterRoutes(protected)

			// Collaboration room bootstrap endpoints for the dedicated /ws/collab service.
			collab := protected.Group("/collab")
			{
				collab.POST("/join/:projectId", collaborationHandler.JoinRoom)
				collab.POST("/leave/:roomId", collaborationHandler.LeaveRoom)
				collab.GET("/users/:roomId", collaborationHandler.GetUsers)
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
				buildHandler.RegisterArchitectureAdminRoutes(admin)
			}
		}
	}

	// WebSocket endpoint for real-time build updates
	router.GET("/ws/build/:buildId", wsHub.HandleWebSocket)

	// WebSocket endpoint for interactive terminal sessions
	if executionHandler != nil {
		router.GET("/ws/terminal/:sessionId", executionHandler.HandleTerminalWebSocket)
	} else {
		router.GET("/ws/terminal/:sessionId", featureUnavailableHandler("Terminal websocket is currently unavailable"))
	}

	// MCP WebSocket endpoint (for APEX.BUILD as MCP server)
	if mcpHandler != nil {
		router.GET("/mcp/ws", mcpHandler.HandleMCPWebSocket)
	} else {
		router.GET("/mcp/ws", featureUnavailableHandler("MCP websocket is currently unavailable"))
	}

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

func featureUnavailableHandler(message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   message,
			"code":    "FEATURE_UNAVAILABLE",
		})
	}
}

func registerUnavailableRoutes(parent *gin.RouterGroup, relativePath, message string) {
	group := parent.Group(relativePath)
	handler := featureUnavailableHandler(message)
	group.Any("", handler)
	group.Any("/*path", handler)
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

func previewRuntimeVerificationEnabled(environment, explicitSetting, chromePath string) bool {
	setting := strings.TrimSpace(explicitSetting)
	if strings.EqualFold(setting, "true") {
		return true
	}
	if strings.EqualFold(setting, "false") {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(environment), "production") && strings.TrimSpace(chromePath) != ""
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

// previewVerifierBridge adapts preview.Verifier to the agents.BuildPreviewVerifier
// interface without creating a circular import between the two packages.
type previewVerifierBridge struct {
	verifier *preview.Verifier
}

func (b *previewVerifierBridge) VerifyBuildFiles(
	ctx context.Context,
	files []agents.VerifiableFile,
	isFullStack bool,
) *agents.PreviewVerificationResult {
	pvFiles := make([]preview.VerifiableFile, len(files))
	for i, f := range files {
		pvFiles[i] = preview.VerifiableFile{Path: f.Path, Content: f.Content}
	}
	res := b.verifier.VerifyFiles(ctx, pvFiles, isFullStack)
	if res == nil {
		return &agents.PreviewVerificationResult{Passed: true}
	}
	return &agents.PreviewVerificationResult{
		Passed:                       res.Passed,
		FailureKind:                  res.FailureKind,
		RepairHints:                  res.RepairHints,
		Details:                      res.Details,
		ScreenshotBase64:             res.ScreenshotBase64,
		CanaryErrors:                 res.CanaryErrors,
		CanaryClickCount:             res.CanaryClickCount,
		CanaryVisibleControls:        res.CanaryVisibleControls,
		CanaryPostInteractionChecked: res.CanaryPostInteractionChecked,
		CanaryPostInteractionHealthy: res.CanaryPostInteractionHealthy,
		VisionSeverity:               res.VisionSeverity,
	}
}
