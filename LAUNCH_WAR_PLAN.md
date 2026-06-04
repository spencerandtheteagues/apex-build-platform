# APEX-BUILD LAUNCH WAR PLAN
## Master Task List for 98% Confidence of Beating Replit by 10%+ Across 10 Categories

**Prepared by:** APEX Launch Master Orchestrator
**Date:** 2026-05-26
**Codebase:** `/root/apex-build-platform`
**Live:** https://apex-build.dev (backend: https://api.apex-build.dev)
**Timeline:** 10 days to paid public launch

---

## VERIFIED GROUND TRUTH (from direct code read, 2026-05-25)

- Backend compiles clean (`cd backend && go build ./...` EXIT 0). Strong reliability baseline.
- **Stripe webhook idempotency is REAL** — `ProcessedStripeEvent` unique key + transactional `ApplyCreditGrant`, with passing local replay tests. Most P0 billing work is *live verification*, not new code.
- **Stripe is LIVE and processing real payments** — a real customer payment was confirmed by the launch owner from Stripe/dashboard config on 2026-05-25; no secret, customer, or payment payload is stored in the repo. TASK-001 is complete; TASK-002/TASK-003 still need controlled replay/lifecycle evidence before broad public launch.
- **Explore page has fork/remix/publish already built** — linked from the landing page and app navigation in current code; deploy and smoke evidence still need to be recorded for the launch commit.
- **Blank IDE is reachable at `/ide`** — current builder onboarding code adds a first-run blank-workspace escape hatch; deploy and smoke evidence still need to be recorded for the launch commit.
- **Always-on controller + hosting keep-alive exist** and are wired into `backend/cmd/main.go:849`.
- **Cost transparency** (CostTicker, SpendDashboard, BudgetSettings, PanicKillButton) and **BYOK** (APIKeySettings, ModelSelector) are built — our moat is unsurfaced.
- The genuinely weak category needing net-new work is **Onboarding** (passive slideshow, no guided first-build).

**Chosen path: Path E (Hybrid).** Tier 1 prove reliability → Tier 2 close the three churn gaps → Tier 3 amplify the moat in UI → Tier 4 polish IDE/builder.

---

## STRATEGIC PATH ANALYSIS (why Path E, weighed against A–D)

| Path | Core bet | Fatal weakness | P(10% better, all cats) |
|---|---|---|---|
| A: Fix everything | Brute force every bug + feature | 332KB AppBuilder + 1.2MB manager.go = high regression risk in 10 days; dilutes focus | 60% |
| B: Amplify advantages | Lean into cost/BYOK/honest builds | Onboarding (4/10) and Community (3/10) still lose to Replit | 75% |
| C: Close 3 biggest gaps | Onboarding + blank + persistent preview | Ignores that you can't *charge* without proven payments/canary | 70% |
| D: Reliability-first | Prove every flow, then polish | Risks "boring" first impression; leaves Onboarding weak | 80% |
| **E: Hybrid (CHOSEN)** | Reliability gate → 3 gaps → amplify moat → polish | Requires disciplined sequencing | **88%+** |

**Decisive realization:** Because Community and Deployment gaps are *already-built-but-unsurfaced*, the only category needing heavy net-new effort is Onboarding. That collapses the risk surface dramatically. Path E with "surface-first" sequencing is optimal.

---

## AGENT ASSIGNMENTS

### openclaw — 20 tasks
P0 (6): 001, 002, 003, 004, 005, 006
P1 (7): 101, 102, 103, 104, 105, 106, 107
P2 (5): 201, 202, 203, 204, 205
P3 (2): 301, 302

### hernmes — 21 tasks
P0 (6): 007, 008, 009, 010, 011, 012
P1 (7): 108, 109, 110, 111, 112, 113, 114
P2 (4): 206, 207, 208, 209
P3 (4): 303, 304, 305, 306

---

# SECTION 1: P0 TASKS — LAUNCH BLOCKERS
### (must complete before broad paid public launch)

---

```
TASK-001: Configure all production Stripe env vars in Render and verify via config-status
Priority: P0
Category: Billing
Status: COMPLETE — Stripe is live, real customer payment confirmed 2026-05-25
Owner: openclaw
```

```
TASK-002: Create live Stripe webhook endpoint and replay all critical events
Priority: P0
Category: Billing
Status: PARTIAL — endpoint is live, invalid signatures are rejected, and non-paid Checkout session probes pass; controlled replay of all critical event types is still required
Owner: openclaw
```

```
TASK-003: Execute one controlled live paid checkout end-to-end with a real card
Priority: P0
Category: Billing
Status: PARTIAL — real customer payment observed 2026-05-25 and non-paid subscription/credit Checkout session creation passed on 2026-06-02; controlled paid checkout completion, portal, plan-change, cancellation, and webhook replay evidence still required
Owner: openclaw
```

```
TASK-004: Pass paid-balanced full-stack build canary in production
Priority: P0
Category: Build-Quality
Estimated Time: 10 minutes
Owner: openclaw
Status: FUNCTIONAL PASS — balanced full-stack canary passed 2026-05-25 with build `69d3582e`; screenshot/console artifact path still needs archival evidence

CONTEXT:
The tracker confirms free-fast and fast-frontend-only golden builds pass, and the paid-balanced
full-stack canary passed on 2026-05-25. Paid-max and broader matrix evidence remain open.

FILES TO CHANGE:
- None (uses scripts/ live golden tooling against api.apex-build.dev)

EXACT CHANGE REQUIRED:
Trigger a full-stack balanced-mode build via the live golden harness using a representative
prompt (use the TemplateGallery "SaaS Dashboard" prompt: React + Node + PostgreSQL with JWT
auth, RBAC, charts, user table, billing page). Watch the build reach completed at 100%, confirm
preview starts, confirm placeholder-gate passes (no generic KPI/skeleton-only output), and
capture a styled screenshot.

ACCEPTANCE CRITERIA:
- [x] Build reaches status=completed at 100% (no 95% stall)
- [x] Preview server starts and serves a real, styled, interactive app
- [x] Placeholder-only preview gate (preview_gate.go) PASSES (not triggered)
- [x] quality_gate_status=passed
- [ ] Screenshot/console artifact path captured and reviewed for visual quality

VERIFICATION COMMAND:
PROMPT_FILE=prompts/canary/02-saas-dashboard-ops-command-center.md MODE=full POWER_MODE=balanced LOGIN_EMAIL='<paid-canary-email>' LOGIN_PASSWORD='<paid-canary-password>' node scripts/run_live_golden_build.mjs  # expects GOLDEN_BUILD_PASSED
```

