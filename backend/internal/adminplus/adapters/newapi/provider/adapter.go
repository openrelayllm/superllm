package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
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
	if nonBlankSecret(in.Token) && (strings.TrimSpace(in.Username) == "" || !nonBlankSecret(in.Password)) {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED", "new api direct login requires username and password")
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
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_FAILED", "failed to request new api login endpoint").WithCause(err)
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
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_RESPONSE_INVALID", "new api login response did not include session cookie")
	}
	expiresAt := expiresAtFromCookies(resp.Cookies(), capturedAt)
	bundle := buildSessionBundle(in.SupplierID, origin, apiBaseURL, userID, envelope.Data, cookies, capturedAt, expiresAt)
	probe, err := c.ProbeSub2APIUserProfile(ctx, ports.SessionProbeInput{
		SupplierID: in.SupplierID,
		Origin:     origin,
		APIBaseURL: apiBaseURL,
		Bundle:     bundle,
	})
	if err != nil {
		return nil, err
	}
	return &ports.DirectLoginResult{
		SupplierID:    in.SupplierID,
		Origin:        origin,
		APIBaseURL:    apiBaseURL,
		SessionBundle: bundle,
		CapturedAt:    capturedAt,
		ExpiresAt:     expiresAt,
		Diagnostics: map[string]any{
			"login_endpoint":       loginEndpoint,
			"profile_status":       stringFromProbeStatus(probe),
			"auth_header_required": "New-Api-User",
			"login_response_keys":  rawKeys(envelope.Data),
		},
	}, nil
}

func (c *Client) doSessionJSON(ctx context.Context, method string, endpoint string, bundle map[string]any) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applySessionHeaders(req, bundle)
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
		return nil, withHTTPDiagnostics(infraerrors.New(resp.StatusCode, "SUPPLIER_SESSION_PERMISSION_DENIED", "new api session cannot access requested endpoint"), endpoint, resp.StatusCode, resp.Header.Get("Content-Type"), data)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, withHTTPDiagnostics(infraerrors.New(http.StatusBadGateway, "SUPPLIER_SESSION_BAD_STATUS", "new api session endpoint returned non-success status"), endpoint, resp.StatusCode, resp.Header.Get("Content-Type"), data)
	}
	return data, nil
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
