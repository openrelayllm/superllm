# Release Notes

## v0.9.1 - 2026-06-22

### 新增

- 新增供应商第三方兑换入口和本地充值入口字段，数据库迁移 `171_admin_plus_supplier_recharge_urls.sql` 会为既有供应商补齐空值列。
- 供应商管理表格行操作菜单新增“第三方兑换”入口；未配置时保持禁用提示，避免误跳转。
- Chrome 扩展捕获供应商会话时会识别 `/custom/`、`/recharge`、`/payment`、`/topup`、`/redeem`、`/card`、`/pay` 等充值页路径，并把第三方兑换 URL 回传 Admin Plus。

### 更新

- 扩展自动匹配到既有供应商时，只补齐缺失的充值入口，不覆盖人工维护的 URL。
- 手动创建、编辑供应商和从站点候选创建供应商时，统一校验并持久化充值入口 URL。
- 本地 Sub2API 分组与账号落地失败时，错误信息会带上可诊断原因，同时自动脱敏 `api_key`、token、cookie、password、secret 等敏感字段。

### 发布

- 更新版本号到 `0.9.1`。
- GitHub Release 继续只发布 Linux 产物：`linux_amd64`、`linux_arm64` 和 `checksums.txt`。
- DockerHub 镜像继续由 GitHub Actions 发布，不依赖本地 Docker。
