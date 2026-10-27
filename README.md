# provider-openwebui

A native Crossplane v2 provider for Open WebUI. Work in progress; see `docs/progress.md`.

## Development environment

The devcontainer is the only execution environment. The host needs a container runtime (Docker or Podman) and the [devcontainer CLI](https://github.com/devcontainers/cli); everything else is installed inside the container. Tool versions are pinned in `mise.toml`, mise itself in `.devcontainer/Dockerfile` and `.github/workflows/ci.yml`, which Renovate updates together. The tools live in the `provider-openwebui-mise` volume, so rebuilding the container does not download them again. Containers started from inside (Open WebUI instances, kind clusters) run in the devcontainer's own Docker daemon (docker-in-docker).

```bash
devcontainer up --workspace-folder .                    # add --docker-path podman with Podman
devcontainer exec --workspace-folder . <command>        # e.g. mise run owui:up 0.12.4
```

Every command in this repository is meant to run through `devcontainer exec` (or a shell opened with `devcontainer exec --workspace-folder . bash`).

### Building the provider

The provider is scaffolded from [`crossplane/provider-template`](https://github.com/crossplane/provider-template) and builds with its `make` targets, using the `build` submodule (`git submodule update --init` after cloning). `make` takes every tool from `mise.toml` and never downloads its own; outside the devcontainer it stops with a message.

```bash
make reviewable     # generate code and CRDs, go mod tidy, lint, unit tests
make build          # binary, image and package (xpkg) for the host platform
make provider.addtype provider=OpenWebUI group=<group> kind=<Kind>   # scaffold a new kind, then make generate
```

### Throwaway Open WebUI instances

```bash
mise run owui:up 0.12.4        # slim image, SQLite, ENV=dev; prints OWUI_URL and OWUI_API_KEY
mise run owui:openapi 0.12.4   # stores docs/api-notes/openapi/v0.12.4.json
mise run owui:down 0.12.4      # removes the container and its data
```

Each version gets a fixed port (`0.12.4` → `31204`) so several versions can run side by side. The admin credentials are disposable defaults from `hack/openwebui/lib.sh`, overridable through `OWUI_ADMIN_EMAIL`, `OWUI_ADMIN_PASSWORD` and `OWUI_ADMIN_NAME`.

### Podman on macOS

- The bind-mounted workspace keeps the host UID (e.g. 501). `.devcontainer/on-create.sh` gives the `vscode` user that UID so it can write to the workspace; it does nothing where the UIDs already match.
- `devcontainer up` sometimes stops progressing right after `Not setting dockerd DNS manually.` (devcontainer CLI 0.87.0, Podman 5.8.2), with no child process running. Interrupt it and run the same command again: the second run reuses the started container and completes.
