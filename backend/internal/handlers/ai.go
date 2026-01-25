// APEX.BUILD AI Handlers
// AI generation, analysis, and management endpoints

package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"apex-build/internal/ai"
	"apex-build/internal/middleware"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// containsIgnoreCase is a case-insensitive substring check
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// AIGenerateRequest represents an AI generation request
type AIGenerateRequest struct {
	Prompt      string                 `json:"prompt" binding:"required"`
	Provider    string                 `json:"provider,omitempty"`
	Capability  string                 `json:"capability" binding:"required"`
	Language    string                 `json:"language,omitempty"`
	Code        string                 `json:"code,omitempty"`
	ProjectID   *uint                  `json:"project_id,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Temperature *float32               `json:"temperature,omitempty"`
	MaxTokens   *int                   `json:"max_tokens,omitempty"`
}

// GenerateAI handles AI code generation requests
func (h *Handler) GenerateAI(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	var req AIGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Validate prompt length and content
	if !utf8.ValidString(req.Prompt) {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Prompt contains invalid characters",
			Code:    "INVALID_PROMPT",
		})
		return
	}

	// Enforce prompt length limits
	if len(req.Prompt) > ai.MaxPromptLength {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Prompt exceeds maximum length of 100,000 characters",
			Code:    "PROMPT_TOO_LONG",
		})
		return
	}

	if len(req.Code) > ai.MaxCodeLength {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Code exceeds maximum length of 50,000 characters",
			Code:    "CODE_TOO_LONG",
		})
		return
	}

	// Sanitize prompt - remove potentially dangerous content
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Prompt cannot be empty",
			Code:    "EMPTY_PROMPT",
		})
		return
	}

	// Validate capability
	validCapabilities := map[string]ai.AICapability{
		"code_generation":  ai.CapabilityCodeGeneration,
		"code_review":      ai.CapabilityCodeReview,
		"code_completion":  ai.CapabilityCodeCompletion,
		"debugging":        ai.CapabilityDebugging,
		"explanation":      ai.CapabilityExplanation,
		"refactoring":      ai.CapabilityRefactoring,
		"testing":          ai.CapabilityTesting,
		"documentation":    ai.CapabilityDocumentation,
	}

	capability, valid := validCapabilities[req.Capability]
	if !valid {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid capability. Must be one of: code_generation, code_review, code_completion, debugging, explanation, refactoring, testing, documentation",
			Code:    "INVALID_CAPABILITY",
		})
		return
	}

	// Check if user has reached usage limits
	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to fetch user information",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Check monthly limits based on subscription
	monthlyLimit := getUserMonthlyLimit(user.SubscriptionType)
	if user.MonthlyAIRequests >= monthlyLimit {
		c.JSON(http.StatusTooManyRequests, StandardResponse{
			Success: false,
			Error:   "Monthly AI usage limit exceeded. Upgrade your subscription for more requests.",
			Code:    "USAGE_LIMIT_EXCEEDED",
			Data: map[string]interface{}{
				"current_usage":  user.MonthlyAIRequests,
				"monthly_limit":  monthlyLimit,
				"subscription":   user.SubscriptionType,
			},
		})
		return
	}

	// Create AI request
	aiReq := &ai.AIRequest{
		ID:          uuid.New().String(),
		Provider:    ai.AIProvider(req.Provider),
		Capability:  capability,
		Prompt:      req.Prompt,
		Code:        req.Code,
		Language:    req.Language,
		Context:     req.Context,
		UserID:      strconv.Itoa(int(userID)),
		CreatedAt:   time.Now(),
	}

	if req.ProjectID != nil {
		aiReq.ProjectID = strconv.Itoa(int(*req.ProjectID))
	}

	if req.Temperature != nil {
		aiReq.Temperature = *req.Temperature
	} else {
		aiReq.Temperature = 0.7 // Default temperature
	}

	if req.MaxTokens != nil {
		aiReq.MaxTokens = *req.MaxTokens
	}

	// Make AI request with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	startTime := time.Now()
	aiResp, err := h.AIRouter.Generate(ctx, aiReq)
	duration := time.Since(startTime)

	if err != nil {
		// Log the failed request with full error details internally
		h.logAIRequest(userID, aiReq, nil, err, duration)

		// Return a sanitized error message to the user (don't leak internal details)
		errorMsg := "AI generation failed. Please try again."
		errorCode := "AI_GENERATION_FAILED"

		// Provide user-friendly messages for common errors
		errStr := err.Error()
		if containsIgnoreCase(errStr, "rate limit") || containsIgnoreCase(errStr, "429") {
			errorMsg = "AI service is currently busy. Please try again in a moment."
			errorCode = "RATE_LIMITED"
		} else if containsIgnoreCase(errStr, "timeout") || containsIgnoreCase(errStr, "deadline") {
			errorMsg = "Request timed out. Please try with a shorter prompt."
			errorCode = "TIMEOUT"
		} else if containsIgnoreCase(errStr, "maximum length") {
			errorMsg = errStr // This is safe to show
			errorCode = "INPUT_TOO_LONG"
		}

		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   errorMsg,
			Code:    errorCode,
		})
		return
	}

	// Log the successful request
	h.logAIRequest(userID, aiReq, aiResp, nil, duration)

	// Update user usage statistics
	h.updateUserUsage(userID, aiResp.Usage)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"id":       aiResp.ID,
			"provider": aiResp.Provider,
			"content":  aiResp.Content,
			"usage":    aiResp.Usage,
			"duration": aiResp.Duration.String(),
		},
		Message: "AI generation completed successfully",
	})
}

// GetAIUsage returns the user's AI usage statistics
func (h *Handler) GetAIUsage(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	// Get user information
	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to fetch user information",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Get provider usage from AI router
	providerUsage := h.AIRouter.GetProviderUsage()

	// Get usage statistics for current month
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var monthlyStats struct {
		RequestCount int64   `json:"request_count"`
		TotalCost    float64 `json:"total_cost"`
		TotalTokens  int64   `json:"total_tokens"`
	}

	h.DB.Model(&models.AIRequest{}).
		Where("user_id = ? AND created_at >= ?", userID, startOfMonth).
		Select("COUNT(*) as request_count, COALESCE(SUM(cost), 0) as total_cost, COALESCE(SUM(tokens_used), 0) as total_tokens").
		Scan(&monthlyStats)

	monthlyLimit := getUserMonthlyLimit(user.SubscriptionType)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"monthly_usage": map[string]interface{}{
				"requests":      monthlyStats.RequestCount,
				"cost":         monthlyStats.TotalCost,
				"tokens":       monthlyStats.TotalTokens,
				"limit":        monthlyLimit,
				"remaining":    monthlyLimit - int(monthlyStats.RequestCount),
				"reset_date":   getNextMonthStart(),
			},
			"provider_usage": providerUsage,
			"subscription": map[string]interface{}{
				"type":         user.SubscriptionType,
				"end_date":     user.SubscriptionEnd,
			},
		},
	})
}

// GetAIHistory returns the user's AI request history
func (h *Handler) GetAIHistory(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	page, limit := parsePaginationParams(c)

	var requests []models.AIRequest
	var total int64

	// Get total count
	h.DB.Model(&models.AIRequest{}).Where("user_id = ?", userID).Count(&total)

	// Get paginated results
	result := h.DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Scopes(paginate(page, limit)).
		Find(&requests)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to fetch AI history",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, PaginatedResponse{
		StandardResponse: StandardResponse{
			Success: true,
			Data:    requests,
		},
		Pagination: getPaginationInfo(page, limit, total),
	})
}

// RateAIResponse allows users to rate AI responses
func (h *Handler) RateAIResponse(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	requestID := c.Param("id")
	if requestID == "" {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Request ID is required",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	var req struct {
		Rating   int     `json:"rating" binding:"required,min=1,max=5"`
		Feedback *string `json:"feedback"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format. Rating must be between 1 and 5.",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Find the AI request
	var aiRequest models.AIRequest
	if err := h.DB.Where("request_id = ? AND user_id = ?", requestID, userID).First(&aiRequest).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "AI request not found",
			Code:    "REQUEST_NOT_FOUND",
		})
		return
	}

	// Update rating
	updates := map[string]interface{}{
		"user_rating": req.Rating,
	}
	if req.Feedback != nil {
		updates["user_feedback"] = *req.Feedback
	}

	if err := h.DB.Model(&aiRequest).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to save rating",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Rating saved successfully",
	})
}

