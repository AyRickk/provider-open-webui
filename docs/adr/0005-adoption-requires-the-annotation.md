# ADR-0005: Adoption fails without the annotation

- **Status:** accepted on 2026-10-29 by the maintainer: the refusal follows spec #3 (Further Notes, Controllers), the rest follows the practice of the major Crossplane providers ([Precedents](#precedents))
- **Date:** 2026-10-28, revised 2026-10-29
- **Scope:** creating and adopting objects, for every managed resource kind.

## Context

Crossplane's provider guide asks that create not fail when the external resource already exists, and also that a provider avoid adopting an unrelated existing resource; it does not say how to reconcile the two. Spec #3 deviates from the first rule (stories 25 to 27). Open WebUI objects are routinely created in the UI or by the post-install Jobs this provider replaces. A silent takeover would put such an object under the provider's control, and with the default `deletionPolicy: Delete`, deleting the managed resource would delete an object nobody declared as owned.

The unique key used to find an existing object is the identifier for `Model`, `Tool` and `Function`, and the group name, prompt command or knowledge name for the others ([ADR-0003](0003-required-immutable-identifiers.md)). Phase 0 adjusted the lookup ([ADR-0001](0001-phase-0-api-consolidation.md)): group and knowledge names are not unique; a prompt command matches with or without its leading `/`; a model's duplicate id answers 401, so `Create` checks with a get first.

crossplane-runtime records an attempted create with the `crossplane.io/external-create-pending`, `-succeeded` and `-failed` annotations. When a create is pending and its result unknown, the reconciler stops with `cannot determine creation result - remove the crossplane.io/external-create-pending annotation if it is safe to proceed`, unless the controller declares the external-name deterministic. The reconciler persists the annotations it writes around `Create`, but metadata that `Observe` changes is saved only through late-initialization, when the management policies include `LateInitialize`.

Crossplane's documented import flow creates the managed resource with `managementPolicies: ["Observe"]` and a hand-set `crossplane.io/external-name`, then switches the policies to `["*"]`.

## Decision

- **Without the annotation, finding an existing object fails.** The condition is `Synced=False` with `object "<key>" already exists in Open WebUI (id=<id>); set annotation openwebui.crossplane.io/adopt="true" to manage it`, and nothing is written.
- **`openwebui.crossplane.io/adopt: "true"` adopts the object**: the external-name is set to its id, and the next reconcile updates it as any managed object.
- **Several exact matches are a conflict**, never an adoption, even with the annotation ([ADR-0001, decision 2](0001-phase-0-api-consolidation.md#decisions)).
- **A managed resource owns an object only once it created or adopted it.** Before any write or delete, an object it does not own is refused as above. Ownership is decided per kind:
  - **`Group`, `Prompt`, `Knowledge`** (UUID external-name): a non-empty external-name is ownership. The provider sets it only when a create succeeds or the `adopt` annotation adopts a key match, so a hand-set external-name is the operator's explicit import, as in Crossplane's import flow. No annotation is needed for it.
  - **`Model`, `Tool`, `Function`** (deterministic external-name, set by the initializer before any create): the external-name proves nothing, and they find an existing object in `Observe`. They own it when the resource carries `external-create-succeeded`, or when its create is incomplete (crossplane-runtime's `meta.ExternalCreateIncomplete`: `-pending` newer than both `-succeeded` and `-failed`). A create refused because the object exists sets `-failed`, so a later `Observe` does not mistake it for the provider's own. Crossplane's import flow therefore needs `adopt` too on these kinds, and the README and the condition message say so.
- **An adoption is persisted explicitly.** Adoption in `Create` (UUID kinds) is saved by the reconciler with the external-name. Adoption in `Observe` (deterministic kinds) records `external-create-succeeded` with its own update of the resource, since the reconciler would save it only under `LateInitialize`. Removing `adopt` later does not release the object.
- **Deleting a refused resource never deletes the object**: it was never owned, so only the Kubernetes object goes.
- **`managementPolicies: ["Observe"]` needs no annotation** (story 28): it never writes or deletes.
- **Crash recovery** (story 27): `Model`, `Tool` and `Function` pass crossplane-runtime's `WithDeterministicExternalName(true)`, and an object found while their create is incomplete is the one the provider created. The UUID kinds keep crossplane-runtime's stop, `cannot determine creation result`: the operator removes the pending annotation, and either sets the external-name to the UUID of the object the crash left, or sets `adopt`. They never adopt a key match on their own: an object someone created in the UI with the same key during the crash window would be taken over.
- **`adopt` is the operator's assertion**: two managed resources annotated to adopt the same object both manage it, and fight. The provider does not detect this.

## Consequences

- Migrating from the post-install Jobs is one annotation per resource (story 26), and nothing created in the UI is taken over by accident.
- The S2 tests cover adoption with and without the annotation, and the e2e suite an adoption refused then accepted (spec #3, Testing Decisions).
- The behavior departs from most Crossplane providers, so the README and the condition message say how to adopt.
- Recovery after a crash on `Group`, `Prompt` or `Knowledge` is manual, and rare.
- The S2 tests cover the explicit persistence of an adoption made in `Observe`, under policies with and without `LateInitialize`.

## Precedents

Surveyed on 2026-10-29 in the source of crossplane-contrib's native providers (kubernetes, helm, http, sql, argocd, gitlab), the Upjet providers, and crossplane-runtime:

- No provider has an adopt annotation. Providers whose objects are named by the client (kubernetes, helm, argocd, sql, Upjet names) take an existing object over silently. Providers with server-assigned ids (gitlab, the archived cloudflare) create, let the API reject a duplicate, and import an object whose id is set by hand in the external-name, as the UUID kinds here do.
- crossplane-runtime merged a `MustCreate` management policy ("fail if the external resource already exists instead of silently adopting it") in February 2026 and reverted it before v2.2 ([crossplane-runtime#873](https://github.com/crossplane/crossplane-runtime/pull/873), [#930](https://github.com/crossplane/crossplane-runtime/issues/930)). The need is acknowledged upstream, with no mechanism: the refusal stays the provider's.
- Every provider with client-chosen names studied passes `WithDeterministicExternalName(true)`, and every server-id kind keeps the reconciler's stop after a pending create, as decided above. provider-keycloak also treats `external-create-succeeded` as proof that the resource created its object.
