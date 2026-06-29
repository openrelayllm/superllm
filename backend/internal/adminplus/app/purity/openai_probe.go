package purity

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"strings"
	"time"
)

func (s *Service) probeModels(ctx context.Context, client *http.Client, baseURL string, apiKey string) httpProbe {
	return s.doJSON(ctx, client, http.MethodGet, buildOpenAIEndpointURL(baseURL, "/v1/models"), apiKey, nil, "application/json")
}

func (s *Service) probeResponsesNonStream(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string) httpProbe {
	return s.doJSON(ctx, client, http.MethodPost, buildOpenAIEndpointURL(baseURL, "/v1/responses"), apiKey, responsesToolProbePayload(model, false), "application/json")
}

func (s *Service) probeResponsesStructuredOutput(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string) httpProbe {
	return s.doJSON(ctx, client, http.MethodPost, buildOpenAIEndpointURL(baseURL, "/v1/responses"), apiKey, responsesStructuredOutputPayload(model), "application/json")
}

func (s *Service) probeResponsesStoreInclude(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string) httpProbe {
	return s.doJSON(ctx, client, http.MethodPost, buildOpenAIEndpointURL(baseURL, "/v1/responses"), apiKey, responsesStoreIncludePayload(model), "application/json")
}

func (s *Service) probeChatCompletions(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string) httpProbe {
	return s.probeChatCompletionsWithPrompt(ctx, client, baseURL, apiKey, model, "Return exactly: ok", 2)
}

func (s *Service) probeChatCompletionsWithPrompt(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string, prompt string, choiceCount int) httpProbe {
	if strings.TrimSpace(prompt) == "" {
		prompt = "Return exactly: ok"
	}
	bodyMap := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream": false,
	}
	if choiceCount > 0 {
		bodyMap["n"] = choiceCount
	}
	body, _ := json.Marshal(bodyMap)
	return s.doJSON(ctx, client, http.MethodPost, buildOpenAIEndpointURL(baseURL, "/v1/chat/completions"), apiKey, body, "application/json")
}

func (s *Service) probeResponsesMultimodal(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string) httpProbe {
	return s.doJSON(ctx, client, http.MethodPost, buildOpenAIEndpointURL(baseURL, "/v1/responses"), apiKey, responsesMultimodalProbePayload(model), "application/json")
}

type streamProbe struct {
	StatusCode     int
	Headers        map[string]string
	FirstTokenMS   int64
	TotalLatencyMS int64
	SeenData       bool
	SeenDelta      bool
	SeenCompleted  bool
	SeenDone       bool
	ErrorClass     string
	ErrorMessage   string
}

func (s *Service) probeResponsesStream(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string) streamProbe {
	started := s.currentTime()
	body := responsesStreamProbePayload(model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildOpenAIEndpointURL(baseURL, "/v1/responses"), bytes.NewReader(body))
	if err != nil {
		return streamProbe{ErrorClass: "request_build_error", ErrorMessage: sanitizeMessage(err.Error(), apiKey)}
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if client == nil {
		client = s.clientForRun(checkRunOptions{})
	}
	resp, err := client.Do(req)
	if err != nil {
		return streamProbe{
			TotalLatencyMS: int64(s.currentTime().Sub(started) / time.Millisecond),
			ErrorClass:     "network_error",
			ErrorMessage:   sanitizeMessage(err.Error(), apiKey),
		}
	}
	defer func() { _ = resp.Body.Close() }()

	result := streamProbe{StatusCode: resp.StatusCode, Headers: selectedResponseHeaders(resp.Header)}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, maxProbeBodyBytes))
		result.TotalLatencyMS = int64(s.currentTime().Sub(started) / time.Millisecond)
		errorMessage := upstreamErrorMessage(bodyBytes)
		result.ErrorClass = errorClassForStatusAndMessage(resp.StatusCode, errorMessage)
		result.ErrorMessage = sanitizeMessage(errorMessage, apiKey)
		return result
	}
	readResponsesStream(resp.Body, started, s.currentTime, &result, apiKey)
	result.TotalLatencyMS = int64(s.currentTime().Sub(started) / time.Millisecond)
	return result
}

