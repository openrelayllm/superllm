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
| `roadmap/supplier-architecture/README.md` | 供应商、Chrome 插件、第三方令牌分组、本地账号、用户 API Key、检测同步、路由补池和运营可视化全局关系 | 当前事实源 |
| `roadmap/supplier-architecture/07-iteration-plan.md` | 基于当前版本的 P0-P3 迭代顺序，覆盖重构、清理、本地账号运营镜像、补池和运营验收 | 当前执行计划 |
| `roadmap/supplier-architecture/08-database-design.md` | Admin Plus 数据库设计、ER 图、表域划分、导入导出边界和核心流程表级数据流转 | 当前事实源 |
| `roadmap/routing/README.md` | 本地分组耗尽后的最低倍率补池专题详设，覆盖触发、端口、算法、写回、坏账号关闭和测试验收 | 专题详设 |
| `roadmap/accounts/README.md` | 供应商分组、第三方密钥、本地 Sub2API 账号创建与绑定流程 | 历史方案，具体关系以 supplier-architecture 为准 |
| `roadmap/newapi/README.md` | New API 供应商登录、会话、当前用户信息和后续适配能力路线图 | 当前事实源 |
| `legal/admin-compliance.zh.md` | 中文合规责任说明 | 保留 |
| `legal/admin-compliance.en.md` | 英文合规责任说明 | 保留 |
| `PAYMENT.md` | Sub2API 原支付系统英文文档 | 上游遗留参考，不是 Admin Plus MVP 范围 |
| `PAYMENT_CN.md` | Sub2API 原支付系统中文文档 | 上游遗留参考，不是 Admin Plus MVP 范围 |
| `ADMIN_PAYMENT_INTEGRATION_API.md` | Sub2API 原外部支付集成 API 文档 | 上游遗留参考，不是 Admin Plus MVP 范围 |

## 当前优先级

1. 先按 `roadmap/supplier-architecture/07-iteration-plan.md` 推进 P0：本地账号运营镜像、Sub2API 本地状态同步、drift 检测、供应商详情增强和统一动作审计。
2. 插件开发可以并行推进，但插件只负责站点识别、授权、一键登录、会话包采集和上报；业务采集结果以后端 Provider Adapter 为准。
3. 首批供应商优先支持上游也是 Sub2API 的系统。
4. 供应商是父级，供应商账号/Key 是子级，所有成本、余额、健康、对账和切换建议最终必须能落到子级。
5. Admin Plus 不新增权限系统，复用 Sub2API 管理员身份。
6. Admin Plus 私有运营事实写入 Admin Plus 表；读本地 Sub2API 可走 service/Admin API/只读 DB/Redis，写本地调度状态必须走 Admin Plus 端口并触发 Sub2API 现有校验与 `scheduler_outbox`。
7. 日常调度切换优先在 Admin Plus 完成；Sub2API 原后台作为应急备选，手工变更必须同步、审计并可采纳或恢复。
8. Admin Plus 数据库新增或调整必须先更新 `roadmap/supplier-architecture/08-database-design.md`，并说明表级读写流转、导入导出边界和敏感字段处理。
9. 源站 OpenAI、Anthropic、Gemini 账号添加继续由 Sub2API 完成，Admin Plus 只运营已存在账号。
10. 无余额供应商可以监控费率和公告，但不能作为自动切换候选；低倍率余额不足供应商必须保留为充值后候选。

## 下一阶段完成定义

下一阶段不是继续扩表或堆页面，而是补齐真实运营闭环：

- Sub2API 供应商适配器能从真实 Admin API、只读 PostgreSQL 或只读 Redis 拉取费率、余额、并发、账单和健康指标。
- Chrome 插件至少完成一个真实 Sub2API 供应商后台的当前网站识别、sub2apiplus 授权连接、一键登录或已登录会话获取、会话包上报和失败回传；费率、余额、账单等业务采集优先由后端使用会话 API 完成，页面读取/账单下载只作为兜底。
- Scheduler 默认生成会话上报任务；显式选择时直接执行分组、费率、余额、公告、健康和账单后端同步，旧插件业务结果摄取仅作为兼容补录路径，并支持幂等、超时回收和重试可视化。
- 飞书通知覆盖余额、费率、健康、公告和对账异常事件，具备 SQL 投递审计、事件级去重、窗口限流、测试诊断、通知记录页面和失败投递重试；后续补多通道通知。
- 动作建议经管理员确认后，通过 Admin Plus 本地 Sub2API 端口执行调度状态、优先级或并发调整，并记录审计；远程实例才降级调用现有 Sub2API Admin API。
