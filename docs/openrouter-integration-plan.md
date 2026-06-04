# OpenRouter Integration + Auto Model Selection Plan

## IMPLEMENTATION STATUS (last updated: 2026-06-03, commit 09321ad)

### ✅ COMPLETED (backend)
- `backend/internal/ai/openrouter_catalog.go` — 70+ curated models with quality scores, free tier flagged, `catalogScore()` algorithmic selector
- `backend/internal/ai/openrouter_selector.go` — 3-tier auto-selection: (1) GPT-5.5 dispatcher via OpenRouter, (2) deepseek-v4-pro fallback via OpenRouter, (3) Ollama cloud DeepSeek fallback, (4) pure catalog scoring
- `backend/internal/ai/openrouter.go` — Full `AIClient` implementation; `FetchLiveModels()` for frontend picker; hard-blocks `anthropic/*`; free-only gating; cost tracking
- `backend/internal/api/openrouter.go` — `GET /api/ai/openrouter/models` with 5-min cache + catalog fallback
- `backend/internal/ai/types.go` — `ProviderOpenRouter` constant added; `DefaultRouterConfig` updated (OpenRouter is now default for code/test/refactor/debug); OpenRouter in all fallback chains
- `backend/internal/ai/model.go` — `ParseProvider` updated
- `backend/internal/ai/router.go` — `NewAIRouter` wires OpenRouter via `extraKeys[3]`; `GetConfiguredProviders` updated
- `backend/cmd/main.go` — `AppConfig.OpenRouterAPIKey`, `loadConfig()` reads `OPENROUTER_API_KEY`, passes to `NewAIRouter`, logs startup

### ✅ COMPLETE (frontend)
1. ✅ **Auto mode button** — added "Auto ❆" as 4th power mode button (grid-cols-4) in both launch and full-build panels
   - Sends `power_mode: "auto"`, `provider: "openrouter"`, `model: "auto"` in build request
2. ✅ **Manual OpenRouter picker modal** — `OpenRouterModelPicker.tsx` (searchable, filterable by tier/free, sortable)
   - Calls `GET /api/ai/openrouter/models`, searchable, filterable (Free toggle, category, sort)
   - `FREE` badge on free models, cost shown per 1M tokens
3. ✅ **`ModelRoleConfig.tsx` provider strip** — added OpenRouter provider card + Auto ✦ quick-assign strip per role
4. ⚠️ **Build transparency** — deferred; backend already sets `resp.metadata.model` but frontend phase display unchanged
   - Data comes from `resp.metadata.model` already set in `openrouter.go`

### ❌ REMAINING (deployment)
- Set `OPENROUTER_API_KEY` in Render environment variables
  - Key is in `~/.secrets/api-keys.sh` on the dev machine

### ❌ REMAINING (testing)
- `OPENROUTER_TEST_MODEL=moonshotai/kimi-k2.6:free` — run canary build end-to-end for free
- Verify GPT-5.5 dispatcher actually returns valid model IDs (integration test)
- Confirm `isAnthropicModel()` hard-guard works in production

## Goal

1. Add OpenRouter as a unified provider gateway for all non-Claude, non-Ollama models — one API key instead of separate OpenAI/Gemini/Grok keys.
2. Add an **Auto mode** with a smart LLM-powered dispatcher that picks the best available OpenRouter model for each task (quality-first, then cost).
3. Add a **Manual OpenRouter picker** — any of OpenRouter's 300+ models assignable to any agent role.
4. Leverage **OpenRouter free-tier models** for development testing, canary builds, and a genuine free user tier.
5. Claude stays on direct Anthropic API. Ollama cloud stays as-is.

---

## Architecture Overview

```
Request comes in
    │
    ├─ provider = "claude"     → direct Anthropic API (unchanged)
    ├─ provider = "ollama"     → Ollama cloud/local (unchanged)
    ├─ provider = "openrouter" → OpenRouter client
    │       └─ model = "auto"  → dispatcher → picks best model → OpenRouter
    │       └─ model = "<id>"  → OpenRouter with that specific model
    └─ provider = ""           → existing router logic (falls back to openrouter for non-claude tasks)
```

