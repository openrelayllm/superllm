package purity

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

const (
	defaultClaudeModel       = "claude-opus-4-8"
	anthropicVersion         = "2023-06-01"
	claudeCodeProbeVersion   = "2.1.161"
	claudeFingerprintSalt    = "59cf53e54c78"
	claudeCodeProbeUserAgent = "claude-cli/2.1.161 (external, cli)"
	claudeCodeSystemPrompt   = "You are Claude Code, Anthropic's official CLI for Claude."
	claudeAPIKeyBetaHeader   = "claude-code-20250219,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14"
)

type claudeProbeContext struct {
	deviceID  string
	sessionID string
}

type claudeAuditTurn struct {
	Round         int
	UserText      string
	AssistantText string
}

func newClaudeProbeContext() claudeProbeContext {
	deviceBytes := make([]byte, 32)
	if _, err := rand.Read(deviceBytes); err != nil {
		fallback := sha256.Sum256([]byte(uuid.NewString()))
		copy(deviceBytes, fallback[:])
	}
	return claudeProbeContext{
		deviceID:  hex.EncodeToString(deviceBytes),
		sessionID: uuid.NewString(),
	}
}

func (probe claudeProbeContext) metadata() map[string]any {
	userID, _ := json.Marshal(map[string]string{
		"device_id":    probe.deviceID,
		"account_uuid": "",
		"session_id":   probe.sessionID,
	})
	return map[string]any{"user_id": string(userID)}
}

