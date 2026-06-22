package sessions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

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
	now         func() time.Time
}

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
	password := strings.TrimSpace(in.Password)
	token := strings.TrimSpace(in.Token)
	apiBaseURL := firstNonEmpty(in.APIBaseURL, supplier.APIBaseURL, supplier.DashboardURL)
	origin := firstNonEmpty(in.Origin, supplier.DashboardURL, originFromRawURL(apiBaseURL))
	if username == "" && password == "" && token == "" {
		if s.credentials == nil {
			return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED", "supplier login credential is required")
		}
		credential, err := s.credentials.GetBrowserCredential(ctx, in.SupplierID)
		if err != nil {
			return nil, err
		}
		username = strings.TrimSpace(credential.Username)
		password = strings.TrimSpace(credential.Password)
		token = strings.TrimSpace(credential.Token)
		apiBaseURL = firstNonEmpty(in.APIBaseURL, credential.APIBaseURL, supplier.APIBaseURL, credential.DashboardURL, supplier.DashboardURL)
		origin = firstNonEmpty(in.Origin, credential.DashboardURL, supplier.DashboardURL, originFromRawURL(apiBaseURL))
	}
	if token == "" && (username == "" || password == "") {
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
		return nil, err
	}
	if loginResult == nil || len(loginResult.SessionBundle) == 0 {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_LOGIN_EMPTY_SESSION", "supplier direct login returned empty session")
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
		return nil, err
	}
	return &LoginResult{
		Session:     session,
		Diagnostics: cloneMap(loginResult.Diagnostics),
	}, nil
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
		stringValue(requiredHeaders, "New-Api-User"),
		stringValue(requiredHeaders, "new-api-user"),
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
	return bundle, session, nil
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
