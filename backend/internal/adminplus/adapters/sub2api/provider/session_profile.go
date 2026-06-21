package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const browserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"

type rateEndpointCandidate struct {
	path   string
	parser func([]byte) []ports.ProviderRateEntry
}

type billingEndpointCandidate struct {
	path string
}

type capabilityEndpoint struct {
	key   string
	path  string
	query map[string]string
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

func (c *SessionProfileClient) DirectLogin(ctx context.Context, in ports.DirectLoginInput) (*ports.DirectLoginResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "provider adapter is not configured")
	}
	apiBaseURL := firstNonEmpty(in.APIBaseURL, in.Origin)
	if apiBaseURL == "" {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_DIRECT_LOGIN_API_BASE_URL_REQUIRED", "supplier api base url is required for direct login")
	}
	if in.Token != "" {
		return c.directLoginFromToken(ctx, in, apiBaseURL)
	}
	if strings.TrimSpace(in.Username) == "" || strings.TrimSpace(in.Password) == "" {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED", "supplier username and password are required")
	}
	revision, settingsDiagnostics, err := c.fetchLoginAgreementRevision(ctx, apiBaseURL)
	if err != nil {
		return nil, err
	}
	loginEndpoint, err := buildSub2APIUserEndpointURL(apiBaseURL, "/auth/login")
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"email":    strings.TrimSpace(in.Username),
		"password": strings.TrimSpace(in.Password),
	}
	if revision != "" {
		payload["login_agreement_revision"] = revision
	}
	for key, value := range in.LoginContext {
		if strings.TrimSpace(key) == "" {
			continue
		}
		payload[key] = value
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	applyBrowserCompatHeaders(req)
	origin := firstNonEmpty(in.Origin, originFromRawURL(apiBaseURL))
	if origin != "" {
		req.Header.Set("Origin", origin)
		req.Header.Set("Referer", strings.TrimRight(origin, "/")+"/")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_FAILED", "failed to request supplier login endpoint").WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, infraerrors.New(resp.StatusCode, "LOGIN_CREDENTIAL_INVALID", "supplier direct login credential is invalid")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, classifyDirectLoginFailure(resp.StatusCode, data)
	}
	token, expiresAt, raw, err := parseSub2APILoginResponse(data)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_RESPONSE_INVALID", "supplier login response is invalid").WithCause(err)
	}
	if token == "" {
		return nil, classifyDirectLoginFailure(resp.StatusCode, data)
	}
	capturedAt := c.now().UTC()
	bundle := buildDirectLoginSessionBundle(ports.DirectLoginInput{
		SupplierID: in.SupplierID,
		Origin:     origin,
		APIBaseURL: apiBaseURL,
	}, token, capturedAt, expiresAt)
	return &ports.DirectLoginResult{
		SupplierID:    in.SupplierID,
		Origin:        origin,
		APIBaseURL:    apiBaseURL,
		SessionBundle: bundle,
		CapturedAt:    capturedAt,
		ExpiresAt:     expiresAt,
		Diagnostics: map[string]any{
			"login_endpoint":           loginEndpoint,
			"settings_endpoint":        settingsDiagnostics["settings_endpoint"],
			"login_agreement_revision": revision != "",
			"login_response_keys":      rawKeys(raw),
		},
	}, nil
}

func (c *SessionProfileClient) directLoginFromToken(ctx context.Context, in ports.DirectLoginInput, apiBaseURL string) (*ports.DirectLoginResult, error) {
	origin := firstNonEmpty(in.Origin, originFromRawURL(apiBaseURL))
	capturedAt := c.now().UTC()
	bundle := buildDirectLoginSessionBundle(ports.DirectLoginInput{
		SupplierID: in.SupplierID,
		Origin:     origin,
		APIBaseURL: apiBaseURL,
	}, strings.TrimSpace(in.Token), capturedAt, nil)
	probe, err := c.ProbeSub2APIUserProfile(ctx, ports.SessionProbeInput{
		SupplierID: in.SupplierID,
		Origin:     origin,
		APIBaseURL: apiBaseURL,
		Bundle:     bundle,
	})
	if err != nil {
		return nil, err
	}
	diagnostics := map[string]any{"token_probe": "ok"}
	if probe != nil {
		diagnostics["profile_status"] = probe.Status
	}
	return &ports.DirectLoginResult{
		SupplierID:    in.SupplierID,
		Origin:        origin,
		APIBaseURL:    apiBaseURL,
		SessionBundle: bundle,
		CapturedAt:    capturedAt,
		Diagnostics:   diagnostics,
	}, nil
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
	capabilities := c.probeSub2APICapabilities(ctx, apiBaseURL, in.Bundle)
	capabilities["can_read_profile"] = true
	capabilities["can_read_balance"] = true
	return &ports.SessionProbeResult{
		SupplierID:      in.SupplierID,
		Status:          "valid",
		SystemType:      "sub2api",
		Origin:          in.Origin,
		APIBaseURL:      apiBaseURL,
		Capabilities:    capabilities,
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

func (c *SessionProfileClient) ReadChannelMonitors(ctx context.Context, in ports.SessionProbeInput) (*ports.ReadChannelMonitorsResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "provider adapter is not configured")
	}
	apiBaseURL := firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url"), in.Origin)
	endpoint, err := buildSub2APIUserEndpointURL(apiBaseURL, "/channel-monitors")
	if err != nil {
		return nil, err
	}
	body, err := c.doSessionJSON(ctx, http.MethodGet, endpoint, in.Bundle, true)
	if err != nil {
		return nil, err
	}
	items, err := parseChannelMonitorViews(body)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_CHANNEL_MONITORS_RESPONSE_INVALID", "supplier channel monitor response is invalid").WithCause(err)
	}
	return &ports.ReadChannelMonitorsResult{
		SupplierID: in.SupplierID,
		SystemType: "sub2api",
		Origin:     in.Origin,
		APIBaseURL: apiBaseURL,
		Items:      items,
		CapturedAt: c.now().UTC(),
	}, nil
}

func buildSub2APIUserProfileURL(apiBaseURL string) (string, error) {
	return buildSub2APIUserEndpointURL(apiBaseURL, "/user/profile")
}

func (c *SessionProfileClient) fetchLoginAgreementRevision(ctx context.Context, apiBaseURL string) (string, map[string]any, error) {
	endpoint, err := buildSub2APIUserEndpointURL(apiBaseURL, "/settings/public")
	if err != nil {
		return "", nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", nil, err
	}
	applyBrowserCompatHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_SETTINGS_FAILED", "failed to request supplier public settings").WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, classifyDirectLoginUpstreamFailure("SUPPLIER_DIRECT_LOGIN_SETTINGS_BAD_STATUS", resp.StatusCode, data)
	}
	body, ok := unwrapDataObject(data)
	if !ok {
		return "", nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_SETTINGS_INVALID", "supplier public settings response is invalid")
	}
	if boolFromAny(firstExisting(body, "turnstile_enabled", "turnstileEnabled")) {
		return "", nil, infraerrors.New(http.StatusConflict, "LOGIN_CAPTCHA_REQUIRED", "supplier direct login requires captcha; use browser extension fallback")
	}
	if boolFromAny(firstExisting(body, "totp_enabled", "totpEnabled", "mfa_enabled", "mfaEnabled")) {
		return "", nil, infraerrors.New(http.StatusConflict, "LOGIN_MFA_REQUIRED", "supplier direct login requires 2FA; use browser extension fallback")
	}
	if boolFromAny(firstExisting(body, "backend_mode_enabled", "backendModeEnabled")) {
		return "", nil, infraerrors.New(http.StatusConflict, "SUPPLIER_DIRECT_LOGIN_ADMIN_REQUIRED", "supplier backend mode requires an admin account for direct login")
	}
	return firstNonEmpty(stringFromAny(body["login_agreement_revision"]), stringFromAny(body["loginAgreementRevision"])), map[string]any{
		"settings_endpoint": endpoint,
		"settings_keys":     rawKeys(body),
	}, nil
}

