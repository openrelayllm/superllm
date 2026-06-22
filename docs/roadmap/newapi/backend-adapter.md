# 后端 New API Adapter

## 文件职责

```text
backend/internal/adminplus/adapters/newapi/provider/
  adapter.go      # HTTP client、DirectLogin、session JSON 请求入口
  session.go      # Cookie、New-Api-User、base URL 和会话包归一化
  errors.go       # New API 业务错误到 Admin Plus 错误码的映射
  profile.go      # /api/user/self profile、余额、capability 归一化
  groups.go       # /api/user/self/groups 分组和倍率归一化
  keys.go         # /api/token/* 用户 Key 创建、搜索和明文读取归一化
  channel_monitors.go # speed/Pulse 模型级延迟、响应耗时和成功率归一化
  client.go       # provider 包说明和兼容占位
```

## 直登流程

1. 从供应商 `api_base_url` 或 `dashboard_url` 解析 origin。
2. 调用 `POST /api/user/login`，请求体包含 `username`、`password`。
3. 如果返回 `require_2fa`、Turnstile 或浏览器挑战，返回明确错误码并提示 Chrome 插件兜底。
4. 登录成功后读取 Set-Cookie 中的 session。
5. 使用 Cookie 和 `New-Api-User` 调用 `GET /api/user/self`。
6. `/self` 验证成功后保存 direct-login session，并触发余额同步。

## Provider Router

`backend/internal/adminplus/adapters/providerrouter/router.go` 根据以下字段选择 adapter：

- direct-login context 中的 `provider_type`
- session bundle 中的 `provider_type` / `system_type`
- supplier 的 `type`

New API 统一分发到 `ProviderKindNewAPI`，Sub2API 继续走原有 adapter。

## Key / 本地账号补齐

New API 的用户 Key 管理走 `UserAuth`，所以必须复用已保存的 Cookie 和 `New-Api-User`：

1. `supplierkeys.Service` 支持 `SupplierTypeSub2API` 和 `SupplierTypeNewAPI`，其他类型仍返回 `SUPPLIER_KEY_PROVIDER_UNSUPPORTED`。
2. provider router 对 `provider_type = new_api` 的 `CreateKey` 转发到 New API adapter。
3. New API adapter 先调用 `GET /api/token/search?keyword=<name>&p=1&page_size=100` 查找同名同分组 token，命中时复用。
4. 未命中时调用 `POST /api/token/` 创建 token，请求体包含：
   - `name`
   - `group`
   - `expired_time`
   - `unlimited_quota`
   - `remain_quota`（仅手工指定正额度时）
   - `cross_group_retry=false`
5. 创建后再次搜索 token id，再调用 `POST /api/token/:id/key` 读取明文 key。
6. 明文 key 只在 Adapter -> Provision Worker -> 本地 Sub2API Admin API 的内存链路流转；`provision_response` 会移除 `key`、`api_key`、`token`、`secret` 等字段。
7. New API 返回的裸 key 会补齐 `sk-` 前缀后写入本地 Sub2API account credential，兼容 OpenAI-style Bearer token 调用。
8. 全量“补齐 Key/账号”继续复用 `provision_all_group_keys` job，每个分组一个 `ensure_third_party_key` step。

## 一键上报后的后端链路

1. Chrome 插件创建 capture-session task 时传入 `type` / `supplier_type` / `provider_type`。
2. `extension_handler.go` 自动创建或匹配供应商时把 `new-api`、`newapi`、`new_api` 归一化为 `SupplierTypeNewAPI`。
3. `suppliers/service.go` 自动创建 New API supplier 时默认 `BalanceCurrency = QTA`。
4. `extension/ingest.go` 保存浏览器 session 前兜底补齐：
   - `provider_type = new_api`
   - `system_type = new_api`
   - `auth_header_name = New-Api-User`
   - `auth_header_value = <user_id>`
   - `required_headers.New-Api-User = <user_id>`
5. session 保存后调用 `balances.SyncFromSession`。
6. `sessions.DecryptedProbeInput` 解密会话包并交给 provider router。
7. New API provider 调用 `/api/user/self`，读取 profile 和 `QTA` 余额快照。

## 渠道状态读取

Codex APIs 的渠道状态不是 New API 主站接口，也不是 Sub2API `/api/v1/channel-monitors`。当前实现：

1. 对 `codexapis.com` / `*.codexapis.com` New API supplier 自动读取 `https://speed.codexapis.com/api/pulse`。
2. 解析最近 60 秒窗口中的 `models[]`。
3. 将 `avg_ttft_ms` 映射为主延迟，将 `avg_resp_sec` 映射为响应耗时毫秒。
4. 将 `success_rate` 映射到现有 `availability_7d` 展示字段，前端窗口标签显示为 `60 秒`。
5. 将 `health` 映射为 `operational` / `degraded` / `failed` / `error`。

## 错误码

| 场景 | 错误码 |
|------|--------|
| 网络不可达、DNS、TLS、超时 | `provider_unreachable` |
| JSON 协议不匹配 | `provider_protocol_mismatch` |
| 密码登录关闭 | `password_login_disabled` |
| Turnstile 或浏览器挑战 | `browser_challenge_required` |
| 账号密码错误 | `login_failed` |
| 2FA 必需 | `two_factor_required` |
| 缺少 session Cookie | `provider_protocol_mismatch` |
| `/self` 缺少或错带 `New-Api-User` | `provider_adapter_bug` |
| session 过期 | `session_expired` |
| 用户禁用或无权限 | `permission_denied` |
