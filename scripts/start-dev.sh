#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

SERVER_PORT="${SERVER_PORT:-8080}"
FRONTEND_PORT="${FRONTEND_PORT:-3000}"

PIDS=()

cleanup() {
  for pid in "${PIDS[@]}"; do
    if kill -0 "${pid}" >/dev/null 2>&1; then
      kill "${pid}" >/dev/null 2>&1 || true
    fi
  done
}

trap cleanup EXIT INT TERM

bash "${ROOT_DIR}/scripts/start-backend.sh" &
PIDS+=("$!")

echo "waiting for backend on http://127.0.0.1:${SERVER_PORT}"
for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:${SERVER_PORT}/setup/status" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! curl -fsS "http://127.0.0.1:${SERVER_PORT}/setup/status" >/dev/null 2>&1; then
  echo "backend did not become ready on port ${SERVER_PORT}" >&2
  exit 1
fi

bash "${ROOT_DIR}/scripts/start-frontend.sh" &
PIDS+=("$!")

cat <<EOF
Admin Plus dev stack is starting.
  frontend: http://127.0.0.1:${FRONTEND_PORT}
  backend:  http://127.0.0.1:${SERVER_PORT}

Press Ctrl+C to stop both processes.
EOF

wait
