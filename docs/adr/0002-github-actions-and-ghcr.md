# ADR-0002: GitHub Actions and ghcr instead of GitLab CI

- **Status:** accepted, decided by spec #3 (Further Notes); recorded on 2026-10-28
- **Date:** 2026-10-28
- **Scope:** continuous integration, release and package hosting.

## Context

The original brief asked for GitLab CI. Spec #3 deviates from it: the repository is `github.com/AyRickk/provider-openwebui`, built contrib-ready so that it can later move to `crossplane-contrib` without rewriting history, and its images and packages are published to `ghcr.io/ayrickk/provider-openwebui`, signed with cosign keyless and attested with an SPDX SBOM. The `xpkg.upbound.io` publishing of `crossplane/provider-template` is removed.

What weighs on the choice:

- The repository, its issues and its PRs live on GitHub. A GitLab pipeline would need a mirror, and its results would not show on the PRs that the ticket workflow opens.
- `crossplane-contrib` providers are on GitHub and run GitHub Actions; `crossplane/provider-template` ships its workflows for it.
- Cosign keyless signing needs an OIDC identity. GitHub Actions issues one per workflow run, so a release is signed by the workflow that built it, with no long-lived key.
- OpenSSF Scorecard and Renovate, both required by spec #3, run against GitHub repositories (`.github/workflows/scorecard.yml`, `.github/renovate.json5`).
- Crossplane installs a package from any OCI registry by its full reference, so a registry next to the repository serves users as well as `xpkg.upbound.io` would.

## Decision

- **CI is GitHub Actions**, in `.github/workflows/`. Every tool comes from `mise.toml` through `jdx/mise-action`, the file the devcontainer uses, so local runs match CI. Actions are pinned by SHA, and each workflow declares minimal `permissions:`.
- **Images and packages go to `ghcr.io/ayrickk/provider-openwebui`** (`XPKG_REG_ORGS` in the `Makefile`). Nothing is pushed to `xpkg.upbound.io`.
- **Releases** run on a `vX.Y.Z` tag, need the maintainer's explicit approval, and sign the image and the package with cosign keyless, with an SPDX SBOM produced by syft and attested (spec #3, story 63).
- Until the release lands (phase 4), CI builds the multi-arch image and package and scans them without publishing.

## Consequences

- The release workflow (phase 4) uses the GitHub OIDC token (`id-token: write`) for cosign, and a protected environment for the maintainer's approval. `SECURITY.md` documents how to verify a release against the workflow identity.
- The packages on ghcr are private when first pushed: the maintainer makes them public once, in the package settings.
- Users install the provider by its full reference, `ghcr.io/ayrickk/provider-openwebui:<version>`. It is not listed on the Upbound Marketplace, whose generated documentation is therefore unavailable (see [ADR-0008](0008-crd-reference-with-crd-ref-docs.md)).
- A move to `crossplane-contrib` keeps the workflows; only the registry organization and the signing identity change.
