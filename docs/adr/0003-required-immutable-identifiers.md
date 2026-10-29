# ADR-0003: Required, immutable identifier fields instead of `metadata.name`

- **Status:** accepted, decided by spec #3 (Further Notes) with the adjustments of [ADR-0001](0001-phase-0-api-consolidation.md); recorded on 2026-10-28
- **Date:** 2026-10-28
- **Scope:** how each managed resource names its Open WebUI object: `Model`, `Tool` and `Function` by an identifier field; `Group`, `Prompt` and `Knowledge` by the server's UUID.

## Context

The original brief let the Open WebUI identifier default to `metadata.name`, as Crossplane's `NameAsExternalName` initializer does. Spec #3 (story 23 and Controllers) makes `modelId`, `toolId` and `functionId` required and immutable instead, and derives the external-name annotation from them, or from the server-assigned UUID for the other kinds (story 24).

`metadata.name` cannot carry these identifiers:

- It is a DNS subdomain name: lowercase alphanumerics, `-` and `.`. Tool and function ids are Python identifiers, with `_` and no `-`; model ids routinely carry `:` or `/` (`llama3.1:8b`, `org/model`).
- The managed resources are namespaced. Two namespaces may hold a resource of the same name, which would point both at one Open WebUI object.
- An object cannot be renamed, so a name chosen for Kubernetes would be the Open WebUI id forever, with no field saying so.

Phase 0 adjusted the rule ([ADR-0001](0001-phase-0-api-consolidation.md), deviations 21 and 38):

- The server accepts any Python identifier as a tool or function id and lowercases it, so a looser rule lets the declared id and the stored id differ.
- Prompts are routed by a server UUID, and their command is updatable, so the command cannot be a stable identifier.
- Group and knowledge names are not unique, and have no identifier of their own.

## Decision

- **`modelId`, `toolId` and `functionId` are required and immutable.** Each carries `+immutable` and the CEL rule `self == oldSelf`, so the API server rejects a change.
- **Format rules, by CEL:** `toolId` and `functionId` match `^[a-z_][a-z0-9_]*$`. `modelId` is a non-empty string. Its further format rules are deferred to the `Model` ticket, which derives them from the [model notes](../api-notes/model.md).
- **External-name is the identifier** for `Model`, `Tool` and `Function`, set by the provider's initializer in place of `NameAsExternalName`. An external-name that differs from the identifier field gives `Synced=False`, and nothing is written.
- **`Group`, `Prompt` and `Knowledge` are identified by the server's UUID.** The external-name is empty until the object is created or adopted, or until the operator sets it by hand to import an object, then holds the UUID. Their unique key (group name, prompt command, knowledge name) only serves to find an existing object on create ([ADR-0005](0005-adoption-requires-the-annotation.md)).
- **A prompt's `command` is an ordinary, mutable field**, written without its leading `/` ([ADR-0001, decision 1](0001-phase-0-api-consolidation.md#decisions)). Whether a group or knowledge `name` is mutable is decided by those kinds' tickets.
- **The other required fields of `Group`, `Prompt` and `Knowledge`** (name, command) are required only when the management policies can create or update (`*`, `Create` or `Update`), by CEL, as the Upjet providers generate it. An `Observe`-only import by UUID then declares nothing but the external-name. The identifier fields of `Model`, `Tool` and `Function` stay always required: they are the external-name.
- **CEL is what enforces immutability**; `+immutable` only documents it.

## Consequences

- Each resource states its Open WebUI id explicitly, and editing it is refused at admission, so an object cannot be orphaned by a typo fix (story 23).
- `metadata.name` is free for Kubernetes naming. The same id declared in two namespaces against one instance is refused for the second resource, unless it carries the adopt annotation ([ADR-0005](0005-adoption-requires-the-annotation.md)).
- `Model`, `Tool` and `Function` have deterministic external-names, so they recover from a crash during create on their own; the UUID kinds do not ([ADR-0005](0005-adoption-requires-the-annotation.md), crash recovery).
- Renaming a tool or function id is a delete and re-create: a new resource, then the old one deleted.
