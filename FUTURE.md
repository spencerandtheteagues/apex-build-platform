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

## Current Priority Directive (Updated 2026-05-26)

The 2026-05-25 build-stall audit produced fixes that are now reflected in launch docs. Do not use older May 24 failure notes as current state without rechecking production evidence.

### Current evidence baseline

- As of 2026-05-30, production health remains green: `/health` ready/healthy, `/health/features` phase ready, critical 6/6, optional 40/40, Redis connected, Stripe launch config ready, E2B execution launch-ready, preview Docker launch-ready, and browser runtime proof ready.
- Strict Render launch verification and safe Stripe launch verification passed locally on 2026-05-30; controlled billing lifecycle and real webhook replay remain open.
- Local launch verification is green on 2026-05-30: `bash scripts/verify_all.sh` passed backend build/vet/test plus frontend typecheck/Vitest/lint/build, and the serialized backend gate passed with `go test -p 1 -parallel 4 ./... -timeout 20m`.
- Safe production Playwright checks passed on 2026-05-30: launch smoke `5 passed / 1 skipped` and preview verification health wiring `3 passed / 4 skipped`. Authenticated build-generation checks remain skipped until verified canary credentials are supplied.
- Public k6 load testing on 2026-05-30 exposed intermittent production `/ready` 503s under 200 public VUs. Commit `f01dfac` hardened the k6 gate to fail on any public 5xx and routed `/ready` through a lightweight readiness handler instead of per-request DB pinging.
- After deploying `f01dfac`, the hardened public 200-VU k6 gate passed with `public_5xx_errors count=0`, landing p95 `32.07ms`, and health p95 `99ms`. Public load evidence is green; authenticated API and build-start load evidence remain credential-gated.
- After the local `/ready` change, `bash scripts/verify_all.sh` is green and public generated Playwright smoke passed against production. Static Render/mobile verifiers also pass in no-secret mode, but strict Render/mobile evidence remains credential-gated.
- Ollama Cloud model discovery sees `kimi-k2.6`, `glm-5.1`, and `deepseek-v4-pro`, but all generation attempts are blocked by the `apexbuildai` account weekly usage limit. Do not queue Ollama-pinned live canaries until the quota is raised or extra usage is added.
- A fresh non-Ollama free frontend live canary was attempted on 2026-05-30 but build start was rejected with `email_not_verified` for the disposable account. A later disposable free canary account was verified through the admin path and reached build execution, proving the auth prerequisite can be cleared without storing canary credentials.
- Free frontend canaries on 2026-05-30 exposed, then closed, a sequence of preview blockers: stale React/Vite/plugin manifests, Vite/plugin peer conflicts, missed Vite ready logs under Render churn, and placeholder/loading-only first screens. Commits `aaee9d6`, `18171c4`, `e049318`, and `eae9320` add deterministic repair coverage for those cases.
- Post-deploy free frontend canary `ddf92e65-9817-4141-933a-7d0a2eac0fe3` passed on 2026-05-31 UTC: completed at 100%, quality gate passed, reliability summary clean, 22 generated files. The smoke did not assert an already-running preview instance (`preview/status` was not active), but internal `require_preview_ready` completed successfully.
- Git identity and stale orphan-build cleanup are already handled.
- TASK-004 paid balanced full-stack canary functionally passed on 2026-05-25 with build `69d3582e`; screenshot/console artifact archival remains open in the launch tracker.
- Free-fast and fast frontend-only canaries have prior passing evidence.
- Live health read on 2026-05-26 08:03 UTC reported `ready=true`, `feature_readiness_status=healthy`, 5/7 providers healthy, E2B code execution launch-ready, preview service launch-ready, and runtime browser proof enabled.
- Gemini and Grok remain degraded from provider billing/permission state and must not be hidden if 7/7 provider health is part of the launch claim.

### Priority action queue

1. Keep local verification green on the current tree before pushing.
2. Provision reusable verified launch canary credentials for one free account and one paid account; store them only in the approved secret manager or transient operator environment, never in repo files. Disposable admin-verified free accounts are acceptable for one-off proof but are not a durable launch gate.
3. Clear the Ollama Cloud quota block before running Ollama-pinned live canaries; then use `kimi-k2.6`, `glm-5.1`, and `deepseek-v4-pro` as the first three model-specific live proofs.
4. Pass and document TASK-005 max-power full-stack canary after the latest code is deployed and paid canary credentials/provider budget are available.
5. Verify paid preview gating with the paid canary account now that the verified free frontend path has fresh passing evidence.
6. Complete Stripe launch evidence through controlled live checkout, billing portal, upgrade/downgrade, cancellation, and real webhook replay. Use credentials from the secret manager or admin-controlled environment only.
7. Complete production canary, rollback drill, failed-build restart, authenticated build-load, and the diverse prompt matrix when required GitHub/Render/Stripe/admin access is available.

Do not skip docs. The docs-first contract in `AGENTS.md` is binding: update launch docs when evidence, blockers, or stable workflows change. The rule is to avoid claiming readiness before evidence exists, not to postpone documentation until later.

## Active Pathways

| ID | Status | Area | Pathway | Next Action | Evidence Gate |
| --- | --- | --- | --- | --- | --- |
| FT-001 | Queued | Architecture Intelligence | Reference counting for AI agents. | Defer until FT-002/003 pass. | Admin map shows counts. |
| FT-002 | Active | First-Pass Reliability | Balanced full-stack canary passed; max and broad matrix still need proof. | Run paid max canary and then the 20-prompt matrix after deploy/access is available. | Paid max and diverse matrix complete to 100% with working previews. |
| FT-003 | Active | Preview Stability | Fast and balanced preview evidence exists; max/matrix preview breadth still needs proof. | Capture screenshot/console evidence for max and matrix builds. | Preview loads, no white screen, stays up 30s, screenshot captured for each launch-critical profile. |
| FT-004 | Queued | Knowledge Pockets | Defer until FT-002/003/006 complete. | No action. | N/A |
| FT-005 | Deferred | Mobile Output | Lower priority than core build reliability for launch. | No action until core paths are stable. | N/A |
| FT-006 | Active | Billing Launch Readiness | Stripe checkout must work end-to-end. | Run an admin-approved controlled live checkout/lifecycle pass with a canary payment method; do not use Stripe test cards against live mode. | Successful paid checkout, plan upgrade in DB, billing portal access, upgrade/downgrade, cancellation, and real webhook replay evidence. |
| FT-007 | Complete | Render Launch Readiness | Render env healthy, preview environments now enabled. | Rerun after env changes. | Completed 2026-05-09; updated 2026-05-25 with preview envs enabled. |
| FT-008 | Active | Free vs Paid Preview Gate | Free users: frontend-only preview. Paid users: full backend. | Test with free account, verify gate, verify upgrade prompt. | Free: frontend only + upgrade prompt. Paid: full backend unlocked. |

## Pathway Template

```md
| FT-000 | Proposed | Area | One-sentence pathway. | Smallest useful next action. | Concrete proof required before marking complete. |
```

## Completion Standard

A pathway is not complete because code was written. It is complete only when the relevant tests pass, contract edges are covered, user-facing behavior is verified when applicable, and any remaining risk is explicitly documented.
