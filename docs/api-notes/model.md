# Model

Workspace models: an upstream base model wrapped with a name, metadata, parameters and sharing. Observed on **0.11.4** and **0.12.4**. Evidence is cited as `<version>/<scenario>#<step>`, which resolves to step `<step>` of `test/fixtures/openwebui/v<version>/model/<scenario>.json`. Regenerate with `mise run owui:capture model <version>` (scenarios in `hack/capture/scenarios/model.sh`; a fake OpenAI-compatible upstream, `hack/capture/fake-openai.py`, exposes the base model `capture-base`). Claims marked *(source)* come from `open_webui/routers/models.py` and `open_webui/models/models.py` in each image. Sharing is described in [access-grants.md](access-grants.md).

## Endpoints

Models are addressed by their user-chosen `id`, passed as a **query parameter** or in the body (never in the path), so ids may contain `/` *(source)*. All accept the admin's API key.

| Operation | Method and path | Request body | Response |
| --- | --- | --- | --- |
| Create | `POST /api/v1/models/create` | `ModelForm` | model |
| Get | `GET /api/v1/models/model?id={id}` | | model + `write_access`, `user` |
| List all | `GET /api/v1/models/all` (admin) | | model[], no pagination |
| List (paginated) | `GET /api/v1/models/list?page=` | | `{items, total}`, 30 per page *(source)* |
| Export by ids | `GET /api/v1/models/export?ids={id}&ids={id}` | | model[] + `background_image_data` |
| Update | `POST /api/v1/models/model/update` | `ModelForm` | model |
| Update access | `POST /api/v1/models/model/access/update` | `{id, access_grants}` | model |
| Toggle active | `POST /api/v1/models/model/toggle?id={id}` | | model |
| Delete | `POST /api/v1/models/model/delete` | `{"id": "…"}` (**id in the body**) | `true` |
| History (0.12.4) | `GET /api/v1/models/model/history?id={id}&page=`, `GET …/history/{history_id}?id={id}` | | entries |
| Base models for the warning | `GET /api/models` | | `{data: [...]}` |
| Import, sync | `POST /api/v1/models/import`, `POST /api/v1/models/sync` | | not used, see below |

There is no lookup by name; the model `id` is the unique key (`*/create#create-duplicate-id`).

## Payloads

`ModelForm`: `id` (required, no whitespace), `name` (required), `meta` and `params` (required objects, both accept any extra key), `base_model_id`, `access_grants`, `is_active` (defaults to `true`). 0.12.4 adds `commit_message` (history only) *(source)*.

```json
{"id": "capture-assistant", "name": "Capture Assistant", "base_model_id": "capture-base",
 "meta": {"description": "Assistant defined as code", "tags": [{"name": "capture"}],
          "capabilities": {"vision": false, "citations": true},
          "suggestion_prompts": [{"content": "Summarize this document"}],
          "toolIds": [], "knowledge": [], "filterIds": [], "actionIds": [], "hidden": false},
 "params": {"system": "You are a capture assistant.", "temperature": 0.2, "top_p": 0.9, "max_tokens": 512},
 "is_active": true}
```

The stored model (`*/get#get`) returns that payload with `user_id`, `created_at`, `updated_at`, `access_grants`, and on 0.12.4 `version_id`. The server adds `null` meta keys of its own: `profile_image_url`, `background_image_url`, `i18n`, plus `voice` and `voice_avatar` on 0.12.4.

## Field mapping

Spec #3 user stories 8 and 9, and the `access` block (story 20):

