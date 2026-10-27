#!/usr/bin/env bash
# Prepare the devcontainer: align the vscode UID with the workspace owner, then install the
# tools pinned in mise.toml into the mise volume.
#
# updateRemoteUserUID only aligns UIDs on Linux hosts; on macOS with Podman the bind-mounted
# workspace keeps the host UID (e.g. 501) and vscode (1000) cannot write to it. /etc/passwd is
# edited directly because usermod refuses while this very shell runs as vscode. After the edit
# the current UID no longer exists, so the rest runs inside a single root shell, and mise
# runs as the renamed vscode user. The devcontainer CLI skips postCreateCommand once the UID
# has changed, which is why the install lives here.
set -euo pipefail

ws_uid="$(stat -c %u "$PWD")"
install="cd $(printf %q "$PWD") && git submodule update --init --recursive && mise trust --yes && mise install --yes"

if [[ "$ws_uid" == 0 || "$ws_uid" == "$(id -u)" ]]; then
  exec bash -c "$install"
fi

# sudo and su reset the environment, so HOME and MISE_DATA_DIR are passed explicitly.
as_vscode="export HOME=/home/vscode MISE_DATA_DIR=$(printf %q "$MISE_DATA_DIR") && $install"
sudo bash -c "
  chown -R $ws_uid /home/vscode &&
  sed -i 's/^vscode:x:$(id -u):/vscode:x:$ws_uid:/' /etc/passwd &&
  echo 'vscode now has UID $ws_uid, matching the workspace owner' &&
  su vscode -c $(printf %q "$as_vscode")
"