```
TASK-005: Pass paid-max full-stack build canary in production
Priority: P0
Category: Build-Quality
Estimated Time: 10 minutes
Owner: openclaw
Status: BLOCKED — 2026-06-02 BYOK/Ollama paid-max substitute attempt reached production build `8ae1326f-bb88-4e2d-b452-f2afae15a6df` but failed before generation because Ollama Cloud returned HTTP 429 session usage limit for account `apexbuildai`; local VPS Ollama routing is now hardened for installed model discovery, but flagship/production-reachable paid-max evidence remains open

CONTEXT:
Max-power routing (Claude Opus, GPT, Gemini Pro, Grok, Kimi/local) is our premium tier and the
most expensive. It must complete reliably or we will burn credits on failed flagship builds and
break Pro/Team trust on day one.

FILES TO CHANGE:
- None (live golden tooling)

EXACT CHANGE REQUIRED:
Trigger a max-mode build using the e-commerce canary prompt (`prompts/canary/16-ecommerce-store-checkout.md`):
catalog, cart, simulated checkout, order history, and account/order surfaces. This canary safely
proves max-power build/preview reliability without exercising real Stripe payments or external
APIs; Stripe lifecycle proof remains TASK-002/TASK-003. Confirm the planning hard-deadline patch
holds (no 95% stall on the larger plan), the build completes, and preview is real.

ACCEPTANCE CRITERIA:
- [ ] Build reaches status=completed at 100%
- [ ] No planning stall at 95% (build_spec.go deadline patch holds under max-power load)
- [ ] Preview is real, styled, interactive; placeholder gate passes
- [ ] Per-agent cost attribution recorded and non-zero in the spend ledger
- [ ] Screenshot evidence reviewed

VERIFICATION COMMAND:
PROMPT_FILE=prompts/canary/16-ecommerce-store-checkout.md MODE=full POWER_MODE=max LOGIN_EMAIL='<paid-canary-email>' LOGIN_PASSWORD='<paid-canary-password>' node scripts/run_live_golden_build.mjs  # expects GOLDEN_BUILD_PASSED
```

```
TASK-006: Enable and pass the production canary GitHub Actions workflow against live URLs
Priority: P0
Category: Reliability
Estimated Time: 8 minutes (config) + watch
Owner: openclaw
Status: CONFIG ENABLED / STRICT PAID-CANARY GUARDRAILS ADDED / STRICT RENDER ENV VERIFIED LOCALLY / PASS EVIDENCE PENDING — `APEX_ENABLE_GITHUB_ACTIONS=true` and `RENDER_API_KEY` are configured per 2026-05-26 handoff; a passing production-canary workflow run is still required as launch evidence

CONTEXT:
production-canary.yml exists and the enabling variable is now configured, but this tracker still
needs a recorded passing run against apex-build.dev with strict Render/Stripe/canary secrets.
This is the automated safety net that prevents shipping a broken deploy.

FILES TO CHANGE:
- None (GitHub repo Actions secrets + variable APEX_ENABLE_GITHUB_ACTIONS=true)

EXACT CHANGE REQUIRED:
Set repo secrets: RENDER_API_KEY, RENDER_BACKEND_SERVICE_ID, RENDER_FRONTEND_SERVICE_ID, and any
Stripe/canary secrets the workflow expects (see production-canary.yml). With
`APEX_REQUIRE_PAID_CANARIES=true`, the Stripe verifier requires an existing paid canary account
that can open the billing portal, and paid build canaries fail instead of skipping when
credentials are absent. Set repo variables
APEX_ENABLE_GITHUB_ACTIONS=true and APEX_REQUIRE_PAID_CANARIES=true when using the workflow as
launch evidence. Manually dispatch the workflow with paid canary credentials present.

ACCEPTANCE CRITERIA:
- [ ] "Launch Verification Scripts" job passes (Stripe + Render + mobile verifiers)
- [x] Strict Render env verification passes without printing secret values (local Render API verifier passed 2026-06-02 05:05 UTC; GitHub workflow evidence still pending)
- [ ] Public launch smoke enforces runtime launch readiness and passes
- [ ] No secret value appears in any workflow log

VERIFICATION COMMAND:
gh run list --workflow=production-canary.yml --limit 1 --json conclusion,status | jq
```

```
TASK-007: Run and document a rollback drill on Render
Priority: P0
Category: Reliability
Estimated Time: 10 minutes
Owner: hernmes
Status: DRY-RUN-FIRST ROLLBACK SCRIPT ADDED / RENDER API DRY-RUN PASSED / EXECUTION EVIDENCE PENDING — `scripts/run_render_rollback_drill.mjs` uses Render's current rollback API and exact deploy-id confirmations; a real rollback/roll-forward drill is still required in `docs/launch-runbook.md`

CONTEXT:
The tracker lists "rollback drill" as required-but-undone evidence. If a launch-day deploy breaks
builds or payments, we must be able to roll back to a known-good commit in under 5 minutes with
confidence. Never tested = unknown blast radius. The 2026-06-02 Render API dry-run selected
current live deploy `dep-d8dqqkbrjlhs73bh6qj0` and rollback target
`dep-d8dqbfvlk1mc73dl2ibg`, but did not execute rollback or roll-forward.

FILES TO CHANGE:
- docs/launch-runbook.md (append a dated "Rollback Drill" evidence section after execution)
- scripts/run_render_rollback_drill.mjs (already added as dry-run-first tooling)

EXACT CHANGE REQUIRED:
1. Read the current and prior known-good deploy IDs/commits from Render deploy metadata immediately
before the drill. 2. Dry-run `scripts/run_render_rollback_drill.mjs` and copy its exact rollback
and roll-forward deploy IDs into `APEX_RENDER_CONFIRM_ROLLBACK_DEPLOY_ID` and
`APEX_RENDER_CONFIRM_ROLL_FORWARD_DEPLOY_ID`. 3. Execute the script during the approved window,
which calls Render `POST /services/{id}/rollback` for rollback and roll-forward. 4. Confirm
/health returns healthy and a smoke build still works. 5. Record start/end timestamps, deploy IDs,
commit IDs, and total downtime in launch-runbook.md.

ACCEPTANCE CRITERIA:
- [ ] Rollback to prior deploy completes and /health is healthy
- [ ] Total time-to-rollback measured and < 5 minutes
- [ ] Roll-forward restores current code cleanly
- [ ] Evidence section added to docs/launch-runbook.md with timestamps and deploy IDs

VERIFICATION COMMAND:
grep -A5 "Rollback Drill" docs/launch-runbook.md
```

```
TASK-008: Verify full test suite passes (backend + frontend) on the launch commit
Priority: P0
Category: Reliability
Estimated Time: 10 minutes (backend) + parallel (frontend)
Owner: hernmes
Status: BACKEND PASS / FRONTEND PASS ON DIRTY LAUNCH TREE — backend serialized tests and build passed locally on 2026-06-02 after stopping stale agent processes; frontend typecheck, full Vitest, lint, and build also passed locally on 2026-06-02. Re-run aggregate backend+frontend on the final launch commit if more code changes land.

CONTEXT:
The launch evidence checklist requires the serialized backend suite, frontend typecheck/test/lint/build, and
Playwright suites to pass. Backend compiles clean but the test suite must be green on the exact
commit we ship.

FILES TO CHANGE:
- None (fix any failures discovered -> spin off as new tasks)

EXACT CHANGE REQUIRED:
Run the backend test suite and the frontend gate. Capture any failures. If failures exist,
classify each as launch-blocking (build/payment/preview path) or deferrable, and create follow-up
tasks. Do not mark launch-ready with red tests on critical paths.

ACCEPTANCE CRITERIA:
- [x] cd backend && go test -p 1 -parallel 4 ./... -timeout 20m passes
- [x] cd backend && go build ./... succeeds
- [x] cd frontend && npm run typecheck passes
- [x] cd frontend && npm run test -- --run passes
- [x] cd frontend && npm run build succeeds
- [ ] Any failure on a build/payment/preview path is escalated as a new P0

VERIFICATION COMMAND:
cd backend && go test -p 1 -parallel 4 ./... -timeout 20m 2>&1 | tail -20 ; go build ./... ; cd ../frontend && npm run typecheck && npm run test -- --run && npm run build 2>&1 | tail -20
```

