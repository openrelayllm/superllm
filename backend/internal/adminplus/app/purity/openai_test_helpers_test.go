package purity

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func newOpenAIStoreIncludeServer(t *testing.T, storeIncludeStatus int, storeIncludeMessage string, responseHeaders map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer sk-store-include", r.Header.Get("Authorization"))
		for key, value := range responseHeaders {
			w.Header().Set(key, value)
		}
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
			if openAIStoreIncludeProbeRequest(body) {
				if storeIncludeStatus >= 200 && storeIncludeStatus < 300 {
					writeOpenAITextResponse(t, w, "resp_store_include", "gpt-5.4", "ok")
					return
				}
				w.WriteHeader(storeIncludeStatus)
				writeJSON(t, w, map[string]any{"error": map[string]any{"message": storeIncludeMessage}})
				return
			}
			if payloadHasInputImage(body) {
				writeOpenAITextResponse(t, w, "resp_multimodal", "gpt-5.4", "ok")
				return
			}
			if openAIStructuredOutputProbeRequest(body) {
				writeOpenAITextResponse(t, w, "resp_structured_output", "gpt-5.4", `{"name":"Jane","age":54}`)
				return
			}
			if choiceCount := chatCompletionsChoiceCount(body); choiceCount > 0 {
				writeJSON(t, w, map[string]any{
					"id":      "chatcmpl-test",
					"object":  "chat.completion",
					"model":   "gpt-5.4",
					"choices": buildChatCompletionChoices(choiceCount),
					"usage": map[string]any{
						"prompt_tokens":     11,
						"completion_tokens": 2 * choiceCount,
						"total_tokens":      11 + 2*choiceCount,
					},
				})
				return
			}
			writeJSON(t, w, map[string]any{
				"id":     "resp_1",
				"object": "response",
				"model":  "gpt-5.4",
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
}

func openAIStructuredOutputProbeRequest(payload map[string]any) bool {
	text, _ := payload["text"].(map[string]any)
	if text == nil {
		return false
	}
	format, _ := text["format"].(map[string]any)
	if format == nil {
		return false
	}
	if kind, _ := format["type"].(string); kind != "json_schema" {
		return false
	}
	if strict, _ := format["strict"].(bool); !strict {
		return false
	}
	schema, _ := format["schema"].(map[string]any)
	return schema != nil
}

func chatCompletionsChoiceCount(payload map[string]any) int {
	switch value := payload["n"].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func buildChatCompletionChoices(count int) []map[string]any {
	choices := make([]map[string]any, 0, count)
	for i := 0; i < count; i++ {
		choices = append(choices, map[string]any{
			"index": i,
			"message": map[string]any{
				"role":    "assistant",
				"content": "ok",
			},
			"finish_reason": "stop",
		})
	}
	return choices
}
