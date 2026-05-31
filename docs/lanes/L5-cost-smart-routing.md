# Lane L5 — Cost / Smart Routing (audit 2026-05-16)

Source: Wave-1 Auditor. This is the highest direct "least money" leverage lane.

| ID | Tier | File:line | Gap | Leverage |
| --- | --- | --- | --- | --- |
| L5-01 | S | build_telemetry.go:100 | No $/successful-build KPI exists | Foundational — makes all else measurable |
| L5-02 | S | m1_feature_flags.go:21 | `APEX_DETERMINISTIC_TASK_GATES` off by default → paid critique on broken candidates | Stops paying AI to critique code a free compile-check rejects |
| L5-03 | A | routing_waterfall.go:43 | Flagship escalation on ANY retry≥2 incl. transient/rate-limit | Cuts Opus-tier spend on non-quality retries |
| L5-04 | A | manager_task_routing.go:621 | `DisableProviderFallback:true` blocks cost-threshold cheaper-provider skip on primary path | Activates already-built cheaper-provider fallthrough |
| L5-05 | A | orchestration_contracts.go:1498 | Reviewer/Solver provider hardcoded (Grok/Gemini), bypasses cost-ranked scorecard | Lets scorecard pick cheaper-equal provider |
| L5-06 | A | routing_waterfall.go:90 | Balanced builds use Haiku for judge/critique (binary Max/Fast) | +$0.002/judge but fewer wasted dual-candidate picks |
| L5-07 | B | orchestration_contracts.go:1754 | Cost penalty weight 0.5 too small vs quality signals | Strengthens cost signal under CostSensitivityHigh |
| L5-08 | B | ai_adapter.go:874 | Legacy GenerateFullStackApp hardcodes Claude+GPT4, no waterfall | Brings single-agent path under cost control |

**Execution order:** L5-01 (KPI first — baseline), then L5-02 (biggest immediate $ cut, one-line default flip + Render env), then L5-04/L5-03/L5-05, then B-tier. Every change verified against `cost_per_success_usd grouped by power_mode` from L5-01 telemetry. **All items touch backend/internal/agents — execute only after L2/L8 auditors finish to avoid invalidating their anchors.**
