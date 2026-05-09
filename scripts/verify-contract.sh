#!/usr/bin/env bash
# APEX.BUILD API Contract Verification Script
# Run after every deploy to verify live endpoints match API_CONTRACT.md

set -euo pipefail

API_BASE="${API_BASE:-https://api.apex-build.dev/api/v1}"
WS_BASE="${WS_BASE:-wss://api.apex-build.dev}"
TIMEOUT="${TIMEOUT:-10}"
VERBOSE="${VERBOSE:-0}"

PASS=0
FAIL=0
WARN=0

log_pass() { echo "  ✅ $1"; ((PASS++)); }
log_fail() { echo "  ❌ $1"; ((FAIL++)); }
log_warn() { echo "  ⚠️  $1"; ((WARN++)); }
log_info() { echo "  ℹ️  $1"; }

# ------------------------------------------------------------------
# Health endpoints (no auth)
# ------------------------------------------------------------------
echo ""
echo "=== Health Endpoints ==="

# /health
if curl -sf -m "$TIMEOUT" "$API_BASE/health" >/dev/null 2>&1; then
  log_pass "GET /health"
else
  # Try without /api/v1 prefix (some deployments)
  BASE_NO_V1="${API_BASE%/api/v1}"
  if curl -sf -m "$TIMEOUT" "$BASE_NO_V1/health" >/dev/null 2>&1; then
    log_pass "GET /health (at root, not /api/v1)"
  else
    log_fail "GET /health"
  fi
fi

# /ready
if curl -sf -m "$TIMEOUT" "$API_BASE/ready" >/dev/null 2>&1 || \
   curl -sf -m "$TIMEOUT" "${API_BASE%/api/v1}/ready" >/dev/null 2>&1; then
  log_pass "GET /ready"
else
  log_warn "GET /ready (may be OK if not deployed to k8s)"
fi

# /health/features
if curl -sf -m "$TIMEOUT" "$API_BASE/health/features" >/dev/null 2>&1 || \
   curl -sf -m "$TIMEOUT" "${API_BASE%/api/v1}/health/features" >/dev/null 2>&1; then
  log_pass "GET /health/features"
else
  log_warn "GET /health/features"
fi

# ------------------------------------------------------------------
# Auth endpoints (no auth required)
# ------------------------------------------------------------------
echo ""
echo "=== Auth Endpoints ==="

# Register - check OPTIONS first, then a test register (will likely fail validation but should not 404)
if curl -sf -X POST -m "$TIMEOUT" -H "Content-Type: application/json" \
   "$API_BASE/auth/register" -d '{"username":"test","email":"test@test.com","password":"test123"}' \
   >/dev/null 2>&1; then
  log_pass "POST /auth/register"
else
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST -m "$TIMEOUT" -H "Content-Type: application/json" \
    "$API_BASE/auth/register" -d '{"username":"test","email":"test@test.com","password":"test123"}' 2>/dev/null || echo "000")
  if [ "$HTTP_CODE" = "400" ] || [ "$HTTP_CODE" = "409" ] || [ "$HTTP_CODE" = "422" ]; then
    log_pass "POST /auth/register (endpoint exists, returned $HTTP_CODE as expected for bad input)"
  elif [ "$HTTP_CODE" = "404" ]; then
    log_fail "POST /auth/register (404 - endpoint missing)"
  else
    log_warn "POST /auth/register (HTTP $HTTP_CODE)"
  fi
fi

# Login
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST -m "$TIMEOUT" -H "Content-Type: application/json" \
  "$API_BASE/auth/login" -d '{"username_or_email":"test","password":"test123"}' 2>/dev/null || echo "000")
if [ "$HTTP_CODE" = "401" ] || [ "$HTTP_CODE" = "200" ]; then
  log_pass "POST /auth/login (endpoint exists, returned $HTTP_CODE)"
elif [ "$HTTP_CODE" = "404" ]; then
  log_fail "POST /auth/login (404)"
else
  log_warn "POST /auth/login (HTTP $HTTP_CODE)"
fi

# CSRF token
if curl -sf -m "$TIMEOUT" "$API_BASE/csrf-token" >/dev/null 2>&1; then
  log_pass "GET /csrf-token"
else
  log_fail "GET /csrf-token"
fi

# ------------------------------------------------------------------
# Protected endpoints (need auth - we just verify they exist by checking 401 vs 404)
# ------------------------------------------------------------------
echo ""
echo "=== Protected Endpoints (401 = exists, 404 = missing) ==="

check_protected() {
  local method="$1"
  local path="$2"
  local name="$3"
  
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" -m "$TIMEOUT" \
    -H "Content-Type: application/json" \
    "$API_BASE$path" 2>/dev/null || echo "000")
  
  if [ "$HTTP_CODE" = "401" ] || [ "$HTTP_CODE" = "403" ]; then
    log_pass "$name (protected, returns $HTTP_CODE)"
  elif [ "$HTTP_CODE" = "404" ] || [ "$HTTP_CODE" = "405" ]; then
    log_fail "$name (404/405 - endpoint missing or wrong method)"
  elif [ "$HTTP_CODE" = "000" ]; then
    log_warn "$name (connection error)"
  else
    log_warn "$name (HTTP $HTTP_CODE - unexpected)"
  fi
}

