# Sub2API Admin Plus 程序代码结构

版本：v0.2  
日期：2026-06-20  
定位：基于完整 Sub2API 复制基线演进的自动化运营扩展系统

## 1. 当前基线结论

`sub2api-admin-plus` 已完整复制 Sub2API 的前后端架构、UI 设计、构建脚本和部署目录。MVP 0 不再采用“新建一个完全独立 Go 服务目录”的结构，而是在当前复制基线上做业务隔离。

核心约束：

- 不修改 `/Users/coso/Documents/dev/go/sub2api` 上游项目。
- 当前仓库可以修改，但 Admin Plus 私有业务必须集中放在独立命名空间，减少后续同步上游时的冲突。
- 后端 module 路径暂时保留 `github.com/Wei-Shaw/sub2api`，MVP 0 不做全仓 import 迁移。
- 前端继续复用 Sub2API Vue/Vite/Tailwind/UI 风格，不使用 iframe。
- 不开发独立用户、角色、权限系统。
- 写 Sub2API 状态只走 Admin API；读 Sub2API 可以走 Admin API、只读数据库、只读 Redis。
- Admin Plus 写自己的 PostgreSQL 独立库，Redis 写入必须使用独立前缀。

## 2. 现有目录职责

```text
sub2api-admin-plus/
  backend/                  # 复制后的 Sub2API Go 后端，Admin Plus 后端业务在这里增量开发
  frontend/                 # 复制后的 Sub2API Vue 前端，Admin Plus 页面在这里增量开发
  deploy/                   # 复制后的部署目录，后续改为 Admin Plus 部署配置
  docs/                     # 产品、架构、基线和开发文档
  assets/                   # 复制后的静态资产
  skills/                   # 复制后的 Codex/运维辅助工具
  tools/                    # 复制后的工具脚本
  README.md                 # Admin Plus 项目说明
  DEV_GUIDE.md              # Admin Plus 开发指南
```

复制基线保留大量 Sub2API 原有模块是有意为之。MVP 0 的目标是先保证可运行和可构建，MVP 1 再逐步隐藏或清理非运营扩展所需的页面与路由。

## 3. 后端目标结构

Admin Plus 后端业务建议统一进入 `backend/internal/adminplus/`，避免散落到原 Sub2API 的 `service`、`repository`、`handler/admin` 中。

```text
backend/
  cmd/
    server/                         # MVP 0 继续复用现有入口

  internal/
    adminplus/
      config/                       # Admin Plus 自有配置解析
      domain/                       # Admin Plus 领域模型、枚举、业务错误
      app/
        sub2api/                    # Sub2API 只读身份、账号、用量和运行态适配
        suppliers/                  # 供应商父级台账、供应商下挂账号/Key 子级绑定
        suppliergroups/             # 供应商分组事实表，Provider Adapter 同步后落库
        supplierkeys/               # 第三方 Key 元数据、本地账号创建和绑定编排
        rates/                      # 费率抓取、快照、变更事件
        announcements/              # 公告监控、关键词分类、充值建议
        billing/                    # 账单导入、账单明细归一化
        reconciliation/             # 对账、成本、收入、毛利计算
        health/                     # 首 token、耗时、错误率、并发、健康分
        actions/                    # 切换/降权/暂停/调并发/充值建议
        extension/                  # Chrome 插件任务协议
        audit/                      # 操作审计、敏感凭据使用审计
      adapters/
        sub2api/
          local/                    # 本地 Sub2API Admin API/只读数据源
          sourceaccounts/           # OpenAI/Anthropic/Gemini 源站账号运营读取
          provider/                 # 上游也是 Sub2API 的供应商适配
          readonlydb/               # Sub2API 主库只读查询
          readonlyredis/            # Sub2API Redis 只读查询
        browser/                    # 网页后台/Chrome 插件抓取适配
        newapi/                     # New API 供应商预留适配
      clients/
        sub2apiadmin/               # Sub2API Admin API client
        chromeextension/            # Chrome 插件通信 client/model
      db/
        ent/                        # Admin Plus 自有 Ent schema/client，独立于 backend/ent
        migrations/                 # Admin Plus 独立库迁移
      redis/                        # Admin Plus Redis 前缀封装
      scheduler/                    # 定时任务、分布式锁、重试、幂等
      crypto/                       # 供应商凭据加密/脱敏
      ports/                        # 跨模块接口，降低具体实现耦合

    handler/
      adminplus/                    # Admin Plus HTTP handler

    server/
      routes/
        adminplus.go                # /api/admin-plus/* 路由注册
```

暂不建议新增 `cmd/admin-plus-api` 和 `cmd/admin-plus-worker`。当前阶段继续复用 `cmd/server`，通过独立路由、独立包和独立调度模块完成业务隔离。只有当后台 worker 与 Web 生命周期明显冲突时，再拆第二个入口。

### 3.1 已落地的 MVP1 后端业务面

当前已落地供应商台账最小闭环：

```text
backend/internal/adminplus/domain/supplier.go
backend/internal/adminplus/app/suppliers/
backend/internal/handler/adminplus/supplier_handler.go
backend/internal/server/routes/adminplus.go
```

已注册路由：

```text
GET   /api/v1/admin-plus/suppliers
POST  /api/v1/admin-plus/suppliers
GET   /api/v1/admin-plus/suppliers/:id
PATCH /api/v1/admin-plus/suppliers/:id/status
```

当前运行时使用 `SQLRepository` 持久化到 Admin Plus 自有表：

```text
admin_plus_suppliers
```

`MemoryRepository` 仅保留给单元测试使用，不进入 Wire 运行时对象图。部署时应把 SuperLLM 指向独立 PostgreSQL 库 `superllm`；可以复用同一个 PostgreSQL 实例，但不得指向 Sub2API 原生产库。

