package purity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coreservice "github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestServiceRunPublicCheck_OpenAICompatible(t *testing.T) {
	auditResponseIndex := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/v1/models":
			writeJSON(t, w, map[string]any{
				"object": "list",
				"data": []map[string]any{
					{"id": "gpt-5.4", "object": "model"},
				},
			})
		case "/v1/responses":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			if stream, _ := body["stream"].(bool); stream {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprintln(w, `data: {"type":"response.output_text.delta","delta":"ok"}`)
				_, _ = fmt.Fprintln(w)
				_, _ = fmt.Fprintln(w, `data: {"type":"response.completed","response":{"status":"completed"}}`)
				_, _ = fmt.Fprintln(w)
				_, _ = fmt.Fprintln(w, "data: [DONE]")
				return
			}
			if _, ok := body["prompt_cache_key"].(string); ok {
				writeOpenAITokenAuditTestResponse(t, w, body, &auditResponseIndex)
				return
			}
			writeJSON(t, w, map[string]any{
				"id":     "resp_1",
				"object": "response",
				"output": []map[string]any{
					{"type": "function_call", "name": "probe_ping", "arguments": `{"ok":true}`},
				},
				"usage": map[string]any{
					"input_tokens":  8,
					"output_tokens": 3,
					"total_tokens":  11,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewService(nil)
	service.httpClient = server.Client()
	service.allowPrivateHosts = true
	service.limiter = nil
	service.now = func() time.Time { return time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC) }

	report, err := service.RunPublicCheck(context.Background(), PublicCheckInput{
		Provider:   ProviderOpenAI,
		APIBaseURL: server.URL,
		APIKey:     "sk-test",
		ModelID:    "gpt-5.4",
		ClientIP:   "203.0.113.10",
	})
	require.NoError(t, err)
	require.Equal(t, VerdictOfficialOpenAI, report.Verdict)
	require.NotEmpty(t, report.ReportID)
	require.Equal(t, 100, report.Score)
	require.Equal(t, 100, report.CompatibilityScore)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "tool_call").Status)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "streaming").Status)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "multimodal").Status)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "token_audit").Status)
	require.Len(t, report.Validations, 6)
	require.Equal(t, "llm_fingerprint", report.Validations[0].ID)
	require.Equal(t, CheckStatusPass, findValidation(t, report, "behavior").Status)
	require.Equal(t, "programmatic_probe", findValidation(t, report, "behavior").Details["detector"])
	require.NotNil(t, report.TokenAudit)
	require.Equal(t, tokenAuditSamples, report.TokenAudit.SampleCount)
	require.Len(t, report.TokenAudit.Samples, tokenAuditSamples)
	require.Len(t, report.TokenAudit.Rows, tokenAuditSamples)
	require.Equal(t, report.TokenAudit.ActualCostUSD, report.TokenAudit.TotalCostUSD)
	require.Equal(t, float64(1), report.TokenAudit.Multiplier)
	require.True(t, report.TokenAudit.PreviousChainOK)
	require.Equal(t, tokenAuditSamples-1, report.TokenAudit.StatefulRounds)
	require.NotZero(t, report.TokenAudit.CachedTokens)
	require.True(t, strings.HasPrefix(report.TokenAudit.PromptCacheKey, "proxyai_best_"))
	require.Empty(t, report.TokenAudit.Samples[0].PreviousResponseID)
	require.Equal(t, "resp_audit_1", report.TokenAudit.Samples[1].PreviousResponseID)
}

