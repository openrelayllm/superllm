package sessions

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type UpsertInput struct {
	SupplierID              int64
	SessionSource           adminplusdomain.SupplierSessionSource
	Origin                  string
	APIBaseURL              string
	SessionSummary          map[string]any
	SessionBundle           map[string]any
	SessionBundleCiphertext string
	CapturedAt              time.Time
	ExpiresAt               *time.Time
	SourceExtensionTaskID   int64
}

type LoginInput struct {
	SupplierID   int64
	Origin       string
	APIBaseURL   string
	Username     string
	Password     string
	Token        string
	LoginContext map[string]any
}

type LoginResult struct {
	Session     *adminplusdomain.SupplierBrowserSession `json:"session"`
	Diagnostics map[string]any                          `json:"diagnostics,omitempty"`
}

type Repository interface {
	Upsert(ctx context.Context, session *adminplusdomain.SupplierBrowserSession) (*adminplusdomain.SupplierBrowserSession, error)
	Get(ctx context.Context, supplierID int64) (*adminplusdomain.SupplierBrowserSession, error)
}

type SupplierLookup interface {
	Get(ctx context.Context, id int64) (*adminplusdomain.Supplier, error)
}

type SupplierCredentialLookup interface {
	GetBrowserCredential(ctx context.Context, id int64) (*adminplusdomain.SupplierBrowserCredential, error)
}

type Cipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

type Service struct {
	repo        Repository
	cipher      Cipher
	suppliers   SupplierLookup
	credentials SupplierCredentialLookup
	prober      ports.SessionProbeAdapter
	monitors    ports.SessionChannelMonitorAdapter
	login       ports.SessionLoginAdapter
	bizlog      *bizlogs.Recorder
	now         func() time.Time
}

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

var (
	newAPIUnderscoreIDPattern = regexp.MustCompile(`_(\d{1,10})`)
	newAPIContextIDPattern    = regexp.MustCompile(`(?i)(?:user(?:_?id|name)?|uid|id)[^0-9]{0,16}(\d{1,10})`)
)

func NewService(repo Repository, cipher Cipher) *Service {
	return &Service{
		repo:   repo,
		cipher: cipher,
		now:    time.Now,
	}
}

func NewServiceWithSupplier(repo Repository, cipher Cipher, suppliers SupplierLookup) *Service {
	return NewServiceWithDependencies(repo, cipher, suppliers, nil)
}

func NewServiceWithDependencies(repo Repository, cipher Cipher, suppliers SupplierLookup, prober ports.SessionProbeAdapter, login ...ports.SessionLoginAdapter) *Service {
	service := NewService(repo, cipher)
	service.suppliers = suppliers
	if credentials, ok := any(suppliers).(SupplierCredentialLookup); ok {
		service.credentials = credentials
	}
	service.prober = prober
	if len(login) > 0 {
		service.login = login[0]
	}
	return service
}

func (s *Service) WithChannelMonitorReader(reader ports.SessionChannelMonitorAdapter) *Service {
	if s != nil {
		s.monitors = reader
	}
	return s
}

func (s *Service) WithDiagnostics(recorder *bizlogs.Recorder) *Service {
	if s != nil {
		s.bizlog = recorder
	}
	return s
}

