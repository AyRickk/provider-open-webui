# Agent instructions

`provider-openwebui` is a native Crossplane v2 provider for Open WebUI, hosted at `AyRickk/provider-openwebui`.

## Start here

- **Spec**: issue #3, the source of truth for scope, phases and testing seams.
- **Progress**: `docs/progress.md`, the current phase, open tickets and frontier.
- **Ticket workflow**: `docs/agents/ticket-workflow.md`, the steps every ticket session follows.
- **Issue tracker**: GitHub Issues via the `gh` CLI. See `docs/agents/issue-tracker.md`.
- **Triage labels**: `docs/agents/triage-labels.md`.
- **Domain docs**: `docs/agents/domain.md`, which points to `GLOSSARY.md` and `docs/adr/` (once they exist).
- **Conventions**: `CONTRIBUTING.md` (once it exists).

## Guardrails

- One ticket per session: finish it, open its PR, then stop and hand over.
- Every commit carries a DCO sign-off of its own author (`git commit -s`), using the identity configured in git.
- Commit messages and PR titles follow Conventional Commits, with no AI attribution trailers or lines.
- Commands run inside the devcontainer, with tools pinned in the mise file (once they exist).
- Repository content is written in English.
- Observed Open WebUI API behavior takes precedence over the spec; record the difference in an ADR.

## Keeping this file short

A line enters this file only if it applies to every session. Lessons and decisions go to ADRs, docs or tickets.
