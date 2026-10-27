# Progress

This document is **rewritten, never appended to**. It describes the current state only; history lives in git and in the issues.

## Current phase

**Phase 1: foundation.** Phase 0 is closed: [ADR-0001](adr/0001-phase-0-api-consolidation.md) is accepted, with the maintainer's [decisions](adr/0001-phase-0-api-consolidation.md#decisions). Phase 1 builds what every resource needs, per spec #3 and ADR-0001's [phase 1 consequences](adr/0001-phase-0-api-consolidation.md#phase-1-foundation). No resource kind yet: `Group` opens phase 2. The tracer bullet is #28: the provider in kind, with a `ProviderConfig` healthy and showing the detected version.

## Open phase 1 tickets

GitHub's issue dependencies are the source of truth for blockers; this table is a snapshot.

| Ticket | Title | Blocked by |
| --- | --- | --- |
| #31 | build: scaffold the provider from crossplane/provider-template | none |
| #30 | docs(adr): ADRs for the deviations from the original brief | none |
| #23 | docs: contrib-ready repository files | #31 |
| #24 | ci: lint, unit tests, coverage, security, build | #31 |
| #25 | feat(client): HTTP transport and error mapping | #31 |
| #26 | feat(client): version detection and API profiles | #25 |
| #27 | feat(apis): ProviderConfig and ClusterProviderConfig | #31, #26 |
| #28 | test(e2e): kind + Crossplane + Open WebUI harness | #24, #27 |
| #29 | test(contract): contract test harness against real instances | #24, #26 |

Signed publishing to ghcr (cosign, SBOM, maintainer approval) is phase 4, with the release.

## Frontier

**#31** (scaffold) first: it unblocks most of phase 1. #30 (ADRs, docs only) has no blocker either.

## Open questions

None.
