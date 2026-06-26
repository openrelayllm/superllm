package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (c *SessionProfileClient) RegisterAccount(ctx context.Context, in ports.DirectRegistrationInput) (*ports.DirectRegistrationResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "provider adapter is not configured")
	}
	apiBaseURL := firstNonEmpty(in.APIBaseURL, in.RegisterURL, in.Origin)
	if apiBaseURL == "" {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_DIRECT_REGISTRATION_API_BASE_URL_REQUIRED", "supplier api base url is required for direct registration")
	}
	email := strings.TrimSpace(in.Email)
	if email == "" || !nonBlankSecret(in.Password) {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_DIRECT_REGISTRATION_CREDENTIAL_REQUIRED", "supplier direct registration requires email and password")
	}
	client, err := c.httpClientForProxy(in.ProxyURL)
	if err != nil {
		return nil, err
	}
	settings, diagnostics, err := c.fetchRegistrationSettings(ctx, client, apiBaseURL)
	if err != nil {
		return nil, err
	}
	if !boolFromAny(firstExisting(settings, "registration_enabled", "registrationEnabled", "register_enabled", "registerEnabled")) {
		return nil, infraerrors.New(http.StatusConflict, "REGISTRATION_DISABLED", "supplier registration is disabled")
	}
	if boolFromAny(firstExisting(settings, "backend_mode_enabled", "backendModeEnabled")) {
		return nil, infraerrors.New(http.StatusConflict, "BROWSER_FALLBACK_REQUIRED", "supplier backend mode requires browser registration")
	}
	emailVerification := boolFromAny(firstExisting(settings, "email_verification_enabled", "emailVerificationEnabled", "email_verify_enabled", "emailVerifyEnabled", "email_verification", "emailVerification"))
	turnstileEnabled := boolFromAny(firstExisting(settings, "turnstile_enabled", "turnstileEnabled"))
	if turnstileEnabled && !emailVerification {
		return nil, infraerrors.New(http.StatusConflict, "BROWSER_FALLBACK_REQUIRED", "supplier registration requires browser verification")
	}
	origin := firstNonEmpty(originFromRawURL(in.Origin), originFromRawURL(apiBaseURL))
	if emailVerification && strings.TrimSpace(in.VerificationCode) == "" {
		if err := c.requestRegistrationVerifyCode(ctx, client, apiBaseURL, email, origin, turnstileEnabled); err != nil {
			return nil, err
		}
		return &ports.DirectRegistrationResult{
			ProviderType:      adminplusdomain.SupplierTypeSub2API,
			Origin:            origin,
			APIBaseURL:        apiBaseURL,
			Stage:             ports.DirectRegistrationStageNeedEmailCode,
			EmailCodeRequired: true,
			CapturedAt:        c.now().UTC(),
			Diagnostics: map[string]any{
				"settings_endpoint":    diagnostics["settings_endpoint"],
				"settings_keys":        diagnostics["settings_keys"],
				"email_verification":   true,
				"turnstile_enabled":    turnstileEnabled,
				"verify_code_endpoint": mustSub2APIEndpoint(apiBaseURL, "/auth/send-verify-code"),
				"register_endpoint":    mustSub2APIEndpoint(apiBaseURL, "/auth/register"),
			},
		}, nil
	}
	return c.submitRegistration(ctx, client, in, apiBaseURL, origin, emailVerification, turnstileEnabled, diagnostics)
}

func (c *SessionProfileClient) fetchRegistrationSettings(ctx context.Context, client *http.Client, apiBaseURL string) (map[string]any, map[string]any, error) {
	endpoint, err := buildSub2APIUserEndpointURL(apiBaseURL, "/settings/public")
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, err
	}
	applyBrowserCompatHeaders(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_SETTINGS_FAILED", "failed to request supplier public settings").WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, withHTTPDiagnostics(classifyDirectRegistrationUpstreamFailure("SUPPLIER_DIRECT_REGISTRATION_SETTINGS_BAD_STATUS", resp.StatusCode, data), endpoint, resp.StatusCode, resp.Header.Get("Content-Type"), data)
	}
	body, ok := unwrapDataObject(data)
	if !ok {
		return nil, nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_SETTINGS_INVALID", "supplier public settings response is invalid")
	}
	return body, map[string]any{
		"settings_endpoint": endpoint,
		"settings_keys":     rawKeys(body),
	}, nil
}

