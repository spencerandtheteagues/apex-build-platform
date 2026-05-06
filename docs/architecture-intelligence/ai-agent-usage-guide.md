# AI Agent Usage Guide — Architecture Intelligence Map

## Rule 1: Use the map before touching code

Before any build, repair, refactor, or feature implementation, the AI must query the architecture map.

Minimum required AI preflight:

1. Identify relevant nodes.
2. Identify relevant contracts.
3. Identify relevant files.
4. Identify affected tests.
5. Compute blast radius.
6. Compute risk.
7. Create safe change plan.
8. Create rollback plan for high/critical risk.

## Rule 2: Do not trust the map blindly

The map guides inspection. It does not replace code reading.

If map and code disagree:
- Trust code.
- Update map.
- Record discrepancy.

## Rule 3: Every file touch needs an explanation

For each file:

```json
{
  "file": "path",
  "reason_inspected": "why this file was relevant",
  "reason_modified": "why this file had to change",
  "related_nodes": [],
  "related_contracts": [],
  "risk": "low|medium|high|critical",
  "tests_required": []
}
```

## Rule 4: Contract-first repairs

When a bug crosses UI/API/DB/AI-tool boundaries, repair the contract mismatch first. Do not patch only the visible symptom.

## Rule 5: High-risk edit rule

High-risk or critical nodes require:
- blast-radius report
- contract check
- focused tests
- rollback plan
- deploy health check

## Rule 6: Debugging order

When diagnosing, inspect in this order:

1. User-visible symptom
2. Entry node
3. Runtime trace
4. Contract boundary
5. Recent deploy diff
6. Data/cache/queue state
7. Logs/errors
8. Tests
9. Smallest safe patch
10. Validation and rollback note
