# ADR-0001: Phase 0 consolidation of the Open WebUI API

- **Status:** proposed, awaiting the maintainer's validation (gate to phase 1, spec #3 "Delivery process")
- **Date:** 2026-10-27
- **Scope:** the six resources of spec #3 (`Group`, `Model`, `Prompt`, `Tool`, `Function`, `Knowledge`) and the cross-cutting API facts, on Open WebUI **0.11.4** and **0.12.4**.

## Context

Spec #3 states that "API facts are not assumed: phase 0 establishes the endpoints, payloads, server-computed fields, pagination, the member-read endpoint, valve read-back behavior, toggle semantics, how `public` is encoded in `access_grants`, and what happens to files and vectors when a knowledge base is deleted", and that "observed API behavior takes precedence over this document". Phase 0 (#4 to #11) captured every resource on both versions against throwaway instances and recorded the results:

- [API notes](../api-notes/README.md): one page per resource ([Group](../api-notes/group.md), [Model](../api-notes/model.md), [Prompt](../api-notes/prompt.md), [Tool](../api-notes/tool.md), [Function](../api-notes/function.md), [Knowledge](../api-notes/knowledge.md)), the cross-cutting facts, and [access grants](../api-notes/access-grants.md). Every claim cites a fixture step under `test/fixtures/openwebui/`, or is marked *(source)* when read from Open WebUI's code.
- [OpenAPI diff](../api-notes/openapi-diff.md) of the routes the provider uses, 0.11.4 → 0.12.4.
- [Minimal API-key allow-list](../api-notes/api-key-allowed-endpoints.txt), verified by `test/fixtures/openwebui/v*/endpoint-restrictions/`, with an unrestricted control in `…/endpoint-restrictions-control/`.
- [Marker storage](#marker-storage), verified by `test/fixtures/openwebui/v*/marker/`.
- [Limitations](../limitations.md), with the Knowledge Base Sync (oikb) evaluation.

This ADR confirms or adjusts each part of spec #3 against those facts. It does not cover the other ADRs spec #3 asks for (GitHub Actions and ghcr, required identifiers, no `Group.members`, adoption without the annotation, RBAC scope, chainsaw, `crd-ref-docs`); they come with phase 1.

## Decision

### Cross-cutting

| Spec #3 | Observed | Decision |
| --- | --- | --- |
| Version from `GET /api/version` (unauthenticated, `version` and `deployment_id`) | Confirmed on both versions; the route is outside API-key restrictions | **Confirmed.** |
| Bootstrap with `WEBUI_ADMIN_*`, `ENABLE_API_KEYS`, `ENABLE_API_KEYS_ENDPOINT_RESTRICTIONS`, `API_KEYS_ALLOWED_ENDPOINTS` | Names confirmed; the `*API_KEYS*` settings are **persistent config**, seeded only on a database's first boot ([README](../api-notes/README.md#bootstrap-environment-variables)) | **Adjusted:** the bootstrap guide says they must be set on first boot or changed in the admin config afterwards. |
| Credential check | `GET /api/v1/auths/` refuses API keys on 0.12.4 ("A human session is required.") | **Decided:** the provider checks its credentials with `GET /api/v1/users/default/permissions`, which is admin-only on both versions, accepts API keys, and is needed anyway for the `Exact` policy (`*/endpoint-restrictions/cross#default-permissions`). |
| `Authorization: Bearer` | Works on both; `x-api-key` too, renamed by `CUSTOM_API_KEY_HEADER` | **Confirmed**, Bearer only. |
| API-key endpoint restrictions (story 61) | Matching is exact path or path prefix followed by `/`, method-agnostic, no wildcards *(source: `utils/auth.py`, both versions)* | **Decided:** the [allow-list](../api-notes/api-key-allowed-endpoints.txt) below is documented in the bootstrap guide. |
| Sharing (`access` block, stories 20–22) | `public` is the `user:*` read grant; invalid grants are dropped silently; principals are not checked for existence ([access-grants.md](../api-notes/access-grants.md)) | **Adjusted:** see [Sharing](#sharing). |
| No spurious writes, no spurious versions (stories 31–34) | Every update bumps `updated_at`; a no-op update creates **no** history version on any resource with history | **Confirmed**; the anti-loop history check is valid. |

#### Minimal allow-list

```text
/api/v1/users/default/permissions    credential check, Exact policy
/api/v1/groups/                      list (find by name); exact path
/api/v1/groups/create
/api/v1/groups/id                    get, update, delete (and, unavoidably, member routes)
/api/models                          base models for the warning
/api/v1/models/create
/api/v1/models/model                 get, update, access, delete (id in query or body)
/api/v1/prompts/create
/api/v1/prompts/list                 find by command
/api/v1/prompts/id                   get, update, toggle, access, delete
/api/v1/tools/create
/api/v1/tools/id                     get, update, access, valves, delete
/api/v1/functions/create
/api/v1/functions/id                 get, update, toggles, valves, delete
/api/v1/knowledge                    every knowledge route (ids sit directly under the prefix)
```

With exactly this list, every operation of [`provider-routes.tsv`](../api-notes/provider-routes.tsv) succeeds, valves reads and writes included (`*/endpoint-restrictions/*`), and the routes the provider never needs are refused with 403 (`/api/v1/auths/`, `/api/v1/users/`, `/api/v1/models/sync`, `/api/v1/models/delete/all`, `/api/v1/models/all`, `/openai/config`, `/api/v1/files/`), on both versions (`*/endpoint-restrictions/outside-the-list`). The restriction's 403 carries the generic access-prohibited message, so the same read-only routes were called on an unrestricted instance as a control: they answer 200 there (`*/endpoint-restrictions-control/outside-the-list`; 0.12.4's `/api/v1/auths/` keeps its own "A human session is required." refusal). Limits: an entry with a trailing slash matches only that exact path; the method is not checked, so `POST /api/v1/users/default/permissions` (which sets the defaults) is allowed too; prefixes also open group member routes and every knowledge route, file and reindex routes included (`*/endpoint-restrictions/outside-the-list#groups-users-add-covered`, `#knowledge-reindex-covered`). The list is the minimum the API allows, not least privilege.

### Not-found and conflict map

The client's per-endpoint quirk mapping (spec #3, "Errors map to the sentinels … with per-endpoint quirk mapping") is **per version as well as per endpoint**:

| Resource | Get | Update | Delete | Access update | Duplicate key on create |
| --- | --- | --- | --- | --- | --- |
| Group | 0.11.4 **401**, 0.12.4 404 | 0.11.4 **400**, 0.12.4 404 | 0.11.4 **`200 true`**, 0.12.4 404 | n/a | none (names not unique) |
| Model | 404 | **401** | **401** | **re-creates the model** | **401** |
| Prompt | 404 | 404 | 404 | 404 | 400 |
| Tool | 404 | **401** | **401** | 404 | 400 |
| Function | **401** | **400** | **`200 true`** | n/a | 400 |
| Knowledge | 404 | **400** | **400** | 404 | none (names not unique) |

Valves follow their resource: tool valves read 404 and write **401**, function valves read and write **401** on a missing object ([tool.md](../api-notes/tool.md#missing-tool), [function.md](../api-notes/function.md#missing-function)).

**Decided:** `Observe` decides existence with a get only; `Delete` treats any of the codes above after a confirming not-found get as success; the provider never calls an access endpoint for an object `Observe` did not find (the Model one re-creates it); a delete that answers `200 true` is followed by a get before the resource is reported gone.

### Group

Spec #3 stories 1–7 are **confirmed** with these adjustments ([group.md](../api-notes/group.md)):

- **Find-by-name**: names are not unique. Find-by-name lists all groups and filters on the exact name; **more than one match is a conflict** (`Synced=False`, no adoption), never a pick.
- **`Merge` policy**: the server replaces `permissions` as a whole, so `Merge` is a read-modify-write: stored map, overlaid with the declared keys, sent whole.
- **`Exact` policy**: "server-known keys" are those of `GET /api/v1/users/default/permissions` (65 keys on 0.11.4, 67 on 0.12.4), read at reconcile time, never hardcoded.
- **Update** always sends `name` and `description` (both required).
- **Member count** comes from `member_count` on every group read; no member route is used (story 6 holds).
- 0.12.4's `parent_group_id` (group hierarchy) is not managed and is never sent; it keeps its value when omitted.

### Model

Stories 8–11 are **confirmed** with these adjustments ([model.md](../api-notes/model.md)):

- **Field mapping**: `hidden` is `meta.hidden`; `active` is top-level `is_active`; `capabilities` is the open object `meta.capabilities` (unknown keys kept); tags, knowledge and suggestion prompts are objects (`{"name"}`, `{"id", "name", "type": "collection"}`, `{"content"}`); tool, filter and action references are id lists in `meta.toolIds`, `meta.filterIds`, `meta.actionIds`.
- **`additionalParams` is flattened** into `params`, next to `system`, `temperature`, `top_p` and `max_tokens`; a key that collides with a named param is **rejected by CEL** at admission.
- **`meta` and `params` are replaced** on update: the provider reads the stored ones, overlays the declared fields and sends them whole, so server keys (`profile_image_url`, `voice`, `params.model_controls`, …) survive.
- **`is_active` is always sent** (0.11.4 resets it to `true` when omitted).
- **References are not validated by the server**, base model included: the existence checks, the wrong-function-type check (story 10) and the base-model warning (story 11, computed from `GET /api/models` entries that carry a `urlIdx`) are the provider's.
- **Duplicate id answers 401**: `Create` checks with a get first and maps an existing id to the adoption rules.
- **History (0.12.4)**: a version is written only when `name`, `base_model_id`, `meta` or `params` change; access and `is_active` changes add none.

### Prompt

Stories 12–13 are **adjusted** ([prompt.md](../api-notes/prompt.md)):

- **Routing and external-name are the server UUID**, not the command; prompts have no immutable identifier field. `command` is an ordinary updatable field (the server lets an update change it); its uniqueness is checked by the server on create and update.
- **Leading `/`**: the server stores commands exactly as sent and treats `/x` and `x` as different. The provider normalizes the declared command to **one** spelling before writing and comparing, and find-by-command / adoption look for **both** spellings; two matches are a conflict. *Which spelling to write is a decision for the maintainer (see [Decisions needed](#decisions-needed)); the Open WebUI UI's own convention was not verified.*
- **Find-by-command** pages through `GET /api/v1/prompts/list?query=` (substring over name, command, content and owner) and filters on the exact command.
- **Active** is a toggle that flips; it is called only when the observed value differs (as story 36 for functions). New prompts start active. `GET /api/v1/prompts/` hides inactive prompts, so it is never used to observe.
- **Tags** are a list of strings, replaced by an update and kept when the field is omitted *(source)*; the provider always sends them.
- **Update** always sends `command`, `name`, `content`, and never `is_production: false`.

### Tool

Story 14 is **confirmed** with these adjustments ([tool.md](../api-notes/tool.md)):

- **`toolId` CEL rule**: `^[a-z_][a-z0-9_]*$`. The server accepts any Python identifier and lowercases it, so anything looser lets the CRD id and the stored id differ.
- **Code comparison applies `replace_imports`**: the server rewrites `from utils|apps|main|config` to `from open_webui.…` anywhere in the code, comments included; the client applies the same four replacements to the desired code before comparing. Line endings and whitespace are stored as sent.
- **Valves** (stories 15, 38): a valve write replaces the stored set, so the provider always sends the complete set (plain values overlaid with Secret values); unknown keys are dropped silently, so declared keys are validated against `GET …/valves/spec`; values, secrets included, read back **in clear**, so the hash is computed over that read-back and nothing from it reaches status, events or logs (story 57).
- **Syntax errors** are a 400 whose detail depends on version and operation; the condition carries the truncated detail (story 51).

### Function

Stories 17–18 and 36 are **confirmed** with these adjustments ([function.md](../api-notes/function.md)):

- **Toggles flip** (`is_active`, `is_global`); each is called only when the observed value differs, at most once per reconcile. New functions start **inactive and not global**.
- **`functionId` CEL rule**: `^[a-z_][a-z0-9_]*$`, as for tools.
- **Type** is detected from the code (`Pipe`, `Filter`, `Action`, `Event`, in that order) and **re-detected on every update**; status shows the server's `type`, which may be `event`; the wrong-type check for model references re-runs after a code change.
- **Code and valves** behave as for tools (same `replace_imports`, same whole-set valve writes, clear read-back); setting valves on a function without a `Valves` class answers 401, so valves are only written when `GET …/valves/spec` is not `null`.

### Knowledge

Story 19 is **confirmed** with these adjustments ([knowledge.md](../api-notes/knowledge.md)):

- **Find-by-name**: names are not unique and may be empty; `GET /api/v1/knowledge/search?query=` is a substring match, 30 per page. Find-by-name pages and filters on the exact name; more than one match is a conflict. The CRD requires a non-empty name.
- **Update** always sends `name` and `description` (422 without `description`).
- **Deletion side effects**: deleting a base keeps its files, their bytes and their `file-<id>` vectors (orphans), and removes the base from every model's `meta.knowledge` (a new model version on 0.12.4). Documented in [Limitations](../limitations.md#knowledge-base-content), with its issue in phase 1; `Usage` ordering (story 55) is the recommended guard.
- **Ingestion** needs pgvector and an external embedding engine on the slim image; out of the provider's scope, stated in the GitOps and bootstrap guides. **oikb** is the recommended ingestion path, provisionally ([evaluation](../limitations.md#evaluation-knowledge-base-sync-oikb)).

### Sharing

For every shareable resource ([access-grants.md](../api-notes/access-grants.md)):

- The `access` block maps to grants: `public` → `user:*` read; group and user read and write → one grant per principal and permission; `access: {}` → `[]`.
- **Sharing is written only through the access endpoint**: `access_grants` in an object update are applied on 0.11.4 and ignored on 0.12.4, so the object update never carries them (story 33 becomes a correctness rule, not only an optimization).
- Grants are compared as a set of `(principal_type, principal_id, permission)`, read back after every write; a difference after a write (a grant silently dropped) is reported, not retried in a loop.
- The server stores grants to principals that do not exist. Group references are resolved by the provider before writing (through the group routes of the allow-list). User ids are written as declared: checking them needs `GET /api/v1/users/`, which the minimal allow-list deliberately refuses, so an unknown user id is not detected (stated in the CRD reference).

### Marker storage

Documented only; **the provider does not depend on a marker** (adoption stays governed by the annotation, spec #3). Where a `managed_by` key is stored on create, and whether it survives an update that does not send it again, from `test/fixtures/openwebui/v*/marker/`:

| Resource | Field | Stored on create (0.11.4 / 0.12.4) | Survives an update without it | Note |
| --- | --- | --- | --- | --- |
| Group | `data.managed_by` | yes / yes | **yes** (`data` is merged) | `meta` is not writable |
| Model | `meta.managed_by` | yes / yes | **no** (`meta` is replaced) | the read-modify-write must carry it |
| Prompt | `meta.managed_by`, `data.managed_by` | yes / yes | **yes** | |
| Tool | `meta.managed_by` | **no** (dropped) / yes | **no** (`meta` is replaced) | 0.11.4's `ToolMeta` keeps only known keys |
| Function | `meta.managed_by` | yes / yes | **no** (`meta` is replaced) | |
| Knowledge | none | no / no | n/a | the create ignores `meta`; only the description could carry it |

On 0.12.4, `commit_message` on model, tool and function writes is a second, history-only place to label the provider's versions ([OpenAPI diff](../api-notes/openapi-diff.md)).

### OpenAPI diff

Of the 43 routes the provider uses, 25 are unchanged between 0.11.4 and 0.12.4 and 18 only gained fields; nothing was removed or changed type. 35 are classified **none** (no field the provider sends or reads changed) and 8 **profile** (model, tool and function writes and reads: `commit_message`, server-owned model keys, tool `meta` extra keys), **none breaking** ([report](../api-notes/openapi-diff.md)). Behavior changes without a schema change are what the 0.12.4 profile mostly absorbs: the not-found map above, `access_grants` in object updates, `is_active` on model update, the human-session guard on `/api/v1/auths/`, and the error details of tool updates.

## Deviations from spec #3

Every deviation recorded in the API notes, in one place. Each is resolved by the decision above; those needing the maintainer are repeated under [Decisions needed](#decisions-needed).

| # | Spec #3 | Deviation | Resolution |
| --- | --- | --- | --- |
| 1 | Bootstrap variables | `*API_KEYS*` settings only apply on first boot; `GET /api/v1/auths/` refuses API keys on 0.12.4 | Bootstrap guide; credential check on `/api/v1/users/default/permissions` |
| 2 | "find by unique key (group name …)" | Group names are not unique | Exact-name filter; several matches = conflict |
| 3 | "a missing group returning 401" | Only 0.11.4 get, info, export; 0.11.4 update 400, delete `200 true`; 0.12.4 404 everywhere | Per-version not-found map |
| 4 | `Merge` policy (story 4) | Server replaces `permissions` whole | Read-modify-write |
| 5 | `Exact` policy (story 5) | Server-known keys come from the defaults endpoint and differ per version | Read at reconcile time |
| 6 | Group update | `name` and `description` required | Always sent |
| 7 | Member count (story 7) | No member route needed | `member_count` on reads |
| 8 | Model `additionalParams` (story 8) | Flat keys of `params`, can collide with named ones | Flatten; CEL rejects collisions |
| 9 | Model tags, knowledge, suggestion prompts | Objects, not strings or ids | Client layer builds the objects |
| 10 | Model `hidden` | `meta.hidden`, while `active` is `is_active` | Field mapping |
| 11 | Base-model warning (story 11) | Base model never rejected; `/api/v1/models/base` cannot back the warning | Computed from `GET /api/models` |
| 12 | Wrong function type (story 10) | Server validates no reference | Provider-side checks |
| 13 | Model update | `meta` and `params` replaced whole | Read-modify-write |
| 14 | Model `active` | 0.11.4 resets `is_active` when omitted | Always sent |
| 15 | Sharing-only difference (story 33) | Object updates carry grants on 0.11.4, ignore them on 0.12.4 | Access endpoints only, every resource |
| 16 | Steady state (story 34) | No-op updates create no version | Confirms the anti-loop test |
| 17 | Model not-found | Update and delete 401, export 403, access update re-creates | Not-found map; no access call without a found object |
| 18 | Model create conflict | Duplicate id answers 401, not 409 | Get before create |
| 19 | `access` block (stories 20–22) | `public` is a grant; invalid grants dropped silently; principals unchecked | Set comparison, read-back; group references resolved, user ids not checked |
| 20 | Function toggles (story 36) | Toggles flip; new functions inactive | Toggle only on difference |
| 21 | Identifier CEL rule (Python identifier) | Server lowercases ids | `^[a-z_][a-z0-9_]*$` for `toolId` and `functionId` |
| 22 | Function type (story 18) | `event` type exists; type re-detected on update | Show the server's type; re-check references |
| 23 | Valves (stories 15, 38) | Written whole, unknown keys dropped, secrets read back in clear | Whole-set writes, spec validation, hash over the read-back, never exposed |
| 24 | Function valves | 401 on a function without `Valves` | Write only when the spec is not `null` |
| 25 | Function not-found | Get 401, update 400, delete `200 true` | Not-found map |
| 26 | Tool and function code (stories 14, 37) | `replace_imports` rewrites stored code | Same rewrite before comparing |
| 27 | Tool syntax errors (story 51) | Not distinguishable by status; detail varies | Truncated detail in the condition |
| 28 | Tool not-found | Get and valves read 404; update, delete and valves write 401 | Not-found map |
| 29 | "find by unique key (… knowledge name)" | Names not unique, may be empty; search is a substring | Exact-name filter; conflict; non-empty name in the CRD |
| 30 | Knowledge delete | Orphans files and vectors; edits referencing models | Limitation + issue; `Usage` ordering |
| 31 | Knowledge update | `description` required | Always sent |
| 32 | Knowledge not-found | Update and delete 400 | Not-found map |
| 33 | Knowledge ingestion | Needs pgvector and an embedding engine on slim | Guides; oikb evaluation |
| 34 | Leading `/` normalized away (story 13) | Server stores commands as sent; `/x` and `x` coexist | Provider normalizes; adoption matches both spellings |
| 35 | "find by unique key (… prompt command)" | Exact, case-sensitive; no lookup by command | Paged search + exact filter; external-name = UUID |
| 36 | Prompt active flag (story 12) | A toggle, active on create; inactive prompts hidden from `GET /` | Toggle only on difference; observe by id or `/list` |
| 37 | Prompt update | `is_production: false` writes only the command and piles up drafts | Never sent false |
| 38 | Prompt identity (command as the unique key) | The command is updatable and the server routes by UUID, so the command cannot serve as a stable key the way story 23's identifier fields do | `command` mutable; identity is the UUID |

## Consequences

### Phase 1 (foundation)

- The `Client` interface (seam S1) carries: per-version not-found and conflict mapping; access writes only through access endpoints; set comparison of grants; the `replace_imports` normalization; whole-set valve writes; toggles that flip. Two profiles, **0.11** and **0.12**, differ in the not-found map, `access_grants` handling in object updates, model `is_active` handling, tool error details and the optional `commit_message`.
- The credential check uses `GET /api/v1/users/default/permissions`; the bootstrap guide documents first-boot settings and the [allow-list](../api-notes/api-key-allowed-endpoints.txt).
- Contract tests (S1) reuse the capture scenarios' operations and fixtures; the S2 fake `Client` is seeded from `test/fixtures/openwebui/`.
- The CRD definitions use the adjusted identifier rules (`^[a-z_][a-z0-9_]*$` for `toolId` and `functionId`, no immutable field for prompts) and reject `additionalParams` collisions and empty knowledge names by CEL.

### Phase 2 (Group, then Model)

- Group: exact-name find with conflict on duplicates; `Merge` as read-modify-write; `Exact` from the defaults endpoint; `member_count` in status.
- Model: field mapping above; read-modify-write of `meta` and `params`; `is_active` always sent; base-model warning from `/api/models`; provider-side reference and type checks; get-before-create.
- The Flux demo exercises a sharing-only change: one access call, no model version on 0.12.4.

### Phase 3 (Prompt, Knowledge, Tool, Function)

- Prompt: UUID identity, command normalization and two-spelling adoption, paged find, toggle on difference.
- Tool and Function: valves whole-set and hash over the read-back, spec validation, toggles on difference, type re-check.
- Knowledge: exact-name find with conflict; orphan limitation and its issue; `Usage` guidance in the GitOps guide.

## Decisions needed

The maintainer validates this ADR, and in particular:

1. **Prompt command spelling**: write commands **without** the leading `/` (the API examples and `prompt.md` fixtures use that form) or **with** it; adoption matches both either way. The Open WebUI UI's own convention was not verified.
2. **Duplicate names or commands at adoption**: treat several exact matches as a conflict (`Synced=False`, as proposed) rather than adopting the oldest.
3. **Optional marker**: leave it out of v1 (proposed), or write `managed_by` where it survives (table above) for operators' visibility.
4. **Knowledge deletion**: document the orphaned files and model edits as a limitation with a `Usage` recommendation (proposed), or have `Delete` refuse while models still reference the base.
5. **Model version label**: set `commit_message` on 0.12.4 model, tool and function writes (optional, profile-only), or not.

## Not verified

- The Open WebUI UI's spelling of prompt commands (decision 1).
- Endpoint restrictions with a non-admin API key, and behavior under `BYPASS_ADMIN_ACCESS_CONTROL=false`: phase 0 used the admin's key only.
- Group hierarchy (0.12.4 `parent_group_id`, inherited permissions and membership): no parent group was created; it is out of scope.
- Knowledge ingestion on 0.11.4 without external services (observed on 0.12.4 only), and oikb against the supported versions.
- The source-only claims marked *(source)* in the API notes.
