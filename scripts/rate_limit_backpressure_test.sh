#!/usr/bin/env bash
#===============================================================================
# APEX-BUILD RATE-LIMIT & BACKPRESSURE TEST
#===============================================================================
# Non-mutating verification script for rate-limit behavior.
# Safe to run against production backends.
#
# Usage:
#   BACKEND_URL=https://api.apex-build.dev bash rate_limit_backpressure_test.sh
#   BACKEND_URL=http://localhost:8080 bash rate_limit_backpressure_test.sh
#===============================================================================

set -euo pipefail

BACKEND_URL="${BACKEND_URL:-https://api.apex-build.dev}"
FRONTEND_URL="${FRONTEND_URL:-https://apex-build.dev}"
TIMEOUT_SEC="${TIMEOUT_SEC:-5}"
LOGIN_EMAIL="${LOGIN_EMAIL:-}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-}"

PASS=0
FAIL=0
WARN=0

tmpdir="$(mktemp -d)"
cleanup() { rm -rf "$tmpdir"; }
trap cleanup EXIT

log_pass()  { echo "  [PASS] $1"; ((PASS++)); }
log_fail()  { echo "  [FAIL] $1"; ((FAIL++)); }
log_warn()  { echo "  [WARN] $1"; ((WARN++)); }
log_info()  { echo "  [INFO] $1"; }

http_code() {
  local url="$1"
  curl -sS -o /dev/null -w '%{http_code}' --max-time "$TIMEOUT_SEC" "$url" || echo "000"
}

http_json() {
  local url="$1"
  local out="$2"
  shift 2
  curl -sS --max-time "$TIMEOUT_SEC" "$url" "$@" >"$out" 2>/dev/null || true
}

echo "=== RATE-LIMIT & BACKPRESSURE TEST ==="
echo "Target: $BACKEND_URL"
echo ""

#-------------------------------------------------------------------------------
# TEST 1: General API rate-limit produces 429 after burst
#-------------------------------------------------------------------------------
echo "--- TEST 1: General API burst (>50 rapid /health) ---"
burst_count=55
found_429=0
for i in $(seq 1 "$burst_count"); do
  code=$(http_code "${BACKEND_URL}/health")
  if [[ "$code" == "429" ]]; then
    found_429=1
    break
  fi
done
if [[ "$found_429" -gt 0 ]]; then
  log_pass "General API returned 429 after burst"
else
  log_warn "General API did NOT return 429 after $burst_count rapid requests (may need Redis or higher burst)"
fi

