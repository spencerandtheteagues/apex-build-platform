# Lane L2 — Builder Hardening (audit 2026-05-16)

Source: Wave-1 Auditor vs docs/builder-hardening-plan.md. **0 done, 2 partial, 11 not done.** Highest build-reliability leverage in the program.

| ID | Tier | File:line | Gap | Status |
| --- | --- | --- | --- | --- |
| H-01 | CRITICAL | manager.go:6527; orchestration_contracts.go:659 | Plan freeze enrichment all inside `EnableIntentBrief/BuildContract/ValidatedBuildSpec` flag block — flag flip → advisory-only empty shell | NOT DONE |
| H-02 | CRITICAL | planning_contracts.go:334 | TaskStartAck/CompletionReport downgraded to non-fatal logs; "no ACK no execution" not enforced | NOT DONE |
| H-03 | HIGH | manager.go:6220,22977 | Forbidden-file strip runs post-`TaskCompleted`; no-work-order builds skip ownership check | PARTIAL |
| H-04 | HIGH | manager.go:24451; build_spec.go:532 | `spec_hash` threaded but never compared at execution → stale tasks run vs new plan | NOT DONE |
| H-05 | HIGH | manager.go:28306 | Team coordination = 6×180-char summaries; no spec hash / ownership map / API contract | NOT DONE |
| H-06 | HIGH | manager.go:5481; ai_adapter.go:512 | No per-task MaxFiles/MaxOutputChars; only 25% token cut on retry | NOT DONE |
| H-07 | HIGH | manager.go:25969 | Intervention steering freeform; no engine_actions enum (provider/power/replan applied atomically) | NOT DONE |
| H-08 | MED | orchestration_contracts.go:3524; manager.go:4746 | Failure classifier missing artifact_truncation/spec_drift/etc; truncation cuts context not batch | PARTIAL |
| H-09 | MED | orchestration_contracts.go:729 | Empty acceptance seed doesn't block build start (no must_have gate) | NOT DONE |
| H-10 | MED | types.go:673; planning_contracts.go:236 | TaskStartAck.Dependencies parsed, never validated vs allowed list | NOT DONE |
| H-11 | MED | manager.go:6748 | Gate 2 scaffold boot smoke (npm install + build) not implemented | NOT DONE |
| H-12 | LOW | manager.go:24472 | Acceptance checklist is prompt context, not a hard completion gate | NOT DONE |
| H-13 | LOW | manager.go:28465 | No DesignSpec struct; design is inline planner prose | NOT DONE |

**Execution order:** H-01 (unconditional plan-freeze + kill-switch-only env) → H-02 (ACK soft-retry / synthesize completion) → H-04 (spec_hash staleness guard) → H-06 (WorkOrder MaxFiles/MaxChars + follow-up split) → H-05/H-07/H-03 → MED → LOW. Every change proven by the named unit test + paid full-stack canary. All in `backend/internal/agents` — coordinate with L5 (same files) to avoid collisions; single Hardener owns this lane in a worktree.
