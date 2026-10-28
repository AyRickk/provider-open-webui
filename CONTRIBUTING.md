# Contributing to provider-openwebui

Thank you for your interest in the project. This guide covers how a change gets in, and the coding and testing conventions reviews hold it to. Everyone taking part follows the [Code of Conduct](CODE_OF_CONDUCT.md). Report vulnerabilities as [`SECURITY.md`](SECURITY.md) describes, never in a public issue.

## Before you start

- The scope, phases and testing seams of the project are in the spec, [issue #3](https://github.com/AyRickk/provider-openwebui/issues/3). [`docs/progress.md`](docs/progress.md) gives the current phase and the next tickets.
- Decisions live in [`docs/adr/`](docs/adr/), and observed Open WebUI API behavior in [`docs/api-notes/`](docs/api-notes/). Observed API behavior takes precedence over the spec; a change that departs from the spec records the difference in an ADR.
- Work happens in GitHub issues. Open or pick an issue before a non-trivial change, so the approach is agreed first. One issue gets one branch and one pull request.
- Agents, and humans who want the full checklist, follow [`docs/agents/ticket-workflow.md`](docs/agents/ticket-workflow.md).
- Everything in the repository is written in English: code, comments, docs, commit messages, issues and pull requests.

## Development environment

The devcontainer is the only execution environment: the host needs only a container runtime and the [devcontainer CLI](https://github.com/devcontainers/cli). Tool versions are pinned in [`mise.toml`](mise.toml), which CI uses too, so a local run matches CI. Run every command through `devcontainer exec`:

```bash
devcontainer up --workspace-folder .
devcontainer exec --workspace-folder . make reviewable
```

The [README](README.md#development-environment) covers building, throwaway Open WebUI instances, the supported version matrix, and the contract and e2e suites.

## Making a change

1. Branch from an up-to-date `main`, as `<type>/<issue>-<slug>`, where `<type>` is the Conventional Commits type of the change (e.g. `feat/12-group`).
2. Write the test first, at one of the seams below, then the code.
3. Run `make reviewable` (generated code and CRDs, `go mod tidy`, lint, unit tests). Run the contract tests (`mise run contract`) when you touch the client layer, and the e2e suite (`mise run e2e`) when you touch a controller, an API type or an example.
4. Commit, signed off (see below), and open a pull request.

### Commits

- **DCO.** Every commit carries a `Signed-off-by` line matching its author, certifying the [Developer Certificate of Origin](DCO). Sign off with your own identity, the one configured in git: `git commit -s` adds the line from `user.name` and `user.email`. To sign off commits already made on your branch, run `git rebase --signoff origin/main`. A pull request is not merged until every commit is signed off by its own author.
- **Conventional Commits.** Messages follow [Conventional Commits](https://www.conventionalcommits.org/): `<type>(<scope>): <description>`, with types such as `feat`, `fix`, `docs`, `test`, `ci`, `build`, `refactor` and `chore`, and scopes naming the area (`client`, `apis`, `controller`, `e2e`, `contract`, ...). The description is lowercase, imperative and has no trailing period.
- **No AI attribution.** Following Crossplane's AI policy, commits and pull requests carry no AI attribution trailers or lines (`Co-Authored-By` an AI tool, "Generated with ..."). You are the author of what you submit, and your sign-off certifies it.

### Pull requests

- The title follows Conventional Commits, usually the issue title.
- The body says `Closes #<issue>`, describes the change and how it was tested, and ticks the DCO checkbox of the [template](.github/PULL_REQUEST_TEMPLATE.md).
- CI must be green: lint, generated code up to date, unit tests with the coverage gate, contract and e2e matrices, govulncheck, image scan and build.
- A maintainer from [`OWNERS.md`](OWNERS.md) reviews and merges.

## Coding conventions

These follow the Crossplane provider conventions; the template's controller code is the reference style.

### Files

Go files start with the template's SPDX header:

```go
// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0
```

The project is licensed under [Apache-2.0](LICENSE). Do not copy code from projects under an incompatible license, such as the MPL-2.0 or unlicensed Terraform providers for Open WebUI; they may be read to understand API pitfalls.

### Tests

- Unit tests use only the standard library and [`go-cmp`](https://github.com/google/go-cmp). Testify, Ginkgo and Gomega are not used.
- Tests are table-driven: a `map[string]struct{ reason string; args args; want want }`, one case per entry, where `reason` says in a sentence what the case shows. Failures print a `-want, +got` diff after the reason. Errors are compared with `cmpopts.EquateErrors()`.

  ```go
  cases := map[string]struct {
  	reason string
  	args   args
  	want   want
  }{
  	"MissingSecret": {
  		reason: "A missing API key Secret is an error naming it.",
  		args:   args{...},
  		want:   want{err: errBoom},
  	},
  }
  for name, tc := range cases {
  	t.Run(name, func(t *testing.T) {
  		got, err := Do(tc.args.in)
  		if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
  			t.Errorf("\n%s\nDo(...): -want error, +got error:\n%s", tc.reason, diff)
  		}
  		if diff := cmp.Diff(tc.want.out, got); diff != "" {
  			t.Errorf("\n%s\nDo(...): -want, +got:\n%s", tc.reason, diff)
  		}
  	})
  }
  ```

- A test exercises external behavior through a seam and never inspects internals. The seams of spec #3 are:
  - **S1**, the internal `Client` interface: contract tests against real Open WebUI instances (`test/contract`), and transport tests against an `httptest` server;
  - **S2**, each kind's external client (`Observe`, `Create`, `Update`, `Delete`), with an in-memory fake `Client` seeded from the fixtures;
  - **S3**, the cluster: chainsaw tests on kind (`test/e2e`).
- Tests are deterministic. Fixtures come from real instances, recorded with `mise run contract:record` or captured with `mise run owui:capture`; never hand-write an API payload.
- The coverage gate is 80% on `internal/client/...` and on every controller package.

### Errors

- Wrap errors with `github.com/crossplane/crossplane-runtime/v2/pkg/errors` (`errors.Wrap`, `errors.Wrapf`, `errors.New`), not `fmt.Errorf` or another errors package.
- Write messages inline at the call site, in lowercase, starting with what failed: `errors.Wrap(err, "cannot get the provider config")`.
- Quote values with `%q`: `errors.Wrapf(err, "cannot get %s %q", kind, name)`.
- Keep output deterministic: no timestamps, no map iteration order, no random values. The same failure gives the same message, so conditions stay stable.
- Never put a token, an API key or a Secret value in an error, a log line, an event, a status field or a condition message. Error bodies from Open WebUI go through the transport's redaction.

### Logging and events

- Loggers and event recorders are created in `main` and injected through the controller options. Never use a package-level logger or recorder.
- Log mostly at debug level, from `Reconcile` rather than deep in helpers, with the error under the `error` key: `log.Debug("Cannot observe the external resource", "error", err)`.
- Emit an event only when something changes: `CreatedExternalResource`, `UpdatedExternalResource` (with a summary of the diff), `DeletedExternalResource`, or an error. A steady-state reconcile emits nothing and writes nothing.

### Conditions and status

- Managed resources report `Ready` and `Synced`. Condition messages are stable: no timestamps, counters or other values that change between reconciles of the same state.
- `status.atProvider` holds observed, reproducible state only (IDs, timestamps from the server, detected types), never a secret.

### API types

- Every API field has GoDoc written for readers of `kubectl explain`: what the field does in Open WebUI, its default, and its constraints. Do not mention Go types or internals.
- Optional fields are pointers, marked `+optional`, with `omitempty` in their JSON tag.
- After changing a type, run `make generate` and commit the result; CI fails if the generated code is out of date.