// Helper functions

// logAIRequest logs AI requests to the database
func (h *Handler) logAIRequest(userID uint, req *ai.AIRequest, resp *ai.AIResponse, err error, duration time.Duration) {
	aiRequest := models.AIRequest{
		RequestID:   req.ID,
		UserID:      userID,
		Provider:    string(req.Provider),
		Capability:  string(req.Capability),
		Prompt:      req.Prompt,
		Code:        req.Code,
		Language:    req.Language,
		Context:     req.Context,
		Duration:    duration.Milliseconds(),
		Status:      "completed",
	}

	if req.ProjectID != "" {
		if projectID, parseErr := strconv.ParseUint(req.ProjectID, 10, 32); parseErr == nil {
			projectIDUint := uint(projectID)
			aiRequest.ProjectID = &projectIDUint
		}
	}

	if resp != nil {
		aiRequest.Response = resp.Content
		if resp.Usage != nil {
			aiRequest.TokensUsed = resp.Usage.TotalTokens
			aiRequest.Cost = resp.Usage.Cost
		}
	}

	if err != nil {
		aiRequest.Status = "failed"
		aiRequest.ErrorMsg = err.Error()
	}

	h.DB.Create(&aiRequest)
}

// updateUserUsage updates user's monthly usage statistics
func (h *Handler) updateUserUsage(userID uint, usage *ai.Usage) {
	if usage == nil {
		return
	}

	updates := map[string]interface{}{
		"monthly_ai_requests": gorm.Expr("monthly_ai_requests + ?", 1),
		"monthly_ai_cost":     gorm.Expr("monthly_ai_cost + ?", usage.Cost),
	}

	h.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates)
}

// getUserMonthlyLimit returns the monthly request limit based on subscription type
func getUserMonthlyLimit(subscriptionType string) int {
	switch subscriptionType {
	case "pro":
		return 10000
	case "team":
		return 50000
	default: // free
		return 100
	}
}

// getNextMonthStart returns the start of next month (handles December correctly)
func getNextMonthStart() time.Time {
	now := time.Now()
	// Use AddDate to properly handle month overflow (December -> January)
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, 1, 0)
}