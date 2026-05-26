# Docs Agent Contract

## Purpose

`docs/` owns Apex Build narrative documentation: architecture guides, launch-readiness evidence, deployment instructions, API maps, roadmap/handoff material, pricing and runbook docs, and product/competitive strategy artifacts.

Docs must help agents and humans make correct launch decisions. They must not become stale optimism or duplicate implementation contracts that contradict `AGENTS.md` files.

## Documentation Hierarchy

This file is the level 1 docs contract. Add child docs if a documentation subtree becomes its own maintained surface, such as `docs/architecture-intelligence/AGENTS.md`.

## Owned Files And Surfaces

- `architecture.md`, `api.md`, `development.md`, `deployment.md`: stable human-readable technical maps.
- `launch-readiness-tracker.md`, `launch-runbook.md`, `builder-hardening-plan.md`, `canary-reliability-handoff.md`: launch evidence, blockers, runbook, and reliability handoff docs.
- `replit-overtake-roadmap.md`, `contract-first-orchestration-plan.md`, `mobile-app-builder-architecture.md`, `stripe-pricing-runbook.md`: strategy and subsystem plans.
- `architecture-intelligence/`: AI-oriented repo maps and analysis artifacts.
- Root-level launch and business docs such as `LAUNCH_WAR_PLAN.md`, `FUTURE.md`, and pricing/audit docs are governed by this documentation policy even though they live at repo root.

## Stable Contracts

- `README.md` is public-facing. Keep it focused on pitch, quick start, links, screenshots, release/community paths, and the high-level documentation map.
- `AGENTS.md` files are binding engineering contracts. Do not hide durable implementation rules only in narrative docs.
- Launch docs must reflect evidence, dates, command output summaries, and remaining blockers accurately.
- Architecture docs must be updated when ownership boundaries, runtime flow, API/WebSocket contracts, or deployment model changes.
- Pricing and billing docs must match backend payment contracts and frontend customer copy.
- Roadmaps and future docs must distinguish shipped, verified, in progress, blocked, and speculative items.

## Development Guidance

- Prefer one owning doc over scattered notes for the same concept.
- Remove or reconcile stale claims immediately when code disproves them.
- Keep operational commands copy-pasteable and safe by default.
- Do not paste secret-bearing logs, provider transcripts, customer prompts, or private customer app output.
- When adding evidence, include exact dates and enough context to understand environment and scope.

## Verification

For doc-only changes, verify links and command names by reading the referenced files. For docs that claim a test, canary, deploy, or billing result, confirm the evidence exists before updating the status.

## Documentation Updates

Update this file when the documentation system, ownership model, evidence policy, public README boundary, or launch-doc workflow changes.
