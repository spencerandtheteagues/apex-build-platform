package api

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"apex-build/internal/ai"
	"apex-build/internal/auth"
	"apex-build/internal/cache"
	"apex-build/internal/db"
	"apex-build/internal/email"
	appmiddleware "apex-build/internal/middleware"
	"apex-build/internal/mobile"
	"apex-build/internal/origins"
	"apex-build/internal/payments"
	"apex-build/internal/pricing"
	"apex-build/internal/startup"
	"apex-build/internal/storage"
	"apex-build/internal/usage"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Server represents the API server
type Server struct {
	db           *db.Database
	auth         *auth.AuthService
	aiRouter     *ai.AIRouter
	byok         *ai.BYOKManager
	usage        *usage.Tracker
	readiness    *startup.Registry
	storage      storage.Provider
	cache        func() cache.Status
	email        *email.Service
	mobile       *mobile.MobileBuildService
	mobileSubmit *mobile.MobileSubmissionService
}

// NewServer creates a new API server
func NewServer(database *db.Database, authService *auth.AuthService, aiRouter *ai.AIRouter, byokManager *ai.BYOKManager) *Server {
	return &Server{
		db:       database,
		auth:     authService,
		aiRouter: aiRouter,
		byok:     byokManager,
	}
}

// SetStorageProvider sets the storage provider for asset handling
func (s *Server) SetStorageProvider(provider storage.Provider) {
	s.storage = provider
}

func (s *Server) SetReadinessRegistry(registry *startup.Registry) {
	s.readiness = registry
}

func (s *Server) SetUsageTracker(tracker *usage.Tracker) {
	s.usage = tracker
}

func (s *Server) SetCacheStatusProvider(provider func() cache.Status) {
	s.cache = provider
}

func (s *Server) SetMobileBuildService(service *mobile.MobileBuildService) {
	s.mobile = service
}

func (s *Server) SetMobileSubmissionService(service *mobile.MobileSubmissionService) {
	s.mobileSubmit = service
}

// Health endpoint - Returns quickly for load balancer health checks
func (s *Server) Health(c *gin.Context) {
	summary := s.runtimeReadinessSummary(false)
	aiHealth, healthyProviders := s.aiHealthSnapshot()
	c.JSON(http.StatusOK, gin.H{
		"status":                   topLevelHealthStatus(summary),
		"ready":                    summary.Ready,
		"phase":                    summary.Phase,
		"database":                 "configured",
		"ai_providers":             aiHealth,
		"healthy_providers":        healthyProviders,
		"total_providers":          len(aiHealth),
		"version":                  "1.0.0",
		"feature_readiness_status": summary.Status,
		"startup":                  summary,
	})
}

// DeepHealth endpoint - Full health check with database ping (for monitoring)
func (s *Server) DeepHealth(c *gin.Context) {
	summary := s.runtimeReadinessSummary(true)

	// Check AI providers health
	aiHealth, healthyProviders := s.aiHealthSnapshot()

	statusCode := http.StatusOK
	if !summary.Ready {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, gin.H{
		"status":                   topLevelHealthStatus(summary),
		"ready":                    summary.Ready,
		"database":                 primaryDatabaseHealth(summary),
		"ai_providers":             aiHealth,
		"healthy_providers":        healthyProviders,
		"total_providers":          len(aiHealth),
		"version":                  "1.0.0",
		"feature_readiness_status": summary.Status,
		"startup":                  summary,
	})
}

func (s *Server) FeatureReadiness(c *gin.Context) {
	summary := s.runtimeReadinessSummary(true)
	statusCode := http.StatusOK
	if !summary.Ready {
		statusCode = http.StatusServiceUnavailable
	}
	c.JSON(statusCode, summary)
}

func (s *Server) readinessSummary() startup.Summary {
	if s.readiness == nil {
		return startup.Summary{
			Phase:  startup.PhaseReady,
			Status: "healthy",
			Ready:  true,
		}
	}
	return s.readiness.Snapshot()
}

