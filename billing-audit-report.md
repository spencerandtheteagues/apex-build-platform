# Billing Audit Report

Date: 2026-03-21
Scope: `backend/internal/payments`, `backend/internal/handlers/payments.go`, `backend/internal/usage`, `backend/internal/ai/byok.go`, related frontend billing surfaces.

## Executive Summary

The Stripe integration is real and substantially farther along than the old stub billing code, but the monetization system still has four material risks:

1. The advertised free/BYOK path is internally contradictory and can still require platform credits.
2. Credit debits are not written to the immutable credit ledger.
3. Plan limits are defined in more than one place and already disagree.
4. Webhook handling acknowledges Stripe before downstream credit/subscription writes are known to be durable.
5. Existing paid users are pushed into new Checkout sessions for plan changes instead of a true subscription-update path.

## Findings

### Critical

#### 1. The repo advertises free BYOK usage, but BYOK requests still reserve platform credits

- Free is described as "Evaluate the platform with your own API keys" in `backend/internal/payments/plans.go:105`.
- The build preflight intentionally exempts BYOK users from the zero-credit hard stop in `backend/internal/agents/handlers.go:289`.
- Actual AI request paths still call `ReserveCredits(...)` even when `isBYOK == true` in:
  - `backend/internal/api/handlers.go:451`
  - `backend/internal/completions/completions.go:299`
  - `backend/internal/agents/ai_adapter.go:251`
  - `backend/internal/agents/autonomous/ai_adapter.go:106`
- `backend/internal/ai/byok.go:470` then deducts `users.credit_balance` without any `isBYOK` exemption.

Impact:

- A zero-balance user with valid personal API keys can clear preflight and still fail at reservation time.
- The free/BYOK promise is not actually implementable as written.
- Pricing design cannot safely rely on the current "free BYOK" story until this is fixed.

Recommended fix:

- Decide explicitly between:
  - true BYOK-free operation for free-tier low-cost flows, or
  - paid BYOK routing everywhere.
- Then enforce that choice centrally in one entitlement/credit service.

#### 2. Credit balance mutations from AI usage are not reconciled through `CreditLedgerEntry`

- `backend/pkg/models/models.go:571` says the ledger is the append-only audit trail for every credit change.
- `backend/internal/handlers/payments.go:211` writes ledger entries for credit grants from Stripe events.
- `backend/internal/ai/byok.go:470` and `backend/internal/ai/byok.go:500` reserve and settle AI spend by updating `users.credit_balance` directly.

Impact:

- The visible balance can change without a matching immutable debit record.
- Refunds, disputes, support adjustments, and balance forensics cannot be reconstructed reliably.
- `GET /billing/credits/ledger` is incomplete by design today, even though the API contract implies otherwise.

Recommended fix:

- Move all credit debits, refunds, and holds into one transactional ledger service.
- Treat `users.credit_balance` as a cached projection of the ledger, not the source of truth.
- Introduce explicit entry types such as `spend_reservation`, `spend_settlement`, and `spend_refund`.

#### 3. Stripe webhooks return `200` before downstream mutations are guaranteed durable

- `backend/internal/handlers/payments.go:166` parses the event and dispatches handler methods.
- `backend/internal/handlers/payments.go:204` always returns `200`.
- `backend/internal/handlers/payments.go:368`, `backend/internal/handlers/payments.go:468`, and similar paths only log downstream transaction failures.

Impact:

- A transient DB error during `invoice.paid` or credit purchase processing can permanently drop a monthly credit allocation or top-up.
- Stripe will not retry because the server already acknowledged success.
- Customer balances and subscription state can silently drift from Stripe.

Recommended fix:

- Make credit- and subscription-mutating webhook handlers return errors.
- Only return `200` after durable success or a verified idempotent duplicate.
- Send failed events to a retry queue / dead-letter table for operator replay.

### High

#### 4. Plan and quota definitions are duplicated and inconsistent

- Payment plan definitions live in `backend/internal/payments/plans.go:100`.
- Quota-enforcement plan definitions live in `backend/internal/usage/tracker.go:50`.
- Examples of current drift:
  - Free storage is `1 GB` in `backend/internal/payments/plans.go:120` but `100 MB` in `backend/internal/usage/tracker.go:56`.
  - Team storage is `50 GB` in `backend/internal/payments/plans.go:233` but `100 GB` in `backend/internal/usage/tracker.go:77`.
  - Payment plans talk about `CodeExecutionsPerDay` in `backend/internal/payments/plans.go:58`, while enforcement uses execution minutes in `backend/internal/usage/tracker.go:47`.
