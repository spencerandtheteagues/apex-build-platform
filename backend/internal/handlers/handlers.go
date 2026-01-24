// APEX.BUILD API Handlers
// Production-ready REST API handlers

package handlers

import (
	"net/http"
	"strconv"

	"apex-build/internal/ai"
	"apex-build/internal/auth"
	"apex-build/internal/middleware"
	"apex-build/internal/websocket"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler contains all the dependencies for API handlers
type Handler struct {
	DB          *gorm.DB
	AIRouter    *ai.AIRouter
	AuthService *auth.AuthService
	WSHub       *websocket.Hub
}

// NewHandler creates a new handler instance
func NewHandler(db *gorm.DB, aiRouter *ai.AIRouter, authService *auth.AuthService, wsHub *websocket.Hub) *Handler {
	return &Handler{
		DB:          db,
		AIRouter:    aiRouter,
		AuthService: authService,
		WSHub:       wsHub,
	}
}

// StandardResponse represents a standard API response
type StandardResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Code    string      `json:"code,omitempty"`
	Message string      `json:"message,omitempty"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	StandardResponse
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// PaginationInfo contains pagination metadata
type PaginationInfo struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// Authentication Handlers

// Register handles user registration
func (h *Handler) Register(c *gin.Context) {
	var req auth.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Create user
	user, err := h.AuthService.CreateUser(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   err.Error(),
			Code:    "USER_CREATION_FAILED",
		})
		return
	}

	// Check if user already exists
	var existingUser models.User
	if err := h.DB.Where("email = ? OR username = ?", user.Email, user.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, StandardResponse{
			Success: false,
			Error:   "User with this email or username already exists",
			Code:    "USER_EXISTS",
		})
		return
	}

	// Save user to database
	if err := h.DB.Create(user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to create user",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Generate tokens
	tokens, err := h.AuthService.GenerateTokens(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to generate authentication tokens",
			Code:    "TOKEN_GENERATION_FAILED",
		})
		return
	}

	c.JSON(http.StatusCreated, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"user": map[string]interface{}{
				"id":       user.ID,
				"username": user.Username,
				"email":    user.Email,
				"full_name": user.FullName,
				"subscription_type": user.SubscriptionType,
			},
			"tokens": tokens,
		},
		Message: "User registered successfully",
	})
}

// Login handles user authentication
func (h *Handler) Login(c *gin.Context) {
	var req auth.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Find user by username or email
	var user models.User
	if err := h.DB.Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusUnauthorized, StandardResponse{
				Success: false,
				Error:   "Invalid credentials",
				Code:    "INVALID_CREDENTIALS",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Check if user is active
	if !user.IsActive {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "Account is disabled",
			Code:    "ACCOUNT_DISABLED",
		})
		return
	}

	// Verify password
	if err := h.AuthService.CheckPassword(req.Password, user.PasswordHash); err != nil {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "Invalid credentials",
			Code:    "INVALID_CREDENTIALS",
		})
		return
	}

	// Generate tokens
	tokens, err := h.AuthService.GenerateTokens(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to generate authentication tokens",
			Code:    "TOKEN_GENERATION_FAILED",
		})
		return
	}

	// Update last login
	h.DB.Model(&user).Updates(models.User{})

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"user": map[string]interface{}{
				"id":       user.ID,
				"username": user.Username,
				"email":    user.Email,
				"full_name": user.FullName,
				"subscription_type": user.SubscriptionType,
				"preferred_theme": user.PreferredTheme,
				"preferred_ai": user.PreferredAI,
			},
			"tokens": tokens,
		},
		Message: "Login successful",
	})
}

// RefreshToken handles token refresh
func (h *Handler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Refresh token is required",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Validate refresh token
	claims, err := h.AuthService.ValidateToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "Invalid refresh token",
			Code:    "INVALID_REFRESH_TOKEN",
		})
		return
	}

	// Get user from database
	var user models.User
	if err := h.DB.First(&user, claims.UserID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not found",
			Code:    "USER_NOT_FOUND",
		})
		return
	}

	// Generate new tokens
	tokens, err := h.AuthService.RefreshTokens(req.RefreshToken, &user)
	if err != nil {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "Failed to refresh tokens",
			Code:    "TOKEN_REFRESH_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data:    tokens,
		Message: "Tokens refreshed successfully",
	})
}

// Logout handles user logout
func (h *Handler) Logout(c *gin.Context) {
	// In a real implementation, you might want to blacklist the token
	// For now, we'll just return success
	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Logged out successfully",
	})
}

// User Profile Handlers

// GetProfile returns the current user's profile
func (h *Handler) GetProfile(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	var user models.User
	if err := h.DB.Preload("Projects").First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "User not found",
				Code:    "USER_NOT_FOUND",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"id":                 user.ID,
			"username":           user.Username,
			"email":             user.Email,
			"full_name":         user.FullName,
			"avatar_url":        user.AvatarURL,
			"subscription_type": user.SubscriptionType,
			"subscription_end":  user.SubscriptionEnd,
			"is_verified":       user.IsVerified,
			"preferred_theme":   user.PreferredTheme,
			"preferred_ai":      user.PreferredAI,
			"monthly_ai_requests": user.MonthlyAIRequests,
			"monthly_ai_cost":   user.MonthlyAICost,
			"created_at":        user.CreatedAt,
			"project_count":     len(user.Projects),
		},
	})
}

// UpdateProfile updates the current user's profile
func (h *Handler) UpdateProfile(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	var req struct {
		FullName       *string `json:"full_name"`
		AvatarURL      *string `json:"avatar_url"`
		PreferredTheme *string `json:"preferred_theme"`
		PreferredAI    *string `json:"preferred_ai"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Prepare updates
	updates := make(map[string]interface{})
	if req.FullName != nil {
		updates["full_name"] = *req.FullName
	}
	if req.AvatarURL != nil {
		updates["avatar_url"] = *req.AvatarURL
	}
	if req.PreferredTheme != nil {
		// Validate theme
		validThemes := []string{"cyberpunk", "matrix", "synthwave", "neonCity"}
		valid := false
		for _, theme := range validThemes {
			if *req.PreferredTheme == theme {
				valid = true
				break
			}
		}
		if !valid {
			c.JSON(http.StatusBadRequest, StandardResponse{
				Success: false,
				Error:   "Invalid theme. Must be one of: cyberpunk, matrix, synthwave, neonCity",
				Code:    "INVALID_THEME",
			})
			return
		}
		updates["preferred_theme"] = *req.PreferredTheme
	}
	if req.PreferredAI != nil {
		// Validate AI preference
		validAIs := []string{"auto", "claude", "gpt4", "gemini"}
		valid := false
		for _, ai := range validAIs {
			if *req.PreferredAI == ai {
				valid = true
				break
			}
		}
		if !valid {
			c.JSON(http.StatusBadRequest, StandardResponse{
				Success: false,
				Error:   "Invalid AI preference. Must be one of: auto, claude, gpt4, gemini",
				Code:    "INVALID_AI_PREFERENCE",
			})
			return
		}
		updates["preferred_ai"] = *req.PreferredAI
	}

	// Update user
	if err := h.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to update profile",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Profile updated successfully",
	})
}

// Helper function for pagination
func paginate(page, limit int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if page <= 0 {
			page = 1
		}
		if limit <= 0 || limit > 100 {
			limit = 10
		}

		offset := (page - 1) * limit
		return db.Offset(offset).Limit(limit)
	}
}

// Helper function to get pagination info
func getPaginationInfo(page, limit int, total int64) *PaginationInfo {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))

	return &PaginationInfo{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

// Helper function to parse pagination parameters
func parsePaginationParams(c *gin.Context) (int, int) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 10
	}

	return page, limit
}