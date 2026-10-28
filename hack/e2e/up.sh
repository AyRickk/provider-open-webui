#!/usr/bin/env bash
# Create the e2e cluster: kind, a local registry on the kind network, and Crossplane.
# Every step is skipped when already done, so running it again is harmless.
# The provider itself is installed by hack/e2e/deploy.sh.
set -Eeuo pipefail
cd "$(dirname "$0")/../.."
# shellcheck source=hack/e2e/lib.sh
source hack/e2e/lib.sh

if ! kind get clusters 2>/dev/null | grep -qx "$E2E_CLUSTER"; then
  kind create cluster --name "$E2E_CLUSTER" --config hack/e2e/kind.yaml --wait 2m
fi

if ! docker container inspect "$E2E_REGISTRY" >/dev/null 2>&1; then
  docker run --detach --name "$E2E_REGISTRY" --restart always \
    --network "$E2E_NETWORK" \
    --publish "127.0.0.1:$E2E_REGISTRY_HOST_PORT:$E2E_REGISTRY_PORT" \
    "$E2E_REGISTRY_IMAGE" >/dev/null
fi

# containerd pulls the provider's runtime image from the registry over plain HTTP.
registry="$(e2e_network_ip "$E2E_REGISTRY"):$E2E_REGISTRY_PORT"
for node in $(kind get nodes --name "$E2E_CLUSTER"); do
  docker exec "$node" mkdir -p "/etc/containerd/certs.d/$registry"
  printf '[host."http://%s"]\n  capabilities = ["pull", "resolve"]\n' "$registry" \
    | docker exec -i "$node" tee "/etc/containerd/certs.d/$registry/hosts.toml" >/dev/null
done

helm upgrade --install crossplane crossplane \
  --kube-context "$E2E_CONTEXT" \
  --repo https://charts.crossplane.io/stable --version "$E2E_CROSSPLANE_VERSION" \
  --namespace "$E2E_CROSSPLANE_NAMESPACE" --create-namespace \
  --wait --timeout 5m

echo "cluster $E2E_CLUSTER ready (context $E2E_CONTEXT, Crossplane $E2E_CROSSPLANE_VERSION)"