func buildDirectLoginSessionBundle(in ports.DirectLoginInput, accessToken string, capturedAt time.Time, expiresAt *time.Time) map[string]any {
	origin := firstNonEmpty(in.Origin, originFromRawURL(in.APIBaseURL))
	apiBaseURL := firstNonEmpty(in.APIBaseURL, origin)
	requiredHeaders := map[string]any{}
	if origin != "" {
		requiredHeaders["origin"] = origin
		requiredHeaders["referer"] = strings.TrimRight(origin, "/") + "/"
	}
	bundle := map[string]any{
		"origin":           origin,
		"api_base_url":     apiBaseURL,
		"access_token":     accessToken,
		"tokens":           map[string]any{"access_token": accessToken},
		"required_headers": requiredHeaders,
		"context": map[string]any{
			"api_base_url":   apiBaseURL,
			"login_method":   "direct_login",
			"session_source": "direct_login",
		},
		"captured_at":    capturedAt.UTC().Format(time.RFC3339),
		"session_source": "direct_login",
	}
	if expiresAt != nil && !expiresAt.IsZero() {
		bundle["expires_at"] = expiresAt.UTC().Format(time.RFC3339)
	}
	return bundle
}

func parseSub2APILoginResponse(data []byte) (string, *time.Time, map[string]any, error) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return "", nil, nil, err
	}
	body := root
	if dataValue, ok := root["data"].(map[string]any); ok {
		body = dataValue
	}
	token := firstNonEmpty(
		stringFromAny(body["access_token"]),
		stringFromAny(body["accessToken"]),
		stringFromAny(body["token"]),
		stringFromAny(root["access_token"]),
		stringFromAny(root["accessToken"]),
	)
	var expiresAt *time.Time
	if t, ok := firstTimeValue(body, "expires_at", "expiresAt", "access_token_expires_at", "accessTokenExpiresAt"); ok {
		value := t.UTC()
		expiresAt = &value
	} else if expiresIn := int64FromAny(firstExisting(body, "expires_in", "expiresIn")); expiresIn > 0 {
		value := time.Now().UTC().Add(time.Duration(expiresIn) * time.Second)
		expiresAt = &value
	}
	return token, expiresAt, body, nil
}

func classifyDirectLoginFailure(statusCode int, body []byte) error {
	lower := strings.ToLower(string(body))
	switch {
	case isCloudflareOriginFailure(statusCode, lower):
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_UPSTREAM_ORIGIN_ERROR", "supplier site returned a Cloudflare or origin server error")
	case looksLikeHTMLResponse(lower):
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_UPSTREAM_HTML", "supplier login endpoint returned an HTML response")
	case strings.Contains(lower, "captcha") || strings.Contains(lower, "turnstile") || strings.Contains(lower, "recaptcha"):
		return infraerrors.New(http.StatusConflict, "LOGIN_CAPTCHA_REQUIRED", "supplier direct login requires captcha; use browser extension fallback")
	case strings.Contains(lower, "2fa") || strings.Contains(lower, "totp") || strings.Contains(lower, "mfa"):
		return infraerrors.New(http.StatusConflict, "LOGIN_MFA_REQUIRED", "supplier direct login requires 2FA; use browser extension fallback")
	case strings.Contains(lower, "challenge") || strings.Contains(lower, "risk") || strings.Contains(lower, "verify"):
		return infraerrors.New(http.StatusConflict, "BROWSER_FALLBACK_REQUIRED", "supplier direct login requires browser verification")
	case strings.Contains(lower, "invalid") || strings.Contains(lower, "password") || strings.Contains(lower, "credential"):
		return infraerrors.New(http.StatusUnauthorized, "LOGIN_CREDENTIAL_INVALID", "supplier direct login credential is invalid")
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return infraerrors.New(statusCode, "LOGIN_CREDENTIAL_INVALID", "supplier direct login credential is invalid")
	default:
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_BAD_STATUS", "supplier login endpoint returned non-success status")
	}
}

func classifyDirectLoginUpstreamFailure(defaultReason string, statusCode int, body []byte) error {
	lower := strings.ToLower(string(body))
	if isCloudflareOriginFailure(statusCode, lower) {
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_UPSTREAM_ORIGIN_ERROR", "supplier site returned a Cloudflare or origin server error")
	}
	if looksLikeHTMLResponse(lower) {
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_UPSTREAM_HTML", "supplier endpoint returned an HTML response")
	}
	return infraerrors.New(http.StatusBadGateway, defaultReason, "supplier endpoint returned non-success status")
}

func isCloudflareOriginFailure(statusCode int, lowerBody string) bool {
	if statusCode == 520 || statusCode == 521 || statusCode == 522 || statusCode == 523 || statusCode == 524 {
		return true
	}
	return strings.Contains(lowerBody, "cloudflare") &&
		(strings.Contains(lowerBody, "origin web server") ||
			strings.Contains(lowerBody, "invalid or incomplete response") ||
			strings.Contains(lowerBody, "error 520"))
}

