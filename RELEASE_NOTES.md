# Release Notes

## v0.7.3 - 2026-06-21

### 修复

- 修复 Railway 从 `0.6.x` / `0.7.x` 升级时，生产库历史 `fetch_promotions` 插件任务不满足 `admin_plus_extension_tasks_type_check`，导致迁移 `159_admin_plus_extension_session_capture.sql` 失败的问题。
- 更新迁移 checksum 兼容规则，让已记录旧版 `159_admin_plus_extension_session_capture.sql` checksum 的环境继续启动。
- 新增 `167_admin_plus_extension_task_fetch_promotions.sql`，把已经跑过旧版 `159` 的环境也收敛到补齐后的任务类型约束。
- 修复供应商分组没有已绑定 Key 时，供应商 Key 查询路径把 not-found 包装错误当作真实错误返回的问题。

### 发布

- 更新版本号到 `0.7.3`。
- GitHub Release 继续只发布 Linux 产物：`linux_amd64`、`linux_arm64` 和 `checksums.txt`。
- DockerHub 镜像继续由 GitHub Actions 发布，不依赖本地 Docker。
