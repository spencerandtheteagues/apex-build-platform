// APEX.BUILD Usage API Handlers
// Endpoints for viewing current usage, history, and plan limits

package handlers

import (
	"net/http"
	"strconv"

	"apex-build/internal/middleware"
	"apex-build/internal/usage"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UsageHandlers contains all usage-related HTTP handlers
type UsageHandlers struct {
	db      *gorm.DB
	tracker *usage.Tracker
}

// NewUsageHandlers creates a new usage handlers instance
func NewUsageHandlers(db *gorm.DB, tracker *usage.Tracker) *UsageHandlers {
	return &UsageHandlers{
		db:      db,
		tracker: tracker,
	}
}

// GetTracker returns the usage tracker for middleware integration
func (h *UsageHandlers) GetTracker() *usage.Tracker {
	return h.tracker
}

// GetCurrentUsage returns the user's current usage and limits
// GET /api/v1/usage/current
func (h *UsageHandlers) GetCurrentUsage(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
			"code":    "NOT_AUTHENTICATED",
		})
		return
	}

	// Get user's subscription type
	var user models.User
	if err := h.db.Select("subscription_type", "bypass_billing", "has_unlimited_credits").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get user information",
			"code":    "DATABASE_ERROR",
		})
		return
	}

	plan := usage.PlanType(user.SubscriptionType)
	if plan == "" {
		plan = usage.PlanFree
	}

	// Check if user has unlimited access
	unlimited := user.BypassBilling || user.HasUnlimitedCredits

	// Get current usage
	currentUsage, err := h.tracker.GetCurrentUsage(c.Request.Context(), userID, plan)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get usage data",
			"code":    "USAGE_ERROR",
		})
		return
	}

	// Calculate percentages
	response := gin.H{
		"success": true,
		"data": gin.H{
			"user_id":  userID,
			"plan":     string(plan),
			"unlimited": unlimited,
			"usage": gin.H{
				"projects": gin.H{
					"current":    currentUsage.Projects,
					"limit":      currentUsage.ProjectsLimit,
					"percentage": calculatePercentage(int64(currentUsage.Projects), int64(currentUsage.ProjectsLimit)),
					"unlimited":  currentUsage.ProjectsLimit == -1 || unlimited,
				},
				"storage": gin.H{
					"current":           currentUsage.StorageBytes,
					"limit":             currentUsage.StorageLimit,
					"percentage":        calculatePercentage(currentUsage.StorageBytes, currentUsage.StorageLimit),
					"unlimited":         currentUsage.StorageLimit == -1 || unlimited,
					"current_formatted": formatBytes(currentUsage.StorageBytes),
					"limit_formatted":   formatBytes(currentUsage.StorageLimit),
				},
				"ai_requests": gin.H{
					"current":     currentUsage.AIRequests,
					"limit":       currentUsage.AIRequestsLimit,
					"percentage":  calculatePercentage(int64(currentUsage.AIRequests), int64(currentUsage.AIRequestsLimit)),
					"unlimited":   currentUsage.AIRequestsLimit == -1 || unlimited,
					"period":      "monthly",
					"period_start": currentUsage.PeriodStart,
					"period_end":   currentUsage.PeriodEnd,
				},
				"execution_minutes": gin.H{
					"current":    currentUsage.ExecutionMinutes,
					"limit":      currentUsage.ExecutionLimit,
					"percentage": calculatePercentage(int64(currentUsage.ExecutionMinutes), int64(currentUsage.ExecutionLimit)),
					"unlimited":  currentUsage.ExecutionLimit == -1 || unlimited,
					"period":     "daily",
				},
			},
			"warnings": getUsageWarnings(currentUsage, unlimited),
			"cached_at": currentUsage.CachedAt,
		},
	}

	c.JSON(http.StatusOK, response)
}

// GetUsageHistory returns historical usage data
// GET /api/v1/usage/history
func (h *UsageHandlers) GetUsageHistory(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
			"code":    "NOT_AUTHENTICATED",
		})
		return
	}

	// Parse days parameter (default 30)
	days := 30
	if daysParam := c.Query("days"); daysParam != "" {
		if d, err := strconv.Atoi(daysParam); err == nil && d > 0 && d <= 365 {
			days = d
		}
	}

	history, err := h.tracker.GetUsageHistory(c.Request.Context(), userID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get usage history",
			"code":    "HISTORY_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"user_id": userID,
			"days":    days,
			"daily":   history.Daily,
			"monthly": history.Monthly,
		},
	})
}

