#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

SERVER_PORT="${SERVER_PORT:-8080}"
FRONTEND_PORT="${FRONTEND_PORT:-3000}"
STOP_EXISTING_DEV="${STOP_EXISTING_DEV:-true}"

PIDS=()

check_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

listener_pids() {
  local port="$1"
  lsof -tiTCP:"${port}" -sTCP:LISTEN -n -P 2>/dev/null | sort -u || true
}

process_cwd() {
  local pid="$1"
  lsof -a -p "${pid}" -d cwd -Fn 2>/dev/null | sed -n 's/^n//p' | head -n 1
}

process_command() {
  local pid="$1"
  ps -p "${pid}" -o command= 2>/dev/null || true
}

is_repo_process() {
  local pid="$1"
  local cwd
  cwd="$(process_cwd "${pid}")"
  case "${cwd}" in
    "${ROOT_DIR}"|"${ROOT_DIR}"/*)
      return 0
      ;;
  esac
  case "$(process_command "${pid}")" in
    *"${ROOT_DIR}"*)
      return 0
      ;;
  esac
  return 1
}

child_pids() {
  local pid="$1"
  if command -v pgrep >/dev/null 2>&1; then
    pgrep -P "${pid}" 2>/dev/null || true
  fi
}

terminate_tree() {
  local pid="$1"
  local child
  if ! kill -0 "${pid}" >/dev/null 2>&1; then
    return
  fi
  for child in $(child_pids "${pid}"); do
    terminate_tree "${child}"
  done
  kill "${pid}" >/dev/null 2>&1 || true
}

force_kill_if_alive() {
  local pid="$1"
  if kill -0 "${pid}" >/dev/null 2>&1; then
    kill -9 "${pid}" >/dev/null 2>&1 || true
  fi
}

wait_for_port_free() {
  local port="$1"
  local i
  for i in $(seq 1 20); do
    if [ -z "$(listener_pids "${port}")" ]; then
      return 0
    fi
    sleep 0.25
  done
  return 1
}

stop_existing_dev_on_port() {
  local port="$1"
  local label="$2"
  local pid
  local pids
  pids="$(listener_pids "${port}")"
  if [ -z "${pids}" ]; then
    return
  fi
  if [ "${STOP_EXISTING_DEV}" != "true" ]; then
    echo "port ${port} is already in use. Set ${label}_PORT to another port or STOP_EXISTING_DEV=true." >&2
    return 1
  fi
  for pid in ${pids}; do
    if ! is_repo_process "${pid}"; then
      echo "port ${port} is used by a non-repo process; not stopping it:" >&2
      echo "  pid ${pid}: $(process_command "${pid}")" >&2
      return 1
    fi
    echo "stopping existing ${label} dev process on port ${port} (pid ${pid})"
    terminate_tree "${pid}"
  done
  sleep 0.5
  for pid in ${pids}; do
    force_kill_if_alive "${pid}"
  done
  if ! wait_for_port_free "${port}"; then
    echo "port ${port} is still in use after stopping ${label} dev process" >&2
    return 1
  fi
}

cleanup() {
  local pid
  for pid in "${PIDS[@]}"; do
    terminate_tree "${pid}"
  done
  sleep 0.5
  for pid in "${PIDS[@]}"; do
    force_kill_if_alive "${pid}"
  done
}

trap cleanup EXIT INT TERM

check_command lsof
stop_existing_dev_on_port "${SERVER_PORT}" "SERVER"
stop_existing_dev_on_port "${FRONTEND_PORT}" "FRONTEND"

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
