package provider

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const newAPIConsumeLogType = "2"

func (c *Client) ReadUsageCosts(ctx context.Context, in ports.SessionProbeInput, request ports.ReadUsageCostsInput) (*ports.ReadUsageCostsResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "new api provider adapter is not configured")
	}
	if request.SupplierID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if request.StartedAt.IsZero() || request.EndedAt.IsZero() || !request.StartedAt.Before(request.EndedAt) {
		return nil, infraerrors.New(http.StatusBadRequest, "SUPPLIER_USAGE_COST_TIME_RANGE_INVALID", "invalid supplier usage cost time range")
	}
	apiBaseURL, origin, err := normalizeBaseURLs(firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url")), firstNonEmpty(in.Origin, stringValue(in.Bundle, "origin")))
	if err != nil {
		return nil, err
	}
	role, roleKnown := newAPIRoleFromBundle(in.Bundle)
	baseEndpoint, err := buildEndpointURL(apiBaseURL, "/api/log")
	if err != nil {
		return nil, err
	}
	lines := make([]ports.ProviderUsageCostLine, 0)
	for page := 1; page <= newAPIHistoryMaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		endpoint := appendNewAPIQueryValues(baseEndpoint, map[string]string{
			"p":               strconv.Itoa(page),
			"page_size":       strconv.Itoa(newAPIPageSize),
			"type":            newAPIConsumeLogType,
			"start_timestamp": strconv.FormatInt(request.StartedAt.UTC().Unix(), 10),
			"end_timestamp":   strconv.FormatInt(request.EndedAt.UTC().Unix(), 10),
		})
		raw, err := c.doSessionJSON(ctx, http.MethodGet, endpoint, in.Bundle)
		if err != nil {
			if isNewAPISessionPermissionError(err) {
				return nil, newAPISessionPermissionRequired(role, roleKnown, err)
			}
			return nil, err
		}
		envelope, err := decodeEnvelope(raw)
		if err != nil {
			return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_USAGE_COST_RESPONSE_INVALID", "new api usage log response is invalid").WithCause(err)
		}
		if !envelope.Success {
			err := classifySessionBusinessFailure(envelope.Message)
			if isNewAPISessionPermissionError(err) {
				return nil, newAPISessionPermissionRequired(role, roleKnown, err)
			}
			return nil, err
		}
		pageLines := parseNewAPIUsageCostLines(envelope.Data)
		if len(pageLines) == 0 {
			break
		}
		lines = append(lines, pageLines...)
		total := int64FromAny(envelope.Data["total"])
		if total > 0 {
			if len(lines) >= int(total) {
				break
			}
		}
		if total == 0 && len(pageLines) < newAPIPageSize {
			break
		}
		if page == newAPIHistoryMaxPages {
			return nil, newAPIPageLimitExceeded("usage_cost_lines", newAPIHistoryMaxPages, newAPIPageSize)
		}
		if err := waitForProviderPage(ctx, newAPIPageDelay); err != nil {
			return nil, err
		}
	}
	return &ports.ReadUsageCostsResult{
		SupplierID: in.SupplierID,
		SystemType: "new_api",
		Origin:     origin,
		APIBaseURL: apiBaseURL,
		Lines:      lines,
		CapturedAt: c.now().UTC(),
	}, nil
}

func parseNewAPIUsageCostLines(data map[string]any) []ports.ProviderUsageCostLine {
	values, ok := data["items"].([]any)
	if !ok {
		return nil
	}
	lines := make([]ports.ProviderUsageCostLine, 0, len(values))
	for _, value := range values {
		raw, ok := value.(map[string]any)
		if !ok {
			continue
		}
		line, ok := parseNewAPIUsageCostLine(raw)
		if ok {
			lines = append(lines, line)
		}
	}
	return lines
}

func parseNewAPIUsageCostLine(raw map[string]any) (ports.ProviderUsageCostLine, bool) {
	model := firstNonEmpty(
		stringFromAny(raw["model_name"]),
		stringFromAny(raw["modelName"]),
		stringFromAny(raw["model"]),
	)
	if model == "" {
		return ports.ProviderUsageCostLine{}, false
	}
	startedAt := unixTimePtr(raw["created_at"])
	if startedAt == nil {
		startedAt = unixTimePtr(raw["createdAt"])
	}
	if startedAt == nil {
		return ports.ProviderUsageCostLine{}, false
	}
	durationMS := newAPIUsageDurationMS(raw)
	var endedAt *time.Time
	if durationMS > 0 {
		endedAtValue := startedAt.Add(time.Duration(durationMS) * time.Millisecond).UTC()
		endedAt = &endedAtValue
	}
	return ports.ProviderUsageCostLine{
		ExternalUsageCostID: firstNonEmpty(
			stringFromAny(raw["external_usage_cost_id"]),
			stringFromAny(raw["externalUsageCostId"]),
			newAPIIDString(raw["id"]),
		),
		ExternalRequestID: firstNonEmpty(
			stringFromAny(raw["request_id"]),
			stringFromAny(raw["requestId"]),
			stringFromAny(raw["upstream_request_id"]),
			stringFromAny(raw["upstreamRequestId"]),
		),
		APIKeyName:      firstNonEmpty(stringFromAny(raw["token_name"]), stringFromAny(raw["tokenName"]), stringFromAny(raw["api_key_name"]), stringFromAny(raw["apiKeyName"])),
		Model:           model,
		Endpoint:        firstNonEmpty(stringFromAny(raw["endpoint"]), stringFromAny(raw["path"]), stringFromAny(raw["request_path"]), stringFromAny(raw["requestPath"])),
		RequestType:     "consume",
		BillingMode:     "new_api_quota",
		ReasoningEffort: firstNonEmpty(stringFromAny(raw["group"]), stringFromAny(raw["reasoning_effort"]), stringFromAny(raw["reasoningEffort"])),
		Currency:        "USD",
		CostCents:       newAPIQuotaToUSDCents(float64FromAny(raw["quota"])),
		InputTokens:     int64FromAny(raw["prompt_tokens"]),
		OutputTokens:    int64FromAny(raw["completion_tokens"]),
		TotalTokens:     newAPIUsageTotalTokens(raw),
		DurationMS:      durationMS,
		StartedAt:       startedAt.UTC(),
		EndedAt:         endedAt,
		RawPayload:      sanitizeNewAPIPayload(raw),
	}, true
}

func newAPIUsageTotalTokens(raw map[string]any) int64 {
	if tokens := int64FromAny(raw["total_tokens"]); tokens > 0 {
		return tokens
	}
	if tokens := int64FromAny(raw["totalTokens"]); tokens > 0 {
		return tokens
	}
	return int64FromAny(raw["prompt_tokens"]) + int64FromAny(raw["completion_tokens"])
}

func newAPIUsageDurationMS(raw map[string]any) int64 {
	useTime := int64FromAny(raw["use_time"])
	if useTime == 0 {
		useTime = int64FromAny(raw["useTime"])
	}
	if useTime <= 0 {
		return 0
	}
	if useTime < 1000 {
		return useTime * 1000
	}
	return useTime
}
