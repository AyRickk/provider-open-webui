#!/usr/bin/env bash
# Install the provider package built by `make build` into the e2e cluster, or upgrade it.
# The package is pushed to the local registry under a tag derived from its content, so a
# rebuilt package always rolls out a new revision.
set -Eeuo pipefail
cd "$(dirname "$0")/../.."
# shellcheck source=hack/e2e/lib.sh
source hack/e2e/lib.sh

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) echo "unsupported architecture $(uname -m)" >&2; exit 1 ;;
esac

# The newest package for this architecture: make build names it after the git version.
xpkg="$(find "_output/xpkg/linux_$arch" -name "$E2E_PROVIDER-*.xpkg" -printf '%T@ %p\n' 2>/dev/null \
  | sort -n | tail -n 1 | cut -d ' ' -f 2- || true)"
if [[ -z "$xpkg" ]]; then
  echo "no package in _output/xpkg/linux_$arch: run make build first" >&2
  exit 1
fi

tag="e2e-$(sha256sum "$xpkg" | cut -c1-12)"
crossplane xpkg push --package-files "$xpkg" "127.0.0.1:$E2E_REGISTRY_HOST_PORT/$E2E_PROVIDER:$tag"

package="$(e2e_network_ip "$E2E_REGISTRY"):$E2E_REGISTRY_PORT/$E2E_PROVIDER:$tag"
e2e_kubectl apply -f - <<EOF
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: $E2E_PROVIDER
spec:
  package: $package
EOF

echo "waiting for $package to become healthy ..." >&2
e2e_kubectl wait "provider.pkg.crossplane.io/$E2E_PROVIDER" \
  --for "jsonpath={.status.currentIdentifier}=$package" --timeout 2m
e2e_kubectl wait "provider.pkg.crossplane.io/$E2E_PROVIDER" \
  --for condition=Installed --timeout 2m
e2e_kubectl wait "provider.pkg.crossplane.io/$E2E_PROVIDER" \
  --for condition=Healthy --timeout 3m
echo "provider $package deployed ($xpkg)"
