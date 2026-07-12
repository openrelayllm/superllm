# SuperLLM Docker Image

SuperLLM is an operations automation extension built from the Sub2API codebase.

Release images are published for `linux/amd64` and `linux/arm64` to:

- DockerHub: `wutongci/superllm:<version>`
- GHCR: `ghcr.io/openrelayllm/superllm:<version>`

## Quick Start

```bash
docker run -d \
  --name superllm \
  -p 8080:8080 \
  -e AUTO_SETUP=true \
  -e DATABASE_HOST="postgres-host" \
  -e DATABASE_USER="superllm" \
  -e DATABASE_PASSWORD="change_this_secure_password" \
  -e DATABASE_DBNAME="superllm" \
  -e REDIS_HOST="redis-host" \
  wutongci/superllm:latest
```

## Docker Compose

Use `deploy/docker-compose.local.yml` for a self-contained deployment with local data directories:

```bash
cp .env.example .env
mkdir -p admin_plus_data admin_plus_postgres_data admin_plus_redis_data
docker compose -f docker-compose.local.yml up -d
docker compose -f docker-compose.local.yml logs -f superllm
```

To pin a release image, set `ADMIN_PLUS_IMAGE=wutongci/superllm:X.Y.Z` in `.env`.

## v0.2 Data Boundary

The current v0.2 runtime uses one application database connection. SuperLLM tables use the `admin_plus_*` naming convention, but the runtime does not yet support a separate `ADMIN_PLUS_DATABASE_URL`.

For production, deploy SuperLLM with its own independent PostgreSQL database named `superllm`. Do not point it at an existing production Sub2API database.

Strict sidecar deployment with both a Sub2API readonly database and an SuperLLM write database requires a backend connection split before it can be expressed safely in Compose.

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `AUTO_SETUP` | Recommended | `true` | Auto-initialize config and migrations |
| `DATABASE_HOST` | Yes | - | PostgreSQL host |
| `DATABASE_PORT` | No | `5432` | PostgreSQL port |
| `DATABASE_USER` | No | `superllm` | PostgreSQL user |
| `DATABASE_PASSWORD` | Yes | - | PostgreSQL password |
| `DATABASE_DBNAME` | No | `superllm` | Independent SuperLLM database |
| `REDIS_HOST` | Yes | - | Redis host |
| `REDIS_PORT` | No | `6379` | Redis port |
| `REDIS_PASSWORD` | No | empty | Redis password |
| `SERVER_PORT` | No | `8080` | Internal server port |
| `SUB2API_READONLY_DATABASE_URL` | Yes | - | Readonly Sub2API identity and data source |
| `JWT_SECRET` | Recommended | auto-generated | Stable JWT secret for persistent sessions |
| `TOTP_ENCRYPTION_KEY` | Recommended | auto-generated | Stable TOTP encryption key |
| `TZ` | No | `Asia/Shanghai` | Runtime timezone |

## Supported Architectures

- `linux/amd64`
- `linux/arm64`

## Links

- [GitHub Repository](https://github.com/openrelayllm/superllm)
- [Deployment Guide](https://github.com/openrelayllm/superllm/blob/main/deploy/README.md)