func looksLikeHTMLResponse(lowerBody string) bool {
	trimmed := strings.TrimSpace(lowerBody)
	return strings.HasPrefix(trimmed, "<!doctype html") || strings.HasPrefix(trimmed, "<html")
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

func (c *SessionProfileClient) probeSub2APICapabilities(ctx context.Context, apiBaseURL string, bundle map[string]any) map[string]bool {
	capabilities := map[string]bool{
		"can_read_profile":       false,
		"can_read_balance":       false,
		"can_read_groups":        false,
		"can_read_rates":         false,
		"can_read_announcements": false,
		"can_read_usage_costs":   false,
		"can_create_key":         false,
	}
	for _, endpoint := range []capabilityEndpoint{
		{key: "can_read_groups", path: "/groups/available"},
		{key: "can_read_rates", path: "/channels/available"},
		{key: "can_read_announcements", path: "/announcements", query: map[string]string{"unread_only": "false"}},
		{key: "can_read_announcements", path: "/payment/checkout-info"},
		{key: "can_read_usage_costs", path: "/usage", query: map[string]string{"page": "1", "page_size": "1"}},
		{key: "can_create_key", path: "/keys", query: map[string]string{"page": "1", "page_size": "1"}},
	} {
		if capabilities[endpoint.key] {
			continue
		}
		url, err := buildSub2APIUserEndpointURL(apiBaseURL, endpoint.path)
		if err != nil {
			continue
		}
		url = appendQueryValues(url, endpoint.query)
		_, err = c.doSessionJSON(ctx, http.MethodGet, url, bundle, false)
		capabilities[endpoint.key] = err == nil
	}
	return capabilities
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
	endpoint, err := buildSub2APIUserEndpointURL(apiBaseURL, "/keys")
	if err != nil {
		return nil, err
	}
	if existing, err := c.findExistingProviderKey(ctx, endpoint, in.Bundle, request); err == nil && existing != nil {
		existing.SupplierID = request.SupplierID
		existing.ExternalGroupID = firstNonEmpty(existing.ExternalGroupID, request.ExternalGroupID)
		existing.Name = firstNonEmpty(existing.Name, request.Name)
		if existing.CreatedAt.IsZero() {
			existing.CreatedAt = c.now().UTC()
		}
		return existing, nil
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

func (c *SessionProfileClient) findExistingProviderKey(ctx context.Context, endpoint string, bundle map[string]any, request ports.CreateProviderKeyInput) (*ports.ProviderKeyResult, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return nil, nil
	}
	listEndpoint := appendQueryValues(endpoint, map[string]string{
		"page":      "1",
		"page_size": "100",
		"search":    name,
	})
	body, err := c.doSessionJSON(ctx, http.MethodGet, listEndpoint, bundle, false)
	if err != nil {
		return nil, err
	}
	values, err := unwrapDataArray(body)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		raw, ok := value.(map[string]any)
		if !ok || !providerKeyMatchesRequest(raw, request) {
			continue
		}
		result, err := parseSub2APIKeyCreateResponse(bodyFromMap(raw))
		if err != nil || strings.TrimSpace(result.Secret) == "" {
			continue
		}
		return result, nil
	}
	return nil, nil
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

func (c *SessionProfileClient) ReadAnnouncements(ctx context.Context, in ports.SessionProbeInput) (*ports.ReadAnnouncementsResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "provider adapter is not configured")
	}
	apiBaseURL := firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url"), in.Origin)
	profileEndpoint, err := buildSub2APIUserProfileURL(apiBaseURL)
	if err != nil {
		return nil, err
	}
	profileBody, err := c.doSessionJSON(ctx, http.MethodGet, profileEndpoint, in.Bundle, true)
	if err != nil {
		return nil, err
	}
	profile, _, err := parseSub2APIProfile(profileBody)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_SESSION_PROFILE_INVALID", "supplier profile response is invalid").WithCause(err)
	}
	balanceCents := int64(math.Round(profile.Balance * 100))
	if balanceCents < 0 {
		balanceCents = 0
	}
	runtimeStatus := adminplusdomain.SupplierRuntimeStatusMonitorOnly
	if balanceCents > 0 {
		runtimeStatus = adminplusdomain.SupplierRuntimeStatusCandidate
	}

	var announcements []ports.ProviderAnnouncement
	var lastErr error
	if checkoutEndpoint, err := buildSub2APIUserEndpointURL(apiBaseURL, "/payment/checkout-info"); err == nil {
		if checkoutBody, checkoutErr := c.doSessionJSON(ctx, http.MethodGet, checkoutEndpoint, in.Bundle, false); checkoutErr == nil {
			announcements = append(announcements, parseCheckoutAnnouncements(checkoutBody, balanceCents, runtimeStatus)...)
		} else {
			lastErr = checkoutErr
		}
	}
	if announcementsEndpoint, err := buildSub2APIUserEndpointURL(apiBaseURL, "/announcements"); err == nil {
		if announcementsBody, announcementsErr := c.doSessionJSON(ctx, http.MethodGet, announcementsEndpoint, in.Bundle, false); announcementsErr == nil {
			announcements = append(announcements, parseProviderAnnouncements(announcementsBody, balanceCents, runtimeStatus)...)
		} else {
			lastErr = announcementsErr
		}
	}
	announcements = dedupeAnnouncements(announcements)
	if len(announcements) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return &ports.ReadAnnouncementsResult{
		SupplierID:    in.SupplierID,
		SystemType:    "sub2api",
		Origin:        in.Origin,
		APIBaseURL:    apiBaseURL,
		Announcements: announcements,
		CapturedAt:    c.now().UTC(),
	}, nil
}

func (c *SessionProfileClient) ReadUsageCosts(ctx context.Context, in ports.SessionProbeInput, request ports.ReadUsageCostsInput) (*ports.ReadUsageCostsResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "provider adapter is not configured")
	}
	if request.SupplierID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if request.StartedAt.IsZero() || request.EndedAt.IsZero() || !request.StartedAt.Before(request.EndedAt) {
		return nil, infraerrors.New(http.StatusBadRequest, "SUPPLIER_USAGE_COST_TIME_RANGE_INVALID", "invalid supplier usage cost time range")
	}
	apiBaseURL := firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url"), in.Origin)
	candidates := []billingEndpointCandidate{
		{path: "/usage"},
		{path: "/billing/lines"},
		{path: "/billing/usage"},
		{path: "/usage/lines"},
		{path: "/user/billing"},
	}
	var lastErr error
	for _, candidate := range candidates {
		endpoint, err := buildSub2APIUserEndpointURL(apiBaseURL, candidate.path)
		if err != nil {
			return nil, err
		}
		endpoint = appendUsageCostQuery(endpoint, request)
		body, err := c.doSessionJSON(ctx, http.MethodGet, endpoint, in.Bundle, false)
		if err != nil {
			lastErr = err
			continue
		}
		lines := parseSub2APIUsageCostLines(body)
		if len(lines) == 0 {
			continue
		}
		return &ports.ReadUsageCostsResult{
			SupplierID: in.SupplierID,
			SystemType: "sub2api",
			Origin:     in.Origin,
			APIBaseURL: apiBaseURL,
			Lines:      lines,
			CapturedAt: c.now().UTC(),
		}, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_USAGE_COST_CAPABILITY_MISSING", "supplier session cannot read usage cost lines")
}

func (c *SessionProfileClient) ReadFundingTransactions(ctx context.Context, in ports.SessionProbeInput, request ports.ReadFundingTransactionsInput) (*ports.ReadFundingTransactionsResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "provider adapter is not configured")
	}
	if request.SupplierID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	apiBaseURL := firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url"), in.Origin)
	items := make([]ports.ProviderFundingTransaction, 0)
	var lastErr error
	for page := 1; page <= 20; page++ {
		endpoint, err := buildSub2APIUserEndpointURL(apiBaseURL, "/payment/orders/my")
		if err != nil {
			return nil, err
		}
		endpoint = appendQueryValues(endpoint, map[string]string{
			"page":      strconv.Itoa(page),
			"page_size": "100",
		})
		body, err := c.doSessionJSON(ctx, http.MethodGet, endpoint, in.Bundle, false)
		if err != nil {
			lastErr = err
			break
		}
		pageItems := parseSub2APIFundingTransactions(body)
		if len(pageItems) == 0 {
			break
		}
		items = append(items, pageItems...)
		if len(pageItems) < 100 {
			break
		}
	}
	if len(items) == 0 {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_FUNDING_CAPABILITY_MISSING", "supplier session cannot read funding transactions")
	}
	return &ports.ReadFundingTransactionsResult{
		SupplierID:   in.SupplierID,
		ProviderType: "sub2api",
		SystemType:   "sub2api",
		Origin:       in.Origin,
		APIBaseURL:   apiBaseURL,
		Items:        items,
		CapturedAt:   c.now().UTC(),
	}, nil
}

func (c *SessionProfileClient) ReadEntitlementTransactions(ctx context.Context, in ports.SessionProbeInput, request ports.ReadEntitlementTransactionsInput) (*ports.ReadEntitlementTransactionsResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "provider adapter is not configured")
	}
	if request.SupplierID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	apiBaseURL := firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url"), in.Origin)
	endpoint, err := buildSub2APIUserEndpointURL(apiBaseURL, "/redeem/history")
	if err != nil {
		return nil, err
	}
	body, err := c.doSessionJSON(ctx, http.MethodGet, endpoint, in.Bundle, false)
	if err != nil {
		return nil, err
	}
	items := parseSub2APIEntitlementTransactions(body)
	if len(items) == 0 {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_ENTITLEMENT_CAPABILITY_MISSING", "supplier session cannot read entitlement transactions")
	}
	return &ports.ReadEntitlementTransactionsResult{
		SupplierID:   in.SupplierID,
		ProviderType: "sub2api",
		SystemType:   "sub2api",
		Origin:       in.Origin,
		APIBaseURL:   apiBaseURL,
		Items:        items,
		CapturedAt:   c.now().UTC(),
	}, nil
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
	applyBrowserCompatHeaders(req)
	if userAgent := firstNonEmpty(stringValue(requiredHeaders, "user-agent"), stringValue(requiredHeaders, "User-Agent")); userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
}

func applyBrowserCompatHeaders(req *http.Request) {
	if req == nil {
		return
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("User-Agent", browserUserAgent)
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
		for _, key := range []string{"items", "list", "records", "rows", "lines"} {
			if arr, ok := obj[key].([]any); ok {
				return arr, nil
			}
		}
		if dataValue, ok := obj["data"].(map[string]any); ok {
			for _, key := range []string{"items", "list", "records", "rows", "lines"} {
				if arr, ok := dataValue[key].([]any); ok {
					return arr, nil
				}
			}
		}
	}
	return []any{}, nil
}