func (s *Server) runtimeReadinessSummary(includeDatabaseHealth bool) startup.Summary {
	summary := s.readinessSummary()

	if s.cache != nil {
		cacheStatus := s.cache()
		cacheState := startup.StateReady
		cacheSummary := "Redis cache backend connected"
		if !cacheStatus.RedisConnected {
			cacheState = startup.StateDegraded
			cacheSummary = "Using in-memory cache fallback"
		}
		cacheDetails := map[string]any{
			"backend":         cacheStatus.Backend,
			"redis_connected": cacheStatus.RedisConnected,
			"fallback_reason": cacheStatus.FallbackReason,
		}
		if cacheStatus.RecommendedFix != "" {
			cacheDetails["recommended_fix"] = cacheStatus.RecommendedFix
		}
		summary = startup.ApplyRuntimeService(summary, startup.Service{
			Name:      "redis_cache",
			Tier:      startup.TierOptional,
			State:     cacheState,
			Summary:   cacheSummary,
			Details:   cacheDetails,
			UpdatedAt: time.Now().UTC(),
		})
	}

	if includeDatabaseHealth && s.db != nil {
		now := time.Now().UTC()
		dbState := startup.StateReady
		dbSummary := "Primary database connected"
		dbDetails := map[string]any{}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := s.db.HealthContext(ctx)
		cancel()
		if err != nil {
			dbState = startup.StateFailed
			dbSummary = "Primary database unavailable"
			dbDetails["error"] = err.Error()
		}
		summary = startup.ApplyRuntimeService(summary, startup.Service{
			Name:      "primary_database",
			Tier:      startup.TierCritical,
			State:     dbState,
			Summary:   dbSummary,
			Details:   dbDetails,
			UpdatedAt: now,
		})
	}

	return summary
}

func (s *Server) aiHealthSnapshot() (map[string]*ai.ProviderHealthDetail, int) {
	if s.aiRouter == nil {
		return map[string]*ai.ProviderHealthDetail{}, 0
	}

	aiHealth := s.aiRouter.GetDetailedHealthStatus()
	healthyProviders := 0
	for _, detail := range aiHealth {
		if detail != nil && detail.Status == "ok" {
			healthyProviders++
		}
	}

	return aiHealth, healthyProviders
}

func topLevelHealthStatus(summary startup.Summary) string {
	if summary.Ready {
		return "healthy"
	}
	if summary.Phase == startup.PhaseShuttingDown {
		return "shutting_down"
	}
	if summary.Phase == startup.PhaseStarting {
		return "starting"
	}
	return "unhealthy"
}

func primaryDatabaseHealth(summary startup.Summary) string {
	for _, service := range summary.Services {
		if service.Name != "primary_database" {
			continue
		}
		if service.State == startup.StateReady {
			return "connected"
		}
		return "unavailable"
	}
	return "configured"
}

// Authentication endpoints

func cookieSessionPayload(tokens *auth.TokenPair) gin.H {
	payload := gin.H{
		"session_strategy": "cookie",
	}
	if tokens == nil {
		return payload
	}

	payload["access_token_expires_at"] = tokens.AccessTokenExpiresAt
	payload["refresh_token_expires_at"] = tokens.RefreshTokenExpiresAt
	payload["token_type"] = tokens.TokenType
	return payload
}

