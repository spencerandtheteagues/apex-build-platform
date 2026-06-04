#!/usr/bin/env bash
# No-network guardrails for .github/workflows/production-canary.yml.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

WORKFLOW=".github/workflows/production-canary.yml"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "PASS: $*"
}

echo "== production canary workflow guardrails =="
[[ -f "$WORKFLOW" ]] || fail "missing $WORKFLOW"

grep -q "bash scripts/test/render_launch_guardrail_test.sh" "$WORKFLOW" || fail "workflow must run Render launch guardrail"
grep -q "bash scripts/test/production_canary_workflow_guardrail_test.sh" "$WORKFLOW" || fail "workflow must run workflow self-guardrail"
pass "workflow runs script/workflow guardrails"

grep -q "REQUIRE_PAID_CANARIES:.*APEX_REQUIRE_PAID_CANARIES" "$WORKFLOW" || fail "Stripe verifier must receive APEX_REQUIRE_PAID_CANARIES"
grep -q "APEX_REQUIRE_PAID_CANARIES=true requires an existing verified paid canary account" "$WORKFLOW" || fail "strict mode must require existing verified paid canary credentials"
grep -q "export APEX_STRIPE_REGISTER_SMOKE_USER=0" "$WORKFLOW" || fail "strict mode must disable disposable Stripe smoke registration"
grep -q "export APEX_STRIPE_RUN_PORTAL=1" "$WORKFLOW" || fail "strict mode must require paid canary billing portal probe"
pass "strict paid canary mode hardens Stripe verifier"

grep -q "APEX_REQUIRE_PAID_CANARIES=true requires APEX_CANARY_EMAIL/APEX_CANARY_PASSWORD for paid platform canaries" "$WORKFLOW" || fail "strict mode must fail missing paid platform canary credentials"
grep -q "APEX_REQUIRE_PAID_CANARIES=true requires APEX_CANARY_EMAIL/APEX_CANARY_PASSWORD for the golden live canary" "$WORKFLOW" || fail "strict mode must fail missing golden canary credentials"
grep -q "required paid canary .* did not pass" "$WORKFLOW" || fail "platform canary gate must fail unpaid/skipped paid canaries when strict mode is on"
pass "strict paid canary mode prevents skipped build canaries from going green"

grep -q "PROVIDER_MODE:.*platform" "$WORKFLOW" || fail "paid build canaries must use platform provider mode"
grep -q "APEX_BYOK_OLLAMA_ONLY:.*0" "$WORKFLOW" || fail "paid build canaries must not force Ollama-only routing"
grep -q '"architect":"openrouter","coder":"openrouter","tester":"openrouter","devops":"openrouter"' "$WORKFLOW" || fail "paid build canaries must assign all user-facing roles to OpenRouter"
grep -q '"openrouter":"moonshotai/kimi-k2.6:free"' "$WORKFLOW" || fail "paid build canaries must pin the free OpenRouter model"
grep -q "APEX_LIVE_TEST_MODEL_PROFILE:.*openrouter-free" "$WORKFLOW" || fail "paid build canaries must label the OpenRouter free model profile"
pass "paid build canaries are wired for free OpenRouter routing"

if grep -q "APEX_STRIPE_REGISTER_SMOKE_USER: '1'.*APEX_REQUIRE_PAID_CANARIES" "$WORKFLOW"; then
  fail "workflow should not encode disposable smoke registration as strict-mode evidence"
fi

echo "production canary workflow guardrails passed"
