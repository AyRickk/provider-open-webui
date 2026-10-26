# shellcheck shell=bash
# Shared settings for the throwaway Open WebUI instances. Sourced, not executed.
# Every value is a local, disposable dev default and can be overridden from the environment.

OWUI_IMAGE="${OWUI_IMAGE:-ghcr.io/open-webui/open-webui}"
OWUI_SECRET_KEY="${OWUI_SECRET_KEY:-throwaway-secret-key}"
OWUI_ADMIN_EMAIL="${OWUI_ADMIN_EMAIL:-admin@example.com}"
OWUI_ADMIN_PASSWORD="${OWUI_ADMIN_PASSWORD:-throwaway-admin-password}"
OWUI_ADMIN_NAME="${OWUI_ADMIN_NAME:-Provider Admin}"

# owui_require_version exits unless $1 is a 0.x.y release with x and y below 100 and no
# leading zeros, the range the port scheme of owui_port covers.
owui_require_version() {
  if [[ ! "${1:-}" =~ ^0\.(0|[1-9][0-9]?)\.(0|[1-9][0-9]?)$ ]]; then
    echo "usage: mise run owui:<up|down|openapi> <version>   e.g. 0.12.4 (0.x.y, x and y below 100)" >&2
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

# owui_url prints the base URL of the instance for version $1.
owui_url() {
  echo "http://localhost:$(owui_port "$1")"
}
