# shellcheck shell=bash
# Helpers the knowledge capture needs before the instance starts. The slim image has no local
# vector store or embedding model, so file ingestion needs a pgvector Postgres and an
# OpenAI-compatible embedding endpoint (hack/capture/fake-openai.py), both on a private network.
instance="$(owui_container "$CAPTURE_VERSION")"
network="$instance-net"
pgvector="$instance-pgvector"
upstream="$instance-upstream"
pgvector_password="capture-pgvector-password"
upstream_key="capture-upstream-key"
CAPTURE_SECRETS+=("$pgvector_password" "$upstream_key")

capture_cleanup() {
  docker rm --force "$pgvector" "$upstream" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
}

docker network create "$network" >/dev/null
docker run --detach --name "$pgvector" --network "$network" \
  -e POSTGRES_PASSWORD="$pgvector_password" -e POSTGRES_DB=vectors pgvector/pgvector:0.8.0-pg17 >/dev/null
docker run --detach --name "$upstream" --network "$network" --entrypoint python \
  -v "$CAPTURE_DIR/fake-openai.py:/fake-openai.py:ro" "$OWUI_IMAGE:v$CAPTURE_VERSION-slim" /fake-openai.py >/dev/null
for _ in $(seq 1 60); do
  docker exec "$pgvector" pg_isready -U postgres -d vectors >/dev/null 2>&1 \
    && docker exec "$upstream" python -c 'import urllib.request; urllib.request.urlopen("http://localhost:8000/v1/models")' 2>/dev/null \
    && break
  sleep 1
done
docker exec "$pgvector" pg_isready -U postgres -d vectors >/dev/null
docker exec "$upstream" python -c 'import urllib.request; urllib.request.urlopen("http://localhost:8000/v1/models")'

OWUI_NETWORK="$network"
OWUI_EXTRA_ENV="PGVECTOR_DB_URL=postgresql://postgres:$pgvector_password@$pgvector:5432/vectors
RAG_EMBEDDING_ENGINE=openai
RAG_OPENAI_API_BASE_URL=http://$upstream:8000/v1
RAG_OPENAI_API_KEY=$upstream_key
RAG_EMBEDDING_MODEL=capture-embed"
export OWUI_NETWORK OWUI_EXTRA_ENV
