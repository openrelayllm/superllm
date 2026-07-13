# SuperLLM Linux Deployment Runbook

This runbook is the source of truth for deploying SuperLLM to a user-supplied Linux server. Values inside angle brackets are placeholders, not defaults.

## Fixed Product Facts

| Item | Value |
| --- | --- |
| GitHub repository | `openrelayllm/superllm` |
| Releases | `https://github.com/openrelayllm/superllm/releases` |
| Installer | `https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/install.sh` |
| Binary | `/opt/superllm/superllm` |
| Configuration | `/etc/superllm` |
| systemd service | `superllm.service` |
| Management command | `/usr/local/bin/superllm` |
| System user | `superllm` |
| Supported systems | Linux `amd64`, Linux `arm64` |
| Release assets | `checksums.txt`, `superllm_X.Y.Z_linux_amd64.tar.gz`, `superllm_X.Y.Z_linux_arm64.tar.gz` |
| Health endpoint | `/health` |
| Public settings endpoint | `/api/v1/settings/public` |
| Login endpoint | `/api/v1/auth/login` |

## Deployment Input Checklist

Collect these values before mutation. Record secrets only in the approved secret store or root-readable server configuration, never in this runbook.

| Category | Required information |
| --- | --- |
| SSH | Host or IP, user, port, SSH alias/key reference, sudo availability |
| Platform | Distribution/version, architecture, systemd availability |
| Public access | Domain, DNS provider/status, desired public URL |
| Listener | Local host and port; prefer `127.0.0.1:<port>` behind a reverse proxy |
| Proxy | Nginx, BaoTa Nginx, or Caddy; current vhost path |
| TLS | Existing certificate, ACME client, BaoTa certificate, or other method |
| SuperLLM PostgreSQL | Host, port, dedicated database, user, password source, SSL mode |
| SuperLLM Redis | Host, port, database number, password source, TLS requirement |
| Sub2API PostgreSQL | Readonly URL or its secret source and required table scope |
| Sub2API Redis | Optional readonly URL or shared-host DB number |
| Identity | Whether Sub2API admins use TOTP and whether the same encryption key is available |
| Release | Target version such as `v0.42.0`, or explicit approval to use latest stable |
| Network | Firewall owner and ports that may be opened |
| Recovery | Database backup destination, retention, acceptable downtime, rollback requirement |

Never ask the user to paste an SSH private key. Accept an SSH config alias or key path.

## Phase 1: Read-Only Preflight

Run only non-mutating checks first. Use the provided SSH target values instead of inventing an IP or username.

```bash
ssh -p <ssh-port> <ssh-user>@<ssh-host> 'set -u
uname -a
uname -m
cat /etc/os-release
command -v systemctl curl tar gzip sha256sum
df -h /
free -h 2>/dev/null || true
systemctl is-system-running 2>/dev/null || true
systemctl status superllm --no-pager 2>/dev/null || true
systemctl is-enabled superllm 2>/dev/null || true
ss -lntp
test -x /opt/superllm/superllm && /opt/superllm/superllm --version || true
test -d /etc/superllm && find /etc/superllm -maxdepth 1 -type f -printf "%f\n" || true
command -v nginx 2>/dev/null || true
test -x /www/server/nginx/sbin/nginx && /www/server/nginx/sbin/nginx -V 2>&1 | head -n 2 || true
'
```

Also verify without displaying secrets:

- PostgreSQL and Redis host reachability.
- The dedicated SuperLLM database exists or is approved for creation.
- `SUB2API_READONLY_DATABASE_URL` points to the actual Sub2API database, not `superllm`.
- The Sub2API readonly user can read `users` and required linked data tables.
- At least one Sub2API user has an administrator role and active status.
- The chosen listener is unused or already belongs to SuperLLM.
- DNS A/AAAA records resolve to the intended server.
- Existing proxy configuration and certificate ownership are understood.

