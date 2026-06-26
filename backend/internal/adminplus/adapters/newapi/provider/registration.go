package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const registrationStatusMaxAttempts = 5

func (c *Client) RegisterAccount(ctx context.Context, in ports.DirectRegistrationInput) (*ports.DirectRegistrationResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "new api provider adapter is not configured")
	}
	apiBaseURL, origin, err := normalizeBaseURLs(firstNonEmpty(in.APIBaseURL, in.RegisterURL), in.Origin)
	if err != nil {
		return nil, err
	}
	email := strings.TrimSpace(in.Email)
	if email == "" || !nonBlankSecret(in.Password) {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_DIRECT_REGISTRATION_CREDENTIAL_REQUIRED", "new api direct registration requires email and password")
	}
	client, err := c.httpClientForProxy(in.ProxyURL)
	if err != nil {
		return nil, err
	}
	status, err := c.fetchRegistrationStatus(ctx, client, apiBaseURL)
	if err != nil {
		return nil, err
	}
	if !boolFromAny(firstExisting(status, "register_enabled", "registerEnabled")) {
		return nil, infraerrors.New(http.StatusConflict, "REGISTRATION_DISABLED", "new api registration is disabled")
	}
	if !boolFromAny(firstExisting(status, "password_register_enabled", "passwordRegisterEnabled")) {
		return nil, infraerrors.New(http.StatusConflict, "PASSWORD_REGISTER_DISABLED", "new api password registration is disabled")
	}
	emailVerification := boolFromAny(firstExisting(status, "email_verification", "emailVerification"))
	turnstileCheck := boolFromAny(firstExisting(status, "turnstile_check", "turnstileCheck"))
	systemName := stringFromAny(firstExisting(status, "system_name", "systemName"))
	siteName := stringFromAny(firstExisting(status, "site_name", "siteName"))
	if emailVerification && strings.TrimSpace(in.VerificationCode) == "" {
		if err := c.requestEmailVerificationCode(ctx, client, apiBaseURL, email, origin); err != nil {
			return nil, err
		}
		return &ports.DirectRegistrationResult{
			ProviderType:      adminplusdomain.SupplierTypeNewAPI,
			Origin:            origin,
			APIBaseURL:        apiBaseURL,
			Stage:             ports.DirectRegistrationStageNeedEmailCode,
			EmailCodeRequired: true,
			CapturedAt:        c.now().UTC(),
			Diagnostics: map[string]any{
				"status_endpoint":       strings.TrimRight(apiBaseURL, "/") + "/api/status",
				"verification_endpoint": strings.TrimRight(apiBaseURL, "/") + "/api/verification",
				"email_verification":    true,
				"turnstile_check":       turnstileCheck,
				"system_name":           systemName,
				"site_name":             siteName,
			},
		}, nil
	}

	registerEndpoint, err := buildEndpointURL(apiBaseURL, "/api/user/register")
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"username":          registrationUsername(in),
		"password":          in.Password,
		"email":             email,
		"verification_code": strings.TrimSpace(in.VerificationCode),
		"aff_code":          strings.TrimSpace(in.AffiliateCode),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registerEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	applyBrowserCompatHeaders(req)
	if origin != "" {
		req.Header.Set("Origin", origin)
		req.Header.Set("Referer", strings.TrimRight(origin, "/")+"/")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_FAILED", "failed to request new api registration endpoint").WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, withHTTPDiagnostics(classifyRegistrationFailure(resp.StatusCode, data), registerEndpoint, resp.StatusCode, resp.Header.Get("Content-Type"), data)
	}
	envelope, err := decodeEnvelope(data)
	if err != nil {
		return nil, withHTTPDiagnostics(infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_RESPONSE_INVALID", "new api registration response is invalid").WithCause(err), registerEndpoint, resp.StatusCode, resp.Header.Get("Content-Type"), data)
	}
	if !envelope.Success {
		return nil, classifyRegistrationBusinessFailure(envelope.Message)
	}
	result := &ports.DirectRegistrationResult{
		ProviderType: adminplusdomain.SupplierTypeNewAPI,
		Origin:       origin,
		APIBaseURL:   apiBaseURL,
		Stage:        ports.DirectRegistrationStageCompleted,
		Submitted:    true,
		CapturedAt:   c.now().UTC(),
		Diagnostics: map[string]any{
			"register_endpoint":   registerEndpoint,
			"registration_keys":   rawKeys(envelope.Data),
			"email_verification":  emailVerification,
			"turnstile_check":     turnstileCheck,
			"system_name":         systemName,
			"site_name":           siteName,
			"session_cookie_seen": false,
		},
	}
	if session := c.registrationSessionFromResponse(ctx, in, origin, apiBaseURL, resp, envelope.Data); session != nil {
		result.SessionBundle = session.bundle
		result.ExpiresAt = session.expiresAt
		result.Diagnostics["session_cookie_seen"] = true
		result.Diagnostics["profile_status"] = session.profileStatus
	}
	return result, nil
}

