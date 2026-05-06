# Apex Architecture Intelligence Map — 50% More Accurate Build Plan

## What changed from the earlier version

The earlier concept was a good visual idea, but not yet a real engineering resource. This version becomes engineering-grade by adding:

1. **Contracts** between UI, API, database, events, AI tools, external providers, auth policies, and config.
2. **Typed edges** that distinguish runtime calls, async events, data reads/writes, auth checks, config dependencies, secret dependencies, cost flow, feature flags, tenant boundaries, and deployment dependencies.
3. **Node-level code links** down to file/symbol/line ranges.
4. **Risk and blast-radius scoring** for every proposed AI change.
5. **Security trust-boundary mapping** so the AI does not accidentally bypass auth, leak tenant data, or log secrets.
6. **Cost-flow mapping** so model calls, BYOK use, retries, and build runner minutes are visible.
7. **Latency waterfall mapping** from UI action to backend calls, queue jobs, AI providers, DB, and preview deploy.
8. **Diagnostic playbooks** that tell the AI what to inspect next instead of guessing.
9. **AI explainability mode**: "why did the AI touch this file?"
10. **Safe change and rollback plans** generated before high-risk edits.

This makes the map useful not merely for humans, but for AI agents operating inside Apex-Build.

---

## Core product feature

Add a new product capability called:

**Architecture Intelligence Map**

It should be available inside every Apex-Build project/session and should power AI decisions silently in the background.

The user-visible version is an interactive 2D/3D graph. The AI-facing version is a precise queryable JSON graph.

---

## Data model

Recommended database tables:

```sql
architecture_maps (
  id uuid primary key,
  project_id uuid not null,
  repo_sha text not null,
  schema_version text not null,
  generated_at timestamptz not null,
  source text not null, -- manual, scanner, telemetry_enriched
  confidence numeric not null,
  raw_json jsonb not null
);

architecture_nodes (
  id text not null,
  map_id uuid not null references architecture_maps(id),
  layer text not null,
  type text not null,
  name text not null,
  description text,
  risk_level text not null,
  risk_score numeric not null,
  code_locations jsonb not null,
  security jsonb not null,
  telemetry jsonb not null,
  tests jsonb not null,
  ai_usage jsonb not null,
  primary key (map_id, id)
);

architecture_edges (
  id text not null,
  map_id uuid not null references architecture_maps(id),
  from_node text not null,
  to_node text not null,
  edge_type text not null,
  protocol text not null,
  criticality text not null,
  failure_behavior text,
  latency_budget_ms numeric,
  auth_context_forwarded boolean,
  tenant_context_forwarded boolean,
  metadata jsonb not null,
  primary key (map_id, id)
);

architecture_contracts (
  id text not null,
  map_id uuid not null references architecture_maps(id),
  contract_type text not null,
  producer text not null,
  consumers jsonb not null,
  schema_location text not null,
  runtime_validation boolean not null,
  breaking_change_rules jsonb not null,
  test_locations jsonb not null,
  primary key (map_id, id)
);

architecture_diagnostics (
  id uuid primary key,
  map_id uuid not null references architecture_maps(id),
  session_id uuid,
  symptom text not null,
  suspected_nodes jsonb not null,
  checks jsonb not null,
  decision_trace jsonb not null,
  safe_repair_plan jsonb not null,
  rollback_plan jsonb not null,
  created_at timestamptz not null
);

ai_file_touch_explanations (
  id uuid primary key,
  session_id uuid not null,
  file_path text not null,
  reason_inspected text not null,
  reason_modified text,
  related_nodes jsonb not null,
  related_contracts jsonb not null,
  risk_before numeric not null,
  risk_after numeric,
  created_at timestamptz not null
);
```

---

## Backend services

Create these backend modules:

### 1. Architecture Map Service

Responsibilities:

- Store/retrieve maps by project and repo SHA.
- Validate maps against `architecture-map.schema.json`.
- Serve subgraphs to AI agents.
- Compute blast radius.
- Compute change risk.
- Generate safe change plans.
- Generate rollback plans.

Core APIs:

```http
GET /api/projects/:projectId/architecture-map
POST /api/projects/:projectId/architecture-map/generate
POST /api/projects/:projectId/architecture-map/validate
POST /api/projects/:projectId/architecture-map/blast-radius
POST /api/projects/:projectId/architecture-map/diagnose
POST /api/projects/:projectId/architecture-map/safe-change-plan
```

### 2. Repo Scanner Service

Responsibilities:

- Walk repo tree.
- Detect frameworks.
- Detect routes/pages/components.
- Detect API handlers.
- Detect DB models/migrations.
- Detect queue/workers.
- Detect config/secrets references.
- Detect provider integrations.
- Detect test coverage.
- Emit initial nodes/edges/contracts.

### 3. AST Dependency Extractor

Responsibilities:

- Parse TypeScript, JavaScript, Go, Python, SQL, YAML, Dockerfiles.
- Build import graph.
- Build symbol graph.
- Map UI components to API calls.
- Map API handlers to services/repos/tables.
- Map workers to queues/events.
- Map model/provider calls to cost-flow nodes.
- Map tenant context propagation.

### 4. Runtime Telemetry Importer

Responsibilities:

- Import OpenTelemetry traces.
- Attach P95/P99 latency to edges.
- Attach error rates to nodes.
- Import logs/metrics/alerts.
- Support incident replay.

### 5. AI Diagnostic Engine

Responsibilities:

- Accept symptom + current session context.
- Query map for candidate nodes.
- Run playbook decision tree.
- Return first checks, likely root causes, safe repair plan, rollback plan.
- Explain why each file should be inspected or touched.

---

## Frontend feature

Create an `ArchitectureMapPanel`.

Primary views:

