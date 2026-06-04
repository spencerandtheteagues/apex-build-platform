# Scripts Agent Contract

## Purpose

`scripts/` owns repeatable verification and operational automation. These scripts are evidence gates for launch readiness, not casual utilities. They must produce truthful, bounded, reviewable output without leaking secrets.

## Documentation Hierarchy

This file is the level 1 scripts contract. Add child docs if script families grow enough to need local rules, such as `scripts/test/AGENTS.md` or `scripts/live-canary/AGENTS.md`.

## Owned Files And Surfaces

- `run_live_golden_build.mjs`, `run_live_golden_canary_matrix.sh`, `run_live_prompt_matrix.sh`, `run_platform_build_smoke.sh`, `run_platform_canary_matrix.sh`: live and local build quality gates.
- `verify_render_launch_env.mjs`: Render blueprint, service, env, health, and readiness verification.
- `run_render_rollback_drill.mjs`: dry-run-first Render rollback and roll-forward drill. It must use Render's `/services/{serviceId}/rollback` endpoint with exact deploy-id confirmation before execution, never placeholder commit/deploy-trigger rollback behavior.
- `verify_stripe_launch.mjs`: Stripe billing readiness, config, webhook, checkout, and portal verification.
- `verify_no_credit_launch.mjs`: no-AI-credit production smoke for health, frontend shell, admin entitlements, billing config, and optional throwaway free-account entitlement proof.
- `verify_mobile_external_readiness.mjs`: mobile provider/store readiness evidence gate.
- `verify_platform_reliability.sh`, `verify-contract.sh`, `verify-repo.sh`: reliability, contract, and repo verification.
- `loadtest.js`: k6 load harness for TASK-010 launch-concurrency readiness. Default: 200 concurrent unauthenticated landing and health traffic with zero public 5xx responses required. Opt-in: 50-VU authenticated API (`RUN_AUTH_API=1`), 10 concurrent build starts (`RUN_BUILD_STARTS=1`). Both opt-in scenarios require `LOGIN_EMAIL` and `LOGIN_PASSWORD`. Mutating build traffic never runs by default.
- `scripts/lib/`: shared shell helpers.
- `scripts/test/`: tests for script helper behavior and guardrails.

## Stable Contracts

- Scripts must fail non-zero when the launch claim they verify is false.
- Production-facing scripts must default to safe, non-mutating checks. Any paid, mutating, deploy-triggering, or customer-impacting action must require explicit opt-in env vars.
- Never print secrets. Report missing/present status, resource IDs only when safe, issue codes, and recommended fixes.
- Bound network calls with timeouts and clear retry behavior.
- Keep output human-readable and grep-friendly. Prefer explicit `PASSED` / `FAILED` markers for CI logs.
- Scripts that generate screenshots or artifacts should write them to `/tmp` or an ignored path unless a checked-in fixture is intentionally required.
- Placeholder-preview heuristics in live canary scripts must stay behaviorally aligned with backend preview verification. When one changes, update the other and run both focused test/syntax gates.
- Prompt matrix scripts that support a launch-count claim must expose an expected-count guard and fail before live runs when discovered prompts or expanded power modes would produce an empty or undersized matrix.
- Live canary scripts, including `run_platform_build_smoke.sh`, must support the OpenRouter-free paid-testing profile through `scripts/openrouter-free-canary-env.sh`, `APEX_LIVE_TEST_MODEL_PROFILE=openrouter-free`, all build roles assigned to `openrouter`, and `provider_model_overrides.openrouter` pinned to a `:free` OpenRouter model. Use this profile for paid canary builds and paid testing unless a human explicitly approves paid flagship-provider spend; record that evidence as paid/full-stack behavior under OpenRouter-free routing, not as flagship provider health.
- BYOK/Ollama model-spend controls such as `ROLE_ASSIGNMENTS_JSON`, `APEX_ROLE_ASSIGNMENTS_JSON`, provider model overrides, and `APEX_BYOK_OLLAMA_ONLY=1` may still be used for explicit BYOK/Ollama coverage. Use them with `PROVIDER_MODE=byok` only when the launch evidence is intentionally BYOK/Ollama path coverage.
- Keep Node scripts compatible with the repo-supported Node version from docs and CI.

## Development Guidance

- Share shell helpers through `scripts/lib/` when behavior repeats.
- Keep env var names explicit and documented near usage.
- Separate evidence collection from destructive remediation.
- Do not make scripts depend on local-only state unless their name and docs make that scope obvious.
- When a verifier encodes a product promise, update launch docs in the same session.

## Verification

Run focused script tests where available:

```bash
bash scripts/test/canary_matrix_guardrail_test.sh
bash scripts/test/canary_report_test.sh
bash scripts/test/openrouter_free_canary_env_test.sh
bash scripts/test/render_launch_guardrail_test.sh
bash scripts/test/production_canary_workflow_guardrail_test.sh
```

For changed Node verifiers, run them in the safest supported mode without production secrets when possible, then document any skipped production checks.

## Documentation Updates

Update this file when script safety policy, output format, env var contract, canary matrix semantics, production verification behavior, or launch evidence requirements change.