func parseChannelMonitorViews(data []byte) ([]ports.ChannelMonitorView, error) {
	values, err := unwrapDataArray(data)
	if err != nil {
		return nil, err
	}
	items := make([]ports.ChannelMonitorView, 0, len(values))
	for _, value := range values {
		raw, ok := value.(map[string]any)
		if !ok {
			continue
		}
		item := ports.ChannelMonitorView{
			ID:                   int64FromAny(raw["id"]),
			Name:                 firstNonEmpty(stringFromAny(raw["name"]), stringFromAny(raw["title"])),
			Provider:             firstNonEmpty(stringFromAny(raw["provider"]), stringFromAny(raw["platform"])),
			GroupName:            firstNonEmpty(stringFromAny(raw["group_name"]), stringFromAny(raw["groupName"])),
			PrimaryModel:         firstNonEmpty(stringFromAny(raw["primary_model"]), stringFromAny(raw["primaryModel"]), stringFromAny(raw["model"])),
			PrimaryStatus:        normalizeMonitorStatus(firstNonEmpty(stringFromAny(raw["primary_status"]), stringFromAny(raw["primaryStatus"]), stringFromAny(raw["status"]))),
			PrimaryLatencyMS:     optionalInt64FromAny(firstExisting(raw, "primary_latency_ms", "primaryLatencyMs", "latency_ms", "latencyMs")),
			PrimaryPingLatencyMS: optionalInt64FromAny(firstExisting(raw, "primary_ping_latency_ms", "primaryPingLatencyMs", "ping_latency_ms", "pingLatencyMs")),
			Availability7D:       float64FromAny(firstExisting(raw, "availability_7d", "availability7d", "availability")),
			ExtraModels:          parseChannelMonitorExtraModels(firstExisting(raw, "extra_models", "extraModels")),
			Timeline:             parseChannelMonitorTimeline(firstExisting(raw, "timeline", "history", "buckets")),
		}
		if item.Name == "" {
			item.Name = item.PrimaryModel
		}
		if item.PrimaryStatus == "" {
			item.PrimaryStatus = "unknown"
		}
		items = append(items, item)
	}
	return items, nil
}

func parseChannelMonitorExtraModels(value any) []ports.ChannelMonitorExtraModel {
	values, ok := value.([]any)
	if !ok {
		return []ports.ChannelMonitorExtraModel{}
	}
	items := make([]ports.ChannelMonitorExtraModel, 0, len(values))
	for _, value := range values {
		raw, ok := value.(map[string]any)
		if !ok {
			continue
		}
		items = append(items, ports.ChannelMonitorExtraModel{
			Model:     firstNonEmpty(stringFromAny(raw["model"]), stringFromAny(raw["name"])),
			Status:    normalizeMonitorStatus(firstNonEmpty(stringFromAny(raw["status"]), stringFromAny(raw["latest_status"]), stringFromAny(raw["latestStatus"]))),
			LatencyMS: optionalInt64FromAny(firstExisting(raw, "latency_ms", "latencyMs", "latest_latency_ms", "latestLatencyMs")),
		})
	}
	return items
}

func parseChannelMonitorTimeline(value any) []ports.ChannelMonitorTimelinePoint {
	values, ok := value.([]any)
	if !ok {
		return []ports.ChannelMonitorTimelinePoint{}
	}
	items := make([]ports.ChannelMonitorTimelinePoint, 0, len(values))
	for _, value := range values {
		raw, ok := value.(map[string]any)
		if !ok {
			continue
		}
		items = append(items, ports.ChannelMonitorTimelinePoint{
			Status:        normalizeMonitorStatus(stringFromAny(raw["status"])),
			LatencyMS:     optionalInt64FromAny(firstExisting(raw, "latency_ms", "latencyMs")),
			PingLatencyMS: optionalInt64FromAny(firstExisting(raw, "ping_latency_ms", "pingLatencyMs")),
			CheckedAt:     firstNonEmpty(stringFromAny(raw["checked_at"]), stringFromAny(raw["checkedAt"])),
		})
	}
	return items
}

func normalizeMonitorStatus(raw string) string {
	status := strings.ToLower(strings.TrimSpace(raw))
	if status == "" {
		return ""
	}
	switch status {
	case "ok", "normal", "healthy", "success":
		return "operational"
	case "down", "failed", "error", "timeout":
		return "failed"
	default:
		return status
	}
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

func parseCheckoutAnnouncements(data []byte, balanceCents int64, runtimeStatus adminplusdomain.SupplierRuntimeStatus) []ports.ProviderAnnouncement {
	body, ok := unwrapDataObject(data)
	if !ok {
		return nil
	}
	currency := firstNonEmpty(stringFromAny(body["currency"]), "USD")
	announcements := make([]ports.ProviderAnnouncement, 0)
	if multiplier := float64FromAny(firstExisting(body, "balance_recharge_multiplier", "balanceRechargeMultiplier")); multiplier > 1 {
		bonus := (multiplier - 1) * 100
		minRechargeCents := firstCostCentsValue(body, "global_min", "min_amount", "minAmount", "payment_min_amount")
		announcements = append(announcements, ports.ProviderAnnouncement{
			Type:             adminplusdomain.AnnouncementTypeRechargeBonus,
			Title:            fmt.Sprintf("充值倍率公告 %.2f%%", bonus),
			Description:      "供应商充值页显示余额充值倍率大于 1。",
			Currency:         currency,
			MinRechargeCents: minRechargeCents,
			BonusPercent:     &bonus,
			RuntimeStatus:    runtimeStatus,
			BalanceCents:     balanceCents,
			RawPayload: map[string]any{
				"source":                      "provider_announcement",
				"source_detail":               "payment_checkout_info",
				"classification":              "recharge_bonus",
				"matched_keywords":            []string{"recharge_multiplier"},
				"balance_recharge_multiplier": multiplier,
				"recharge_fee_rate":           float64FromAny(body["recharge_fee_rate"]),
				"global_min":                  body["global_min"],
				"global_max":                  body["global_max"],
			},
		})
	}
	if text := strings.TrimSpace(stringFromAny(body["help_text"])); announcementTextLooksUseful(text) {
		classification := classifyAnnouncementText(text)
		announcements = append(announcements, ports.ProviderAnnouncement{
			Type:          classification.Type,
			Title:         titleFromText("充值页公告", text),
			Description:   text,
			Currency:      currency,
			RuntimeStatus: runtimeStatus,
			BalanceCents:  balanceCents,
			RawPayload: map[string]any{
				"source":           "provider_announcement",
				"source_detail":    "payment_checkout_help",
				"classification":   classification.Name,
				"matched_keywords": classification.MatchedKeywords,
			},
		})
	}
	for _, plan := range checkoutPlans(body) {
		announcement, ok := announcementFromCheckoutPlan(plan, currency, balanceCents, runtimeStatus)
		if ok {
			announcements = append(announcements, announcement)
		}
	}
	return announcements
}

func announcementFromCheckoutPlan(plan map[string]any, currency string, balanceCents int64, runtimeStatus adminplusdomain.SupplierRuntimeStatus) (ports.ProviderAnnouncement, bool) {
	name := firstNonEmpty(stringFromAny(plan["name"]), stringFromAny(plan["product_name"]), "Subscription package")
	price := float64FromAny(plan["price"])
	originalPrice, hasOriginal := positiveFloat(firstExisting(plan, "original_price", "originalPrice"))
	if !hasOriginal || originalPrice <= 0 || price <= 0 || price >= originalPrice {
		return ports.ProviderAnnouncement{}, false
	}
	discount := (originalPrice - price) / originalPrice * 100
	return ports.ProviderAnnouncement{
		Type:             adminplusdomain.AnnouncementTypePackageDeal,
		Title:            fmt.Sprintf("%s package discount %.2f%%", name, discount),
		Description:      firstNonEmpty(stringFromAny(plan["description"]), "Supplier checkout plan is priced below original price."),
		Currency:         currency,
		MinRechargeCents: int64(math.Round(price * 100)),
		DiscountPercent:  &discount,
		RuntimeStatus:    runtimeStatus,
		BalanceCents:     balanceCents,
		RawPayload: map[string]any{
			"source":         "provider_announcement",
			"source_detail":  "payment_checkout_plan",
			"classification": "package_deal",
			"plan_id":        firstExisting(plan, "id"),
			"group_id":       firstExisting(plan, "group_id", "groupId"),
			"price":          price,
			"original_price": originalPrice,
		},
	}, true
}

func parseProviderAnnouncements(data []byte, balanceCents int64, runtimeStatus adminplusdomain.SupplierRuntimeStatus) []ports.ProviderAnnouncement {
	values, err := unwrapDataArray(data)
	if err != nil {
		return nil
	}
	announcements := make([]ports.ProviderAnnouncement, 0, len(values))
	for _, value := range values {
		raw, ok := value.(map[string]any)
		if !ok {
			continue
		}
		title := firstNonEmpty(stringFromAny(raw["title"]), stringFromAny(raw["name"]))
		content := firstNonEmpty(stringFromAny(raw["content"]), stringFromAny(raw["description"]), stringFromAny(raw["body"]))
		text := strings.TrimSpace(strings.Join([]string{title, content}, " "))
		if !announcementTextLooksUseful(text) {
			continue
		}
		classification := classifyAnnouncementText(text)
		startsAt, hasStartsAt := firstTimeValue(raw, "starts_at", "startsAt")
		endsAt, hasEndsAt := firstTimeValue(raw, "ends_at", "endsAt")
		var startsAtPtr *time.Time
		if hasStartsAt {
			value := startsAt.UTC()
			startsAtPtr = &value
		}
		var endsAtPtr *time.Time
		if hasEndsAt {
			value := endsAt.UTC()
			endsAtPtr = &value
		}
		announcements = append(announcements, ports.ProviderAnnouncement{
			Type:          classification.Type,
			Title:         firstNonEmpty(title, titleFromText("供应商公告", content)),
			Description:   content,
			Currency:      "USD",
			RuntimeStatus: runtimeStatus,
			BalanceCents:  balanceCents,
			StartsAt:      startsAtPtr,
			EndsAt:        endsAtPtr,
			RawPayload: map[string]any{
				"source":           "provider_announcement",
				"source_detail":    "announcements",
				"classification":   classification.Name,
				"matched_keywords": classification.MatchedKeywords,
				"announcement_id":  firstExisting(raw, "id"),
				"notify_mode":      firstExisting(raw, "notify_mode", "notifyMode"),
			},
		})
	}
	return announcements
}

func unwrapDataObject(data []byte) (map[string]any, bool) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, false
	}
	if obj, ok := root.(map[string]any); ok {
		if dataValue, ok := obj["data"].(map[string]any); ok {
			return dataValue, true
		}
		return obj, true
	}
	return nil, false
}