func TestTokenAuditPayloadsUseCumulativeCacheShape(t *testing.T) {
	auditNonce := "audit-test-nonce"
	roundOnePrompt := openAITokenAuditPrompt(1, auditNonce)
	roundElevenPrompt := openAITokenAuditPrompt(11, auditNonce)
	require.Contains(t, roundOnePrompt, "stable-cache-prefix")
	require.Contains(t, roundOnePrompt, auditNonce)
	require.Contains(t, roundOnePrompt, "round-cache-01")
	require.NotContains(t, roundOnePrompt, "round-cache-02")
	require.Contains(t, roundElevenPrompt, "round-cache-11")
	require.Greater(t, len(roundElevenPrompt), len(roundOnePrompt))

	promptCacheKey := openAITokenAuditPromptCacheKey("gpt-5.4", auditNonce)
	var openAIPayload map[string]any
	require.NoError(t, json.Unmarshal(responsesAuditProbePayload("gpt-5.4", 2, auditNonce, "resp_audit_1", promptCacheKey), &openAIPayload))
	require.Equal(t, "gpt-5.4", openAIPayload["model"])
	require.Equal(t, true, openAIPayload["store"])
	require.Equal(t, "resp_audit_1", openAIPayload["previous_response_id"])
	require.Equal(t, promptCacheKey, openAIPayload["prompt_cache_key"])
	require.Equal(t, "auto", openAIPayload["tool_choice"])
	require.Equal(t, true, openAIPayload["parallel_tool_calls"])
	require.Contains(t, openAIPayload["instructions"], "stable-cache-prefix")
	require.NotContains(t, openAIPayload["instructions"], "round-cache-02")
	tools, ok := openAIPayload["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	input, ok := openAIPayload["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	inputMessage, ok := input[0].(map[string]any)
	require.True(t, ok)
	content, ok := inputMessage["content"].([]any)
	require.True(t, ok)
	inputBlock, ok := content[0].(map[string]any)
	require.True(t, ok)
	require.Contains(t, inputBlock["text"], "round-cache-02")

	probeCtx := newClaudeProbeContext()
	history := []claudeAuditTurn{
		{Round: 1, UserText: claudeAuditUserText(1, auditNonce), AssistantText: strings.Repeat("x ", 20)},
		{Round: 2, UserText: claudeAuditUserText(2, auditNonce), AssistantText: strings.Repeat("y ", 10)},
	}
	var claudePayload map[string]any
	require.NoError(t, json.Unmarshal(claudeAuditProbePayload(defaultClaudeModel, 3, auditNonce, probeCtx, history), &claudePayload))
	systemBlocks, ok := claudePayload["system"].([]any)
	require.True(t, ok)
	require.Len(t, systemBlocks, 2)
	systemCacheControlled := 0
	for i, rawBlock := range systemBlocks {
		block, _ := rawBlock.(map[string]any)
		if _, ok := block["cache_control"].(map[string]any); ok {
			systemCacheControlled++
			require.Equal(t, 1, i)
		}
	}
	require.Equal(t, 1, systemCacheControlled)
	cachedSystemBlock, ok := systemBlocks[1].(map[string]any)
	require.True(t, ok)
	require.Contains(t, cachedSystemBlock["text"], "stable-cache-prefix")
	require.NotContains(t, cachedSystemBlock["text"], "round-cache-01")
	require.NotContains(t, cachedSystemBlock["text"], "round-cache-03")

	messages, ok := claudePayload["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 5)
	messageLevelCacheControlled := 0
	for i, rawMessage := range messages {
		message, ok := rawMessage.(map[string]any)
		require.True(t, ok)
		content, ok := message["content"].([]any)
		require.True(t, ok)
		require.NotEmpty(t, content)
		block, ok := content[len(content)-1].(map[string]any)
		require.True(t, ok)
		if _, ok := block["cache_control"].(map[string]any); ok {
			messageLevelCacheControlled++
			require.Equal(t, len(messages)-1, i)
		}
	}
	require.Equal(t, 1, messageLevelCacheControlled)
	message, ok := messages[0].(map[string]any)
	require.True(t, ok)
	content, ok = message["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 1)
	instructionBlock, ok := content[0].(map[string]any)
	require.True(t, ok)
	require.NotContains(t, instructionBlock, "cache_control")
	require.Contains(t, instructionBlock["text"], "round-cache-01")
	currentMessage, ok := messages[4].(map[string]any)
	require.True(t, ok)
	currentContent, ok := currentMessage["content"].([]any)
	require.True(t, ok)
	currentBlock, ok := currentContent[0].(map[string]any)
	require.True(t, ok)
	require.Contains(t, currentBlock["text"], "round-cache-03")
	require.Contains(t, currentBlock["text"], "Purity token audit response target.")

	metadata, ok := claudePayload["metadata"].(map[string]any)
	require.True(t, ok)
	userID, ok := metadata["user_id"].(string)
	require.True(t, ok)
	var parsedUserID map[string]string
	require.NoError(t, json.Unmarshal([]byte(userID), &parsedUserID))
	require.Len(t, parsedUserID["device_id"], 64)
	require.Equal(t, probeCtx.sessionID, parsedUserID["session_id"])
	require.Equal(t, "", parsedUserID["account_uuid"])
}

func TestOfficialPricingBaselines(t *testing.T) {
	openai := openAIModelPricingFor("gpt-5.4")
	require.Equal(t, 2.5e-6, openai.InputPerToken)
	require.Equal(t, 0.25e-6, openai.CacheReadPerToken)
	require.Equal(t, 15e-6, openai.OutputPerToken)
	require.Contains(t, openai.Source, "Official OpenAI API pricing")

	openAIMiniAlias := openAIModelPricingFor("gpt-5.4-mini-preview")
	require.Equal(t, 0.75e-6, openAIMiniAlias.InputPerToken)
	require.Equal(t, 0.075e-6, openAIMiniAlias.CacheReadPerToken)
	require.Equal(t, 4.5e-6, openAIMiniAlias.OutputPerToken)
	require.Contains(t, openAIMiniAlias.Source, "gpt-5.4-mini")

	openAIPro := openAIModelPricingFor("gpt-5.4-pro")
	require.Equal(t, 30e-6, openAIPro.InputPerToken)
	require.Equal(t, 30e-6, openAIPro.CacheReadPerToken)
	require.Equal(t, 180e-6, openAIPro.OutputPerToken)
	require.Contains(t, openAIPro.Source, "cached input not separately listed")

	openAICodexAlias := openAIModelPricingFor("gpt-5.3-codex-spark")
	require.Equal(t, 1.75e-6, openAICodexAlias.InputPerToken)
	require.Equal(t, 0.175e-6, openAICodexAlias.CacheReadPerToken)
	require.Equal(t, 14e-6, openAICodexAlias.OutputPerToken)
	require.Contains(t, openAICodexAlias.Source, "gpt-5.3-codex")

	openAIChatLatest := openAIModelPricingFor("chat-latest")
	require.Equal(t, 5e-6, openAIChatLatest.InputPerToken)
	require.Equal(t, 0.5e-6, openAIChatLatest.CacheReadPerToken)
	require.Equal(t, 30e-6, openAIChatLatest.OutputPerToken)

	openAILegacyCurrent := openAIModelPricingFor("gpt-5.2")
	require.Equal(t, 1.75e-6, openAILegacyCurrent.InputPerToken)
	require.Contains(t, openAILegacyCurrent.Source, "Official OpenAI API pricing")
	require.NotContains(t, openAILegacyCurrent.Source, "fallback")

	opus48 := claudeModelPricingFor("claude-opus-4-8")
	require.Equal(t, 5e-6, opus48.InputPerToken)
	require.Equal(t, 6.25e-6, opus48.CacheWritePerToken)
	require.Equal(t, 0.5e-6, opus48.CacheReadPerToken)
	require.Equal(t, 25e-6, opus48.OutputPerToken)
	require.Contains(t, opus48.Source, "Official Anthropic Claude pricing")
	require.Contains(t, opus48.Source, "5m cache writes")

	legacyOpus := claudeModelPricingFor("claude-opus-4-1")
	require.Equal(t, 15e-6, legacyOpus.InputPerToken)
	require.Equal(t, 75e-6, legacyOpus.OutputPerToken)
}

func TestClaudeTokenAuditUsesCacheAwareCCTestBaseline(t *testing.T) {
	pricing := claudeModelPricingFor("claude-opus-4-8")
	var totalBaseline float64
	for i := 1; i <= tokenAuditSamples; i++ {
		sample := TokenAuditSample{Index: i}
		applyClaudeTokenAuditBaseline(&sample, pricing)
		require.Greater(t, sample.OfficialBaselineUSD, float64(0))
		totalBaseline += sample.OfficialBaselineUSD
	}
	require.InDelta(t, 0.4628, roundMoney(totalBaseline), 0.0001)

	body, err := json.Marshal(map[string]any{
		"type":  "message",
		"model": "claude-opus-4-8",
		"content": []map[string]any{
			{"type": "text", "text": "ok"},
		},
		"usage": map[string]any{
			"input_tokens":                2,
			"output_tokens":               115,
			"cache_creation_input_tokens": 380,
			"cache_read_input_tokens":     23911,
		},
	})
	require.NoError(t, err)
	sample := claudeTokenAuditSampleFromProbe(2, httpProbe{StatusCode: http.StatusOK, Body: body, LatencyMS: 100}, pricing)
	require.Equal(t, CheckStatusPass, sample.Status)
	require.Equal(t, int64(1), sample.BaselineInputTokens)
	require.Equal(t, int64(162), sample.BaselineOutputTokens)
	require.Equal(t, int64(422), sample.BaselineCacheCreation)
	require.Equal(t, int64(23911), sample.BaselineCacheRead)
	require.InDelta(t, 0.92, sample.Multiplier, 0.01)
	require.InDelta(t, 0.92, sample.Ratio, 0.01)

	report := &TokenAuditReport{Samples: []TokenAuditSample{
		{Index: 1, Status: CheckStatusPass, InputTokens: 2, CacheCreationTokens: 100, CachedTokens: 0},
		{Index: 2, Status: CheckStatusPass, InputTokens: 2, CacheCreationTokens: 1, CachedTokens: 99},
		{Index: 3, Status: CheckStatusPass, InputTokens: 2, CacheCreationTokens: 1, CachedTokens: 99},
	}}
	applyClaudeWarmCacheHitRate(report)
	require.Equal(t, 0.97, report.CacheHitRate)
	require.Equal(t, float64(97), report.CacheHitRatePercent)
}

func TestPublicReportCCTestCompatFields(t *testing.T) {
	report := &PublicReport{
		Provider:         ProviderOpenAI,
		ReportID:         "report-compat",
		APIBaseHost:      "api.proxyai.best",
		ModelID:          "gpt-5.4",
		ExpectedModel:    "gpt-5.4",
		ResponseModel:    "gpt-5.4",
		Status:           RunStatusRunning,
		Step:             6,
		StepName:         "token_audit",
		Progress:         0.86,
		Verdict:          VerdictOpenAICompatible,
		StreamChannel:    "openai_compatible",
		NonStreamChannel: "openai_compatible",
		HasVertex:        true,
		IsKiro:           true,
		Checks: []CheckResult{
			passCheck("base_url", "API Base 域名", 20, "ok", nil),
			passCheck("responses_schema", "Responses 非流式结构", 20, "ok", nil),
			passCheck("tool_call", "强制工具调用", 20, "ok", nil),
			passCheck("usage", "Usage 计量", 10, "ok", nil),
			passCheck("streaming", "Responses 流式事件", 15, "ok", nil),
			passCheck("multimodal", "多模态输入", 10, "ok", nil),
			passCheck("token_audit", "Token 用量审计", 15, "ok", nil),
		},
		Metrics: PublicCheckMetrics{
			LatencyMS:            321,
			StreamFirstTokenMS:   45,
			StreamTotalLatencyMS: 180,
			ModelsLatencyMS:      12,
			ResponsesLatencyMS:   98,
			MultimodalLatencyMS:  110,
			TokensPerSecond:      18.75,
			Usage:                &TokenUsage{InputTokens: 1200, OutputTokens: 88, TotalTokens: 1288, CachedTokens: 1024},
		},
		CheckedAt: time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
	}
	finalizeReport(report)

	raw, err := json.Marshal(report)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, "token_audit", payload["stepName"])
	require.Equal(t, float64(100), payload["total"])
	require.Equal(t, "official", payload["verdictKey"])
	require.Equal(t, "gpt-5.4", payload["expectedModel"])
	require.Equal(t, "gpt-5.4", payload["responseModel"])
	require.Equal(t, "openai_compatible", payload["streamChannel"])
	require.Equal(t, "openai_compatible", payload["nonStreamChannel"])
	require.Equal(t, true, payload["hasVertex"])
	require.Equal(t, true, payload["isKiro"])

	metrics, ok := payload["metrics"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(321), metrics["latencyMs"])
	require.Equal(t, float64(45), metrics["ttfbMs"])
	require.Equal(t, 18.75, metrics["tokensPerSec"])
	require.Equal(t, float64(1200), metrics["inputTokens"])
	require.Equal(t, float64(88), metrics["outputTokens"])

	var emitted PublicCheckEvent
	emitPublicCheckEvent(func(event PublicCheckEvent) {
		emitted = event
	}, PublicCheckEvent{
		Type:     PublicCheckEventProgress,
		ReportID: report.ReportID,
		StepName: report.StepName,
		Metrics:  &report.Metrics,
		Report:   report,
	})
	eventRaw, err := json.Marshal(emitted)
	require.NoError(t, err)
	var eventPayload map[string]any
	require.NoError(t, json.Unmarshal(eventRaw, &eventPayload))
	require.Equal(t, "token_audit", eventPayload["stepName"])
	eventMetrics, ok := eventPayload["metrics"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(321), eventMetrics["latencyMs"])
	require.Equal(t, float64(45), eventMetrics["ttfbMs"])
	require.Equal(t, 18.75, eventMetrics["tokensPerSec"])
}

func TestServiceRunPublicCheckStream_EmitsProgressEvents(t *testing.T) {
	auditResponseIndex := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/v1/models":
			writeJSON(t, w, map[string]any{
				"object": "list",
				"data": []map[string]any{
					{"id": "gpt-5.4", "object": "model"},
				},
			})
		case "/v1/responses":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			if stream, _ := body["stream"].(bool); stream {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprintln(w, `data: {"type":"response.output_text.delta","delta":"ok"}`)
				_, _ = fmt.Fprintln(w)
				_, _ = fmt.Fprintln(w, `data: {"type":"response.completed","response":{"status":"completed"}}`)
				_, _ = fmt.Fprintln(w)
				_, _ = fmt.Fprintln(w, "data: [DONE]")
				return
			}
			if _, ok := body["prompt_cache_key"].(string); ok {
				writeOpenAITokenAuditTestResponse(t, w, body, &auditResponseIndex)
				return
			}
			writeJSON(t, w, map[string]any{
				"id":     "resp_1",
				"object": "response",
				"output": []map[string]any{
					{"type": "function_call", "name": "probe_ping", "arguments": `{"ok":true}`},
				},
				"usage": map[string]any{
					"input_tokens":  8,
					"output_tokens": 3,
					"total_tokens":  11,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewService(nil)
	service.httpClient = server.Client()
	service.allowPrivateHosts = true
	service.limiter = nil

	var events []PublicCheckEvent
	report, err := service.RunPublicCheckStream(context.Background(), PublicCheckInput{
		Provider:   ProviderOpenAI,
		APIBaseURL: server.URL,
		APIKey:     "sk-test",
		ModelID:    "gpt-5.4",
		ClientIP:   "203.0.113.10",
	}, func(event PublicCheckEvent) {
		events = append(events, event)
	})
	require.NoError(t, err)
	require.Equal(t, report.ReportID, events[0].ReportID)
	require.Equal(t, PublicCheckEventStarted, events[0].Type)
	require.Contains(t, eventTypes(events), PublicCheckEventCheck)
	require.Contains(t, eventTypes(events), PublicCheckEventValidation)
	require.Contains(t, eventTypes(events), PublicCheckEventTokenAuditSample)
	require.Equal(t, PublicCheckEventReport, events[len(events)-1].Type)
	require.NotNil(t, events[len(events)-1].Report)
	require.Len(t, events[len(events)-1].Report.Validations, 6)
	require.Equal(t, []string{
		"base_url",
		"models_schema",
		"responses_schema",
		"tool_call",
		"usage",
		"streaming",
		"multimodal",
		"token_audit",
	}, checkEventIDs(events))
	require.Equal(t, []string{
		"llm_fingerprint",
		"schema_integrity",
		"behavior",
		"signature",
		"multimodal",
		"token_audit",
	}, validationEventIDs(events))
	require.Contains(t, eventTypes(events), PublicCheckEventProgress)
	require.Contains(t, progressStepNames(events), "signature")
	for _, event := range events {
		if event.Type != PublicCheckEventValidation {
			continue
		}
		require.NotNil(t, event.Validation)
		require.Contains(t, []string{"programmatic_probe", "openai_base_url_and_models_probe"}, event.Validation.Details["detector"])
		require.NotEmpty(t, event.Validation.RelatedCheckIDs)
	}
}

func TestServiceRunPublicCheckStream_ClaudeCompatible(t *testing.T) {
	server := newClaudeCompatibleServer(t, "sk-claude-test")
	defer server.Close()

	service := NewService(nil)
	service.allowPrivateHosts = true
	service.limiter = nil

	var events []PublicCheckEvent
	report, err := service.RunPublicCheckStream(context.Background(), PublicCheckInput{
		Provider:   ProviderAnthropic,
		APIBaseURL: server.URL,
		APIKey:     "sk-claude-test",
		ModelID:    "claude-sonnet-4-6",
		ClientIP:   "203.0.113.10",
	}, func(event PublicCheckEvent) {
		events = append(events, event)
	})
	require.NoError(t, err)
	require.Equal(t, ProviderAnthropic, report.Provider)
	require.Equal(t, VerdictOfficialClaude, report.Verdict)
	require.Equal(t, 100, report.CompatibilityScore)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "claude_messages_schema").Status)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "claude_tool_use").Status)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "claude_streaming").Status)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "claude_multimodal").Status)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "token_audit").Status)
	require.Equal(t, CheckStatusPass, findValidation(t, report, "behavior").Status)
	require.Equal(t, CheckStatusPass, findValidation(t, report, "signature").Status)
	require.NotNil(t, report.TokenAudit)
	require.Equal(t, tokenAuditSamples, report.TokenAudit.SampleCount)
	require.Len(t, report.TokenAudit.Rows, tokenAuditSamples)
	require.Equal(t, report.TokenAudit.ActualCostUSD, report.TokenAudit.TotalCostUSD)
	require.Greater(t, report.TokenAudit.Samples[0].CacheCreationInputTokens, int64(0))
	require.Greater(t, report.TokenAudit.Samples[1].CacheReadInputTokens, int64(0))
	require.Contains(t, eventTypes(events), PublicCheckEventProgress)
	require.Contains(t, eventTypes(events), PublicCheckEventTokenAuditSample)
	require.Contains(t, progressStepNames(events), "signature")
	require.Equal(t, PublicCheckEventReport, events[len(events)-1].Type)
}

