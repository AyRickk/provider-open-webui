# Ticket workflow

Each session implements exactly one ticket, from a clean context, using only what the repository and the issue tracker hold. Follow these steps in order.

## 1. Read

- The spec, issue #3: `gh issue view 3`.
- The ticket: `gh issue view <N> --json number,title,body,labels,comments`.
- `docs/progress.md`, for the current phase and frontier.
- `CONTRIBUTING.md` (once it exists), for coding and testing conventions.
- The ADRs in `docs/adr/` (once they exist) that touch the area of the ticket.

Check that the ticket has no open blocker: `gh api 'repos/{owner}/{repo}/issues/<N>' --jq .issue_dependencies_summary.blocked_by` must print `0`. If it does not, stop and tell the maintainer.

## 2. Branch

Create `<type>/<issue>-<slug>` from an up-to-date `main`, where `<type>` is the Conventional Commits type of the ticket:

```bash
git fetch origin
git switch -c <type>/<N>-<slug> origin/main   # e.g. build/5-devcontainer
```

## 3. Implement test-first

Apply the `mattpocock-skills:tdd` skill at the seams agreed in the spec (issue #3, Testing Decisions): S1 the internal `Client` interface, S2 each kind's external client, S3 the cluster. Tickets with no code (docs, spikes) skip this step and say so in the PR.

## 4. Test

Inside the devcontainer (`devcontainer exec --workspace-folder . <command>`, see `README.md`), run single test files while working, then the full suite once at the end. Report failures as they are; do not commit over a red suite.

## 5. Review

Run the two-axis `mattpocock-skills:code-review` skill (Standards and Spec) against `main`. Fix what it finds, or record in the PR why a finding is left.

## 6. Commit

Commit with `git commit -s`. Each commit carries a `Signed-off-by` line matching its own author, taken from the git identity of whoever makes the commit. Messages follow Conventional Commits, with no AI attribution trailers.

## 7. Open the PR

- Title in Conventional Commits form, usually the ticket title.
- Body containing `Closes #<N>`, the DCO checklist (every commit signed off by its author), and a summary of the review findings and how each was handled.
- No AI attribution lines.

## 8. Rewrite the progress doc

Rewrite `docs/progress.md` to reflect the state once this PR merges: current phase, open tickets, frontier, open questions. Rewrite it; never append to it. Commit it on the same branch.

## 9. Stop and announce

Stop. Do not start the next ticket. Tell the maintainer which frontier ticket comes next (an open sub-issue of #3 with no open blocker and no assignee, first by issue number) and the exact command to type in a fresh session:

```text
/mattpocock-skills:implement #<N>
```

If several tickets are on the frontier, list them all and recommend one.
