# shellcheck shell=bash
# Start the instance with API-key endpoint restrictions on, allowing exactly the list in
# docs/api-notes/api-key-allowed-endpoints.txt (the settings only apply on a first boot).
allowed="$(paste -sd ',' "$CAPTURE_DIR/../../docs/api-notes/api-key-allowed-endpoints.txt")"
OWUI_EXTRA_ENV="ENABLE_API_KEYS_ENDPOINT_RESTRICTIONS=true
API_KEYS_ALLOWED_ENDPOINTS=$allowed"
export OWUI_EXTRA_ENV
