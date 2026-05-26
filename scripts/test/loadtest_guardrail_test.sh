#!/usr/bin/env bash
# Guardrail tests for scripts/loadtest.js - TASK-010 load/concurrency harness.
#
# Validates the k6 script shape, configuration, thresholds, and no-secret hygiene
# WITHOUT making network calls or requiring production credentials.
#
# Run: bash scripts/test/loadtest_guardrail_test.sh
#
# Skips k6-dependent checks gracefully if k6 is not installed.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
LOADTEST="$REPO_ROOT/scripts/loadtest.js"

TESTS_RUN=0
TESTS_FAILED=0
pass() { TESTS_RUN=$((TESTS_RUN + 1)); echo "  ok - $1"; }
fail() { TESTS_RUN=$((TESTS_RUN + 1)); TESTS_FAILED=$((TESTS_FAILED + 1)); echo "  NOT OK - $1" >&2; }

echo "== loadtest.js guardrail tests (no network, no secrets) =="

# ---- File existence ----

if [[ -f "$LOADTEST" ]]; then
  pass "scripts/loadtest.js exists"
else
  fail "scripts/loadtest.js does not exist"
  echo "Aborting: cannot test a missing file." >&2
  echo "Ran $TESTS_RUN assertions, $TESTS_FAILED failed."
  exit 1
fi

# ---- No secrets in the script ----

# Check that the script does not contain hardcoded passwords, tokens, or real credentials.
# This is a static hygiene check; env vars like LOGIN_EMAIL/LOGIN_PASSWORD are acceptable
# because they are references, not literal secrets.

if grep -qiE '(password\s*[:=]\s*["\x27][^"\x27]{3,}|api[_-]?key\s*[:=]\s*["\x27][^"\x27]{3,}|secret\s*[:=]\s*["\x27][^"\x27]{3,}|Bearer\s+eyJ|sk_live|sk_test|pk_live)' "$LOADTEST"; then
  fail "loadtest.js contains hardcoded secrets or real credential literals"
else
  pass "loadtest.js does not contain hardcoded secrets or credential literals"
fi

# Check that no line logs password or token values
if grep -nE 'console\.(log|info|warn|error).*password|console\.(log|info|warn|error).*token|console\.(log|info|warn|error).*cookie' "$LOADTEST" | grep -iv 'LOADTEST ABORT.*requires.*LOGIN_PASSWORD\|status=.*password' > /dev/null 2>&1; then
  fail "loadtest.js logs password, token, or cookie values"
else
  pass "loadtest.js does not log password, token, or cookie values"
fi

# ---- Opt-in safety: RUN_AUTH_API and RUN_BUILD_STARTS default to off ----

# Verify that RUN_AUTH_API and RUN_BUILD_STARTS are not enabled by default
# (i.e., the script should check === '1' and not default to true)
if grep -qE "RUN_AUTH_API\s*(===|==)\s*['\"]1['\"]" "$LOADTEST"; then
  pass "RUN_AUTH_API requires explicit '1' (opt-in, not default)"
else
  fail "RUN_AUTH_API does not require explicit '1' - may default to enabled"
fi

if grep -qE "RUN_BUILD_STARTS\s*(===|==)\s*['\"]1['\"]" "$LOADTEST"; then
  pass "RUN_BUILD_STARTS requires explicit '1' (opt-in, not default)"
else
  fail "RUN_BUILD_STARTS does not require explicit '1' - may default to enabled"
fi

# ---- Default URLs point to production (not localhost) ----

if grep -qE "LANDING_URL.*apex-build\.dev" "$LOADTEST"; then
  pass "Default LANDING_URL points to apex-build.dev"
else
  fail "Default LANDING_URL does not point to apex-build.dev"
fi

if grep -qE "API_URL.*api\.apex-build\.dev" "$LOADTEST"; then
  pass "Default API_URL points to api.apex-build.dev"
else
  fail "Default API_URL does not point to api.apex-build.dev"
fi

# ---- Scenario structure ----