---

## Backend Changes

### 1. New File: `backend/internal/ai/openrouter.go`

OpenRouter is OpenAI-wire-compatible (`https://openrouter.ai/api/v1/chat/completions`). The client:
- Implements the existing `AIClient` interface
- Accepts a model ID in `AIRequest.Model`; when empty or `"auto"`, delegates to `AutoSelector`
- Adds OpenRouter-specific headers: `HTTP-Referer: https://apex-build.dev`, `X-Title: Apex Build`
- Tracks usage (tokens, cost) same as other clients
- Health check: hits `/api/v1/models` with a short timeout

```go
type OpenRouterClient struct {
    apiKey     string
    httpClient *http.Client
    selector   *AutoSelector
    usage      *ProviderUsage
    usageMu    sync.RWMutex
}
```

### 2. New File: `backend/internal/ai/openrouter_catalog.go`

Static catalog of ~30 curated models with quality/cost metadata. Updated manually as models improve.

```go
type CatalogModel struct {
    ID              string            // OpenRouter model ID e.g. "openai/gpt-4o"
    DisplayName     string
    CapabilityScore map[AICapability]float64  // 0.0–1.0
    InputCostPer1M  float64           // USD per 1M input tokens
    OutputCostPer1M float64           // USD per 1M output tokens
    ContextWindow   int               // tokens
    SpeedRating     float64           // 0.0–1.0 (higher = faster)
    AvailableVia    string            // "openrouter" always
}
```

Initial catalog (verify slugs against OpenRouter API before deploy):

| Model | Code | Reasoning | Speed | $/1M in | $/1M out |
|-------|------|-----------|-------|---------|---------|
| `openai/gpt-4o` | 0.92 | 0.90 | 0.85 | $2.50 | $10.00 |
| `openai/o4-mini` | 0.88 | 0.95 | 0.80 | $1.10 | $4.40 |
| `openai/o3` | 0.90 | 0.98 | 0.55 | $10.00 | $40.00 |
| `google/gemini-2.5-pro` | 0.91 | 0.93 | 0.75 | $1.25 | $10.00 |
| `google/gemini-2.5-flash` | 0.85 | 0.87 | 0.95 | $0.15 | $0.60 |
| `deepseek/deepseek-r1` | 0.89 | 0.96 | 0.65 | $0.55 | $2.19 |
| `deepseek/deepseek-v3` | 0.90 | 0.88 | 0.80 | $0.27 | $1.10 |
| `x-ai/grok-3` | 0.87 | 0.89 | 0.78 | $3.00 | $15.00 |
| `x-ai/grok-3-mini` | 0.82 | 0.85 | 0.90 | $0.30 | $0.50 |
| `meta-llama/llama-4-maverick` | 0.86 | 0.85 | 0.88 | $0.18 | $0.78 |
| `qwen/qwen-2.5-coder-32b-instruct` | 0.88 | 0.83 | 0.82 | $0.07 | $0.16 |
| `mistralai/mistral-large-2411` | 0.83 | 0.85 | 0.85 | $2.00 | $6.00 |
| `anthropic/claude-3-5-sonnet` | — | — | — | **SKIP** | **SKIP** |

> **NOTE**: All `anthropic/*` models must be excluded from OpenRouter routing. Claude always routes direct.

### 3. New File: `backend/internal/ai/openrouter_selector.go`

Two-tier selection:

**Tier A — Algorithmic (instant, used as baseline and fallback):**

```
score(model, capability) = quality² / (1 + log₂(cost_per_call + 1))
```

Where `cost_per_call` is estimated from `(prompt_len/4 * input_cost) + (max_tokens/4 * output_cost)`.

**Tier B — LLM Dispatcher (for auto mode, adds ~400ms):**

