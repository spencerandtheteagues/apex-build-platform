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
	if !strings.Contains(freePrompt, "VISUAL QUALITY MANDATE") {
		t.Fatalf("expected visual quality mandate in free-plan prompt")
	}

	paidPrompt := manager.getSystemPrompt(RoleLead, paidBuild)
	if !strings.Contains(paidPrompt, "full-stack eligible") {
		t.Fatalf("expected paid-plan delivery target guidance, got %q", paidPrompt)
	}
}

func TestPlannerAndFrontendSystemPromptsIncludeVisualPlanningRequirements(t *testing.T) {
	t.Parallel()

	manager := &AgentManager{}
	build := &Build{
		SubscriptionPlan: "builder",
		TechStack: &TechStack{
			Frontend: "React 18",
			Backend:  "Node.js + Express",
			Styling:  "Tailwind CSS",
		},
	}

	plannerPrompt := manager.getSystemPrompt(RolePlanner, build)
	if !strings.Contains(plannerPrompt, "## Visual Design Spec") {
		t.Fatalf("expected planner prompt to require a visual design spec, got %q", plannerPrompt)
	}
	if !strings.Contains(plannerPrompt, "DESIGN DIRECTION — your plan MUST specify this upfront") {
		t.Fatalf("expected planner visual direction guidance, got %q", plannerPrompt)
	}

	frontendPrompt := manager.getSystemPrompt(RoleFrontend, build)
	if !strings.Contains(frontendPrompt, "VISUAL DESIGN MANDATE") {
		t.Fatalf("expected frontend visual design mandate, got %q", frontendPrompt)
	}
	if !strings.Contains(frontendPrompt, "skeleton loaders") {
		t.Fatalf("expected frontend prompt to mention skeleton loaders, got %q", frontendPrompt)
	}
}

func TestDefaultTaskOutputFormatPromptUsesRunnableExamples(t *testing.T) {
	t.Parallel()

	prompt := defaultTaskOutputFormatPrompt()
	if strings.Contains(prompt, "// complete implementation") {
		t.Fatalf("expected concrete backend example, got placeholder prompt %q", prompt)
	}
	if strings.Contains(prompt, "// ... complete JSX") {
		t.Fatalf("expected concrete frontend example, got placeholder prompt %q", prompt)
	}
	if !strings.Contains(prompt, "app.listen(PORT") {
		t.Fatalf("expected runnable server example in output format prompt, got %q", prompt)
	}
}

func TestBuildTechStackDirectiveRecognizesVersionedSelections(t *testing.T) {
	t.Parallel()

	directive := buildTechStackDirective(&TechStack{
		Frontend: "React 18",
		Backend:  "Node.js + Express",
		Styling:  "Tailwind CSS",
	}, &Agent{Role: RoleFrontend})

	for _, snippet := range []string{
		"Use Vite + TypeScript + Tailwind CSS v3 for React",
		"tailwind.config.js — REQUIRED",
		"postcss.config.js — REQUIRED",
		"Tailwind v3 only",
		"Tailwind CSS is REQUIRED for all visual styling",
		"API_BASE_URL: The backend API runs at http://localhost:3001.",
	} {
		if !strings.Contains(directive, snippet) {
			t.Fatalf("expected directive to contain %q, got %q", snippet, directive)
		}
	}
	if strings.Contains(directive, "@tailwindcss/vite") {
		t.Fatalf("expected directive to avoid Tailwind v4 vite plugin guidance, got %q", directive)
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
