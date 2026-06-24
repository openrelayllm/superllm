# Release Notes

## v0.11.3 - 2026-06-24

### 新增

- 安装向导新增 Sub2API 集成配置步骤，可在首次安装时配置只读 Sub2API PostgreSQL、只读 Redis、Sub2API Admin API 和本地开发兼容回退开关。
- 后端新增 `sub2api` 与 `admin_plus` 配置命名空间，安装向导写入的 `config.yaml` 会在运行时生效，并继续兼容原有环境变量。

### 改进

- 安装脚本改为 Admin Plus 专用命令式安装/升级入口，从 GitHub Release 拉取 Linux 二进制产物，支持 `install`、`upgrade`、`rollback` 和 `list-versions`。
- systemd 部署统一使用 `/opt/sub2api-admin-plus`、`sub2api-admin-plus.service` 和 `/etc/sub2api-admin-plus`。
- 升级脚本可识别旧版 `/opt/sub2api/sub2api` 与 `sub2api.service` 部署，迁移旧配置和安装锁，停用旧服务但保留旧目录用于人工回滚。
- 部署文档、Compose 模板和配置示例明确 Admin Plus 主库必须独立，真实 Sub2API 数据通过只读连接和 Admin API 接入。

### 发布

- 更新版本号到 `0.11.3`。
- 修复 CI lint 中的 nil slice 判断与所有已暴露 `Rows.Close()` 错误处理问题。
- GitHub Release 继续只发布 Linux 产物：`linux_amd64`、`linux_arm64` 和 `checksums.txt`。
- DockerHub 镜像继续由 GitHub Actions 发布，不依赖本地 Docker。