func readResponsesStream(body io.Reader, started time.Time, now func() time.Time, result *streamProbe, apiKey string) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		result.SeenData = true
		if data == "[DONE]" {
			result.SeenDone = true
			continue
		}
		eventType := strings.TrimSpace(gjson.Get(data, "type").String())
		switch eventType {
		case "response.output_text.delta":
			if strings.TrimSpace(gjson.Get(data, "delta").String()) != "" {
				result.SeenDelta = true
				if result.FirstTokenMS == 0 {
					result.FirstTokenMS = int64(now().Sub(started) / time.Millisecond)
				}
			}
		case "response.completed", "response.done":
			result.SeenCompleted = true
		case "response.failed":
			result.ErrorClass = "response_failed"
			result.ErrorMessage = sanitizeMessage(upstreamErrorMessage([]byte(data)), apiKey)
		}
		if !result.SeenDelta && strings.TrimSpace(gjson.Get(data, "response.output.0.content.0.text").String()) != "" {
			result.SeenDelta = true
			if result.FirstTokenMS == 0 {
				result.FirstTokenMS = int64(now().Sub(started) / time.Millisecond)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		result.ErrorClass = "stream_error"
		result.ErrorMessage = sanitizeMessage(err.Error(), apiKey)
	}
}

func responsesToolProbePayload(model string, stream bool) []byte {
	if strings.TrimSpace(model) == "" {
		model = openai.DefaultTestModel
	}
	body, _ := json.Marshal(map[string]any{
		"model": model,
		"input": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Call the probe_ping function with ok=true to acknowledge readiness. You must use the tool."},
				},
			},
		},
		"tools": []map[string]any{
			{
				"type":        "function",
				"name":        "probe_ping",
				"description": "Capability probe. Call to acknowledge.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"ok": map[string]any{"type": "boolean"},
					},
					"required": []string{"ok"},
				},
			},
		},
		"tool_choice":       "required",
		"max_output_tokens": 512,
		"stream":            stream,
	})
	return body
}

func responsesStoreIncludePayload(model string) []byte {
	if strings.TrimSpace(model) == "" {
		model = openai.DefaultTestModel
	}
	body, _ := json.Marshal(map[string]any{
		"model": model,
		"input": "Return exactly: ok",
		"reasoning": map[string]any{
			"effort": "minimal",
		},
		"include": []string{
			"reasoning.encrypted_content",
		},
		"store":             false,
		"max_output_tokens": 16,
		"stream":            false,
	})
	return body
}

func responsesStructuredOutputPayload(model string) []byte {
	if strings.TrimSpace(model) == "" {
		model = openai.DefaultTestModel
	}
	body, _ := json.Marshal(map[string]any{
		"model": model,
		"input": "Jane, 54 years old",
		"text": map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"name":   "person",
				"strict": true,
				"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{"type": "string"},
						"age":  map[string]any{"type": "number"},
					},
					"required":             []string{"name", "age"},
					"additionalProperties": false,
				},
			},
		},
		"max_output_tokens": 64,
		"stream":            false,
	})
	return body
}

func responsesStreamProbePayload(model string) []byte {
	if strings.TrimSpace(model) == "" {
		model = openai.DefaultTestModel
	}
	body, _ := json.Marshal(map[string]any{
		"model":             model,
		"input":             "Return exactly: ok",
		"max_output_tokens": 16,
		"stream":            true,
	})
	return body
}

func responsesMultimodalProbePayload(model string) []byte {
	if strings.TrimSpace(model) == "" {
		model = openai.DefaultTestModel
	}
	body, _ := json.Marshal(map[string]any{
		"model": model,
		"input": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Read the attached image and return exactly: ok"},
					{"type": "input_image", "image_url": probePNGData},
				},
			},
		},
		"max_output_tokens": 16,
		"stream":            false,
	})
	return body
}

