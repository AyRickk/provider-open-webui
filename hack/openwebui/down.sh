#!/usr/bin/env bash
# Tear down the throwaway Open WebUI instance of one version, including its data.
set -euo pipefail
# shellcheck source=hack/openwebui/lib.sh
source "$(dirname "$0")/lib.sh"

version="${1:-}"
owui_require_version "$version"

docker rm --force --volumes "$(owui_container "$version")"