func (s *Service) runClaudeCheck(ctx context.Context, in PublicCheckInput, emit PublicCheckEventSink, options checkRunOptions) (*PublicReport, error) {
	apiKey := strings.TrimSpace(in.APIKey)
	if apiKey == "" {
		return nil, infraerrors.BadRequest("PURITY_API_KEY_REQUIRED", "api key is required")
	}
	if len(apiKey) > maxAPIKeyLength {
		return nil, infraerrors.BadRequest("PURITY_API_KEY_TOO_LARGE", "api key is too large")
	}
	if options.EnforceRateLimit {
		if err := s.enforceRateLimit(in.ClientIP, apiKey); err != nil {
			return nil, err
		}
	}

	model := strings.TrimSpace(in.ModelID)
	if model == "" {
		model = defaultClaudeModel
	}
	baseURL, host, officialHost, err := s.normalizeAnthropicBaseURL(in.APIBaseURL, options.AllowPrivateHosts)
	if err != nil {
		return nil, err
	}
	client := s.clientForRun(options)
	probeCtx := newClaudeProbeContext()

	startedAt := s.currentTime()
	checkCtx, cancel := context.WithTimeout(ctx, defaultCheckTimeout)
	defer cancel()

	checkedAt := startedAt.UTC()
	report := &PublicReport{
		Provider:        ProviderAnthropic,
		ReportID:        newReportID(ProviderAnthropic, host, model, checkedAt),
		AccessMode:      normalizeAccessMode(options.AccessMode),
		BillingMode:     normalizeBillingMode(options.BillingMode),
		APIBaseHost:     host,
		ModelID:         model,
		CheckTokenUsage: !in.SkipTokenAudit,
		ExpectedModel:   model,
		Status:          RunStatusRunning,
		Verdict:         VerdictUnknown,
		Validations:     []ValidationResult{},
		Checks:          []CheckResult{},
		CheckedAt:       checkedAt,
		Metrics:         PublicCheckMetrics{},
	}
	emitPublicCheckEvent(emit, PublicCheckEvent{
		Type:     PublicCheckEventStarted,
		ReportID: report.ReportID,
		Status:   report.Status,
		Report:   clonePublicReport(report),
	})

	emitProgress(report, emit, 1, "tag")
	baseURLCheck := buildClaudeBaseURLCheck(host, officialHost)
	appendAndEmitChecks(report, emit, baseURLCheck)

	emitProgress(report, emit, 2, "structure")
	messagesProbe := s.probeClaudeMessages(checkCtx, client, baseURL, apiKey, model, probeCtx)
	report.Metrics.MessagesLatencyMS = messagesProbe.LatencyMS
	report.Metrics.Usage = parseClaudeUsage(messagesProbe.Body)
	report.ResponseModel = strings.TrimSpace(gjson.GetBytes(messagesProbe.Body, "model").String())
	report.NonStreamChannel = channelFromProbe(ProviderAnthropic, host, officialHost, messagesProbe)
	schemaCheck := buildClaudeMessagesSchemaCheck(messagesProbe, apiKey)
	toolCheck := buildClaudeToolUseCheck(messagesProbe)
	usageCheck := buildClaudeUsageCheck(report.Metrics.Usage, messagesProbe)
	appendAndEmitChecks(report, emit, schemaCheck, toolCheck, usageCheck)
	upsertAndEmitValidation(report, emit, validationFromExecutedChecks("llm_fingerprint", "LLM 指纹验证", []CheckResult{baseURLCheck, schemaCheck}))
	upsertAndEmitValidation(report, emit, validationFromExecutedChecks("schema_integrity", "结构完整性", []CheckResult{schemaCheck}))
	emitMetrics(report, emit)

	if shouldStopAfterProbe(messagesProbe) {
		markReportProbeError(report, messagesProbe, "基础连接或鉴权未通过")
		appendAndEmitChecks(report, emit, skippedClaudeCoreChecks("基础连接或鉴权未通过，后续 Claude 探测未执行")...)
		upsertAndEmitValidation(report, emit, skippedValidation("behavior", "行为验证", []string{"claude_tool_use", "claude_streaming"}, "基础连接或鉴权未通过，行为验证未执行"))
		upsertAndEmitValidation(report, emit, skippedValidation("multimodal", "多模态能力", []string{"claude_multimodal"}, "基础连接或鉴权未通过，多模态探测未执行"))
		upsertAndEmitValidation(report, emit, skippedValidation("token_audit", "Token 用量审计", []string{"token_audit"}, "基础连接或鉴权未通过，Token 用量审计未执行"))
		report.Metrics.ErrorClass, report.Metrics.ErrorMessage = firstProbeError(report.Checks)
		report.Metrics.LatencyMS = int64(s.currentTime().Sub(startedAt) / time.Millisecond)
		report.HasVertex = hasVertexFingerprint(report.APIBaseHost, messagesProbe.Headers)
		report.IsKiro = hasKiroFingerprint(report.APIBaseHost, messagesProbe.Headers)
		s.finalizeAndSave(ctx, report, baseURL)
		emitFinalReport(report, emit)
		return report, nil
	}

	emitProgress(report, emit, 3, "behavior")
	streamProbe := s.probeClaudeStream(checkCtx, client, baseURL, apiKey, model, probeCtx)
	report.Metrics.StreamFirstTokenMS = streamProbe.FirstTokenMS
	report.Metrics.StreamTotalLatencyMS = streamProbe.TotalLatencyMS
	report.StreamChannel = firstNonEmptyString(channelFromStreamProbe(ProviderAnthropic, host, officialHost, streamProbe.Headers), report.NonStreamChannel)
	streamingCheck := buildClaudeStreamingCheck(streamProbe, apiKey)
	appendAndEmitChecks(report, emit, streamingCheck)
	report.Metrics.TokensPerSecond = tokensPerSecond(report.Metrics.Usage, streamProbe.TotalLatencyMS)
	upsertAndEmitValidation(report, emit, validationFromExecutedChecks("behavior", "行为验证", []CheckResult{toolCheck, streamingCheck}))
	emitMetrics(report, emit)

	emitProgress(report, emit, 4, "signature")
	upsertAndEmitValidation(report, emit, validationFromExecutedChecks("signature", "签名校验", []CheckResult{usageCheck}))
	emitMetrics(report, emit)

	emitProgress(report, emit, 5, "multimodal")
	multimodalProbe := s.probeClaudeMultimodal(checkCtx, client, baseURL, apiKey, model, probeCtx)
	report.Metrics.MultimodalLatencyMS = multimodalProbe.LatencyMS
	multimodalCheck := buildClaudeMultimodalCheck(multimodalProbe, apiKey)
	appendAndEmitChecks(report, emit, multimodalCheck)
	upsertAndEmitValidation(report, emit, validationFromExecutedChecks("multimodal", "多模态能力", []CheckResult{multimodalCheck}))
	emitMetrics(report, emit)

	if in.SkipTokenAudit {
		tokenAuditCheck := CheckResult{ID: "token_audit", Name: "Token 用量审计", Status: CheckStatusWarn, Score: 0, MaxScore: 15, Message: "本次请求已关闭 Token 用量审计。", Details: map[string]any{"skipped": true}}
		appendAndEmitChecks(report, emit, tokenAuditCheck)
		upsertAndEmitValidation(report, emit, validationFromExecutedChecks("token_audit", "Token 用量审计", []CheckResult{tokenAuditCheck}))
	} else if messagesProbe.StatusCode >= 200 && messagesProbe.StatusCode < 300 {
		emitProgress(report, emit, 6, "token_audit")
		report.TokenAudit = s.runClaudeTokenAudit(checkCtx, client, baseURL, apiKey, model, probeCtx, func(sample TokenAuditSample) {
			report.TokenAuditPartial = upsertTokenAuditPartial(report.TokenAuditPartial, sample)
			report.TokenAuditProgress = fmt.Sprintf("%d/%d", len(report.TokenAuditPartial), tokenAuditSamples)
			emitPublicCheckEvent(emit, PublicCheckEvent{
				Type:               PublicCheckEventTokenAuditSample,
				ReportID:           report.ReportID,
				Status:             report.Status,
				Step:               report.Step,
				StepName:           report.StepName,
				Progress:           report.Progress,
				Scores:             cloneScores(report.Scores),
				Sample:             &sample,
				TokenAuditProgress: report.TokenAuditProgress,
				TokenAuditPartial:  append([]TokenAuditSample(nil), report.TokenAuditPartial...),
			})
		})
		tokenAuditCheck := buildTokenAuditCheck(report.TokenAudit)
		appendAndEmitChecks(report, emit, tokenAuditCheck)
		upsertAndEmitValidation(report, emit, validationFromExecutedChecks("token_audit", "Token 用量审计", []CheckResult{tokenAuditCheck}))
		emitPublicCheckEvent(emit, PublicCheckEvent{
			Type:               PublicCheckEventTokenAudit,
			ReportID:           report.ReportID,
			Status:             report.Status,
			Step:               report.Step,
			StepName:           report.StepName,
			Progress:           report.Progress,
			Scores:             cloneScores(report.Scores),
			TokenAudit:         report.TokenAudit,
			TokenAuditProgress: report.TokenAuditProgress,
			TokenAuditPartial:  append([]TokenAuditSample(nil), report.TokenAuditPartial...),
		})
	} else {
		tokenAuditCheck := failCheck("token_audit", "Token 用量审计", 15, "Messages 非流式探测未通过，Token 用量审计未执行。", nil)
		appendAndEmitChecks(report, emit, tokenAuditCheck)
		upsertAndEmitValidation(report, emit, validationFromExecutedChecks("token_audit", "Token 用量审计", []CheckResult{tokenAuditCheck}))
	}

	report.HasVertex = hasVertexFingerprint(report.APIBaseHost, messagesProbe.Headers, streamProbe.Headers)
	report.IsKiro = hasKiroFingerprint(report.APIBaseHost, messagesProbe.Headers, streamProbe.Headers)
	emitProgress(report, emit, 7, "evaluate")
	report.Metrics.LatencyMS = int64(s.currentTime().Sub(startedAt) / time.Millisecond)
	s.finalizeAndSave(ctx, report, baseURL)
	emitFinalReport(report, emit)
	return report, nil
}

