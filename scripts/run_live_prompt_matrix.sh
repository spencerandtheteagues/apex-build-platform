#!/usr/bin/env bash
set -euo pipefail

APEX_LIVE_TEST_MODEL_PROFILE="${APEX_LIVE_TEST_MODEL_PROFILE:-ollama-credit-saver}"
if [[ "$APEX_LIVE_TEST_MODEL_PROFILE" == "ollama-credit-saver" && "${APEX_SKIP_OLLAMA_CREDIT_SAVER_SOURCE:-0}" != "1" && -f scripts/ollama-credit-saver-env.sh ]]; then
  # shellcheck disable=SC1091
  source scripts/ollama-credit-saver-env.sh
fi

BASE_URL="${BASE_URL:-https://api.apex-build.dev/api/v1}"
PROMPT_DIR="${PROMPT_DIR:-prompts/canary}"
PROMPT_FILES="${PROMPT_FILES:-}"
POWER_MODES="${POWER_MODES:-balanced}"
MODE="${MODE:-full}"
POLL_SECONDS="${POLL_SECONDS:-15}"
MAX_POLLS="${MAX_POLLS:-220}"
PREVIEW_STABILITY_SECONDS="${PREVIEW_STABILITY_SECONDS:-12}"
PREVIEW_STABILITY_POLL_MS="${PREVIEW_STABILITY_POLL_MS:-1000}"
PROJECT_NAME_PREFIX="${PROJECT_NAME_PREFIX:-prompt-matrix-canary}"
ARTIFACT_ROOT="${ARTIFACT_ROOT:-/tmp/apex-live-prompt-matrix-$(date -u +%Y%m%dT%H%M%SZ)}"
LOGIN_EMAIL="${LOGIN_EMAIL:-}"
LOGIN_USERNAME="${LOGIN_USERNAME:-}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-}"
LOGIN_FULL_NAME="${LOGIN_FULL_NAME:-APEX Matrix Canary}"
AUTO_REGISTER="${AUTO_REGISTER:-0}"
STOP_ON_AUTH_PREREQ="${STOP_ON_AUTH_PREREQ:-1}"
DRY_RUN="${DRY_RUN:-0}"

discover_prompts() {
  if [[ -n "$PROMPT_FILES" ]]; then
    # shellcheck disable=SC2086
    printf '%s\n' $PROMPT_FILES
    return
  fi
  find "$PROMPT_DIR" -maxdepth 1 -type f -name '*.md' | sort
}

slug_from_prompt() {
  local prompt_file="$1"
  local base
  base="$(basename "$prompt_file" .md)"
  printf '%s\n' "$base" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//'
}

prompt_files=()
while IFS= read -r prompt_file; do
  prompt_files+=("$prompt_file")
done < <(discover_prompts)

if [[ "${#prompt_files[@]}" -eq 0 ]]; then
  echo "PROMPT_MATRIX_EMPTY: no prompt files found" >&2
  exit 1
fi

for prompt_file in "${prompt_files[@]}"; do
  if [[ ! -f "$prompt_file" ]]; then
    echo "PROMPT_MATRIX_PROMPT_MISSING: $prompt_file" >&2
    exit 1
  fi
done

if [[ "$DRY_RUN" == "1" ]]; then
  echo "PROMPT_MATRIX_DRY_RUN"
  echo "BASE_URL=$BASE_URL"
  echo "APEX_LIVE_TEST_MODEL_PROFILE=${APEX_LIVE_TEST_MODEL_PROFILE:-}"
  echo "POWER_MODES=$POWER_MODES"
  echo "ARTIFACT_ROOT=$ARTIFACT_ROOT"
  for prompt_file in "${prompt_files[@]}"; do
    echo "PROMPT=$(slug_from_prompt "$prompt_file") path=$prompt_file"
  done
  exit 0
fi

if [[ -z "$LOGIN_PASSWORD" && "$AUTO_REGISTER" != "1" ]]; then
  echo "PROMPT_MATRIX_CREDENTIALS_MISSING: LOGIN_PASSWORD is required" >&2
  exit 1
