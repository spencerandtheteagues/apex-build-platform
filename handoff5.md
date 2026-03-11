# Handoff 5

Date: 2026-03-11
Repo: `/home/s/projects/apex-build-platform`
Branch: `main`
Current pushed HEAD: `2b5c1f2` (`Harden builder scaffolds and IDE preview flows`)

This handoff is for the next model to continue the work without needing to rediscover what has already been fixed, what is still broken, and what the intended architecture should be.

## Executive Summary

The platform is materially more stable than it was at the start of this session.

The biggest builder failure that was actually happening in production was not “the models are bad” but a coordination and artifact-handling failure:

- large multi-file model outputs were truncating
- truncation recovery sometimes repaired the files
- stale parser warnings still caused verification to fail
- the system had no frozen shared build contract, so different agents could drift into different versions of the same app
- fast and balanced mode still left too much infrastructure invention to the models

Those core issues have now been partially addressed and pushed.

Separately, the IDE and preview flow are improved:

- the IDE can open even without a recovered/current project
- the IDE chrome is cleaner and more readable
- preview no longer hard-fails when server Docker is unavailable

However, the real “use the user’s own Docker on their own machine” path is not implemented yet. I was in the design/discovery phase for that when this handoff was requested. The correct implementation is a reverse-tunnel local bridge, not a naive localhost iframe hack.

## What Has Already Been Fixed

## 1. Truncated build recovery bug

Problem:

- the builder could receive a large multi-file response
- a file would be cut off mid-code-block
- `parseTaskOutput` would record unterminated/truncation warnings
- `completeTruncatedFiles` would sometimes successfully append the missing continuation
- even after a successful continuation, stale parser warnings could still fail readiness verification
- result: build died around 90-95% even when the underlying artifact was salvageable

Why it mattered:

- this was the concrete reason the TranscriptVault build failed
- the failure looked like “AI couldn’t finish” but the system itself was treating recovered output as still-broken

Fix status:

- fixed and pushed previously in commit `001d92b`

Behavioral effect:

- unresolved truncations still fail
- successfully repaired truncations stop poisoning later readiness checks

Important note:

- this fix removed one major false-negative verification path
- it does not by itself solve architecture drift or poor multi-agent coordination

## 2. Production DB log noise

Problem:

- production was logging GORM queries too aggressively
- Render logs were getting buried in metrics/health SQL noise
- important websocket/build logs were harder to find

Why it mattered:

- when diagnosing IDE and preview failures, the useful logs were effectively hidden

Fix status:

- fixed and pushed previously in commit `001d92b`

Behavioral effect:

- production/staging log output is quieter and more useful

## 3. Frozen BuildSpec and work-order path

Problem:

- planning existed, but the downstream system was still effectively prompt-only
- agents were still largely working from the raw user description instead of a frozen, shared contract
- there was no real spec hash, ownership contract, API contract, acceptance checklist, or role-specific required outputs

Why it mattered:

- different agents could build different apps
- architecture drift was not just possible, it was structurally encouraged
- fast and balanced mode were too dependent on prompt discipline

Fix status:

- implemented and pushed in commit `2b5c1f2`

Key additions:

- structured planning bundle promoted into the main planning path
- `BuildPlan` now carries:
  - `spec_hash`
  - `scaffold_id`
  - `ownership`
  - `env_vars`
  - `acceptance`
  - `work_orders`
  - `api_contract`
  - `preflight`
- task outputs now support:
  - `Plan`
  - `TaskStartAck`
  - `TaskCompletionReport`

Important files:

