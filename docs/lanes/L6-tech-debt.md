# Lane L6 — Tech-Debt Sweep (audit 2026-05-16)

Source: Wave-1 Auditor. ~99 grep hits → **16 real items** (rest are false positives inside AI prompt strings / validator regex / doc comments).

## High-risk shortlist (fix first)

| Pri | File:line | Risk |
| --- | --- | --- |
| P1 | storage/provider.go:37 | `panic()` at startup if `./uploads` uncreatable → Render restart loop, no diagnostic |
| P2 | agents/autonomous/executor.go:616-631 | `executeDeploy`/`executeRollback` silent no-op → task "succeeds", nothing happens |
| P3 | handlers/system.go:110-134 | `GET /execute/:id`, `POST /execute/:id/stop` return live 501 |
| P4 | packages/pypi.go:122-155 | PyPI search silently returns empty for Python projects (live endpoint) |
| P5 | config/secrets.go:774-781 | `MustGetSecret` panics on missing env; public+unused → landmine for next caller |

## By tier

- **T0 (3):** migrate/main.go:361 TODO in SQL template; bundler/cache.go:345 noisy no-op log; stub-type comments (advanced_scanner.go:769, optimizer.go:542)
- **T1 (5):** export_templates/dockerfile.go:61 TODO shipped to user repos; system.go 501 endpoints; billing/enterprise.go:537 stub types; security/enterprise_auth.go:441 dead stubs; manager_optimized.go:740 empty parse → no files if live
- **T2 (4):** bundler WarmCache no-op (cold-compile latency); autonomous deploy/rollback no-op; backup S3/GCS not implemented; pypi search
- **T3 (3):** secrets.go MustGetSecret → startup validation pass; storage/provider.go panic → error+log.Fatalf; Redis spend caching (from memory, still missing)
- **T4 (1):** enterprise_auth MFA + risk analysis (full impl per subsystem; keep errNotImplemented defensively until replaced)

**Conductor note:** P1 + P5 are the cheapest highest-safety wins (panic→error). All backend — execute after L2/L8 auditors finish to preserve their anchors. T0 SQL/Dockerfile TODO-string fixes are safe-anytime (not in agents files).