fi

mkdir -p "$ARTIFACT_ROOT"
summary_jsonl="$ARTIFACT_ROOT/results.jsonl"
: > "$summary_jsonl"

overall_status=0

summarize_run() {
  local prompt_slug="$1"
  local prompt_file="$2"
  local power_mode="$3"
  local run_artifacts="$4"
  local exit_code="$5"

  node - "$prompt_slug" "$prompt_file" "$power_mode" "$run_artifacts" "$exit_code" <<'NODE'
const fs = require('node:fs')
const path = require('node:path')

const [promptSlug, promptFile, powerMode, artifactDir, exitCodeRaw] = process.argv.slice(2)
const exitCode = Number(exitCodeRaw || 0)

function readJSON(name) {
  try {
    return JSON.parse(fs.readFileSync(path.join(artifactDir, name), 'utf8'))
  } catch {
    return null
  }
}

function readTail(name, maxChars = 4000) {
  try {
    const text = fs.readFileSync(path.join(artifactDir, name), 'utf8')
    return text.slice(Math.max(0, text.length - maxChars))
  } catch {
    return ''
  }
}

const start = readJSON('build-start.json')
const detail = readJSON('build-detail.json')
const previewStart = readJSON('preview-start.json')
const previewProof = readJSON('preview-proof.json')
const logTail = readTail('harness.log')

process.stdout.write(`${JSON.stringify({
  prompt_slug: promptSlug,
  prompt_file: promptFile,
  power_mode: powerMode,
  passed: exitCode === 0,
  exit_code: exitCode,
  artifact_dir: artifactDir,
  build_id: detail?.id || detail?.build_id || start?.build_id || start?.buildID || '',
  project_id: detail?.project_id || previewStart?.project_id || '',
  status: detail?.status || '',
  progress: detail?.progress ?? null,
  quality_gate_passed: detail?.quality_gate_passed ?? null,
  quality_gate_status: detail?.quality_gate_status || '',
  preview_url: previewProof?.url || previewStart?.proxy_url || previewStart?.preview?.url || '',
  preview_screenshot: previewProof?.screenshot || '',
  body_length: previewProof?.body_length ?? null,
  console_error_count: Array.isArray(previewProof?.console_errors) ? previewProof.console_errors.length : null,
  page_error_count: Array.isArray(previewProof?.page_errors) ? previewProof.page_errors.length : null,
  stability_seconds: previewProof?.stability?.seconds ?? null,
  stability_sample_count: Array.isArray(previewProof?.stability?.samples) ? previewProof.stability.samples.length : null,
  main_frame_navigations_after_stable_start: previewProof?.stability?.main_frame_navigations_after_stable_start ?? null,
  failure_tail: exitCode === 0 ? '' : logTail,
})}\n`)
NODE
}

