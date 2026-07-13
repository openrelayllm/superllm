#!/bin/bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
INSTALLER="$SCRIPT_DIR/install.sh"

bash -n "$INSTALLER"

if grep -Eq 'UPSTREAM_(INSTALL|CONFIG|SERVICE|BINARY)' "$INSTALLER"; then
    echo "install.sh must not classify the co-located Sub2API service as an upgrade source" >&2
    exit 1
fi

if grep -Eq 'systemctl (stop|disable).*sub2api([. ]|$)' "$INSTALLER"; then
    echo "install.sh must not stop or disable the authoritative Sub2API service" >&2
    exit 1
fi

SUPERLLM_INSTALLER_LIB_ONLY=1 source "$INSTALLER"

is_interactive() {
    return 1
}

port_is_listening() {
    [ "$1" = "8080" ]
}

SERVER_HOST="127.0.0.1"
SERVER_PORT="8080"
SERVER_PORT_EXPLICIT=false
configure_server >/dev/null
if [ "$SERVER_PORT" != "8081" ]; then
    echo "install.sh must choose an available port when the default is occupied" >&2
    exit 1
fi

if (
    SERVER_PORT="8080"
    SERVER_PORT_EXPLICIT=true
    configure_server >/dev/null 2>&1
); then
    echo "install.sh must reject an explicitly selected occupied port" >&2
    exit 1
fi

echo "install safety checks passed"
