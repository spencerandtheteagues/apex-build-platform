package agents

import "strings"

type QAWorkOrder struct {
	Frameworks                 []string `json:"frameworks,omitempty"`
	OwnedTestPaths             []string `json:"owned_test_paths,omitempty"`
	Goals                      []string `json:"goals,omitempty"`
	ExcludeFromPreviewProof    bool     `json:"exclude_from_preview_proof,omitempty"`
	RequireFrontendSmoke       bool     `json:"require_frontend_smoke,omitempty"`
	RequireBackendContractTest bool     `json:"require_backend_contract_test,omitempty"`
}

func derivedTestContract(plan *BuildPlan) *QAWorkOrder {
	contract := &QAWorkOrder{
		Frameworks:              []string{"vitest"},
		OwnedTestPaths:          []string{"src/__tests__/", "tests/"},
		Goals:                   []string{"Cover the happy path first, then edge cases and failures with executable tests."},
		ExcludeFromPreviewProof: true,
	}
	if plan == nil {
		return contract
	}

	if strings.TrimSpace(plan.TechStack.Frontend) != "" {
		contract.Frameworks = append(contract.Frameworks,
			"@testing-library/react",
			"@testing-library/user-event",
		)
		contract.RequireFrontendSmoke = true
		contract.Goals = append(contract.Goals,
			"Create component smoke tests for the main routes and critical UI states.",
			"Verify forms, empty states, error states, and loading states with Testing Library assertions.",
		)
	}
	if strings.TrimSpace(plan.TechStack.Backend) != "" || strings.EqualFold(strings.TrimSpace(plan.AppType), "fullstack") {
		contract.RequireBackendContractTest = true
		contract.Goals = append(contract.Goals,
			"Compare frontend API usage against the shared backend contract and fail with exact route mismatches.",
		)
	}
	if strings.TrimSpace(plan.TechStack.Frontend) != "" {
		contract.Frameworks = append(contract.Frameworks, "@playwright/test")
		contract.OwnedTestPaths = append(contract.OwnedTestPaths, "e2e/")
		contract.Goals = append(contract.Goals,
			"Generate one Playwright smoke path that proves the app boots and the primary CTA or workflow is reachable.",
		)
	}

	contract.Frameworks = dedupeStrings(contract.Frameworks)
	contract.OwnedTestPaths = dedupeStrings(contract.OwnedTestPaths)
	contract.Goals = dedupeStrings(contract.Goals)
	return contract
}
