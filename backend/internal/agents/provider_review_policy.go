package agents

import (
	"os"
	"strings"
)

const byokCrossProviderReviewFlag = "APEX_ALLOW_BYOK_CROSS_PROVIDER_REVIEW"

func providerAssistedReviewAllowed(build *Build) bool {
	if build == nil {
		return true
	}
	if !strings.EqualFold(strings.TrimSpace(build.ProviderMode), "byok") {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(os.Getenv(byokCrossProviderReviewFlag)), "true")
}
