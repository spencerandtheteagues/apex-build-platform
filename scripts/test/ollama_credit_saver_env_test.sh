#!/usr/bin/env bash
# Guardrail tests for scripts/ollama-credit-saver-env.sh.
#
# These tests make no network calls. A fake ollama binary supplies deterministic
# local model discovery so local credit-saver defaults do not drift back to
# Cloud-only model names when a localhost daemon is available.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/ollama-credit-saver-env.sh"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

TESTS_RUN=0
TESTS_FAILED=0
pass() { TESTS_RUN=$((TESTS_RUN + 1)); echo "  ok - $1"; }
fail() { TESTS_RUN=$((TESTS_RUN + 1)); TESTS_FAILED=$((TESTS_FAILED + 1)); echo "  NOT OK - $1" >&2; }

cat > "$TMP_DIR/ollama" <<'EOS'
#!/usr/bin/env bash
if [[ "$1" == "list" ]]; then
  printf 'NAME                 ID              SIZE      MODIFIED\n'
  printf 'qwen2.5-coder:14b    testdigest      9.0 GB    now\n'
  exit 0
fi
exit 1
EOS
chmod +x "$TMP_DIR/ollama"

source_with_env() {
  env -i \
    PATH="$TMP_DIR:/usr/bin:/bin" \
    OLLAMA_URL="${1:-http://127.0.0.1:11434}" \
    KIMI_OLLAMA_MODEL="${KIMI_OLLAMA_MODEL-}" \
    GLM_OLLAMA_MODEL="${GLM_OLLAMA_MODEL-}" \
    DEEPSEEK_OLLAMA_MODEL="${DEEPSEEK_OLLAMA_MODEL-}" \
    QWEN_OLLAMA_MODEL="${QWEN_OLLAMA_MODEL-}" \
    bash -c 'source "$1" >/dev/null; printf "%s\n%s\n%s\n%s\n" "$KIMI_OLLAMA_MODEL" "$GLM_OLLAMA_MODEL" "$DEEPSEEK_OLLAMA_MODEL" "$QWEN_OLLAMA_MODEL"' _ "$SCRIPT"
}

echo "== ollama credit-saver env guardrails (no network) =="

output="$(source_with_env "http://127.0.0.1:11434")"
if [[ "$output" == $'qwen2.5-coder:14b\nqwen2.5-coder:14b\nqwen2.5-coder:14b\nqwen2.5-coder:14b' ]]; then
  pass "localhost defaults use installed local model"
else
  fail "localhost defaults did not use installed local model"
  echo "$output" >&2
fi

output="$(source_with_env "https://ollama.com/v1")"
if [[ "$output" == $'kimi-k2.6\nglm-5.1\ndeepseek-v4-pro\nqwen3:latest' ]]; then
  pass "hosted Ollama keeps Cloud model defaults"
else
  fail "hosted Ollama defaults changed unexpectedly"
  echo "$output" >&2
fi

KIMI_OLLAMA_MODEL="custom-kimi"
output="$(source_with_env "http://127.0.0.1:11434")"
unset KIMI_OLLAMA_MODEL
if [[ "$output" == $'custom-kimi\nqwen2.5-coder:14b\nqwen2.5-coder:14b\nqwen2.5-coder:14b' ]]; then
  pass "explicit model overrides are preserved"
else
  fail "explicit model override was not preserved"
  echo "$output" >&2
fi

echo
echo "Ran $TESTS_RUN assertions, $TESTS_FAILED failed."
if [[ "$TESTS_FAILED" -gt 0 ]]; then
  echo "OLLAMA_CREDIT_SAVER_ENV_TESTS_FAILED"
  exit 1
fi
echo "OLLAMA_CREDIT_SAVER_ENV_TESTS_PASSED"
