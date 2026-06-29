package purity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceRunPublicCheck_OpenAIStructuredOutputProbePasses(t *testing.T) {
	auditResponseIndex := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			writeJSON(t, w, map[string]any{
				"object": "list",
				"data":   []map[string]any{{"id": "gpt-5.4", "object": "model"}},
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
				"output": []map[string]any{{"type": "function_call", "name": "probe_ping", "arguments": `{"ok":true}`}},
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

	report, err := service.RunDeveloperAPICheck(context.Background(), PublicCheckInput{
		Provider:       ProviderOpenAI,
		APIBaseURL:     server.URL,
		APIKey:         "sk-test",
		ModelID:        "gpt-5.4",
		ClientIP:       "203.0.113.10",
		SkipTokenAudit: true,
	})
	require.NoError(t, err)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "responses_structured_output").Status)
	require.Equal(t, CheckStatusPass, findValidation(t, report, "schema_integrity").Status)
}

func TestServiceRunPublicCheck_OpenAIChatFallbackHonorsChoiceCount(t *testing.T) {
	choiceCounts := map[int]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			writeJSON(t, w, map[string]any{
				"object": "list",
				"data":   []map[string]any{{"id": "gpt-5.4", "object": "model"}},
			})
		case "/v1/responses":
			w.WriteHeader(http.StatusNotFound)
			writeJSON(t, w, map[string]any{"error": map[string]any{"message": "responses unsupported"}})
		case "/v1/chat/completions":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			choiceCount := chatCompletionsChoiceCount(body)
			choiceCounts[choiceCount]++
			switch choiceCount {
			case 2:
				writeJSON(t, w, map[string]any{
					"id":     "chatcmpl-test",
					"object": "chat.completion",
					"model":  "gpt-5.4",
					"choices": []map[string]any{
						{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"},
						{"index": 1, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"},
					},
					"usage": map[string]any{
						"prompt_tokens":     11,
						"completion_tokens": 4,
						"total_tokens":      15,
					},
				})
			case 1:
				writeJSON(t, w, map[string]any{
					"id":     "chatcmpl-test",
					"object": "chat.completion",
					"model":  "gpt-5.4",
					"choices": []map[string]any{
						{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"},
					},
					"usage": map[string]any{
						"prompt_tokens":     11,
						"completion_tokens": 2,
						"total_tokens":      13,
					},
				})
			default:
				t.Fatalf("unexpected chat completions choice count: %d", choiceCount)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewService(nil)
	service.httpClient = server.Client()
	service.allowPrivateHosts = true
	service.limiter = nil

	report, err := service.RunDeveloperAPICheck(context.Background(), PublicCheckInput{
		Provider:   ProviderOpenAI,
		APIBaseURL: server.URL,
		APIKey:     "sk-chat-only",
		ModelID:    "gpt-5.4",
		ClientIP:   "203.0.113.10",
	})
	require.NoError(t, err)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "chat_completions").Status)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "chat_completions_n").Status)
	require.Equal(t, 1, choiceCounts[2])
	require.GreaterOrEqual(t, choiceCounts[1], 1)
}
