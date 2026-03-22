# Monetization Map

Date: 2026-03-21

## Revenue Capture

- Recurring subscriptions:
  - `builder`: `$19/mo`, includes `$10` credits
  - `pro`: `$49/mo`, includes `$35` credits
  - `team`: `$99/mo`, includes `$80` credits
  - Source: `backend/internal/payments/plans.go`
- One-time credit packs:
  - `$10`, `$25`, `$50`, `$100`
  - Source: `backend/internal/payments/stripe.go`
- BYOK routing fee:
  - `$0.25 / 1M` tokens
  - Source: `backend/internal/pricing/pricing.go`

## Cost-Creating Actions

- AI generation via `/api/v1/ai/generate`
- Code completion via `backend/internal/completions/completions.go`
- Build-agent generation via `backend/internal/agents/ai_adapter.go`
- Autonomous-agent generation via `backend/internal/agents/autonomous/ai_adapter.go`
- Execution sandbox usage via `/api/v1/execute`
- Ongoing storage and hosted runtime / preview surfaces

## Current Enforcement Surfaces

- Credit reservation and settlement:
  - `backend/internal/ai/byok.go`
- Spend tracking:
  - `backend/internal/spend/tracker.go`
- Budget caps:
  - `backend/internal/budget/enforcer.go`
- Quota middleware:
  - `backend/internal/middleware/quota.go`
- Subscription / Stripe state:
  - `backend/internal/handlers/payments.go`
  - `backend/internal/payments/stripe.go`

## Structural Problems

- Credits, spend tracking, and quotas are three adjacent systems, not one integrated billing core.
- Credit balance is mutable outside the credit ledger.
- Plan limits are duplicated in `payments` and `usage`.
- Billing usage APIs and quota middleware do not share a single entitlement resolver.

## Long-Term Shape

- One entitlement model:
  - plan
  - status
  - included credits
  - purchased credits
  - BYOK eligibility
  - execution/storage/project quotas
- One monetary ledger:
  - subscription allocations
  - purchases
  - holds
  - settlements
  - refunds
  - admin adjustments
- One usage source:
  - tracker-backed numbers reused by UI and enforcement