func (s *Service) normalizeAnthropicBaseURL(raw string, allowPrivate bool) (string, string, bool, error) {
	normalized, err := urlvalidator.ValidateHTTPURL(raw, true, urlvalidator.ValidationOptions{
		AllowPrivate: allowPrivate,
	})
	if err != nil {
		return "", "", false, infraerrors.BadRequest("PURITY_BASE_URL_INVALID", "invalid api base url")
	}
	parsed, err := url.Parse(normalized)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", "", false, infraerrors.BadRequest("PURITY_BASE_URL_INVALID", "invalid api base url")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = normalizeAnthropicBasePath(parsed.EscapedPath())
	parsed.RawPath = ""

	host := strings.ToLower(parsed.Host)
	officialHost := strings.EqualFold(parsed.Hostname(), "api.anthropic.com")
	return strings.TrimRight(parsed.String(), "/"), host, officialHost, nil
}

func normalizeAnthropicBasePath(path string) string {
	value := strings.TrimRight(strings.TrimSpace(path), "/")
	if value == "" {
		return ""
	}
	for _, suffix := range []string{"/v1/messages/count_tokens", "/messages/count_tokens", "/v1/messages", "/messages", "/v1/models", "/models"} {
		if strings.HasSuffix(value, suffix) {
			value = strings.TrimRight(strings.TrimSuffix(value, suffix), "/")
			break
		}
	}
	return value
}

func (s *Service) probeClaudeMessages(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string, probeCtx claudeProbeContext) httpProbe {
	return s.doJSONWithHeaders(ctx, client, http.MethodPost, buildOpenAIEndpointURL(baseURL, "/v1/messages"), claudeToolProbePayload(model, probeCtx), claudeHeaders("application/json", apiKey, probeCtx), apiKey)
}

func (s *Service) probeClaudeMultimodal(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string, probeCtx claudeProbeContext) httpProbe {
	return s.doJSONWithHeaders(ctx, client, http.MethodPost, buildOpenAIEndpointURL(baseURL, "/v1/messages"), claudeMultimodalProbePayload(model, probeCtx), claudeHeaders("application/json", apiKey, probeCtx), apiKey)
}

func (s *Service) probeClaudeAudit(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string, round int, auditNonce string, probeCtx claudeProbeContext, history []claudeAuditTurn) httpProbe {
	return s.doJSONWithHeaders(ctx, client, http.MethodPost, buildOpenAIEndpointURL(baseURL, "/v1/messages"), claudeAuditProbePayload(model, round, auditNonce, probeCtx, history), claudeHeaders("application/json", apiKey, probeCtx), apiKey)
}

func claudeHeaders(accept string, apiKey string, probeCtx claudeProbeContext) map[string]string {
	headers := map[string]string{
		"User-Agent":                                claudeCodeProbeUserAgent,
		"X-Stainless-Lang":                          "js",
		"X-Stainless-Package-Version":               "0.94.0",
		"X-Stainless-OS":                            "Linux",
		"X-Stainless-Arch":                          "arm64",
		"X-Stainless-Runtime":                       "node",
		"X-Stainless-Runtime-Version":               "v24.3.0",
		"X-Stainless-Retry-Count":                   "0",
		"X-Stainless-Timeout":                       "600",
		"X-App":                                     "cli",
		"Anthropic-Dangerous-Direct-Browser-Access": "true",
		"X-Claude-Code-Session-Id":                  probeCtx.sessionID,
		"x-client-request-id":                       uuid.NewString(),
		"Accept":                                    accept,
		"Content-Type":                              "application/json",
		"x-api-key":                                 apiKey,
		"anthropic-version":                         anthropicVersion,
		"anthropic-beta":                            claudeAPIKeyBetaHeader,
	}
	return headers
}

func claudeToolProbePayload(model string, probeCtx claudeProbeContext) []byte {
	body, _ := json.Marshal(map[string]any{
		"model":       firstNonEmptyString(model, defaultClaudeModel),
		"max_tokens":  512,
		"temperature": 1,
		"system":      claudeSystemBlocks("Call the requested tool exactly once."),
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					claudeTextBlock("Call the probe_ping tool with ok=true. Do not answer in text.", true),
				},
			},
		},
		"tools": []map[string]any{
			{
				"name":        "probe_ping",
				"description": "Capability probe. Call to acknowledge.",
				"input_schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"ok": map[string]any{"type": "boolean"},
					},
					"required": []string{"ok"},
				},
			},
		},
		"tool_choice": map[string]any{"type": "tool", "name": "probe_ping"},
		"metadata":    probeCtx.metadata(),
	})
	return body
}

func claudeStreamProbePayload(model string, probeCtx claudeProbeContext) []byte {
	body, _ := json.Marshal(map[string]any{
		"model":       firstNonEmptyString(model, defaultClaudeModel),
		"max_tokens":  32,
		"temperature": 1,
		"stream":      true,
		"system":      claudeSystemBlocks("Return concise responses."),
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					claudeTextBlock("Return exactly: ok", true),
				},
			},
		},
		"metadata": probeCtx.metadata(),
	})
	return body
}

