package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyurl"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyutil"
)

const browserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"

type Client struct {
	httpClient *http.Client
	now        func() time.Time
}

func NewClient(client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	return &Client{httpClient: client, now: time.Now}
}

func (c *Client) httpClientForProxy(rawProxyURL string) (*http.Client, error) {
	trimmed, parsed, err := proxyurl.Parse(rawProxyURL)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadRequest, "SUPPLIER_PROXY_URL_INVALID", "supplier proxy url is invalid").WithCause(err)
	}
	if trimmed == "" {
		return c.httpClient, nil
	}
	transport := &http.Transport{}
	if err := proxyutil.ConfigureTransportProxy(transport, parsed); err != nil {
		return nil, infraerrors.New(http.StatusBadRequest, "SUPPLIER_PROXY_URL_INVALID", "supplier proxy url is invalid").WithCause(err)
	}
	return &http.Client{Transport: transport, Timeout: c.httpClient.Timeout}, nil
}

func (c *Client) DirectLogin(ctx context.Context, in ports.DirectLoginInput) (*ports.DirectLoginResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "new api provider adapter is not configured")
	}
	if nonBlankSecret(in.Token) {
		return c.directLoginFromToken(ctx, in)
	}
	if strings.TrimSpace(in.Username) == "" || !nonBlankSecret(in.Password) {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED", "new api username and password are required")
	}
	apiBaseURL, origin, err := normalizeBaseURLs(in.APIBaseURL, in.Origin)
	if err != nil {
		return nil, err
	}
	loginEndpoint, err := buildEndpointURL(apiBaseURL, "/api/user/login")
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"username": strings.TrimSpace(in.Username),
		"password": in.Password,
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
	if origin != "" {
		req.Header.Set("Origin", origin)
		req.Header.Set("Referer", strings.TrimRight(origin, "/")+"/")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, withRequestDiagnostics(err, loginEndpoint, "SUPPLIER_DIRECT_LOGIN_FAILED", "new api login endpoint is unreachable")
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, classifyLoginFailure(resp.StatusCode, data)
	}
	envelope, err := decodeEnvelope(data)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_RESPONSE_INVALID", "new api login response is invalid").WithCause(err)
	}
	if !envelope.Success {
		return nil, classifyBusinessFailure(envelope.Message)
	}
	if boolFromAny(envelope.Data["require_2fa"]) {
		return nil, infraerrors.New(http.StatusConflict, "LOGIN_MFA_REQUIRED", "new api direct login requires 2FA; use browser extension fallback")
	}
	userID := int64FromAny(envelope.Data["id"])
	if userID <= 0 {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_RESPONSE_INVALID", "new api login response did not include user id")
	}
	capturedAt := c.now().UTC()
	cookies := cookiesFromResponse(resp, apiBaseURL)
	if len(cookies) == 0 {
		accessToken := newAPILoginAccessToken(envelope)
		if accessToken == "" {
			return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_RESPONSE_INVALID", "new api login response did not include session cookie")
		}
		bundle := buildAccessTokenSessionBundle(in.SupplierID, origin, apiBaseURL, strconv.FormatInt(userID, 10), accessToken, capturedAt)
		probe, err := c.ProbeSub2APIUserProfile(ctx, ports.SessionProbeInput{
			SupplierID: in.SupplierID,
			Origin:     origin,
			APIBaseURL: apiBaseURL,
			Bundle:     bundle,
		})
		if err != nil {
			return nil, err
		}
		applyProfileToSessionBundle(bundle, probe)
		return &ports.DirectLoginResult{
			SupplierID:    in.SupplierID,
			Origin:        origin,
			APIBaseURL:    apiBaseURL,
			SessionBundle: bundle,
			CapturedAt:    capturedAt,
			Diagnostics: map[string]any{
				"login_endpoint":       loginEndpoint,
				"login_method":         "access_token_from_login_response",
				"profile_status":       stringFromProbeStatus(probe),
				"auth_header_required": "New-Api-User",
				"login_response_keys":  rawKeys(envelope.Data),
			},
		}, nil
	}
	expiresAt := expiresAtFromCookies(resp.Cookies(), capturedAt)
	bundle := buildSessionBundle(in.SupplierID, origin, apiBaseURL, userID, envelope.Data, cookies, capturedAt, expiresAt)
	diagnostics := map[string]any{
		"login_endpoint":       loginEndpoint,
		"auth_header_required": "New-Api-User",
		"login_response_keys":  rawKeys(envelope.Data),
	}
	probe, err := c.ProbeSub2APIUserProfile(ctx, ports.SessionProbeInput{
		SupplierID: in.SupplierID,
		Origin:     origin,
		APIBaseURL: apiBaseURL,
		Bundle:     bundle,
	})
	if err == nil {
		applyProfileToSessionBundle(bundle, probe)
		diagnostics["profile_status"] = stringFromProbeStatus(probe)
	} else if isOptionalNewAPIDirectLoginProbeError(err) {
		diagnostics["profile_status"] = "unverified"
		diagnostics["profile_probe_failed"] = true
		diagnostics["profile_probe_reason"] = infraerrors.Reason(err)
		diagnostics["profile_probe_message"] = infraerrors.Message(err)
	} else {
		return nil, err
	}
	return &ports.DirectLoginResult{
		SupplierID:    in.SupplierID,
		Origin:        origin,
		APIBaseURL:    apiBaseURL,
		SessionBundle: bundle,
		CapturedAt:    capturedAt,
		ExpiresAt:     expiresAt,
		Diagnostics:   diagnostics,
	}, nil
}

