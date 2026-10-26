#!/usr/bin/env bash
# Start a throwaway Open WebUI instance (slim image, SQLite, ENV=dev) for one version,
# bootstrap its admin, create an API key and print the base URL and the key.
# On any failure after the container is created, it is removed again so the next run starts clean.
set -Eeuo pipefail
# shellcheck source=hack/openwebui/lib.sh
source "$(dirname "$0")/lib.sh"

version="${1:-}"
owui_require_version "$version"
name="$(owui_container "$version")"
url="$(owui_url "$version")"

if docker container inspect "$name" >/dev/null 2>&1; then
  echo "$name already exists; run 'mise run owui:down $version' first" >&2
  exit 1
fi

trap 'echo "owui:up $version failed; removing $name" >&2; docker rm --force --volumes "$name" >/dev/null 2>&1 || true' ERR

# Optional: OWUI_NETWORK attaches the instance to a docker network (to reach helper containers),
# OWUI_EXTRA_ENV adds environment variables, one KEY=VALUE per line.
extra=()
if [[ -n "${OWUI_NETWORK:-}" ]]; then
  extra+=(--network "$OWUI_NETWORK")
fi
while IFS= read -r line; do
  [[ -z "$line" ]] && continue
  if [[ "$line" != [A-Za-z_]*=* ]]; then
    echo "OWUI_EXTRA_ENV: '$line' is not KEY=VALUE" >&2
    exit 2
  fi
  extra+=(--env "$line")
done <<<"${OWUI_EXTRA_ENV:-}"

# ENV=dev serves /openapi.json (the image defaults to ENV=prod).
# PYTHONHASHSEED=0 keeps /openapi.json reproducible: multi-method routes derive their
# operationId from a Python set, whose order otherwise changes on every start.
# ENABLE_OLLAMA_API=false skips probing a non-existent Ollama backend at startup.
docker run --detach --name "$name" \
  --publish "$(owui_port "$version"):8080" \
  --env ENV=dev \
  --env PYTHONHASHSEED=0 \
  --env WEBUI_SECRET_KEY="$OWUI_SECRET_KEY" \
  --env WEBUI_ADMIN_EMAIL="$OWUI_ADMIN_EMAIL" \
  --env WEBUI_ADMIN_PASSWORD="$OWUI_ADMIN_PASSWORD" \
  --env WEBUI_ADMIN_NAME="$OWUI_ADMIN_NAME" \
  --env ENABLE_API_KEYS=true \
  --env ENABLE_OLLAMA_API=false \
  "${extra[@]}" \
  "$OWUI_IMAGE:v$version-slim" >/dev/null

echo "waiting for $url/health ..." >&2
for _ in $(seq 1 180); do
  curl -fsS "$url/health" >/dev/null 2>&1 && break
  if [[ "$(docker inspect --format '{{.State.Running}}' "$name")" != true ]]; then
    echo "$name exited during startup; last logs:" >&2
    docker logs --tail 50 "$name" >&2
    false
  fi
  sleep 2
done
if ! curl -fsS "$url/health" >/dev/null 2>&1; then
  echo "$name not healthy after 6 minutes; last logs:" >&2
  docker logs --tail 50 "$name" >&2
  false
fi

token="$(curl -fsS "$url/api/v1/auths/signin" \
  -H 'Content-Type: application/json' \
  -d "$(jq -n --arg e "$OWUI_ADMIN_EMAIL" --arg p "$OWUI_ADMIN_PASSWORD" '{email: $e, password: $p}')" \
  | jq -er .token)"

api_key="$(curl -fsS -X POST "$url/api/v1/auths/api_key" \
  -H "Authorization: Bearer $token" \
  | jq -er .api_key)"

trap - ERR
echo "OWUI_URL=$url"
echo "OWUI_API_KEY=$api_key"
