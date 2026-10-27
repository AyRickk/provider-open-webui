# shellcheck shell=bash
# Where a "managed by provider-openwebui" marker could be stored: for each resource, a marker is
# sent in each candidate field on create and on update, and read back. Documented only; the
# provider does not depend on it. Sourced by capture.sh; see hack/capture/lib.sh for capture_*.

marker='{"managed_by":"provider-openwebui"}'
tool_code='class Tools:
    def ping(self) -> str:
        """Answer pong."""
        return "pong"
'
filter_code='class Filter:
    def inlet(self, body: dict) -> dict:
        return body
'

capture_scenario group
group="$(capture_call create-data POST /api/v1/groups/create "{\"name\":\"capture-marker\",\"description\":\"d\",\"data\":$marker}" | jq -er .id)"
capture_call get-after-create GET "/api/v1/groups/id/$group" >/dev/null
capture_call update-without-data POST "/api/v1/groups/id/$group/update" '{"name":"capture-marker","description":"changed"}' >/dev/null
capture_call get-after-update-without-data GET "/api/v1/groups/id/$group" >/dev/null
capture_call create-meta POST /api/v1/groups/create "{\"name\":\"capture-marker-meta\",\"description\":\"d\",\"meta\":$marker}" >/dev/null

capture_scenario model
capture_call create-meta POST /api/v1/models/create "{\"id\":\"capture-marker\",\"name\":\"M\",\"meta\":$marker,\"params\":{}}" >/dev/null
capture_call get-after-create GET "/api/v1/models/model?id=capture-marker" >/dev/null
capture_call update-without-marker POST /api/v1/models/model/update '{"id":"capture-marker","name":"M2","meta":{},"params":{}}' >/dev/null
capture_call get-after-update-without-marker GET "/api/v1/models/model?id=capture-marker" >/dev/null

capture_scenario prompt
prompt="$(capture_call create-meta-and-data POST /api/v1/prompts/create \
  "{\"command\":\"capture-marker\",\"name\":\"P\",\"content\":\"c\",\"meta\":$marker,\"data\":$marker}" | jq -er .id)"
capture_call get-after-create GET "/api/v1/prompts/id/$prompt" >/dev/null
capture_call update-without-meta POST "/api/v1/prompts/id/$prompt/update" '{"command":"capture-marker","name":"P2","content":"c"}' >/dev/null
capture_call get-after-update-without-meta GET "/api/v1/prompts/id/$prompt" >/dev/null

capture_scenario tool
capture_call create-meta POST /api/v1/tools/create \
  "$(jq -nc --arg c "$tool_code" --argjson m "$marker" '{id: "capture_marker", name: "T", content: $c, meta: ({description: "d"} + $m)}')" >/dev/null
capture_call get-after-create GET /api/v1/tools/id/capture_marker >/dev/null
capture_call update-without-marker POST /api/v1/tools/id/capture_marker/update \
  "$(jq -nc --arg c "$tool_code" '{id: "capture_marker", name: "T2", content: $c, meta: {description: "d"}}')" >/dev/null
capture_call get-after-update-without-marker GET /api/v1/tools/id/capture_marker >/dev/null

capture_scenario function
capture_call create-meta POST /api/v1/functions/create \
  "$(jq -nc --arg c "$filter_code" --argjson m "$marker" '{id: "capture_marker", name: "F", content: $c, meta: ({description: "d"} + $m)}')" >/dev/null
capture_call get-after-create GET /api/v1/functions/id/capture_marker >/dev/null
capture_call update-without-marker POST /api/v1/functions/id/capture_marker/update \
  "$(jq -nc --arg c "$filter_code" '{id: "capture_marker", name: "F2", content: $c, meta: {description: "d"}}')" >/dev/null
capture_call get-after-update-without-marker GET /api/v1/functions/id/capture_marker >/dev/null

capture_scenario knowledge
kb="$(capture_call create-meta POST /api/v1/knowledge/create "{\"name\":\"capture-marker\",\"description\":\"d\",\"meta\":$marker}" | jq -er .id)"
capture_call get-after-create GET "/api/v1/knowledge/$kb" >/dev/null