func isOptionalNewAPIDirectLoginProbeError(err error) bool {
	return infraerrors.Reason(err) == "SUPPLIER_SESSION_PERMISSION_DENIED"
}

func newAPILoginAccessToken(envelope *apiEnvelope) string {
	if envelope == nil {
		return ""
	}
	token := firstNonEmpty(
		stringFromAny(envelope.Data["access_token"]),
		stringFromAny(envelope.Data["accessToken"]),
		stringFromAny(envelope.Data["auth_token"]),
		stringFromAny(envelope.Data["authToken"]),
		stringFromAny(envelope.Data["token"]),
		stringFromAny(envelope.Data["authorization"]),
		stringFromAny(envelope.Raw["access_token"]),
		stringFromAny(envelope.Raw["accessToken"]),
		stringFromAny(envelope.Raw["auth_token"]),
		stringFromAny(envelope.Raw["authToken"]),
		stringFromAny(envelope.Raw["token"]),
		stringFromAny(envelope.Raw["authorization"]),
	)
	if token == "" {
		if data, ok := envelope.Raw["data"].(string); ok {
			token = data
		}
	}
	return normalizeAccessTokenValue(token)
}

func (c *Client) directLoginFromToken(ctx context.Context, in ports.DirectLoginInput) (*ports.DirectLoginResult, error) {
	apiBaseURL, origin, err := normalizeBaseURLs(in.APIBaseURL, in.Origin)
	if err != nil {
		return nil, err
	}
	accessToken, userID, err := parseNewAPIAccessTokenCredential(in.Token, in.Username, in.LoginContext)
	if err != nil {
		return nil, err
	}
	capturedAt := c.now().UTC()
	bundle := buildAccessTokenSessionBundle(in.SupplierID, origin, apiBaseURL, userID, accessToken, capturedAt)
	probe, err := c.ProbeSub2APIUserProfile(ctx, ports.SessionProbeInput{
		SupplierID: in.SupplierID,
		Origin:     origin,
		APIBaseURL: apiBaseURL,
		Bundle:     bundle,
	})
	if err != nil {
		return nil, err
	}
	applyProfileToSessionBundle(bundle, probe)
	return &ports.DirectLoginResult{
		SupplierID:    in.SupplierID,
		Origin:        origin,
		APIBaseURL:    apiBaseURL,
		SessionBundle: bundle,
		CapturedAt:    capturedAt,
		Diagnostics: map[string]any{
			"login_method":         "access_token",
			"token_present":        true,
			"profile_status":       stringFromProbeStatus(probe),
			"auth_header_required": "New-Api-User",
		},
	}, nil
}

