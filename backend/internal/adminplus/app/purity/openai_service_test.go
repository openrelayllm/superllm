package purity

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServiceRunPublicCheck_OpenAIPureBehindCustomBaseURL(t *testing.T) {
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
			if openAIStoreIncludeProbeRequest(body) {
				writeOpenAITextResponse(t, w, "resp_store_include", "gpt-5.4", "ok")
				return
			}
			if openAIStructuredOutputProbeRequest(body) {
				writeOpenAITextResponse(t, w, "resp_structured_output", "gpt-5.4", `{"name":"Jane","age":54}`)
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
	require.Equal(t, AccessModeWeb, report.AccessMode)
	require.Equal(t, AccessModeWeb, report.AccessModeCompat)
	require.Equal(t, BillingModeCaptchaRateLimit, report.BillingMode)
	require.Equal(t, BillingModeCaptchaRateLimit, report.BillingModeCompat)
	require.Equal(t, VerdictOfficialOpenAI, report.Verdict)
	require.NotEmpty(t, report.ReportID)
	require.Equal(t, 100, report.Score)
	require.Equal(t, 100, report.CompatibilityScore)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "tool_call").Status)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "responses_structured_output").Status)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "streaming").Status)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "multimodal").Status)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "token_audit").Status)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "wrapper_fingerprint").Status)
	require.Len(t, report.Validations, 8)
	require.Equal(t, "llm_fingerprint", report.Validations[0].ID)
	require.Equal(t, CheckStatusPass, findValidation(t, report, "behavior").Status)
	require.Equal(t, CheckStatusWarn, findValidation(t, report, "model_identity").Status)
	require.Equal(t, "programmatic_probe", findValidation(t, report, "behavior").Details["detector"])
	require.NotNil(t, report.TokenAudit)
	require.Equal(t, tokenAuditSamples, report.TokenAudit.SampleCount)
	require.Len(t, report.TokenAudit.Samples, tokenAuditSamples)
	require.Len(t, report.TokenAudit.Rows, tokenAuditSamples)
	require.Equal(t, report.TokenAudit.ActualCostUSD, report.TokenAudit.TotalCostUSD)
	require.Less(t, report.TokenAudit.Multiplier, float64(1))
	require.NotZero(t, report.TokenAudit.CacheCreationTokens)
	require.Equal(t, int64(640), report.TokenAudit.Samples[0].CacheCreationTokens)
	require.Equal(t, int64(640), report.TokenAudit.Samples[0].CacheCreationInputTokens)
	require.True(t, report.TokenAudit.PreviousChainOK)
	require.True(t, report.TokenAudit.ContextReplayOK)
	require.Equal(t, tokenAuditSamples-openAITokenAuditCacheProbeRounds, report.TokenAudit.ContextReplayRounds)
	require.Equal(t, tokenAuditSamples-openAITokenAuditCacheProbeRounds-1, report.TokenAudit.ContextReplayLinks)
	require.Equal(t, tokenAuditSamples-openAITokenAuditCacheProbeRounds-1, report.TokenAudit.ContextReplayLinksExpected)
	require.Equal(t, report.TokenAudit.ContextReplayLinks, report.TokenAudit.StatefulRounds)
	require.Equal(t, openAITokenAuditCacheProbeRounds, report.TokenAudit.CacheProbeRounds)
	require.Equal(t, openAITokenAuditCacheProbeRounds-1, report.TokenAudit.CacheProbeHits)
	require.True(t, report.TokenAudit.CachedTokensFieldObserved)
	require.NotZero(t, report.TokenAudit.CachedTokens)
	require.True(t, strings.HasPrefix(report.TokenAudit.PromptCacheKey, "proxyai_best_"))
	require.Empty(t, report.TokenAudit.Samples[0].PreviousResponseID)
	require.Equal(t, openAITokenAuditModeCacheProbe, report.TokenAudit.Samples[1].RequestMode)
	require.Equal(t, openAITokenAuditModeContextReplay, report.TokenAudit.Samples[openAITokenAuditCacheProbeRounds].RequestMode)
	require.Empty(t, report.TokenAudit.Samples[openAITokenAuditCacheProbeRounds].PreviousResponseID)
	require.Empty(t, report.TokenAudit.Samples[openAITokenAuditCacheProbeRounds+1].PreviousResponseID)

	developerReport, err := service.RunDeveloperAPICheck(context.Background(), PublicCheckInput{
		Provider:       ProviderOpenAI,
		APIBaseURL:     server.URL,
		APIKey:         "sk-test",
		ModelID:        "gpt-5.4",
		ClientIP:       "203.0.113.10",
		SkipTokenAudit: true,
	})
	require.NoError(t, err)
	require.Equal(t, AccessModeDeveloperAPI, developerReport.AccessMode)
	require.Equal(t, AccessModeDeveloperAPI, developerReport.AccessModeCompat)
	require.Equal(t, BillingModeAPIKeyMetered, developerReport.BillingMode)
	require.Equal(t, BillingModeAPIKeyMetered, developerReport.BillingModeCompat)
}
