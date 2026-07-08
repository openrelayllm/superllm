package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (c *Client) CreateKey(ctx context.Context, in ports.SessionProbeInput, request ports.CreateProviderKeyInput) (*ports.ProviderKeyResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "new api provider adapter is not configured")
	}
	apiBaseURL, _, err := normalizeBaseURLs(firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url")), firstNonEmpty(in.Origin, stringValue(in.Bundle, "origin")))
	if err != nil {
		return nil, err
	}
	tokenEndpoint, err := buildEndpointURL(apiBaseURL, "/api/token/")
	if err != nil {
		return nil, err
	}
	if existing, err := c.findNewAPIProviderToken(ctx, tokenEndpoint, in.Bundle, request); err == nil && existing != nil {
		return existing, nil
	}
	payload := newAPITokenCreatePayload(request, c.now().UTC())
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	raw, err := c.doSessionJSONBody(ctx, http.MethodPost, tokenEndpoint, in.Bundle, body)
	if err != nil {
		return nil, err
	}
	envelope, err := decodeEnvelope(raw)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_KEY_RESPONSE_INVALID", "new api token create response is invalid").WithCause(err)
	}
	if !envelope.Success {
		return nil, classifyKeyBusinessFailure(envelope.Message)
	}
	created, err := c.findNewAPIProviderToken(ctx, tokenEndpoint, in.Bundle, request)
	if err != nil {
		return nil, err
	}
	if created == nil || strings.TrimSpace(created.Secret) == "" {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_KEY_RESPONSE_INVALID", "new api token create response did not expose created key")
	}
	return created, nil
}

func (c *Client) RenameKey(ctx context.Context, in ports.SessionProbeInput, request ports.RenameProviderKeyInput) (*ports.ProviderKeyResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "new api provider adapter is not configured")
	}
	tokenID, err := strconv.ParseInt(strings.TrimSpace(request.ExternalKeyID), 10, 64)
	if err != nil || tokenID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "SUPPLIER_KEY_EXTERNAL_ID_INVALID", "invalid new api token id")
	}
	name := newAPITokenName(request.Name)
	if name == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "SUPPLIER_KEY_PROVIDER_NAME_INVALID", "target provider key name is empty")
	}
	apiBaseURL, _, err := normalizeBaseURLs(firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url")), firstNonEmpty(in.Origin, stringValue(in.Bundle, "origin")))
	if err != nil {
		return nil, err
	}
	tokenEndpoint, err := buildEndpointURL(apiBaseURL, "/api/token/")
	if err != nil {
		return nil, err
	}
	getEndpoint := strings.TrimRight(tokenEndpoint, "/") + "/" + strconv.FormatInt(tokenID, 10)
	raw, err := c.doSessionJSON(ctx, http.MethodGet, getEndpoint, in.Bundle)
	if err != nil {
		return nil, err
	}
	envelope, err := decodeEnvelope(raw)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_KEY_RESPONSE_INVALID", "new api token read response is invalid").WithCause(err)
	}
	if !envelope.Success {
		return nil, classifyKeyBusinessFailure(envelope.Message)
	}
	payload := newAPITokenRenamePayload(envelope.Data, tokenID, name)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	updatedRaw, err := c.doSessionJSONBody(ctx, http.MethodPut, tokenEndpoint, in.Bundle, body)
	if err != nil {
		return nil, err
	}
	updatedEnvelope, err := decodeEnvelope(updatedRaw)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_KEY_RESPONSE_INVALID", "new api token update response is invalid").WithCause(err)
	}
	if !updatedEnvelope.Success {
		return nil, classifyKeyBusinessFailure(updatedEnvelope.Message)
	}
	token := parseNewAPITokenSnapshot(updatedEnvelope.Data)
	if token.ID <= 0 {
		token = parseNewAPITokenSnapshot(payload)
	}
	token.Name = name
	return newAPIRenamedProviderKeyResult(request, token, c.now().UTC()), nil
}