供应商状态规则已在业务层固化：

- `monitor_only` 允许无余额，用于无充值供应商的公告/费率监控。
- `candidate` 和 `active` 必须有正余额，否则不允许作为切换候选。
- 凭据响应只返回是否配置和脱敏值，不返回浏览器登录密码、临时 token、PostgreSQL DSN、Redis DSN 或可选管理 API Key 明文。

后续供应商模型必须保持父子结构：

```text
admin_plus_suppliers               # 供应商父级，保存供应商后台、类型、运行状态和采集凭据状态
admin_plus_supplier_groups         # 供应商真实分组，由 Provider Adapter 同步
admin_plus_supplier_keys           # 第三方 Key 脱敏元数据，按供应商分组创建
admin_plus_supplier_accounts       # 账号/Key 子级，绑定第三方 Key 和本地 Sub2API accounts.id
```

父级供应商不直接保存单个本地账号绑定字段。成本、健康、并发、对账和切换建议可以按父级聚合，也可以追溯到 `admin_plus_supplier_accounts` 子级。余额默认是我们在供应商侧的下游用户余额，属于供应商父级或供应商侧账户口径，不作为单个 Key 的余额字段展示。

当前账号落地主路径已经收敛到供应商管理页的“分组”弹窗：

- `GET /api/v1/admin-plus/suppliers/:id/groups` 读取分组事实表。
- `GET /api/v1/admin-plus/suppliers/:id/keys` 读取该供应商已开通 Key，并按 `supplier_group_id` 映射到分组行。
- 未绑定分组行通过 `POST /api/v1/admin-plus/suppliers/:id/keys/provision` 创建第三方 Key、同步创建本地 Sub2API 账号，并写入 `admin_plus_supplier_keys` 和 `admin_plus_supplier_accounts`。
- `keys/provision` 已接入通用 `Idempotency-Key` 去重和结果重放；同一供应商分组还通过 `admin_plus_supplier_keys` 唯一索引限制只能存在一个 `provisioning` / `bound` / `manual_secret_required` Key。
- 本地账号创建或绑定失败的 Key 通过 `POST /api/v1/admin-plus/suppliers/:id/keys/:keyID/repair-binding` 在分组弹窗内选择已有本地 Sub2API account 修复；该入口不调用 Provider Adapter，不创建第三方 Key。
- 独立账号/Key 绑定页当前只读展示已生成绑定，用作审计和历史查看；列表按 Sub2API API 密钥页密度展示本地账号名称、脱敏供应商 Key、供应商上下文、供应商分组名称、渠道颜色、倍率、真实本地用量 token 和账号成本金额，不展示单 Key 余额；不在该页保留创建、编辑或删除按钮。

当前已落地费率快照与变更事件最小闭环：

```text
backend/internal/adminplus/domain/rate.go
backend/internal/adminplus/app/rates/
backend/internal/handler/adminplus/rate_handler.go
backend/internal/server/routes/adminplus.go
```

已注册路由：

```text
POST  /api/v1/admin-plus/rates/snapshots
GET   /api/v1/admin-plus/rates/snapshots
GET   /api/v1/admin-plus/rates/events
PATCH /api/v1/admin-plus/rates/events/:id/ack
POST  /api/v1/admin-plus/suppliers/:id/rates/sync
```

当前运行时使用 `SQLRepository` 持久化到 Admin Plus 自有表：

```text
admin_plus_rate_snapshots
admin_plus_rate_change_events
```

费率监控的事实源是 `rates.Service.RecordSnapshot`。当前 Sub2API 同源供应商已支持通过已保存浏览器会话执行 `rates.Service.SyncFromSession`，由后端 Provider Adapter 读取供应商用户侧 API 并归一化后写入快照。Chrome 插件不解析费率，旧 `fetch_rates` 任务仅作为兼容路径；手工导入和后续 10 分钟调度任务也应调用同一服务，由同一业务规则比较上一条可比快照并生成变更事件。

费率可比维度：

- `supplier_id`
- `model`
- `billing_mode`
- `price_item`
- `unit`
- `currency`

变更事件规则：

- 第一次出现的费率生成 `new` 事件。
- 同价不生成事件。
- 涨价生成 `increase`，降价生成 `decrease`。
- 小于阈值的变化仍记录事件，但 `threshold_exceeded=false`，方便后续做审计和趋势分析。

当前已落地运营业务 API 与持久化层：

```text
backend/internal/adminplus/domain/balance.go
backend/internal/adminplus/domain/announcement.go
backend/internal/adminplus/domain/health.go
backend/internal/adminplus/domain/extension.go
backend/internal/adminplus/domain/billing.go
backend/internal/adminplus/domain/action.go
backend/internal/adminplus/app/balances/
backend/internal/adminplus/app/announcements/
backend/internal/adminplus/app/health/
backend/internal/adminplus/app/extension/
backend/internal/adminplus/app/scheduler/
backend/internal/adminplus/app/billing/
backend/internal/adminplus/app/reconciliation/
backend/internal/adminplus/app/actions/
backend/internal/adminplus/app/sub2api/
backend/internal/handler/adminplus/balance_handler.go
backend/internal/handler/adminplus/announcement_handler.go
backend/internal/handler/adminplus/health_handler.go
backend/internal/handler/adminplus/billing_handler.go
backend/internal/handler/adminplus/extension_handler.go
backend/internal/handler/adminplus/scheduler_handler.go
backend/internal/handler/adminplus/action_handler.go
backend/internal/handler/adminplus/sub2api_handler.go
backend/internal/adminplus/ports/provider.go
```

这些模块当前已进入 Wire 运行时对象图，并注册在 `/api/v1/admin-plus/*` 路由下。运行时默认使用 SQL repository；`MemoryRepository` 仅保留给单元测试和路由 surface 测试使用。

