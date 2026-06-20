package provider

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const defaultUserAgent = "sub2api-admin-plus-provider-adapter/0.1"

type SessionProfileClient struct {
	httpClient *http.Client
	now        func() time.Time
}

func NewSessionProfileClient(client *http.Client) *SessionProfileClient {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	return &SessionProfileClient{httpClient: client, now: time.Now}
}

func (c *SessionProfileClient) ProbeSub2APIUserProfile(ctx context.Context, in ports.SessionProbeInput) (*ports.SessionProbeResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "provider adapter is not configured")
	}
	apiBaseURL := firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url"), in.Origin)
	endpoint, err := buildSub2APIUserProfileURL(apiBaseURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applySessionHeaders(req, in.Bundle)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_SESSION_PROBE_FAILED", "failed to probe supplier session").WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, infraerrors.New(resp.StatusCode, "SUPPLIER_SESSION_PERMISSION_DENIED", "supplier session cannot access user profile")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_SESSION_PROBE_BAD_STATUS", "supplier profile endpoint returned non-success status")
	}
	profile, raw, err := parseSub2APIProfile(data)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_SESSION_PROFILE_INVALID", "supplier profile response is invalid").WithCause(err)
	}
	balanceCents := int64(math.Round(profile.Balance * 100))
	if balanceCents < 0 {
		balanceCents = 0
	}
	return &ports.SessionProbeResult{
		SupplierID: in.SupplierID,
		Status:     "valid",
		SystemType: "sub2api",
		Origin:     in.Origin,
		APIBaseURL: apiBaseURL,
		Capabilities: map[string]bool{
			"can_read_profile": true,
			"can_read_balance": true,
			"can_read_groups":  len(profile.AllowedGroups) > 0,
			"can_create_key":   false,
			"can_read_billing": false,
		},
		Profile:         profile,
		BalanceCents:    &balanceCents,
		BalanceCurrency: "USD",
		Diagnostics: map[string]any{
			"profile_endpoint": endpoint,
			"profile_keys":     rawKeys(raw),
		},
		ProbedAt: c.now().UTC(),
	}, nil
}

func buildSub2APIUserProfileURL(apiBaseURL string) (string, error) {
	return buildSub2APIUserEndpointURL(apiBaseURL, "/user/profile")
}

func buildSub2APIUserEndpointURL(apiBaseURL string, endpointPath string) (string, error) {
	u, err := parseSafeURL(apiBaseURL, "SUPPLIER_SESSION_API_BASE_URL_INVALID")
	if err != nil {
		return "", err
	}
	path := strings.TrimRight(u.Path, "/")
	if strings.HasSuffix(path, "/api/v1") {
		u.Path = path + endpointPath
	} else {
		u.Path = path + "/api/v1" + endpointPath
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func (c *SessionProfileClient) ReadGroups(ctx context.Context, in ports.SessionProbeInput) (*ports.ReadGroupsResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "provider adapter is not configured")
	}
	apiBaseURL := firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url"), in.Origin)
	availableEndpoint, err := buildSub2APIUserEndpointURL(apiBaseURL, "/groups/available")
	if err != nil {
		return nil, err
	}
	body, err := c.doSessionJSON(ctx, http.MethodGet, availableEndpoint, in.Bundle, true)
	if err != nil {
		return nil, err
	}
	groups, err := parseSub2APIGroups(body)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_GROUPS_RESPONSE_INVALID", "supplier groups response is invalid").WithCause(err)
	}

	ratesEndpoint, err := buildSub2APIUserEndpointURL(apiBaseURL, "/groups/rates")
	if err == nil {
		if ratesBody, ratesErr := c.doSessionJSON(ctx, http.MethodGet, ratesEndpoint, in.Bundle, false); ratesErr == nil {
			applyUserGroupRates(groups, parseSub2APIGroupRates(ratesBody))
		}
	}
	return &ports.ReadGroupsResult{
		SupplierID: in.SupplierID,
		SystemType: "sub2api",
		Origin:     in.Origin,
		APIBaseURL: apiBaseURL,
		Groups:     groups,
		CapturedAt: c.now().UTC(),
	}, nil
}