```
TASK-009: Run 20-prompt diverse build matrix and confirm 100% completion + 100% preview
Priority: P0
Category: Build-Quality
Estimated Time: 10 minutes per batch of prompts (run in parallel batches)
Owner: hernmes

CONTEXT:
Our non-negotiable is 100% build completion and 100% preview success. We have golden proofs for a
couple of profiles but not a broad diverse matrix. The first 100 users will submit unpredictable
prompts; we must prove breadth. The `prompts/canary` fixture set now has 20 topic fixtures across
simple, operational, commerce, collaboration, finance, admin, and simulated-AI surfaces. It is still
architecturally narrow because most fixtures use React/Tailwind/shadcn, in-memory data, and local
simulation. This closes topic-fixture count only; it does not satisfy the 20-prompt launch gate
until the live matrix runs and records 20/20 passing build, preview, quality, console, and artifact
evidence.

FILES TO CHANGE:
- docs/launch-readiness-tracker.md (append the matrix results table)

EXACT CHANGE REQUIRED:
Run 20 builds spanning: 6 simple (landing page, portfolio, calculator, to-do, pricing page, blog
frontend), 8 medium operational/productivity prompts, and 6 more complex domains (e-commerce,
real-time chat simulation, social feed, finance tracker, admin panel, AI chatbot simulation). The
current GitHub `prompt-reliability-live-matrix` job runs all prompt fixtures with one full-mode
profile and is evidence for paid full-mode frontend/in-memory reliability only. Mixed-tier launch
evidence still requires a follow-up profile split that runs simple prompts through free-fast and
backend/persistence/auth/realtime/integration-heavy prompts through paid full-stack modes. Record
completion %, preview success, time-to-preview, and a visual-quality 1-10 score for each run.

ACCEPTANCE CRITERIA:
- [ ] 20/20 builds reach completed (100% completion rate)
- [ ] 20/20 produce a working, viewable, styled preview (100% preview rate)
- [ ] Zero placeholder-only previews slip through the gate
- [ ] Median visual-quality score >= 8/10; none below 7
- [ ] Results table appended to launch-readiness-tracker.md with build IDs

VERIFICATION COMMAND:
echo "Completion: X/20  Preview: X/20  MedianQuality: X"
```

```
TASK-010: Load test the platform at expected launch concurrency
Priority: P0
Category: Reliability
Estimated Time: 10 minutes
Owner: hernmes

CONTEXT:
The launch authorization criteria require a passed load test at expected launch traffic. A flood
of signups + concurrent builds on day one could exhaust provider rate limits, DB connections, or
preview containers. We must know the ceiling before customers find it.

FILES TO CHANGE:
- docs/launch-runbook.md (append load-test results)

EXACT CHANGE REQUIRED:
Use a load tool (k6/vegeta/autocannon) to drive: (a) 200 concurrent unauthenticated
landing+health hits, (b) 50 concurrent authenticated /api/v1/usage/limits + project-list calls,
(c) 10 concurrent build starts (free-fast, frontend-only to bound cost). Measure p95 latency,
error rate, and confirm preview containers and DB pool do not exhaust.

ACCEPTANCE CRITERIA:
- [ ] /health and landing p95 < 800ms under 200 concurrent
- [ ] Authenticated API error rate < 1% under 50 concurrent
- [ ] 10 concurrent builds all reach completed; no DB pool exhaustion errors in logs
- [ ] No 5xx spikes; rate-limit responses (if any) are graceful 429s, not crashes
- [ ] Results appended to docs/launch-runbook.md

VERIFICATION COMMAND:
k6 run scripts/loadtest.js 2>&1 | tail -25   # or autocannon equivalent; create script if absent
```

```
TASK-011: Add a backend smoke test asserting webhook dedup under concurrent duplicate delivery
Priority: P0
Category: Billing
Estimated Time: 10 minutes
Owner: hernmes

CONTEXT:
Idempotency is enforced by a unique-constraint insert in ApplyCreditGrant. Existing tests cover
sequential duplicates, but Stripe can deliver the same event concurrently (retries overlapping).
A race that double-inserts the ledger before the unique constraint commits would double-credit a
paying customer — a financial integrity bug.

FILES TO CHANGE:
- backend/internal/handlers/payments_idempotency_test.go (add a concurrent-delivery test)

EXACT CHANGE REQUIRED:
Add a test that fires N (e.g. 10) goroutines calling handleInvoicePaid with the SAME event
EventID simultaneously against an in-memory/sqlite DB with the ProcessedStripeEvent unique index
present. Assert that exactly one CreditLedgerEntry is created and credit_balance increased by
exactly one allocation. Confirm isDuplicateCreditInsertError catches the unique-violation under
the concurrent path.

ACCEPTANCE CRITERIA:
- [ ] New test spawns >=10 concurrent identical-event handlers
- [ ] Exactly one ledger entry created; balance increments exactly once
- [ ] Test passes deterministically across 5 runs (no flakiness)
- [ ] No regression: existing payments_idempotency_test.go cases still pass

VERIFICATION COMMAND:
cd backend && go test ./internal/handlers -run TestHandleInvoicePaidConcurrentDedup -count=5 -race
```

```
TASK-012: Verify "build failure -> restart" recovery path in production
Priority: P0
Category: Reliability
Estimated Time: 10 minutes
Owner: hernmes

CONTEXT:
We claim no silent failures and a restartable build. The tracker lists "failed-build restart in
production" as unverified. If a provider hiccups mid-build, the user must get a clean restart, not
a stuck or silently-dead build.

FILES TO CHANGE:
- None (verification; fixes spin off as tasks)

EXACT CHANGE REQUIRED:
Start a build, then force a failure mid-flight (e.g., a prompt known to trip a provider
cost-threshold, or temporarily constrain a provider). Confirm the orchestration classifies it as
a provider-level failure and either fails over to a cheaper available provider (per the tracker's
cost-threshold-skip fix) or surfaces a clean restart CTA. Then exercise the restart and confirm
it completes.

ACCEPTANCE CRITERIA:
- [ ] A mid-build provider failure does NOT leave the build silently stuck
- [ ] Orchestration either auto-fails-over OR presents a clear restart action
- [ ] Restart completes to a working preview
- [ ] Failure + recovery are visible in the build activity feed (no black box)

VERIFICATION COMMAND:
curl -s -H "Cookie: <session>" https://api.apex-build.dev/api/v1/build/<id>/poll-status | jq '.status'
```

