# APEX.BUILD Pipeline Testing Instructions for Claude Code

## CONTEXT

You are working on APEX.BUILD, a multi-AI cloud development platform (Replit competitor). The codebase is a Go backend (`backend/`) and React frontend (`frontend/`). The repo was just cloned from:
```
https://github.com/spencerandtheteagues/apex-build-platform.git
```

**The goal: Get builds completing successfully end-to-end using Ollama local models. All cloud API keys (OpenAI, Anthropic, Gemini, Grok) are intentionally blank — we have no credits. Ollama is the only AI provider available on this machine.**

---

## SETUP BEFORE TESTING

1. Ensure Ollama is running:
```bash
ollama serve &
```

2. Verify models are available:
```bash
ollama list
```
The codebase expects these models (in `backend/internal/ai/ollama.go:221-231`):
- `deepseek-r1:18b` — used for code generation, refactoring, architecture, debugging, code review (default for most capabilities)
- `qwen3-coder:30b` — used for code completion

If these exact models aren't installed, either pull them or note which models ARE installed and update the `getModel()` function in `backend/internal/ai/ollama.go` to use what's available. Any decent coding model works (e.g., `codellama`, `deepseek-coder`, `qwen2.5-coder`, etc).

3. Configure `.env` in the project root:
```bash
cp .env.example .env
```
Then set:
```
OLLAMA_URL=http://localhost:11434
ANTHROPIC_API_KEY=
OPENAI_API_KEY=
GOOGLE_AI_API_KEY=
XAI_API_KEY=
```
The `OLLAMA_URL` env var was just wired into `backend/main.go` in the latest commit. Verify the `initAI()` function reads it and passes it as the second extra key to `ai.NewAIRouter()`.

4. Ensure PostgreSQL is running and the database exists:
```
DATABASE_URL=postgresql://postgres:apex_build_2024@localhost:5432/apex_build?sslmode=disable
```

5. Ensure Docker is installed and running (needed for code execution/preview sandboxing).

---

## WHAT THE BUILD PIPELINE DOES

The build system is an agent orchestration engine. When a user submits a build request:

1. `backend/cmd/main.go` — **This is the real entry point** (NOT `backend/main.go` which is a legacy/unused file). It initializes the `AgentManager`.
2. `AgentManager.CreateBuild()` (`backend/internal/agents/manager.go:137`) — Creates build session
3. `AgentManager.StartBuild()` (`manager.go:202`) — Spawns a lead agent, creates a planning task
4. `taskDispatcher()` (`manager.go:1397`) — Picks up tasks from queue, runs them via `executeTask()`
5. `executeTask()` (`manager.go:1408`) — Calls `aiRouter.Generate()` with the agent's provider, then `parseTaskOutput()` to extract files from AI response
6. `resultProcessor()` (`manager.go:1735`) → `processResult()` — Handles success/failure, runs build verification, triggers next tasks or retries
7. `handleTaskCompletion()` — Spawns next agents (Architect, Frontend, Backend, Testing, etc.) and queues their tasks
8. Build completes when all tasks finish successfully

**The build handler and WebSocket hub are initialized at `cmd/main.go:209`:**
```go
wsHub := agents.NewWSHub(agentManager)
buildHandler := agents.NewBuildHandler(agentManager, wsHub)
```

---

## KNOWN ISSUES TO INVESTIGATE AND FIX

### Issue 1: Builds were failing around 80% completion
Previously caused by exhausted cloud API credits. With Ollama this shouldn't happen since it's free/local, but watch for:
- Ollama connection failures (timeout, unreachable)
- Model not found errors (model name mismatch)
- Empty responses from smaller models
- The 5-minute per-task timeout at `manager.go:1562` — local models can be slow

### Issue 2: OptimizedAgentManager has a gutted file parser
`backend/internal/agents/manager_optimized.go:738-743` has this:
```go
func (am *OptimizedAgentManager) parseTaskOutputFromResponse(taskType TaskType, response string) *TaskOutput {
    return &TaskOutput{
        Messages: []string{response},
        Files:    []GeneratedFile{},
    }
}
```
This returns ZERO files — it never parses anything. The `OptimizedAgentManager` is NOT currently wired up (the active path uses `AgentManager`), but if it's intended as a replacement, its parser needs to be fixed to match the real parser at `manager.go:5905-6052`.

