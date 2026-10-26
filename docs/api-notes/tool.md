# Tool

Python toolkits whose methods the model can call. Observed on **0.11.4** and **0.12.4**. Evidence is cited as `<version>/<scenario>#<step>`, which resolves to step `<step>` of `test/fixtures/openwebui/v<version>/tool/<scenario>.json`. Regenerate with `mise run owui:capture tool <version>` (scenarios in `hack/capture/scenarios/tool.sh`). Claims marked *(source)* come from `open_webui/routers/tools.py` and `open_webui/utils/plugin.py` in each image. Fixtures redact secrets by key name (`hack/capture/secrets.jq`), which is why the captured secret valve is called `api_key`. Sharing is described in [access-grants.md](access-grants.md); code, valves and ids behave much like [functions](function.md).

## Endpoints

The admin's API key works on every one of them (all fixture steps use it); non-admins need the `workspace.tools` permission and access grants *(source)*.

| Operation | Method and path | Request body | Response |
| --- | --- | --- | --- |
| Create | `POST /api/v1/tools/create` | `ToolForm` | tool without `content` and `specs` |
| Get | `GET /api/v1/tools/id/{id}` | | tool with `content`, `specs`, `access_grants`, `write_access`, `user` |
| List | `GET /api/v1/tools/` | | tools with `content`, `specs`, `user` and a top-level `has_user_valves` (`*/list#list`) |
| List with write access | `GET /api/v1/tools/list` | | tools with `content`, `specs`, `user`, `write_access` (`*/list#list-with-users`) |
| Export | `GET /api/v1/tools/export` | | tools with `content`, **without valves**, even with `include_valves=true` (`*/valves#export-include-valves-after-valves-update`) |
| Update | `POST /api/v1/tools/id/{id}/update` | `ToolForm` | tool |
| Update access | `POST /api/v1/tools/id/{id}/access/update` | `{"access_grants": [...]}` | tool |
| Delete | `DELETE /api/v1/tools/id/{id}/delete` | | `true` |
| Valves | `GET …/id/{id}/valves`, `GET …/valves/spec`, `POST …/valves/update` | valves object | valves |
| User valves | `GET …/valves/user`, `…/valves/user/spec`, `POST …/valves/user/update` | | per-user values, not managed |
| Specs (0.12.4) | `GET …/id/{id}/specs` | | the `specs` list *(source)* |
| History (0.12.4) | `GET …/id/{id}/history`, `…/history/{history_id}`, `…/history/diff`, `POST …/update/version` | | entries |
| Load from URL | `POST /api/v1/tools/load/url` | | not used |

Lists are not paginated, and fixtures sort them by id, so their order is not evidence. There is no lookup by name (no such route in `docs/api-notes/openapi/`); the tool `id` is the unique key (`*/create#create-duplicate-id`).

## Payloads

`ToolForm`: `id`, `name`, `content` (Python code defining `class Tools`), `meta` (`description`, …), `access_grants`; 0.12.4 adds `commit_message`.

Stored tool (`*/get#get`, abridged):

```json
{"id": "capture_tool", "user_id": "…", "name": "Capture Tool", "content": "…",
 "specs": [{"name": "get_capture_value", "description": "Return a fixed value for a name.",
            "parameters": {"type": "object", "properties": {"name": {"type": "string", "description": "The name to look up."}},
                           "required": ["name"]}}],
 "meta": {"i18n": null, "description": "Captured tool",
          "manifest": {"title": "Capture Tool", "description": "Toolkit used by the capture scenarios", "version": "1.0.0"},
          "has_user_valves": true},
 "access_grants": [], "updated_at": 1700000000, "created_at": 1700000000}
```

0.12.4 adds `version_id` (`0.12.4/get#get`).

## Server-computed fields

