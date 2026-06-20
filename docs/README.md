# Sub2API Admin Plus 文档索引

更新日期：2026-06-20

## Review 结论

当前 `docs` 目录已经覆盖 PRD、代码结构、MVP0 基线、支付遗留文档和合规说明，但存在三个需要持续修正的问题：

- PRD 内容完整，但“目标能力”和“当前已完成能力”边界曾经不够硬，容易把插件任务协议、测试夹具或通用解析器误读为真实供应商采集完成。
- 部分文档来自 Sub2API 复制基线，例如支付系统文档，不属于 Admin Plus 当前业务范围，应作为上游遗留参考，不作为本项目 MVP 需求。
- 下一阶段需要优先完成真实数据闭环：首批 Sub2API 供应商适配、真实 Chrome 页面适配、每日账单自动导出、动作确认执行、多通道通知和操作审计。

本次 review 后采用以下口径：

- “已完成”只能用于已经走真实 HTTP handler、service、SQL repository、前端真实 API 或真实只读 Sub2API DB/Redis 的能力。
- E2E 中的 `e2e-*` 数据、本地 OpenAI-compatible `/v1/responses` 测试服务和插件解析样例只证明链路有效，不证明外部供应商生产采集已完成；E2E 默认清理本次夹具，历史夹具用 `node tools/cleanup-admin-plus-e2e.mjs` dry-run 检查。
- Chrome 插件相关能力只有在真实登录目标供应商页面、读取实际页面数据并回传后，才能标记为该供应商适配完成。
- 自动执行只有在管理员确认后真实调用本地 Sub2API Admin API，并写入审计日志后，才能标记完成。

## 文档地图

| 文档 | 用途 | 状态 |
|------|------|------|
| `sub2api-admin-plus-prd.md` | 产品目标、用户故事、架构、流程、接口草案、验收标准和当前进度 | 当前事实源 |
| `code-structure.md` | 后端、前端、插件、数据隔离、模块边界和开发顺序 | 当前事实源 |
| `mvp0-baseline.md` | 复制 Sub2API 后的可运行基线、来源 commit 和阶段进度 | 当前事实源 |
| `roadmap/restructure/README.md` | 下一阶段重新梳理顺序、插件并行协作、阶段验收和清理计划 | 当前事实源 |
| `roadmap/Chrome/README.md` | Chrome 插件作为供应商浏览器会话获取器的路线图 | 当前事实源 |
| `roadmap/accounts/README.md` | 供应商分组、第三方密钥、本地 Sub2API 账号创建与绑定流程 | 当前事实源 |
| `legal/admin-compliance.zh.md` | 中文合规责任说明 | 保留 |
| `legal/admin-compliance.en.md` | 英文合规责任说明 | 保留 |
| `PAYMENT.md` | Sub2API 原支付系统英文文档 | 上游遗留参考，不是 Admin Plus MVP 范围 |
| `PAYMENT_CN.md` | Sub2API 原支付系统中文文档 | 上游遗留参考，不是 Admin Plus MVP 范围 |
| `ADMIN_PAYMENT_INTEGRATION_API.md` | Sub2API 原外部支付集成 API 文档 | 上游遗留参考，不是 Admin Plus MVP 范围 |

## 当前优先级

1. 先按 `roadmap/restructure/README.md` 冻结术语、插件会话包 schema、插件任务接口、Provider Adapter capability 和安全白名单。
2. 插件开发可以并行推进，但插件只负责站点识别、授权、一键登录、会话包采集和上报；业务采集结果以后端 Provider Adapter 为准。
3. 首批供应商优先支持上游也是 Sub2API 的系统。
4. 供应商是父级，供应商账号/Key 是子级，所有成本、余额、健康、对账和切换建议最终必须能落到子级。
5. Admin Plus 不新增权限系统，复用 Sub2API 管理员身份。
6. Admin Plus 数据写独立 PostgreSQL 库；Redis 写入必须带独立前缀；Sub2API DB/Redis 只读。
7. 源站 OpenAI、Anthropic、Gemini 账号添加继续由 Sub2API 完成，Admin Plus 只运营已存在账号。
8. 无余额供应商可以监控费率和优惠，但不能作为切换候选，只能生成充值建议。

## 下一阶段完成定义

下一阶段不是继续扩表或堆页面，而是补齐真实运营闭环：

- Sub2API 供应商适配器能从真实 Admin API、只读 PostgreSQL 或只读 Redis 拉取费率、余额、并发、账单和健康指标。
- Chrome 插件至少完成一个真实 Sub2API 供应商后台的当前网站识别、sub2apiplus 授权连接、一键登录或已登录会话获取、会话包上报和失败回传；费率、余额、账单等业务采集优先由后端使用会话 API 完成，页面读取/账单下载只作为兜底。
- Scheduler 能按 10 分钟窗口生成费率、余额、优惠、健康任务，并按日生成账单任务，支持幂等、超时回收和重试可视化。
- 飞书通知覆盖余额、费率、健康、优惠和对账异常事件，具备 SQL 投递审计、事件级去重、窗口限流和通知记录页面；后续补多通道和失败重试策略。
- 动作建议经管理员确认后，通过本地 Sub2API Admin API 执行调度状态、优先级或并发调整，并记录审计。
