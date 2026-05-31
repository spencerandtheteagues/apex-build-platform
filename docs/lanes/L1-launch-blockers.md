# Lane L1 — Launch Blockers (audit 2026-05-16)

Source: Wave-1 Auditor. Anchors verified against code at audit time.

| ID | Tier | In-repo? | Item | Evidence to close |
| --- | --- | --- | --- | --- |
| BLK-5 | T1 | YES | Rollback drill + incident checklist artifact | Dated sign-off row in `docs/launch-runbook.md` |
| BLK-3 | T0 | Partial | Set `APEX_ENABLE_GITHUB_ACTIONS=true` repo var + ensure CI secrets | Canary workflow green; `Launch Verification Scripts` job ok |
| BLK-1 | T1 | NO (Stripe creds) | Real Stripe webhook replay, 6 event types vs live endpoint | Stripe dashboard 200s + duplicate-skip log line |
| BLK-2 | T1 | NO (Stripe creds) | Controlled paid checkout → portal → up/downgrade → cancel | `verify_stripe_launch.mjs` exits 0; subscription active |
| BLK-4 | T1 | NO (canary acct) | Paid balanced+max canary + export/deploy + failed-build restart | Canary paid-balanced/paid-max green |
| BLK-6 | T3-T4 | NO (EAS/Apple/Google) | Real mobile project native build + store submission evidence | `verify_mobile_external_readiness.mjs` exits 0 strict |

**Conductor note:** BLK-1/2/4/6 are external-credential-gated — cannot be closed in-repo and must NOT be marked green without real evidence (Operating Rule: never fake evidence). BLK-5 checklist authored 2026-05-16 (live dry-run sign-off still required). BLK-3 flag flip needs GitHub repo-variable + Actions-secret access (Spencer-side or credentialed).
