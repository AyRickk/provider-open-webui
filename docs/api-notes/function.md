# Function

Python plugins that extend chats: filters, pipes and actions. Observed on **0.11.4** and **0.12.4**. Evidence is cited as `<version>/<scenario>#<step>`, which resolves to step `<step>` of `test/fixtures/openwebui/v<version>/function/<scenario>.json`. Regenerate with `mise run owui:capture function <version>` (scenarios in `hack/capture/scenarios/function.sh`). Claims marked *(source)* come from `open_webui/routers/functions.py`, `open_webui/models/functions.py`, `open_webui/models/function_history.py` and `open_webui/utils/plugin.py` in each image. Fixtures redact secrets by key name (`hack/capture/secrets.jq`): the captured valve is called `api_key` for that reason, and a secret valve under another name would not be redacted.

## Endpoints

All require an admin *(source)*; the admin's API key works on every one of them (all fixture steps use it).

| Operation | Method and path | Request body | Response |
| --- | --- | --- | --- |
| Create | `POST /api/v1/functions/create` | `FunctionForm` | function **without `content`** |
| Get | `GET /api/v1/functions/id/{id}` | | function with `content` |
| List | `GET /api/v1/functions/` | | functions without `content`, no pagination; fixtures sort lists by id, so their order is not evidence |
| List with owners | `GET /api/v1/functions/list` | | same + `user` |
| Export | `GET /api/v1/functions/export?include_valves=true` | | functions with `content` (and `valves`) |
| Update | `POST /api/v1/functions/id/{id}/update` | `FunctionForm` | function |
| Delete | `DELETE /api/v1/functions/id/{id}/delete` | | `true` |
| Toggle active | `POST /api/v1/functions/id/{id}/toggle` | | function |
| Toggle global | `POST /api/v1/functions/id/{id}/toggle/global` | | function |
| Valves | `GET …/id/{id}/valves`, `GET …/valves/spec`, `POST …/valves/update` | valves object | valves |
| User valves | `GET …/valves/user`, `…/valves/user/spec`, `POST …/valves/user/update` | | per-user values, not managed |
| History (0.12.4) | `GET …/id/{id}/history?page=`, `…/history/{history_id}`, `…/history/diff`, `POST …/update/version` | | entries |
| Load from URL, sync | `POST /api/v1/functions/load/url`, `POST /api/v1/functions/sync` | | not used |

There is no lookup by name; the function `id` is the unique key (`*/create#create-duplicate-id`).

## Payloads

`FunctionForm`: `id`, `name`, `content` (the Python code), `meta` (`description`, plus anything else); 0.12.4 adds `commit_message` (history only).

```json
{"id": "capture_filter", "name": "Capture Filter", "content": "\"\"\"\ntitle: Capture Filter\n…\"\"\"\n…class Filter:\n…",
 "meta": {"description": "Captured function"}}
```

Stored function (`*/get#get-capture_filter`):

```json
{"id": "capture_filter", "user_id": "…", "name": "Capture Filter", "type": "filter", "content": "…",
 "meta": {"description": "Captured function",
          "manifest": {"title": "Capture Filter", "description": "Filter used by the capture scenarios", "version": "1.0.0"},
          "toggle": false},
 "is_active": false, "is_global": false, "updated_at": 1700000000, "created_at": 1700000000}
```

0.12.4 adds `version_id` (`0.12.4/get#get-capture_filter`). On 0.11.4, `meta.toggle` is absent unless it is `true` (`0.11.4/get#get-capture_filter`).

## Server-computed fields

