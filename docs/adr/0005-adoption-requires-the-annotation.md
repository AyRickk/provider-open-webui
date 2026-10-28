# ADR-0005: Adoption fails without the annotation

- **Status:** proposed: the refusal follows spec #3 (Further Notes, Controllers); the ownership rule and crash recovery below, and the [open question](#open-questions), await the maintainer's decision
- **Date:** 2026-10-28
- **Scope:** creating and adopting objects, for every managed resource kind.

## Context

Crossplane's provider guide asks that create not fail when the external resource already exists: the provider takes it over. Spec #3 deviates from it (stories 25 to 27). Open WebUI objects are routinely created in the UI or by the post-install Jobs this provider replaces. A silent takeover would put such an object under the provider's control, and with the default `deletionPolicy: Delete`, deleting the managed resource would delete an object nobody declared as owned.

The unique key used to find an existing object is the identifier for `Model`, `Tool` and `Function`, and the group name, prompt command or knowledge name for the others ([ADR-0003](0003-required-immutable-identifiers.md)). Phase 0 adjusted the lookup ([ADR-0001](0001-phase-0-api-consolidation.md)): group and knowledge names are not unique; a prompt command matches with or without its leading `/`; a model's duplicate id answers 401, so `Create` checks with a get first.

crossplane-runtime records an attempted create with the `crossplane.io/external-create-pending`, `-succeeded` and `-failed` annotations. When a create is pending and its result unknown, the reconciler stops with `cannot determine creation result - remove the crossplane.io/external-create-pending annotation if it is safe to proceed`, unless the controller declares the external-name deterministic.

## Decision

- **Without the annotation, finding an existing object fails.** The condition is `Synced=False` with `object "<key>" already exists in Open WebUI (id=<id>); set annotation openwebui.crossplane.io/adopt="true" to manage it`, and nothing is written.
- **`openwebui.crossplane.io/adopt: "true"` adopts the object**: the external-name is set to its id, and the next reconcile updates it as any managed object.
- **Several exact matches are a conflict**, never an adoption, even with the annotation ([ADR-0001, decision 2](0001-phase-0-api-consolidation.md#decisions)).
- **A managed resource owns an object only once it created or adopted it.** Kinds with a deterministic external-name find an existing object in `Observe`, not in `Create`, so the rule runs wherever the object is first found: before any write or delete, the object is refused as above unless the resource carries `external-create-succeeded`, or its create is incomplete (crossplane-runtime's `meta.ExternalCreateIncomplete`: `-pending` newer than both `-succeeded` and `-failed`). A create refused because the object exists sets `-failed`, so a later `Observe` does not mistake it for the provider's own. The provider records an adoption with `external-create-succeeded`, so removing `adopt` later does not release the object.
- **Deleting a refused resource never deletes the object**: it was never owned, so only the Kubernetes object goes.
- **`managementPolicies: ["Observe"]` needs no annotation** (story 28): it never writes or deletes.
- **Crash recovery** (story 27): `Model`, `Tool` and `Function` pass crossplane-runtime's `WithDeterministicExternalName(true)`, and an object found while their create is incomplete is the one the provider created. The UUID kinds keep crossplane-runtime's stop, `cannot determine creation result`: the operator removes the pending annotation and sets `adopt`, which never creates a duplicate.
- **`adopt` is the operator's assertion**: two managed resources annotated to adopt the same object both manage it, and fight. The provider does not detect this.

## Consequences

- Migrating from the post-install Jobs is one annotation per resource (story 26), and nothing created in the UI is taken over by accident.
- The S2 tests cover adoption with and without the annotation, and the e2e suite an adoption refused then accepted (spec #3, Testing Decisions).
- The behavior departs from other Crossplane providers, so the README and the condition message say how to adopt.
- Recovery after a crash on `Group`, `Prompt` or `Knowledge` is manual, and rare.

## Open questions

- Whether the UUID kinds should recover from a pending create on their own, by adopting a single key match. The risk is adopting an object someone created in the UI with the same key during the crash window.