func TestServiceRunAccountCheckStream_InfersClaudeProvider(t *testing.T) {
	server := newClaudeCompatibleServer(t, "sk-claude-account")
	defer server.Close()

	service := NewServiceWithAccountResolver(nil, accountResolverStub{
		account: &coreservice.Account{
			ID:       84,
			Platform: coreservice.PlatformAnthropic,
			Type:     coreservice.AccountTypeAPIKey,
			Credentials: map[string]any{
				"api_key":  "sk-claude-account",
				"base_url": server.URL,
			},
		},
	})
	service.limiter = nil

	report, err := service.RunAccountCheckStream(context.Background(), AccountCheckInput{
		AccountID: 84,
		ModelID:   "claude-sonnet-4-6",
	}, nil)
	require.NoError(t, err)
	require.Equal(t, ProviderAnthropic, report.Provider)
	require.Equal(t, VerdictOfficialClaude, report.Verdict)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "claude_tool_use").Status)
}

func TestServiceRunAccountCheckStream_LoadsAccountCredentialAndEmitsProgress(t *testing.T) {
	auditResponseIndex := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer sk-account", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/v1/models":
			writeJSON(t, w, map[string]any{
				"object": "list",
				"data": []map[string]any{
					{"id": "gpt-5.4", "object": "model"},
				},
			})
		case "/v1/responses":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			if stream, _ := body["stream"].(bool); stream {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprintln(w, `data: {"type":"response.output_text.delta","delta":"ok"}`)
				_, _ = fmt.Fprintln(w)
				_, _ = fmt.Fprintln(w, `data: {"type":"response.completed","response":{"status":"completed"}}`)
				_, _ = fmt.Fprintln(w)
				_, _ = fmt.Fprintln(w, "data: [DONE]")
				return
			}
			if _, ok := body["prompt_cache_key"].(string); ok {
				writeOpenAITokenAuditTestResponse(t, w, body, &auditResponseIndex)
				return
			}
			writeJSON(t, w, map[string]any{
				"id":     "resp_1",
				"object": "response",
				"output": []map[string]any{
					{"type": "function_call", "name": "probe_ping", "arguments": `{"ok":true}`},
				},
				"usage": map[string]any{
					"input_tokens":  8,
					"output_tokens": 3,
					"total_tokens":  11,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewServiceWithAccountResolver(nil, accountResolverStub{
		account: &coreservice.Account{
			ID:       42,
			Platform: coreservice.PlatformOpenAI,
			Type:     coreservice.AccountTypeAPIKey,
			Credentials: map[string]any{
				"api_key":  "sk-account",
				"base_url": server.URL,
			},
		},
	})
	service.httpClient = server.Client()
	service.limiter = nil

	var events []PublicCheckEvent
	report, err := service.RunAccountCheckStream(context.Background(), AccountCheckInput{
		AccountID: 42,
		Provider:  ProviderOpenAI,
		ModelID:   "gpt-5.4",
	}, func(event PublicCheckEvent) {
		events = append(events, event)
	})
	require.NoError(t, err)
	require.Equal(t, VerdictOfficialOpenAI, report.Verdict)
	require.Contains(t, eventTypes(events), PublicCheckEventValidation)
	require.Contains(t, eventTypes(events), PublicCheckEventTokenAuditSample)
	require.Equal(t, PublicCheckEventReport, events[len(events)-1].Type)
	require.Equal(t, CheckStatusPass, findValidation(t, report, "behavior").Status)
	require.Equal(t, tokenAuditSamples, report.TokenAudit.SampleCount)
}

func TestServiceRunPublicCheck_FailsUnexpectedToolCall(t *testing.T) {
	auditResponseIndex := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/v1/models":
			writeJSON(t, w, map[string]any{
				"object": "list",
				"data": []map[string]any{
					{"id": "gpt-5.4", "object": "model"},
				},
			})
		case "/v1/responses":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			if stream, _ := body["stream"].(bool); stream {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprintln(w, `data: {"type":"response.output_text.delta","delta":"ok"}`)
				_, _ = fmt.Fprintln(w)
				_, _ = fmt.Fprintln(w, `data: {"type":"response.completed","response":{"status":"completed"}}`)
				_, _ = fmt.Fprintln(w)
				_, _ = fmt.Fprintln(w, "data: [DONE]")
				return
			}
			if _, ok := body["prompt_cache_key"].(string); ok {
				writeOpenAITokenAuditTestResponse(t, w, body, &auditResponseIndex)
				return
			}
			writeJSON(t, w, map[string]any{
				"id":     "resp_1",
				"object": "response",
				"output": []map[string]any{
					{"type": "function_call", "name": "wrong_tool", "arguments": `{"ok":true}`},
				},
				"usage": map[string]any{
					"input_tokens":  8,
					"output_tokens": 3,
					"total_tokens":  11,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewService(nil)
	service.httpClient = server.Client()
	service.allowPrivateHosts = true
	service.limiter = nil

	report, err := service.RunPublicCheck(context.Background(), PublicCheckInput{
		Provider:   ProviderOpenAI,
		APIBaseURL: server.URL,
		APIKey:     "sk-test",
		ModelID:    "gpt-5.4",
		ClientIP:   "203.0.113.10",
	})
	require.NoError(t, err)
	require.Equal(t, CheckStatusFail, findCheck(t, report, "tool_call").Status)
	require.Equal(t, CheckStatusFail, findValidation(t, report, "behavior").Status)
}

func TestServiceRunPublicCheck_BlocksPrivateBaseURL(t *testing.T) {
	service := NewService(nil)
	service.limiter = nil

	_, err := service.RunPublicCheck(context.Background(), PublicCheckInput{
		Provider:   ProviderOpenAI,
		APIBaseURL: "http://127.0.0.1:8080",
		APIKey:     "sk-test",
		ModelID:    "gpt-5.4",
		ClientIP:   "203.0.113.10",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "PURITY_BASE_URL_INVALID")
}

func TestServiceRunPublicCheck_RedactsAPIKeyFromReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(t, w, map[string]any{
			"error": map[string]any{"message": "bad key sk-secret-value"},
		})
	}))
	defer server.Close()

	service := NewService(nil)
	service.httpClient = server.Client()
	service.allowPrivateHosts = true
	service.limiter = nil

	report, err := service.RunPublicCheck(context.Background(), PublicCheckInput{
		Provider:   ProviderOpenAI,
		APIBaseURL: server.URL,
		APIKey:     "sk-secret-value",
		ModelID:    "gpt-5.4",
		ClientIP:   "203.0.113.10",
	})
	require.NoError(t, err)
	require.Equal(t, RunStatusError, report.Status)
	require.Contains(t, report.Error, "[redacted]")
	raw, err := json.Marshal(report)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "sk-secret-value")
	require.Contains(t, string(raw), "[redacted]")
}

func writeJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(payload))
}

