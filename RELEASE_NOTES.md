# Release Notes

## v0.23.0 - 2026-06-27

### 新增

- 新增 ProxyAI 公开纯度检测独立 Turnstile 配置，支持后台单独启停、配置 Site Key 和 Secret Key。
- 新增公开 ProxyAI runtime config 接口，用于前端按需展示 2C 纯度检测验证码。
- 新增后台设置页“纯度检测验证码”配置入口。

### 改进

- Web 公开纯度检测使用 ProxyAI 专属 Turnstile Secret 校验，避免复用登录验证码配置。
- 纯度检测对账号余额不足、quota/credit/billing 类上游错误进行独立归类并展示更明确提示。
- 后台账号纯度检测把非致命探针异常降级为部分探针异常，不再把整份报告误判为失败。

### 修复

- 补充 ProxyAI 纯度检测 Turnstile 配置校验、runtime config 和余额不足错误归类测试。

### 发布

- 更新版本号到 `0.23.0`。
- GitHub Release 继续只发布 Linux 产物：`linux_amd64`、`linux_arm64` 和 `checksums.txt`。
- DockerHub/Railway 镜像渠道不随常规发布自动执行；如需要，单独走“镜像渠道发布”流程。