func (c *Client) DisableKey(ctx context.Context, in ports.SessionProbeInput, request ports.DisableProviderKeyInput) (*ports.ProviderKeyResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "new api provider adapter is not configured")
	}
	tokenID, err := strconv.ParseInt(strings.TrimSpace(request.ExternalKeyID), 10, 64)
	if err != nil || tokenID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "SUPPLIER_KEY_EXTERNAL_ID_INVALID", "invalid new api token id")
	}
	apiBaseURL, _, err := normalizeBaseURLs(firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url")), firstNonEmpty(in.Origin, stringValue(in.Bundle, "origin")))
	if err != nil {
		return nil, err
	}
	tokenEndpoint, err := buildEndpointURL(apiBaseURL, "/api/token/")
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(map[string]any{
		"id":     tokenID,
		"status": 2,
	})
	if err != nil {
		return nil, err
	}
	raw, err := c.doSessionJSONBody(ctx, http.MethodPut, newAPITokenStatusOnlyEndpoint(tokenEndpoint), in.Bundle, body)
	if err != nil {
		return nil, err
	}
	envelope, err := decodeEnvelope(raw)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_KEY_RESPONSE_INVALID", "new api token disable response is invalid").WithCause(err)
	}
	if !envelope.Success {
		return nil, classifyKeyBusinessFailure(envelope.Message)
	}
	token := parseNewAPITokenSnapshot(envelope.Data)
	return &ports.ProviderKeyResult{
		SupplierID:      request.SupplierID,
		ExternalGroupID: firstNonEmpty(token.Group, request.ExternalGroupID),
		ExternalKeyID:   strconv.FormatInt(tokenID, 10),
		Name:            firstNonEmpty(token.Name, request.Name),
		Status:          "disabled",
		RawPayload:      token.RawPayload,
		CreatedAt:       c.now().UTC(),
	}, nil
}

func (c *Client) DeleteKey(ctx context.Context, in ports.SessionProbeInput, request ports.DeleteProviderKeyInput) (*ports.ProviderKeyResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "new api provider adapter is not configured")
	}
	tokenID, err := strconv.ParseInt(strings.TrimSpace(request.ExternalKeyID), 10, 64)
	if err != nil || tokenID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "SUPPLIER_KEY_EXTERNAL_ID_INVALID", "invalid new api token id")
	}
	apiBaseURL, _, err := normalizeBaseURLs(firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url")), firstNonEmpty(in.Origin, stringValue(in.Bundle, "origin")))
	if err != nil {
		return nil, err
	}
	tokenEndpoint, err := buildEndpointURL(apiBaseURL, "/api/token/")
	if err != nil {
		return nil, err
	}
	deleteEndpoint := strings.TrimRight(tokenEndpoint, "/") + "/" + strconv.FormatInt(tokenID, 10)
	raw, err := c.doSessionJSON(ctx, http.MethodDelete, deleteEndpoint, in.Bundle)
	if err != nil {
		return nil, err
	}
	envelope, err := decodeEnvelope(raw)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_KEY_RESPONSE_INVALID", "new api token delete response is invalid").WithCause(err)
	}
	if !envelope.Success {
		return nil, classifyKeyBusinessFailure(envelope.Message)
	}
	return &ports.ProviderKeyResult{
		SupplierID:      request.SupplierID,
		ExternalGroupID: request.ExternalGroupID,
		ExternalKeyID:   strconv.FormatInt(tokenID, 10),
		Name:            request.Name,
		Status:          "deleted",
		RawPayload:      sanitizeNewAPIKeyPayload(envelope.Raw),
		CreatedAt:       c.now().UTC(),
	}, nil
}

