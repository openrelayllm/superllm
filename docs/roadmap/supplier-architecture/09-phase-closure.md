# 09. P1/P2 阶段收口基线

版本：v0.1.0
日期：2026-07-09
状态：P1 主线收口，P2 第一阶段收口，P3 本轮不实施

## 1. 收口结论

当前阶段可以把 P1 和 P2 第一阶段关闭。后续继续推进时，不再把 P1/P2.x 增强项当作当前版本阻塞。

| 阶段 | 结论 | 关闭范围 |
|------|------|----------|
| P1 | 可以关闭 | 自动补池、坏账号关调度、Key 配额计划、余额门禁、统一候选评估、统一写回端口、动作执行历史、失败重试、成功回滚 |
| P2 | 可以关闭第一阶段 | 模型级候选、纯度检测联动、代理联动、通知矩阵、动作建议超时可视化、实测预算/冷却、接入验收步骤矩阵、利润缺口和建议价偏离 |
| P3 | 不实施 | 多 Sub2API 实例、跨实例容量、迁移冲突增强和外部事件适配不进入当前闭环 |

## 2. P1 关闭证据

| 验收点 | 当前证据 | 结论 |
|--------|----------|------|
| 写回统一端口 | `backend/internal/adminplus/app/sub2api/service.go` 定义 `Sub2APIRoutingPort`；补池和本地账号操作复用端口能力 | 通过 |
| 候选评估统一 | `backend/internal/adminplus/app/candidateeval/evaluator.go` 统一输出 `candidate_status/blocked_reason/check_source` | 通过 |
| Key 配额和批量开通计划 | `backend/internal/adminplus/app/supplierkeys/service.go` 使用供应商级、分组级配额和 Provider `ReadKeyCapacity` | 通过 |
| 余额门禁 | 候选评估和动作建议区分 `balance_blocked/recharge_required`，余额不足不触发渠道坏判断 | 通过 |
| 本地分组补池 | `backend/internal/adminplus/app/sub2api/routing_refill.go` 支持 dry-run、真实写回、冷却、确认窗口、失败候选抑制和 `model_scope` | 通过 |
| 坏账号关调度 | `local_account_schedule_disable` 动作复用本地账号运营 preview/apply，不自动关闭余额不足或配额不足账号 | 通过 |
| 动作执行历史 | `admin_plus_action_executions` 承载补池、关调度、手工本地账号操作、成本对账执行记录 | 通过 |
| 失败重试和成功回滚 | 动作建议执行历史支持 `routing_refill/local_account_schedule_disable` 的 retry 和 rollback | 通过 |

P1 不再阻塞的增强项：

- 真实最大 Key 上限自动读取：依赖第三方是否暴露稳定上限字段；当前 new-api/sub2api 已能读取 active Key 数，但 `LimitKnown=false` 时仍以运营配置为准。
- 账单明细自动定位和批量导入：属于财务运营增强，不影响补池、门禁和调度闭环。
- 非本地路由类动作的通用回滚：后续按 Provider Adapter 语义逐类扩展。

## 3. P2 第一阶段关闭证据

| 验收点 | 当前证据 | 结论 |
|--------|----------|------|
| 模型级候选 | `model_scope/model_match_status` 已进入候选评估和补池运行；明确不匹配输出 `model_scope_unsupported`，未知不阻断 | 通过 |
| 纯度检测联动 | 候选读模型复用最近 `run_purity_check` step，输出 `purity_failed/purity_risk/purity_stale`；前端支持复检深链和当前页受控复检队列 | 通过 |
| 代理联动 | 读取 Sub2API `accounts.proxy_id -> proxies`，明确 deleted/disabled/expired/error 代理阻断，unknown 或无代理不误阻断 | 通过 |
| 通知矩阵 | 动作建议按余额、Key 配额、路由容量、本地状态、通道失败、代理、纯度、成本和利润风险映射到通知中心 | 通过 |
| 超时未处理可视化 | `frontend/src/views/admin/operations/ActionRecommendationsView.vue` 基于 `created_at/status/severity` 派生严重 2 小时、警告 12 小时、普通 24 小时超时统计 | 通过 |
| 实测预算/冷却 | `channelchecks.Check` 支持每日 token 预算、单次估算 token 和同分组冷却；预算或冷却跳过不会写失败快照 | 通过 |
| 接入验收步骤矩阵 | Kanban 验收报告按连通性、模型列表、纯度、轻量调用、Usage、缓存、余额和并发汇总阻断点 | 通过 |
| 成本利润第一阶段 | 模型利润行展示目标毛利缺口和建议价相对市场中位价偏离 | 通过 |

