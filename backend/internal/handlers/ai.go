// APEX.BUILD AI Handlers
// AI generation, analysis, and management endpoints

package handlers

import (
	"context"
	"encoding/json"
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

// CodeReviewRequest represents a request for AI-powered code review
type CodeReviewRequest struct {
	Code               string `json:"code" binding:"required"`
	Language           string `json:"language,omitempty"`
	Context            string `json:"context,omitempty"`
	FileName           string `json:"file_name,omitempty"`
	CheckBugs          *bool  `json:"check_bugs,omitempty"`
	CheckSecurity      *bool  `json:"check_security,omitempty"`
	CheckPerformance   *bool  `json:"check_performance,omitempty"`
	CheckStyle         *bool  `json:"check_style,omitempty"`
	CheckBestPractices *bool  `json:"check_best_practices,omitempty"`
}

// ReviewCode handles AI-powered code review requests
func (h *Handler) ReviewCode(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	var req CodeReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format. Code is required.",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Validate code length
	if len(req.Code) > ai.MaxCodeLength {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Code exceeds maximum length of 50,000 characters",
			Code:    "CODE_TOO_LONG",
		})
		return
	}

	if strings.TrimSpace(req.Code) == "" {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Code cannot be empty",
			Code:    "EMPTY_CODE",
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
				"current_usage": user.MonthlyAIRequests,
				"monthly_limit": monthlyLimit,
				"subscription":  user.SubscriptionType,
			},
		})
		return
	}

	// Build the review prompt for AI
	prompt := buildCodeReviewPrompt(req)

	// Create AI request
	aiReq := &ai.AIRequest{
		ID:          uuid.New().String(),
		Capability:  ai.CapabilityCodeReview,
		Prompt:      prompt,
		Code:        req.Code,
		Language:    req.Language,
		UserID:      strconv.Itoa(int(userID)),
		CreatedAt:   time.Now(),
		MaxTokens:   4000,
		Temperature: 0.3, // Lower temperature for consistent analysis
	}

	// Make AI request with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	startTime := time.Now()
	aiResp, err := h.AIRouter.Generate(ctx, aiReq)
	duration := time.Since(startTime)

	if err != nil {
		// Log the failed request
		h.logAIRequest(userID, aiReq, nil, err, duration)

		errorMsg := "Code review failed. Please try again."
		errorCode := "CODE_REVIEW_FAILED"

		errStr := err.Error()
		if containsIgnoreCase(errStr, "rate limit") || containsIgnoreCase(errStr, "429") {
			errorMsg = "AI service is currently busy. Please try again in a moment."
			errorCode = "RATE_LIMITED"
		} else if containsIgnoreCase(errStr, "timeout") || containsIgnoreCase(errStr, "deadline") {
			errorMsg = "Request timed out. Please try with a smaller code snippet."
			errorCode = "TIMEOUT"
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

	// Parse the AI response into structured findings
	findings, summary, score := parseCodeReviewResponse(aiResp.Content, req.Code)

	// Calculate statistics
	stats := calculateReviewStats(findings)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"findings": findings,
			"summary":  summary,
			"score":    score,
			"stats":    stats,
			"duration": duration.Milliseconds(),
			"provider": aiResp.Provider,
		},
		Message: "Code review completed successfully",
	})
}

