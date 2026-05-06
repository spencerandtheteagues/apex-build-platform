# Repo Scanner Spec

## Goal

Generate an accurate architecture map from a repo with minimal manual input.

## Scanner pipeline

1. File inventory
2. Framework detection
3. Package/dependency detection
4. Route detection
5. API handler detection
6. Component detection
7. State management detection
8. Auth/security detection
9. DB/schema/migration detection
10. Queue/worker detection
11. AI provider/model/tool detection
12. Config/secrets detection
13. Test detection
14. Contract detection
15. Edge extraction
16. Confidence scoring
17. Map validation

## Detection examples

### Frontend
Look for:
- React/Vite/Next/Svelte/Vue configs
- routes/pages/app directories
- components
- forms
- API client calls
- state stores
- feature flag clients

### Backend
Look for:
- route handlers
- controllers
- service modules
- middleware
- auth policies
- repositories
- migrations
- workers
- queues
- provider adapters

### AI-specific
Look for:
- model router
- prompt router
- context selector
- tool executor
- code generator
- repair planner
- provider clients
- token/cost accounting

## AST extraction

Recommended tools:
- TypeScript compiler API or ts-morph for TS/JS
- Go parser for Go
- Python ast for Python
- sqlparser for SQL
- yaml parser for YAML
- Dockerfile parser

## Output

The scanner emits:
- nodes
- edges
- contracts
- unresolved references
- confidence scores
- recommended manual annotations
