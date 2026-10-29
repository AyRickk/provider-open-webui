# Architecture decision records

Each ADR records one decision: its context, the decision and its consequences. Observed Open WebUI API behavior takes precedence over spec #3, and a departure from the spec is recorded here.

| ADR | Decision |
| --- | --- |
| [0001](0001-phase-0-api-consolidation.md) | Phase 0 consolidation of the Open WebUI API |
| [0002](0002-github-actions-and-ghcr.md) | GitHub Actions and ghcr instead of GitLab CI |
| [0003](0003-required-immutable-identifiers.md) | Required, immutable identifier fields instead of `metadata.name` |
| [0004](0004-no-group-members.md) | No `Group.members` |
| [0005](0005-adoption-requires-the-annotation.md) | Adoption fails without the annotation |
| [0006](0006-rbac-scope.md) | RBAC scope |
| [0007](0007-chainsaw-instead-of-uptest.md) | chainsaw instead of uptest |
| [0008](0008-crd-reference-with-crd-ref-docs.md) | CRD reference generated with `crd-ref-docs` |

ADRs 0002 to 0008 are the deviations from the original brief that spec #3 lists under Further Notes.