---

# SECTION 2: P1 TASKS — CRITICAL QUALITY
### (complete before broad public launch)

---

```
TASK-101: Surface Explore in landing nav and in-app nav
Priority: P1
Category: Community
Estimated Time: 7 minutes
Owner: openclaw

CONTEXT:
Community is our weakest category (3/10) yet Explore.tsx already has fork/remix/publish fully
built. Replit's community is a 9. Simply linking Explore is the single highest-leverage,
lowest-risk win in the whole plan.

FILES TO CHANGE:
- frontend/src/pages/Landing.tsx:1473-1478 (nav links block)
- frontend/src/App.tsx (in-app top nav / view switcher)

EXACT CHANGE REQUIRED:
In Landing.tsx nav (line 1474-1477), add an Explore link: <a href="/explore">Explore</a> between
"Agents" and "Pricing". In App.tsx, add an "Explore" entry to the primary in-app nav that calls
navigateToView('explore'). The /explore route already resolves (App.tsx:160-161), so no routing
work is needed.

ACCEPTANCE CRITERIA:
- [ ] Landing nav shows Explore and clicking it loads the Explore gallery
- [ ] In-app nav shows Explore and switches to the explore view
- [ ] No regression: existing nav links (Features/Agents/Pricing/Docs) still work
- [ ] Mobile nav also includes Explore

VERIFICATION COMMAND:
grep -n '"/explore"\|>Explore<' frontend/src/pages/Landing.tsx && grep -n "explore" frontend/src/App.tsx | grep -i nav
```

```
TASK-102: Add "Open blank workspace" entry point on the builder screen
Priority: P1
Category: Onboarding
Estimated Time: 10 minutes
Owner: openclaw

CONTEXT:
Replit's killer feature is instant project start (9/10 onboarding). Our builder forces a prompt
before anything runs (4/10). The /ide route ALREADY drops into a blank IDE (App.tsx:151) — we
just never give users a button to get there.

FILES TO CHANGE:
- frontend/src/components/builder/AppBuilder.tsx (near the build CTA / under TemplateGallery, ~line 7535)

EXACT CHANGE REQUIRED:
Add a secondary action near the primary Build button: a text/ghost button "Open a blank
workspace ->" and "Import from GitHub". The blank-workspace button should navigate to /ide.
Place copy: "Prefer to start from scratch or explore the IDE first?"

ACCEPTANCE CRITERIA:
- [ ] A visible "Open a blank workspace" control appears on the builder screen
- [ ] Clicking it lands the user in the empty IDE without a build
- [ ] "Import from GitHub" is reachable from the same area
- [ ] No regression: the primary prompt -> Build flow is unchanged

VERIFICATION COMMAND:
grep -n "blank workspace\|/ide\|Open a blank" frontend/src/components/builder/AppBuilder.tsx
```

```
TASK-103: Rebuild OnboardingTour into a guided first-run flow that ends in a real preview
Priority: P1
Category: Onboarding
Estimated Time: 10 minutes
Owner: openclaw

CONTEXT:
OnboardingTour.tsx is a passive 6-slide modal. The roadmap target is a guided flow: "What are you
building?" -> template/blank -> orientation -> first build that ends with a working preview.

FILES TO CHANGE:
- frontend/src/components/builder/OnboardingTour.tsx
- frontend/src/components/builder/AppBuilder.tsx

EXACT CHANGE REQUIRED:
Convert the tour's final step from "Start Building" (which just closes) into an action grid: 3
one-click starters (Portfolio Site, To-Do app, Landing page) plus "Open blank workspace".
Selecting a starter must: close the tour, pre-fill appDescription, set buildMode='fast', and
auto-scroll to the Build button. Persist completion in localStorage key apex_onboarding_completed.

ACCEPTANCE CRITERIA:
- [ ] Final onboarding step presents actionable one-click starters
- [ ] Selecting a starter pre-fills the prompt and focuses the Build CTA
- [ ] A new user can go from first load to a visible preview in < 5 minutes
- [ ] Tour does not reappear after completion (localStorage respected)
- [ ] No regression: existing OnboardingTour tests pass

VERIFICATION COMMAND:
cd frontend && npm run test -- --run OnboardingTour 2>&1 | tail -10
```

```
TASK-104: Surface persistent-preview / "Keep this running" as a first-class post-build action
Priority: P1
Category: Deployment
Estimated Time: 10 minutes
Owner: openclaw

CONTEXT:
Deployment is 5/10. The always-on controller (deploy/alwayson/service.go) and hosting keep-alive
(SetAlwaysOn) ALREADY exist and are wired in main.go, but there is no prominent post-build CTA.

FILES TO CHANGE:
- frontend/src/components/builder/AppBuilder.tsx (post-build success panel)
- frontend/src/components/deployment/ (add/extend a "Keep running" toggle)

EXACT CHANGE REQUIRED:
On build completion, add a "Keep this app running" action for paid users calling the hosting
SetAlwaysOn path. Add a warm/cold status pill near the preview. Gate behind paid plan (free users
see an upgrade tooltip).

ACCEPTANCE CRITERIA:
- [ ] Completed paid builds show a "Keep this app running" toggle
- [ ] Toggling on calls the always-on endpoint and reflects success
- [ ] A warm/cold status indicator is visible near the preview
- [ ] Free users see an upgrade prompt instead of the toggle
- [ ] No regression

VERIFICATION COMMAND:
grep -rn "SetAlwaysOn\|always.on\|always_on" backend/internal/handlers/hosting.go | head && grep -n "alwaysOn\|keepRunning\|Keep this app" frontend/src/components/builder/AppBuilder.tsx frontend/src/services/api.ts
```

```
TASK-105: Make cost transparency impossible to miss in the build view
Priority: P1
Category: Cost-Transparency
Estimated Time: 10 minutes
Owner: openclaw

CONTEXT:
Cost transparency is our #1 advantage (8 vs Replit's 4). CostTicker, SpendToast, and per-agent
attribution exist but must be visually prominent. Target: 9/10.

FILES TO CHANGE:
- frontend/src/components/builder/AppBuilder.tsx and/or BuildScreen.tsx
- frontend/src/components/ide/CostTicker.tsx

EXACT CHANGE REQUIRED:
Ensure the live cost ticker is persistently visible (sticky/pinned) during an active build,
showing running USD spend, per-agent/per-model breakdown, and remaining budget. Add a one-line
tagline: "Every token, every agent — accounted for. No mystery cycles."

ACCEPTANCE CRITERIA:
- [ ] During an active build, live USD spend is visible without scrolling
- [ ] Per-agent or per-model cost breakdown is reachable in one click/hover
- [ ] Remaining budget / cap is shown
- [ ] The "no mystery cycles" framing appears once
- [ ] No regression: CostTicker tests pass

VERIFICATION COMMAND:
cd frontend && npm run test -- --run CostTicker 2>&1 | tail -10
```