P2 不再阻塞的增强项：

- 通知自动升级投递、值班分组、多渠道通知。
- Admin Plus 代理中心节点健康、出口失败、fallback 建议和本地账号绑定代理深度联动。
- 跨页或后台批量纯度复检、按模型预算归集。
- 完整财务汇总：成本、收入、毛利、异常对账和账单批量导入。
- 主动实测按模型、动作来源和原因做细粒度成本归集。

## 4. 当前版本发布前验收清单

发布当前阶段前，只需要验证核心闭环，不需要补齐 P2.x。

| 场景 | 验收方式 | 通过标准 |
|------|----------|----------|
| 本地分组空池 | 在动作建议页或调度中心预览补池 | 能看到最低倍率候选、用户影响和 dry-run 结果 |
| 真实补池 | 审批后执行 `routing_refill` | 写回走 `Sub2APIRoutingPort`，执行历史有前后快照 |
| 通道失败账号 | 生成 `local_account_schedule_disable` 建议 | 余额不足、配额不足不生成坏账号关调度建议 |
| 原后台手工修改 | 执行本地状态同步 | 能看到 drift，写前保护阻断覆盖 |
| Key 配额不足 | 生成开通计划 | 不静默创建部分 Key，必须运营显式确认部分开通 |
| 余额不足低倍率账号 | 打开低倍率余额机会队列 | 保留机会并提示充值，不标记供应商坏 |
| 纯度过期账号 | 从动作建议或本地账号运营触发复检 | 由运营显式触发，不自动消耗 token |
| 动作失败 | 对失败执行点击重试 | 不盲重放旧 payload，重新进入安全 apply 链路 |
| 动作成功 | 对成功补池或关调度执行回滚 | 回滚写入新的 execution，并保留来源 execution ID |

### 4.1 可执行验收矩阵

人工验收建议在测试环境或可回滚的运营窗口执行。每一项都应保存页面截图、请求响应摘要或执行记录 ID；不要用生产敏感 Key 明文作为验收材料。

