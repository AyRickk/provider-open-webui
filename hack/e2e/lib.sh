# shellcheck shell=bash
# shellcheck disable=SC2034 # the settings are read by the scripts that source this file
# Shared settings for the end-to-end harness. Sourced, not executed.
# The harness runs in the devcontainer's Docker daemon: a kind cluster, a local registry for
# the provider package, and one Open WebUI container per tested version, on the kind network.

E2E_CLUSTER="${E2E_CLUSTER:-provider-openwebui-e2e}"
E2E_CONTEXT="kind-$E2E_CLUSTER"
# kind creates this Docker network; the registry and the Open WebUI containers join it.
E2E_NETWORK=kind

# The registry serves the provider package to Crossplane and its runtime image to containerd.
# Both reach it by its IP on the kind network: Crossplane talks plain HTTP to a private
# (RFC 1918) address, and containerd is told to through a hosts.toml.
E2E_REGISTRY="${E2E_CLUSTER}-registry"
E2E_REGISTRY_PORT=5000
# The registry's port on the devcontainer's 127.0.0.1, where the package is pushed from.
E2E_REGISTRY_HOST_PORT="${E2E_REGISTRY_HOST_PORT:-5001}"
# renovate: datasource=docker depName=registry
E2E_REGISTRY_TAG="3.0.0@sha256:6c5666b861f3505b116bb9aa9b25175e71210414bd010d92035ff64018f9457e"
E2E_REGISTRY_IMAGE="registry:$E2E_REGISTRY_TAG"

# renovate: datasource=helm depName=crossplane registryUrl=https://charts.crossplane.io/stable
E2E_CROSSPLANE_VERSION="2.4.3"
E2E_CROSSPLANE_NAMESPACE=crossplane-system

E2E_PROVIDER=provider-openwebui

# The supported Open WebUI matrix of spec #3, as in the e2e job of .github/workflows/ci.yml.
# renovate: datasource=docker depName=ghcr.io/open-webui/open-webui
E2E_OWUI_PREVIOUS="0.11.4"
# renovate: datasource=docker depName=ghcr.io/open-webui/open-webui
E2E_OWUI_LATEST="0.12.4"

# The in-cluster Service in front of the Open WebUI container under test.
E2E_OWUI_NAMESPACE=open-webui
E2E_OWUI_SERVICE=open-webui

# e2e_kubectl runs kubectl against the e2e cluster, whatever the current context.
e2e_kubectl() {
  kubectl --context "$E2E_CONTEXT" "$@"
}

# e2e_network_ip prints the IPv4 address of container $1 on the kind network.
e2e_network_ip() {
  docker inspect --format "{{(index .NetworkSettings.Networks \"$E2E_NETWORK\").IPAddress}}" "$1"
}
