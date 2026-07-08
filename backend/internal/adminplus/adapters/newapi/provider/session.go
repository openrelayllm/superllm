package provider

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var newAPIUserIDHeaderNames = []string{
	"New-Api-User",
	"New-API-User",
	"Veloera-User",
	"voapi-user",
	"User-id",
	"X-User-Id",
	"Rix-Api-User",
	"neo-api-user",
}

func buildSessionBundle(supplierID int64, origin string, apiBaseURL string, userID int64, loginData map[string]any, cookies []any, capturedAt time.Time, expiresAt *time.Time) map[string]any {
	userIDText := strconv.FormatInt(userID, 10)
	requiredHeaders := buildNewAPIRequiredHeaders(userIDText, origin)
	contextValue := map[string]any{
		"api_base_url":   apiBaseURL,
		"login_method":   "direct_login",
		"session_source": "direct_login",
		"provider_type":  "new_api",
		"system_type":    "new_api",
		"user_id":        userIDText,
		"username":       stringFromAny(loginData["username"]),
		"display_name":   stringFromAny(loginData["display_name"]),
		"group":          stringFromAny(loginData["group"]),
		"role":           newAPIIntText(loginData["role"]),
		"status":         newAPIIntText(loginData["status"]),
	}
	bundle := map[string]any{
		"provider_type":     "new_api",
		"system_type":       "new_api",
		"supplier_id":       supplierID,
		"origin":            origin,
		"api_base_url":      apiBaseURL,
		"cookies":           cookies,
		"required_headers":  requiredHeaders,
		"context":           contextValue,
		"captured_at":       capturedAt.UTC().Format(time.RFC3339),
		"session_source":    "direct_login",
		"auth_header_name":  "New-Api-User",
		"auth_header_value": userIDText,
	}
	if expiresAt != nil && !expiresAt.IsZero() {
		bundle["expires_at"] = expiresAt.UTC().Format(time.RFC3339)
	}
	return bundle
}

func buildAccessTokenSessionBundle(supplierID int64, origin string, apiBaseURL string, userID string, accessToken string, capturedAt time.Time) map[string]any {
	userID = strings.TrimSpace(userID)
	accessToken = strings.TrimSpace(accessToken)
	requiredHeaders := buildNewAPIRequiredHeaders(userID, origin)
	contextValue := map[string]any{
		"api_base_url":   apiBaseURL,
		"login_method":   "access_token",
		"session_source": "direct_login",
		"provider_type":  "new_api",
		"system_type":    "new_api",
		"user_id":        userID,
	}
	tokens := map[string]any{"access_token": accessToken}
	return map[string]any{
		"provider_type":     "new_api",
		"system_type":       "new_api",
		"supplier_id":       supplierID,
		"origin":            origin,
		"api_base_url":      apiBaseURL,
		"access_token":      accessToken,
		"tokens":            tokens,
		"cookies":           []any{},
		"required_headers":  requiredHeaders,
		"context":           contextValue,
		"captured_at":       capturedAt.UTC().Format(time.RFC3339),
		"session_source":    "direct_login",
		"auth_header_name":  "New-Api-User",
		"auth_header_value": userID,
	}
}

func applySessionHeaders(req *http.Request, bundle map[string]any) {
	applyBrowserCompatHeaders(req)
	requiredHeaders := mapValue(bundle, "required_headers")
	userID := newAPIUserIDFromSessionBundle(bundle)
	if userID != "" {
		applyNewAPIUserIDHeaders(req, userID)
	}
	if accessToken := firstNonEmpty(stringValue(bundle, "access_token"), stringValueAt(bundle, "tokens", "access_token")); accessToken != "" {
		req.Header.Set("Authorization", accessToken)
	}
	if cookie := firstNonEmpty(stringValue(requiredHeaders, "cookie"), stringValue(bundle, "cookie"), cookieHeaderFromBundle(bundle)); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if origin := firstNonEmpty(stringValue(requiredHeaders, "origin"), stringValue(bundle, "origin")); origin != "" {
		req.Header.Set("Origin", origin)
	}
	if referer := stringValue(requiredHeaders, "referer"); referer != "" {
		req.Header.Set("Referer", referer)
	}
}

func buildNewAPIRequiredHeaders(userID string, origin string) map[string]any {
	requiredHeaders := map[string]any{}
	if userID = strings.TrimSpace(userID); userID != "" {
		for _, headerName := range newAPIUserIDHeaderNames {
			requiredHeaders[headerName] = userID
		}
	}
	if origin != "" {
		requiredHeaders["origin"] = origin
		requiredHeaders["referer"] = strings.TrimRight(origin, "/") + "/"
	}
	return requiredHeaders
}

func newAPIUserIDFromSessionBundle(bundle map[string]any) string {
	requiredHeaders := mapValue(bundle, "required_headers")
	values := make([]string, 0, len(newAPIUserIDHeaderNames)+3)
	for _, headerName := range newAPIUserIDHeaderNames {
		values = append(values, stringValue(requiredHeaders, headerName))
	}
	values = append(values,
		stringValue(requiredHeaders, "new-api-user"),
		stringValue(bundle, "auth_header_value"),
		stringValueAt(bundle, "context", "user_id"),
	)
	return firstNonEmpty(values...)
}

func applyNewAPIUserIDHeaders(req *http.Request, userID string) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return
	}
	for _, headerName := range newAPIUserIDHeaderNames {
		req.Header.Set(headerName, userID)
	}
}

func applyBrowserCompatHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-store")
	req.Header.Set("User-Agent", browserUserAgent)
}