func (c *Client) ListKeys(ctx context.Context, in ports.SessionProbeInput, request ports.ListProviderKeysInput) (*ports.ListProviderKeysResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "new api provider adapter is not configured")
	}
	apiBaseURL, _, err := normalizeBaseURLs(firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url")), firstNonEmpty(in.Origin, stringValue(in.Bundle, "origin")))
	if err != nil {
		return nil, err
	}
	tokenEndpoint, err := buildEndpointURL(apiBaseURL, "/api/token/")
	if err != nil {
		return nil, err
	}
	limit := providerKeyListLimit(request.Limit)
	page := providerKeyListPage(request.Page)
	query := url.Values{}
	query.Set("p", strconv.Itoa(page))
	query.Set("page_size", strconv.Itoa(limit))
	listEndpoint := tokenEndpoint
	if encoded := query.Encode(); encoded != "" {
		listEndpoint += "?" + encoded
	}
	raw, err := c.doSessionJSON(ctx, http.MethodGet, listEndpoint, in.Bundle)
	if err != nil {
		return nil, err
	}
	envelope, err := decodeEnvelope(raw)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_KEY_RESPONSE_INVALID", "new api token list response is invalid").WithCause(err)
	}
	if !envelope.Success {
		return nil, classifyKeyBusinessFailure(envelope.Message)
	}
	supplierID := request.SupplierID
	if supplierID <= 0 {
		supplierID = in.SupplierID
	}
	tokens := parseNewAPITokenList(envelope.Raw)
	keys := make([]ports.ProviderKeySnapshot, 0, len(tokens))
	for _, token := range tokens {
		keys = append(keys, ports.ProviderKeySnapshot{
			SupplierID:      supplierID,
			ExternalGroupID: token.Group,
			ExternalKeyID:   strconv.FormatInt(token.ID, 10),
			Name:            token.Name,
			Status:          firstNonEmpty(token.Status, "active"),
			RawPayload:      token.RawPayload,
		})
	}
	total := newAPITokenListTotal(envelope.Raw)
	if total <= 0 {
		total = len(keys)
	}
	return &ports.ListProviderKeysResult{
		SupplierID: supplierID,
		SystemType: "new_api",
		Origin:     in.Origin,
		APIBaseURL: apiBaseURL,
		Keys:       keys,
		Total:      total,
		CapturedAt: c.now().UTC(),
	}, nil
}

func (c *Client) ReadKeyCapacity(ctx context.Context, in ports.SessionProbeInput, request ports.ReadProviderKeyCapacityInput) (*ports.ProviderKeyCapacityResult, error) {
	firstPage, err := c.ListKeys(ctx, in, ports.ListProviderKeysInput{
		SupplierID: request.SupplierID,
		Page:       1,
		Limit:      request.Limit,
	})
	if err != nil {
		return nil, err
	}
	keys, diagnostics := collectProviderKeysFromPages(ctx, firstPage, func(page int) (*ports.ListProviderKeysResult, error) {
		return c.ListKeys(ctx, in, ports.ListProviderKeysInput{
			SupplierID: request.SupplierID,
			Page:       page,
			Limit:      request.Limit,
		})
	})
	activeCount := countProviderKeysOccupyingCapacity(keys)
	return &ports.ProviderKeyCapacityResult{
		SupplierID:        firstPage.SupplierID,
		SystemType:        firstPage.SystemType,
		Origin:            firstPage.Origin,
		APIBaseURL:        firstPage.APIBaseURL,
		KeyLimitPolicy:    "unknown",
		KeyLimitValue:     0,
		ActiveKeyCount:    activeCount,
		RemainingKeySlots: 0,
		KeyCapacityStatus: "unknown",
		LimitKnown:        false,
		Keys:              keys,
		Diagnostics:       diagnostics,
		CapturedAt:        c.now().UTC(),
	}, nil
}