func parseResponsesUsage(body []byte) *TokenUsage {
	usage := gjson.GetBytes(body, "usage")
	if !usage.Exists() || !usage.IsObject() {
		return nil
	}
	return &TokenUsage{
		InputTokens:               usage.Get("input_tokens").Int(),
		OutputTokens:              usage.Get("output_tokens").Int(),
		TotalTokens:               usage.Get("total_tokens").Int(),
		CacheCreationTokens:       firstUsageInt(usage, "input_tokens_details.cache_creation_tokens", "input_tokens_details.cache_creation_input_tokens", "prompt_tokens_details.cache_creation_tokens", "cache_creation_input_tokens", "cache_creation_tokens"),
		CachedTokens:              firstUsageInt(usage, "input_tokens_details.cached_tokens", "prompt_tokens_details.cached_tokens"),
		CachedTokensFieldPresent:  usagePathExists(usage, "input_tokens_details.cached_tokens", "prompt_tokens_details.cached_tokens"),
		CacheCreationFieldPresent: usagePathExists(usage, "input_tokens_details.cache_creation_tokens", "input_tokens_details.cache_creation_input_tokens", "prompt_tokens_details.cache_creation_tokens", "cache_creation_input_tokens", "cache_creation_tokens"),
		ReasoningTokens:           usage.Get("output_tokens_details.reasoning_tokens").Int(),
	}
}

func parseChatCompletionsUsage(body []byte) *TokenUsage {
	usage := gjson.GetBytes(body, "usage")
	if !usage.Exists() || !usage.IsObject() {
		return nil
	}
	promptTokens := usage.Get("prompt_tokens").Int()
	completionTokens := usage.Get("completion_tokens").Int()
	totalTokens := usage.Get("total_tokens").Int()
	if totalTokens == 0 {
		totalTokens = promptTokens + completionTokens
	}
	return &TokenUsage{
		InputTokens:               promptTokens,
		OutputTokens:              completionTokens,
		TotalTokens:               totalTokens,
		CacheCreationTokens:       firstUsageInt(usage, "prompt_tokens_details.cache_creation_tokens", "prompt_tokens_details.cache_creation_input_tokens", "cache_creation_input_tokens", "cache_creation_tokens"),
		CachedTokens:              firstUsageInt(usage, "prompt_tokens_details.cached_tokens"),
		CachedTokensFieldPresent:  usagePathExists(usage, "prompt_tokens_details.cached_tokens"),
		CacheCreationFieldPresent: usagePathExists(usage, "prompt_tokens_details.cache_creation_tokens", "prompt_tokens_details.cache_creation_input_tokens", "cache_creation_input_tokens", "cache_creation_tokens"),
		ReasoningTokens:           firstUsageInt(usage, "completion_tokens_details.reasoning_tokens"),
	}
}

func openAIResponseModelFromProbe(probe httpProbe) (string, string, string, string) {
	headerModel := strings.TrimSpace(probe.Headers["openai-model"])
	bodyModel := strings.TrimSpace(gjson.GetBytes(probe.Body, "model").String())
	if headerModel != "" && normalizeModelName(headerModel) != normalizeModelName(bodyModel) {
		return headerModel, "openai-model", bodyModel, headerModel
	}
	if bodyModel != "" {
		return bodyModel, "body.model", bodyModel, headerModel
	}
	if headerModel != "" {
		return headerModel, "openai-model", bodyModel, headerModel
	}
	return "", "", "", ""
}

func firstUsageInt(usage gjson.Result, paths ...string) int64 {
	for _, path := range paths {
		if value := usage.Get(path); value.Exists() {
			return value.Int()
		}
	}
	return 0
}

func firstPositiveUsageInt(usage gjson.Result, paths ...string) int64 {
	for _, path := range paths {
		if value := usage.Get(path); value.Exists() && value.Int() > 0 {
			return value.Int()
		}
	}
	return firstUsageInt(usage, paths...)
}

func usagePathExists(usage gjson.Result, paths ...string) bool {
	for _, path := range paths {
		if usage.Get(path).Exists() {
			return true
		}
	}
	return false
}

func shouldStopAfterProbe(probe httpProbe) bool {
	if probe.StatusCode == http.StatusUnauthorized || probe.StatusCode == http.StatusForbidden {
		return true
	}
	return probe.StatusCode == 0 && probe.ErrorClass == "network_error"
}

func needsChatFallback(checks []CheckResult) bool {
	for _, check := range checks {
		if check.ID == "responses_schema" || check.ID == "tool_call" || check.ID == "streaming" {
			if check.Status != CheckStatusPass {
				return true
			}
		}
	}
	return false
}
