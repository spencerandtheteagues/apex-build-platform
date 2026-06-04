#!/usr/bin/env bash
# Guardrail tests for scripts/openrouter-free-canary-env.sh.
#
# These tests make no network calls. They verify that paid canary/testing runs
# pin all agent roles to OpenRouter's free model profile instead of paid models.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/openrouter-free-canary-env.sh"

TESTS_RUN=0
TESTS_FAILED=0
pass() { TESTS_RUN=$((TESTS_RUN + 1)); echo "  ok - $1"; }
fail() { TESTS_RUN=$((TESTS_RUN + 1)); TESTS_FAILED=$((TESTS_FAILED + 1)); echo "  NOT OK - $1" >&2; }

source_with_env() {
  env -i \
    PATH="/usr/bin:/bin" \
    APEX_OPENROUTER_FREE_MODEL="${1:-}" \
    bash -c 'source "$1" >/dev/null; printf "%s\n%s\n%s\n%s\n%s\n" "$APEX_AI_TESTING_PROFILE" "$APEX_LIVE_TEST_MODEL_PROFILE" "$APEX_PROVIDER_MODE" "$APEX_ROLE_ASSIGNMENTS_JSON" "$APEX_PROVIDER_MODEL_OVERRIDES_JSON"' _ "$SCRIPT"
}

echo "== openrouter free canary env guardrails (no network) =="

if [[ ! -f "$SCRIPT" ]]; then
  fail "missing openrouter free canary env script"
else
  pass "env script exists"
fi

output="$(source_with_env)"
expected=$'openrouter-free\nopenrouter-free\nplatform\n{"architect":"openrouter","coder":"openrouter","tester":"openrouter","devops":"openrouter"}\n{"openrouter":"moonshotai/kimi-k2.6:free"}'
if [[ "$output" == "$expected" ]]; then
  pass "defaults pin paid testing to OpenRouter free model"
else
  fail "defaults did not pin OpenRouter free routing"
  echo "$output" >&2
fi

output="$(source_with_env "qwen/qwen3-coder:free")"
expected=$'openrouter-free\nopenrouter-free\nplatform\n{"architect":"openrouter","coder":"openrouter","tester":"openrouter","devops":"openrouter"}\n{"openrouter":"qwen/qwen3-coder:free"}'
if [[ "$output" == "$expected" ]]; then
  pass "explicit free model override is preserved"
else
  fail "explicit free model override was not preserved"
  echo "$output" >&2
fi

echo
echo "Ran $TESTS_RUN assertions, $TESTS_FAILED failed."
if [[ "$TESTS_FAILED" -gt 0 ]]; then
  echo "OPENROUTER_FREE_CANARY_ENV_TESTS_FAILED"
  exit 1
fi
echo "OPENROUTER_FREE_CANARY_ENV_TESTS_PASSED"