func claudeMultimodalProbePayload(model string, probeCtx claudeProbeContext) []byte {
	body, _ := json.Marshal(map[string]any{
		"model":       firstNonEmptyString(model, defaultClaudeModel),
		"max_tokens":  32,
		"temperature": 1,
		"system":      claudeSystemBlocks("Return concise responses."),
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": "Read the attached image and return exactly: ok"},
					{
						"type": "image",
						"source": map[string]any{
							"type":       "base64",
							"media_type": "image/png",
							"data":       probePNGBase64,
						},
					},
				},
			},
		},
		"metadata": probeCtx.metadata(),
	})
	return body
}

func claudeAuditProbePayload(model string, round int, auditNonce string, probeCtx claudeProbeContext, history []claudeAuditTurn) []byte {
	body, _ := json.Marshal(map[string]any{
		"model":       firstNonEmptyString(model, defaultClaudeModel),
		"max_tokens":  claudeTokenAuditOutputBudget(round),
		"temperature": 1,
		"system":      claudeAuditSystemBlocks(round, auditNonce),
		"messages":    claudeAuditMessages(round, auditNonce, history),
		"metadata":    probeCtx.metadata(),
	})
	return body
}

func claudeAuditSystemBlocks(round int, auditNonce string) []map[string]any {
	round = clampAuditRound(round)
	return []map[string]any{
		claudeTextBlock(claudeBillingAttributionText(fmt.Sprintf("Purity token audit round %02d", round)), false),
		claudeTextBlock(strings.Join([]string{
			claudeCodeSystemPrompt,
			auditStableCacheText(auditNonce),
		}, "\n\n"), true),
	}
}

func claudeAuditMessages(round int, auditNonce string, history []claudeAuditTurn) []map[string]any {
	round = clampAuditRound(round)
	messages := make([]map[string]any, 0, len(history)*2+1)
	for _, turn := range history {
		userText := strings.TrimSpace(turn.UserText)
		if userText == "" {
			userText = claudeAuditUserText(turn.Round, auditNonce)
		}
		messages = append(messages, map[string]any{
			"role":    "user",
			"content": []map[string]any{claudeTextBlock(userText, false)},
		})
		assistantText := strings.TrimSpace(turn.AssistantText)
		if assistantText == "" {
			continue
		}
		messages = append(messages, map[string]any{
			"role":    "assistant",
			"content": []map[string]any{claudeTextBlock(assistantText, false)},
		})
	}
	messages = append(messages, map[string]any{
		"role":    "user",
		"content": []map[string]any{claudeTextBlock(claudeAuditUserText(round, auditNonce), true)},
	})
	return messages
}

func clampAuditRound(round int) int {
	if round < 1 {
		return 1
	}
	if round > tokenAuditSamples {
		return tokenAuditSamples
	}
	return round
}

func claudeSystemBlocks(firstUserText string) []map[string]any {
	return []map[string]any{
		claudeTextBlock(claudeBillingAttributionText(firstUserText), false),
		claudeTextBlock(claudeCodeSystemPrompt, true),
	}
}

func claudeAuditResponseInstruction(round int) string {
	return strings.Join([]string{
		"Purity token audit response target.",
		claudeTokenAuditRoundInstruction(round),
	}, "\n\n")
}

func claudeTokenAuditOutputBudget(round int) int {
	target := claudeTokenAuditOutputTarget(round)
	if target <= 0 {
		return tokenAuditOutputBudget(round)
	}
	return minInt(1800, target+160)
}

func claudeTokenAuditRoundInstruction(round int) string {
	target := claudeTokenAuditOutputTarget(round)
	return fmt.Sprintf("Round %02d: output exactly %d comma-separated items. Each item must be the single lowercase letter x. Use no spaces, no numbering, no markdown, and no text before or after the list.", clampAuditRound(round), target)
}

func claudeTokenAuditOutputTarget(round int) int {
	targets := []int{152, 162, 770, 336, 616, 668, 1413, 783, 128, 111, 387}
	round = clampAuditRound(round)
	return targets[round-1]
}

func claudeAuditUserText(round int, auditNonce string) string {
	round = clampAuditRound(round)
	return strings.Join([]string{
		fmt.Sprintf("proxyai.best Claude cache audit turn %02d. audit_nonce=%s", round, auditNonce),
		auditRoundCacheText(round),
		claudeAuditResponseInstruction(round),
	}, "\n\n")
}

func claudeTextBlock(text string, cache bool) map[string]any {
	block := map[string]any{"type": "text", "text": text}
	if cache {
		block["cache_control"] = map[string]any{"type": "ephemeral"}
	}
	return block
}

func claudeBillingAttributionText(firstUserText string) string {
	chars := []byte{'0', '0', '0'}
	for idx, pos := range []int{4, 7, 20} {
		if pos < len(firstUserText) {
			chars[idx] = firstUserText[pos]
		}
	}
	sum := sha256.Sum256([]byte(claudeFingerprintSalt + string(chars) + claudeCodeProbeVersion))
	fp := hex.EncodeToString(sum[:])[:3]
	return fmt.Sprintf("x-anthropic-billing-header: cc_version=%s.%s; cc_entrypoint=cli; cch=00000;", claudeCodeProbeVersion, fp)
}

