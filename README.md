# APEX.BUILD

> **The cloud IDE that makes model routing explicit, shows build spend, and lets you own your generated code.**

[![License: APEX Proprietary](https://img.shields.io/badge/License-Proprietary-red.svg)](#license)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://golang.org)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react)](https://react.dev)
[![TypeScript](https://img.shields.io/badge/TypeScript-5-3178C6?logo=typescript)](https://typescriptlang.org)

**Live:** [apex-build.dev](https://apex-build.dev) &nbsp;|&nbsp; **Contact:** [SpencerAndTheTeagues@gmail.com](mailto:SpencerAndTheTeagues@gmail.com)

---

## What Is APEX.BUILD?

APEX.BUILD is a full-stack cloud development platform where you describe what you want to build in plain English and a coordinated team of AI agents writes the code, reviews it, fixes it, tests it, and deploys it — while you watch in real time and stay in complete control of every decision.

This is not a toy. It runs production-grade multi-agent orchestration across configured Anthropic, OpenAI, Google, xAI, Ollama, and BYOK routes, a VS Code-style Monaco IDE, live preview with hot reload, deployment handoff to Vercel / Netlify / Render, real-time collaboration, Git workflow controls, and a billing system with tracked model spend, hard budget caps, and an immutable transaction ledger so you can see what was spent and why.

---

## Table of Contents

1. [The Problem This Solves](#the-problem-this-solves)
2. [Feature Deep-Dive](#feature-deep-dive)
3. [Multi-AI Architecture](#multi-ai-architecture)
4. [The Ten-Agent System](#the-ten-agent-system)
5. [Live Preview & Execution](#live-preview--execution)
6. [Deployment](#deployment)
7. [Real-Time Collaboration](#real-time-collaboration)
8. [Billing & Cost Transparency](#billing--cost-transparency)
9. [Security](#security)
10. [Plans & Pricing](#plans--pricing)
11. [Technical Stack](#technical-stack)
12. [Running Locally](#running-locally)
13. [The Builder's Story](#the-builders-story)
14. [Acquisition](#acquisition)
15. [License](#license)

---

## The Problem This Solves

If you have used Replit, Bolt, v0, or another AI coding platform, you have likely hit tradeoffs around routing control, spend visibility, artifact ownership, and verification. APEX.BUILD is designed for builders who want those controls exposed during the build.

### Head-to-Head: APEX.BUILD vs. the Alternatives

| Builder Need | Typical AI builder tradeoff | APEX.BUILD |
|---|---|---|
| **Explicit model routing** | Many platforms expose model or agent controls, but per-role routing and spend attribution are often not the center of the build stream. | Configured Anthropic, OpenAI, Google, xAI, Ollama, and BYOK routes. Switch per task where enabled. Mix mid-build. |
| **Transparent spend** | Usage can be hard to attribute to a specific model, role, request, or build step. | Tracked spend ticker for provider usage events. Immutable credit ledger. Budget cap enforcement before expensive managed-credit work runs. |
| **Deployment ownership** | Hosted projects often depend on the platform's runtime and plan limits. | Handoff deployments to Vercel, Netlify, or Render so the app can run on your hosting account where supported. |
| **Code ownership** | Export and Git workflows vary by platform and plan. | GitHub export and ZIP handoff for generated projects on supported plans. Your generated code remains inspectable and portable. |
| **Named specialist workflow** | Competitors generally do not publicly break app generation into Apex-style named specialist roles. | Ten specialized agents with distinct roles. The Reviewer is deliberately separate from the writers. The Solver specializes in failures no other agent could fix. |
| **Large files get mangled** | AI context limits cause files to get truncated and content to disappear mid-build. | Chunked editor protocol: files over 400 lines split into overlapping 300-line windows, edited per chunk, reassembled without loss. |
| **IDE-level Git workflow** | Some app builders emphasize prompt and preview flows more than IDE-native Git operations. | Full Git panel: branch, commit, push, pull, merge, PR creation and review inside the IDE. |
| **Preview verification** | Preview and deploy workflows vary by platform; automated runtime verification is not always exposed as a named gate. | WebSocket hot reload plus explicit preview/runtime verification for supported generated preview flows. |
| **Runtime breadth** | Many AI app builders are strongest around web-app and JavaScript/TypeScript workflows. | Node.js, TypeScript, Python, Go, Rust, C/C++, Java, Ruby, PHP, Bash, and more. |
| **Cost control** | Budget controls and usage transparency vary by platform and plan. | Per-build budget cap. Global monthly spend ceiling. Confirmation dialogs. Auto-pause at limit. |
| **Provider and hosting optionality** | Defaults are often managed credentials, managed hosting, or platform-managed model access. | Use managed credits or BYOK routes; BYOK calls use your configured provider account with any Apex routing/platform fee surfaced separately. |
| **Multiplayer workflow** | Real-time collaboration depth varies across major platforms. | Operational transformation (Google Docs-style) with live cursors, presence tracking, and per-project access roles. |
| **No spending history** | No way to see spend over time, by project, or by provider. | Full spend analytics dashboard: daily/weekly/monthly graphs, cost by provider and project, downloadable invoices. |

---

## Feature Deep-Dive

### Natural Language to Working App

Type what you want in plain English. APEX.BUILD parses your prompt, routes it to the Planner and Architect agents to create a structured build plan, then hands off to specialized agents for frontend, backend, database, testing, and DevOps work — running in parallel. You watch it happen through the build progress panel as each agent's actions surface in real time.

When the build completes, proposed changes appear in a full diff review panel. You approve, reject, or modify individual files before anything is written. Nothing is forced on you. You are always in control.

### Three Power Modes

**Fast** — lower-cost configured routes. Fastest turnaround, lowest managed-credit use. Best for scaffolding, boilerplate, and rapid iteration.

**Balanced** — mid/high-quality configured routes. The sweet spot for real work and production-quality output at a reasonable price.

**Max** — highest-quality configured routes across Anthropic, OpenAI, Google, xAI, Ollama, and BYOK where available. Full validation loops, deep code review, maximum quality. For code going to production.

### Full-Featured IDE

Monaco — the same editor core that powers VS Code — running in your browser with:

- Syntax highlighting for every major language
- Multi-file editing with split pane views
- File tree explorer with drag-and-drop organization
- Collaborative code comments with threaded replies
- AI-assisted code review panel with structured quality metrics
- Inline diff viewer for proposed changes
- xterm-based terminal emulator (a real shell, not a simulation)
- Database console with schema explorer and query runner
- Environment variable manager (all values encrypted at rest)
- Package manager UI for npm, pip, and Go modules
- Version history timeline with checkpoint restore

### Checkpoint System

Every significant build step creates a checkpoint. Browse the complete history of your project, see exactly what changed at each step, and restore to any previous state instantly. No undo limit. No lost work.

### Bring Your Own Key (BYOK)

Add your own API keys in Settings. Your API calls go directly from the APEX backend to the AI provider. Managed model markup is bypassed for BYOK calls; any Apex routing or platform fee is surfaced separately. BYOK keys are stored with AES-256 encryption and are never returned in plaintext.

---

## Multi-AI Architecture

APEX.BUILD treats AI providers as interchangeable infrastructure. The AI router sits between every agent and every provider and handles:

**Intelligent routing** — Assigns the best model to each task type based on your power mode.

**Automatic fallback chains** — If a provider is rate-limited or down and fallback routes are healthy, the router can switch across configured providers instead of silently stalling the build.

**Rate limit awareness** — Provider health, throttling, retries, and fallback routing are tracked in real time so unhealthy or constrained routes do not silently stall a build.

**Cost ceiling enforcement** — Maximum spend per request enforced per provider. Requests that would exceed the ceiling are rerouted to a cheaper model in the same chain.

### Configured Provider Routes

| Provider | Best For | Available Models |
|---|---|---|
| **Claude (Anthropic)** | Complex reasoning, code review, documentation | Configured Anthropic routes by power mode and account access |
| **OpenAI** | Agentic coding, fast iteration | Configured OpenAI routes by power mode and account access |
| **Gemini (Google)** | Long-context, multi-modal, budget tasks | Configured Google routes by power mode and account access |
| **Grok (xAI)** | Logic, analysis, sharp reasoning | Configured xAI routes by power mode and account access |
| **Ollama (Local / Cloud)** | BYOK/open-model routing, privacy-sensitive work, cost control | Any model exposed by the configured Ollama endpoint |
| **BYOK** | Your own quotas and billing | Anything your keys allow |

---

## The Ten-Agent System

APEX.BUILD exposes ten specialized agents with distinct roles so planning, implementation, review, repair, and verification are visible instead of collapsed into one opaque generation stream.

| Agent | Role |
|---|---|
| **Lead** | Project manager — coordinates all agents, routes tasks, communicates status |
| **Planner** | Breaks your prompt into a sequenced task list with dependencies |
| **Architect** | Designs system architecture, tech choices, database schema, API contracts |
| **Frontend** | Builds UI components, routing, state management — React/TypeScript specialist |
| **Backend** | Creates API endpoints, business logic, auth flows — Go specialist |
| **Database** | Schema design, migrations, queries, indexing decisions |
| **Testing** | Writes and runs unit, integration, and E2E tests |
| **DevOps** | Dockerfiles, CI/CD, deployment configs, environment templates |
| **Reviewer** | Quality gate — reads finished code with fresh eyes, catches bugs and security issues |
| **Solver** | Failure recovery specialist — classifies errors, selects context, retries with targeted repairs |

### Error Recovery Pipeline

Three systems back the Solver:

**ErrorAnalyzer** — LLM-powered classification into 10 error types with structured `RepairPlan` output.

**ContextSelector** — Selects the most relevant files within an 80,000-token budget by scoring error relevance, import graph proximity, recency, and file type.

**ChunkedEditor** — Splits files over 400 lines into overlapping 300-line windows, edits per chunk, reassembles. Eliminates truncation-related content loss.

### How a Build Runs

```
Your prompt
  → Planner: sequenced task list
  → Architect: system design
  → Frontend + Backend + Database: parallel development
  → Testing: test suite
  → DevOps: deployment config
  → Reviewer: quality gate
  → You: diff review — approve / reject / edit per file
  → (on failure) Solver: AI error analysis + retry loop
  → Live Preview: see it running
  → Deploy: one click to Vercel, Netlify, or Render
```

---

## Live Preview & Execution

A containerized Docker sandbox runs your project. A persistent WebSocket streams console output, network requests, and runtime errors back to your IDE. Hot reload updates the preview in under a second when you save a file.

**What runs:** React / Vue / Svelte / vanilla JS, Node.js / Express / Fastify, Python / Flask / FastAPI, Go / Gin, full-stack projects, databases, shell scripts, and CLI tools.

**Preview panel shows:** Live rendered app, console output (color-coded), network request inspector, server stdout/stderr, build traces, open-in-new-tab.

---

## Deployment

One-click deployment with automatic framework detection and config generation.

| Platform | Best For |
|---|---|
| **Vercel** | React / Next.js / static sites — instant CDN, preview deployments per branch |
| **Netlify** | JAMstack / static — forms, functions, split testing |
| **Render** | Full-stack with databases — persistent servers, managed PostgreSQL, Docker |

Auto-generated: build commands, output directory, environment variable templates, Dockerfile, branch-to-environment mapping.

**You own the deployment.** Your app runs on your platform account. Hosting cost is between you and Vercel / Netlify / Render. APEX charges nothing for hosting.

---

## Real-Time Collaboration

Operational transformation (Google Docs-style conflict resolution) with live cursor positions, presence indicators, per-file edit warnings, threaded code comments, and instant file sync across all clients on a single persistent WebSocket connection.

**Access levels:** Owner (full access + collaborator management), Editor (read/write + build/deploy), Viewer (read-only + comments).

---

## Billing & Cost Transparency

### Real-Time Cost Ticker
Every AI request shows provider, model, input tokens, output tokens, and total cost as tokens stream. You see spending as it happens.

### Immutable Credit Ledger
Every transaction is written to an append-only ledger with timestamps and source metadata. Downloadable. Nothing is a black box.

### Budget Controls
- Global monthly hard cap — builds pause automatically when hit
- Per-build cost estimation with confirmation dialog above your threshold
- Spend alerts at 50%, 80%, and 100% of monthly budget
- BYOK lowers managed-credit burn with direct provider billing plus the platform routing fee

### One-Time Top-Ups
Buy additional credits via Stripe Checkout: $25, $50, $100, or $250. Added to balance instantly.
Credit packs extend managed usage; they do not unlock backend/full-stack capabilities without a paid plan.

---

## Security

- JWT access tokens (15-min expiry) + refresh token rotation
- BCrypt password hashing (cost 12)
- AES-256 encryption for all stored API keys
- Keys never returned in plaintext after initial storage
- CSP: `script-src 'self'` — no unsafe-inline
- X-Frame-Options, X-Content-Type-Options, Referrer-Policy headers
- Rate limiting: 10 req/s API, 1 req/s auth endpoints
- Docker sandbox with memory/CPU/disk caps and network isolation
- Stripe webhook signature verification + idempotency (no double-processing)
- Server-side checkout URL generation — client never touches Stripe keys

---

## Plans & Pricing

| | Free | Builder | Pro | Team | Enterprise |
|---|---|---|---|---|---|
| **Price** | $0/mo | $24/mo | $79/mo | $149/mo | Contact |
| **Annual** | — | $230/yr | $758/yr | $1430/yr | Negotiated |
| **AI credits/mo** | 0 | $12 | $40 | $110 | Negotiated |
| **Projects** | 3 | Unlimited | Unlimited | Unlimited | Unlimited |
| **Storage** | 1 GB | 5 GB | 20 GB | 100 GB | Custom |
| **Executions/day** | 50 | 200 | 1,000 | 5,000 | Unlimited |
| **Collaborators** | 1 | 1 | 3 | Unlimited | Unlimited |
| **Backend / full-stack builds** | No | Yes | Yes | Yes | Yes |
| **Configured provider routes** | Limited | Yes | Yes | Yes | Yes |
| **GitHub export** | No | Yes | Yes | Yes | Yes |
| **Priority queue** | No | No | Yes | Yes | Yes |
| **SSO / Audit logs** | No | No | No | No | Yes |

Paid plans include the full build workspace with specialist agent roles, live preview, deployment handoff, Git controls, real-time collaboration, Monaco IDE, cost dashboard, budget controls, checkpoint system, and BYOK support where enabled by plan.

---

## Technical Stack

**Backend:** Go 1.26+, Gin, GORM, PostgreSQL 15, Redis 7, Stripe Go SDK, Docker

**Frontend:** React 18, TypeScript 4.9, Vite 4, TailwindCSS 4, Monaco Editor, xterm.js, Framer Motion, Zustand

**Infrastructure:** Render (backend + frontend), Render Managed PostgreSQL, Render Key Value, Stripe live mode

### Repository Layout

```
apex-build/
├── backend/
│   ├── cmd/main.go                    # Entry point, route wiring
│   ├── internal/
│   │   ├── agents/                    # Multi-agent orchestration engine
│   │   │   ├── manager.go             # Build pipeline controller
│   │   │   ├── chunked_edit.go        # Sliding-window large-file editor
│   │   │   ├── context_selector.go    # Token-budget-aware file selection
│   │   │   ├── error_analyzer.go      # AI-powered error classification
│   │   │   └── proposed_edits.go      # Diff review system
│   │   ├── ai/                        # Provider clients + intelligent router
│   │   ├── handlers/                  # HTTP handlers
│   │   ├── payments/                  # Stripe integration
│   │   ├── preview/                   # Live preview + server runner
│   │   └── auth/                      # JWT, OAuth, sessions
│   └── migrations/                    # SQL schema (7 versions)
├── frontend/
│   ├── src/
│   │   ├── components/builder/        # App builder UI
│   │   ├── components/ide/            # Full IDE layout + panels
│   │   ├── components/ai/             # AI chat, model selector
│   │   └── components/billing/        # Plans, spend dashboard
│   ├── nginx.conf                     # Production nginx
│   └── Dockerfile                     # Multi-stage production image
├── docs/                              # Architecture, API, deployment guides
├── tests/                             # Unit, integration, Playwright E2E
├── render.yaml                        # Production deployment blueprint
└── docker-compose.yml                 # Local development stack
```

---

## Running Locally

### Prerequisites
Go 1.26+, Node.js 20+, PostgreSQL 15+, Docker

### Setup

```bash
git clone https://github.com/spencerandtheteagues/apex-build-platform.git
cd apex-build-platform
cp .env.example .env
# Edit .env with database credentials and at least one AI provider key

# Backend
cd backend && go mod download && go run ./cmd/main.go

# Frontend (new terminal)
cd frontend && npm install && npm run dev
```

Frontend: http://localhost:5180 | Backend: http://localhost:8080/api/v1

### Docker Compose

```bash
docker compose up --build
```

Frontend: http://localhost:3000 | Backend: http://localhost:8080

### Required Environment Variables

```bash
DATABASE_URL=postgresql://user:pass@localhost:5432/apex_build
JWT_SECRET=at-least-32-chars-random
JWT_REFRESH_SECRET=at-least-32-chars-random-different-from-above
ANTHROPIC_API_KEY=sk-ant-...   # at least one AI key required
APP_URL=http://localhost:5180
CORS_ALLOWED_ORIGINS=http://localhost:5180
STRIPE_SECRET_KEY=sk_test_...  # test keys fine for local dev
STRIPE_WEBHOOK_SECRET=whsec_...
```

### Verification

```bash
cd backend && go build ./cmd/main.go && go test ./...
cd frontend && npm run typecheck && npm run build
```

### Health Endpoints
- `/health` — liveness
- `/ready` — readiness
- `/health/features` — feature readiness summary

### Documentation
- [Development Guide](docs/development.md)
- [Deployment Guide](docs/deployment.md)
- [Architecture Guide](docs/architecture.md)
- [API Guide](docs/api.md)

---

## The Builder's Story

APEX.BUILD was not built by a team of engineers in a San Francisco office. It was built by one person — Spencer Teague — in a small town in Texas. He used to be a carpenter.

Spencer has never held a software job. He has no computer science degree. He has not taken a bootcamp. What he has is an obsessive drive to build things and the willingness to spend thousands of hours learning how to use AI tools as genuine development partners — not shortcuts.

His tools: **Claude Code CLI**, **ChatGPT Codex CLI**, and **Gemini CLI**. His process: describe what needs to happen, review what comes back, understand it well enough to know when it is wrong, push back, iterate, fix, repeat. Thousands of hours. Across five large platforms.

### His Other Platforms

Every one of these was built 100% solo:

**[My AI Social Media Manager](https://myaimediamgr.com)** — AI-powered social media management. Scheduling, content generation, engagement tracking, multi-network analytics. Production live.

**SilverGuard ElderCare** — AI-powered elder care coordination. Emergency alerts, medication tracking, family communication, caregiver scheduling for aging-in-place.

**Specter** — Security monitoring and threat detection. Network analysis, alert triage, incident response.

**Aegis App Architect** — The direct precursor to APEX.BUILD. AI-assisted application design and scaffolding. The platform where the core concepts of APEX were first developed.

The only money spent: API keys and tool subscriptions. No VC funding. No co-founders. No employees.

APEX.BUILD is the culmination of everything learned across four previous platforms — the architecture patterns that work, the failure modes that don't, the production incidents that teach you what matters. Every line of it came from one person refusing to let not knowing how to code be a reason to stop building.

---

## Acquisition

APEX.BUILD is actively available for acquisition.

### What Transfers

- Complete source code (frontend + backend)
- Full documentation suite
- Live production deployment on Render
- Custom domain: apex-build.dev
- Stripe live mode integration (account, products, prices, webhooks)
- Configured AI provider integrations
- PostgreSQL database with complete schema and migration history
- Render infrastructure credentials
- 30 days of transition support from the builder

### Why It Has Real Value

The multi-agent orchestration engine (planner → architect → frontend/backend/database → testing → DevOps → reviewer → solver, with ChunkedEditor, ErrorAnalyzer, ContextSelector) represents thousands of hours of design and refinement that cannot be bought off a shelf. The billing system handles real money in production today. The architecture scales horizontally — the Go backend is stateless, the database is managed PostgreSQL, the frontend is a static React app behind nginx.

This is a platform built to the standard of production software, not a side project demo.

### Contact

**Spencer Teague**
- **Phone / Text:** 512-666-7450 *(Google Voice — text preferred)*
- **Email:** [SpencerAndTheTeagues@gmail.com](mailto:SpencerAndTheTeagues@gmail.com)

Serious inquiries only.

---

## License

**APEX.BUILD Proprietary License**

Copyright © 2025 Spencer Teague. All rights reserved.

This source code is made available for **viewing and evaluation purposes only**. No license is granted to copy, modify, distribute, sublicense, use in production, or create derivative works from any part of this codebase without explicit written permission from the copyright holder.

This repository is public to demonstrate the work and capabilities of the platform. Viewing the code does not grant any rights to use it.

For licensing inquiries, contact: [SpencerAndTheTeagues@gmail.com](mailto:SpencerAndTheTeagues@gmail.com)

---

*Built solo in Texas. Powered by stubbornness and API keys.*