| 编号 | 场景 | 入口/API | 操作步骤 | 必须留存的通过证据 |
|------|------|----------|----------|--------------------|
| A1 | 本地分组空池发现 | `/admin/scheduler`、`GET /api/v1/admin-plus/sub2api/groups` | 准备一个有启用用户 API Key 且可调度账号为 0 的本地分组；刷新调度中心容量矩阵 | 容量矩阵显示空池或低容量；对应工作台动作可跳到 `/admin/actions?type=routing_refill&local_group_id=...` |
| A2 | 补池 dry-run | `/admin/local-account-ops` 或 `/admin/scheduler`、`POST /api/v1/admin-plus/sub2api/routing/refill-local-group` with `dry_run=true` | 选择目标本地分组，点击预览补池 | 返回最低倍率候选、`availability_before`、用户影响摘要；没有写入目标分组 |
| A3 | 审批后真实补池 | `/admin/actions`、`POST /api/v1/admin-plus/actions/recommendations/:id/execute` | 将 `routing_refill` 建议审批为 approved 后执行 | `admin_plus_action_executions` 出现 succeeded/skipped/failed 记录；成功时 `after_snapshot` 或补池结果显示目标分组可调度账号增加 |
| A4 | 补池影响追溯 | `/admin/scheduler/routing-refill-history`、`GET /api/v1/admin-plus/sub2api/routing/group-impact/api-keys` | 打开补池影响历史，查看对应运行 | 能看到运行状态、候选、前后容量、脱敏受影响 Key 和最近失败请求摘要；敏感明细查询要求原因 |
| A5 | 坏账号关调度建议 | `/admin/actions`、`GET /api/v1/admin-plus/actions/recommendations` | 准备通道监控失败或主动实测失败且仍在调度的本地账号，生成动作建议 | 生成 `local_account_schedule_disable`；余额不足、Key 配额不足、drift 不生成坏账号关调度建议 |
| A6 | 关调度执行与回滚 | `/admin/actions`、`POST /api/v1/admin-plus/actions/recommendations/:id/execute`、`.../executions/:executionID/rollback` | 审批并执行关调度建议，再对 succeeded execution 执行回滚 | 执行记录包含前后快照；回滚新建 execution，并在 payload 中保留 `rollback_source_execution_id` |
| A7 | 原后台 drift 写前保护 | `/admin/local-account-ops`、`POST /api/v1/admin-plus/sub2api/local-account-ops/sync-local-state` | 在 Sub2API 原后台修改账号分组或调度，再回 Admin Plus 同步本地状态并尝试写回 | 页面显示 drift；写回返回或提示 `LOCAL_ACCOUNT_STATE_DRIFT_PENDING`；可采纳或恢复基线 |
| A8 | Key 配额开通计划 | 供应商分组弹窗、`POST /api/v1/admin-plus/suppliers/:id/keys/ensure-all-plan` | 配置供应商或分组级有限/未知/不支持自动开通策略，生成开通计划 | 计划明确展示可创建、已覆盖、被阻塞分组和阻塞原因；没有运营显式 `allow_partial` 时不静默创建部分 Key |
| A9 | 余额不足低倍率保护 | `/admin/actions` 和本地账号运营镜像 | 准备低倍率但余额不足的供应商或账号，刷新余额和候选状态 | 候选显示 `balance_blocked/recharge_required`；进入低倍率余额机会或充值/复检建议；不生成渠道坏或关调度建议 |
| A10 | 纯度过期受控复检 | `/admin/local-account-ops`、`/admin/actions` | 选择纯度快照超过 7 天或缺失的账号，按当前页模型/能力标签圈选并启动复检队列 | 只打开复检弹窗或队列，不自动批量消耗 token；复检成功后调度 step 快照可被候选评估读取 |
| A11 | 调度来源追溯 | `/admin/scheduler` 运行详情弹窗、`GET /api/v1/admin-plus/scheduler/runs/:id`、`/admin/actions` | 从调度 run/step 详情进入补池或关调度建议并执行 | action execution 记录 `scheduler_run_id/scheduler_step_id`；执行历史可反跳回调度运行详情 |
| A12 | 幂等 replay | 本地账号运营 apply 或补池 apply API | 使用相同 `Idempotency-Key` 重放同一写动作 | 不新增重复 execution，不重复写回；原执行记录或最新同指纹记录标记 `idempotency_replayed=true` |

### 4.2 不纳入当前发布阻塞

以下项目可以继续排入 P1.x/P2.x/P3，但不应阻塞当前 P1/P2 收口验收：

| 队列 | 不阻塞项 | 原因 |
|------|----------|------|
| P1.x | 真实最大 Key 上限自动读取 | new-api/sub2api 当前可读取 active Key 数；最大上限依赖具体供应商是否暴露稳定字段 |
| P1.x | 账单明细自动定位、重复账单冲突预检和批量导入 | 当前已有人工对账调整和明细修复第一阶段；自动定位属于财务运营增强 |
| P2.x | 通知自动升级、值班分组和多渠道投递 | 当前通知矩阵第一阶段已复用通知中心规则、投递、去重和静默窗口 |
| P2.x | 代理中心深度质量联动和 fallback 建议 | 当前候选评估已读取本地账号绑定代理状态，明确代理不可用会阻断 |
| P2.x | 容量矩阵行内今日请求、限流账号、错误账号和最低可补倍率 | 这些指标已在补池影响面板和动作建议信号中用于执行确认，矩阵行内展示是扫盘效率增强 |
| P2.x | 跨页/后台纯度复检和按模型预算归集 | 当前支持当前页模型/能力圈选和受控复检队列，避免默认自动消耗 token |
| P3 | 多 Sub2API 实例、外部事件、迁移冲突增强 | 当前决策明确不实施 P3；远程写回第一阶段只保留为单实例远程端口能力 |

