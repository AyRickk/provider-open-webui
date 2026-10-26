# Progress

This document is **rewritten, never appended to**. It describes the current state only; history lives in git and in the issues.

## Current phase

**Phase 0: API spike.** Establish the Open WebUI API facts for every resource on the supported versions, then consolidate them in ADR-0001. Human validation is required before phase 1 starts. See the spec, issue #3.

## Open phase 0 tickets

GitHub's issue dependencies are the source of truth for blockers; this table is a snapshot.

| Ticket | Title | Blocked by |
| --- | --- | --- |
| #5 | build: devcontainer, mise toolchain and throwaway Open WebUI instances | none |
| #6 | spike(group): capture tooling and Group API notes | #5 |
| #7 | spike(model): Model API capture and field mapping | #6 |
| #8 | spike(prompt): Prompt API capture and notes | #6 |
| #9 | spike(knowledge): Knowledge API capture and notes | #6 |
| #10 | spike(tool): Tool API capture and notes | #6 |
| #11 | spike(function): Function API capture and notes | #6 |
| #12 | docs(adr): ADR-0001 phase 0 consolidation and validation gate | #7, #8, #9, #10, #11 |

## Frontier

**#5**, the devcontainer and mise toolchain. Done so far: #4, the agent docs.

## Open questions

None.