已注册业务路由：

```text
GET    /api/v1/admin-plus/suppliers
POST   /api/v1/admin-plus/suppliers
GET    /api/v1/admin-plus/suppliers/:id
PUT    /api/v1/admin-plus/suppliers/:id
DELETE /api/v1/admin-plus/suppliers/:id
PATCH  /api/v1/admin-plus/suppliers/:id/status
GET    /api/v1/admin-plus/suppliers/:id/accounts
POST   /api/v1/admin-plus/suppliers/:id/accounts
PUT    /api/v1/admin-plus/suppliers/:id/accounts/:accountID
DELETE /api/v1/admin-plus/suppliers/:id/accounts/:accountID
GET    /api/v1/admin-plus/suppliers/:id/groups
POST   /api/v1/admin-plus/suppliers/:id/groups/sync
GET    /api/v1/admin-plus/suppliers/:id/keys
POST   /api/v1/admin-plus/suppliers/:id/keys/provision
POST   /api/v1/admin-plus/suppliers/:id/keys/:keyID/repair-binding
GET    /api/v1/admin-plus/sub2api/accounts
GET    /api/v1/admin-plus/sub2api/account-runtime
GET    /api/v1/admin-plus/sub2api/usage-lines
GET    /api/v1/admin-plus/sub2api/usage-summary
POST   /api/v1/admin-plus/rates/snapshots
GET    /api/v1/admin-plus/rates/snapshots
GET    /api/v1/admin-plus/rates/events
PATCH  /api/v1/admin-plus/rates/events/:id/ack
POST   /api/v1/admin-plus/balances/snapshots
GET    /api/v1/admin-plus/balances/snapshots
GET    /api/v1/admin-plus/balances/events
PATCH  /api/v1/admin-plus/balances/events/:id/ack
POST   /api/v1/admin-plus/announcements
GET    /api/v1/admin-plus/announcements
PATCH  /api/v1/admin-plus/announcements/:id/ack
POST   /api/v1/admin-plus/health/probe
POST   /api/v1/admin-plus/health/samples
GET    /api/v1/admin-plus/health/samples
GET    /api/v1/admin-plus/health/events
PATCH  /api/v1/admin-plus/health/events/:id/ack
GET    /api/v1/admin-plus/notifications/deliveries
POST   /api/v1/admin-plus/billing/lines/import
GET    /api/v1/admin-plus/billing/lines
POST   /api/v1/admin-plus/suppliers/:id/billing/sync
POST   /api/v1/admin-plus/extension/tasks
GET    /api/v1/admin-plus/extension/tasks
POST   /api/v1/admin-plus/extension/tasks/claim
POST   /api/v1/admin-plus/extension/tasks/:id/heartbeat
POST   /api/v1/admin-plus/extension/tasks/:id/browser-credential
POST   /api/v1/admin-plus/extension/tasks/:id/complete
POST   /api/v1/admin-plus/extension/tasks/:id/fail
GET    /api/v1/admin-plus/scheduler/status
POST   /api/v1/admin-plus/scheduler/run
POST   /api/v1/admin-plus/reconciliation/run
POST   /api/v1/admin-plus/actions/generate
GET    /api/v1/admin-plus/actions/recommendations
PATCH  /api/v1/admin-plus/actions/recommendations/:id/status
```

本地 Sub2API 只读适配当前能力：

- `GET /admin-plus/sub2api/accounts` 从 Sub2API 只读库读取真实 `accounts`，用于供应商父级下挂本地账号/Key 子级。
- `GET /admin-plus/sub2api/usage-lines` 从 `usage_logs` 读取真实本地用量明细，供对账使用。
- `GET /admin-plus/sub2api/usage-summary` 按账号和模型聚合真实请求数、token、收入、原始成本和延迟。
- 配置 `SUB2API_READONLY_DATABASE_URL` 后读取独立 Sub2API 库；未配置时回退当前连接，仅适合本地单库 MVP 验证。
- 线上 `sub2api-plus` 页面不会直接抓取 `sub2api` 后台页面里的账号列表，而是通过 Admin Plus 后端查只读库。线上读取不到账号时，优先检查 `SUB2API_READONLY_DATABASE_URL` 是否指向真实 Sub2API 生产库，且数据库用户是否拥有 `accounts` 只读权限。

已修正隔离约束：

- `admin_plus_supplier_accounts.local_sub2api_account_id` 是对 Sub2API `accounts.id` 的逻辑引用，不再建立跨库外键。
- Admin Plus 自有业务数据仍写入 Admin Plus 独立库。
- 对 Sub2API 数据的读取集中在只读 adapter/repository 层，不直接写 Sub2API 主库。

仍未完成的真实自动化能力：

- 面向具体 Sub2API/New API 供应商后台的稳定 Chrome 页面适配器。
- 每日账单自动导出文件下载、上传和解析。
- Sub2API 窗口成本、深度 Channel Monitor 指标和动作执行写入适配。
- 确认后调用 Sub2API Admin API 执行动作建议。
- 多通道通知和操作审计闭环。

余额监控规则：

- 记录供应商余额快照时计算 `switch_eligible`。
- 只有 `candidate` / `active` 且余额大于 0 的供应商可以作为切换候选。
- `monitor_only` 即使有余额，也只监控，不参与切换。
- 余额从正数变成 0 生成 `depleted` 事件。
- 余额首次或从高于阈值跌到低于阈值生成 `low_balance` 事件。
- 余额从 0 或低余额恢复到可用区间生成 `recovered` 事件。

公告监控规则：