- `backend/internal/agents/autonomous/planner.go`
- `backend/internal/agents/types.go`
- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/planning_contracts.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/build_spec_test.go`

Behavioral effect:

- plan creation is now structured
- specialists receive a frozen build context and role-specific work order
- start and completion check-ins are parsed and validated
- out-of-scope file output can be rejected and retried

## 4. Deterministic scaffold bootstrap for fast and balanced modes

Problem:

- even after freezing a plan, the system was still effectively asking the models to invent the repo scaffold from scratch
- this is especially brittle for weaker or fast-tier models
- shared root files like `package.json` and `tsconfig.json` could also be incorrectly treated as forbidden because of overlapping ownership rules

Why it mattered:

- the most failure-prone part of the build was still base infra generation
- smaller models were spending tokens on boilerplate instead of product logic
- shared root manifests were vulnerable to ownership confusion

Fix status:

- implemented and pushed in commit `2b5c1f2`

What changed:

- `BuildPlan` now includes `ScaffoldFiles []GeneratedFile`
- deterministic starter files are generated for supported scaffolds
- `handlePlanCompletion` now bootstraps those files into the build before specialists start
- work orders now include `required_files`
- prompts now include the agent’s existing owned scaffold files so agents modify a real repo state instead of hallucinating one
- reviewer/solver `**` ownership no longer poisons every specialist’s forbidden list
- shared files are no longer simultaneously “owned” and “forbidden”

Current deterministic scaffold coverage:

- `fullstack/react-vite-express-ts`
- `frontend/react-vite-spa`
- `api/go-http`
- `api/express-typescript`

What these scaffolds now preload:

- root manifests
- basic TS config
- Vite config
- Tailwind/PostCSS where appropriate
- root HTML entry
- minimal frontend shell
- minimal backend entry and health route
- `.env.example`
- `README.md`

Behavioral effect:

- fast and balanced builds now start from a real repo skeleton
- models are more likely to converge on the same app structure
- specialists operate on owned scaffold files instead of inventing new layout

## 5. IDE opening without current project

Problem:

- the IDE shell effectively required a current project or recovered build files to mount cleanly
- if there was no current project, the IDE could fail to open in a useful way

Why it mattered:

- the IDE should be a shell that is always openable, not a fragile projection of current build state

Fix status:

- implemented locally earlier and included in pushed commit `2b5c1f2`

Files involved:

- `frontend/src/App.tsx`
- `frontend/src/components/builder/AppBuilder.tsx`
- `frontend/src/components/ide/IDELayout.tsx`
- `frontend/src/components/project/ProjectList.tsx`

Behavioral effect:

- the IDE view can mount even without an active project
- project create/select now consistently sets current project state

## 6. IDE readability cleanup

Problem:

- the IDE chrome had low-contrast, overly small, or visually messy controls

Why it mattered:

- the IDE felt noisy and some controls were hard to read

Fix status:

- implemented locally earlier and included in pushed commit `2b5c1f2`

Main file:

- `frontend/src/components/ide/IDELayout.tsx`

Behavioral effect:

- higher-contrast button styles
- cleaner top bar
- clearer panel tabs
- more readable icon/button affordances

## 7. Preview fallback when server Docker is missing

Problem:

- preview could fail hard if server-side Docker was unavailable
- the platform could become unusable in environments where container preview was expected but not present

Why it mattered:

- users still need a preview even when platform Docker is unavailable

Fix status:

- implemented locally earlier and included in pushed commit `2b5c1f2`

Files involved:

- `backend/internal/handlers/preview.go`
- `backend/internal/handlers/preview_test.go`
- `backend/internal/preview/container_preview.go`
- `frontend/src/components/preview/LivePreview.tsx`

Behavioral effect:

- when secure sandbox is required but server Docker is unavailable, preview can degrade to process mode instead of hard-failing
- UI surfaces degraded sandbox state
- backend preview can still run in degraded mode where allowed
- Docker host environment is honored more consistently in container preview commands

Important limitation:

- this is not user-local Docker
- this is still server-side fallback behavior

## Commits That Matter

## Already pushed before this handoff

- `001d92b` `Fix truncated build recovery and quiet prod DB logs`
- `2b5c1f2` `Harden builder scaffolds and IDE preview flows`

## Remote commits that landed before my last push

These were already on `origin/main` before `2b5c1f2` was rebased and pushed:

- `cf291f0` `Block failed build snapshot downloads`
- `6b17fc3` `Route orchestrator completion through readiness validation`
- `149154d` `Align router provider ids and exact model labels`
- `3bd31d1` `Update docs and metadata for locked AI tiers`
- `a2d377e` `Align frontend model labels with locked AI tiers`
- `f7c8173` `Fix spend dashboard data flow and build warnings`
- `98b3042` `Expand builder provider telemetry panels`

The local repo was rebased cleanly before push. `HEAD` and `origin/main` matched at the end of the push flow.

## Verification Already Run

These were run successfully after the hardening work:

- `cd backend && go test ./internal/agents/...`
- `cd backend && go test ./...`
- `cd frontend && npm run typecheck`

The pushed hardening/scaffold/preview changes were verified before push.

## What I Was Working On When This Handoff Was Requested

I was actively designing the real user-local Docker preview path.

This is the correct next major platform task because the current system still has a structural limitation:

- preview proxy assumes the running preview lives on the server
- even with better fallback behavior, the hosted platform cannot currently use Docker on the user’s own machine

The user explicitly wants:

- preview to work even if platform Docker is missing
- the platform to be able to use local system resources on the user’s PC, with permission
- a seamless experience

## Critical architectural conclusion

Do not implement this as a naive localhost iframe solution.

Why:

- if the app is served over HTTPS, embedding `http://127.0.0.1:PORT` content is mixed-content hostile and generally unreliable
- the hosted backend cannot reverse proxy to the user’s localhost
- a plain localhost-only helper does not let the platform stay in control of routing and auth

