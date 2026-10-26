# shellcheck shell=bash
# Model capture scenarios. Sourced by capture.sh; see hack/capture/lib.sh for capture_*.
# Writes are followed by a read wherever what the server stores matters.

# A fake OpenAI-compatible upstream (hack/capture/fake-openai.py) exposes the base model
# "capture-base", on a private network shared with the instance.
instance="$(owui_container "$CAPTURE_VERSION")"
upstream="$instance-upstream"
network="$instance-net"
capture_cleanup() {
  docker rm --force "$upstream" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
}
docker network create "$network" >/dev/null
docker network connect "$network" "$instance"
docker run --detach --name "$upstream" --network "$network" --entrypoint python \
  -v "$CAPTURE_DIR/fake-openai.py:/fake-openai.py:ro" "$OWUI_IMAGE:v$CAPTURE_VERSION-slim" /fake-openai.py >/dev/null
# The upstream must answer before the connection is configured, or /api/models would vary.
for _ in $(seq 1 30); do
  docker exec "$upstream" python -c 'import urllib.request; urllib.request.urlopen("http://localhost:8000/v1/models")' 2>/dev/null && break
  sleep 1
done
docker exec "$upstream" python -c 'import urllib.request; urllib.request.urlopen("http://localhost:8000/v1/models")'

