#!/usr/bin/env bash

ROOT_DIR="${ROOT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"

DEV_INFRA_DIR="${DEV_INFRA_DIR:-${ROOT_DIR}/.local/infra}"
DEV_POSTGRES_DATA_DIR="${DEV_POSTGRES_DATA_DIR:-${DEV_INFRA_DIR}/postgres}"
DEV_REDIS_DATA_DIR="${DEV_REDIS_DATA_DIR:-${DEV_INFRA_DIR}/redis}"

export DATABASE_HOST="${DATABASE_HOST:-127.0.0.1}"
export DATABASE_PORT="${DATABASE_PORT:-5432}"
export DATABASE_USER="${DATABASE_USER:-root}"
export DATABASE_PASSWORD="${DATABASE_PASSWORD:-root}"
export DATABASE_DBNAME="${DATABASE_DBNAME:-superllm}"
export DATABASE_SSLMODE="${DATABASE_SSLMODE:-disable}"

export REDIS_HOST="${REDIS_HOST:-127.0.0.1}"
export REDIS_PORT="${REDIS_PORT:-6379}"
export REDIS_PASSWORD="${REDIS_PASSWORD:-}"
export REDIS_DB="${REDIS_DB:-0}"

DEV_INFRA_STARTED_POSTGRES=false
DEV_INFRA_STARTED_REDIS=false

dev_infra_find_command() {
  local name="$1"
  local candidate
  if command -v "${name}" >/dev/null 2>&1; then
    command -v "${name}"
    return 0
  fi
  for candidate in \
    "/opt/homebrew/bin/${name}" \
    "/opt/homebrew/opt/postgresql@16/bin/${name}" \
    "/opt/homebrew/opt/postgresql@15/bin/${name}" \
    "/usr/local/bin/${name}" \
    "/usr/local/opt/postgresql@16/bin/${name}" \
    "/usr/local/opt/postgresql@15/bin/${name}"
  do
    if [ -x "${candidate}" ]; then
      echo "${candidate}"
      return 0
    fi
  done
  return 1
}

dev_infra_port_listening() {
  local port="$1"
  lsof -iTCP:"${port}" -sTCP:LISTEN -n -P >/dev/null 2>&1
}

dev_infra_wait_tcp() {
  local host="$1"
  local port="$2"
  local i
  for i in $(seq 1 60); do
    if (echo >"/dev/tcp/${host}/${port}") >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done
  return 1
}

dev_infra_psql() {
  local psql_bin="$1"
  shift
  PGCONNECT_TIMEOUT=1 PGPASSWORD="${DATABASE_PASSWORD}" "${psql_bin}" -w "$@"
}

dev_infra_wait_postgres() {
  local psql_bin="$1"
  local i
  for i in $(seq 1 20); do
    if dev_infra_psql "${psql_bin}" -h "${DATABASE_HOST}" -p "${DATABASE_PORT}" -U "${DATABASE_USER}" -d postgres -Atqc "SELECT 1" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done
  return 1
}

dev_infra_sql_literal() {
  printf "%s" "$1" | sed "s/'/''/g"
}

dev_infra_sql_identifier() {
  printf "%s" "$1" | sed 's/"/""/g'
}

dev_infra_ensure_postgres_database() {
  local psql_bin="$1"
  local db_literal db_identifier
  db_literal="$(dev_infra_sql_literal "${DATABASE_DBNAME}")"
  db_identifier="$(dev_infra_sql_identifier "${DATABASE_DBNAME}")"
  if ! dev_infra_psql "${psql_bin}" -h "${DATABASE_HOST}" -p "${DATABASE_PORT}" -U "${DATABASE_USER}" -d postgres -Atqc "SELECT 1 FROM pg_database WHERE datname = '${db_literal}'" | grep -q 1; then
    dev_infra_psql "${psql_bin}" -h "${DATABASE_HOST}" -p "${DATABASE_PORT}" -U "${DATABASE_USER}" -d postgres -c "CREATE DATABASE \"${db_identifier}\"" >/dev/null
  fi
}

