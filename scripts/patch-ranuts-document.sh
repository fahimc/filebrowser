#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VENDOR="$ROOT/third_party/ranuts-document"
PATCH_FILE="$ROOT/patches/ranuts-document-disable-embedded-service-worker.patch"

if ! grep -q "if (!isEmbedded && 'serviceWorker' in navigator)" "$VENDOR/index.ts"; then
  git -C "$VENDOR" apply "$PATCH_FILE"
fi

if ! grep -q 'payload.preserveChanges' "$VENDOR/lib/embed-api.ts"; then
  git -C "$VENDOR" apply "$ROOT/patches/ranuts-document-preserve-collaboration-history.patch"
elif ! grep -q 'Embedded live sessions must still export' "$VENDOR/lib/onlyoffice/guards/x2t-worker.ts"; then
  git -C "$VENDOR" apply --include=lib/onlyoffice/guards/x2t-worker.ts "$ROOT/patches/ranuts-document-preserve-collaboration-history.patch"
fi