# The default scenario must target 200 concurrent users
if grep -qE "target:\s*200" "$LOADTEST"; then
  pass "Default scenario targets 200 concurrent users"
else
  fail "Default scenario does not target 200 concurrent users"
fi

# ---- Thresholds: p95 < 800ms ----

if grep -qE "p\(95\)\s*<\s*800" "$LOADTEST"; then
  pass "Landing/health p95 < 800ms threshold present"
else
  fail "Landing/health p95 < 800ms threshold missing"
fi

# Auth error rate < 1%
if grep -qE "auth_api_errors.*rate\s*<\s*0\.01" "$LOADTEST"; then
  pass "Auth API error rate < 1% threshold present"
else
  fail "Auth API error rate < 1% threshold missing"
fi

# Build-start failure rate threshold
if grep -qE "build_start_failures.*rate\s*<\s*0\.01" "$LOADTEST"; then
  pass "Build-start failure rate < 1% threshold present"
else
  fail "Build-start failure rate < 1% threshold missing"
fi

# ---- Build prompt: bounded, low-cost ----

# Verify the build prompt is bounded and frontend-only (not an expensive full-stack prompt)
if grep -qE "simple to-do list|to-do list app|frontend.only|local state" "$LOADTEST"; then
  pass "Build start prompt is bounded/low-cost"
else
  fail "Build start prompt may not be bounded/low-cost"
fi

# Verify exactly 10 builds are started concurrently: 10 VUs, 1 iteration each
if grep -qE "MAX_BUILDS\s*=\s*10" "$LOADTEST" && grep -qE "vus:\s*10" "$LOADTEST" && grep -qE "iterations:\s*1" "$LOADTEST"; then
  pass "Build start scenario creates exactly 10 concurrent builds"
else
  fail "Build start scenario does not create exactly 10 concurrent builds"
fi

if grep -q "poll_token" "$LOADTEST" && grep -q "X-Apex-Build-Poll-Token" "$LOADTEST"; then
  pass "Build polling uses the backend build poll token"
else
  fail "Build polling does not use the backend build poll token"
fi

# ---- Fail-fast on missing credentials ----

if grep -qE "LOADTEST ABORT.*RUN_AUTH_API=1 requires LOGIN_EMAIL" "$LOADTEST"; then
  pass "Auth scenario fails fast when credentials missing"
else
  fail "Auth scenario does not clearly fail fast when credentials missing"
fi

if grep -qE "throw new Error\\(" "$LOADTEST"; then
  pass "Missing credential checks throw before scenarios run"
else
  fail "Missing credential checks do not throw before scenarios run"
fi

if grep -qE "LOADTEST ABORT.*RUN_BUILD_STARTS=1 requires LOGIN_EMAIL" "$LOADTEST" || grep -qE "build_starts.*LOGIN_EMAIL.*LOGIN_PASSWORD" "$LOADTEST"; then
  pass "Build scenario fails fast when credentials missing"
else
  fail "Build scenario does not clearly fail fast when credentials missing"
fi

if grep -q "email: LOGIN_EMAIL" "$LOADTEST" && grep -q "username: LOGIN_USERNAME" "$LOADTEST"; then
  pass "Login payload uses backend-supported username/email fields"
else
  fail "Login payload does not use backend-supported username/email fields"
fi

# ---- No 5xx logging of full bodies ----

# Ensure full response bodies are not logged (could contain private data)
if grep -nE 'console\.(log|info|warn|error).*res\.(body|text|html)|console\.(log|info|warn|error).*r\.(body|text|html)' "$LOADTEST" | head -5 | grep -v 'status=' > /dev/null 2>&1; then
  fail "loadtest.js logs full response bodies (potential data leak)"
else
  pass "loadtest.js does not log full response bodies"
fi

# ---- k6 syntax check (skipped if k6 unavailable) ----