func (s *Service) Upsert(ctx context.Context, in UpsertInput) (*adminplusdomain.SupplierBrowserSession, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier browser session service is not configured")
	}
	if in.SupplierID <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if err := s.validateSessionScope(ctx, in.SupplierID, in.Origin, in.APIBaseURL, in.SessionBundle); err != nil {
		return nil, err
	}
	source := in.SessionSource
	if source == "" {
		source = adminplusdomain.SupplierSessionSourceBrowserExtension
	}
	if !source.Valid() {
		return nil, badRequest("SUPPLIER_SESSION_SOURCE_INVALID", "invalid supplier session source")
	}
	ciphertext := strings.TrimSpace(in.SessionBundleCiphertext)
	if ciphertext == "" {
		if s.cipher == nil {
			return nil, internalError("supplier browser session cipher is not configured")
		}
		raw, err := json.Marshal(in.SessionBundle)
		if err != nil {
			return nil, err
		}
		encrypted, err := s.cipher.Encrypt(string(raw))
		if err != nil {
			return nil, err
		}
		ciphertext = encrypted
	}
	capturedAt := in.CapturedAt.UTC()
	if capturedAt.IsZero() {
		capturedAt = s.now().UTC()
	}
	now := s.now().UTC()
	return s.repo.Upsert(ctx, &adminplusdomain.SupplierBrowserSession{
		SupplierID:              in.SupplierID,
		SessionSource:           source,
		Origin:                  trimLimit(in.Origin, 240),
		APIBaseURL:              trimLimit(in.APIBaseURL, 240),
		SessionSummary:          cloneMap(in.SessionSummary),
		SessionBundleCiphertext: ciphertext,
		CapturedAt:              capturedAt,
		ExpiresAt:               in.ExpiresAt,
		SourceExtensionTaskID:   in.SourceExtensionTaskID,
		CreatedAt:               now,
		UpdatedAt:               now,
	})
}

func (s *Service) Login(ctx context.Context, in LoginInput) (*LoginResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier browser session service is not configured")
	}
	if s.login == nil {
		return nil, internalError("supplier direct login adapter is not configured")
	}
	if in.SupplierID <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if s.suppliers == nil {
		return nil, internalError("supplier lookup is not configured")
	}
	supplier, err := s.suppliers.Get(ctx, in.SupplierID)
	if err != nil {
		return nil, err
	}
	if supplier.Type != adminplusdomain.SupplierTypeSub2API && supplier.Type != adminplusdomain.SupplierTypeNewAPI {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_DIRECT_LOGIN_UNSUPPORTED", "supplier direct login is only implemented for Sub2API and New API suppliers")
	}
	username := strings.TrimSpace(in.Username)
	password := in.Password
	token := in.Token
	apiBaseURL := firstNonEmpty(in.APIBaseURL, supplier.APIBaseURL, supplier.DashboardURL)
	origin := firstOrigin(in.Origin, supplier.DashboardURL, apiBaseURL)
	if username == "" && password == "" && token == "" {
		if s.credentials == nil {
			return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED", "supplier login credential is required")
		}
		credential, err := s.credentials.GetBrowserCredential(ctx, in.SupplierID)
		if err != nil {
			return nil, err
		}
		username = strings.TrimSpace(credential.Username)
		password = credential.Password
		token = credential.Token
		apiBaseURL = firstNonEmpty(in.APIBaseURL, credential.APIBaseURL, supplier.APIBaseURL, credential.DashboardURL, supplier.DashboardURL)
		origin = firstOrigin(in.Origin, credential.DashboardURL, supplier.DashboardURL, apiBaseURL)
	}
	if !nonBlankSecret(token) && (username == "" || !nonBlankSecret(password)) {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED", "supplier username and password or access token is required")
	}
	loginContext := cloneMap(in.LoginContext)
	loginContext["provider_type"] = string(supplier.Type)
	loginContext["supplier_type"] = string(supplier.Type)
	loginResult, err := s.login.DirectLogin(ctx, ports.DirectLoginInput{
		SupplierID:   in.SupplierID,
		Origin:       origin,
		APIBaseURL:   apiBaseURL,
		Username:     username,
		Password:     password,
		Token:        token,
		LoginContext: loginContext,
	})
	if err != nil {
		s.recordLoginFailure(ctx, supplier, err, apiBaseURL, origin)
		return nil, err
	}
	if loginResult == nil || len(loginResult.SessionBundle) == 0 {
		err := infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_EMPTY_SESSION", "supplier direct login returned empty session")
		s.recordLoginFailure(ctx, supplier, err, apiBaseURL, origin)
		return nil, err
	}
	session, err := s.Upsert(ctx, UpsertInput{
		SupplierID:     in.SupplierID,
		SessionSource:  adminplusdomain.SupplierSessionSourceDirectLogin,
		Origin:         firstNonEmpty(loginResult.Origin, origin),
		APIBaseURL:     firstNonEmpty(loginResult.APIBaseURL, apiBaseURL),
		SessionSummary: sessionSummaryFromBundle(loginResult.SessionBundle),
		SessionBundle:  loginResult.SessionBundle,
		CapturedAt:     loginResult.CapturedAt,
		ExpiresAt:      loginResult.ExpiresAt,
	})
	if err != nil {
		s.recordLoginFailure(ctx, supplier, err, apiBaseURL, origin)
		return nil, err
	}
	s.recordLoginSuccess(ctx, supplier, session, loginResult)
	return &LoginResult{
		Session:     session,
		Diagnostics: cloneMap(loginResult.Diagnostics),
	}, nil
}

