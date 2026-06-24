# MVP 0 Baseline

## Goal

MVP 0 creates a runnable Sub2API Admin Plus baseline by copying the full Sub2API frontend/backend architecture and UI design into this repository.

This phase intentionally keeps most Sub2API modules intact. The goal is a stable development base, not immediate business pruning.

## Source

| Field | Value |
|-------|-------|
| Source path | `/Users/coso/Documents/dev/go/sub2api` |
| Source commit | `4a5665da5b2c6b83c4597844ea6e573746c821b1` |
| Target path | `/Users/coso/Documents/dev/ai/openrelayllm/sub2api-admin-plus` |

## Copied Areas

- `backend/`
- `frontend/`
- `deploy/`
- `.github/`
- root Docker and build files
- `assets/`
- `skills/`
- `tools/`

The source `.git/`, `node_modules/`, `dist/`, and `.DS_Store` files were not copied.

## Intentional Carryovers

- Backend module path remains `github.com/Wei-Shaw/sub2api`.
- Existing backend package imports remain unchanged.
- Existing frontend route/component structure remains unchanged.
- Existing deployment files remain mostly unchanged.

These carryovers keep the copied baseline buildable. Rename/import migration and feature pruning should be handled as explicit later tasks.

## MVP 0 Cleanup Done

- Root `README.md` now describes Sub2API Admin Plus.
- Root `DEV_GUIDE.md` now describes the Admin Plus development baseline.
- `frontend/package.json` package name changed to `sub2api-admin-plus-frontend`.
- `.gitignore` explicitly allows Admin Plus docs and frontend lockfile to be tracked.

## Verification

| Check | Result | Note |
|-------|--------|------|
| `cd backend && go build -o bin/server ./cmd/server` | PASS | Local Go is `go1.24.3`; Go toolchain downloaded/used `go1.26.4` required by `backend/go.mod`. |
| `cd frontend && pnpm run typecheck` | PASS | TypeScript/Vue type check passed. |
| `cd frontend && pnpm run build` | PASS | Build passed with a non-blocking Vite chunk-size warning. |

Generated artifacts are intentionally ignored:

- `backend/bin/`
- `backend/internal/web/dist/`
- `frontend/node_modules/`
- `frontend/*.tsbuildinfo`
- `frontend/vite.config.d.ts`
- `frontend/vite.config.js`

## Next Cleanup Candidates

- Add Admin Plus login proxy routes.
- Add Admin Plus navigation entry points.
- Decide whether to keep or disable upstream Sub2API release workflows.
- Rename Docker image/container/service names after the baseline build is verified.
- Remove or hide non-MVP user-facing pages only after route and auth impact is understood.

## Post-MVP0 Progress

截至 2026-06-20，项目已经超过纯复制基线阶段，进入 Admin Plus 业务 MVP 开发：

- 后端已注册 `/api/v1/admin-plus/*` 业务路由。
- 运行时已使用 SQL repository，不再依赖内存仓储。
- 已完成供应商父级、供应商账号/Key 子级绑定。
- 已完成费率、余额、健康、公告、插件任务、账单、对账和动作建议基础 API。
- 已完成调度中心 API 和页面，可生成幂等 Chrome 插件任务。
- 已完成插件任务结果摄取兼容层；结构化业务结果只作为旧路径补录，不再作为费率、余额、公告、健康和账单主事实源。
- 已完成供应商浏览器登录凭据加密持久化，只有持有有效插件任务租约的执行器可以读取明文凭据。
- 已创建 `extension/` Chrome MV3 最小执行器，可领取任务、读取凭据、打开供应商后台、采集会话包并回写成功或失败。
- 已抽出 Chrome 插件页面解析纯函数作为兼容兜底，并通过 `node extension/test-parser.cjs` 覆盖费率、余额、公告、账单和并发基础样例。
- 已完成本地 Sub2API 真实只读数据能力：
  - 读取 `accounts`。
  - 读取 `usage_logs` 明细。
  - 聚合账号/模型维度请求数、token、收入、成本和延迟。
  - 读取 Redis `concurrency:account:*`、`wait:account:*` 和 `temp_unsched:account:*` 运行态。
- 已完成 OpenAI-compatible Responses 健康探测链路：
  - 默认模型 `gpt-5.5`。
  - 从供应商账号/Key 子级绑定的本地 Sub2API `accounts.credentials` 读取 API Key 和 base URL。
  - 前端不输入、不展示 API Key。
  - 探测结果写入健康样本和健康事件。
- 已完成飞书自定义机器人基础通知：
  - 通用变量为 `ADMIN_PLUS_FEISHU_WEBHOOK_URL` 和 `ADMIN_PLUS_FEISHU_WEBHOOK_SECRET`。
  - 兼容旧余额变量 `ADMIN_PLUS_FEISHU_BALANCE_WEBHOOK_URL` 和 `ADMIN_PLUS_FEISHU_BALANCE_WEBHOOK_SECRET`。
  - 覆盖余额、费率、健康、公告和对账异常事件。
  - 通知发送前写入 `admin_plus_notification_deliveries`。
  - 同一业务事件同一通道通过 `dedupe_key` 去重。
  - 费率、健康和公告等高频事件支持窗口去重，避免同一事件窗口重复刷屏。
  - 通知中心提供后台飞书配置、测试诊断、业务规则、防打扰、投递记录和失败投递重试。
  - 已提供通知记录页面和 `GET /api/v1/admin-plus/notifications/deliveries`。
  - 通知成功或失败都会记录投递状态，不回滚业务快照或事件。
- 前端已提供 Admin Plus 独立业务导航和页面。
- `tools/admin-plus-e2e.mjs` 覆盖真实 HTTP、真实 PostgreSQL、真实 Redis 运行态、OpenAI-compatible `/v1/responses` 探测、调度生成、租约凭据读取和插件结果摄取链路。

尚未完成：面向具体 Sub2API/New API 供应商后台的稳定页面适配、每日账单自动导出、多通道通知、操作审计和确认后动作执行。

MVP0 的“复制后可运行”目标仍然保留为底线；后续所有业务能力必须继续遵守不修改上游 `/Users/coso/Documents/dev/go/sub2api`、不新增权限系统、Admin Plus 自有数据独立库、只读 Sub2API DB/Redis 的约束。