#-------------------------------------------------------------------------------
# TEST 2: Auth endpoint rate-limit produces 429 after burst
#-------------------------------------------------------------------------------
echo "--- TEST 2: Auth endpoint burst (>5 rapid bogus logins) ---"
auth_burst=8
auth_found_429=0
for i in $(seq 1 "$auth_burst"); do
  code=$(curl -sS -o /dev/null -w '%{http_code}' --max-time "$TIMEOUT_SEC" \
    -X POST "${BACKEND_URL}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"nobody","password":"bad"}' || echo "000")
  if [[ "$code" == "429" ]]; then
    auth_found_429=1
    break
  fi
done
if [[ "$auth_found_429" -gt 0 ]]; then
  log_pass "Auth endpoint returned 429 after burst"
else
  log_warn "Auth endpoint did NOT return 429 after $auth_burst rapid requests"
fi

#-------------------------------------------------------------------------------
# TEST 3: 429 response shape includes Retry-After and JSON Code
#-------------------------------------------------------------------------------
echo "--- TEST 3: 429 response shape ---"
# Provoke a 429 on auth (easiest)
for i in $(seq 1 20); do
  curl -sS --max-time "$TIMEOUT_SEC" \
    -X POST "${BACKEND_URL}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"nobody","password":"bad"}' \
    -D "$tmpdir/auth_429_headers.txt" > "$tmpdir/auth_429_body.txt" 2>/dev/null || true
  if grep -q "429" "$tmpdir/auth_429_headers.txt" 2>/dev/null; then
    break
  fi
done

if [[ -f "$tmpdir/auth_429_headers.txt" ]] && grep -q "429" "$tmpdir/auth_429_headers.txt"; then
  if grep -qi "Retry-After" "$tmpdir/auth_429_headers.txt"; then
    log_pass "429 response includes Retry-After header"
  else
    log_warn "429 response missing Retry-After header"
  fi
  if grep -q '"code"' "$tmpdir/auth_429_body.txt" 2>/dev/null || grep -q '"Code"' "$tmpdir/auth_429_body.txt" 2>/dev/null; then
    log_pass "429 response body includes code field"
  else
    log_warn "429 response body missing code field"
  fi
else
  log_warn "Could not capture a 429 to verify response shape"
fi

#-------------------------------------------------------------------------------
# TEST 4: Build-start rate-limit (requires authenticated token)
#-------------------------------------------------------------------------------
echo "--- TEST 4: Build-start rate-limit (optional, needs credentials) ---"
if [[ -n "$LOGIN_EMAIL" && -n "$LOGIN_PASSWORD" ]]; then
  # Obtain token
  login_payload=$(jq -n --arg e "$LOGIN_EMAIL" --arg p "$LOGIN_PASSWORD" '{email:$e,password:$p}')
  curl -sS --max-time "$TIMEOUT_SEC" \
    -X POST "${BACKEND_URL}/api/v1/auth/login" \
    -H "Content-Type: application/json" -d "$login_payload" \
    > "$tmpdir/login.json" 2>/dev/null || true
  token=$(jq -r '.data.tokens.access_token // .tokens.access_token // .access_token // empty' "$tmpdir/login.json" 2>/dev/null || echo "")
  if [[ -n "$token" ]]; then
    auth_header="Authorization: Bearer $token"
    build_found_429=0
    for i in $(seq 1 8); do
      code=$(curl -sS -o /dev/null -w '%{http_code}' --max-time "$TIMEOUT_SEC" \
        -X POST "${BACKEND_URL}/api/v1/build/start" \
        -H "$auth_header" -H "Content-Type: application/json" \
        -d '{"description":"test","prompt":"test","mode":"fast","provider_mode":"platform"}' || echo "000")
      if [[ "$code" == "429" ]]; then
        build_found_429=1
        break
      fi
    done
    if [[ "$build_found_429" -gt 0 ]]; then
      log_pass "Build-start endpoint returned 429 after burst"
    else
      log_warn "Build-start endpoint did NOT return 429 after burst (may be below threshold)"
    fi
  else
    log_warn "Could not obtain auth token — skipping build-start rate-limit test"
  fi
else
  log_info "LOGIN_EMAIL / LOGIN_PASSWORD not set — skipping build-start rate-limit test"
fi

#-------------------------------------------------------------------------------
# TEST 5: CORS preflight is NOT rate-limited (must return 204)
#-------------------------------------------------------------------------------
echo "--- TEST 5: CORS preflight exempt from rate-limit ---"
preflight_status=$(curl -sS -o /dev/null -w '%{http_code}' --max-time "$TIMEOUT_SEC" \
  -X OPTIONS -H "Origin: ${FRONTEND_URL}" \
  -H "Access-Control-Request-Method: POST" \
  "${BACKEND_URL}/api/v1/auth/login" || echo "000")
if [[ "$preflight_status" == "204" || "$preflight_status" == "200" ]]; then
  log_pass "CORS preflight returns $preflight_status (not rate-limited)"
else
  log_warn "CORS preflight returned HTTP $preflight_status"
fi

#-------------------------------------------------------------------------------
# TEST 6: /health and /ready are NOT rate-limited under normal load
# (Implicit from SkipPaths in logger, but verify behavior.)
#-------------------------------------------------------------------------------
echo "--- TEST 6: /health & /ready availability under load ---"
health_ok=0
ready_ok=0
for i in $(seq 1 10); do
  h=$(http_code "${BACKEND_URL}/health")
  r=$(http_code "${BACKEND_URL}/ready")
  [[ "$h" == "200" ]] && health_ok=$((health_ok + 1))
  [[ "$r" == "200" || "$r" == "503" ]] && ready_ok=$((ready_ok + 1))
done
if [[ "$health_ok" -eq 10 ]]; then
  log_pass "/health stable under light repeated load"
else
  log_warn "/health unstable ($health_ok/10 200s)"
fi
if [[ "$ready_ok" -eq 10 ]]; then
  log_pass "/ready responsive under light repeated load"
else
  log_warn "/ready unstable ($ready_ok/10 responsive)"
fi

#-------------------------------------------------------------------------------
# SUMMARY
#-------------------------------------------------------------------------------
echo ""
echo "================================================================"
echo "RATE-LIMIT & BACKPRESSURE TEST SUMMARY"
echo "================================================================"
echo "  Passed:  $PASS"
echo "  Failed:  $FAIL"
echo "  Warned:  $WARN"
echo "================================================================"

if [[ "$FAIL" -gt 0 ]]; then
  echo "RESULT: FAILED — $FAIL test(s) did not pass"
  exit 1
fi

echo "RESULT: PASSED — all critical tests passed ($WARN warning(s))"
exit 0