// ReviewFinding represents a single finding from the code review
type ReviewFinding struct {
	Type       string `json:"type"`
	Severity   string `json:"severity"`
	Line       int    `json:"line"`
	EndLine    int    `json:"end_line,omitempty"`
	Column     int    `json:"column,omitempty"`
	EndColumn  int    `json:"end_column,omitempty"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
	Code       string `json:"code,omitempty"`
	RuleID     string `json:"rule_id,omitempty"`
}

// ReviewStats provides statistics about the review findings
type ReviewStats struct {
	TotalFindings int            `json:"total_findings"`
	Errors        int            `json:"errors"`
	Warnings      int            `json:"warnings"`
	Info          int            `json:"info"`
	ByType        map[string]int `json:"by_type"`
}

// buildCodeReviewPrompt constructs the prompt for the AI code review
func buildCodeReviewPrompt(req CodeReviewRequest) string {
	var sb strings.Builder

	language := req.Language
	if language == "" {
		language = "code"
	}

	sb.WriteString("You are an expert code reviewer. Analyze the following ")
	sb.WriteString(language)
	sb.WriteString(" code and identify issues.\n\n")

	sb.WriteString("Review the code for:\n")

	// Default to checking all if none specified
	checkAll := (req.CheckBugs == nil || !*req.CheckBugs) &&
		(req.CheckSecurity == nil || !*req.CheckSecurity) &&
		(req.CheckPerformance == nil || !*req.CheckPerformance) &&
		(req.CheckStyle == nil || !*req.CheckStyle) &&
		(req.CheckBestPractices == nil || !*req.CheckBestPractices)

	if checkAll || (req.CheckBugs != nil && *req.CheckBugs) {
		sb.WriteString("- BUGS: Logic errors, null/undefined references, off-by-one errors, race conditions\n")
	}
	if checkAll || (req.CheckSecurity != nil && *req.CheckSecurity) {
		sb.WriteString("- SECURITY: SQL injection, XSS, CSRF, insecure data handling, hardcoded secrets\n")
	}
	if checkAll || (req.CheckPerformance != nil && *req.CheckPerformance) {
		sb.WriteString("- PERFORMANCE: Inefficient algorithms, memory leaks, unnecessary computations, N+1 queries\n")
	}
	if checkAll || (req.CheckStyle != nil && *req.CheckStyle) {
		sb.WriteString("- STYLE: Code formatting, naming conventions, readability issues\n")
	}
	if checkAll || (req.CheckBestPractices != nil && *req.CheckBestPractices) {
		sb.WriteString("- BEST PRACTICES: Design patterns, error handling, code organization, maintainability\n")
	}

	if req.Context != "" {
		sb.WriteString("\nContext (surrounding code):\n```\n")
		sb.WriteString(req.Context)
		sb.WriteString("\n```\n")
	}

	sb.WriteString("\nCode to review:\n```")
	sb.WriteString(language)
	sb.WriteString("\n")
	sb.WriteString(req.Code)
	sb.WriteString("\n```\n\n")

	sb.WriteString(`Respond in the following JSON format ONLY (no markdown, no explanation outside JSON):
{
  "findings": [
    {
      "type": "bug|security|performance|style|best_practice|deprecation|complexity",
      "severity": "error|warning|info",
      "line": <line_number>,
      "end_line": <end_line_number_or_same_as_line>,
      "message": "<clear description of the issue>",
      "suggestion": "<how to fix it with code example if applicable>",
      "code": "<the problematic code snippet>"
    }
  ],
  "summary": "<1-2 sentence overall assessment>",
  "score": <0-100 quality score>
}

Important:
- Line numbers must be accurate based on the code provided (1-indexed)
- Only include real issues, not stylistic preferences unless they affect readability
- Severity: error = must fix, warning = should fix, info = consider fixing
- Be specific in messages and suggestions
- Include code examples in suggestions when helpful`)

	return sb.String()
}

// parseCodeReviewResponse parses the AI response into structured findings
func parseCodeReviewResponse(content string, originalCode string) ([]ReviewFinding, string, int) {
	content = strings.TrimSpace(content)

	// Remove markdown code blocks if present
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	// Parse the JSON response
	var response struct {
		Findings []ReviewFinding `json:"findings"`
		Summary  string          `json:"summary"`
		Score    int             `json:"score"`
	}

	if err := json.Unmarshal([]byte(content), &response); err != nil {
		// Return empty findings if parsing fails
		return []ReviewFinding{}, "Code review completed but response could not be parsed.", 70
	}

	// Validate and normalize findings
	codeLines := strings.Split(originalCode, "\n")
	maxLine := len(codeLines)

	validFindings := make([]ReviewFinding, 0, len(response.Findings))
	for _, f := range response.Findings {
		// Validate line numbers
		if f.Line < 1 {
			f.Line = 1
		}
		if f.Line > maxLine {
			f.Line = maxLine
		}
		if f.EndLine < f.Line {
			f.EndLine = f.Line
		}
		if f.EndLine > maxLine {
			f.EndLine = maxLine
		}

		// Validate severity
		if f.Severity == "" {
			f.Severity = "warning"
		}

		// Validate type
		if f.Type == "" {
			f.Type = "best_practice"
		}

		validFindings = append(validFindings, f)
	}

	// Validate score
	score := response.Score
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	summary := response.Summary
	if summary == "" {
		if len(validFindings) == 0 {
			summary = "No issues found. The code looks good!"
		} else {
			summary = "Found " + strconv.Itoa(len(validFindings)) + " issues in the code."
		}
	}

	return validFindings, summary, score
}

// calculateReviewStats calculates statistics from the findings
func calculateReviewStats(findings []ReviewFinding) ReviewStats {
	stats := ReviewStats{
		TotalFindings: len(findings),
		ByType:        make(map[string]int),
	}

	for _, f := range findings {
		switch f.Severity {
		case "error":
			stats.Errors++
		case "warning":
			stats.Warnings++
		case "info":
			stats.Info++
		}
		stats.ByType[f.Type]++
	}

	return stats
}