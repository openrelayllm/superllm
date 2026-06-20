package provider

import (
	"bytes"
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

type rateEndpointCandidate struct {
	path   string
	parser func([]byte) []ports.ProviderRateEntry
}

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

func (c *SessionProfileClient) CreateKey(ctx context.Context, in ports.SessionProbeInput, request ports.CreateProviderKeyInput) (*ports.ProviderKeyResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "provider adapter is not configured")
	}
	apiBaseURL := firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url"), in.Origin)
	endpoint, err := buildSub2APIUserEndpointURL(apiBaseURL, "/api-keys")
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"name": strings.TrimSpace(request.Name),
	}
	if groupID, ok := parsePositiveInt64(request.ExternalGroupID); ok {
		payload["group_id"] = groupID
	} else if strings.TrimSpace(request.ExternalGroupID) != "" {
		payload["group_id"] = strings.TrimSpace(request.ExternalGroupID)
	}
	if request.QuotaUSD > 0 {
		payload["quota"] = request.QuotaUSD
	}
	if request.ExpiresInDays != nil && *request.ExpiresInDays > 0 {
		payload["expires_in_days"] = *request.ExpiresInDays
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	respBody, err := c.doSessionJSONBody(ctx, http.MethodPost, endpoint, in.Bundle, body, true)
	if err != nil {
		return nil, err
	}
	result, err := parseSub2APIKeyCreateResponse(respBody)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_KEY_RESPONSE_INVALID", "supplier key response is invalid").WithCause(err)
	}
	result.SupplierID = request.SupplierID
	result.ExternalGroupID = firstNonEmpty(result.ExternalGroupID, request.ExternalGroupID)
	result.Name = firstNonEmpty(result.Name, request.Name)
	if result.CreatedAt.IsZero() {
		result.CreatedAt = c.now().UTC()
	}
	return result, nil
}

func (c *SessionProfileClient) ReadRates(ctx context.Context, in ports.SessionProbeInput) (*ports.ReadRatesResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "provider adapter is not configured")
	}
	apiBaseURL := firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url"), in.Origin)
	candidates := []rateEndpointCandidate{
		{path: "/rates/snapshots", parser: parseRateEntriesResponse},
		{path: "/channels/available", parser: parseAvailableChannelRates},
	}
	var lastErr error
	for _, candidate := range candidates {
		endpoint, err := buildSub2APIUserEndpointURL(apiBaseURL, candidate.path)
		if err != nil {
			return nil, err
		}
		body, err := c.doSessionJSON(ctx, http.MethodGet, endpoint, in.Bundle, false)
		if err != nil {
			lastErr = err
			continue
		}
		entries := candidate.parser(body)
		if len(entries) == 0 {
			continue
		}
		return &ports.ReadRatesResult{
			SupplierID: in.SupplierID,
			SystemType: "sub2api",
			Origin:     in.Origin,
			APIBaseURL: apiBaseURL,
			Entries:    entries,
			CapturedAt: c.now().UTC(),
		}, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_RATE_CAPABILITY_MISSING", "supplier session cannot read model rates")
}

func (c *SessionProfileClient) doSessionJSON(ctx context.Context, method string, endpoint string, bundle map[string]any, strict bool) ([]byte, error) {
	return c.doSessionJSONBody(ctx, method, endpoint, bundle, nil, strict)
}

func (c *SessionProfileClient) doSessionJSONBody(ctx context.Context, method string, endpoint string, bundle map[string]any, body []byte, strict bool) ([]byte, error) {
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	applySessionHeaders(req, bundle)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
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

func parseSub2APIKeyCreateResponse(data []byte) (*ports.ProviderKeyResult, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	body := root
	if obj, ok := root.(map[string]any); ok {
		if dataValue, exists := obj["data"]; exists {
			body = dataValue
		}
	}
	raw, ok := body.(map[string]any)
	if !ok {
		return &ports.ProviderKeyResult{RawPayload: map[string]any{}}, nil
	}
	secret := firstNonEmpty(
		stringFromAny(raw["key"]),
		stringFromAny(raw["api_key"]),
		stringFromAny(raw["apiKey"]),
		stringFromAny(raw["token"]),
		stringFromAny(raw["secret"]),
	)
	result := &ports.ProviderKeyResult{
		ExternalKeyID:   firstNonEmpty(stringFromAny(raw["id"]), strconv.FormatInt(int64FromAny(raw["id"]), 10)),
		ExternalGroupID: firstNonEmpty(stringFromAny(raw["group_id"]), strconv.FormatInt(int64FromAny(raw["group_id"]), 10)),
		Name:            stringFromAny(raw["name"]),
		Secret:          secret,
		Status:          firstNonEmpty(stringFromAny(raw["status"]), "active"),
		RawPayload:      sanitizeProviderKeyPayload(raw),
	}
	if result.ExternalKeyID == "0" {
		result.ExternalKeyID = ""
	}
	if result.ExternalGroupID == "0" {
		result.ExternalGroupID = ""
	}
	return result, nil
}

func sanitizeProviderKeyPayload(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "key", "api_key", "apikey", "token", "secret":
			continue
		default:
			out[key] = value
		}
	}
	return out
}

