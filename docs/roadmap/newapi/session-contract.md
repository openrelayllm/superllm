# New API 会话契约

New API 后续用户态接口依赖 Cookie 加 `New-Api-User`。Admin Plus 保存会话时必须把这两个事实同时保留下来。

## 最小会话包

```json
{
  "provider_type": "new_api",
  "system_type": "new_api",
  "origin": "https://new-api.example.com",
  "api_base_url": "https://new-api.example.com",
  "auth_header_name": "New-Api-User",
  "auth_header_value": "1",
  "cookies": [
    {
      "name": "session",
      "value": "<encrypted>",
      "domain": "new-api.example.com",
      "path": "/",
      "http_only": true
    }
  ],
  "context": {
    "provider_type": "new_api",
    "system_type": "new_api",
    "api_base_url": "https://new-api.example.com",
    "user_id": "1",
    "username": "operator",
    "group": "default"
  },
  "required_headers": {
    "origin": "https://new-api.example.com",
    "referer": "https://new-api.example.com/console",
    "New-Api-User": "1"
  }
}
```

## 保存规则

- `provider_type` 和 `system_type` 统一归一化为 `new_api`。
- `New-Api-User` 从登录响应 `data.id`、`/api/user/self` 响应 `data.id` 或浏览器 localStorage 的可信用户态字段推导，禁止运营者手填。
- 浏览器一键上报必须同时保存 Cookie 和 `New-Api-User`，否则不能认为 New API session 可用于余额读取。
- `api_base_url` 优先使用供应商配置，其次使用当前页面 origin。
- Cookie、密码、access token 和第三方 Key 明文只允许出现在内存和加密存储中。
- 日志、任务结果和 UI 只能展示脱敏 summary，不回显 Cookie 或密码。

## 余额口径

New API 当前用户余额来自 `/api/user/self.data.quota`：

- `raw_quota` 表示 New API 当前用户剩余额度，不等价于本地 Sub2API 账号余额。
- MVP 使用 `BalanceCurrency = QTA`。
- `BalanceCents = raw_quota * 100`，让现有 UI 按两位小数展示原始 quota，例如 `50.00 QTA`。
- `raw_used_quota` 和 `request_count` 保留在 provider diagnostics 中，后续账单/usage 采集另行实现。
