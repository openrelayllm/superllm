package purity

import (
	"github.com/tidwall/gjson"
	"net/http"
	"strings"
)

func buildBaseURLCheck(host string, officialHost bool) CheckResult {
	if officialHost {
		return CheckResult{
			ID:       "base_url",
			Name:     "API Base 域名",
			Status:   CheckStatusPass,
			Score:    20,
			MaxScore: 20,
			Message:  "命中 OpenAI 官方 API 域名。",
			Details:  map[string]any{"host": host, "official_host": true},
		}
	}
	return CheckResult{
		ID:       "base_url",
		Name:     "API Base 域名",
		Status:   CheckStatusPass,
		Score:    20,
		MaxScore: 20,
		Message:  "当前为自定义 OpenAI API Base；仅作为链路信息，不单独影响纯度评分。",
		Details:  map[string]any{"host": host, "official_host": false},
	}
}

func buildModelsCheck(probe httpProbe, model string) CheckResult {
	details := probeDetails(probe)
	if probe.StatusCode == 0 {
		return failCheck("models_schema", "模型列表结构", 15, "无法连接模型列表端点。", details)
	}
	if probe.ErrorClass == errorClassAccountBalanceInsufficient {
		return failCheck("models_schema", "模型列表结构", 15, "账号余额不足，模型列表探测无法执行。", details)
	}
	if probe.StatusCode == http.StatusUnauthorized || probe.StatusCode == http.StatusForbidden {
		return failCheck("models_schema", "模型列表结构", 15, "API Key 鉴权失败。", details)
	}
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		return CheckResult{ID: "models_schema", Name: "模型列表结构", Status: CheckStatusWarn, Score: 4, MaxScore: 15, Message: "模型列表端点未返回标准 2xx 响应。", Details: details}
	}
	object := gjson.GetBytes(probe.Body, "object").String()
	data := gjson.GetBytes(probe.Body, "data")
	if object != "list" || !data.IsArray() {
		return CheckResult{ID: "models_schema", Name: "模型列表结构", Status: CheckStatusWarn, Score: 6, MaxScore: 15, Message: "模型列表响应不是标准 OpenAI list 结构。", Details: details}
	}
	modelListed := false
	count := 0
	for _, item := range data.Array() {
		count++
		if strings.TrimSpace(item.Get("id").String()) == model {
			modelListed = true
		}
	}
	details["model_count"] = count
	details["model_listed"] = modelListed
	if modelListed {
		return passCheck("models_schema", "模型列表结构", 15, "模型列表结构标准，且包含本次检测模型。", details)
	}
	return CheckResult{ID: "models_schema", Name: "模型列表结构", Status: CheckStatusWarn, Score: 12, MaxScore: 15, Message: "模型列表结构标准，但未列出本次检测模型。", Details: details}
}

func buildResponsesSchemaCheck(probe httpProbe, apiKey string) CheckResult {
	details := probeDetails(probe)
	if probe.StatusCode == 0 {
		return failCheck("responses_schema", "Responses 非流式结构", 20, "无法连接 Responses 端点。", details)
	}
	if probe.ErrorClass == errorClassAccountBalanceInsufficient {
		return failCheck("responses_schema", "Responses 非流式结构", 20, "账号余额不足，Responses 探测无法执行。", details)
	}
	if probe.StatusCode == http.StatusUnauthorized || probe.StatusCode == http.StatusForbidden {
		return failCheck("responses_schema", "Responses 非流式结构", 20, "Responses 端点鉴权失败。", details)
	}
	if probe.StatusCode == http.StatusNotFound || probe.StatusCode == http.StatusMethodNotAllowed {
		return failCheck("responses_schema", "Responses 非流式结构", 20, "Responses 端点不存在或方法不支持。", details)
	}
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		details["error_message"] = sanitizeMessage(upstreamErrorMessage(probe.Body), apiKey)
		return failCheck("responses_schema", "Responses 非流式结构", 20, "Responses 端点未返回可用响应。", details)
	}
	if gjson.GetBytes(probe.Body, "object").String() == "response" && gjson.GetBytes(probe.Body, "output").IsArray() {
		return passCheck("responses_schema", "Responses 非流式结构", 20, "Responses 响应结构符合 OpenAI 预期。", details)
	}
	return CheckResult{ID: "responses_schema", Name: "Responses 非流式结构", Status: CheckStatusWarn, Score: 8, MaxScore: 20, Message: "Responses 返回 2xx，但响应结构不完整。", Details: details}
}