func (s *Service) recordLoginFailure(ctx context.Context, supplier *adminplusdomain.Supplier, err error, apiBaseURL string, origin string) {
	if s == nil || s.bizlog == nil {
		return
	}
	event := bizlogs.EventFromError(bizlogs.Event{
		Level:        bizlogs.LevelWarn,
		Category:     bizlogs.CategoryLogin,
		Action:       "direct_login",
		Outcome:      bizlogs.OutcomeFailed,
		Message:      "supplier direct login failed",
		SupplierID:   supplierIDFromSupplier(supplier),
		SupplierName: supplierNameFromSupplier(supplier),
		ProviderType: supplierTypeFromSupplier(supplier),
		Metadata: map[string]any{
			"api_base_url": apiBaseURL,
			"origin":       origin,
		},
	}, err)
	s.bizlog.Record(ctx, event)
}

func (s *Service) recordLoginSuccess(ctx context.Context, supplier *adminplusdomain.Supplier, session *adminplusdomain.SupplierBrowserSession, result *ports.DirectLoginResult) {
	if s == nil || s.bizlog == nil {
		return
	}
	metadata := map[string]any{}
	if session != nil {
		metadata["origin"] = session.Origin
		metadata["api_base_url"] = session.APIBaseURL
		metadata["session_source"] = string(session.SessionSource)
	}
	if result != nil && len(result.Diagnostics) > 0 {
		for key, value := range result.Diagnostics {
			metadata[key] = value
		}
	}
	s.bizlog.Record(ctx, bizlogs.Event{
		Level:        bizlogs.LevelInfo,
		Category:     bizlogs.CategoryLogin,
		Action:       "direct_login",
		Outcome:      bizlogs.OutcomeSucceeded,
		Message:      "supplier direct login succeeded",
		SupplierID:   supplierIDFromSupplier(supplier),
		SupplierName: supplierNameFromSupplier(supplier),
		ProviderType: supplierTypeFromSupplier(supplier),
		Metadata:     metadata,
	})
}

func supplierIDFromSupplier(supplier *adminplusdomain.Supplier) int64 {
	if supplier == nil {
		return 0
	}
	return supplier.ID
}

func supplierNameFromSupplier(supplier *adminplusdomain.Supplier) string {
	if supplier == nil {
		return ""
	}
	return supplier.Name
}

func supplierTypeFromSupplier(supplier *adminplusdomain.Supplier) string {
	if supplier == nil {
		return ""
	}
	return string(supplier.Type)
}

func (s *Service) Get(ctx context.Context, supplierID int64) (*adminplusdomain.SupplierBrowserSession, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier browser session service is not configured")
	}
	if supplierID <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	return s.repo.Get(ctx, supplierID)
}