// Register handles user registration
// SECURITY: Fixed TOCTOU race condition - now uses transaction with proper locking
func (s *Server) Register(c *gin.Context) {
	var req auth.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.AcceptanceIP = c.ClientIP()
	req.AcceptanceAgent = c.Request.UserAgent()

	// Create user object (validates input)
	user, err := s.auth.CreateUser(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use transaction with row-level locking to prevent race conditions
	err = s.db.DB.Transaction(func(tx *gorm.DB) error {
		// Check if user already exists within transaction
		var existingUser models.User
		if err := tx.Where("LOWER(username) = LOWER(?) OR LOWER(email) = LOWER(?)", req.Username, req.Email).First(&existingUser).Error; err == nil {
			return fmt.Errorf("user already exists")
		}

		// Create user within same transaction
		if err := tx.Create(user).Error; err != nil {
			// Handle unique constraint violation gracefully
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
				return fmt.Errorf("user already exists")
			}
			return err
		}

		if err := payments.ApplyCreditGrant(
			tx,
			user.ID,
			payments.FreeSignupTrialCreditsUSD,
			payments.CreditEntryTypeSignupTrial,
			"One-time free managed trial credits",
			"",
			"",
			string(payments.PlanFree),
		); err != nil {
			return err
		}
		user.CreditBalance = payments.FreeSignupTrialCreditsUSD

		// Create a hard spending cap matching the free trial balance so
		// PreAuthorize blocks builds once credits are exhausted.
		// Without this row GetCaps returns empty → unlimited spending.
		now := time.Now().UTC()
		cap := struct {
			CreatedAt time.Time
			UpdatedAt time.Time
			UserID    uint
			CapType   string
			LimitUSD  float64
			Action    string
			IsActive  bool
		}{
			CreatedAt: now,
			UpdatedAt: now,
			UserID:    user.ID,
			CapType:   "monthly",
			LimitUSD:  payments.FreeSignupTrialCreditsUSD,
			Action:    "stop",
			IsActive:  true,
		}
		if err := tx.Table("budget_caps").Create(&cap).Error; err != nil {
			return fmt.Errorf("failed to create free-tier budget cap: %w", err)
		}

		return nil
	})

	if err != nil {
		if err.Error() == "user already exists" {
			c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Issue email verification code (best-effort, non-blocking)
	go func() {
		if err := s.issueVerificationCode(user); err != nil {
			// Already logged inside issueVerificationCode
			_ = err
		}
	}()

	// Generate tokens
	tokens, err := s.auth.GenerateTokens(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	auth.SetAccessTokenCookie(c, tokens.AccessToken)
	auth.SetRefreshTokenCookie(c, tokens.RefreshToken)

	response := cookieSessionPayload(tokens)
	response["message"] = "User created successfully"
	response["email_verification_required"] = true
	response["user"] = gin.H{
		"id":                    user.ID,
		"username":              user.Username,
		"email":                 user.Email,
		"full_name":             user.FullName,
		"is_admin":              user.IsAdmin,
		"is_super_admin":        user.IsSuperAdmin,
		"has_unlimited_credits": user.HasUnlimitedCredits,
		"subscription_type":     user.SubscriptionType,
		"credit_balance":        user.CreditBalance,
	}

	c.JSON(http.StatusCreated, response)
}

// Login handles user login — accepts username or email
func (s *Server) Login(c *gin.Context) {
	var req auth.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Determine login identifier: accept email or username field
	identifier := strings.TrimSpace(req.Username)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Email)
	}
	if identifier == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username or email is required"})
		return
	}

	// Find user — if identifier contains '@', look up by email; otherwise try both
	// Use case-insensitive matching so "Admin", "admin", "ADMIN" all work
	var user models.User
	var dbErr error
	if strings.Contains(identifier, "@") {
		dbErr = s.db.DB.Where("LOWER(email) = LOWER(?)", identifier).First(&user).Error
	} else {
		dbErr = s.db.DB.Where("LOWER(username) = LOWER(?)", identifier).First(&user).Error
	}
	if dbErr != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Check password
	if err := s.auth.CheckPassword(req.Password, user.PasswordHash); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Check if user is active
	if !user.IsActive {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Account is deactivated"})
		return
	}

	// Block unverified users — admin/super-admin bypass for convenience
	if !user.IsVerified && user.EmailVerifiedAt == nil && !user.IsAdmin {
		go func(user models.User) { _ = s.issueVerificationCodeIfNeeded(&user) }(user)
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Email not verified. Enter the latest verification code from your email, or request a new one.",
			"error_code": "email_not_verified",
			"email":      maskEmail(user.Email),
		})
		return
	}

	// Generate tokens
	tokens, err := s.auth.GenerateTokens(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	auth.SetAccessTokenCookie(c, tokens.AccessToken)
	auth.SetRefreshTokenCookie(c, tokens.RefreshToken)

	response := cookieSessionPayload(tokens)
	response["message"] = "Login successful"
	response["user"] = gin.H{
		"id":                    user.ID,
		"username":              user.Username,
		"email":                 user.Email,
		"full_name":             user.FullName,
		"preferred_theme":       user.PreferredTheme,
		"preferred_ai":          user.PreferredAI,
		"is_admin":              user.IsAdmin,
		"is_super_admin":        user.IsSuperAdmin,
		"has_unlimited_credits": user.HasUnlimitedCredits,
		"bypass_billing":        user.BypassBilling,
		"subscription_type":     user.SubscriptionType,
		"credit_balance":        user.CreditBalance,
	}

	c.JSON(http.StatusOK, response)
}

