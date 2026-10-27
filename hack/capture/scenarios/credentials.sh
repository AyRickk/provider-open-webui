# shellcheck shell=bash
# The provider's credential check, GET /api/v1/users/default/permissions (ADR-0001): the answer
# to the admin's API key, to an invalid key, and to the API key of a non-admin user. Sourced by
# capture.sh; see hack/capture/lib.sh for capture_*.

user_email="capture-user@example.com"
user_password="throwaway-user-password"
CAPTURE_SECRETS+=("$user_password")

capture_scenario check
capture_call version GET /api/version >/dev/null
capture_call admin-key GET /api/v1/users/default/permissions >/dev/null
CAPTURE_KEY="sk-00000000000000000000000000000000" capture_call invalid-key GET /api/v1/users/default/permissions >/dev/null

# A non-admin user may only create an API key once the default permissions allow it.
capture_scenario non-admin-setup
capture_call add-user POST /api/v1/auths/add \
  "$(jq -nc --arg e "$user_email" --arg p "$user_password" '{name: "Capture User", email: $e, password: $p, role: "user"}')" >/dev/null
defaults="$(capture_call get-default-permissions GET /api/v1/users/default/permissions)"
capture_call allow-api-keys POST /api/v1/users/default/permissions "$(jq -c '.features.api_keys = true' <<<"$defaults")" >/dev/null
user_token="$(curl -fsS "$CAPTURE_URL/api/v1/auths/signin" -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg e "$user_email" --arg p "$user_password" '{email: $e, password: $p}')" | jq -er .token)"
user_key="$(curl -fsS -X POST "$CAPTURE_URL/api/v1/auths/api_key" -H "Authorization: Bearer $user_token" | jq -er .api_key)"
CAPTURE_SECRETS+=("$user_token" "$user_key")

capture_scenario non-admin
CAPTURE_KEY="$user_key" capture_call user-key GET /api/v1/users/default/permissions >/dev/null