func sessionSummaryFromBundle(bundle map[string]any) map[string]any {
	tokens := mapValue(bundle, "tokens")
	context := mapValue(bundle, "context")
	requiredHeaders := mapValue(bundle, "required_headers")
	cookiesRaw, _ := bundle["cookies"].([]any)
	cookieCount := len(cookiesRaw)
	if cookieCount == 0 && firstNonEmpty(stringValue(requiredHeaders, "cookie"), stringValue(bundle, "cookie"), stringValue(bundle, "cookies")) != "" {
		cookieCount = 1
	}
	providerType := firstNonEmpty(stringValue(bundle, "provider_type"), stringValue(context, "provider_type"))
	systemType := firstNonEmpty(stringValue(bundle, "system_type"), stringValue(context, "system_type"), providerType)
	newAPIUserHeader := firstNonEmpty(
		newAPIUserIDFromBundle(bundle, context, requiredHeaders),
		stringValue(bundle, "auth_header_value"),
	)
	return map[string]any{
		"origin":                  stringValue(bundle, "origin"),
		"provider_type":           providerType,
		"system_type":             systemType,
		"session_source":          stringValue(bundle, "session_source"),
		"captured_at":             stringValue(bundle, "captured_at"),
		"expires_at":              stringValue(bundle, "expires_at"),
		"has_access_token":        firstNonEmpty(stringValue(bundle, "access_token"), stringValue(bundle, "accessToken"), stringValue(tokens, "access_token"), stringValue(tokens, "accessToken")) != "",
		"has_refresh_token":       firstNonEmpty(stringValue(bundle, "refresh_token"), stringValue(bundle, "refreshToken"), stringValue(tokens, "refresh_token"), stringValue(tokens, "refreshToken")) != "",
		"has_csrf_token":          firstNonEmpty(stringValue(bundle, "csrf_token"), stringValue(bundle, "csrfToken"), stringValue(tokens, "csrf_token"), stringValue(tokens, "csrfToken")) != "",
		"has_new_api_user_header": newAPIUserHeader != "" || strings.EqualFold(stringValue(bundle, "auth_header_name"), "New-Api-User"),
		"cookie_count":            cookieCount,
		"user_id":                 stringValue(context, "user_id"),
		"role":                    firstNonEmpty(stringValue(context, "role"), stringValue(bundle, "role")),
		"organization_id":         stringValue(context, "organization_id"),
		"project_id":              stringValue(context, "project_id"),
		"account_id":              stringValue(context, "account_id"),
		"api_base_url":            stringValue(context, "api_base_url"),
		"login_method":            stringValue(context, "login_method"),
		"has_required_origin":     stringValue(requiredHeaders, "origin") != "",
		"has_required_referer":    stringValue(requiredHeaders, "referer") != "",
	}
}

func (s *Service) DecryptedBundle(ctx context.Context, supplierID int64) (map[string]any, *adminplusdomain.SupplierBrowserSession, error) {
	session, err := s.Get(ctx, supplierID)
	if err != nil {
		return nil, nil, err
	}
	if session.ExpiresAt != nil && session.ExpiresAt.Before(s.now().UTC()) {
		return nil, nil, infraerrors.New(http.StatusConflict, "SUPPLIER_SESSION_EXPIRED", "supplier browser session is expired")
	}
	if s.cipher == nil {
		return nil, nil, internalError("supplier browser session cipher is not configured")
	}
	plain, err := s.cipher.Decrypt(session.SessionBundleCiphertext)
	if err != nil {
		return nil, nil, infraerrors.New(http.StatusConflict, "SUPPLIER_SESSION_DECRYPT_FAILED", "supplier browser session cannot be decrypted; refresh supplier session").WithCause(err)
	}
	var bundle map[string]any
	if err := json.Unmarshal([]byte(plain), &bundle); err != nil {
		return nil, nil, err
	}
	if normalizeSessionBundleForRead(bundle, session.APIBaseURL, session.Origin) {
		s.repairDecryptedBundle(ctx, session, bundle)
	}
	return bundle, session, nil
}

func (s *Service) repairDecryptedBundle(ctx context.Context, session *adminplusdomain.SupplierBrowserSession, bundle map[string]any) {
	if s == nil || s.repo == nil || s.cipher == nil || session == nil || len(bundle) == 0 {
		return
	}
	raw, err := json.Marshal(bundle)
	if err != nil {
		return
	}
	ciphertext, err := s.cipher.Encrypt(string(raw))
	if err != nil {
		return
	}
	repaired := *session
	repaired.SessionBundleCiphertext = ciphertext
	repaired.SessionSummary = sessionSummaryFromBundle(bundle)
	repaired.UpdatedAt = s.now().UTC()
	if repaired.CreatedAt.IsZero() {
		repaired.CreatedAt = repaired.UpdatedAt
	}
	if repaired.CapturedAt.IsZero() {
		repaired.CapturedAt = repaired.UpdatedAt
	}
	if saved, err := s.repo.Upsert(ctx, &repaired); err == nil && saved != nil {
		*session = *saved
	}
}

