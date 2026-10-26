# Access grants

How sharing is encoded for every shareable resource (model, prompt, tool, knowledge). Observed on **0.11.4** and **0.12.4** through models; the other resource pages reference this one and only add what differs. Evidence is cited as for [Group](group.md), here in `test/fixtures/openwebui/v<version>/model/`. Claims marked *(source)* come from `open_webui/models/access_grants.py` and `open_webui/utils/access_control/__init__.py`.

## Encoding

A resource's sharing is a list of grants, each one principal and one permission:

```json
{"principal_type": "group", "principal_id": "<group id>", "permission": "read"}
```

| Spec #3 `access` field | Grant |
| --- | --- |
| `public: true` | `{"principal_type": "user", "principal_id": "*", "permission": "read"}` |
| group read | `{"principal_type": "group", "principal_id": "<group id>", "permission": "read"}` |
| group write | `{"principal_type": "group", "principal_id": "<group id>", "permission": "write"}` |
| user read | `{"principal_type": "user", "principal_id": "<user id>", "permission": "read"}` |
| user write | `{"principal_type": "user", "principal_id": "<user id>", "permission": "write"}` |
| `access: {}` (private) | `[]` |

Write does not imply read: a principal with both has two grants (`*/create-with-access-grants#get`). A public-write grant (`user:*`, `write`) also exists *(source)*; spec #3 has no field for it.

Grants come back with server fields added: `id` (UUID), `resource_type`, `resource_id`, `created_at` (`*/create-with-access-grants#get`). Comparison must use only `(principal_type, principal_id, permission)` and ignore order.

## Writing grants

- **Create** accepts `access_grants` in the create body and stores them (`*/create-with-access-grants#create`).
- **The access endpoint replaces the whole list**: `POST /api/v1/models/model/access/update` with `{"id": "…", "access_grants": [...]}`. Sending one group grant leaves exactly that grant (`*/access-update#get-after-update-access`); sending `[]` makes the resource private (`*/access-update#get-after-make-private`).
- **The object update and grants differ per version.** On 0.11.4, `access_grants` in the model update body replace the grants (`0.11.4/access-update#get-after-update-with-grants`: they become `user:*:read`). On 0.12.4 they are ignored (`0.12.4/access-update#get-after-update-with-grants`: unchanged). The provider must never send `access_grants` in an object update, and write access only through the access endpoint.

## Silently dropped grants

The server drops invalid grants **without an error**; the create still answers 200 (`*/create-with-access-grants#create`, `#get`). Of eight grants sent, five were stored. Dropped:

- `{"principal_type": "anyone", "principal_id": "*", "permission": "read"}`: the `anyone` audience exists, but only for shared chats; it is stripped for models even for an admin *(source: `_SHARING_PERMISSION_KEYS`)*.
- `{"principal_type": "bogus", …}`: unknown principal type.
- `{"principal_type": "group", …, "permission": "admin"}`: unknown permission.

Duplicates by `(principal_type, principal_id, permission)` collapse into one *(source: `normalize_access_grants`)*. For non-admin callers the server also drops public, user and group grants their permissions do not allow (`sharing.public_*`, `access_grants.allow_users`, `access_grants.allow_groups`) *(source)*; the provider's admin account is not filtered.

Principal ids are **not** checked for existence: a grant to `missing-group-id` is stored like any other (`*/access-update#get-after-grant-missing-group`). The provider must resolve group and user references itself before writing.

## Deviations from spec #3

- `public` is not a flag: it is the `user:*` read grant inside the same list.
- A create can silently store fewer grants than sent. The provider must read the grants back after a write and report a difference instead of looping on it.
- On 0.11.4 the object update rewrites grants when the field is present; the provider must leave `access_grants` out of every object update.
