# Release Notes

## v0.10.0 - 2026-06-23

### 新增

- 供应商分组新增 `official_name`、`model_family`、`model_spec`、`standard_key_name` 和 `naming_updated_at` 字段，并通过迁移 `174_admin_plus_supplier_group_naming.sql` 建立标准 Key 名称索引。
- Key 开通和补齐流程默认使用本地规范 Key 名称，新增“规范 Key 名称”操作，可批量修正本地 Key 名称，并可选择同步第三方 Key 名称。
- Sub2API / New API provider adapter 支持 Key 重命名，第三方侧使用稳定别名，避免直接暴露本地完整命名。
- 本地 Sub2API 账号同步补充分组 ID / 分组名，调度列表支持按本地分组筛选并展示本地调度分组。

### 修复

- 供应商充值倍率优先从充值订单到账额度和现金实付推导，供应商成本展示与账本实际支付金额保持一致。
- 开通任务快照保留 `sync_provider_name`，异步补齐 Key/账号时不会丢失是否同步第三方名称的选择。

### 发布

- 更新版本号到 `0.10.0`。
- GitHub Release 继续只发布 Linux 产物：`linux_amd64`、`linux_arm64` 和 `checksums.txt`。
- DockerHub 镜像继续由 GitHub Actions 发布，不依赖本地 Docker。
