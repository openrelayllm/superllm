package purity

import (
	"context"
	"encoding/json"
	"fmt"
	coreservice "github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
			if openAIStoreIncludeProbeRequest(body) {
				writeOpenAITextResponse(t, w, "resp_store_include", "gpt-5.4", "ok")
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
	require.Len(t, events[len(events)-1].Report.Validations, 8)
	require.Equal(t, []string{
		"base_url",
		"models_schema",
		"responses_schema",
		"tool_call",
		"responses_structured_output",
		"usage",
		"streaming",
		"responses_store_include",
		"multimodal",
		"model_identity",
		"wrapper_fingerprint",
		"token_audit",
	}, checkEventIDs(events))
	require.Equal(t, []string{
		"llm_fingerprint",
		"schema_integrity",
		"behavior",
		"signature",
		"multimodal",
		"model_identity",
		"wrapper_fingerprint",
		"token_audit",
	}, validationEventIDs(events))
	require.Contains(t, eventTypes(events), PublicCheckEventProgress)
	require.Contains(t, progressStepNames(events), "signature")
	for _, event := range events {
		if event.Type != PublicCheckEventValidation {
			continue
		}
		require.NotNil(t, event.Validation)
		require.Contains(t, []string{"programmatic_probe", "openai_base_url_and_models_probe", "channel_signal_detectors"}, event.Validation.Details["detector"])
		require.NotEmpty(t, event.Validation.RelatedCheckIDs)
	}
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
	require.Equal(t, AccessModeAccount, report.AccessMode)
	require.Equal(t, BillingModeAccountInternal, report.BillingMode)
	require.Equal(t, ProviderAnthropic, report.Provider)
	require.Equal(t, VerdictOfficialClaude, report.Verdict)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "claude_tool_use").Status)
}
func TestServiceRunAccountCheckStream_InfersGeminiProvider(t *testing.T) {
	server := newGeminiCompatibleServer(t, "AIza-account", "gemini-3-pro-preview")
	defer server.Close()

	service := NewServiceWithAccountResolver(nil, accountResolverStub{
		account: &coreservice.Account{
			ID:       85,
			Platform: coreservice.PlatformGemini,
			Type:     coreservice.AccountTypeAPIKey,
			Credentials: map[string]any{
				"api_key":  "AIza-account",
				"base_url": server.URL,
			},
		},
	})
	service.httpClient = server.Client()
	service.allowPrivateHosts = true
	service.limiter = nil

	report, err := service.RunAccountCheckStream(context.Background(), AccountCheckInput{
		AccountID: 85,
		ModelID:   "gemini-3-pro-preview",
	}, nil)
	require.NoError(t, err)
	require.Equal(t, AccessModeAccount, report.AccessMode)
	require.Equal(t, BillingModeAccountInternal, report.BillingMode)
	require.Equal(t, ProviderGemini, report.Provider)
	require.NotEqual(t, VerdictInvalidOrUnavailable, report.Verdict)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "tool_call").Status)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "multimodal").Status)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "token_audit").Status)
	require.NotNil(t, report.TokenAudit)
	require.True(t, report.TokenAudit.HistoryReplayOK)
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
			if openAIStoreIncludeProbeRequest(body) {
				writeOpenAITextResponse(t, w, "resp_store_include", "gpt-5.4", "ok")
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
	require.Equal(t, AccessModeAccount, report.AccessMode)
	require.Equal(t, AccessModeAccount, report.AccessModeCompat)
	require.Equal(t, BillingModeAccountInternal, report.BillingMode)
	require.Equal(t, BillingModeAccountInternal, report.BillingModeCompat)
	require.Equal(t, VerdictOfficialOpenAI, report.Verdict)
	require.Contains(t, eventTypes(events), PublicCheckEventValidation)
	require.Contains(t, eventTypes(events), PublicCheckEventTokenAuditSample)
	require.Equal(t, PublicCheckEventReport, events[len(events)-1].Type)
	require.Equal(t, CheckStatusPass, findValidation(t, report, "behavior").Status)
	require.Equal(t, tokenAuditSamples, report.TokenAudit.SampleCount)
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

type accountResolverStub struct {
	account *coreservice.Account
	err     error
}

func (s accountResolverStub) GetByID(context.Context, int64) (*coreservice.Account, error) {
	return s.account, s.err
}
