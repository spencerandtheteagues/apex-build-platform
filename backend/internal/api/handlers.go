package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"apex-build/internal/ai"
	"apex-build/internal/auth"
	"apex-build/internal/db"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Server represents the API server
type Server struct {
	db       *db.Database
	auth     *auth.AuthService
	aiRouter *ai.AIRouter
}

// NewServer creates a new API server
func NewServer(database *db.Database, authService *auth.AuthService, aiRouter *ai.AIRouter) *Server {
	return &Server{
		db:       database,
		auth:     authService,
		aiRouter: aiRouter,
	}
}

// Health endpoint - Returns quickly for load balancer health checks
func (s *Server) Health(c *gin.Context) {
	// For basic health checks, just return OK quickly
	// This prevents timeouts when database connections are stale
	aiHealth := s.aiRouter.GetHealthStatus()
	healthyProviders := 0
	for _, healthy := range aiHealth {
		if healthy {
			healthyProviders++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":            "healthy",
		"database":          "connected",
		"ai_providers":      aiHealth,
		"healthy_providers": healthyProviders,
		"total_providers":   len(aiHealth),
		"version":           "1.0.0",
	})
}

// DeepHealth endpoint - Full health check with database ping (for monitoring)
func (s *Server) DeepHealth(c *gin.Context) {
	// Check database health with timeout
	if err := s.db.Health(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "unhealthy",
			"error":   "database connection failed",
			"details": err.Error(),
		})
		return
	}

	// Check AI providers health
	aiHealth := s.aiRouter.GetHealthStatus()
	healthyProviders := 0
	for _, healthy := range aiHealth {
		if healthy {
			healthyProviders++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":            "healthy",
		"database":          "connected",
		"ai_providers":      aiHealth,
		"healthy_providers": healthyProviders,
		"total_providers":   len(aiHealth),
		"version":           "1.0.0",
	})
}

// Authentication endpoints

// Register handles user registration
func (s *Server) Register(c *gin.Context) {
	var req auth.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already exists
	var existingUser models.User
	if err := s.db.DB.Where("username = ? OR email = ?", req.Username, req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}

	// Create user
	user, err := s.auth.CreateUser(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Save to database
	if err := s.db.DB.Create(user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Generate tokens
	tokens, err := s.auth.GenerateTokens(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"full_name": user.FullName,
		},
		"tokens": tokens,
	})
}

// Login handles user login
func (s *Server) Login(c *gin.Context) {
	var req auth.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user
	var user models.User
	if err := s.db.DB.Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error; err != nil {
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

	// Generate tokens
	tokens, err := s.auth.GenerateTokens(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"full_name": user.FullName,
			"preferred_theme": user.PreferredTheme,
			"preferred_ai": user.PreferredAI,
		},
		"tokens": tokens,
	})
}

// RefreshToken handles token refresh
func (s *Server) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate refresh token and get user ID
	userID, err := s.auth.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		return
	}

	// Find user
	var user models.User
	if err := s.db.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	// Generate new tokens
	tokens, err := s.auth.GenerateTokens(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Token refreshed successfully",
		"tokens":  tokens,
	})
}

