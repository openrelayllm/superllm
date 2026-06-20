# Sub2API Admin Plus Deployment

This directory contains Docker and systemd deployment files for Sub2API Admin Plus.

## Supported Deployment In v0.2

Use Admin Plus as an independent application instance with its own PostgreSQL database, for example `sub2api_admin_plus`.

Current v0.2 runtime uses one database connection and applies all migrations through that connection. Admin Plus business tables are named `admin_plus_*`, but there is no separate `ADMIN_PLUS_DATABASE_URL` yet.

Do not point Admin Plus at an existing production Sub2API database. Strict sidecar deployment with:

```text
Sub2API production DB      -> readonly
Admin Plus DB              -> read/write
Shared Redis               -> admin-plus-prefixed writes
```

requires a backend connection split before it can be deployed safely.

## Files

| File | Description |
|------|-------------|
| `docker-compose.local.yml` | Recommended Compose file using local data directories |
| `docker-compose.yml` | Compose file using named Docker volumes |
| `docker-compose.standalone.yml` | App-only Compose file for external PostgreSQL and Redis |
| `.env.example` | Environment variable template |
| `docker-deploy.sh` | One-click Docker deployment preparation script |
| `DOCKER.md` | Docker image usage notes |
| `Dockerfile` | Multi-stage image build file |
| `install.sh` | Legacy binary installer, still inherited from the Sub2API baseline |
| `config.example.yaml` | Example runtime configuration |

## Recommended Docker Deployment

```bash
curl -sSL https://raw.githubusercontent.com/openrelayllm/sub2api-admin-plus/main/deploy/docker-deploy.sh | bash
docker compose up -d
docker compose logs -f admin-plus
```

The script downloads the local-directory Compose file, creates `.env`, generates secrets, and creates:

```text
admin_plus_data/
admin_plus_postgres_data/
admin_plus_redis_data/
```

Open the Web UI at:

```text
http://localhost:8080
```

If `ADMIN_PASSWORD` is empty, the application generates the first admin password on startup. Check logs:

```bash
docker compose logs admin-plus | grep "admin password"
```

## Manual Docker Deployment

```bash
cp .env.example .env
mkdir -p admin_plus_data admin_plus_postgres_data admin_plus_redis_data

# Edit POSTGRES_PASSWORD, JWT_SECRET, TOTP_ENCRYPTION_KEY, ADMIN_PASSWORD, etc.
nano .env

docker compose -f docker-compose.local.yml up -d
docker compose -f docker-compose.local.yml logs -f admin-plus
```

## External PostgreSQL And Redis

Use `docker-compose.standalone.yml` when PostgreSQL and Redis are managed outside this Compose stack:

```bash
cp .env.example .env

# Required for standalone mode
DATABASE_HOST=postgres.example.internal
DATABASE_PORT=5432
DATABASE_USER=sub2api_admin_plus
DATABASE_PASSWORD=change_this_secure_password
DATABASE_DBNAME=sub2api_admin_plus
REDIS_HOST=redis.example.internal
REDIS_PORT=6379

docker compose -f docker-compose.standalone.yml up -d
```

The target database must be an Admin Plus database. Do not use a live Sub2API production database.

## Updating

For the local-directory deployment:

```bash
docker compose pull
docker compose up -d
docker compose logs -f admin-plus
```

For a pinned release image, set `ADMIN_PLUS_IMAGE` in `.env`:

```env
ADMIN_PLUS_IMAGE=ghcr.io/openrelayllm/sub2api-admin-plus:0.3.0
```

## Backups

For the local-directory deployment, stop the stack and archive the deployment directory:

```bash
docker compose down
tar czf sub2api-admin-plus-deploy.tar.gz .
```

At minimum, back up:

```text
.env
admin_plus_data/
admin_plus_postgres_data/
admin_plus_redis_data/
```

## Environment Notes

Important variables:

| Variable | Purpose |
|----------|---------|
| `ADMIN_PLUS_IMAGE` | Docker image to run |
| `POSTGRES_DB` | Admin Plus database, defaults to `sub2api_admin_plus` |
| `POSTGRES_USER` | PostgreSQL user, defaults to `sub2api_admin_plus` |
| `POSTGRES_PASSWORD` | Required PostgreSQL password |
| `JWT_SECRET` | Stable JWT secret |
| `TOTP_ENCRYPTION_KEY` | Stable TOTP encryption key |
| `ADMIN_EMAIL` | Bootstrap admin email |
| `ADMIN_PASSWORD` | Bootstrap admin password, auto-generated if empty |
| `REDIS_PASSWORD` | Redis password, optional for local deployments |

## Current Limitations

- `ADMIN_PLUS_DATABASE_URL` is not supported yet.
- `SUB2API_READONLY_DATABASE_URL` is not supported yet.
- Admin Plus repositories currently use the main application database connection.
- Redis prefix isolation is partially implemented through existing cache prefixes; do not share a Redis instance with untrusted workloads until the dedicated prefix wrapper is complete.