func (c *Client) findNewAPIProviderToken(ctx context.Context, tokenEndpoint string, bundle map[string]any, request ports.CreateProviderKeyInput) (*ports.ProviderKeyResult, error) {
	name := newAPITokenName(request.Name)
	if name == "" {
		return nil, nil
	}
	query := url.Values{}
	query.Set("keyword", name)
	query.Set("p", "1")
	query.Set("page_size", "100")
	searchEndpoint := strings.TrimRight(tokenEndpoint, "/") + "/search?" + query.Encode()
	raw, err := c.doSessionJSON(ctx, http.MethodGet, searchEndpoint, bundle)
	if err != nil {
		return nil, err
	}
	envelope, err := decodeEnvelope(raw)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_KEY_RESPONSE_INVALID", "new api token search response is invalid").WithCause(err)
	}
	if !envelope.Success {
		return nil, classifyKeyBusinessFailure(envelope.Message)
	}
	tokens := parseNewAPITokenList(envelope.Raw)
	for _, token := range tokens {
		if !newAPITokenMatchesRequest(token, name, request.ExternalGroupID) {
			continue
		}
		secret, err := c.readNewAPIProviderTokenKey(ctx, tokenEndpoint, bundle, token.ID)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(secret) == "" {
			continue
		}
		return newAPIProviderKeyResult(request, token, secret, c.now().UTC()), nil
	}
	return nil, nil
}

func (c *Client) readNewAPIProviderTokenKey(ctx context.Context, tokenEndpoint string, bundle map[string]any, tokenID int64) (string, error) {
	if tokenID <= 0 {
		return "", nil
	}
	endpoint := strings.TrimRight(tokenEndpoint, "/") + "/" + strconv.FormatInt(tokenID, 10) + "/key"
	raw, err := c.doSessionJSONBody(ctx, http.MethodPost, endpoint, bundle, []byte(`{}`))
	if err != nil {
		return "", err
	}
	envelope, err := decodeEnvelope(raw)
	if err != nil {
		return "", infraerrors.New(http.StatusBadGateway, "SUPPLIER_KEY_RESPONSE_INVALID", "new api token key response is invalid").WithCause(err)
	}
	if !envelope.Success {
		return "", classifyKeyBusinessFailure(envelope.Message)
	}
	return normalizeNewAPISecret(stringFromAny(envelope.Data["key"])), nil
}

func (c *Client) doSessionJSONBody(ctx context.Context, method string, endpoint string, bundle map[string]any, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	applySessionHeaders(req, bundle)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_SESSION_REQUEST_FAILED", "failed to request new api session endpoint").WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, infraerrors.New(resp.StatusCode, "SUPPLIER_SESSION_PERMISSION_DENIED", "new api session cannot access requested endpoint")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_SESSION_BAD_STATUS", "new api session endpoint returned non-success status")
	}
	return data, nil
}

type newAPITokenSnapshot struct {
	ID          int64
	Name        string
	Group       string
	Status      string
	ExpiredTime int64
	RawPayload  map[string]any
}

func newAPITokenCreatePayload(request ports.CreateProviderKeyInput, now time.Time) map[string]any {
	name := newAPITokenName(request.Name)
	if name == "" {
		name = "AdminPlus-" + now.Format("20060102150405")
	}
	payload := map[string]any{
		"name":              name,
		"expired_time":      int64(-1),
		"unlimited_quota":   true,
		"group":             strings.TrimSpace(request.ExternalGroupID),
		"cross_group_retry": false,
	}
	if request.ExpiresInDays != nil && *request.ExpiresInDays > 0 {
		payload["expired_time"] = now.Add(time.Duration(*request.ExpiresInDays) * 24 * time.Hour).Unix()
	}
	if request.QuotaUSD > 0 {
		payload["remain_quota"] = usdAmountToNewAPIQuotaUnits(request.QuotaUSD)
		payload["unlimited_quota"] = false
	}
	return payload
}

func newAPITokenRenamePayload(raw map[string]any, tokenID int64, name string) map[string]any {
	payload := map[string]any{
		"id":                   tokenID,
		"name":                 name,
		"expired_time":         int64FromAny(raw["expired_time"]),
		"remain_quota":         int64FromAny(raw["remain_quota"]),
		"unlimited_quota":      boolFromAny(raw["unlimited_quota"]),
		"model_limits_enabled": boolFromAny(raw["model_limits_enabled"]),
		"model_limits":         stringFromAny(raw["model_limits"]),
		"allow_ips":            raw["allow_ips"],
		"group":                stringFromAny(raw["group"]),
		"cross_group_retry":    boolFromAny(raw["cross_group_retry"]),
	}
	if status := int64FromAny(raw["status"]); status > 0 {
		payload["status"] = status
	}
	return payload
}

