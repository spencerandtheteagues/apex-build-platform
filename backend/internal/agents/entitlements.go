package agents

import (
	"strings"

	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
)

const backendSubscriptionRequiredCode = "BACKEND_SUBSCRIPTION_REQUIRED"

var paidBuildPlans = map[string]bool{
	"builder":    true,
	"pro":        true,
	"team":       true,
	"enterprise": true,
	"owner":      true,
}

var paidBuildCapabilityReasons = map[CapabilityRequirement]string{
	CapabilityAuth:       "authentication flows",
	CapabilityDatabase:   "database-backed apps",
	CapabilityStorage:    "server-side file storage",
	CapabilityJobs:       "background jobs",
	CapabilityRealtime:   "realtime features",
	CapabilityFileUpload: "file uploads",
}

func (h *BuildHandler) currentSubscriptionType(c *gin.Context, userID uint) string {
	if plan, ok := c.Get("subscription_type"); ok {
		if planType, ok := plan.(string); ok && strings.TrimSpace(planType) != "" {
			return strings.ToLower(strings.TrimSpace(planType))
		}
	}

	if h.db == nil {
		return "free"
	}

	var user models.User
	if err := h.db.Select("subscription_type").First(&user, userID).Error; err != nil {
		return "free"
	}

	planType := strings.ToLower(strings.TrimSpace(user.SubscriptionType))
	if planType == "" {
		return "free"
	}
	return planType
}

func isPaidBuildPlan(planType string) bool {
	return paidBuildPlans[strings.ToLower(strings.TrimSpace(planType))]
}

func buildSubscriptionRequirement(req *BuildRequest) (bool, string) {
	if req == nil {
		return false, ""
	}

	if req.TechStack != nil {
		if strings.TrimSpace(req.TechStack.Backend) != "" {
			return true, "backend services"
		}
		if strings.TrimSpace(req.TechStack.Database) != "" {
			return true, "database-backed apps"
		}
	}

	description := normalizeCompactText(req.Description)
	if description == "" {
		description = normalizeCompactText(req.Prompt)
	}
	if description == "" {
		return false, ""
	}

	normalized := " " + strings.ToLower(description) + " "
	for _, phrase := range []struct {
		term   string
		reason string
	}{
		{" full stack ", "full-stack apps"},
		{" fullstack ", "full-stack apps"},
		{" backend ", "backend services"},
		{" app with api ", "API-backed apps"},
		{" api ", "API-backed apps"},
		{" server ", "backend services"},
		{" stripe checkout ", "payment flows"},
		{" billing portal ", "payment flows"},
		{" subscription management ", "payment flows"},
		{" payment processing ", "payment flows"},
		{" webhook ", "backend integrations"},
	} {
		if strings.Contains(normalized, phrase.term) {
			return true, phrase.reason
		}
	}

	for _, capability := range detectRequiredCapabilities(description, req.TechStack) {
		if reason, ok := paidBuildCapabilityReasons[capability]; ok {
			return true, reason
		}
	}

	return false, ""
}
