#!/usr/bin/env bash
# L0 — Apex Dominance Master Plan verification gate.
# The single source of truth for "is this branch mergeable?".
# No lane output is accepted unless this exits 0.
#
# Usage:  scripts/verify_all.sh [--fast]
#   --fast  skip the full Go test run (build+vet+typecheck+fe-test+lint+fe-build only)
#
# Exit 0 = GREEN (mergeable). Non-zero = RED (do not merge).

set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FAST=0
[[ "${1:-}" == "--fast" ]] && FAST=1

PASS=(); FAIL=()
step() {
  local name="$1"; shift
  echo ""
  echo "==> $name"
  if "$@"; then PASS+=("$name"); else FAIL+=("$name"); fi
}

step "backend: go build"  bash -c 'cd backend && go build ./...'
step "backend: go vet"    bash -c 'cd backend && go vet ./...'
if [[ $FAST -eq 0 ]]; then
  step "backend: go test"  bash -c 'cd backend && go test ./... -timeout 12m'
else
  echo "(skipping go test — --fast)"
fi
step "frontend: typecheck" bash -c 'cd frontend && npm run -s typecheck'
step "frontend: test"      bash -c 'cd frontend && npm run -s test -- --run'
step "frontend: lint"      bash -c 'cd frontend && npm run -s lint'
step "frontend: build"     bash -c 'cd frontend && npm run -s build'

echo ""
echo "================ VERIFY_ALL REPORT ================"
for p in "${PASS[@]:-}"; do [[ -n "$p" ]] && echo "  PASS  $p"; done
for f in "${FAIL[@]:-}"; do [[ -n "$f" ]] && echo "  FAIL  $f"; done
echo "=================================================="

if [[ ${#FAIL[@]} -gt 0 ]]; then
  echo "GATE: RED (${#FAIL[@]} failing) — do NOT merge"
  exit 1
fi
echo "GATE: GREEN — mergeable"
exit 0
