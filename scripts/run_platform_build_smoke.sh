#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080/api/v1}"
MODE="${MODE:-fast}"
PROJECT_NAME="${PROJECT_NAME:-agency-ops-platform}"
POLL_SECONDS="${POLL_SECONDS:-10}"
MAX_POLLS="${MAX_POLLS:-120}"
SMOKE_PROFILE="${SMOKE_PROFILE:-free_frontend}"
LOGIN_EMAIL="${LOGIN_EMAIL:-}"
LOGIN_USERNAME="${LOGIN_USERNAME:-}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-}"
LOGIN_FULL_NAME="${LOGIN_FULL_NAME:-Platform Test}"
EXPECT_STATUS="${EXPECT_STATUS:-completed}"
ASSERT_FRONTEND_FILES="${ASSERT_FRONTEND_FILES:-}"

if [[ -n "${POWER_MODE:-}" ]]; then
  EFFECTIVE_POWER_MODE="$POWER_MODE"
elif [[ "$SMOKE_PROFILE" == "paid_fullstack" ]]; then
  EFFECTIVE_POWER_MODE="balanced"
else
  EFFECTIVE_POWER_MODE="fast"
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 1
fi

prompt_file=""
cookie_jar=""
TOKEN=""
CSRF_TOKEN=""
cleanup() {
  if [[ -n "${prompt_file}" && -f "${prompt_file}" ]]; then
    rm -f "${prompt_file}"
  fi
  if [[ -n "${cookie_jar}" && -f "${cookie_jar}" ]]; then
    rm -f "${cookie_jar}"
  fi
  rm -f /tmp/apex_refresh.json
}
trap cleanup EXIT

cookie_jar="$(mktemp)"
CSRF_TOKEN=""

fetch_csrf_token() {
  local raw
  raw="$(curl -sS -c "$cookie_jar" -b "$cookie_jar" "$BASE_URL/csrf-token" 2>/dev/null || true)"
  local tok
  tok="$(jq -r '.token // empty' <<<"$raw" 2>/dev/null || true)"
  if [[ -n "$tok" && "$tok" != "null" ]]; then
    CSRF_TOKEN="$tok"
  fi
}

refresh_auth_args() {
  auth_args=(-b "$cookie_jar" -c "$cookie_jar")
  if [[ -n "${TOKEN}" && "${TOKEN}" != "null" ]]; then
    auth_args+=(-H "Authorization: Bearer $TOKEN")
  fi
  if [[ -n "${CSRF_TOKEN}" && "${CSRF_TOKEN}" != "null" ]]; then
    auth_args+=(-H "X-CSRF-Token: $CSRF_TOKEN")
  fi
}

json_string_field() {
  local file="$1"
  local expr="$2"
  jq -r "$expr // empty" "$file" 2>/dev/null || true
}

json_bool_field() {
  local file="$1"
  local expr="$2"
  jq -r "if ($expr) == true then \"true\" elif ($expr) == false then \"false\" else \"\" end" "$file" 2>/dev/null || true
}

require_true_field() {
  local file="$1"
  local expr="$2"
  local label="$3"
  local actual
  actual="$(json_bool_field "$file" "$expr")"
  if [[ "$actual" != "true" ]]; then
    echo "ASSERTION_FAILED: $label expected true, got '${actual:-unset}'" >&2
    jq '{status,progress,error,quality_gate_passed,current_phase,policy_state,capability_state,build_contract,approvals,blockers,files_count}' "$file" 2>/dev/null || cat "$file"
    exit 1
  fi
}

require_false_field() {
  local file="$1"
  local expr="$2"
  local label="$3"
  local actual
  actual="$(json_bool_field "$file" "$expr")"
  if [[ "$actual" == "true" ]]; then
    echo "ASSERTION_FAILED: $label expected false, got true" >&2
    jq '{status,progress,error,quality_gate_passed,current_phase,policy_state,capability_state,build_contract,approvals,blockers,files_count}' "$file" 2>/dev/null || cat "$file"
    exit 1
  fi
}

