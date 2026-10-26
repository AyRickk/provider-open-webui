# Progress

This document is **rewritten, never appended to**. It describes the current state only; history lives in git and in the issues.

## Current phase

**Phase 0: API spike.** Establish the Open WebUI API facts for every resource on the supported versions, then consolidate them in ADR-0001. Human validation is required before phase 1 starts. See the spec, issue #3.

Done so far: #4 (agent docs), #5 (devcontainer, throwaway instances, cross-cutting API notes), #6 (capture tooling `mise run owui:capture <resource> <version>`, Group fixtures and `docs/api-notes/group.md`, the template for the other resource pages).

## Open phase 0 tickets

GitHub's issue dependencies are the source of truth for blockers; this table is a snapshot.

| Ticket | Title | Blocked by |
| --- | --- | --- |
| #7 | spike(model): Model API capture and field mapping | none |
| #8 | spike(prompt): Prompt API capture and notes | none |
| #9 | spike(knowledge): Knowledge API capture and notes | none |
| #10 | spike(tool): Tool API capture and notes | none |
| #11 | spike(function): Function API capture and notes | none |
| #12 | docs(adr): ADR-0001 phase 0 consolidation and validation gate | #7, #8, #9, #10, #11 |

## Frontier

**#7 to #11** can be taken in any order; #7 (Model) first, as phase 2 implements Group then Model.

## Open questions

- On 0.12.4, `GET /api/v1/auths/` refuses API keys ("A human session is required."). Which authenticated endpoint the provider uses to check its credentials is to be settled in ADR-0001.
- The `*API_KEYS*` settings are persistent config and only apply on a database's first boot. The bootstrap guide must say so.
- Group names are not unique, while Keycloak group sync matches by name: ADR-0001 must decide how find-by-name and adoption treat several groups with the same name.
- Not-found handling differs per version and per endpoint (0.11.4: 401 on get, 400 on update, `200 true` on delete); ADR-0001 records the quirk map.
