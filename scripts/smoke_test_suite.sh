#!/usr/bin/env bash
#===============================================================================
# APEX-BUILD UNIFIED SMOKE TEST & HEALTH CHECK SUITE
#===============================================================================
# Master orchestrator that runs all runtime stability, heartbeat/dead-agent,
# and rate-limit/backpressure checks in sequence.
#
# Usage:
#   BACKEND_URL=https://api.apex-build.dev bash smoke_test_suite.sh
#   BACKEND_URL=http://localhost:8080 bash smoke_test_suite.sh
#
# Individual suites can be run standalone:
#   bash runtime_stability_checklist.sh
#   bash heartbeat_deadagent_check.sh
#   bash rate_limit_backpressure_test.sh
#
# Exits non-zero if any suite fails.
#===============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_URL="${BACKEND_URL:-https://api.apex-build.dev}"
FRONTEND_URL="${FRONTEND_URL:-https://apex-build.dev}"
LOGIN_EMAIL="${LOGIN_EMAIL:-}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-}"

export BACKEND_URL FRONTEND_URL LOGIN_EMAIL LOGIN_PASSWORD

SUITE_PASS=0
SUITE_FAIL=0
SUITE_WARN=0

log_suite() { echo ""; echo "▶▶▶ $1 ◀◀◀"; }

run_suite() {
  local name="$1"
  local script="$2"
  local warn_only="${3:-0}"

  log_suite "$name"
  if [[ -x "$SCRIPT_DIR/$script" ]]; then
    if bash "$SCRIPT_DIR/$script"; then
      echo "  [SUITE PASS] $name"
      ((SUITE_PASS++))
    else
      if [[ "$warn_only" == "1" ]]; then
        echo "  [SUITE WARN] $name (non-zero exit, treated as warning)"
        ((SUITE_WARN++))
      else
        echo "  [SUITE FAIL] $name"
        ((SUITE_FAIL++))
      fi
    fi
  else
    echo "  [SUITE SKIP] $script not found or not executable"
    ((SUITE_WARN++))
  fi
}

echo "================================================================"
echo "APEX-BUILD UNIFIED SMOKE TEST & HEALTH CHECK SUITE"
echo "================================================================"
echo "Backend: $BACKEND_URL"
echo "Frontend: $FRONTEND_URL"
echo "Started: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "================================================================"

#-------------------------------------------------------------------------------
# PHASE 1: Runtime Stability
#-------------------------------------------------------------------------------
run_suite "PHASE 1: Runtime Stability Checklist" "runtime_stability_checklist.sh" 0

#-------------------------------------------------------------------------------
# PHASE 2: Heartbeat / Dead-Agent Detection
#-------------------------------------------------------------------------------
run_suite "PHASE 2: Heartbeat / Dead-Agent Check" "heartbeat_deadagent_check.sh" 0

#-------------------------------------------------------------------------------
# PHASE 3: Rate-Limit & Backpressure
#-------------------------------------------------------------------------------
run_suite "PHASE 3: Rate-Limit & Backpressure Test" "rate_limit_backpressure_test.sh" 0

#-------------------------------------------------------------------------------
# SUMMARY
#-------------------------------------------------------------------------------
echo ""
echo "================================================================"
echo "UNIFIED SUITE SUMMARY"
echo "================================================================"
echo "  Suites Passed:  $SUITE_PASS"
echo "  Suites Failed:  $SUITE_FAIL"
echo "  Suites Warned:  $SUITE_WARN"
echo "  Total:        $((SUITE_PASS + SUITE_FAIL + SUITE_WARN))"
echo "================================================================"
echo "Finished: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "================================================================"

if [[ "$SUITE_FAIL" -gt 0 ]]; then
  echo "RESULT: FAILED — $SUITE_FAIL suite(s) did not pass"
  exit 1
fi

if [[ "$SUITE_WARN" -gt 0 ]]; then
  echo "RESULT: PASSED WITH WARNINGS — $SUITE_WARN suite(s) had warnings"
  exit 0
fi

echo "RESULT: ALL CLEAR — all suites passed"
exit 0
