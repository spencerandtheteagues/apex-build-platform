# Backend Agent Contract

## Purpose

`backend/` owns the Go infrastructure and product-critical runtime for Apex Build: API routing, authentication, billing, credits, build orchestration, AI provider routing, preview/execution, deployment, hosting, persistence, startup readiness, metrics, and WebSockets.

Backend changes are high leverage and high risk. A backend success state must mean the underlying operation actually succeeded; never paper over orchestration, billing, provider, sandbox, preview, or persistence failures with optimistic responses.

## Documentation Hierarchy

This file is the level 1 backend contract. Add deeper `AGENTS.md` files when a backend subsystem gains enough stable rules that this doc would become too broad. Likely future child docs include:

- `backend/internal/agents/AGENTS.md` for build orchestration and repair loops.
- `backend/internal/ai/AGENTS.md` for provider routing, model policy, timeouts, and fallback behavior.
- `backend/internal/preview/AGENTS.md` for preview startup, verification, placeholder gates, and screenshots.
- `backend/internal/payments/AGENTS.md` for Stripe, plans, credits, idempotency, and billing safety.
- `backend/internal/execution/AGENTS.md` for sandbox and command execution boundaries.
- `backend/internal/handlers/AGENTS.md` for HTTP handler conventions and API surface ownership.

When child docs exist, list them here and update parent/child docs together when a contract crosses scopes.

## Owned Files And Surfaces

- `cmd/main.go`: process entrypoint, dependency wiring, route registration, startup checks, and service initialization.
- `internal/agents/`: autonomous build orchestration, task flow, progress, repair, and completion semantics.
- `internal/ai/`: provider clients, routing, model selection, cost/timeout policy, and fallback behavior.
- `internal/handlers/`: REST handlers for projects, files, builds, previews, deploys, packages, billing, spend, search, and related API surfaces.
- `internal/preview/` and `internal/execution/`: preview runtime startup, command execution, runtime verification, sandbox boundaries, and generated app proof.
- `internal/payments/`, `internal/billing/`, `internal/pricing/`, `internal/spend/`, `internal/budget/`, `internal/usage/`: paid plans, credits, Stripe, cost tracking, limits, and billing UX data.
- `internal/auth/`, `internal/middleware/`, `internal/secrets/`, `internal/security/`: identity, authorization, secret handling, and request protection.
- `internal/database/`, `internal/db/`, `migrations/`, `pkg/models/`: persistence, schema evolution, models, and data access.
- `internal/websocket/`, `internal/collaboration/`, `internal/terminal/`, `internal/debugging/`: real-time channels and long-running interactive sessions.
- `api/openapi.yaml`: machine-readable API contract.

## Stable Contracts