func buildResponsesStructuredOutputCheck(probe httpProbe, apiKey string) CheckResult {
	details := probeDetails(probe)
	details["response_format_type"] = "json_schema"
	details["response_format_name"] = "person"
	details["response_format_strict"] = true
	if probe.ErrorMessage != "" {
		details["error_message"] = sanitizeMessage(probe.ErrorMessage, apiKey)
	}
	if probe.StatusCode == 0 {
		return failCheck("responses_structured_output", "结构化输出", 10, "无法连接 Responses structured output 探测端点。", details)
	}
	if probe.ErrorClass == errorClassAccountBalanceInsufficient {
		return failCheck("responses_structured_output", "结构化输出", 10, "账号余额不足，structured output 探测无法执行。", details)
	}
	if probe.StatusCode == http.StatusUnauthorized || probe.StatusCode == http.StatusForbidden {
		return failCheck("responses_structured_output", "结构化输出", 10, "Responses structured output 端点鉴权失败。", details)
	}
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		return CheckResult{ID: "responses_structured_output", Name: "结构化输出", Status: CheckStatusWarn, Score: 4, MaxScore: 10, Message: "Responses structured output 端点未返回可用响应。", Details: details}
	}
	if gjson.GetBytes(probe.Body, "object").String() != "response" {
		return CheckResult{ID: "responses_structured_output", Name: "结构化输出", Status: CheckStatusWarn, Score: 5, MaxScore: 10, Message: "structured output 探测返回 2xx，但响应不是标准 Responses 结构。", Details: details}
	}
	outputText := strings.TrimSpace(gjson.GetBytes(probe.Body, "output.0.content.0.text").String())
	if outputText == "" {
		outputText = strings.TrimSpace(gjson.GetBytes(probe.Body, "output_text").String())
	}
	details["output_text"] = outputText
	if outputText == "" {
		return CheckResult{ID: "responses_structured_output", Name: "结构化输出", Status: CheckStatusWarn, Score: 6, MaxScore: 10, Message: "structured output 响应缺少文本内容。", Details: details}
	}
	if gjson.Valid(outputText) && gjson.Get(outputText, "name").String() != "" && gjson.Get(outputText, "age").Exists() {
		return passCheck("responses_structured_output", "结构化输出", 10, "Responses 接受 json_schema structured output，并返回符合预期的结构化内容。", details)
	}
	if strings.Contains(strings.ToLower(outputText), "jane") || strings.Contains(strings.ToLower(outputText), "54") {
		return CheckResult{ID: "responses_structured_output", Name: "结构化输出", Status: CheckStatusWarn, Score: 6, MaxScore: 10, Message: "structured output 端点可用，但返回内容未严格落在 JSON schema 上。", Details: details}
	}
	return failCheck("responses_structured_output", "结构化输出", 10, "structured output 未返回符合 json_schema 的内容。", details)
}

func buildToolCallCheck(probe httpProbe) CheckResult {
	details := probeDetails(probe)
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		return failCheck("tool_call", "强制工具调用", 20, "Responses 探测未成功，无法确认工具调用。", details)
	}
	ok, toolDetails := responsesBodyHasExpectedFunctionCall(probe.Body)
	for key, value := range toolDetails {
		details[key] = value
	}
	if ok {
		return passCheck("tool_call", "强制工具调用", 20, "tool_choice=required 成功产出 probe_ping(ok=true) function_call。", details)
	}
	return failCheck("tool_call", "强制工具调用", 20, "强制工具调用没有产出预期 function_call。", details)
}

func buildUsageCheck(usage *TokenUsage, probe httpProbe) CheckResult {
	details := probeDetails(probe)
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		return failCheck("usage", "Usage 计量", 10, "Responses 探测未成功，无法读取 usage。", details)
	}
	if usage == nil {
		return failCheck("usage", "Usage 计量", 10, "响应缺少 usage 计量字段。", details)
	}
	details["input_tokens"] = usage.InputTokens
	details["output_tokens"] = usage.OutputTokens
	details["total_tokens"] = usage.TotalTokens
	if usage.TotalTokens >= usage.InputTokens+usage.OutputTokens && usage.TotalTokens > 0 {
		return passCheck("usage", "Usage 计量", 10, "usage token 计量字段完整。", details)
	}
	return CheckResult{ID: "usage", Name: "Usage 计量", Status: CheckStatusWarn, Score: 5, MaxScore: 10, Message: "usage 字段存在，但 token 汇总不完全一致。", Details: details}
}