type newAPIRegistrationSession struct {
	bundle        map[string]any
	expiresAt     *time.Time
	profileStatus string
}

func (c *Client) fetchRegistrationStatus(ctx context.Context, client *http.Client, apiBaseURL string) (map[string]any, error) {
	endpoint, err := buildEndpointURL(apiBaseURL, "/api/status")
	if err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 1; attempt <= registrationStatusMaxAttempts; attempt++ {
		status, err := c.fetchRegistrationStatusOnce(ctx, client, endpoint)
		if err == nil {
			return status, nil
		}
		lastErr = err
		if !isRetryableRegistrationStatusError(err) || attempt == registrationStatusMaxAttempts {
			break
		}
		if err := sleepRegistrationStatusRetry(ctx, attempt); err != nil {
			return nil, err
		}
	}
	if lastErr == nil {
		lastErr = infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_STATUS_FAILED", "failed to request new api status endpoint")
	}
	return nil, lastErr
}

func (c *Client) fetchRegistrationStatusOnce(ctx context.Context, client *http.Client, endpoint string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyBrowserCompatHeaders(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_STATUS_FAILED", "failed to request new api status endpoint").WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, withHTTPDiagnostics(infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_STATUS_BAD_STATUS", "new api status endpoint returned non-success status"), endpoint, resp.StatusCode, resp.Header.Get("Content-Type"), data)
	}
	envelope, err := decodeEnvelopeReader(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_STATUS_INVALID", "new api status response is invalid").WithCause(err)
	}
	if !envelope.Success {
		return nil, classifyRegistrationBusinessFailure(envelope.Message)
	}
	return envelope.Data, nil
}

func isRetryableRegistrationStatusError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "eof") ||
		strings.Contains(lower, "connection reset") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "tls handshake timeout") ||
		strings.Contains(lower, "server closed idle connection")
}