| Field | Value | Evidence |
| --- | --- | --- |
| `type` | detected from the code, see [Type detection](#type-detection) | `*/get#get-*` |
| `meta.manifest` | the code's leading docstring frontmatter (`title`, `description`, `version`, …), parsed | `*/get#get-capture_filter` |
| `meta.toggle` | `true` for a filter whose class sets `toggle = True`. Otherwise `false` on 0.12.4 and **absent** on 0.11.4; on 0.11.4 it is also absent from every create response, the toggle filter's included, and appears only on later reads | `*/get#get-capture_toggle_filter`, `0.11.4/get#get-capture_filter`, `0.11.4/create#create-toggle-filter` |
| `id` | lowercased: `Capture_Upper` is stored as `capture_upper` | `*/create#create-uppercase-id` |
| `user_id`, `created_at`, `updated_at` | creator, epoch seconds | `*/create#create-filter` |
| `version_id` (0.12.4) | current history entry | `0.12.4/history#*` |

## Type detection

The server executes the code and looks for a class, in this order: `Pipe` → `pipe`, `Filter` → `filter`, `Action` → `action`, `Event` → `event` *(source: `load_function_module_by_id`, identical on both versions)*. The result is the `type` field on every read (`*/get#get-capture_filter`: `filter`, `#get-capture_pipe`: `pipe`, `#get-capture_action`: `action`).

- **The type is detected again on every update.** Replacing a pipe's code with an action changes `type` to `action` (`*/update#get-after-type-change`), so a code change can turn a function a model references as a filter into something else.
- Code without any of those classes is refused with 400, on create (`*/create#create-no-class`) and on update, where the stored function is left unchanged (`*/update#get-after-no-class`).
- The code runs on the server at create and update; frontmatter `requirements` are `pip install`ed *(source)*.
- The code is stored after `replace_imports`, which rewrites `from utils`, `from apps`, `from main` and `from config` to `from open_webui.…` anywhere in the text *(source)*; observed on tools, see [tool.md](tool.md#code).

## Toggles

Both toggles **flip** the stored value; they take no body and return the function in its new state. Starting from `is_active: false, is_global: false` (`*/toggles#state-initial`):

| Call | `is_active` | `is_global` | Evidence |
| --- | --- | --- | --- |
| toggle | true | false | `*/toggles#state-after-toggle-active-1` |
| toggle again | false | false | `*/toggles#state-after-toggle-active-2` |
| toggle/global | false | true | `*/toggles#state-after-toggle-global-1` |
| toggle/global again | false | false | `*/toggles#state-after-toggle-global-2` |

- **New functions are inactive and not global** (`*/get#get-*`). A function declared active needs one toggle after create.
- **An update keeps both toggles**: changing code and description while both are on leaves them on (`*/toggles#update-while-on` sends the changed code, `#state-after-update-while-on`), and on 0.12.4 writes a new version.
- The response of each toggle is already the new state: each toggle step equals the read that follows it (`*/toggles#toggle-active-1` and `#state-after-toggle-active-1`, through `#toggle-global-on` and `#state-after-toggle-global-on`).

## Valves

Valves are the admin settings declared by the code's `Valves` class (pydantic). They are stored **encrypted** *(source: `encrypt_valves`)* and read back **in clear**.

- `GET …/valves` returns only the values that were set: `{}` before any write (`*/valves#valves-initial`), even though the class has defaults.
- `GET …/valves/spec` returns the class's JSON schema (`*/valves#valves-spec`), or `null` when the code has no `Valves` class (`*/valves#valves-spec-function-without-valves`).
- **A write replaces the stored valves with exactly the keys sent**: writing `{"priority": 7}` after `{"api_key": …, "priority": 5}` drops `api_key` (`*/valves#valves-after-partial`). Unknown keys are dropped without error (`*/valves#valves-after-unknown-key`); a value of the wrong type gives 400 and leaves the stored valves unchanged (`*/valves#valves-after-wrong-type`).
- Secret valve values come back in clear from `GET …/valves` and from `GET …/export?include_valves=true` (redacted in fixtures: `*/valves#valves-after-update`, `#export-with-valves-after-update`).
- Writing valves to a function **without** a `Valves` class answers **401** "We could not find what you're looking for" (`*/valves#valves-update-function-without-valves`).
- Valve writes do not touch `content`, toggles or history *(source)*.

## Version history (0.12.4)

A version holds `name`, `content` and `meta` without the server-computed `manifest`, `toggle` and `has_user_valves` *(source: `function_snapshot`)*. Starting from a fresh function:

| Write | History entries after | Evidence |
| --- | --- | --- |
| create | 1 | `0.12.4/history#history-after-create` |
| update with **the same payload** | **1, no new version** | `0.12.4/history#history-after-no-change` |
| update changing `name` | 2 | `0.12.4/history#history-after-change` |
| toggle | 2 | `0.12.4/history#history-after-toggle` |

0.11.4 has no history: the routes answer the web UI's HTML page (`0.11.4/history#history-after-create`).

## Missing function

| Request on a missing id | 0.11.4 | 0.12.4 |
| --- | --- | --- |
| `GET /id/{id}` | **401** `We could not find what you're looking for :/` | **401** |
| `POST /id/{id}/update` | **400** `[ERROR: Error updating function]` | **400** `Function not found` |
| `DELETE /id/{id}/delete` | **200 `true`** | **200 `true`** |
| valves read, valves write, toggle, toggle/global | **401** | **401** |

Evidence: `*/get-missing#*`, on a function that existed and was deleted (`*/delete#delete`).

## Differences between versions

- 0.12.4 adds version history (`version_id`, history endpoints, `commit_message`).
- `meta.toggle`: always a boolean on 0.12.4; on 0.11.4 present only when `true`, and never in the create response.
- Error text for an update on a missing id or with invalid code: `[ERROR: Error updating function]` on 0.11.4, the underlying message (`Function not found`, `No Function class found in the module`) on 0.12.4 (`*/get-missing#update-deleted`, `*/update#update-no-class`).

## Deviations from spec #3

- **Story 36 is confirmed**: both toggles flip, so a toggle must be called only when the observed value differs from the desired one, and never twice per reconcile.
- **New functions start inactive and not global**: `isActive: true` (or `isGlobal: true`) costs one toggle after create.
- **Identifier format**: the server refuses ids that are not Python identifiers, with 400 "Only alphanumeric characters and underscores are allowed in the id" (a dash: `*/create#create-invalid-id`; a leading digit: `*/create#create-leading-digit-id`), and **lowercases** the rest (`*/create#create-uppercase-id`). The CEL rule must therefore be stricter than the server, a lowercase identifier (`^[a-z_][a-z0-9_]*$`); otherwise the CRD's `functionId` and the stored id differ and external-name never matches.
- **Function type**: besides filter, pipe and action the server knows `event`, and re-detects the type on every update. Status shows whatever the server returns (story 18); the wrong-type check for model references (story 10) must use the observed `type`, re-checked after code changes.
- **Valves are written whole**: a valve write keeps only the keys it carries, so the provider must always send the complete set (plain values overlaid with Secret values, story 15). Unknown keys are dropped silently, so a typo in a valve name is never reported by the server; the provider should validate keys against `GET …/valves/spec`.
- **Valve read-back holds secrets in clear**: comparison by hash (story 38) must hash what `GET …/valves` returns, and nothing from that response may reach status, events or logs (story 57).
- **Valves on a function without a `Valves` class answer 401**, not 400 or 404: the provider must not set valves when the spec endpoint returns `null`.
- **Not-found quirks**: get answers **401**, update answers 400, and delete answers `200 true` even when nothing was deleted, so delete cannot detect not-found.
