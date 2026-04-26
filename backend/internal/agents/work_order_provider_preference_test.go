package agents

import (
	"testing"

	"apex-build/internal/ai"
)

func TestCompileWorkOrdersFromPlanWithCost_PrefersClaudeForFastFrontendPreviewWork(t *testing.T) {
	t.Parallel()

	plan := &BuildPlan{
		ID:           "plan-preview-fast",
		BuildID:      "build-preview-fast",
		AppType:      "web",
		DeliveryMode: "frontend_preview_only",
		WorkOrders: []BuildWorkOrder{
			{
				Role:          RoleFrontend,
				Summary:       "Build the landing page",
				OwnedFiles:    []string{"src/**"},
				RequiredFiles: []string{"src/App.tsx"},
			},
		},
	}

	orders := compileWorkOrdersFromPlanWithCost("build-preview-fast", nil, plan, defaultProviderScorecards("platform"), CostSensitivityHigh)
	if len(orders) != 1 {
		t.Fatalf("expected one work order, got %+v", orders)
	}
	if got := orders[0].PreferredProvider; got != ai.ProviderClaude {
		t.Fatalf("preferred provider = %s, want %s", got, ai.ProviderClaude)
	}
}