// RefreshToken issues a new access/refresh token pair from a valid refresh token.
func (s *Server) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid refresh request"})
		return
	}

	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		if cookieToken, err := auth.GetRefreshTokenFromCookie(c); err == nil {
			refreshToken = strings.TrimSpace(cookieToken)
		}
	}
	if refreshToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token required"})
		return
	}

	tokens, err := s.auth.RefreshTokens(refreshToken, nil)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid refresh token",
			"details": err.Error(),
		})
		return
	}

	auth.SetAccessTokenCookie(c, tokens.AccessToken)
	auth.SetRefreshTokenCookie(c, tokens.RefreshToken)

	response := cookieSessionPayload(tokens)
	response["message"] = "Tokens refreshed successfully"

	c.JSON(http.StatusOK, response)
}

// Logout invalidates the presented access token when possible.
func (s *Server) Logout(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		req.RefreshToken = ""
	}

	if token, err := auth.AccessTokenFromRequest(c); err == nil && token != "" {
		_ = s.auth.BlacklistToken(token)
	}
	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		if cookieToken, err := auth.GetRefreshTokenFromCookie(c); err == nil {
			refreshToken = strings.TrimSpace(cookieToken)
		}
	}
	if refreshToken != "" {
		_ = s.auth.RevokeRefreshToken(refreshToken)
	}
	auth.ClearAuthCookies(c)

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// AI endpoints

