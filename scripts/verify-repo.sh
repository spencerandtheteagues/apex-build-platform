#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "== Backend =="
cd "$ROOT_DIR/backend"
go version
go mod download
go test ./...
go build -o /tmp/apex-server ./cmd

echo "== Frontend =="
cd "$ROOT_DIR/frontend"
node --version
npm ci
npm run typecheck
npm run build