func (c *SessionProfileClient) doSessionJSON(ctx context.Context, method string, endpoint string, bundle map[string]any, strict bool) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applySessionHeaders(req, bundle)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_SESSION_REQUEST_FAILED", "failed to request supplier session endpoint").WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		if strict {
			return nil, infraerrors.New(resp.StatusCode, "SUPPLIER_SESSION_PERMISSION_DENIED", "supplier session cannot access requested endpoint")
		}
		return nil, infraerrors.New(resp.StatusCode, "SUPPLIER_SESSION_PERMISSION_DENIED", "supplier session cannot access requested endpoint")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if strict {
			return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_SESSION_BAD_STATUS", "supplier session endpoint returned non-success status")
		}
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_SESSION_BAD_STATUS", "supplier session endpoint returned non-success status")
	}
	return data, nil
}

func applySessionHeaders(req *http.Request, bundle map[string]any) {
	requiredHeaders := mapValue(bundle, "required_headers")
	accessToken := firstNonEmpty(
		stringValue(bundle, "access_token"),
		stringValue(bundle, "accessToken"),
		stringValueAt(bundle, "tokens", "access_token"),
		stringValueAt(bundle, "tokens", "accessToken"),
	)
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	if cookie := firstNonEmpty(stringValue(requiredHeaders, "cookie"), stringValue(bundle, "cookie"), cookieHeaderFromBundle(bundle)); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if origin := stringValue(requiredHeaders, "origin"); origin != "" {
		req.Header.Set("Origin", origin)
	}
	if referer := stringValue(requiredHeaders, "referer"); referer != "" {
		req.Header.Set("Referer", referer)
	}
	if csrf := firstNonEmpty(
		stringValue(bundle, "csrf_token"),
		stringValue(bundle, "csrfToken"),
		stringValueAt(bundle, "tokens", "csrf_token"),
		stringValueAt(bundle, "tokens", "csrfToken"),
	); csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
		req.Header.Set("X-XSRF-Token", csrf)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", defaultUserAgent)
}

func cookieHeaderFromBundle(bundle map[string]any) string {
	if raw := stringValue(bundle, "cookies"); raw != "" {
		return raw
	}
	items, ok := bundle["cookies"].([]any)
	if !ok || len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		cookie, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(stringFromAny(cookie["name"]))
		value := stringFromAny(cookie["value"])
		if name == "" || strings.ContainsAny(name, "=\r\n;") || strings.ContainsAny(value, "\r\n;") {
			continue
		}
		parts = append(parts, name+"="+value)
	}
	return strings.Join(parts, "; ")
}

func parseSub2APIProfile(data []byte) (*ports.UserProfileSnapshot, map[string]any, error) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, nil, err
	}
	body := root
	if dataValue, ok := root["data"].(map[string]any); ok {
		body = dataValue
	}
	profile := &ports.UserProfileSnapshot{
		ID:          int64FromAny(body["id"]),
		Email:       stringFromAny(body["email"]),
		Username:    stringFromAny(body["username"]),
		Role:        stringFromAny(body["role"]),
		Status:      stringFromAny(body["status"]),
		Balance:     float64FromAny(body["balance"]),
		Concurrency: int(int64FromAny(body["concurrency"])),
	}
	if groups, ok := body["allowed_groups"].([]any); ok {
		for _, raw := range groups {
			if id := int64FromAny(raw); id > 0 {
				profile.AllowedGroups = append(profile.AllowedGroups, id)
			}
		}
	}
	return profile, body, nil
}

