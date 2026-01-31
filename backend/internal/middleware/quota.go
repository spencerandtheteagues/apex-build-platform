// APEX.BUILD Quota Enforcement Middleware
// Production-ready middleware to enforce plan limits and protect revenue
// Returns 429 with upgrade prompt when quotas are exceeded

package middleware

import (
	"fmt"
	"net/http"
	"time"

	"apex-build/internal/usage"

	"github.com/gin-gonic/gin"
)

// QuotaExceededResponse represents the response when quota is exceeded
type QuotaExceededResponse struct {
	Error       string                 `json:"error"`
	Code        string                 `json:"code"`
	UsageType   string                 `json:"usage_type"`
	Current     int64                  `json:"current"`
	Limit       int64                  `json:"limit"`
	Plan        string                 `json:"plan"`
	UpgradeURL  string                 `json:"upgrade_url"`
	UpgradeMsg  string                 `json:"upgrade_message"`
	NextPlan    string                 `json:"next_plan,omitempty"`
	NextLimit   int64                  `json:"next_limit,omitempty"`
	ResetTime   *time.Time             `json:"reset_time,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	RequestID   string                 `json:"request_id,omitempty"`
}

// QuotaChecker holds the usage tracker for quota enforcement
type QuotaChecker struct {
	tracker *usage.Tracker
}

// NewQuotaChecker creates a new quota checker
func NewQuotaChecker(tracker *usage.Tracker) *QuotaChecker {
	return &QuotaChecker{
		tracker: tracker,
	}
}

// CheckProjectQuota middleware checks if user can create more projects
func (q *QuotaChecker) CheckProjectQuota() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip for GET requests (viewing projects doesn't need quota check)
		if c.Request.Method == http.MethodGet {
			c.Next()
			return
		}

		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		plan := q.getUserPlan(c)

		// Check if user bypasses billing (admin/owner)
		if q.bypassesBilling(c) {
			c.Next()
			return
		}

		allowed, current, limit, err := q.tracker.CheckQuota(
			c.Request.Context(),
			userID.(uint),
			plan,
			usage.UsageProjects,
			1, // Creating 1 project
		)

		if err != nil {
			// Log error but allow request (fail open for reliability)
			c.Next()
			return
		}

		if !allowed {
			q.sendQuotaExceeded(c, usage.UsageProjects, current, limit, plan)
			return
		}

		c.Next()
	}
}

// CheckStorageQuota middleware checks if user has storage quota
func (q *QuotaChecker) CheckStorageQuota(estimatedBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		plan := q.getUserPlan(c)

		if q.bypassesBilling(c) {
			c.Next()
			return
		}

		allowed, current, limit, err := q.tracker.CheckQuota(
			c.Request.Context(),
			userID.(uint),
			plan,
			usage.UsageStorageBytes,
			estimatedBytes,
		)

		if err != nil {
			c.Next()
			return
		}

		if !allowed {
			q.sendQuotaExceeded(c, usage.UsageStorageBytes, current, limit, plan)
			return
		}

		c.Next()
	}
}

// CheckAIQuota middleware checks if user has AI request quota
func (q *QuotaChecker) CheckAIQuota() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		plan := q.getUserPlan(c)

		if q.bypassesBilling(c) {
			c.Next()
			return
		}

		allowed, current, limit, err := q.tracker.CheckQuota(
			c.Request.Context(),
			userID.(uint),
			plan,
			usage.UsageAIRequests,
			1, // 1 AI request
		)

		if err != nil {
			c.Next()
			return
		}

		if !allowed {
			q.sendQuotaExceeded(c, usage.UsageAIRequests, current, limit, plan)
			return
		}

		c.Next()
	}
}

// CheckExecutionQuota middleware checks if user has execution quota
func (q *QuotaChecker) CheckExecutionQuota(estimatedMinutes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		plan := q.getUserPlan(c)

		if q.bypassesBilling(c) {
			c.Next()
			return
		}

		if estimatedMinutes < 1 {
			estimatedMinutes = 1 // Minimum 1 minute
		}

		allowed, current, limit, err := q.tracker.CheckQuota(
			c.Request.Context(),
			userID.(uint),
			plan,
			usage.UsageExecutionMinutes,
			estimatedMinutes,
		)

		if err != nil {
			c.Next()
			return
		}

		if !allowed {
			q.sendQuotaExceeded(c, usage.UsageExecutionMinutes, current, limit, plan)
			return
		}

		c.Next()
	}
}

// GenericQuotaCheck is a generic quota check that can be used for any usage type
func (q *QuotaChecker) GenericQuotaCheck(usageType usage.UsageType, amount int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		plan := q.getUserPlan(c)

		if q.bypassesBilling(c) {
			c.Next()
			return
		}

		allowed, current, limit, err := q.tracker.CheckQuota(
			c.Request.Context(),
			userID.(uint),
			plan,
			usageType,
			amount,
		)

		if err != nil {
			c.Next()
			return
		}

		if !allowed {
			q.sendQuotaExceeded(c, usageType, current, limit, plan)
			return
		}

		c.Next()
	}
}

// getUserPlan extracts the user's plan from context
func (q *QuotaChecker) getUserPlan(c *gin.Context) usage.PlanType {
	// First check if plan is already in context (set by auth middleware)
	if plan, exists := c.Get("subscription_type"); exists {
		if planStr, ok := plan.(string); ok {
			return usage.PlanType(planStr)
		}
	}

	// Default to free plan
	return usage.PlanFree
}

// bypassesBilling checks if user bypasses billing checks
func (q *QuotaChecker) bypassesBilling(c *gin.Context) bool {
	// Check for bypass_billing flag
	if bypass, exists := c.Get("bypass_billing"); exists {
		if b, ok := bypass.(bool); ok && b {
			return true
		}
	}

	// Check for admin/owner status
	if isAdmin, exists := c.Get("is_admin"); exists {
		if admin, ok := isAdmin.(bool); ok && admin {
			return true
		}
	}

	if isSuperAdmin, exists := c.Get("is_super_admin"); exists {
		if superAdmin, ok := isSuperAdmin.(bool); ok && superAdmin {
			return true
		}
	}

	// Check for unlimited credits
	if unlimited, exists := c.Get("has_unlimited_credits"); exists {
		if u, ok := unlimited.(bool); ok && u {
			return true
		}
	}

	return false
}

// sendQuotaExceeded sends a 429 response with upgrade information
func (q *QuotaChecker) sendQuotaExceeded(c *gin.Context, usageType usage.UsageType, current, limit int64, plan usage.PlanType) {
	requestID := c.GetHeader("X-Request-ID")
	if requestID == "" {
		if rid, exists := c.Get("request_id"); exists {
			requestID = rid.(string)
		}
	}

	response := QuotaExceededResponse{
		Error:      getQuotaErrorMessage(usageType, plan),
		Code:       "QUOTA_EXCEEDED",
		UsageType:  string(usageType),
		Current:    current,
		Limit:      limit,
		Plan:       string(plan),
		UpgradeURL: "/settings/billing",
		UpgradeMsg: getUpgradeMessage(usageType, plan),
		Timestamp:  time.Now().UTC(),
		RequestID:  requestID,
		Details:    make(map[string]interface{}),
	}

	// Add next plan info
	nextPlan, nextLimit := getNextPlanInfo(usageType, plan)
	if nextPlan != "" {
		response.NextPlan = nextPlan
		response.NextLimit = nextLimit
	}

	// Add reset time for time-based quotas
	if usageType == usage.UsageAIRequests {
		// Resets at start of next month
		now := time.Now().UTC()
		nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		response.ResetTime = &nextMonth
		response.Details["period"] = "monthly"
	} else if usageType == usage.UsageExecutionMinutes {
		// Resets at start of next day
		now := time.Now().UTC()
		tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
		response.ResetTime = &tomorrow
		response.Details["period"] = "daily"
	}

	// Add human-readable values
	response.Details["current_formatted"] = formatUsageValue(usageType, current)
	response.Details["limit_formatted"] = formatUsageValue(usageType, limit)

	c.AbortWithStatusJSON(http.StatusTooManyRequests, response)
}

// getQuotaErrorMessage returns a user-friendly error message
func getQuotaErrorMessage(usageType usage.UsageType, plan usage.PlanType) string {
	switch usageType {
	case usage.UsageProjects:
		return "Project limit reached. Upgrade your plan to create more projects."
	case usage.UsageStorageBytes:
		return "Storage limit reached. Upgrade your plan for more storage space."
	case usage.UsageAIRequests:
		return "Monthly AI request limit reached. Upgrade your plan for more AI capabilities."
	case usage.UsageExecutionMinutes:
		return "Daily execution time limit reached. Upgrade your plan for more execution time."
	default:
		return "Usage limit reached. Please upgrade your plan."
	}
}

// getUpgradeMessage returns a persuasive upgrade message
func getUpgradeMessage(usageType usage.UsageType, plan usage.PlanType) string {
	switch plan {
	case usage.PlanFree:
		switch usageType {
		case usage.UsageProjects:
			return "Upgrade to Pro ($12/month) for 25 projects, or Team ($29/month) for 100 projects."
		case usage.UsageStorageBytes:
			return "Upgrade to Pro ($12/month) for 5GB storage, or Team ($29/month) for 25GB."
		case usage.UsageAIRequests:
			return "Upgrade to Pro ($12/month) for 10,000 AI requests/month, or Team ($29/month) for 50,000."
		case usage.UsageExecutionMinutes:
			return "Upgrade to Pro ($12/month) for 2 hours/day execution time, or Team ($29/month) for 8 hours/day."
		}
	case usage.PlanPro:
		switch usageType {
		case usage.UsageProjects:
			return "Upgrade to Team ($29/month) for 100 projects, or Enterprise ($79/month) for unlimited."
		case usage.UsageStorageBytes:
			return "Upgrade to Team ($29/month) for 25GB storage, or Enterprise ($79/month) for unlimited."
		case usage.UsageAIRequests:
			return "Upgrade to Team ($29/month) for 50,000 AI requests/month, or Enterprise ($79/month) for unlimited."
		case usage.UsageExecutionMinutes:
			return "Upgrade to Team ($29/month) for 8 hours/day execution time, or Enterprise ($79/month) for unlimited."
		}
	case usage.PlanTeam:
		return "Upgrade to Enterprise ($79/month) for unlimited usage across all features."
	}
	return "Contact sales for a custom enterprise plan."
}

// getNextPlanInfo returns info about the next tier
func getNextPlanInfo(usageType usage.UsageType, currentPlan usage.PlanType) (string, int64) {
	nextPlan := getNextPlan(currentPlan)
	if nextPlan == "" {
		return "", 0
	}

	limits := usage.GetPlanLimits(usage.PlanType(nextPlan))
	switch usageType {
	case usage.UsageProjects:
		return nextPlan, int64(limits.Projects)
	case usage.UsageStorageBytes:
		return nextPlan, limits.StorageBytes
	case usage.UsageAIRequests:
		return nextPlan, int64(limits.AIRequests)
	case usage.UsageExecutionMinutes:
		return nextPlan, int64(limits.ExecutionMinutes)
	}
	return nextPlan, 0
}

// getNextPlan returns the next plan tier
func getNextPlan(current usage.PlanType) string {
	switch current {
	case usage.PlanFree:
		return "pro"
	case usage.PlanPro:
		return "team"
	case usage.PlanTeam:
		return "enterprise"
	default:
		return ""
	}
}

// formatUsageValue formats usage values for human readability
func formatUsageValue(usageType usage.UsageType, value int64) string {
	switch usageType {
	case usage.UsageStorageBytes:
		if value >= 1024*1024*1024 {
			return formatFloat(float64(value)/(1024*1024*1024)) + " GB"
		} else if value >= 1024*1024 {
			return formatFloat(float64(value)/(1024*1024)) + " MB"
		} else if value >= 1024 {
			return formatFloat(float64(value)/1024) + " KB"
		}
		return formatInt(value) + " bytes"
	case usage.UsageExecutionMinutes:
		if value >= 60 {
			hours := value / 60
			minutes := value % 60
			if minutes > 0 {
				return formatInt(hours) + "h " + formatInt(minutes) + "m"
			}
			return formatInt(hours) + " hours"
		}
		return formatInt(value) + " minutes"
	case usage.UsageProjects:
		if value == -1 {
			return "Unlimited"
		}
		return formatInt(value) + " projects"
	case usage.UsageAIRequests:
		if value == -1 {
			return "Unlimited"
		}
		return formatInt(value) + " requests"
	default:
		return formatInt(value)
	}
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%.1f", f)
}

func formatInt(i int64) string {
	if i == -1 {
		return "Unlimited"
	}
	return fmt.Sprintf("%d", i)
}

