#!/usr/bin/env bash
# No-network guardrails for Render launch verification and rollback drill scripts.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "PASS: $*"
}

echo "== render launch script syntax =="
node --check scripts/verify_render_launch_env.mjs >/dev/null
node --check scripts/run_render_rollback_drill.mjs >/dev/null
pass "node --check passes"

echo "== render.yaml Ruby-free parser fallback =="
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT
mkdir -p "$tmpdir/bin"
cat > "$tmpdir/bin/ruby" <<'RUBY'
#!/usr/bin/env bash
exit 127
RUBY
chmod +x "$tmpdir/bin/ruby"

fallback_output="$(PATH="$tmpdir/bin:$PATH" node scripts/verify_render_launch_env.mjs 2>&1)"
case "$fallback_output" in
  *"using built-in render.yaml fallback parser"* ) ;;
  *) fail "verify_render_launch_env.mjs did not report Ruby-free fallback parser use" ;;
esac
case "$fallback_output" in
  *"Render launch verification completed."* ) ;;
  *) fail "verify_render_launch_env.mjs fallback parser did not complete static verification" ;;
esac
pass "verify_render_launch_env.mjs can parse render.yaml without Ruby"

echo "== rollback drill destructive guardrails =="
rollback_script="scripts/run_render_rollback_drill.mjs"
grep -q "/services/.*rollback" "$rollback_script" || fail "rollback drill must use Render /rollback endpoint"
grep -q "APEX_RENDER_CONFIRM_ROLLBACK_DEPLOY_ID" "$rollback_script" || fail "rollback deploy-id confirmation missing"
grep -q "APEX_RENDER_CONFIRM_ROLL_FORWARD_DEPLOY_ID" "$rollback_script" || fail "roll-forward deploy-id confirmation missing"
grep -q "refusing to execute rollback drill without exact deploy-id confirmations" "$rollback_script" || fail "exact confirmation refusal missing"
if grep -q "commitId.*previous" "$rollback_script"; then
  fail "rollback drill must not use placeholder commitId rollback guidance"
fi

missing_creds_output="$(node "$rollback_script" 2>&1 || true)"
case "$missing_creds_output" in
  *"RENDER_API_KEY or RENDER_TOKEN is required"* ) ;;
  *) fail "rollback drill without credentials should fail before network execution" ;;
esac
pass "rollback drill stays dry-run-first and credential-gated"

echo "render launch guardrails passed"