- `backend/internal/handlers/payments.go:674` and `backend/internal/handlers/payments.go:1070` use payment-plan limits for billing endpoints.
- `backend/internal/middleware/quota.go:147` and `backend/internal/usage/tracker.go:413` use usage-tracker limits for enforcement.

Impact:

- The product can promise one entitlement and enforce another.
- Billing/settings screens can show limits that do not match the gate users actually hit.
- Support, refunds, and trust costs go up because there is no single contract.

Recommended fix:

- Collapse to one source of truth for plan shape, quotas, features, and included credits.
- Make billing APIs, frontend pricing UI, and middleware all read from that same schema.

#### 5. Paid-plan changes are routed through new Checkout, not subscription mutation

- The frontend exposes an `Upgrade` action in `frontend/src/components/billing/BillingSettings.tsx:419`.
- That button calls `createCheckoutSession` in `frontend/src/components/billing/BillingSettings.tsx:227`.
- The backend route only creates a new subscription checkout in `backend/internal/handlers/payments.go:53`.
- A real update path exists in `backend/internal/payments/stripe.go:520`, but no handler or route uses it.

Impact:

- Upgrades/downgrades are not modeled as plan changes.
- Existing subscribers may be pushed into a second checkout flow instead of a clean Stripe subscription update with proration.
- Customer billing state can become confusing or duplicated.

Recommended fix:

- Expose a dedicated `POST /billing/subscription/change` endpoint backed by `StripeService.UpdateSubscription`.
- In the UI, show `Change plan` for subscribers and reserve Checkout for net-new subscriptions only.

### Medium

#### 6. `past_due` users retain paid-plan entitlements because enforcement keys off plan, not status

- Failed invoices only set `subscription_status = past_due` in `backend/internal/handlers/payments.go:476`.
- Quota middleware reads `subscription_type` from auth context in `backend/internal/middleware/quota.go:249`.
- Auth middleware injects `subscription_type` into request context in `backend/internal/middleware/auth.go:57`.

Impact:

- A user can remain on paid quotas/features while their billing state is delinquent.
- The system avoids credit re-allocation on failed invoice, but non-credit entitlements can still stay elevated.

Recommended fix:

- Introduce an entitlement resolver that derives effective access from both plan and status.
- Define explicit behavior for `trialing`, `past_due`, `unpaid`, and `canceled`.

#### 7. Billing usage endpoints do not describe the same thing the quota middleware enforces

- `backend/internal/handlers/payments.go:701` reports execution usage as a count of execution records.
- The quota system enforces execution minutes, not execution count, in `backend/internal/usage/tracker.go:47` and `backend/internal/usage/tracker.go:589`.
- `backend/internal/handlers/payments.go:679` also computes usage ad hoc instead of using the same tracker object that middleware uses.

Impact:

- Billing UI can tell users they are within limits while the middleware blocks them, or vice versa.
- Support and debugging become much harder because there is no canonical usage number.

Recommended fix:

- Delete the ad hoc usage math in payment handlers.
- Read usage exclusively from the usage tracker / shared entitlement service.

### Low

#### 8. Legacy billing implementation remains in the repo and is materially misleading

- `backend/internal/handlers/billing.go:33` through `backend/internal/handlers/billing.go:249` contains placeholder/manual billing logic and fake subscription/invoice data.
- It does not appear to be mounted in `backend/cmd/main.go`, but it is still available as live-looking code.

Impact:

- High maintenance risk.
- Easy to accidentally rewire or cargo-cult from the wrong implementation.

Recommended fix:

- Delete it, or move it behind an explicit deprecated/internal-only package.

## Priority Order

1. Resolve the BYOK/free-tier contract.
2. Unify credit debits and ledgering.
3. Make webhook success depend on durable writes.
4. Collapse plan definitions to one source of truth.
5. Add a true subscription-change endpoint and stop using Checkout for upgrades.
6. Gate effective entitlements on billing status.