func parseNewAPIAccessTokenCredential(rawToken string, username string, loginContext map[string]any) (string, string, error) {
	token := strings.TrimSpace(rawToken)
	userID := firstNonEmpty(
		newAPIUserIDFromContext(loginContext),
		numericText(username),
	)
	if token == "" {
		return "", "", infraerrors.New(http.StatusConflict, "SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED", "new api access token is required")
	}
	token, userID = parseNewAPITokenHeaderBlock(token, userID)
	if strings.HasPrefix(strings.TrimSpace(token), "{") {
		if parsedToken, parsedUserID := parseNewAPITokenJSON(token); parsedToken != "" {
			token = parsedToken
			userID = firstNonEmpty(userID, parsedUserID)
		}
	}
	if candidateToken, candidateUserID := parseNewAPIColonToken(token); candidateToken != "" {
		token = candidateToken
		userID = firstNonEmpty(userID, candidateUserID)
	}
	token = normalizeAccessTokenValue(token)
	userID = firstNonEmpty(userID, newAPIUserIDFromAccessToken(token))
	if token == "" {
		return "", "", infraerrors.New(http.StatusConflict, "SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED", "new api access token is required")
	}
	if userID == "" {
		return "", "", infraerrors.New(
			http.StatusConflict,
			"SUPPLIER_DIRECT_LOGIN_NEW_API_USER_REQUIRED",
			"new api access token login requires New-Api-User; put the numeric user id in login username or provide token JSON with user_id and access_token",
		)
	}
	return token, userID, nil
}

func parseNewAPITokenHeaderBlock(raw string, userID string) (string, string) {
	token := raw
	for _, line := range strings.Split(raw, "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "authorization":
			if strings.TrimSpace(value) != "" {
				token = strings.TrimSpace(value)
			}
		case "new-api-user", "new_api_user", "newapiuser", "veloera-user", "voapi-user", "user-id", "x-user-id", "rix-api-user", "neo-api-user":
			userID = firstNonEmpty(userID, numericText(value))
		}
	}
	return token, userID
}

func parseNewAPITokenJSON(raw string) (string, string) {
	var root map[string]any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return "", ""
	}
	token := firstNonEmpty(
		stringFromAny(root["access_token"]),
		stringFromAny(root["accessToken"]),
		stringFromAny(root["auth_token"]),
		stringFromAny(root["token"]),
		stringFromAny(root["authorization"]),
	)
	if token == "" {
		if data, ok := root["data"].(string); ok {
			token = data
		} else if data, ok := root["data"].(map[string]any); ok {
			token = firstNonEmpty(
				stringFromAny(data["access_token"]),
				stringFromAny(data["accessToken"]),
				stringFromAny(data["auth_token"]),
				stringFromAny(data["token"]),
				stringFromAny(data["authorization"]),
			)
		}
	}
	userID := firstNonEmpty(
		newAPIIntLikeText(root["user_id"]),
		newAPIIntLikeText(root["userID"]),
		newAPIIntLikeText(root["uid"]),
		newAPIIntLikeText(root["new_api_user"]),
		newAPIIntLikeText(root["newApiUser"]),
		newAPIIntLikeText(root["New-Api-User"]),
		newAPIIntLikeText(root["New-API-User"]),
		newAPIIntLikeText(root["new-api-user"]),
		newAPIIntLikeText(root["Veloera-User"]),
		newAPIIntLikeText(root["voapi-user"]),
		newAPIIntLikeText(root["User-id"]),
		newAPIIntLikeText(root["X-User-Id"]),
		newAPIIntLikeText(root["Rix-Api-User"]),
		newAPIIntLikeText(root["neo-api-user"]),
	)
	if userID == "" {
		if user, ok := root["user"].(map[string]any); ok {
			userID = firstNonEmpty(newAPIIntLikeText(user["id"]), newAPIIntLikeText(user["user_id"]), newAPIIntLikeText(user["uid"]))
		}
	}
	if userID == "" {
		if data, ok := root["data"].(map[string]any); ok {
			userID = firstNonEmpty(
				newAPIIntLikeText(data["id"]),
				newAPIIntLikeText(data["sub"]),
				newAPIIntLikeText(data["user_id"]),
				newAPIIntLikeText(data["userID"]),
				newAPIIntLikeText(data["uid"]),
			)
			if userID == "" {
				if user, ok := data["user"].(map[string]any); ok {
					userID = firstNonEmpty(newAPIIntLikeText(user["id"]), newAPIIntLikeText(user["user_id"]), newAPIIntLikeText(user["uid"]))
				}
			}
		}
	}
	return token, userID
}

