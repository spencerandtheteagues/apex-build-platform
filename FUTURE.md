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

## Active Pathways

| ID | Status | Area | Pathway | Next Action | Evidence Gate |
| --- | --- | --- | --- | --- | --- |
| FT-001 | Active | Architecture Intelligence | Count which directories, databases, structures, and contracts AI agents reference during builds, then improve the most-used knowledge pockets. | Rank the hottest live plus historical reference pockets and enrich only the top source-of-truth gaps. | Admin map aggregates live and completed snapshot counts; backend tests prove metadata-only storage and terminal snapshot aggregation. |
| FT-002 | Active | First-Pass Reliability | Reduce stalls near 95% and make failed validation/preview states produce deterministic repair actions instead of credit-burning loops. | Deploy the planning hard-deadline/cancel patch, rerun the live golden canary, then run the paid full-stack canary matrix once provider throughput is clear. | Repeated prompt matrix runs with completed builds, stable preview screenshots, and logged failure classification. |
| FT-003 | Active | Preview Stability | Make generated app previews start consistently and stop white-screen restart loops. | Rerun live preview screenshot proof after the placeholder-only preview gate and planning-stall patch are both deployed. | `/health/features` reports preview launch readiness and browser proof clean, then preview canary reaches ready state, stays loaded, and captures screenshot evidence after refresh. |
| FT-004 | Queued | Knowledge Pockets | Use reference-count telemetry to identify high-value docs, schemas, and templates that agents actually consult. | Add a top-pocket report that separates live, historical, and combined rankings before any prompt injection. | Admin report shows counts by node, directory, database, and structure from both live builds and terminal snapshots. |
| FT-005 | Active | Mobile Output | Add first-class mobile app generation as its own path, led by Expo/React Native and kept separate from responsive web or Capacitor wrapping. | Run `scripts/verify_mobile_external_readiness.mjs` with a real mobile project before enabling public native/store claims. | Generated Expo source validates locally, exports cleanly, native/store flags stay gated by default, and strict external readiness proves credentials, native artifacts, store package, and submission evidence. |
| FT-006 | Active | Billing Launch Readiness | Make Stripe subscription, credit purchase, and invoice handling safe enough for public signup. | Replay real Stripe test webhooks through the deployed endpoint, then run a controlled live checkout and billing portal pass. | `/health/features` reports payments ready, local replay tests pass, the launch verifier passes in strict mode, Stripe CLI/dashboard replay proves duplicate delivery, and controlled live checkout plus billing portal flows succeed. |
| FT-007 | Complete | Render Launch Readiness | Prove the deployed Render environment matches the production blueprint and launch runtime expectations. | Keep strict Render verification in the launch gate and rerun it after env or deploy changes. | Completed 2026-05-09: strict Render verification passed against live services, required env vars were present without printing values, Redis/runtime/preview browser proof were ready, and production smoke canaries passed on `main` commit `2358a30`. |

## Pathway Template

Use this template for new entries:

```md
| FT-000 | Proposed | Area | One-sentence pathway. | Smallest useful next action. | Concrete proof required before marking complete. |
```

## Completion Standard

A pathway is not complete because code was written. It is complete only when the relevant tests pass, contract edges are covered, user-facing behavior is verified when applicable, and any remaining risk is explicitly documented.
