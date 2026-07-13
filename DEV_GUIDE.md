# SuperLLM 开发指南

本文档记录 `superllm` MVP 0 基线的开发约束和常用命令。

## 项目定位

`superllm` 是基于 Sub2API 代码库复制出来的自动化运营扩展系统。

MVP 0 目标不是立即重构业务，而是先得到一个完整、可运行、风格一致的 Sub2API 克隆基线，之后再逐步清理和开发运营业务。

## 基线来源

| 项目 | 值 |
|------|----|
| 上游本地路径 | `/Users/coso/Documents/dev/go/sub2api` |
| 复制来源 commit | `4a5665da5b2c6b83c4597844ea6e573746c821b1` |
| 当前项目路径 | `/Users/coso/Documents/dev/ai/openrelayllm/superllm` |

## 技术栈

| 模块 | 技术 |
|------|------|
| 后端 | Go, Gin, Ent |
| 前端 | Vue 3, Vite, TailwindCSS, pnpm |
| 数据库 | PostgreSQL |
| 缓存 | Redis |

## MVP 0 约束

- 不修改 `/Users/coso/Documents/dev/go/sub2api`。
- 保留 Sub2API 前后端架构和 UI 设计。
- 保留 Sub2API 后端 module 路径，暂不做全仓 import 迁移。
- 先保证克隆基线可构建，再逐步做业务清理。
- SuperLLM 私有业务进入独立模块，不直接写入上游复制区的核心逻辑。
- 文档优先维护在 `docs/`。

## 常用命令

后端：

```bash
cd backend
go test ./...
go build -o bin/server ./cmd/server
```

前端：

```bash
cd frontend
pnpm install
pnpm run typecheck
pnpm run build
```

整体：

```bash
./scripts/start-dev.sh
make dev
make dev-backend
make dev-frontend
make e2e-local
make build
make test-backend
make test-frontend
```

本地登录身份：

```text
使用 SUB2API_READONLY_DATABASE_URL 指向的 Sub2API 数据库中的 active 管理员
```

默认连接：

```text
Backend  http://127.0.0.1:8080
Frontend http://127.0.0.1:3000
PostgreSQL root:root@127.0.0.1:5432/superllm
Sub2API 通过 SUB2API_READONLY_DATABASE_URL 连接
Redis 127.0.0.1:6379/0
```

`./scripts/start-dev.sh` 只使用本机 PostgreSQL/Redis 命令行工具，不安装、不启动 Docker，也不会创建或重置管理员。启动前必须准备可读的 Sub2API 数据库和 active 管理员。

如需换端口：

```bash
SERVER_PORT=8081 FRONTEND_PORT=3001 make dev
```

## 当前注意事项

- `backend/go.mod` 要求 Go `1.26.5`。本机 Go 版本较低时，默认 Go toolchain 可自动下载并使用 `1.26.5`；如果环境禁用了 toolchain，则需要手动安装或切换 Go 版本。
- 前端依赖使用 pnpm，不要用 npm 生成 lockfile。
- `docs/sub2api-admin-plus-prd.md` 是产品边界来源。
- `docs/code-structure.md` 是后续代码拆分和模块落地来源。