func parseNewAPITokenList(data map[string]any) []newAPITokenSnapshot {
	items := newAPITokenItems(data)
	if len(items) == 0 {
		return nil
	}
	out := make([]newAPITokenSnapshot, 0, len(items))
	for _, item := range items {
		raw, ok := item.(map[string]any)
		if !ok {
			continue
		}
		token := parseNewAPITokenSnapshot(raw)
		if token.ID > 0 && token.Name != "" {
			out = append(out, token)
		}
	}
	return out
}

func newAPITokenItems(payload map[string]any) []any {
	if payload == nil {
		return nil
	}
	for _, value := range []any{
		payload["items"],
		payload["list"],
		payload["data"],
	} {
		if items := newAPITokenItemsFromAny(value); len(items) > 0 {
			return items
		}
	}
	if data, ok := payload["data"].(map[string]any); ok {
		for _, value := range []any{
			data["items"],
			data["list"],
			data["data"],
		} {
			if items := newAPITokenItemsFromAny(value); len(items) > 0 {
				return items
			}
		}
	}
	return nil
}

func newAPITokenItemsFromAny(value any) []any {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	return items
}

func parseNewAPITokenSnapshot(raw map[string]any) newAPITokenSnapshot {
	if len(raw) == 0 {
		return newAPITokenSnapshot{}
	}
	return newAPITokenSnapshot{
		ID:          int64FromAny(raw["id"]),
		Name:        stringFromAny(raw["name"]),
		Group:       stringFromAny(raw["group"]),
		Status:      normalizeNewAPITokenStatus(raw["status"]),
		ExpiredTime: int64FromAny(raw["expired_time"]),
		RawPayload:  sanitizeNewAPIKeyPayload(raw),
	}
}

func newAPITokenListTotal(payload map[string]any) int {
	if payload == nil {
		return 0
	}
	if total := int64FromAny(payload["total"]); total > 0 {
		return int(total)
	}
	if data, ok := payload["data"].(map[string]any); ok {
		if total := int64FromAny(data["total"]); total > 0 {
			return int(total)
		}
	}
	return 0
}

func newAPIProviderKeyResult(request ports.CreateProviderKeyInput, token newAPITokenSnapshot, secret string, capturedAt time.Time) *ports.ProviderKeyResult {
	return &ports.ProviderKeyResult{
		SupplierID:      request.SupplierID,
		ExternalGroupID: firstNonEmpty(token.Group, request.ExternalGroupID),
		ExternalKeyID:   strconv.FormatInt(token.ID, 10),
		Name:            firstNonEmpty(token.Name, request.Name),
		Secret:          secret,
		Status:          firstNonEmpty(token.Status, "active"),
		RawPayload:      token.RawPayload,
		CreatedAt:       capturedAt,
	}
}

func newAPIRenamedProviderKeyResult(request ports.RenameProviderKeyInput, token newAPITokenSnapshot, capturedAt time.Time) *ports.ProviderKeyResult {
	externalKeyID := strings.TrimSpace(request.ExternalKeyID)
	if token.ID > 0 {
		externalKeyID = strconv.FormatInt(token.ID, 10)
	}
	return &ports.ProviderKeyResult{
		SupplierID:      request.SupplierID,
		ExternalGroupID: firstNonEmpty(token.Group, request.ExternalGroupID),
		ExternalKeyID:   externalKeyID,
		Name:            firstNonEmpty(token.Name, request.Name),
		Status:          firstNonEmpty(token.Status, "active"),
		RawPayload:      token.RawPayload,
		CreatedAt:       capturedAt,
	}
}

