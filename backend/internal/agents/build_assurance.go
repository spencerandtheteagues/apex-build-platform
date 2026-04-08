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

VISUAL QUALITY MANDATE — every build must produce an app that looks professionally designed:
- NEVER produce a plain white page with unstyled text or default browser inputs. Every element must be styled.
- Choose a visual identity specifically tailored to the app's domain and purpose — not a generic template.
- Use Tailwind CSS (or the selected styling library) with a coherent color palette, typography scale, and spacing system.
- Design patterns by app type:
  * SaaS dashboards: dark or slate backgrounds, sidebar nav, card-based content, accent color for CTAs
  * E-commerce: clean product grid, bold imagery placeholders, clear pricing, cart interactions
  * Social/community: avatar-based layouts, activity feeds, notification patterns
  * Productivity tools: focus-optimized layout, keyboard shortcuts, dense but readable information density
  * Developer tools / APIs: dark terminal aesthetic or clean IDE-like light theme
  * Landing pages: hero sections, feature grids, testimonials, clear pricing tiers
- Every component needs: hover states, focus states, loading states (skeleton loaders), error states, and empty states
- Mobile-first responsive design: works at 375px, scales gracefully to desktop
- Typography hierarchy: clear h1/h2/h3/body distinction with appropriate font sizes and weights
- The generated app must feel like a real product a startup would ship, not a coding exercise
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
