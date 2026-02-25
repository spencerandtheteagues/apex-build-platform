# tests/e2e

Root Playwright infrastructure for APEX.BUILD Phase 4.

## What this adds
- App-manifest driven smoke spec generation (`apps/*.json` -> `specs/generated/*.spec.ts`)
- CI-friendly root Playwright config independent of frontend-local E2E setup
- A place to add cross-app production verification journeys

## Usage
- `cd tests/e2e`
- `npm install`
- `npm run generate`
- `npm test`

Environment overrides:
- `PLAYWRIGHT_BASE_URL` (default `http://localhost:5173`)
- `PLAYWRIGHT_API_URL` (default `http://localhost:8080`)
