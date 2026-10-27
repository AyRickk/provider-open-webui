# Progress

This document is **rewritten, never appended to**. It describes the current state only; history lives in git and in the issues.

## Current phase

**Phase 1: foundation.** Phase 0 is closed: [ADR-0001](adr/0001-phase-0-api-consolidation.md) is accepted, with the maintainer's [decisions](adr/0001-phase-0-api-consolidation.md#decisions). Phase 1 builds what every resource needs, per spec #3 and ADR-0001's [phase 1 consequences](adr/0001-phase-0-api-consolidation.md#phase-1-foundation). No resource kind yet: `Group` opens phase 2. The tracer bullet is #28: the provider in kind, with a `ProviderConfig` healthy and showing the detected version.

Done so far:

- The scaffold from `crossplane/provider-template` (#31).
- The HTTP transport of the client layer, `internal/client/transport` (#25): timeouts, retries, TLS, auth header, request counter, sentinel errors, redacted error details. Per-request `StatusErrors` override the default sentinels by status and, optionally, by detail.
- Version detection and API profiles, `internal/client` (#26):
  - `VersionCache` reads `GET /api/version`, cached per provider-config key for 5 minutes.
  - `SelectProfile` picks the 0.11 or 0.12 profile and applies the unknown-minor policy. With `allowUnsupported`, it returns the newest profile and reports the version unsupported.
  - `Profile.CheckCredentials` calls `GET /api/v1/users/default/permissions`. A bad key gives `ErrUnauthorized`. A non-admin key gives `ErrForbidden`: it also answers 401, recognized by its detail.
  - ADR-0001's not-found and conflict map is profile data: `Profile.StatusErrors(op)` and `Profile.MissingSucceeds(op)`. Resource tickets pass them into their requests.

## Open phase 1 tickets

GitHub's issue dependencies are the source of truth for blockers; this table is a snapshot.

| Ticket | Title | Blocked by |
| --- | --- | --- |
| #23 | docs: contrib-ready repository files | none |
| #24 | ci: lint, unit tests, coverage, security, build | none |
| #30 | docs(adr): ADRs for the deviations from the original brief | none |
| #27 | feat(apis): ProviderConfig and ClusterProviderConfig | none |
| #28 | test(e2e): kind + Crossplane + Open WebUI harness | #24, #27 |
| #29 | test(contract): contract test harness against real instances | #24 |

Signed publishing to ghcr (cosign, SBOM, maintainer approval) is phase 4, with the release.

## Frontier

**#27** (ProviderConfig and ClusterProviderConfig) first. It is on the critical path to the tracer bullet #28. It wires `VersionCache`, `SelectProfile` (with `allowUnsupportedVersions` and its `Warning` event) and `CheckCredentials` into the config's status. #24 (CI) unblocks #28 and #29. #23 and #30 (docs) have no blocker either.

## Open questions

- 0.11.4 answers a function update with the same `400 "[ERROR: Error updating function]"` for a missing function and for invalid code, so the 0.11 profile maps it to no sentinel. The `Function` ticket must decide existence with a get, as ADR-0001 already requires, and surface the detail as is.
- The profiles are tested directly, against the fixtures, because the resource `Client` interface (seam S1) does not exist yet. Spec #3 wants profiles tested only through S1, so these tests move behind it when the first resource lands.
