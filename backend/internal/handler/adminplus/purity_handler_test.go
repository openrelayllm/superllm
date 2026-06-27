package adminplus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	purityapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/purity"
	coreservice "github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPurityHandlerAccountCheckStreamUsesStoredAccountCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auditResponseIndex := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer sk-account-handler", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/v1/models":
			writePurityHandlerJSON(t, w, map[string]any{
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
				writePurityHandlerOpenAITokenAuditResponse(t, w, body, &auditResponseIndex)
				return
			}
			writePurityHandlerJSON(t, w, map[string]any{
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
	defer upstream.Close()

	service := purityapp.NewServiceWithAccountResolver(nil, purityHandlerAccountResolverStub{
		account: &coreservice.Account{
			ID:       42,
			Platform: coreservice.PlatformOpenAI,
			Type:     coreservice.AccountTypeAPIKey,
			Credentials: map[string]any{
				"api_key":  "sk-account-handler",
				"base_url": upstream.URL,
			},
		},
	})
	handler := NewPurityHandler(service, nil)
	router := gin.New()
	router.POST("/accounts/:accountID/purity/checks/stream", handler.AccountCheckStream)

	req := httptest.NewRequest(http.MethodPost, "/accounts/42/purity/checks/stream", strings.NewReader(`{"provider":"openai","model_id":"gpt-5.4"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/x-ndjson")
	require.Contains(t, w.Body.String(), `"type":"validation"`)
	require.Contains(t, w.Body.String(), `"type":"token_audit_sample"`)
	require.Contains(t, w.Body.String(), `"type":"report"`)
	require.NotContains(t, w.Body.String(), "sk-account-handler")
}

type purityHandlerAccountResolverStub struct {
	account *coreservice.Account
	err     error
}

func (s purityHandlerAccountResolverStub) GetByID(context.Context, int64) (*coreservice.Account, error) {
	return s.account, s.err
}

func writePurityHandlerJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(payload))
}

func writePurityHandlerOpenAITokenAuditResponse(t *testing.T, w http.ResponseWriter, body map[string]any, responseIndex *int) {
	t.Helper()
	require.NotNil(t, responseIndex)
	*responseIndex++
	index := *responseIndex
	require.Equal(t, true, body["store"])
	require.NotEmpty(t, body["prompt_cache_key"])
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
	writePurityHandlerJSON(t, w, map[string]any{
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
