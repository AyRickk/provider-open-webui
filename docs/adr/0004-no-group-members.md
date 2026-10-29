# ADR-0004: No `Group.members`

- **Status:** accepted, decided by spec #3 (Further Notes); recorded on 2026-10-28
- **Date:** 2026-10-28
- **Scope:** the `Group` resource in `v1alpha1`.

## Context

The original brief had a `members` field on `Group`. Spec #3 drops it: the provider never reads or writes group members (story 6), shows the member count in status (story 7), and puts user and membership management out of scope for v1.

The platform spec #3 targets runs Keycloak OIDC with group sync (`ENABLE_OAUTH_GROUP_MANAGEMENT=true`). With it, Open WebUI sets a user's group memberships from the token's groups claim at every login. A `members` field would give membership two writers: the provider would add a user, the next login would remove them, and the next poll would add them back.

Phase 0 found ([group notes](../api-notes/group.md)):

- `member_count` is part of every group read, so status needs no member route.
- Members are read through `export` or `POST …/users` on both versions, and `GET …/members` on 0.12.4 only. Writing them takes user ids, which the provider could only check through `GET /api/v1/users/`, a route the [minimal allow-list](../api-notes/api-key-allowed-endpoints.txt) deliberately refuses.
- 0.12.4 adds group hierarchy, with inherited membership.

## Decision

- **`Group` has no `members` field in `v1alpha1`.** The provider calls no member route.
- **`status.atProvider.memberCount`** reports `member_count` from the group read, so an operator can check that group sync works.
- **Exact membership management is a documented limitation**, with its own issue.

## Consequences

- The provider and Keycloak's group sync never fight. A group the provider manages is the group Keycloak fills, as long as its name matches the claim (story 2).
- Deployments without an identity provider manage members in the UI, or through the API by other means.
- The limitations doc gets a "Group membership" section pointing to this ADR, and its issue is opened with the other limitation issues.
- Adding membership later needs its own ADR.
