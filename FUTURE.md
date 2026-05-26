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

- Git identity and stale orphan-build cleanup are already handled.
- TASK-004 paid balanced full-stack canary functionally passed on 2026-05-25 with build `69d3582e`; screenshot/console artifact archival remains open in the launch tracker.
- Free-fast and fast frontend-only canaries have prior passing evidence.
- Live health read on 2026-05-26 08:03 UTC reported `ready=true`, `feature_readiness_status=healthy`, 5/7 providers healthy, E2B code execution launch-ready, preview service launch-ready, and runtime browser proof enabled.
- Gemini and Grok remain degraded from provider billing/permission state and must not be hidden if 7/7 provider health is part of the launch claim.

### Priority action queue

1. Keep local verification green on the current tree before pushing.
2. Pass and document TASK-005 max-power full-stack canary after the latest code is deployed and paid canary credentials/provider budget are available.
3. Verify free vs paid preview gating with a fresh free account and a paid canary account; record evidence without storing credentials.
4. Complete Stripe launch evidence through controlled live checkout, billing portal, upgrade/downgrade, cancellation, and real webhook replay. Use credentials from the secret manager or admin-controlled environment only.
5. Complete production canary, rollback drill, failed-build restart, load test, and the diverse prompt matrix when required GitHub/Render/Stripe/admin access is available.

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