| Field | Value | Evidence |
| --- | --- | --- |
| `specs` | a function spec per method of `Tools`, from its signature and docstring (one method observed; one spec per method *(source: `get_local_tool_specs`)*) | `*/get#get` |
| `meta.manifest` | the leading docstring frontmatter, parsed | `*/get#get` |
| `meta.has_user_valves` | whether the code defines `UserValves` (only `true` observed; `false` otherwise *(source)*) | `*/get#get` |
| `meta.i18n` | `null` | `*/get#get` |
| `id` | lowercased (`Capture_Upper` → `capture_upper`) | `*/create#create-uppercase-id` |
| `user_id`, `created_at`, `updated_at` | creator, epoch seconds | `*/create#create` |
| `version_id` (0.12.4) | current history entry | `0.12.4/history#*` |

## Ids

Only Python identifiers are accepted, then lowercased, exactly as for functions *(source: same check in `create_new_tools`)*:

| Id sent | Result | Evidence |
| --- | --- | --- |
| `Capture_Upper` | stored as `capture_upper` | `*/create#create-uppercase-id` |
| `capture-dash` | 400 `Only alphanumeric characters and underscores are allowed in the id` | `*/create#create-invalid-id` |
| `1capture` | 400, same message | `*/create#create-leading-digit-id` |
| an existing id | 400 `Uh-oh! This id is already registered…` | `*/create#create-duplicate-id` |

## Code

The server executes the code on create and update and requires a `class Tools` *(source: `load_tool_module_by_id`)*.

**Stored as sent, with one exception.** CRLF line endings, a missing trailing newline and trailing spaces are kept byte for byte (`*/code-normalization#get-crlf`, `#get-no-trailing-newline`, `#get-trailing-spaces`). But before storing, the server **rewrites four import prefixes anywhere in the text, comments and strings included**: `from utils` → `from open_webui.utils`, `from apps` → `from open_webui.apps`, `from main` → `from open_webui.main`, `from config` → `from open_webui.config` *(source: `replace_imports`)*. A comment `# from utils import nothing` is stored as `# from open_webui.utils import nothing`, on create (`*/code-normalization#get-import-rewrite`) and on update (`*/code-normalization#get-after-update-import-rewrite`). Functions go through the same `replace_imports` on create and update *(source: `routers/functions.py`)*.

**Invalid code** is refused with 400 and the stored tool is left unchanged (`*/update#get-after-syntax-error`). The error shape:

| Case | 0.11.4 | 0.12.4 |
| --- | --- | --- |
| create with a syntax error | `{"detail": "[ERROR: Error creating tool]"}` | same |
| create without `class Tools` | `{"detail": "[ERROR: Error creating tool]"}` | same |
| update with a syntax error | `{"detail": "[ERROR: Error updating tool]"}` | `{"detail": "expected ':' (<string>, line 2)"}`, Python's message with the line |

Evidence: `*/create#create-syntax-error`, `*/create#create-no-class`, `*/update#update-syntax-error`. No status or body field separates a syntax error from any other create failure.

## Valves

Same model as functions: the code's `Tools.Valves` class defines them, `GET …/valves/spec` returns its JSON schema (`*/valves#valves-spec`), and `GET …/valves/user/spec` the `UserValves` schema (`*/valves#user-valves-spec`).

- `GET …/valves` returns only the values that were set: `{}` before any write (`*/valves#valves-initial`).
- **Values come back in clear**, secrets included. Fixtures redact `api_key`, so the capture checks it instead: `*/valves#valves-read-back-in-clear` records that `GET …/valves` returned the written secret exactly, not a mask. The export never includes valves, even with `include_valves=true` (`*/valves#export-include-valves-after-valves-update`), whereas the function export does with that parameter.
- **A write replaces the stored valves with exactly the keys sent**: `{"priority": 7}` after `{"api_key": …, "priority": 5}` drops `api_key` (`*/valves#valves-after-partial`). Unknown keys are dropped without error (`*/valves#valves-after-unknown-key`); a wrong type gives 400 and leaves the valves unchanged (`*/valves#valves-after-wrong-type`).
- A valve write adds no history version (`0.12.4/history#history-after-valves`).

