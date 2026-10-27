# Open WebUI API notes

Observed behavior of the Open WebUI API on the supported versions. Observed behavior takes precedence over the spec (issue #3); differences are listed under each section and consolidated in ADR-0001 (once it exists).

- Versions observed: **0.11.4** and **0.12.4**, `ghcr.io/open-webui/open-webui:v<version>-slim`, SQLite, `ENV=dev`.
- OpenAPI snapshots: [`openapi/v0.11.4.json`](openapi/v0.11.4.json) (485 paths) and [`openapi/v0.12.4.json`](openapi/v0.12.4.json) (533 paths), keys sorted. Regenerate with `mise run owui:openapi <version>` against a running instance.
- Throwaway instances: `mise run owui:up <version>` / `mise run owui:down <version>`, see `hack/openwebui/`.

- Fixtures: `test/fixtures/openwebui/v<version>/<resource>/<scenario>.json`, captured with `mise run owui:capture <resource> <version>` (see `hack/capture/`). Re-running a capture regenerates byte-identical fixtures; check with `git diff --exit-code -- test/fixtures`. A step holding an `observation` records something seen outside the API (database rows, files on disk). A step holding a `check` instead of a request and response is a fact the scenario verified at capture time but cannot show without leaking it (e.g. that a secret reads back unmasked).

## Resources

One page per resource, each with the same sections: endpoints, payloads, server-computed fields, behavior, missing object, differences between versions, deviations from spec #3. [`group.md`](group.md) is the template.

- [Group](group.md)
- [Model](model.md)
- [Function](function.md)
- [Tool](tool.md)
- [Knowledge](knowledge.md)
- [Prompt](prompt.md)
- [Access grants](access-grants.md): sharing for every shareable resource (model, prompt, tool, knowledge)

## Cross-cutting

### Version endpoint

`GET /api/version` needs no authentication on both versions and returns `200`:

```json
{"version":"0.12.4","deployment_id":""}
```

`version` is the release without a `v` prefix. `deployment_id` is empty unless configured.

### Authentication

API keys look like `sk-` followed by 32 hex characters. On both versions a key is accepted in either header:

- `Authorization: Bearer sk-…`
- `x-api-key: sk-…`, the default of `CUSTOM_API_KEY_HEADER`, which renames this header.

A missing credential gives `401`; an invalid key gives `401` with `{"detail":"Your session has expired or the token is invalid. Please sign in again."}`.

Admin endpoints accept the key on both versions (observed with `GET /api/v1/groups/`, `200`).

**0.12.4 only:** routes that depend on `get_human_user` refuse API keys with `403 {"detail":"A human session is required."}`. They are `GET /api/v1/auths/` (the current session user) and the MFA routes. On 0.11.4, `GET /api/v1/auths/` returns the key owner. The provider must therefore not use `GET /api/v1/auths/` to check its credentials; `GET /api/version` (no auth) plus an authenticated read of the resource being reconciled works on both versions.

### Bootstrap environment variables

The names are identical on both versions (`open_webui/env.py`, `open_webui/config.py`):

| Variable | Default | Observed behavior |
| --- | --- | --- |
| `WEBUI_ADMIN_EMAIL` | empty | With `WEBUI_ADMIN_PASSWORD`, creates an admin at startup when no user exists, then disables signup (`enable_signup: false` in `/api/config`). |
| `WEBUI_ADMIN_PASSWORD` | empty | See above. Both must be set, otherwise nothing is created. |
| `WEBUI_ADMIN_NAME` | `Admin` | Display name of that admin. |
| `ENABLE_API_KEYS` | `False` | Case-insensitive `true` enables API keys. Without it, `POST /api/v1/auths/api_key` gives `403 {"detail":"API key creation is not allowed in the environment."}`. |
| `ENABLE_API_KEYS_ENDPOINT_RESTRICTIONS` | `False` | Legacy alias `ENABLE_API_KEY_ENDPOINT_RESTRICTIONS`. When `true`, an API key only reaches the paths in `API_KEYS_ALLOWED_ENDPOINTS`; see below. |
| `API_KEYS_ALLOWED_ENDPOINTS` | empty | Legacy alias `API_KEY_ALLOWED_ENDPOINTS`. Comma-separated paths. |
| `CUSTOM_API_KEY_HEADER` | `x-api-key` | Name of the alternative API-key header. It **replaces** `x-api-key`: with `X-Provider-Key`, that header works and `x-api-key` gives `401 {"detail":"Not authenticated"}`. `Authorization: Bearer` keeps working. |

Endpoint restrictions, observed on both versions with `ENABLE_API_KEYS_ENDPOINT_RESTRICTIONS=true` and `API_KEYS_ALLOWED_ENDPOINTS=/api/v1/groups/,/api/v1/prompts/` on first boot:

| Request with the API key | Result |
| --- | --- |
| `GET /api/v1/groups/`, `GET /api/v1/prompts/` | `200` |
| `GET /api/v1/tools/` | `403 {"detail":"You do not have permission to access this resource. …"}` |
| `GET /api/version` | `200` (public route) |

Paths that match no API route, such as `GET /api/v1/models/` with the trailing slash, return `200` with the web UI's HTML page rather than `404`; a client must check the content type.

Which paths the provider needs, and whether entries match as prefixes, is settled per resource by the spike tickets.

The three `*API_KEYS*` settings are **persistent config** (`auth.enable_api_keys`, `auth.api_key.endpoint_restrictions`, `auth.api_key.allowed_endpoints`): the environment only seeds them on the first boot of a database. Restarting the same database with `ENABLE_API_KEYS=true` after a first boot without it leaves keys disabled (still `403`, admin config shows `ENABLE_API_KEYS: false`). Observed on both versions. They must therefore be set on the very first boot, or changed through the admin config afterwards.

An API key is created with a session token: `POST /api/v1/auths/signin` with `{email, password}` returns `token`, then `POST /api/v1/auths/api_key` with `Authorization: Bearer <token>` returns `api_key`.

### `ENV` and the OpenAPI document

The official image sets `ENV=prod`, although the code default is `dev`. `/docs` and `/openapi.json` are served only when `ENV=dev`, so the throwaway instances set `ENV=dev` explicitly.

On 0.11.4 the generated document is not reproducible by default: `/api/v1/terminals/{server_id}/{path}` is registered for several methods and every one gets the same `operationId`, picked from a Python set (`…__post`, `…__patch`, …), which changes from one start to the next. The throwaway instances set `PYTHONHASHSEED=0`, which makes both snapshots byte-identical across runs. Clients must not rely on `operationId` for that route.

### Deviations from spec #3

- Spec #3 names `WEBUI_ADMIN_EMAIL`, `WEBUI_ADMIN_PASSWORD`, `WEBUI_ADMIN_NAME`, `ENABLE_API_KEYS`, `ENABLE_API_KEYS_ENDPOINT_RESTRICTIONS` and `API_KEYS_ALLOWED_ENDPOINTS` for bootstrap. The names hold on both versions, with two additions: the `*API_KEYS*` settings only take effect on first boot (persistent config), and 0.12.4 refuses API keys on `GET /api/v1/auths/`. Both are to be recorded in ADR-0001 (#12).
