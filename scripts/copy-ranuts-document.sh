#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DESTINATION="${1:-$ROOT/backend/internal/web/embed}"
SOURCE="$ROOT/third_party/ranuts-document/dist"

if [[ ! -f "$SOURCE/editor.html" ]]; then
  echo "ranuts/document has not been built: $SOURCE/editor.html is missing" >&2
  echo "Run pnpm install --frozen-lockfile and pnpm run build in third_party/ranuts-document." >&2
  exit 1
fi

mkdir -p "$DESTINATION/ranuts-document"
cp -a "$SOURCE/." "$DESTINATION/ranuts-document/"
