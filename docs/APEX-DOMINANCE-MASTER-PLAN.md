# APEX BUILD — DOMINANCE MASTER PLAN

**Status:** ACTIVE — single source of truth. Supersedes the *coordination* role of every other roadmap doc (those remain as detailed lane references, not competing plans).
**Created:** 2026-05-16
**Owner:** Spencer + Conductor (autonomous execution loop)
**Mission:** Apex Build is, without doubt, the best AI app builder on the market — the best build, most of the time, for the least money, with the smartest agents, producing the most useful end product, able to handle the most complex app in the industry with ease.

---

## 0. Grounding Truth (verified 2026-05-16, not assumed)

This plan is built on the *actual* repo state, not vibes:

- `backend`: `go build ./...` clean, `go vet ./...` clean, package tests pass. 172 Go test files.
- `frontend`: `tsc --noEmit` clean. 40 frontend test files.
- Working tree clean on `main`, even with origin; recent commits already about launch-readiness hardening.
- ~6,900 Go/TS source files. ~99 TODO/FIXME/HACK/stub/panic markers in non-test code.
- Extensive prior planning exists and is *good*: `launch-readiness-tracker.md`, `replit-overtake-roadmap.md`, `builder-hardening-plan.md`, `FUTURE.md`, `overhaul.md`, plus pricing/unit-economics CSVs.

**Conclusion:** This is not a rescue. It is a *consolidation + relentless finish*. The failure mode to beat is fragmentation, not broken code.

---

## 1. The Triage Taxonomy — every tiny issue has a home

Every issue — a 3-second copy fix or a 3-month redesign — is assigned exactly one **Tier** and one **Lane**. Nothing is "too small to track" and nothing is "too big to start."

| Tier | Meaning | Effort | Handling |
| --- | --- | --- | --- |
| **T0** | Trivial / cosmetic / one-liner | < 5 min | Fixed inline during any sweep touching that file; never deferred. |
| **T1** | Small, isolated, low-risk | < 1 hr | Batched into a lane's "quick wins" wave. |
| **T2** | Standard feature/bugfix | < 1 day | Normal lane work item, single agent, gated. |
| **T3** | Multi-file feature / subsystem change | 1–5 days | Lane epic, scoped agent in worktree isolation, design note required. |
| **T4** | Cross-cutting / architectural | 1–4 wks | Design doc + phased delivery + behind a flag. |
| **T5** | Strategic redesign | 1–3 mo | Written RFC, milestone breakdown, each milestone becomes T2/T3 items. |

**Rule:** A T5 is never an excuse to not start. It is immediately decomposed into its first T2/T3 milestone, which enters a wave this cycle. "It has a spot and it will get done."

---

## 2. The Lanes (the consolidated backlog spine)

Every existing doc folds into one of these 10 lanes. Lanes run in parallel; items within a lane are tiered and waved.

| Lane | Task # | Absorbs | Win condition |
| --- | --- | --- | --- |
| **L0 QA & Verification Gate** | #1 | "Evidence Required For Public Launch" | `scripts/verify_all.sh` green; no merge without it. |
| **L1 Launch Blockers** | #2 | launch-readiness-tracker "Blockers Still Open"; FUTURE FT-006 | Every blocker has its named evidence artifact. |
| **L2 Builder Hardening** | #3 | builder-hardening-plan.md (597 lines) | Plan freeze + file ownership + anti-truncation; paid canary green. |
| **L3 Instant Start** | #4 | replit-overtake gaps #1, #8 | Blank/template/GitHub-import path; first-build < 5 min. |
| **L4 Persistence** | #5 | replit-overtake gap #2; alwayson service | Persistent paid preview + deploy surfaced + warm/cold status. |
| **L5 Cost/Smart Routing** | #6 | replit-overtake "Where Apex wins"; unit-economics | Measured $/successful-build beats max-tier baseline; honest comparison surface. |
| **L6 Tech-Debt Sweep** | #7 | 99 in-code markers | Zero untracked debt markers in shipping code. |
| **L7 Premium UI/UX** | #8 | launch-ui-punchlist; theme-parity; UI_POLISH_REPORT | Punchlist "still worth doing" cleared; visual-regression coverage. |
| **L8 Reliability** | #9 | FUTURE FT-002/003; canary-reliability-handoff | Paid full-stack canary matrix consistently green. |
| **L9 Conductor** | #10 | this doc | Backlog driven to zero; loop never idles while work remains. |

