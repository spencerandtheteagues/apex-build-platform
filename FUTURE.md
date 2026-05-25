# Future Pathways

This file is the shared planning surface for Apex Build feature pathways. Read it before non-trivial work on APEX-BUILD.DEV, especially build reliability, preview stability, orchestration, provider routing, architecture intelligence, billing, or release readiness.

## Operating Rules

- Treat this file as guidance, not runtime truth. Repository code, tests, logs, production telemetry, and current contracts remain authoritative.
- Keep entries concise, evidence-based, and actionable. Each pathway should name the next concrete step and the proof needed before claiming completion.
- Do not store secrets, API keys, customer data, full prompts, provider transcripts, or private production payloads here.
- Update this file when a pathway materially changes, completes, gets blocked, or when a better next step is discovered.
- Prefer low-risk reliability work before speculative feature expansion.

## North Star

APEX-BUILD.DEV should reliably turn a user prompt into a complete project, reach 100% build completion, and open a stable running preview with evidence the user can trust.

## PRIORITY DIRECTIVE (Written 2026-05-25 by Claude audit — READ THIS FIRST)

Spencer ran a full audit. The following is the authoritative action queue before anything else.

### ALREADY FIXED (no further action needed)
- Git email fixed on VPS: `spencerandtheteagues@gmail.com` (was root@hostname, was blocking GitHub pushes with GH007)
- Orphan builds cancelled (IDs 6b52a8cf, 3e7f907f — stuck in_progress for 4+ hours)

### WHAT IS BROKEN RIGHT NOW
**Last successful production builds: May 1-4. All May 24 builds failed:**
- Two 99% failures: auth contract bug — FIXED by commit d3ac616, deployed at 22:44 May 24
- Two orphans: cancelled (stale from container restart)
- One 0% failure at 23:41 May 24: planning stall — Ollama Cloud failed, ALL fallbacks also failed

**Root cause of 0% stall:** `planningProviderOrder()` balanced-mode places Ollama Cloud first (5 models: kimi-k2.6, glm-5.1, deepseek-v4-pro, deepseek-v4-flash, qwen3.5). All get 120s each = 10min cap. Then falls through to `rankedFallbackProvidersForTask` and `configuredPlatformPlanningFallbackProviders`. If Ollama Cloud is slow AND non-Ollama fallbacks also fail, build stalls at 0% → orchestrator waits 40min → timeout. This happened at 23:41 when Ollama Cloud was apparently unhealthy.

### PRIORITY ACTION QUEUE — WORK THROUGH IN ORDER

**STEP 1: Run a live build test immediately and observe to completion**
- Admin login: username=`admin`, password=`TheStarsh1pKEY!`
- URL: https://apex-build.dev
- Prompt: "Build a simple React todo app with Node.js/Express backend and PostgreSQL database. Include JWT user auth, task CRUD, and a clean dashboard UI."
- Mode: balanced
- Monitor for 15 minutes. At 0% stall > 3min: Ollama Cloud is failing.
- At 100%: take screenshot of preview.
- Evidence: screenshot of 100% complete build with working preview.

**STEP 2: If 0% stall — diagnose and fix Ollama Cloud**
- Test Ollama Cloud directly from VPS terminal (use OLLAMA_API_KEY from Render env vars, NOT from this repo)
- If Ollama Cloud fails: set `APEX_BALANCED_OLLAMA_PLANNING_MODELS=` empty in Render to disable Ollama planning chain
- OR add Claude/GPT-4 as explicit fallback at end of `ollamaBalancedPlanningModelFallbacks()` in `build_spec.go`

**STEP 3: Verify free vs paid preview distinction**
- Register a FREE account at apex-build.dev (not admin)
- Try creating a backend app — should be frontend-only or blocked
- Verify upgrade prompt appears
- Evidence: screenshot showing free user sees frontend preview only

**STEP 4: Test Stripe billing (FT-006)**
- Use Stripe test card `4242 4242 4242 4242`
- Complete checkout for Builder plan
- Verify subscription_type updates in DB (check via admin panel or API)
- Verify backend access unlocks after upgrade
- Evidence: DB shows plan=builder after checkout

**STEP 5: Fix any issues found and confirm clean matrix**
- If build fails, diagnose and fix the specific blocker
- Re-run build tests until 2+ builds complete to 100% with working previews

**DO NOT WASTE TIME ON:**
- Skill setup (skills are complete and saved correctly)
- Browser-harness debugging (use Python requests for API testing)
- render.yaml tweaks (infrastructure is correctly configured)
- Writing docs before evidence gates are passed

## Active Pathways

| ID | Status | Area | Pathway | Next Action | Evidence Gate |
| --- | --- | --- | --- | --- | --- |
| FT-001 | Queued | Architecture Intelligence | Reference counting for AI agents. | Defer until FT-002/003 pass. | Admin map shows counts. |
| FT-002 | **BLOCKING** | First-Pass Reliability | Last 4 builds failed. Must verify current deployment completes to 100%. | Run fresh build (balanced, full-stack with auth) and observe. | 2+ builds complete to 100% with no 0%/99% failure. |
| FT-003 | **BLOCKING** | Preview Stability | Must verify preview loads after successful build. | After FT-002 first success, screenshot the preview. | Preview loads, no white screen, stays up 30s, screenshot captured. |
| FT-004 | Queued | Knowledge Pockets | Defer until FT-002/003/006 complete. | No action. | N/A |
| FT-005 | Deferred | Mobile Output | Lower priority than core build reliability for launch. | No action until core paths are stable. | N/A |
| FT-006 | Active | Billing Launch Readiness | Stripe checkout must work end-to-end. | Test checkout with test card in live env. | Successful test checkout, plan upgrades in DB, billing portal accessible. |
| FT-007 | Complete | Render Launch Readiness | Render env healthy, preview environments now enabled. | Rerun after env changes. | Completed 2026-05-09; updated 2026-05-25 with preview envs enabled. |
| FT-008 | Active | Free vs Paid Preview Gate | Free users: frontend-only preview. Paid users: full backend. | Test with free account, verify gate, verify upgrade prompt. | Free: frontend only + upgrade prompt. Paid: full backend unlocked. |

## Pathway Template

```md
| FT-000 | Proposed | Area | One-sentence pathway. | Smallest useful next action. | Concrete proof required before marking complete. |
```

## Completion Standard

A pathway is not complete because code was written. It is complete only when the relevant tests pass, contract edges are covered, user-facing behavior is verified when applicable, and any remaining risk is explicitly documented.