for prompt_file in "${prompt_files[@]}"; do
  prompt_slug="$(slug_from_prompt "$prompt_file")"
  for power_mode in $POWER_MODES; do
    run_artifacts="$ARTIFACT_ROOT/$prompt_slug/$power_mode"
    mkdir -p "$run_artifacts"

    echo
    echo "== Prompt matrix: $prompt_slug / $power_mode =="
    set +e
    BASE_URL="$BASE_URL" \
    MODE="$MODE" \
    POWER_MODE="$power_mode" \
    PROMPT_FILE="$prompt_file" \
    POLL_SECONDS="$POLL_SECONDS" \
    MAX_POLLS="$MAX_POLLS" \
    PREVIEW_STABILITY_SECONDS="$PREVIEW_STABILITY_SECONDS" \
    PREVIEW_STABILITY_POLL_MS="$PREVIEW_STABILITY_POLL_MS" \
    PROJECT_NAME="${PROJECT_NAME_PREFIX}-${prompt_slug}-${power_mode}-$(date -u +%Y%m%dT%H%M%SZ)" \
    ARTIFACT_DIR="$run_artifacts" \
    LOGIN_EMAIL="$LOGIN_EMAIL" \
    LOGIN_USERNAME="$LOGIN_USERNAME" \
    LOGIN_PASSWORD="$LOGIN_PASSWORD" \
    LOGIN_FULL_NAME="$LOGIN_FULL_NAME" \
    AUTO_REGISTER="$AUTO_REGISTER" \
    node scripts/run_live_golden_build.mjs "$prompt_file" 2>&1 | tee "$run_artifacts/harness.log"
    exit_code="${PIPESTATUS[0]}"
    set -e

    summarize_run "$prompt_slug" "$prompt_file" "$power_mode" "$run_artifacts" "$exit_code" >> "$summary_jsonl"
    if [[ "$exit_code" -ne 0 ]]; then
      overall_status=1
      if [[ "$STOP_ON_AUTH_PREREQ" == "1" ]] && grep -Eq 'EMAIL_VERIFICATION_REQUIRED|email_not_verified|AUTH_RATE_LIMIT_EXCEEDED|Too many authentication attempts' "$run_artifacts/harness.log"; then
        echo "PROMPT_MATRIX_AUTH_PREREQUISITE_FAILED: verified canary credentials are required before continuing the live matrix." >&2
        break 2
      fi
    fi
  done
done

node - "$summary_jsonl" "$ARTIFACT_ROOT/summary.json" "$ARTIFACT_ROOT/summary.md" <<'NODE'
const fs = require('node:fs')

const [jsonlPath, summaryJSONPath, summaryMDPath] = process.argv.slice(2)
const rows = fs.readFileSync(jsonlPath, 'utf8')
  .split(/\r?\n/)
  .filter(Boolean)
  .map(line => JSON.parse(line))

const byPrompt = {}
for (const row of rows) {
  byPrompt[row.prompt_slug] ||= { total: 0, passed: 0, failed: 0 }
  byPrompt[row.prompt_slug].total += 1
  if (row.passed) byPrompt[row.prompt_slug].passed += 1
  else byPrompt[row.prompt_slug].failed += 1
}

const summary = {
  generated_at: new Date().toISOString(),
  passed: rows.every(row => row.passed),
  total: rows.length,
  passed_count: rows.filter(row => row.passed).length,
  failed_count: rows.filter(row => !row.passed).length,
  by_prompt: byPrompt,
  results: rows,
}

fs.writeFileSync(summaryJSONPath, `${JSON.stringify(summary, null, 2)}\n`)

const lines = [
  '# Live Prompt Reliability Matrix',
  '',
  `Generated: ${summary.generated_at}`,
  '',
  '| Prompt | Mode | Result | Build | Project | Progress | Gate | Console | Page | Stability | Screenshot |',
  '| --- | --- | --- | --- | --- | ---: | --- | ---: | ---: | --- | --- |',
]

for (const row of rows) {
  const stability = row.stability_seconds == null
    ? ''
    : `${row.stability_seconds}s / ${row.stability_sample_count ?? 0} samples / ${row.main_frame_navigations_after_stable_start ?? 0} navs`
  lines.push([
    row.prompt_slug,
    row.power_mode,
    row.passed ? 'PASS' : 'FAIL',
    row.build_id || '',
    row.project_id || '',
    row.progress ?? '',
    row.quality_gate_passed === true ? 'passed' : (row.quality_gate_status || ''),
    row.console_error_count ?? '',
    row.page_error_count ?? '',
    stability,
    row.preview_screenshot || '',
  ].map(value => String(value).replace(/\|/g, '\\|')).join(' | ').replace(/^/, '| ').replace(/$/, ' |'))
}

fs.writeFileSync(summaryMDPath, `${lines.join('\n')}\n`)
NODE

echo
echo "PROMPT_MATRIX_SUMMARY=$ARTIFACT_ROOT/summary.md"

if [[ "$overall_status" -ne 0 ]]; then
  echo "PROMPT_MATRIX_FAILED" >&2
  exit "$overall_status"
fi

echo "PROMPT_MATRIX_PASSED"
