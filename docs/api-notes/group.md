# Group

Observed on **0.11.4** and **0.12.4**. Evidence is cited as `<version>/<scenario>#<step>`, which resolves to step `<step>` of `test/fixtures/openwebui/v<version>/group/<scenario>.json`. Regenerate with `mise run owui:capture group <version>` (scenarios in `hack/capture/scenarios/group.sh`). In fixtures, UUIDs are numbered by first appearance (`…-000000000001`) and `*_at` timestamps read `1700000000`. Claims marked *(source)* come from reading `open_webui/routers/groups.py` and `open_webui/models/groups.py` in each image, not from a fixture.

## Endpoints

All require an admin except list and info, which accept any verified user; a non-admin only lists the groups they belong to *(source)*. The admin's API key works on every one of them (all fixture steps use it).

| Operation | Method and path | Request body | Response |
| --- | --- | --- | --- |
| Create | `POST /api/v1/groups/create` | `GroupForm` | `GroupResponse` |
| Get | `GET /api/v1/groups/id/{id}` | | `GroupResponse` |
| Get (light) | `GET /api/v1/groups/id/{id}/info` | | `GroupResponse` without `permissions`, `data`, `meta` (`*/get#get-info`) |
| List | `GET /api/v1/groups/` | | `GroupResponse[]`, no pagination |
| Update | `POST /api/v1/groups/id/{id}/update` | `GroupUpdateForm` | `GroupResponse` |
| Delete | `DELETE /api/v1/groups/id/{id}/delete` | | `true` |
| Export (with members) | `GET /api/v1/groups/id/{id}/export` | | `GroupResponse` + `user_ids` |
| Members | `POST /api/v1/groups/id/{id}/users` (both), `GET /api/v1/groups/id/{id}/members` (0.12.4 only) | | users |
| Preview | `GET /api/v1/groups/id/{id}/preview` | | permissions: own, inherited, effective |
| Known permission keys | `GET /api/v1/users/default/permissions` | | category → key → bool |

There is **no lookup by name**: find-by-name lists all groups and filters client-side (`*/find-by-name#list-after-duplicate`).

## Payloads

`GroupForm` and `GroupUpdateForm`: `name` and `description` are **required on update too** (both strings); `permissions` and `data` are optional objects. 0.12.4 adds an optional `parent_group_id`.

```json
{"name": "capture-engineering", "description": "Engineering",
 "permissions": {"workspace": {"models": true, "tools": true}, "chat": {"file_upload": false}}}
```

`GroupResponse` (0.12.4, `0.12.4/get#get`):

```json
{"parent_group_id": null, "id": "…", "user_id": "…", "name": "capture-engineering", "description": "Engineering",
 "data": {"config": {"share": "members"}}, "meta": null,
 "permissions": {"workspace": {"models": true, "tools": true}, "chat": {"file_upload": false}},
 "created_at": 1700000000, "updated_at": 1700000000, "member_count": 0}
```

0.11.4 returns the same object without `parent_group_id` (`0.11.4/get#get`).

## Server-computed fields

| Field | Value | Evidence |
| --- | --- | --- |
| `id` | UUID assigned on create (`uuid4` *(source)*) | `*/create#create-minimal` |
| `user_id` | id of the admin that created the group (the API key owner) | `*/create#create-minimal` |
| `created_at`, `updated_at` | epoch seconds; `updated_at` is bumped by **every** update, even one that changes nothing *(source: `update_group_by_id`)* | normalized in fixtures |
| `member_count` | number of members, present on get, info, list and export | `*/members#get` (`1`) |
| `data.config.share` | `"members"`, injected on create when absent | `*/create#create-minimal` |

`meta` is `null` and `permissions` is `null` when not sent (`*/create#create-minimal`).

## Behavior