func checkoutPlans(body map[string]any) []map[string]any {
	values, ok := firstArrayValue(body, "plans", "subscription_plans", "subscriptionPlans")
	if !ok {
		return nil
	}
	plans := make([]map[string]any, 0, len(values))
	for _, value := range values {
		plan, ok := value.(map[string]any)
		if ok {
			plans = append(plans, plan)
		}
	}
	return plans
}

func positiveFloat(value any) (float64, bool) {
	n, ok := float64Value(value)
	return n, ok && n > 0
}

type announcementClassification struct {
	Name            string
	Type            adminplusdomain.AnnouncementType
	MatchedKeywords []string
}

var announcementKeywordPatterns = []struct {
	name     string
	eventTyp adminplusdomain.AnnouncementType
	keywords []string
	pattern  *regexp.Regexp
}{
	{
		name:     "recharge_bonus",
		eventTyp: adminplusdomain.AnnouncementTypeRechargeBonus,
		keywords: []string{"充值", "赠送", "余额", "bonus", "top up", "recharge"},
		pattern:  regexp.MustCompile(`(?i)(充值|赠送|余额|bonus|top\s*up|recharge)`),
	},
	{
		name:     "package_deal",
		eventTyp: adminplusdomain.AnnouncementTypePackageDeal,
		keywords: []string{"套餐", "package", "plan"},
		pattern:  regexp.MustCompile(`(?i)(套餐|package|plan)`),
	},
	{
		name:     "rate_discount",
		eventTyp: adminplusdomain.AnnouncementTypeRateDiscount,
		keywords: []string{"费率", "折扣", "优惠", "discount", "coupon", "promo"},
		pattern:  regexp.MustCompile(`(?i)(费率|折扣|优惠|discount|coupon|promo|promotion)`),
	},
	{
		name:     "maintenance",
		eventTyp: adminplusdomain.AnnouncementTypeMaintenance,
		keywords: []string{"维护", "停机", "maintenance", "downtime"},
		pattern:  regexp.MustCompile(`(?i)(维护|停机|maintenance|downtime)`),
	},
	{
		name:     "incident",
		eventTyp: adminplusdomain.AnnouncementTypeIncident,
		keywords: []string{"故障", "异常", "中断", "incident", "outage"},
		pattern:  regexp.MustCompile(`(?i)(故障|异常|中断|incident|outage)`),
	},
	{
		name:     "limited_offer",
		eventTyp: adminplusdomain.AnnouncementTypeLimitedOffer,
		keywords: []string{"活动", "限时", "返利", "返现", "sale", "rebate"},
		pattern:  regexp.MustCompile(`(?i)(活动|限时|返利|返现|sale|rebate)`),
	},
	{
		name:     "notice",
		eventTyp: adminplusdomain.AnnouncementTypeNotice,
		keywords: []string{"公告", "通知", "notice", "announcement"},
		pattern:  regexp.MustCompile(`(?i)(公告|通知|notice|announcement)`),
	},
}

func announcementTextLooksUseful(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for _, item := range announcementKeywordPatterns {
		if item.pattern.MatchString(text) {
			return true
		}
	}
	return false
}

func classifyAnnouncementText(text string) announcementClassification {
	text = strings.TrimSpace(text)
	for _, item := range announcementKeywordPatterns {
		if !item.pattern.MatchString(text) {
			continue
		}
		return announcementClassification{
			Name:            item.name,
			Type:            item.eventTyp,
			MatchedKeywords: matchedAnnouncementKeywords(text, item.keywords),
		}
	}
	return announcementClassification{Name: "other", Type: adminplusdomain.AnnouncementTypeOther}
}

func matchedAnnouncementKeywords(text string, keywords []string) []string {
	normalized := strings.ToLower(text)
	out := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		if strings.Contains(normalized, strings.ToLower(keyword)) {
			out = append(out, keyword)
		}
	}
	return out
}

func titleFromText(prefix string, text string) string {
	value := strings.TrimSpace(text)
	if value == "" {
		return prefix
	}
	value = strings.ReplaceAll(value, "\n", " ")
	if len(value) > 80 {
		value = value[:80]
	}
	return firstNonEmpty(value, prefix)
}

func dedupeAnnouncements(announcements []ports.ProviderAnnouncement) []ports.ProviderAnnouncement {
	if len(announcements) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(announcements))
	out := make([]ports.ProviderAnnouncement, 0, len(announcements))
	for _, announcement := range announcements {
		key := strings.ToLower(strings.Join([]string{
			string(announcement.Type),
			announcement.Title,
			fmt.Sprintf("%d", announcement.MinRechargeCents),
			fmt.Sprintf("%.4f", floatFromPtr(announcement.BonusPercent)),
			fmt.Sprintf("%.4f", floatFromPtr(announcement.DiscountPercent)),
		}, "\x00"))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, announcement)
	}
	return out
}

