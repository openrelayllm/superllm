# Release Notes

## v0.9.6 - 2026-06-22

### 新增

- 新增 New API 供应商后端适配器和 provider router，支持直登、会话探测、用户余额、分组读取和 Codex APIs speed/Pulse 渠道状态。
- Chrome 扩展新增 New API 内容脚本，采集 Cookie、`New-Api-User` 和供应商类型，并支持从当前站点自动创建 New API 供应商。
- 供应商页面补充 New API 渠道状态展示、会话摘要字段和当前余额读取反馈。
- 文档新增 New API 路线图、会话契约、后端适配、Chrome 扩展和验收说明。

### 修复

- 扩展会话上报在后端已保存会话但余额/profile 探测权限不足时显示“已保存”状态，并记录 `balance_probe_error`，避免误判为 token/cookie 上报失败。
- New API 会话包保存前兜底归一化 `provider_type`、`system_type`、`New-Api-User` 和 API base URL，提升插件采集与后端余额同步的一致性。
- 成本同步和会话摘要根据会话包识别真实 provider type，避免 New API 数据落入 Sub2API 口径。

### 发布

- 更新版本号到 `0.9.6`。
- GitHub Release 继续只发布 Linux 产物：`linux_amd64`、`linux_arm64` 和 `checksums.txt`。
- DockerHub 镜像继续由 GitHub Actions 发布，不依赖本地 Docker。
