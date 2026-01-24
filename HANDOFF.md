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
   - Admin account: `spencerandtheteagues@gmail.com` / `The$t@r$h1pKey!` (unlimited access)
   - JWT-based auth with refresh tokens
   - Backend endpoints: `/api/v1/auth/login`, `/api/v1/auth/register`, `/api/v1/auth/refresh`, `/api/v1/auth/logout`

2. **Admin Account System**
   - Admin users have unlimited access (no credit requirements)
   - IsAdmin field in User model
   - Auto-created on database migration
   - Admin credentials: spencerandtheteagues@gmail.com / The$t@r$h1pKey!

3. **Multi-AI Orchestration System** (`backend/internal/agents/orchestrator.go`)
   - Claude Opus 4.5 (Strategist): Spawns Architect, Planner, Reviewer, Documentor sub-agents
   - GPT-5 (Coder): Spawns Frontend Dev, Backend Dev, API Dev, UI Dev, Database Dev sub-agents
   - Gemini 3 (Validator): Spawns Tester, Optimizer, Debugger, Completer sub-agents
   - 8 build phases: Initializing, Planning, Architecture, Coding, Testing, Review, Optimization, Complete
   - Parallel task execution with goroutines and channels

4. **Live App Preview** (`backend/internal/preview/preview.go`)
   - Auto-detects app type (React, Vue, Next.js, Node, Python, Go, Static)
   - Process management for different app types
   - Output capture and logging

5. **Frontend App Builder** (`frontend/src/FixedApp.tsx`)
   - Description input with Fast/Full mode selection
   - Real-time agent cards showing progress
   - Chat panel for agent communication
   - Live preview pane with iframe (Replit-style)
   - Build phase indicator in header

6. **File Manager**
   - Tree view of generated files with collapsible folders
   - File icons by type (tsx, go, css, json, etc.)
   - Click to view file contents with syntax highlighting
   - Toggle button in header

7. **ZIP Download with Payment Gate**
   - Download ZIP button appears after build complete
   - Requires credits for free users
   - Pro/Team/Admin users can download freely

8. **Credit-Based Pricing System**
   - Free tier: 3 builds/month, no downloads (need credits)
   - Credits: $9 per 100 credits (10% cheaper than Replit)
   - Download cost: 5 credits per ZIP
   - Build cost: 10 credits (fast) / 25 credits (full)
   - API endpoints: `/api/v1/credits`, `/api/v1/credits/purchase`, `/api/v1/pricing`

9. **Subscription Tiers**
   - Free: $0, 3 builds/month, credits required for downloads
   - Pro: $18/month (10% cheaper than Replit), 50 builds, unlimited downloads
   - Team: $45/month, 200 builds, team features

10. **Secret/Environment Variable Manager**
    - Create secrets: `POST /api/v1/projects/:projectId/secrets`
    - List secrets (names only, never values): `GET /api/v1/projects/:projectId/secrets`
    - Delete secrets: `DELETE /api/v1/secrets/:id`
    - Values never exposed in JSON responses

11. **Version History & Checkpoints**
    - Create version: `POST /api/v1/projects/:projectId/versions`
    - List versions: `GET /api/v1/projects/:projectId/versions`
    - Get version with snapshot: `GET /api/v1/versions/:id`
    - Auto-save and manual checkpoint support

12. **Repository Cloning**
    - Clone endpoint: `POST /api/v1/clone`
    - Auto-detect project type (react, vue, go, python)
    - Creates project from cloned repo

### What Still Needs Work

1. **One-Click Deploy Integration**
   - Vercel deployment
   - Netlify deployment
   - Railway deployment

2. **Backend Integration** (CRITICAL)
   - Wire orchestrator.go into build handlers (partially done)
   - Wire preview.go into main routes (partially done)
   - Connect real AI APIs (Claude, GPT, Gemini)

3. **Fly.io Backend Deployment**
   - Frontend deployed: https://apex-build-platform-zmbl89de.devinapps.com
   - Backend deployment failed due to authorization issues

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

## API Endpoints Summary

### Authentication (No auth required)
- `POST /api/v1/auth/register` - Register new user
- `POST /api/v1/auth/login` - Login
- `POST /api/v1/auth/refresh` - Refresh token
- `POST /api/v1/auth/logout` - Logout
- `GET /api/v1/pricing` - Get pricing info

### Protected Endpoints (Auth required)
- `GET /api/v1/credits` - Get user credits and usage
- `POST /api/v1/credits/purchase` - Purchase credits
- `POST /api/v1/credits/deduct` - Deduct credits
- `POST /api/v1/build/record` - Record a build
- `POST /api/v1/download/record` - Record a download
- `POST /api/v1/projects/:projectId/secrets` - Create secret
- `GET /api/v1/projects/:projectId/secrets` - List secrets
- `DELETE /api/v1/secrets/:id` - Delete secret
- `POST /api/v1/projects/:projectId/versions` - Create version
- `GET /api/v1/projects/:projectId/versions` - List versions
- `GET /api/v1/versions/:id` - Get version
- `POST /api/v1/clone` - Clone repository

## Pricing Strategy (Implemented)

| Tier | Price | Builds/Month | Downloads | Features |
|------|-------|--------------|-----------|----------|
| Free | $0 | 3 | Credits required | Basic app building, view code, live preview |
| Pro | $18/mo | 50 | Included | Priority AI, version history, deploy integrations |
| Team | $45/mo | 200 | Included | Team collaboration, shared projects, admin dashboard |

Credits: $9 per 100 (10% cheaper than Replit)
- Download: 5 credits
- Fast build: 10 credits
- Full build: 25 credits

## Known Issues

1. **Database migration error**: `constraint "uni_users_username" of relation "users" does not exist` (pre-existing)
2. **Fly.io backend deployment**: Authorization error prevents deployment
3. **Frontend simulation only**: Real AI calls not connected yet

## Next Steps (Priority Order)

1. Add one-click deploy integrations (Vercel, Netlify, Railway)
2. Wire real AI APIs into orchestrator
3. Fix Fly.io backend deployment
4. Add Monaco editor integration for file editing
5. Add real-time collaboration features

## Contact

- **User**: Spencer Allen Teague (@spencerandtheteagues)
- **Email**: spencerandtheteagues@gmail.com
- **Admin Account**: spencerandtheteagues@gmail.com / The$t@r$h1pKey!

## Session Link

https://app.devin.ai/sessions/fe078ee91803482487757e9e7d2edc8e