check_protected GET  "/user/profile"             "GET /user/profile"
check_protected PUT  "/user/profile"              "PUT /user/profile"
check_protected GET  "/projects"                  "GET /projects"
check_protected POST "/projects"                  "POST /projects"
check_protected GET  "/projects/1"                "GET /projects/:id"
check_protected PUT  "/projects/1"                "PUT /projects/:id"
check_protected DELETE "/projects/1"                "DELETE /projects/:id"
check_protected GET  "/projects/1/files"          "GET /projects/:id/files"
check_protected POST "/projects/1/files"          "POST /projects/:id/files"
check_protected GET  "/files/1"                   "GET /files/:id"
check_protected PUT  "/files/1"                   "PUT /files/:id"
check_protected DELETE "/files/1"                 "DELETE /files/:id"
check_protected GET  "/ai/usage"                  "GET /ai/usage"
check_protected POST "/ai/generate"               "POST /ai/generate"
check_protected GET  "/byok/keys"                 "GET /byok/keys"
check_protected POST "/byok/keys"                 "POST /byok/keys"
check_protected GET  "/byok/models"               "GET /byok/models"
check_protected GET  "/byok/usage"                "GET /byok/usage"
check_protected GET  "/budget/caps"               "GET /budget/caps"
check_protected POST "/budget/caps"               "POST /budget/caps"
check_protected POST "/budget/kill-all"           "POST /budget/kill-all"
check_protected GET  "/spend/dashboard"           "GET /spend/dashboard"
check_protected GET  "/spend/breakdown"           "GET /spend/breakdown"
check_protected GET  "/templates"                 "GET /templates"
check_protected GET  "/templates/categories"       "GET /templates/categories"
check_protected POST "/templates/create-project"    "POST /templates/create-project"
check_protected GET  "/search/quick"              "GET /search/quick"
check_protected POST "/search"                    "POST /search"
check_protected GET  "/preview/status/1"          "GET /preview/status/:projectId"
check_protected POST "/preview/start"             "POST /preview/start"
check_protected POST "/preview/stop"              "POST /preview/stop"
check_protected GET  "/preview/list"              "GET /preview/list"
check_protected GET  "/git/repo/1"                "GET /git/repo/:projectId"
check_protected POST "/git/connect"               "POST /git/connect"
check_protected GET  "/billing/subscription"      "GET /billing/subscription"
check_protected GET  "/billing/plans"             "GET /billing/plans"
check_protected POST "/billing/checkout"          "POST /billing/checkout"
check_protected GET  "/billing/credits/balance"   "GET /billing/credits/balance"
check_protected POST "/collab/join/1"             "POST /collab/join/:projectId"
check_protected GET  "/collab/users/1"            "GET /collab/users/:roomId"

# ------------------------------------------------------------------
# WebSocket endpoints
# ------------------------------------------------------------------
echo ""
echo "=== WebSocket Endpoints (connection test) ==="

check_ws() {
  local path="$1"
  local name="$2"
  
  # Try to connect via websocket - we expect it to fail auth but the endpoint should exist
  # Use curl to check if the upgrade is accepted (101) or rejected (401/403)
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -m "$TIMEOUT" \
    -H "Upgrade: websocket" \
    -H "Connection: Upgrade" \
    -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
    -H "Sec-WebSocket-Version: 13" \
    "$WS_BASE$path" 2>/dev/null || echo "000")
  
  if [ "$HTTP_CODE" = "101" ] || [ "$HTTP_CODE" = "401" ] || [ "$HTTP_CODE" = "403" ]; then
    log_pass "$name (WS endpoint exists, $HTTP_CODE)"
  elif [ "$HTTP_CODE" = "404" ]; then
    log_fail "$name (404 - WS endpoint missing)"
  else
    log_warn "$name (HTTP $HTTP_CODE)"
  fi
}

check_ws "/ws/build/test"           "WS /ws/build/:buildId"
check_ws "/ws/collab"               "WS /ws/collab"
check_ws "/ws/terminal/test"        "WS /ws/terminal/:sessionId"
check_ws "/ws/debug/test"           "WS /ws/debug/:sessionId"
check_ws "/ws/deploy/test"          "WS /ws/deploy/:deploymentId"

# ------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------
echo ""
echo "=================================="
echo "API Contract Verification Summary"
echo "=================================="
echo "  ✅ Passed:  $PASS"
echo "  ⚠️  Warned: $WARN"
echo "  ❌ Failed:  $FAIL"
echo ""

if [ "$FAIL" -gt 0 ]; then
  echo "❌ VERIFICATION FAILED — $FAIL endpoint(s) missing or broken"
  exit 1
else
  echo "✅ VERIFICATION PASSED — all endpoints reachable"
  if [ "$WARN" -gt 0 ]; then
    echo "⚠️  $WARN warning(s) — review recommended"
  fi
  exit 0
fi
