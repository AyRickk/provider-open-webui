# Prompt

Shared slash-prompts: a command, a name, content and tags, plus an active flag and sharing. Observed on **0.11.4** and **0.12.4**. Evidence is cited as `<version>/<scenario>#<step>`, which resolves to step `<step>` of `test/fixtures/openwebui/v<version>/prompt/<scenario>.json`. Regenerate with `mise run owui:capture prompt <version>` (scenarios in `hack/capture/scenarios/prompt.sh`). Claims marked *(source)* come from `open_webui/routers/prompts.py` and `open_webui/models/prompts.py` in each image. Sharing is described in [access-grants.md](access-grants.md).

## Endpoints

The admin's API key works on every one of them (all fixture steps use it); non-admins need `workspace.prompts` and write access *(source)*. The routes are the same on both versions (`docs/api-notes/openapi/`).

| Operation | Method and path | Request body | Response |
| --- | --- | --- | --- |
| Create | `POST /api/v1/prompts/create` | `PromptForm` | prompt |
| Get | `GET /api/v1/prompts/id/{prompt_id}` | | prompt + `write_access`, `user` |
| List active | `GET /api/v1/prompts/` | | prompts, **active only**, no pagination (`*/active#list-while-inactive`) |
| List / search | `GET /api/v1/prompts/list?query=&page=` | | `{items, total}`, active and inactive, 30 per page *(source)* (`*/list#list-paginated`) |
| Update | `POST /api/v1/prompts/id/{prompt_id}/update` | `PromptForm` | prompt |
| Toggle active | `POST /api/v1/prompts/id/{prompt_id}/toggle` | | prompt |
| Update access | `POST /api/v1/prompts/id/{prompt_id}/access/update` | `{"access_grants": [...]}` | prompt |
| Delete | `DELETE /api/v1/prompts/id/{prompt_id}/delete` | | `true` |
| History | `GET …/history?page=` (`*/history#history-after-create`); `…/history/{history_id}`, `…/history/diff`, `POST …/update/version` *(source)* | | entries, both versions |
| Tags | `GET /api/v1/prompts/tags` | | tag names (`*/list#tags`) |

