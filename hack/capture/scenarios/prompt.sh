# shellcheck shell=bash
# Prompt capture scenarios. Sourced by capture.sh; see hack/capture/lib.sh for capture_*.
# Writes are read back wherever what the server stores matters.

# prompt_body <command> <name> <content> [jq-filter] prints a PromptForm.
prompt_body() {
  jq -nc --arg command "$1" --arg name "$2" --arg content "$3" \
    "{command: \$command, name: \$name, content: \$content, tags: [\"capture\"]} | ${4:-.}"
}

changed_tags='.tags = ["capture", "changed"]'

# next_second waits for the clock to move on. A new history entry's parent is the latest entry by
# created_at, in seconds; two versions written in the same second make the parent arbitrary.
next_second() {
  sleep 1
}

capture_scenario setup
readers_group_id="$(capture_call setup-group-readers POST /api/v1/groups/create '{"name":"capture-readers","description":"Readers"}' | jq -er .id)"

capture_scenario create
prompt="$(capture_call create POST /api/v1/prompts/create "$(prompt_body capture-summary "Capture Summary" "Summarize: {{CLIPBOARD}}")" | jq -er .id)"
slashed="$(capture_call create-leading-slash POST /api/v1/prompts/create "$(prompt_body /capture-slashed "Slashed" "Slashed command")" | jq -er .id)"
capture_call create-duplicate-command POST /api/v1/prompts/create "$(prompt_body capture-summary "Same command" "Other content")" >/dev/null
capture_call create-duplicate-command-with-slash POST /api/v1/prompts/create "$(prompt_body /capture-summary "Same command with a slash" "Other content")" >/dev/null
capture_call create-slashed-command-without-slash POST /api/v1/prompts/create "$(prompt_body capture-slashed "Slashed without the slash" "Other content")" >/dev/null
capture_call create-command-other-case POST /api/v1/prompts/create "$(prompt_body Capture-Summary "Same command, other case" "Other content")" >/dev/null
capture_call create-duplicate-name POST /api/v1/prompts/create "$(prompt_body capture-other "Capture Summary" "Same name, other command")" >/dev/null
shared="$(capture_call create-with-access-grants POST /api/v1/prompts/create "$(prompt_body capture-shared "Capture Shared" "Shared prompt" \
  ".access_grants = [{principal_type: \"group\", principal_id: \"$readers_group_id\", permission: \"read\"}]")" | jq -er .id)"

# Prompts are routed by their UUID; a command in the id position is not found.
capture_scenario get
capture_call get-by-id GET "/api/v1/prompts/id/$prompt" >/dev/null
capture_call get-slashed GET "/api/v1/prompts/id/$slashed" >/dev/null
capture_call get-by-command GET /api/v1/prompts/id/capture-summary >/dev/null
capture_call get-shared GET "/api/v1/prompts/id/$shared" >/dev/null

capture_scenario list
capture_call list GET /api/v1/prompts/ >/dev/null
capture_call list-paginated GET "/api/v1/prompts/list?page=1" >/dev/null
capture_call tags GET /api/v1/prompts/tags >/dev/null

# No lookup by command: the paginated list filters on a query string.
capture_scenario find-by-command
capture_call query-command GET "/api/v1/prompts/list?query=capture-summary" >/dev/null
capture_call query-command-prefix GET "/api/v1/prompts/list?query=capture-s" >/dev/null
capture_call query-slashed-command GET "/api/v1/prompts/list?query=/capture-slashed" >/dev/null

capture_scenario update
next_second
capture_call update-name POST "/api/v1/prompts/id/$prompt/update" "$(prompt_body capture-summary "Capture Summary Renamed" "Summarize: {{CLIPBOARD}}")" >/dev/null
capture_call get-after-name GET "/api/v1/prompts/id/$prompt" >/dev/null
next_second
capture_call update-content POST "/api/v1/prompts/id/$prompt/update" "$(prompt_body capture-summary "Capture Summary Renamed" "Changed content")" >/dev/null
capture_call get-after-content GET "/api/v1/prompts/id/$prompt" >/dev/null
next_second
capture_call update-tags POST "/api/v1/prompts/id/$prompt/update" "$(prompt_body capture-summary "Capture Summary Renamed" "Changed content" "$changed_tags")" >/dev/null
capture_call get-after-tags GET "/api/v1/prompts/id/$prompt" >/dev/null
next_second
capture_call update-command POST "/api/v1/prompts/id/$prompt/update" "$(prompt_body capture-summary-v2 "Capture Summary Renamed" "Changed content" "$changed_tags")" >/dev/null
capture_call get-after-command GET "/api/v1/prompts/id/$prompt" >/dev/null
capture_call update-command-to-taken POST "/api/v1/prompts/id/$prompt/update" "$(prompt_body capture-other "Capture Summary Renamed" "Changed content" "$changed_tags")" >/dev/null
capture_call get-after-command-to-taken GET "/api/v1/prompts/id/$prompt" >/dev/null
# is_production=false with a new command, name and content: what is written, and is a version recorded?
capture_call history-before-not-production GET "/api/v1/prompts/id/$prompt/history" >/dev/null
next_second
capture_call update-not-production POST "/api/v1/prompts/id/$prompt/update" "$(prompt_body capture-summary-v3 "Draft name" "Draft content" "$changed_tags | .is_production = false")" >/dev/null
capture_call get-after-not-production GET "/api/v1/prompts/id/$prompt" >/dev/null
capture_call history-after-not-production GET "/api/v1/prompts/id/$prompt/history" >/dev/null

