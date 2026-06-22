# New API 支持路线图

版本：v0.1.0
日期：2026-06-22
状态：MVP 登录、浏览器会话采集、当前用户信息、余额读取、分组读取和 speed/Pulse 渠道状态已落地；费率、Key、账单和用量明细待后续实现。

## 文档分层

- [source-facts.md](source-facts.md)：从 `/Users/coso/Documents/dev/go/new-api` 核对出来的路由、鉴权和兼容差异。
- [session-contract.md](session-contract.md)：Admin Plus 保存 New API 会话包时必须满足的字段、头部和安全约束。
- [backend-adapter.md](backend-adapter.md)：后端 New API adapter、provider router、一键登录保存和余额同步链路。
- [chrome-extension.md](chrome-extension.md)：Chrome 插件内容脚本拆分、一键上报、Cookie 与 `New-Api-User` 采集。
- [testing-acceptance.md](testing-acceptance.md)：真实站点 Playwright 流程、单元测试和后续验收标准。

## 当前实现落点

```text
backend/internal/adminplus/adapters/newapi/
  provider/
    adapter.go              # Client / NewClient / DirectLogin / doSessionJSON
    session.go              # New API 会话包、Cookie、请求头和 URL 归一化
    errors.go               # 登录、业务响应和会话响应错误码映射
    profile.go              # ProbeSub2APIUserProfile / /api/user/self 响应归一化
    groups.go               # /api/user/self/groups 响应归一化
    channel_monitors.go     # https://speed.codexapis.com/api/pulse 模型级延迟/成功率归一化
    client.go               # provider 包说明和兼容占位

backend/internal/adminplus/adapters/providerrouter/
  router.go                 # 根据 provider_type / supplier.type 分发 Sub2API 或 New API adapter

extension/src/content/
  newapi.js                 # New API 页面识别、/api/user/self 探测、New-Api-User 归一化
  sub2api.js                # 通用浏览器会话采集入口
```

## 边界原则

- `sub2api.js` 只保留通用采集能力，不承载 New API 专属识别和接口探测。
- `newapi.js` 负责 New API 专属的页面态识别、当前用户探测和会话包补齐。
- 后端 ingest 只做兜底归一化，不能替代 provider adapter 的真实能力实现。
- New API 余额使用 `QTA` 单位保存原始 quota 口径，不换算人民币、美元或 token 成本。
- Codex APIs 的渠道状态来自 `https://speed.codexapis.com/api/pulse` 最近 60 秒窗口，映射到现有 ChannelMonitorView 展示；它不是 Sub2API 的 `/api/v1/channel-monitors`。
