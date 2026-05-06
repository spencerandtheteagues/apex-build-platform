Create a professional engineering-command-center 3D diagram titled:

"Architecture Intelligence Map — AI-Navigable App Structure"

The diagram must look like a precise software architecture and debugging instrument, not marketing art.

Visual requirements:
- Dark technical background.
- Layered 3D stack with 13 horizontal layers:
  1. User / Device
  2. Presentation
  3. Client Runtime
  4. Edge / API Entry
  5. Auth / Security
  6. Application Services
  7. AI / Agent
  8. Data Access
  9. Persistence
  10. Eventing / Async
  11. Observability
  12. DevOps / Runtime
  13. External Integrations
- Each layer contains rectangular nodes with labels, status badges, risk score, p95 latency, error rate, and code path label.
- Typed edge colors:
  white = sync call
  blue dashed = async event
  green = data read/write
  yellow = dependency
  red = diagnostic path
  purple = AI tool/model path
  orange = cost flow
  cyan = telemetry
- Show trust boundaries as translucent vertical shields between client, edge, trusted backend, privileged AI agent layer, data, and external providers.
- Show multi-tenant boundary as glowing tenant_id/project_id propagation markers across API, service, DB, cache, queue, and AI tools.
- Show contract badges between UI/API, API/DB, queue/event, AI tool, external provider, and auth policy.
- Include overlays:
  blast radius
  change risk
  test coverage
  security
  cost flow
  latency waterfall
  deployment diff
  migration risk
  feature flags
- Right-side diagnostic panel:
  Symptom: "AI generated broken code"
  Path:
  1. Context selector omitted file
  2. Model router picked underpowered model
  3. Tool executor modified wrong layer
  4. Contract test failed
  5. Safe repair plan generated
  6. Rollback plan available
- Bottom panel:
  "Why did the AI touch this file?"
  "What should the AI inspect next?"
  "Safe change plan"
  "Rollback plan"
- Include legend, SLO indicators, map confidence, repo SHA, last scan time, selected deployment diff.
- Use clean technical typography, crisp labels, no gibberish text.
- Make it feel like a flight-control dashboard for software systems.
