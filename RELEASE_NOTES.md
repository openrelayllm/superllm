# Release Notes

## v0.42.1 - 2026-07-13

### 修复

- 修复独立 `sub2api` 与 `superllm` PostgreSQL 数据库部署下，本地账户运营接口因跨库 JOIN 返回 500 的问题。
- 本地账户、分组和代理信息改从 Sub2API 只读库读取；供应商绑定、余额、通道检测和纯度投影从 SuperLLM 主库读取，并在应用层按账户 ID 合并。
- 保持关键词、供应商、分组、倍率、余额、通道和可调度状态筛选语义，补充双数据库与过滤回归测试。

### 安装与升级

- 一键安装器不再把同机运行的 `/opt/sub2api/sub2api` 与 `sub2api.service` 当作旧版 SuperLLM，也不会停止或禁用权威身份源。
- 支持 `--host` 和 `--port` 参数；默认端口被占用时自动选择可用端口，显式端口冲突时安全失败。
- 增加安装器安全测试、同机部署说明，以及 Linux systemd 发布、验证和回滚 Runbook。

### 验证

- 后端执行完整 `go test ./...`。
- 前端执行完整 Vitest、TypeScript 检查和生产构建。
- 安装器执行 Bash 语法检查及同机部署安全回归测试。

## v0.42.0 - 2026-07-12

### 架构

- SuperLLM 不再维护独立用户体系。登录、当前用户、JWT 校验、刷新令牌、管理员权限和 TOTP 均从 `SUB2API_READONLY_DATABASE_URL` 指向的现有 Sub2API 数据库读取。
- 仅允许 Sub2API 中状态为 `active` 的管理员进入 SuperLLM；本地管理员创建、开发环境密码重置和用户导入导出已删除。
- 缺少 Sub2API 只读身份源时认证失败关闭，不再回退到 SuperLLM 本地 `users` 表。

### 产品收敛

- 项目、二进制、systemd 服务、安装目录、Docker 镜像、GitHub 地址和界面品牌统一改名为 `SuperLLM`。
- 删除运营看板、代理出口与 Mihomo、邮箱自动取码、网址目录、续费提醒及对应前后端实现。
- 精简供应商操作，移除调度列、状态、会话、第三方兑换等入口；隐藏数据备份与导入导出导航，删除 Admin API Key 管理能力。
- 删除 `/api/v1/public/proxyai/*` 前台业务路由及专用 CORS/API Key 装配，账号纯度检测仅保留在管理员 API。
- 兼容迁移历史站点名 `Sub2API`、`Sub2API Admin`、`Sub2API Admin Plus`，现有安装自动显示 `SuperLLM`。

### 安装与升级

- 完善 Linux `amd64/arm64` 一键安装器，支持校验和验证、指定版本、升级、回滚、旧目录迁移及 `superllm` 管理命令。
- Web、CLI 与 Docker 安装均要求配置 Sub2API 只读数据库；Docker Compose 模板不再接受本地管理员变量。
- SuperLLM 自身读写数据库默认名统一为 `superllm`，Sub2API 身份与业务数据继续通过独立只读连接访问。
- README 增加只读 PostgreSQL 角色、首次安装、旧版迁移、升级、回滚、Docker Compose 与故障排查说明。

### 验证

- 后端执行 `go test ./...` 与管理员认证 unit tests。
- 前端执行完整 Vitest、Vue TypeScript 检查和生产构建。
- 部署脚本执行 Bash 语法检查，GoReleaser 保持 Linux-only 资产。

### 发布

- 更新版本号到 `0.42.0`。
- GitHub Release 保持 Linux-only 二进制资产：`linux_amd64`、`linux_arm64` 和 `checksums.txt`。
- tag 发布同步发布 DockerHub 与 GHCR 多架构镜像：`0.42.0`、`latest`、`0.42` 和 `0`。
- Railway 默认不自动部署；如需部署，单独启用 Release workflow 的 `deploy_railway` 或仓库变量 `RAILWAY_AUTO_DEPLOY=true`。
- 裸机 systemd 部署继续使用 GitHub Release 二进制升级；容器部署通过拉取新镜像升级。