The correct design is a reverse-tunnel local bridge:

- the user runs a local bridge daemon on their machine
- the browser asks the platform to prepare a local preview session
- the platform issues a short-lived scoped token
- the browser instructs the local bridge on `127.0.0.1` to connect outward to the platform using that token
- the bridge opens a WebSocket connection to the platform
- the platform sends commands to the bridge
- the bridge:
  - downloads or receives the project archive
  - materializes files locally
  - uses local Docker
  - proxies local HTTP responses back through the tunnel
- the browser continues talking to the hosted platform preview proxy URL
- the platform proxy routes requests through the connected local bridge instead of assuming `127.0.0.1` on the server

This keeps the product model clean:

- the hosted app remains the stable origin
- local resources are explicitly permissioned
- the user’s Docker is usable
- the preview pane continues to work from the hosted IDE

## Existing code that should be reused for local bridge work

Explorer findings already completed:

- `backend/internal/handlers/files.go`
  - `(*Handler).DownloadProject`
  - streams a project from `models.File` rows into ZIP

- `backend/internal/api/handlers.go`
  - `(*Server).DownloadProject`
  - older parallel ZIP export path

- `backend/internal/deploy/builder.go`
  - `(*BuildService).PackageProject`
  - `(*BuildService).CreateZipArchive`
  - `(*BuildService).CreateBase64Archive`
  - useful if an in-memory archive payload is needed

- `backend/internal/preview/server_runner.go`
  - `(*ServerRunner).writeProjectFiles`
  - best existing helper for materializing a project onto disk safely

- `backend/internal/preview/container_preview.go`
  - `(*ContainerPreviewServer).StartContainerPreview`
  - current full flow from DB files -> temp directory -> generated Dockerfile -> build/run
  - also includes a tar helper near the end of the file

- `backend/internal/agents/handlers.go`
  - `(*BuildHandler).DownloadCompletedBuild`
  - useful reference for artifact ZIP delivery

- `backend/pkg/models/models.go`
  - `models.File`
  - canonical project file object

## Current local bridge feature status

Nothing has been implemented yet for the actual bridge runtime.

I was still in discovery/planning.

There are no uncommitted code changes for bridge functionality that need recovery.

## What Still Needs To Be Fixed

## Immediate next feature: user-local Docker bridge

This is the highest-value remaining preview/runtime task.

### Backend changes required

Add a new bridge subsystem under something like:

- `backend/internal/preview/local_bridge.go`
- `backend/internal/preview/local_bridge_session.go`
- `backend/internal/preview/local_bridge_proxy.go`

Potential responsibilities:

- maintain connected local bridge sessions keyed by user/device/project
- issue short-lived bridge-scoped tokens
- expose session prep endpoint
- expose archive download endpoint scoped by bridge token
- route preview proxy traffic through connected bridge instead of assuming server-local `127.0.0.1`
- optionally route backend proxy traffic through bridge as well

