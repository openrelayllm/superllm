package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (c *Client) ProbeSub2APIUserProfile(ctx context.Context, in ports.SessionProbeInput) (*ports.SessionProbeResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "new api provider adapter is not configured")
	}
	apiBaseURL, origin, err := normalizeBaseURLs(firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url")), firstNonEmpty(in.Origin, stringValue(in.Bundle, "origin")))
	if err != nil {
		return nil, err
	}
	endpoint, err := buildEndpointURL(apiBaseURL, "/api/user/self")
	if err != nil {
		return nil, err
	}
	raw, err := c.doSessionJSON(ctx, http.MethodGet, endpoint, in.Bundle)
	if err != nil {
		return nil, err
	}
	envelope, err := decodeEnvelope(raw)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_SESSION_PROFILE_INVALID", "new api profile response is invalid").WithCause(err)
	}
	if !envelope.Success {
		return nil, classifySessionBusinessFailure(envelope.Message)
	}
	profile, rawQuota, rawUsedQuota, requestCount := parseProfile(envelope.Data)
	if profile.ID <= 0 {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_SESSION_PROFILE_INVALID", "new api profile response did not include user id")
	}
	balanceCents := int64(math.Round(rawQuota * 100))
	if balanceCents < 0 {
		balanceCents = 0
	}
	return &ports.SessionProbeResult{
		SupplierID: in.SupplierID,
		Status:     "valid",
		SystemType: "new_api",
		Origin:     origin,
		APIBaseURL: apiBaseURL,
		Capabilities: map[string]bool{
			"can_read_profile": true,
			"can_read_balance": true,
			"can_read_groups":  true,
			"can_create_key":   true,
		},
		Profile:         profile,
		BalanceCents:    &balanceCents,
		BalanceCurrency: "QTA",
		Diagnostics: map[string]any{
			"profile_endpoint": endpoint,
			"profile_keys":     rawKeys(envelope.Data),
			"raw_quota":        rawQuota,
			"raw_used_quota":   rawUsedQuota,
			"request_count":    requestCount,
		},
		ProbedAt: c.now().UTC(),
	}, nil
}

type apiEnvelope struct {
	Success bool
	Message string
	Data    map[string]any
	Raw     map[string]any
}

func decodeEnvelope(data []byte) (*apiEnvelope, error) {
	var root map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		return nil, err
	}
	body := map[string]any{}
	if value, ok := root["data"].(map[string]any); ok {
		body = value
	}
	success, _ := boolValue(root["success"])
	return &apiEnvelope{
		Success: success,
		Message: stringFromAny(root["message"]),
		Data:    body,
		Raw:     root,
	}, nil
}

func parseProfile(data map[string]any) (*ports.UserProfileSnapshot, float64, float64, int64) {
	rawQuota := float64FromAny(data["quota"])
	rawUsedQuota := float64FromAny(data["used_quota"])
	requestCount := int64FromAny(data["request_count"])
	role := firstNonEmpty(stringFromAny(data["role"]), strconv.FormatInt(int64FromAny(data["role"]), 10))
	status := normalizeStatus(data["status"])
	return &ports.UserProfileSnapshot{
		ID:       int64FromAny(data["id"]),
		Username: stringFromAny(data["username"]),
		Role:     role,
		Status:   status,
		Balance:  rawQuota,
	}, rawQuota, rawUsedQuota, requestCount
}

func normalizeStatus(value any) string {
	if s := stringFromAny(value); s != "" {
		return s
	}
	switch int64FromAny(value) {
	case 1:
		return "enabled"
	case 2:
		return "disabled"
	default:
		return strconv.FormatInt(int64FromAny(value), 10)
	}
}

func stringFromProbeStatus(probe *ports.SessionProbeResult) string {
	if probe == nil {
		return ""
	}
	return probe.Status
}

func rawKeys(in map[string]any) []string {
	if len(in) == 0 {
		return nil
	}
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	return keys
}

func boolFromAny(value any) bool {
	ok, _ := boolValue(value)
	return ok
}

func boolValue(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		return parsed, err == nil
	default:
		return false, false
	}
}

func int64FromAny(value any) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return n
	default:
		return 0
	}
}

func float64FromAny(value any) float64 {
	n, _ := float64Value(value)
	return n
}

func float64Value(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, false
		}
		n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return n, err == nil
	default:
		return 0, false
	}
}