```
TASK-106: Elevate BYOK as a visible paid-plan hero control
Priority: P1
Category: Cost-Transparency
Estimated Time: 8 minutes
Owner: openclaw

CONTEXT:
BYOK (bring your own keys) is a first-class differentiator. APIKeySettings and ModelSelector exist
but BYOK isn't framed as a headline benefit.

FILES TO CHANGE:
- frontend/src/components/settings/APIKeySettings.tsx
- frontend/src/pages/Landing.tsx

EXACT CHANGE REQUIRED:
In APIKeySettings, add a clear "Bring Your Own Keys" header with per-provider connected/active
status. In Landing.tsx, add a BYOK benefit line in the features or pricing section.

ACCEPTANCE CRITERIA:
- [ ] APIKeySettings clearly explains BYOK and shows per-provider connection status
- [ ] Landing communicates BYOK as a benefit
- [ ] No secrets rendered in plaintext
- [ ] No regression: existing key save/validate flows work

VERIFICATION COMMAND:
grep -in "bring your own\|byok" frontend/src/components/settings/APIKeySettings.tsx frontend/src/pages/Landing.tsx
```

```
TASK-107: Add a Dependencies panel to the IDE sidebar
Priority: P1
Category: IDE
Estimated Time: 10 minutes
Owner: openclaw
Status: COMPLETE — existing package manager is now exposed as a Dependencies sidebar tab in the IDE; focused IDELayout test and frontend typecheck passed on 2026-06-02

CONTEXT:
Replit auto-detects imports and shows a package manager UI. backend/internal/packages and
handlers/packages.go (20KB) exist — the backend is there. Surfacing it raises IDE quality 7->8.5.

FILES TO CHANGE:
- frontend/src/components/ide/ (new DependenciesPanel.tsx)
- frontend/src/components/explorer/ or the IDE sidebar host (register the panel)

EXACT CHANGE REQUIRED:
Create a DependenciesPanel that lists installed packages (name + version) for the current project
via the packages API. Phase 1: read-only listing with a refresh. Register as a sidebar tab in the
IDE next to the file explorer.

ACCEPTANCE CRITERIA:
- [x] A Dependencies tab appears in the IDE sidebar
- [x] It lists current project packages with versions
- [x] Empty/blank projects show a graceful empty state
- [x] No regression: file explorer and other sidebar tabs still work

VERIFICATION COMMAND:
cd frontend && npm run test -- --run src/components/ide/IDELayout.test.tsx && npm run typecheck
```

```
TASK-108: Add "Featured Builds" showcase section to the landing page
Priority: P1
Category: Community
Estimated Time: 10 minutes
Owner: hernmes

CONTEXT:
Community is 3/10. The roadmap calls for a "Featured Builds" section showcasing real apps.
Published apps + fork counts already exist in Explore data.

FILES TO CHANGE:
- frontend/src/pages/Landing.tsx

EXACT CHANGE REQUIRED:
Add a "Built on APEX" section showing 3-6 published apps (title, thumbnail/screenshot, stack
badges, fork count) from the public published-projects endpoint. Each card links to Explore or a
one-click fork. If no published apps exist yet, seed 3-6 curated demo builds.

ACCEPTANCE CRITERIA:
- [ ] Landing shows a Featured Builds section with real or seeded cards
- [ ] Cards show stack + fork/star counts and link to Explore/fork
- [ ] Section never renders empty (graceful fallback to curated builds)
- [ ] No regression: landing layout/performance unaffected

VERIFICATION COMMAND:
grep -in "Featured\|Built on APEX\|published" frontend/src/pages/Landing.tsx | head && cd frontend && npm run build 2>&1 | tail -5
```

```
TASK-109: Verify and polish the upgrade/downgrade in-app flow UX
Priority: P1
Category: Billing
Estimated Time: 10 minutes
Owner: hernmes

CONTEXT:
Billing UX target is 8.5. The Stripe backend (UpdateSubscription with proration,
CancelSubscription) is complete. The in-app upgrade/downgrade flow must clearly communicate
proration, effective dates, and credit changes.

FILES TO CHANGE:
- frontend/src/components/billing/BillingSettings.tsx
- frontend/src/components/billing/BuyCreditsModal.tsx

EXACT CHANGE REQUIRED:
Audit upgrade/downgrade buttons: ensure each plan tier shows current vs target, explains proration
("you'll be charged the prorated difference now"), and downgrade explains "changes take effect at
period end." Confirm BuyCreditsModal offers exactly $25/$50/$100/$250.

ACCEPTANCE CRITERIA:
- [ ] Upgrade clearly states immediate prorated charge
- [ ] Downgrade clearly states period-end effect
- [ ] Credit packs show $25/$50/$100/$250 with credit value
- [ ] Post-checkout return shows a success/cancel toast
- [ ] No regression: BillingSettings tests pass

VERIFICATION COMMAND:
cd frontend && npm run test -- --run BillingSettings 2>&1 | tail -10
```

```
TASK-110: Time-to-first-preview instrumentation + speed verification
Priority: P1
Category: Speed
Estimated Time: 10 minutes
Owner: hernmes

CONTEXT:
Speed target is < 3 min frontend, < 8 min full-stack. We can't improve what we don't measure.

FILES TO CHANGE:
- backend/internal/agents/build_telemetry.go
- docs/launch-readiness-tracker.md

EXACT CHANGE REQUIRED:
Confirm build telemetry records: build start -> plan ready -> first file -> preview-served
timestamps. Compute median and p90 time-to-first-preview per tier using the TASK-009 matrix.
If frontend median > 3 min or full-stack median > 8 min, file a targeted optimization task.

ACCEPTANCE CRITERIA:
- [ ] Time-to-first-preview is recorded per build in telemetry
- [ ] Median + p90 computed per tier and recorded in the tracker
- [ ] Frontend-only median < 3 min; full-stack median < 8 min (or optimization task filed)
- [ ] No regression: telemetry tests pass

VERIFICATION COMMAND:
cd backend && go test ./internal/agents/ -run Telemetry 2>&1 | tail -10
```

