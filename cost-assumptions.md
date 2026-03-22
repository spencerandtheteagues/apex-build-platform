# Cost Assumptions

Date: 2026-03-21

## Verified From Code

- Platform AI billing formula:
  - `billed_cost = max(raw_cost * profit_margin * power_surcharge, raw_cost)`
  - Source: `backend/internal/pricing/pricing.go`
- Default markup:
  - `profit_margin = 1.50`
- Default power surcharges:
  - `fast = 1.00`
  - `balanced = 1.12`
  - `max = 1.25`
- BYOK routing fee:
  - `$0.25 / 1M` tokens

## Important Unmodeled Costs

- Stripe processing fees
- Refunds / disputes
- Sandbox/container compute
- preview / hosting runtime
- storage and egress
- failed build retries
- support burden

## Interpretation

- The current pricing engine protects against selling raw model access below provider token cost.
- It does not prove full contribution-margin positivity at the product level.
- Credit packs sold at 1:1 dollar-to-credit face value leave no explicit buffer for payment processing and non-token variable cost.
- The current codebase also does not yet implement a coherent answer to whether BYOK should be free, metered, or subscription-bundled.

## Assumption Policy

- Treat current token pricing as a lower-bound cost model, not a full unit-economics model.
- Do not increase free-tier generosity until runtime, storage, and retry costs are measured.
- Do not widen included monthly credits until processor fees and non-token infra are incorporated into the margin model.
- Do not market "free BYOK" until the reserve path matches that promise.
