#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080/api/v1}"
MODE="${MODE:-fast}"
POWER_MODE="${POWER_MODE:-balanced}"
PROJECT_NAME="${PROJECT_NAME:-agency-ops-platform}"
POLL_SECONDS="${POLL_SECONDS:-10}"
MAX_POLLS="${MAX_POLLS:-120}"
SMOKE_PROFILE="${SMOKE_PROFILE:-free_frontend}"
LOGIN_EMAIL="${LOGIN_EMAIL:-}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-}"
LOGIN_FULL_NAME="${LOGIN_FULL_NAME:-Platform Test}"
EXPECT_STATUS="${EXPECT_STATUS:-completed}"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 1
fi

prompt_file=""
cookie_jar=""
cleanup() {
  if [[ -n "${prompt_file}" && -f "${prompt_file}" ]]; then
    rm -f "${prompt_file}"
  fi
  if [[ -n "${cookie_jar}" && -f "${cookie_jar}" ]]; then
    rm -f "${cookie_jar}"
  fi
}
trap cleanup EXIT

cookie_jar="$(mktemp)"

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
USER="platform${SUFFIX}"
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

login_payload="$(jq -n --arg u "$USER" --arg e "$EMAIL" --arg p "$PASS" '{username:($e // $u),email:$e,password:$p}')"
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

auth_args=(-b "$cookie_jar" -c "$cookie_jar")
if [[ -n "$TOKEN" && "$TOKEN" != "null" ]]; then
  auth_args+=(-H "Authorization: Bearer $TOKEN")
fi

build_payload="$(jq -n \
  --arg d "$PROMPT" \
  --arg mode "$MODE" \
  --arg power "$POWER_MODE" \
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
echo "FINAL_DETAIL_SUMMARY"
jq '{id,status,progress,error,provider_mode,power_mode,require_preview_ready,files_count:(.files|length)}' /tmp/apex_build_detail.json || cat /tmp/apex_build_detail.json

if [[ "$final_status" == "completed" ]]; then
  curl -sS "${auth_args[@]}" "$BASE_URL/builds/$BUILD_ID" >/tmp/apex_build_completed.json || true
  echo "COMPLETED_BUILD_SUMMARY"
  jq '{build_id,status,progress,error,files_count,live,resumable}' /tmp/apex_build_completed.json || cat /tmp/apex_build_completed.json
  completed_status="$(jq -r '.status // empty' /tmp/apex_build_completed.json 2>/dev/null || true)"
  if [[ "$completed_status" != "completed" ]]; then
    echo "COMPLETED_BUILD_STATUS_MISMATCH=$completed_status"
    exit 1
  fi
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
