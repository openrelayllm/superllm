# Release Notes

## v0.9.0 - 2026-06-21

### 新增

- 新增供应商渠道状态视图，支持从 Sub2API 供应商会话读取 `/channel-monitors`，展示主模型状态、延迟、7 日可用率、额外模型和时间线。
- 新增本地 Sub2API 账号模型读取与连接测试接口，供应商账号页面可对真实落地账号执行模型级连通性验证。
- 新增供应商分组开通结果的“修复落地”动作，第三方 Key 已创建但真实 Sub2API 分组或账号未完成时，可在分组步骤内重新修复。

### 更新

- Sub2API 供应商会话请求改用浏览器兼容请求头，并细分 Cloudflare/HTML、验证码、2FA 和后台模式等直接登录失败原因。
- Sub2API 真实账号落地网关默认要求显式配置远程 Admin API；未配置时返回可诊断失败，不再隐式写入当前嵌入式服务。
- 启动期安全密钥引导补齐 `totp_encryption_key` 持久化，避免 TOTP 加密密钥在生产重启后漂移。
- 补齐 `169_admin_plus_usage_cost_lines_compat.sql` 迁移 checksum 兼容规则，兼容 v0.8.0 热修后的生产迁移记录。
- 开发启动脚本增强前后端端口检查和进程清理，降低本地 dev 启动冲突。

### 发布

- 更新版本号到 `0.9.0`。
- GitHub Release 继续只发布 Linux 产物：`linux_amd64`、`linux_arm64` 和 `checksums.txt`。
- DockerHub 镜像继续由 GitHub Actions 发布，不依赖本地 Docker。