// AIGenerate handles AI generation requests
func (s *Server) AIGenerate(c *gin.Context) {
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	var request struct {
		Capability  string                 `json:"capability" binding:"required"`
		Prompt      string                 `json:"prompt" binding:"required"`
		Code        string                 `json:"code,omitempty"`
		Language    string                 `json:"language,omitempty"`
		Context     map[string]interface{} `json:"context,omitempty"`
		MaxTokens   int                    `json:"max_tokens,omitempty"`
		Temperature float32                `json:"temperature,omitempty"`
		ProjectID   string                 `json:"project_id,omitempty"`
		Provider    string                 `json:"provider,omitempty"`
		Model       string                 `json:"model,omitempty"`
		PowerMode   string                 `json:"power_mode,omitempty"` // fast, balanced, max
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create AI request
	provider := strings.ToLower(request.Provider)
	if provider == "auto" {
		provider = ""
	}
	aiReq := &ai.AIRequest{
		ID:          uuid.New().String(),
		Capability:  ai.AICapability(request.Capability),
		Prompt:      request.Prompt,
		Code:        request.Code,
		Language:    request.Language,
		Context:     request.Context,
		MaxTokens:   request.MaxTokens,
		Temperature: request.Temperature,
		Provider:    ai.AIProvider(provider),
		Model:       request.Model,
		UserID:      fmt.Sprintf("%d", uid),
		ProjectID:   request.ProjectID,
	}

	if aiReq.Temperature == 0 {
		aiReq.Temperature = 0.7
	}

	// Select router with BYOK awareness
	targetRouter := s.aiRouter
	isBYOK := false
	if s.byok != nil {
		if userRouter, hasBYOK, err := s.byok.GetRouterForUser(uid); err == nil && userRouter != nil {
			targetRouter = userRouter
			isBYOK = hasBYOK
		}
	}

	// Estimate and reserve credits before making the AI call
	var reservation *ai.CreditReservation
	if s.byok != nil {
		estimateProvider := provider
		if estimateProvider == "" {
			estimateProvider = string(s.aiRouter.GetDefaultProvider(aiReq.Capability))
		}
		powerMode := request.PowerMode
		if powerMode == "" {
			powerMode = pricing.ModeFast
		}
		maxTokens := request.MaxTokens
		if maxTokens <= 0 {
			maxTokens = 2000
		}
		estimatedCost := s.byok.EstimateCost(
			estimateProvider,
			request.Model,
			len(request.Prompt)+len(request.Code),
			maxTokens,
			powerMode,
			isBYOK,
		)
		if estimatedCost > 0 {
			res, err := s.byok.ReserveCredits(uid, estimatedCost, isBYOK)
			if err != nil {
				if strings.Contains(err.Error(), "INSUFFICIENT_CREDITS") {
					c.JSON(http.StatusPaymentRequired, gin.H{
						"error":      "Insufficient credits for this request",
						"code":       "INSUFFICIENT_CREDITS",
						"required":   estimatedCost,
						"power_mode": powerMode,
					})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reserve credits"})
				return
			}
			reservation = res
		}
	}

	// Generate AI response
	response, err := targetRouter.Generate(c.Request.Context(), aiReq)
	if err != nil {
		if s.byok != nil && reservation != nil {
			_ = s.byok.FinalizeCredits(reservation, 0)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	actualProvider := ai.ActualProvider(response, aiReq.Provider)

	// Save request to database
	dbRequest := &models.AIRequest{
		RequestID:  aiReq.ID,
		UserID:     uid,
		Provider:   string(actualProvider),
		Capability: string(aiReq.Capability),
		Prompt:     aiReq.Prompt,
		Code:       aiReq.Code,
		Language:   aiReq.Language,
		Response:   response.Content,
		Status:     "completed",
		Duration:   response.Duration.Milliseconds(),
	}

	if request.ProjectID != "" {
		if projectIDUint, err := strconv.ParseUint(request.ProjectID, 10, 32); err == nil {
			projectID := uint(projectIDUint)
			dbRequest.ProjectID = &projectID
		}
	}

	if response.Usage != nil {
		dbRequest.TokensUsed = response.Usage.TotalTokens
		dbRequest.Cost = response.Usage.Cost
	}

	if response.Error != "" {
		dbRequest.Status = "failed"
		dbRequest.ErrorMsg = response.Error
	}

	// Record BYOK usage and finalize credits if applicable
	if s.byok != nil {
		if response != nil {
			var projectID *uint
			if request.ProjectID != "" {
				if projectIDUint, err := strconv.ParseUint(request.ProjectID, 10, 32); err == nil {
					pid := uint(projectIDUint)
					projectID = &pid
				}
			}
			inputTokens := 0
			outputTokens := 0
			cost := 0.0
			if response.Usage != nil {
				inputTokens = response.Usage.PromptTokens
				outputTokens = response.Usage.CompletionTokens
			}
			modelUsed := ai.GetModelUsed(response, aiReq)
			powerMode := request.PowerMode
			if powerMode == "" {
				powerMode = pricing.ModeFast
			}
			cost = s.byok.BilledCost(string(actualProvider), modelUsed, inputTokens, outputTokens, powerMode, isBYOK)
			if response.Usage != nil {
				response.Usage.Cost = cost
			}
			dbRequest.Cost = cost
			s.byok.RecordUsage(uid, projectID, string(actualProvider), modelUsed, isBYOK,
				inputTokens, outputTokens, cost, string(aiReq.Capability), response.Duration, "success")
			if reservation != nil {
				_ = s.byok.FinalizeCredits(reservation, cost)
			}
		}
	}

	// Persist request after cost adjustments
	s.db.DB.Create(dbRequest)

	if s.usage != nil {
		var projectID *uint
		if dbRequest.ProjectID != nil {
			projectID = dbRequest.ProjectID
		}
		tokensUsed := dbRequest.TokensUsed
		if err := s.usage.RecordAIRequest(c.Request.Context(), uid, projectID, string(response.Provider), tokensUsed); err != nil {
			fmt.Printf("usage tracker: failed to record AI request for user %d: %v\n", uid, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"request_id": response.ID,
		"provider":   response.Provider,
		"content":    response.Content,
		"usage":      response.Usage,
		"duration":   response.Duration.Milliseconds(),
		"created_at": response.CreatedAt,
	})
}

// GetAIUsage returns AI usage statistics for a user
func (s *Server) GetAIUsage(c *gin.Context) {
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	var requests []models.AIRequest
	if err := s.db.DB.Where("user_id = ?", uid).Order("created_at DESC").Limit(100).Find(&requests).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch usage data"})
		return
	}

	// Calculate statistics
	totalRequests := len(requests)
	totalCost := 0.0
	totalTokens := 0
	providerStats := make(map[string]*ProviderStat)

	for _, req := range requests {
		totalCost += req.Cost
		totalTokens += req.TokensUsed

		if _, exists := providerStats[req.Provider]; !exists {
			providerStats[req.Provider] = &ProviderStat{}
		}

		stat := providerStats[req.Provider]
		stat.Requests++
		stat.Cost += req.Cost
		stat.Tokens += req.TokensUsed
	}

	c.JSON(http.StatusOK, gin.H{
		"total_requests":  totalRequests,
		"total_cost":      totalCost,
		"total_tokens":    totalTokens,
		"by_provider":     providerStats,
		"recent_requests": requests,
	})
}

type ProviderStat struct {
	Requests int     `json:"requests"`
	Cost     float64 `json:"cost"`
	Tokens   int     `json:"tokens"`
}

// Project endpoints

// CreateProject creates a new project
func (s *Server) CreateProject(c *gin.Context) {
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	var req struct {
		Name        string                 `json:"name" binding:"required"`
		Description string                 `json:"description"`
		Language    string                 `json:"language"`
		Framework   string                 `json:"framework"`
		IsPublic    bool                   `json:"is_public"`
		Environment map[string]interface{} `json:"environment"`
		mobile.ProjectMetadataFields
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.IsPublic && !requirePaidBackendPlan(c, s.db.DB, uid, "Publishing projects") {
		return
	}

	// Default language if not provided
	if req.Language == "" {
		req.Language = "javascript"
	}

	project := &models.Project{
		Name:        req.Name,
		Description: req.Description,
		Language:    req.Language,
		Framework:   req.Framework,
		OwnerID:     uid,
		IsPublic:    req.IsPublic,
		Environment: req.Environment,
	}
	mobile.ApplyProjectMetadata(project, req.ProjectMetadataFields)

	if err := s.db.DB.Create(project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Project created successfully",
		"project": project,
	})
}

// GetProjects returns projects for a user
func (s *Server) GetProjects(c *gin.Context) {
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	var projects []models.Project
	if err := s.db.DB.Where("owner_id = ?", uid).Order("updated_at DESC").Find(&projects).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch projects"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"projects": projects,
	})
}

// GetProject returns a specific project
func (s *Server) GetProject(c *gin.Context) {
	projectID := c.Param("id")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	var project models.Project
	query := s.db.DB.Where("id = ?", projectID)

	// Only allow access to own projects or public projects
	query = query.Where("owner_id = ? OR is_public = ?", uid, true)

	if err := query.Preload("Files").First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"project": project,
	})
}

