# GitHub Workflow Agent Contract

## Purpose

`.github/` owns repository automation: CI, deploy workflows, production canaries, reliability-nightly jobs, and repo-level GitHub configuration. These workflows protect launch readiness and must remain safe for accounts where hosted Actions are intentionally disabled.

## Documentation Hierarchy

This file is the level 1 GitHub automation contract. Add child docs only if workflow families become complex enough to need local ownership.

## Owned Files And Surfaces

- `.github/workflows/ci.yml`: standard verification workflow.
- `.github/workflows/deploy.yml`: deployment automation.
- `.github/workflows/production-canary.yml`: opt-in production launch verification.
- `.github/workflows/reliability-nightly.yml`: opt-in reliability matrix and scheduled checks.

## Stable Contracts

- Hosted GitHub Actions must remain opt-in through `APEX_ENABLE_GITHUB_ACTIONS=true` unless the repo owner explicitly changes that policy.
- Workflows must not print secrets. Use GitHub secrets and masked output.
- Production canaries must verify live URLs and launch readiness honestly, and they must fail when critical checks fail.
- Manually requested launch-evidence jobs must fail when their required secrets or credentials are absent; optional scheduled coverage may skip unavailable paid/external checks only when launch docs describe that limitation.
- When `APEX_REQUIRE_PAID_CANARIES=true`, production canary workflows must require existing verified paid canary credentials, must not use disposable smoke registration as paid evidence, and must fail skipped paid build/golden canaries.
- Paid canary, golden, and prompt-matrix jobs must route paid/full-stack build traffic through the OpenRouter-free testing profile by default: `PROVIDER_MODE=platform`, all build roles assigned to `openrouter`, and `APEX_PROVIDER_MODEL_OVERRIDES_JSON` pinned to a `:free` OpenRouter model. Do not switch these jobs back to paid flagship providers or BYOK/Ollama without an explicit operator decision and launch-doc update.
- Use the repo-supported Node and Go versions consistently with `docs/development.md` and package configs.
- Keep workflow names, required secrets, required variables, and manual dispatch inputs documented in launch docs when they affect launch evidence.

## Development Guidance

- Prefer small workflow changes with explicit env vars and clear job names.
- Keep paid, mutating, deploy-triggering, or customer-impacting checks behind manual dispatch inputs or explicit secrets/vars.
- Reuse scripts under `scripts/` instead of duplicating verifier logic in YAML.
- When changing a workflow gate, update `docs/launch-readiness-tracker.md` or `docs/launch-runbook.md` if launch evidence collection changes.

## Verification

Validate YAML syntax by reading the workflow and, when available, running local script commands that the workflow invokes. For GitHub-hosted verification, use `gh run list` and workflow logs without exposing secrets.

## Documentation Updates

Update this file when workflow opt-in policy, required secrets/vars, launch canary behavior, deploy automation, or CI verification gates change.