Primary dispatcher model: **ChatGPT 5.5 / Codex 5.5** via OpenRouter
- OpenRouter model ID: `openai/codex-mini-latest` or the current ChatGPT 5.5 slug
- **TODO**: Verify exact slug at `https://openrouter.ai/api/v1/models` before deploy — this is a post-August-2025 model; slug may differ
- Short structured prompt (~200 tokens in, ~80 tokens out)
- Response: `{"model": "<id>", "reason": "<short>", "tier": "pro|balanced|fast"}`

Fallback chain if dispatcher fails or times out (2s timeout on dispatcher):
1. **DeepSeek VR Pro** via Ollama cloud API — uses existing `OllamaClient` with model `deepseek-vr-pro` at `OLLAMA_BASE_URL`
   - TODO: confirm exact model name in Ollama's catalog — likely `deepseek-vr-pro` or `deepseek-v3-pro`
2. **Pure catalog selection** (Tier A algorithmic) — zero latency, always available

```go
type AutoSelector struct {
    catalog         []CatalogModel
    dispatcherModel string        // "openai/codex-mini-latest" or confirmed slug
    fallbackOllama  *OllamaClient // DeepSeek VR Pro on Ollama cloud
    httpClient      *http.Client
}

func (s *AutoSelector) Select(ctx context.Context, req *AIRequest) (string, error) {
    // 1. Try LLM dispatcher (2s deadline)
    // 2. Fallback to DeepSeek VR Pro on Ollama
    // 3. Fallback to catalog score
}
```

### 4. `backend/internal/ai/types.go` — add new constant

```go
ProviderOpenRouter AIProvider = "openrouter"
```

Update `ParseProvider` and `GetConfiguredProviders` to include `ProviderOpenRouter`.

### 5. `backend/internal/ai/router.go` — wire OpenRouter

In `NewAIRouter`, add a new `openRouterKey` parameter (or read from env directly). Wire `ProviderOpenRouter`:

```go
if openRouterKey != "" {
    clients[ProviderOpenRouter] = NewOpenRouterClient(openRouterKey, ollamaClient)
}
```

Pass the existing `OllamaClient` into the OpenRouter client so the dispatcher's DeepSeek fallback reuses the same connection pool and API key.

Update `DefaultRouterConfig` to include `ProviderOpenRouter` in fallback chains and load balancing. Suggested load balancing addition:
```
ProviderOpenRouter: 0.35  // becomes primary non-claude workhorse
```

Update `GetConfiguredProviders` preferred order to include `ProviderOpenRouter` between Ollama and Claude.

### 6. `backend/cmd/main.go` — env var + wiring

Add env var: `OPENROUTER_API_KEY`

```go
openRouterKey := getEnv("OPENROUTER_API_KEY", "")
```

Pass to `NewAIRouter`. Log whether OpenRouter is enabled at startup.

### 7. `backend/internal/pricing/` — cost estimates for OpenRouter

Add OpenRouter model costs to the pricing engine so `estimateRequestCostForProvider` works correctly for `ProviderOpenRouter`. The catalog data feeds this directly.

---

## New API Endpoint

### `GET /api/ai/openrouter/models`

Returns the merged model list (OpenRouter live list + internal quality scores) for the frontend model picker.

Response:
```json
{
  "models": [
    {
      "id": "deepseek/deepseek-r1",
      "display_name": "DeepSeek R1",
      "capability_scores": { "code_generation": 0.89, "architecture": 0.88 },
      "cost_per_1m_input": 0.55,
      "cost_per_1m_output": 2.19,
      "speed_rating": 0.65
    }
  ]
}
```

---

## Frontend Changes

### 1. Auto Mode Button

In `AppBuilder` / build trigger UI, add **Auto** alongside the existing Balanced / Max power mode buttons.

When Auto is selected:
- Sends `power_mode: "auto"` and `provider: "openrouter"` with `model: "auto"` in the build request
- Backend dispatcher picks model per-phase (architect gets best reasoning model, coder gets best code model, etc.)

### 2. Build Transparency

