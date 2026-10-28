# shellcheck shell=bash
# Start the instance with API-key endpoint restrictions on, allowing exactly the list in
# docs/api-notes/api-key-allowed-endpoints.txt (owui_restricted_env in hack/openwebui/lib.sh).
OWUI_EXTRA_ENV="$(owui_restricted_env)"
export OWUI_EXTRA_ENV
