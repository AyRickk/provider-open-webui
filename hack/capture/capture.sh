#!/usr/bin/env bash
# Capture the API fixtures of one resource on one Open WebUI version: start a fresh throwaway
# instance, replay hack/capture/scenarios/<resource>.sh against it, normalize and redact the
# exchanges (normalize.jq), replace test/fixtures/openwebui/v<version>/<resource>/ and tear the
# instance down. Re-running it regenerates byte-identical fixtures. Any failure (a step without
# HTTP response, a missing id, a leaked secret) leaves the committed fixtures untouched.
set -Eeuo pipefail
shopt -s inherit_errexit
here="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "$here/../.." && pwd)"
# shellcheck source=hack/openwebui/lib.sh
source "$repo_root/hack/openwebui/lib.sh"

resource="${1:-}"
version="${2:-}"
scenario_file="$here/scenarios/$resource.sh"
if [[ -z "$resource" || "$resource" == *.before || ! -f "$scenario_file" ]]; then
  echo "usage: mise run owui:capture <resource> <version>   resources: $(find "$here/scenarios" -name '*.sh' ! -name '*.before.sh' -exec basename {} .sh \; | sort | xargs)" >&2
  exit 2
fi
owui_require_version "$version"

# shellcheck disable=SC2034 # read by the scenario files, which shellcheck cannot follow
CAPTURE_VERSION="$version" CAPTURE_DIR="$here"
CAPTURE_SECRETS=()
work="$(mktemp -d)"
# A scenario that starts helpers (e.g. a fake upstream) defines capture_cleanup to remove them.
trap 'if declare -F capture_cleanup >/dev/null; then capture_cleanup || true; fi; rm -rf "$work"' EXIT

# scenarios/<resource>.before.sh, when present, runs before the instance starts: it can start
# helpers and set OWUI_NETWORK / OWUI_EXTRA_ENV for hack/openwebui/up.sh.
if [[ -f "$here/scenarios/$resource.before.sh" ]]; then
  # shellcheck source=/dev/null
  source "$here/scenarios/$resource.before.sh"
fi
"$repo_root/hack/openwebui/up.sh" "$version" >"$work/up.env"
# The instance goes first: helpers' networks cannot be removed while it is still attached.
trap '"$repo_root/hack/openwebui/down.sh" "$version" >/dev/null 2>&1 || true
  if declare -F capture_cleanup >/dev/null; then capture_cleanup || true; fi; rm -rf "$work"' EXIT

CAPTURE_URL="$(sed -n 's/^OWUI_URL=//p' "$work/up.env")"
CAPTURE_KEY="$(sed -n 's/^OWUI_API_KEY=//p' "$work/up.env")"
CAPTURE_RAW="$work/raw.jsonl"
# shellcheck source=hack/capture/lib.sh
source "$here/lib.sh"
# shellcheck source=/dev/null
source "$scenario_file"

# Fixtures are cited as <scenario>#<step>, so step names must be unique within a scenario.
duplicates="$(jq -rs 'group_by([.scenario, .step]) | map(select(length > 1) | .[0] | "\(.scenario)#\(.step)") | .[]' "$CAPTURE_RAW")"
if [[ -n "$duplicates" ]]; then
  echo "duplicate step names: $duplicates" >&2
  exit 1
fi

staged="$work/fixtures"
mkdir -p "$staged"
jq -s -L "$here" --arg version "$version" --arg resource "$resource" -f "$here/normalize.jq" "$CAPTURE_RAW" >"$work/fixtures.json"
jq -r 'keys_unsorted[]' "$work/fixtures.json" | while read -r scenario; do
  jq --arg s "$scenario" '.[$s]' "$work/fixtures.json" >"$staged/$scenario.json"
done

# No secret sent or received during the run, nor anything shaped like one, may reach a fixture.
jq -r -L "$here" 'include "secrets"; secret_values | select(length >= 8)' "$CAPTURE_RAW" >"$work/secrets"
# A before.sh may add the credentials of its helpers to CAPTURE_SECRETS.
printf '%s\n' "$CAPTURE_KEY" "$OWUI_ADMIN_PASSWORD" "$OWUI_SECRET_KEY" "${CAPTURE_SECRETS[@]}" >>"$work/secrets"
if grep -rqF -f "$work/secrets" "$staged" || grep -rqE 'sk-[0-9a-f]{32}|eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.' "$staged"; then
  echo "a secret leaked into the captured fixtures; nothing written" >&2
  exit 1
fi

out="$repo_root/test/fixtures/openwebui/v$version/$resource"
rm -rf "$out"
mkdir -p "$(dirname "$out")"
mv "$staged" "$out"
echo "wrote $(find "$out" -name '*.json' | wc -l | tr -d ' ') fixtures to ${out#"$repo_root/"} (no secret found)"
