package agents

import (
	"log"
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
	// Always query the database for the authoritative subscription tier.
	// The JWT context value (set by middleware) is used only when the DB is
	// unavailable, because an attacker with a forged or leaked JWT could claim
	// a higher-tier plan — the DB is the ground truth.
	if h.db != nil {
		var user models.User
		if err := h.db.Select("subscription_type").First(&user, userID).Error; err == nil {
			dbPlan := strings.ToLower(strings.TrimSpace(user.SubscriptionType))
			if dbPlan != "" {
				// Cross-check: warn loudly if JWT and DB disagree so we can detect
				// compromised tokens without breaking the request.
				if ctxPlan, ok := c.Get("subscription_type"); ok {
					if ctxPlanStr, ok := ctxPlan.(string); ok {
						ctxPlanStr = strings.ToLower(strings.TrimSpace(ctxPlanStr))
						if ctxPlanStr != "" && ctxPlanStr != dbPlan {
							log.Printf("[WARN] subscription mismatch for user %d: JWT claims %q, DB has %q — using DB value", userID, ctxPlanStr, dbPlan)
						}
					}
				}
				return dbPlan
			}
		}
	}

	// DB unavailable — fall back to the JWT context value with a warning.
	if plan, ok := c.Get("subscription_type"); ok {
		if planType, ok := plan.(string); ok && strings.TrimSpace(planType) != "" {
			log.Printf("[WARN] using JWT-cached subscription type for user %d (DB unavailable)", userID)
			return strings.ToLower(strings.TrimSpace(planType))
		}
	}

	return "free"
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

	normalized := normalizeDetectionText(description)
	for _, capability := range detectRequiredCapabilities(description, req.TechStack) {
		if reason, ok := paidBuildCapabilityReasons[capability]; ok {
			return true, reason
		}
	}

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
		if containsAffirmedTerm(normalized, normalizeDetectionText(phrase.term)) {
			return true, phrase.reason
		}
	}

	return false, ""
}
