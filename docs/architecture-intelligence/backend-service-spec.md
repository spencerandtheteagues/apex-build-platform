# Backend Service Spec — Architecture Intelligence Map

## Services

### ArchitectureMapService

Responsibilities:
- Store map versions.
- Validate maps.
- Return full map or subgraph.
- Compute blast radius.
- Compute risk score.
- Generate AI-readable context summaries.

Functions:

```ts
getMap(projectId, repoSha)
saveMap(projectId, repoSha, map)
validateMap(map)
getSubgraph(projectId, query)
computeBlastRadius(projectId, changedFiles)
computeChangeRisk(projectId, proposedDiff)
generateSafeChangePlan(projectId, proposedDiff)
generateRollbackPlan(projectId, proposedDiff)
```

### RepoScannerService

Responsibilities:
- Scan repo.
- Detect architecture nodes.
- Emit confidence scores.
- Detect code locations.
- Detect tests.
- Detect contracts.

### DependencyGraphService

Responsibilities:
- AST parse.
- Import graph.
- Symbol graph.
- Route -> service -> repo -> table mapping.
- UI -> API mapping.
- Worker -> queue mapping.
- Tool -> model/provider mapping.

### ContractMapService

Responsibilities:
- Extract contracts.
- Validate producer/consumer compatibility.
- Detect breaking changes.
- Link contracts to tests.

### SecurityBoundaryService

Responsibilities:
- Detect auth middleware.
- Verify tenant propagation.
- Detect secret usage.
- Detect PII access paths.
- Detect unsafe logs.

### TelemetryImportService

Responsibilities:
- Import OTel traces.
- Attach runtime data to edges.
- Import logs/metrics/alerts.
- Support incident replay.

### DiagnosticEngine

Responsibilities:
- Symptom classification.
- Playbook execution.
- Candidate root cause ranking.
- "Inspect next" recommendations.
- Safe repair and rollback plan generation.

## API endpoints

```http
GET    /api/projects/:projectId/architecture-map
POST   /api/projects/:projectId/architecture-map/generate
POST   /api/projects/:projectId/architecture-map/validate
POST   /api/projects/:projectId/architecture-map/subgraph
POST   /api/projects/:projectId/architecture-map/blast-radius
POST   /api/projects/:projectId/architecture-map/change-risk
POST   /api/projects/:projectId/architecture-map/diagnose
POST   /api/projects/:projectId/architecture-map/safe-change-plan
POST   /api/projects/:projectId/architecture-map/rollback-plan
POST   /api/projects/:projectId/architecture-map/file-touch-explanation
```

## Non-negotiable safeguards

- Never store plaintext BYOK secrets in the map.
- Never log prompt contents if user/project setting disables that.
- Never allow AI tool execution to bypass architecture safety checks for high-risk nodes.
- Never consider generated map perfect; every node/edge should carry confidence.
- Never block emergency rollback because map validation failed.
