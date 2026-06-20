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
	Origin                  string
	APIBaseURL              string
	SessionSummary          map[string]any
	SessionBundle           map[string]any
	SessionBundleCiphertext string
	CapturedAt              time.Time
	ExpiresAt               *time.Time
	SourceExtensionTaskID   int64
}

type Repository interface {
	Upsert(ctx context.Context, session *adminplusdomain.SupplierBrowserSession) (*adminplusdomain.SupplierBrowserSession, error)
	Get(ctx context.Context, supplierID int64) (*adminplusdomain.SupplierBrowserSession, error)
}

type SupplierLookup interface {
	Get(ctx context.Context, id int64) (*adminplusdomain.Supplier, error)
}

type Cipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

type Service struct {
	repo      Repository
	cipher    Cipher
	suppliers SupplierLookup
	prober    ports.SessionProbeAdapter
	now       func() time.Time
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

func NewServiceWithDependencies(repo Repository, cipher Cipher, suppliers SupplierLookup, prober ports.SessionProbeAdapter) *Service {
	service := NewService(repo, cipher)
	service.suppliers = suppliers
	service.prober = prober
	return service
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

func (s *Service) Get(ctx context.Context, supplierID int64) (*adminplusdomain.SupplierBrowserSession, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier browser session service is not configured")
	}
	if supplierID <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	return s.repo.Get(ctx, supplierID)
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
		return nil, nil, err
	}
	var bundle map[string]any
	if err := json.Unmarshal([]byte(plain), &bundle); err != nil {
		return nil, nil, err
	}
	return bundle, session, nil
}

func (s *Service) ProbeSub2APIUserProfile(ctx context.Context, supplierID int64) (*ports.SessionProbeResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier browser session service is not configured")
	}
	if s.prober == nil {
		return nil, internalError("supplier provider adapter is not configured")
	}
	bundle, session, err := s.DecryptedBundle(ctx, supplierID)
	if err != nil {
		return nil, err
	}
	if err := s.validateSessionScope(ctx, supplierID, session.Origin, session.APIBaseURL, bundle); err != nil {
		return nil, err
	}
	return s.prober.ProbeSub2APIUserProfile(ctx, ports.SessionProbeInput{
		SupplierID: supplierID,
		Origin:     session.Origin,
		APIBaseURL: session.APIBaseURL,
		Bundle:     bundle,
	})
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

func stringValue(in map[string]any, key string) string {
	if in == nil {
		return ""
	}
	return stringFromAny(in[key])
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