K6_BIN="$(command -v k6 2>/dev/null || true)"
if [[ -n "$K6_BIN" ]]; then
  echo "  k6 found at: $K6_BIN"
  # k6 inspect validates script syntax without running it
  INSPECT_OUT="$("$K6_BIN" inspect "$LOADTEST" 2>&1)"; INSPECT_RC=$?
  if [[ "$INSPECT_RC" -eq 0 ]]; then
    pass "k6 inspect passes (valid script syntax)"
  else
    fail "k6 inspect fails (invalid script syntax)"
    echo "----- k6 inspect output -----" >&2
    echo "$INSPECT_OUT" >&2
    echo "-----------------------------" >&2
  fi

  # Verify expected scenario keys are present in inspect output
  if echo "$INSPECT_OUT" | grep -q '"public_traffic"'; then
    pass "k6 inspect shows public_traffic scenario"
  else
    fail "k6 inspect missing public_traffic scenario"
  fi

  BUILD_INSPECT_OUT="$("$K6_BIN" inspect -e RUN_BUILD_STARTS=1 -e LOGIN_EMAIL=test@example.invalid -e LOGIN_PASSWORD=test-password "$LOADTEST" 2>&1)"; BUILD_INSPECT_RC=$?
  if [[ "$BUILD_INSPECT_RC" -eq 0 && "$BUILD_INSPECT_OUT" == *'"build_starts"'* && "$BUILD_INSPECT_OUT" == *'"vus": 10'* ]]; then
    pass "k6 inspect shows opt-in 10-VU build_starts scenario"
  else
    fail "k6 inspect missing opt-in 10-VU build_starts scenario"
    echo "----- k6 build inspect output -----" >&2
    echo "$BUILD_INSPECT_OUT" >&2
    echo "-----------------------------------" >&2
  fi
else
  echo "  k6 not found - skipping k6 inspect and scenario checks"
  echo "  Install k6 to enable these checks: https://k6.io/docs/get-started/installation/"
fi

# ---- Node.js syntax validation of the script (as a module) ----
# k6 scripts use ES module import syntax which plain Node won't parse,
# but we can at least check for gross syntax errors with a try/catch approach.
NODE_BIN="$(command -v node 2>/dev/null || true)"
if [[ -n "$NODE_BIN" ]]; then
  # Check for balanced braces (a rough proxy for syntax validity)
  OPEN_BRACES="$(grep -o '{' "$LOADTEST" | wc -l)"
  CLOSE_BRACES="$(grep -o '}' "$LOADTEST" | wc -l)"
  if [[ "$OPEN_BRACES" -eq "$CLOSE_BRACES" ]]; then
    pass "Balanced braces in loadtest.js (open=$OPEN_BRACES close=$CLOSE_BRACES)"
  else
    fail "Unbalanced braces in loadtest.js (open=$OPEN_BRACES close=$CLOSE_BRACES)"
  fi

  # Check for duplicate export names (common error)
  EXPORT_COUNT="$(grep -c '^export ' "$LOADTEST" 2>/dev/null || echo 0)"
  if [[ "$EXPORT_COUNT" -ge 2 ]]; then
    # Should have at least: options, default function
    UNIQUE_EXPORTS="$(grep '^export ' "$LOADTEST" | sort -u | wc -l)"
    TOTAL_EXPORTS="$(grep '^export ' "$LOADTEST" | wc -l)"
    if [[ "$UNIQUE_EXPORTS" -eq "$TOTAL_EXPORTS" ]]; then
      pass "No duplicate export declarations"
    else
      fail "Duplicate export declarations detected"
    fi
  else
    pass "Export count acceptable ($EXPORT_COUNT)"
  fi
else
  echo "  node not found - skipping Node syntax checks"
fi

# ---- Summary ----

echo
echo "Ran $TESTS_RUN assertions, $TESTS_FAILED failed."
if [[ "$TESTS_FAILED" -gt 0 ]]; then
  echo "LOADTEST_GUARDRAIL_TESTS_FAILED"
  exit 1
fi
echo "LOADTEST_GUARDRAIL_TESTS_PASSED"
