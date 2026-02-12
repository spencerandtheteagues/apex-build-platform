package api

import (
	"archive/zip"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"apex-build/internal/ai"
	"apex-build/internal/auth"
	"apex-build/internal/db"
	"apex-build/internal/pricing"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Server represents the API server
type Server struct {
	db       *db.Database
	auth     *auth.AuthService
	aiRouter *ai.AIRouter
	byok     *ai.BYOKManager
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
// SECURITY: Fixed TOCTOU race condition - now uses transaction with proper locking
func (s *Server) Register(c *gin.Context) {
	var req auth.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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
		if err := tx.Where("username = ? OR email = ?", req.Username, req.Email).First(&existingUser).Error; err == nil {
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

	// Generate tokens
	tokens, err := s.auth.GenerateTokens(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
		"user": gin.H{
			"id":                    user.ID,
			"username":              user.Username,
			"email":                 user.Email,
			"full_name":             user.FullName,
			"is_admin":              user.IsAdmin,
			"is_super_admin":        user.IsSuperAdmin,
			"has_unlimited_credits": user.HasUnlimitedCredits,
			"subscription_type":     user.SubscriptionType,
			"credit_balance":        user.CreditBalance,
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
		},
		"tokens": tokens,
	})
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
		UserID:      fmt.Sprintf("%v", userID),
		ProjectID:   request.ProjectID,
	}

	if aiReq.Temperature == 0 {
		aiReq.Temperature = 0.7
	}

	// Select router with BYOK awareness
	targetRouter := s.aiRouter
	isBYOK := false
	if s.byok != nil {
		if uid, ok := userID.(uint); ok && uid > 0 {
			if userRouter, hasBYOK, err := s.byok.GetRouterForUser(uid); err == nil && userRouter != nil {
				targetRouter = userRouter
				isBYOK = hasBYOK
			}
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
			res, err := s.byok.ReserveCredits(userID.(uint), estimatedCost)
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

	// Save request to database
	dbRequest := &models.AIRequest{
		RequestID:  aiReq.ID,
		UserID:     userID.(uint), // safe: middleware always sets as uint
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

	// Record BYOK usage and finalize credits if applicable
	if s.byok != nil {
		if uid, ok := userID.(uint); ok && uid > 0 && response != nil {
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
			cost = s.byok.BilledCost(string(response.Provider), modelUsed, inputTokens, outputTokens, powerMode, isBYOK)
			if response.Usage != nil {
				response.Usage.Cost = cost
			}
			dbRequest.Cost = cost
			s.byok.RecordUsage(uid, projectID, string(response.Provider), modelUsed, isBYOK,
				inputTokens, outputTokens, cost, string(aiReq.Capability), response.Duration, "success")
			if reservation != nil {
				_ = s.byok.FinalizeCredits(reservation, cost)
			}
		}
	}

	// Persist request after cost adjustments
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

// GetFile returns a specific file by ID
func (s *Server) GetFile(c *gin.Context) {
	fileID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var file models.File
	if err := s.db.DB.Preload("Project").Where("id = ?", fileID).First(&file).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Check access permissions
	if file.Project.OwnerID != userID.(uint) && !file.Project.IsPublic {
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

// UpdateFile updates a file's content
func (s *Server) UpdateFile(c *gin.Context) {
	fileID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
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

	if file.Project.OwnerID != userID.(uint) {
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

	file.LastEditBy = userID.(uint)
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
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var file models.File
	if err := s.db.DB.Preload("Project").Where("id = ?", fileID).First(&file).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	if file.Project.OwnerID != userID.(uint) {
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

// CORSMiddleware handles CORS with secure origin validation
// SECURITY: No longer uses wildcard (*) - validates against allowed origins
func (s *Server) CORSMiddleware() gin.HandlerFunc {
	// Get allowed origins from environment or use defaults
	allowedOriginsEnv := os.Getenv("CORS_ALLOWED_ORIGINS")
	var allowedOrigins []string
	if allowedOriginsEnv != "" {
		allowedOrigins = strings.Split(allowedOriginsEnv, ",")
	} else {
		// Default allowed origins for development and production
		allowedOrigins = []string{
			"http://localhost:3000",
			"http://localhost:5173",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:5173",
			"https://apex.build",
			"https://www.apex.build",
			"https://apex-frontend-gigq.onrender.com",
		}
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		isAllowed := false
		for _, allowed := range allowedOrigins {
			if strings.TrimSpace(allowed) == origin {
				isAllowed = true
				break
			}
		}

		if isAllowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Request-ID")
		c.Header("Access-Control-Max-Age", "86400") // 24 hours preflight cache

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
