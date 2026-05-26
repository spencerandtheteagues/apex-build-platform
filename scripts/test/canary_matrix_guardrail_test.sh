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

echo "== matrix false-green guardrails (no network) =="

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
echo "Ran $TESTS_RUN assertions, $TESTS_FAILED failed."
if [[ "$TESTS_FAILED" -gt 0 ]]; then
  echo "CANARY_MATRIX_GUARDRAIL_TESTS_FAILED"
  exit 1
fi
echo "CANARY_MATRIX_GUARDRAIL_TESTS_PASSED"
