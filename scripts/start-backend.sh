#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"

SERVER_HOST="${SERVER_HOST:-127.0.0.1}"
SERVER_PORT="${SERVER_PORT:-8080}"
DATA_DIR="${DATA_DIR:-${ROOT_DIR}/.local/admin-plus-data}"

DATABASE_HOST="${DATABASE_HOST:-127.0.0.1}"
DATABASE_PORT="${DATABASE_PORT:-5432}"
DATABASE_USER="${DATABASE_USER:-root}"
DATABASE_PASSWORD="${DATABASE_PASSWORD:-root}"
DATABASE_DBNAME="${DATABASE_DBNAME:-sub2api_admin_plus}"
DATABASE_SSLMODE="${DATABASE_SSLMODE:-disable}"

REDIS_HOST="${REDIS_HOST:-127.0.0.1}"
REDIS_PORT="${REDIS_PORT:-6379}"
REDIS_PASSWORD="${REDIS_PASSWORD:-}"
REDIS_DB="${REDIS_DB:-0}"

ADMIN_EMAIL="${ADMIN_EMAIL:-admin@sub2api-admin-plus.local}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-AdminPlus@123456}"
JWT_SECRET="${JWT_SECRET:-admin-plus-local-dev-jwt-secret-32}"

export AUTO_SETUP="${AUTO_SETUP:-true}"
export SERVER_HOST SERVER_PORT SERVER_MODE="${SERVER_MODE:-debug}"
export DATA_DIR
export DATABASE_HOST DATABASE_PORT DATABASE_USER DATABASE_PASSWORD DATABASE_DBNAME DATABASE_SSLMODE
export REDIS_HOST REDIS_PORT REDIS_PASSWORD REDIS_DB
export ADMIN_EMAIL ADMIN_PASSWORD JWT_SECRET
export ADMIN_PLUS_SCHEDULER_ENABLED="${ADMIN_PLUS_SCHEDULER_ENABLED:-false}"
export SUB2API_READONLY_REDIS_DB="${SUB2API_READONLY_REDIS_DB:-${REDIS_DB}}"
export TZ="${TZ:-Asia/Shanghai}"

check_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

check_port() {
  local port="$1"
  if lsof -iTCP:"${port}" -sTCP:LISTEN -n -P >/dev/null 2>&1; then
    echo "port ${port} is already in use. Set SERVER_PORT to another port." >&2
    exit 1
  fi
}

check_command go
check_command lsof
mkdir -p "${DATA_DIR}"
check_port "${SERVER_PORT}"

cat <<EOF
Admin Plus backend
  URL:      http://${SERVER_HOST}:${SERVER_PORT}
  data:     ${DATA_DIR}
  database: ${DATABASE_USER}@${DATABASE_HOST}:${DATABASE_PORT}/${DATABASE_DBNAME}
  redis:    ${REDIS_HOST}:${REDIS_PORT}/${REDIS_DB}
  admin:    ${ADMIN_EMAIL} / ${ADMIN_PASSWORD}
EOF

cd "${BACKEND_DIR}"
exec go run ./cmd/server