```
TASK-111: Strict mobile readiness gating audit (truth-in-marketing guard)
Priority: P1
Category: Reliability
Estimated Time: 8 minutes
Owner: hernmes
Status: COMPLETE — 2026-06-02 no-credit audit passed; public landing copy has no native/store GA claims, authenticated mobile surfaces label native binaries/store upload as gated/separate, and live platform truth keeps mobile source beta plus EAS/store paths gated

CONTEXT:
Native mobile builds/store upload remain gated beta until real EAS/Apple/Google evidence exists.
Claiming native mobile at launch without evidence would be false marketing.

FILES TO CHANGE:
- frontend/src/pages/Landing.tsx (audit only; correct if over-claiming)

EXACT CHANGE REQUIRED:
Grep all public-facing copy for native mobile / App Store / Google Play claims. Confirm anything
implying native builds or store publishing is either removed, or clearly labeled "beta / coming
soon". Run verify_mobile_external_readiness.mjs.

ACCEPTANCE CRITERIA:
- [x] No public copy claims live native iOS/Android builds or store publishing as GA
- [x] Any mobile claim is labeled beta/coming-soon
- [x] verify_mobile_external_readiness.mjs passes its launch-safe gate
- [x] Source/export + Expo Web preview claims (which are true) remain

EVIDENCE:
2026-06-02: `node scripts/verify_mobile_external_readiness.mjs` passed the default launch-safe gate; `APEX_MOBILE_CHECK_LIVE=1 node scripts/verify_mobile_external_readiness.mjs` passed live platform truth for `mobile_source_generation=flagged_beta`, `mobile_eas_builds=gated`, and `mobile_store_submission=gated`. Focused grep found no `frontend/src/pages/Landing.tsx` native App Store / Google Play / native-build GA claims. Authenticated mobile project copy separates Expo Web preview, native binaries, and store upload as gated workflows.

VERIFICATION COMMAND:
node scripts/verify_mobile_external_readiness.mjs
! grep -in "app store\|google play\|native app\|native build" frontend/src/pages/Landing.tsx
```

```
TASK-112: Error-recovery visibility audit — eliminate silent failures in the build UI
Priority: P1
Category: Reliability
Estimated Time: 10 minutes
Owner: hernmes

CONTEXT:
Reliability target 8.5. Any state where a build dies, stalls, or a preview fails without a clear
user-facing message + recovery action is a silent failure that erodes the trust advantage.

FILES TO CHANGE:
- frontend/src/components/builder/AppBuilder.tsx
- frontend/src/components/builder/LiveActivityFeed.tsx

EXACT CHANGE REQUIRED:
Audit the build status state machine for terminal/failure states. Ensure every failure path
renders: (1) plain-English explanation, (2) recovery CTA (retry/restart/contact), (3) preserved
access to generated files. Confirm "preview unavailable" never renders as a dead-end. Confirm
WebSocket disconnect shows a reconnecting indicator.

ACCEPTANCE CRITERIA:
- [ ] No build failure state renders without explanation + recovery action
- [ ] "Preview unavailable" always offers a retry/restart path
- [ ] WebSocket disconnect shows a reconnecting state
- [ ] Generated files remain accessible after a failure
- [ ] No regression: AppBuilder tests pass

VERIFICATION COMMAND:
cd frontend && npm run test -- --run AppBuilder 2>&1 | tail -15
```

```
TASK-113: Lint + typecheck + build green gate on launch commit (frontend)
Priority: P1
Category: Reliability
Estimated Time: 8 minutes
Owner: hernmes

CONTEXT:
The evidence checklist requires npm run lint to pass alongside typecheck/test/build.

FILES TO CHANGE:
- Whatever files lint flags (fix -> no behavior change)

EXACT CHANGE REQUIRED:
Run frontend lint. Fix all errors. Prefer mechanical fixes (unused imports, exhaustive-deps) over
disabling rules. Do not refactor logic.

ACCEPTANCE CRITERIA:
- [ ] cd frontend && npm run lint passes with zero errors
- [ ] No eslint-disable added except with a justifying comment
- [ ] No regression: typecheck + build still pass after fixes

VERIFICATION COMMAND:
cd frontend && npm run lint && npm run typecheck && npm run build 2>&1 | tail -8
```

```
TASK-114: Run Playwright launch + preview-verify suites against production
Priority: P1
Category: Reliability
Estimated Time: 10 minutes
Owner: hernmes

CONTEXT:
The evidence checklist requires the Playwright launch smoke and preview-verify suite to pass.
These are our end-to-end guarantees that a real browser can sign up, see launch-ready state,
and view a generated preview.

FILES TO CHANGE:
- None (fixes spin off)

EXACT CHANGE REQUIRED:
Run the launch smoke with PLAYWRIGHT_EXPECT_LIVE_STRIPE=1 PLAYWRIGHT_EXPECT_LAUNCH_READY=1 and
the preview-verify suite, both on chromium against production.

ACCEPTANCE CRITERIA:
- [ ] test:launch passes (parity with prior 5 passed / 1 skipped or better)
- [ ] test:preview-verify passes
- [ ] Live Stripe + launch-readiness assertions hold
- [ ] Any failure escalated as P0

VERIFICATION COMMAND:
cd tests/e2e && PLAYWRIGHT_EXPECT_LIVE_STRIPE=1 PLAYWRIGHT_EXPECT_LAUNCH_READY=1 npm run test:launch -- --project=chromium 2>&1 | tail -15 && npm run test:preview-verify -- --project=chromium 2>&1 | tail -10
```

---

# SECTION 3: P2 TASKS — POLISH
### (complete during launch week)

---

```
TASK-201: UI declutter pass on the builder + IDE (cyberpunk theme consistency)
Priority: P2
Category: UI-Polish
Estimated Time: 10 minutes per surface
Owner: openclaw

FILES TO CHANGE:
- frontend/src/components/builder/AppBuilder.tsx, BuildScreen.tsx, OrchestrationOverview.tsx
- frontend/src/styles/

EXACT CHANGE REQUIRED:
Audit the builder and IDE for visual noise: reduce competing glows/animations to one focal point
per view, enforce consistent spacing scale, ensure typography hierarchy is clear. Reference
UI_POLISH_REPORT.md and theme-parity-report.md.

ACCEPTANCE CRITERIA:
- [ ] No more than one primary animated focal element per view
- [ ] Consistent spacing + typography scale across builder/IDE
- [ ] No unstyled or visually broken regions at common breakpoints
- [ ] No regression: component tests pass

VERIFICATION COMMAND:
cd frontend && npm run build 2>&1 | tail -5
```

```
TASK-202: Add presence indicators to the Monaco editor (collaboration surface)
Priority: P2
Category: Community
Estimated Time: 10 minutes
Owner: openclaw

FILES TO CHANGE:
- frontend/src/components/editor/ (Monaco host)
- frontend/src/hooks/useCollaboration.ts

EXACT CHANGE REQUIRED:
For Team-plan sessions, render connected collaborators' avatars in the editor header and remote
cursor positions/labels in Monaco using the presence data already flowing through useCollaboration.
Gate behind Team plan + feature flag.

ACCEPTANCE CRITERIA:
- [ ] Team sessions show collaborator avatars
- [ ] Remote cursor positions render with user labels
- [ ] Non-Team users are unaffected
- [ ] No regression: collaboration tests pass

VERIFICATION COMMAND:
cd frontend && npm run test -- --run collaboration 2>&1 | tail -10
```

