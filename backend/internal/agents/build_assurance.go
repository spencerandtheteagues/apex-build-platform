package agents

import (
	"fmt"
	"strings"
)

const apexBuildAssuranceMission = `
APEX BUILD ASSURANCE MANDATE:
- Avoid preventable build failure. If a truthful, runnable deliverable can still be produced, keep converging toward it instead of giving up early.
- The preview pane is the truth surface. Every build should optimize for a working interactive preview, not just file generation.
- Free-plan builds must still ship a prompt-matching, preview-ready frontend UI. If the original request included paid-only backend/runtime scope, freeze that future contract honestly and defer the runtime implementation instead of failing the build.
- Paid builds are not complete until the full stack works end-to-end in preview with real routes, real persistence/auth logic, and coherent frontend/backend integration.
- Frontend-first remains mandatory: think through the backend deeply first, freeze the contract, land the UI shell early, then fill the runtime behind it.
`

func buildRequiresStaticFrontendFallback(build *Build) bool {
	if build == nil {
		return false
	}
	build.mu.RLock()
	defer build.mu.RUnlock()
	return buildRequiresStaticFrontendFallbackState(build)
}

func buildRequiresStaticFrontendFallbackState(build *Build) bool {
	if build == nil {
		return false
	}
	if build.SnapshotState.PolicyState != nil && build.SnapshotState.PolicyState.StaticFrontendOnly {
		return !isPaidBuildPlan(build.SnapshotState.PolicyState.PlanType)
	}
	return !isPaidBuildPlan(build.SubscriptionPlan)
}

func buildAssurancePromptContext(build *Build) string {
	if build == nil {
		return strings.TrimSpace(apexBuildAssuranceMission)
	}

	if buildRequiresStaticFrontendFallback(build) {
		return strings.TrimSpace(apexBuildAssuranceMission + `

CURRENT DELIVERY TARGET:
- This build is running on the free/static tier.
- Deliver a preview-ready frontend shell that matches the requested product direction as closely as possible within frontend-only scope.
- Do NOT fabricate a fake backend, fake auth, or fake persistence and present it as working runtime truth.
- If the prompt asked for backend/data/auth/billing/jobs/realtime scope, capture the deferred contract honestly in frontend states and architecture docs so a later paid pass can wire it in without redesigning the UI.`)
	}

	return strings.TrimSpace(apexBuildAssuranceMission + `

CURRENT DELIVERY TARGET:
- This build is full-stack eligible.
- The app is not done until the interactive preview works with the real backend contract, real runtime behavior, and verified vertical-slice flows.
- Prefer repair loops, provider fallback, and minimal deterministic fixes over terminal failure.`)
}

func buildAssuranceTaskContext(build *Build) string {
	if build == nil {
		return ""
	}

	mode := "paid_full_stack_preview"
	note := "Complete the backend, data, integration, and preview path end-to-end."
	if buildRequiresStaticFrontendFallback(build) {
		mode = "free_frontend_preview"
		note = "Ship the strongest truthful frontend-only preview possible and defer paid runtime scope without blocking the build."
	}

	return fmt.Sprintf(`
<build_assurance>
mission: %s
delivery_mode: %s
delivery_note: %s
</build_assurance>
`, strings.TrimSpace(apexBuildAssuranceMission), mode, note)
}
