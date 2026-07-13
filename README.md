# SuperLLM

SuperLLM is an operations automation extension built from the Sub2API codebase.

MVP 0 keeps the Sub2API frontend/backend architecture, UI conventions, build scripts, and deployment layout as a runnable baseline. The current business layer already includes real operations APIs, pages, scheduler task generation, Chrome-extension result ingestion, and a minimal Chrome MV3 executor. Supplier-specific browser page adapters are still being built.

## 部署方式

### 方式一：脚本安装（推荐）

一键安装脚本会从 [GitHub Releases](https://github.com/openrelayllm/superllm/releases) 下载预编译二进制文件，适合直接部署到 Linux 服务器。

#### 前置条件

- Linux 服务器（`amd64` 或 `arm64`）
- Bash 4+
- PostgreSQL 15+（已安装并运行）
- Redis 7+（已安装并运行）
- 已运行的 Sub2API 及其 PostgreSQL 只读连接
- `curl`、`tar`、`gzip`、`sha256sum`、`systemctl`、`useradd`
- root 权限或可用的 `sudo`

#### 安装步骤

```bash
curl -sSL https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/install.sh | sudo bash
```

脚本会自动：

1. 检测 Linux 系统架构（`amd64` 或 `arm64`）。
2. 获取并下载最新稳定 Release。
3. 使用 `checksums.txt` 校验 SHA-256。
4. 把二进制安装到 `/opt/superllm/superllm`。
5. 创建 `superllm` 系统用户、配置目录、systemd 服务和管理命令。
6. 启动服务并设置开机自启。

#### 安装后配置

```bash
# 1. 启动服务（安装脚本已自动执行，重复执行是安全的）
sudo systemctl start superllm

# 2. 设置开机自启（安装脚本已自动执行）
sudo systemctl enable superllm

# 3. 查看服务状态
sudo systemctl status superllm --no-pager

# 4. 在浏览器中打开安装向导
# http://你的服务器IP:8080
```

安装向导将引导你完成：

- SuperLLM PostgreSQL 配置，数据库名使用 `superllm`
- Redis 配置
- Sub2API 只读数据库 URL 配置

SuperLLM 不创建管理员账号。完成向导后，请使用 Sub2API 中 `role = 'admin'`、`status = 'active'` 的管理员账号登录。

安装完成后会创建：

| 项目 | 路径或名称 |
|------|------------|
| 二进制 | `/opt/superllm/superllm` |
| 配置与运行数据 | `/etc/superllm` |
| systemd 服务 | `superllm.service` |
| 管理命令 | `/usr/local/bin/superllm` |
| 系统用户 | `superllm` |

确认健康状态和日志：

```bash
superllm status
superllm logs -n 200
curl -fsS http://127.0.0.1:8080/health
```

Web 地址：

```text
http://服务器IP:8080
```

生产环境应使用独立的 `superllm` 数据库，不要把 SuperLLM 的读写连接指向现有 Sub2API 生产主库。

#### 准备 Sub2API 只读账号

SuperLLM 登录时直接读取现有 Sub2API 的 `users` 表。建议在 Sub2API PostgreSQL 中创建专用只读角色：

```sql
CREATE ROLE superllm_readonly LOGIN PASSWORD 'replace-with-a-strong-password';
GRANT CONNECT ON DATABASE sub2api TO superllm_readonly;

-- 连接到 sub2api 数据库后执行
GRANT USAGE ON SCHEMA public TO superllm_readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO superllm_readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT ON TABLES TO superllm_readonly;
```

连接 URL 示例：

```text
postgresql://superllm_readonly:password@127.0.0.1:5432/sub2api?sslmode=require
```

该账号不需要且不应拥有 `INSERT`、`UPDATE`、`DELETE` 或 DDL 权限。安装检查会确认 `users` 表可读且至少存在一个有效管理员。

#### 安装指定版本

```bash
curl -fsSL https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/install.sh \
  | sudo bash -s -- install -v v0.42.1 --host 127.0.0.1 --port 8081
```

同机运行 Sub2API 时，建议显式监听 `127.0.0.1:8081` 并由 Nginx/Caddy 反向代理。未指定端口且默认 `8080` 已被占用时，安装器会自动选择下一个可用端口；显式指定的端口被占用时会直接报错，不会覆盖现有服务。

查看可安装版本：

```bash
curl -fsSL https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/install.sh \
  | bash -s -- list-versions
```

#### 升级

从旧版升级前，先在 `/etc/superllm/config.yaml` 中配置 Sub2API 身份源：

```yaml
sub2api:
  readonly_database_url: "postgresql://readonly:password@127.0.0.1:5432/sub2api?sslmode=require"
```

只读账号至少需要 `users` 及 SuperLLM 所用账号、分组、用量表的 `SELECT` 权限。未配置时服务仍可启动，但所有登录请求都会失败关闭，不会回退到 SuperLLM 本地 `users` 表。

推荐使用安装时写入的管理命令：

```bash
# 升级到最新稳定 Release
sudo superllm upgrade

# 升级到指定版本
sudo superllm upgrade -v vX.Y.Z
```

也可以直接执行远程安装器：

```bash
curl -fsSL https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/install.sh \
  | sudo bash -s -- upgrade
```

升级流程会：

1. 读取现有监听地址和端口。
2. 记录当前版本并停止服务。
3. 备份当前二进制为同目录下的 `.backup` 或带版本号备份。
4. 下载目标 Release 并校验 SHA-256。
5. 保留现有配置目录和数据库，更新二进制、管理命令及 systemd unit。
6. 启动服务并恢复开机自启。

升级前仍建议单独备份 PostgreSQL 和 `/etc/superllm`。二进制备份不能替代数据库备份。

#### 从旧名称升级

安装器可识别以下旧布局：

- `/opt/sub2api-admin-plus/sub2api-admin-plus` + `sub2api-admin-plus.service`

升级时会把旧 Admin Plus 配置和安装锁迁移到 `/etc/superllm`，安装新的 `superllm.service`，并停用 `sub2api-admin-plus.service`。旧目录不会自动删除，可用于人工回退或确认数据完整性。迁移完成并验证无误后，再手动清理旧目录和旧 systemd unit。

现有 `/opt/sub2api/sub2api` 与 `sub2api.service` 是 SuperLLM 的身份和数据来源，不属于旧版 SuperLLM。安装器会保留该服务，支持 Sub2API 与 SuperLLM 在同一台服务器并行运行。

#### 回滚

```bash
sudo superllm rollback vX.Y.Z
```

回滚同样从 GitHub Release 下载指定版本并校验文件，不会回滚数据库结构或业务数据。跨多个数据库迁移版本回滚前，应先阅读对应 Release Notes 并准备数据库备份。

#### 服务管理

```bash
superllm status
superllm logs -n 200
superllm follow
sudo superllm restart
sudo superllm stop
sudo superllm start
```

如果旧安装没有管理命令，可只补装命令入口，不覆盖应用二进制：

```bash
curl -fsSL https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/install.sh \
  | sudo bash -s -- install-command
```

#### 卸载

```bash
curl -fsSL https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/install.sh \
  | sudo bash -s -- uninstall -y
```

默认保留 `/etc/superllm`。仅在确认配置和运行数据均不再需要时使用 `--purge`：

```bash
curl -fsSL https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/install.sh \
  | sudo bash -s -- uninstall -y --purge
```

卸载脚本不会删除外部 PostgreSQL 数据库或 Redis 数据。

### 方式二：Docker Compose

需要由容器同时运行 SuperLLM、PostgreSQL 和 Redis 时：

```bash
mkdir -p superllm-deploy && cd superllm-deploy
export SUB2API_READONLY_DATABASE_URL='postgresql://readonly:password@sub2api-db:5432/sub2api?sslmode=require'
curl -fsSL https://raw.githubusercontent.com/openrelayllm/superllm/main/deploy/docker-deploy.sh | bash
docker compose up -d
docker compose logs -f superllm
```

容器升级：

```bash
docker compose pull
docker compose up -d
```

固定版本可在 `.env` 中设置：

```env
ADMIN_PLUS_IMAGE=wutongci/superllm:X.Y.Z
```

完整 Docker、外部 PostgreSQL/Redis 和数据备份说明见 [deploy/README.md](deploy/README.md)。

### 常见问题

```bash
# 服务启动失败
sudo systemctl status superllm --no-pager
sudo journalctl -u superllm -n 200 --no-pager

# 端口监听检查
sudo ss -lntp | grep ':8080'

# 配置目录权限
sudo chown -R superllm:superllm /etc/superllm /opt/superllm

# 重新加载 systemd 配置
sudo systemctl daemon-reload
sudo systemctl restart superllm
```

若服务器位于反向代理之后，应把代理上游指向 `127.0.0.1:8080`，启用 HTTPS，并放行 WebSocket 与长连接响应。

## Scope

- Keep the Sub2API Go/Gin backend structure.
- Keep the Sub2API Vue/Vite/Tailwind frontend structure and UI style.
- Reuse Sub2API admin authentication semantics.
- Reuse PostgreSQL and Redis infrastructure, with SuperLLM data isolated by database and Redis key prefix.
- Add operations automation features incrementally.

## Current Status

Implemented:

- Supplier parent records.
- Supplier account/key child bindings to local Sub2API `accounts.id`.
- Rate, balance, health, promotion, billing, reconciliation, extension task, and action recommendation APIs.
- OpenAI-compatible Responses health probe for bound local OpenAI accounts, defaulting to `gpt-5.5`.
- Feishu/Lark webhook notifications for supplier balance, rate, health, and promotion events, with SQL delivery audit, event-level dedupe, and an SuperLLM delivery log page.
- Scheduler API and page for generating idempotent Chrome extension tasks.
- Chrome extension task result ingestion into rate, balance, promotion, health, and billing tables.
- Browser login credentials encrypted at rest and exposed only through a valid extension task lease.
- Minimal Chrome extension executor in `extension/`.
- Chrome extension parser smoke tests in `extension/test-parser.cjs`.
- Local Sub2API read adapter for real `accounts` and `usage_logs`.
- Local Sub2API Redis read adapter for account concurrency and waiting queue runtime.
- SuperLLM operation pages, including supplier bindings, account runtime, billing reconciliation, and local usage.
- API E2E script using real HTTP, PostgreSQL, Redis fixtures, and a local OpenAI-compatible `/v1/responses` probe server.

Not implemented yet:

- Supplier-specific Chrome extension adapters for stable Sub2API/New API page login, scraping, and bill export.
- Sub2API window-cost/runtime limit adapter beyond current concurrency keys.
- Notification rate-limit, multi-channel delivery, and reconciliation-alert loop.
- Confirmed action execution through Sub2API Admin API.

## MVP 0 Rules

- Do not modify the upstream Sub2API repository at `/Users/coso/Documents/dev/go/sub2api`.
- Do not rewrite the Go module path yet; the backend still imports `github.com/Wei-Shaw/sub2api` internally to keep the cloned baseline buildable.
- Do not delete large Sub2API backend/frontend modules until the baseline is verified.
- Keep product and architecture notes in `docs/`.

## Source Baseline

- Source path: `/Users/coso/Documents/dev/go/sub2api`
- Source commit: `4a5665da5b2c6b83c4597844ea6e573746c821b1`

## Development

Local dev stack:

```bash
./scripts/start-dev.sh
# or
make dev
```

The local script uses native PostgreSQL/Redis binaries only; it does not install or start Docker. During local debug startup the default admin account is reset/created so stale local data cannot break login.

Defaults:

- Frontend: `http://127.0.0.1:3000`
- Backend: `http://127.0.0.1:8080`
- Admin: `admin@superllm.local` / `AdminPlus@123456`
- PostgreSQL: `root:root@127.0.0.1:5432/superllm`
- Redis: `127.0.0.1:6379/0`

You can override ports and local infrastructure through environment variables:

```bash
SERVER_PORT=8081 FRONTEND_PORT=3001 make dev
DATABASE_PORT=15432 REDIS_PORT=16379 DATABASE_DBNAME=superllm REDIS_DB=0 make dev-backend
```

Single-process startup:

```bash
make dev-backend
make dev-frontend
```

Backend:

```bash
cd backend
go test ./...
go build -o bin/server ./cmd/server
```

Frontend:

```bash
cd frontend
pnpm install
pnpm run typecheck
pnpm run build
```

Focused verification:

```bash
cd backend
go test ./internal/adminplus/... ./internal/handler/adminplus/... ./internal/server/routes/...

cd ../frontend
pnpm run typecheck
pnpm run test:run -- src/router/__tests__/admin-plus-routes.spec.ts

cd ..
node tools/admin-plus-e2e.mjs
```

Or start a dedicated backend on port `3010` and run the local E2E script:

```bash
make e2e-local
```

E2E defaults:

- `ADMIN_PLUS_BASE_URL=http://localhost:3000`
- `ADMIN_PLUS_E2E_EMAIL=admin@superllm.local`
- `ADMIN_PLUS_E2E_PASSWORD=AdminPlus@123456`
- `ADMIN_PLUS_E2E_DB_URL=postgresql://root:root@127.0.0.1:5432/superllm?sslmode=disable`
- `ADMIN_PLUS_E2E_REDIS_URL=redis://127.0.0.1:6379/0`

The E2E script creates `e2e-*` PostgreSQL rows, temporary Redis runtime keys, and a local OpenAI-compatible `/v1/responses` test server to verify real API/DB/Redis/HTTP probe paths. It cleans its fixtures by default. To inspect historical E2E rows without deleting them, run `node tools/cleanup-admin-plus-e2e.mjs`; set `ADMIN_PLUS_CLEAN_E2E_EXECUTE=1` only when you intentionally want to delete those test fixtures.

## Health Probe

`POST /api/v1/admin-plus/health/probe` probes a supplier account child binding through the local Sub2API `accounts` row. The frontend never accepts or displays an API key. The backend reads the bound account credentials, calls OpenAI-compatible `/v1/responses` with streaming enabled, records TTFT and total latency, then persists a health sample and derived events.

The default probe model is `gpt-5.5`. Real external probing requires a valid OpenAI-compatible key and base URL in the bound Sub2API account. Without that, only local fixture verification can pass.

## Feishu Notifications

Supplier balance, rate, health, promotion, and reconciliation anomaly events can be sent to a Feishu/Lark custom bot:

```bash
export ADMIN_PLUS_FEISHU_WEBHOOK_URL='https://open.feishu.cn/open-apis/bot/v2/hook/...'
export ADMIN_PLUS_FEISHU_WEBHOOK_SECRET='optional-signature-secret'
```

Legacy balance-only variables are still accepted for compatibility:

```bash
export ADMIN_PLUS_FEISHU_BALANCE_WEBHOOK_URL='https://open.feishu.cn/open-apis/bot/v2/hook/...'
export ADMIN_PLUS_FEISHU_BALANCE_WEBHOOK_SECRET='optional-signature-secret'
```

Notifications are emitted when business events are created, for example `low_balance`, `depleted`, rate increases, slow health probes, request errors, supplier promotions, or reconciliation anomalies. Each event is written to `admin_plus_notification_deliveries` before sending, so repeated delivery for the same event/channel is skipped. High-frequency rate, health, and promotion events use windowed dedupe keys to avoid alert floods. Delivery failure is logged in SQL and visible in SuperLLM without rolling back snapshots or events.

## Chrome Extension

The minimal MV3 executor lives in `extension/`.

It can:

- import the current SuperLLM `auth_token` from an SuperLLM tab;
- claim extension tasks;
- fetch supplier browser credentials with `task_id + device_id + lease_token`;
- open the supplier dashboard and run generic Sub2API/New API-like DOM extraction;
- complete the task only when real page data is parsed, otherwise fail the task.

Generic DOM extraction is intentionally conservative and covered by `node extension/test-parser.cjs`. Production support still requires supplier-specific adapters for each real dashboard shape.

## Sub2API Read Integration

SuperLLM writes its own operational data to the SuperLLM database, but user identity always comes from the existing Sub2API database. Configure a dedicated readonly connection:

```bash
export SUB2API_READONLY_DATABASE_URL='postgresql://root:root@127.0.0.1:5432/sub2api?sslmode=disable'
```

This variable is required. SuperLLM validates email/password, role, status and TOTP against Sub2API and only allows active administrators to enter the management UI. It never creates, updates, imports or exports Sub2API users.

Grant the readonly database account `SELECT` on `users` plus the account and usage tables used by SuperLLM. If Sub2API administrators use TOTP, configure the same `TOTP_ENCRYPTION_KEY` in both applications.

To read Sub2API runtime concurrency from another Redis DB or URL, set one of:

```bash
export SUB2API_READONLY_REDIS_DB=0
export SUB2API_READONLY_REDIS_URL='redis://127.0.0.1:6379/0'
```

If neither variable is set, SuperLLM reuses the current Redis client. The runtime adapter only reads Sub2API keys such as `concurrency:account:{id}` and `wait:account:{id}`.

## Documentation

- Product requirements: `docs/sub2api-admin-plus-prd.md`
- Code structure plan: `docs/code-structure.md`
- MVP baseline/progress: `docs/mvp0-baseline.md`
