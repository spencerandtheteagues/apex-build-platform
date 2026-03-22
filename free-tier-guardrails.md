# Free Tier Guardrails

## Allowed actions

- Account creation and onboarding
- Small static frontend generation only
- BYOK-assisted chat/completion in `fast` mode only
- Ephemeral preview with strict duration cap
- Manual editor flows

## Blocked or paywalled actions

- Managed AI after the one-time trial bucket is exhausted
- Backend generation
- Long autonomous builds
- `balanced` or `max` power mode
- Deployments and persistent runtimes
- Database provisioning
- Team collaboration and high-frequency execution loops

## Rate limits and caps

- One-time managed trial credits: `$5`, non-renewing
- Active projects: `2`
- Storage: `500 MB`
- Execution minutes: `20/day`
- Managed AI build attempts: `3/day` during trial only
- BYOK requests after trial: `150/month`
- Queue priority: lowest

## Mode restrictions

- Free:
  - `fast` only
  - no autonomous retries beyond one repair pass
  - no premium provider routing
- Builder:
  - `fast` + `balanced`
- Pro and above:
  - full power-mode access

## Cost spike protections

- Reserve credits before every managed AI call
- Cap free retry loops at one automatic retry
- Kill switch if a free user exceeds:
  - 3 failed builds in 30 minutes
  - 5 execution jobs in 10 minutes
  - repeated prompt patterns indicating scripted abuse

## Abuse monitoring signals

- High failed-build rate per user
- Many new accounts from the same IP/device fingerprint
- Repeated zero-value prompts with high token usage
- Large bursts of execution traffic with minimal project edits
- BYOK accounts attempting autonomous or backend-heavy flows repeatedly

## Emergency kill switches

- Disable free managed AI globally
- Disable free BYOK autonomy globally
- Reduce free build concurrency to zero
- Force free users to manual review queue only
- Disable expensive providers / power modes by environment flag
