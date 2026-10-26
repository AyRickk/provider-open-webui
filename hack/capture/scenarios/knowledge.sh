# shellcheck shell=bash
# Knowledge capture scenarios: the container and its access, plus, for the spike only, what a
# deletion does to files and vectors. Sourced by capture.sh after scenarios/knowledge.before.sh;
# see hack/capture/lib.sh for capture_*.
# shellcheck disable=SC2154 # instance and pgvector come from knowledge.before.sh, work from capture.sh

# Observations are lists of objects keyed by `id` so that normalize.jq sorts them like lists.
# observe_vectors <step> records the pgvector collections as [{id: <collection>, chunks: <n>}].
observe_vectors() {
  local rows
  rows="$(docker exec "$pgvector" psql -U postgres -d vectors -At -F $'\t' \
    -c 'select collection_name, count(*) from document_chunk group by 1' \
    | jq -Rn '[inputs | select(length > 0) | split("\t") | {id: .[0], chunks: (.[1] | tonumber)}]')"
  capture_observe "$1" "pgvector document_chunk rows per collection" "$rows"
}

# observe_uploads <step> records the files in the instance's upload directory as [{id: <name>}].
observe_uploads() {
  local files
  files="$(docker exec "$instance" ls /app/backend/data/uploads | jq -Rn '[inputs | select(length > 0) | {id: .}]')"
  capture_observe "$1" "files in data/uploads" "$files"
}

# wait_processed <file id> waits until ingestion of a file finished, without recording the polls.
wait_processed() {
  local status=""
  for _ in $(seq 1 60); do
    status="$(curl -fsS -H "Authorization: Bearer $CAPTURE_KEY" "$CAPTURE_URL/api/v1/files/$1" | jq -r '.data.status // ""')"
    [[ "$status" == completed ]] && return 0
    if [[ "$status" == failed ]]; then
      echo "capture: ingestion of file $1 failed; is the embedding upstream up?" >&2
      return 1
    fi
    sleep 1
  done
  echo "capture: file $1 still '$status' after 60s" >&2
  return 1
}

capture_scenario setup
readers_group_id="$(capture_call setup-group-readers POST /api/v1/groups/create '{"name":"capture-readers","description":"Readers"}' | jq -er .id)"

capture_scenario create
kb="$(capture_call create POST /api/v1/knowledge/create '{"name":"capture-kb","description":"Knowledge base defined as code"}' | jq -er .id)"
shared="$(capture_call create-with-access-grants POST /api/v1/knowledge/create \
  "{\"name\":\"capture-shared\",\"description\":\"Shared\",\"access_grants\":[{\"principal_type\":\"group\",\"principal_id\":\"$readers_group_id\",\"permission\":\"read\"}]}" | jq -er .id)"
capture_call create-duplicate-name POST /api/v1/knowledge/create '{"name":"capture-kb","description":"Same name"}' >/dev/null
capture_call create-empty-name POST /api/v1/knowledge/create '{"name":"","description":"No name"}' >/dev/null

capture_scenario get
capture_call get GET "/api/v1/knowledge/$kb" >/dev/null
capture_call get-shared GET "/api/v1/knowledge/$shared" >/dev/null

# 31 more bases, so that the 30-item page boundary shows. Lists are sorted by updated_at (seconds),
# so each base gets its own second: otherwise ties make page membership vary between runs.
capture_scenario list
for n in $(seq -w 1 31); do
  sleep 1
  capture_call "setup-create-page-$n" POST /api/v1/knowledge/create "{\"name\":\"capture-page-$n\",\"description\":\"Paging\"}" >/dev/null
done
capture_call list-page-1 GET "/api/v1/knowledge/?page=1" >/dev/null
capture_call list-page-2 GET "/api/v1/knowledge/?page=2" >/dev/null
capture_call list-page-3 GET "/api/v1/knowledge/?page=3" >/dev/null

# No lookup by name: the search endpoint filters on a query string.
capture_scenario find-by-name
capture_call search-exact-name GET "/api/v1/knowledge/search?query=capture-kb" >/dev/null
capture_call search-prefix GET "/api/v1/knowledge/search?query=capture-page-0" >/dev/null

capture_scenario update
capture_call update-name POST "/api/v1/knowledge/$kb/update" '{"name":"capture-kb-renamed","description":"Knowledge base defined as code"}' >/dev/null
capture_call get-after-name GET "/api/v1/knowledge/$kb" >/dev/null
capture_call update-description POST "/api/v1/knowledge/$kb/update" '{"name":"capture-kb-renamed","description":"Changed description"}' >/dev/null
capture_call get-after-description GET "/api/v1/knowledge/$kb" >/dev/null
capture_call update-name-only POST "/api/v1/knowledge/$kb/update" '{"name":"capture-kb-renamed"}' >/dev/null
capture_call get-after-name-only GET "/api/v1/knowledge/$kb" >/dev/null