func buildClaudeBaseURLCheck(host string, officialHost bool) CheckResult {
	if officialHost {
		return CheckResult{
			ID:       "base_url",
			Name:     "API Base 域名",
			Status:   CheckStatusPass,
			Score:    20,
			MaxScore: 20,
			Message:  "命中 Anthropic 官方 API 域名。",
			Details:  map[string]any{"host": host, "official_host": true},
		}
	}
	return CheckResult{
		ID:       "base_url",
		Name:     "API Base 域名",
		Status:   CheckStatusPass,
		Score:    20,
		MaxScore: 20,
		Message:  "当前为 Claude 兼容入口，域名仅作路由记录，纯度由后续探针判断。",
		Details:  map[string]any{"host": host, "official_host": false},
	}
}

func buildClaudeMessagesSchemaCheck(probe httpProbe, apiKey string) CheckResult {
	details := probeDetails(probe)
	if probe.StatusCode == 0 {
		return failCheck("claude_messages_schema", "Messages 非流式结构", 20, "无法连接 Messages 端点。", details)
	}
	if probe.ErrorClass == errorClassAccountBalanceInsufficient {
		return failCheck("claude_messages_schema", "Messages 非流式结构", 20, "账号余额不足，Messages 探测无法执行。", details)
	}
	if probe.StatusCode == http.StatusUnauthorized || probe.StatusCode == http.StatusForbidden {
		return failCheck("claude_messages_schema", "Messages 非流式结构", 20, "API Key 鉴权失败。", details)
	}
	if probe.StatusCode == http.StatusNotFound || probe.StatusCode == http.StatusMethodNotAllowed {
		return failCheck("claude_messages_schema", "Messages 非流式结构", 20, "Messages 端点不存在或方法不支持。", details)
	}
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		details["error_message"] = sanitizeMessage(upstreamErrorMessage(probe.Body), apiKey)
		return failCheck("claude_messages_schema", "Messages 非流式结构", 20, "Messages 端点未返回可用响应。", details)
	}
	if gjson.GetBytes(probe.Body, "type").String() == "message" && gjson.GetBytes(probe.Body, "content").IsArray() {
		details["response_model"] = gjson.GetBytes(probe.Body, "model").String()
		return passCheck("claude_messages_schema", "Messages 非流式结构", 20, "Messages 响应结构符合 Anthropic 预期。", details)
	}
	return CheckResult{ID: "claude_messages_schema", Name: "Messages 非流式结构", Status: CheckStatusWarn, Score: 8, MaxScore: 20, Message: "Messages 返回 2xx，但响应结构不完整。", Details: details}
}

func buildClaudeToolUseCheck(probe httpProbe) CheckResult {
	details := probeDetails(probe)
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		return failCheck("claude_tool_use", "强制工具调用", 20, "Messages 探测未成功，无法确认工具调用。", details)
	}
	ok, toolDetails := claudeBodyHasExpectedToolUse(probe.Body)
	for key, value := range toolDetails {
		details[key] = value
	}
	if ok {
		return passCheck("claude_tool_use", "强制工具调用", 20, "tool_choice 成功产出 probe_ping(ok=true) tool_use。", details)
	}
	return failCheck("claude_tool_use", "强制工具调用", 20, "强制工具调用没有产出预期 tool_use。", details)
}

func buildClaudeUsageCheck(usage *TokenUsage, probe httpProbe) CheckResult {
	details := probeDetails(probe)
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		return failCheck("claude_usage", "Usage 计量", 10, "Messages 探测未成功，无法读取 usage。", details)
	}
	if usage == nil {
		return failCheck("claude_usage", "Usage 计量", 10, "响应缺少 usage 计量字段。", details)
	}
	details["input_tokens"] = usage.InputTokens
	details["output_tokens"] = usage.OutputTokens
	details["cache_creation_input_tokens"] = usage.CacheCreationTokens
	details["cache_read_input_tokens"] = usage.CachedTokens
	if usage.InputTokens+usage.OutputTokens+usage.CacheCreationTokens+usage.CachedTokens > 0 {
		return passCheck("claude_usage", "Usage 计量", 10, "usage token 计量字段完整。", details)
	}
	return CheckResult{ID: "claude_usage", Name: "Usage 计量", Status: CheckStatusWarn, Score: 5, MaxScore: 10, Message: "usage 字段存在，但 token 值为空。", Details: details}
}

func buildClaudeMultimodalCheck(probe httpProbe, apiKey string) CheckResult {
	details := probeDetails(probe)
	if probe.ErrorMessage != "" {
		details["error_message"] = sanitizeMessage(probe.ErrorMessage, apiKey)
	}
	if probe.StatusCode == 0 {
		return failCheck("claude_multimodal", "多模态输入", 10, "无法连接 Messages 多模态探测端点。", details)
	}
	if probe.ErrorClass == errorClassAccountBalanceInsufficient {
		return failCheck("claude_multimodal", "多模态输入", 10, "账号余额不足，多模态探测无法执行。", details)
	}
	if probe.StatusCode >= 200 && probe.StatusCode < 300 && gjson.GetBytes(probe.Body, "type").String() == "message" {
		return passCheck("claude_multimodal", "多模态输入", 10, "Messages 接受 image block 多模态输入结构。", details)
	}
	if probe.StatusCode == http.StatusBadRequest || probe.StatusCode == http.StatusUnprocessableEntity {
		return CheckResult{ID: "claude_multimodal", Name: "多模态输入", Status: CheckStatusWarn, Score: 5, MaxScore: 10, Message: "端点存在，但当前模型或上游不接受 image block。", Details: details}
	}
	return failCheck("claude_multimodal", "多模态输入", 10, "多模态探测未返回标准 Messages 响应。", details)
}

