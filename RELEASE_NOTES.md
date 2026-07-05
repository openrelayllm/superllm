# Release Notes

## v0.38.0 - 2026-07-06

### 新增

- 看板缓存效率和供应质量新增真实 usage 派生证据，可从近 7 天 `usage_logs` 与 `ops_error_logs` 自动生成缓存命中、错误率、模型一致性、余额风险和并发证据。
- 接入验收报告新增 usage 派生证据读取，缺少人工快照时也能基于真实调用日志判断连通性、缓存审计和质量状态。
- 看板概览会合并人工快照与 usage 派生快照，模型利润和风险判断可直接反映真实流量质量。

### 改进

- 看板列表页补充 usage 派生证据展示，区分手工录入快照与自动派生快照。
- 供应质量和缓存效率筛选支持空 supply/source 类型查询，不再把空筛选默认收窄为 supplier/manual。
- `deploy/sub2api.service` 补充 `/etc/sub2api-admin-plus` 写权限和 `DATA_DIR`，避免 strict systemd 沙箱下配置目录不可写。
- 看板 PRD 更新 usage 派生证据和接入验收自动证据的当前状态。

### 修复

- 修正生成接入验收报告时绕过 service 层、无法读取 usage 派生缓存/质量证据的问题。
- 修正看板概览只统计人工快照、导致真实流量风险未进入模型利润视图的问题。

### 测试

- 增加 usage 派生缓存风险、usage 派生验收证据、筛选合并和看板概览风险判断回归测试。

### 发布

- 更新版本号到 `0.38.0`。
- GitHub Release 继续只发布 Linux 产物：`linux_amd64`、`linux_arm64` 和 `checksums.txt`。
- DockerHub/Railway 镜像渠道不随常规发布自动执行；如需要，单独走镜像渠道发布流程。