func (s *Service) DecryptedProbeInput(ctx context.Context, supplierID int64) (ports.SessionProbeInput, error) {
	bundle, session, err := s.DecryptedBundle(ctx, supplierID)
	if err != nil {
		return ports.SessionProbeInput{}, err
	}
	if err := s.validateSessionScope(ctx, supplierID, session.Origin, session.APIBaseURL, bundle); err != nil {
		return ports.SessionProbeInput{}, err
	}
	return ports.SessionProbeInput{
		SupplierID: supplierID,
		Origin:     session.Origin,
		APIBaseURL: session.APIBaseURL,
		Bundle:     bundle,
	}, nil
}

func (s *Service) ProbeSub2APIUserProfile(ctx context.Context, supplierID int64) (*ports.SessionProbeResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier browser session service is not configured")
	}
	if s.prober == nil {
		return nil, internalError("supplier provider adapter is not configured")
	}
	input, err := s.DecryptedProbeInput(ctx, supplierID)
	if err != nil {
		return nil, err
	}
	return s.prober.ProbeSub2APIUserProfile(ctx, input)
}

func (s *Service) ReadChannelMonitors(ctx context.Context, supplierID int64) (*ports.ReadChannelMonitorsResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier browser session service is not configured")
	}
	if s.monitors == nil {
		return nil, internalError("supplier channel monitor adapter is not configured")
	}
	input, err := s.DecryptedProbeInput(ctx, supplierID)
	if err != nil {
		return nil, err
	}
	return s.monitors.ReadChannelMonitors(ctx, input)
}

func normalizeSessionBundleForRead(bundle map[string]any, apiBaseURL string, origin string) bool {
	providerType := normalizeSessionProviderType(firstNonEmpty(
		stringValue(bundle, "provider_type"),
		stringValue(bundle, "system_type"),
		stringValueAt(bundle, "context", "provider_type"),
		stringValueAt(bundle, "context", "system_type"),
	))
	if providerType != "new_api" {
		return false
	}
	changed := false
	contextValue := mapValue(bundle, "context")
	if contextValue == nil {
		contextValue = map[string]any{}
		bundle["context"] = contextValue
		changed = true
	}
	requiredHeaders := mapValue(bundle, "required_headers")
	if requiredHeaders == nil {
		requiredHeaders = map[string]any{}
		bundle["required_headers"] = requiredHeaders
		changed = true
	}
	userID := firstNonEmpty(
		newAPIUserIDFromBundle(bundle, contextValue, requiredHeaders),
	)
	changed = setStringValue(bundle, "provider_type", "new_api") || changed
	changed = setStringValue(bundle, "system_type", "new_api") || changed
	changed = setStringValue(contextValue, "provider_type", "new_api") || changed
	changed = setStringValue(contextValue, "system_type", "new_api") || changed
	if userID != "" {
		changed = setStringValue(bundle, "auth_header_name", "New-Api-User") || changed
		changed = setStringValue(bundle, "auth_header_value", userID) || changed
		changed = setStringValue(contextValue, "user_id", userID) || changed
		for _, headerName := range newAPIUserIDHeaderNames {
			changed = setStringValue(requiredHeaders, headerName, userID) || changed
		}
	}
	apiBaseURL = firstNonEmpty(stringValue(contextValue, "api_base_url"), stringValue(bundle, "api_base_url"), apiBaseURL, origin)
	if apiBaseURL != "" {
		changed = setStringValue(contextValue, "api_base_url", apiBaseURL) || changed
		changed = setStringValue(bundle, "api_base_url", apiBaseURL) || changed
	}
	origin = firstNonEmpty(stringValue(bundle, "origin"), originFromRawURL(origin), originFromRawURL(apiBaseURL))
	if origin != "" {
		changed = setStringValue(bundle, "origin", origin) || changed
		changed = setStringValue(requiredHeaders, "origin", origin) || changed
		if stringValue(requiredHeaders, "referer") == "" {
			changed = setStringValue(requiredHeaders, "referer", strings.TrimRight(origin, "/")+"/") || changed
		}
	}
	return changed
}

