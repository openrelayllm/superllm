#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="${ROOT_DIR}/frontend"

FRONTEND_HOST="${FRONTEND_HOST:-0.0.0.0}"
FRONTEND_PORT="${FRONTEND_PORT:-3000}"
SERVER_PORT="${SERVER_PORT:-8080}"
VITE_DEV_PROXY_TARGET="${VITE_DEV_PROXY_TARGET:-http://127.0.0.1:${SERVER_PORT}}"
VITE_DEV_PORT="${VITE_DEV_PORT:-${FRONTEND_PORT}}"

export VITE_DEV_PROXY_TARGET VITE_DEV_PORT

check_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

check_port() {
  local port="$1"
  if lsof -iTCP:"${port}" -sTCP:LISTEN -n -P >/dev/null 2>&1; then
    echo "port ${port} is already in use. Set FRONTEND_PORT to another port." >&2
    exit 1
  fi
}

check_command pnpm
check_command lsof
check_port "${FRONTEND_PORT}"

if [ ! -d "${FRONTEND_DIR}/node_modules" ]; then
  echo "frontend dependencies are missing; running pnpm install"
  pnpm --dir "${FRONTEND_DIR}" install
fi

cat <<EOF
Admin Plus frontend
  URL:     http://127.0.0.1:${FRONTEND_PORT}
  backend: ${VITE_DEV_PROXY_TARGET}
EOF

exec pnpm --dir "${FRONTEND_DIR}" run dev -- --host "${FRONTEND_HOST}"
