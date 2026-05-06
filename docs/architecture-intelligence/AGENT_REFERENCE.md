# Architecture Intelligence Map Agent Reference

## Purpose

This directory contains the imported Apex Architecture Intelligence Map package. Use it as an advisory orientation layer for AI-assisted work on Apex Build.

## Files

- `architecture-map.schema.json`: JSON schema for architecture maps.
- `example-apex-build-map.json`: example Apex Build map with nodes, edges, contracts, overlays, diagnostics, and quality gates.
- `diagnostic-playbooks.json`: symptom-driven debugging playbooks.
- `implementation-plan.md`: proposed product build plan for a future map feature.
- `backend-service-spec.md`, `repo-scanner-spec.md`, `frontend-component-spec.md`: proposed implementation specs.
- `ai-agent-usage-guide.md`: original guidance for AI agents using the map.
- `image-generation-prompt.md`: visual prompt for producing architecture-map imagery.

## Agent Rules

1. Treat repo files, tests, runtime logs, and current product contracts as the source of truth.
2. Use the map to identify likely related files, contracts, tests, blast radius, and rollback considerations before changing code.
3. Do not assume the example map is current. Verify every referenced file, endpoint, schema, and runtime behavior before relying on it.
4. If the map conflicts with code, trust the code and note the discrepancy.
5. For high-risk work in orchestration, billing, auth, preview, provider routing, or deployment, use the map to produce a compact risk/blast-radius note before editing.
6. Do not paste the entire map into prompts by default. Load only the relevant nodes, contracts, or diagnostic playbooks for the current task.

## Current Integration Status

The files are committed as reference documentation only. They are not loaded automatically by the Apex Build runtime, orchestration engine, model router, or generated-app context pipeline.

Runtime integration should be implemented separately with tests if Apex Build needs the map to drive automatic agent behavior.
