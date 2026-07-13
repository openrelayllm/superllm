# SuperLLM Deployment

This directory contains Docker and systemd deployment files for SuperLLM.

## Supported Deployment

Use SuperLLM as an independent application instance with its own PostgreSQL database named `superllm`.

Do not point SuperLLM at an existing production Sub2API database. Use this boundary instead:

```text
SuperLLM DB              -> read/write, stores SuperLLM tables and baseline runtime tables
SuperLLM Redis           -> read/write cache for SuperLLM
Sub2API production DB      -> required readonly identity/data source
Sub2API runtime Redis      -> readonly through SUB2API_READONLY_REDIS_URL or SUB2API_READONLY_REDIS_DB
Sub2API Admin API          -> write/provision through ADMIN_PLUS_SUB2API_ADMIN_BASE_URL and API key
```

The Sub2API readonly database user should have only `SELECT` on `users` and the required runtime tables such as `accounts`, `groups`, `account_groups`, and `usage_logs`. SuperLLM does not create or update users; only active Sub2API administrators can sign in.

Example PostgreSQL grants, executed by the Sub2API database owner:

```sql
CREATE ROLE superllm_readonly LOGIN PASSWORD 'replace-with-a-strong-password';
GRANT CONNECT ON DATABASE sub2api TO superllm_readonly;
GRANT USAGE ON SCHEMA public TO superllm_readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO superllm_readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO superllm_readonly;
```

## Method 1: Script Installation (Recommended)

The one-click installer downloads pre-built Linux binaries from GitHub Releases.

### Prerequisites

- Linux server (`amd64` or `arm64`)
- Bash 4+
- PostgreSQL 15+ and Redis 7+, installed and running
- A readonly PostgreSQL connection to an existing Sub2API instance
- Root privileges

### Installation Steps

```bash
curl -sSL https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/install.sh | sudo bash
```

The script will:

1. Detect the system architecture.
2. Download the latest GitHub Release.
3. Verify the archive against `checksums.txt`.
4. Install the binary to `/opt/superllm/superllm`.
5. Create the `superllm` system user, configuration directory, systemd service, and management command.
6. Start the service and enable auto-start on boot.

### Post-Installation

```bash
# Check the service
sudo systemctl status superllm --no-pager

# Follow logs
sudo journalctl -u superllm -f

# Open the Setup Wizard
# http://YOUR_SERVER_IP:8080
```

The Setup Wizard configures the dedicated `superllm` PostgreSQL database, Redis, and the required Sub2API readonly database URL. It does not create an administrator; sign in with an active Sub2API administrator.

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
| `install.sh` | Binary installer/upgrader. Downloads pinned GitHub Release assets like the upstream Sub2API installer |
| `superllmctl` | Command wrapper installed as `/usr/local/bin/superllm` for systemd upgrades and service operations |
| `config.example.yaml` | Example runtime configuration |

## Build, Release, And Deployment Channels

Release publishing is tag-driven. A `v*` tag is the single version fact for GitHub Release assets. Container images and Railway deployment use separate, explicit workflows so a registry credential issue cannot block Linux binary releases.

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `Build Artifacts` | push, pull request, tag, or manual dispatch | Compile Linux `amd64` and `arm64` binary archives and upload CI artifacts. It does not publish a GitHub Release, Docker image, or deployment. |
| `GitHub Release` | `v*` tag push or manual dispatch | Publish Linux `amd64` and `arm64` release assets plus `checksums.txt`. It never waits for container registry credentials. |
| `DockerHub` | manual dispatch only | Build and push the multi-architecture DockerHub image from a selected git ref. Run it after a GitHub Release when container images are required. |
| `Deploy Railway` | manual dispatch only | Deploy a selected existing container image to Railway. |

Systemd script deployment consumes GitHub Release assets through `install.sh`. Docker deployment consumes `wutongci/superllm:<version>` or `latest`.

## Method 2: Docker Compose

```bash
export SUB2API_READONLY_DATABASE_URL='postgresql://readonly:password@sub2api-db:5432/sub2api?sslmode=require'
curl -sSL https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/docker-deploy.sh | bash
docker compose up -d
docker compose logs -f superllm
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

Sign in with an active administrator from the connected Sub2API database. When Sub2API administrators use TOTP, set `TOTP_ENCRYPTION_KEY` to the same value as Sub2API so SuperLLM can verify the existing encrypted secret.

## Manual Docker Deployment

```bash
cp .env.example .env
mkdir -p admin_plus_data admin_plus_postgres_data admin_plus_redis_data

# Edit POSTGRES_PASSWORD, SUB2API_READONLY_DATABASE_URL, JWT_SECRET, TOTP_ENCRYPTION_KEY, etc.
nano .env

docker compose -f docker-compose.local.yml up -d
docker compose -f docker-compose.local.yml logs -f superllm
```

## External PostgreSQL And Redis

Use `docker-compose.standalone.yml` when PostgreSQL and Redis are managed outside this Compose stack:

```bash
cp .env.example .env

