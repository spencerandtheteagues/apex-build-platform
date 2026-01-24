package api

import (
	"net/http"
	"strconv"
	"time"

	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
)

// AdminMiddleware checks if the user is an admin
func (s *Server) AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		var user models.User
		if err := s.db.DB.First(&user, userID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		if !user.IsAdmin && !user.IsSuperAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}

		c.Set("admin_user", user)
		c.Next()
	}
}

// AdminDashboard returns admin dashboard statistics
func (s *Server) AdminDashboard(c *gin.Context) {
	// Get user counts
	var totalUsers int64
	var activeUsers int64
	var adminUsers int64
	var proUsers int64

	s.db.DB.Model(&models.User{}).Count(&totalUsers)
	s.db.DB.Model(&models.User{}).Where("is_active = ?", true).Count(&activeUsers)
	s.db.DB.Model(&models.User{}).Where("is_admin = ?", true).Count(&adminUsers)
	s.db.DB.Model(&models.User{}).Where("subscription_type = ?", "pro").Count(&proUsers)

	// Get project counts
	var totalProjects int64
	var activeProjects int64
	s.db.DB.Model(&models.Project{}).Count(&totalProjects)
	s.db.DB.Model(&models.Project{}).Where("is_archived = ?", false).Count(&activeProjects)

	// Get AI usage stats
	var totalAIRequests int64
	var totalAICost float64
	s.db.DB.Model(&models.AIRequest{}).Count(&totalAIRequests)
	s.db.DB.Model(&models.AIRequest{}).Select("COALESCE(SUM(cost), 0)").Scan(&totalAICost)

	// Get recent activity
	var recentUsers []models.User
	s.db.DB.Order("created_at DESC").Limit(10).Find(&recentUsers)

	c.JSON(http.StatusOK, gin.H{
		"stats": gin.H{
			"users": gin.H{
				"total":  totalUsers,
				"active": activeUsers,
				"admins": adminUsers,
				"pro":    proUsers,
			},
			"projects": gin.H{
				"total":  totalProjects,
				"active": activeProjects,
			},
			"ai": gin.H{
				"total_requests": totalAIRequests,
				"total_cost":     totalAICost,
			},
		},
		"recent_users": recentUsers,
		"server_time":  time.Now(),
	})
}

// AdminGetUsers returns all users for admin management
func (s *Server) AdminGetUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	search := c.Query("search")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	offset := (page - 1) * limit

	var users []models.User
	var total int64

	query := s.db.DB.Model(&models.User{})

	if search != "" {
		query = query.Where("username LIKE ? OR email LIKE ? OR full_name LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	query.Count(&total)
	query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&users)

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// AdminGetUser returns a specific user's details
func (s *Server) AdminGetUser(c *gin.Context) {
	userID := c.Param("id")

	var user models.User
	if err := s.db.DB.Preload("Projects").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get user's AI usage
	var aiRequests []models.AIRequest
	s.db.DB.Where("user_id = ?", userID).Order("created_at DESC").Limit(20).Find(&aiRequests)

	c.JSON(http.StatusOK, gin.H{
		"user":        user,
		"ai_requests": aiRequests,
	})
}

// AdminUpdateUser updates a user's details and privileges
func (s *Server) AdminUpdateUser(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		IsActive            *bool    `json:"is_active"`
		IsVerified          *bool    `json:"is_verified"`
		IsAdmin             *bool    `json:"is_admin"`
		IsSuperAdmin        *bool    `json:"is_super_admin"`
		HasUnlimitedCredits *bool    `json:"has_unlimited_credits"`
		BypassBilling       *bool    `json:"bypass_billing"`
		BypassRateLimits    *bool    `json:"bypass_rate_limits"`
		SubscriptionType    *string  `json:"subscription_type"`
		CreditBalance       *float64 `json:"credit_balance"`
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

	// Check if trying to modify super admin (only super admins can modify other super admins)
	adminUser, _ := c.Get("admin_user")
	currentAdmin := adminUser.(models.User)

	if user.IsSuperAdmin && !currentAdmin.IsSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot modify super admin"})
		return
	}

	// Apply updates
	updates := make(map[string]interface{})

	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.IsVerified != nil {
		updates["is_verified"] = *req.IsVerified
	}
	if req.IsAdmin != nil && currentAdmin.IsSuperAdmin {
		updates["is_admin"] = *req.IsAdmin
	}
	if req.IsSuperAdmin != nil && currentAdmin.IsSuperAdmin {
		updates["is_super_admin"] = *req.IsSuperAdmin
	}
	if req.HasUnlimitedCredits != nil {
		updates["has_unlimited_credits"] = *req.HasUnlimitedCredits
	}
	if req.BypassBilling != nil {
		updates["bypass_billing"] = *req.BypassBilling
	}
	if req.BypassRateLimits != nil {
		updates["bypass_rate_limits"] = *req.BypassRateLimits
	}
	if req.SubscriptionType != nil {
		updates["subscription_type"] = *req.SubscriptionType
	}
	if req.CreditBalance != nil {
		updates["credit_balance"] = *req.CreditBalance
	}

	if err := s.db.DB.Model(&user).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	// Reload user
	s.db.DB.First(&user, userID)

	c.JSON(http.StatusOK, gin.H{
		"message": "User updated successfully",
		"user":    user,
	})
}

