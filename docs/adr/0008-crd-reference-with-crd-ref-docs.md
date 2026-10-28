# ADR-0008: CRD reference generated with `crd-ref-docs`

- **Status:** accepted, decided by spec #3 (Further Notes); recorded on 2026-10-28
- **Date:** 2026-10-28
- **Scope:** the reference documentation of the provider's API types.

## Context

Spec #3 lists a generated CRD reference among the documentation deliverables, generated with [`crd-ref-docs`](https://github.com/elastic/crd-ref-docs), a deviation from the original brief.

Crossplane providers usually rely on the Upbound Marketplace for their API reference, rendered from packages published to `xpkg.upbound.io`. This provider publishes to ghcr instead ([ADR-0002](0002-github-actions-and-ghcr.md)), so no marketplace page exists. The reference must therefore come from the repository.

`crd-ref-docs` reads the Go API types under `apis/`, with their GoDoc and kubebuilder markers, and renders Markdown or AsciiDoc through templates. It runs offline, as a single binary, so it fits the devcontainer and the mise toolchain. Spec #3 already requires GoDoc on every API field, written for `kubectl explain` readers.

## Decision

- **The CRD reference is generated with `crd-ref-docs`** from `apis/`, as Markdown, committed under `docs/`, and pinned in `mise.toml`.
- **The GoDoc of the API types is the single source** of field documentation: `kubectl explain` and the reference show the same text. Nothing in the generated file is edited by hand.
- **CI checks the reference is up to date**, as it does for `make generate`.

## Consequences

- The reference is added by its own ticket, with the generator's configuration.
- Field documentation lands in GoDoc, not in hand-written pages. The guides (GitOps, bootstrap, compatibility) link to the reference instead of repeating it.
- Statements the CRD reference must carry come from the ADRs, for instance unchecked user ids in `access` ([ADR-0001](0001-phase-0-api-consolidation.md#sharing)) and the RBAC scope ([ADR-0006](0006-rbac-scope.md)): they go in the GoDoc of the fields concerned.
