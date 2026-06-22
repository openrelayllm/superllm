# Release Notes

## v0.9.5 - 2026-06-22

### 新增

- Chrome 扩展新增图标资源、Admin Plus 后端地址配置和最近上报结果展示。
- Chrome 扩展改为弹窗内直接加载捕获应用，不再注册后台 service worker，减少 MV3 后台生命周期导致的连接不稳定。
- Docker 和 GoReleaser 镜像构建会随镜像一起打包 `extension/` 目录，保证生产环境可下载最新扩展包。
- 前端默认 Logo 切换为 SVG 资源，并补充浏览器 favicon 资源。

### 修复

- Sub2API 直登会同时发送 snake_case 和 camelCase 登录协议同意字段，并保存 refresh token。
- 直登预检不再因为站点全局 TOTP 开关直接拒绝登录，避免误伤不需要二次验证的供应商账号。
- 扩展会话上报在余额探测失败时仍保留已捕获会话，并把探测错误写入 ingest 结果。
- Admin Plus 自动创建的本地 Sub2API 账号默认不可调度，避免导入账号在人工确认前进入账号池。
- 扩展会更精确识别 access token，避免把过期时间、统计 token 等非凭据字段误当成登录 token。

### 发布

- 更新版本号到 `0.9.5`。
- GitHub Release 继续只发布 Linux 产物：`linux_amd64`、`linux_arm64` 和 `checksums.txt`。
- DockerHub 镜像继续由 GitHub Actions 发布，不依赖本地 Docker。