Lower-priority replit-overtake items (collaboration, community gallery, DB browser, native mobile, dependencies panel) are **L3/L4/L7 Phase-2 epics** — tracked here, scheduled after the High-priority gaps and reliability close. Anti-goals from `replit-overtake-roadmap.md` are honored (no Replit DB clone, no bounties, no classroom tools).

---

## 3. The Execution Engine

### 3.1 Why a bounded fleet, not "hundreds of cold agents"

Hundreds of cold parallel agents = re-derived context, file collisions, unmergeable diffs, unverifiable output, and maximum spend for minimum quality — the opposite of "best build for the least money." The engine instead uses:

- **One master backlog** (the Task list, IDs #1–#10 + children).
- **Bounded waves** of specialized agents (typically 4–10 in flight), each with: a single scoped objective, worktree isolation when it writes code, a named evidence gate, and a hard "don't touch outside your lane" boundary.
- **Verification before merge.** No lane output is accepted until L0's `verify_all.sh` is green on its branch.
- **Re-triage between waves.** Conductor folds discoveries back into the taxonomy, re-prioritizes, dispatches the next wave.

This is genuinely parallel and genuinely fast — it just isn't chaotic.

### 3.2 Specialized fleet roles

| Role | Charter | Isolation |
| --- | --- | --- |
| **Auditor** | Read-only deep scan of a lane; produces a tiered item list with file:line. | none (read-only) |
| **Implementer** | Executes scoped T0–T3 items in one lane. | worktree |
| **Hardener** | L2-specific: architecture-truth changes with design note. | worktree |
| **Verifier** | Runs L0 gate + lane-specific evidence; pass/fail report only. | none |
| **Conductor** | This loop. Dispatches, re-triages, never idles while work remains. | n/a |

### 3.3 The relentless loop (L9)

```
loop:
  1. TaskList → find lanes with available, unblocked work
  2. For each active lane without an in-flight agent: dispatch the right role
     (Auditor first time, Implementer/Hardener thereafter), worktree-isolated,
     scoped to that lane only, with its evidence gate named in the prompt.
  3. As agents return: run L0 gate on their branch via Verifier.
     - green  → integrate, mark items done, re-triage remainder
     - red    → bounce back to same agent with the failure, do NOT merge
  4. Fold newly discovered issues into the taxonomy (every one gets a Tier+Lane).
  5. Update this doc's Scoreboard + the Task list.
  6. If any work remains anywhere → go to 1. Only stop when backlog == 0.
```

The loop does not ask for permission to continue. It continues until the backlog is empty, then re-audits to confirm empty, then reports done.

---

## 4. "Best build for the least money" — the explicit competitive bar

Apex must measurably win on the axes Spencer named. These are L5/L8 acceptance metrics, tracked on the Scoreboard:

- **Cheapest:** measured median $ per *successful* full-stack build ≤ Replit-equivalent, with per-agent/per-token receipts the competitor cannot show.
- **Smartest:** cost-quality arbitrage — every task routed to the cheapest model clearing its quality bar; escalate only on validated failure, never blindly.
- **Most useful output:** generated app reaches 100% completion with a stable, screenshot-proven preview and honest truth-state — no fake full-stack.
- **Most complex apps with ease:** the paid *max* canary matrix builds a genuinely complex multi-service app, completes, previews, and survives a failed-build restart.
- **Best most of the time:** first-pass success rate tracked as a number, not a claim; stalls near 95% eliminated (FT-002).

Honesty rule (Spencer's standing order): every comparative claim on any public surface must be backed by `cost-assumptions.csv` / `unit-economics.csv` evidence or it does not ship.

---

## 5. Scoreboard (Conductor updates every wave)

| Lane | State | Last evidence | Next action |
| --- | --- | --- | --- |
| L0 | LIVE | `verify_all.sh` GREEN on main 2026-05-16 (build/vet/tsc/test/lint/build) | gate every Wave-2 branch |
| L1 | audited | `docs/lanes/L1-launch-blockers.md` — 6 items, 5 external-gated | BLK-5 checklist authored; rest need creds |
| L2 | audited | `docs/lanes/L2-builder-hardening.md` — 13 items, 0 done, 2 CRITICAL | Wave-2 Hardener: H-01→H-02→H-04 |
| L3 | in progress | L3-01 onboarding mounted, gate GREEN | Wave-2 FE: L3-02/04/06 |
| L4 | not started | — | Wave-3 Auditor |
| L5 | audited | `docs/lanes/L5-cost-smart-routing.md` — 8 items, S-tier money bugs | Wave-2 Hardener: L5-02/01/04 |
| L6 | audited | `docs/lanes/L6-tech-debt.md` — 16 real items, 5 high-risk | Wave-2: P1/P5 panic→error + T0 strings |
| L7 | not started | — | Wave-3 Auditor |
| L8 | audited | `docs/lanes/L8-reliability.md` — 13 items, 3 HIGH-RISK credit-burn | Wave-2 Hardener: L8-HI-003/001/002 |
| L9 | running | Wave-1 (6 auditors) complete; backlog persisted | dispatch Wave 2 (3 agents) |

### Wave log
- **Wave 1 (2026-05-16):** 6 read-only Sonnet Auditors → L1/L2/L3/L5/L6/L8 backlogs persisted to `docs/lanes/`. Cost-disciplined (Sonnet). All returned.
- **Wave 2 (2026-05-16):** 3 worktree implementers, non-overlapping. ALL INTEGRATED, full gate GREEN on main:
  - A: L8-HI-003 (inverted credit-burn guard), L8-HI-001/002 (loop caps 3→1/2), L8-008, L8-002 (solver-bypass), L5-02 (deterministic gates default→true).
  - B: L6-P1 (storage panic→error+log.Fatalf), L6-P5 (ValidateRequiredSecrets), L6-T0-1 (migrate TODO), L6-T1-1 (dockerfile default template).
  - C: L3-06 (GitHub import in top nav), L3-04 (`/import/...` deep-link), L3-02 (Start-blank → IDE, reused existing endpoint).
  - Conductor ACTION REQUIRED for L5-02: set Render backend env `APEX_DETERMINISTIC_TASK_GATES` only via `~/.secrets/render-env-update.sh` if an opt-out is ever needed; default is now safe-on.
- **Wave 3 (2026-05-16):** (D) backend-agents Hardener serialized: L2 H-01/H-02/H-04 + L5-01 KPI + L8-001/003; (E) Auditor: L4 persistence + L7 UI (un-audited lanes); (F) L6 remainder non-agents: system.go 501s, bundler WarmCache, pypi search, backup S3/GCS.

---

## 6. Definition of Done (the only thing that ends the loop)

The program is done — and Apex is launch-ready and best-in-class — when **all** hold simultaneously:

1. Every lane L0–L8 win condition met, each with its named evidence artifact committed.
2. `scripts/verify_all.sh` green on `main`.
3. Zero untracked debt markers in shipping code (L6).
4. Paid full-stack canary matrix (fast / balanced / max) consistently green with stable preview screenshots (L8/L2).
5. Stripe live evidence + production canary closed (L1).
6. Scoreboard shows measured cost/quality bar met vs baseline (L5).
7. A re-audit pass finds no new untiered issues.

A pathway is not done because code was written (per `FUTURE.md` Completion Standard) — only when tests pass, contract edges are covered, user-facing behavior is verified, and residual risk is documented.

---

## 7. Operating Rules (non-negotiable)

- No secrets in repo/docs/logs (matches existing tracker rule).
- Render env vars only via `~/.secrets/render-env-update.sh` — never raw PUT.
- Never fake evidence. If a gate can't be run (external dependency, missing creds), say so explicitly and mark the item blocked — do not claim green.
- Stay in your lane. Cross-lane coupling is escalated to Conductor, not solved by reaching across.
- Re-triage beats heroics. A discovered issue gets a Tier+Lane before it gets a fix.
