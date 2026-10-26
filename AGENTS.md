## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues (AyRickk/provider-open-webui) via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default vocabulary: needs-triage, needs-info, ready-for-agent, ready-for-human, wontfix. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one root `GLOSSARY.md` + `docs/adr/`. See `docs/agents/domain.md`.

## Git conventions

- NEVER add a `Co-Authored-By: Claude …` trailer to commits, and never add a "Generated with Claude Code" (or any Claude attribution) line to PR descriptions.
- Always follow [Conventional Commits](https://www.conventionalcommits.org/) for commit messages and PR titles: `<type>(<optional scope>): <description>`, e.g. `feat: …`, `fix(api): …`, `docs: …`, `chore: …`, `refactor: …`, `test: …`, `ci: …`, `build: …`, `perf: …`. Lowercase imperative description, no trailing period; mark breaking changes with `!` or a `BREAKING CHANGE:` footer.
