# Tests Agent Contract

## Purpose

`tests/` owns repo-level verification harnesses, Playwright E2E coverage, generated app specs, SLO thresholds, and launch/preview smoke tests. Tests are the evidence layer for Apex Build reliability claims.

## Documentation Hierarchy

This file is the level 1 tests contract. Add child docs for stable sub-harnesses when needed, especially under `tests/e2e/` if app spec generation, launch smoke, and preview verification diverge.

## Owned Files And Surfaces

- `tests/backend/` and `tests/comprehensive_test_suite.go`: repo-level Go test surfaces outside backend packages.
- `tests/e2e/package.json`, `playwright.config.ts`, `tsconfig.json`: Playwright harness configuration.
- `tests/e2e/specs/launch.prod.smoke.spec.ts`: production launch smoke expectations.
- `tests/e2e/specs/preview-verification.spec.ts`: generated preview proof and runtime verification.
- `tests/e2e/apps/*.json`: generated app fixtures and benchmark specs.
- `tests/e2e/scripts/`: app spec generation and result summaries.
- `tests/e2e/slo-thresholds.json`: performance and reliability thresholds.

## Stable Contracts

- Tests must assert real product behavior, not implementation trivia.
- Launch tests should fail when production readiness is missing, unless the test is explicitly documented as a local-only smoke.
- Preview tests must reject placeholder-only or skeleton-only generated apps.
- E2E fixtures should be realistic enough to catch underbuilt app output.
- Keep test credentials, tokens, cookies, provider keys, and customer data out of the repo.
- If a test is skipped due to missing external config, the skip reason must be explicit and launch docs must still record the evidence gap when launch-relevant.

## Development Guidance

- Add focused unit tests near frontend/backend code for local behavior, and use Playwright for cross-service user flows.
- Prefer deterministic waits on app state, network responses, or visible UI over fixed sleeps.
- Keep generated app specs versioned and intentional; do not overwrite fixtures casually.
- When fixing a regression, add a test that would have caught it unless the cost is disproportionate and documented.

## Verification

Common gates:

```bash
cd tests/e2e
npm ci
npm run test:launch -- --project=chromium
npm run test:preview-verify -- --project=chromium
```

Also run backend and frontend package tests when changes affect those domains.

## Documentation Updates

Update this file when harness structure, launch expectations, preview verification rules, SLO thresholds, fixture policy, or external config requirements change.
