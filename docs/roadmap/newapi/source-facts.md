# New API 源码事实

核对路径：`/Users/coso/Documents/dev/go/new-api`

## 路由与鉴权

| 能力 | 路由 | 鉴权 | 结论 |
|------|------|------|------|
| 密码登录 | `POST /api/user/login` | 未登录态；可能经过 Turnstile | 请求 JSON 为 `username`、`password`；成功写入 `session` Cookie |
| 2FA 登录完成 | `POST /api/user/login/2fa` | pending session Cookie | 请求 JSON 为 `code`；MVP 不在 Admin Plus 收验证码 |
| 登出 | `GET /api/user/logout` | Cookie | 清空 session |
| 当前用户信息 | `GET /api/user/self` | `UserAuth` | 必须有有效 Cookie 或 `Authorization`，且带匹配的 `New-Api-User` |
| 生成管理 access token | `GET /api/user/token` | `UserAuth` | 会生成或覆盖用户 access token，不作为只读探测接口 |
| 当前用户分组 | `GET /api/user/self/groups` | `UserAuth` | 返回可用分组、描述和倍率 |
| 当前用户模型 | `GET /api/user/models` | `UserAuth` | 返回当前用户可用模型 |
| 价格与分组倍率 | `GET /api/pricing` | HeaderNav 规则，可能公共或登录态 | 后续用于费率同步 |
| 用户用量汇总 | `GET /api/data/self`、`GET /api/data/flow/self` | `UserAuth` | 后续用于用量采集；时间跨度最多 1 个月 |
| 用户 Key 管理 | `/api/token/*` | `UserAuth` | 后续 P2；涉及明文 Key 和审计 |
| Uptime Kuma 聚合 | `GET /api/uptime/status` | 公共 | 依赖 New API 后台配置的 Uptime Kuma 分组，不是 Codex APIs 当前 speed 页面 |
| Codex APIs speed/Pulse | `GET https://speed.codexapis.com/api/pulse` | 公共 | 返回最近 60 秒模型级 `avg_ttft_ms`、`avg_resp_sec`、`success_rate` 和 `health` |

## 响应事实

登录成功只返回用户摘要，认证事实在 Cookie 中：

```json
{
  "success": true,
  "data": {
    "id": 1,
    "username": "operator",
    "display_name": "operator",
    "role": 1,
    "status": 1,
    "group": "default"
  }
}
```

启用 2FA 时：

```json
{
  "success": true,
  "message": "需要进行两步验证",
  "data": {
    "require_2fa": true
  }
}
```

`GET /api/user/self` 的关键字段：

```json
{
  "success": true,
  "data": {
    "id": 1,
    "username": "operator",
    "display_name": "operator",
    "email": "user@example.com",
    "group": "default",
    "quota": 100000,
    "used_quota": 2000,
    "request_count": 12
  }
}
```

## 与 Sub2API 的差异

| 维度 | Sub2API | New API |
|------|---------|---------|
| 登录结果 | 可按用户侧 API/token 假设处理 | 登录写 session Cookie，不返回常规 JWT |
| 请求头 | 按 Sub2API 用户侧接口要求 | `UserAuth` 接口必须额外带 `New-Api-User: <user_id>` |
| access token | 可作为常规凭据 | `/api/user/token` 是写操作，默认不调用 |
| 业务失败 | 需要检查业务响应 | 很多失败仍是 HTTP 200，必须检查 `success` |
| 余额字段 | Sub2API profile 口径 | `quota`、`used_quota`、`request_count`，MVP 保存原始 quota |
| 渠道状态 | Sub2API 用户侧 `/api/v1/channel-monitors` | Codex APIs 通过独立 `speed.codexapis.com/api/pulse` 提供模型级实时状态 |
| 浏览器挑战 | 视供应商实现 | Turnstile/2FA 默认走 Chrome 插件已登录会话兜底 |
