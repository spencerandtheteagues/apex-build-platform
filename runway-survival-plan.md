# Runway Survival Plan

## 30 / 60 / 90 Day Monetization Posture

### First 30 days

- Launch with:
  - Free = one-time trial + strict BYOK caps
  - Builder = `$24`
  - Pro = `$59`
  - Team = `$149`
- Priorities:
  - verify webhook durability
  - fix ledgering
  - unify limits
  - watch free-to-paid conversion and top-up frequency

### First 60 days

- If free abuse is low and conversion is healthy:
  - consider small increases in Builder credits or free BYOK quota
- If cost spikes appear:
  - reduce free managed trial
  - shorten preview/runtime lifetime
  - tighten failed-build retry rules

### First 90 days

- Decide whether BYOK should be:
  - a free low-cost funnel, or
  - a metered orchestration product
- Adjust top-up behavior:
  - add auto-recharge if customers frequently hit zero mid-build
  - only expand pack variety after balance/usage trust is solid

## Minimum Healthy Thresholds

- Free-to-paid conversion:
  - target `>= 4%` of activated free accounts
- Paid-to-top-up attachment:
  - target `>= 20%` of active paid users monthly
- Sustainable ARPU:
  - blended target `>= $38`
- Danger zone:
  - if blended variable cost exceeds `55%` of revenue for 2 consecutive weeks, tighten free limits immediately

## Low / Medium / High Adoption Burn

- Low adoption:
  - burn risk comes from a too-generous free tier and weak conversion
- Medium adoption:
  - burn risk comes from retry-heavy autonomous flows and generous included credits
- High adoption:
  - burn risk comes from any unresolved entitlement drift, webhook loss, or unlimited-like Team behavior

## Signals The Pricing Is Too Generous

- frequent zero-margin or near-zero-margin overage usage
- many free users hitting autonomous/build-heavy paths
- rising support tickets around invisible quota mismatches
- Team workspaces consuming credits far above plan assumptions without top-up conversion

## Signals The Pricing Is Too Restrictive

- strong activation but weak first payment conversion
- frequent Builder users exhausting credits almost immediately
- BYOK users never converting because paid value feels too close to BYOK-only behavior

## Do Not Do Yet

- Do not widen free managed AI
- Do not increase Team included credits
- Do not market "unlimited" anything except negotiated enterprise contracts

## Operator Rule

- If billing state and entitlement state ever disagree, entitlement must fail closed in favor of protecting runway, then surface a support path.
