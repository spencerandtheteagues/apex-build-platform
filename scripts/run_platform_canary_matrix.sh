#!/usr/bin/env bash
# Drives the platform build canary scenarios (free-fast, paid-balanced, paid-max)
# and emits a single, false-green-resistant verdict. Unlike a naive `set -e`
# pipeline, this collects every scenario's result so one failure doesn't hide the
# others, and it refuses to print PASSED when required scenarios were skipped or
# nothing ran.
set -uo pipefail

BASE_URL="${BASE_URL:-https://api.apex-build.dev/api/v1}"
POLL_SECONDS_FREE="${POLL_SECONDS_FREE:-10}"
MAX_POLLS_FREE="${MAX_POLLS_FREE:-120}"
POLL_SECONDS_PAID="${POLL_SECONDS_PAID:-15}"
MAX_POLLS_PAID="${MAX_POLLS_PAID:-180}"
RUN_FREE_FRONTEND="${RUN_FREE_FRONTEND:-1}"
RUN_PAID_BALANCED="${RUN_PAID_BALANCED:-1}"
RUN_PAID_MAX="${RUN_PAID_MAX:-1}"
REQUIRE_PAID_SCENARIOS="${REQUIRE_PAID_SCENARIOS:-0}"
# Paid full-stack scenarios assert a real, reachable preview by default so a
# "completed" build with no working preview cannot pass as green. Override to 0
# only when intentionally probing a deployment that does not auto-start previews.
ASSERT_PREVIEW_READY="${ASSERT_PREVIEW_READY:-1}"
LOGIN_EMAIL="${LOGIN_EMAIL:-}"
LOGIN_USERNAME="${LOGIN_USERNAME:-}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-}"
LOGIN_FULL_NAME="${LOGIN_FULL_NAME:-APEX Canary}"
PROVIDER_MODE="${PROVIDER_MODE:-${APEX_PROVIDER_MODE:-platform}}"
ROLE_ASSIGNMENTS_JSON="${ROLE_ASSIGNMENTS_JSON:-${APEX_ROLE_ASSIGNMENTS_JSON:-}}"
PROVIDER_MODEL_OVERRIDES_JSON="${PROVIDER_MODEL_OVERRIDES_JSON:-${APEX_PROVIDER_MODEL_OVERRIDES_JSON:-}}"
APEX_BYOK_OLLAMA_ONLY="${APEX_BYOK_OLLAMA_ONLY:-${BYOK_OLLAMA_ONLY:-0}}"
APEX_LIVE_TEST_MODEL_PROFILE="${APEX_LIVE_TEST_MODEL_PROFILE:-${APEX_AI_TESTING_PROFILE:-platform}}"
PROJECT_NAME_PREFIX="${PROJECT_NAME_PREFIX:-platform-canary}"
# When set, each scenario writes its build/preview artifacts under this root in a
# per-scenario subdirectory (stable paths for CI artifact upload + the gate job).
ARTIFACT_ROOT="${ARTIFACT_ROOT:-}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ "$APEX_LIVE_TEST_MODEL_PROFILE" =~ ^(openrouter-free|openrouter-free-canary|free-openrouter)$ && "${APEX_SKIP_OPENROUTER_FREE_SOURCE:-0}" != "1" && -f "$SCRIPT_DIR/openrouter-free-canary-env.sh" ]]; then
  # shellcheck disable=SC1091
  source "$SCRIPT_DIR/openrouter-free-canary-env.sh"
  PROVIDER_MODE="${PROVIDER_MODE:-${APEX_PROVIDER_MODE:-platform}}"
  ROLE_ASSIGNMENTS_JSON="${ROLE_ASSIGNMENTS_JSON:-${APEX_ROLE_ASSIGNMENTS_JSON:-}}"
  PROVIDER_MODEL_OVERRIDES_JSON="${PROVIDER_MODEL_OVERRIDES_JSON:-${APEX_PROVIDER_MODEL_OVERRIDES_JSON:-}}"
  APEX_BYOK_OLLAMA_ONLY="${APEX_BYOK_OLLAMA_ONLY:-${BYOK_OLLAMA_ONLY:-0}}"
fi
# shellcheck source=lib/canary_report.sh
source "$SCRIPT_DIR/lib/canary_report.sh"

PASSED=0
FAILED=0
REQUIRED_SKIPPED=0
declare -a RESULTS=()

