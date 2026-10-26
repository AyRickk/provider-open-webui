# Limitations

What `provider-openwebui` deliberately does not do, and what to use instead. Each limitation gets its own issue once phase 1 starts.

## Knowledge base content

The `Knowledge` resource manages a knowledge base's name, description and access. It does not upload, sync or delete files: document ingestion happens by other means (spec #3, story 19).

Deleting a `Knowledge` resource deletes the base but **not its files**: Open WebUI keeps the file records, their bytes and their vectors (see [Knowledge: files and vectors on deletion](api-notes/knowledge.md#files-and-vectors-on-deletion)). Remove the files first, or clean them up afterwards.

### Evaluation: Knowledge Base Sync (oikb)

[oikb](https://docs.openwebui.com/ecosystem/knowledge-base-sync) is Open WebUI's official companion tool for filling knowledge bases. It is evaluated here as an ingestion path; it is not integrated.

- **What it does**: mirrors a local folder or an external source (GitHub, Confluence, S3 and other connectors) into one knowledge base. It hashes every file (SHA-256), asks the server for the difference (`POST /api/v1/knowledge/{id}/sync/diff`), uploads only new and changed files and removes deleted ones (`POST /api/v1/knowledge/{id}/sync/cleanup`). Runs as a one-shot CLI (`pip install oikb`, Python 3.11+), a long-running daemon (scheduled, webhook or on-demand syncs), or the container image `ghcr.io/open-webui/oikb`.
- **Fit with the provider**: good, and complementary. oikb targets an **existing** knowledge base **by id** (`--kb-id`, or `kb-id` in `.oikb.yaml`; `oikb validate --deep` checks it exists), so the provider creates the base, its sharing and its lifecycle, and publishes the id (external-name and status); oikb fills it. The two touch disjoint parts of the API: the provider never calls the file endpoints, oikb never changes the name, description or access.
- **Compatibility**: oikb requires Open WebUI 0.9.6 or newer; both supported versions (0.11.4, 0.12.4) expose `sync/diff` and `sync/cleanup` (`docs/api-notes/openapi/`). One report of `oikb status` and `oikb ls` failing against a 0.11 development build ([open-webui/oikb#91](https://github.com/open-webui/oikb/issues/91)) while sync itself worked; to recheck on the supported versions before recommending it.
- **Operations**: it authenticates with an Open WebUI API key, which needs `ENABLE_API_KEYS` (off by default) and, for a non-admin user, the API Keys permission; the user needs write access to the base, not admin rights; the daemon must run as a **single replica** (schedules and per-base locks live in one process). Ingestion still requires the vector store and embedding engine Open WebUI is configured with; on the slim image that means pgvector and an external engine (see [Knowledge: ingestion needs external services](api-notes/knowledge.md#ingestion-needs-external-services)).
- **Risks**: `oikb reset` deletes every file of a base, not only the ones oikb uploaded. Deleting the `Knowledge` resource orphans what oikb uploaded (above).
- **Verdict**, provisional until oikb is rechecked on the supported versions: the recommended ingestion path, documented in the GitOps guide as a separate deployment (for Kubernetes, one `Deployment` with `replicas: 1` per daemon, its API key from a Secret, the base id taken from the `Knowledge` resource's status), with no code in the provider. Revisit only if a single-replica daemon is unacceptable.

Sources: [Knowledge Base Sync documentation](https://docs.openwebui.com/ecosystem/knowledge-base-sync), [oikb on PyPI](https://pypi.org/project/oikb/) and [open-webui/oikb#91](https://github.com/open-webui/oikb/issues/91). Consulted on 2026-10-26.
