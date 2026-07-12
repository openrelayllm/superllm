#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

SERVER_PORT="${SERVER_PORT:-3010}"
DATA_DIR="${DATA_DIR:-${ROOT_DIR}/.local/admin-plus-e2e-data}"
ADMIN_PLUS_BASE_URL="${ADMIN_PLUS_BASE_URL:-http://127.0.0.1:${SERVER_PORT}}"
ADMIN_PLUS_E2E_DB_URL="${ADMIN_PLUS_E2E_DB_URL:-postgresql://root:root@127.0.0.1:5432/superllm?sslmode=disable}"
ADMIN_PLUS_E2E_REDIS_URL="${ADMIN_PLUS_E2E_REDIS_URL:-redis://127.0.0.1:6379/0}"

export SERVER_PORT DATA_DIR
export ADMIN_PLUS_BASE_URL ADMIN_PLUS_E2E_DB_URL ADMIN_PLUS_E2E_REDIS_URL
export ADMIN_PLUS_E2E_EMAIL="${ADMIN_PLUS_E2E_EMAIL:-admin@superllm.local}"
export ADMIN_PLUS_E2E_PASSWORD="${ADMIN_PLUS_E2E_PASSWORD:-AdminPlus@123456}"
export ADMIN_PLUS_SCHEDULER_ENABLED="${ADMIN_PLUS_SCHEDULER_ENABLED:-false}"

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

echo "waiting for backend on ${ADMIN_PLUS_BASE_URL}"
for _ in $(seq 1 90); do
  if curl -fsS "${ADMIN_PLUS_BASE_URL}/setup/status" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! curl -fsS "${ADMIN_PLUS_BASE_URL}/setup/status" >/dev/null 2>&1; then
  echo "backend did not become ready on ${ADMIN_PLUS_BASE_URL}" >&2
  exit 1
fi

node "${ROOT_DIR}/tools/admin-plus-e2e.mjs"