func buildResponsesStoreIncludeCheck(probe httpProbe, apiKey string) CheckResult {
	details := probeDetails(probe)
	details["store"] = false
	details["include"] = []string{"reasoning.encrypted_content"}
	if probe.ErrorMessage != "" {
		details["error_message"] = sanitizeMessage(probe.ErrorMessage, apiKey)
	}
	if probe.StatusCode == 0 {
		return CheckResult{ID: "responses_store_include", Name: "Responses store/include 探针", Status: CheckStatusFail, Score: 0, MaxScore: 0, Message: "无法连接 Responses store/include 探测端点。", Details: details}
	}
	if probe.ErrorClass == errorClassAccountBalanceInsufficient {
		return CheckResult{ID: "responses_store_include", Name: "Responses store/include 探针", Status: CheckStatusWarn, Score: 0, MaxScore: 0, Message: "账号余额不足，store/include 探测未完成，不能据此判断非官方。", Details: details}
	}
	if probe.StatusCode >= 200 && probe.StatusCode < 300 && gjson.GetBytes(probe.Body, "object").String() == "response" {
		return CheckResult{ID: "responses_store_include", Name: "Responses store/include 探针", Status: CheckStatusPass, Score: 0, MaxScore: 0, Message: "Responses 接受 store=false 与 include=reasoning.encrypted_content，符合官方客户端请求形态。", Details: details}
	}
	if probe.StatusCode == http.StatusBadRequest || probe.StatusCode == http.StatusUnprocessableEntity || probe.StatusCode == http.StatusNotFound || probe.StatusCode == http.StatusMethodNotAllowed {
		return CheckResult{ID: "responses_store_include", Name: "Responses store/include 探针", Status: CheckStatusFail, Score: 0, MaxScore: 0, Message: "Responses 不接受 store=false 或 reasoning.encrypted_content include，疑似兼容层未完整实现官方 Responses 行为。", Details: details}
	}
	if probe.StatusCode >= 400 && probe.StatusCode < 500 {
		return CheckResult{ID: "responses_store_include", Name: "Responses store/include 探针", Status: CheckStatusWarn, Score: 0, MaxScore: 0, Message: "store/include 探测被 4xx 拒绝，但错误形态不能确认是否为协议不支持。", Details: details}
	}
	return CheckResult{ID: "responses_store_include", Name: "Responses store/include 探针", Status: CheckStatusFail, Score: 0, MaxScore: 0, Message: "store/include 探测未返回标准 Responses 响应。", Details: details}
}

func buildChatCompletionsChoiceCountCheck(probe httpProbe, apiKey string, expectedChoices int) CheckResult {
	details := probeDetails(probe)
	details["requested_n"] = expectedChoices
	if probe.ErrorMessage != "" {
		details["error_message"] = sanitizeMessage(probe.ErrorMessage, apiKey)
	}
	if probe.StatusCode == 0 {
		return failCheck("chat_completions_n", "Chat Completions 多候选", 0, "无法连接 Chat Completions n=2 探测端点。", details)
	}
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		return CheckResult{ID: "chat_completions_n", Name: "Chat Completions 多候选", Status: CheckStatusWarn, Score: 0, MaxScore: 0, Message: "Chat Completions n=2 探测未返回可用响应。", Details: details}
	}
	choices := gjson.GetBytes(probe.Body, "choices").Array()
	details["choice_count"] = len(choices)
	if len(choices) >= expectedChoices {
		return passCheck("chat_completions_n", "Chat Completions 多候选", 0, "Chat Completions 的 n=2 请求返回了多候选。", details)
	}
	return CheckResult{ID: "chat_completions_n", Name: "Chat Completions 多候选", Status: CheckStatusWarn, Score: 0, MaxScore: 0, Message: "Chat Completions 的 n=2 请求仅返回单候选。", Details: details}
}