- 公告监控是一级能力，充值赠送、费率折扣、套餐、维护、故障和通知都是关键词分类结果。
- 无余额供应商仍可记录公告事件。
- 成本类公告在无余额供应商上只生成 `recharge_to_unlock` 建议，提醒及时充值以获取更低成本。
- 成本类公告在有余额且状态为 `candidate` / `active` 的供应商上生成 `switch_candidate` 建议。
- 维护、故障和普通通知只作为 `informational` 信息，不进入切换候选。

健康监控规则：

- `POST /api/v1/admin-plus/health/probe` 对供应商账号/Key 子级绑定的本地 OpenAI-compatible 账号发起真实 Responses 流式请求。
- 探测默认模型为 `gpt-5.5`，API Key 和 base URL 只从本地 Sub2API `accounts.credentials` 读取，前端不输入、不展示 API Key。
- 探测目标优先使用子账号凭据中的 `base_url`，其次使用供应商父级 `api_base_url`，最后回退 OpenAI 官方地址。
- 记录供应商首 token 耗时、总耗时、HTTP 状态码、错误类别、观察到的并发数。
- 首 token 超过阈值生成 `slow_first_token` 事件。
- 总耗时超过阈值生成 `slow_total` 事件。
- HTTP 状态码大于等于 400 或存在错误类别时生成 `request_error` 事件。
- 观察并发达到配置饱和比例时生成 `concurrency_full` 事件，用于后续切换或降权建议。

Chrome 插件任务规则：

- 当前 Chrome 插件主任务类型是 `capture_supplier_session`：识别当前供应商网站、一键登录或读取已登录浏览器会话、上报会话包。
- `fetch_rates`、`fetch_groups`、`fetch_balance`、`fetch_announcements`、`fetch_health`、`export_bills` 作为任务类型名保留，但调度中心显式选择时直接执行后端同步，不再写入插件业务采集队列。
- 插件设备通过 `device_id` 领取任务，领取后获得短期 `lease_token`。
- 插件读取供应商浏览器凭据必须提交 `task_id + device_id + lease_token`，且任务必须处于 `claimed` 或 `running`、租约未过期。
- 供应商浏览器登录账号、密码和临时 token 使用现有 AES-GCM `SecretEncryptor` 加密落库，普通供应商列表/详情 API 不返回明文。
- 完成 `capture_supplier_session` 时，后端删除明文 `session_bundle`，只在任务结果中保留 `session_summary`，并把加密后的会话包放入 ingest 结果。
- SQL repository 领取任务使用 `FOR UPDATE SKIP LOCKED` 原子更新，避免多个 Chrome 设备同时领取同一任务。
- 心跳会刷新租约并把任务推进到 `running`。
- 完成任务写入结果并进入 `succeeded`。
- 完成 `capture_supplier_session` 后，余额、分组、费率、公告、健康、账单和第三方 Key 创建主路径都由后端 Provider Adapter 或后端 app service 使用已保存会话/本地账号执行；插件不解析和上报业务结果。
- `fetch_rates`、`fetch_balance`、`fetch_announcements`、`fetch_health`、`export_bills` 的结构化结果摄取仅保留为 compat 路径，不再作为插件 Popup 或新能力的主交互继续增强。
- 失败任务在未超过 `max_attempts` 前回到 `pending`，超过后进入 `failed`。
- 当前 `MemoryRepository` 仅用于单元测试，不进入运行时对象图。

调度中心规则：

- 调度中心和插件任务共享 `admin_plus_extension_tasks` 队列；默认只生成 `capture_supplier_session` 会话上报任务。
- `POST /api/v1/admin-plus/scheduler/run` 是统一调度入口：默认创建插件会话上报任务；显式选择 `fetch_groups`、`fetch_rates`、`fetch_balance`、`fetch_announcements`、`fetch_health`、`export_bills` 时直接调用后端 Provider Adapter / app service 同步分组、费率、余额、公告、健康和账单，不写插件队列。
- 旧插件结构化业务结果摄取仅作为 compat 补录路径保留，调度中心不再创建 `fetch_*` / `export_bills` 插件业务任务。
- 使用 `schedule_key` 对同一供应商、任务类型和时间窗口做幂等去重。
- 默认窗口为 10 分钟，账单导出使用日级窗口。
- 停用、暂停、凭据失效的供应商不执行调度；插件任务仍要求启用浏览器登录、配置后台地址和登录凭据；后端直连同步只要求供应商已配置后台或 API 地址并存在可解密会话。
- 无余额或不可切换供应商仍可生成费率、分组、余额、公告监控任务，但不会生成健康探测和账单同步任务。
- 周期 Worker 默认启用，可通过 `ADMIN_PLUS_SCHEDULER_ENABLED=false` 关闭，通过 `ADMIN_PLUS_SCHEDULER_INTERVAL_SECONDS` 调整间隔。

账单与对账规则：

- 供应商账单导入统一归一化为 `SupplierBillLine`，金额以 cents 计。
- 第三方账单明细必须显式保存 API Key 名称、端点、请求类型、计费模式、推理强度、输入 token、输出 token、缓存读取 token、总 token、费用、首 token、总耗时、User-Agent 和原始 payload；这些字段不能只依赖 `raw_payload`。
- 本地 Sub2API 使用记录归一化为 `LocalUsageLine`，后续由只读适配器从 Sub2API 主库读取。
- 优先按 `external_request_id` 匹配供应商账单和本地使用记录。
- 无请求 ID 时按 `model + 时间容忍窗口` 匹配。
- 供应商有账单但本地无使用记录标记为 `supplier_only`。
- 本地有使用记录但供应商无账单标记为 `local_only`。
- 币种不一致标记为 `currency_mismatch`。
- 收入低于成本或过于接近成本标记为 `cost_mismatch`。
- 对账输出收入、成本、毛利和毛利率，用于判断中转商是否仍有合理利润。

动作建议规则：

