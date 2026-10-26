# Turns the raw exchanges of one capture run (a slurped array) into stable fixtures:
#   - secrets are redacted: string values, or arrays of strings, under keys ending in token(s),
#     secret(s), password(s) or api_key(s) (secrets.jq), and sk- keys
#     or JWTs anywhere inside a string;
#   - server-computed timestamps (numbers under *_at or created) become 1700000000, keeping
#     their type;
#   - UUIDs become 00000000-0000-4000-8000-<ordinal>, numbered by first appearance in the run,
#     so references between exchanges and scenarios survive;
#   - arrays of objects with an id are sorted by that numbered id (creation order): servers sort
#     lists by updated_at, whose second-granularity ties make the order vary between runs;
# and groups them per scenario, in the order the scenarios ran.
# Arguments: -L hack/capture --arg version <x.y.z> --arg resource <name>.

include "secrets";

def uuid_re: "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";

def redact:
  walk(
    if type == "object" then
      with_entries(
        if (.key | test(secret_key_re; "i")) and (.value | type == "string") then .value = "<redacted>"
        elif (.key | test(secret_key_re; "i")) and (.value | type == "array") then .value |= map(if type == "string" then "<redacted>" else . end)
        elif (.key | test("_at$|^created$")) and (.value | type == "number") then .value = 1700000000
        else . end)
    elif type == "string" then gsub(secret_re; "<redacted>")
    else . end);

def number_uuids:
  tojson as $s
  | ([$s | match(uuid_re; "g") | .string] | reduce .[] as $u ([]; if IN(.[]; $u) then . else . + [$u] end)) as $ids
  | reduce range(0; $ids | length) as $i ($s;
      gsub($ids[$i]; "00000000-0000-4000-8000-" + ("000000000000" + ($i + 1 | tostring))[-12:]))
  | fromjson;

def sort_lists:
  walk(if type == "array" and length > 0 and all(.[]; type == "object" and has("id")) then sort_by(.id) else . end);

redact
| number_uuids
| map(.response.body |= sort_lists)
| reduce .[] as $x ({}; .[$x.scenario] += [$x | del(.scenario)])
| with_entries(.value = {openwebui_version: $version, resource: $resource, scenario: .key, steps: .value})
