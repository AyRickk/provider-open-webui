# Progress

This document is **rewritten, never appended to**. It describes the current state only; history lives in git and in the issues.

## Current phase

**Phase 1: foundation.** Phase 0 is closed: [ADR-0001](adr/0001-phase-0-api-consolidation.md) is accepted, with the maintainer's [decisions](adr/0001-phase-0-api-consolidation.md#decisions). Phase 1 builds what every resource needs, per spec #3 and ADR-0001's [phase 1 consequences](adr/0001-phase-0-api-consolidation.md#phase-1-foundation). No resource kind yet: `Group` opens phase 2. The tracer bullet is #28: the provider in kind, with a `ProviderConfig` healthy and showing the detected version.

Done so far:

- The scaffold from `crossplane/provider-template` (#31).
- The HTTP transport of the client layer, `internal/client/transport` (#25): timeouts, retries, TLS, auth header, request counter, sentinel errors, redacted error details. Per-request `StatusErrors` override the default sentinels by status and, optionally, by detail.
- Version detection and API profiles, `internal/client` (#26):
  - `VersionCache` reads `GET /api/version`, cached per provider-config key for 5 minutes.
  - `SelectProfile` picks the 0.11 or 0.12 profile and applies the unknown-minor policy. An unknown minor reads `Open WebUI vX.Y.Z is not in the supported range [0.11, 0.12]`. With `allowUnsupported`, it returns the newest profile and reports the version unsupported.
  - `Profile.CheckCredentials` calls `GET /api/v1/users/default/permissions`. A bad key gives `ErrUnauthorized`. A non-admin key gives `ErrForbidden`: it also answers 401, recognized by its detail.
  - ADR-0001's not-found and conflict map is profile data: `Profile.StatusErrors(op)` and `Profile.MissingSucceeds(op)`. Resource tickets pass them into their requests.
- `ProviderConfig` and `ClusterProviderConfig` (#27):
  - Spec: `url`, `apiKeySecretRef`, `authHeader`, `caCertSecretRef`, `insecureSkipVerify`, `allowUnsupportedVersions`. A `ProviderConfig` reads its Secrets in its own namespace; a `ClusterProviderConfig` names theirs.
  - `config.Connector`, in `internal/controller/config`, is shared by every controller. It reads the Secrets, keeps one `Doer` per provider config, detects the version, selects the profile and checks the credentials. `Connect(ctx, mg)` tracks the usage first, then resolves the managed resource's provider config. Resource controllers receive the connector in `Setup`, and the `provider.addtype` scaffold already uses it.
  - The provider config controllers report `status.version`, `status.profile` and the `Ready` condition, checked every poll interval. A `Warning` event is emitted when `Ready` turns to an error or to an allowed unsupported version.
- The baseline CI (#24):
  - `.github/workflows/ci.yml`, with every tool coming from `mise.toml` through `jdx/mise-action`. Its jobs:
    - lint: golangci-lint, `go vet`, and `make generate` leaving the tree clean;
    - Renovate configuration validation;
    - unit tests, through `hack/ci/test-coverage.sh`;
    - govulncheck;
    - a multi-arch build of the image and the xpkg, scanned by trivy (HIGH and CRITICAL with a fix), without publishing.
  - The coverage table per package goes to the job summary. The gate is 80% on `internal/client/...` and on every `internal/controller/<pkg>`, but not on the controller root, which only wires the controllers together.
  - `.github/workflows/scorecard.yml` runs OpenSSF Scorecard.
  - `.github/renovate.json5` watches Go modules, `mise.toml`, actions, Dockerfiles, and versions annotated with `# renovate: datasource=... depName=...`. The contract and e2e matrices annotate each Open WebUI version this way with `depName=ghcr.io/open-webui/open-webui`. Patch bumps are grouped, and a new minor waits for approval on the dependency dashboard.

## Open phase 1 tickets

GitHub's issue dependencies are the source of truth for blockers; this table is a snapshot.

| Ticket | Title | Blocked by |
| --- | --- | --- |
| #23 | docs: contrib-ready repository files | none |
| #30 | docs(adr): ADRs for the deviations from the original brief | none |
| #28 | test(e2e): kind + Crossplane + Open WebUI harness | none |
| #29 | test(contract): contract test harness against real instances | none |

Signed publishing to ghcr (cosign, SBOM, maintainer approval) is phase 4, with the release.

## Frontier

**#28** (e2e harness) first: it is the tracer bullet of phase 1. #29 (contract harness) follows, and #23 and #30 (docs) have no blocker either.

## Open questions

- A namespaced `ProviderConfig` lets a tenant point the provider at any URL the provider can reach. The provider sends two fixed GETs there, and up to 512 redacted characters of an error body reach that tenant's status and events. This follows spec #3 (stories 41 and 51). The URL pattern refuses credentials, a query and a fragment. Whether to go further (a host allowlist, refusing link-local and loopback addresses, generic messages) is the maintainer's call, best recorded in #30's RBAC and tenancy ADR.
- With the Secret cache on (the template default), the provider lists and watches Secrets cluster-wide. Crossplane's provider role grants this, but it is broader than spec #3's "Secrets (get)". #30's RBAC ADR should record it, or the cache should be scoped.
- Provider configs have no `Synced` condition, so an unknown minor shows there as `Ready=False`. Managed resources get `Synced=False` from the connector's error, with the same message. `Connect` reports an allowed unsupported version through `Connection.Unsupported`; only the provider config emits the `Warning` event, so resource controllers need not emit one.
- 0.11.4 answers a function update with the same `400 "[ERROR: Error updating function]"` for a missing function and for invalid code, so the 0.11 profile maps it to no sentinel. The `Function` ticket must decide existence with a get, as ADR-0001 already requires, and surface the detail as is.
- The profiles are tested directly, against the fixtures, because the resource `Client` interface (seam S1) does not exist yet. Spec #3 wants profiles tested only through S1, so these tests move behind it when the first resource lands.
- The connector runs the credential check on every connection, including each managed resource's reconcile: one extra GET per reconcile. If the request counter shows it matters, cache the check alongside the version.
- The Renovate GitHub app must be installed on the repository by the maintainer for the configuration to take effect. Dependabot security updates are still on, so the two can open overlapping PRs. The maintainer can turn Dependabot off once Renovate runs.
- `npm:renovate` cannot go in `mise.toml`: mise's npm trust policy refuses a transitive dependency (`@yarnpkg/libzip@3.2.2`) whose provenance attestation was dropped. CI runs the validator from the Renovate image, pinned by digest. Revisit when upstream republishes with provenance.
- Trivy fails the build on any HIGH or CRITICAL CVE with a fix, so a CVE published upstream can turn an unrelated PR red until the dependency is bumped. This is the release requirement of spec #3, applied to every PR.
- `hack/helpers/addtype.sh` is committed without the executable bit, and it needs `GOMPLATE` set, so `make provider.addtype` fails as documented in the README. Running `GOMPLATE=$(command -v gomplate) bash ./hack/helpers/addtype.sh` with the same variables works.
