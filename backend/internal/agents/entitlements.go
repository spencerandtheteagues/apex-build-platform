package agents

import (
	"errors"
	"fmt"
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

type buildSubscriptionRequiredError struct {
	CurrentPlan   string
	RequiredPlan  string
	BlockedReason string
	Suggestion    string
}

func (e *buildSubscriptionRequiredError) Error() string {
	if e == nil {
		return ""
	}
	currentPlan := firstNonEmptyString(strings.TrimSpace(e.CurrentPlan), "free")
	requiredPlan := firstNonEmptyString(strings.TrimSpace(e.RequiredPlan), "builder")
	blockedReason := firstNonEmptyString(strings.TrimSpace(e.BlockedReason), "backend/runtime implementation")
	return fmt.Sprintf("%s requires %s or higher on the %s plan", blockedReason, requiredPlan, currentPlan)
}

func asBuildSubscriptionRequiredError(err error) (*buildSubscriptionRequiredError, bool) {
	var target *buildSubscriptionRequiredError
	if !errors.As(err, &target) || target == nil {
		return nil, false
	}
	return target, true
}

func newBuildSubscriptionRequiredError(currentPlan string, blockedReason string) *buildSubscriptionRequiredError {
	requiredPlan := "builder"
	reason := strings.TrimSpace(blockedReason)
	if reason == "" {
		reason = "backend/runtime implementation"
	}
	return &buildSubscriptionRequiredError{
		CurrentPlan:   strings.ToLower(strings.TrimSpace(currentPlan)),
		RequiredPlan:  requiredPlan,
		BlockedReason: reason,
		Suggestion:    fmt.Sprintf("The frontend preview can keep iterating on the current free plan. Upgrade to Builder or higher to unlock %s on this same app.", reason),
	}
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

func buildFollowupRequiresPaidRuntime(build *Build, message string, target buildMessageTarget, matchedAgents []*Agent) (bool, string) {
	if build == nil {
		return false, ""
	}
	if isPaidBuildPlan(buildSubscriptionPlan(build)) {
		return false, ""
	}
	if !buildRequiresStaticFrontendFallbackState(build) && !buildUsesFrontendPreviewOnlyDelivery(build) {
		return false, ""
	}

	if roleReason := buildMessageTargetPaidRuntimeReason(target, matchedAgents); roleReason != "" {
		return true, roleReason
	}

	if requiresPaid, reason := buildSubscriptionRequirement(&BuildRequest{
		Description: message,
		Prompt:      message,
		TechStack:   build.TechStack,
	}); requiresPaid {
		return true, reason
	}

	normalized := normalizeDetectionText(message)
	for _, phrase := range []struct {
		term   string
		reason string
	}{
		{"make it functional", "backend/runtime implementation"},
		{"make it fully functional", "backend/runtime implementation"},
		{"make this functional", "backend/runtime implementation"},
		{"make it work", "backend/runtime implementation"},
		{"make this work", "backend/runtime implementation"},
		{"wire it up", "backend/runtime implementation"},
		{"hook it up", "backend/runtime implementation"},
		{"connect it for real", "backend/runtime implementation"},
		{"real backend", "backend services"},
		{"real auth", "authentication flows"},
		{"real login", "authentication flows"},
		{"save data", "database-backed apps"},
		{"persist data", "database-backed apps"},
		{"store user data", "database-backed apps"},
		{"deploy it", "deployment"},
		{"publish it", "deployment"},
		{"go live", "deployment"},
	} {
		if containsAffirmedTerm(normalized, normalizeDetectionText(phrase.term)) {
			return true, phrase.reason
		}
	}

	return false, ""
}

func buildMessageTargetPaidRuntimeReason(target buildMessageTarget, matchedAgents []*Agent) string {
	if buildRoleRequiresPaidRuntime(target.AgentRole) {
		return buildRolePaidRuntimeReason(target.AgentRole)
	}
	for _, agent := range matchedAgents {
		if agent == nil {
			continue
		}
		if buildRoleRequiresPaidRuntime(string(agent.Role)) {
			return buildRolePaidRuntimeReason(string(agent.Role))
		}
	}
	return ""
}

func buildRoleRequiresPaidRuntime(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "backend", "database", "devops":
		return true
	default:
		return false
	}
}

func buildRolePaidRuntimeReason(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "database":
		return "database-backed apps"
	case "devops":
		return "deployment"
	default:
		return "backend services"
	}
}