func (c *SessionProfileClient) requestRegistrationVerifyCode(ctx context.Context, client *http.Client, apiBaseURL string, email string, origin string, turnstileEnabled bool) error {
	if turnstileEnabled {
		return infraerrors.New(http.StatusConflict, "BROWSER_FALLBACK_REQUIRED", "supplier verification code request requires browser verification")
	}
	endpoint, err := buildSub2APIUserEndpointURL(apiBaseURL, "/auth/send-verify-code")
	if err != nil {
		return err
	}
	payload := map[string]any{"email": email}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	applyBrowserCompatHeaders(req)
	if origin != "" {
		req.Header.Set("Origin", origin)
		req.Header.Set("Referer", strings.TrimRight(origin, "/")+"/")
	}
	resp, err := client.Do(req)
	if err != nil {
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_VERIFICATION_CODE_REQUEST_FAILED", "failed to request supplier email verification endpoint").WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return withHTTPDiagnostics(classifySub2APIVerificationCodeFailure(resp.StatusCode, data), endpoint, resp.StatusCode, resp.Header.Get("Content-Type"), data)
	}
	if err := ensureSub2APISuccess(data, "SUPPLIER_VERIFICATION_CODE_FAILED", "supplier verification code request failed"); err != nil {
		return withHTTPDiagnostics(err, endpoint, resp.StatusCode, resp.Header.Get("Content-Type"), data)
	}
	return nil
}

func (c *SessionProfileClient) submitRegistration(ctx context.Context, client *http.Client, in ports.DirectRegistrationInput, apiBaseURL string, origin string, emailVerification bool, turnstileEnabled bool, diagnostics map[string]any) (*ports.DirectRegistrationResult, error) {
	if turnstileEnabled && strings.TrimSpace(in.VerificationCode) == "" {
		return nil, infraerrors.New(http.StatusConflict, "BROWSER_FALLBACK_REQUIRED", "supplier registration requires browser verification")
	}
	endpoint, err := buildSub2APIUserEndpointURL(apiBaseURL, "/auth/register")
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"email":           strings.TrimSpace(in.Email),
		"password":        in.Password,
		"verify_code":     strings.TrimSpace(in.VerificationCode),
		"promo_code":      strings.TrimSpace(in.PromoCode),
		"invitation_code": strings.TrimSpace(in.InvitationCode),
		"aff_code":        strings.TrimSpace(in.AffiliateCode),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
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
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_FAILED", "failed to request supplier registration endpoint").WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, withHTTPDiagnostics(classifySub2APIRegistrationFailure(resp.StatusCode, data), endpoint, resp.StatusCode, resp.Header.Get("Content-Type"), data)
	}
	if err := ensureSub2APISuccess(data, "SUPPLIER_DIRECT_REGISTRATION_FAILED", "supplier registration failed"); err != nil {
		return nil, withHTTPDiagnostics(err, endpoint, resp.StatusCode, resp.Header.Get("Content-Type"), data)
	}
	token, refreshToken, expiresAt, raw, err := parseSub2APILoginResponse(data)
	if err != nil {
		return nil, withHTTPDiagnostics(infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_RESPONSE_INVALID", "supplier registration response is invalid").WithCause(err), endpoint, resp.StatusCode, resp.Header.Get("Content-Type"), data)
	}
	capturedAt := c.now().UTC()
	result := &ports.DirectRegistrationResult{
		ProviderType: adminplusdomain.SupplierTypeSub2API,
		Origin:       origin,
		APIBaseURL:   apiBaseURL,
		Stage:        ports.DirectRegistrationStageCompleted,
		Submitted:    true,
		CapturedAt:   capturedAt,
		ExpiresAt:    expiresAt,
		Diagnostics: map[string]any{
			"settings_endpoint":  diagnostics["settings_endpoint"],
			"settings_keys":      diagnostics["settings_keys"],
			"register_endpoint":  endpoint,
			"registration_keys":  rawKeys(raw),
			"email_verification": emailVerification,
			"turnstile_enabled":  turnstileEnabled,
			"token_present":      token != "",
		},
	}
	if token != "" {
		result.SessionBundle = buildDirectLoginSessionBundle(ports.DirectLoginInput{
			Origin:     origin,
			APIBaseURL: apiBaseURL,
		}, token, refreshToken, capturedAt, expiresAt)
	}
	return result, nil
}

func ensureSub2APISuccess(data []byte, defaultReason string, defaultMessage string) error {
	var root map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_RESPONSE_INVALID", "supplier registration response is invalid").WithCause(err)
	}
	if success, ok := boolValueFromAny(root["success"]); ok && !success {
		return classifySub2APIRegistrationBusinessFailure(stringFromAny(root["message"]))
	}
	if code := strings.TrimSpace(stringFromAny(root["code"])); code != "" && code != "0" && !strings.EqualFold(code, "success") {
		return classifySub2APIRegistrationBusinessFailure(firstNonEmpty(stringFromAny(root["message"]), stringFromAny(root["reason"]), defaultMessage))
	}
	if success, ok := boolValueFromAny(root["success"]); ok && success {
		return nil
	}
	if len(root) == 0 {
		return infraerrors.New(http.StatusBadGateway, defaultReason, defaultMessage)
	}
	return nil
}

