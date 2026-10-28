# shellcheck shell=bash
# Shared settings for the throwaway Open WebUI instances. Sourced, not executed.
# Every value is a local, disposable dev default and can be overridden from the environment.

OWUI_IMAGE="${OWUI_IMAGE:-ghcr.io/open-webui/open-webui}"
OWUI_SECRET_KEY="${OWUI_SECRET_KEY:-throwaway-secret-key}"
OWUI_ADMIN_EMAIL="${OWUI_ADMIN_EMAIL:-admin@example.com}"
OWUI_ADMIN_PASSWORD="${OWUI_ADMIN_PASSWORD:-throwaway-admin-password}"
OWUI_ADMIN_NAME="${OWUI_ADMIN_NAME:-Provider Admin}"

# The supported matrix of spec #3: one OWUI_MATRIX_<major>_<minor> per supported minor, holding
# the patch release tested. This is the only place it is defined: the contract and e2e runs and
# their CI jobs read it through owui_matrix. Adding a version is adding one such variable with
# its Renovate annotation; Renovate bumps the patches. A new minor that Renovate proposes for an
# existing variable is not approved: it gets its own variable instead.
# renovate: datasource=docker depName=ghcr.io/open-webui/open-webui
OWUI_MATRIX_0_11="0.11.4"
# renovate: datasource=docker depName=ghcr.io/open-webui/open-webui
OWUI_MATRIX_0_12="0.12.4"

# owui_require_version exits unless $1 is a 0.x.y release with x and y below 100 and no
# leading zeros, the range the port scheme of owui_port covers.
owui_require_version() {
  if [[ ! "${1:-}" =~ ^0\.(0|[1-9][0-9]?)\.(0|[1-9][0-9]?)$ ]]; then
    echo "invalid version '${1:-}': expected 0.x.y with x and y below 100, e.g. 0.12.4" >&2
    exit 2
  fi
}

# owui_container prints the container name for version $1.
owui_container() {
  echo "owui-${1//./-}"
}

# owui_port prints a stable host port for version $1: 0.12.4 -> 31204.
owui_port() {
  local minor patch
  IFS=. read -r _ minor patch <<<"$1"
  echo $((30000 + minor * 100 + patch))
}

# owui_url prints the base URL of the instance for version $1. It names 127.0.0.1, not
# localhost: on an IPv6-enabled network such as kind's, the port published on ::1 resets
# connections, and localhost resolves to ::1 first.
owui_url() {
  echo "http://127.0.0.1:$(owui_port "$1")"
}

# owui_matrix prints the versions of the supported matrix, one per line, oldest first.
owui_matrix() {
  local v
  for v in $(compgen -v OWUI_MATRIX_); do
    echo "${!v}"
  done | sort -t. -k1,1n -k2,2n -k3,3n
}

# owui_restricted_env prints the OWUI_EXTRA_ENV of an instance whose API keys are restricted to
# exactly docs/api-notes/api-key-allowed-endpoints.txt (ADR-0001). The settings only apply on a
# database's first boot, so they suit a fresh instance from hack/openwebui/up.sh.
owui_restricted_env() {
  local list
  list="$(dirname "${BASH_SOURCE[0]}")/../../docs/api-notes/api-key-allowed-endpoints.txt"
  echo "ENABLE_API_KEYS_ENDPOINT_RESTRICTIONS=true"
  echo "API_KEYS_ALLOWED_ENDPOINTS=$(paste -sd ',' "$list")"
}
