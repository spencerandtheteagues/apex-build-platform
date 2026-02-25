#!/usr/bin/env bash
set -euo pipefail

FRONTEND_URL="${FRONTEND_URL:-https://apex.build}"
BACKEND_URL="${BACKEND_URL:-https://apex-backend-5ypy.onrender.com}"
API_BASE="${API_BASE:-$BACKEND_URL/api/v1}"
RUN_BUILD_TERMINALIZATION="${RUN_BUILD_TERMINALIZATION:-0}"
BUILD_MODE="${BUILD_MODE:-fast}"
BUILD_POLL_SECONDS="${BUILD_POLL_SECONDS:-10}"
BUILD_MAX_POLLS="${BUILD_MAX_POLLS:-30}"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 1
fi

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

join_url_path() {
  local base="$1"
  local route="$2"
  [[ "$route" == /* ]] || route="/$route"
  if [[ "$base" == *\?* ]]; then
    local path_part="${base%%\?*}"
    local query_part="${base#*\?}"
    printf '%s%s?%s\n' "${path_part%/}" "$route" "$query_part"
  else
    printf '%s%s\n' "${base%/}" "$route"
  fi
}

curl_json() {
  local method="$1"
  local url="$2"
  local out="$3"
  shift 3
  curl -fsS -X "$method" "$url" "$@" >"$out"
}

echo "== Frontend health =="
curl -fsSI "$FRONTEND_URL/" >"$tmpdir/frontend-head.txt"
sed -n '1,5p' "$tmpdir/frontend-head.txt"

echo "== Backend health =="
curl_json GET "$BACKEND_URL/health" "$tmpdir/backend-health.json"
jq . "$tmpdir/backend-health.json"

echo "== Backend API health route (informational) =="
api_health_status="$(curl -sS -o /dev/null -w '%{http_code}' "$API_BASE/health" || true)"
echo "API /health status: $api_health_status"

suffix="$(date +%s)-$RANDOM"
username="verify_${suffix}"
email="${username}@example.com"
password="Passw0rd!Passw0rd!"

reg_payload="$(jq -n --arg u "$username" --arg e "$email" --arg p "$password" --arg n "Reliability Verify" \
  '{username:$u,email:$e,password:$p,full_name:$n}')"

echo "== Register test user =="
reg_status="$(curl -sS -o "$tmpdir/register.json" -w '%{http_code}' \
  -X POST "$API_BASE/auth/register" \
  -H 'Content-Type: application/json' \
  -d "$reg_payload")"
echo "register status: $reg_status"
sed -n '1,40p' "$tmpdir/register.json"

token="$(jq -r '.data.tokens.access_token // .tokens.access_token // .access_token // empty' "$tmpdir/register.json")"
if [[ -z "$token" ]]; then
  login_payload="$(jq -n --arg u "$username" --arg e "$email" --arg p "$password" '{username:$u,email:$e,password:$p}')"
  login_status="$(curl -sS -o "$tmpdir/login.json" -w '%{http_code}' \
    -X POST "$API_BASE/auth/login" \
    -H 'Content-Type: application/json' \
    -d "$login_payload")"
  echo "login status: $login_status"
  token="$(jq -r '.data.tokens.access_token // .tokens.access_token // .access_token // empty' "$tmpdir/login.json")"
fi

if [[ -z "$token" ]]; then
  echo "FAILED: could not obtain auth token" >&2
  exit 1
fi

auth_header="Authorization: Bearer $token"

echo "== Create preview verification project =="
project_payload="$(jq -n \
  --arg name "Reliability Preview Fixture ${suffix}" \
  --arg desc "Validates fullstack preview proxy routes" \
  '{name:$name,description:$desc,language:"javascript",framework:""}')"
curl_json POST "$API_BASE/projects" "$tmpdir/project.json" \
  -H "$auth_header" -H 'Content-Type: application/json' -d "$project_payload"
project_id="$(jq -r '.project.id // .data.project.id // empty' "$tmpdir/project.json")"
if [[ -z "$project_id" ]]; then
  echo "FAILED: project creation did not return project.id" >&2
  jq . "$tmpdir/project.json"
  exit 1
fi
echo "project_id=$project_id"

index_html='<!doctype html><html><body><h1>Render Preview Verify</h1></body></html>'
server_js='const http=require("http");const port=Number(process.env.PORT||9100);http.createServer((req,res)=>{if(req.url.startsWith("/api/hello")){res.writeHead(200,{"content-type":"application/json"});return res.end(JSON.stringify({hello:"world"}));}if(req.url.startsWith("/graphql")){res.writeHead(200,{"content-type":"application/json"});return res.end(JSON.stringify({data:{ping:"pong"}}));}if(req.url.startsWith("/trpc/ping")){res.writeHead(200,{"content-type":"application/json"});return res.end(JSON.stringify({result:{data:"trpc-ok"}}));}res.writeHead(404);res.end("not found");}).listen(port,"0.0.0.0");'

for file_path in "/index.html" "server.js"; do
  if [[ "$file_path" == "/index.html" ]]; then
    content="$index_html"
  else
    content="$server_js"
  fi
  file_payload="$(jq -n --arg p "$file_path" --arg n "$(basename "$file_path")" --arg c "$content" \
    '{path:$p,name:$n,type:"file",content:$c}')"
  curl_json POST "$API_BASE/projects/$project_id/files" "$tmpdir/file-$(basename "$file_path").json" \
    -H "$auth_header" -H 'Content-Type: application/json' -d "$file_payload"
done

echo "== Start unified full-stack preview =="
preview_payload="$(jq -n \
  --argjson project_id "$project_id" \
  '{project_id:$project_id,start_backend:true,require_backend:true,backend_entry_file:"server.js",backend_command:"node server.js",sandbox:false}')"
preview_status="$(curl -sS -o "$tmpdir/preview-start.json" -w '%{http_code}' \
  -X POST "$API_BASE/preview/fullstack/start" \
  -H "$auth_header" -H 'Content-Type: application/json' -d "$preview_payload")"
echo "preview start status: $preview_status"
sed -n '1,80p' "$tmpdir/preview-start.json"
if [[ "$preview_status" != "200" ]]; then
  echo "FAILED: fullstack preview did not start" >&2
  exit 1
fi

proxy_url="$(jq -r '.proxy_url // .preview.url // empty' "$tmpdir/preview-start.json")"
if [[ -z "$proxy_url" ]]; then
  echo "FAILED: preview start response missing proxy_url" >&2
  exit 1
fi
echo "proxy_url=$proxy_url"

echo "== Verify proxied routes =="
root_url="$(join_url_path "$proxy_url" "/")"
api_url="$(join_url_path "$proxy_url" "/api/hello")"
graphql_url="$(join_url_path "$proxy_url" "/graphql")"
trpc_url="$(join_url_path "$proxy_url" "/trpc/ping")"

curl_json GET "$root_url" "$tmpdir/root.html" -H "$auth_header"
grep -q "Render Preview Verify" "$tmpdir/root.html"
echo "root route OK"

curl_json GET "$api_url" "$tmpdir/api.json" -H "$auth_header"
jq -e '.hello == "world"' "$tmpdir/api.json" >/dev/null
echo "/api/hello OK"

curl_json GET "$graphql_url" "$tmpdir/graphql.json" -H "$auth_header"
jq -e '.data.ping == "pong"' "$tmpdir/graphql.json" >/dev/null
echo "/graphql OK"

curl_json GET "$trpc_url" "$tmpdir/trpc.json" -H "$auth_header"
jq -e '.result.data == "trpc-ok"' "$tmpdir/trpc.json" >/dev/null
echo "/trpc/ping OK"

echo "== Reliability metrics snapshot =="
curl -fsS "$BACKEND_URL/metrics" >"$tmpdir/metrics.txt"
grep -E 'apex_reliability_(preview_starts_total|preview_backend_starts_total|build_finalizations_total|build_stalls_total)' "$tmpdir/metrics.txt" || true

if [[ "$RUN_BUILD_TERMINALIZATION" == "1" ]]; then
  echo "== Build terminalization probe =="
  build_prompt="Create a tiny full-stack app with React frontend and Node backend. It can be minimal but must include runnable scripts."
  build_payload="$(jq -n --arg d "$build_prompt" --arg mode "$BUILD_MODE" \
    '{description:$d,prompt:$d,mode:$mode,power_mode:"fast",provider_mode:"platform"}')"
  curl_json POST "$API_BASE/build/start" "$tmpdir/build-start.json" \
    -H "$auth_header" -H 'Content-Type: application/json' -d "$build_payload"
  build_id="$(jq -r '.build_id // empty' "$tmpdir/build-start.json")"
  if [[ -z "$build_id" ]]; then
    echo "FAILED: build start missing build_id" >&2
    jq . "$tmpdir/build-start.json"
    exit 1
  fi
  echo "build_id=$build_id"

  final_status=""
  for _ in $(seq 1 "$BUILD_MAX_POLLS"); do
    curl_json GET "$API_BASE/build/$build_id/status" "$tmpdir/build-status.json" -H "$auth_header"
    status="$(jq -r '.status // empty' "$tmpdir/build-status.json")"
    progress="$(jq -r '.progress // empty' "$tmpdir/build-status.json")"
    err="$(jq -r '.error // empty' "$tmpdir/build-status.json")"
    echo "build status=$status progress=$progress error=${err:0:180}"
    if [[ "$status" == "completed" || "$status" == "failed" || "$status" == "cancelled" ]]; then
      final_status="$status"
      break
    fi
    sleep "$BUILD_POLL_SECONDS"
  done
  if [[ -z "$final_status" ]]; then
    echo "FAILED: build did not terminalize within poll window" >&2
    exit 1
  fi
  echo "build terminalization OK ($final_status)"
fi

echo "SUCCESS: platform reliability verification checks passed"