func parseRateEntriesResponse(data []byte) []ports.ProviderRateEntry {
	values, err := unwrapDataArray(data)
	if err != nil {
		return nil
	}
	entries := make([]ports.ProviderRateEntry, 0, len(values))
	for _, value := range values {
		raw, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if entry, ok := parseProviderRateEntry(raw); ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

func parseAvailableChannelRates(data []byte) []ports.ProviderRateEntry {
	values, err := unwrapDataArray(data)
	if err != nil {
		return nil
	}
	entries := make([]ports.ProviderRateEntry, 0)
	for _, value := range values {
		channel, ok := value.(map[string]any)
		if !ok {
			continue
		}
		entries = append(entries, parseSupportedModelRates(channel)...)
		entries = append(entries, parseModelPricingRates(channel)...)
	}
	return dedupeRateEntries(entries)
}

func parseSupportedModelRates(channel map[string]any) []ports.ProviderRateEntry {
	values, ok := firstArrayValue(channel, "supported_models", "supportedModels", "SupportedModels")
	if !ok {
		return nil
	}
	out := make([]ports.ProviderRateEntry, 0, len(values)*4)
	for _, value := range values {
		model, ok := value.(map[string]any)
		if !ok {
			continue
		}
		modelName := firstNonEmpty(stringFromAny(model["name"]), stringFromAny(model["model"]), stringFromAny(model["Name"]), stringFromAny(model["Model"]))
		pricing := firstMapValue(model, "pricing", "Pricing")
		if len(pricing) == 0 {
			continue
		}
		out = append(out, priceEntriesFromPricing(modelName, pricing, mergeRawPayload(channel, model, pricing))...)
	}
	return out
}

func parseModelPricingRates(channel map[string]any) []ports.ProviderRateEntry {
	values, ok := firstArrayValue(channel, "model_pricing", "modelPricing", "ModelPricing", "pricing", "Pricing")
	if !ok {
		return nil
	}
	out := make([]ports.ProviderRateEntry, 0, len(values)*4)
	for _, value := range values {
		pricing, ok := value.(map[string]any)
		if !ok {
			continue
		}
		models := modelNamesFromPricing(pricing)
		for _, model := range models {
			out = append(out, priceEntriesFromPricing(model, pricing, mergeRawPayload(channel, pricing))...)
		}
	}
	return out
}

func parseProviderRateEntry(raw map[string]any) (ports.ProviderRateEntry, bool) {
	model := firstNonEmpty(stringFromAny(raw["model"]), stringFromAny(raw["name"]), stringFromAny(raw["Model"]), stringFromAny(raw["Name"]))
	if model == "" {
		return ports.ProviderRateEntry{}, false
	}
	price, ok := directMicrosFromAny(firstExisting(raw, "price_micros", "priceMicros", "PriceMicros"))
	if !ok {
		return ports.ProviderRateEntry{}, false
	}
	entry := ports.ProviderRateEntry{
		Model:       model,
		BillingMode: firstNonEmpty(stringFromAny(raw["billing_mode"]), stringFromAny(raw["billingMode"]), stringFromAny(raw["BillingMode"]), "token"),
		PriceItem:   firstNonEmpty(stringFromAny(raw["price_item"]), stringFromAny(raw["priceItem"]), stringFromAny(raw["PriceItem"]), "input"),
		Unit:        firstNonEmpty(stringFromAny(raw["unit"]), stringFromAny(raw["Unit"]), "1m_tokens"),
		Currency:    firstNonEmpty(stringFromAny(raw["currency"]), stringFromAny(raw["Currency"]), "USD"),
		PriceMicros: price,
		RawPayload:  raw,
	}
	if entry.PriceMicros < 0 {
		return ports.ProviderRateEntry{}, false
	}
	return entry, true
}

func priceEntriesFromPricing(model string, pricing map[string]any, raw map[string]any) []ports.ProviderRateEntry {
	model = strings.TrimSpace(model)
	if model == "" || len(pricing) == 0 {
		return nil
	}
	billingMode := firstNonEmpty(stringFromAny(pricing["billing_mode"]), stringFromAny(pricing["billingMode"]), stringFromAny(pricing["BillingMode"]), "token")
	currency := firstNonEmpty(stringFromAny(pricing["currency"]), stringFromAny(pricing["Currency"]), "USD")
	out := make([]ports.ProviderRateEntry, 0, 5)
	addMicros := func(item string, unit string, rawValue any) {
		price, ok := directMicrosFromAny(rawValue)
		if !ok {
			return
		}
		out = append(out, ports.ProviderRateEntry{
			Model:       model,
			BillingMode: billingMode,
			PriceItem:   item,
			Unit:        unit,
			Currency:    currency,
			PriceMicros: price,
			RawPayload:  raw,
		})
	}
	addTokenUSD := func(item string, rawValue any) {
		price, ok := usdPriceToMicros(rawValue, 1_000_000)
		if !ok {
			return
		}
		out = append(out, ports.ProviderRateEntry{
			Model:       model,
			BillingMode: billingMode,
			PriceItem:   item,
			Unit:        "1m_tokens",
			Currency:    currency,
			PriceMicros: price,
			RawPayload:  raw,
		})
	}
	addRequestUSD := func(rawValue any) {
		price, ok := usdPriceToMicros(rawValue, 1)
		if !ok {
			return
		}
		out = append(out, ports.ProviderRateEntry{
			Model:       model,
			BillingMode: billingMode,
			PriceItem:   "per_request",
			Unit:        "request",
			Currency:    currency,
			PriceMicros: price,
			RawPayload:  raw,
		})
	}
	addMicros("input", "1m_tokens", firstExisting(pricing, "input_price_micros", "inputPriceMicros", "InputPriceMicros"))
	addMicros("output", "1m_tokens", firstExisting(pricing, "output_price_micros", "outputPriceMicros", "OutputPriceMicros"))
	addMicros("cache_write", "1m_tokens", firstExisting(pricing, "cache_write_price_micros", "cacheWritePriceMicros", "CacheWritePriceMicros"))
	addMicros("cache_read", "1m_tokens", firstExisting(pricing, "cache_read_price_micros", "cacheReadPriceMicros", "CacheReadPriceMicros"))
	addMicros("image_output", "1m_tokens", firstExisting(pricing, "image_output_price_micros", "imageOutputPriceMicros", "ImageOutputPriceMicros"))
	addMicros("per_request", "request", firstExisting(pricing, "per_request_price_micros", "perRequestPriceMicros", "PerRequestPriceMicros"))

	addTokenUSD("input", firstExisting(pricing, "input_price", "inputPrice", "InputPrice"))
	addTokenUSD("output", firstExisting(pricing, "output_price", "outputPrice", "OutputPrice"))
	addTokenUSD("cache_write", firstExisting(pricing, "cache_write_price", "cacheWritePrice", "CacheWritePrice"))
	addTokenUSD("cache_read", firstExisting(pricing, "cache_read_price", "cacheReadPrice", "CacheReadPrice"))
	addTokenUSD("image_output", firstExisting(pricing, "image_output_price", "imageOutputPrice", "ImageOutputPrice"))
	addRequestUSD(firstExisting(pricing, "per_request_price", "perRequestPrice", "PerRequestPrice"))
	return out
}

func directMicrosFromAny(value any) (int64, bool) {
	if value == nil {
		return 0, false
	}
	switch v := value.(type) {
	case int:
		if v < 0 {
			return 0, false
		}
		return int64(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return v, true
	case float64:
		if v < 0 {
			return 0, false
		}
		return int64(math.Round(v)), true
	case json.Number:
		if n, err := v.Int64(); err == nil && n >= 0 {
			return n, true
		}
		f, err := v.Float64()
		if err != nil || f < 0 {
			return 0, false
		}
		return int64(math.Round(f)), true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(trimmed, 64)
		if err != nil || f < 0 {
			return 0, false
		}
		return int64(math.Round(f)), true
	default:
		return 0, false
	}
}

func usdPriceToMicros(value any, unitMultiplier float64) (int64, bool) {
	usd, ok := float64Value(value)
	if !ok || usd < 0 {
		return 0, false
	}
	return int64(math.Round(usd * unitMultiplier * 1_000_000)), true
}

func float64Value(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func firstExisting(in map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, exists := in[key]; exists && value != nil {
			return value
		}
	}
	return nil
}

func firstMapValue(in map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		value, ok := in[key].(map[string]any)
		if ok {
			return value
		}
	}
	return nil
}

func firstArrayValue(in map[string]any, keys ...string) ([]any, bool) {
	for _, key := range keys {
		values, ok := in[key].([]any)
		if ok {
			return values, true
		}
	}
	return nil, false
}

func modelNamesFromPricing(pricing map[string]any) []string {
	if values, ok := firstArrayValue(pricing, "models", "Models"); ok {
		out := make([]string, 0, len(values))
		for _, value := range values {
			if model := stringFromAny(value); model != "" && !strings.Contains(model, "*") {
				out = append(out, model)
			}
		}
		return out
	}
	model := firstNonEmpty(stringFromAny(pricing["model"]), stringFromAny(pricing["name"]), stringFromAny(pricing["Model"]), stringFromAny(pricing["Name"]))
	if model == "" || strings.Contains(model, "*") {
		return nil
	}
	return []string{model}
}

func mergeRawPayload(values ...map[string]any) map[string]any {
	out := make(map[string]any, len(values))
	for idx, value := range values {
		for key, raw := range value {
			out[key] = raw
		}
		if len(value) > 0 {
			out["source_part_"+strconv.Itoa(idx)] = value
		}
	}
	return out
}

func dedupeRateEntries(entries []ports.ProviderRateEntry) []ports.ProviderRateEntry {
	if len(entries) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(entries))
	out := make([]ports.ProviderRateEntry, 0, len(entries))
	for _, entry := range entries {
		key := strings.ToLower(strings.Join([]string{entry.Model, entry.BillingMode, entry.PriceItem, entry.Unit, entry.Currency}, "\x00"))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, entry)
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

func parsePositiveInt64(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
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
