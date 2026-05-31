# Lane L8 — Reliability (audit 2026-05-16)

Source: Wave-1 Auditor. Passing: free-fast + fast FE-golden. Not run: paid-balanced/paid-max (credential-gated).

## HIGH-RISK credit-burn (fix first)

| ID | Tier | File:line | Bug |
| --- | --- | --- | --- |
| L8-HI-003 | HIGH-RISK | manager.go:7785 | `if limit<=0 { return true }` INVERTED — env kill-switch `=0` makes loop UNBOUNDED. Fix → `return false` |
| L8-HI-001 | HIGH-RISK | manager.go:7842 | `fix_integration_contract` falls through to defaultLimit=3 (should be 1) |
| L8-HI-002 | HIGH-RISK | manager.go:7840 | `fix_tests` keeps defaultLimit=3 (should be 2) |

## HIGH — stall / first-pass

| ID | Tier | File:line | Gap |
| --- | --- | --- | --- |
| L8-001 | HIGH | manager.go:21774; preview_gate.go:579 | Progress hard-pinned at 95 during all recovery → looks stalled (FT-002). Step 95→98 |
| L8-002 | HIGH | manager.go:21240 | repeated-class exhaustion fires at attempts≥3 = solver cap → solver bypassed on final attempt. Use `> maxAutomatedRecoveryAttempts` |
| L8-003 | HIGH | preview_gate.go:192 | Working fallback shell hard-failed before LLM repair; cancels recovery. Launch fix_preview_verification first |

## MEDIUM / LOW

| ID | Tier | File:line | Gap |
| --- | --- | --- | --- |
| L8-004 | MED | run_live_golden_build.mjs:807 | Stability window misses HMR silent reloads; diff bodyTextLength ±20% |
| L8-005 | MED | preview_gate.go:448 | js_runtime_error w/ non-keyword details misses shell fallback; add TypeError keywords |
| L8-006 | MED | run_platform_canary_matrix.sh:58 | Paid canary creds never provisioned; REQUIRE_PAID_SCENARIOS=0 skips (external) |
| L8-007 | MED | run_live_golden_canary_matrix.sh:12 | Golden never run balanced/max (external canary acct) |
| L8-008 | LOW | manager.go:7766 | nil build/empty action → return true (should false) |
| L8-009 | LOW | manager.go:22411 | snapshot upsert `>=` ts → same-ms clobber; use `>` |
| L8-010 | LOW | run_platform_build_smoke.sh:280 | paid_fullstack smoke doesn't assert backend artifact presence |

**Execution order:** L8-HI-003 → L8-HI-001 → L8-HI-002 (all manager.go ~7780-7860, ~T0/T1, pure money) → L8-002 → L8-001 → L8-003 → L8-008/009 → MED. L8-006/007 external-gated (overlaps L1 BLK-4). All manager.go/preview_gate.go — **same worktree+owner as L2/L5 (shared files); strictly serialized**.