| CRD field | Payload | Behavior | Evidence |
| --- | --- | --- | --- |
| `modelId` | `id` | user-chosen, unique; a second create with the same id gives **401** "model id is already registered" | `*/create#create-duplicate-id` |
| display name | `name` | required in every update *(source: `ModelForm`)* | `*/update-fields#get-after-name` |
| base model | `base_model_id` | **not validated**: a base exposed by no connection is accepted; omitted on update keeps the stored value | `*/create#create-unexposed-base`, `*/update-fields#get-after-partial-payload` |
| `active` | `is_active` (top level) | see [Behavior](#behavior): omitted on update resets to `true` on 0.11.4, kept on 0.12.4 | `*/update-fields#get-after-inactive`, `#get-after-partial-payload` |
| `hidden` | `meta.hidden` | plain meta key | `*/update-fields#get-after-hidden` |
| meta description | `meta.description` | | `*/update-fields#get-after-description` |
| meta tags | `meta.tags` | list of `{"name": …}`; plain strings are converted to that form | `*/update-fields#get-after-tags` |
| meta capabilities | `meta.capabilities` | open object; unknown keys stored verbatim | `*/update-fields#get-after-capabilities` |
| meta suggestion prompts | `meta.suggestion_prompts` | list of `{"content": …}` | `*/update-fields#get-after-suggestion-prompts` |
| params system prompt | `params.system` | | `*/update-fields#get-after-system` |
| params temperature | `params.temperature` | | `*/update-fields#get-after-temperature` |
| params top-p | `params.top_p` | | `*/update-fields#get-after-top-p` |
| params max tokens | `params.max_tokens` | | `*/update-fields#get-after-max-tokens` |
| params additional params | **flat keys of `params`** (`params.seed`, `params.custom_param`) | there is no nested object: additional params sit next to `system`, `temperature`… and can collide with them | `*/update-fields#get-after-additional-params` |
| tools | `meta.toolIds` | list of tool ids; **not validated**: an unknown id is stored | `*/update-fields#get-after-tools` |
| knowledge | `meta.knowledge` | list of **objects**, `{"id": "<knowledge id>", "name": …, "type": "collection"}`; stored verbatim. The server reads `id` and `type` (`file`, `note`, anything else = knowledge base) *(source)* | `*/update-fields#get-after-knowledge` |
| filters | `meta.filterIds` | list of function ids; **not validated** | `*/update-fields#get-after-filters` |
| actions | `meta.actionIds` | list of function ids; **not validated** | `*/update-fields#get-after-actions` |
| `access` | `access_grants`, through the access endpoint only | [access-grants.md](access-grants.md) | `*/access-update#*` |

Fields with no API counterpart: none. Fields that behave differently from the CRD shape: additional params (flat, not nested), knowledge (objects, not ids), tags (objects, not strings), `hidden` (in `meta`, not next to `is_active`).

## Server-computed fields

| Field | Value | Evidence |
| --- | --- | --- |
| `user_id` | id of the creating admin (API key owner) | `*/create#create` |
| `created_at`, `updated_at` | epoch seconds; `updated_at` is bumped by updates and access updates *(source)* | normalized in fixtures |
| `version_id` (0.12.4) | id of the current history entry; changes only when a new version is written | `0.12.4/history#*` |
| `access_grants[].id`, `resource_type`, `resource_id`, `created_at` | see [access-grants.md](access-grants.md) | `*/create-with-access-grants#get` |
| `meta.profile_image_url`, `background_image_url`, `i18n` (+ `voice`, `voice_avatar` on 0.12.4) | `null` unless set | `*/get#get` |
| `write_access`, `user` (get only) | caller's write right; owner info (`null` here) | `*/get#get` |

## Behavior

- **`meta` and `params` are replaced wholesale.** An update with `meta: {}` and `params: {}` clears description, tags, capabilities, tool ids and every param (`*/update-fields#get-after-partial-payload`). The provider must send the complete desired `meta` and `params`, starting from the stored ones so unmanaged keys survive.
- **`base_model_id` omitted on update keeps the stored value** on both versions (`*/update-fields#get-after-partial-payload`).
- **`is_active` omitted on update**: 0.11.4 resets it to `true` (`0.11.4/update-fields#get-after-partial-payload`, after `#get-after-inactive` showed `false`); 0.12.4 keeps it (`0.12.4/update-fields#get-after-partial-payload`). The provider must always send `is_active`.
- **Toggle flips** `is_active` (`*/update-fields#get-after-toggle`); it is not idempotent and the provider must not use it.
- **References are not validated** on create or update: unknown tool, filter and action ids are stored as sent (`*/update-fields#get-after-tools`, `#get-after-filters`, `#get-after-actions`). On 0.12.4 they are validated only when restoring a history version (`POST …/model/update/version`), which also refuses an unavailable base model *(source: `_verify_version_dependencies`)*.
- **Base models for the "not exposed" warning**: `GET /api/models` lists the connections' models (entries with `urlIdx` and no `preset`, e.g. `capture-base`) together with workspace models (`preset: true`, with `info`) (`*/base-models#app-models`, `*/list#app-models`). A base model is exposed when its id appears among the entries that carry a `urlIdx` (the connection index). Neither `preset` nor its absence is enough: 0.11.4 also lists the built-in `arena-model`, with no `preset` and no `urlIdx` (`0.11.4/base-models#app-models`). `GET /api/v1/models/base` lists stored base-model records, not connection models, and is empty here (`*/base-models#base`): it cannot back the warning.
- **Export by ids** takes repeated `ids` parameters (`*/export#export-by-ids`). If one id does not exist the whole call answers **403** (`*/get-missing#export-deleted`).
- **Import** upserts each model of the payload after per-model permission checks; **sync** writes the given models and **deletes every model not in the payload** *(source, both versions)*. Neither fits a per-object reconciler; the provider uses neither.

### Version history (0.12.4)

Each history entry holds a `snapshot` of `name`, `base_model_id`, `meta` and `params`, plus `parent_id`, `commit_message`, `user_id` and `created_at` (`0.12.4/history#history-entry`). Which writes add a version, starting from a fresh model:

| Write | History entries after | Evidence |
| --- | --- | --- |
| create | 1 | `0.12.4/history#history-after-create` |
| update with **the same payload** (no change) | **1, no new version** | `0.12.4/history#history-after-no-change` |
| update changing `params.temperature` | 2 | `0.12.4/history#history-after-change` |
| access update only | 2 | `0.12.4/history#history-after-access-only` |
| update changing only `is_active` | 2 | `0.12.4/history#history-after-is-active-only` |
| toggle | 2 | `0.12.4/history#history-after-toggle` |
| access update on a **missing** id (re-creates the model, see [Missing model](#missing-model)) | a new model with its own first version | `0.12.4/get-missing#get-after-update-access-deleted` (`version_id` set) |

A version is written only when the snapshot differs from the stored one *(source: `_write_model`)*, and `version_id` stays the same otherwise. Entries are listed newest first, 20 per page *(source: `get_history_by_model_id`)*; fixtures sort lists by id, so their order is not evidence. 0.11.4 has no history: the routes do not exist and answer the web UI's HTML page (`0.11.4/history#history-after-create`).

## Missing model

| Request on a missing id | 0.11.4 | 0.12.4 |
| --- | --- | --- |
| `GET /model?id=` | 404 | 404 |
| `POST /model/update` | **401** `We could not find what you're looking for :/` | **401** |
| `POST /model/delete` | **401** | **401** |
| `GET /export?ids=` | **403** permission message | **403** |
| `GET /model/history?id=` | route absent: `200` HTML | 404 |
| `POST /model/access/update` | **200, re-creates the model** | **200, re-creates the model** |

Evidence: `*/get-missing#*`, on a model that existed and was deleted (`*/delete#delete`). The access endpoint on a missing id inserts a minimal model (`name` = id, empty `meta` and `params`, no base model, active) and applies the grants (`*/get-missing#get-after-update-access-deleted`).

## Differences between versions

- 0.12.4 adds version history (`version_id`, history endpoints, `commit_message`) and the `voice` / `voice_avatar` meta keys.
- `is_active` omitted on update: reset to `true` on 0.11.4, kept on 0.12.4.
- `access_grants` in the update body: applied on 0.11.4, ignored on 0.12.4 (see [access-grants.md](access-grants.md)).
- `GET /api/models` lists the built-in `arena-model` on 0.11.4 only.

## Deviations from spec #3

- **Additional params are not a nested object**: they are extra keys of `params`, next to the named ones. The CRD's free-form `additionalParams` must be flattened into `params`, and the provider must reject a key that collides with a named field.
- **Tags, knowledge and suggestion prompts are objects** (`{"name"}`, `{"id", "name", "type"}`, `{"content"}`), not plain strings or ids.
- **`hidden` is `meta.hidden`**, while `active` is the top-level `is_active`.
- **The base model is never rejected**, so story 11's warning is the only signal; it must be computed from `GET /api/models`, not `GET /api/v1/models/base`.
- **References are not validated by the server**: story 10's wrong-function-type check and the existence of tools, knowledge and functions are entirely the provider's job.
- **`meta` and `params` are replaced on update**, so writing only the declared fields requires a read-modify-write that keeps unmanaged keys.
- **`is_active` must always be sent** (0.11.4 resets it otherwise).
- **Story 33 (sharing-only difference calls only the access endpoint) is required for correctness, not only to avoid versions**: on 0.11.4 an object update carrying `access_grants` rewrites the grants (`0.11.4/access-update#get-after-update-with-grants`), while 0.12.4 ignores them (`0.12.4/access-update#get-after-update-with-grants`), so sharing can only be written reliably through the access endpoint.
- **Steady state creates no versions**: an update with the same payload adds no history entry on 0.12.4, so the anti-loop test's history check holds even if the provider rewrote an unchanged model; "writes only on a real difference" still matters for `updated_at` and events.
- **Not-found quirks**: update and delete answer **401** for a missing model (get answers 404), export answers 403, and the access endpoint re-creates the model. `Delete` must treat 401 after a confirming 404 get as gone, and the provider must never call the access endpoint for a model `Observe` did not find.
- **A duplicate id answers 401, not 409**: `Create` must map it to "already exists" (adoption rules) by checking with a get first.
