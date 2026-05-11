#!/usr/bin/env bash
# GemmaLink — Dev build (static files letti da disco, no embed)
# Usage: ./scripts/build-dev.sh
# Or use go run -tags dev . from root
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
go build -tags dev -o gemmalink .
echo "✓ gemmalink (dev) built"