When inspecting environment files, print only key names. Do not output values for keys containing `PASSWORD`, `SECRET`, `TOKEN`, `KEY`, `URL`, or `DSN`.

## Phase 2: Change Approval

Before making changes, present:

```text
⚠️ 危险操作检测！
操作类型：SuperLLM 生产服务器安装/升级
影响范围：<service>, <listener>, <database>, <proxy-vhost>, <firewall>, <expected-downtime>
风险评估：服务重启会产生短暂中断；数据库迁移可能限制二进制回滚；代理或 TLS 配置错误会影响公网访问。

请确认是否继续？[需要明确的"是"、"确认"、"继续"]
```

Database creation, role/grant changes, package installation, firewall changes, service replacement, proxy reload, rollback, and cleanup are covered only when explicitly listed in the impact range.

## Phase 3: Backup

Use a timestamped directory outside `/opt/superllm` and `/etc/superllm`:

```bash
backup_root="/var/backups/superllm/$(date +%Y%m%d-%H%M%S)"
sudo install -d -m 0700 "$backup_root"
sudo cp -a /etc/superllm "$backup_root/config" 2>/dev/null || true
sudo cp -a /opt/superllm/superllm "$backup_root/superllm.binary" 2>/dev/null || true
sudo cp -a /etc/systemd/system/superllm.service "$backup_root/superllm.service" 2>/dev/null || true
```

Copy the selected proxy vhost into the same directory. Back up the dedicated `superllm` PostgreSQL database with the operator-approved authentication method. Do not embed its password in the command line. Record the backup path and validate the dump is non-empty.

## Phase 4: Install or Upgrade

Fresh install, latest stable:

```bash
curl -sSL https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/install.sh | sudo bash
```

Fresh install, pinned version:

```bash
curl -fsSL https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/install.sh \
  | sudo bash -s -- install -v <version> --host 127.0.0.1 --port <local-port>
```

Existing installation:

```bash
sudo superllm upgrade -v <version>
```

The installer must download an official Release archive and verify it against `checksums.txt`. It installs or preserves:

- `/opt/superllm/superllm`
- `/etc/superllm`
- `/etc/systemd/system/superllm.service`
- `/usr/local/bin/superllm`
- system user `superllm`

If `/opt/sub2api/sub2api` and `sub2api.service` already exist, keep them running. They provide the authoritative identities and linked data and must never be treated as a legacy SuperLLM layout. Only `sub2api-admin-plus.service` is eligible for automatic legacy-name migration.

If a local Nginx/Caddy/BaoTa reverse proxy is used, configure the service listener as `127.0.0.1:<local-port>`. Do not expose the application port through the firewall unless direct access is explicitly required.

## Phase 5: Application Configuration

The primary database must be a dedicated SuperLLM database:

```yaml
database:
  host: "<superllm-postgres-host>"
  port: 5432
  user: "<superllm-postgres-user>"
  password: "<secret>"
  dbname: "superllm"
  sslmode: "require"

redis:
  host: "<superllm-redis-host>"
  port: 6379
  password: "<secret>"
  db: 0

sub2api:
  readonly_database_url: "<secret-readonly-url>"
  readonly_redis_url: "<optional-secret-readonly-url>"
  readonly_redis_db: 1

totp:
  encryption_key: "<same-key-as-sub2api-when-existing-admins-use-totp>"
```

Use the actual SSL and Redis DB requirements supplied by the operator; values above are structural examples. Apply restrictive permissions:

```bash
sudo chown -R superllm:superllm /etc/superllm
sudo find /etc/superllm -type d -exec chmod 0750 {} +
sudo find /etc/superllm -type f -exec chmod 0640 {} +
```

Identity invariants:

- SuperLLM has no independently managed login users.
- Credentials, role, active status, and TOTP are validated against Sub2API.
- Only active administrators are allowed into the management UI.
- The Sub2API database account is readonly and receives no DDL, `INSERT`, `UPDATE`, or `DELETE` grants.
- SuperLLM's migrations and business writes target only its own primary database.

