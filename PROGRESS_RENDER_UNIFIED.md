Progress Summary (Feb 11, 2026)

Context
- Goal: unify frontend + backend into a single Render web service and enforce BYOK routing.
- User keys were shared in chat; do not use them. User must revoke and reissue.

BYOK enforcement + DeepSeek defaults (already implemented)
- Enforced strict BYOK routing (no platform fallback if any active BYOK key exists).
- Added BYOK usage logging across API/completions/agents/autonomous.
- Default model set to deepseek-r1:18b (Ollama) and added to model lists.
- Files changed:
  - backend/internal/api/handlers.go
  - backend/internal/completions/completions.go
  - backend/internal/agents/ai_adapter.go
  - backend/internal/agents/autonomous/{agent.go,ai_adapter.go,context.go}
  - backend/internal/ai/{byok.go,model.go,ollama.go}
  - backend/cmd/main.go (wiring)
  - frontend/src/components/{settings/APIKeySettings.tsx,ai/ModelSelector.tsx}

Pending work: single Render service (frontend+backend)
- Serve frontend from Go backend:
  - Add static file serving in backend router (likely in backend/cmd/main.go setupRouter).
  - Serve built frontend from a directory inside the backend image, with SPA fallback to index.html.
- Update backend Dockerfile to build frontend (multi-stage):
  - Build frontend with Vite using same-origin URLs: VITE_API_URL=/api/v1, VITE_WS_URL=/ws.
  - Copy frontend dist into backend image (e.g., /app/public).
- Update render.yaml:
  - Remove separate frontend service.
  - Single web service uses backend Dockerfile.
  - Remove VITE_* envs or set them for build args if needed.
- Update frontend runtime routing assumptions:
  - Ensure api.ts/websocket.ts default to same origin in production or keep logic intact.
  - Remove hardcoded Render backend URL if it overrides same-origin.
- CORS / WS origins:
  - Add new unified domain to allowed origins or set CORS_ALLOWED_ORIGINS env.
  - Update collaboration/terminal WS origin lists if necessary.

Working tree state (git status -sb)
- Modified:
  - backend/cmd/main.go
  - backend/internal/agents/ai_adapter.go
  - backend/internal/agents/autonomous/agent.go
  - backend/internal/agents/autonomous/ai_adapter.go
  - backend/internal/ai/byok.go
  - backend/internal/ai/ollama.go
  - backend/internal/api/handlers.go
  - backend/internal/completions/completions.go
  - frontend/src/components/ai/ModelSelector.tsx
  - frontend/src/components/settings/APIKeySettings.tsx
- Untracked:
  - Apex-Build-Model.md
  - apex-backend.env
  - apex-build-mobile-client-prompt.md
  - backend/internal/agents/autonomous/context.go
  - backend/internal/ai/model.go
  - screenshot.png