capture_scenario access-update
capture_call update-access POST "/api/v1/knowledge/$shared/access/update" \
  "{\"access_grants\":[{\"principal_type\":\"group\",\"principal_id\":\"$readers_group_id\",\"permission\":\"write\"}]}" >/dev/null
capture_call get-after-update-access GET "/api/v1/knowledge/$shared" >/dev/null
capture_call update-with-grants-in-body POST "/api/v1/knowledge/$shared/update" \
  '{"name":"capture-shared","description":"Shared","access_grants":[{"principal_type":"user","principal_id":"*","permission":"read"}]}' >/dev/null
capture_call get-after-update-with-grants GET "/api/v1/knowledge/$shared" >/dev/null
capture_call make-private POST "/api/v1/knowledge/$shared/access/update" '{"access_grants":[]}' >/dev/null
capture_call get-after-make-private GET "/api/v1/knowledge/$shared" >/dev/null

# Spike only: a file in a knowledge base that a model references, then the base is deleted.
capture_scenario files-and-deletion
doc="$work/capture-doc.txt"
printf 'Capture knowledge document.\nIt has two lines.\n' >"$doc"
file="$(capture_upload upload "$doc" | jq -er .id)"
wait_processed "$file"
capture_call file-after-processing GET "/api/v1/files/$file" >/dev/null
docs="$(capture_call create-kb POST /api/v1/knowledge/create '{"name":"capture-docs","description":"With a file"}' | jq -er .id)"
capture_call add-file POST "/api/v1/knowledge/$docs/file/add" "{\"file_id\":\"$file\"}" >/dev/null
capture_call kb-files GET "/api/v1/knowledge/$docs/files" >/dev/null
capture_call create-model-using-kb POST /api/v1/models/create \
  "{\"id\":\"capture-kb-model\",\"name\":\"Uses the knowledge base\",\"meta\":{\"knowledge\":[{\"id\":\"$docs\",\"name\":\"capture-docs\",\"type\":\"collection\"}]},\"params\":{}}" >/dev/null
observe_vectors vectors-before-delete
observe_uploads uploads-before-delete
capture_call delete-kb DELETE "/api/v1/knowledge/$docs/delete" >/dev/null
observe_vectors vectors-after-delete
observe_uploads uploads-after-delete
capture_call file-after-kb-delete GET "/api/v1/files/$file" >/dev/null
capture_call model-after-kb-delete GET "/api/v1/models/model?id=capture-kb-model" >/dev/null
# Removing a file from a base instead, with the default and with delete_file=false.
kept="$(capture_call create-kb-2 POST /api/v1/knowledge/create '{"name":"capture-docs-2","description":"Remove files"}' | jq -er .id)"
capture_call add-file-2 POST "/api/v1/knowledge/$kept/file/add" "{\"file_id\":\"$file\"}" >/dev/null
capture_call remove-file-keep POST "/api/v1/knowledge/$kept/file/remove?delete_file=false" "{\"file_id\":\"$file\"}" >/dev/null
observe_vectors vectors-after-remove-keep
observe_uploads uploads-after-remove-keep
capture_call file-after-remove-keep GET "/api/v1/files/$file" >/dev/null
capture_call add-file-3 POST "/api/v1/knowledge/$kept/file/add" "{\"file_id\":\"$file\"}" >/dev/null
capture_call remove-file-default POST "/api/v1/knowledge/$kept/file/remove" "{\"file_id\":\"$file\"}" >/dev/null
observe_vectors vectors-after-remove-default
observe_uploads uploads-after-remove-default
capture_call file-after-remove-default GET "/api/v1/files/$file" >/dev/null

capture_scenario delete
capture_call delete DELETE "/api/v1/knowledge/$kb/delete" >/dev/null
capture_call search-after-delete GET "/api/v1/knowledge/search?query=capture-kb" >/dev/null

capture_scenario get-missing
capture_call get-deleted GET "/api/v1/knowledge/$kb" >/dev/null
capture_call update-deleted POST "/api/v1/knowledge/$kb/update" '{"name":"x","description":"x"}' >/dev/null
capture_call delete-deleted DELETE "/api/v1/knowledge/$kb/delete" >/dev/null
capture_call update-access-deleted POST "/api/v1/knowledge/$kb/access/update" '{"access_grants":[]}' >/dev/null
capture_call get-after-update-access-deleted GET "/api/v1/knowledge/$kb" >/dev/null