# model_body [jq-filter] prints the full model payload used throughout, with the filter applied.
model_body() {
  jq -nc "{
    id: \"capture-assistant\", name: \"Capture Assistant\", base_model_id: \"capture-base\",
    meta: {description: \"Assistant defined as code\", tags: [{name: \"capture\"}],
           capabilities: {vision: false, citations: true},
           suggestion_prompts: [{content: \"Summarize this document\"}],
           toolIds: [], knowledge: [], filterIds: [], actionIds: [], hidden: false},
    params: {system: \"You are a capture assistant.\", temperature: 0.2, top_p: 0.9, max_tokens: 512},
    is_active: true
  } | ${1:-.}"
}

# --- setup: the upstream connection, two groups and a user to share with ---
capture_scenario setup
capture_call setup-connection POST /openai/config/update \
  "{\"ENABLE_OPENAI_API\":true,\"OPENAI_API_BASE_URLS\":[\"http://$upstream:8000/v1\"],\"OPENAI_API_KEYS\":[\"capture-upstream-key\"],\"OPENAI_API_CONFIGS\":{}}" >/dev/null
readers="$(capture_call setup-group-readers POST /api/v1/groups/create '{"name":"capture-readers","description":"Readers"}' | jq -er .id)"
writers="$(capture_call setup-group-writers POST /api/v1/groups/create '{"name":"capture-writers","description":"Writers"}' | jq -er .id)"
user="$(capture_call setup-user POST /api/v1/auths/add \
  '{"name":"Capture Reader","email":"reader@example.com","password":"capture-reader-password","role":"user"}' | jq -er .id)"
knowledge="$(capture_call setup-knowledge POST /api/v1/knowledge/create '{"name":"capture-kb","description":"Knowledge base to reference"}' | jq -er .id)"

# --- base models: what the "base model not exposed" warning can be checked against ---
capture_scenario base-models
capture_call app-models GET /api/models | jq -e 'any(.data[]; .id == "capture-base")' >/dev/null
capture_call base GET /api/v1/models/base >/dev/null

capture_scenario create
capture_call create POST /api/v1/models/create "$(model_body)" >/dev/null
capture_call create-duplicate-id POST /api/v1/models/create "$(model_body '.name = "Same id"')" >/dev/null
capture_call create-unexposed-base POST /api/v1/models/create \
  "$(model_body '.id = "capture-unexposed" | .base_model_id = "not-exposed-anywhere"')" >/dev/null

# Grants for every audience, plus entries the server should reject.
capture_scenario create-with-access-grants
capture_call create POST /api/v1/models/create "$(model_body ".id = \"capture-shared\" | .access_grants = [
  {principal_type: \"user\", principal_id: \"*\", permission: \"read\"},
  {principal_type: \"group\", principal_id: \"$readers\", permission: \"read\"},
  {principal_type: \"group\", principal_id: \"$writers\", permission: \"write\"},
  {principal_type: \"user\", principal_id: \"$user\", permission: \"read\"},
  {principal_type: \"user\", principal_id: \"$user\", permission: \"write\"},
  {principal_type: \"anyone\", principal_id: \"*\", permission: \"read\"},
  {principal_type: \"bogus\", principal_id: \"x\", permission: \"read\"},
  {principal_type: \"group\", principal_id: \"$readers\", permission: \"admin\"}]")" >/dev/null
capture_call get GET "/api/v1/models/model?id=capture-shared" >/dev/null

capture_scenario get
capture_call get GET "/api/v1/models/model?id=capture-assistant" >/dev/null

capture_scenario list
capture_call all GET /api/v1/models/all >/dev/null
capture_call list GET /api/v1/models/list >/dev/null
capture_call app-models GET /api/models >/dev/null

capture_scenario export
capture_call export-by-ids GET "/api/v1/models/export?ids=capture-assistant&ids=capture-shared" >/dev/null
capture_call export-all GET /api/v1/models/export >/dev/null

# One managed field at a time, each followed by a read.
capture_scenario update-fields
step_update() {
  capture_call "update-$1" POST /api/v1/models/model/update "$(model_body "$2")" >/dev/null
  capture_call "get-after-$1" GET "/api/v1/models/model?id=capture-assistant" >/dev/null
}
step_update name '.name = "Capture Assistant v2"'
step_update base-model '.base_model_id = "capture-unexposed"'
step_update description '.meta.description = "Changed description"'
step_update tags '.meta.tags = ["plain-string", {name: "object"}]'
step_update capabilities '.meta.capabilities = {vision: true, citations: false, unknown_capability: true}'
step_update suggestion-prompts '.meta.suggestion_prompts = [{content: "Another prompt"}]'
step_update system '.params.system = "Changed system prompt"'
step_update temperature '.params.temperature = 0.7'
step_update top-p '.params.top_p = 0.5'
step_update max-tokens '.params.max_tokens = 1024'
step_update additional-params '.params.seed = 42 | .params.custom_param = {nested: [1, 2]}'
step_update tools '.meta.toolIds = ["capture_tool_that_does_not_exist"]'
step_update knowledge ".meta.knowledge = [{id: \"$knowledge\", name: \"capture-kb\", type: \"collection\"}]"
step_update filters '.meta.filterIds = ["capture_filter_that_does_not_exist"]'
step_update actions '.meta.actionIds = ["capture_action_that_does_not_exist"]'
step_update hidden '.meta.hidden = true'
step_update inactive '.is_active = false'
step_update partial-payload '{id: .id, name: .name, meta: {}, params: {}}'
capture_call toggle POST "/api/v1/models/model/toggle?id=capture-assistant" >/dev/null
capture_call get-after-toggle GET "/api/v1/models/model?id=capture-assistant" >/dev/null

capture_scenario access-update
capture_call update-access POST /api/v1/models/model/access/update \
  "{\"id\":\"capture-shared\",\"access_grants\":[{\"principal_type\":\"group\",\"principal_id\":\"$readers\",\"permission\":\"read\"}]}" >/dev/null
capture_call get-after-update-access GET "/api/v1/models/model?id=capture-shared" >/dev/null
capture_call update-with-grants-in-body POST /api/v1/models/model/update "$(model_body ".id = \"capture-shared\" | .access_grants = [
  {principal_type: \"user\", principal_id: \"*\", permission: \"read\"}]")" >/dev/null
capture_call get-after-update-with-grants GET "/api/v1/models/model?id=capture-shared" >/dev/null
capture_call grant-missing-group POST /api/v1/models/model/access/update \
  '{"id":"capture-shared","access_grants":[{"principal_type":"group","principal_id":"missing-group-id","permission":"read"}]}' >/dev/null
capture_call get-after-grant-missing-group GET "/api/v1/models/model?id=capture-shared" >/dev/null
capture_call make-private POST /api/v1/models/model/access/update '{"id":"capture-shared","access_grants":[]}' >/dev/null
capture_call get-after-make-private GET "/api/v1/models/model?id=capture-shared" >/dev/null

# Version history (0.12.4): which writes add a version, starting from a fresh model.
capture_scenario history
capture_call create POST /api/v1/models/create "$(model_body '.id = "capture-history"')" >/dev/null
capture_call history-after-create GET "/api/v1/models/model/history?id=capture-history" >/dev/null
capture_call update-no-change POST /api/v1/models/model/update "$(model_body '.id = "capture-history"')" >/dev/null
capture_call history-after-no-change GET "/api/v1/models/model/history?id=capture-history" >/dev/null
capture_call update-change POST /api/v1/models/model/update "$(model_body '.id = "capture-history" | .params.temperature = 0.9')" >/dev/null
capture_call history-after-change GET "/api/v1/models/model/history?id=capture-history" >/dev/null
capture_call update-access-only POST /api/v1/models/model/access/update \
  "{\"id\":\"capture-history\",\"access_grants\":[{\"principal_type\":\"group\",\"principal_id\":\"$readers\",\"permission\":\"read\"}]}" >/dev/null
capture_call history-after-access-only GET "/api/v1/models/model/history?id=capture-history" >/dev/null
capture_call update-is-active-only POST /api/v1/models/model/update "$(model_body '.id = "capture-history" | .params.temperature = 0.9 | .is_active = false')" >/dev/null
capture_call history-after-is-active-only GET "/api/v1/models/model/history?id=capture-history" >/dev/null
capture_call toggle POST "/api/v1/models/model/toggle?id=capture-history" >/dev/null
capture_call history-after-toggle GET "/api/v1/models/model/history?id=capture-history" >/dev/null
# Only 0.12.4 has versions; on 0.11.4 the model has no version_id.
current_version="$(capture_call get GET "/api/v1/models/model?id=capture-history" | jq -r '.version_id // ""')"
if [[ "$CAPTURE_VERSION" != 0.11.* ]]; then
  [[ -n "$current_version" ]] || { echo "capture: history/get: no version_id on $CAPTURE_VERSION" >&2; exit 1; }
  capture_call history-entry GET "/api/v1/models/model/history/$current_version?id=capture-history" >/dev/null
fi

capture_scenario delete
capture_call delete POST /api/v1/models/model/delete '{"id":"capture-assistant"}' >/dev/null
capture_call list-after-delete GET /api/v1/models/all >/dev/null

capture_scenario get-missing
capture_call get-deleted GET "/api/v1/models/model?id=capture-assistant" >/dev/null
capture_call update-deleted POST /api/v1/models/model/update "$(model_body)" >/dev/null
capture_call delete-deleted POST /api/v1/models/model/delete '{"id":"capture-assistant"}' >/dev/null
capture_call history-deleted GET "/api/v1/models/model/history?id=capture-assistant" >/dev/null
capture_call export-deleted GET "/api/v1/models/export?ids=capture-assistant" >/dev/null
# The access endpoint on a missing id: does it recreate the model?
capture_call update-access-deleted POST /api/v1/models/model/access/update '{"id":"capture-assistant","access_grants":[]}' >/dev/null
capture_call get-after-update-access-deleted GET "/api/v1/models/model?id=capture-assistant" >/dev/null
