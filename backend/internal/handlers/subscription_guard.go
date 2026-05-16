package handlers

import (
	"net/http"
	"strings"

	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const backendSubscriptionRequiredCode = "BACKEND_SUBSCRIPTION_REQUIRED"
const byokSubscriptionRequiredCode = "BYOK_SUBSCRIPTION_REQUIRED"

func currentSubscriptionType(c *gin.Context, db *gorm.DB, userID uint) string {
	if db != nil && userID != 0 {
		var user models.User
		if err := db.Select("subscription_type").First(&user, userID).Error; err == nil {
			planType := strings.ToLower(strings.TrimSpace(user.SubscriptionType))
			if planType != "" {
				return planType
			}
		}
	}

	if plan, ok := c.Get("subscription_type"); ok {
		if planType, ok := plan.(string); ok && strings.TrimSpace(planType) != "" {
			return strings.ToLower(strings.TrimSpace(planType))
		}
	}
	return "free"
}

func hasPaidBackendPlan(c *gin.Context, db *gorm.DB, userID uint) bool {
	if db != nil && userID != 0 {
		var user models.User
		if err := db.Select("subscription_type", "subscription_status", "bypass_billing", "has_unlimited_credits").First(&user, userID).Error; err == nil {
			if user.BypassBilling || user.HasUnlimitedCredits {
				return true
			}
			return isActivePaidBackendPlan(user.SubscriptionType, user.SubscriptionStatus)
		}
	}

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

	return isActivePaidBackendPlan(currentSubscriptionType(c, nil, 0), currentSubscriptionStatus(c))
}

func currentSubscriptionStatus(c *gin.Context) string {
	if status, ok := c.Get("subscription_status"); ok {
		if statusType, ok := status.(string); ok && strings.TrimSpace(statusType) != "" {
			return strings.ToLower(strings.TrimSpace(statusType))
		}
	}
	return ""
}

func isActivePaidBackendPlan(planType, status string) bool {
	plan := strings.ToLower(strings.TrimSpace(planType))
	if plan == "owner" {
		return true
	}
	switch plan {
	case "builder", "pro", "team", "enterprise":
		switch strings.ToLower(strings.TrimSpace(status)) {
		case "active", "trialing":
			return true
		}
	}
	return false
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

func requirePaidBYOKPlan(c *gin.Context, db *gorm.DB, userID uint) bool {
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return false
	}
	if hasPaidBackendPlan(c, db, userID) {
		return true
	}

	planType := currentSubscriptionType(c, db, userID)
	c.JSON(http.StatusPaymentRequired, gin.H{
		"error":         "BYOK requires a paid subscription",
		"error_code":    byokSubscriptionRequiredCode,
		"current_plan":  planType,
		"required_plan": "builder",
		"suggestion":    "Bring Your Own Key is available on Builder or higher. Free accounts can build static frontend websites, but connecting personal provider keys requires a paid subscription.",
	})
	return false
}
