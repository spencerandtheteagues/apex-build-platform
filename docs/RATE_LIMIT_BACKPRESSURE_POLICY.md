# APEX-BUILD RATE-LIMIT & BACKPRESSURE POLICY
# Version 1.0 — 2026-06-05
# Owner: harnesskimi (Neo)
# Reviewer: Claude
#
#===============================================================================
# PURPOSE
#===============================================================================
# This document defines the operational rate-limiting and backpressure rules
# for the Apex Build production backend. It is the source of truth for:
#   - Per-scope request limits
#   - Burst allowances
#   - Graceful degradation under load
#   - Redis vs in-memory fallback behavior
#   - What happens when limits are exceeded
#
#===============================================================================
# SCOPES & LIMITS
#===============================================================================

## 1. General API (scope: "api")
- Limit:   1,000 requests per minute per IP
- Burst:   50 requests
- Window:  60 seconds
- Source:  `middleware/middleware.go` — `InitRateLimiter(1000, 50)`
- Behavior:
  - In-memory token bucket per IP (golang.org/x/time/rate)
  - Redis-backed shared counter when REDIS_URL is configured
  - Returns HTTP 429 with `Retry-After: 60s` and `Code: RATE_LIMIT_EXCEEDED`

## 2. Authentication Endpoints (scope: "auth")
- Limit:   10 requests per minute per IP
- Burst:   5 requests
- Window:  60 seconds
- Source:  `middleware/middleware.go` — `InitAuthRateLimiter()`
- Routes protected:
  - POST /api/v1/auth/login
  - POST /api/v1/auth/register
  - POST /api/v1/auth/forgot-password
  - POST /api/v1/auth/refresh
- Behavior:
  - Strict brute-force protection
  - Returns HTTP 429 with `Code: AUTH_RATE_LIMIT_EXCEEDED`
  - Logs: `Auth rate limit exceeded for IP: <ip> on path: <path>`

## 3. Build Start Endpoints (scope: "build")
- Limit:   30 requests per minute per authenticated user
- Burst:   5 requests
- Window:  60 seconds
- Source:  `backend/internal/handlers/builds.go` — build handler middleware stack
- Behavior:
  - User-scoped, not IP-scoped (users share office IPs)
  - Returns HTTP 429 with `Code: BUILD_RATE_LIMIT_EXCEEDED`
  - Does NOT consume AI credits on 429

## 4. AI Provider Routing Backpressure
- Limit:   Provider-specific (OpenRouter ~20 req/min for :free models)
- Burst:   Provider-defined
- Window:  Provider-defined
- Source:  `backend/internal/ai/router.go`
- Behavior:
  - On provider 429/5xx, rotate to next healthy fallback provider
  - Flagship escalation on retry ≥ 2 only for quality-related failures
  - Do NOT escalate to Opus/GPT-4 on transient/rate-limit retries
  - Record provider failure in `failedProviders` list for telemetry

## 5. WebSocket Message Rate (scope: "websocket")
- Limit:   120 messages per minute per connection
- Burst:   20 messages
- Window:  60 seconds
- Source:  `backend/internal/websocket/hub.go`
- Behavior:
  - Connection-scoped token bucket
  - Excess messages are dropped silently (no 429 over WS)
  - After 3 consecutive drops, server sends `rate_limited` control frame

## 6. Preview Runtime Spawn (scope: "preview_spawn")
- Limit:   10 spawns per minute per project
- Burst:   3 spawns
- Window:  60 seconds
- Source:  `backend/internal/preview/`
- Behavior:
  - Returns HTTP 429 with `Code: PREVIEW_SPAWN_RATE_LIMITED`
  - Reuses existing preview container if one is warm

#===============================================================================
# BACKPRESSURE & DEGRADATION
#===============================================================================

## Load-Shedding Priority (highest → lowest)
1. **Keep alive**: /health, /ready, /health/features, /metrics
2. **Keep serving**: Auth (login/register must work for revenue)
3. **Shed first**: Non-essential reads (explore page, public project lists)
4. **Shed next**: Build starts (queue them, do not drop)
5. **Shed last**: Preview spawns (reuse warm containers)

## Queueing Behavior
- Build starts that hit the rate limit are queued in Redis (max queue depth 100)
- Queue overflow returns 429 with `Code: BUILD_QUEUE_FULL`
- Queue TTL: 5 minutes; stale entries are purged by a cron goroutine

## Redis Fallback
- If Redis is unavailable:
  - Shared rate limits fall back to in-memory per-process counters
  - Cross-instance rate limiting becomes best-effort
  - Log a WARNING: `shared rate limit fallback for scope <scope>: <err>`

#===============================================================================
# ENVIRONMENT VARIABLES
#===============================================================================

| Variable | Default | Description |
|---|---|---|
| `APEX_API_RATE_LIMIT_RPM` | 1000 | General API requests per minute |
| `APEX_API_RATE_BURST`     | 50   | General API burst allowance |
| `APEX_AUTH_RATE_LIMIT_RPM`| 10   | Auth endpoint requests per minute |
| `APEX_AUTH_RATE_BURST`    | 5    | Auth endpoint burst allowance |
| `APEX_BUILD_RATE_LIMIT_RPM`| 30  | Build start requests per minute |
| `APEX_BUILD_RATE_BURST`   | 5    | Build start burst allowance |
| `REDIS_URL`               | ""   | Redis connection string for shared counters |

#===============================================================================
# VERIFICATION
#===============================================================================

Run the companion script to verify live behavior:
```bash
BACKEND_URL=https://api.apex-build.dev bash rate_limit_backpressure_test.sh
```

Expected results:
- General API burst > 50 rapid /health calls → at least one 429
- Auth burst > 5 rapid bogus logins → at least one 429
- Build start burst > 5 rapid starts → at least one 429 (requires auth token)
- All 429 responses include `Retry-After` header and JSON `Code` field

#===============================================================================
# CHANGE LOG
#===============================================================================
- 2026-06-05 v1.0 — Initial policy extracted from middleware.go and router.go
