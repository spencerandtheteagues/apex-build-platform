#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080/api/v1}"
MODE="${MODE:-fast}"
POWER_MODE="${POWER_MODE:-balanced}"
PROJECT_NAME="${PROJECT_NAME:-agency-ops-platform}"
POLL_SECONDS="${POLL_SECONDS:-10}"
MAX_POLLS="${MAX_POLLS:-120}"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 1
fi

prompt_file=""
cleanup() {
  if [[ -n "${prompt_file}" && -f "${prompt_file}" ]]; then
    rm -f "${prompt_file}"
  fi
}
trap cleanup EXIT

if [[ $# -gt 0 ]]; then
  prompt_file="$1"
else
  prompt_file="$(mktemp)"
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
fi

PROMPT="$(cat "$prompt_file")"
SUFFIX="$(date +%s)"
USER="platform${SUFFIX}"
EMAIL="${USER}@example.com"
PASS="Passw0rd!Passw0rd!"

reg_payload="$(jq -n --arg u "$USER" --arg e "$EMAIL" --arg p "$PASS" --arg n "Platform Test" \
  '{username:$u,email:$e,password:$p,full_name:$n}')"
login_payload="$(jq -n --arg e "$EMAIL" --arg p "$PASS" '{email:$e,password:$p}')"

curl -sS -X POST "$BASE_URL/auth/register" -H "Content-Type: application/json" -d "$reg_payload" >/tmp/apex_reg.json
curl -sS -X POST "$BASE_URL/auth/login" -H "Content-Type: application/json" -d "$login_payload" >/tmp/apex_login.json

TOKEN="$(jq -r '.access_token // .token // .data.access_token // .tokens.access_token // empty' /tmp/apex_login.json)"
if [[ -z "$TOKEN" || "$TOKEN" == "null" ]]; then
  echo "LOGIN_FAILED"
  cat /tmp/apex_login.json
  exit 1
fi

build_payload="$(jq -n \
  --arg d "$PROMPT" \
  --arg mode "$MODE" \
  --arg power "$POWER_MODE" \
  --arg project "$PROJECT_NAME" \
  '{description:$d,prompt:$d,mode:$mode,power_mode:$power,provider_mode:"platform",require_preview_ready:true,project_name:$project}')"

curl -sS -X POST "$BASE_URL/build/start" \
  -H "Authorization: Bearer $TOKEN" \
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

final_status=""
for _ in $(seq 1 "$MAX_POLLS"); do
  status_json="$(curl -sS "$BASE_URL/build/$BUILD_ID/status" -H "Authorization: Bearer $TOKEN" || true)"
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

curl -sS "$BASE_URL/build/$BUILD_ID" -H "Authorization: Bearer $TOKEN" >/tmp/apex_build_detail.json || true
echo "FINAL_DETAIL_SUMMARY"
jq '{id,status,progress,error,provider_mode,power_mode,require_preview_ready,files_count:(.files|length)}' /tmp/apex_build_detail.json || cat /tmp/apex_build_detail.json

if [[ -n "$final_status" ]]; then
  exit 0
fi
echo "BUILD_DID_NOT_TERMINATE_WITHIN_POLL_WINDOW"
exit 2
