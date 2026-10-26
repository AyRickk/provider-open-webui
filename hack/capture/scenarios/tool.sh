# shellcheck shell=bash
# Tool capture scenarios. Sourced by capture.sh; see hack/capture/lib.sh for capture_*.
# Writes are read back wherever what the server stores matters.

tool_code='"""
title: Capture Tool
description: Toolkit used by the capture scenarios
version: 1.0.0
"""
from pydantic import BaseModel, Field


class Tools:
    class Valves(BaseModel):
        api_key: str = ""
        priority: int = Field(default=0, description="Order among tools")

    class UserValves(BaseModel):
        nickname: str = ""

    def __init__(self):
        self.valves = self.Valves()

    def get_capture_value(self, name: str) -> str:
        """
        Return a fixed value for a name.
        :param name: The name to look up.
        """
        return f"capture-{name}"
'
changed_tool_code="${tool_code/return f\"capture-\{name\}\"/return f\"changed-\{name\}\"}"
syntax_error_code='class Tools:
    def broken(self) -> str
        return "missing colon"
'
no_class_code='def helper():
    return 1
'
# Code variants to see what the server changes before storing.
crlf_code="${tool_code//$'\n'/$'\r\n'}"
no_trailing_newline_code="${tool_code%$'\n'}"
trailing_spaces_code="${tool_code/version: 1.0.0/version: 1.0.0   }"
# replace_imports rewrites "from utils" anywhere in the text, comments included.
import_rewrite_code="${tool_code/from pydantic import/# from utils import nothing
from pydantic import}"

# tool_body <id> <name> <code> [description] [jq-filter] prints a ToolForm.
tool_body() {
  jq -nc --arg id "$1" --arg name "$2" --arg content "$3" --arg description "${4:-Captured tool}" \
    "{id: \$id, name: \$name, content: \$content, meta: {description: \$description}} | ${5:-.}"
}

capture_scenario setup
readers_group_id="$(capture_call setup-group-readers POST /api/v1/groups/create '{"name":"capture-readers","description":"Readers"}' | jq -er .id)"

capture_scenario create
capture_call create POST /api/v1/tools/create "$(tool_body capture_tool "Capture Tool" "$tool_code")" >/dev/null
capture_call create-with-access-grants POST /api/v1/tools/create "$(tool_body capture_shared "Capture Shared" "$tool_code" "Shared tool" \
  ".access_grants = [{principal_type: \"group\", principal_id: \"$readers_group_id\", permission: \"read\"}, {principal_type: \"user\", principal_id: \"*\", permission: \"read\"}]")" >/dev/null
capture_call create-syntax-error POST /api/v1/tools/create "$(tool_body capture_broken "Broken" "$syntax_error_code")" >/dev/null
capture_call create-no-class POST /api/v1/tools/create "$(tool_body capture_no_class "No class" "$no_class_code")" >/dev/null
capture_call create-duplicate-id POST /api/v1/tools/create "$(tool_body capture_tool "Same id" "$tool_code")" >/dev/null
capture_call create-uppercase-id POST /api/v1/tools/create "$(tool_body Capture_Upper "Upper" "$tool_code")" >/dev/null
capture_call create-invalid-id POST /api/v1/tools/create "$(tool_body capture-dash "Dash" "$tool_code")" >/dev/null
capture_call create-leading-digit-id POST /api/v1/tools/create "$(tool_body 1capture "Digit" "$tool_code")" >/dev/null

capture_scenario get
capture_call get GET /api/v1/tools/id/capture_tool >/dev/null
capture_call get-shared GET /api/v1/tools/id/capture_shared >/dev/null
capture_call get-uppercase GET /api/v1/tools/id/capture_upper >/dev/null

capture_scenario list
capture_call list GET /api/v1/tools/ >/dev/null
capture_call list-with-users GET /api/v1/tools/list >/dev/null
capture_call export GET /api/v1/tools/export >/dev/null

# Is the code stored as sent?
capture_scenario code-normalization
# code_variant <variant> <code> creates a tool from the variant and reads it back.
code_variant() {
  capture_call "create-$1" POST /api/v1/tools/create "$(tool_body "capture_${1//-/_}" "Capture $1" "$2")" >/dev/null
  capture_call "get-$1" GET "/api/v1/tools/id/capture_${1//-/_}" >/dev/null
}
code_variant crlf "$crlf_code"
code_variant no-trailing-newline "$no_trailing_newline_code"
code_variant trailing-spaces "$trailing_spaces_code"
code_variant import-rewrite "$import_rewrite_code"
# The same rewrite on update, the path drift correction uses.
capture_call update-import-rewrite POST /api/v1/tools/id/capture_no_trailing_newline/update \
  "$(tool_body capture_no_trailing_newline "Capture no-trailing-newline" "$import_rewrite_code")" >/dev/null
capture_call get-after-update-import-rewrite GET /api/v1/tools/id/capture_no_trailing_newline >/dev/null

capture_scenario update
capture_call update-name POST /api/v1/tools/id/capture_tool/update "$(tool_body capture_tool "Capture Tool Renamed" "$tool_code")" >/dev/null
capture_call get-after-name GET /api/v1/tools/id/capture_tool >/dev/null
capture_call update-description POST /api/v1/tools/id/capture_tool/update "$(tool_body capture_tool "Capture Tool Renamed" "$tool_code" "Changed description")" >/dev/null
capture_call get-after-description GET /api/v1/tools/id/capture_tool >/dev/null
capture_call update-code POST /api/v1/tools/id/capture_tool/update "$(tool_body capture_tool "Capture Tool Renamed" "$changed_tool_code" "Changed description")" >/dev/null
capture_call get-after-code GET /api/v1/tools/id/capture_tool >/dev/null
capture_call update-syntax-error POST /api/v1/tools/id/capture_tool/update "$(tool_body capture_tool "Capture Tool Renamed" "$syntax_error_code")" >/dev/null
capture_call get-after-syntax-error GET /api/v1/tools/id/capture_tool >/dev/null

capture_scenario valves
capture_call valves-initial GET /api/v1/tools/id/capture_tool/valves >/dev/null
capture_call valves-spec GET /api/v1/tools/id/capture_tool/valves/spec >/dev/null
capture_call user-valves-spec GET /api/v1/tools/id/capture_tool/valves/user/spec >/dev/null
capture_call valves-update POST /api/v1/tools/id/capture_tool/valves/update '{"api_key":"capture-valve-secret","priority":5}' >/dev/null
read_back="$(capture_call valves-after-update GET /api/v1/tools/id/capture_tool/valves | jq -r .api_key)"
# Fixtures redact api_key, so record whether the server returned the secret itself or a mask.
capture_check valves-read-back-in-clear "GET valves returns the api_key exactly as written" \
  "$([[ "$read_back" == capture-valve-secret ]] && echo true || echo false)"
capture_call export-after-valves-update GET /api/v1/tools/export >/dev/null
capture_call export-include-valves-after-valves-update GET "/api/v1/tools/export?include_valves=true" >/dev/null
capture_call valves-update-partial POST /api/v1/tools/id/capture_tool/valves/update '{"priority":7}' >/dev/null
capture_call valves-after-partial GET /api/v1/tools/id/capture_tool/valves >/dev/null
capture_call valves-update-unknown-key POST /api/v1/tools/id/capture_tool/valves/update '{"priority":7,"not_a_valve":true}' >/dev/null
capture_call valves-after-unknown-key GET /api/v1/tools/id/capture_tool/valves >/dev/null
capture_call valves-update-wrong-type POST /api/v1/tools/id/capture_tool/valves/update '{"priority":"high"}' >/dev/null
capture_call valves-after-wrong-type GET /api/v1/tools/id/capture_tool/valves >/dev/null
capture_call get-after-valves GET /api/v1/tools/id/capture_tool >/dev/null

capture_scenario access-update
capture_call update-access POST /api/v1/tools/id/capture_shared/access/update \
  "{\"access_grants\":[{\"principal_type\":\"group\",\"principal_id\":\"$readers_group_id\",\"permission\":\"write\"}]}" >/dev/null
capture_call get-after-update-access GET /api/v1/tools/id/capture_shared >/dev/null
capture_call update-with-grants-in-body POST /api/v1/tools/id/capture_shared/update "$(tool_body capture_shared "Capture Shared" "$tool_code" "Shared tool" \
  '.access_grants = [{principal_type: "user", principal_id: "*", permission: "read"}]')" >/dev/null
capture_call get-after-update-with-grants GET /api/v1/tools/id/capture_shared >/dev/null
capture_call make-private POST /api/v1/tools/id/capture_shared/access/update '{"access_grants":[]}' >/dev/null
capture_call get-after-make-private GET /api/v1/tools/id/capture_shared >/dev/null

# Version history: which writes add a version on 0.12.4, on a fresh tool. On 0.11.4 the history
# routes do not exist, and the captured HTML fallback is the evidence of that.
capture_scenario history
capture_call create POST /api/v1/tools/create "$(tool_body capture_history "Capture History" "$tool_code")" >/dev/null
capture_call history-after-create GET /api/v1/tools/id/capture_history/history >/dev/null
capture_call update-no-change POST /api/v1/tools/id/capture_history/update "$(tool_body capture_history "Capture History" "$tool_code")" >/dev/null
capture_call history-after-no-change GET /api/v1/tools/id/capture_history/history >/dev/null
capture_call update-change POST /api/v1/tools/id/capture_history/update "$(tool_body capture_history "Capture History v2" "$tool_code")" >/dev/null
capture_call history-after-change GET /api/v1/tools/id/capture_history/history >/dev/null
capture_call valves-update POST /api/v1/tools/id/capture_history/valves/update '{"priority":3}' >/dev/null
capture_call history-after-valves GET /api/v1/tools/id/capture_history/history >/dev/null
capture_call update-access POST /api/v1/tools/id/capture_history/access/update \
  "{\"access_grants\":[{\"principal_type\":\"group\",\"principal_id\":\"$readers_group_id\",\"permission\":\"read\"}]}" >/dev/null
capture_call history-after-access GET /api/v1/tools/id/capture_history/history >/dev/null

capture_scenario delete
capture_call delete DELETE /api/v1/tools/id/capture_upper/delete >/dev/null
capture_call list-after-delete GET /api/v1/tools/ >/dev/null

capture_scenario get-missing
capture_call get-deleted GET /api/v1/tools/id/capture_upper >/dev/null
capture_call update-deleted POST /api/v1/tools/id/capture_upper/update "$(tool_body capture_upper "Upper" "$tool_code")" >/dev/null
capture_call delete-deleted DELETE /api/v1/tools/id/capture_upper/delete >/dev/null
capture_call valves-deleted GET /api/v1/tools/id/capture_upper/valves >/dev/null
capture_call valves-update-deleted POST /api/v1/tools/id/capture_upper/valves/update '{"priority":1}' >/dev/null
capture_call update-access-deleted POST /api/v1/tools/id/capture_upper/access/update '{"access_grants":[]}' >/dev/null
capture_call get-after-update-access-deleted GET /api/v1/tools/id/capture_upper >/dev/null