func newAPIUserIDFromRequiredHeaders(requiredHeaders map[string]any) string {
	values := make([]string, 0, len(newAPIUserIDHeaderNames)+1)
	for _, headerName := range newAPIUserIDHeaderNames {
		values = append(values, stringValue(requiredHeaders, headerName))
	}
	values = append(values, stringValue(requiredHeaders, "new-api-user"))
	return firstNonEmpty(values...)
}

func newAPIUserIDFromBundle(bundle map[string]any, contextValue map[string]any, requiredHeaders map[string]any) string {
	return firstNonEmpty(
		newAPIUserIDFromRequiredHeaders(requiredHeaders),
		stringValue(bundle, "auth_header_value"),
		stringValue(contextValue, "user_id"),
		stringValue(contextValue, "id"),
		stringValue(contextValue, "uid"),
		uniqueNewAPIUserIDFromEvidence(bundle, requiredHeaders),
	)
}

func uniqueNewAPIUserIDFromEvidence(bundle map[string]any, requiredHeaders map[string]any) string {
	candidates := make([]string, 0, 2)
	push := func(value string) {
		id := normalizePositiveIntegerText(value)
		if id == "" {
			return
		}
		for _, existing := range candidates {
			if existing == id {
				return
			}
		}
		candidates = append(candidates, id)
	}
	for _, value := range []string{
		stringValue(bundle, "access_token"),
		stringValue(bundle, "accessToken"),
		stringValueAt(bundle, "tokens", "access_token"),
		stringValueAt(bundle, "tokens", "accessToken"),
		stringValue(requiredHeaders, "cookie"),
		stringValue(bundle, "cookie"),
		stringValue(bundle, "cookies"),
	} {
		collectNewAPIUserIDsFromText(value, push)
	}
	for _, cookie := range cookieItemsFromBundle(bundle) {
		collectNewAPIUserIDsFromText(cookie, push)
	}
	if len(candidates) != 1 {
		return ""
	}
	return candidates[0]
}

func cookieItemsFromBundle(bundle map[string]any) []string {
	items, ok := bundle["cookies"].([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items)*2)
	for _, item := range items {
		cookie, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := stringFromAny(cookie["name"])
		value := stringFromAny(cookie["value"])
		if name != "" || value != "" {
			out = append(out, name+"="+value, value)
		}
	}
	return out
}

func collectNewAPIUserIDsFromText(value string, push func(string)) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return
	}
	variants := []string{raw}
	if decoded, err := url.QueryUnescape(raw); err == nil && decoded != "" && decoded != raw {
		variants = append(variants, decoded)
	}
	if payload := decodeJWTPayload(raw); payload != "" {
		variants = append(variants, payload)
	}
	for _, part := range strings.Split(raw, ";") {
		if _, cookieValue, ok := strings.Cut(strings.TrimSpace(part), "="); ok {
			cookieValue = strings.TrimSpace(cookieValue)
			if cookieValue != "" {
				variants = append(variants, cookieValue)
			}
		}
	}
	for index := 0; index < len(variants) && index < 64; index++ {
		text := variants[index]
		if decoded := decodeBase64Loose(text); decoded != "" {
			variants = append(variants, decoded)
			for _, part := range strings.Split(decoded, "|") {
				if nested := decodeBase64Loose(part); nested != "" {
					variants = append(variants, nested)
				}
			}
		}
		collectNewAPIUserIDsFromJSON(text, push)
		if isLikelyNewAPIUserEvidenceText(text) {
			for _, match := range newAPIUnderscoreIDPattern.FindAllStringSubmatch(text, -1) {
				if len(match) > 1 {
					push(match[1])
				}
			}
			for _, match := range newAPIContextIDPattern.FindAllStringSubmatch(text, -1) {
				if len(match) > 1 {
					push(match[1])
				}
			}
			collectNewAPIGobUserIDs([]byte(text), push)
		}
	}
}

