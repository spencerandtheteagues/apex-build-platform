#!/usr/bin/env bash
# Integration tests for the false-green guardrails in run_platform_canary_matrix.sh.
# These exercise only the env combinations where NO scenario actually runs, so the
# tests make no network calls and need no secrets — they verify the matrix refuses
# to report green when nothing ran or when required scenarios were skipped.
#
# Run: ./scripts/test/canary_matrix_guardrail_test.sh
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
MATRIX="$REPO_ROOT/scripts/run_platform_canary_matrix.sh"
SMOKE="$REPO_ROOT/scripts/run_platform_build_smoke.sh"
PROMPT_MATRIX="$REPO_ROOT/scripts/run_live_prompt_matrix.sh"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT
PROMPT_FIXTURE="$TMP_DIR/01-test-prompt.md"
printf 'Build a small test app.\n' > "$PROMPT_FIXTURE"

TESTS_RUN=0
TESTS_FAILED=0
pass() { TESTS_RUN=$((TESTS_RUN + 1)); echo "  ok - $1"; }
fail() { TESTS_RUN=$((TESTS_RUN + 1)); TESTS_FAILED=$((TESTS_FAILED + 1)); echo "  NOT OK - $1" >&2; }

# run_matrix_safe <expected_rc> <expected_token> <label> -- <env assignments...>
run_matrix_safe() {
  local expected_rc="$1" expected_token="$2" label="$3"
  shift 3
  [[ "$1" == "--" ]] && shift
  local out rc
  # All invocations disable the free scenario and provide no credentials, so no
  # scenario body executes and no network call is made.
  out="$(env "$@" RUN_FREE_FRONTEND=0 LOGIN_EMAIL="" LOGIN_PASSWORD="" \
    bash "$MATRIX" 2>&1)"
  rc=$?
  if [[ "$rc" == "$expected_rc" && "$out" == *"$expected_token"* ]]; then
    pass "$label (rc=$rc)"
  else
    fail "$label (rc=$rc want $expected_rc; token '$expected_token' present=$([[ "$out" == *"$expected_token"* ]] && echo yes || echo no))"
    echo "----- output -----" >&2
    echo "$out" >&2
    echo "------------------" >&2
  fi
}

# run_prompt_matrix_safe <expected_rc> <expected_token> <label> -- <env assignments...>
run_prompt_matrix_safe() {
  local expected_rc="$1" expected_token="$2" label="$3"
  shift 3
  [[ "$1" == "--" ]] && shift
  local out rc
  out="$(env "$@" APEX_SKIP_OLLAMA_CREDIT_SAVER_SOURCE=1 DRY_RUN=1 PROMPT_FILES="$PROMPT_FIXTURE" \
    bash "$PROMPT_MATRIX" 2>&1)"
  rc=$?
  if [[ "$rc" == "$expected_rc" && "$out" == *"$expected_token"* ]]; then
    pass "$label (rc=$rc)"
  else
    fail "$label (rc=$rc want $expected_rc; token '$expected_token' present=$([[ "$out" == *"$expected_token"* ]] && echo yes || echo no))"
    echo "----- output -----" >&2
    echo "$out" >&2
    echo "------------------" >&2
  fi
}

echo "== matrix false-green guardrails (no network) =="

if grep -q 'USER="${LOGIN_USERNAME:-platform${SUFFIX}}"' "$SMOKE"; then
  fail "platform smoke does not use generated username for email-only login"
elif grep -q 'Email-only login must not send a generated username' "$SMOKE" && grep -q 'USER=""' "$SMOKE"; then
  pass "platform smoke preserves email-only login semantics"
else
  fail "platform smoke email-only login guardrail missing"
fi

if grep -q 'PROVIDER_MODE="${PROVIDER_MODE:-${APEX_PROVIDER_MODE:-platform}}"' "$SMOKE" \
  && grep -q 'openrouter-free-canary-env.sh' "$SMOKE" \
  && grep -q 'role_assignments' "$SMOKE" \
  && grep -q 'provider_model_overrides' "$SMOKE"; then
  pass "platform smoke supports free OpenRouter routing controls"
else
  fail "platform smoke free OpenRouter routing controls missing"
fi

# Required paid scenarios without credentials must fail as INCOMPLETE, not green.
run_matrix_safe 1 "CANARY_MATRIX_INCOMPLETE_REQUIRED_SCENARIOS_SKIPPED" \
  "required paid + no creds -> incomplete" -- \
  REQUIRE_PAID_SCENARIOS=1 RUN_PAID_BALANCED=1 RUN_PAID_MAX=1

# Nothing enabled at all must fail as NO_SCENARIOS_RAN (can't be a silent green).
run_matrix_safe 1 "CANARY_MATRIX_NO_SCENARIOS_RAN" \
  "nothing enabled -> no scenarios ran" -- \
  REQUIRE_PAID_SCENARIOS=0 RUN_PAID_BALANCED=0 RUN_PAID_MAX=0

# Paid optional + no creds: scenarios skipped, still not green (nothing passed).
run_matrix_safe 1 "CANARY_MATRIX_NO_SCENARIOS_RAN" \
  "optional paid + no creds -> not green" -- \
  REQUIRE_PAID_SCENARIOS=0 RUN_PAID_BALANCED=1 RUN_PAID_MAX=1

echo
echo "== prompt matrix shape guardrails (dry run, no network) =="

run_prompt_matrix_safe 0 "PROMPT_MATRIX_DRY_RUN" \
  "expected prompt count and balanced mode -> dry-run ok" -- \
  EXPECTED_PROMPT_COUNT=1 POWER_MODES=balanced

run_prompt_matrix_safe 1 "PROMPT_MATRIX_PROMPT_COUNT_MISMATCH" \
  "expected prompt count mismatch -> fails before live run" -- \
  EXPECTED_PROMPT_COUNT=2 POWER_MODES=balanced

run_prompt_matrix_safe 1 "PROMPT_MATRIX_POWER_MODES_EMPTY" \
  "blank power modes -> fails before live run" -- \
  EXPECTED_PROMPT_COUNT=1 POWER_MODES="   "

run_prompt_matrix_safe 1 "PROMPT_MATRIX_POWER_MODE_INVALID" \
  "unknown power mode -> fails before live run" -- \
  EXPECTED_PROMPT_COUNT=1 POWER_MODES=turbo

# ---- prompt fixture breadth guardrail (no network) ----
echo
BREADTH_TEST="$SCRIPT_DIR/prompt_matrix_fixture_breadth_test.sh"
if [[ -x "$BREADTH_TEST" || -f "$BREADTH_TEST" ]]; then
  if bash "$BREADTH_TEST"; then
    pass "prompt fixture breadth guardrail passes"
  else
    fail "prompt fixture breadth guardrail passes"
  fi
else
  fail "prompt matrix fixture breadth guardrail script missing: $BREADTH_TEST"
fi

echo
echo "Ran $TESTS_RUN assertions, $TESTS_FAILED failed."
if [[ "$TESTS_FAILED" -gt 0 ]]; then
  echo "CANARY_MATRIX_GUARDRAIL_TESTS_FAILED"
  exit 1
fi
echo "CANARY_MATRIX_GUARDRAIL_TESTS_PASSED"
