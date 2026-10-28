#!/usr/bin/env bash
# Turn the raw exchanges of one recording into the fixtures of one resource on one version:
# check the step names are unique, normalize and redact them (normalize.jq), check no secret
# reached them, and replace test/fixtures/openwebui/v<version>/<resource>/.
#   hack/capture/write.sh <raw.jsonl> <secrets-file> <version> <resource>
# The raw exchanges are those of capture_call (hack/capture/lib.sh), or of the contract tests'
# recorder (test/contract). The secrets file lists, one per line, the secrets the recording
# used; any value under a secret key of the raw exchanges is added to it. Any failure leaves
# the committed fixtures untouched.
set -Eeuo pipefail
shopt -s inherit_errexit
here="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "$here/../.." && pwd)"

if [[ $# -ne 4 ]]; then
  echo "usage: hack/capture/write.sh <raw.jsonl> <secrets-file> <version> <resource>" >&2
  exit 2
fi
raw="$1" secrets_file="$2" version="$3" resource="$4"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# Fixtures are cited as <scenario>#<step>, so step names must be unique within a scenario.
duplicates="$(jq -rs 'group_by([.scenario, .step]) | map(select(length > 1) | .[0] | "\(.scenario)#\(.step)") | .[]' "$raw")"
if [[ -n "$duplicates" ]]; then
  echo "duplicate step names: $duplicates" >&2
  exit 1
fi

staged="$work/fixtures"
mkdir -p "$staged"
jq -s -L "$here" --arg version "$version" --arg resource "$resource" -f "$here/normalize.jq" "$raw" >"$work/fixtures.json"
jq -r 'keys_unsorted[]' "$work/fixtures.json" | while read -r scenario; do
  jq --arg s "$scenario" '.[$s]' "$work/fixtures.json" >"$staged/$scenario.json"
done

# No secret sent or received during the run, nor anything shaped like one, may reach a fixture.
jq -r -L "$here" 'include "secrets"; secret_values | select(length >= 8)' "$raw" >"$work/secrets"
grep -v '^$' "$secrets_file" >>"$work/secrets" || true
if grep -rqF -f "$work/secrets" "$staged" || grep -rqE 'sk-[0-9a-f]{32}|eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.' "$staged"; then
  echo "a secret leaked into the captured fixtures; nothing written" >&2
  exit 1
fi

out="$repo_root/test/fixtures/openwebui/v$version/$resource"
rm -rf "$out"
mkdir -p "$(dirname "$out")"
mv "$staged" "$out"
echo "wrote $(find "$out" -name '*.json' | wc -l | tr -d ' ') fixtures to ${out#"$repo_root/"} (no secret found)"