### 4.3 验收记录模板

每次发布前验收应创建一份记录。可以复制下面模板到发布 issue、飞书文档或本地验收记录；不要把生产密钥、完整用户 API Key、完整第三方 Key、请求体或 headers 放入记录。

验收批次：

| 字段 | 填写 |
|------|------|
| 验收日期 |  |
| 验收环境 | 测试 / 预发 / 生产灰度 |
| 代码版本 | commit SHA、tag 或镜像版本 |
| 数据库迁移版本 |  |
| 执行人 |  |
| 回滚负责人 |  |
| 关联发布单 |  |
| 备份状态 | 已备份 / 不涉及 / 未完成 |
| 验收开始时间 |  |
| 验收结束时间 |  |

单项记录：

| 编号 | 结果 | 证据 | 问题链接 | 处理结论 |
|------|------|------|----------|----------|
| A1 | pass / fail / skipped | 截图、execution id、run id 或响应摘要 |  |  |
| A2 | pass / fail / skipped | 截图、execution id、run id 或响应摘要 |  |  |
| A3 | pass / fail / skipped | 截图、execution id、run id 或响应摘要 |  |  |
| A4 | pass / fail / skipped | 截图、execution id、run id 或响应摘要 |  |  |
| A5 | pass / fail / skipped | 截图、execution id、run id 或响应摘要 |  |  |
| A6 | pass / fail / skipped | 截图、execution id、run id 或响应摘要 |  |  |
| A7 | pass / fail / skipped | 截图、execution id、run id 或响应摘要 |  |  |
| A8 | pass / fail / skipped | 截图、execution id、run id 或响应摘要 |  |  |
| A9 | pass / fail / skipped | 截图、execution id、run id 或响应摘要 |  |  |
| A10 | pass / fail / skipped | 截图、execution id、run id 或响应摘要 |  |  |
| A11 | pass / fail / skipped | 截图、execution id、run id 或响应摘要 |  |  |
| A12 | pass / fail / skipped | 截图、execution id、run id 或响应摘要 |  |  |

### 4.4 放行与阻断规则

| 结论 | 条件 | 处理 |
|------|------|------|
| 放行 | A1-A12 全部 pass，且自动校验记录中的命令在最终代码基线通过 | 可以进入发布或灰度 |
| 条件放行 | 仅 P1.x/P2.x/P3 非阻塞项失败，且已在验收记录写明跳过原因、影响范围和后续负责人 | 可以灰度；不得把 skipped 项宣传为已完成 |
| 阻断 | A1-A9 任一 fail，或出现余额不足被误判为渠道坏、Key 配额静默部分创建、drift 被覆盖、无审计写回、重复写回等核心闭环问题 | 停止发布，修复后重跑相关验收 |
| 阻断 | A10-A12 任一 fail 且影响纯度受控复检、调度来源追溯或幂等 replay 的可信性 | 停止发布，除非发布范围明确不包含相关功能且有回滚方案 |
| 阻断 | 验收证据包含生产敏感明文、完整 Key、请求体或 headers | 清理证据并重新留存脱敏材料 |

最小放行口径：

- A1-A9 是 P1 主线和运营闭环硬门禁。
- A10-A12 是 P2 第一阶段可信性门禁；如果跳过，必须说明当前发布不触发对应能力，并在灰度计划中保留补验时间。
- 自动校验命令必须基于最终待发布代码重新执行；不能复用改动前的旧结果。
- 发布后若运营仍需要回 Sub2API 原后台完成主路径操作，应视为当前版本未达到“Admin Plus 日常主操作入口”的目标，需要回到 P1/P2 问题池处理。