**Routing is by UUID only.** Every `/id/{prompt_id}` route takes the server-assigned `id`; a command in that position is not found (`*/get#get-by-command`: 404). There is no lookup by command, see [Behavior](#behavior).

## Payloads

`PromptForm`: `command`, `name` and `content` are **required on update too**; `tags` (list of strings), `data`, `meta`, `access_grants`, `commit_message`, `version_id` are optional, and `is_production` defaults to `true` *(source: OpenAPI snapshots)*. There is no `is_active` field: the active flag is only changed by the toggle.

```json
{"command": "capture-summary", "name": "Capture Summary", "content": "Summarize: {{CLIPBOARD}}", "tags": ["capture"]}
```

Stored prompt (`*/get#get-by-id`):

```json
{"id": "…", "command": "capture-summary", "user_id": "…", "name": "Capture Summary", "content": "Summarize: {{CLIPBOARD}}",
 "data": {}, "meta": {}, "tags": ["capture"], "is_active": true, "version_id": "…", "access_grants": [],
 "write_access": true, "user": null, "created_at": 1700000000, "updated_at": 1700000000}
```

## Server-computed fields

| Field | Value | Evidence |
| --- | --- | --- |
| `id` | UUID assigned on create; the only routing key | `*/create#create`, `*/get#get-by-command` |
| `user_id` | creating admin (API key owner) | `*/create#create` |
| `version_id` | current history entry, on both versions | `*/history#*` |
| `is_active` | `true` on create | `*/create#create` |
| `data`, `meta` | `{}` when not sent | `*/get#get-by-id` |
| `created_at`, `updated_at` | epoch seconds | normalized in fixtures |

## Commands

**Stored exactly as sent.** A leading `/` is kept (`*/create#create-leading-slash`: `"/capture-slashed"`), and the case is kept (`*/create#create-command-other-case`: `"Capture-Summary"`).

**Unique by exact string, on create and on update.** Creating or renaming to a command that exists gives 400 `Uh-oh! This command is already registered. Please choose another command string.` (`*/create#create-duplicate-command`, `*/update#update-command-to-taken`, after which the stored command is unchanged: `#get-after-command-to-taken`). Nothing else is compared, so these all coexist:

| Command | Coexists with | Evidence |
| --- | --- | --- |
| `/capture-summary` | `capture-summary` | `*/create#create-duplicate-command-with-slash` |
| `capture-slashed` | `/capture-slashed` | `*/create#create-slashed-command-without-slash` |
| `Capture-Summary` | `capture-summary` | `*/create#create-command-other-case` |

Names are not unique (`*/create#create-duplicate-name`). A deleted prompt's command can be used again (`*/delete#create-reusing-deleted-command`). The command can be changed by an update (`*/update#get-after-command`); the `id` stays.

## Behavior

- **No lookup by command.** `GET /list?query=` matches a case-insensitive substring of the name, the command, the **content**, or the owner's name or email *(source: `search_prompts`)*: `capture-summary` returns `capture-summary`, `/capture-summary` and `Capture-Summary` (`*/find-by-command#query-command`), `capture-s` returns six (`#query-command-prefix`). Find-by-command must page through the results and filter on the exact command.
- **Inactive prompts vanish from `GET /`** but stay in `GET /list` with `is_active: false` and in get by id (`*/active#list-while-inactive`, `#list-paginated-while-inactive`, `#state-after-toggle-1`). Observe must not rely on `GET /`.
- **Toggle flips** `is_active` and returns the new state (`*/active#toggle-1`: `false`, `#toggle-2`: `true`); an update keeps it (`*/active#state-after-update-while-inactive`: still `false`, content changed).
- **Update replaces** `name`, `content` and `tags` (`*/update#get-after-name`, `#get-after-content`, `#get-after-tags`), keeps `tags` when the field is omitted and `data` / `meta` when empty *(source)*. With `is_production: false` only the command is written: a new command is stored while the new name and content are not, and `version_id` stays (`*/update#get-after-not-production`); a history entry holding the draft is still recorded, but not made current (`*/update#history-before-not-production`: 5 entries, `#history-after-not-production`: 6, the new one with the draft name and content).
- **Access**: grants follow [access-grants.md](access-grants.md). A create with grants stores them (`*/get#get-shared`); the access endpoint replaces the list (`*/access-update#get-after-update-access`), `[]` makes the prompt private (`#get-after-make-private`). `access_grants` in the **update** body are applied on 0.11.4 (`0.11.4/access-update#get-after-update-with-grants`: `user:*:read`) and ignored on 0.12.4 (`0.12.4/access-update#get-after-update-with-grants`), as for the other resources.

### Version history (both versions)

| Write | History entries after | Evidence |
| --- | --- | --- |
| create | 1 | `*/history#history-after-create` |
| update with **the same payload** | **1, no new version** | `*/history#history-after-no-change` |
| update changing `content` | 2 | `*/history#history-after-change` |
| toggle | 2 | `*/history#history-after-toggle` |
| access update granting a group read | 2 | `*/history#history-after-access` |

A version is written only when `name`, `command`, `content` or `tags` change *(source: `update_prompt_by_id`)*. Unlike models, functions and tools, prompts already have history on 0.11.4.

## Missing prompt

Every route answers **404** `We could not find what you're looking for :/` on both versions: get, update, delete, toggle, and the access endpoint, which creates nothing (`*/get-missing#*`, `#get-after-update-access-deleted`), on a prompt that existed and was deleted (`*/delete#delete`).

## Differences between versions

- `access_grants` in the update body: applied on 0.11.4, ignored on 0.12.4.

Everything else observed is identical: routes, payloads, command rules, toggle, history and error codes.

## Deviations from spec #3

- **"a leading `/` in a prompt command to be normalized away" (story 13)**: the server does no normalization; `/x` and `x` are different, coexisting commands. The provider must normalize the command itself (strip one leading `/`) before writing and comparing, and find-by-command / adoption must look for both spellings, since prompts created in the UI may carry either.
- **"find by unique key (… prompt command)"**: the command is unique only as an exact, case-sensitive string, and there is no lookup by command; find-by-command pages through `GET /list?query=` and filters on the exact command. The external-name is the UUID `id`, not the command (routing is by UUID only).
- **The active flag is a toggle** (story 12's "active flag"): no field sets it; the toggle flips it, so it must be called only when the observed value differs, as for functions (story 36). New prompts start active.
- **Inactive prompts disappear from `GET /api/v1/prompts/`**: the provider must observe through get by id or `GET /list`.
- **Update always sends `command`, `name` and `content`**, and `is_production: true` (or omits it): with `false`, name and content are not written and a draft version piles up in the history.
- **The command is not an immutable identifier**: an update can change it and the `id` stays (`*/update#get-after-command`), whereas spec #3 makes identifier fields `+immutable`. For prompts the identifier is the server-assigned `id` (external-name); `command` is an ordinary, updatable field whose uniqueness is checked on update too, so renaming to a taken command fails with 400 (`*/update#update-command-to-taken`).
- **Not-found is a clean 404 everywhere**: no quirk mapping needed for prompts.
