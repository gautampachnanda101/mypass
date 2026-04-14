#!/usr/bin/env bash
# quality-gate.sh — run all quality gates (used in CI and local dev)
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

echo "→ go vet"
go vet ./...

echo "→ go test"
go test ./...

echo "✓ All quality gates passed"
