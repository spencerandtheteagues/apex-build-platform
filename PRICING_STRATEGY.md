# Pricing Strategy

**Product:** Apex Build
**Date:** 2026-03-21
**Positioning:** Premium / best return per dollar
**Launch budget considered:** Tight-launch posture; pricing must protect runway before growth optimization

---

## Executive summary

The best pricing structure for the current operational flow is:

- a tightly capped free tier with a one-time managed trial bucket
- a clear core paid tier that unlocks managed AI and backend/full-stack generation
- a power tier that unlocks premium modes, longer autonomous flows, and materially higher runtime limits
- a team tier with pooled credits and collaboration entitlements
- simple, non-expiring top-ups starting at `$25`

This structure fits the codebase because Apex already has:

- subscription state
- recurring included credits
- one-time credit top-ups
- BYOK routing logic
- power modes
- project/storage/execution quotas

It does **not** fit a generous free managed-AI tier. The launch contract is intentionally conservative: Free proves the platform with a one-time managed trial bucket, while backend, BYOK, publishing, and high-cost workflows require a paid plan.

## Recommended tier structure

### Free

- Price: `$0`
- Included managed AI: one-time `$5` onboarding trial credits, non-renewing
- After trial: static frontend-only; no backend, BYOK, deploy, or managed-AI continuation
- Limits:
  - `3` active projects
  - `1 GB` storage
  - `20` execution minutes/day
  - `0` recurring managed-AI requests/month
- Allowed:
  - static frontend generation
  - prompt-to-UI exploration inside the trial bucket
  - small ephemeral previews
- Blocked:
  - backend generation
  - BYOK routing
  - long autonomous builds
  - deploy
  - `balanced` / `max`

### Core paid

- Name: `Builder`
- Price: `$24/mo`
- Included recurring credits: `$12/mo`
- Access:
  - managed AI unlocked
  - BYOK supported
  - `fast` + `balanced`
  - backend/full-stack generation unlocked
- Limits:
  - unlimited projects
  - `5 GB` storage
  - `240` execution minutes/day
  - `1` concurrent autonomous build

### Power tier

- Name: `Pro`
- Price: `$59/mo`
- Included recurring credits: `$40/mo`
- Access:
  - `fast` + `balanced` + `max`
  - autonomous long builds
  - deployment unlock
  - higher build concurrency
- Limits:
  - unlimited projects
  - `20 GB` storage
  - `720` execution minutes/day
  - `3` collaborators/project
  - `3` concurrent autonomous builds

### Team

- Name: `Team`
- Price: `$149/mo`
- Included recurring credits: `$110/mo`
- Access:
  - pooled team credits
  - shared workspaces
  - team controls and audit surfaces
  - full power-mode access
- Limits:
  - `5` seats included
  - `100 GB` storage
  - `1440` execution minutes/day shared
  - `10` concurrent autonomous builds

### Overage credits

- Top-ups:
  - `$25`, `$50`, `$100`, `$250`
- Credit value:
  - 1:1 face value for trust and simplicity
- Policy:
  - no `$10` pack
  - credits do not expire while the account remains active
  - optional auto-recharge for paid tiers

## Why this structure wins

- It matches the existing architecture instead of requiring a new billing philosophy.
- It makes Free useful without letting Free create unbounded cost.
- It keeps Builder low enough to convert, while still leaving margin room.
- It reserves the best autonomous and max-mode experience for the tier that can pay for it.
- It uses simple overage mechanics instead of clever metering that customers will resent.

## Margin targets

| Tier / mode | Estimated revenue | Estimated variable cost | Contribution margin | Notes |
|---|---:|---:|---:|---|
| Free | $0 | cap at <= $1.50 / active free user / month | intentionally negative, tightly capped | only acceptable with one-time trial credits and no BYOK/backend access |
| Core paid | $24 | $7-$9 / active Builder user / month | 62%-71% | assumes moderate managed AI use and light runtime usage |
| Power tier | $59 | $18-$22 / active Pro user / month | 63%-69% | supports premium models and longer autonomous sessions |
| Team | $149 | $45-$55 / active Team workspace / month | 63%-70% | pooled-credit model with heavier runtime/collaboration cost |
| Overage | $1.00 credit sold | target <= $0.60 fully loaded cost | >=40% | includes AI cost plus processor/infrastructure buffer |

## Customer value narrative

- Free: prove the product works, not replace a paid plan.
- Builder: the default paid plan for solo builders who want reliable managed AI without surprise spend.
- Pro: for people shipping constantly who want the best models and long-running agent workflows.
- Team: for shared delivery, pooled credits, and collaboration controls.

The pricing message should be:

- subscription unlocks the platform
- credits pay for managed AI consumption
- top-ups are explicit, optional, and easy to understand
- BYOK exists for control, not as a hidden billing trap

## Guardrails required for safety

1. Free cannot retain backend-generation access after the trial bucket is gone.
2. BYOK is paid-plan-only for launch; do not advertise free BYOK.
3. Every credit mutation must go through the ledger.
4. One plan-definition schema must drive UI, billing APIs, and enforcement.
5. Effective access must depend on billing status as well as subscription type.

## Risks and assumptions

- Measured:
  - code-level token pricing logic
  - included-credit structure in repo
  - current Stripe flow and entitlement wiring
- Estimated:
  - non-token infra cost
  - support/refund burden
  - per-user monthly runtime cost
  - best conversion point by tier

The largest operating assumption is BYOK economics. For launch, BYOK is part of paid-plan value and must still surface any Apex routing/platform fee separately from the user's provider bill.

## Recommended next actions

1. Change the pricing contract to the tier structure above.
2. Keep the paid-plan-only BYOK policy reflected in backend entitlements, UI copy, and docs.
3. Replace the current `19/49/99` and `10/35/80` structure with `24/59/149` and `12/40/110`.
4. Remove the `$10` top-up pack.
5. Build a subscription-change endpoint and stop using Checkout for upgrades.
