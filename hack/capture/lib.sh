# shellcheck shell=bash
# Recording helpers for capture scenarios. Sourced by capture.sh, which sets CAPTURE_URL,
# CAPTURE_KEY, CAPTURE_RAW, CAPTURE_VERSION and CAPTURE_DIR (hack/capture) before sourcing a
# scenario file.

# capture_scenario starts a new scenario; every following capture_call belongs to it.
capture_scenario() {
  CAPTURE_SCENARIO="$1"
}

# capture_call <step> <METHOD> <path> [json-body] sends one authenticated request, records the
# exchange in CAPTURE_RAW and prints the response body so scenarios can read ids from it.
# Any HTTP status is recorded as data; only a transport failure (no HTTP response) fails.
capture_call() {
  local step="$1" method="$2" path="$3" body="${4:-}"
  local resp_file meta status content_type
  resp_file="$(mktemp)"
  local args=(-sS -X "$method" -o "$resp_file" -w '%{http_code} %{content_type}' -H "Authorization: Bearer $CAPTURE_KEY")
  if [[ -n "$body" ]]; then
    args+=(-H 'Content-Type: application/json' -d "$body")
  fi
  if ! meta="$(curl "${args[@]}" "$CAPTURE_URL$path")"; then
    echo "capture: $CAPTURE_SCENARIO/$step: $method $path got no HTTP response" >&2
    rm -f "$resp_file"
    return 1
  fi
  read -r status content_type <<<"$meta"

  # Non-JSON bodies (the web UI's HTML page, served for unknown paths) are recorded as a marker.
  jq -nc \
    --arg scenario "$CAPTURE_SCENARIO" --arg step "$step" \
    --arg method "$method" --arg path "$path" --arg body "$body" \
    --argjson status "$status" --arg content_type "$content_type" --rawfile resp "$resp_file" \
    '{scenario: $scenario, step: $step,
      request: {method: $method, path: $path, body: (if $body == "" then null else ($body | fromjson) end)},
      response: {status: $status, content_type: $content_type,
                 body: ($resp | try fromjson catch "<non-JSON body>")}}' >>"$CAPTURE_RAW"

  cat "$resp_file"
  rm -f "$resp_file"
}

# capture_check <step> <claim> <true|false> records a fact the scenario verified at capture time
# but cannot show in a fixture without leaking it, e.g. that a secret reads back unmasked.
capture_check() {
  jq -nc --arg scenario "$CAPTURE_SCENARIO" --arg step "$1" --arg claim "$2" --argjson holds "$3" \
    '{scenario: $scenario, step: $step, check: {claim: $claim, holds: $holds}}' >>"$CAPTURE_RAW"
}

# capture_observe <step> <what> <json> records something observed outside the API (database rows,
# files on disk), normalized and redacted like any response.
capture_observe() {
  jq -nc --arg scenario "$CAPTURE_SCENARIO" --arg step "$1" --arg what "$2" --argjson value "$3" \
    '{scenario: $scenario, step: $step, observation: {what: $what, value: $value}}' >>"$CAPTURE_RAW"
}

# capture_upload <step> <path> uploads a file as multipart form data, records the exchange (the
# request as the file's name and size, not its bytes) and prints the response body.
capture_upload() {
  local step="$1" path="$2" resp_file meta status content_type
  resp_file="$(mktemp)"
  if ! meta="$(curl -sS -X POST -o "$resp_file" -w '%{http_code} %{content_type}' \
    -H "Authorization: Bearer $CAPTURE_KEY" -F "file=@$path" "$CAPTURE_URL/api/v1/files/")"; then
    echo "capture: $CAPTURE_SCENARIO/$step: upload got no HTTP response" >&2
    rm -f "$resp_file"
    return 1
  fi
  read -r status content_type <<<"$meta"
  jq -nc --arg scenario "$CAPTURE_SCENARIO" --arg step "$step" --arg name "$(basename "$path")" \
    --argjson size "$(wc -c <"$path")" --argjson status "$status" --arg content_type "$content_type" \
    --rawfile resp "$resp_file" \
    '{scenario: $scenario, step: $step,
      request: {method: "POST", path: "/api/v1/files/", body: {multipart_file: $name, size: $size}},
      response: {status: $status, content_type: $content_type, body: ($resp | try fromjson catch "<non-JSON body>")}}' >>"$CAPTURE_RAW"
  cat "$resp_file"
  rm -f "$resp_file"
}