- 只生成 `ActionRecommendation`，不直接调用 Sub2API Admin API。
- 动作建议 SQL repository 只负责保存、查询和更新状态，不参与建议生成算法。
- `active` 供应商余额耗尽时生成暂停和切换建议。
- 无余额供应商的成本类公告只生成充值建议，不生成切换建议。
- 请求错误生成暂停建议；如果存在可用候选，再生成切换建议。
- 首 token 慢、总耗时慢或并发饱和生成降权建议。
- 低毛利率生成利润排查建议，避免中转商长期亏损。
- 所有动作默认 `requires_approval=true`，后续执行层必须由管理员确认。

通知规则：

- 余额事件、费率变更事件、公告事件、健康事件和对账异常当前已接入飞书自定义机器人。
- 通用配置使用 `ADMIN_PLUS_FEISHU_WEBHOOK_URL` 和 `ADMIN_PLUS_FEISHU_WEBHOOK_SECRET`。
- 兼容旧余额变量 `ADMIN_PLUS_FEISHU_BALANCE_WEBHOOK_URL` 和 `ADMIN_PLUS_FEISHU_BALANCE_WEBHOOK_SECRET`。
- 发送前写入 `admin_plus_notification_deliveries`，同一业务事件同一通道通过 `dedupe_key` 去重。
- 费率、健康和公告等高频事件使用窗口化 `dedupe_key` 限流，避免同一事件窗口重复刷屏。
- 通知成功或失败都会记录投递状态，不回滚业务快照或事件。
- 通知中心已提供飞书配置、测试诊断、规则开关、防打扰、投递记录和失败投递重试；当前尚未完成多通道通知。

### 3.2 当前治理分类

后续开发必须继续收敛到以下事实源：

```text
current:
  backend/internal/adminplus/domain/*
  backend/internal/adminplus/app/suppliers/*
  backend/internal/adminplus/app/suppliergroups/*
  backend/internal/adminplus/app/supplierkeys/*
  backend/internal/adminplus/app/rates/*
  backend/internal/adminplus/app/balances/*
  backend/internal/adminplus/app/announcements/*
  backend/internal/adminplus/app/health/*
  backend/internal/adminplus/app/extension/*
  backend/internal/adminplus/app/billing/*
  backend/internal/adminplus/app/reconciliation/*
  backend/internal/adminplus/app/actions/*
  backend/internal/adminplus/ports/*
  backend/internal/handler/adminplus/*
  backend/internal/server/routes/adminplus.go
  /api/v1/admin-plus/*
  backend/internal/handler/admin/dashboard*
  backend/internal/handler/admin/group_handler.go     # 只读 /api/v1/admin/groups/all，直接依赖 GroupService
  backend/internal/handler/admin/setting_handler.go
  backend/internal/handler/admin/ops_*
  backend/internal/handler/admin/system_handler.go
  backend/internal/server/routes/admin.go             # 仅 Dashboard / Groups(all) / Settings / Ops / System
  cmd/server runtime cleanup/startup                 # 只启动 Admin Plus 运营任务和必要只读/认证兼容任务
  backend/internal/service/ops_service.go             # 只依赖 Ops 仓储、设置、账号/用户只读仓储、并发服务和系统日志 sink
  ConcurrencyService                                  # 仅用于 Ops 并发监控读取，不启动 Sub2API 并发 key 清理 worker
  ProvideAdminPlusAuthService                         # 只注入登录、JWT、refresh token、Turnstile、邮件/TOTP 所需依赖

compat:
  backend/internal/handler/auth_handler.go            # 运行时只挂 login / login2fa / refresh / logout / me
  backend/internal/handler/auth_profile_response.go   # /auth/me 兼容响应 DTO 映射
  backend/internal/handler/setting_handler.go          # 公开设置，支撑登录页和前端初始化
  backend/internal/service/group_service.go            # GroupHandler 只使用只读分组方法，避免回拉 AdminService
  backend/internal/service/*                           # 原 Sub2API service 仅作为认证、只读分析、运行时依赖和迁移期兼容层保留
  复制自 Sub2API 的 service/repository/schema/cache，只允许为 Admin Plus 读取、认证复用、运行时依赖和后续迁移服务。

dead:
  已删除公开网关 handler、用户端 handler、支付/兑换/订阅/API Key/用量/个人资料 handler。
  已删除旧后台账号、渠道、用户、支付、兑换、订阅、公告、代理、OAuth、数据导入等 CRUD handler。
  已删除未注册的 OAuth 注册、邮箱注册、密码找回、用户 TOTP 设置和 revoke-all-sessions 认证分支。
  注册、找回密码、OAuth 绑定/登录、用户端 TOTP 设置、撤销所有会话等入口不得重新挂载为 Admin Plus 业务入口。
  原 Sub2API 写运行态后台任务不得进入 Admin Plus cmd/server startup/cleanup 对象图：TokenRefreshService、AccountExpiryService、ProxyExpiryService、SubscriptionExpiryService。
  原 Sub2API 网关转发运行态不得通过 OpsService 回流进 Admin Plus 启动对象图：GatewayService、OpenAIGatewayService、GeminiMessagesCompatService、AntigravityGatewayService 及其 token provider / upstream forwarder 依赖。
  Admin Plus 启动期不得清理或写入 Sub2API 原生并发 Redis key：CleanupStaleProcessSlots、StartSlotCleanupWorker 只能留在 compat 源码中，不进入 runtime provider。
  原 Sub2API 用户端注册/促销/兑换/返利/默认订阅依赖不得通过 AuthService 回流进 Admin Plus 启动对象图：NewAuthService 在 runtime 由 ProvideAdminPlusAuthService 替代，NewSubscriptionService 不进入 cleanup。
```

## 4. 前端目标结构

