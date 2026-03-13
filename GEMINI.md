# Gemini Instructions For Apex.Build

This repository includes project-level Gemini subagents in `.gemini/agents`. They are for improving APEX.BUILD itself, not for generating random user apps.

## Install And Enable

1. Install or update Gemini CLI.
   Recommended:
   ```bash
   npm install -g @google/gemini-cli@latest
   ```
2. Enable experimental subagents in `~/.gemini/settings.json`:
   ```json
   {
     "experimental": {
       "enableAgents": true
     }
   }
   ```
3. Start Gemini from the repo root:
   ```bash
   cd /home/s/projects/apex-build-platform
   gemini
   ```
4. After pulling updates that change `GEMINI.md` or `.gemini/agents`, restart the Gemini session so it reloads the project instructions and agent definitions.

Project-level subagents in `.gemini/agents/*.md` should become available automatically when agents are enabled.

## Read These First

Before changing code, Gemini should read:

1. `geminihandoff.md`
2. `docs/contract-first-orchestration-plan.md`
3. `AGENTS.md`

`geminihandoff.md` is the current execution and constraint file. Do not skip it.

## Effective Use Of The Project Agents

Use one primary specialist and at most two supporting specialists for a slice of work. Do not fan out to a large swarm for routine tasks. For important work, explicitly name the agents you want Gemini to use instead of relying on implicit routing.

Recommended mapping for the current remaining work in `geminihandoff.md`:

- Section `3.1` repair-work-order selection from fingerprints:
  - Primary: `apex-repair-ladder-marshal`
  - Support: `apex-provider-economics-analyst`, `apex-regression-sentinel`
- Section `3.2` model-driven repair tasks becoming patch-first:
  - Primary: `apex-patch-promotion-foreman`
  - Support: `apex-surface-verification-judge`, `apex-regression-sentinel`
- Section `3.3` repair promotion boundaries:
  - Primary: `apex-surface-verification-judge`
  - Support: `apex-repair-ladder-marshal`, `apex-regression-sentinel`
- Section `3.4` truthfulness preservation:
  - Primary: `apex-surface-verification-judge`
  - Support: `apex-orchestration-architect`
- User-in-the-loop builder flow, agent messaging, build transparency:
  - Primary: `apex-build-governor` or `apex-build-experience-conductor`
  - Support: `apex-preview-runtime-steward`, `apex-regression-sentinel`
- Preview/runtime/standalone preview reliability:
  - Primary: `apex-preview-runtime-steward`
  - Support: `apex-surface-verification-judge`
- Replit-parity prioritization and sequencing:
  - Primary: `apex-replit-parity-strategist`
  - Support: `apex-build-governor`, `apex-provider-economics-analyst`

## Agent Roster

- `apex-build-governor`: build control, user intervention, direct agent messaging, truthful coordination
- `apex-orchestration-architect`: manager/orchestrator flow, contracts, work orders, patch-oriented execution
- `apex-repair-ladder-marshal`: failure classification, retry history, solver escalation, localized recovery
- `apex-patch-promotion-foreman`: patch bundles, mutation seams, repair promotion, compatibility with raw file outputs
- `apex-surface-verification-judge`: verification reports, truth tags, promotion gates, readiness evidence
- `apex-provider-economics-analyst`: provider routing, scorecards, fingerprints, quality-per-dollar
- `apex-preview-runtime-steward`: preview handoff, runtime smoke, env coherence, IDE preview decoupling
- `apex-build-experience-conductor`: section-by-section build UX, telemetry, completion UX, restored build context
- `apex-regression-sentinel`: tests, failure replay, regression hardening
- `apex-replit-parity-strategist`: prioritization toward beating Replit on smoothness, control, reliability, and cost efficiency

## Strict Repo Rules

- Preserve the existing phased manager, provider router, context selector, error analyzer, deterministic repairs, final readiness validation, and preview/backend verification.
- Prefer contract-first, patch-first, gated changes over broad rewrites.
- Keep hosted builds on Claude, GPT, Gemini, and Grok only. Keep Ollama BYOK/local-only.
- Keep the user involved in the build flow. Favor section-by-section visibility and direct controls over one-shot black-box generation.
- Do not claim access to hidden provider chain-of-thought that model APIs never returned.
- Do not create or commit junk files such as `BOOTSTRAP.md`, `HEARTBEAT.md`, `IDENTITY.md`, `SOUL.md`, `TOOLS.md`, `USER.md`, or `.openclaw/`.
- Do not push if the verification commands in `geminihandoff.md` fail.

## Recommended Prompt Pattern

Use prompts like:

```text
Read GEMINI.md and geminihandoff.md first.
Use apex-repair-ladder-marshal as primary, apex-provider-economics-analyst and apex-regression-sentinel as support.
Complete section 3.1 only.
Preserve the current compatibility path and do not touch unrelated frontend files.
```

or:

```text
Read GEMINI.md and geminihandoff.md first.
Use apex-patch-promotion-foreman as primary and apex-surface-verification-judge as support.
Complete section 3.2 only.
Keep TaskOutput.Files compatibility intact and add focused regression coverage.
```

Be explicit about the section, primary owner, supporting agents, and file scope.
