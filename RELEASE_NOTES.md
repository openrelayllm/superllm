# Release Notes

## v0.9.7 - 2026-06-22

### 新增

- New API provider adapter 新增 Key provisioning：通过 `/api/token/search` 幂等查找、`POST /api/token/` 创建、`POST /api/token/:id/key` 读取明文 Key。
- `supplierkeys` 服务支持 New API 供应商执行“补齐 Key/账号”，可为每个 active group 创建或复用第三方 Key，并同步创建本地 Sub2API account。
- New API profile capability 新增 `can_create_key`，前后端可据此识别供应商支持 Key 补齐。
- New API 路线图文档补充用户 Key 管理接口、内存链路约束和验收标准。

### 修复

- Provider router 不再对 New API `CreateKey` 返回 capability missing，而是转发到 New API adapter。
- New API Key 创建失败会映射为明确错误码，包括 session 失效、token 数量上限和额度参数错误。
- New API token 响应快照会移除 `key`、`api_key`、`token`、`secret` 等敏感字段，避免明文凭据进入任务结果或日志口径。

### 发布

- 更新版本号到 `0.9.7`。
- GitHub Release 继续只发布 Linux 产物：`linux_amd64`、`linux_arm64` 和 `checksums.txt`。
- DockerHub 镜像继续由 GitHub Actions 发布，不依赖本地 Docker。
