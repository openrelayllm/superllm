# Release Notes

## v0.18.0 - 2026-06-27

### 新增

- 新增公开 ProxyAI 站点目录 API，提供 `/api/v1/public/proxyai/summary`、`/sites` 和 `/sites/:slug`。
- 公开站点列表支持分页、搜索、推荐级别、风险级别、站点类型、供应商类型和排序参数。

### 改进

- 站点目录查询会同步加载来源观测数据，用于公开 API 输出可用状态、最低倍率、响应时间和首 token 延迟等指标。
- 公开 ProxyAI API 只投影可公开字段，过滤私有、草稿和重复站点，并隐藏后台控制台链接与内部供应商 ID。
- 公开 ProxyAI API 增加跨域响应和 OPTIONS 预检处理，便于独立公开页面直接调用。

### 修复

- 补充公开 ProxyAI 路由挂载测试和 handler 投影测试，防止公开页面接口被认证路由或后台字段回归影响。

### 发布

- 更新版本号到 `0.18.0`。
- GitHub Release 继续只发布 Linux 产物：`linux_amd64`、`linux_arm64` 和 `checksums.txt`。
- DockerHub/Railway 镜像渠道不随常规发布自动执行；如需要，单独走“镜像渠道发布”流程。
