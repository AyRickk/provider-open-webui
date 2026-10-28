# Security policy

## Supported versions

The project has no release yet. Once `v0.1.0` is out, security fixes go to the latest release only.

## Reporting a vulnerability

Do not open a public issue, pull request or discussion for a vulnerability.

Report it through GitHub's [private vulnerability reporting](https://github.com/AyRickk/provider-openwebui/security/advisories/new): on the repository's **Security** tab, choose **Report a vulnerability**. Include the affected version or commit, the steps to reproduce, and the impact you see.

The maintainers in [`OWNERS.md`](OWNERS.md) acknowledge the report, work on a fix in the advisory's temporary private fork, and publish the advisory with the fixed release. You are credited in the advisory unless you ask otherwise.

A vulnerability in Open WebUI itself belongs to the [Open WebUI project](https://github.com/open-webui/open-webui/security). A vulnerability in Crossplane or `crossplane-runtime` belongs to the [Crossplane project](https://github.com/crossplane/crossplane/security).

## Verifying a release

Release images and packages are published to `ghcr.io/ayrickk/provider-openwebui`, signed with [cosign](https://docs.sigstore.dev/cosign/) keyless by the release workflow of this repository, with an SPDX SBOM attached as a signed attestation. Verify both before installing, replacing `vX.Y.Z` with the release:

```bash
cosign verify ghcr.io/ayrickk/provider-openwebui:vX.Y.Z \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/AyRickk/provider-openwebui/\.github/workflows/.+@refs/tags/vX\.Y\.Z$'

cosign verify-attestation ghcr.io/ayrickk/provider-openwebui:vX.Y.Z \
  --type spdxjson \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/AyRickk/provider-openwebui/\.github/workflows/.+@refs/tags/vX\.Y\.Z$'
```

A signature that does not verify, or one issued for another repository or ref, means the artifact did not come from this project's release: do not install it, and report it as above.
