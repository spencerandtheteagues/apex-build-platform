# APEX.BUILD Platform - AI Agent Handoff Document

## Project Overview

APEX.BUILD is a 22nd-century cloud development platform where AI agents (Claude Opus 4.5, GPT-5, Gemini 3) collaborate to build applications from natural language descriptions. Think of it as a competitor to Replit but with multi-AI orchestration.

## Repository Location

```
/home/ubuntu/repos/apex-build-platform
```

## Current State

### What's Implemented

1. **Authentication System**
   - Login/Register UI in `frontend/src/FixedApp.tsx`
   - Demo account: `apex_demo` / `demo12345678`
   - JWT-based auth with refresh tokens
   - Backend endpoints: `/api/v1/auth/login`, `/api/v1/auth/register`, `/api/v1/auth/refresh`, `/api/v1/auth/logout`

2. **Multi-AI Orchestration System** (`backend/internal/agents/orchestrator.go`)
   - Claude Opus 4.5 (Strategist): Spawns Architect, Planner, Reviewer, Documentor sub-agents
   - GPT-5 (Coder): Spawns Frontend Dev, Backend Dev, API Dev, UI Dev, Database Dev sub-agents
   - Gemini 3 (Validator): Spawns Tester, Optimizer, Debugger, Completer sub-agents
   - 8 build phases: Initializing, Planning, Architecture, Coding, Testing, Review, Optimization, Complete
   - Parallel task execution with goroutines and channels
   - **NOTE**: Backend orchestrator exists but is NOT yet wired into main.go routes

3. **Live App Preview** (`backend/internal/preview/preview.go`)
   - Auto-detects app type (React, Vue, Next.js, Node, Python, Go, Static)
   - Process management for different app types
   - Output capture and logging
   - **NOTE**: Backend preview manager exists but is NOT yet wired into main.go routes

4. **Frontend App Builder** (`frontend/src/FixedApp.tsx`)
   - Description input with Fast/Full mode selection
   - Real-time agent cards showing progress
   - Chat panel for agent communication
   - Live preview pane with iframe (Replit-style)
   - Build phase indicator in header
   - **Currently runs simulation** - needs backend integration for real AI calls

### What Needs to Be Implemented

1. **File Manager** (HIGH PRIORITY)
   - Tree view of generated files
   - Click to view/edit in Monaco editor
   - File icons by type
   - Individual file download

2. **ZIP Download with Payment Gate** (HIGH PRIORITY)
   - Generate ZIP of all project files
   - Require payment/credits to download
   - Pricing: ~10% cheaper than Replit

3. **Credit-Based Pricing System** (HIGH PRIORITY)
   - Free tier: Build simple apps, view code (no download)
   - Credits required for: ZIP downloads, complex builds, continued building after limit
   - Subscription tiers with build limits (NOT unlimited - API costs concern)
   - Suggested pricing research needed against Replit

4. **Repository Cloning Feature**
   - Clone existing GitHub repos
   - Auto-detect project type
   - Set up environment automatically
   - Install dependencies

5. **Secret/Environment Variable Manager**
   - Secure storage for API keys, tokens
   - Inject into build environment
   - Never expose in generated code

6. **Version History & Checkpoints**
   - Save snapshots during build
   - Rollback to previous versions
   - Compare versions

7. **One-Click Deploy Integration**
   - Vercel deployment
   - Netlify deployment
   - Railway deployment

8. **Backend Integration** (CRITICAL)
   - Wire orchestrator.go into build handlers
   - Wire preview.go into main routes
   - Connect real AI APIs (Claude, GPT, Gemini)

## Tech Stack

### Backend
- **Language**: Go
- **Framework**: Standard library + Gorilla Mux
- **Database**: PostgreSQL (Neon serverless)
- **ORM**: GORM
- **Auth**: JWT tokens

### Frontend
- **Framework**: React 18 with TypeScript
- **Build Tool**: Vite
- **Editor**: Monaco Editor
- **Styling**: Inline styles (cyberpunk theme)

### Infrastructure
- **Deployment**: Fly.io (configured in fly.toml)
- **Database**: Neon PostgreSQL

## Key Files

### Backend
- `backend/cmd/main.go` - Entry point, route registration
- `backend/internal/api/handlers.go` - HTTP handlers
- `backend/internal/auth/auth.go` - Authentication service
- `backend/internal/db/database.go` - Database connection
- `backend/internal/agents/orchestrator.go` - Multi-AI orchestration (NEW)
- `backend/internal/preview/preview.go` - App preview manager (NEW)

### Frontend
- `frontend/src/FixedApp.tsx` - Main application component (1700+ lines)
- `frontend/src/services/api.ts` - API service
- `frontend/vite.config.ts` - Vite configuration with proxy

## Running Locally

### Backend
```bash
cd ~/repos/apex-build-platform/backend/cmd
go build -o /tmp/apex-server .
DB_PASSWORD=postgres /tmp/apex-server
```
**Note**: There's a database migration issue with constraint "uni_users_username". The backend may fail to start due to this.

### Frontend
```bash
cd ~/repos/apex-build-platform/frontend
npm install --include=dev --legacy-peer-deps
./node_modules/.bin/vite --host 0.0.0.0 --port 5173
```

## Lint/Build Commands

### Frontend Lint
```bash
cd ~/repos/apex-build-platform/frontend && ./node_modules/.bin/eslint src --ext ts,tsx --rule 'no-restricted-globals: off' --quiet
```

### Backend Build
```bash
cd ~/repos/apex-build-platform/backend/cmd && go build -o /tmp/apex-build-test .
```

## Environment Variables

The backend uses these environment variables (check `.envrc` or Fly.io secrets):
- `DATABASE_URL` - PostgreSQL connection string
- `JWT_SECRET` - Secret for JWT signing
- `CLAUDE_API_KEY` - Anthropic API key
- `OPENAI_API_KEY` - OpenAI API key
- `GOOGLE_API_KEY` - Google AI API key

## PR Information

- **PR #1**: https://github.com/spencerandtheteagues/apex-build-platform/pull/1
- **Branch**: `devin/1769220775-auth-and-execution-endpoints`
- **No CI configured** - Tests pass locally

## Pricing Strategy (User Requirements)

The user wants:
1. **Free tier**: Build simple apps, view code, NO free downloads
2. **Credit system**: Pay for ZIP downloads, complex builds
3. **Subscription with limits**: NOT unlimited (API costs concern)
4. **Pricing**: ~10% cheaper than Replit
5. **Profitable model**: Ensure healthy profit margin

Research Replit pricing:
- Replit Core: $20/month (1000 Cycles)
- Suggested APEX pricing: ~$18/month with similar limits

## Known Issues

1. **Database migration error**: `constraint "uni_users_username" of relation "users" does not exist`
2. **Backend orchestrator not integrated**: Code exists but not wired to routes
3. **Preview manager not integrated**: Code exists but not wired to routes
4. **Frontend simulation only**: Real AI calls not connected

## Next Steps (Priority Order)

1. Fix database migration issue
2. Wire orchestrator.go into main.go
3. Wire preview.go into main.go
4. Implement File Manager component
5. Implement ZIP download with payment
6. Implement credit system
7. Add subscription tiers
8. Add repo cloning feature
9. Add secret manager
10. Add version history
11. Add deploy integrations

## Contact

- **User**: Spencer Allen Teague (@spencerandtheteagues)
- **Email**: spencerandtheteagues@gmail.com

## Session Link

https://app.devin.ai/sessions/fe078ee91803482487757e9e7d2edc8e