// File endpoints

// CreateFile creates a new file in a project
func (s *Server) CreateFile(c *gin.Context) {
	projectID := c.Param("id")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	var req struct {
		Path     string `json:"path" binding:"required"`
		Name     string `json:"name" binding:"required"`
		Type     string `json:"type" binding:"required"`
		Content  string `json:"content"`
		MimeType string `json:"mime_type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify project ownership
	var project models.Project
	if err := s.db.DB.Where("id = ? AND owner_id = ?", projectID, uid).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	projectIDUint, _ := strconv.ParseUint(projectID, 10, 32)
	file := &models.File{
		ProjectID:  uint(projectIDUint),
		Path:       req.Path,
		Name:       req.Name,
		Type:       req.Type,
		Content:    req.Content,
		MimeType:   req.MimeType,
		Size:       int64(len(req.Content)),
		LastEditBy: uid,
	}

	if err := s.db.DB.Create(file).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create file"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "File created successfully",
		"file":    file,
	})
}

// GetFiles returns files for a project
func (s *Server) GetFiles(c *gin.Context) {
	projectID := c.Param("id")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	// Verify project access
	var project models.Project
	query := s.db.DB.Where("id = ?", projectID)
	query = query.Where("owner_id = ? OR is_public = ?", uid, true)

	if err := query.First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	var files []models.File
	if err := s.db.DB.Where("project_id = ?", projectID).Order("path ASC").Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch files"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"files": files,
	})
}

// GetFile returns a specific file by ID
func (s *Server) GetFile(c *gin.Context) {
	fileID := c.Param("id")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	var file models.File
	if err := s.db.DB.Preload("Project").Where("id = ?", fileID).First(&file).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Check access permissions
	if file.Project.OwnerID != uid && !file.Project.IsPublic {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"file": file,
	})
}

// DownloadProject exports all project files as a zip archive
func (s *Server) DownloadProject(c *gin.Context) {
	projectID := c.Param("id")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	// Verify project access
	var project models.Project
	query := s.db.DB.Where("id = ?", projectID)
	query = query.Where("owner_id = ? OR is_public = ?", uid, true)

	if err := query.First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.OwnerID == uid {
		if err := mobile.PrepareExpoProjectFiles(c.Request.Context(), s.db.DB, project); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare mobile export files"})
			return
		}
	}

	// Get all files for the project
	var files []models.File
	if err := s.db.DB.Where("project_id = ?", projectID).Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch files"})
		return
	}

	// Create zip archive
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.zip\"", project.Name))

	zipWriter := zip.NewWriter(c.Writer)
	defer zipWriter.Close()

	for _, file := range files {
		// Skip directories
		if file.Type == "directory" {
			continue
		}

		// Remove leading slash from path
		path := file.Path
		if len(path) > 0 && path[0] == '/' {
			path = path[1:]
		}

		// Create file entry in zip
		w, err := zipWriter.Create(path)
		if err != nil {
			continue
		}

		// Write content
		if _, err := w.Write([]byte(file.Content)); err != nil {
			continue
		}
	}
}

// GetProjectMobileValidation validates the generated Expo source package for a project.
func (s *Server) GetProjectMobileValidation(c *gin.Context) {
	projectID := c.Param("id")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	var project models.Project
	query := s.db.DB.Where("id = ?", projectID)
	query = query.Where("owner_id = ? OR is_public = ?", uid, true)
	if err := query.First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.OwnerID == uid {
		if err := mobile.PrepareExpoProjectFiles(c.Request.Context(), s.db.DB, project); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare mobile validation files"})
			return
		}
	}

	var files []models.File
	if err := s.db.DB.Where(
		"project_id = ? AND (path LIKE ? OR path LIKE ? OR path LIKE ? OR path LIKE ? OR path = ? OR path = ?)",
		project.ID,
		"mobile/%",
		"/mobile/%",
		"backend/%",
		"/backend/%",
		"docs/mobile-backend-routes.md",
		"/docs/mobile-backend-routes.md",
	).Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch mobile files"})
		return
	}

	report := mobile.ValidateProjectSourcePackage(project, files)
	c.JSON(http.StatusOK, gin.H{"validation": report})
}

// GetProjectMobileScorecard reports objective readiness toward the 95% Android/iOS launch target.
func (s *Server) GetProjectMobileScorecard(c *gin.Context) {
	projectID := c.Param("id")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	var project models.Project
	query := s.db.DB.Where("id = ?", projectID)
	query = query.Where("owner_id = ? OR is_public = ?", uid, true)
	if err := query.First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.OwnerID == uid {
		if err := mobile.PrepareExpoProjectFiles(c.Request.Context(), s.db.DB, project); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare mobile readiness files"})
			return
		}
	}

	var files []models.File
	if err := s.db.DB.Where(
		"project_id = ? AND (path LIKE ? OR path LIKE ? OR path LIKE ? OR path LIKE ? OR path = ? OR path = ?)",
		project.ID,
		"mobile/%",
		"/mobile/%",
		"backend/%",
		"/backend/%",
		"docs/mobile-backend-routes.md",
		"/docs/mobile-backend-routes.md",
	).Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch mobile files"})
		return
	}

	validation := mobile.ValidateProjectSourcePackage(project, files)
	scorecard := mobile.BuildMobileReadinessScorecard(project, files, validation)
	c.JSON(http.StatusOK, gin.H{"scorecard": scorecard})
}

// UpdateFile updates a file's content
func (s *Server) UpdateFile(c *gin.Context) {
	fileID := c.Param("id")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	var req struct {
		Content *string `json:"content"`
		Name    *string `json:"name"`
		Path    *string `json:"path"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Content == nil && req.Name == nil && req.Path == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No updates provided"})
		return
	}

	// Find file and verify ownership
	var file models.File
	if err := s.db.DB.Preload("Project").Where("id = ?", fileID).First(&file).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	if file.Project.OwnerID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Update file (content and/or metadata)
	tx := s.db.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start update transaction"})
		return
	}

	// Handle rename/path update
	if req.Name != nil || req.Path != nil {
		newPath := file.Path
		if req.Path != nil {
			newPath = *req.Path
		}
		if req.Name != nil && req.Path == nil {
			pathParts := strings.Split(file.Path, "/")
			if len(pathParts) > 0 {
				pathParts[len(pathParts)-1] = *req.Name
				newPath = strings.Join(pathParts, "/")
			}
		}

		if req.Name != nil {
			file.Name = *req.Name
		} else if newPath != "" {
			parts := strings.Split(newPath, "/")
			file.Name = parts[len(parts)-1]
		}

		if newPath != "" && newPath != file.Path {
			oldPath := file.Path
			file.Path = newPath

			// If renaming a directory, update all child paths
			if file.Type == "directory" {
				var children []models.File
				if err := tx.Where("project_id = ? AND path LIKE ?", file.ProjectID, oldPath+"/%").Find(&children).Error; err != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load directory contents"})
					return
				}

				for _, child := range children {
					child.Path = newPath + strings.TrimPrefix(child.Path, oldPath)
					pathParts := strings.Split(child.Path, "/")
					child.Name = pathParts[len(pathParts)-1]
					if err := tx.Save(&child).Error; err != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update child paths"})
						return
					}
				}
			}
		}
	}

	if req.Content != nil {
		file.Content = *req.Content
		file.Size = int64(len(*req.Content))
	}

	file.LastEditBy = uid
	file.Version++

	if err := tx.Save(&file).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update file"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit file update"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "File updated successfully",
		"file":    file,
	})
}