## Access

Grants follow [access-grants.md](access-grants.md). A create with grants stores them (`*/get#get-shared`: `group:read`, `user:*:read`); the access endpoint replaces the list (`*/access-update#get-after-update-access`), `[]` makes the tool private (`*/access-update#get-after-make-private`).

`access_grants` in the **update** body: applied on 0.11.4 (`0.11.4/access-update#get-after-update-with-grants`: grants become `user:*:read`), ignored on 0.12.4 (`0.12.4/access-update#get-after-update-with-grants`: unchanged), as for models.

The tool access endpoint takes the id in the path, and on a missing tool answers 404 **without** re-creating it (`*/get-missing#update-access-deleted`, `#get-after-update-access-deleted`), unlike the model one.

## Version history (0.12.4)

| Write | History entries after | Evidence |
| --- | --- | --- |
| create | 1 | `0.12.4/history#history-after-create` |
| update with **the same payload** | **1, no new version** | `0.12.4/history#history-after-no-change` |
| update changing `name` | 2 | `0.12.4/history#history-after-change` |
| valves write | 2 | `0.12.4/history#history-after-valves` |
| access update granting a group read | 2 | `0.12.4/history#history-after-access` |

0.11.4 has no history: the routes answer **200** with the web UI's HTML page (`content_type: text/html`, `0.11.4/history#history-after-create`), so a client checking only the status would be misled.

## Missing tool

| Request on a missing id | 0.11.4 | 0.12.4 |
| --- | --- | --- |
| `GET /id/{id}` | **404** | **404** |
| `POST /id/{id}/update` | **401** | **401** |
| `DELETE /id/{id}/delete` | **401** | **401** |
| `GET /id/{id}/valves` | 404 | 404 |
| `POST /id/{id}/valves/update` | **401** | **401** |
| `POST /id/{id}/access/update` | 404, nothing created | 404, nothing created |

All with `We could not find what you're looking for :/`. Evidence: `*/get-missing#*`, on a tool that existed and was deleted (`*/delete#delete`).

## Differences between versions

- 0.12.4 adds version history and `GET …/specs`.
- `access_grants` in the update body: applied on 0.11.4, ignored on 0.12.4.
- Update with invalid code: generic `[ERROR: Error updating tool]` on 0.11.4, Python's message with the line on 0.12.4.

## Deviations from spec #3

- **Code comparison must apply `replace_imports`** (stories 14 and 37): the stored code differs from the sent code whenever it contains `from utils`, `from apps`, `from main` or `from config`, even in a comment. Comparing the raw desired code with the stored one would update forever; the provider must apply the same four replacements to the desired code before comparing. Line endings and trailing whitespace are not touched, so no other normalization is needed. The same holds for functions.
- **Valves are written whole and read back in clear** (stories 15 and 38): every valve write must carry the complete set (plain values overlaid with Secret values), and the hash comparison must hash what `GET …/valves` returns; that response holds the secrets in clear, so nothing from it may reach status, events or logs (story 57). Unknown valve keys are dropped silently, so keys should be validated against `GET …/valves/spec`.
- **Identifier format**: same as functions: the CEL rule for `toolId` must be stricter than the server, a lowercase Python identifier (`^[a-z_][a-z0-9_]*$`), or external-name never matches.
- **Syntax errors are not distinguishable by status**: a bad toolkit gives 400 with either a generic detail or Python's message depending on version and operation. The condition message should carry the truncated detail (story 51) and may include a code line number on 0.12.4.
- **Sharing goes only through the access endpoint**, for the same reason as models: on 0.11.4 an update carrying `access_grants` rewrites them.
- **Not-found quirks**: get and valves read answer 404, update, delete and valves write answer **401**.
