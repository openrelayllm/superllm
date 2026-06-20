package adminplus

import (
	"net/http"
	"strings"
	"time"

	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	sessionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sessions"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type SessionHandler struct {
	service  *sessionsapp.Service
	balances *balancesapp.Service
}

func NewSessionHandler(service *sessionsapp.Service, balances *balancesapp.Service) *SessionHandler {
	return &SessionHandler{service: service, balances: balances}
}

type probeSupplierSessionRequest struct {
	LowBalanceThresholdCents int64 `json:"low_balance_threshold_cents"`
	RecordBalanceSnapshot    *bool `json:"record_balance_snapshot"`
}

type upsertSupplierBrowserSessionRequest struct {
	Origin        string         `json:"origin"`
	APIBaseURL    string         `json:"api_base_url"`
	SessionBundle map[string]any `json:"session_bundle"`
	CapturedAt    string         `json:"captured_at"`
	ExpiresAt     string         `json:"expires_at"`
}

func (h *SessionHandler) Get(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	session, err := h.service.Get(c.Request.Context(), supplierID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, sanitizeSupplierBrowserSession(session))
}

func (h *SessionHandler) Upsert(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	var req upsertSupplierBrowserSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if len(req.SessionBundle) == 0 {
		response.BadRequest(c, "session_bundle is required")
		return
	}
	session, err := h.service.Upsert(c.Request.Context(), sessionsapp.UpsertInput{
		SupplierID:     supplierID,
		Origin:         firstNonEmptySessionString(req.Origin, sessionStringValue(req.SessionBundle, "origin")),
		APIBaseURL:     firstNonEmptySessionString(req.APIBaseURL, sessionStringValueAt(req.SessionBundle, "context", "api_base_url"), sessionStringValue(req.SessionBundle, "api_base_url")),
		SessionSummary: supplierBrowserSessionSummary(req.SessionBundle),
		SessionBundle:  req.SessionBundle,
		CapturedAt:     parseSessionTime(req.CapturedAt),
		ExpiresAt:      parseOptionalSessionTime(req.ExpiresAt),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "success",
		"data":    sanitizeSupplierBrowserSession(session),
	})
}

func (h *SessionHandler) Probe(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	var req probeSupplierSessionRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "invalid request: "+err.Error())
			return
		}
	}
	probe, err := h.service.ProbeSub2APIUserProfile(c.Request.Context(), supplierID)
	if response.ErrorFrom(c, err) {
		return
	}
	out := gin.H{"probe": probe}
	shouldRecord := req.RecordBalanceSnapshot == nil || *req.RecordBalanceSnapshot
	if shouldRecord && h.balances != nil && probe.BalanceCents != nil {
		event, snapshot, err := h.balances.RecordSnapshot(c.Request.Context(), balancesapp.RecordSnapshotInput{
			SupplierID:               supplierID,
			Source:                   "provider_session",
			RuntimeStatus:            runtimeStatusForBalance(*probe.BalanceCents),
			BalanceCents:             *probe.BalanceCents,
			Currency:                 probe.BalanceCurrency,
			LowBalanceThresholdCents: req.LowBalanceThresholdCents,
			RawPayload: map[string]any{
				"provider":     probe.SystemType,
				"profile":      probe.Profile,
				"capabilities": probe.Capabilities,
			},
			CapturedAt: &probe.ProbedAt,
		})
		if response.ErrorFrom(c, err) {
			return
		}
		out["balance_snapshot"] = snapshot
		out["balance_event"] = event
	}
	response.Success(c, out)
}

func supplierBrowserSessionSummary(bundle map[string]any) map[string]any {
	tokens := sessionMapValue(bundle, "tokens")
	context := sessionMapValue(bundle, "context")
	requiredHeaders := sessionMapValue(bundle, "required_headers")
	cookiesRaw, _ := bundle["cookies"].([]any)
	cookieCount := len(cookiesRaw)
	if cookieCount == 0 && firstNonEmptySessionString(sessionStringValue(requiredHeaders, "cookie"), sessionStringValue(bundle, "cookie"), sessionStringValue(bundle, "cookies")) != "" {
		cookieCount = 1
	}
	return map[string]any{
		"origin":               sessionStringValue(bundle, "origin"),
		"captured_at":          sessionStringValue(bundle, "captured_at"),
		"expires_at":           sessionStringValue(bundle, "expires_at"),
		"has_access_token":     firstNonEmptySessionString(sessionStringValue(bundle, "access_token"), sessionStringValue(bundle, "accessToken"), sessionStringValue(tokens, "access_token"), sessionStringValue(tokens, "accessToken")) != "",
		"has_refresh_token":    firstNonEmptySessionString(sessionStringValue(bundle, "refresh_token"), sessionStringValue(bundle, "refreshToken"), sessionStringValue(tokens, "refresh_token"), sessionStringValue(tokens, "refreshToken")) != "",
		"has_csrf_token":       firstNonEmptySessionString(sessionStringValue(bundle, "csrf_token"), sessionStringValue(bundle, "csrfToken"), sessionStringValue(tokens, "csrf_token"), sessionStringValue(tokens, "csrfToken")) != "",
		"cookie_count":         cookieCount,
		"user_id":              sessionStringValue(context, "user_id"),
		"organization_id":      sessionStringValue(context, "organization_id"),
		"project_id":           sessionStringValue(context, "project_id"),
		"account_id":           sessionStringValue(context, "account_id"),
		"api_base_url":         sessionStringValue(context, "api_base_url"),
		"has_required_origin":  sessionStringValue(requiredHeaders, "origin") != "",
		"has_required_referer": sessionStringValue(requiredHeaders, "referer") != "",
	}
}

func parseSessionTime(raw string) time.Time {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func parseOptionalSessionTime(raw string) *time.Time {
	parsed := parseSessionTime(raw)
	if parsed.IsZero() {
		return nil
	}
	return &parsed
}

func sessionStringValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	if value, ok := values[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func sessionStringValueAt(values map[string]any, path ...string) string {
	var current any = values
	for _, key := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = obj[key]
	}
	if value, ok := current.(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func sessionMapValue(values map[string]any, key string) map[string]any {
	if values == nil {
		return nil
	}
	if value, ok := values[key].(map[string]any); ok {
		return value
	}
	return nil
}

func firstNonEmptySessionString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func sanitizeSupplierBrowserSession(session *adminplusdomain.SupplierBrowserSession) gin.H {
	if session == nil {
		return gin.H{}
	}
	status := "valid"
	if session.ExpiresAt != nil && session.ExpiresAt.Before(time.Now().UTC()) {
		status = "expired"
	}
	return gin.H{
		"supplier_id":              session.SupplierID,
		"origin":                   session.Origin,
		"api_base_url":             session.APIBaseURL,
		"session_summary":          session.SessionSummary,
		"captured_at":              session.CapturedAt,
		"expires_at":               session.ExpiresAt,
		"source_extension_task_id": session.SourceExtensionTaskID,
		"created_at":               session.CreatedAt,
		"updated_at":               session.UpdatedAt,
		"status":                   status,
		"has_encrypted_bundle":     session.SessionBundleCiphertext != "",
	}
}

func runtimeStatusForBalance(balanceCents int64) adminplusdomain.SupplierRuntimeStatus {
	if balanceCents > 0 {
		return adminplusdomain.SupplierRuntimeStatusCandidate
	}
	return adminplusdomain.SupplierRuntimeStatusMonitorOnly
}