func floatFromPtr(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
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

func providerKeyMatchesRequest(raw map[string]any, request ports.CreateProviderKeyInput) bool {
	name := strings.TrimSpace(request.Name)
	if name != "" && strings.TrimSpace(stringFromAny(raw["name"])) != name {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(stringFromAny(raw["status"])))
	if status != "" && status != "active" {
		return false
	}
	expectedGroupID := strings.TrimSpace(request.ExternalGroupID)
	if expectedGroupID == "" {
		return true
	}
	actualGroupID := providerKeyGroupID(raw)
	return actualGroupID == "" || actualGroupID == expectedGroupID
}

func providerKeyGroupID(raw map[string]any) string {
	groupID := firstNonEmpty(stringFromAny(raw["group_id"]), idStringFromAny(raw["group_id"]))
	if groupID != "" && groupID != "0" {
		return groupID
	}
	if group := firstMapValue(raw, "group", "Group"); len(group) > 0 {
		groupID = firstNonEmpty(stringFromAny(group["id"]), idStringFromAny(group["id"]))
		if groupID != "" && groupID != "0" {
			return groupID
		}
	}
	if routes, ok := firstArrayValue(raw, "group_routes", "groupRoutes", "GroupRoutes"); ok {
		for _, value := range routes {
			route, ok := value.(map[string]any)
			if !ok {
				continue
			}
			groupID = firstNonEmpty(stringFromAny(route["group_id"]), idStringFromAny(route["group_id"]))
			if groupID != "" && groupID != "0" {
				return groupID
			}
		}
	}
	return ""
}

func bodyFromMap(raw map[string]any) []byte {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	return data
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
		if platforms, ok := firstArrayValue(channel, "platforms", "Platforms"); ok {
			for _, platformValue := range platforms {
				platform, ok := platformValue.(map[string]any)
				if !ok {
					continue
				}
				entries = append(entries, parseSupportedModelRatesWithContext(channel, platform)...)
				entries = append(entries, parseModelPricingRatesWithContext(channel, platform)...)
			}
		}
	}
	return dedupeRateEntries(entries)
}

func parseSupportedModelRates(channel map[string]any) []ports.ProviderRateEntry {
	return parseSupportedModelRatesWithContext(nil, channel)
}

func parseSupportedModelRatesWithContext(channel map[string]any, platform map[string]any) []ports.ProviderRateEntry {
	if len(platform) == 0 {
		return nil
	}
	values, ok := firstArrayValue(platform, "supported_models", "supportedModels", "SupportedModels")
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
		out = append(out, priceEntriesFromPricing(modelName, pricing, mergeRawPayload(channel, platform, model, pricing))...)
	}
	return out
}

func parseModelPricingRates(channel map[string]any) []ports.ProviderRateEntry {
	return parseModelPricingRatesWithContext(nil, channel)
}

func parseModelPricingRatesWithContext(channel map[string]any, platform map[string]any) []ports.ProviderRateEntry {
	if len(platform) == 0 {
		return nil
	}
	values, ok := firstArrayValue(platform, "model_pricing", "modelPricing", "ModelPricing", "pricing", "Pricing")
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
			out = append(out, priceEntriesFromPricing(model, pricing, mergeRawPayload(channel, platform, pricing))...)
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
	addMicros("image_input", "1m_tokens", firstExisting(pricing, "image_input_price_micros", "imageInputPriceMicros", "ImageInputPriceMicros"))
	addMicros("image_cache_read", "1m_tokens", firstExisting(pricing, "image_cache_read_price_micros", "imageCacheReadPriceMicros", "ImageCacheReadPriceMicros"))
	addMicros("image_output", "1m_tokens", firstExisting(pricing, "image_output_price_micros", "imageOutputPriceMicros", "ImageOutputPriceMicros"))
	addMicros("per_request", "request", firstExisting(pricing, "per_request_price_micros", "perRequestPriceMicros", "PerRequestPriceMicros"))

	addTokenUSD("input", firstExisting(pricing, "input_price", "inputPrice", "InputPrice"))
	addTokenUSD("output", firstExisting(pricing, "output_price", "outputPrice", "OutputPrice"))
	addTokenUSD("cache_write", firstExisting(pricing, "cache_write_price", "cacheWritePrice", "CacheWritePrice"))
	addTokenUSD("cache_read", firstExisting(pricing, "cache_read_price", "cacheReadPrice", "CacheReadPrice"))
	addTokenUSD("image_input", firstExisting(pricing, "image_input_price", "imageInputPrice", "ImageInputPrice"))
	addTokenUSD("image_cache_read", firstExisting(pricing, "image_cache_read_price", "imageCacheReadPrice", "ImageCacheReadPrice"))
	addTokenUSD("image_output", firstExisting(pricing, "image_output_price", "imageOutputPrice", "ImageOutputPrice"))
	addRequestUSD(firstExisting(pricing, "per_request_price", "perRequestPrice", "PerRequestPrice"))
	return out
}

func appendUsageCostQuery(endpoint string, request ports.ReadUsageCostsInput) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	values := u.Query()
	startedAt := request.StartedAt.UTC().Format(time.RFC3339)
	endedAt := request.EndedAt.UTC().Format(time.RFC3339)
	values.Set("started_at", startedAt)
	values.Set("ended_at", endedAt)
	values.Set("from", startedAt)
	values.Set("to", endedAt)
	u.RawQuery = values.Encode()
	return u.String()
}

