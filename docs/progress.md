# Progress

This document is **rewritten, never appended to**. It describes the current state only; history lives in git and in the issues.

## Current phase

**Phase 1: foundation**, not yet ticketed. Phase 0 (API spike, #4 to #12) is closed: the maintainer validated [ADR-0001](adr/0001-phase-0-api-consolidation.md) on 2026-10-27 and settled its five [decisions](adr/0001-phase-0-api-consolidation.md#decisions) (spec #3).

Phase 1 builds what every resource needs, per spec #3 and ADR-0001's [phase 1 consequences](adr/0001-phase-0-api-consolidation.md#phase-1-foundation): the repository scaffold from `crossplane/provider-template`, the `ProviderConfig` / `ClusterProviderConfig`, the versioned `Client` layer with its 0.11 and 0.12 profiles (seam S1) and its contract tests, the credential and version checks, CI, and the contrib-ready repository files.

## Open tickets

None yet. Phase 1 is cut into tracer-bullet tickets (sub-issues of spec #3, with blocking links) before any code.

## Frontier

**Cut phase 1 into tickets** from spec #3 and ADR-0001; then the first frontier ticket.

## Open questions

None. The phase 0 inputs to phase 1 are in ADR-0001 and [`docs/api-notes/`](api-notes/README.md).