func parseNewAPIColonToken(raw string) (string, string) {
	left, right, ok := strings.Cut(strings.TrimSpace(raw), ":")
	if !ok {
		return "", ""
	}
	if userID := numericText(left); userID != "" && strings.TrimSpace(right) != "" {
		return strings.TrimSpace(right), userID
	}
	return "", ""
}

func normalizeAccessTokenValue(value string) string {
	token := strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	return token
}

func newAPIUserIDFromAccessToken(token string) string {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
	}
	if err != nil {
		return ""
	}
	var body map[string]any
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&body); err != nil {
		return ""
	}
	return firstNonEmpty(
		newAPIIntLikeText(body["id"]),
		newAPIIntLikeText(body["sub"]),
		newAPIIntLikeText(body["user_id"]),
		newAPIIntLikeText(body["userID"]),
		newAPIIntLikeText(body["uid"]),
	)
}

func newAPIUserIDFromContext(context map[string]any) string {
	return firstNonEmpty(
		newAPIIntLikeText(context["user_id"]),
		newAPIIntLikeText(context["uid"]),
		newAPIIntLikeText(context["new_api_user"]),
		newAPIIntLikeText(context["newApiUser"]),
		newAPIIntLikeText(context["New-Api-User"]),
		newAPIIntLikeText(context["New-API-User"]),
		newAPIIntLikeText(context["new-api-user"]),
		newAPIIntLikeText(context["Veloera-User"]),
		newAPIIntLikeText(context["voapi-user"]),
		newAPIIntLikeText(context["User-id"]),
		newAPIIntLikeText(context["X-User-Id"]),
		newAPIIntLikeText(context["Rix-Api-User"]),
		newAPIIntLikeText(context["neo-api-user"]),
	)
}

func newAPIIntLikeText(value any) string {
	if text := numericText(stringFromAny(value)); text != "" {
		return text
	}
	if n := int64FromAny(value); n > 0 {
		return strconv.FormatInt(n, 10)
	}
	return ""
}

func numericText(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil || n <= 0 {
		return ""
	}
	return strconv.FormatInt(n, 10)
}

func (c *Client) doSessionJSON(ctx context.Context, method string, endpoint string, bundle map[string]any) ([]byte, error) {
	data, statusCode, _, err := c.doSessionJSONOnce(ctx, method, endpoint, bundle, "")
	if err == nil {
		return data, nil
	}
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		if bearer := bearerAuthorizationValue(sessionAccessToken(bundle)); bearer != "" {
			retryData, _, _, retryErr := c.doSessionJSONOnce(ctx, method, endpoint, bundle, bearer)
			if retryErr == nil {
				return retryData, nil
			}
			return nil, retryErr
		}
	}
	return nil, err
}

func (c *Client) doSessionJSONOnce(ctx context.Context, method string, endpoint string, bundle map[string]any, authorizationOverride string) ([]byte, int, string, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, 0, "", err
	}
	applySessionHeaders(req, bundle)
	if authorizationOverride != "" {
		req.Header.Set("Authorization", authorizationOverride)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, "", withRequestDiagnostics(err, endpoint, "SUPPLIER_SESSION_REQUEST_FAILED", "new api session endpoint is unreachable")
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, resp.Header.Get("Content-Type"), err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, resp.StatusCode, resp.Header.Get("Content-Type"), withHTTPDiagnostics(infraerrors.New(resp.StatusCode, "SUPPLIER_SESSION_PERMISSION_DENIED", "new api session cannot access requested endpoint"), endpoint, resp.StatusCode, resp.Header.Get("Content-Type"), data)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, resp.Header.Get("Content-Type"), withHTTPDiagnostics(infraerrors.New(http.StatusBadGateway, "SUPPLIER_SESSION_BAD_STATUS", "new api session endpoint returned non-success status"), endpoint, resp.StatusCode, resp.Header.Get("Content-Type"), data)
	}
	return data, resp.StatusCode, resp.Header.Get("Content-Type"), nil
}