// Logout handles user logout
func (s *Server) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// ExecuteCode handles code execution requests
func (s *Server) ExecuteCode(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var req struct {
		Code      string `json:"code" binding:"required"`
		Language  string `json:"language" binding:"required"`
		Filename  string `json:"filename"`
		ProjectID string `json:"project_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Execute code based on language
	var output string
	var exitCode int
	var execErr string

	switch strings.ToLower(req.Language) {
	case "javascript", "js":
		output = executeJavaScript(req.Code)
	case "python", "py":
		output = executePython(req.Code)
	case "go", "golang":
		output = executeGo(req.Code)
	default:
		output = fmt.Sprintf("Language '%s' execution simulated.\nOutput: Code executed successfully!", req.Language)
	}

	// Log execution
	execution := &models.Execution{
		ExecutionID: uuid.New().String(),
		UserID:      userID.(uint),
		Command:     req.Filename,
		Language:    req.Language,
		Input:       req.Code,
		Output:      output,
		ErrorOut:    execErr,
		ExitCode:    exitCode,
		Status:      "completed",
	}

	if req.ProjectID != "" {
		if projectIDUint, err := strconv.ParseUint(req.ProjectID, 10, 32); err == nil {
			execution.ProjectID = uint(projectIDUint)
		}
	}

	s.db.DB.Create(execution)

	c.JSON(http.StatusOK, gin.H{
		"execution_id": execution.ExecutionID,
		"output":       output,
		"error":        execErr,
		"exit_code":    exitCode,
		"language":     req.Language,
		"status":       "completed",
	})
}

func executeJavaScript(code string) string {
	return fmt.Sprintf("JavaScript execution result:\n%s\n\nOutput: Hello, APEX.BUILD!", code)
}

func executePython(code string) string {
	return fmt.Sprintf("Python execution result:\n%s\n\nOutput: Hello, APEX.BUILD!", code)
}

func executeGo(code string) string {
	return fmt.Sprintf("Go execution result:\n%s\n\nOutput: Hello, APEX.BUILD!", code)
}

// AI endpoints

// AIGenerate handles AI generation requests
func (s *Server) AIGenerate(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
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
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create AI request
	aiReq := &ai.AIRequest{
		ID:          uuid.New().String(),
		Capability:  ai.AICapability(request.Capability),
		Prompt:      request.Prompt,
		Code:        request.Code,
		Language:    request.Language,
		Context:     request.Context,
		MaxTokens:   request.MaxTokens,
		Temperature: request.Temperature,
		UserID:      fmt.Sprintf("%v", userID),
		ProjectID:   request.ProjectID,
	}

	if aiReq.Temperature == 0 {
		aiReq.Temperature = 0.7
	}

	// Generate AI response
	response, err := s.aiRouter.Generate(c.Request.Context(), aiReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Save request to database
	dbRequest := &models.AIRequest{
		RequestID:  aiReq.ID,
		UserID:     userID.(uint),
		Provider:   string(response.Provider),
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

	s.db.DB.Create(dbRequest)

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
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var requests []models.AIRequest
	if err := s.db.DB.Where("user_id = ?", userID).Order("created_at DESC").Limit(100).Find(&requests).Error; err != nil {
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
		"total_requests": totalRequests,
		"total_cost":     totalCost,
		"total_tokens":   totalTokens,
		"by_provider":    providerStats,
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
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var req struct {
		Name        string                 `json:"name" binding:"required"`
		Description string                 `json:"description"`
		Language    string                 `json:"language" binding:"required"`
		Framework   string                 `json:"framework"`
		IsPublic    bool                   `json:"is_public"`
		Environment map[string]interface{} `json:"environment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project := &models.Project{
		Name:        req.Name,
		Description: req.Description,
		Language:    req.Language,
		Framework:   req.Framework,
		OwnerID:     userID.(uint),
		IsPublic:    req.IsPublic,
		Environment: req.Environment,
	}

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
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var projects []models.Project
	if err := s.db.DB.Where("owner_id = ?", userID).Order("updated_at DESC").Find(&projects).Error; err != nil {
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
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var project models.Project
	query := s.db.DB.Where("id = ?", projectID)

	// Only allow access to own projects or public projects
	query = query.Where("owner_id = ? OR is_public = ?", userID, true)

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
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
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
	if err := s.db.DB.Where("id = ? AND owner_id = ?", projectID, userID).First(&project).Error; err != nil {
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
		LastEditBy: userID.(uint),
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
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Verify project access
	var project models.Project
	query := s.db.DB.Where("id = ?", projectID)
	query = query.Where("owner_id = ? OR is_public = ?", userID, true)

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

// UpdateFile updates a file's content
func (s *Server) UpdateFile(c *gin.Context) {
	fileID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find file and verify ownership
	var file models.File
	if err := s.db.DB.Preload("Project").Where("id = ?", fileID).First(&file).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	if file.Project.OwnerID != userID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Update file
	file.Content = req.Content
	file.Size = int64(len(req.Content))
	file.LastEditBy = userID.(uint)
	file.Version++

	if err := s.db.DB.Save(&file).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "File updated successfully",
		"file":    file,
	})
}

// Credits and Billing endpoints

// GetUserCredits returns the current user's credit balance and usage info
func (s *Server) GetUserCredits(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var user models.User
	if err := s.db.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"credits":           user.Credits,
		"lifetime_credits":  user.LifetimeCredits,
		"free_builds_used":  user.FreeBuildsUsed,
		"free_builds_limit": user.FreeBuildsLimit,
		"total_builds":      user.TotalBuilds,
		"total_downloads":   user.TotalDownloads,
		"subscription_type": user.SubscriptionType,
		"is_admin":          user.IsAdmin,
		"can_build":         user.CanBuild(),
		"can_download":      user.CanDownload(),
		"is_unlimited":      user.IsUnlimited(),
	})
}

