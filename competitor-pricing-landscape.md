# Competitor Pricing Landscape

Snapshot date: 2026-03-21

## Official Pricing Snapshots

- Lovable: `https://lovable.dev/pricing`
  - Free tier with daily credits
  - Pro at `$25/mo`
  - Business at `$50/mo`
  - Uses credits plus usage-based cloud/AI
- Bolt.new: `https://bolt.new/pricing`
  - Free tier with daily and monthly token caps
  - Pro at `$25/mo`
  - Teams at `$30/user/mo`
  - Explicitly directs subscribers to Stripe billing portal for plan changes
- v0: `https://v0.app/pricing`
  - Free includes `$5` monthly credits
  - Premium at `$20/mo` with `$20` monthly credits
  - Team at `$30/user/mo`
  - Additional credits sold separately
- Replit: `https://replit.com/pricing`
  - Free starter tier
  - Core around `$20/mo`
  - Higher plans position more agent power and materially larger usage budgets
  - Product messaging emphasizes spend controls and usage-based billing

## Market Pattern

- The category has broadly moved to:
  - subscription + included usage credits
  - explicit overage / top-up mechanics
  - customer-visible usage tracking
  - a meaningful free tier, but capped
- Competitors increasingly separate:
  - plan price
  - included credits
  - pay-as-you-go / top-up usage
- The strongest trust pattern is:
  - simple plan ladder
  - portal-based subscription management
  - explicit credit balances
  - no hidden backend-only limits that disagree with the billing UI

## Implications For Apex

- Apex is directionally aligned on subscription + credits.
- Apex is behind on the operational pieces that make this model trustworthy:
  - clean entitlement resolution
  - durable ledgering
  - true plan-change workflow
  - consistent limit messaging

## Competitive Risk

- If billing feels inconsistent, users will compare that confusion against competitors with simpler portal-based subscription management and clearer included-usage contracts.
- The fastest path to competitive trust is not cheaper pricing; it is cleaner billing state and clearer limits.
- The biggest avoidable trap is making BYOK or free usage feel deceptively free and then failing requests on hidden platform-credit rules.
