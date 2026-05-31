#!/usr/bin/env bash
# No-network prompt-matrix fixture breadth guardrail.
# Asserts that prompts/canary contains a launch-diverse set, not just 20
# similarly shaped React demos. Reads prompts/canary/matrix-manifest.json
# and verifies fixture diversity against explicit category/capability rules.
#
# Run: ./scripts/test/prompt_matrix_fixture_breadth_test.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PROMPT_DIR="$REPO_ROOT/prompts/canary"
MANIFEST="$PROMPT_DIR/matrix-manifest.json"

TESTS_RUN=0
TESTS_FAILED=0
pass() { TESTS_RUN=$((TESTS_RUN + 1)); echo "  ok - $1"; }
fail() { TESTS_RUN=$((TESTS_RUN + 1)); TESTS_FAILED=$((TESTS_FAILED + 1)); echo "  NOT OK - $1" >&2; }

echo "== prompt matrix fixture breadth guardrail (no network) =="

if [[ ! -f "$MANIFEST" ]]; then
  fail "manifest file not found: $MANIFEST"
  echo "Ran $TESTS_RUN assertions, $TESTS_FAILED failed."
  echo "PROMPT_MATRIX_BREADTH_TESTS_FAILED"
  exit 1
fi
pass "manifest file exists at $MANIFEST"

prompt_files=()
while IFS= read -r prompt_file; do
  prompt_files+=("$prompt_file")
done < <(find "$PROMPT_DIR" -maxdepth 1 -type f -name '*.md' ! -name 'matrix-manifest.*' | sort)
prompt_count="${#prompt_files[@]}"

# Run full validation in a single Node process.
# Output: one line per assertion: "ok|NOT OK <message>", then "DONE".
validations="$(node - "$MANIFEST" "$PROMPT_DIR" "$prompt_count" <<'NODE'
const fs = require("fs");
const path = require("path");
const [manifestPath, promptDir, rawPromptCount] = process.argv.slice(2);
const promptCount = Number(rawPromptCount);
const output = [];

function assert(condition, message) {
  output.push(condition ? `ok ${message}` : `NOT OK ${message}`);
}

const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
const entries = manifest.files || [];
const advertisedCategories = manifest.categories || {};

const mdFiles = fs.readdirSync(promptDir).filter(f => f.endsWith(".md"));
assert(mdFiles.length === 20, `exactly 20 markdown prompt files (found ${mdFiles.length})`);
assert(entries.length === 20, `exactly 20 manifest entries (found ${entries.length})`);

const entryFiles = new Set(entries.map(e => e.file));
let missing = 0;
for (const e of entries) {
  if (!mdFiles.includes(e.file)) { output.push(`NOT OK manifest references missing file: ${e.file}`); missing = 1; }
}
assert(!missing, "all manifest entries reference existing prompt files");

let unrep = 0;
for (const f of mdFiles) {
  if (!entryFiles.has(f)) { output.push(`NOT OK prompt file not represented in manifest: ${f}`); unrep = 1; }
}
assert(!unrep, "all prompt files are represented in the manifest");

const categoryCount = {};
for (const e of entries) {
  categoryCount[e.category] = (categoryCount[e.category] || 0) + 1;
}
let categoryMismatch = 0;
for (const c of ["simple", "medium", "complex"]) {
  if ((advertisedCategories[c] || 0) !== (categoryCount[c] || 0)) categoryMismatch = 1;
}
assert(!categoryMismatch, "advertised category counts match manifest entries");

output.push(""); // spacer for cat counts display
output.push(`CATS simple=${categoryCount.simple || 0} medium=${categoryCount.medium || 0} complex=${categoryCount.complex || 0}`);

assert((categoryCount.simple || 0) >= 6, `at least 6 simple/frontend-safe prompts (found ${categoryCount.simple || 0})`);
assert((categoryCount.medium || 0) >= 8, `at least 8 medium product/ops prompts (found ${categoryCount.medium || 0})`);
assert((categoryCount.complex || 0) >= 6, `at least 6 complex prompts (found ${categoryCount.complex || 0})`);

const REQ_CAPS = ["backend_api", "persistence", "auth_roles", "realtime_or_collab",
  "file_or_media", "commerce_or_billing_sim", "admin_reporting", "ai_simulation"];
const allCaps = new Set();
for (const e of entries) {
  for (const c of (e.capabilities || [])) allCaps.add(c);
}
let capCount = {};
for (const e of entries) {
  for (const c of (e.capabilities || [])) {
    capCount[c] = (capCount[c] || 0) + 1;
  }
}
for (const cap of REQ_CAPS) {
  const n = capCount[cap] || 0;
  assert(n > 0, `capability '${cap}' covered by at least 1 prompt (found ${n})`);
}

const complexViolations = entries
  .filter(e => e.category === "complex" && e.simulated !== true)
  .map(e => e.file || "unknown");
let nlc = 0;
for (const v of complexViolations) { output.push(`NOT OK complex prompt '${v}' is not marked simulated=true`); nlc = 1; }
assert(!nlc, "all complex prompts are marked simulated=true");

process.stdout.write(output.join("\n") + "\nDONE\n");
NODE
)"

# Parse Node output: assertions until DONE
while IFS= read -r line; do
  if [[ "$line" == "DONE" ]]; then
    break
  fi
  if [[ "$line" == ok\ * ]]; then
    pass "$(echo "$line" | sed 's/^ok //')"
  elif [[ "$line" == NOT\ OK\ * ]]; then
    fail "$(echo "$line" | sed 's/^NOT OK //')"
  elif [[ "$line" == CATS\ * ]]; then
    echo "  $(echo "$line" | sed 's/^CATS /categories: /')"
  fi
done <<< "$validations"

echo
echo "Ran $TESTS_RUN assertions, $TESTS_FAILED failed."
if [[ "$TESTS_FAILED" -gt 0 ]]; then
  echo "PROMPT_MATRIX_BREADTH_TESTS_FAILED"
  exit 1
fi
echo "PROMPT_MATRIX_BREADTH_TESTS_PASSED"