```
TASK-203: Dependencies panel — add/remove controls (phase 2 of TASK-107)
Priority: P2
Category: IDE
Estimated Time: 10 minutes
Owner: openclaw

FILES TO CHANGE:
- frontend/src/components/ide/DependenciesPanel.tsx
- Wire to add/remove endpoints in backend/internal/handlers/packages.go

EXACT CHANGE REQUIRED:
Add an "Add package" input and a remove control per listed package, calling the packages add/remove
API and refreshing the list + manifest. Surface install success/failure inline.

ACCEPTANCE CRITERIA:
- [ ] User can add a package by name and see it installed + listed
- [ ] User can remove a package and see it gone
- [ ] Install/remove failures surface a clear inline error
- [ ] No regression: read-only listing still works

VERIFICATION COMMAND:
grep -n "addPackage\|removePackage\|install" frontend/src/components/ide/DependenciesPanel.tsx && cd backend && go test ./internal/handlers/ -run Packages 2>&1 | tail -5
```

```
TASK-204: Contextual tooltips during first build explaining each agent/stage
Priority: P2
Category: Onboarding
Estimated Time: 10 minutes
Owner: openclaw

FILES TO CHANGE:
- frontend/src/components/builder/OrchestrationOverview.tsx and/or LiveActivityFeed.tsx

EXACT CHANGE REQUIRED:
On a user's first build (gate via the onboarding localStorage flag), show dismissible coachmark
tooltips on the agent roles and build stages (Architect/Backend/Frontend/Reviewer), one-time,
non-blocking.

ACCEPTANCE CRITERIA:
- [ ] First build shows one-time contextual tooltips on agents/stages
- [ ] Tooltips are dismissible and never reappear after the first build
- [ ] They do not block interaction or obscure the cost ticker
- [ ] No regression

VERIFICATION COMMAND:
cd frontend && npm run test -- --run Orchestration 2>&1 | tail -10
```

```
TASK-205: Integrated database browser panel (read/simple query)
Priority: P2
Category: IDE
Estimated Time: 10 minutes
Owner: openclaw

FILES TO CHANGE:
- frontend/src/components/ide/ (new DatabasePanel.tsx)
- Wire to databases endpoints (handlers/databases.go)

EXACT CHANGE REQUIRED:
Add a Database tab to the IDE sidebar that lists tables for the project's connected DB and shows
rows for a selected table (paginated, read-only). Gate behind the build having provisioned a
database.

ACCEPTANCE CRITERIA:
- [ ] Database tab lists tables for a project with a provisioned DB
- [ ] Selecting a table shows paginated rows
- [ ] Projects without a DB show a graceful empty state
- [ ] No regression: other IDE panels work

VERIFICATION COMMAND:
grep -rn "database\|tables" frontend/src/components/ide/DatabasePanel.tsx 2>/dev/null && cd backend && go test ./internal/handlers/ -run Database 2>&1 | tail -5
```

```
TASK-206: Mobile-first build-progress flow (track a build from a phone)
Priority: P2
Category: UI-Polish
Estimated Time: 10 minutes
Owner: hernmes

FILES TO CHANGE:
- frontend/src/components/mobile/ (build-progress mobile view)
- frontend/src/hooks/useMobile.ts

EXACT CHANGE REQUIRED:
For mobile breakpoints, render a streamlined build experience: a prompt input, build-mode toggle,
and a vertical progress/activity feed with the cost ticker — without forcing the full desktop IDE
chrome.

ACCEPTANCE CRITERIA:
- [ ] On a phone viewport, a user can enter a prompt and start a build
- [ ] Build progress + cost are legible without horizontal scroll
- [ ] Completed preview is viewable on mobile
- [ ] No regression: desktop builder unaffected

VERIFICATION COMMAND:
cd frontend && npm run test -- --run mobile 2>&1 | tail -10
```

```
TASK-207: Deploy-to-Render/Vercel/Netlify prominence + one-click polish
Priority: P2
Category: Deployment
Estimated Time: 10 minutes
Owner: hernmes

FILES TO CHANGE:
- frontend/src/components/deployment/ (deploy panel prominence)
- frontend/src/components/builder/AppBuilder.tsx (post-build deploy CTA)

EXACT CHANGE REQUIRED:
On build completion, surface a prominent "Deploy" action listing Render/Vercel/Netlify with
one-click flows and live deploy status (queued->building->live + the resulting URL). Show clear
auth/connect prompts if a provider isn't connected.

ACCEPTANCE CRITERIA:
- [ ] Completed builds show a prominent Deploy action with provider choices
- [ ] One-click deploy shows live status and returns a live URL
- [ ] Unconnected providers show a connect prompt, not an error
- [ ] No regression: export/download still works

VERIFICATION COMMAND:
grep -rn "deploy\|Deploy" frontend/src/components/deployment/ | head && grep -n "deploy" frontend/src/services/api.ts | head
```

```
TASK-208: Pricing/value clarity pass on landing (vs Replit cycles)
Priority: P2
Category: Cost-Transparency
Estimated Time: 8 minutes
Owner: hernmes

FILES TO CHANGE:
- frontend/src/pages/Landing.tsx (pricing section)

EXACT CHANGE REQUIRED:
Ensure the pricing section shows accurate plan prices (Builder $24/mo, Pro $59/mo, Team $149/mo),
what each includes (power modes, credits, BYOK, persistent preview), and a clear "transparent
credits, no mystery cycles" value line. Verify amounts match PRICING_STRATEGY.md.

ACCEPTANCE CRITERIA:
- [ ] Plan prices match backend plans exactly (Builder $24 / Pro $59 / Team $149)
- [ ] Each plan lists power modes, monthly credits, BYOK, persistent preview
- [ ] Transparent-credits value framing present
- [ ] No regression: pricing CTAs route to checkout correctly

VERIFICATION COMMAND:
grep -in "24\|59\|149\|cycles\|transparent\|credits" frontend/src/pages/Landing.tsx | head
```

```
TASK-209: Support/incident checklist + status page link reviewed
Priority: P2
Category: Reliability
Estimated Time: 8 minutes
Owner: hernmes

FILES TO CHANGE:
- docs/launch-runbook.md (incident response + support section)

EXACT CHANGE REQUIRED:
Confirm launch-runbook.md has: who is on-call, how users report issues, escalation path for P0,
the rollback procedure (links TASK-007), and key dashboards/log locations.

ACCEPTANCE CRITERIA:
- [ ] Incident response path documented for P0 categories
- [ ] On-call owner named; escalation defined
- [ ] Rollback procedure referenced
- [ ] Team confirmed briefed

VERIFICATION COMMAND:
grep -iA3 "incident\|on-call\|escalation\|rollback" docs/launch-runbook.md | head -30
```

---

# SECTION 4: P3 TASKS — NICE-TO-HAVE
### (post-launch-week backlog)

```
TASK-301: Full real-time co-editing with conflict resolution (beyond presence)
Priority: P3 | Category: Community | Owner: openclaw
CONTEXT: Presence ships in TASK-202; full CRDT/OT co-editing is a larger effort better validated
post-launch with real Team demand.
VERIFY: cd frontend && npm run test -- --run collaboration
```

