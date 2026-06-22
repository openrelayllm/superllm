package provider

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func buildSessionBundle(supplierID int64, origin string, apiBaseURL string, userID int64, loginData map[string]any, cookies []any, capturedAt time.Time, expiresAt *time.Time) map[string]any {
	userIDText := strconv.FormatInt(userID, 10)
	requiredHeaders := map[string]any{
		"New-Api-User": userIDText,
	}
	if origin != "" {
		requiredHeaders["origin"] = origin
		requiredHeaders["referer"] = strings.TrimRight(origin, "/") + "/"
	}
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

func applySessionHeaders(req *http.Request, bundle map[string]any) {
	applyBrowserCompatHeaders(req)
	requiredHeaders := mapValue(bundle, "required_headers")
	userID := firstNonEmpty(
		stringValue(requiredHeaders, "New-Api-User"),
		stringValue(requiredHeaders, "new-api-user"),
		stringValue(bundle, "auth_header_value"),
		stringValueAt(bundle, "context", "user_id"),
	)
	if userID != "" {
		req.Header.Set("New-Api-User", userID)
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