// AdminDeleteUser deletes a user (soft delete)
func (s *Server) AdminDeleteUser(c *gin.Context) {
	userID := c.Param("id")

	var user models.User
	if err := s.db.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Prevent deleting super admins
	adminUser, _ := c.Get("admin_user")
	currentAdmin := adminUser.(models.User)

	if user.IsSuperAdmin && !currentAdmin.IsSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete super admin"})
		return
	}

	if err := s.db.DB.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User deleted successfully",
	})
}

// AdminGetSystemStats returns detailed system statistics
func (s *Server) AdminGetSystemStats(c *gin.Context) {
	// AI provider usage
	var claudeRequests int64
	var gptRequests int64
	var geminiRequests int64

	s.db.DB.Model(&models.AIRequest{}).Where("provider = ?", "claude").Count(&claudeRequests)
	s.db.DB.Model(&models.AIRequest{}).Where("provider = ?", "gpt4").Count(&gptRequests)
	s.db.DB.Model(&models.AIRequest{}).Where("provider = ?", "gemini").Count(&geminiRequests)

	// Subscription breakdown
	var freeUsers int64
	var proUsers int64
	var teamUsers int64
	var enterpriseUsers int64
	var ownerUsers int64

	s.db.DB.Model(&models.User{}).Where("subscription_type = ?", "free").Count(&freeUsers)
	s.db.DB.Model(&models.User{}).Where("subscription_type = ?", "pro").Count(&proUsers)
	s.db.DB.Model(&models.User{}).Where("subscription_type = ?", "team").Count(&teamUsers)
	s.db.DB.Model(&models.User{}).Where("subscription_type = ?", "enterprise").Count(&enterpriseUsers)
	s.db.DB.Model(&models.User{}).Where("subscription_type = ?", "owner").Count(&ownerUsers)

	// File and storage stats
	var totalFiles int64
	var totalStorage int64

	s.db.DB.Model(&models.File{}).Count(&totalFiles)
	s.db.DB.Model(&models.File{}).Select("COALESCE(SUM(size), 0)").Scan(&totalStorage)

	c.JSON(http.StatusOK, gin.H{
		"ai_providers": gin.H{
			"claude": claudeRequests,
			"gpt4":   gptRequests,
			"gemini": geminiRequests,
		},
		"subscriptions": gin.H{
			"free":       freeUsers,
			"pro":        proUsers,
			"team":       teamUsers,
			"enterprise": enterpriseUsers,
			"owner":      ownerUsers,
		},
		"storage": gin.H{
			"total_files": totalFiles,
			"total_bytes": totalStorage,
		},
		"timestamp": time.Now(),
	})
}

// AdminAddCredits adds credits to a user's account
func (s *Server) AdminAddCredits(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		Amount float64 `json:"amount" binding:"required"`
		Reason string  `json:"reason"`
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

	user.CreditBalance += req.Amount

	if err := s.db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add credits"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Credits added successfully",
		"new_balance": user.CreditBalance,
	})
}