Suggested backend endpoints:

- `POST /api/v1/preview/local/session`
  - create or refresh a local bridge preview session
  - return:
    - bridge token
    - websocket URL
    - session ID
    - archive URL
    - desired runtime mode
    - project metadata

- `GET /api/v1/preview/local/status/:projectId`
  - whether a local bridge is connected for that project/user
  - device name/version/capabilities if known

- `GET /api/v1/preview/local/archive/:sessionId`
  - download project ZIP or tarball using bridge token

- `GET /api/v1/preview/local/bridge/ws`
  - bridge websocket

Existing preview endpoints that must become bridge-aware:

- `GetPreviewStatus`
- `GetPreviewURL`
- `ProxyPreview`
- `ProxyBackend`
- `StartPreview`
- `StartFullStackPreview`
- `StopPreview`

Do not special-case bridge behavior only in the frontend.

The backend needs a real runtime mode and a session registry so proxying logic stays authoritative.

### Authentication for the bridge

Do not put full user bearer tokens into the local daemon.

Instead:

- add a short-lived bridge token type, or reuse/extend preview-scoped tokens carefully
- scope it to:
  - user
  - project
  - session
  - expiry
  - allowed actions

Best direction:

- add dedicated bridge claims in `backend/internal/auth/auth.go`
- similar to `PreviewTokenClaims`, but explicitly for bridge connect/archive access

Examples:

- `BridgeTokenClaims`
  - `user_id`
  - `project_id`
  - `session_id`
  - `device_id`
  - `permissions`

### Bridge transport

Use WebSocket.

Why:

- already supported in repo
- good for request/response multiplexing
- works for control messages and HTTP proxy traffic

Suggested message types:

- `bridge_hello`
- `bridge_ready`
- `start_preview`
- `stop_preview`
- `status_request`
- `status_response`
- `proxy_http_request`
- `proxy_http_response`
- `proxy_http_chunk`
- `proxy_http_error`
- `bridge_log`
- `heartbeat`

The bridge should support concurrent proxied requests with request IDs.

### Archive transport

Preferred pattern:

- platform creates bridge session
- platform returns a short-lived archive URL
- local bridge downloads the archive directly from the platform

Do not inline large project archives into websocket messages.

Reuse candidates:

- ZIP streaming logic from `DownloadProject`
- archive helpers from `internal/deploy/builder.go`

### Project materialization

Best reuse direction:

- factor or reuse `ServerRunner.writeProjectFiles`

This is safer than open-coding file writes because it already deals with path normalization.

### Local execution model

The local bridge should support at least:

- local Docker container preview for frontend/static preview workloads
- local backend process launch or local container launch for full-stack preview

Minimum viable practical approach:

- start with frontend preview via local Docker
- optionally start backend via process runner first
- add backend-in-Docker after the tunnel path is stable

But if implemented carefully, both can be supported in v1.

### Proxy path behavior

When local runtime is active:

- `ProxyPreview` should not reverse proxy to server `127.0.0.1:status.Port`
- it should package the incoming HTTP request into a bridge message
- wait for bridge response
- write headers/status/body back to the browser

When local backend runtime is active:

- `ProxyBackend` should do the same through the bridge

This is the critical difference between a real local bridge and a fake local-only helper.

## Frontend changes required

Main UI surface:

- `frontend/src/components/preview/LivePreview.tsx`

This is the primary place to add:

- runtime selection
- bridge detection
- bridge connection state
- user permission state
- local Docker readiness state

Recommended runtime choices:

- `Platform`
- `Platform Docker`
- `Local Docker`

Display requirements:

- current runtime
- connected/disconnected bridge
- whether local Docker is available
- whether secure sandbox is degraded
- concise explanation when falling back

There is already permission UI in:

- `frontend/src/components/builder/AppBuilder.tsx`

Relevant existing capabilities:

- pending permission request display
- granted permission rule display
- approve once / approve for build / deny

