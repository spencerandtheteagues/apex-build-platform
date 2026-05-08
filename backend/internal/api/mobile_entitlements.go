package api

import (
	"net/http"
	"time"

	"apex-build/internal/mobile"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	mobileBuildPlanRequiredCode       = "MOBILE_BUILD_PLAN_REQUIRED"
	mobileBuildQuotaExceededCode      = "MOBILE_BUILD_QUOTA_EXCEEDED"
	mobileSubmissionPlanRequiredCode  = "MOBILE_SUBMISSION_PLAN_REQUIRED"
	mobileSubmissionQuotaExceededCode = "MOBILE_SUBMISSION_QUOTA_EXCEEDED"
)

func hasBillingBypass(c *gin.Context) bool {
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
	return false
}

func requireMobileBuildEntitlement(c *gin.Context, db *gorm.DB, userID uint, platform mobile.MobilePlatform) bool {
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return false
	}
	if hasBillingBypass(c) {
		return true
	}

	planType := currentSubscriptionType(c, db, userID)
	allowed, requiredPlan, monthlyLimit := mobileBuildEntitlementForPlan(planType, platform)
	if !allowed {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error":         "Native mobile builds require a paid mobile-enabled plan",
			"code":          mobileBuildPlanRequiredCode,
			"current_plan":  planType,
			"required_plan": requiredPlan,
			"platform":      platform,
			"suggestion":    "Free accounts can generate and export mobile source. Builder unlocks Android builds; Pro or higher unlocks iOS builds.",
		})
		return false
	}
	if monthlyLimit < 0 || db == nil {
		return true
	}
	current, err := countMonthlyMobileBuildAttempts(db, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to evaluate mobile build quota", "code": "MOBILE_BUILD_QUOTA_CHECK_FAILED"})
		return false
	}
	if current >= monthlyLimit {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":         "Monthly native mobile build quota exceeded",
			"code":          mobileBuildQuotaExceededCode,
			"current_plan":  planType,
			"monthly_limit": monthlyLimit,
			"current_usage": current,
			"suggestion":    "Wait for the next billing cycle or upgrade before queueing more native Android/iOS builds.",
		})
		return false
	}
	return true
}

func requireMobileSubmissionEntitlement(c *gin.Context, db *gorm.DB, userID uint, platform mobile.MobilePlatform) bool {
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return false
	}
	if hasBillingBypass(c) {
		return true
	}

	planType := currentSubscriptionType(c, db, userID)
	allowed, requiredPlan, monthlyLimit := mobileSubmissionEntitlementForPlan(planType, platform)
	if !allowed {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error":         "Mobile store-upload workflows require Pro or higher",
			"code":          mobileSubmissionPlanRequiredCode,
			"current_plan":  planType,
			"required_plan": requiredPlan,
			"platform":      platform,
			"suggestion":    "Builder users can export mobile source and run Android builds. Pro or higher unlocks gated store-upload workflows.",
		})
		return false
	}
	if monthlyLimit < 0 || db == nil {
		return true
	}
	current, err := countMonthlyMobileSubmissionAttempts(db, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to evaluate mobile submission quota", "code": "MOBILE_SUBMISSION_QUOTA_CHECK_FAILED"})
		return false
	}
	if current >= monthlyLimit {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":         "Monthly mobile store-upload quota exceeded",
			"code":          mobileSubmissionQuotaExceededCode,
			"current_plan":  planType,
			"monthly_limit": monthlyLimit,
			"current_usage": current,
			"suggestion":    "Wait for the next billing cycle or upgrade before uploading more mobile builds to store pipelines.",
		})
		return false
	}
	return true
}

func mobileBuildEntitlementForPlan(planType string, platform mobile.MobilePlatform) (bool, string, int64) {
	switch platform {
	case mobile.MobilePlatformIOS:
		switch planType {
		case "pro":
			return true, "pro", 20
		case "team", "enterprise", "owner":
			return true, "pro", -1
		default:
			return false, "pro", 0
		}
	default:
		switch planType {
		case "builder":
			return true, "builder", 5
		case "pro":
			return true, "builder", 20
		case "team", "enterprise", "owner":
			return true, "builder", -1
		default:
			return false, "builder", 0
		}
	}
}

func mobileSubmissionEntitlementForPlan(planType string, _ mobile.MobilePlatform) (bool, string, int64) {
	switch planType {
	case "pro":
		return true, "pro", 5
	case "team":
		return true, "pro", 25
	case "enterprise", "owner":
		return true, "pro", -1
	default:
		return false, "pro", 0
	}
}

func countMonthlyMobileBuildAttempts(db *gorm.DB, userID uint) (int64, error) {
	start := monthStartUTC(time.Now())
	var count int64
	err := db.Model(&mobile.MobileBuildRecord{}).
		Where("user_id = ? AND created_at >= ?", userID, start).
		Count(&count).Error
	return count, err
}

func countMonthlyMobileSubmissionAttempts(db *gorm.DB, userID uint) (int64, error) {
	start := monthStartUTC(time.Now())
	var count int64
	err := db.Model(&mobile.MobileSubmissionRecord{}).
		Where("user_id = ? AND created_at >= ?", userID, start).
		Count(&count).Error
	return count, err
}

func monthStartUTC(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
}