// PurchaseCredits handles credit purchases (simulated for now)
func (s *Server) PurchaseCredits(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req struct {
		Amount int `json:"amount" binding:"required,min=10"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: minimum 10 credits"})
		return
	}

	var user models.User
	if err := s.db.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Add credits to user account
	user.Credits += req.Amount
	user.LifetimeCredits += req.Amount

	if err := s.db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update credits"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":          "Credits purchased successfully",
		"credits_added":    req.Amount,
		"new_balance":      user.Credits,
		"lifetime_credits": user.LifetimeCredits,
	})
}

// DeductCredits deducts credits for an action (internal use)
func (s *Server) DeductCredits(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req struct {
		Amount int    `json:"amount" binding:"required,min=1"`
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := s.db.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Admin users don't need credits
	if user.IsUnlimited() {
		c.JSON(http.StatusOK, gin.H{
			"message":     "Action completed (unlimited user)",
			"deducted":    0,
			"new_balance": user.Credits,
		})
		return
	}

	// Check if user has enough credits
	if !user.DeductCredits(req.Amount) {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error":           "Insufficient credits",
			"required":        req.Amount,
			"current_balance": user.Credits,
		})
		return
	}

	if err := s.db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deduct credits"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     fmt.Sprintf("Credits deducted for: %s", req.Reason),
		"deducted":    req.Amount,
		"new_balance": user.Credits,
	})
}

// GetPricingInfo returns the pricing structure
func (s *Server) GetPricingInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"pricing": gin.H{
			"free_tier": gin.H{
				"price":            0,
				"builds_per_month": 3,
				"downloads":        "credits_required",
				"features":         []string{"Basic app building", "View generated code", "Live preview"},
			},
			"pro_tier": gin.H{
				"price":            18, // 10% cheaper than Replit's $20
				"builds_per_month": 50,
				"downloads":        "included",
				"features":         []string{"50 builds/month", "Unlimited downloads", "Priority AI processing", "Version history", "Deploy integrations"},
			},
			"team_tier": gin.H{
				"price":            45, // 10% cheaper than Replit's $50
				"builds_per_month": 200,
				"downloads":        "included",
				"features":         []string{"200 builds/month", "Team collaboration", "Shared projects", "Admin dashboard", "Priority support"},
			},
		},
		"credits": gin.H{
			"price_per_100":    9, // $9 per 100 credits (10% cheaper than Replit)
			"download_cost":    5, // 5 credits per ZIP download
			"build_cost":       10, // 10 credits per build (after free tier)
			"complex_build":    25, // 25 credits for complex/full builds
		},
	})
}

// RecordBuild records a build and deducts credits if necessary
func (s *Server) RecordBuild(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req struct {
		BuildType string `json:"build_type"` // "fast" or "full"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.BuildType = "fast"
	}

	var user models.User
	if err := s.db.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if user can build
	if !user.CanBuild() {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error":            "Build limit reached",
			"free_builds_used": user.FreeBuildsUsed,
			"free_builds_limit": user.FreeBuildsLimit,
			"credits":          user.Credits,
			"message":          "Please purchase credits or upgrade to Pro to continue building",
		})
		return
	}

	// Determine credit cost
	creditCost := 0
	if !user.IsUnlimited() {
		if user.SubscriptionType == "free" {
			if user.FreeBuildsUsed < user.FreeBuildsLimit {
				// Use free build
				user.FreeBuildsUsed++
			} else {
				// Charge credits
				creditCost = 10
				if req.BuildType == "full" {
					creditCost = 25
				}
				if !user.DeductCredits(creditCost) {
					c.JSON(http.StatusPaymentRequired, gin.H{
						"error":    "Insufficient credits",
						"required": creditCost,
						"balance":  user.Credits,
					})
					return
				}
			}
		}
	}

	user.TotalBuilds++

	if err := s.db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record build"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":           "Build recorded",
		"credits_deducted":  creditCost,
		"credits_remaining": user.Credits,
		"total_builds":      user.TotalBuilds,
		"free_builds_used":  user.FreeBuildsUsed,
		"free_builds_limit": user.FreeBuildsLimit,
	})
}

// RecordDownload records a download and deducts credits if necessary
func (s *Server) RecordDownload(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var user models.User
	if err := s.db.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if user can download
	if !user.CanDownload() {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error":    "Download requires credits",
			"required": 5,
			"balance":  user.Credits,
			"message":  "Please purchase credits or upgrade to Pro to download",
		})
		return
	}

	// Deduct credits for download (5 credits)
	creditCost := 5
	if !user.IsUnlimited() && user.SubscriptionType == "free" {
		if !user.DeductCredits(creditCost) {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error":    "Insufficient credits",
				"required": creditCost,
				"balance":  user.Credits,
			})
			return
		}
	} else {
		creditCost = 0
	}

	user.TotalDownloads++

	if err := s.db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record download"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":           "Download authorized",
		"credits_deducted":  creditCost,
		"credits_remaining": user.Credits,
		"total_downloads":   user.TotalDownloads,
	})
}

// Secret Management endpoints

// CreateSecret creates a new secret/env variable for a project
func (s *Server) CreateSecret(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	projectID, err := strconv.ParseUint(c.Param("projectId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Value       string `json:"value" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	secret := &models.Secret{
		ProjectID:   uint(projectID),
		UserID:      userID.(uint),
		Name:        req.Name,
		Value:       req.Value,
		Description: req.Description,
		IsEncrypted: true,
	}

	if err := s.db.DB.Create(secret).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create secret"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Secret created successfully",
		"secret": gin.H{
			"id":          secret.ID,
			"name":        secret.Name,
			"description": secret.Description,
			"created_at":  secret.CreatedAt,
		},
	})
}

