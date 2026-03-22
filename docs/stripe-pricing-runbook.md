# Stripe Pricing Runbook

This repo expects Stripe subscription prices to match the current APEX.BUILD pricing ladder:

- `Builder Monthly` = `$24.00`
- `Builder Annual` = `$230.40`
- `Pro Monthly` = `$59.00`
- `Pro Annual` = `$566.40`
- `Team Monthly` = `$149.00`
- `Team Annual` = `$1430.40`
- `Enterprise` = custom / sales-managed

## Stripe Dashboard Setup

Create or update recurring Stripe prices for these products:

- `APEX.BUILD Builder`
  - monthly recurring price: `$24.00`
  - annual recurring price: `$230.40`
- `APEX.BUILD Pro`
  - monthly recurring price: `$59.00`
  - annual recurring price: `$566.40`
- `APEX.BUILD Team`
  - monthly recurring price: `$149.00`
  - annual recurring price: `$1430.40`
- `APEX.BUILD Enterprise`
  - create only if you use Stripe-managed enterprise subscriptions

Recommended metadata per Stripe product/price:

- `plan_type=builder|pro|team|enterprise`
- `billing_cycle=monthly|annual`

## Environment Variables

Set these backend environment variables to the new Stripe price IDs:

- `STRIPE_PRICE_BUILDER_MONTHLY`
- `STRIPE_PRICE_BUILDER_ANNUAL`
- `STRIPE_PRICE_PRO_MONTHLY`
- `STRIPE_PRICE_PRO_ANNUAL`
- `STRIPE_PRICE_TEAM_MONTHLY`
- `STRIPE_PRICE_TEAM_ANNUAL`
- `STRIPE_PRICE_ENTERPRISE_MONTHLY`
- `STRIPE_PRICE_ENTERPRISE_ANNUAL`

Render blueprint support for those vars is defined in:

- [render.yaml](/Users/spencerteague/apex-build/render.yaml)

Example local placement is documented in:

- [.env.example](/Users/spencerteague/apex-build/.env.example)

## Product Contract Notes

- Free does not unlock backend or full-stack generation.
- Credits are a usage layer, not a capability unlock.
- Credit top-ups are handled as dynamic one-time checkout amounts in code, not Stripe price IDs.
- Current supported top-up amounts are `$25`, `$50`, `$100`, and `$250`.

## Verification Checklist

1. Update the Stripe recurring prices in the dashboard.
2. Copy the new price IDs into Render and any local/staging env files.
3. Redeploy the backend.
4. Open Billing in the app and confirm Builder/Pro/Team checkout buttons redirect successfully.
5. Confirm the returned subscription plan matches the selected Stripe price.
6. Confirm credit top-up checkout still works for `$25`, `$50`, `$100`, and `$250`.

