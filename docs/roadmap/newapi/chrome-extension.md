# Chrome 插件与一键上报

## Content Script 边界

```text
extension/src/content/
  newapi.js   # New API 专属识别和会话补齐
  sub2api.js  # 通用采集入口
```

`manifest.json` 和动态注入必须保持顺序：

```json
[
  "src/content/newapi.js",
  "src/content/sub2api.js"
]
```

原因：`sub2api.js` 负责通用采集并调用 `window.AdminPlusNewAPI.enrichSessionBundle`，所以 New API 扩展点必须先注入。

## New API 页面识别

`newapi.js` 使用两层判断：

- 已配置供应商时，`supplier.type` / `supplier_type` / `provider_type` 为 `new_api` 即进入 New API 探测。
- 未配置供应商时，必须同时看到用户态字段和 New API quota 配置痕迹，避免误判普通站点。

识别成功后，content script 同源只读调用：

```http
GET /api/user/self
Cookie: <browser cookies>
New-Api-User: <user_id>
Accept: application/json
```

调用成功才补齐 `provider_type`、`system_type`、`context.user_id` 和 `required_headers.New-Api-User`。

## 一键上报流程

1. popup 或后台识别当前站点。
2. 如果没有匹配供应商，按当前站点自动创建候选供应商，并带上 `type = new_api`。
3. 后台创建 capture-session task，payload 写入 `supplier_type` 和 `provider_type`。
4. content script 采集 localStorage、sessionStorage、页面 origin、referer 和 New API 当前用户信息。
5. background 使用 `chrome.cookies.getAll` 采集同源 Cookie，并合并到 session bundle。
6. 后台上报前再次执行 New API 兜底归一化，保证 Cookie 与 `New-Api-User` 同时存在。
7. 后端 ingest 保存 session 后触发余额读取。

## Session Evidence

普通供应商可以用 token、csrf 或 Cookie 作为登录证据。

New API 必须同时满足：

- `cookies.length > 0`
- `required_headers.New-Api-User` 或等价 user id 存在

否则插件不应把它标记为可用于 New API 余额同步的会话。