func parseSub2APIGroups(data []byte) ([]*ports.ProviderGroup, error) {
	values, err := unwrapDataArray(data)
	if err != nil {
		return nil, err
	}
	groups := make([]*ports.ProviderGroup, 0, len(values))
	for _, value := range values {
		raw, ok := value.(map[string]any)
		if !ok {
			continue
		}
		externalID := firstNonEmpty(stringFromAny(raw["id"]), strconv.FormatInt(int64FromAny(raw["id"]), 10))
		if externalID == "0" || externalID == "" {
			externalID = firstNonEmpty(stringFromAny(raw["name"]), stringFromAny(raw["slug"]))
		}
		name := firstNonEmpty(stringFromAny(raw["name"]), externalID)
		if externalID == "" || name == "" {
			continue
		}
		rateMultiplier := float64FromAny(raw["rate_multiplier"])
		if rateMultiplier <= 0 {
			rateMultiplier = 1
		}
		status := normalizeSub2APIGroupStatus(stringFromAny(raw["status"]))
		group := &ports.ProviderGroup{
			ExternalGroupID:         externalID,
			Name:                    name,
			Description:             stringFromAny(raw["description"]),
			ProviderFamily:          normalizeProviderFamily(stringFromAny(raw["platform"])),
			RateMultiplier:          rateMultiplier,
			EffectiveRateMultiplier: rateMultiplier,
			RPMLimit:                optionalInt64(raw, "rpm_limit"),
			DailyLimitUSD:           optionalFloat64(raw, "daily_limit_usd"),
			WeeklyLimitUSD:          optionalFloat64(raw, "weekly_limit_usd"),
			MonthlyLimitUSD:         optionalFloat64(raw, "monthly_limit_usd"),
			AllowImageGeneration:    boolFromAny(raw["allow_image_generation"]),
			IsPrivate:               boolFromAny(raw["is_exclusive"]),
			Status:                  status,
			RawPayload:              raw,
		}
		groups = append(groups, group)
	}
	return groups, nil
}

func unwrapDataArray(data []byte) ([]any, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if arr, ok := root.([]any); ok {
		return arr, nil
	}
	if obj, ok := root.(map[string]any); ok {
		if arr, ok := obj["data"].([]any); ok {
			return arr, nil
		}
	}
	return []any{}, nil
}

func parseSub2APIGroupRates(data []byte) map[string]float64 {
	if len(data) == 0 {
		return nil
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}
	body := root
	if obj, ok := root.(map[string]any); ok {
		if dataValue, exists := obj["data"]; exists {
			body = dataValue
		}
	}
	obj, ok := body.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]float64, len(obj))
	for key, value := range obj {
		rate := float64FromAny(value)
		if rate >= 0 {
			out[strings.TrimSpace(key)] = rate
		}
	}
	return out
}

func applyUserGroupRates(groups []*ports.ProviderGroup, rates map[string]float64) {
	if len(groups) == 0 || len(rates) == 0 {
		return
	}
	for _, group := range groups {
		if group == nil {
			continue
		}
		rate, ok := rates[group.ExternalGroupID]
		if !ok {
			continue
		}
		value := rate
		group.UserRateMultiplier = &value
		group.EffectiveRateMultiplier = value
	}
}

func parseSafeURL(raw string, reason string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, infraerrors.New(http.StatusBadRequest, reason, "invalid supplier session url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, infraerrors.New(http.StatusBadRequest, reason, "supplier session url must use http or https")
	}
	if u.User != nil {
		return nil, infraerrors.New(http.StatusBadRequest, reason, "supplier session url must not contain user info")
	}
	return u, nil
}

func stringValue(in map[string]any, key string) string {
	if in == nil {
		return ""
	}
	return stringFromAny(in[key])
}

func mapValue(in map[string]any, key string) map[string]any {
	if in == nil {
		return nil
	}
	if value, ok := in[key].(map[string]any); ok {
		return value
	}
	return nil
}

func stringValueAt(in map[string]any, path ...string) string {
	var current any = in
	for _, key := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = obj[key]
	}
	return stringFromAny(current)
}

func stringFromAny(value any) string {
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
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
	default:
		return 0
	}
}

func float64FromAny(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	default:
		return 0
	}
}

func boolFromAny(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		parsed, _ := strconv.ParseBool(strings.TrimSpace(v))
		return parsed
	default:
		return false
	}
}

func optionalInt64(raw map[string]any, key string) *int64 {
	value, exists := raw[key]
	if !exists || value == nil {
		return nil
	}
	n := int64FromAny(value)
	out := n
	return &out
}

func optionalFloat64(raw map[string]any, key string) *float64 {
	value, exists := raw[key]
	if !exists || value == nil {
		return nil
	}
	n := float64FromAny(value)
	out := n
	return &out
}

func normalizeProviderFamily(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return "mixed"
	}
	if len(v) > 60 {
		return v[:60]
	}
	return v
}

func normalizeSub2APIGroupStatus(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "", "active", "enabled":
		return "active"
	case "missing":
		return "missing"
	default:
		return "disabled"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func rawKeys(in map[string]any) []string {
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	return keys
}