func isLikelyNewAPIUserEvidenceText(value string) bool {
	text := strings.TrimSpace(value)
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	return strings.ContainsAny(text, "{}[]\":_|;") ||
		strings.Contains(lower, "user") ||
		strings.Contains(lower, "uid") ||
		strings.Contains(text, "\x03int\x04")
}

func decodeJWTPayload(value string) string {
	token := strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	parts := strings.Split(token, ".")
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
	return string(payload)
}

func decodeBase64Loose(value string) string {
	raw := strings.TrimSpace(value)
	if raw == "" || len(raw) > 8192 {
		return ""
	}
	encodings := []*base64.Encoding{
		base64.RawStdEncoding,
		base64.StdEncoding,
		base64.RawURLEncoding,
		base64.URLEncoding,
	}
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(raw)
		if err == nil && len(decoded) > 0 {
			return string(decoded)
		}
	}
	return ""
}

func collectNewAPIUserIDsFromJSON(text string, push func(string)) {
	raw := strings.TrimSpace(text)
	if raw == "" || (!strings.HasPrefix(raw, "{") && !strings.HasPrefix(raw, "[")) {
		return
	}
	var value any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return
	}
	collectNewAPIUserIDsFromValue(value, push, 0)
}

func collectNewAPIUserIDsFromValue(value any, push func(string), depth int) {
	if value == nil || depth > 5 {
		return
	}
	switch v := value.(type) {
	case map[string]any:
		for _, key := range []string{"id", "sub", "user_id", "userId", "uid"} {
			push(stringFromAny(v[key]))
		}
		for _, nested := range v {
			collectNewAPIUserIDsFromValue(nested, push, depth+1)
		}
	case []any:
		for _, nested := range v {
			collectNewAPIUserIDsFromValue(nested, push, depth+1)
		}
	}
}

func collectNewAPIGobUserIDs(payload []byte, push func(string)) {
	if len(payload) == 0 {
		return
	}
	for _, fieldName := range []string{"id", "Id", "uid", "UID", "user_id", "UserID", "UserId"} {
		for _, id := range extractGobFieldInts(payload, fieldName) {
			push(strconv.FormatInt(id, 10))
		}
	}
}

func extractGobFieldInts(payload []byte, fieldName string) []int64 {
	marker := append([]byte(fieldName), 0x03)
	marker = append(marker, []byte("int")...)
	marker = append(marker, 0x04)
	values := []int64{}
	start := 0
	for start < len(payload) {
		position := bytes.Index(payload[start:], marker)
		if position < 0 {
			break
		}
		position += start
		if position+len(marker)+2 <= len(payload) {
			encodedLength := int(payload[position+len(marker)])
			delimiter := payload[position+len(marker)+1]
			byteLength := encodedLength - 1
			valueStart := position + len(marker) + 2
			valueEnd := valueStart + byteLength
			if delimiter == 0x00 && byteLength > 0 && valueEnd <= len(payload) {
				if decoded := decodeGobSignedInt(payload[valueStart:valueEnd]); decoded > 0 {
					values = append(values, decoded)
				}
			}
		}
		start = position + len(marker)
	}
	return values
}

func decodeGobSignedInt(encoded []byte) int64 {
	if len(encoded) == 0 {
		return 0
	}
	var unsigned uint64
	if encoded[0] < 0x80 {
		unsigned = uint64(encoded[0])
	} else {
		width := 0x100 - int(encoded[0])
		if width <= 0 || width > 8 || len(encoded) != width+1 {
			return 0
		}
		for index := 1; index < len(encoded); index++ {
			unsigned = (unsigned << 8) | uint64(encoded[index])
		}
	}
	var signed int64
	if unsigned&1 == 0 {
		signed = int64(unsigned >> 1)
	} else {
		signed = -int64((unsigned >> 1) + 1)
	}
	if signed <= 0 || signed > 10000000 {
		return 0
	}
	return signed
}