前端不是当前重点，但如果需要页面，应沿用 Sub2API 现有 Vue 结构和 UI 风格。

```text
frontend/src/
  api/adminplus/                   # Admin Plus API 封装
  components/adminplus/            # Admin Plus 业务组件
  views/adminplus/                 # Admin Plus 管理页面
  stores/adminplus/                # Admin Plus Pinia store
  router/                          # 增加 Admin Plus 路由
```

前端原则：

- 不做 iframe。
- 不做独立权限系统。
- 登录页可以存在于 Admin Plus，但用户、密码哈希、角色、状态和 TOTP 均从 Sub2API 只读数据库读取。
- 页面视觉、表格、弹窗、表单、布局复用 Sub2API 现有风格。
- MVP 1 优先做运营后台页面，不做营销页。

## 5. Chrome 插件结构

```text
extension/
  manifest.json
  README.md
  src/
    background.js
    popup.html
    popup.css
    popup.js
    lib/
      admin-plus-client.js
      storage.js
    content/
      parser.js
      sub2api.js
  test-parser.cjs
```

插件只负责浏览器侧能力：

- 识别当前 active tab 是否匹配已配置供应商。
- 在 sub2apiplus Web 已登录时连接插件；未登录时只打开 Web 登录页，不在插件内设计登录表单。
- 使用供应商后台账号密码或临时 token 自动登录，或读取当前已登录状态。
- 采集前端 storage、可访问 Cookie、HttpOnly Cookie、CSRF 和必要请求上下文，形成供应商会话包。
- 将会话包上报给 Admin Plus，由后端使用会话 API 完成费率、余额、公告、账单和健康采集。
- 页面 DOM 解析、截图、CSV 下载只作为供应商无可用会话 API 时的兜底能力。
- 上报任务状态、错误和最小诊断信息。

插件不设计 Sub2API 管理员登录 UI，不匿名上传第三方会话包，不参与 Admin Plus 用户权限体系。当前过渡实现从已登录 sub2apiplus 页面读取本域 `auth_token` 后保存在扩展本地存储；最终应替换为可吊销的插件设备 token / pairing code。

当前 `extension/` 是 MV3 会话获取器：Popup 只展示连接状态、当前网站识别、供应商匹配、登录状态和“一键登录/获取并上报”；Background 创建 `capture_supplier_session` 租约任务、读取供应商浏览器凭据、采集 Cookie 并完成上报；Content script 负责供应商页面登录辅助和 storage 会话提取。`parser.js` 和旧任务领取协议作为浏览器兜底/兼容路径保留，不能作为 Chrome 插件新的主路径继续扩展。

## 6. 认证复用

当前后端身份边界：

```text
backend/internal/adminplus/app/sub2api/identity_repository.go
backend/internal/service/auth_service.go
backend/internal/server/middleware/admin_auth.go
```

现有路由：

```text
POST /api/v1/auth/login
POST /api/v1/auth/login/2fa
POST /api/v1/auth/refresh
POST /api/v1/auth/logout
GET  /api/v1/auth/me
```

实现规则：

- Admin Plus 可以有自己的登录页。
- 登录请求由 SuperLLM 使用 `SUB2API_READONLY_DATABASE_URL` 直接读取并验证 Sub2API 用户。
- 只允许 Sub2API 中 `role=admin`、`status=active` 且未删除的用户登录。
- SuperLLM 签发自己的 access/refresh token，并在鉴权时重新读取 Sub2API 身份和密码派生版本。
- Admin Plus 不保存管理员密码。
- Admin Plus 不维护用户表、角色表和权限表。
- 缺少只读身份源时认证失败关闭，不回退到 SuperLLM 主库的 `users` 表。

## 7. 数据库与 Redis

### 7.1 Admin Plus 自有数据库

Admin Plus 自有表使用独立 PostgreSQL 库，例如：

```text
superllm
```

建议自有表前缀：

```text
admin_plus_suppliers
admin_plus_supplier_groups
admin_plus_supplier_keys
admin_plus_supplier_accounts
admin_plus_supplier_credentials
admin_plus_rate_snapshots
admin_plus_rate_change_events
admin_plus_announcement_events
admin_plus_bill_imports
admin_plus_bill_items
admin_plus_reconciliation_runs
admin_plus_usage_reconciliation_items
admin_plus_health_metrics
admin_plus_action_recommendations
admin_plus_extension_tasks
admin_plus_audit_logs
```

如果使用 Ent，优先放在 `backend/internal/adminplus/db/ent`，不要和复制来的 `backend/ent` 混在一起。这样可以避免 Admin Plus 迁移污染 Sub2API 原有 schema。

### 7.2 Sub2API 主库读取

Sub2API 主库只读连接放在：

```text
backend/internal/adminplus/adapters/sub2api/readonlydb
```

允许读取：

- `accounts`
- `usage_logs`
- `channel_monitors`
- `channel_monitor_histories`
- `channel_monitor_daily_rollups`
- 其它经确认只读安全的分析表

禁止直接写入 Sub2API 主库。

### 7.3 Redis

Admin Plus 写入共享 Redis 时必须加前缀：

```text
admin_plus:
```

Sub2API Redis 只读适配器放在：

```text
backend/internal/adminplus/adapters/sub2api/readonlyredis
```

允许读取明确列入白名单的 key，例如：

```text
session_limit:account:{accountID}
window_cost:account:{accountID}
```

禁止：

- 写无前缀 key。
- 写 Sub2API 原生 key。
- 执行 `FLUSH*`。
- 清理非 `admin_plus:` 前缀的数据。

## 8. 供应商适配器

统一接口建议放在：

```text
backend/internal/adminplus/ports/provider.go
```

建议能力：