This should be reused conceptually, but preview runtime selection probably belongs in `LivePreview` while build-agent local resource permissions remain in `AppBuilder`.

Potential UI additions:

- “Connect Local Bridge” button
- “Use Local Docker” toggle or runtime segmented control
- “Bridge not installed / not reachable” state
- “Open setup instructions” link
- bridge capability badge:
  - connected
  - docker ready
  - version
  - device name

Additional likely files:

- `frontend/src/services/api.ts`
  - add local bridge/session endpoints
- possibly a small utility under `frontend/src/lib` or `frontend/src/services`
  - localhost bridge handshake calls

### Exact recommended UI insertion points

These came from targeted repo inspection and should be treated as the preferred implementation map.

#### `LivePreview`

- Primary runtime selector:
  - add it in the preview toolbar around `LivePreview.tsx` near the current preview controls
  - this is the correct place for a compact segmented control or dropdown
  - do not hide the main runtime choice behind a tiny icon

- Detailed runtime settings:
  - add a `Runtime` section in the settings popover in `LivePreview.tsx`
  - list:
    - `Platform process`
    - `Platform Docker`
    - `Local Docker bridge`
  - also show read-only state:
    - bridge connected
    - local Docker available
    - permission granted/denied

- Runtime problem banner:
  - extend the current degraded banner area in `LivePreview.tsx`
  - use it for:
    - platform Docker unavailable
    - local bridge disconnected
    - local permission missing
    - selected runtime unavailable

- Blocking preflight/empty state:
  - use the empty preview state area in `LivePreview.tsx`
  - if the user selected `Local Docker bridge` but the bridge is not reachable or not approved, this should be the main explanatory/CTA surface

- Ongoing status:
  - show active runtime and bridge health in the preview status bar in `LivePreview.tsx`
  - this gives continuous visibility without adding more clutter to the toolbar

#### `IDELayout`

- Keep full runtime controls out of the dense top chrome
- Only show a small runtime badge or tooltip near:
  - the preview toggle in `frontend/src/components/ide/IDELayout.tsx`
  - the main `Run/Stop` control in the same file

- Use the right-panel settings area in `IDELayout.tsx` for persistent per-project preview defaults
  - e.g. preferred preview runtime
  - this should be preferences, not the main runtime control

- `IDELayout` should host the preview pane
- `LivePreview` should own runtime controls and runtime-specific status

#### `AppBuilder`

- Keep consent and pre-approval in `frontend/src/components/builder/AppBuilder.tsx`
- The correct surface is the existing `Build Controls` card
- The most natural exact spot is beside the current pre-approve local tools controls
- This should remain the authority for:
  - bridge authorization
  - local Docker approval
  - localhost approval
  - persistent build-level permission rules

- Also add a concise blocked/waiting badge in the build-state row when:
  - a build is waiting on local bridge permission
  - a build expects local runtime but bridge is not connected

### Placement verdict

- Runtime choice:
  - `LivePreview` toolbar + `LivePreview` settings

- Connection health:
  - `LivePreview` degraded banner + `LivePreview` status bar
  - optional light summary badge in `IDELayout` top chrome

- Permission / approval:
  - `AppBuilder` `Build Controls`

- Persistent default preferences:
  - `IDELayout` settings tab

## Local bridge daemon required

Add a separate command, likely:

- `backend/cmd/localbridge/main.go`

Reason:

- Go is already the backend language
- easy cross-platform single-binary distribution
- easy reuse of project materialization / Docker invocation code

Local bridge responsibilities:

- bind to localhost only
- expose small local HTTP API for the browser:
  - `GET /health`
  - `GET /capabilities`
  - `POST /connect`
  - `POST /disconnect`
- open outbound WebSocket to platform
- download project archive from platform
- unpack project locally
- build and run preview using local Docker
- optionally run backend process or backend container
- proxy local HTTP responses back through the tunnel

Potential local bridge capabilities payload:

- version
- OS
- arch
- docker available
- docker version
- running session count
- bridge connected
- active project IDs

Security rules:

- listen only on `127.0.0.1`
- reject non-local origins unless explicitly allowed
- never persist full user JWT
- only accept short-lived bridge session tokens
- clean temp dirs after stop
- enforce per-session workspace isolation

