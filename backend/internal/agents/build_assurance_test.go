package agents

import (
	"strings"
	"testing"
)

func TestGetSystemPromptIncludesBuildAssuranceMission(t *testing.T) {
	t.Parallel()

	manager := &AgentManager{}
	freeBuild := &Build{
		SubscriptionPlan: "free",
		SnapshotState: BuildSnapshotState{
			PolicyState: &BuildPolicyState{
				PlanType:           "free",
				Classification:     BuildClassificationUpgradeRequired,
				UpgradeRequired:    true,
				StaticFrontendOnly: true,
			},
		},
	}
	paidBuild := &Build{
		SubscriptionPlan: "builder",
		SnapshotState: BuildSnapshotState{
			PolicyState: &BuildPolicyState{
				PlanType:           "builder",
				Classification:     BuildClassificationFullStackCandidate,
				UpgradeRequired:    false,
				StaticFrontendOnly: false,
			},
		},
	}

	freePrompt := manager.getSystemPrompt(RoleLead, freeBuild)
	if !strings.Contains(freePrompt, "APEX BUILD ASSURANCE MANDATE") {
		t.Fatalf("expected assurance mandate in free-plan prompt")
	}
	if !strings.Contains(freePrompt, "free/static tier") {
		t.Fatalf("expected free-plan delivery target guidance, got %q", freePrompt)
	}

	paidPrompt := manager.getSystemPrompt(RoleLead, paidBuild)
	if !strings.Contains(paidPrompt, "full-stack eligible") {
		t.Fatalf("expected paid-plan delivery target guidance, got %q", paidPrompt)
	}
}

func TestPlanningDescriptionForBuildAddsFreeTierFallbackGuidance(t *testing.T) {
	t.Parallel()

	build := &Build{
		Description:      "Build a full-stack CRM with auth, data sync, and billing.",
		SubscriptionPlan: "free",
		SnapshotState: BuildSnapshotState{
			PolicyState: &BuildPolicyState{
				PlanType:           "free",
				Classification:     BuildClassificationUpgradeRequired,
				UpgradeRequired:    true,
				StaticFrontendOnly: true,
			},
		},
	}

	description := planningDescriptionForBuild(build)
	if !strings.Contains(description, "APEX BUILD DELIVERY TARGET") {
		t.Fatalf("expected planning fallback guidance, got %q", description)
	}
	if !strings.Contains(description, "frontend-only app preview") {
		t.Fatalf("expected frontend-only preview language, got %q", description)
	}
}