func cookiesFromResponse(resp *http.Response, apiBaseURL string) []any {
	if resp == nil {
		return nil
	}
	u, _ := url.Parse(apiBaseURL)
	out := make([]any, 0, len(resp.Cookies()))
	for _, cookie := range resp.Cookies() {
		if cookie == nil || strings.TrimSpace(cookie.Name) == "" {
			continue
		}
		domain := strings.TrimSpace(cookie.Domain)
		if domain == "" && u != nil {
			domain = u.Hostname()
		}
		path := cookie.Path
		if path == "" {
			path = "/"
		}
		item := map[string]any{
			"name":      cookie.Name,
			"value":     cookie.Value,
			"domain":    domain,
			"path":      path,
			"http_only": cookie.HttpOnly,
			"secure":    cookie.Secure,
		}
		if !cookie.Expires.IsZero() {
			item["expires_at"] = cookie.Expires.UTC().Format(time.RFC3339)
		}
		out = append(out, item)
	}
	return out
}

func expiresAtFromCookies(cookies []*http.Cookie, now time.Time) *time.Time {
	for _, cookie := range cookies {
		if cookie == nil || !strings.EqualFold(cookie.Name, "session") {
			continue
		}
		if !cookie.Expires.IsZero() {
			value := cookie.Expires.UTC()
			return &value
		}
		if cookie.MaxAge > 0 {
			value := now.UTC().Add(time.Duration(cookie.MaxAge) * time.Second)
			return &value
		}
	}
	return nil
}

func normalizeBaseURLs(apiBaseURL string, origin string) (string, string, error) {
	raw := firstNonEmpty(apiBaseURL, origin)
	if raw == "" {
		return "", "", infraerrors.New(http.StatusConflict, "SUPPLIER_DIRECT_LOGIN_API_BASE_URL_REQUIRED", "new api base url is required")
	}
	parsed, err := parseSafeURL(raw, "SUPPLIER_SESSION_API_BASE_URL_INVALID")
	if err != nil {
		return "", "", err
	}
	base := parsed.Scheme + "://" + parsed.Host
	originValue := firstNonEmpty(originFromRawURL(origin), base)
	return base, originValue, nil
}

func buildEndpointURL(apiBaseURL string, endpointPath string) (string, error) {
	parsed, err := parseSafeURL(apiBaseURL, "SUPPLIER_SESSION_API_BASE_URL_INVALID")
	if err != nil {
		return "", err
	}
	return strings.TrimRight(parsed.Scheme+"://"+parsed.Host, "/") + endpointPath, nil
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
		if name == "" || value == "" {
			continue
		}
		parts = append(parts, name+"="+value)
	}
	return strings.Join(parts, "; ")
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func firstExisting(in map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, exists := in[key]; exists && value != nil {
			return value
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func nonBlankSecret(value string) bool {
	return strings.TrimSpace(value) != ""
}

func originFromRawURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
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

func stringValue(in map[string]any, key string) string {
	if in == nil {
		return ""
	}
	return stringFromAny(in[key])
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

func newAPIRoleFromBundle(bundle map[string]any) (int64, bool) {
	contextValue := mapValue(bundle, "context")
	for _, value := range []any{
		contextValue["role"],
		bundle["role"],
		contextValue["user_role"],
		bundle["user_role"],
	} {
		if role := int64FromAny(value); role != 0 {
			return role, true
		}
	}
	return 0, false
}

func newAPIIntText(value any) string {
	if text := stringFromAny(value); text != "" {
		return text
	}
	if n := int64FromAny(value); n != 0 {
		return strconv.FormatInt(n, 10)
	}
	return ""
}

func applyProfileToSessionBundle(bundle map[string]any, probe *ports.SessionProbeResult) {
	if bundle == nil || probe == nil || probe.Profile == nil {
		return
	}
	contextValue := mapValue(bundle, "context")
	if contextValue == nil {
		contextValue = map[string]any{}
		bundle["context"] = contextValue
	}
	if strings.TrimSpace(probe.Profile.Role) != "" {
		contextValue["role"] = strings.TrimSpace(probe.Profile.Role)
	}
	if strings.TrimSpace(probe.Profile.Status) != "" {
		contextValue["status"] = strings.TrimSpace(probe.Profile.Status)
	}
	if strings.TrimSpace(probe.Profile.Username) != "" {
		contextValue["username"] = strings.TrimSpace(probe.Profile.Username)
	}
}

func newAPISessionPermissionRequired(currentRole int64, roleKnown bool, cause error) error {
	metadata := map[string]string{}
	if appErr := infraerrors.FromError(cause); appErr != nil {
		for key, value := range appErr.Metadata {
			metadata[key] = value
		}
	}
	metadata["suggestion"] = "当前 New API 注册用户无权读取该接口；请确认账号在供应商后台具备对应权限，或换用可读取该数据的账号/token 与数字 User ID。"
	if roleKnown {
		metadata["current_role"] = strconv.FormatInt(currentRole, 10)
	}
	message := "new api session cannot access requested endpoint"
	if causeMessage := infraerrors.Message(cause); causeMessage != "" {
		message = causeMessage
	}
	err := infraerrors.New(http.StatusForbidden, "SUPPLIER_SESSION_PERMISSION_DENIED", message).WithMetadata(metadata)
	if cause != nil {
		return err.WithCause(cause)
	}
	return err
}

func isNewAPISessionPermissionError(err error) bool {
	if err == nil {
		return false
	}
	reason := strings.ToUpper(strings.TrimSpace(infraerrors.Reason(err)))
	if reason == "SUPPLIER_SESSION_PERMISSION_DENIED" {
		return true
	}
	code := infraerrors.Code(err)
	return code == http.StatusUnauthorized || code == http.StatusForbidden
}

func waitForProviderPage(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