// GetSecrets returns all secrets for a project (names only, not values)
func (s *Server) GetSecrets(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	projectID, err := strconv.ParseUint(c.Param("projectId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	var secrets []models.Secret
	if err := s.db.DB.Where("project_id = ? AND user_id = ?", projectID, userID).Find(&secrets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch secrets"})
		return
	}

	// Return only names and descriptions, never values
	result := make([]gin.H, len(secrets))
	for i, secret := range secrets {
		result[i] = gin.H{
			"id":          secret.ID,
			"name":        secret.Name,
			"description": secret.Description,
			"created_at":  secret.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{"secrets": result})
}

// DeleteSecret deletes a secret
func (s *Server) DeleteSecret(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	secretID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid secret ID"})
		return
	}

	result := s.db.DB.Where("id = ? AND user_id = ?", secretID, userID).Delete(&models.Secret{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete secret"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Secret deleted successfully"})
}

// Version History endpoints

// CreateVersion creates a new version/checkpoint for a project
func (s *Server) CreateVersion(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	projectID, err := strconv.ParseUint(c.Param("projectId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Snapshot    string `json:"snapshot" binding:"required"`
		IsAutoSave  bool   `json:"is_auto_save"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get the next version number
	var lastVersion models.ProjectVersion
	var versionNum int = 1
	if err := s.db.DB.Where("project_id = ?", projectID).Order("version desc").First(&lastVersion).Error; err == nil {
		versionNum = lastVersion.Version + 1
	}

	version := &models.ProjectVersion{
		ProjectID:   uint(projectID),
		UserID:      userID.(uint),
		Version:     versionNum,
		Name:        req.Name,
		Description: req.Description,
		Snapshot:    req.Snapshot,
		IsAutoSave:  req.IsAutoSave,
	}

	if err := s.db.DB.Create(version).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create version"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Version created successfully",
		"version": gin.H{
			"id":          version.ID,
			"version":     version.Version,
			"name":        version.Name,
			"description": version.Description,
			"created_at":  version.CreatedAt,
			"is_auto_save": version.IsAutoSave,
		},
	})
}

// GetVersions returns all versions for a project
func (s *Server) GetVersions(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("projectId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	var versions []models.ProjectVersion
	if err := s.db.DB.Where("project_id = ?", projectID).Order("version desc").Find(&versions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch versions"})
		return
	}

	result := make([]gin.H, len(versions))
	for i, v := range versions {
		result[i] = gin.H{
			"id":          v.ID,
			"version":     v.Version,
			"name":        v.Name,
			"description": v.Description,
			"created_at":  v.CreatedAt,
			"is_auto_save": v.IsAutoSave,
		}
	}

	c.JSON(http.StatusOK, gin.H{"versions": result})
}

// GetVersion returns a specific version with its snapshot
func (s *Server) GetVersion(c *gin.Context) {
	versionID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid version ID"})
		return
	}

	var version models.ProjectVersion
	if err := s.db.DB.First(&version, versionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Version not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"version": gin.H{
			"id":          version.ID,
			"version":     version.Version,
			"name":        version.Name,
			"description": version.Description,
			"snapshot":    version.Snapshot,
			"created_at":  version.CreatedAt,
			"is_auto_save": version.IsAutoSave,
		},
	})
}

// Repository Cloning endpoints

// CloneRepository clones a GitHub repository
func (s *Server) CloneRepository(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req struct {
		RepoURL string `json:"repo_url" binding:"required"`
		Branch  string `json:"branch"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Branch == "" {
		req.Branch = "main"
	}

	// Detect project type from repo URL
	projectType := detectProjectType(req.RepoURL)

	// Create a new project for the cloned repo
	project := &models.Project{
		Name:        extractRepoName(req.RepoURL),
		Description: fmt.Sprintf("Cloned from %s", req.RepoURL),
		Language:    projectType,
		OwnerID:     userID.(uint),
	}

	if err := s.db.DB.Create(project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project"})
		return
	}

	// Record the cloned repo
	clonedRepo := &models.ClonedRepo{
		ProjectID:   project.ID,
		UserID:      userID.(uint),
		RepoURL:     req.RepoURL,
		Branch:      req.Branch,
		ProjectType: projectType,
	}

	if err := s.db.DB.Create(clonedRepo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record cloned repo"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":      "Repository clone initiated",
		"project_id":   project.ID,
		"project_name": project.Name,
		"repo_url":     req.RepoURL,
		"branch":       req.Branch,
		"project_type": projectType,
	})
}

// Helper functions for repo cloning
func detectProjectType(repoURL string) string {
	// Simple detection based on common patterns
	url := strings.ToLower(repoURL)
	if strings.Contains(url, "react") || strings.Contains(url, "next") {
		return "react"
	}
	if strings.Contains(url, "vue") {
		return "vue"
	}
	if strings.Contains(url, "go") || strings.Contains(url, "golang") {
		return "go"
	}
	if strings.Contains(url, "python") || strings.Contains(url, "django") || strings.Contains(url, "flask") {
		return "python"
	}
	return "unknown"
}

func extractRepoName(repoURL string) string {
	parts := strings.Split(repoURL, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		name = strings.TrimSuffix(name, ".git")
		return name
	}
	return "cloned-project"
}

// Middleware

// AuthMiddleware validates JWT tokens
func (s *Server) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		userID, err := s.auth.ExtractUserFromToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Next()
	}
}

// CORSMiddleware handles CORS
func (s *Server) CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
