package provider

import (
	"net/http"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func classifyLoginFailure(statusCode int, body []byte) error {
	lower := strings.ToLower(string(body))
	switch {
	case looksLikeHTMLResponse(lower):
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_UPSTREAM_HTML", "new api login endpoint returned an HTML response")
	case containsBrowserChallenge(lower):
		return infraerrors.New(http.StatusConflict, "BROWSER_CHALLENGE_REQUIRED", "new api direct login requires browser verification")
	case strings.Contains(lower, "2fa") || strings.Contains(lower, "totp") || strings.Contains(lower, "mfa"):
		return infraerrors.New(http.StatusConflict, "LOGIN_MFA_REQUIRED", "new api direct login requires 2FA; use browser extension fallback")
	case strings.Contains(lower, "invalid") || strings.Contains(lower, "password") || strings.Contains(lower, "credential"):
		return infraerrors.New(http.StatusUnauthorized, "LOGIN_CREDENTIAL_INVALID", "new api direct login credential is invalid")
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return infraerrors.New(statusCode, "LOGIN_CREDENTIAL_INVALID", "new api direct login credential is invalid")
	default:
		return infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_BAD_STATUS", "new api login endpoint returned non-success status")
	}
}

func classifyBusinessFailure(message string) error {
	lower := strings.ToLower(strings.TrimSpace(message))
	switch {
	case containsBrowserChallenge(lower):
		return infraerrors.New(http.StatusConflict, "BROWSER_CHALLENGE_REQUIRED", "new api direct login requires browser verification")
	case strings.Contains(lower, "2fa") || strings.Contains(lower, "totp") || strings.Contains(lower, "mfa"):
		return infraerrors.New(http.StatusConflict, "LOGIN_MFA_REQUIRED", "new api direct login requires 2FA; use browser extension fallback")
	case strings.Contains(lower, "禁用") || strings.Contains(lower, "disabled"):
		return infraerrors.New(http.StatusConflict, "PASSWORD_LOGIN_DISABLED", "new api password login is disabled")
	case strings.Contains(lower, "密码") || strings.Contains(lower, "password") || strings.Contains(lower, "credential") || strings.Contains(lower, "用户名"):
		return infraerrors.New(http.StatusUnauthorized, "LOGIN_CREDENTIAL_INVALID", "new api direct login credential is invalid")
	default:
		return infraerrors.New(http.StatusUnauthorized, "LOGIN_CREDENTIAL_INVALID", firstNonEmpty(message, "new api direct login failed"))
	}
}

func classifySessionBusinessFailure(message string) error {
	lower := strings.ToLower(strings.TrimSpace(message))
	if strings.Contains(lower, "not logged in") || strings.Contains(lower, "未登录") || strings.Contains(lower, "access token") || strings.Contains(lower, "new-api-user") || strings.Contains(lower, "unauthorized") {
		return infraerrors.New(http.StatusConflict, "SUPPLIER_SESSION_EXPIRED", "new api session is expired or invalid")
	}
	return infraerrors.New(http.StatusForbidden, "SUPPLIER_SESSION_PERMISSION_DENIED", firstNonEmpty(message, "new api session cannot access requested endpoint"))
}

func classifyKeyBusinessFailure(message string) error {
	lower := strings.ToLower(strings.TrimSpace(message))
	if strings.Contains(lower, "not logged in") || strings.Contains(lower, "未登录") || strings.Contains(lower, "access token") || strings.Contains(lower, "new-api-user") || strings.Contains(lower, "unauthorized") {
		return infraerrors.New(http.StatusConflict, "SUPPLIER_SESSION_EXPIRED", "new api session is expired or invalid")
	}
	if strings.Contains(lower, "最大令牌") || strings.Contains(lower, "maximum token") || strings.Contains(lower, "max token") {
		return infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_LIMIT_REACHED", firstNonEmpty(message, "new api token limit reached"))
	}
	if strings.Contains(lower, "quota") || strings.Contains(lower, "额度") {
		return infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_QUOTA_INVALID", firstNonEmpty(message, "new api token quota is invalid"))
	}
	return infraerrors.New(http.StatusBadGateway, "SUPPLIER_KEY_CREATE_FAILED", firstNonEmpty(message, "new api token creation failed"))
}

func containsBrowserChallenge(lower string) bool {
	return strings.Contains(lower, "turnstile") ||
		strings.Contains(lower, "captcha") ||
		strings.Contains(lower, "recaptcha") ||
		strings.Contains(lower, "challenge") ||
		strings.Contains(lower, "校验失败")
}

func looksLikeHTMLResponse(lower string) bool {
	return strings.Contains(lower, "<!doctype html") || strings.Contains(lower, "<html")
}