```
TASK-302: Native mobile build pipeline (EAS Build/Submit) — un-gate with real evidence
Priority: P3 | Category: Deployment | Owner: openclaw
CONTEXT: Tracker mandates real EAS/Apple/Google evidence before native claims. Post-launch, run a
real mobile project through EAS Build/Submit and capture store-readiness evidence.
VERIFY: APEX_MOBILE_EXPECT_NATIVE_READY=1 node scripts/verify_mobile_external_readiness.mjs
```

```
TASK-303: Build-learning / prompt-evolution tuning from launch telemetry
Priority: P3 | Category: Build-Quality | Owner: hernmes
CONTEXT: build_learning.go + prompt_evolution.go exist. After launch, feed real failure taxonomy
back to raise first-pass quality further.
VERIFY: cd backend && go test ./internal/agents/ -run Learning
```

```
TASK-304: Public API docs polish + developer onboarding for API_CONTRACT.md
Priority: P3 | Category: IDE | Owner: hernmes
CONTEXT: API_CONTRACT.md (27KB) and docs/api.md exist. Polishing public API docs serves technical
founders (Segment C).
VERIFY: grep -c "POST\|GET" API_CONTRACT.md
```

```
TASK-305: Onboarding A/B instrumentation for free->paid conversion measurement
Priority: P3 | Category: Onboarding | Owner: hernmes
CONTEXT: Success metric is free->paid > 8%. Instrument the new guided onboarding (TASK-103).
VERIFY: grep -rn "track\|analytics\|funnel" frontend/src/components/builder/OnboardingTour.tsx
```

```
TASK-306: Explore page SEO + shareable published-app pages
Priority: P3 | Category: Community | Owner: hernmes
CONTEXT: Drives organic growth. Give published apps shareable, indexable pages.
VERIFY: curl -s https://apex-build.dev/explore | grep -i "og:\|meta"
```

---

# LAUNCH CONFIDENCE ASSESSMENT

## Current state vs Replit (verified from code, 2026-05-25)

| # | Category | Replit | Apex now | Notes |
|---|---|---|---|---|
| 1 | Build Quality | 6 | 7.0 | Paid-balanced passed; paid-max and diverse matrix remain unproven |
| 2 | Speed | 7 | 6.0 | Inconsistent preview load; not yet measured per-tier |
| 3 | UI Polish | 7 | 6.0 | Distinctive but cluttered |
| 4 | Onboarding | 9 | 5.5 | Current tree adds starter prompts and blank-IDE escape hatch; deployed first-run evidence still needed |
| 5 | Cost Transparency | 4 | 8.0 | Built but under-surfaced (our moat) |
| 6 | Deployment | 6 | 5.0 | Always-on + providers exist but hidden |
| 7 | IDE | 6 | 7.0 | Monaco+BYOK+git already > Replit |
| 8 | Community | 4 | 4.5 | Explore is linked from landing/app nav; landing proof, onboarding link, and focused tests still pending |
| 9 | Billing | 5 | 8.0 | Idempotency real; real payment observed; controlled checkout/portal/plan-change/cancellation/webhook replay still open |
| 10 | Reliability | 6 | 7.0 | Good logging; recovery paths unproven |

## After P0 + P1 complete (projected)

| # | Category | Replit | Apex | Margin |
|---|---|---|---|---|
| 1 | Build Quality | 6 | 8.5 | +42% |
| 2 | Speed | 7 | 7.8 | +11% |
| 3 | UI Polish | 7 | 7.0 | 0% |
| 4 | Onboarding | 9 | 7.5 | -17% |
| 5 | Cost Transparency | 4 | 9.0 | +125% |
| 6 | Deployment | 6 | 7.5 | +25% |
| 7 | IDE | 6 | 8.0 | +33% |
| 8 | Community | 4 | 6.5 | +63% |
| 9 | Billing | 5 | 8.5 | +70% |
| 10 | Reliability | 6 | 8.5 | +42% |

## After P0 + P1 + P2 complete (projected)

| # | Category | Replit | Apex | Margin |
|---|---|---|---|---|
| 1 | Build Quality | 6 | 8.7 | +45% |
| 2 | Speed | 7 | 8.0 | +14% |
| 3 | UI Polish | 7 | 8.5 | +21% |
| 4 | Onboarding | 9 | 8.5 | -6% |
| 5 | Cost Transparency | 4 | 9.2 | +130% |
| 6 | Deployment | 6 | 8.0 | +33% |
| 7 | IDE | 6 | 8.7 | +45% |
| 8 | Community | 4 | 7.0 | +75% |
| 9 | Billing | 5 | 8.7 | +74% |
| 10 | Reliability | 6 | 8.7 | +45% |

**After P0+P1+P2: ~8.4/10 vs Replit ~6.0. +40% composite margin.**

---

## EXECUTION SEQUENCING (10-day cadence)

- **Day 1-2:** TASK-004, 005 (paid canaries) + TASK-101, 102, 105, 106 (cheap surfacing wins). TASK-008 baseline test gate.
- **Day 3-4:** TASK-009 (20-prompt matrix) + TASK-103 (guided onboarding) + TASK-104 (persistent preview).
- **Day 5-6:** TASK-006, 007, 010, 011, 012 (canary workflow, rollback, load, concurrency dedup, recovery) + TASK-107, 108, 109.
- **Day 7:** TASK-110, 111, 112, 113, 114 — full P1 green.
- **Day 8:** P2 batch (201, 207, 208 prioritized).
- **Day 9:** Launch Readiness Verification — re-run full matrix, full test suite, Playwright, canary; final launch authorization sign-off.
- **Day 10:** Launch window + on-call (TASK-209).

**Hard gate:** Broad paid public launch must not proceed until TASK-004 through TASK-012 are checked
or have explicit launch-owner acceptance notes. Current live checkout exposure/control must be
confirmed by an admin as disabled, allowlisted, or intentionally risk-accepted before additional
customers are directed to paid signup.

---

## RELEVANT FILES

- Build engine: `backend/internal/agents/{handlers.go,build_spec.go,manager.go,preview_gate.go,compile_validator.go}`
- Payments: `backend/internal/payments/{stripe.go,bootstrap.go,launch_readiness.go}`, `backend/internal/handlers/payments.go`
- Preview/deploy: `backend/internal/preview/`, `backend/internal/deploy/alwayson/service.go`, `backend/internal/hosting/service.go`
- Frontend: `frontend/src/components/builder/{AppBuilder.tsx,OnboardingTour.tsx,TemplateGallery.tsx}`, `frontend/src/pages/{Landing.tsx,Explore.tsx}`, `frontend/src/App.tsx`
- Status: `docs/{launch-readiness-tracker.md,replit-overtake-roadmap.md,launch-runbook.md}`
- Verifiers: `scripts/{verify_stripe_launch.mjs,verify_render_launch_env.mjs,verify_mobile_external_readiness.mjs}`

---

## OVERSIGHT NOTE

Claude Code is acting as orchestrator/supervisor for this war plan. openclaw and hernmes are the executing agents. All P0 tasks require verification before the launch gate is cleared. Neither agent should push to production or modify Stripe/Render configuration without human (Spencer) sign-off.

*End of War Plan. 10 days. 10% superiority minimum. 100% build and preview success.*
