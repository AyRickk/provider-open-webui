#!/usr/bin/env bash
# Run the chainsaw suite of test/e2e against one Open WebUI version, on the cluster of
# hack/e2e/up.sh with the provider of hack/e2e/deploy.sh.
#   hack/e2e/test.sh <version> [chainsaw test flags, e.g. --include-test-regex providerconfig]
# A fresh Open WebUI instance is started on the kind network and removed on exit. Pods reach
# it through the Service open-webui.open-webui.svc:8080, and the tests through $values:
#   url      the in-cluster URL, for provider configs;
#   apiUrl   the URL from this shell, for direct API calls;
#   apiKey   an admin API key;
#   version  the Open WebUI version, e.g. 0.12.4, and profile its minor, e.g. 0.12.
set -Eeuo pipefail
cd "$(dirname "$0")/../.."
# shellcheck source=hack/e2e/lib.sh
source hack/e2e/lib.sh
# shellcheck source=hack/openwebui/lib.sh
source hack/openwebui/lib.sh

version="${1:-}"
owui_require_version "$version"
shift

work="$(mktemp -d)"
cleanup() {
  hack/openwebui/down.sh "$version" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

OWUI_NETWORK="$E2E_NETWORK" hack/openwebui/up.sh "$version" >"$work/owui.env"
api_url="$(sed -n 's/^OWUI_URL=//p' "$work/owui.env")"
api_key="$(sed -n 's/^OWUI_API_KEY=//p' "$work/owui.env")"
ip="$(e2e_network_ip "$(owui_container "$version")")"

# A Service without selector, whose endpoint is the Open WebUI container.
e2e_kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: $E2E_OWUI_NAMESPACE
---
apiVersion: v1
kind: Service
metadata:
  namespace: $E2E_OWUI_NAMESPACE
  name: $E2E_OWUI_SERVICE
spec:
  ports:
    - name: http
      port: 8080
      targetPort: 8080
---
apiVersion: discovery.k8s.io/v1
kind: EndpointSlice
metadata:
  namespace: $E2E_OWUI_NAMESPACE
  name: $E2E_OWUI_SERVICE
  labels:
    kubernetes.io/service-name: $E2E_OWUI_SERVICE
addressType: IPv4
ports:
  - name: http
    port: 8080
endpoints:
  - addresses: ["$ip"]
EOF

jq -n \
  --arg url "http://$E2E_OWUI_SERVICE.$E2E_OWUI_NAMESPACE.svc:8080" \
  --arg apiUrl "$api_url" \
  --arg apiKey "$api_key" \
  --arg version "$version" \
  --arg profile "${version%.*}" \
  '{url: $url, apiUrl: $apiUrl, apiKey: $apiKey, version: $version, profile: $profile}' \
  >"$work/values.json"

chainsaw test \
  --kube-context "$E2E_CONTEXT" \
  --config test/e2e/.chainsaw.yaml \
  --test-dir test/e2e \
  --values "$work/values.json" \
  "$@"
