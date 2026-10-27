# shellcheck shell=bash
# Every operation the provider performs, with the API key restricted to
# docs/api-notes/api-key-allowed-endpoints.txt (endpoint-restrictions.before.sh), then routes the
# provider never needs. Sourced by capture.sh; see hack/capture/lib.sh for capture_*.

tool_code='from pydantic import BaseModel


class Tools:
    class Valves(BaseModel):
        priority: int = 0

    def ping(self) -> str:
        """Answer pong."""
        return "pong"
'
filter_code='from pydantic import BaseModel


class Filter:
    class Valves(BaseModel):
        priority: int = 0

    def inlet(self, body: dict) -> dict:
        return body
'

capture_scenario cross
capture_call version GET /api/version >/dev/null
capture_call default-permissions GET /api/v1/users/default/permissions >/dev/null

capture_scenario group
group="$(capture_call create POST /api/v1/groups/create '{"name":"capture-restricted","description":"d"}' | jq -er .id)"
capture_call list GET /api/v1/groups/ >/dev/null
capture_call get GET "/api/v1/groups/id/$group" >/dev/null
capture_call update POST "/api/v1/groups/id/$group/update" '{"name":"capture-restricted","description":"changed"}' >/dev/null

capture_scenario model
capture_call base-models GET /api/models >/dev/null
capture_call create POST /api/v1/models/create '{"id":"capture-restricted","name":"R","meta":{},"params":{}}' >/dev/null
capture_call get GET "/api/v1/models/model?id=capture-restricted" >/dev/null
capture_call update POST /api/v1/models/model/update '{"id":"capture-restricted","name":"R2","meta":{},"params":{}}' >/dev/null
capture_call update-access POST /api/v1/models/model/access/update \
  "{\"id\":\"capture-restricted\",\"access_grants\":[{\"principal_type\":\"group\",\"principal_id\":\"$group\",\"permission\":\"read\"}]}" >/dev/null
capture_call delete POST /api/v1/models/model/delete '{"id":"capture-restricted"}' >/dev/null

capture_scenario prompt
prompt="$(capture_call create POST /api/v1/prompts/create '{"command":"capture-restricted","name":"R","content":"c"}' | jq -er .id)"
capture_call find GET "/api/v1/prompts/list?query=capture-restricted" >/dev/null
capture_call get GET "/api/v1/prompts/id/$prompt" >/dev/null
capture_call update POST "/api/v1/prompts/id/$prompt/update" '{"command":"capture-restricted","name":"R2","content":"c"}' >/dev/null
capture_call toggle POST "/api/v1/prompts/id/$prompt/toggle" >/dev/null
capture_call update-access POST "/api/v1/prompts/id/$prompt/access/update" '{"access_grants":[]}' >/dev/null
capture_call delete DELETE "/api/v1/prompts/id/$prompt/delete" >/dev/null

capture_scenario tool
capture_call create POST /api/v1/tools/create "$(jq -nc --arg c "$tool_code" '{id: "capture_restricted", name: "R", content: $c, meta: {description: "d"}}')" >/dev/null
capture_call get GET /api/v1/tools/id/capture_restricted >/dev/null
capture_call update POST /api/v1/tools/id/capture_restricted/update "$(jq -nc --arg c "$tool_code" '{id: "capture_restricted", name: "R2", content: $c, meta: {description: "d"}}')" >/dev/null
capture_call valves GET /api/v1/tools/id/capture_restricted/valves >/dev/null
capture_call valves-spec GET /api/v1/tools/id/capture_restricted/valves/spec >/dev/null
capture_call valves-update POST /api/v1/tools/id/capture_restricted/valves/update '{"priority":1}' >/dev/null
capture_call update-access POST /api/v1/tools/id/capture_restricted/access/update '{"access_grants":[]}' >/dev/null
capture_call delete DELETE /api/v1/tools/id/capture_restricted/delete >/dev/null

capture_scenario function
capture_call create POST /api/v1/functions/create "$(jq -nc --arg c "$filter_code" '{id: "capture_restricted", name: "R", content: $c, meta: {description: "d"}}')" >/dev/null
capture_call get GET /api/v1/functions/id/capture_restricted >/dev/null
capture_call update POST /api/v1/functions/id/capture_restricted/update "$(jq -nc --arg c "$filter_code" '{id: "capture_restricted", name: "R2", content: $c, meta: {description: "d"}}')" >/dev/null
capture_call toggle POST /api/v1/functions/id/capture_restricted/toggle >/dev/null
capture_call toggle-global POST /api/v1/functions/id/capture_restricted/toggle/global >/dev/null
capture_call valves GET /api/v1/functions/id/capture_restricted/valves >/dev/null
capture_call valves-spec GET /api/v1/functions/id/capture_restricted/valves/spec >/dev/null
capture_call valves-update POST /api/v1/functions/id/capture_restricted/valves/update '{"priority":1}' >/dev/null
capture_call delete DELETE /api/v1/functions/id/capture_restricted/delete >/dev/null

capture_scenario knowledge
kb="$(capture_call create POST /api/v1/knowledge/create '{"name":"capture-restricted","description":"d"}' | jq -er .id)"
capture_call find GET "/api/v1/knowledge/search?query=capture-restricted" >/dev/null
capture_call get GET "/api/v1/knowledge/$kb" >/dev/null
capture_call update POST "/api/v1/knowledge/$kb/update" '{"name":"capture-restricted","description":"changed"}' >/dev/null
capture_call update-access POST "/api/v1/knowledge/$kb/access/update" '{"access_grants":[]}' >/dev/null
capture_call delete DELETE "/api/v1/knowledge/$kb/delete" >/dev/null

capture_scenario group-delete
capture_call delete DELETE "/api/v1/groups/id/$group/delete" >/dev/null

# Routes the provider never needs: refused (403), except where an allowed prefix covers them.
# endpoint-restrictions-control.sh calls the read-only ones without restrictions, to show the
# 403 comes from the restriction (its detail is the generic "access prohibited" message).
capture_scenario outside-the-list
capture_call session-user GET /api/v1/auths/ >/dev/null
capture_call list-users GET /api/v1/users/ >/dev/null
capture_call models-sync POST /api/v1/models/sync '{"models":[]}' >/dev/null
capture_call models-delete-all DELETE /api/v1/models/delete/all >/dev/null
capture_call model-list-all GET /api/v1/models/all >/dev/null
capture_call openai-config GET /openai/config >/dev/null
capture_call files GET /api/v1/files/ >/dev/null
# Covered by an allowed prefix although the provider never calls them.
capture_call groups-users-add-covered POST /api/v1/groups/id/missing-group-id/users/add '{"user_ids":[]}' >/dev/null
capture_call knowledge-reindex-covered POST /api/v1/knowledge/reindex >/dev/null
