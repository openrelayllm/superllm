# Release Notes

## v0.29.0 - 2026-06-30

### 新增

- 新增 simple run mode 运行时节流保护，旧配置或显式低间隔配置也会被运行时最小值兜底。
- 新增渠道监控、Ops metrics、调度快照和 Token refresh 的 simple run mode 间隔保护测试。

### 改进

- simple run mode 下渠道监控调度间隔最小提升到 300 秒，避免历史 15/60 秒监控继续高频运行。
- simple run mode 下 dashboard aggregation、Ops metrics、scheduler outbox/full rebuild 和 token refresh 使用更保守的运行时间隔。

### 修复

- 修正只调整默认配置时，已有数据库设置仍可能绕过 simple run mode 低频运行策略的问题。
- 修正 simple run mode 下调度快照 outbox 轮询和全量重建可能沿用过低历史配置的问题。

### 发布

- 更新版本号到 `0.29.0`。
- GitHub Release 继续只发布 Linux 产物：`linux_amd64`、`linux_arm64` 和 `checksums.txt`。
- DockerHub/Railway 镜像渠道不随常规发布自动执行；如需要，单独走“镜像渠道发布”流程。
