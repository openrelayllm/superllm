# 测试与验收

## Playwright 真实站点流程

测试站点：`https://www.codexapis.com/`

已验证流程：

- 登录成功，进入 `/console`。
- 控制台首页能读取余额、历史消耗、请求次数、使用统计、公告和服务可用性。
- 令牌管理能进入，列表接口 `/api/token/?p=1&size=10` 返回 200。
- 使用日志能进入，统计和列表接口返回 200，页面显示日志数据。
- 钱包管理能进入，读取余额、历史消耗、请求次数和兑换码入口；未执行充值、兑换或划转写操作。
- 个人设置能进入，读取用户 ID、余额、分组和绑定邮箱；未执行安全设置写操作。
- 模型广场能进入，`/api/pricing` 返回 200，搜索可正常过滤模型。
- 直达 `/console/token` 可加载，核心 API 返回 200。
- 移动端 390x844 抽查令牌页卡片布局可用。
- Admin Plus 供应商页的“渠道状态”可读取 `https://speed.codexapis.com/api/pulse`，展示 `gpt-5.4-mini`、`gpt-5.5`、`gpt-5.4` 等模型的 60 秒延迟、响应耗时和成功率。
- Admin Plus 供应商分组“补齐 Key/账号”支持 New API：通过 `/api/token/search`、`POST /api/token/`、`POST /api/token/:id/key` 创建或复用第三方 Key，并同步创建本地 Sub2API account。

注意：早期网络日志中过 SPA 子路由的 503 记录，但直达复核未复现为阻断问题，核心 API 均为 200。

## 单元测试要求

- New API provider fake server 必须断言 `/api/user/self` 请求带 `New-Api-User`。
- 登录成功、`require_2fa`、Turnstile 失败、`success=false`、缺 Cookie、`/self` 401、`New-Api-User` 缺失或不匹配都要覆盖。
- suppliers service 覆盖自动创建 New API supplier，断言 `Type = new_api`、`BalanceCurrency = QTA`。
- extension ingest 覆盖浏览器一键上报 New API session，断言 summary、加密前 bundle 和传给 provider router 的 probe input 都包含 `New-Api-User`。
- New API provider fake server 覆盖 `/api/token/search` -> `POST /api/token/` -> `/api/token/:id/key` 两步创建和明文读取，断言请求同时带 Cookie 与 `New-Api-User`，并且响应快照不保存明文 key。
- supplierkeys service 覆盖 `SupplierTypeNewAPI` 的 `provision_all_group_keys`，断言能创建第三方 Key、本地 Sub2API account 和供应商账号绑定。

## 最小验收

1. 添加 `type = new_api` 供应商并配置 base URL、用户名、密码。
2. 后端直登真实调用 `POST /api/user/login`。
3. 登录成功后真实调用 `GET /api/user/self`，请求包含 Cookie 和 `New-Api-User`。
4. Admin Plus 保存 direct-login session，只展示脱敏状态。
5. 余额快照保存 `raw_quota`、`raw_used_quota`、`request_count`、`group` 和来源时间。
6. 再次探测复用已保存会话，能成功读取 `/api/user/self`。
7. Chrome 插件一键上报 New API 页面时，保存 Cookie + `New-Api-User`，并触发后端余额同步。
8. 供应商“渠道状态”对 Codex APIs 自动读取 `https://speed.codexapis.com/api/pulse`，返回 `system_type = new_api`、`api_base_url = https://speed.codexapis.com/api/pulse`，并至少展示一个模型卡片。
9. 供应商分组“补齐 Key/账号”对 New API 不再返回 `SUPPLIER_KEY_PROVIDER_UNSUPPORTED`；任务成功后每个 active group 至少有一个 bound supplier key 或明确的分组级失败原因。
10. 登录失败、2FA、Turnstile、session 过期、缺少 `New-Api-User`、接口不可达都有明确错误码。

## 后续扩展

P1：

- 读取 `/api/pricing` 归一化模型价格、`group_ratio` 和 `supported_endpoint`。
- 读取 `/api/data/self`、`/api/data/flow/self` 作为用户侧用量快照。

P2：

- 使用第三方 Key 做 OpenAI-compatible 健康探测和可用模型校验。
