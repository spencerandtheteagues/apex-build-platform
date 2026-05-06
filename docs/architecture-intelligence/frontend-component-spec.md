# Frontend Component Spec — Architecture Intelligence Map

## Main component

`ArchitectureMapPanel.tsx`

Props:

```ts
type ArchitectureMapPanelProps = {
  projectId: string;
  sessionId?: string;
  selectedFilePath?: string;
  selectedSymptom?: string;
  mode?: "layer" | "dependency" | "runtime" | "diagnostic" | "blast-radius" | "security" | "cost" | "latency" | "coverage" | "deployment-diff";
};
```

## Core UX

The panel should not be decorative. It should answer engineering questions quickly.

Top bar:
- Current repo SHA
- Map confidence
- Last scan time
- Active overlay
- Risk score
- Export buttons

Left sidebar:
- Layer filters
- Node type filters
- Risk filters
- Contract filters
- Search by file/symbol/node

Center:
- Graph canvas

Right inspector:
- Node details
- Edge details
- Related files
- Contracts
- Tests
- Telemetry
- Failure modes
- AI guidance
- Safe change plan
- Rollback plan

Bottom drawer:
- Diagnostic timeline
- Incident replay
- AI file-touch explanations

## Graph rendering

Start with 2D. Use 3D later.

Recommended:
- React Flow for v1 because it is fast to ship and inspectable.
- Cytoscape if graph algorithms become more important.
- Three.js/React Three Fiber only after graph correctness is proven.

## Overlays

Each overlay changes node/edge styling and right-panel data.

- Risk overlay: color by risk score.
- Blast radius: highlight affected transitive closure.
- Security: show trust boundaries and tenant propagation.
- Cost: show token/cost metrics on AI/provider/build-runner paths.
- Latency: edge width by p95/p99.
- Test coverage: show covered/uncovered nodes.
- Deployment diff: show changed nodes since last healthy deploy.
- Feature flags: show gated paths and exposed tenants.

## Critical design choices

Bad idea:
- Starting with 3D first.

Why:
- 3D looks impressive but can hide graph errors and make debugging slower.

Correct approach:
- Build reliable data model + 2D graph first.
- Add 3D presentation after the graph has accurate nodes, edges, contracts, and telemetry.

## Required user workflows

### Diagnose stale UI
1. User clicks "Diagnose."
2. Selects symptom: stale UI data.
3. System highlights UI state, API client, session service, Redis, Postgres, queue, worker.
4. Right panel shows ordered checks.
5. User can click "Generate safe repair plan."

### Explain AI edit
1. User selects a changed file.
2. Panel shows:
   - Why inspected
   - Why modified
   - Related node
   - Related contracts
   - Tests run
   - Risk before/after

### Blast radius
1. User selects file or proposed diff.
2. System highlights affected components, APIs, tables, workers, contracts, tests, deployment units.
3. AI cannot proceed with high-risk change until a safe plan exists.