func boolValueFromAny(value any) (bool, bool) {
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

func classifySub2APIVerificationCodeFailure(statusCode int, body []byte) error {
	lower := strings.ToLower(string(body))
	switch {
	case isCloudflareOriginFailure(statusCode, lower):
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_VERIFICATION_CODE_UPSTREAM_ORIGIN_ERROR", "supplier site returned a Cloudflare or origin server error")
	case looksLikeHTMLResponse(lower):
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_VERIFICATION_CODE_UPSTREAM_HTML", "supplier verification endpoint returned an HTML response")
	case strings.Contains(lower, "turnstile") || strings.Contains(lower, "captcha") || strings.Contains(lower, "recaptcha"):
		return infraerrors.New(http.StatusConflict, "BROWSER_FALLBACK_REQUIRED", "supplier verification code request requires browser verification")
	default:
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_VERIFICATION_CODE_BAD_STATUS", "supplier verification endpoint returned non-success status")
	}
}

func classifySub2APIRegistrationFailure(statusCode int, body []byte) error {
	lower := strings.ToLower(string(body))
	switch {
	case isCloudflareOriginFailure(statusCode, lower):
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_UPSTREAM_ORIGIN_ERROR", "supplier site returned a Cloudflare or origin server error")
	case looksLikeHTMLResponse(lower):
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_UPSTREAM_HTML", "supplier registration endpoint returned an HTML response")
	case strings.Contains(lower, "turnstile") || strings.Contains(lower, "captcha") || strings.Contains(lower, "recaptcha"):
		return infraerrors.New(http.StatusConflict, "BROWSER_FALLBACK_REQUIRED", "supplier registration requires browser verification")
	case strings.Contains(lower, "verify") || strings.Contains(lower, "verification") || strings.Contains(lower, "验证码"):
		return infraerrors.New(http.StatusConflict, "REGISTRATION_VERIFICATION_CODE_INVALID", "supplier registration verification code is invalid")
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return infraerrors.New(statusCode, "REGISTRATION_FORBIDDEN", "supplier registration is forbidden")
	default:
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_BAD_STATUS", "supplier registration endpoint returned non-success status")
	}
}

func classifySub2APIRegistrationBusinessFailure(message string) error {
	lower := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(lower, "turnstile") || strings.Contains(lower, "captcha") || strings.Contains(lower, "recaptcha"):
		return infraerrors.New(http.StatusConflict, "BROWSER_FALLBACK_REQUIRED", "supplier registration requires browser verification")
	case strings.Contains(lower, "verify") || strings.Contains(lower, "verification") || strings.Contains(lower, "验证码"):
		return infraerrors.New(http.StatusConflict, "REGISTRATION_VERIFICATION_CODE_INVALID", firstNonEmpty(message, "supplier registration verification code is invalid"))
	case strings.Contains(lower, "exists") || strings.Contains(lower, "already") || strings.Contains(lower, "已存在") || strings.Contains(lower, "已注册"):
		return infraerrors.New(http.StatusConflict, "REGISTRATION_EMAIL_ALREADY_EXISTS", firstNonEmpty(message, "registration email already exists"))
	case strings.Contains(lower, "invitation"):
		return infraerrors.New(http.StatusConflict, "REGISTRATION_INVITATION_REQUIRED", firstNonEmpty(message, "supplier registration requires invitation code"))
	default:
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_FAILED", firstNonEmpty(message, "supplier registration failed"))
	}
}

func classifyDirectRegistrationUpstreamFailure(defaultReason string, statusCode int, body []byte) error {
	lower := strings.ToLower(string(body))
	if isCloudflareOriginFailure(statusCode, lower) {
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_UPSTREAM_ORIGIN_ERROR", "supplier site returned a Cloudflare or origin server error")
	}
	if looksLikeHTMLResponse(lower) {
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_UPSTREAM_HTML", "supplier endpoint returned an HTML response")
	}
	return infraerrors.New(http.StatusBadGateway, defaultReason, "supplier endpoint returned non-success status")
}

func mustSub2APIEndpoint(apiBaseURL string, path string) string {
	endpoint, err := buildSub2APIUserEndpointURL(apiBaseURL, path)
	if err != nil {
		return ""
	}
	return endpoint
}