// DeleteFile deletes a file by ID
func (s *Server) DeleteFile(c *gin.Context) {
	fileID := c.Param("id")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	var file models.File
	if err := s.db.DB.Preload("Project").Where("id = ?", fileID).First(&file).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	if file.Project.OwnerID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := s.db.DB.Delete(&file).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "File deleted successfully",
	})
}

// Middleware

// AuthMiddleware validates JWT tokens
func (s *Server) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := auth.AccessTokenFromRequest(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			c.Abort()
			return
		}

		claims, err := s.auth.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Set all user context including bypass/admin flags for quota middleware
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Set("subscription_type", claims.SubscriptionType)
		c.Set("subscription_status", claims.SubscriptionStatus)
		c.Set("is_admin", claims.IsAdmin)
		c.Set("is_super_admin", claims.IsSuperAdmin)
		c.Set("has_unlimited_credits", claims.HasUnlimitedCredits)
		c.Set("bypass_billing", claims.BypassBilling)
		c.Set("bypass_rate_limits", claims.BypassRateLimits)
		c.Next()
	}
}

// CORSMiddleware handles CORS with secure origin validation
// SECURITY: No longer uses wildcard (*) - validates against allowed origins
func (s *Server) CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origins.IsAllowedOrigin(origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Request-ID, X-Apex-Build-Poll-Token")
		c.Header("Access-Control-Max-Age", "86400") // 24 hours preflight cache

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