```text
ProviderAdapter
  FetchRateCatalog()
  FetchUsageBills()
  FetchBalance()
  FetchConcurrency()
  FetchHealthMetrics()
  FetchModels()
  ProbeCredential()
  ExportBillsByBrowserTask()
  ValidateCredential()
```

MVP 1 优先实现：

- `Sub2APISourceAccountAdapter`：读取并运营 Sub2API 已添加的 OpenAI、Anthropic、Gemini 源站账号。
- `Sub2APIProviderAdapter`：上游供应商也是 Sub2API 部署实例。

后续再补：

- `NewAPIProviderAdapter`
- `BrowserOnlyProviderAdapter`
- `CustomProviderAdapter`

## 9. 业务模块职责

### `suppliers`

- 供应商父级 CRUD。
- 供应商类型：`source_account`、`relay`、`browser_only`、`custom`。
- 供应商运行状态：`monitor_only`、`candidate`、`active`、`disabled`。
- 供应商后台登录凭据状态：账号、密码、临时 token、只读 DB、只读 Redis。
- 供应商下挂账号/Key 子级绑定。
- 子级绑定本地 Sub2API `accounts.id`，并缓存账号名称、平台、类型和调度状态快照。
- 供应商父级管理页已对齐 Sub2API 后台表格工作台形态：筛选条、右侧工具、选择列、批量状态、批量删除、行内编辑、行内状态、账号入口和删除确认。
- 账号/Key 子级绑定页已对齐 Sub2API API 密钥列表形态：供应商筛选、本地账号搜索、分页、刷新、名称/API Key、分组、真实用量、状态、并发、创建时间和跳转分组操作。分组使用 Sub2API `GroupBadge` 同款名称、渠道颜色和倍率展示；该页不提供新增、编辑、删除和批量操作。

无余额供应商只能监控费率和公告，不能进入切换候选。切换候选实际落在子账号/Key 维度，但余额门禁使用父级供应商余额口径；子级负责成本、健康、并发和对账追溯。

### `rates`

- 每 10 分钟抓取费率。
- 记录费率快照。
- 对比费率变化。
- 生成变更事件和通知。
- 计算费率变化对毛利的影响。

### `announcements`

- 读取供应商公告、通知、充值页和活动页。
- 按关键词分类为充值赠送、费率折扣、套餐、限时公告、维护、故障、普通通知。
- 成本类公告可以生成充值或调权建议；非成本类公告只生成信息或风险信号。

### `billing`

- 使用已保存供应商会话触发 `billing.Service.SyncFromSession`。
- 由后端 Provider Adapter 执行 `ReadBilling(session, date_range)`，Sub2API 同源供应商优先读取用户侧 `/api/v1/usage`，并写入 `admin_plus_supplier_bill_lines`。
- 手工导入和旧插件导出只作为补录/compat 路径，不作为账单主事实源。

### `reconciliation`

- 对齐供应商账单与本地 Sub2API 使用记录。
- 计算收入、成本、毛利和毛利率。
- 标记未匹配、成本异常、token 差异。

### `health`

- 采集首 token 时间、总耗时、错误率、可用率。
- 采集余额、额度、可并发数。
- 对绑定到供应商账号/Key 子级的本地 OpenAI-compatible 账号执行 `/v1/responses` 探测。
- 计算供应商父级和账号/Key 子级健康分。

### `actions`

- 生成暂停、恢复、降权、升权、调并发、切换、充值建议。
- 管理员确认后才调用 Sub2API Admin API 执行动作。
- 所有动作必须审计和可回滚评估。

### `extension`

- 管理 Chrome 插件设备。
- 创建、领取、心跳、完成、失败任务。
- 接收文件、截图和错误上下文。

### `audit`

- 记录敏感凭据使用。
- 记录所有外部写操作。
- 记录自动化建议确认和执行。

## 10. 路由规划

```text
/api/v1/admin-plus/auth/*
/api/v1/admin-plus/suppliers
/api/v1/admin-plus/suppliers/:id/accounts
/api/v1/admin-plus/sub2api/accounts
/api/v1/admin-plus/sub2api/account-runtime
/api/v1/admin-plus/rates
/api/v1/admin-plus/announcements
/api/v1/admin-plus/bills
/api/v1/admin-plus/reconciliation
/api/v1/admin-plus/health
/api/v1/admin-plus/actions
/api/v1/admin-plus/extension
/api/v1/admin-plus/audit
```

路由规则：

- 管理接口必须通过 Sub2API 管理员身份校验。
- 插件接口使用短期设备 token。
- 写操作必须带幂等键或业务去重键；当前 `keys/provision` 和 `keys/:keyID/repair-binding` 使用 `Idempotency-Key`，同分组唯一 Key 守卫防止重复开通。
- 执行动作必须记录审计。

## 11. 定时任务

建议放在：

```text
backend/internal/adminplus/scheduler
```

MVP 1 任务：

- `rate_poll_job`：每 10 分钟抓取费率。
- `announcement_poll_job`：定时同步公告并执行关键词分类。
- `balance_poll_job`：定时检查余额和额度。
- `health_probe_job`：定时测速和并发探测。
- `bill_export_job`：每天触发账单导出。
- `reconciliation_job`：每天对账。
- `action_generation_job`：生成运营建议。
- `extension_task_timeout_job`：处理插件任务超时。

任务要求：

- 支持分布式锁。
- 支持失败重试。
- 支持幂等。
- 支持任务审计。
- 不因单个供应商失败阻塞全部任务。

## 12. 依赖方向

允许：

```text
handler/adminplus -> adminplus/app
adminplus/app -> adminplus/ports
adminplus/app -> adminplus/db
adminplus/app -> adminplus/redis
adminplus/adapters -> adminplus/clients
adminplus/adapters -> copied Sub2API DTO/util when necessary
adminplus/actions -> clients/sub2apiadmin
```

禁止：

