#!/usr/bin/env bash
# Run the contract tests of test/contract (spec #3, seam S1) against real Open WebUI instances.
#   hack/contract/run.sh [--record] [version...]   defaults to every version of owui_matrix
# For each version, one fresh instance after the other (hack/openwebui/up.sh):
#   default     the instance as bootstrapped, for the contracts that need no restriction;
#   restricted  its API keys restricted to docs/api-notes/api-key-allowed-endpoints.txt.
# With --record, the recording mode: the recordings of the default instance's run replace the
# fixtures of their resources under test/fixtures/openwebui/v<version>/ (hack/capture/write.sh),
# once every test of that version passed. Recording again regenerates byte-identical fixtures.
set -Eeuo pipefail
cd "$(dirname "$0")/../.."
# shellcheck source=hack/openwebui/lib.sh
source hack/openwebui/lib.sh

record=false
if [[ "${1:-}" == --record ]]; then
  record=true
  shift
fi
versions=("$@")
if [[ ${#versions[@]} -eq 0 ]]; then
  mapfile -t versions < <(owui_matrix)
fi
for version in "${versions[@]}"; do
  owui_require_version "$version"
done

work="$(mktemp -d)"
current=""
cleanup() {
  if [[ -n "$current" ]]; then
    hack/openwebui/down.sh "$current" >/dev/null 2>&1 || true
  fi
  rm -rf "$work"
}
trap cleanup EXIT

# contract_run <version> <instance kind> [record dir] starts an instance, runs the tests
# against it and removes it.
contract_run() {
  local version="$1" kind="$2" record_dir="${3:-}" env status=0
  env="$work/$version-$kind.env"
  current="$version"
  if [[ "$kind" == restricted ]]; then
    OWUI_EXTRA_ENV="$(owui_restricted_env)" hack/openwebui/up.sh "$version" >"$env"
  else
    hack/openwebui/up.sh "$version" >"$env"
  fi
  OWUI_URL="$(sed -n 's/^OWUI_URL=//p' "$env")" \
    OWUI_API_KEY="$(sed -n 's/^OWUI_API_KEY=//p' "$env")" \
    OWUI_VERSION="$version" CONTRACT_INSTANCE="$kind" CONTRACT_RECORD_DIR="$record_dir" \
    go test -tags contract -count=1 -v ./test/contract/... || status=$?
  if [[ -n "$record_dir" ]]; then
    # The secrets of the instance, for write.sh's leak check.
    sed -n 's/^OWUI_API_KEY=//p' "$env" >"$record_dir/instance.secrets"
    printf '%s\n' "$OWUI_ADMIN_PASSWORD" "$OWUI_SECRET_KEY" >>"$record_dir/instance.secrets"
  fi
  hack/openwebui/down.sh "$version" >/dev/null
  current=""
  return "$status"
}

failed=()
for version in "${versions[@]}"; do
  echo "=== contract tests against Open WebUI $version" >&2
  record_dir=""
  if [[ "$record" == true ]]; then
    record_dir="$work/record-$version"
    mkdir -p "$record_dir"
  fi
  if ! contract_run "$version" default "$record_dir" || ! contract_run "$version" restricted; then
    failed+=("$version")
    continue
  fi
  if [[ "$record" == true ]]; then
    for raw in "$record_dir"/*.jsonl; do
      [[ -e "$raw" ]] || continue
      resource="$(basename "$raw" .jsonl)"
      cat "$record_dir/$resource.secrets" "$record_dir/instance.secrets" >"$record_dir/$resource.all-secrets"
      hack/capture/write.sh "$raw" "$record_dir/$resource.all-secrets" "$version" "$resource"
    done
  fi
done

if [[ ${#failed[@]} -gt 0 ]]; then
  echo "contract tests failed against Open WebUI ${failed[*]}" >&2
  exit 1
fi
echo "contract tests passed against Open WebUI ${versions[*]}" >&2