## Phase 6: Nginx or BaoTa Reverse Proxy

Example dedicated vhost:

```nginx
server {
    listen 443 ssl http2;
    server_name <superllm-domain>;

    ssl_certificate <certificate-path>;
    ssl_certificate_key <private-key-path>;

    location / {
        proxy_pass http://127.0.0.1:<local-port>;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_buffering off;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

For standard Nginx:

```bash
sudo nginx -t
sudo systemctl reload nginx
```

For BaoTa Nginx, validate the panel-managed main configuration and edit only the selected site's vhost:

```bash
sudo /www/server/nginx/sbin/nginx -t -c /www/server/nginx/conf/nginx.conf
sudo /www/server/nginx/sbin/nginx -s reload
```

Never overwrite another domain's vhost. Obtain separate approval before changing DNS, requesting a certificate, or opening ports 80/443.

## Phase 7: Verification Gates

Service and listener:

```bash
SYSTEMD_PAGER=cat systemctl --no-pager status superllm | sed -n '1,24p'
systemctl is-active superllm
systemctl is-enabled superllm
ss -lntp | rg '<local-port>|superllm' || ss -lntp | grep -E '<local-port>|superllm'
/opt/superllm/superllm --version
```

Local and public HTTP:

```bash
curl -fsS -w '\nHTTP %{http_code}\n' http://127.0.0.1:<local-port>/health
curl -fsS -w '\nHTTP %{http_code}\n' https://<superllm-domain>/health
curl -fsS -w '\nHTTP %{http_code}\n' https://<superllm-domain>/api/v1/settings/public
```

TLS and logs:

```bash
curl -fsSI https://<superllm-domain>/
openssl s_client -connect <superllm-domain>:443 -servername <superllm-domain> </dev/null 2>/dev/null \
  | openssl x509 -noout -subject -issuer -dates
journalctl -u superllm -n 200 --no-pager
```

Check the reverse-proxy error log associated with the vhost. Inspect for startup failures, migration errors, refused database/Redis connections, readonly Sub2API errors, authentication-source failures, HTTP 5xx, WebSocket failures, and TLS errors.

Login smoke test:

1. Open the public domain in an authenticated interactive browser session.
2. Sign in with an existing active Sub2API administrator without logging the credential values.
3. Confirm the management UI loads and the authenticated session survives one page refresh.
4. When approved test accounts exist, confirm an inactive or non-admin Sub2API account is rejected.
5. When TOTP is enabled, confirm the same Sub2API TOTP code path succeeds.

Do not treat a successful `/health` request as proof that cross-database login works.

## Phase 8: Rollback

For a compatible binary rollback:

```bash
sudo superllm rollback <previous-version>
systemctl is-active superllm
curl -fsS http://127.0.0.1:<local-port>/health
```

If the management command is unavailable, restore the timestamped binary and unit backups, run `systemctl daemon-reload`, restart `superllm`, and re-run all verification gates.

Do not assume binary rollback reverses database schema changes. If the target version cannot use the migrated schema, stop the service and restore the approved database dump before restarting. Database restore is destructive and requires a separate explicit confirmation.

Restore the proxy vhost only when the deployment changed it and the restored upstream matches the running application.

## Final Report Template

```text
SuperLLM deployment result
- Target: <redacted-host-or-alias>
- Version: <version-and-commit>
- Service: active/enabled
- Listener: <local-host:port>
- Public URL: <https-url>
- Primary database: <host/redacted>/superllm
- Sub2API source: readonly connection verified (credentials redacted)
- Health: local <status>, public <status>
- Public settings: <status>
- Admin login: <pass/fail/not-run-and-reason>
- TLS: <issuer-and-expiry>
- Logs: <clean/issues>
- Backups: <paths>
- Rollback: <command-or-procedure>
- Remaining risks: <items-or-none>
```