func claudeBodyHasExpectedToolUse(body []byte) (bool, map[string]any) {
	details := map[string]any{"tool_use_seen": false}
	content := gjson.GetBytes(body, "content")
	if !content.IsArray() {
		return false, details
	}
	for _, item := range content.Array() {
		if strings.TrimSpace(item.Get("type").String()) != "tool_use" {
			continue
		}
		details["tool_use_seen"] = true
		details["tool_name"] = item.Get("name").String()
		if strings.TrimSpace(item.Get("name").String()) != "probe_ping" {
			continue
		}
		if item.Get("input.ok").Bool() {
			details["arguments_ok"] = true
			return true, details
		}
		details["arguments_ok"] = false
	}
	return false, details
}

func parseClaudeUsage(body []byte) *TokenUsage {
	usage := gjson.GetBytes(body, "usage")
	if !usage.Exists() || !usage.IsObject() {
		return nil
	}
	input := usage.Get("input_tokens").Int()
	output := usage.Get("output_tokens").Int()
	cacheCreate := usage.Get("cache_creation_input_tokens").Int()
	cacheRead := usage.Get("cache_read_input_tokens").Int()
	return &TokenUsage{
		InputTokens:         input,
		OutputTokens:        output,
		TotalTokens:         input + output + cacheCreate + cacheRead,
		CacheCreationTokens: cacheCreate,
		CachedTokens:        cacheRead,
	}
}

type claudeStreamProbe struct {
	StatusCode            int
	Headers               map[string]string
	FirstTokenMS          int64
	TotalLatencyMS        int64
	SeenData              bool
	SeenMessageStart      bool
	SeenContentBlockStart bool
	SeenDelta             bool
	SeenMessageDelta      bool
	SeenMessageStop       bool
	SeenToolUse           bool
	ErrorClass            string
	ErrorMessage          string
}

func (s *Service) probeClaudeStream(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string, probeCtx claudeProbeContext) claudeStreamProbe {
	started := s.currentTime()
	body := claudeStreamProbePayload(model, probeCtx)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildOpenAIEndpointURL(baseURL, "/v1/messages"), bytes.NewReader(body))
	if err != nil {
		return claudeStreamProbe{ErrorClass: "request_build_error", ErrorMessage: sanitizeMessage(err.Error(), apiKey)}
	}
	for key, value := range claudeHeaders("text/event-stream", apiKey, probeCtx) {
		req.Header.Set(key, value)
	}
	if client == nil {
		client = s.clientForRun(checkRunOptions{})
	}
	resp, err := client.Do(req)
	if err != nil {
		return claudeStreamProbe{
			TotalLatencyMS: int64(s.currentTime().Sub(started) / time.Millisecond),
			ErrorClass:     "network_error",
			ErrorMessage:   sanitizeMessage(err.Error(), apiKey),
		}
	}
	defer func() { _ = resp.Body.Close() }()

	result := claudeStreamProbe{StatusCode: resp.StatusCode, Headers: selectedResponseHeaders(resp.Header)}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, maxProbeBodyBytes))
		result.TotalLatencyMS = int64(s.currentTime().Sub(started) / time.Millisecond)
		errorMessage := upstreamErrorMessage(bodyBytes)
		result.ErrorClass = errorClassForStatusAndMessage(resp.StatusCode, errorMessage)
		result.ErrorMessage = sanitizeMessage(errorMessage, apiKey)
		return result
	}
	readClaudeStream(resp.Body, started, s.currentTime, &result, apiKey)
	result.TotalLatencyMS = int64(s.currentTime().Sub(started) / time.Millisecond)
	return result
}

func readClaudeStream(body io.Reader, started time.Time, now func() time.Time, result *claudeStreamProbe, apiKey string) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		result.SeenData = true
		eventType := strings.TrimSpace(gjson.Get(data, "type").String())
		switch eventType {
		case "message_start":
			result.SeenMessageStart = true
		case "content_block_start":
			result.SeenContentBlockStart = true
			if gjson.Get(data, "content_block.type").String() == "tool_use" {
				result.SeenToolUse = true
			}
		case "content_block_delta":
			deltaType := gjson.Get(data, "delta.type").String()
			if deltaType == "text_delta" || deltaType == "input_json_delta" || deltaType == "thinking_delta" {
				result.SeenDelta = true
				if result.FirstTokenMS == 0 {
					result.FirstTokenMS = int64(now().Sub(started) / time.Millisecond)
				}
			}
		case "message_delta":
			result.SeenMessageDelta = true
		case "message_stop":
			result.SeenMessageStop = true
		case "error":
			result.ErrorClass = "response_failed"
			result.ErrorMessage = sanitizeMessage(upstreamErrorMessage([]byte(data)), apiKey)
		}
	}
	if err := scanner.Err(); err != nil {
		result.ErrorClass = "stream_error"
		result.ErrorMessage = sanitizeMessage(err.Error(), apiKey)
	}
}