1. **Layer View** — human-readable architecture stack.
2. **Dependency View** — actual code dependency graph.
3. **Runtime Flow View** — request/job trace from UI to DB/provider/deploy.
4. **Diagnostic View** — symptom-driven root-cause path.
5. **Blast Radius View** — affected nodes/contracts/tests/deployments for selected change.
6. **Security View** — trust boundaries, auth checks, secrets, tenant propagation.
7. **Cost View** — model call costs, retries, BYOK vs platform-paid usage.
8. **Latency View** — waterfall overlay with P50/P95/P99.
9. **Test Coverage View** — tested/untested nodes and edges.
10. **Deployment Diff View** — last healthy deploy vs current deploy.

Recommended components:

```txt
frontend/src/features/architecture-map/
  ArchitectureMapPanel.tsx
  ArchitectureGraphCanvas.tsx
  ArchitectureNodeCard.tsx
  ArchitectureEdgeInspector.tsx
  ArchitectureOverlayToolbar.tsx
  ArchitectureDiagnosticPanel.tsx
  ArchitectureBlastRadiusPanel.tsx
  ArchitectureContractPanel.tsx
  ArchitectureFileTouchExplanation.tsx
  ArchitectureMapApi.ts
  graphLayout.ts
  graphTypes.ts
```

Use a graph renderer first. Start with 2D using React Flow or Cytoscape. Add 3D after the data model is reliable. Do not start with 3D; that is a design trap. The engineering value comes from the graph accuracy, not visual depth.

---

## AI-agent integration

Every AI build/repair session should do this before edits:

1. Load current architecture map for repo SHA.
2. Identify relevant nodes from user prompt.
3. Identify contracts touched.
4. Identify tests covering affected nodes.
5. Compute blast radius.
6. Compute risk score.
7. Produce "files to inspect first."
8. Produce "files safe to modify."
9. Produce "files not to modify without explicit reason."
10. Generate safe change plan.
11. Execute change.
12. Record why each file was touched.
13. Validate contracts/tests/build.
14. Update architecture map if structure changed.

The AI should not blindly search the repo. It should use the map as its first-pass navigation brain.

---

## Phased implementation

### Phase 1 — Static schema and manual maps

Goal: Ship the schema, validator, and manually authored example maps.

Backend:
- Add schema validation endpoint.
- Store map JSON in DB.
- Serve map to frontend and AI session service.

Frontend:
- Render simple graph/list view.
- Node detail drawer.
- Edge detail drawer.

AI:
- Add architecture-map lookup before build/repair sessions.

Tests:
- Schema validation tests.
- API tests.
- Basic UI render tests.

Acceptance:
- A project can have a valid architecture map.
- AI can retrieve relevant nodes by layer/type/code path.

### Phase 2 — Repo scanner

Goal: Auto-generate 60-70% of the architecture map from repo files.

Backend:
- File tree scanner.
- Framework detector.
- Route/API detector.
- Config/secret detector.
- Test detector.

Acceptance:
- New repo scan identifies frontend, backend, DB, workers, tests, config, providers.

### Phase 3 — AST-aware dependency extraction

Goal: Move from file listing to real dependency intelligence.

Backend:
- Parse imports.
- Parse function/class symbols.
- Map route handlers to services.
- Map services to repos/tables.
- Map API clients to endpoints.
- Map workers to queues.
- Flag unresolved dependencies.

Acceptance:
- Blast-radius query can identify affected files/contracts/tests.

### Phase 4 — Contract mapping

Goal: Make the graph safe for AI edits.

Backend:
- Detect OpenAPI/Zod/TypeScript schemas.
- Detect DB migration/table ownership.
- Detect event schemas.
- Detect AI tool schemas.
- Detect provider adapters.

Acceptance:
- UI/API/DB/AI tool breakages are detected before deploy.

### Phase 5 — Security and tenant overlays

Goal: Prevent tenant leaks and auth bypasses.

Backend:
- Auth middleware detector.
- Tenant context propagation checker.
- Cache key tenant checker.
- Queue payload tenant checker.
- Secret reference detector.

Acceptance:
- Any data/cache/queue edge missing tenant context is flagged.

### Phase 6 — Telemetry import

Goal: Turn static architecture into runtime intelligence.

Backend:
- Import traces/logs/metrics.
- Attach latencies/errors to edges/nodes.
- Build incident replay.

Acceptance:
- Selected request/session can show actual runtime path.

### Phase 7 — Cost-flow overlay

Goal: Make model and infrastructure spend visible.

Backend:
- Model call ledger.
- Token estimator vs actual usage comparison.
- BYOK attribution.
- Retry/fallback duplication detection.

Acceptance:
- Every AI session has complete cost path and margin risk.

### Phase 8 — Diagnostic engine

Goal: Give AI a real debugging brain.

Backend:
- Symptom classifier.
- Playbook runner.
- "Inspect next" recommender.
- Safe repair plan generator.
- Rollback generator.

Acceptance:
- Given stale UI/broken build/cost spike/500 error, system returns candidate root causes and check order.

### Phase 9 — Interactive graph UI

Goal: Make it excellent for humans.

Frontend:
- Layered graph.
- Overlay toggles.
- Search.
- Diff view.
- Diagnostic path animation.
- File/code links.
- Export JSON/PNG/Markdown.

Acceptance:
- User can diagnose an issue from graph without reading raw JSON.

### Phase 10 — Architecture Intelligence as competitive moat

Goal: Make this a core Apex-Build advantage.

System:
- Build sessions continuously update architecture map.
- AI refuses high-risk edits without map context.
- Enterprise users get audit reports.
- Every generated app ships with its own architecture map.

Acceptance:
- Apex-Build produces apps with built-in engineering intelligence that competitors do not provide by default.
