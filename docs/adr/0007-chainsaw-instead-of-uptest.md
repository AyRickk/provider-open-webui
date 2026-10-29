# ADR-0007: chainsaw instead of uptest

- **Status:** accepted, decided by spec #3 (Testing Decisions, Further Notes); recorded on 2026-10-28
- **Date:** 2026-10-28, revised 2026-10-29
- **Scope:** the end-to-end tests, seam S3.

## Context

The original brief asked for uptest, Crossplane's end-to-end tool. Uptest takes annotated example manifests, applies them, waits for `Ready`, and runs its own import and delete steps. It generates chainsaw tests to do so. It is what most crossplane-contrib providers use (kubernetes, helm, http, keycloak, the Upjet providers).

The cluster tests of spec #3 make assertions on Open WebUI itself, by calling its API directly:

- drift corrected within one poll: a field changed through the API, then read back;
- anti-loop: after `Ready`, five poll intervals with zero `UpdatedExternalResource` events, zero writes on the request counter, and no growth of the model version history (`GET /api/v1/models/model/history`, 0.12);
- `Orphan` keeping the object, and `Observe`-only import with no writes;
- adoption refused, then accepted, on an object created through the API first;
- an upgrade from 0.11.4 to 0.12.4 on the same database.

Uptest can make such calls: its setup, teardown, pre- and post-assert, and pre- and post-delete hooks run arbitrary scripts, and provider-http checks its test server this way. But the tests above are multi-step sequences (count events over five polls, change the object then wait for the correction, refuse then accept an adoption, upgrade the instance mid-test) that do not fit one apply-assert-import-delete cycle per example: in uptest, each would become a shell script in a hook. Chainsaw expresses them directly: its `script` steps run commands between assertions, and its assertions read `$values` and the cluster. Uptest generates chainsaw tests anyway, and provider-keycloak already runs hand-written chainsaw tests next to uptest for cases uptest cannot express.

Spec #3 rejects uptest because "it cannot perform the direct Open WebUI API calls"; that is inaccurate, as its hooks show. The decision stands on the structure of the tests, not on uptest's capabilities.

## Decision

- **The e2e suite is written in chainsaw directly**, in `test/e2e`, one directory per test. Chainsaw is pinned in `mise.toml`.
- Each test reads the instance from `$values` (`url`, `apiUrl`, `apiKey`, `version`, `profile`), set by `hack/e2e/test.sh`, and calls the Open WebUI API from `script` steps.
- Uptest is not used.

## Consequences

- The harness (#28) is in place: `mise run e2e`, the per-version CI matrix job, and `test/e2e/providerconfig` as the first test.
- Uptest's generated steps are written by hand: apply and wait for `Ready` and `Synced`, import, delete. Every example is exercised by a chainsaw test (spec #3), so a new example comes with its test.
- Tests share one Open WebUI instance per version, so they run one at a time (`test/e2e/.chainsaw.yaml`).
- The suite follows chainsaw's releases through Renovate, and not uptest's conventions. If a move to `crossplane-contrib` asks for uptest over `examples/`, it can be added next to the chainsaw tests, as provider-keycloak does.