# Each toggle twice, with the state read after every call; is an inactive prompt still listed?
capture_scenario active
capture_call state-initial GET "/api/v1/prompts/id/$slashed" >/dev/null
capture_call toggle-1 POST "/api/v1/prompts/id/$slashed/toggle" >/dev/null
capture_call state-after-toggle-1 GET "/api/v1/prompts/id/$slashed" >/dev/null
capture_call list-while-inactive GET /api/v1/prompts/ >/dev/null
capture_call list-paginated-while-inactive GET "/api/v1/prompts/list?query=capture-slashed" >/dev/null
capture_call update-while-inactive POST "/api/v1/prompts/id/$slashed/update" "$(prompt_body /capture-slashed "Slashed" "Slashed command, changed while inactive")" >/dev/null
capture_call state-after-update-while-inactive GET "/api/v1/prompts/id/$slashed" >/dev/null
capture_call toggle-2 POST "/api/v1/prompts/id/$slashed/toggle" >/dev/null
capture_call state-after-toggle-2 GET "/api/v1/prompts/id/$slashed" >/dev/null

capture_scenario access-update
capture_call update-access POST "/api/v1/prompts/id/$shared/access/update" \
  "{\"access_grants\":[{\"principal_type\":\"group\",\"principal_id\":\"$readers_group_id\",\"permission\":\"write\"}]}" >/dev/null
capture_call get-after-update-access GET "/api/v1/prompts/id/$shared" >/dev/null
capture_call update-with-grants-in-body POST "/api/v1/prompts/id/$shared/update" "$(prompt_body capture-shared "Capture Shared" "Shared prompt" \
  '.access_grants = [{principal_type: "user", principal_id: "*", permission: "read"}]')" >/dev/null
capture_call get-after-update-with-grants GET "/api/v1/prompts/id/$shared" >/dev/null
capture_call make-private POST "/api/v1/prompts/id/$shared/access/update" '{"access_grants":[]}' >/dev/null
capture_call get-after-make-private GET "/api/v1/prompts/id/$shared" >/dev/null

# Version history, both versions: which writes add a version, on a fresh prompt.
capture_scenario history
fresh="$(capture_call create POST /api/v1/prompts/create "$(prompt_body capture-history "Capture History" "History content")" | jq -er .id)"
capture_call history-after-create GET "/api/v1/prompts/id/$fresh/history" >/dev/null
capture_call update-no-change POST "/api/v1/prompts/id/$fresh/update" "$(prompt_body capture-history "Capture History" "History content")" >/dev/null
capture_call history-after-no-change GET "/api/v1/prompts/id/$fresh/history" >/dev/null
next_second
capture_call update-change POST "/api/v1/prompts/id/$fresh/update" "$(prompt_body capture-history "Capture History" "Changed history content")" >/dev/null
capture_call history-after-change GET "/api/v1/prompts/id/$fresh/history" >/dev/null
capture_call toggle POST "/api/v1/prompts/id/$fresh/toggle" >/dev/null
capture_call history-after-toggle GET "/api/v1/prompts/id/$fresh/history" >/dev/null
capture_call update-access POST "/api/v1/prompts/id/$fresh/access/update" \
  "{\"access_grants\":[{\"principal_type\":\"group\",\"principal_id\":\"$readers_group_id\",\"permission\":\"read\"}]}" >/dev/null
capture_call history-after-access GET "/api/v1/prompts/id/$fresh/history" >/dev/null

capture_scenario delete
capture_call delete DELETE "/api/v1/prompts/id/$prompt/delete" >/dev/null
capture_call list-after-delete GET /api/v1/prompts/ >/dev/null
# The command of a deleted prompt can be used again.
capture_call create-reusing-deleted-command POST /api/v1/prompts/create "$(prompt_body capture-summary-v3 "Reused command" "Reused")" >/dev/null

capture_scenario get-missing
capture_call get-deleted GET "/api/v1/prompts/id/$prompt" >/dev/null
capture_call update-deleted POST "/api/v1/prompts/id/$prompt/update" "$(prompt_body capture-summary "x" "x")" >/dev/null
capture_call delete-deleted DELETE "/api/v1/prompts/id/$prompt/delete" >/dev/null
capture_call toggle-deleted POST "/api/v1/prompts/id/$prompt/toggle" >/dev/null
capture_call update-access-deleted POST "/api/v1/prompts/id/$prompt/access/update" '{"access_grants":[]}' >/dev/null
capture_call get-after-update-access-deleted GET "/api/v1/prompts/id/$prompt" >/dev/null