func appendQueryValues(endpoint string, pairs map[string]string) string {
	if len(pairs) == 0 {
		return endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	values := u.Query()
	for key, value := range pairs {
		values.Set(key, value)
	}
	u.RawQuery = values.Encode()
	return u.String()
}

func parseSub2APIUsageCostLines(data []byte) []ports.ProviderUsageCostLine {
	values, err := unwrapDataArray(data)
	if err != nil {
		return nil
	}
	lines := make([]ports.ProviderUsageCostLine, 0, len(values))
	for _, value := range values {
		raw, ok := value.(map[string]any)
		if !ok {
			continue
		}
		line, ok := parseSub2APIUsageCostLine(raw)
		if ok {
			lines = append(lines, line)
		}
	}
	return lines
}

func parseSub2APIFundingTransactions(data []byte) []ports.ProviderFundingTransaction {
	values, err := unwrapDataArray(data)
	if err != nil {
		return nil
	}
	items := make([]ports.ProviderFundingTransaction, 0, len(values))
	for _, value := range values {
		raw, ok := value.(map[string]any)
		if !ok {
			continue
		}
		item, ok := parseSub2APIFundingTransaction(raw)
		if ok {
			items = append(items, item)
		}
	}
	return items
}

func parseSub2APIFundingTransaction(raw map[string]any) (ports.ProviderFundingTransaction, bool) {
	externalID := firstNonEmpty(
		stringFromAny(raw["external_order_id"]),
		stringFromAny(raw["externalOrderId"]),
		stringFromAny(raw["order_id"]),
		stringFromAny(raw["orderId"]),
		stringFromAny(raw["id"]),
		idStringFromAny(raw["id"]),
	)
	if externalID == "" {
		return ports.ProviderFundingTransaction{}, false
	}
	createdAt, hasCreatedAt := firstTimeValue(raw, "created_at", "createdAt", "created_time", "createdTime")
	paidAt, hasPaidAt := firstTimeValue(raw, "paid_at", "paidAt", "pay_at", "payAt", "paid_time", "paidTime")
	completedAt, hasCompletedAt := firstTimeValue(raw, "completed_at", "completedAt", "completed_time", "completedTime", "updated_at", "updatedAt")
	item := ports.ProviderFundingTransaction{
		ExternalID: firstNonEmpty(externalID),
		OutTradeNo: firstNonEmpty(
			stringFromAny(raw["out_trade_no"]),
			stringFromAny(raw["outTradeNo"]),
			stringFromAny(raw["trade_no"]),
			stringFromAny(raw["tradeNo"]),
		),
		PaymentTradeNo: maskTail(firstNonEmpty(
			stringFromAny(raw["payment_trade_no"]),
			stringFromAny(raw["paymentTradeNo"]),
			stringFromAny(raw["transaction_id"]),
			stringFromAny(raw["transactionId"]),
		), 6),
		PaymentType: firstNonEmpty(stringFromAny(raw["payment_type"]), stringFromAny(raw["paymentType"]), stringFromAny(raw["pay_type"]), stringFromAny(raw["payType"])),
		OrderType:   firstNonEmpty(stringFromAny(raw["order_type"]), stringFromAny(raw["orderType"]), stringFromAny(raw["type"])),
		Status:      strings.ToUpper(firstNonEmpty(stringFromAny(raw["status"]), stringFromAny(raw["state"]), "UNKNOWN")),
		Currency:    firstNonEmpty(stringFromAny(raw["currency"]), stringFromAny(raw["Currency"]), "USD"),
		AmountCents: firstCostCentsValue(raw,
			"amount_cents", "amountCents", "balance_amount_cents", "balanceAmountCents",
			"amount", "balance_amount", "balanceAmount", "actual_amount", "actualAmount",
		),
		CashAmountCents: firstCostCentsValue(raw,
			"pay_amount_cents", "payAmountCents", "cash_amount_cents", "cashAmountCents",
			"pay_amount", "payAmount", "cash_amount", "cashAmount", "payment_amount", "paymentAmount",
		),
		RefundAmountCents: firstCostCentsValue(raw,
			"refund_amount_cents", "refundAmountCents",
			"refund_amount", "refundAmount",
		),
		FeeRate:    optionalFloat64Ptr(raw, "fee_rate", "feeRate"),
		RawPayload: sanitizeUsageCostPayload(raw),
	}
	if item.CashAmountCents == 0 {
		item.CashAmountCents = item.AmountCents
	}
	if hasCreatedAt {
		t := createdAt.UTC()
		item.CreatedAtExternal = &t
	}
	if hasPaidAt {
		t := paidAt.UTC()
		item.PaidAt = &t
	}
	if hasCompletedAt {
		t := completedAt.UTC()
		item.CompletedAt = &t
	}
	return item, true
}

func parseSub2APIEntitlementTransactions(data []byte) []ports.ProviderEntitlementTransaction {
	values, err := unwrapDataArray(data)
	if err != nil {
		return nil
	}
	items := make([]ports.ProviderEntitlementTransaction, 0, len(values))
	for _, value := range values {
		raw, ok := value.(map[string]any)
		if !ok {
			continue
		}
		item, ok := parseSub2APIEntitlementTransaction(raw)
		if ok {
			items = append(items, item)
		}
	}
	return items
}

func parseSub2APIEntitlementTransaction(raw map[string]any) (ports.ProviderEntitlementTransaction, bool) {
	code := firstNonEmpty(
		stringFromAny(raw["code"]),
		stringFromAny(raw["redeem_code"]),
		stringFromAny(raw["redeemCode"]),
	)
	externalID := firstNonEmpty(
		stringFromAny(raw["external_redeem_id"]),
		stringFromAny(raw["externalRedeemId"]),
		stringFromAny(raw["redeem_id"]),
		stringFromAny(raw["redeemId"]),
		stringFromAny(raw["id"]),
		idStringFromAny(raw["id"]),
		codeFingerprint(code),
	)
	if externalID == "" {
		return ports.ProviderEntitlementTransaction{}, false
	}
	usedAt, hasUsedAt := firstTimeValue(raw, "used_at", "usedAt", "redeemed_at", "redeemedAt", "updated_at", "updatedAt")
	createdAt, hasCreatedAt := firstTimeValue(raw, "created_at", "createdAt")
	rawValue := float64FromAny(firstExisting(raw, "raw_value", "rawValue", "value", "amount", "balance", "quota"))
	itemType := firstNonEmpty(stringFromAny(raw["type"]), stringFromAny(raw["redeem_type"]), stringFromAny(raw["redeemType"]), "balance")
	item := ports.ProviderEntitlementTransaction{
		ExternalID:      externalID,
		CodeFingerprint: codeFingerprint(code),
		CodeLast4:       lastN(code, 4),
		SourceFamily:    entitlementSourceFamily(code),
		Type:            itemType,
		Status:          strings.ToLower(firstNonEmpty(stringFromAny(raw["status"]), stringFromAny(raw["state"]), "used")),
		Currency:        firstNonEmpty(stringFromAny(raw["currency"]), stringFromAny(raw["Currency"]), "USD"),
		ValueCents: firstCostCentsValue(raw,
			"value_cents", "valueCents", "amount_cents", "amountCents", "balance_cents", "balanceCents",
			"value", "amount", "balance", "quota",
		),
		RawValue:     rawValue,
		GroupID:      int64FromAny(firstExisting(raw, "group_id", "groupId")),
		ValidityDays: int(int64FromAny(firstExisting(raw, "validity_days", "validityDays", "days"))),
		RawPayload:   sanitizeUsageCostPayload(raw),
	}
	if hasUsedAt {
		t := usedAt.UTC()
		item.UsedAt = &t
	}
	if hasCreatedAt {
		t := createdAt.UTC()
		item.CreatedAtExternal = &t
	}
	return item, true
}

func parseSub2APIUsageCostLine(raw map[string]any) (ports.ProviderUsageCostLine, bool) {
	model := firstNonEmpty(
		stringFromAny(raw["model"]),
		stringFromAny(raw["requested_model"]),
		stringFromAny(raw["requestedModel"]),
		stringFromAny(raw["upstream_model"]),
		stringFromAny(raw["upstreamModel"]),
		stringFromAny(raw["billing_model"]),
		stringFromAny(raw["billingModel"]),
	)
	startedAt, ok := firstTimeValue(raw, "started_at", "startedAt", "created_at", "createdAt", "timestamp", "time")
	if model == "" || !ok {
		return ports.ProviderUsageCostLine{}, false
	}
	endedAt, hasEndedAt := firstTimeValue(raw, "ended_at", "endedAt", "completed_at", "completedAt", "finished_at", "finishedAt")
	var endedAtPtr *time.Time
	if hasEndedAt {
		endedAtUTC := endedAt.UTC()
		endedAtPtr = &endedAtUTC
	}
	return ports.ProviderUsageCostLine{
		ExternalUsageCostID: firstNonEmpty(
			stringFromAny(raw["external_usage_cost_id"]),
			stringFromAny(raw["externalUsageCostId"]),
			stringFromAny(raw["external_bill_id"]),
			stringFromAny(raw["externalBillId"]),
			stringFromAny(raw["bill_id"]),
			stringFromAny(raw["billId"]),
			stringFromAny(raw["billing_id"]),
			stringFromAny(raw["billingId"]),
			stringFromAny(raw["id"]),
			idStringFromAny(raw["id"]),
		),
		ExternalRequestID: firstNonEmpty(
			stringFromAny(raw["external_request_id"]),
			stringFromAny(raw["externalRequestId"]),
			stringFromAny(raw["request_id"]),
			stringFromAny(raw["requestId"]),
			stringFromAny(raw["upstream_request_id"]),
			stringFromAny(raw["upstreamRequestId"]),
		),
		APIKeyName: firstNonEmpty(
			stringFromAny(raw["api_key_name"]),
			stringFromAny(raw["apiKeyName"]),
			stringFromAny(raw["key_name"]),
			stringFromAny(raw["keyName"]),
			stringFromAny(raw["token_name"]),
			stringFromAny(raw["tokenName"]),
		),
		Model:           model,
		Endpoint:        firstNonEmpty(stringFromAny(raw["endpoint"]), stringFromAny(raw["path"]), stringFromAny(raw["inbound_endpoint"]), stringFromAny(raw["upstream_endpoint"])),
		RequestType:     firstNonEmpty(stringFromAny(raw["request_type"]), stringFromAny(raw["requestType"]), stringFromAny(raw["type"])),
		BillingMode:     firstNonEmpty(stringFromAny(raw["billing_mode"]), stringFromAny(raw["billingMode"]), stringFromAny(raw["billing_type"]), stringFromAny(raw["billingType"]), "token"),
		ReasoningEffort: firstNonEmpty(stringFromAny(raw["reasoning_effort"]), stringFromAny(raw["reasoningEffort"]), stringFromAny(raw["service_tier"]), stringFromAny(raw["serviceTier"])),
		Currency:        firstNonEmpty(stringFromAny(raw["currency"]), stringFromAny(raw["Currency"]), "USD"),
		CostCents: firstCostCentsValue(raw,
			"cost_cents", "costCents", "amount_cents", "amountCents", "fee_cents", "feeCents",
			"cost", "amount", "fee", "actual_cost", "actualCost", "total_cost", "totalCost",
		),
		InputTokens:     firstInt64Value(raw, "input_tokens", "inputTokens", "prompt_tokens", "promptTokens"),
		OutputTokens:    firstInt64Value(raw, "output_tokens", "outputTokens", "completion_tokens", "completionTokens"),
		CacheReadTokens: firstInt64Value(raw, "cache_read_tokens", "cacheReadTokens", "cached_tokens", "cachedTokens"),
		TotalTokens:     firstInt64Value(raw, "total_tokens", "totalTokens", "tokens"),
		FirstTokenMS:    firstInt64Value(raw, "first_token_ms", "firstTokenMs", "first_token_latency_ms", "firstTokenLatencyMs"),
		DurationMS:      firstInt64Value(raw, "duration_ms", "durationMs", "total_latency_ms", "totalLatencyMs", "latency_ms", "latencyMs"),
		UserAgent:       firstNonEmpty(stringFromAny(raw["user_agent"]), stringFromAny(raw["userAgent"])),
		StartedAt:       startedAt.UTC(),
		EndedAt:         endedAtPtr,
		RawPayload:      sanitizeUsageCostPayload(raw),
	}, true
}

func firstInt64Value(in map[string]any, keys ...string) int64 {
	for _, key := range keys {
		value, exists := in[key]
		if !exists || value == nil {
			continue
		}
		if n, ok := int64Value(value); ok {
			return n
		}
		if n, ok := numericStringToInt64(value); ok {
			return n
		}
	}
	return 0
}

func idStringFromAny(value any) string {
	n := int64FromAny(value)
	if n <= 0 {
		return ""
	}
	return strconv.FormatInt(n, 10)
}

func firstCostCentsValue(in map[string]any, keys ...string) int64 {
	for _, key := range keys {
		value, exists := in[key]
		if !exists || value == nil {
			continue
		}
		switch key {
		case "cost_cents", "costCents", "amount_cents", "amountCents", "fee_cents", "feeCents",
			"balance_amount_cents", "balanceAmountCents", "pay_amount_cents", "payAmountCents",
			"cash_amount_cents", "cashAmountCents", "refund_amount_cents", "refundAmountCents",
			"value_cents", "valueCents", "balance_cents", "balanceCents":
			if n, ok := int64Value(value); ok {
				return n
			}
			if n, ok := numericStringToInt64(value); ok {
				return n
			}
		default:
			if n, ok := currencyAmountToCents(value); ok {
				return n
			}
		}
	}
	return 0
}

func int64Value(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(math.Round(v)), true
	case json.Number:
		n, err := v.Int64()
		if err == nil {
			return n, true
		}
		f, err := v.Float64()
		if err != nil {
			return 0, false
		}
		return int64(math.Round(f)), true
	default:
		return 0, false
	}
}

