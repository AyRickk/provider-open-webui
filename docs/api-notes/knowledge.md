# Knowledge

Knowledge bases: a named container of files used for retrieval, plus its sharing. The provider manages the container and its access only, not the files. Observed on **0.11.4** and **0.12.4**. Evidence is cited as `<version>/<scenario>#<step>`, which resolves to step `<step>` of `test/fixtures/openwebui/v<version>/knowledge/<scenario>.json`. Regenerate with `mise run owui:capture knowledge <version>` (scenarios in `hack/capture/scenarios/knowledge.sh`, helpers in `knowledge.before.sh`). Claims marked *(source)* come from `open_webui/routers/knowledge.py` and `open_webui/retrieval/` in each image. Sharing is described in [access-grants.md](access-grants.md).

Steps holding an `observation` record what the scenario saw outside the API: rows of the pgvector `document_chunk` table per collection, and the files in the instance's `data/uploads` directory.

## Endpoints

The admin's API key works on every one of them (all fixture steps use it); non-admins need `workspace.knowledge` and write access *(source)*. The routes are the same on both versions.

| Operation | Method and path | Request body | Response |
| --- | --- | --- | --- |
| Create | `POST /api/v1/knowledge/create` | `{name, description, access_grants?}` | knowledge base |
| Get | `GET /api/v1/knowledge/{id}` | | knowledge base + `write_access`, `files: null` |
| List | `GET /api/v1/knowledge/?page=` | | `{items, total}`, 30 per page; items carry `file_count` and `user` instead of `files` (`*/list#list-page-1`) |
| Search | `GET /api/v1/knowledge/search?query=` | | `{items, total}`, items as in the list, substring match on the name (`*/find-by-name#search-exact-name`); also takes `page`, `order_by`, `direction`, 30 per page *(source)* |
| Update | `POST /api/v1/knowledge/{id}/update` | `{name, description, access_grants?}` | knowledge base with `files: []` (`*/update#update-name`) |
| Update access | `POST /api/v1/knowledge/{id}/access/update` | `{"access_grants": [...]}` | knowledge base |
| Delete | `DELETE /api/v1/knowledge/{id}/delete` | | `true` |
| Files (not managed) | `GET …/{id}/files`, `POST …/{id}/file/add`, `…/file/remove?delete_file=`, `…/files/batch/add`, `…/reset`, `…/sync/diff`, `…/sync/cleanup`, … | | used by ingestion tools such as [oikb](../limitations.md#knowledge-base-content) |

## Payloads

Create and update take `name` and `description`, **both required** on update: sending only `name` gives 422 `Field required` for `description` (`*/update#update-name-only`). `access_grants` is optional.

Stored knowledge base (`*/get#get`):

```json
{"id": "…", "user_id": "…", "name": "capture-kb", "description": "Knowledge base defined as code",
 "meta": null, "access_grants": [], "files": null, "write_access": true,
 "created_at": 1700000000, "updated_at": 1700000000}
```

## Server-computed fields

| Field | Value | Evidence |
| --- | --- | --- |
| `id` | UUID assigned on create | `*/create#create` |
| `user_id` | creating admin (API key owner) | `*/create#create` |
| `created_at`, `updated_at` | epoch seconds | normalized in fixtures |
| `meta` | `null` for a plain knowledge base; holds the connection for external ones *(source)* | `*/get#get` |
| `files` | `null` on get; the files are listed by `GET …/{id}/files` | `*/get#get`, `*/files-and-deletion#kb-files` |
| `access_grants` | see [access-grants.md](access-grants.md) | `*/get#get-shared` |

## Behavior

- **Names are not unique, and may be empty.** A second base named `capture-kb` and a base named `""` are both created (`*/create#create-duplicate-name`, `#create-empty-name`).
- **No lookup by name.** `GET …/search?query=` matches a **substring** of the name: `capture-kb` returns both bases of that name, `capture-page-0` returns nine (`*/find-by-name#search-exact-name`, `#search-prefix`). Find-by-name must filter the results on the exact name, and paginate.
- **Pagination**: `page` (1-based; values below 1 count as 1, and it is the only parameter *(source)*) on `GET /api/v1/knowledge/`; each page holds 30 items and the response carries `total`. With 35 bases, page 1 holds 30, page 2 holds 5, page 3 is empty with the same `total` (`*/list#list-page-1`, `#list-page-2`, `#list-page-3`). Page 1 holds the 30 most recently updated bases (`updated_at` descending, *(source)*); within a page, fixtures sort items by id.
- **Access**: grants follow [access-grants.md](access-grants.md). A create with grants stores them (`*/get#get-shared`); the access endpoint replaces the list (`*/access-update#get-after-update-access`), `[]` makes the base private (`*/access-update#get-after-make-private`). `access_grants` in the **update** body are applied on 0.11.4 (`0.11.4/access-update#get-after-update-with-grants`: they become `user:*:read`) and ignored on 0.12.4 (`0.12.4/access-update#get-after-update-with-grants`), as for models and tools.
- The access endpoint on a missing base answers 404 and creates nothing (`*/get-missing#update-access-deleted`, then `#get-after-update-access-deleted`: still 404), unlike the model one.

## Files and vectors on deletion

Observed with one text file, ingested into a base that a model referenced. The slim image ships neither a local vector store nor an embedding model, so the capture ran with a pgvector Postgres and an OpenAI-compatible embedding endpoint (`knowledge.before.sh`); see [Ingestion needs external services](#ingestion-needs-external-services).

| Moment | Vector collections (rows) | `data/uploads` | File record | Evidence |
| --- | --- | --- | --- | --- |
| file uploaded, added to the base | `<knowledge id>` (1), `file-<file id>` (1), `knowledge-bases` (one row per base) | the file | exists | `*/files-and-deletion#vectors-before-delete`, `#uploads-before-delete` |
| **base deleted** | `file-<file id>` (1) kept; `<knowledge id>` gone; its row in `knowledge-bases` gone | **the file is kept** | **still exists** (`200`) | `#vectors-after-delete`, `#uploads-after-delete`, `#file-after-kb-delete` |
| file removed from another base with `delete_file=false` | `file-<file id>` kept | kept | exists | `#vectors-after-remove-keep`, `#uploads-after-remove-keep`, `#file-after-remove-keep` |
| file removed with the default `delete_file` | `file-<file id>` gone | **removed** | **gone** (`404`) | `#vectors-after-remove-default`, `#uploads-after-remove-default`, `#file-after-remove-default` |

So:

- **Deleting a knowledge base orphans its files**: the file records, their bytes on disk and their own vectors (`file-<id>`) stay; only the base's collection and its metadata embedding are removed.
- **Deleting a knowledge base rewrites every model that referenced it**: its entry is removed from `meta.knowledge` (`*/files-and-deletion#model-after-kb-delete`: `[]`), which on 0.12.4 adds a model version (`version_id` changes, `0.12.4/files-and-deletion#create-model-using-kb` vs `#model-after-kb-delete`).
- Removing a file from a base deletes the file, its bytes and its vectors by default; `delete_file=false` keeps them. The default is `not ENABLE_KNOWLEDGE_FILE_RETENTION` *(source)*.
- Every create, update and delete of a base also writes or removes one row in the `knowledge-bases` collection, the embedding of its name and description used for semantic search (`#vectors-before-delete`: 36 rows for 36 bases) *(source: `embed_knowledge_base_metadata`)*.

### Ingestion needs external services

On the slim image, without configuration, a file upload is accepted (`200`, `status: pending`) and then fails with `503: Configure PGVECTOR_DB_URL for remote vector storage.`; adding it to a base answers `400`, and knowledge base metadata embedding logs `Configure an external embedding engine (openai, ollama, azure_openai)` while the create itself succeeds. Observed on 0.12.4 during the spike, not in the fixtures: `VECTOR_DB` defaults to `pgvector` and the slim image has neither `chromadb` nor `torch` or `sentence_transformers` on both versions *(source: `config.py`, `retrieval/vector/factory.py`)*. Creating, updating, sharing and deleting bases, which is all the provider does, works without either service.

## Missing knowledge base

| Request on a missing id | 0.11.4 | 0.12.4 |
| --- | --- | --- |
| `GET /{id}` | 404 | 404 |
| `POST /{id}/update` | **400** `We could not find what you're looking for :/` | **400** |
| `DELETE /{id}/delete` | **400** | **400** |
| `POST /{id}/access/update` | 404, nothing created | 404, nothing created |

Evidence: `*/get-missing#*` (`#get-after-update-access-deleted` shows nothing was created), on a base that existed and was deleted (`*/delete#delete`).

## Differences between versions

- `access_grants` in the update body: applied on 0.11.4, ignored on 0.12.4.
- A knowledge base deletion adds a version to every model that referenced it on 0.12.4 (models have no history on 0.11.4).

Everything else observed is identical: routes, payloads, pagination, deletion side effects and error codes.

## Deviations from spec #3

- **"find by unique key (… knowledge name)"**: knowledge base names are not unique and may be empty, and search is a substring match. Find-by-name must filter on the exact name and treat several matches as a conflict, as for groups.
- **Deleting a knowledge base orphans its files and their vectors**: spec #3's knowledge resource manages the container only, so a `Delete` leaves files, bytes on disk and `file-<id>` vectors behind. This needs a documented limitation (and its issue); the provider must not delete files it did not create.
- **Deleting a knowledge base edits models**: Open WebUI removes the base from every model's `meta.knowledge`. A `Model` that still declares it then drifts and re-adds the id, which points to nothing. Ordering deletions with Crossplane `Usage` (story 55) avoids it.
- **Update requires `description`**: an update always sends both `name` and `description`.
- **Not-found quirks**: update and delete answer **400** for a missing base, get and the access endpoint 404.
- **Ingestion needs external services on the slim image** (pgvector and an embedding engine): outside the provider's scope, but the GitOps and bootstrap guides must say so, since a knowledge base without them accepts files that then fail.