func buildClaudeStreamingCheck(probe claudeStreamProbe, apiKey string) CheckResult {
	details := map[string]any{
		"status_code":              probe.StatusCode,
		"first_token_ms":           probe.FirstTokenMS,
		"total_latency_ms":         probe.TotalLatencyMS,
		"seen_data":                probe.SeenData,
		"seen_message_start":       probe.SeenMessageStart,
		"seen_content_block_start": probe.SeenContentBlockStart,
		"seen_delta":               probe.SeenDelta,
		"seen_message_delta":       probe.SeenMessageDelta,
		"seen_message_stop":        probe.SeenMessageStop,
		"seen_tool_use":            probe.SeenToolUse,
	}
	if probe.ErrorClass != "" {
		details["error_class"] = probe.ErrorClass
		details["error_message"] = sanitizeMessage(probe.ErrorMessage, apiKey)
	}
	if probe.StatusCode == 0 {
		return failCheck("claude_streaming", "Messages 流式事件", 15, "无法连接 Messages 流式端点。", details)
	}
	if probe.ErrorClass == errorClassAccountBalanceInsufficient {
		return failCheck("claude_streaming", "Messages 流式事件", 15, "账号余额不足，Messages 流式探测无法执行。", details)
	}
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		return failCheck("claude_streaming", "Messages 流式事件", 15, "Messages 流式端点未返回可用响应。", details)
	}
	if probe.SeenMessageStart && probe.SeenContentBlockStart && probe.SeenDelta && probe.SeenMessageDelta && probe.SeenMessageStop && probe.ErrorClass == "" {
		return passCheck("claude_streaming", "Messages 流式事件", 15, "SSE message/content/delta/stop 事件序列完整。", details)
	}
	if probe.SeenMessageStart && probe.SeenMessageStop {
		return CheckResult{ID: "claude_streaming", Name: "Messages 流式事件", Status: CheckStatusWarn, Score: 8, MaxScore: 15, Message: "SSE 生命周期结束，但中间 delta 或 message_delta 不完整。", Details: details}
	}
	return failCheck("claude_streaming", "Messages 流式事件", 15, "SSE 生命周期不完整。", details)
}

func skippedClaudeCoreChecks(message string) []CheckResult {
	return []CheckResult{
		failCheck("claude_streaming", "Messages 流式事件", 15, message, nil),
		failCheck("claude_multimodal", "多模态输入", 10, message, nil),
		failCheck("token_audit", "Token 用量审计", 15, message, nil),
	}
}

type claudeModelPricing struct {
	InputPerToken      float64
	OutputPerToken     float64
	CacheWritePerToken float64
	CacheReadPerToken  float64
	Source             string
}

func claudeModelPricingFor(model string) claudeModelPricing {
	key := strings.ToLower(strings.TrimSpace(model))
	if strings.Contains(key, "opus-4-8") || strings.Contains(key, "opus-4-7") || strings.Contains(key, "opus-4-6") || strings.Contains(key, "opus-4-5") {
		return claudeModelPricing{InputPerToken: 5e-6, OutputPerToken: 25e-6, CacheWritePerToken: 6.25e-6, CacheReadPerToken: 0.5e-6, Source: "Official Anthropic Claude pricing, Opus 4.x current generation, 5m cache writes and cache hits, verified 2026-06-27"}
	}
	if strings.Contains(key, "opus") {
		return claudeModelPricing{InputPerToken: 15e-6, OutputPerToken: 75e-6, CacheWritePerToken: 18.75e-6, CacheReadPerToken: 1.5e-6, Source: "Official Anthropic Claude pricing, legacy Opus class, 5m cache writes and cache hits, verified 2026-06-27"}
	}
	if strings.Contains(key, "haiku-4-5") || strings.Contains(key, "haiku-4.5") {
		return claudeModelPricing{InputPerToken: 1e-6, OutputPerToken: 5e-6, CacheWritePerToken: 1.25e-6, CacheReadPerToken: 0.1e-6, Source: "Official Anthropic Claude pricing, Haiku 4.5, 5m cache writes and cache hits, verified 2026-06-27"}
	}
	if strings.Contains(key, "haiku") {
		return claudeModelPricing{InputPerToken: 0.8e-6, OutputPerToken: 4e-6, CacheWritePerToken: 1e-6, CacheReadPerToken: 0.08e-6, Source: "Official Anthropic Claude pricing, legacy Haiku class, 5m cache writes and cache hits, verified 2026-06-27"}
	}
	return claudeModelPricing{InputPerToken: 3e-6, OutputPerToken: 15e-6, CacheWritePerToken: 3.75e-6, CacheReadPerToken: 0.3e-6, Source: "Official Anthropic Claude pricing, Sonnet 4.x, 5m cache writes and cache hits, verified 2026-06-27"}
}

func (s *Service) runClaudeTokenAudit(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string, probeCtx claudeProbeContext, emitSample func(TokenAuditSample)) *TokenAuditReport {
	pricing := claudeModelPricingFor(model)
	auditNonce := newAuditNonce(model, s.currentTime())
	report := &TokenAuditReport{
		Status:      CheckStatusWarn,
		Summary:     "Token 用量审计样本不足。",
		PriceSource: pricing.Source,
		Samples:     make([]TokenAuditSample, 0, tokenAuditSamples),
	}
	var totalOfficial float64
	var totalActual float64
	history := make([]claudeAuditTurn, 0, tokenAuditSamples)
	for i := 1; i <= tokenAuditSamples; i++ {
		select {
		case <-ctx.Done():
			report.Summary = "Token 用量审计超时，已保留完成样本。"
			finalizeClaudeTokenAudit(report)
			return report
		default:
		}
		probe := s.probeClaudeAudit(ctx, client, baseURL, apiKey, model, i, auditNonce, probeCtx, history)
		sample := claudeTokenAuditSampleFromProbe(i, probe, pricing)
		report.Samples = append(report.Samples, sample)
		if emitSample != nil {
			emitSample(sample)
		}
		if sample.Status != CheckStatusPass {
			continue
		}
		report.InputTokens += sample.InputTokens
		report.OutputTokens += sample.OutputTokens
		report.CacheCreationTokens += sample.CacheCreationTokens
		report.CachedTokens += sample.CachedTokens
		totalOfficial += sample.OfficialBaselineUSD
		totalActual += sample.ActualCostUSD
		if assistantText := claudeAssistantTextFromBody(probe.Body); assistantText != "" {
			history = append(history, claudeAuditTurn{
				Round:         i,
				UserText:      claudeAuditUserText(i, auditNonce),
				AssistantText: assistantText,
			})
		}
	}
	report.OfficialBaselineUSD = roundMoney(totalOfficial)
	report.ActualCostUSD = roundMoney(totalActual)
	finalizeClaudeTokenAudit(report)
	return report
}

