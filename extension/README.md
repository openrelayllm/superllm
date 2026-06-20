# Sub2API Plus Session Capture

这是 Admin Plus 的 Chrome MV3 会话获取器。插件只负责浏览器侧必须完成的能力：连接已登录的 sub2apiplus、识别当前供应商网站、一键登录或读取已登录会话，并把供应商会话包上报给后端采集器。

插件不提供 Sub2API 管理员登录 UI，不展示或要求输入管理员 Token。

使用方式：

1. 打开 Chrome `chrome://extensions`。
2. 启用开发者模式。
3. 加载 `extension/` 目录或后台下载的解压目录。
4. 在已登录的 sub2apiplus Web 页面点击插件里的“连接当前页面”。
5. 打开已配置供应商后台，点击“一键登录并上报”或“一键获取并上报”。

当前主路径是 `capture_supplier_session`。费率、余额、优惠、健康和账单采集应优先由后端使用供应商会话 API 完成；页面 DOM 解析、CSV 下载和旧任务领取协议只作为浏览器兜底能力保留。