// GetLimits returns the limits for the user's plan and all plans
// GET /api/v1/usage/limits
func (h *UsageHandlers) GetLimits(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
			"code":    "NOT_AUTHENTICATED",
		})
		return
	}

	// Get user's subscription type
	var user models.User
	if err := h.db.Select("subscription_type").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get user information",
			"code":    "DATABASE_ERROR",
		})
		return
	}

	currentPlan := usage.PlanType(user.SubscriptionType)
	if currentPlan == "" {
		currentPlan = usage.PlanFree
	}

	// Get limits for all plans
	allPlans := map[string]usage.PlanLimits{
		"free":       usage.GetPlanLimits(usage.PlanFree),
		"pro":        usage.GetPlanLimits(usage.PlanPro),
		"team":       usage.GetPlanLimits(usage.PlanTeam),
		"enterprise": usage.GetPlanLimits(usage.PlanEnterprise),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"current_plan":   string(currentPlan),
			"current_limits": usage.GetPlanLimits(currentPlan),
			"all_plans":      allPlans,
			"pricing": gin.H{
				"free": gin.H{
					"price_monthly": 0,
					"price_yearly":  0,
				},
				"pro": gin.H{
					"price_monthly": 1200, // $12
					"price_yearly":  12000, // $120 (2 months free)
				},
				"team": gin.H{
					"price_monthly": 2900, // $29
					"price_yearly":  29000, // $290 (2 months free)
				},
				"enterprise": gin.H{
					"price_monthly": 7900, // $79
					"price_yearly":  79000, // $790 (2 months free)
				},
			},
		},
	})
}

// RefreshUsage forces a refresh of cached usage data
// POST /api/v1/usage/refresh
func (h *UsageHandlers) RefreshUsage(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
			"code":    "NOT_AUTHENTICATED",
		})
		return
	}

	if err := h.tracker.ForceRefresh(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to refresh usage cache",
			"code":    "REFRESH_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Usage cache refreshed",
	})
}

// RegisterUsageRoutes registers all usage-related routes
func (h *UsageHandlers) RegisterUsageRoutes(group *gin.RouterGroup) {
	usageGroup := group.Group("/usage")
	{
		usageGroup.GET("/current", h.GetCurrentUsage)
		usageGroup.GET("/history", h.GetUsageHistory)
		usageGroup.GET("/limits", h.GetLimits)
		usageGroup.POST("/refresh", h.RefreshUsage)
	}
}

// Helper functions

func calculatePercentage(current, limit int64) float64 {
	if limit <= 0 {
		return 0 // Unlimited or invalid
	}
	percentage := float64(current) / float64(limit) * 100
	if percentage > 100 {
		percentage = 100
	}
	return percentage
}

func formatBytes(bytes int64) string {
	if bytes == -1 {
		return "Unlimited"
	}
	const unit = 1024
	if bytes < unit {
		return strconv.FormatInt(bytes, 10) + " B"
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return strconv.FormatFloat(float64(bytes)/float64(div), 'f', 1, 64) + " " + units[exp]
}

func getUsageWarnings(usage *usage.CurrentUsage, unlimited bool) []gin.H {
	warnings := make([]gin.H, 0)

	if unlimited {
		return warnings
	}

	// Check for high usage (80%+)
	thresholds := []struct {
		name       string
		current    int64
		limit      int64
		percentage float64
	}{
		{"projects", int64(usage.Projects), int64(usage.ProjectsLimit), 80},
		{"storage", usage.StorageBytes, usage.StorageLimit, 80},
		{"ai_requests", int64(usage.AIRequests), int64(usage.AIRequestsLimit), 80},
		{"execution_minutes", int64(usage.ExecutionMinutes), int64(usage.ExecutionLimit), 80},
	}

	for _, t := range thresholds {
		if t.limit > 0 {
			percentage := calculatePercentage(t.current, t.limit)
			if percentage >= 100 {
				warnings = append(warnings, gin.H{
					"type":       t.name,
					"severity":   "critical",
					"message":    getWarningMessage(t.name, "exceeded"),
					"percentage": percentage,
				})
			} else if percentage >= 90 {
				warnings = append(warnings, gin.H{
					"type":       t.name,
					"severity":   "high",
					"message":    getWarningMessage(t.name, "high"),
					"percentage": percentage,
				})
			} else if percentage >= t.percentage {
				warnings = append(warnings, gin.H{
					"type":       t.name,
					"severity":   "warning",
					"message":    getWarningMessage(t.name, "warning"),
					"percentage": percentage,
				})
			}
		}
	}

	return warnings
}

func getWarningMessage(usageType, severity string) string {
	messages := map[string]map[string]string{
		"projects": {
			"exceeded": "You've reached your project limit. Upgrade to create more projects.",
			"high":     "You're at 90% of your project limit. Consider upgrading soon.",
			"warning":  "You're approaching your project limit.",
		},
		"storage": {
			"exceeded": "You've reached your storage limit. Upgrade for more space.",
			"high":     "You're at 90% of your storage limit. Consider upgrading soon.",
			"warning":  "You're approaching your storage limit.",
		},
		"ai_requests": {
			"exceeded": "You've used all your AI requests this month. Upgrade for more.",
			"high":     "You're at 90% of your monthly AI request limit.",
			"warning":  "You're approaching your monthly AI request limit.",
		},
		"execution_minutes": {
			"exceeded": "You've used all your execution time today. Upgrade for more.",
			"high":     "You're at 90% of your daily execution time limit.",
			"warning":  "You're approaching your daily execution time limit.",
		},
	}

	if typeMessages, ok := messages[usageType]; ok {
		if msg, ok := typeMessages[severity]; ok {
			return msg
		}
	}
	return "Usage limit warning"
}