func finalizeClaudeTokenAudit(report *TokenAuditReport) {
	finalizeTokenAudit(report)
	applyClaudeWarmCacheHitRate(report)
}

func applyClaudeWarmCacheHitRate(report *TokenAuditReport) {
	if report == nil || len(report.Samples) < 2 {
		return
	}
	var inputTokens int64
	var cacheCreationTokens int64
	var cachedTokens int64
	for _, sample := range report.Samples[1:] {
		if sample.Status != CheckStatusPass {
			continue
		}
		inputTokens += sample.InputTokens
		cacheCreationTokens += sample.CacheCreationTokens
		cachedTokens += sample.CachedTokens
	}
	denominator := inputTokens + cacheCreationTokens + cachedTokens
	if denominator <= 0 {
		return
	}
	report.CacheHitRate = roundRatio(float64(cachedTokens) / float64(denominator))
	report.CacheHitRatePercent = math.Round(report.CacheHitRate * 100)
}

func claudeTokenAuditSampleFromProbe(index int, probe httpProbe, pricing claudeModelPricing) TokenAuditSample {
	sample := TokenAuditSample{
		Index:     index,
		Round:     index,
		LatencyMS: probe.LatencyMS,
		Status:    CheckStatusFail,
	}
	applyClaudeTokenAuditBaseline(&sample, pricing)
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		return sample
	}
	usage := parseClaudeUsage(probe.Body)
	if usage == nil {
		return sample
	}
	sample.InputTokens = usage.InputTokens
	sample.OutputTokens = usage.OutputTokens
	sample.CachedTokens = usage.CachedTokens
	sample.CacheReadInputTokens = usage.CachedTokens
	sample.CacheCreationTokens = usage.CacheCreationTokens
	sample.CacheCreationInputTokens = usage.CacheCreationTokens
	sample.UncachedInputTokens = usage.InputTokens
	sample.TotalTokens = usage.TotalTokens
	sample.ActualCostUSD = roundMoney(claudeTokenCost(usage, pricing, true))
	sample.CostUSD = sample.ActualCostUSD
	if sample.OfficialBaselineUSD > 0 {
		sample.Multiplier = roundRatio(sample.ActualCostUSD / sample.OfficialBaselineUSD)
		sample.Ratio = sample.Multiplier
	}
	sample.Status = CheckStatusPass
	applyTokenAuditSampleDerivedFields(&sample)
	return sample
}

func applyClaudeTokenAuditBaseline(sample *TokenAuditSample, pricing claudeModelPricing) {
	if sample == nil {
		return
	}
	baseline := claudeTokenAuditBaselineUsage(sample.Index)
	if baseline == nil {
		return
	}
	sample.BaselineInputTokens = baseline.InputTokens
	sample.BaselineOutputTokens = baseline.OutputTokens
	sample.BaselineCacheCreation = baseline.CacheCreationTokens
	sample.BaselineCacheRead = baseline.CachedTokens
	sample.OfficialBaselineUSD = roundMoney(claudeTokenCost(baseline, pricing, true))
	sample.BaselineCostUSD = sample.OfficialBaselineUSD
}

func claudeTokenAuditBaselineUsage(round int) *TokenUsage {
	round = clampAuditRound(round)
	inputTokens := []int64{2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	outputTokens := []int64{152, 162, 770, 336, 616, 668, 1413, 783, 128, 111, 387}
	cacheCreationTokens := []int64{21300, 422, 169, 822, 400, 676, 572, 2441, 919, 1135, 418}
	cacheReadTokens := []int64{15590, 23911, 24291, 24441, 25263, 25643, 26319, 26891, 29332, 30251, 31386}
	return &TokenUsage{
		InputTokens:         inputTokens[round-1],
		OutputTokens:        outputTokens[round-1],
		CacheCreationTokens: cacheCreationTokens[round-1],
		CachedTokens:        cacheReadTokens[round-1],
		TotalTokens:         inputTokens[round-1] + outputTokens[round-1] + cacheCreationTokens[round-1] + cacheReadTokens[round-1],
	}
}

func claudeAssistantTextFromBody(body []byte) string {
	content := gjson.GetBytes(body, "content")
	if !content.IsArray() {
		return ""
	}
	var parts []string
	for _, item := range content.Array() {
		if strings.TrimSpace(item.Get("type").String()) != "text" {
			continue
		}
		text := strings.TrimSpace(item.Get("text").String())
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func claudeTokenCost(usage *TokenUsage, pricing claudeModelPricing, cacheAware bool) float64 {
	if usage == nil {
		return 0
	}
	cost := float64(usage.InputTokens) * pricing.InputPerToken
	if cacheAware {
		cost += float64(usage.CacheCreationTokens) * pricing.CacheWritePerToken
		cost += float64(usage.CachedTokens) * pricing.CacheReadPerToken
	} else {
		cost += float64(usage.CacheCreationTokens+usage.CachedTokens) * pricing.InputPerToken
	}
	cost += float64(usage.OutputTokens) * pricing.OutputPerToken
	return cost
}
