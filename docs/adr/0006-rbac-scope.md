# ADR-0006: RBAC scope

- **Status:** accepted on 2026-10-29 by the maintainer: the grants follow spec #3 (Further Notes, Security), adjusted to Crossplane's observed RBAC; the rest follows the practice of the major Crossplane providers ([Precedents](#precedents))
- **Date:** 2026-10-28, revised 2026-10-29
- **Scope:** the Kubernetes permissions of the provider, and what tenants can do with namespaced provider configs.

## Context

The original brief asked for RBAC limited to the objects the provider uses. Spec #3 (story 61, Security) narrows that to Crossplane's standard per-type grants: its own CRDs, Secrets (get), ConfigMaps requested through the package's permission requests, events and leases, with "the inability to scope access to referenced objects" documented.

The provider does not write its own RBAC: Crossplane's RBAC manager renders it from the package. In Crossplane 2.4.3, the version spec #3 tests ([`internal/controller/rbac/provider/roles/roles.go`](https://github.com/crossplane/crossplane/blob/v2.4.3/internal/controller/rbac/provider/roles/roles.go) in the Crossplane repository), the `system` ClusterRole bound to the provider's service account grants:

- the provider's CRDs and their `status`: `get`, `list`, `watch`, `create`, `update`, `patch`; `update` on their `finalizers`;
- **every verb on Secrets, ConfigMaps, events and leases, cluster-wide**, to every provider. A package cannot narrow this rule;
- `get`, `list`, `watch` on CRDs, for the `safe-start` capability in `package/crossplane.yaml`.

So the grants are broader than story 61's "Secrets (get)", and ConfigMaps need no permission request. Kubernetes RBAC cannot scope a grant to the objects a resource references, so no package could narrow it to referenced Secrets and ConfigMaps either.

What the provider uses:

- Secrets: the API key and CA of a provider config, and valve values (stories 15, 41, 44). With the template default `--enable-secret-cache=true` (`ENABLE_SECRET_CACHE`), they are served from an informer, which lists and watches every Secret in the cluster and keeps them in memory.
- ConfigMaps: `contentFrom` code of tools and functions (story 14), read on each poll.
- Events and leases: the event recorder and leader election.

Tenancy follows Crossplane v2 (story 41). A namespaced `ProviderConfig` reads its Secrets in its own namespace, and a namespaced managed resource references objects in its own namespace. A `ClusterProviderConfig` names its Secret's namespace, and managed resources default to the one named `default` (story 42).

A namespaced `ProviderConfig` also lets a tenant point the provider at any URL the provider can reach. The provider sends `GET /api/version` and the credential check there, and up to 512 redacted characters of an error body reach that tenant's status and events (stories 41, 51). The URL pattern refuses credentials, a query and a fragment.

## Decision

- **The provider's RBAC is Crossplane's standard rendering**, with no permission request: ConfigMaps are already granted. The package does not add rules.
- **Isolation between tenants is the provider's**, not RBAC's: a `ProviderConfig` reads only Secrets of its namespace, and a managed resource reads only Secrets and ConfigMaps of its namespace. Namespaced kinds reference them with local selectors, which have no namespace field (`LocalSecretKeySelector` in `apis/v1alpha1/types.go`); only the cluster-scoped `ClusterProviderConfig` names a namespace. crossplane-runtime's `ExtractSecret` reads whatever namespace a reference names, so a namespace field on a namespaced kind would let a tenant read another namespace's Secret.
- **Who may use which Open WebUI instance is governed by RBAC on the provider's kinds**: a tenant allowed to create managed resources can use any `ClusterProviderConfig`, `default` included. Clusters where tenants must not share the instance's admin key give them their own `ProviderConfig`, and no `default` `ClusterProviderConfig`.
- **The Secret cache keeps the template default** (on). Operators who do not want a cluster-wide Secret informer set `ENABLE_SECRET_CACHE=false` through a `DeploymentRuntimeConfig`; Secrets are then read with direct gets.
- **Namespaced `ProviderConfig` URLs are not restricted further in v1**: no host allowlist, no loopback or link-local guard. The stated outbound requests and the redacted error body are the accepted exposure; operators restrict where the provider can connect with a `NetworkPolicy` on its pod, and who may create provider configs with RBAC. An allowlist, if one is ever needed, would be an opt-in provider flag, never a default.

## Consequences

- The README's security section and the CRD reference state the effective grants, the per-object limitation, the Secret cache and its flag, and the tenancy rules above (story 61's "documented" requirement).
- Every Secret and ConfigMap reference added by a resource ticket is a local selector on namespaced kinds, and is read in the resource's namespace.
- A provider compromise exposes every Secret and ConfigMap of the cluster, as for any Crossplane provider. The hardened pod (story 60) limits how it can be compromised, not what it can read.

## Precedents

Surveyed on 2026-10-29 in the source of crossplane-contrib's native providers (kubernetes, helm, http, sql, argocd, gitlab), the Upjet providers and `crossplane/provider-template`:

- No provider package declares permission requests; they rely on Crossplane's system role, as decided above.
- Every provider keeps the Secret cache on by default, or has no flag and always caches.
- None restricts its endpoint URLs or guards against loopback or link-local targets; gitlab's `baseURL`, argocd's `serverAddr` and provider-http's request URLs are free-form.
- The large v2 providers pin a namespaced provider config's Secret to its own namespace (upjet-aws, helm, gitlab), as the local selectors here do; the template and provider-http do not. Crossplane v2's core types use a local reference for a namespaced resource's connection Secret too.
