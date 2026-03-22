package api

import (
	"net/http"
	"strings"

	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const backendSubscriptionRequiredCode = "BACKEND_SUBSCRIPTION_REQUIRED"

func currentSubscriptionType(c *gin.Context, db *gorm.DB, userID uint) string {
	if plan, ok := c.Get("subscription_type"); ok {
		if planType, ok := plan.(string); ok && strings.TrimSpace(planType) != "" {
			return strings.ToLower(strings.TrimSpace(planType))
		}
	}

	if db == nil || userID == 0 {
		return "free"
	}

	var user models.User
	if err := db.Select("subscription_type").First(&user, userID).Error; err != nil {
		return "free"
	}

	planType := strings.ToLower(strings.TrimSpace(user.SubscriptionType))
	if planType == "" {
		return "free"
	}
	return planType
}

func hasPaidBackendPlan(c *gin.Context, db *gorm.DB, userID uint) bool {
	if bypassBilling, ok := c.Get("bypass_billing"); ok {
		if bypass, ok := bypassBilling.(bool); ok && bypass {
			return true
		}
	}
	if hasUnlimited, ok := c.Get("has_unlimited_credits"); ok {
		if unlimited, ok := hasUnlimited.(bool); ok && unlimited {
			return true
		}
	}

	switch currentSubscriptionType(c, db, userID) {
	case "builder", "pro", "team", "enterprise", "owner":
		return true
	default:
		return false
	}
}

func requirePaidBackendPlan(c *gin.Context, db *gorm.DB, userID uint, feature string) bool {
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return false
	}
	if hasPaidBackendPlan(c, db, userID) {
		return true
	}

	planType := currentSubscriptionType(c, db, userID)
	c.JSON(http.StatusPaymentRequired, gin.H{
		"error":          feature + " requires a paid subscription",
		"error_code":     backendSubscriptionRequiredCode,
		"current_plan":   planType,
		"required_plan":  "builder",
		"blocked_reason": feature,
		"suggestion":     "Free accounts can build static frontend websites. Upgrade to Builder or higher to unlock backend previews, databases, deployments, and full-stack app generation.",
	})
	return false
}
