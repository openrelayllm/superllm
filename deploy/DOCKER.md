# Sub2API Admin Plus Docker Image

Sub2API Admin Plus is an operations automation extension built from the Sub2API codebase.

## Quick Start

```bash
docker run -d \
  --name sub2api-admin-plus \
  -p 8080:8080 \
  -e AUTO_SETUP=true \
  -e DATABASE_HOST="postgres-host" \
  -e DATABASE_USER="sub2api_admin_plus" \
  -e DATABASE_PASSWORD="change_this_secure_password" \
  -e DATABASE_DBNAME="sub2api_admin_plus" \
  -e REDIS_HOST="redis-host" \
  ghcr.io/openrelayllm/sub2api-admin-plus:latest
```

## Docker Compose

Use `deploy/docker-compose.local.yml` for a self-contained deployment with local data directories:

```bash
cp .env.example .env
mkdir -p admin_plus_data admin_plus_postgres_data admin_plus_redis_data
docker compose -f docker-compose.local.yml up -d
docker compose -f docker-compose.local.yml logs -f admin-plus
```

## v0.2 Data Boundary

The current v0.2 runtime uses one application database connection. Admin Plus tables use the `admin_plus_*` naming convention, but the runtime does not yet support a separate `ADMIN_PLUS_DATABASE_URL`.

For production, deploy Admin Plus with its own independent PostgreSQL database, for example `sub2api_admin_plus`. Do not point it at an existing production Sub2API database.

Strict sidecar deployment with both a Sub2API readonly database and an Admin Plus write database requires a backend connection split before it can be expressed safely in Compose.

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `AUTO_SETUP` | Recommended | `true` | Auto-initialize config, migrations, and admin user |
| `DATABASE_HOST` | Yes | - | PostgreSQL host |
| `DATABASE_PORT` | No | `5432` | PostgreSQL port |
| `DATABASE_USER` | No | `sub2api_admin_plus` | PostgreSQL user |
| `DATABASE_PASSWORD` | Yes | - | PostgreSQL password |
| `DATABASE_DBNAME` | No | `sub2api_admin_plus` | Independent Admin Plus database |
| `REDIS_HOST` | Yes | - | Redis host |
| `REDIS_PORT` | No | `6379` | Redis port |
| `REDIS_PASSWORD` | No | empty | Redis password |
| `SERVER_PORT` | No | `8080` | Internal server port |
| `ADMIN_EMAIL` | No | `admin@sub2api-admin-plus.local` | Bootstrap admin email |
| `ADMIN_PASSWORD` | No | auto-generated | Bootstrap admin password |
| `JWT_SECRET` | Recommended | auto-generated | Stable JWT secret for persistent sessions |
| `TOTP_ENCRYPTION_KEY` | Recommended | auto-generated | Stable TOTP encryption key |
| `TZ` | No | `Asia/Shanghai` | Runtime timezone |

## Supported Architectures

- `linux/amd64`
- `linux/arm64`

## Links

- [GitHub Repository](https://github.com/openrelayllm/sub2api-admin-plus)
- [Deployment Guide](https://github.com/openrelayllm/sub2api-admin-plus/blob/main/deploy/README.md)
