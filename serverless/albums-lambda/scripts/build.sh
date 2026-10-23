#!/usr/bin/env bash
# Builds the Lambda for provided.al2023 and packs it into build/webhook.zip.
# Usage: scripts/build.sh
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/env.sh

command -v go  >/dev/null || { echo "Go is required (https://go.dev/dl/)"; exit 1; }
command -v zip >/dev/null || { echo "zip is required"; exit 1; }

echo "Building for linux/${GOARCH_LAMBDA} (Lambda architecture: ${LAMBDA_ARCH})"
rm -rf build && mkdir -p build/webhook
# Every flag shortens the cold start (slide 16): CGO_ENABLED=0 gives a static binary,
# lambda.norpc drops the legacy RPC mode, -s -w drop the symbol table and DWARF.
GOOS=linux GOARCH="${GOARCH_LAMBDA}" CGO_ENABLED=0 \
  go build -tags lambda.norpc -ldflags="-s -w" -o build/webhook/bootstrap ./cmd/webhook
# provided.al2023 runs an executable named bootstrap (slide 15).
(cd build/webhook && zip -q -X ../webhook.zip bootstrap)
echo "  build/webhook.zip ($(du -h build/webhook.zip | cut -f1))"
