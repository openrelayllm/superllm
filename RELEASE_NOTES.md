# Release Notes

## v0.27.0 - 2026-06-30

### 新增

- 新增 Responses structured output 探针，单独验证 `json_schema` 输出能力并纳入结构完整性校验。
- 新增 Chat Completions `n=2` 回退探针，补齐兼容站多候选返回能力的检测。
- 新增渠道检测候选上限到 20，批量供应商分组检测可覆盖更多候选。

### 改进

- OpenAI 检测链路增加 structured output、Chat Completions 多候选和相关测试桩，结构完整性判断更细。
- 渠道检测和前端批量提交统一使用更高候选上限，减少只看少量样本带来的偏差。
- 兼容性分数随新的结构化输出和多候选探针重新校准。

### 修复

- 修正 OpenAI structured output 探针缺失时无法单独定位结构问题的情况。
- 修正 Chat Completions 回退只看单候选而漏掉多候选能力的情况。
- 修正渠道检测候选上限过低导致批量任务覆盖不足的问题。

### 发布

- 更新版本号到 `0.27.0`。
- GitHub Release 继续只发布 Linux 产物：`linux_amd64`、`linux_arm64` 和 `checksums.txt`。
- DockerHub/Railway 镜像渠道不随常规发布自动执行；如需要，单独走“镜像渠道发布”流程。