## What Should Be Fixed After the Local Bridge

Once the local bridge exists, the next priority is deep orchestration hardening so weak and strong models alike are more likely to succeed.

## Orchestration roadmap from here

The platform now has:

- structured plan
- scaffold ID
- spec hash
- ownership map
- work orders
- check-in validation
- deterministic scaffold bootstrap

That is good progress, but it is not the finished orchestration system.

The system still needs the following.

### A. BuildIntent and BuildSpec v2

Current state:

- `BuildPlan` is much better than before
- but it is still a builder-internal planning structure, not a fully explicit immutable artifact contract

Needed:

- promote a stricter `BuildIntent` and `BuildSpec v2`
- separate:
  - raw user request
  - normalized must-haves
  - scaffold choice
  - design envelope
  - runtime contract
  - verification contract

Why:

- this allows better replanning without losing user intent
- this gives future recovery loops something more stable than prompt prose

### B. Scaffold registry expansion

Current state:

- only a few archetypes are deterministic

Needed:

- add more scaffold families so fast/balanced mode is almost never starting from open-ended infra invention

At minimum:

- React + Vite + Express + Postgres
- Next.js + API
- Go API + SPA
- Python/FastAPI + SPA
- worker/queue projects
- auth/dashboard CRUD archetype
- SaaS landing + dashboard archetype

Important rule:

- scaffold must own boring infrastructure
- app/product identity must still vary

Variation should come from:

- design spec
- domain model
- routes
- workflow behavior
- content

### C. Role DAG and dependency gates

Current state:

- phase tasks exist, but they can still run with too much soft coordination

Needed:

- explicit DAG per build
- some tasks should not start until prerequisite artifacts are accepted

Example:

- frontend should not start if API contract is still inconsistent
- testing should not run until scaffold required files exist
- reviewer should validate role outputs against BuildSpec before solver loops

### D. Patch-first large-file editing

Current state:

- the platform still relies on full-file outputs for most generation tasks

Needed:

- for established files, shift to patch/replace-region protocol rather than full-file re-emission
- reserve full-file outputs for initial scaffold generation or very small files

Why:

- reduces truncation surface area
- reduces token waste
- makes solver loops much more precise

### E. Failure classifier

Current state:

- some recovery exists, but failure handling is still too generic

Needed:

- classify failures by type:
  - truncation
  - syntax
  - missing manifest
  - ownership violation
  - route mismatch
  - dependency mismatch
  - preview boot failure
  - backend start failure
  - bundle failure
  - provider failure

Then map to specific strategies:

- truncation -> split output / continuation
- ownership violation -> retry with stricter work order reminder
- missing manifest -> deterministic scaffold repair
- preview boot failure -> runtime-specific repair
- provider failure -> provider fallback only if engine state permits

### F. Engine-level intervention actions

Current state:

- user interventions are better than before, but provider/model changes are still vulnerable to conversational over-claiming unless fully engine-backed

Needed:

- interventions must produce validated state changes, not just lead-agent narration

Examples:

- provider switch
- power mode switch
- scaffold reset
- scope reduction
- restart from checkpoint
- runtime target switch

These should all be real structured actions with validation and audit trail.

### G. Per-role acceptance scoring

Current state:

- acceptance checks exist, but they are still primarily descriptive

Needed:

- each role should produce machine-checkable completion evidence where possible

Examples:

- frontend:
  - required scaffold files exist
  - API base wiring exists
  - root component renders
- backend:
  - health route exists
  - CORS exists
  - declared port contract is respected
- database:
  - migration/schema aligns to planned models
- testing:
  - core path validation completed

### H. Artifact lineage and provenance

Needed:

- track which agent created or last modified each file
- track which work order authorized it
- track which verification gates it passed

Why:

- this makes solver targeting better
- reviewer findings become more precise
- future UI can display “who changed what and why”

### I. Continuous evaluation harness

Needed:

- regression suite of representative builds across power modes:
  - fast
  - balanced
  - max