# Required for standalone mode
DATABASE_HOST=postgres.example.internal
DATABASE_PORT=5432
DATABASE_USER=superllm
DATABASE_PASSWORD=change_this_secure_password
DATABASE_DBNAME=superllm
REDIS_HOST=redis.example.internal
REDIS_PORT=6379

docker compose -f docker-compose.standalone.yml up -d
```

The target database must be an SuperLLM database. Do not use a live Sub2API production database.

## Updating

For the systemd binary deployment:

Prerequisites are Linux `amd64` or `arm64`, Bash 4+, systemd, PostgreSQL 15+, Redis 7+, and the commands `curl`, `tar`, `gzip`, `sha256sum`, and `useradd`.

```bash
# Fresh install
curl -sSL https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/install.sh | sudo bash

# Existing installation: bootstrap the command wrapper only.
# This detects the installed binary and does not download, overwrite, or restart the app.
curl -sSL https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/install.sh | sudo bash -s -- install-command

# After the command wrapper exists, use it for future operations
sudo superllm upgrade
sudo superllm upgrade -v vX.Y.Z
sudo superllm rollback vX.Y.Z
superllm status
superllm logs -n 200
superllm follow
```

The command wrapper delegates release downloads, checksum verification, backup, and systemd restart to `install.sh`. It is installed at `/usr/local/bin/superllm` and refreshed on every binary install or upgrade.

One-off remote execution is still supported:

```bash
curl -sSL https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/install.sh | sudo bash -s -- upgrade
curl -sSL https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/install.sh | sudo bash -s -- upgrade -v vX.Y.Z
```

For a fresh co-located installation, select a loopback listener that does not conflict with Sub2API:

```bash
curl -fsSL https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/install.sh \
  | sudo bash -s -- install -v vX.Y.Z --host 127.0.0.1 --port 8081
```

When the default port is occupied and no port was supplied, the installer selects the next available port. An explicitly selected occupied port fails instead of taking over the existing listener.

The upgrader recognizes `/opt/sub2api-admin-plus/sub2api-admin-plus` with `sub2api-admin-plus.service` as the legacy Admin Plus layout. It migrates that configuration into `/etc/superllm`, backs up the old binary, installs `/opt/superllm/superllm`, and disables only `sub2api-admin-plus.service` without deleting its directory.

An existing `/opt/sub2api/sub2api` with `sub2api.service` is the authoritative identity and data source, not a legacy SuperLLM installation. The installer preserves it so Sub2API and SuperLLM can run side by side on the same server.

For the local-directory deployment:

```bash
docker compose pull
docker compose up -d
docker compose logs -f superllm
```

For a pinned release image, set `ADMIN_PLUS_IMAGE` in `.env`:

```env
ADMIN_PLUS_IMAGE=wutongci/superllm:X.Y.Z
```

Container deployments do not perform in-place binary self-updates from the admin UI. Upgrade them by pulling the released image and recreating the service:

```bash
docker compose pull
docker compose up -d
```

The admin UI one-click update is for systemd binary deployments. It downloads the matching GitHub Release asset, verifies `checksums.txt`, replaces the binary, and restarts the service through systemd.

## Backups

For the local-directory deployment, stop the stack and archive the deployment directory:

```bash
docker compose down
tar czf superllm-deploy.tar.gz .
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
| `POSTGRES_DB` | SuperLLM database, defaults to `superllm` |
| `POSTGRES_USER` | PostgreSQL user, defaults to `superllm` |
| `POSTGRES_PASSWORD` | Required PostgreSQL password |
| `JWT_SECRET` | Stable JWT secret |
| `TOTP_ENCRYPTION_KEY` | Stable TOTP encryption key |
| `REDIS_PASSWORD` | Redis password, optional for local deployments |
| `SUB2API_READONLY_DATABASE_URL` | Required readonly PostgreSQL URL; source of Sub2API users and linked data |
| `SUB2API_READONLY_REDIS_URL` | Optional readonly Redis URL for existing Sub2API runtime state |
| `SUB2API_READONLY_REDIS_DB` | Optional DB override when Sub2API shares the Redis host |
| `ADMIN_PLUS_SUB2API_ADMIN_BASE_URL` | Optional Sub2API Admin API base URL for provisioning |
| `ADMIN_PLUS_SUB2API_ADMIN_API_KEY` | Optional Sub2API Admin API key for provisioning |
| `ADMIN_PLUS_ALLOW_EMBEDDED_SUB2API_GATEWAY` | Local development fallback switch; keep false in production |

## Current Limitations

- `ADMIN_PLUS_DATABASE_URL` is not supported yet.
- Redis prefix isolation is partially implemented through existing cache prefixes; do not share a Redis instance with untrusted workloads until the dedicated prefix wrapper is complete.