func buildStreamingCheck(probe streamProbe, apiKey string) CheckResult {
	details := map[string]any{
		"status_code":      probe.StatusCode,
		"first_token_ms":   probe.FirstTokenMS,
		"total_latency_ms": probe.TotalLatencyMS,
		"seen_data":        probe.SeenData,
		"seen_delta":       probe.SeenDelta,
		"seen_completed":   probe.SeenCompleted,
		"seen_done":        probe.SeenDone,
	}
	if probe.ErrorClass != "" {
		details["error_class"] = probe.ErrorClass
		details["error_message"] = sanitizeMessage(probe.ErrorMessage, apiKey)
	}
	if probe.StatusCode == 0 {
		return failCheck("streaming", "Responses 流式事件", 15, "无法连接 Responses 流式端点。", details)
	}
	if probe.ErrorClass == errorClassAccountBalanceInsufficient {
		return failCheck("streaming", "Responses 流式事件", 15, "账号余额不足，Responses 流式探测无法执行。", details)
	}
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		return failCheck("streaming", "Responses 流式事件", 15, "Responses 流式端点未返回可用响应。", details)
	}
	if probe.SeenDelta && (probe.SeenCompleted || probe.SeenDone) && probe.ErrorClass == "" {
		return passCheck("streaming", "Responses 流式事件", 15, "SSE delta 与完成事件完整。", details)
	}
	if probe.SeenData && (probe.SeenCompleted || probe.SeenDone) {
		return CheckResult{ID: "streaming", Name: "Responses 流式事件", Status: CheckStatusWarn, Score: 8, MaxScore: 15, Message: "SSE 生命周期结束，但未观察到文本 delta。", Details: details}
	}
	return failCheck("streaming", "Responses 流式事件", 15, "SSE 生命周期不完整。", details)
}

func buildMultimodalCheck(probe httpProbe, apiKey string) CheckResult {
	details := probeDetails(probe)
	if probe.ErrorMessage != "" {
		details["error_message"] = sanitizeMessage(probe.ErrorMessage, apiKey)
	}
	if probe.StatusCode == 0 {
		return failCheck("multimodal", "多模态输入", 10, "无法连接 Responses 多模态探测端点。", details)
	}
	if probe.ErrorClass == errorClassAccountBalanceInsufficient {
		return failCheck("multimodal", "多模态输入", 10, "账号余额不足，多模态探测无法执行。", details)
	}
	if probe.StatusCode >= 200 && probe.StatusCode < 300 && gjson.GetBytes(probe.Body, "object").String() == "response" {
		return passCheck("multimodal", "多模态输入", 10, "Responses 接受 input_image 多模态输入结构。", details)
	}
	if probe.StatusCode == http.StatusBadRequest || probe.StatusCode == http.StatusUnprocessableEntity {
		return CheckResult{ID: "multimodal", Name: "多模态输入", Status: CheckStatusWarn, Score: 5, MaxScore: 10, Message: "端点存在，但当前模型或上游不接受 input_image 输入。", Details: details}
	}
	return failCheck("multimodal", "多模态输入", 10, "多模态探测未返回标准 Responses 响应。", details)
}

func buildTokenAuditCheck(audit *TokenAuditReport) CheckResult {
	details := map[string]any{}
	if audit != nil {
		details["sample_count"] = audit.SampleCount
		details["multiplier"] = audit.Multiplier
		details["overall_ratio"] = audit.OverallRatio
		details["cache_hit_rate"] = audit.CacheHitRate
		details["cached_tokens_field_observed"] = audit.CachedTokensFieldObserved
		details["cache_creation_field_observed"] = audit.CacheCreationFieldObserved
		details["cache_read_field_observed"] = audit.CacheReadFieldObserved
		details["cache_probe_rounds"] = audit.CacheProbeRounds
		details["cache_probe_hits"] = audit.CacheProbeHits
		details["context_replay_rounds"] = audit.ContextReplayRounds
		details["context_replay_links"] = audit.ContextReplayLinks
		details["context_replay_links_expected"] = audit.ContextReplayLinksExpected
		details["context_replay_ok"] = audit.ContextReplayOK
		details["history_replay_rounds"] = audit.HistoryReplayRounds
		details["history_replay_links"] = audit.HistoryReplayLinks
		details["history_replay_links_expected"] = audit.HistoryReplayLinksExpected
		details["history_replay_ok"] = audit.HistoryReplayOK
		details["official_baseline_usd"] = audit.OfficialBaselineUSD
		details["uncached_baseline_usd"] = audit.UncachedBaselineUSD
		details["actual_cost_usd"] = audit.ActualCostUSD
		details["total_cost"] = audit.TotalCostUSD
		if audit.BillingMultiplier != nil {
			details["billing_multiplier"] = *audit.BillingMultiplier
			details["billing_multiplier_source"] = audit.BillingMultiplierSource
		}
		if audit.PromptCacheKey != "" {
			details["prompt_cache_key"] = audit.PromptCacheKey
		}
		details["store_enabled"] = audit.StoreEnabled
		details["stateful_rounds"] = audit.StatefulRounds
		details["previous_response_chain_ok"] = audit.PreviousChainOK
		if len(audit.Anomalies) > 0 {
			details["anomalies"] = append([]string(nil), audit.Anomalies...)
		}
	}
	switch {
	case audit == nil:
		return failCheck("token_audit", "Token 用量审计", 15, "未执行 Token 用量审计。", details)
	case audit.Status == CheckStatusPass:
		return passCheck("token_audit", "Token 用量审计", 15, audit.Summary, details)
	case audit.Status == CheckStatusWarn:
		return CheckResult{ID: "token_audit", Name: "Token 用量审计", Status: CheckStatusWarn, Score: 8, MaxScore: 15, Message: audit.Summary, Details: details}
	default:
		return failCheck("token_audit", "Token 用量审计", 15, audit.Summary, details)
	}
}