dev_infra_start_postgres() {
  local psql_bin
  psql_bin="$(dev_infra_find_command psql)" || {
    echo "missing psql. Install PostgreSQL command-line tools, or set START_LOCAL_INFRA=false and DATABASE_* to an existing PostgreSQL. Docker is not used." >&2
    return 1
  }

  if dev_infra_port_listening "${DATABASE_PORT}"; then
    if dev_infra_wait_postgres "${psql_bin}"; then
      dev_infra_ensure_postgres_database "${psql_bin}"
      echo "using existing PostgreSQL on ${DATABASE_HOST}:${DATABASE_PORT}"
      return 0
    fi
    if dev_infra_port_listening "${DATABASE_PORT}"; then
      echo "PostgreSQL port ${DATABASE_PORT} is in use but not ready for psql connections." >&2
      echo "Stop that process, or set DATABASE_PORT to another port. Docker is not used." >&2
      return 1
    fi
  fi

  local initdb_bin pg_ctl_bin
  initdb_bin="$(dev_infra_find_command initdb)" || {
    echo "missing initdb. Install PostgreSQL command-line tools, or set DATABASE_* to an existing PostgreSQL. Docker is not used." >&2
    return 1
  }
  pg_ctl_bin="$(dev_infra_find_command pg_ctl)" || {
    echo "missing pg_ctl. Install PostgreSQL command-line tools, or set DATABASE_* to an existing PostgreSQL. Docker is not used." >&2
    return 1
  }

  mkdir -p "${DEV_POSTGRES_DATA_DIR}" "${DEV_INFRA_DIR}/logs"
  if [ ! -f "${DEV_POSTGRES_DATA_DIR}/PG_VERSION" ]; then
    "${initdb_bin}" -D "${DEV_POSTGRES_DATA_DIR}" -A trust -U "${DATABASE_USER}" >/dev/null
  fi

  echo "starting local PostgreSQL on ${DATABASE_HOST}:${DATABASE_PORT}"
  "${pg_ctl_bin}" -D "${DEV_POSTGRES_DATA_DIR}" \
    -l "${DEV_INFRA_DIR}/logs/postgres.log" \
    -o "-h ${DATABASE_HOST} -p ${DATABASE_PORT}" \
    start >/dev/null
  DEV_INFRA_STARTED_POSTGRES=true

  dev_infra_wait_postgres "${psql_bin}" || {
    echo "local PostgreSQL did not become ready" >&2
    return 1
  }

  dev_infra_ensure_postgres_database "${psql_bin}"
}

dev_infra_start_redis() {
  if dev_infra_port_listening "${REDIS_PORT}"; then
    echo "using existing Redis on ${REDIS_HOST}:${REDIS_PORT}"
    return 0
  fi

  local redis_server_bin redis_cli_bin
  redis_server_bin="$(dev_infra_find_command redis-server)" || {
    echo "missing redis-server. Install Redis, or set REDIS_* to an existing Redis. Docker is not used." >&2
    return 1
  }
  redis_cli_bin="$(dev_infra_find_command redis-cli)" || true

  mkdir -p "${DEV_REDIS_DATA_DIR}" "${DEV_INFRA_DIR}/logs"
  echo "starting local Redis on ${REDIS_HOST}:${REDIS_PORT}"
  "${redis_server_bin}" \
    --bind "${REDIS_HOST}" \
    --port "${REDIS_PORT}" \
    --dir "${DEV_REDIS_DATA_DIR}" \
    --save 60 1 \
    --appendonly yes \
    --daemonize yes \
    --pidfile "${DEV_REDIS_DATA_DIR}/redis.pid" \
    --logfile "${DEV_INFRA_DIR}/logs/redis.log" >/dev/null
  DEV_INFRA_STARTED_REDIS=true

  dev_infra_wait_tcp "${REDIS_HOST}" "${REDIS_PORT}" || {
    echo "local Redis did not become ready" >&2
    return 1
  }
  if [ -n "${redis_cli_bin}" ]; then
    "${redis_cli_bin}" -h "${REDIS_HOST}" -p "${REDIS_PORT}" ping >/dev/null 2>&1 || true
  fi
}

dev_infra_start() {
  dev_infra_start_postgres
  dev_infra_start_redis
}

dev_infra_stop() {
  if [ "${DEV_INFRA_STARTED_REDIS}" = "true" ] && [ -f "${DEV_REDIS_DATA_DIR}/redis.pid" ]; then
    kill "$(cat "${DEV_REDIS_DATA_DIR}/redis.pid")" >/dev/null 2>&1 || true
  fi
  if [ "${DEV_INFRA_STARTED_POSTGRES}" = "true" ]; then
    local pg_ctl_bin
    pg_ctl_bin="$(dev_infra_find_command pg_ctl)" || return 0
    "${pg_ctl_bin}" -D "${DEV_POSTGRES_DATA_DIR}" stop -m fast >/dev/null 2>&1 || true
  fi
}
