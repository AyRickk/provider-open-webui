FROM mcr.microsoft.com/devcontainers/base:2.0.5-ubuntu-24.04@sha256:d94c97dd9cacf183d0a6fd12a8e87b526e9e928307674ae9c94139139c0c6eae

# mise itself is pinned here; every other tool is pinned in mise.toml.
ARG MISE_VERSION=v2026.10.8

RUN curl -fsSL https://mise.run \
      | MISE_VERSION="${MISE_VERSION}" MISE_INSTALL_PATH=/usr/local/bin/mise sh \
 && mise --version

# Pre-create the mise data directory so the named volume mounted over it is owned by vscode.
RUN mkdir -p /home/vscode/.local/share/mise \
 && chown -R vscode:vscode /home/vscode/.local \
 && echo 'eval "$(mise activate bash)"' >> /home/vscode/.bashrc

ENV MISE_DATA_DIR=/home/vscode/.local/share/mise \
    PATH=/home/vscode/.local/share/mise/shims:${PATH}