In the build progress UI, show the selected model per phase:
```
Architect  →  DeepSeek R1   ($0.0004 est.)
Coder      →  GPT-4o        ($0.0018 est.)
Tester     →  Gemini Flash  ($0.0001 est.)
```

Pull from build telemetry/response metadata.

### 3. Manual OpenRouter Model Picker (per agent position)

A dedicated **OpenRouter** button appears in `ModelRoleConfig.tsx` for each of the four agent roles (Architect, Coder, Tester, DevOps). Clicking it opens a full model browser modal distinct from the Auto flow.

**UX flow:**
```
[Claude] [ChatGPT] [Gemini] [Grok] [Ollama] [OpenRouter ▾] [Auto ✦]
                                              ↓ click
                                   ┌─────────────────────────┐
                                   │ 🔍 Search models...      │
                                   │                          │
                                   │ Filter: [All ▾] [Sort ▾] │
                                   │ ─────────────────────── │
                                   │ ⭐ deepseek/deepseek-r1  │
                                   │    DeepSeek R1           │
                                   │    Code 89% · $0.55/1M   │
                                   │                          │
                                   │    openai/gpt-4o         │
                                   │    GPT-4o                │
                                   │    Code 92% · $2.50/1M   │
                                   │                          │
                                   │    google/gemini-2.5-pro │
                                   │    Gemini 2.5 Pro        │
                                   │    Reasoning 93% · $1.25 │
                                   │  ... (all OR models)     │
                                   └─────────────────────────┘
```

**Model list source:** `GET /api/ai/openrouter/models` — live from OpenRouter + merged quality scores. Falls back to internal catalog if the API is unreachable.

**Per-role state:** Each role stores `{ provider: "openrouter", model: "<slug>" }` independently. Architect can run `deepseek/deepseek-r1` while Coder runs `openai/gpt-4o` and Tester runs `google/gemini-2.5-flash`.

**Search/filter capabilities:**
- Full-text search on model name, provider org, description
- Filter by: category (All, Coding, Reasoning, Fast/Cheap, Multimodal)
- Sort by: Quality score, Cost (low→high), Speed, Newest
- Badge indicators: free tier available, supports streaming, context window size

**Persistence:** User's per-role OpenRouter selections saved to project settings (same storage as existing `ModelRoleConfig` assignments). Persists across sessions.

**In-build display:** When a manually pinned OpenRouter model is used, the build UI shows the exact model slug and live cost tally, same as Auto mode transparency.

**Backend:** No new backend logic needed for this feature beyond the existing OpenRouter client + `GET /api/ai/openrouter/models` endpoint. The frontend simply sends `provider: "openrouter", model: "<user-picked-slug>"` in the build request, and the OpenRouter client routes it directly without going through the Auto dispatcher.

### 4. `ModelRoleConfig.tsx` — updated provider strip

The provider row per role becomes:
```
[Claude] [ChatGPT] [Gemini] [Grok] [Ollama] [OpenRouter ▾] [Auto ✦]
```
- `[OpenRouter ▾]` — opens the manual model picker modal described above; shows the currently pinned model slug as a subtitle when one is selected
- `[Auto ✦]` — sends `model: "auto"` and lets the dispatcher decide per-phase; no manual model needed

Both are mutually exclusive with the other provider buttons per role.

---

## Free Model Tier

OpenRouter exposes a large set of completely free models (`:free` suffix, rate-limited but no cost). This unlocks several things immediately.

### Free models available (representative sample)
| Model | Best for |
|-------|----------|
| `google/gemini-2.0-flash-exp:free` | Fast general tasks, testing |
| `meta-llama/llama-3.1-8b-instruct:free` | Light coding, explanations |
| `meta-llama/llama-3.3-70b-instruct:free` | Stronger coding, reasoning |
| `nousresearch/hermes-3-llama-3.1-405b:free` | Complex tasks |
| `mistralai/mistral-7b-instruct:free` | Fast, cheap equivalent tasks |
| `qwen/qwen-2.5-coder-7b-instruct:free` | Code-specific, free |
| `deepseek/deepseek-r1:free` | Reasoning, free tier |