- Use `gofmt` and idiomatic Go package names. Keep package names short, lowercase, and underscore-free.
- API behavior must match `backend/api/openapi.yaml`, `API_CONTRACT.md`, and `docs/api.md` when externally visible.
- Build progress, WebSocket envelopes, REST status responses, and database state must agree. Do not emit a terminal success event before persistence and preview verification complete.
- Long-running AI/provider tasks need bounded deadlines, cancellation, and clear error classification. Hangs are launch blockers.
- Structured planner provider attempts must reject responses that complete after the configured attempt deadline, even if the Go timeout timer is delayed under host contention. Late plans are timeout failures and should rotate to the next eligible provider instead of being accepted as successful.
- AI provider clients must respect request contexts. Hard-timeout wrappers can stop the orchestration wait path, but they cannot safely kill provider code that ignores cancellation after it has been called.
- Provider fallback must preserve build quality. Cheap fallback is unacceptable if it causes underbuilt apps, placeholder output, or silent failure.
- BYOK routing must preserve the user's provider boundary. When a paid/owner user starts a build in BYOK mode with an active key, do not mix in platform Claude/GPT/Gemini/Grok/DeepSeek/GLM/Ollama providers unless an explicit emergency fallback policy is enabled and documented.
- Ollama Cloud reasoning must not consume the visible file-output budget for code-producing tasks. Keep reasoning enabled for planning/review where it improves decisions, but default code generation, completion, refactoring, and testing calls to visible output so BYOK/Ollama builds do not stall on reasoning-only truncation.
- Paid canary builds and paid testing must use the `APEX_AI_TESTING_PROFILE=openrouter-free` / `APEX_LIVE_TEST_MODEL_PROFILE=openrouter-free` profile unless a human explicitly approves paid flagship-provider spend. That profile keeps `provider_mode=platform`, assigns build roles to OpenRouter, and pins `provider_model_overrides.openrouter` to a `:free` OpenRouter model. This is test evidence for paid/full-stack routing and build behavior, not evidence that paid flagship provider accounts have credits or production health.
- Billing mutations must be idempotent and auditable. Stripe webhooks, checkout completion, credits, plan changes, and invoice events require replay-safe tests.
- Secrets and provider keys must never be logged. Production readiness verifiers may report presence, status, and issue codes, not secret values.
- Database migrations must be additive or safely migrated for production. Keep model structs, migrations, and tests aligned.
- Preview verification must reject placeholder-only output and must not let generated app failures pass as completed builds.
- Preview and backend runtime HTTP probes must confirm a stable response from the spawned process before passing. A ready-looking log line followed by process exit is a generated-app/runtime failure unless the error is an explicit host condition such as bind/port allocation failure.
- Vite runtime verification must prove readiness from the spawned Vite process by matching the exact expected localhost URL/port in that process's logs before running HTTP, browser, vision, or canary checks. A foreign listener on the released reserved port is not proof of generated-app readiness.
- Placeholder-preview heuristics in `internal/preview/` must stay behaviorally aligned with live canary checks in `scripts/run_live_golden_build.mjs`; update both runtimes and their tests together.
- Startup-smoked Chrome/Chromium readiness must be the browser path used by runtime preview verification and interaction canaries. Do not rediscover a different Chrome binary after startup health has rejected one.
- Startup health must distinguish hard launch blockers from optional degraded subsystems.

## Development Guidance

- Prefer small focused changes over broad rewrites in `cmd/main.go` or large orchestration files.
- For new handlers, keep parsing, authorization, service calls, and response shaping explicit. Avoid hidden side effects in helpers.
- For shared interfaces, prefer typed Go interfaces or concrete services with narrow methods over runtime feature probing.
- Keep filesystem, Docker, E2B, provider, and deploy side effects behind testable service boundaries.
- Do not add provider-specific policy in random handlers; keep model/provider routing in the AI/orchestration layer.
- Do not duplicate pricing literals in handlers. Derive plan and credit behavior from the payments/pricing contracts.
- For WebSocket changes, update frontend consumers and tests in the same session.
- Tests that spawn local Node, Python, browser, or HTTP verifier processes should avoid unnecessary parallelism and ultra-tight timeouts. A verifier-host timeout may be documented as an environment skip in production code, but backend tests must not let that skip mask the product failure they are asserting.
- Agent activity heartbeats must emit an initial progress event synchronously when a task enters a long-running stage, then continue on the configured interval. Do not make the first observable progress update depend on a ticker goroutine being scheduled.

## Verification

For backend-only changes, run the focused package test first, then broaden when the code touches shared runtime or launch-critical behavior:

```bash
cd backend
go test ./path/to/package -run TestName -count=1
go test -p 1 -parallel 4 ./... -timeout 20m
go build ./...
```

For API, WebSocket, preview, billing, provider routing, or build lifecycle changes, also consider the relevant scripts under `scripts/` and Playwright specs under `tests/e2e/`.

## Documentation Updates

Update this file when backend architecture, route ownership, build lifecycle, provider policy, billing semantics, preview verification, startup readiness, or verification commands change. Update root `AGENTS.md` when the change affects repo-wide launch rules or cross-domain ownership.
