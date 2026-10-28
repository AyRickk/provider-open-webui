#!/usr/bin/env bash
# Delete the e2e cluster and its registry.
set -euo pipefail
cd "$(dirname "$0")/../.."
# shellcheck source=hack/e2e/lib.sh
source hack/e2e/lib.sh

kind delete cluster --name "$E2E_CLUSTER"
docker rm --force --volumes "$E2E_REGISTRY" >/dev/null 2>&1 || true
