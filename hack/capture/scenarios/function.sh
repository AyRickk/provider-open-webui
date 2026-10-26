# shellcheck shell=bash
# Function capture scenarios. Sourced by capture.sh; see hack/capture/lib.sh for capture_*.
# One function of each type (filter, pipe, action); every write is read back.

filter_code='"""
title: Capture Filter
description: Filter used by the capture scenarios
version: 1.0.0
"""
from pydantic import BaseModel


class Filter:
    class Valves(BaseModel):
        api_key: str = ""
        priority: int = 0

    def __init__(self):
        self.valves = self.Valves()

    def inlet(self, body: dict) -> dict:
        return body
'
pipe_code='"""
title: Capture Pipe
"""


class Pipe:
    def pipe(self, body: dict) -> str:
        return "capture"
'
action_code='"""
title: Capture Action
"""


class Action:
    async def action(self, body: dict) -> None:
        return None
'
toggle_filter_code="${filter_code/class Filter:/class Filter:
    toggle = True}"
no_class_code='def helper():
    return 1
'
changed_pipe_code="${pipe_code/return \"capture\"/return \"changed\"}"
changed_filter_code="${filter_code/priority: int = 0/priority: int = 1}"
renamed_pipe="Capture Pipe Renamed"

# function_body <id> <name> <code> [description] prints a FunctionForm.
function_body() {
  jq -nc --arg id "$1" --arg name "$2" --arg content "$3" --arg description "${4:-Captured function}" \
    '{id: $id, name: $name, content: $content, meta: {description: $description}}'
}

capture_scenario create
capture_call create-filter POST /api/v1/functions/create "$(function_body capture_filter "Capture Filter" "$filter_code")" >/dev/null
capture_call create-pipe POST /api/v1/functions/create "$(function_body capture_pipe "Capture Pipe" "$pipe_code")" >/dev/null
capture_call create-action POST /api/v1/functions/create "$(function_body capture_action "Capture Action" "$action_code")" >/dev/null
capture_call create-toggle-filter POST /api/v1/functions/create "$(function_body capture_toggle_filter "Capture Toggle Filter" "$toggle_filter_code")" >/dev/null
capture_call create-duplicate-id POST /api/v1/functions/create "$(function_body capture_filter "Same id" "$filter_code")" >/dev/null
capture_call create-uppercase-id POST /api/v1/functions/create "$(function_body Capture_Upper "Upper" "$pipe_code")" >/dev/null
capture_call create-invalid-id POST /api/v1/functions/create "$(function_body capture-dash "Dash" "$pipe_code")" >/dev/null
capture_call create-leading-digit-id POST /api/v1/functions/create "$(function_body 1capture "Digit" "$pipe_code")" >/dev/null
capture_call create-no-class POST /api/v1/functions/create "$(function_body capture_no_class "No class" "$no_class_code")" >/dev/null

capture_scenario get
for id in capture_filter capture_pipe capture_action capture_toggle_filter capture_upper; do
  capture_call "get-$id" GET "/api/v1/functions/id/$id" >/dev/null
done

capture_scenario list
capture_call list GET /api/v1/functions/ >/dev/null
capture_call list-with-users GET /api/v1/functions/list >/dev/null
capture_call export GET /api/v1/functions/export >/dev/null
capture_call export-with-valves GET "/api/v1/functions/export?include_valves=true" >/dev/null

capture_scenario update
capture_call update-name POST /api/v1/functions/id/capture_pipe/update "$(function_body capture_pipe "$renamed_pipe" "$pipe_code")" >/dev/null
capture_call get-after-name GET /api/v1/functions/id/capture_pipe >/dev/null
capture_call update-code POST /api/v1/functions/id/capture_pipe/update "$(function_body capture_pipe "$renamed_pipe" "$changed_pipe_code")" >/dev/null
capture_call get-after-code GET /api/v1/functions/id/capture_pipe >/dev/null
capture_call update-description POST /api/v1/functions/id/capture_pipe/update \
  "$(function_body capture_pipe "$renamed_pipe" "$changed_pipe_code" "Changed description")" >/dev/null
capture_call get-after-description GET /api/v1/functions/id/capture_pipe >/dev/null
# New code with another class: the server re-detects the type.
capture_call update-type-pipe-to-action POST /api/v1/functions/id/capture_pipe/update "$(function_body capture_pipe "$renamed_pipe" "$action_code")" >/dev/null
capture_call get-after-type-change GET /api/v1/functions/id/capture_pipe >/dev/null
capture_call update-no-class POST /api/v1/functions/id/capture_pipe/update "$(function_body capture_pipe "$renamed_pipe" "$no_class_code")" >/dev/null
capture_call get-after-no-class GET /api/v1/functions/id/capture_pipe >/dev/null

capture_scenario valves
capture_call valves-initial GET /api/v1/functions/id/capture_filter/valves >/dev/null
capture_call valves-spec GET /api/v1/functions/id/capture_filter/valves/spec >/dev/null
capture_call valves-update POST /api/v1/functions/id/capture_filter/valves/update '{"api_key":"capture-valve-secret","priority":5}' >/dev/null
capture_call valves-after-update GET /api/v1/functions/id/capture_filter/valves >/dev/null
capture_call export-with-valves-after-update GET "/api/v1/functions/export?include_valves=true" >/dev/null
capture_call valves-update-partial POST /api/v1/functions/id/capture_filter/valves/update '{"priority":7}' >/dev/null
capture_call valves-after-partial GET /api/v1/functions/id/capture_filter/valves >/dev/null
capture_call valves-update-unknown-key POST /api/v1/functions/id/capture_filter/valves/update '{"priority":7,"not_a_valve":true}' >/dev/null
capture_call valves-after-unknown-key GET /api/v1/functions/id/capture_filter/valves >/dev/null
capture_call valves-update-wrong-type POST /api/v1/functions/id/capture_filter/valves/update '{"priority":"high"}' >/dev/null
capture_call valves-after-wrong-type GET /api/v1/functions/id/capture_filter/valves >/dev/null
capture_call valves-function-without-valves GET /api/v1/functions/id/capture_action/valves >/dev/null
capture_call valves-spec-function-without-valves GET /api/v1/functions/id/capture_action/valves/spec >/dev/null
capture_call valves-update-function-without-valves POST /api/v1/functions/id/capture_action/valves/update '{"x":1}' >/dev/null
capture_call get-after-valves GET /api/v1/functions/id/capture_filter >/dev/null

# Each toggle twice, with the state read before and after every call.
capture_scenario toggles
capture_call state-initial GET /api/v1/functions/id/capture_filter >/dev/null
for round in 1 2; do
  capture_call "toggle-active-$round" POST /api/v1/functions/id/capture_filter/toggle >/dev/null
  capture_call "state-after-toggle-active-$round" GET /api/v1/functions/id/capture_filter >/dev/null
done
for round in 1 2; do
  capture_call "toggle-global-$round" POST /api/v1/functions/id/capture_filter/toggle/global >/dev/null
  capture_call "state-after-toggle-global-$round" GET /api/v1/functions/id/capture_filter >/dev/null
done
# Does an update that changes the code and description carry or reset the toggles?
capture_call toggle-active-on POST /api/v1/functions/id/capture_filter/toggle >/dev/null
capture_call state-after-toggle-active-on GET /api/v1/functions/id/capture_filter >/dev/null
capture_call toggle-global-on POST /api/v1/functions/id/capture_filter/toggle/global >/dev/null
capture_call state-after-toggle-global-on GET /api/v1/functions/id/capture_filter >/dev/null
capture_call update-while-on POST /api/v1/functions/id/capture_filter/update \
  "$(function_body capture_filter "Capture Filter" "$changed_filter_code" "Changed while on")" >/dev/null
capture_call state-after-update-while-on GET /api/v1/functions/id/capture_filter >/dev/null

# Version history: which writes add a version on 0.12.4, on a fresh function. On 0.11.4 the
# history routes do not exist, and the captured HTML fallback is the evidence of that.
capture_scenario history
capture_call create POST /api/v1/functions/create "$(function_body capture_history "Capture History" "$pipe_code")" >/dev/null
capture_call history-after-create GET /api/v1/functions/id/capture_history/history >/dev/null
capture_call update-no-change POST /api/v1/functions/id/capture_history/update "$(function_body capture_history "Capture History" "$pipe_code")" >/dev/null
capture_call history-after-no-change GET /api/v1/functions/id/capture_history/history >/dev/null
capture_call update-change POST /api/v1/functions/id/capture_history/update "$(function_body capture_history "Capture History v2" "$pipe_code")" >/dev/null
capture_call history-after-change GET /api/v1/functions/id/capture_history/history >/dev/null
capture_call toggle-active POST /api/v1/functions/id/capture_history/toggle >/dev/null
capture_call history-after-toggle GET /api/v1/functions/id/capture_history/history >/dev/null

capture_scenario delete
capture_call delete DELETE /api/v1/functions/id/capture_action/delete >/dev/null
capture_call list-after-delete GET /api/v1/functions/ >/dev/null

capture_scenario get-missing
capture_call get-deleted GET /api/v1/functions/id/capture_action >/dev/null
capture_call update-deleted POST /api/v1/functions/id/capture_action/update "$(function_body capture_action "Capture Action" "$action_code")" >/dev/null
capture_call delete-deleted DELETE /api/v1/functions/id/capture_action/delete >/dev/null
capture_call valves-deleted GET /api/v1/functions/id/capture_action/valves >/dev/null
capture_call valves-update-deleted POST /api/v1/functions/id/capture_action/valves/update '{"x":1}' >/dev/null
capture_call toggle-deleted POST /api/v1/functions/id/capture_action/toggle >/dev/null
capture_call toggle-global-deleted POST /api/v1/functions/id/capture_action/toggle/global >/dev/null