require_absent_array_match() {
  local file="$1"
  local expr="$2"
  local target="$3"
  local label="$4"
  if ! jq -e --arg target "$target" "$expr | index(\$target) | not" "$file" >/dev/null 2>&1; then
    echo "ASSERTION_FAILED: $label unexpectedly contains '$target'" >&2
    jq '{approvals,blockers,policy_state,capability_state,build_contract}' "$file" 2>/dev/null || cat "$file"
    exit 1
  fi
}

require_no_actionable_approval() {
  local file="$1"
  local target="$2"
  local label="$3"
  if ! jq -e --arg target "$target" '
    (.approvals // [])
    | map(select(
        (.code // .type // .id // "") == $target
        and (
          (.required // false) == true
          or (
            (.status // "") as $status
            | $status != ""
            and $status != "not_required"
            and $status != "satisfied"
            and $status != "approved"
            and $status != "resolved"
          )
        )
      ))
    | length == 0
  ' "$file" >/dev/null 2>&1; then
    echo "ASSERTION_FAILED: $label unexpectedly requires approval '$target'" >&2
    jq '{approvals,blockers,policy_state,capability_state,build_contract}' "$file" 2>/dev/null || cat "$file"
    exit 1
  fi
}

require_file_path() {
  local file="$1"
  local path="$2"
  if ! jq -e --arg path "$path" '(.files // []) | map(.path // "") | index($path) != null' "$file" >/dev/null 2>&1; then
    echo "ASSERTION_FAILED: expected generated file '$path'" >&2
    jq '{status,files_count,files}' "$file" 2>/dev/null || cat "$file"
    exit 1
  fi
}

assert_completed_build_contract() {
  local detail_file="$1"
  local completed_file="$2"

  local detail_status completed_status detail_files_count completed_files_count
  detail_status="$(json_string_field "$detail_file" '.status')"
  completed_status="$(json_string_field "$completed_file" '.status')"
  detail_files_count="$(json_string_field "$detail_file" '(.files_count // ((.files // []) | length))')"
  completed_files_count="$(json_string_field "$completed_file" '(.files_count // ((.files // []) | length))')"

  if [[ "$detail_status" != "$EXPECT_STATUS" ]]; then
    echo "ASSERTION_FAILED: build detail status expected '$EXPECT_STATUS', got '${detail_status:-unset}'" >&2
    jq '{status,progress,error,current_phase,quality_gate_passed}' "$detail_file" 2>/dev/null || cat "$detail_file"
    exit 1
  fi
  if [[ "$completed_status" != "$EXPECT_STATUS" ]]; then
    echo "ASSERTION_FAILED: completed-build status expected '$EXPECT_STATUS', got '${completed_status:-unset}'" >&2
    jq '{status,progress,error,current_phase,quality_gate_passed}' "$completed_file" 2>/dev/null || cat "$completed_file"
    exit 1
  fi

  require_true_field "$detail_file" '.quality_gate_passed' 'build detail quality_gate_passed'
  require_true_field "$completed_file" '.quality_gate_passed' 'completed build quality_gate_passed'

  if [[ "${detail_files_count:-0}" -lt 1 ]]; then
    echo "ASSERTION_FAILED: build detail files_count must be > 0, got '${detail_files_count:-unset}'" >&2
    jq '{status,files_count,files}' "$detail_file" 2>/dev/null || cat "$detail_file"
    exit 1
  fi
  if [[ "${completed_files_count:-0}" -lt 1 ]]; then
    echo "ASSERTION_FAILED: completed build files_count must be > 0, got '${completed_files_count:-unset}'" >&2
    jq '{status,files_count,files}' "$completed_file" 2>/dev/null || cat "$completed_file"
    exit 1
  fi

  if [[ "$SMOKE_PROFILE" == "free_frontend" ]]; then
    require_false_field "$detail_file" '.upgrade_required' 'free frontend detail upgrade_required'
    require_false_field "$completed_file" '.upgrade_required' 'free frontend completed upgrade_required'
    require_false_field "$detail_file" '.capability_state.requires_backend_runtime' 'free frontend detail requires_backend_runtime'
    require_false_field "$detail_file" '.capability_state.requires_database' 'free frontend detail requires_database'
    require_false_field "$detail_file" '.capability_state.requires_storage' 'free frontend detail requires_storage'
    require_false_field "$detail_file" '.capability_state.requires_jobs' 'free frontend detail requires_jobs'
    require_false_field "$detail_file" '.capability_state.requires_billing' 'free frontend detail requires_billing'
    require_false_field "$detail_file" '.capability_state.requires_realtime' 'free frontend detail requires_realtime'
    require_false_field "$detail_file" '.capability_state.requires_publish' 'free frontend detail requires_publish'
    require_no_actionable_approval "$detail_file" 'full_stack_upgrade' 'free frontend approvals'
    require_no_actionable_approval "$detail_file" 'plan_upgrade_acknowledgement' 'free frontend approvals'
    require_no_actionable_approval "$detail_file" 'file_storage' 'free frontend approvals'
    require_absent_array_match "$detail_file" '(.blockers // []) | map(.id // .code // .type // "")' 'plan-upgrade-required' 'free frontend blockers'

    local delivery_mode
    delivery_mode="$(json_string_field "$detail_file" '.build_contract.delivery_mode')"
    if [[ -n "$delivery_mode" && "$delivery_mode" != "frontend_preview_only" ]]; then
      echo "ASSERTION_FAILED: free frontend delivery mode expected frontend_preview_only, got '$delivery_mode'" >&2
      jq '{build_contract,policy_state,capability_state}' "$detail_file" 2>/dev/null || cat "$detail_file"
      exit 1
    fi

    local expect_frontend_files
    expect_frontend_files="${ASSERT_FRONTEND_FILES:-1}"
    if [[ "$expect_frontend_files" == "1" ]]; then
      require_file_path "$detail_file" "index.html"
      require_file_path "$detail_file" "package.json"
      require_file_path "$detail_file" "src/main.tsx"
      require_file_path "$detail_file" "src/App.tsx"
    fi
  fi

  if [[ "$SMOKE_PROFILE" == "paid_fullstack" ]]; then
    require_false_field "$detail_file" '.upgrade_required' 'paid fullstack detail upgrade_required'
    require_false_field "$completed_file" '.upgrade_required' 'paid fullstack completed upgrade_required'

    local paid_delivery_mode
    paid_delivery_mode="$(json_string_field "$detail_file" '.build_contract.delivery_mode')"
    if [[ "$paid_delivery_mode" == "frontend_preview_only" ]]; then
      echo "ASSERTION_FAILED: paid full-stack canary should not finish in frontend_preview_only mode" >&2
      jq '{build_contract,policy_state,capability_state}' "$detail_file" 2>/dev/null || cat "$detail_file"
      exit 1
    fi
  fi
}

login_or_exit() {
  local login_payload
  login_payload="$(jq -n --arg u "$USER" --arg e "$EMAIL" --arg p "$PASS" '{username:$u,email:$e,password:$p}')"
  curl -sS -c "$cookie_jar" -b "$cookie_jar" -X POST "$BASE_URL/auth/login" -H "Content-Type: application/json" -d "$login_payload" >/tmp/apex_login.json

  TOKEN="$(jq -r '.access_token // .token // .data.access_token // .tokens.access_token // empty' /tmp/apex_login.json)"
  LOGIN_ERROR="$(jq -r '.error // empty' /tmp/apex_login.json)"
  HAS_SESSION_COOKIE="0"
  if [[ -s "$cookie_jar" ]] && grep -q $'apex_access_token\t' "$cookie_jar"; then
    HAS_SESSION_COOKIE="1"
  fi
  if [[ "$LOGIN_ERROR" != "" && ( -z "$TOKEN" || "$TOKEN" == "null" ) && "$HAS_SESSION_COOKIE" != "1" ]]; then
    echo "LOGIN_FAILED"
    cat /tmp/apex_login.json
    exit 1
  fi

  refresh_auth_args
}

refresh_or_relogin() {
  fetch_csrf_token
  refresh_auth_args

  curl -sS "${auth_args[@]}" -X POST "$BASE_URL/auth/refresh" \
    -H "Content-Type: application/json" \
    -d '{}' >/tmp/apex_refresh.json || true

  local refreshed_token refresh_error
  refreshed_token="$(jq -r '.access_token // .token // .data.access_token // .tokens.access_token // empty' /tmp/apex_refresh.json 2>/dev/null || true)"
  refresh_error="$(jq -r '.error // empty' /tmp/apex_refresh.json 2>/dev/null || true)"

  if [[ -n "$refreshed_token" && "$refreshed_token" != "null" ]]; then
    TOKEN="$refreshed_token"
    refresh_auth_args
    return 0
  fi

  if [[ -z "$refresh_error" ]]; then
    refresh_auth_args
    return 0
  fi

  login_or_exit
}

if [[ $# -gt 0 ]]; then
  prompt_file="$1"
else
  prompt_file="$(mktemp)"
  if [[ "$SMOKE_PROFILE" == "paid_fullstack" ]]; then
    cat >"$prompt_file" <<'EOF'
Build a production-style multi-tenant agency operations platform with:
- React + TypeScript frontend suitable for preview pane testing
- Node/TypeScript backend API with authentication and role-based access (admin/manager/agent)
- PostgreSQL persistence for tenants, users, clients, projects, tasks, invoices, payments, activity logs
- Dashboard with KPIs, task board, project list/detail, invoice list/detail, client management
- Seed data for one tenant with multiple users and clients so the UI is interactive immediately
- REST endpoints used by frontend (not mocked)
- Clean monorepo structure is allowed, but it must build successfully and run in preview
- Include runnable scripts and avoid fake package names
EOF
  else
    cat >"$prompt_file" <<'EOF'
Build a polished frontend-only client dashboard called PulseBoard using React 18, Vite, and Tailwind CSS with:
- a responsive dark modern UI that works well in the preview pane
- a dashboard home with KPI cards, trend widgets, an activity feed, and a highlighted primary action
- a clients page with searchable cards, filters, empty states, and detail panels
- a projects page with kanban-style status columns and clear progress visuals
- a settings page with profile, notifications, and theme sections
- realistic seed content in the UI so the preview feels complete immediately
- strong loading, empty, and error states
- reusable components and a clean file structure
- no backend, no database, and no fake API requirements in this free-tier preview pass
EOF
  fi
fi

PROMPT="$(cat "$prompt_file")"
SUFFIX="$(date +%s)"
USER="${LOGIN_USERNAME:-platform${SUFFIX}}"
EMAIL="${LOGIN_EMAIL:-${USER}@example.com}"
PASS="${LOGIN_PASSWORD:-Passw0rd!Passw0rd!}"

if [[ -z "$LOGIN_EMAIL" || -z "$LOGIN_PASSWORD" ]]; then
  reg_payload="$(jq -n --arg u "$USER" --arg e "$EMAIL" --arg p "$PASS" --arg n "$LOGIN_FULL_NAME" \
    '{username:$u,email:$e,password:$p,full_name:$n,accept_legal_terms:true}')"
  curl -sS -c "$cookie_jar" -b "$cookie_jar" -X POST "$BASE_URL/auth/register" -H "Content-Type: application/json" -d "$reg_payload" >/tmp/apex_reg.json
  REGISTER_TOKEN="$(jq -r '.access_token // .token // .data.access_token // .tokens.access_token // empty' /tmp/apex_reg.json)"
  REGISTER_ERROR="$(jq -r '.error // empty' /tmp/apex_reg.json)"
  if [[ -n "$REGISTER_ERROR" && -z "$REGISTER_TOKEN" ]]; then
    echo "REGISTER_FAILED"
    cat /tmp/apex_reg.json
    exit 1
  fi
fi

fetch_csrf_token
login_or_exit

build_payload="$(jq -n \
  --arg d "$PROMPT" \
  --arg mode "$MODE" \
  --arg power "$EFFECTIVE_POWER_MODE" \
  --arg project "$PROJECT_NAME" \
  '{description:$d,prompt:$d,mode:$mode,power_mode:$power,provider_mode:"platform",require_preview_ready:true,project_name:$project}')"

curl -sS "${auth_args[@]}" -X POST "$BASE_URL/build/start" \
  -H "Content-Type: application/json" \
  -d "$build_payload" >/tmp/apex_build_start.json

BUILD_ID="$(jq -r '.build_id // empty' /tmp/apex_build_start.json)"
if [[ -z "$BUILD_ID" ]]; then
  echo "BUILD_START_FAILED"
  cat /tmp/apex_build_start.json
  exit 1
fi

echo "BUILD_ID=$BUILD_ID"
echo "TOKEN_FILE=/tmp/apex_login.json"
echo "SMOKE_PROFILE=$SMOKE_PROFILE"

final_status=""
for _ in $(seq 1 "$MAX_POLLS"); do
  status_json="$(curl -sS "${auth_args[@]}" "$BASE_URL/build/$BUILD_ID/status" || true)"
  auth_error="$(jq -r '.error // empty' <<<"$status_json" 2>/dev/null || true)"
  if [[ "$auth_error" == "authentication required" || "$auth_error" == "invalid or expired token" ]]; then
    refresh_or_relogin
    status_json="$(curl -sS "${auth_args[@]}" "$BASE_URL/build/$BUILD_ID/status" || true)"
  fi
  status="$(jq -r '.status // empty' <<<"$status_json" 2>/dev/null || true)"
  progress="$(jq -r '.progress // empty' <<<"$status_json" 2>/dev/null || true)"
  files="$(jq -r '.files_count // empty' <<<"$status_json" 2>/dev/null || true)"
  err="$(jq -r '.error // empty' <<<"$status_json" 2>/dev/null || true)"
  now="$(date +%H:%M:%S)"
  echo "[$now] status=$status progress=$progress files=$files error=${err:0:220}"

  if [[ "$status" == "completed" || "$status" == "failed" || "$status" == "cancelled" ]]; then
    final_status="$status"
    break
  fi
  sleep "$POLL_SECONDS"
done

curl -sS "${auth_args[@]}" "$BASE_URL/build/$BUILD_ID" >/tmp/apex_build_detail.json || true
detail_auth_error="$(jq -r '.error // empty' /tmp/apex_build_detail.json 2>/dev/null || true)"
if [[ "$detail_auth_error" == "authentication required" || "$detail_auth_error" == "invalid or expired token" ]]; then
  refresh_or_relogin
  curl -sS "${auth_args[@]}" "$BASE_URL/build/$BUILD_ID" >/tmp/apex_build_detail.json || true
fi
echo "FINAL_DETAIL_SUMMARY"
jq '{id,status,progress,error,provider_mode,power_mode,require_preview_ready,files_count:(.files|length)}' /tmp/apex_build_detail.json || cat /tmp/apex_build_detail.json

if [[ "$final_status" == "completed" ]]; then
  curl -sS "${auth_args[@]}" "$BASE_URL/builds/$BUILD_ID" >/tmp/apex_build_completed.json || true
  completed_auth_error="$(jq -r '.error // empty' /tmp/apex_build_completed.json 2>/dev/null || true)"
  if [[ "$completed_auth_error" == "authentication required" || "$completed_auth_error" == "invalid or expired token" ]]; then
    refresh_or_relogin
    curl -sS "${auth_args[@]}" "$BASE_URL/builds/$BUILD_ID" >/tmp/apex_build_completed.json || true
  fi
  echo "COMPLETED_BUILD_SUMMARY"
  jq '{build_id,status,progress,error,files_count,live,resumable}' /tmp/apex_build_completed.json || cat /tmp/apex_build_completed.json
  completed_status="$(jq -r '.status // empty' /tmp/apex_build_completed.json 2>/dev/null || true)"
  if [[ "$completed_status" != "completed" ]]; then
    echo "COMPLETED_BUILD_STATUS_MISMATCH=$completed_status"
    exit 1
  fi

  assert_completed_build_contract /tmp/apex_build_detail.json /tmp/apex_build_completed.json
  echo "ASSERTIONS_PASSED profile=$SMOKE_PROFILE power_mode=$EFFECTIVE_POWER_MODE"
fi

if [[ "$final_status" == "$EXPECT_STATUS" ]]; then
  exit 0
fi
if [[ -n "$final_status" ]]; then
  echo "BUILD_TERMINATED_WITH_UNEXPECTED_STATUS=$final_status"
  exit 1
fi
echo "BUILD_DID_NOT_TERMINATE_WITHIN_POLL_WINDOW"
exit 2