func normalizePositiveIntegerText(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil || n <= 0 || n > 10000000 {
		return ""
	}
	return strconv.FormatInt(n, 10)
}

func normalizeSessionProviderType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "newapi", "new-api":
		return "new_api"
	case "subapi", "sub api", "sub-api", "sub_api", "sub2api", "sub2 api", "sub2-api", "sub2_api":
		return "sub2api"
	default:
		return normalized
	}
}

func setStringValue(values map[string]any, key string, value string) bool {
	value = strings.TrimSpace(value)
	if values == nil || value == "" || stringValue(values, key) == value {
		return false
	}
	values[key] = value
	return true
}

func (s *Service) validateSessionScope(ctx context.Context, supplierID int64, origin string, apiBaseURL string, bundle map[string]any) error {
	origin = firstNonEmpty(origin, stringValue(bundle, "origin"))
	apiBaseURL = firstNonEmpty(apiBaseURL, stringValueAt(bundle, "context", "api_base_url"), stringValue(bundle, "api_base_url"))
	if origin != "" {
		if _, err := parseSafeURL(origin, "SUPPLIER_SESSION_ORIGIN_INVALID"); err != nil {
			return err
		}
	}
	if apiBaseURL != "" {
		if _, err := parseSafeURL(apiBaseURL, "SUPPLIER_SESSION_API_BASE_URL_INVALID"); err != nil {
			return err
		}
	}
	if s.suppliers == nil {
		return nil
	}
	supplier, err := s.suppliers.Get(ctx, supplierID)
	if err != nil {
		return err
	}
	allowedHosts := map[string]struct{}{}
	addURLHost(allowedHosts, supplier.DashboardURL)
	addURLHost(allowedHosts, supplier.APIBaseURL)
	if len(allowedHosts) == 0 {
		return infraerrors.New(http.StatusConflict, "SUPPLIER_SESSION_BASE_URL_REQUIRED", "supplier dashboard url or api base url is required before accepting browser session")
	}
	for _, raw := range []string{origin, apiBaseURL} {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		parsed, err := parseSafeURL(raw, "SUPPLIER_SESSION_URL_INVALID")
		if err != nil {
			return err
		}
		if _, ok := allowedHosts[canonicalHost(parsed)]; !ok {
			return infraerrors.New(http.StatusBadRequest, "SUPPLIER_SESSION_HOST_NOT_ALLOWED", "supplier session url host does not match configured supplier")
		}
	}
	return nil
}

func trimLimit(value string, limit int) string {
	v := strings.TrimSpace(value)
	if len(v) <= limit {
		return v
	}
	return v[:limit]
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func addURLHost(hosts map[string]struct{}, raw string) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return
	}
	hosts[canonicalHost(u)] = struct{}{}
}

func parseSafeURL(raw string, reason string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, badRequest(reason, "invalid supplier session url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, badRequest(reason, "supplier session url must use http or https")
	}
	if u.User != nil {
		return nil, badRequest(reason, "supplier session url must not contain user info")
	}
	return u, nil
}

func canonicalHost(u *url.URL) string {
	if u == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(u.Host))
}

func originFromRawURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func firstOrigin(values ...string) string {
	for _, value := range values {
		if origin := originFromRawURL(value); origin != "" {
			return origin
		}
	}
	return ""
}

func nonBlankSecret(value string) bool {
	return strings.TrimSpace(value) != ""
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
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case int:
		return strconv.Itoa(v)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float64:
		if math.IsInf(v, 0) || math.IsNaN(v) || math.Trunc(v) != v {
			return ""
		}
		return strconv.FormatFloat(v, 'f', 0, 64)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return strconv.FormatInt(parsed, 10)
		}
		parsed, err := v.Float64()
		if err != nil || math.IsInf(parsed, 0) || math.IsNaN(parsed) || math.Trunc(parsed) != parsed {
			return ""
		}
		return strconv.FormatFloat(parsed, 'f', 0, 64)
	default:
		return ""
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

func badRequest(reason string, message string) error {
	return infraerrors.New(http.StatusBadRequest, reason, message)
}

func internalError(message string) error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", message)
}