func sessionAccessToken(bundle map[string]any) string {
	return firstNonEmpty(stringValue(bundle, "access_token"), stringValueAt(bundle, "tokens", "access_token"))
}

func bearerAuthorizationValue(accessToken string) string {
	token := strings.TrimSpace(accessToken)
	if token == "" || strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return ""
	}
	return "Bearer " + token
}

func withHTTPDiagnostics(err error, endpoint string, statusCode int, contentType string, body []byte) error {
	if err == nil {
		return nil
	}
	var appErr *infraerrors.ApplicationError
	if !errors.As(err, &appErr) {
		return err
	}
	metadata := make(map[string]string, len(appErr.Metadata)+5)
	for key, value := range appErr.Metadata {
		metadata[key] = value
	}
	if strings.TrimSpace(endpoint) != "" {
		metadata["endpoint"] = endpoint
	}
	if statusCode > 0 {
		metadata["status_code"] = strconv.Itoa(statusCode)
	}
	if strings.TrimSpace(contentType) != "" {
		metadata["content_type"] = strings.TrimSpace(contentType)
	}
	if len(body) > 0 {
		lower := strings.ToLower(string(body))
		switch {
		case looksLikeHTMLResponse(lower):
			metadata["body_type"] = "html"
		case json.Valid(body):
			metadata["body_type"] = "json"
		default:
			metadata["body_type"] = "text"
		}
		if excerpt := responseExcerpt(body, 240); excerpt != "" {
			metadata["body_excerpt"] = excerpt
		}
	}
	return appErr.WithMetadata(metadata)
}

func withRequestDiagnostics(err error, endpoint string, reason string, message string) error {
	if err == nil {
		return nil
	}
	appErr := infraerrors.New(http.StatusBadGateway, reason, message).WithCause(err)
	metadata := map[string]string{}
	if strings.TrimSpace(endpoint) != "" {
		metadata["endpoint"] = endpoint
	}
	kind, detail := requestErrorDiagnostic(err)
	if kind != "" {
		metadata["error_kind"] = kind
	}
	if detail != "" {
		metadata["error_detail"] = detail
	}
	return appErr.WithMetadata(metadata)
}

func requestErrorDiagnostic(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	if errors.Is(err, context.Canceled) {
		return "canceled", "request was canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout", "request timed out"
	}
	cause := unwrapURLRequestError(err)
	var dnsErr *net.DNSError
	if errors.As(cause, &dnsErr) {
		return "dns", safeRequestErrorDetail(dnsErr.Err)
	}
	var netErr net.Error
	if errors.As(cause, &netErr) && netErr.Timeout() {
		return "timeout", safeRequestErrorDetail(cause.Error())
	}
	lower := strings.ToLower(cause.Error())
	switch {
	case strings.Contains(lower, "no such host"):
		return "dns", safeRequestErrorDetail(cause.Error())
	case strings.Contains(lower, "connection refused"):
		return "connection_refused", safeRequestErrorDetail(cause.Error())
	case strings.Contains(lower, "connection reset"):
		return "connection_reset", safeRequestErrorDetail(cause.Error())
	case strings.Contains(lower, "tls"):
		return "tls", safeRequestErrorDetail(cause.Error())
	case strings.Contains(lower, "proxyconnect") || strings.Contains(lower, "proxy"):
		return "proxy", safeRequestErrorDetail(cause.Error())
	case strings.Contains(lower, "timeout"):
		return "timeout", safeRequestErrorDetail(cause.Error())
	default:
		var opErr *net.OpError
		if errors.As(cause, &opErr) && strings.TrimSpace(opErr.Op) != "" {
			return "network_" + strings.ToLower(strings.TrimSpace(opErr.Op)), safeRequestErrorDetail(cause.Error())
		}
		return "request_error", safeRequestErrorDetail(cause.Error())
	}
}

func unwrapURLRequestError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		return urlErr.Err
	}
	return err
}

func safeRequestErrorDetail(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= 240 {
		return value
	}
	return value[:240]
}

func responseExcerpt(body []byte, limit int) string {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) <= limit {
		return text
	}
	return text[:limit]
}
