#!/usr/bin/env bash
# The whole e2e run: build the provider, create the cluster, deploy the provider, run the
# suite against each Open WebUI version, then delete the cluster.
#   hack/e2e/run.sh [version...]    defaults to every version of the supported matrix
# The cluster is deleted afterwards, even one that existed before the run, unless
# E2E_KEEP_CLUSTER=true, to iterate with hack/e2e/test.sh.
set -Eeuo pipefail
cd "$(dirname "$0")/../.."
# shellcheck source=hack/e2e/lib.sh
source hack/e2e/lib.sh

versions=("$@")
if [[ ${#versions[@]} -eq 0 ]]; then
  versions=("$E2E_OWUI_PREVIOUS" "$E2E_OWUI_LATEST")
fi

if [[ "${E2E_KEEP_CLUSTER:-false}" != true ]]; then
  trap 'hack/e2e/down.sh' EXIT
fi

make build
hack/e2e/up.sh
hack/e2e/deploy.sh

failed=()
for version in "${versions[@]}"; do
  echo "=== e2e against Open WebUI $version" >&2
  hack/e2e/test.sh "$version" || failed+=("$version")
done

if [[ ${#failed[@]} -gt 0 ]]; then
  echo "e2e failed against Open WebUI ${failed[*]}" >&2
  exit 1
fi
echo "e2e passed against Open WebUI ${versions[*]}" >&2
