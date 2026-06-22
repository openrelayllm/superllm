# Release Notes

## v0.9.10 - 2026-06-23

### 新增

- 供应商运营页拆分为独立表格、弹窗、调度、分组、开通和生命周期模块，降低单文件复杂度并保持现有页面入口不变。
- 供应商 view model 统一封装列表加载、筛选、状态切换、分组同步、开通任务、调度列表和删除修复流程，供拆分后的组件复用。

### 修复

- Channel check 最佳渠道识别支持独立 `pro` / `plus` 分组名归类为 OpenAI 协议，避免低倍率 OpenAI 分组被归到 `other`。
- GroupBadge 倍率展示去除多余尾零，同时保留小数精度，避免 `0.0003x` 等低倍率被显示成不易读的原始浮点值。

### 发布

- 更新版本号到 `0.9.10`。
- GitHub Release 继续只发布 Linux 产物：`linux_amd64`、`linux_arm64` 和 `checksums.txt`。
- DockerHub 镜像继续由 GitHub Actions 发布，不依赖本地 Docker。