func buildChatFallbackCheck(probe httpProbe, apiKey string) CheckResult {
	details := probeDetails(probe)
	if probe.ErrorMessage != "" {
		details["error_message"] = sanitizeMessage(probe.ErrorMessage, apiKey)
	}
	if probe.StatusCode >= 200 && probe.StatusCode < 300 && gjson.GetBytes(probe.Body, "choices").IsArray() {
		details["choice_count"] = len(gjson.GetBytes(probe.Body, "choices").Array())
		details["requested_n"] = 2
		return CheckResult{ID: "chat_completions", Name: "Chat Completions 兼容回退", Status: CheckStatusPass, Score: 0, MaxScore: 0, Message: "Chat Completions 回退端点可用。", Details: details}
	}
	return CheckResult{ID: "chat_completions", Name: "Chat Completions 兼容回退", Status: CheckStatusFail, Score: 0, MaxScore: 0, Message: "Chat Completions 回退端点不可用。", Details: details}
}

func responsesBodyHasExpectedFunctionCall(body []byte) (bool, map[string]any) {
	details := map[string]any{"function_call_seen": false}
	output := gjson.GetBytes(body, "output")
	if !output.IsArray() {
		return false, details
	}
	for _, item := range output.Array() {
		if strings.TrimSpace(item.Get("type").String()) == "function_call" {
			details["function_call_seen"] = true
			details["function_name"] = item.Get("name").String()
			arguments := item.Get("arguments").String()
			details["arguments_json"] = arguments != ""
			if strings.TrimSpace(item.Get("name").String()) != "probe_ping" {
				continue
			}
			if gjson.Get(arguments, "ok").Bool() {
				details["arguments_ok"] = true
				return true, details
			}
			details["arguments_ok"] = false
		}
	}
	return false, details
}

func skippedCoreChecks(message string) []CheckResult {
	return []CheckResult{
		failCheck("responses_schema", "Responses 非流式结构", 20, message, nil),
		failCheck("tool_call", "强制工具调用", 20, message, nil),
		failCheck("streaming", "Responses 流式事件", 15, message, nil),
		failCheck("usage", "Usage 计量", 10, message, nil),
		failCheck("multimodal", "多模态输入", 10, message, nil),
		failCheck("token_audit", "Token 用量审计", 15, message, nil),
	}
}

func passCheck(id, name string, max int, message string, details map[string]any) CheckResult {
	return CheckResult{ID: id, Name: name, Status: CheckStatusPass, Score: max, MaxScore: max, Message: message, Details: details}
}

func failCheck(id, name string, max int, message string, details map[string]any) CheckResult {
	return CheckResult{ID: id, Name: name, Status: CheckStatusFail, Score: 0, MaxScore: max, Message: message, Details: details}
}

func probeDetails(probe httpProbe) map[string]any {
	details := map[string]any{
		"status_code": probe.StatusCode,
		"latency_ms":  probe.LatencyMS,
	}
	if probe.ErrorClass != "" {
		details["error_class"] = probe.ErrorClass
	}
	if probe.ErrorMessage != "" {
		details["error_message"] = probe.ErrorMessage
	}
	return details
}

func firstProbeError(checks []CheckResult) (string, string) {
	for _, check := range checks {
		if check.Status == CheckStatusPass || check.Details == nil {
			continue
		}
		errorClass, _ := check.Details["error_class"].(string)
		errorMessage, _ := check.Details["error_message"].(string)
		if errorClass != "" || errorMessage != "" {
			return errorClass, errorMessage
		}
	}
	return "", ""
}
