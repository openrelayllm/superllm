package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
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

func (c *Client) DirectLogin(ctx context.Context, in ports.DirectLoginInput) (*ports.DirectLoginResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "new api provider adapter is not configured")
	}
	if strings.TrimSpace(in.Token) != "" && (strings.TrimSpace(in.Username) == "" || strings.TrimSpace(in.Password) == "") {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED", "new api direct login requires username and password")
	}
	if strings.TrimSpace(in.Username) == "" || strings.TrimSpace(in.Password) == "" {
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
		"password": strings.TrimSpace(in.Password),
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
		return nil, infraerrors.New(resp.StatusCode, "SUPPLIER_SESSION_PERMISSION_DENIED", "new api session cannot access requested endpoint")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_SESSION_BAD_STATUS", "new api session endpoint returned non-success status")
	}
	return data, nil
}