```text
adminplus/app -> backend/internal/service 的复杂业务实现
adminplus/app -> Sub2API 主库写操作
adminplus/app -> Sub2API Redis 写操作
adminplus/adapters -> 无白名单 Redis key
Chrome 插件 -> Sub2API 管理员 token
```

原则是业务依赖接口，不依赖复制来的 Sub2API 内部实现细节。确实需要复用 Sub2API 代码时，优先复制小型 DTO、常量或校验函数到 Admin Plus 命名空间，并记录来源。

## 13. 最小落地顺序

1. 在 `backend/internal/adminplus/config` 增加 Admin Plus 配置。
2. 在 `backend/internal/adminplus/app/sub2api` 通过只读数据库完成 Sub2API 管理员身份读取和校验。
3. 在 `backend/internal/server/routes/adminplus.go` 注册 `/api/admin-plus/*`。
4. 在 `backend/internal/handler/adminplus` 增加 `auth/me` 和健康检查接口。
5. 建立 Admin Plus 独立 DB 连接和迁移目录。
6. 建立 Redis 前缀封装。
7. 建立 Sub2API Admin API client。
8. 建立 Sub2API 只读 DB/Redis adapter。
9. 实现供应商台账。
10. 实现供应商下挂账号/Key 子级绑定。
11. 实现本地 Sub2API 账号选择读取接口。
12. 实现 Sub2API 源站账号读取适配。
13. 实现 Sub2API 供应商费率抓取。
14. 实现费率快照和变更事件。
15. 实现余额、公告和健康采集。
16. 实现供应商用户侧账单同步和对账。
17. 实现自动化建议和管理员确认执行。
18. 实现 Chrome 插件任务协议。

Sub2API 同源供应商 Provider Adapter 当前接口口径：

| 能力 | 供应商用户侧接口 | 状态 |
|------|------------------|------|
| 余额/profile | `GET /api/v1/user/profile` | 已落地 |
| 分组 | `GET /api/v1/groups/available`、`GET /api/v1/groups/rates` | 已落地 |
| 费率 | `GET /api/v1/rates/snapshots`、`GET /api/v1/channels/available` | 已落地 |
| 公告 | `GET /api/v1/announcements`、`GET /api/v1/payment/checkout-info` | 已落地，待真实供应商联调 |
| 账单 | `GET /api/v1/usage` | 已落地，待真实供应商联调 |
| Key 创建 | `POST /api/v1/keys` | 基础链路已落地，必须管理员确认 |

能力探测只使用 GET 请求，`GET /api/v1/keys?page=1&page_size=1` 仅表示 Key 管理入口可访问，不会创建第三方 Key。

## 14. 代码审查重点

- 是否误改 `/Users/coso/Documents/dev/go/sub2api`。
- 是否把 Admin Plus 业务散落到 Sub2API 原有 service/repository 中。
- 是否直接写入 Sub2API 主库。
- 是否直接写入 Sub2API Redis key。
- 是否绕过 Admin API 修改 Sub2API 状态。
- 是否记录敏感操作审计。
- 是否对凭据加密和脱敏。
- 是否把无余额供应商或无余额账号/Key 子级加入切换候选。
- 是否误做 OpenAI、Anthropic、Gemini 源站账号添加功能。
- 是否保存了源站账号 API Key、OAuth、Cookie 等应由 Sub2API 管理的凭据。
- 是否把账号采购预留模块误做成下单、付款、售后系统。

## 15. 下一阶段代码结构

M2/M3 阶段优先补真实数据源、真实供应商采集、动作执行和审计，不先做新的大抽象层。

```text
backend/internal/adminplus/
  app/
    sub2api/                 # 已落地：本地 Sub2API accounts / usage_logs / Redis 运行态只读查询
    scheduler/               # 已落地基础任务生成；待补超时回收、失败重试可视化和审计
    notifications/           # 已落地飞书发送、SQL 投递审计、事件级去重、窗口限流、对账异常通知、测试诊断、查询接口和失败重试；待补多通道通知
    audit/                   # 待开发：凭据使用、动作确认、外部写操作审计
  adapters/
    sub2api/
      readonlyredis/         # 待开发：窗口成本等更深层 Redis 适配；账号并发运行态已在 app/sub2api 落地
      provider/              # 待开发：上游供应商也是 Sub2API 时的 Admin API/DB 适配
    browser/
      chromeextension/       # 待开发：真实 Sub2API/New API 后台页面 adapter、截图/文件上传解析
  clients/
    sub2apiadmin/            # 待开发：确认执行动作时调用本地 Sub2API Admin API

frontend/src/views/admin/operations/
  LocalUsageView.vue         # 已落地：真实 usage_logs 聚合查看
  AccountRuntimeView.vue     # 已落地：真实 accounts + Redis 并发运行态查看
  BillingReconciliationView.vue # 已接入真实本地 usage_lines
```

下一阶段最小验收：

- `SUB2API_READONLY_DATABASE_URL` 指向真实 Sub2API 库时，账号绑定和本地用量页面不依赖 Admin Plus 自有库中的复制表。
- `SUB2API_READONLY_REDIS_URL` 或 `SUB2API_READONLY_REDIS_DB` 只读读取并发 key，不写、不删、不 flush。
- scheduler 生成的任务必须持久化、可重试、可审计。
- Chrome 插件完成真实网页登录和页面数据回传前，不能把费率/余额/账单自动采集标记为完成。
- 飞书通知已经覆盖余额、费率、健康、公告和对账异常事件，并具备 SQL 投递审计、事件级去重、窗口限流、通知记录页面、测试诊断和失败投递重试；完成多通道前仍按单飞书通道运行。
- 动作建议执行前必须有管理员确认，执行时必须调用本地 Sub2API Admin API，并记录执行前后快照。
