#!/usr/bin/env bash
# Fetch /openapi.json from the running instance of one version and store it as a
# versioned snapshot with sorted keys, so diffs between versions stay readable.
# The snapshot is replaced only once the new document has been fetched and parsed.
set -euo pipefail
# shellcheck source=hack/openwebui/lib.sh
source "$(dirname "$0")/lib.sh"

version="${1:-}"
owui_require_version "$version"
repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
rel="docs/api-notes/openapi/v$version.json"
out="$repo_root/$rel"

mkdir -p "$(dirname "$out")"
tmp="$(mktemp "$out.XXXXXX")"
trap 'rm -f "$tmp"' EXIT
curl -fsS "$(owui_url "$version")/openapi.json" | jq --sort-keys . >"$tmp"
mv "$tmp" "$out"
echo "wrote $rel"