func numericStringToInt64(value any) (int64, bool) {
	s := stringFromAny(value)
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		return n, true
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return int64(math.Round(f)), true
}

func currencyAmountToCents(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v * 100), true
	case int64:
		return v * 100, true
	case float64:
		return int64(math.Round(v * 100)), true
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, false
		}
		return int64(math.Round(f * 100)), true
	case string:
		s := strings.TrimSpace(v)
		s = strings.TrimLeft(s, "$¥￥€£ ")
		s = strings.ReplaceAll(s, ",", "")
		if s == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return int64(math.Round(f * 100)), true
	default:
		return 0, false
	}
}

func firstTimeValue(in map[string]any, keys ...string) (time.Time, bool) {
	for _, key := range keys {
		value, exists := in[key]
		if !exists || value == nil {
			continue
		}
		if t, ok := timeFromAny(value); ok {
			return t, true
		}
	}
	return time.Time{}, false
}

func timeFromAny(value any) (time.Time, bool) {
	switch v := value.(type) {
	case time.Time:
		return v, !v.IsZero()
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return time.Time{}, false
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
			t, err := time.Parse(layout, s)
			if err == nil {
				return t, true
			}
		}
		return time.Time{}, false
	case int64:
		return unixTimeFromNumber(v)
	case int:
		return unixTimeFromNumber(int64(v))
	case float64:
		return unixTimeFromNumber(int64(v))
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return time.Time{}, false
		}
		return unixTimeFromNumber(n)
	default:
		return time.Time{}, false
	}
}

func unixTimeFromNumber(value int64) (time.Time, bool) {
	if value <= 0 {
		return time.Time{}, false
	}
	if value > 1_000_000_000_000 {
		return time.UnixMilli(value).UTC(), true
	}
	return time.Unix(value, 0).UTC(), true
}

func sanitizeUsageCostPayload(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		if isSensitivePayloadKey(key) {
			continue
		}
		out[key] = sanitizeUsageCostValue(value)
	}
	return out
}

func sanitizeUsageCostValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return sanitizeUsageCostPayload(v)
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, sanitizeUsageCostValue(item))
		}
		return out
	default:
		return value
	}
}

func isSensitivePayloadKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
	switch normalized {
	case "authorization", "cookie", "set_cookie", "password", "secret", "token", "access_token", "refresh_token", "csrf_token", "xsrf_token", "api_key", "apikey", "key":
		return true
	default:
		return strings.Contains(normalized, "authorization") ||
			strings.Contains(normalized, "cookie") ||
			strings.Contains(normalized, "password") ||
			strings.Contains(normalized, "secret") ||
			strings.Contains(normalized, "access_token") ||
			strings.Contains(normalized, "refresh_token")
	}
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

func originFromRawURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
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

func optionalInt64FromAny(value any) *int64 {
	if value == nil {
		return nil
	}
	switch value.(type) {
	case int, int64, float64, json.Number:
		n := int64FromAny(value)
		return &n
	case string:
		text := strings.TrimSpace(stringFromAny(value))
		if text == "" {
			return nil
		}
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return nil
		}
		return &n
	default:
		return nil
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

func optionalFloat64Ptr(in map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		if value := optionalFloat64(in, key); value != nil {
			return value
		}
	}
	return nil
}

func codeFingerprint(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(code))
	return fmt.Sprintf("%x", sum[:])
}

func entitlementSourceFamily(code string) string {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if strings.HasPrefix(normalized, "PAY") {
		return "payment_auto_redeem"
	}
	return "manual_redeem"
}

func lastN(value string, n int) string {
	value = strings.TrimSpace(value)
	if value == "" || n <= 0 {
		return ""
	}
	if len(value) <= n {
		return value
	}
	return value[len(value)-n:]
}

func maskTail(value string, tail int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if tail <= 0 || len(value) <= tail {
		return value
	}
	return strings.Repeat("*", len(value)-tail) + value[len(value)-tail:]
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