func sleepRegistrationStatusRetry(ctx context.Context, attempt int) error {
	delay := time.Duration(attempt*attempt) * 250 * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) requestEmailVerificationCode(ctx context.Context, client *http.Client, apiBaseURL string, email string, origin string) error {
	endpoint, err := buildEndpointURL(apiBaseURL, "/api/verification")
	if err != nil {
		return err
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	query := u.Query()
	query.Set("email", email)
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	applyBrowserCompatHeaders(req)
	if origin != "" {
		req.Header.Set("Origin", origin)
		req.Header.Set("Referer", strings.TrimRight(origin, "/")+"/")
	}
	resp, err := client.Do(req)
	if err != nil {
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_VERIFICATION_CODE_REQUEST_FAILED", "failed to request new api email verification endpoint").WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return withHTTPDiagnostics(classifyVerificationCodeRequestFailure(resp.StatusCode, data), endpoint, resp.StatusCode, resp.Header.Get("Content-Type"), data)
	}
	envelope, err := decodeEnvelope(data)
	if err != nil {
		return withHTTPDiagnostics(infraerrors.New(http.StatusBadGateway, "SUPPLIER_VERIFICATION_CODE_RESPONSE_INVALID", "new api verification response is invalid").WithCause(err), endpoint, resp.StatusCode, resp.Header.Get("Content-Type"), data)
	}
	if !envelope.Success {
		return classifyVerificationBusinessFailure(envelope.Message)
	}
	return nil
}

func (c *Client) registrationSessionFromResponse(ctx context.Context, in ports.DirectRegistrationInput, origin string, apiBaseURL string, resp *http.Response, data map[string]any) *newAPIRegistrationSession {
	userID := int64FromAny(data["id"])
	if userID <= 0 {
		return nil
	}
	cookies := cookiesFromResponse(resp, apiBaseURL)
	if len(cookies) == 0 {
		return nil
	}
	capturedAt := c.now().UTC()
	expiresAt := expiresAtFromCookies(resp.Cookies(), capturedAt)
	bundle := buildSessionBundle(0, origin, apiBaseURL, userID, data, cookies, capturedAt, expiresAt)
	probe, err := c.ProbeSub2APIUserProfile(ctx, ports.SessionProbeInput{
		Origin:     origin,
		APIBaseURL: apiBaseURL,
		Bundle:     bundle,
	})
	if err != nil {
		return nil
	}
	return &newAPIRegistrationSession{bundle: bundle, expiresAt: expiresAt, profileStatus: stringFromProbeStatus(probe)}
}

func registrationUsername(in ports.DirectRegistrationInput) string {
	username := strings.TrimSpace(in.Username)
	if username != "" {
		return username
	}
	email := strings.TrimSpace(in.Email)
	if at := strings.IndexByte(email, '@'); at > 0 {
		return email[:at]
	}
	return email
}

func classifyVerificationCodeRequestFailure(statusCode int, body []byte) error {
	lower := strings.ToLower(string(body))
	switch {
	case looksLikeHTMLResponse(lower):
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_VERIFICATION_CODE_UPSTREAM_HTML", "new api verification endpoint returned an HTML response")
	case containsBrowserChallenge(lower):
		return infraerrors.New(http.StatusConflict, "BROWSER_FALLBACK_REQUIRED", "new api verification code request requires browser verification")
	default:
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_VERIFICATION_CODE_BAD_STATUS", "new api verification endpoint returned non-success status")
	}
}

func classifyVerificationBusinessFailure(message string) error {
	lower := strings.ToLower(strings.TrimSpace(message))
	switch {
	case containsBrowserChallenge(lower):
		return infraerrors.New(http.StatusConflict, "BROWSER_FALLBACK_REQUIRED", "new api verification code request requires browser verification")
	case strings.Contains(lower, "占用") || strings.Contains(lower, "taken") || strings.Contains(lower, "exists"):
		return infraerrors.New(http.StatusConflict, "REGISTRATION_EMAIL_ALREADY_EXISTS", firstNonEmpty(message, "registration email already exists"))
	default:
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_VERIFICATION_CODE_FAILED", firstNonEmpty(message, "new api verification code request failed"))
	}
}

func classifyRegistrationFailure(statusCode int, body []byte) error {
	lower := strings.ToLower(string(body))
	switch {
	case looksLikeHTMLResponse(lower):
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_UPSTREAM_HTML", "new api registration endpoint returned an HTML response")
	case containsBrowserChallenge(lower):
		return infraerrors.New(http.StatusConflict, "BROWSER_FALLBACK_REQUIRED", "new api registration requires browser verification")
	case strings.Contains(lower, "verification") || strings.Contains(lower, "验证码"):
		return infraerrors.New(http.StatusConflict, "REGISTRATION_VERIFICATION_CODE_INVALID", "new api registration verification code is invalid")
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return infraerrors.New(statusCode, "REGISTRATION_FORBIDDEN", "new api registration is forbidden")
	default:
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_BAD_STATUS", "new api registration endpoint returned non-success status")
	}
}

func classifyRegistrationBusinessFailure(message string) error {
	lower := strings.ToLower(strings.TrimSpace(message))
	switch {
	case containsBrowserChallenge(lower):
		return infraerrors.New(http.StatusConflict, "BROWSER_FALLBACK_REQUIRED", "new api registration requires browser verification")
	case strings.Contains(lower, "验证码") || strings.Contains(lower, "verification"):
		return infraerrors.New(http.StatusConflict, "REGISTRATION_VERIFICATION_CODE_INVALID", firstNonEmpty(message, "new api registration verification code is invalid"))
	case strings.Contains(lower, "邮箱") && (strings.Contains(lower, "占用") || strings.Contains(lower, "存在")):
		return infraerrors.New(http.StatusConflict, "REGISTRATION_EMAIL_ALREADY_EXISTS", firstNonEmpty(message, "registration email already exists"))
	case strings.Contains(lower, "exists") || strings.Contains(lower, "taken"):
		return infraerrors.New(http.StatusConflict, "REGISTRATION_EMAIL_ALREADY_EXISTS", firstNonEmpty(message, "registration email already exists"))
	default:
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_FAILED", firstNonEmpty(message, "new api registration failed"))
	}
}