## 5. 后续 Backlog 边界

后续继续迭代时按以下边界拆分，避免把 P3 或大而全能力拉回当前闭环。

| 队列 | 内容 | 是否阻塞当前版本 |
|------|------|------------------|
| P1.x | 真实最大 Key 上限自动读取、账单明细自动定位、批量账单导入、非本地路由动作回滚 | 否 |
| P2.x | 通知升级、代理深度质量、跨页纯度复检、完整财务汇总、细粒度实测成本 | 否 |
| P3 | 多 Sub2API 实例、跨实例容量、迁移冲突增强、外部事件 | 否，本轮不实施 |

## 6. 不再扩大当前阶段的约束

1. 不修改 `/Users/coso/Documents/dev/go/sub2api` upstream。
2. 不做请求热路径 hook patch。
3. 不把余额不足当作渠道坏。
4. 不把主动实测作为默认第一检查。
5. 不把 Chrome 插件作为最终业务事实源。
6. 不做没有 dry-run、审计和写前保护的批量写操作。

## 7. 自动校验记录

本记录只证明当前代码基线通过自动化校验，不替代第 4 节的真实运营场景验收。

| 日期 | 命令 | 结果 | 覆盖范围 |
|------|------|------|----------|
| 2026-07-09 | `go test -count=1 ./internal/adminplus/...` | 通过 | Admin Plus 后端应用、适配器、候选评估、补池、动作、通知、供应商 Key、导入导出、纯度、调度等包 |
| 2026-07-09 | `go test -count=1 ./internal/handler/adminplus ./internal/server/routes ./cmd/server` | 通过 | Admin Plus HTTP handler、服务端路由注册和 server 入口 |
| 2026-07-09 | `pnpm typecheck` | 通过 | 前端 Vue/TypeScript 类型检查，覆盖动作建议、本地账号运营、供应商和调度相关类型引用 |
| 2026-07-09 | `pnpm test:run` | 通过 | 前端 Vitest 回归，32 个测试文件、193 个测试通过 |
| 2026-07-09 | `git diff --check -- docs/roadmap/supplier-architecture/... docs/roadmap/routing/README.md RELEASE_NOTES.md` | 通过 | 本次收口文档、routing 专题文档和 release notes 更新无行尾空格等 diff 格式问题 |

## 8. 文档一致性记录

| 日期 | 文档 | 调整 |
|------|------|------|
| 2026-07-09 | `docs/roadmap/routing/README.md` | 从“自动执行器待实施”的旧口径更新为 P1 已落地口径；明确调度 worker、动作建议执行历史、失败重试和成功回滚已完成，外部事件、多实例和静默自动关调度不进入本轮 P1/P2 收口 |
| 2026-07-09 | `docs/roadmap/supplier-architecture/05-operations-visualization.md` | 明确 24 小时请求/错误/429/token 已在补池影响面板落地；容量矩阵行内今日请求、限流、错误和最低可补倍率归入 P2.x 可视化增强，不阻断当前收口 |
| 2026-07-09 | `docs/roadmap/kanban/README.md` | 从“真实路由执行仍在后续阶段”的旧口径更新为本地路由类 `routing_refill/local_account_schedule_disable` 已进入统一 action execution；线上切流和权重调整仍保持后续阶段 |
| 2026-07-09 | `docs/roadmap/restructure/README.md` | 标记为历史重构计划，当前 P1/P2 收口状态改以 supplier architecture 和本文件为准 |
| 2026-07-09 | `docs/roadmap/accounts/ASYNC_PROVISIONING.md` | 标记为账号开通异步治理专项，真实 E2E 待收口保留为专项风险，不再误读为 P1/P2 全局收口阻塞 |
| 2026-07-09 | `docs/roadmap/scheduler/README.md` | 标记调度底座 Redis Stream 唤醒和计划编辑向导为 scheduler 专项增强；明确 `local.sub2api.routing.capacity_watch` 已接入 worker，默认生成动作建议，开启自动补池后才真实写回空池分组 |