func newAPITokenMatchesRequest(token newAPITokenSnapshot, name string, group string) bool {
	if strings.TrimSpace(token.Name) != strings.TrimSpace(name) {
		return false
	}
	group = strings.TrimSpace(group)
	return group == "" || strings.TrimSpace(token.Group) == group
}

func newAPITokenName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) > 50 {
		name = name[:50]
		for !utf8.ValidString(name) {
			_, size := utf8.DecodeLastRuneInString(name)
			if size <= 0 || len(name) < size {
				return ""
			}
			name = name[:len(name)-size]
		}
	}
	return name
}

func newAPITokenStatusOnlyEndpoint(tokenEndpoint string) string {
	parsed, err := url.Parse(tokenEndpoint)
	if err != nil {
		separator := "?"
		if strings.Contains(tokenEndpoint, "?") {
			separator = "&"
		}
		return tokenEndpoint + separator + "status_only=true"
	}
	q := parsed.Query()
	q.Set("status_only", "true")
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

func normalizeNewAPISecret(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" || strings.HasPrefix(secret, "sk-") {
		return secret
	}
	return "sk-" + secret
}

func normalizeNewAPITokenStatus(value any) string {
	if text := stringFromAny(value); text != "" {
		normalized := strings.ToLower(strings.TrimSpace(text))
		switch normalized {
		case "1", "active", "enabled", "enable":
			return "active"
		case "2", "disabled", "disable", "inactive", "deleted":
			return "disabled"
		default:
			return normalized
		}
	}
	switch int64FromAny(value) {
	case 1:
		return "active"
	case 2:
		return "disabled"
	default:
		return ""
	}
}

func providerKeyListLimit(limit int) int {
	if limit <= 0 {
		return 1000
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func providerKeyListPage(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}

func collectProviderKeysFromPages(ctx context.Context, firstPage *ports.ListProviderKeysResult, readPage func(page int) (*ports.ListProviderKeysResult, error)) ([]ports.ProviderKeySnapshot, map[string]any) {
	diagnostics := map[string]any{
		"capacity_source": "list_keys",
		"limit_source":    "not_exposed_by_provider",
	}
	if firstPage == nil {
		diagnostics["pages_read"] = 0
		return nil, diagnostics
	}
	keys := append([]ports.ProviderKeySnapshot(nil), firstPage.Keys...)
	pageSize := len(firstPage.Keys)
	if pageSize <= 0 {
		diagnostics["pages_read"] = 1
		diagnostics["provider_total"] = firstPage.Total
		return keys, diagnostics
	}
	total := firstPage.Total
	pagesRead := 1
	for total > len(keys) && pagesRead < 100 {
		select {
		case <-ctx.Done():
			diagnostics["truncated"] = true
			diagnostics["truncated_reason"] = "context_cancelled"
			diagnostics["pages_read"] = pagesRead
			diagnostics["provider_total"] = total
			return keys, diagnostics
		default:
		}
		nextPage, err := readPage(pagesRead + 1)
		if err != nil || nextPage == nil || len(nextPage.Keys) == 0 {
			diagnostics["truncated"] = true
			if err != nil {
				diagnostics["truncated_reason"] = err.Error()
			} else {
				diagnostics["truncated_reason"] = "empty_page"
			}
			break
		}
		keys = append(keys, nextPage.Keys...)
		pagesRead++
		if nextPage.Total > total {
			total = nextPage.Total
		}
		if len(nextPage.Keys) < pageSize {
			break
		}
	}
	if total > len(keys) {
		diagnostics["truncated"] = true
	}
	diagnostics["pages_read"] = pagesRead
	diagnostics["provider_total"] = total
	return keys, diagnostics
}

func countProviderKeysOccupyingCapacity(keys []ports.ProviderKeySnapshot) int {
	count := 0
	for _, key := range keys {
		switch strings.ToLower(strings.TrimSpace(key.Status)) {
		case "", "active", "enabled", "enable":
			count++
		}
	}
	return count
}

func sanitizeNewAPIKeyPayload(raw map[string]any) map[string]any {
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