- **Names are not unique.** Creating a second group with an existing name succeeds and both are listed (`*/find-by-name#create-duplicate-name`, `#list-after-duplicate`).
- **`permissions` is replaced, never merged.** Updating with `{"workspace": {"models": false}}` over `{"workspace": {"models": true, "tools": true}, "chat": {"file_upload": false}}` stores exactly `{"workspace": {"models": false}}` (`*/update-permissions#get-after-update`). Omitting `permissions` keeps the stored value (`*/update-description#get-after-update`).
- **Unknown keys and categories are stored verbatim** and returned as sent (`*/update-unknown-permission-keys#get-after-update`).
- **Full permission set.** `GET /api/v1/users/default/permissions` returns every key the server knows, as category → key → bool, in 6 categories (`workspace`, `sharing`, `access_grants`, `chat`, `features`, `settings`). `GET …/default/permissions/defaults` returns the same map on a fresh instance. **The set grows between versions**: 65 keys on 0.11.4, 67 on 0.12.4, which adds `features.encrypted_channels` and `workspace.models_hidden`; the 65 shared keys have the same default values (`*/permission-set#default-permissions`). A group's stored permissions only hold what was written to it. On 0.12.4, `preview` also returns `inherited_permissions` and `effective_permissions` (`0.12.4/permission-set#preview`; built from the defaults, then the ancestors, then the group's own *(source)*, not observable without a parent group); on 0.11.4 it returns only the group's own `permissions` (`0.11.4/permission-set#preview`).
- **Members.** `member_count` is part of every read, so status needs no member call. The member list is read through `export` (`user_ids`, `*/members#export`) or `POST …/users` (user objects, `*/members#users`) on both versions, and `GET …/members` on 0.12.4 (`0.12.4/members#members`, with `membership_type`; pages of 30 *(source)*).
- **List order** is `updated_at` descending with unspecified ties; fixtures sort lists by id, so their order is not evidence.

## Missing group

| Request on a missing id | 0.11.4 | 0.12.4 |
| --- | --- | --- |
| `GET /id/{id}` | **401** `We could not find what you're looking for :/` | 404, same message |
| `GET /id/{id}/info` | **401** | 404 |
| `GET /id/{id}/export` | **401** | 404 |
| `GET /id/{id}/preview` | 404 | 404 |
| `GET /id/{id}/members` | route absent: `200` with the web UI's HTML page | 404 `Group not found.` |
| `POST /id/{id}/update` | **400** `[ERROR: Error updating group]` | 404 `Group not found.` |
| `DELETE /id/{id}/delete` | **200 `true`** | 404 `Group not found.` |

Evidence: `*/get-missing#*-deleted*` (a group that existed and was deleted) and `*/get-missing#*-never-existed*` (an id that never existed). Both give the same result on every endpoint.

## Differences between versions

- 0.12.4 adds group hierarchy: `parent_group_id` in forms and responses (`0.12.4/get#get`), `GET …/members` with `membership_type` (`0.12.4/members#members`), and inherited membership and permissions *(source; the scenarios create no parent group)*.
- 0.12.4 knows two more permission keys: `features.encrypted_channels` and `workspace.models_hidden`.
- Not-found handling, see the table above: 0.11.4 answers 401, 400 or `200 true` where 0.12.4 answers 404.

## Deviations from spec #3

- **"find by unique key (group name …)"**: group names are not unique. Find-by-name can return several groups; the provider must treat more than one match as a conflict, never pick one.
- **"a missing group returning 401"**: only on 0.11.4, and only for get, info and export. On 0.11.4 update answers 400 and delete answers `200 true` for a missing group, so delete cannot detect not-found there. 0.12.4 answers 404 everywhere. The quirk mapping must be per version as well as per endpoint.
- **`Merge` policy (user story 4)**: the server replaces `permissions` as a whole, so "writes only the keys I declare" has to be a read-modify-write: read the stored map, overlay the declared keys, send the full map.
- **`Exact` policy (user story 5)**: "every server-known key" is the set from `GET /api/v1/users/default/permissions`, not the group's stored map, which only holds what was written to it. That set differs per version, so it must be read from the server, never hardcoded.
- **Update sends `name` and `description` every time**: both are required by `GroupUpdateForm`, including for a permissions-only change.
- **Spurious writes**: every update bumps `updated_at`, which confirms the spec's rule of writing only on a real difference.
- **Member count (user story 7)** needs no member-read endpoint: `member_count` is in every group read. The member endpoints are listed above but the provider does not need them.
