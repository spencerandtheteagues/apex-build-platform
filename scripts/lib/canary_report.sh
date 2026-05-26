#!/usr/bin/env bash
# canary_report.sh — sourceable helpers for actionable canary summaries and
# false-green guardrails. These functions perform NO network calls and have no
# side effects beyond stdout and (optionally) $GITHUB_STEP_SUMMARY, so they can
# be unit-tested against JSON fixtures without live secrets.
#
# Source this file; do not execute it directly.

# canary_jq_first <file> <jqpath> [jqpath...]
# Prints the first non-empty/non-null value among the given jq paths. Missing
# file or invalid JSON yields empty output (never an error exit).
canary_jq_first() {
  local file="$1"
  shift
  [[ -f "$file" ]] || return 0
  local path val
  for path in "$@"; do
    val="$(jq -r "($path) // empty" "$file" 2>/dev/null || true)"
    if [[ -n "$val" && "$val" != "null" ]]; then
      printf '%s' "$val"
      return 0
    fi
  done
  return 0
}

# canary_provider_models <build_detail_file>
# Extracts provider/model routing pairs discovered in orchestration/agent/
# reliability subtrees. Restricted to records that carry a routing-context
# sibling (role/agent/phase/stage) so echoed prompt/request payloads that merely
# mention a "provider" or "model" string cannot pollute the summary.
canary_provider_models() {
  local file="$1"
  [[ -f "$file" ]] || return 0
  jq -r '
    [ (.orchestration // {}),
      (.agents // []),
      (.orchestration.agents // []),
      (.reliability_summary // {}),
      (.orchestration.reliability_summary // {})
      | .. | objects
      | select(has("provider") or has("model"))
      | select(has("role") or has("agent") or has("agent_role") or has("phase") or has("stage"))
      | "\((.provider // .ai_provider // "?"))/\((.model // .model_id // .model_name // "?"))"
    ]
    | map(select(. != "?/?"))
    | unique
    | join(", ")
  ' "$file" 2>/dev/null || true
}

# canary_preview_status <preview_status_file>
# Prints a human-readable preview status string derived from a /preview/status
# response. Mirrors the golden runner's readiness definition for the summary.
canary_preview_status() {
  local file="$1"
  if [[ ! -f "$file" ]]; then
    printf 'unknown (no preview status captured)'
    return 0
  fi
  local http_status active url
  http_status="$(jq -r '._http_status // empty' "$file" 2>/dev/null || true)"
  if [[ "$http_status" == "404" ]]; then
    printf 'not_started (preview/status 404)'
    return 0
  fi
  active="$(jq -r '.preview.active // false' "$file" 2>/dev/null || echo false)"
  url="$(jq -r '(.proxy_url // .preview.url // .url) // empty' "$file" 2>/dev/null || true)"
  if [[ "$active" == "true" && -n "$url" ]]; then
    printf 'active url=%s' "$url"
  else
    printf 'not_ready (active=%s url=%s)' "$active" "${url:-none}"
  fi
}

# canary_preview_ready <preview_status_file>
# Returns 0 only when the preview is active AND has a URL (golden-runner parity).
# A 404 / missing file / active-without-url all return non-zero. Used by the
# opt-in preview guardrail to prevent "build completed but no working preview"
# false greens.
canary_preview_ready() {
  local file="$1"
  [[ -f "$file" ]] || return 1
  local http_status active url
  http_status="$(jq -r '._http_status // empty' "$file" 2>/dev/null || true)"
  [[ "$http_status" == "404" ]] && return 1
  active="$(jq -r '.preview.active // false' "$file" 2>/dev/null || echo false)"
  url="$(jq -r '(.proxy_url // .preview.url // .url) // empty' "$file" 2>/dev/null || true)"
  [[ "$active" == "true" && -n "$url" ]]
}

# canary_report_block <label> <build_id> <terminal_state> <provider_models> \
#                      <quality_gate> <preview_status> <artifact_dir>
# Prints a greppable, actionable scenario report. Empty values render as "?" so
# the block always emits, even for malformed/early-failure builds. When
# $GITHUB_STEP_SUMMARY is set and writable, also appends a markdown section.
canary_report_block() {
  local label="${1:-?}"
  local build_id="${2:-?}"
  local terminal_state="${3:-?}"
  local provider_models="${4:-?}"
  local quality_gate="${5:-?}"
  local preview_status="${6:-?}"
  local artifact_dir="${7:-?}"
  : "${build_id:=?}" "${terminal_state:=?}" "${provider_models:=?}" "${quality_gate:=?}" "${preview_status:=?}" "${artifact_dir:=?}"

  local artifacts_listing="(none)"
  if [[ "$artifact_dir" != "?" && -d "$artifact_dir" ]]; then
    artifacts_listing="$(ls -1 "$artifact_dir" 2>/dev/null | tr '\n' ' ' | sed 's/ *$//')"
    [[ -z "$artifacts_listing" ]] && artifacts_listing="(empty)"
  fi

  cat <<EOF
===== CANARY SCENARIO REPORT =====
scenario:        ${label}
build_id:        ${build_id}
terminal_state:  ${terminal_state}
provider/model:  ${provider_models:-?}
quality_gate:    ${quality_gate}
preview:         ${preview_status}
artifact_dir:    ${artifact_dir}
artifacts:       ${artifacts_listing}
==================================
EOF

  if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
    {
      printf '### Canary scenario: `%s`\n\n' "$label"
      printf '| field | value |\n| --- | --- |\n'
      printf '| build_id | `%s` |\n' "$build_id"
      printf '| terminal_state | `%s` |\n' "$terminal_state"
      printf '| provider/model | %s |\n' "${provider_models:-?}"
      printf '| quality_gate | `%s` |\n' "$quality_gate"
      printf '| preview | %s |\n' "$preview_status"
      printf '| artifact_dir | `%s` |\n' "$artifact_dir"
      printf '| artifacts | %s |\n\n' "$artifacts_listing"
    } >>"$GITHUB_STEP_SUMMARY" 2>/dev/null || true
  fi
}

# canary_matrix_verdict <passed> <failed> <required_skipped>
# Encodes false-green prevention as a single decision point. Prints a verdict
# token and returns non-zero unless the run is genuinely, fully green:
#   - any failure                -> CANARY_MATRIX_FAILED (1)
#   - a required scenario skipped -> CANARY_MATRIX_INCOMPLETE_REQUIRED_SCENARIOS_SKIPPED (1)
#   - zero scenarios actually ran -> CANARY_MATRIX_NO_SCENARIOS_RAN (1)
#   - otherwise                   -> CANARY_MATRIX_PASSED (0)
canary_matrix_verdict() {
  local passed="${1:-0}"
  local failed="${2:-0}"
  local required_skipped="${3:-0}"

  if [[ "$failed" -gt 0 ]]; then
    echo "CANARY_MATRIX_FAILED failed=${failed} passed=${passed} required_skipped=${required_skipped}"
    return 1
  fi
  if [[ "$required_skipped" -gt 0 ]]; then
    echo "CANARY_MATRIX_INCOMPLETE_REQUIRED_SCENARIOS_SKIPPED required_skipped=${required_skipped} passed=${passed}"
    return 1
  fi
  if [[ "$passed" -le 0 ]]; then
    echo "CANARY_MATRIX_NO_SCENARIOS_RAN"
    return 1
  fi
  echo "CANARY_MATRIX_PASSED passed=${passed}"
  return 0
}