func findCheck(t *testing.T, report *PublicReport, id string) CheckResult {
	t.Helper()
	for _, check := range report.Checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("check %s not found", id)
	return CheckResult{}
}

func findValidation(t *testing.T, report *PublicReport, id string) ValidationResult {
	t.Helper()
	for _, validation := range report.Validations {
		if validation.ID == id {
			return validation
		}
	}
	t.Fatalf("validation %s not found", id)
	return ValidationResult{}
}

func eventTypes(events []PublicCheckEvent) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		out = append(out, event.Type)
	}
	return out
}

func validationEventIDs(events []PublicCheckEvent) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		if event.Type == PublicCheckEventValidation && event.Validation != nil {
			out = append(out, event.Validation.ID)
		}
	}
	return out
}

func checkEventIDs(events []PublicCheckEvent) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		if event.Type == PublicCheckEventCheck && event.Check != nil {
			out = append(out, event.Check.ID)
		}
	}
	return out
}

func progressStepNames(events []PublicCheckEvent) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		if event.Type == PublicCheckEventProgress {
			out = append(out, event.StepName)
		}
	}
	return out
}

func newClaudeCompatibleServer(t *testing.T, expectedKey string) *httptest.Server {
	t.Helper()
	auditRound := 0
	sessionID := ""
	deviceID := ""
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/messages", r.URL.Path)
		require.Equal(t, expectedKey, r.Header.Get("x-api-key"))
		require.Equal(t, anthropicVersion, r.Header.Get("anthropic-version"))
		require.Equal(t, "cli", r.Header.Get("x-app"))
		require.Equal(t, claudeCodeProbeUserAgent, r.Header.Get("User-Agent"))
		require.Equal(t, claudeAPIKeyBetaHeader, r.Header.Get("anthropic-beta"))
		require.Equal(t, "js", r.Header.Get("X-Stainless-Lang"))
		require.Equal(t, "0.94.0", r.Header.Get("X-Stainless-Package-Version"))
		require.Equal(t, "Linux", r.Header.Get("X-Stainless-OS"))
		require.Equal(t, "arm64", r.Header.Get("X-Stainless-Arch"))
		require.Equal(t, "node", r.Header.Get("X-Stainless-Runtime"))
		require.Equal(t, "v24.3.0", r.Header.Get("X-Stainless-Runtime-Version"))
		require.Equal(t, "0", r.Header.Get("X-Stainless-Retry-Count"))
		require.Equal(t, "600", r.Header.Get("X-Stainless-Timeout"))
		require.Equal(t, "true", r.Header.Get("Anthropic-Dangerous-Direct-Browser-Access"))
		require.NotEmpty(t, r.Header.Get("x-client-request-id"))
		requestSessionID := r.Header.Get("X-Claude-Code-Session-Id")
		require.NotEmpty(t, requestSessionID)
		if sessionID == "" {
			sessionID = requestSessionID
		}
		require.Equal(t, sessionID, requestSessionID)

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		metadata, ok := body["metadata"].(map[string]any)
		require.True(t, ok)
		userID, ok := metadata["user_id"].(string)
		require.True(t, ok)
		var parsedUserID map[string]string
		require.NoError(t, json.Unmarshal([]byte(userID), &parsedUserID))
		require.Len(t, parsedUserID["device_id"], 64)
		if deviceID == "" {
			deviceID = parsedUserID["device_id"]
		}
		require.Equal(t, deviceID, parsedUserID["device_id"])
		require.Equal(t, "", parsedUserID["account_uuid"])
		require.Equal(t, sessionID, parsedUserID["session_id"])

		model, _ := body["model"].(string)
		if model == "" {
			model = defaultClaudeModel
		}
		if stream, _ := body["stream"].(bool); stream {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprintf(w, "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_stream\",\"type\":\"message\",\"role\":\"assistant\",\"model\":%q,\"content\":[]}}\n\n", model)
			_, _ = fmt.Fprintln(w, `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
			_, _ = fmt.Fprintln(w)
			_, _ = fmt.Fprintln(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}`)
			_, _ = fmt.Fprintln(w)
			_, _ = fmt.Fprintln(w, `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`)
			_, _ = fmt.Fprintln(w)
			_, _ = fmt.Fprintln(w, `data: {"type":"message_stop"}`)
			return
		}

		if tools, ok := body["tools"].([]any); ok && len(tools) > 0 {
			writeClaudeMessage(t, w, model, []map[string]any{
				{"type": "tool_use", "id": "toolu_1", "name": "probe_ping", "input": map[string]any{"ok": true}},
			}, map[string]any{
				"input_tokens":  18,
				"output_tokens": 4,
			})
			return
		}
		if claudeBodyContainsImage(body) {
			writeClaudeMessage(t, w, model, []map[string]any{
				{"type": "text", "text": "ok"},
			}, map[string]any{
				"input_tokens":  20,
				"output_tokens": 2,
			})
			return
		}

		auditRound++
		baselineUsage := claudeTokenAuditBaselineUsage(auditRound)
		require.NotNil(t, baselineUsage)
		writeClaudeMessage(t, w, model, []map[string]any{
			{"type": "text", "text": "ok"},
		}, map[string]any{
			"input_tokens":                baselineUsage.InputTokens,
			"output_tokens":               baselineUsage.OutputTokens,
			"cache_creation_input_tokens": baselineUsage.CacheCreationTokens,
			"cache_read_input_tokens":     baselineUsage.CachedTokens,
		})
	}))
}

