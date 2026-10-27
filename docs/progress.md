# Progress

This document is **rewritten, never appended to**. It describes the current state only; history lives in git and in the issues.

## Current phase

**Phase 0: API spike, complete and waiting for the maintainer's validation.** Phase 1 does not start before it (spec #3: "Human validation is required before phase 1").

The consolidated result is [ADR-0001](adr/0001-phase-0-api-consolidation.md) (status: proposed). The phase 0 exit report is posted on spec #3.

## Phase 0 exit criteria

| Criterion (#12) | State | Where |
| --- | --- | --- |
| API notes for all six resources on both versions, no placeholder | done | [`docs/api-notes/`](api-notes/README.md) |
| OpenAPI diff of the used routes, classified | done: 43 routes, 35 none, 8 profile, 0 breaking | [`openapi-diff.md`](api-notes/openapi-diff.md) |
| Minimal API-key allow-list | done, verified on both versions | [`api-key-allowed-endpoints.txt`](api-notes/api-key-allowed-endpoints.txt) |
| Marker storage per resource | done, documented only | [ADR-0001](adr/0001-phase-0-api-consolidation.md#marker-storage) |
| ADR-0001 with every deviation from spec #3 | done: 38 deviations | [ADR-0001](adr/0001-phase-0-api-consolidation.md#deviations-from-spec-3) |
| Exit report on #3 | posted | spec #3 |
| Maintainer's validation | **waiting** | spec #3 |

## Open tickets

None in phase 0. Phase 1 tickets are created after the validation, from ADR-0001's [consequences](adr/0001-phase-0-api-consolidation.md#consequences).

## Frontier

**Stop.** The next step is the maintainer's: validate ADR-0001 and answer its [decisions needed](adr/0001-phase-0-api-consolidation.md#decisions-needed) on spec #3. Phase 1 tickets follow.

## Open questions

The five [decisions needed](adr/0001-phase-0-api-consolidation.md#decisions-needed) in ADR-0001: prompt command spelling, duplicate names at adoption, the optional marker, knowledge deletion, and the 0.12.4 version label.