run_scenario() {
  local name="$1"
  local mode="$2"
  local power_mode="$3"
  local smoke_profile="$4"
  local poll_seconds="$5"
  local max_polls="$6"
  local scenario_login_email=""
  local scenario_login_username=""
  local scenario_login_password=""
  local scenario_login_full_name=""
  local scenario_assert_preview="0"
  local scenario_artifact_dir=""

  if [[ "$smoke_profile" == "paid_fullstack" ]]; then
    scenario_login_email="$LOGIN_EMAIL"
    scenario_login_username="$LOGIN_USERNAME"
    scenario_login_password="$LOGIN_PASSWORD"
    scenario_login_full_name="$LOGIN_FULL_NAME"
    scenario_assert_preview="$ASSERT_PREVIEW_READY"
  fi

  if [[ -n "$ARTIFACT_ROOT" ]]; then
    scenario_artifact_dir="$ARTIFACT_ROOT/$name"
  fi

  echo
  echo "== Scenario: $name =="
  local rc=0
  BASE_URL="$BASE_URL" \
  MODE="$mode" \
  POWER_MODE="$power_mode" \
  SMOKE_PROFILE="$smoke_profile" \
  EXPECT_STATUS="completed" \
  POLL_SECONDS="$poll_seconds" \
  MAX_POLLS="$max_polls" \
  ASSERT_PREVIEW_READY="$scenario_assert_preview" \
  ARTIFACT_DIR="$scenario_artifact_dir" \
  LOGIN_EMAIL="$scenario_login_email" \
  LOGIN_USERNAME="$scenario_login_username" \
  LOGIN_PASSWORD="$scenario_login_password" \
  LOGIN_FULL_NAME="$scenario_login_full_name" \
  PROVIDER_MODE="$PROVIDER_MODE" \
  ROLE_ASSIGNMENTS_JSON="$ROLE_ASSIGNMENTS_JSON" \
  PROVIDER_MODEL_OVERRIDES_JSON="$PROVIDER_MODEL_OVERRIDES_JSON" \
  APEX_BYOK_OLLAMA_ONLY="$APEX_BYOK_OLLAMA_ONLY" \
  APEX_LIVE_TEST_MODEL_PROFILE="$APEX_LIVE_TEST_MODEL_PROFILE" \
  PROJECT_NAME="${PROJECT_NAME_PREFIX}-${name}" \
  "$SCRIPT_DIR/run_platform_build_smoke.sh"
  rc=$?

  if [[ "$rc" -eq 0 ]]; then
    PASSED=$((PASSED + 1))
    RESULTS+=("PASS    $name")
  else
    FAILED=$((FAILED + 1))
    RESULTS+=("FAIL($rc) $name")
  fi
  return 0
}

if [[ "$RUN_FREE_FRONTEND" == "1" ]]; then
  run_scenario "free-fast" "fast" "fast" "free_frontend" "$POLL_SECONDS_FREE" "$MAX_POLLS_FREE"
fi

paid_enabled_count=0
[[ "$RUN_PAID_BALANCED" == "1" ]] && paid_enabled_count=$((paid_enabled_count + 1))
[[ "$RUN_PAID_MAX" == "1" ]] && paid_enabled_count=$((paid_enabled_count + 1))

if [[ -z "$LOGIN_EMAIL" || -z "$LOGIN_PASSWORD" ]]; then
  if [[ "$paid_enabled_count" -gt 0 ]]; then
    if [[ "$REQUIRE_PAID_SCENARIOS" == "1" ]]; then
      echo "PAID_CANARY_CREDENTIALS_MISSING: $paid_enabled_count required paid scenario(s) cannot run." >&2
      REQUIRED_SKIPPED=$((REQUIRED_SKIPPED + paid_enabled_count))
    else
      echo "Skipping $paid_enabled_count paid canary scenario(s): LOGIN_EMAIL/LOGIN_PASSWORD not provided (REQUIRE_PAID_SCENARIOS=0)."
      RESULTS+=("SKIP    paid scenarios (no credentials, not required)")
    fi
  fi
else
  if [[ "$RUN_PAID_BALANCED" == "1" ]]; then
    run_scenario "paid-balanced" "full" "balanced" "paid_fullstack" "$POLL_SECONDS_PAID" "$MAX_POLLS_PAID"
  fi
  if [[ "$RUN_PAID_MAX" == "1" ]]; then
    run_scenario "paid-max" "full" "max" "paid_fullstack" "$POLL_SECONDS_PAID" "$MAX_POLLS_PAID"
  fi
fi

echo
echo "===== CANARY MATRIX RESULTS ====="
for line in ${RESULTS[@]+"${RESULTS[@]}"}; do
  echo "  $line"
done
echo "================================="

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo "## Platform canary matrix"
    echo
    echo "| result | scenario |"
    echo "| --- | --- |"
    for line in ${RESULTS[@]+"${RESULTS[@]}"}; do
      printf '| %s | %s |\n' "${line%% *}" "${line#* }"
    done
    echo
  } >>"$GITHUB_STEP_SUMMARY" 2>/dev/null || true
fi

verdict="$(canary_matrix_verdict "$PASSED" "$FAILED" "$REQUIRED_SKIPPED")"
verdict_rc=$?
echo "$verdict"
exit "$verdict_rc"
