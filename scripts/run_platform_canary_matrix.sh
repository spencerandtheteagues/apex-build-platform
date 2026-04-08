#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-https://api.apex-build.dev/api/v1}"
POLL_SECONDS_FREE="${POLL_SECONDS_FREE:-10}"
MAX_POLLS_FREE="${MAX_POLLS_FREE:-120}"
POLL_SECONDS_PAID="${POLL_SECONDS_PAID:-15}"
MAX_POLLS_PAID="${MAX_POLLS_PAID:-180}"
RUN_FREE_FRONTEND="${RUN_FREE_FRONTEND:-1}"
RUN_PAID_BALANCED="${RUN_PAID_BALANCED:-1}"
RUN_PAID_MAX="${RUN_PAID_MAX:-1}"
REQUIRE_PAID_SCENARIOS="${REQUIRE_PAID_SCENARIOS:-0}"
LOGIN_EMAIL="${LOGIN_EMAIL:-}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-}"
LOGIN_FULL_NAME="${LOGIN_FULL_NAME:-APEX Canary}"
PROJECT_NAME_PREFIX="${PROJECT_NAME_PREFIX:-platform-canary}"

run_scenario() {
  local name="$1"
  local mode="$2"
  local power_mode="$3"
  local smoke_profile="$4"
  local poll_seconds="$5"
  local max_polls="$6"

  echo
  echo "== Scenario: $name =="
  BASE_URL="$BASE_URL" \
  MODE="$mode" \
  POWER_MODE="$power_mode" \
  SMOKE_PROFILE="$smoke_profile" \
  EXPECT_STATUS="completed" \
  POLL_SECONDS="$poll_seconds" \
  MAX_POLLS="$max_polls" \
  LOGIN_EMAIL="$LOGIN_EMAIL" \
  LOGIN_PASSWORD="$LOGIN_PASSWORD" \
  LOGIN_FULL_NAME="$LOGIN_FULL_NAME" \
  PROJECT_NAME="${PROJECT_NAME_PREFIX}-${name}" \
  ./scripts/run_platform_build_smoke.sh
}

if [[ "$RUN_FREE_FRONTEND" == "1" ]]; then
  run_scenario "free-fast" "fast" "fast" "free_frontend" "$POLL_SECONDS_FREE" "$MAX_POLLS_FREE"
fi

if [[ -z "$LOGIN_EMAIL" || -z "$LOGIN_PASSWORD" ]]; then
  if [[ "$REQUIRE_PAID_SCENARIOS" == "1" && ( "$RUN_PAID_BALANCED" == "1" || "$RUN_PAID_MAX" == "1" ) ]]; then
    echo "PAID_CANARY_CREDENTIALS_MISSING" >&2
    exit 1
  fi

  if [[ "$RUN_PAID_BALANCED" == "1" || "$RUN_PAID_MAX" == "1" ]]; then
    echo "Skipping paid canary scenarios because LOGIN_EMAIL/LOGIN_PASSWORD were not provided."
  fi
  exit 0
fi

if [[ "$RUN_PAID_BALANCED" == "1" ]]; then
  run_scenario "paid-balanced" "full" "balanced" "paid_fullstack" "$POLL_SECONDS_PAID" "$MAX_POLLS_PAID"
fi

if [[ "$RUN_PAID_MAX" == "1" ]]; then
  run_scenario "paid-max" "full" "max" "paid_fullstack" "$POLL_SECONDS_PAID" "$MAX_POLLS_PAID"
fi

echo
echo "CANARY_MATRIX_PASSED"
