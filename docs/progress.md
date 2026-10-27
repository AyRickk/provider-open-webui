# Progress

This document is **rewritten, never appended to**. It describes the current state only; history lives in git and in the issues.

## Current phase

**Phase 0: API spike.** Establish the Open WebUI API facts for every resource on the supported versions, then consolidate them in ADR-0001. Human validation is required before phase 1 starts. See the spec, issue #3.

All resource spikes are done: #4 (agent docs), #5 (devcontainer, throwaway instances, cross-cutting API notes), #6 (capture tooling, Group), #7 (Model, access grants), #11 (Function), #10 (Tool), #9 (Knowledge, `docs/limitations.md`), #8 (Prompt). The notes are indexed in `docs/api-notes/README.md`.

## Open phase 0 tickets

GitHub's issue dependencies are the source of truth for blockers; this table is a snapshot.

| Ticket | Title | Blocked by |
| --- | --- | --- |
| #12 | docs(adr): ADR-0001 phase 0 consolidation and validation gate | none |

## Frontier

**#12**, ADR-0001: consolidate the open questions below, then the maintainer's validation gates phase 1.

## Open questions

- On 0.12.4, `GET /api/v1/auths/` refuses API keys ("A human session is required."). Which authenticated endpoint the provider uses to check its credentials is to be settled in ADR-0001.
- The `*API_KEYS*` settings are persistent config and only apply on a database's first boot. The bootstrap guide must say so.
- Group and knowledge base names are not unique (knowledge names may even be empty), prompt commands are unique only as exact, case-sensitive strings, and Keycloak group sync matches by name: ADR-0001 must decide how find-by-name / find-by-command and adoption treat several matching objects.
- Not-found handling differs per resource, version and endpoint (Group: 401, 400 or `200 true` on 0.11.4; Model: 401 on update and delete, 403 on export, the access endpoint re-creates a missing model; Function: 401 on get, 400 on update, `200 true` on delete; Tool: 404 on get, 401 on update and delete; Knowledge: 404 on get, 400 on update and delete; Prompt: 404 everywhere); ADR-0001 records the quirk map.
- Model additional params are flat keys of `params`: ADR-0001 decides how the CRD's `additionalParams` is flattened and how collisions with named fields are rejected.
- Function and tool ids are lowercased by the server: ADR-0001 fixes the CEL rule for `functionId` and `toolId`.
- Tool and function code is rewritten by `replace_imports` before storage: ADR-0001 records that the comparison applies the same rewrite to the desired code.
- Deleting a knowledge base orphans its files and vectors and rewrites the models that referenced it: ADR-0001 decides whether `Delete` warns, and how `Usage` ordering is documented.
- Prompt commands are stored as sent, leading `/` included: ADR-0001 fixes the normalization the provider applies and how adoption matches both spellings.
- `access_grants` in an object update are applied on 0.11.4 and ignored on 0.12.4, for every shareable resource: ADR-0001 records that sharing is written only through the access endpoints.