func writeClaudeMessage(t *testing.T, w http.ResponseWriter, model string, content []map[string]any, usage map[string]any) {
	t.Helper()
	writeJSON(t, w, map[string]any{
		"id":            "msg_1",
		"type":          "message",
		"role":          "assistant",
		"model":         model,
		"content":       content,
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
		"usage":         usage,
	})
}

func claudeBodyContainsImage(body map[string]any) bool {
	messages, _ := body["messages"].([]any)
	for _, rawMessage := range messages {
		message, _ := rawMessage.(map[string]any)
		content, _ := message["content"].([]any)
		for _, rawBlock := range content {
			block, _ := rawBlock.(map[string]any)
			if block["type"] == "image" {
				return true
			}
		}
	}
	return false
}

func writeOpenAITokenAuditTestResponse(t *testing.T, w http.ResponseWriter, body map[string]any, responseIndex *int) {
	t.Helper()
	require.NotNil(t, responseIndex)
	*responseIndex++
	index := *responseIndex
	require.Equal(t, true, body["store"])
	require.Equal(t, "auto", body["tool_choice"])
	require.NotEmpty(t, body["prompt_cache_key"])
	tools, ok := body["tools"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, tools)
	if index == 1 {
		require.NotContains(t, body, "previous_response_id")
	} else {
		require.Equal(t, fmt.Sprintf("resp_audit_%d", index-1), body["previous_response_id"])
	}

	cachedTokens := 0
	if index > 1 {
		cachedTokens = 640
	}
	inputTokens := 1600 + index*13
	outputTokens := 24 + index
	writeJSON(t, w, map[string]any{
		"id":     fmt.Sprintf("resp_audit_%d", index),
		"object": "response",
		"output": []map[string]any{
			{"type": "message", "role": "assistant", "content": []map[string]any{{"type": "output_text", "text": "ok"}}},
		},
		"usage": map[string]any{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
			"input_tokens_details": map[string]any{
				"cached_tokens": cachedTokens,
			},
		},
	})
}

type accountResolverStub struct {
	account *coreservice.Account
	err     error
}

func (s accountResolverStub) GetByID(context.Context, int64) (*coreservice.Account, error) {
	return s.account, s.err
}