### Issue 3: Ollama model output quality
Smaller Ollama models (8B-18B params) are LESS reliable than flagship cloud models at:
- Following the `// File: path/to/file.ts` marker format that the parser expects (`manager.go:5922-5974`)
- Properly closing markdown code fences (``` delimiters)
- Generating multiple well-structured files per response
- Following complex system prompt instructions faithfully
- Producing complete code without truncation (shorter context windows)

**The parser has fallbacks** — unnamed code blocks get auto-named (`generated_1.ts`), and if no files are extracted at all, the whole response is treated as a single file if it looks like code (`manager.go:6034-6048`). So builds won't crash, but file structure may be messier than with flagship models.

**This is expected and acceptable for pipeline testing.** We're testing that the pipeline mechanics work (task scheduling, WebSocket updates, file extraction, artifact storage, preview, deployment), not output quality. When we switch to flagship models for production, code quality improves with zero code changes.

### Issue 4: Build verification retry storms
After code generation, `verifyGeneratedCode()` runs at `manager.go:1826`. If Ollama produces code with syntax errors or missing imports, verification will fail and trigger retries. With smaller models this happens more often. Watch the retry counts — if builds burn through all retries on verification failures, you may need to:
- Relax verification for testing
- Increase `MaxRetries`
- Or improve the system prompts to be more explicit about output format

---

## TESTING PROCEDURE

### Step 1: Verify the backend compiles and starts
```bash
cd backend
go build ./cmd/main.go
# or
go run ./cmd/main.go
```
Watch the startup logs for:
- `✅ Ollama configured at http://localhost:11434`
- `Agent Orchestration System initialized`
- No errors about missing providers

### Step 2: Verify Ollama health check passes
The router runs health checks every 30 seconds (`backend/internal/ai/router.go:397`). Check logs for:
- `Health check passed for provider ollama`

If health check fails, verify Ollama is running and the URL is correct.

### Step 3: Start the frontend
```bash
cd frontend
npm install
npm run dev
```

### Step 4: Trigger a test build
Through the UI or via API:
```bash
curl -X POST http://localhost:8080/api/v1/agents/builds \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "description": "Create a simple hello world page with HTML, CSS, and JavaScript",
    "mode": "full",
    "power_mode": "fast"
  }'
```
Start with something simple. A "hello world" page is the minimum viable test — it should produce 2-3 files max, which smaller models handle well.

### Step 5: Monitor the build
Watch the backend logs for the full lifecycle:
- `StartBuild called for build <id>`
- `Spawning lead agent with provider: ollama`
- `Calling AI router for task <id> with provider ollama`
- `AI generation succeeded for task <id> (response_length: N)`
- File extraction results
- Build completion or failure

### Step 6: If builds fail, diagnose and fix
Common failure modes:
1. **"No AI providers available"** — Ollama URL not configured or Ollama not running
2. **"failed to connect to Ollama server"** — Network/port issue
3. **"MODEL_NOT_FOUND"** — The model name in `getModel()` doesn't match installed models
4. **Empty response** — Model loaded but failed to generate (OOM, timeout)
5. **Parser extracts 0 files** — Model didn't follow the `// File:` format. Check if fallbacks caught it.
6. **Build verification failed** — Generated code has errors. Check retry behavior.
7. **Context deadline exceeded** — 5-minute timeout hit. Increase if needed for slow models.

### Step 7: Scale up complexity
Once hello world works, try progressively harder builds:
- "Create a todo list app with React and local storage"
- "Build a REST API with Express and a React frontend"
- "Create a full-stack app with authentication"

This will stress-test multi-agent coordination, multi-file generation, and the full pipeline.

---

## KEY FILES REFERENCE

| File | Purpose |
|------|---------|
| `backend/cmd/main.go` | **Real entry point** — initializes everything |
| `backend/internal/agents/manager.go` | Agent orchestration, build lifecycle, task execution |
| `backend/internal/agents/types.go` | All type definitions (Build, Agent, Task, etc.) |
| `backend/internal/agents/handlers.go` | HTTP handlers for build API |
| `backend/internal/agents/ai_adapter.go` | Bridges AgentManager ↔ AIRouter |
| `backend/internal/ai/router.go` | AI provider routing with fallbacks |
| `backend/internal/ai/ollama.go` | Ollama client implementation |
| `backend/internal/ai/types.go` | AIClient interface, provider config, defaults |
| `backend/internal/hosting/service.go` | Native .apex.app deployment |
| `backend/internal/execution/` | Code execution (Docker sandbox) |
| `backend/internal/preview/` | Live preview server |
| `backend/main.go` | **Legacy/unused** — NOT the real entry point |

---

## IMPORTANT NOTES

- **Do NOT add cloud API keys.** We're testing with Ollama only. All cloud providers should remain unconfigured.
- **The DefaultRouterConfig in `backend/internal/ai/types.go:144-158` already defaults all capabilities to ProviderOllama.** This is correct for our testing scenario.
- **The build provider selection at `manager.go:280-305` prioritizes Ollama** when available. This is also correct.
- **Fix any issues you find.** If something in the pipeline is broken, fix it. The goal is builds completing end-to-end, not just identifying problems.
- **Commit fixes as you go** with descriptive messages so we can track what changed.
- **When builds complete successfully with simple projects, document what works and what the remaining limitations are.**
