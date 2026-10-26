# shellcheck shell=bash
# Group capture scenarios. Sourced by capture.sh; see hack/capture/lib.sh for capture_*.
# Each scenario reads the server state after each write, so the fixtures show what the
# server stores, not only what it answers.

capture_scenario create
minimal="$(capture_call create-minimal POST /api/v1/groups/create \
  '{"name":"capture-minimal","description":"Group with name and description only"}' | jq -er .id)"
group="$(capture_call create-with-permissions POST /api/v1/groups/create \
  '{"name":"capture-engineering","description":"Engineering","permissions":{"workspace":{"models":true,"tools":true},"chat":{"file_upload":false}}}' | jq -er .id)"

capture_scenario get
capture_call get-minimal GET "/api/v1/groups/id/$minimal" >/dev/null
capture_call get GET "/api/v1/groups/id/$group" >/dev/null
capture_call get-info GET "/api/v1/groups/id/$group/info" >/dev/null

capture_scenario list
capture_call list GET /api/v1/groups/ >/dev/null

# There is no lookup by name: the provider lists and filters. A second group with the
# same name shows whether names are unique.
capture_scenario find-by-name
duplicate="$(capture_call create-duplicate-name POST /api/v1/groups/create \
  '{"name":"capture-engineering","description":"Same name as an existing group"}' | jq -er .id)"
capture_call list-after-duplicate GET /api/v1/groups/ >/dev/null
capture_call delete-duplicate DELETE "/api/v1/groups/id/$duplicate/delete" >/dev/null

capture_scenario update-description
capture_call update-description POST "/api/v1/groups/id/$group/update" \
  '{"name":"capture-engineering","description":"Engineering, updated"}' >/dev/null
capture_call get-after-update GET "/api/v1/groups/id/$group" >/dev/null

# Sends a subset of the stored permissions: the result shows replace versus merge.
capture_scenario update-permissions
capture_call update-permissions POST "/api/v1/groups/id/$group/update" \
  '{"name":"capture-engineering","description":"Engineering, updated","permissions":{"workspace":{"models":false}}}' >/dev/null
capture_call get-after-update GET "/api/v1/groups/id/$group" >/dev/null

capture_scenario update-unknown-permission-keys
capture_call update-unknown-keys POST "/api/v1/groups/id/$group/update" \
  '{"name":"capture-engineering","description":"Engineering, updated","permissions":{"workspace":{"models":true,"not_a_real_key":true},"not_a_real_category":{"anything":false}}}' >/dev/null
capture_call get-after-update GET "/api/v1/groups/id/$group" >/dev/null

# The full permission set the server knows: the defaults, and what a group effectively gets.
capture_scenario permission-set
capture_call default-permissions GET /api/v1/users/default/permissions >/dev/null
capture_call default-permissions-defaults GET /api/v1/users/default/permissions/defaults >/dev/null
capture_call preview GET "/api/v1/groups/id/$group/preview" >/dev/null

# Members are set up here only to observe how they are read; the provider never writes them.
capture_scenario members
member="$(capture_call setup-add-user POST /api/v1/auths/add \
  '{"name":"Capture Member","email":"member@example.com","password":"capture-member-password","role":"user"}' | jq -er .id)"
capture_call setup-add-member POST "/api/v1/groups/id/$group/users/add" "{\"user_ids\":[\"$member\"]}" >/dev/null
capture_call get GET "/api/v1/groups/id/$group" >/dev/null
capture_call export GET "/api/v1/groups/id/$group/export" >/dev/null
capture_call users POST "/api/v1/groups/id/$group/users" >/dev/null
capture_call members GET "/api/v1/groups/id/$group/members" >/dev/null

capture_scenario delete
capture_call delete DELETE "/api/v1/groups/id/$group/delete" >/dev/null
capture_call delete-minimal DELETE "/api/v1/groups/id/$minimal/delete" >/dev/null
capture_call list-after-delete GET /api/v1/groups/ >/dev/null

# A deleted group, then an id that never existed, on every endpoint that takes an id.
capture_scenario get-missing
for missing in "deleted:$group" "never-existed:missing-group-id"; do
  kind="${missing%%:*}"
  id="${missing#*:}"
  for endpoint in "" /info /export /preview /members; do
    capture_call "get-$kind${endpoint//\//-}" GET "/api/v1/groups/id/$id$endpoint" >/dev/null
  done
  capture_call "update-$kind" POST "/api/v1/groups/id/$id/update" '{"name":"missing","description":"missing"}' >/dev/null
  capture_call "delete-$kind" DELETE "/api/v1/groups/id/$id/delete" >/dev/null
done