- score:
  - first-pass success
  - total retries
  - preview readiness
  - scaffold compliance
  - output truncation rate
  - ownership violation rate

This should become the main confidence measure for orchestration changes.

### J. Long-term memory for build heuristics

Eventually:

- store anonymized failure patterns and successful repair recipes
- let solver/reviewer use those heuristics as structured hints

Not as freeform prose memory.

As classified remediation strategies.

## Recommended Exact Next Implementation Order

The next model should work in this order.

### 1. Ship the local bridge backend session model

- create bridge session structs and registry
- add session prep endpoint
- add bridge token claims
- add bridge websocket endpoint

### 2. Add archive delivery

- reuse ZIP export path
- add bridge-scoped archive download

### 3. Build the local bridge daemon

- localhost health/capabilities/connect API
- outbound websocket to platform
- archive download/unpack
- Docker availability detection

### 4. Implement preview request tunneling

- route `ProxyPreview` through bridge when runtime mode is local
- then route `ProxyBackend` through bridge

### 5. Add frontend runtime selection and connection UX

- `LivePreview` runtime control
- local bridge detection
- connect/disconnect and status
- bridge capability badges

### 6. Add tests

- backend:
  - session auth
  - bridge registry
  - archive access control
  - proxy request/response tunnel behavior
- frontend:
  - runtime selector state
  - degraded vs connected UI

### 7. Only then continue deeper builder hardening

After local bridge is landed, continue with:

- broader scaffold coverage
- failure classifier
- patch-first edit protocol
- engine-level intervention actions

## Known Traps

These are the mistakes the next model should avoid.

### 1. Do not build a fake local runtime that only works from localhost dev

If it requires the browser to directly iframe `http://127.0.0.1`, it is not the right architecture for a hosted HTTPS product.

### 2. Do not hand the local bridge the full user JWT

Use short-lived scoped bridge tokens.

### 3. Do not inline large project archives into WebSocket control messages

Use a separate download endpoint.

### 4. Do not bolt local bridge state only into the frontend

The backend preview subsystem must know which runtime owns the session.

### 5. Do not collapse user-local Docker into the same semantics as degraded process fallback

These are different runtime modes:

- server process fallback
- server Docker
- user-local Docker

They should be visible and diagnosable separately.

### 6. Do not let the bridge silently take over without explicit user consent

This is local resource access.

It should be explicit in UI and tracked in permission state.

## Current Repo State At Handoff

Pushed code:

- yes

Untracked workspace-local files intentionally not pushed:

- `.openclaw/`
- `SOUL.md`
- `USER.md`
- `MEMORY.md`
- `memory/`
- other workspace metadata files

These should not be mixed into product commits.

There is also an untracked test file in the workspace:

- `tests/e2e/specs/ui-auth-build.spec.ts`

It was not part of the pushed product commit during this phase.

## If Claude Has Very Little Budget Left

If the next model has limited budget, the minimum useful continuation is:

1. Read this file.
2. Confirm `HEAD` is still `origin/main`.
3. Implement only the backend local bridge session/tunnel skeleton first.
4. Add a tiny local bridge daemon with:
   - `/health`
   - `/capabilities`
   - `/connect`
5. Add `LivePreview` UI for:
   - bridge detection
   - local runtime selection
   - connection state
6. Leave full HTTP proxy tunneling for the next pass if necessary.

But the target architecture should still be the reverse-tunnel bridge described above.

## Final Assessment

The platform is no longer in the same fragile state it was in at the start.

The most important builder/orchestration fixes already landed:

- false-negative truncation failures fixed
- structured planning promoted
- frozen BuildSpec/work-order path added
- coordination check-ins enforced
- deterministic scaffold bootstrap added
- IDE mountability fixed
- preview degraded mode improved

The biggest remaining product/platform gap is:

- real user-local Docker preview integration

The biggest remaining orchestration gap after that is:

- moving from “better prompts plus scaffold” to a full contract-driven execution engine with richer failure classification, patch-based edits, stronger validation gates, and more scaffold coverage.

If the next model follows the sequence in this document, it should not need to waste time rediscovering the same ground.
