#!/usr/bin/env bash
# Unit tests for scripts/lib/canary_report.sh. No network, no secrets — feeds
# JSON fixtures to the pure helper functions and asserts behavior.
#
# Run: ./scripts/test/canary_report_test.sh
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
# shellcheck source=../lib/canary_report.sh
source "$REPO_ROOT/scripts/lib/canary_report.sh"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required to run these tests" >&2
  exit 1
fi

TESTS_RUN=0
TESTS_FAILED=0
WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

pass() { TESTS_RUN=$((TESTS_RUN + 1)); echo "  ok - $1"; }
fail() {
  TESTS_RUN=$((TESTS_RUN + 1))
  TESTS_FAILED=$((TESTS_FAILED + 1))
  echo "  NOT OK - $1" >&2
}

assert_eq() { # <actual> <expected> <label>
  if [[ "$1" == "$2" ]]; then pass "$3"; else fail "$3 (got '$1', want '$2')"; fi
}
assert_contains() { # <haystack> <needle> <label>
  if [[ "$1" == *"$2"* ]]; then pass "$3"; else fail "$3 (missing '$2' in: $1)"; fi
}
assert_not_contains() { # <haystack> <needle> <label>
  if [[ "$1" != *"$2"* ]]; then pass "$3"; else fail "$3 (unexpectedly found '$2')"; fi
}
assert_rc() { # <expected_rc> <actual_rc> <label>
  if [[ "$1" == "$2" ]]; then pass "$3"; else fail "$3 (rc got '$2', want '$1')"; fi
}

echo "== canary_matrix_verdict =="
out="$(canary_matrix_verdict 2 0 0)"; rc=$?
assert_eq "$rc" "0" "all-pass returns 0"
assert_contains "$out" "CANARY_MATRIX_PASSED" "all-pass token"

out="$(canary_matrix_verdict 1 1 0)"; rc=$?
assert_eq "$rc" "1" "a failure returns 1"
assert_contains "$out" "CANARY_MATRIX_FAILED" "failure token"

out="$(canary_matrix_verdict 2 0 1)"; rc=$?
assert_eq "$rc" "1" "required skip returns 1"
assert_contains "$out" "INCOMPLETE_REQUIRED_SCENARIOS_SKIPPED" "required-skip token"

out="$(canary_matrix_verdict 0 0 0)"; rc=$?
assert_eq "$rc" "1" "zero scenarios returns 1 (no false green)"
assert_contains "$out" "CANARY_MATRIX_NO_SCENARIOS_RAN" "no-scenarios token"

echo "== canary_provider_models =="
cat >"$WORKDIR/detail_good.json" <<'JSON'
{
  "id": "build-123",
  "status": "completed",
  "provider_mode": "platform",
  "prompt": "Build an app for provider ACME with model T-1000",
  "orchestration": {
    "agents": [
      {"role": "lead", "provider": "claude", "model": "claude-sonnet-4-6"},
      {"role": "frontend", "provider": "ollama", "model": "kimi-k2.6:cloud"}
    ]
  }
}
JSON
pm="$(canary_provider_models "$WORKDIR/detail_good.json")"
assert_contains "$pm" "claude/claude-sonnet-4-6" "extracts lead routing"
assert_contains "$pm" "ollama/kimi-k2.6:cloud" "extracts frontend routing"
assert_not_contains "$pm" "ACME" "ignores echoed prompt provider text"
assert_not_contains "$pm" "T-1000" "ignores echoed prompt model text"

empty_pm="$(canary_provider_models "$WORKDIR/missing.json")"
assert_eq "$empty_pm" "" "missing file yields empty provider list (no crash)"

echo "== canary_preview_status / canary_preview_ready =="
echo '{"preview":{"active":true,"url":"https://p.example/app"},"proxy_url":"https://proxy.example/app"}' >"$WORKDIR/prev_active.json"
ps="$(canary_preview_status "$WORKDIR/prev_active.json")"
assert_contains "$ps" "active url=" "active preview reports url"
canary_preview_ready "$WORKDIR/prev_active.json"; assert_rc "0" "$?" "active+url is ready"

echo '{"preview":{"active":true}}' >"$WORKDIR/prev_nourl.json"
canary_preview_ready "$WORKDIR/prev_nourl.json"; assert_rc "1" "$?" "active without url is NOT ready"
assert_contains "$(canary_preview_status "$WORKDIR/prev_nourl.json")" "not_ready" "active-no-url status string"

echo '{"_http_status":404}' >"$WORKDIR/prev_404.json"
canary_preview_ready "$WORKDIR/prev_404.json"; assert_rc "1" "$?" "404 preview is NOT ready"
assert_contains "$(canary_preview_status "$WORKDIR/prev_404.json")" "not_started" "404 status string"

canary_preview_ready "$WORKDIR/missing.json"; assert_rc "1" "$?" "missing preview file is NOT ready"

echo "== canary_jq_first =="
assert_eq "$(canary_jq_first "$WORKDIR/detail_good.json" '.does_not_exist' '.status')" "completed" "falls through to first present path"
assert_eq "$(canary_jq_first "$WORKDIR/missing.json" '.status')" "" "missing file yields empty"

echo "== canary_report_block =="
mkdir -p "$WORKDIR/artifacts"
echo '{}' >"$WORKDIR/artifacts/build-detail.json"
block="$(canary_report_block "paid-balanced" "build-123" "failed" "claude/sonnet, ollama/kimi" "failed" "not_ready (active=false url=none)" "$WORKDIR/artifacts")"
assert_contains "$block" "scenario:        paid-balanced" "block has scenario"
assert_contains "$block" "build_id:        build-123" "block has build id"
assert_contains "$block" "terminal_state:  failed" "block has terminal state"
assert_contains "$block" "ollama/kimi" "block has provider/model"
assert_contains "$block" "quality_gate:    failed" "block has quality gate"
assert_contains "$block" "preview:         not_ready" "block has preview status"
assert_contains "$block" "build-detail.json" "block lists artifacts"

# Malformed/empty inputs still produce a block with ? placeholders (no crash).
block_empty="$(canary_report_block "" "" "" "" "" "" "")"
assert_contains "$block_empty" "build_id:        ?" "empty inputs render ? placeholder"

# GITHUB_STEP_SUMMARY is appended when set.
GITHUB_STEP_SUMMARY="$WORKDIR/step_summary.md" canary_report_block \
  "paid-max" "b9" "completed" "claude/opus" "passed" "active url=https://x" "$WORKDIR/artifacts" >/dev/null
assert_contains "$(cat "$WORKDIR/step_summary.md")" "Canary scenario: \`paid-max\`" "writes GITHUB_STEP_SUMMARY"

echo
echo "Ran $TESTS_RUN assertions, $TESTS_FAILED failed."
if [[ "$TESTS_FAILED" -gt 0 ]]; then
  echo "CANARY_REPORT_TESTS_FAILED"
  exit 1
fi
echo "CANARY_REPORT_TESTS_PASSED"