> Check `https://openrouter.ai/models?max_price=0` for the live list — it changes as providers add/remove free quotas.

### How this is used

**1. Development & canary testing**
- All CI canary builds (`scripts/run_live_golden_build.mjs`) can run against free models with zero spend
- New flag: `OPENROUTER_TEST_MODEL=google/gemini-2.0-flash-exp:free` — when set, the OpenRouter client uses this model for all requests regardless of what was selected
- Lets us validate the entire OpenRouter pipeline (client, selector, routing, build phases, transparency UI) end-to-end before touching paid quota

**2. Free user tier upgrade**
- Currently free users are limited to Ollama (local/cloud)
- With OpenRouter free models, free users get access to significantly better models at zero platform cost
- Billing guard: `openrouter_catalog.go` tags each model with `IsFree bool`; the billing layer allows free-tagged models regardless of subscription tier
- Free models are rate-limited by OpenRouter (typically ~20 req/min per model) — the existing rate limiter handles this

**3. Manual picker "Free only" filter**
- The model browser modal gets a **Free** toggle filter
- When active, shows only `:free` models — useful for users watching spend or experimenting
- Free models shown with a `FREE` badge in the picker list

**4. Auto mode cost floor**
- The dispatcher can be configured with a `MaxCostPerBuild` cap (per-user from subscription tier)
- For free tier: cap is $0.00 → dispatcher constrained to `:free` models only
- For paid tiers: cap is configurable, dispatcher optimizes within it

### Catalog change for `openrouter_catalog.go`

```go
type CatalogModel struct {
    // ... existing fields ...
    IsFree bool  // true for :free suffix models — allowed on free subscription tier
}
```

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENROUTER_API_KEY` | OpenRouter API key — enables `ProviderOpenRouter` |
| `OLLAMA_API_KEY` | Existing — also used by dispatcher's DeepSeek VR Pro fallback |
| `OLLAMA_BASE_URL` | Existing — Ollama cloud URL |

Claude (`ANTHROPIC_API_KEY`) unchanged. No new keys needed for the dispatcher — it calls through OpenRouter using `OPENROUTER_API_KEY`.

---

## Implementation Order

1. `openrouter_catalog.go` — model data, no dependencies
2. `openrouter_selector.go` — Tier A algorithmic selection only first
3. `openrouter.go` — client without dispatcher first (model must be explicit)
4. Wire into `types.go`, `router.go`, `cmd/main.go`
5. Add dispatcher to `openrouter_selector.go` (Tier B) once client works
6. Pricing engine update
7. `GET /api/ai/openrouter/models` endpoint
8. Frontend: `GET /api/ai/openrouter/models` handler + caching (5min TTL)
9. Frontend: Auto button in power mode strip
10. Frontend: Manual OpenRouter model picker modal (search, filter, sort)
11. Frontend: `ModelRoleConfig.tsx` — add OpenRouter + Auto buttons to provider strip
12. Frontend: build transparency (model slug + cost per phase in build UI)

---

## Pre-Deploy Checklist

- [ ] Verify ChatGPT 5.5 / Codex 5.5 exact model slug on `https://openrouter.ai/api/v1/models`
- [ ] Verify DeepSeek VR Pro model name in Ollama cloud catalog
- [ ] Confirm all `anthropic/*` models are excluded from OpenRouter routing
- [ ] Set `OPENROUTER_API_KEY` in Render environment variables
- [ ] Run `GET /api/ai/openrouter/models` against live key and confirm catalog slugs match
- [ ] Test auto-selection end-to-end with a canary build

---

## What This Does NOT Change

- Claude client (`claude.go`) — untouched, stays direct Anthropic API
- Ollama client (`ollama.go`) — untouched, used as dispatcher fallback
- Existing BYOK flows — unchanged, users with own keys still use existing slots
- Existing provider slots (gpt4, gemini, grok) — still work; OpenRouter is additive
