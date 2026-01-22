package api

import (
	"net/http"

	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
)

// GetUserProfile returns the current user's profile
func (s *Server) GetUserProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var user models.User
	if err := s.db.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Don't return sensitive information
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":                    user.ID,
			"username":              user.Username,
			"email":                user.Email,
			"full_name":             user.FullName,
			"avatar_url":            user.AvatarURL,
			"is_verified":           user.IsVerified,
			"subscription_type":     user.SubscriptionType,
			"subscription_end":      user.SubscriptionEnd,
			"monthly_ai_requests":   user.MonthlyAIRequests,
			"monthly_ai_cost":       user.MonthlyAICost,
			"preferred_theme":       user.PreferredTheme,
			"preferred_ai":          user.PreferredAI,
			"created_at":            user.CreatedAt,
			"updated_at":            user.UpdatedAt,
		},
	})
}

// UpdateUserProfile updates the current user's profile
func (s *Server) UpdateUserProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var req struct {
		FullName       string `json:"full_name"`
		AvatarURL      string `json:"avatar_url"`
		PreferredTheme string `json:"preferred_theme"`
		PreferredAI    string `json:"preferred_ai"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate theme
	validThemes := map[string]bool{
		"cyberpunk": true,
		"matrix":    true,
		"synthwave": true,
		"neonCity":  true,
	}

	if req.PreferredTheme != "" && !validThemes[req.PreferredTheme] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid theme selection"})
		return
	}

	// Validate AI preference
	validAI := map[string]bool{
		"auto":   true,
		"claude": true,
		"gpt4":   true,
		"gemini": true,
	}

	if req.PreferredAI != "" && !validAI[req.PreferredAI] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid AI preference"})
		return
	}

	// Find and update user
	var user models.User
	if err := s.db.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Update fields if provided
	if req.FullName != "" {
		user.FullName = req.FullName
	}
	if req.AvatarURL != "" {
		user.AvatarURL = req.AvatarURL
	}
	if req.PreferredTheme != "" {
		user.PreferredTheme = req.PreferredTheme
	}
	if req.PreferredAI != "" {
		user.PreferredAI = req.PreferredAI
	}

	if err := s.db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Profile updated successfully",
		"user": gin.H{
			"id":               user.ID,
			"username":         user.Username,
			"email":           user.Email,
			"full_name":       user.FullName,
			"avatar_url":      user.AvatarURL,
			"preferred_theme": user.PreferredTheme,
			"preferred_ai":    user.PreferredAI,
		},
	})
}