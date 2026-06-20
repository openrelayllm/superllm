# Release Notes

## v0.4.0 - 2026-06-20

### 修复

- 修复 Docker 自动初始化 PostgreSQL 连接时将 `host:port` 写入 lib/pq `host=` 字段的问题，避免 Railway TCP Proxy 等非默认端口部署出现 DNS 解析失败。

### 新增

- 新增 Admin Plus 运营管理入口和供应商、费率、余额、健康、优惠、插件任务、账单对账、动作建议等运营页面路由。

### 发布

- 更新版本号到 `0.4.0`。
- 更新 DockerHub 手动发布工作流默认镜像标签为 `0.4.0`。
