#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VENDOR="$ROOT/third_party/ranuts-document"
PATCH_FILE="$ROOT/patches/ranuts-document-disable-embedded-service-worker.patch"

if ! grep -q "if (!isEmbedded && 'serviceWorker' in navigator)" "$VENDOR/index.ts"; then
  git -C "$VENDOR" apply "$PATCH_FILE"
fi
