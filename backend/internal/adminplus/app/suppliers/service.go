package suppliers

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type CreateSupplierInput struct {
	Name                  string
	Kind                  adminplusdomain.SupplierKind
	Type                  adminplusdomain.SupplierType
	RuntimeStatus         adminplusdomain.SupplierRuntimeStatus
	HealthStatus          adminplusdomain.SupplierHealthStatus
	DashboardURL          string
	APIBaseURL            string
	ThirdPartyRechargeURL string
	LocalRechargeURL      string
	Contact               string
	Notes                 string
	PostgresReadDSN       string
	RedisReadDSN          string
	BrowserLoginEnabled   bool
	BrowserLoginUsername  string
	BrowserLoginPassword  string
	BrowserLoginToken     string
	BalanceCents          int64
	BalanceCurrency       string
	RechargeMultiplier    float64
}

type UpdateSupplierInput struct {
	Name                  string
	Kind                  adminplusdomain.SupplierKind
	Type                  adminplusdomain.SupplierType
	RuntimeStatus         adminplusdomain.SupplierRuntimeStatus
	HealthStatus          adminplusdomain.SupplierHealthStatus
	DashboardURL          string
	APIBaseURL            string
	ThirdPartyRechargeURL string
	LocalRechargeURL      string
	Contact               string
	Notes                 string
	PostgresReadDSN       string
	RedisReadDSN          string
	BrowserLoginEnabled   bool
	BrowserLoginUsername  string
	BrowserLoginPassword  string
	BrowserLoginToken     string
	BalanceCents          int64
	BalanceCurrency       string
	RechargeMultiplier    float64
}

type CreateSupplierAccountInput struct {
	SupplierID                int64
	LocalSub2APIAccountID     int64
	SupplierAccountIdentifier string
	SupplierAccountLabel      string
	OrganizationID            string
	ProjectID                 string
	RateProfile               string
	ConfiguredConcurrency     int
	BalanceThresholdCents     int64
	BalanceCents              int64
	BalanceCurrency           string
	RuntimeStatus             adminplusdomain.SupplierRuntimeStatus
	HealthStatus              adminplusdomain.SupplierHealthStatus
}

type UpdateSupplierAccountInput struct {
	SupplierID                int64
	AccountID                 int64
	SupplierAccountIdentifier string
	SupplierAccountLabel      string
	OrganizationID            string
	ProjectID                 string
	RateProfile               string
	ConfiguredConcurrency     int
	ObservedMaxConcurrency    int
	BalanceThresholdCents     int64
	BalanceCents              int64
	BalanceCurrency           string
	RuntimeStatus             adminplusdomain.SupplierRuntimeStatus
	HealthStatus              adminplusdomain.SupplierHealthStatus
}

type UpdateSupplierStatusInput struct {
	RuntimeStatus adminplusdomain.SupplierRuntimeStatus
	HealthStatus  adminplusdomain.SupplierHealthStatus
}

type CreateFromSiteCandidateInput struct {
	Name                  string
	Type                  adminplusdomain.SupplierType
	DashboardURL          string
	APIBaseURL            string
	ThirdPartyRechargeURL string
	LocalRechargeURL      string
	SourceHost            string
	SourceURL             string
	Title                 string
}

type SupplierFilter struct {
	Kind          adminplusdomain.SupplierKind
	Type          adminplusdomain.SupplierType
	RuntimeStatus adminplusdomain.SupplierRuntimeStatus
	HealthStatus  adminplusdomain.SupplierHealthStatus
	Query         string
}

type SiteMatchInput struct {
	URL    string
	Origin string
	Host   string
}

type SiteMatchResult struct {
	Status    string                      `json:"status"`
	Suppliers []*adminplusdomain.Supplier `json:"suppliers"`
	Host      string                      `json:"host,omitempty"`
	Origin    string                      `json:"origin,omitempty"`
	Reason    string                      `json:"reason,omitempty"`
}

type EnsureFromSiteCandidateOptions struct {
	AllowCreate bool
}

type EnsureFromSiteCandidateResult struct {
	Supplier    *adminplusdomain.Supplier `json:"supplier"`
	Created     bool                      `json:"created"`
	Matched     bool                      `json:"matched"`
	MatchStatus string                    `json:"match_status,omitempty"`
}

type Repository interface {
	Create(ctx context.Context, supplier *adminplusdomain.Supplier) (*adminplusdomain.Supplier, error)
	Get(ctx context.Context, id int64) (*adminplusdomain.Supplier, error)
	GetBrowserCredential(ctx context.Context, id int64) (*adminplusdomain.SupplierBrowserCredential, error)
	List(ctx context.Context, filter SupplierFilter) ([]*adminplusdomain.Supplier, error)
	Update(ctx context.Context, supplier *adminplusdomain.Supplier) (*adminplusdomain.Supplier, error)
	UpdateStatus(ctx context.Context, id int64, runtimeStatus adminplusdomain.SupplierRuntimeStatus, healthStatus adminplusdomain.SupplierHealthStatus) (*adminplusdomain.Supplier, error)
	Delete(ctx context.Context, id int64) error
	ListAccounts(ctx context.Context, supplierID int64) ([]*adminplusdomain.SupplierAccount, error)
	CreateAccount(ctx context.Context, account *adminplusdomain.SupplierAccount) (*adminplusdomain.SupplierAccount, error)
	UpdateAccount(ctx context.Context, account *adminplusdomain.SupplierAccount) (*adminplusdomain.SupplierAccount, error)
	DeleteAccount(ctx context.Context, supplierID int64, accountID int64) error
	ListLocalAccounts(ctx context.Context, query string, limit int) ([]*adminplusdomain.LocalSub2APIAccount, error)
}

type Service struct {
	repo Repository
	now  func() time.Time
}

const legacyNewAPIQuotaUnitsPerUSD = 500000.0

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  time.Now,
	}
}

func (s *Service) Create(ctx context.Context, in CreateSupplierInput) (*adminplusdomain.Supplier, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	normalized, err := normalizeSupplierInput(supplierInput{
		Name:                  in.Name,
		Kind:                  in.Kind,
		Type:                  in.Type,
		RuntimeStatus:         in.RuntimeStatus,
		HealthStatus:          in.HealthStatus,
		DashboardURL:          in.DashboardURL,
		APIBaseURL:            in.APIBaseURL,
		ThirdPartyRechargeURL: in.ThirdPartyRechargeURL,
		LocalRechargeURL:      in.LocalRechargeURL,
		BalanceCents:          in.BalanceCents,
		BalanceCurrency:       in.BalanceCurrency,
		RechargeMultiplier:    in.RechargeMultiplier,
	})
	if err != nil {
		return nil, err
	}

	now := s.now().UTC()
	supplier := &adminplusdomain.Supplier{
		Name:                  normalized.Name,
		Kind:                  normalized.Kind,
		Type:                  normalized.Type,
		RuntimeStatus:         normalized.RuntimeStatus,
		HealthStatus:          normalized.HealthStatus,
		DashboardURL:          normalized.DashboardURL,
		APIBaseURL:            normalized.APIBaseURL,
		ThirdPartyRechargeURL: normalized.ThirdPartyRechargeURL,
		LocalRechargeURL:      normalized.LocalRechargeURL,
		Contact:               trimLimit(in.Contact, 120),
		Notes:                 trimLimit(in.Notes, 500),
		BrowserLoginUsername:  strings.TrimSpace(in.BrowserLoginUsername),
		BrowserLoginPassword:  in.BrowserLoginPassword,
		BrowserLoginToken:     in.BrowserLoginToken,
		Credential: adminplusdomain.SupplierCredentialStatus{
			PostgresConfigured:             strings.TrimSpace(in.PostgresReadDSN) != "",
			RedisConfigured:                strings.TrimSpace(in.RedisReadDSN) != "",
			BrowserLoginEnabled:            in.BrowserLoginEnabled,
			BrowserLoginUsernameConfigured: strings.TrimSpace(in.BrowserLoginUsername) != "",
			BrowserLoginPasswordConfigured: nonBlankSecret(in.BrowserLoginPassword),
			BrowserLoginTokenConfigured:    nonBlankSecret(in.BrowserLoginToken),
			MaskedBrowserLoginUsername:     maskUsername(in.BrowserLoginUsername),
		},
		BalanceCents:       normalized.BalanceCents,
		BalanceCurrency:    normalized.BalanceCurrency,
		RechargeMultiplier: normalized.RechargeMultiplier,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if normalized.BalanceCents > 0 {
		t := now
		supplier.BalanceUpdatedAt = &t
	}
	return s.repo.Create(ctx, supplier)
}

func (s *Service) CreateFromSiteCandidate(ctx context.Context, in CreateFromSiteCandidateInput) (*adminplusdomain.Supplier, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	dashboardURL, err := normalizeOptionalURL(firstNonEmpty(in.DashboardURL, in.SourceURL), "SUPPLIER_DASHBOARD_URL_INVALID")
	if err != nil {
		return nil, err
	}
	if dashboardURL == "" {
		return nil, badRequest("SUPPLIER_DASHBOARD_URL_REQUIRED", "supplier dashboard url is required")
	}
	parsed, err := url.Parse(dashboardURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, badRequest("SUPPLIER_DASHBOARD_URL_INVALID", "invalid supplier url")
	}
	if in.SourceHost != "" && !sameHost(parsed.Host, in.SourceHost) {
		return nil, badRequest("SUPPLIER_SITE_HOST_MISMATCH", "supplier candidate host does not match current site")
	}
	siteOrigin := parsed.Scheme + "://" + parsed.Host
	existing, err := s.List(ctx, SupplierFilter{})
	if err != nil {
		return nil, err
	}
	for _, supplier := range existing {
		if supplier == nil {
			continue
		}
		if supplierURLHostMatches(supplier.DashboardURL, parsed.Host) || supplierURLHostMatches(supplier.APIBaseURL, parsed.Host) {
			return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_SITE_ALREADY_EXISTS", "supplier for this site already exists")
		}
	}
	apiBaseURL, err := normalizeCandidateAPIBaseURL(in.APIBaseURL, parsed)
	if err != nil {
		return nil, err
	}
	supplierType := normalizeCandidateSupplierType(in.Type)
	name := strings.TrimSpace(firstNonEmpty(in.Name, in.Title, parsed.Host))
	if len(name) > 80 {
		name = name[:80]
	}
	return s.Create(ctx, CreateSupplierInput{
		Name:                  name,
		Kind:                  adminplusdomain.SupplierKindRelay,
		Type:                  supplierType,
		RuntimeStatus:         adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:          adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:          siteOrigin,
		APIBaseURL:            apiBaseURL,
		ThirdPartyRechargeURL: firstNonEmpty(in.ThirdPartyRechargeURL, inferThirdPartyRechargeURL(in.SourceURL), inferThirdPartyRechargeURL(in.DashboardURL)),
		LocalRechargeURL:      in.LocalRechargeURL,
		Notes:                 "created from Chrome extension site candidate",
		BrowserLoginEnabled:   true,
		BalanceCurrency:       defaultCurrencyForCandidateSupplier(supplierType),
	})
}

func (s *Service) EnsureFromSiteCandidate(ctx context.Context, in CreateFromSiteCandidateInput) (*EnsureFromSiteCandidateResult, error) {
	return s.EnsureFromSiteCandidateWithOptions(ctx, in, EnsureFromSiteCandidateOptions{})
}

func (s *Service) EnsureFromSiteCandidateWithOptions(ctx context.Context, in CreateFromSiteCandidateInput, options EnsureFromSiteCandidateOptions) (*EnsureFromSiteCandidateResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	match, err := s.MatchSite(ctx, SiteMatchInput{
		URL:    firstNonEmpty(in.SourceURL, in.DashboardURL),
		Origin: in.DashboardURL,
		Host:   in.SourceHost,
	})
	if err == nil && len(match.Suppliers) == 1 {
		supplier, enrichErr := s.enrichSupplierRechargeURLs(ctx, match.Suppliers[0], in)
		if enrichErr != nil {
			return nil, enrichErr
		}
		return &EnsureFromSiteCandidateResult{
			Supplier:    supplier,
			Matched:     true,
			MatchStatus: match.Status,
		}, nil
	}
	if err == nil && len(match.Suppliers) > 1 {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_SITE_AMBIGUOUS", "multiple suppliers match current site")
	}
	if err != nil && infraerrors.Code(err) != http.StatusBadRequest {
		return nil, err
	}
	if !options.AllowCreate {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_SITE_REGISTRATION_REQUIRED", "site candidate must be registered before importing as supplier")
	}
	created, err := s.CreateFromSiteCandidate(ctx, in)
	if err != nil {
		if infraerrors.Reason(err) != "SUPPLIER_SITE_ALREADY_EXISTS" {
			return nil, err
		}
		match, matchErr := s.MatchSite(ctx, SiteMatchInput{
			URL:    firstNonEmpty(in.SourceURL, in.DashboardURL),
			Origin: in.DashboardURL,
			Host:   in.SourceHost,
		})
		if matchErr != nil {
			return nil, matchErr
		}
		if len(match.Suppliers) == 1 {
			supplier, enrichErr := s.enrichSupplierRechargeURLs(ctx, match.Suppliers[0], in)
			if enrichErr != nil {
				return nil, enrichErr
			}
			return &EnsureFromSiteCandidateResult{
				Supplier:    supplier,
				Matched:     true,
				MatchStatus: match.Status,
			}, nil
		}
		return nil, err
	}
	return &EnsureFromSiteCandidateResult{
		Supplier: created,
		Created:  true,
	}, nil
}

func (s *Service) EnrichRechargeURLs(ctx context.Context, id int64, in CreateFromSiteCandidateInput) (*adminplusdomain.Supplier, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	supplier, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.enrichSupplierRechargeURLs(ctx, supplier, in)
}

func (s *Service) MatchSite(ctx context.Context, in SiteMatchInput) (*SiteMatchResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	host, origin, err := normalizeSiteMatchInput(in)
	if err != nil {
		return nil, err
	}
	suppliers, err := s.List(ctx, SupplierFilter{})
	if err != nil {
		return nil, err
	}
	matches := make([]*adminplusdomain.Supplier, 0, 1)
	for _, supplier := range suppliers {
		if supplier == nil {
			continue
		}
		if supplierURLExactHostMatches(supplier.DashboardURL, host) || supplierURLExactHostMatches(supplier.APIBaseURL, host) {
			matches = append(matches, supplier)
		}
	}
	if len(matches) == 0 {
		for _, supplier := range suppliers {
			if supplier == nil {
				continue
			}
			if supplierURLHostMatches(supplier.DashboardURL, host) || supplierURLHostMatches(supplier.APIBaseURL, host) {
				matches = append(matches, supplier)
			}
		}
	}
	status := "unknown"
	if len(matches) == 1 {
		status = "matched"
	}
	if len(matches) > 1 {
		status = "ambiguous"
	}
	return &SiteMatchResult{
		Status:    status,
		Suppliers: matches,
		Host:      host,
		Origin:    origin,
	}, nil
}

func (s *Service) Get(ctx context.Context, id int64) (*adminplusdomain.Supplier, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	supplier, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return normalizeSupplierForRead(supplier), nil
}

func (s *Service) GetBrowserCredential(ctx context.Context, id int64) (*adminplusdomain.SupplierBrowserCredential, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	credential, err := s.repo.GetBrowserCredential(ctx, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(credential.DashboardURL) == "" {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_DASHBOARD_URL_REQUIRED", "supplier dashboard url is required for browser automation")
	}
	if strings.TrimSpace(credential.Username) == "" && !nonBlankSecret(credential.Password) && !nonBlankSecret(credential.Token) {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_BROWSER_CREDENTIAL_REQUIRED", "supplier browser credential is required")
	}
	return credential, nil
}

func (s *Service) List(ctx context.Context, filter SupplierFilter) ([]*adminplusdomain.Supplier, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	if filter.Kind != "" && !filter.Kind.Valid() {
		return nil, badRequest("SUPPLIER_KIND_INVALID", "invalid supplier kind")
	}
	if filter.Type != "" && !filter.Type.Valid() {
		return nil, badRequest("SUPPLIER_TYPE_INVALID", "invalid supplier type")
	}
	if filter.RuntimeStatus != "" && !filter.RuntimeStatus.Valid() {
		return nil, badRequest("SUPPLIER_RUNTIME_STATUS_INVALID", "invalid supplier runtime status")
	}
	if filter.HealthStatus != "" && !filter.HealthStatus.Valid() {
		return nil, badRequest("SUPPLIER_HEALTH_STATUS_INVALID", "invalid supplier health status")
	}
	filter.Query = strings.ToLower(strings.TrimSpace(filter.Query))
	items, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	for index, supplier := range items {
		items[index] = normalizeSupplierForRead(supplier)
	}
	return items, nil
}

func (s *Service) Update(ctx context.Context, id int64, in UpdateSupplierInput) (*adminplusdomain.Supplier, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeSupplierInput(supplierInput{
		Name:                  in.Name,
		Kind:                  in.Kind,
		Type:                  in.Type,
		RuntimeStatus:         in.RuntimeStatus,
		HealthStatus:          in.HealthStatus,
		DashboardURL:          in.DashboardURL,
		APIBaseURL:            in.APIBaseURL,
		ThirdPartyRechargeURL: in.ThirdPartyRechargeURL,
		LocalRechargeURL:      in.LocalRechargeURL,
		BalanceCents:          in.BalanceCents,
		BalanceCurrency:       in.BalanceCurrency,
		RechargeMultiplier:    in.RechargeMultiplier,
	})
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	updated := *existing
	updated.Name = normalized.Name
	updated.Kind = normalized.Kind
	updated.Type = normalized.Type
	updated.RuntimeStatus = normalized.RuntimeStatus
	updated.HealthStatus = normalized.HealthStatus
	updated.DashboardURL = normalized.DashboardURL
	updated.APIBaseURL = normalized.APIBaseURL
	updated.ThirdPartyRechargeURL = normalized.ThirdPartyRechargeURL
	updated.LocalRechargeURL = normalized.LocalRechargeURL
	updated.Contact = trimLimit(in.Contact, 120)
	updated.Notes = trimLimit(in.Notes, 500)
	updated.BrowserLoginUsername = strings.TrimSpace(in.BrowserLoginUsername)
	updated.BrowserLoginPassword = in.BrowserLoginPassword
	updated.BrowserLoginToken = in.BrowserLoginToken
	updated.Credential.PostgresConfigured = strings.TrimSpace(in.PostgresReadDSN) != "" || existing.Credential.PostgresConfigured
	updated.Credential.RedisConfigured = strings.TrimSpace(in.RedisReadDSN) != "" || existing.Credential.RedisConfigured
	updated.Credential.BrowserLoginEnabled = in.BrowserLoginEnabled
	if updated.BrowserLoginUsername != "" {
		updated.Credential.BrowserLoginUsernameConfigured = true
		updated.Credential.MaskedBrowserLoginUsername = maskUsername(updated.BrowserLoginUsername)
	}
	if nonBlankSecret(updated.BrowserLoginPassword) {
		updated.Credential.BrowserLoginPasswordConfigured = true
	}
	if nonBlankSecret(updated.BrowserLoginToken) {
		updated.Credential.BrowserLoginTokenConfigured = true
	}
	if updated.Credential.MaskedBrowserLoginUsername == "" {
		updated.Credential.MaskedBrowserLoginUsername = existing.Credential.MaskedBrowserLoginUsername
	}
	if existing.BalanceCents != normalized.BalanceCents || existing.BalanceCurrency != normalized.BalanceCurrency || existing.BalanceUpdatedAt == nil {
		t := now
		updated.BalanceUpdatedAt = &t
	}
	updated.BalanceCents = normalized.BalanceCents
	updated.BalanceCurrency = normalized.BalanceCurrency
	updated.RechargeMultiplier = normalized.RechargeMultiplier
	updated.UpdatedAt = now
	return s.repo.Update(ctx, &updated)
}

func (s *Service) UpdateBalance(ctx context.Context, id int64, balanceCents int64, currency string) (*adminplusdomain.Supplier, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if balanceCents < 0 {
		return nil, badRequest("SUPPLIER_BALANCE_INVALID", "balance cannot be negative")
	}
	normalizedBalanceCents, normalizedCurrency := normalizeBalanceAmountAndCurrency(balanceCents, currency)
	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	updated := *existing
	updated.BalanceCents = normalizedBalanceCents
	updated.BalanceCurrency = normalizedCurrency
	now := s.now().UTC()
	updated.BalanceUpdatedAt = &now
	updated.UpdatedAt = now
	supplier, err := s.repo.Update(ctx, &updated)
	if err != nil {
		return nil, err
	}
	return normalizeSupplierForRead(supplier), nil
}

func (s *Service) UpdateStatus(ctx context.Context, id int64, in UpdateSupplierStatusInput) (*adminplusdomain.Supplier, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if !in.RuntimeStatus.Valid() {
		return nil, badRequest("SUPPLIER_RUNTIME_STATUS_INVALID", "invalid supplier runtime status")
	}
	if !in.HealthStatus.Valid() {
		return nil, badRequest("SUPPLIER_HEALTH_STATUS_INVALID", "invalid supplier health status")
	}
	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.RuntimeStatus == adminplusdomain.SupplierRuntimeStatusCandidate && existing.BalanceCents <= 0 {
		return nil, badRequest("SUPPLIER_BALANCE_REQUIRED_FOR_CANDIDATE", "candidate supplier must have positive balance")
	}
	if in.RuntimeStatus == adminplusdomain.SupplierRuntimeStatusActive && existing.BalanceCents <= 0 {
		return nil, badRequest("SUPPLIER_BALANCE_REQUIRED_FOR_ACTIVE", "active supplier must have positive balance")
	}
	supplier, err := s.repo.UpdateStatus(ctx, id, in.RuntimeStatus, in.HealthStatus)
	if err != nil {
		return nil, err
	}
	return normalizeSupplierForRead(supplier), nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if s == nil || s.repo == nil {
		return internalError("supplier service is not configured")
	}
	if id <= 0 {
		return badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	return s.repo.Delete(ctx, id)
}

func (s *Service) ListAccounts(ctx context.Context, supplierID int64) ([]*adminplusdomain.SupplierAccount, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	if supplierID <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if _, err := s.repo.Get(ctx, supplierID); err != nil {
		return nil, err
	}
	return s.repo.ListAccounts(ctx, supplierID)
}

func (s *Service) CreateAccount(ctx context.Context, in CreateSupplierAccountInput) (*adminplusdomain.SupplierAccount, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	if in.SupplierID <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	parent, err := s.repo.Get(ctx, in.SupplierID)
	if err != nil {
		return nil, err
	}
	if in.LocalSub2APIAccountID <= 0 {
		return nil, badRequest("LOCAL_ACCOUNT_ID_INVALID", "invalid local Sub2API account id")
	}
	runtimeStatus := in.RuntimeStatus
	if runtimeStatus == "" {
		runtimeStatus = adminplusdomain.SupplierRuntimeStatusMonitorOnly
	}
	if !runtimeStatus.Valid() {
		return nil, badRequest("SUPPLIER_ACCOUNT_RUNTIME_STATUS_INVALID", "invalid supplier account runtime status")
	}
	healthStatus := in.HealthStatus
	if healthStatus == "" {
		healthStatus = adminplusdomain.SupplierHealthStatusNormal
	}
	if !healthStatus.Valid() {
		return nil, badRequest("SUPPLIER_ACCOUNT_HEALTH_STATUS_INVALID", "invalid supplier account health status")
	}
	if adminplusdomain.IsSwitchableSupplierStatus(runtimeStatus) && in.BalanceCents <= 0 {
		return nil, badRequest("SUPPLIER_ACCOUNT_BALANCE_REQUIRED", "switchable supplier account must have positive balance")
	}
	if adminplusdomain.IsSwitchableSupplierStatus(runtimeStatus) && !adminplusdomain.IsSwitchableSupplierStatus(parent.RuntimeStatus) {
		return nil, badRequest("SUPPLIER_PARENT_NOT_SWITCHABLE", "parent supplier must be switchable before account can be switchable")
	}
	if in.ConfiguredConcurrency < 0 {
		return nil, badRequest("SUPPLIER_ACCOUNT_CONCURRENCY_INVALID", "configured concurrency cannot be negative")
	}
	if in.BalanceThresholdCents < 0 || in.BalanceCents < 0 {
		return nil, badRequest("SUPPLIER_ACCOUNT_BALANCE_INVALID", "balance values cannot be negative")
	}
	now := s.now().UTC()
	account := &adminplusdomain.SupplierAccount{
		SupplierID:                in.SupplierID,
		LocalSub2APIAccountID:     in.LocalSub2APIAccountID,
		SupplierAccountIdentifier: trimLimit(in.SupplierAccountIdentifier, 160),
		SupplierAccountLabel:      trimLimit(in.SupplierAccountLabel, 120),
		OrganizationID:            trimLimit(in.OrganizationID, 120),
		ProjectID:                 trimLimit(in.ProjectID, 120),
		RateProfile:               trimLimit(in.RateProfile, 120),
		ConfiguredConcurrency:     in.ConfiguredConcurrency,
		BalanceThresholdCents:     in.BalanceThresholdCents,
		BalanceCents:              in.BalanceCents,
		BalanceCurrency:           normalizeCurrency(in.BalanceCurrency),
		HasUsableBalance:          in.BalanceCents > 0,
		RuntimeStatus:             runtimeStatus,
		HealthStatus:              healthStatus,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	return s.repo.CreateAccount(ctx, account)
}

func (s *Service) UpdateAccount(ctx context.Context, in UpdateSupplierAccountInput) (*adminplusdomain.SupplierAccount, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	if in.SupplierID <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if in.AccountID <= 0 {
		return nil, badRequest("SUPPLIER_ACCOUNT_ID_INVALID", "invalid supplier account id")
	}
	parent, err := s.repo.Get(ctx, in.SupplierID)
	if err != nil {
		return nil, err
	}
	runtimeStatus := in.RuntimeStatus
	if runtimeStatus == "" {
		runtimeStatus = adminplusdomain.SupplierRuntimeStatusMonitorOnly
	}
	if !runtimeStatus.Valid() {
		return nil, badRequest("SUPPLIER_ACCOUNT_RUNTIME_STATUS_INVALID", "invalid supplier account runtime status")
	}
	healthStatus := in.HealthStatus
	if healthStatus == "" {
		healthStatus = adminplusdomain.SupplierHealthStatusNormal
	}
	if !healthStatus.Valid() {
		return nil, badRequest("SUPPLIER_ACCOUNT_HEALTH_STATUS_INVALID", "invalid supplier account health status")
	}
	if adminplusdomain.IsSwitchableSupplierStatus(runtimeStatus) && in.BalanceCents <= 0 {
		return nil, badRequest("SUPPLIER_ACCOUNT_BALANCE_REQUIRED", "switchable supplier account must have positive balance")
	}
	if adminplusdomain.IsSwitchableSupplierStatus(runtimeStatus) && !adminplusdomain.IsSwitchableSupplierStatus(parent.RuntimeStatus) {
		return nil, badRequest("SUPPLIER_PARENT_NOT_SWITCHABLE", "parent supplier must be switchable before account can be switchable")
	}
	if in.ConfiguredConcurrency < 0 || in.ObservedMaxConcurrency < 0 {
		return nil, badRequest("SUPPLIER_ACCOUNT_CONCURRENCY_INVALID", "concurrency values cannot be negative")
	}
	if in.BalanceThresholdCents < 0 || in.BalanceCents < 0 {
		return nil, badRequest("SUPPLIER_ACCOUNT_BALANCE_INVALID", "balance values cannot be negative")
	}
	now := s.now().UTC()
	account := &adminplusdomain.SupplierAccount{
		ID:                        in.AccountID,
		SupplierID:                in.SupplierID,
		SupplierAccountIdentifier: trimLimit(in.SupplierAccountIdentifier, 160),
		SupplierAccountLabel:      trimLimit(in.SupplierAccountLabel, 120),
		OrganizationID:            trimLimit(in.OrganizationID, 120),
		ProjectID:                 trimLimit(in.ProjectID, 120),
		RateProfile:               trimLimit(in.RateProfile, 120),
		ConfiguredConcurrency:     in.ConfiguredConcurrency,
		ObservedMaxConcurrency:    in.ObservedMaxConcurrency,
		BalanceThresholdCents:     in.BalanceThresholdCents,
		BalanceCents:              in.BalanceCents,
		BalanceCurrency:           normalizeCurrency(in.BalanceCurrency),
		HasUsableBalance:          in.BalanceCents > 0,
		RuntimeStatus:             runtimeStatus,
		HealthStatus:              healthStatus,
		UpdatedAt:                 now,
	}
	return s.repo.UpdateAccount(ctx, account)
}

func (s *Service) DeleteAccount(ctx context.Context, supplierID int64, accountID int64) error {
	if s == nil || s.repo == nil {
		return internalError("supplier service is not configured")
	}
	if supplierID <= 0 {
		return badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if accountID <= 0 {
		return badRequest("SUPPLIER_ACCOUNT_ID_INVALID", "invalid supplier account id")
	}
	return s.repo.DeleteAccount(ctx, supplierID, accountID)
}

type supplierInput struct {
	Name                  string
	Kind                  adminplusdomain.SupplierKind
	Type                  adminplusdomain.SupplierType
	RuntimeStatus         adminplusdomain.SupplierRuntimeStatus
	HealthStatus          adminplusdomain.SupplierHealthStatus
	DashboardURL          string
	APIBaseURL            string
	ThirdPartyRechargeURL string
	LocalRechargeURL      string
	BalanceCents          int64
	BalanceCurrency       string
	RechargeMultiplier    float64
}

func normalizeSupplierInput(in supplierInput) (supplierInput, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return supplierInput{}, badRequest("SUPPLIER_NAME_REQUIRED", "supplier name is required")
	}
	if len(name) > 80 {
		return supplierInput{}, badRequest("SUPPLIER_NAME_TOO_LONG", "supplier name must be 80 characters or less")
	}
	if !in.Kind.Valid() {
		return supplierInput{}, badRequest("SUPPLIER_KIND_INVALID", "invalid supplier kind")
	}
	if !in.Type.Valid() {
		return supplierInput{}, badRequest("SUPPLIER_TYPE_INVALID", "invalid supplier type")
	}
	runtimeStatus := in.RuntimeStatus
	if runtimeStatus == "" {
		runtimeStatus = adminplusdomain.SupplierRuntimeStatusMonitorOnly
	}
	if !runtimeStatus.Valid() {
		return supplierInput{}, badRequest("SUPPLIER_RUNTIME_STATUS_INVALID", "invalid supplier runtime status")
	}
	healthStatus := in.HealthStatus
	if healthStatus == "" {
		healthStatus = adminplusdomain.SupplierHealthStatusNormal
	}
	if !healthStatus.Valid() {
		return supplierInput{}, badRequest("SUPPLIER_HEALTH_STATUS_INVALID", "invalid supplier health status")
	}
	if in.BalanceCents < 0 {
		return supplierInput{}, badRequest("SUPPLIER_BALANCE_INVALID", "balance cannot be negative")
	}
	balanceCents, balanceCurrency := normalizeBalanceAmountAndCurrency(in.BalanceCents, in.BalanceCurrency)
	rechargeMultiplier, err := normalizeRechargeMultiplier(in.RechargeMultiplier)
	if err != nil {
		return supplierInput{}, err
	}
	if runtimeStatus == adminplusdomain.SupplierRuntimeStatusCandidate && balanceCents <= 0 {
		return supplierInput{}, badRequest("SUPPLIER_BALANCE_REQUIRED_FOR_CANDIDATE", "candidate supplier must have positive balance")
	}
	if runtimeStatus == adminplusdomain.SupplierRuntimeStatusActive && balanceCents <= 0 {
		return supplierInput{}, badRequest("SUPPLIER_BALANCE_REQUIRED_FOR_ACTIVE", "active supplier must have positive balance")
	}
	dashboardURL, err := normalizeOptionalURL(in.DashboardURL, "SUPPLIER_DASHBOARD_URL_INVALID")
	if err != nil {
		return supplierInput{}, err
	}
	apiBaseURL, err := normalizeOptionalURL(in.APIBaseURL, "SUPPLIER_API_BASE_URL_INVALID")
	if err != nil {
		return supplierInput{}, err
	}
	thirdPartyRechargeURL, err := normalizeOptionalURL(in.ThirdPartyRechargeURL, "SUPPLIER_THIRD_PARTY_RECHARGE_URL_INVALID")
	if err != nil {
		return supplierInput{}, err
	}
	localRechargeURL, err := normalizeOptionalURL(in.LocalRechargeURL, "SUPPLIER_LOCAL_RECHARGE_URL_INVALID")
	if err != nil {
		return supplierInput{}, err
	}
	return supplierInput{
		Name:                  name,
		Kind:                  in.Kind,
		Type:                  in.Type,
		RuntimeStatus:         runtimeStatus,
		HealthStatus:          healthStatus,
		DashboardURL:          dashboardURL,
		APIBaseURL:            apiBaseURL,
		ThirdPartyRechargeURL: thirdPartyRechargeURL,
		LocalRechargeURL:      localRechargeURL,
		BalanceCents:          balanceCents,
		BalanceCurrency:       balanceCurrency,
		RechargeMultiplier:    rechargeMultiplier,
	}, nil
}

func (s *Service) ListLocalAccounts(ctx context.Context, query string, limit int) ([]*adminplusdomain.LocalSub2APIAccount, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repo.ListLocalAccounts(ctx, strings.TrimSpace(query), limit)
}

func normalizeOptionalURL(raw string, reason string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", nil
	}
	u, err := url.ParseRequestURI(v)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", badRequest(reason, "invalid supplier url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", badRequest(reason, "supplier url must use http or https")
	}
	return v, nil
}

func normalizeCandidateSupplierType(value adminplusdomain.SupplierType) adminplusdomain.SupplierType {
	normalized := strings.ToLower(strings.TrimSpace(string(value)))
	switch normalized {
	case "", "sub2api":
		return adminplusdomain.SupplierTypeSub2API
	case "newapi", "new-api", "new_api":
		return adminplusdomain.SupplierTypeNewAPI
	default:
		return adminplusdomain.SupplierType(normalized)
	}
}

func defaultCurrencyForCandidateSupplier(supplierType adminplusdomain.SupplierType) string {
	return "USD"
}

func normalizeCandidateAPIBaseURL(raw string, fallback *url.URL) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" && fallback != nil {
		v = fallback.Scheme + "://" + fallback.Host
	}
	return normalizeOptionalURL(v, "SUPPLIER_API_BASE_URL_INVALID")
}

func (s *Service) enrichSupplierRechargeURLs(ctx context.Context, supplier *adminplusdomain.Supplier, in CreateFromSiteCandidateInput) (*adminplusdomain.Supplier, error) {
	if supplier == nil {
		return supplier, nil
	}
	thirdPartyRechargeURL, err := normalizeOptionalURL(firstNonEmpty(in.ThirdPartyRechargeURL, inferThirdPartyRechargeURL(in.SourceURL), inferThirdPartyRechargeURL(in.DashboardURL)), "SUPPLIER_THIRD_PARTY_RECHARGE_URL_INVALID")
	if err != nil {
		return nil, err
	}
	localRechargeURL, err := normalizeOptionalURL(in.LocalRechargeURL, "SUPPLIER_LOCAL_RECHARGE_URL_INVALID")
	if err != nil {
		return nil, err
	}
	if (thirdPartyRechargeURL == "" || supplier.ThirdPartyRechargeURL != "") && (localRechargeURL == "" || supplier.LocalRechargeURL != "") {
		return supplier, nil
	}
	updated := *supplier
	if updated.ThirdPartyRechargeURL == "" {
		updated.ThirdPartyRechargeURL = thirdPartyRechargeURL
	}
	if updated.LocalRechargeURL == "" {
		updated.LocalRechargeURL = localRechargeURL
	}
	updated.UpdatedAt = s.now().UTC()
	return s.repo.Update(ctx, &updated)
}

func inferThirdPartyRechargeURL(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	u, err := url.Parse(v)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	path := strings.ToLower(u.EscapedPath())
	for _, marker := range []string{"/custom/", "/recharge", "/payment", "/topup", "/redeem", "/card", "/pay"} {
		if strings.Contains(path, marker) {
			return v
		}
	}
	return ""
}

func normalizeSiteMatchInput(in SiteMatchInput) (string, string, error) {
	raw := firstNonEmpty(in.URL, in.Origin)
	if raw != "" {
		parsed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "", "", badRequest("SUPPLIER_SITE_URL_INVALID", "invalid site url")
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", "", badRequest("SUPPLIER_SITE_URL_INVALID", "site url must use http or https")
		}
		origin := parsed.Scheme + "://" + parsed.Host
		return parsed.Host, origin, nil
	}
	host := strings.TrimSpace(in.Host)
	if host == "" {
		return "", "", badRequest("SUPPLIER_SITE_REQUIRED", "site url or host is required")
	}
	return host, "", nil
}

func supplierURLHostMatches(raw string, host string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return false
	}
	return sameHost(u.Host, host)
}

func supplierURLExactHostMatches(raw string, host string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(u.Host), strings.TrimSpace(host))
}

func sameHost(left string, right string) bool {
	return strings.EqualFold(hostWithoutPort(left), hostWithoutPort(right))
}

func hostWithoutPort(value string) string {
	host := strings.TrimSpace(value)
	if parsed, err := url.Parse("https://" + host); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	return host
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nonBlankSecret(value string) bool {
	return strings.TrimSpace(value) != ""
}

func trimLimit(value string, limit int) string {
	v := strings.TrimSpace(value)
	if len(v) <= limit {
		return v
	}
	return v[:limit]
}

func normalizeCurrency(value string) string {
	v := strings.ToUpper(strings.TrimSpace(value))
	if v == "" || v == "QTA" || v == "CNY" {
		return "USD"
	}
	if len(v) != 3 {
		return "USD"
	}
	return v
}

func normalizeBalanceAmountAndCurrency(cents int64, currency string) (int64, string) {
	v := strings.ToUpper(strings.TrimSpace(currency))
	if v == "QTA" {
		return int64(math.Round(float64(cents) / legacyNewAPIQuotaUnitsPerUSD)), "USD"
	}
	return cents, normalizeCurrency(v)
}

func normalizeRechargeMultiplier(value float64) (float64, error) {
	if value == 0 {
		return 1, nil
	}
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, badRequest("SUPPLIER_RECHARGE_MULTIPLIER_INVALID", "recharge multiplier must be positive")
	}
	return value, nil
}

func normalizeSupplierForRead(in *adminplusdomain.Supplier) *adminplusdomain.Supplier {
	if in == nil {
		return nil
	}
	out := *in
	out.BalanceCents, out.BalanceCurrency = normalizeBalanceAmountAndCurrency(in.BalanceCents, in.BalanceCurrency)
	if multiplier, err := normalizeRechargeMultiplier(in.RechargeMultiplier); err == nil {
		out.RechargeMultiplier = multiplier
	} else {
		out.RechargeMultiplier = 1
	}
	if in.BalanceUpdatedAt != nil {
		t := *in.BalanceUpdatedAt
		out.BalanceUpdatedAt = &t
	}
	return &out
}

func maskUsername(value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return ""
	}
	if strings.Contains(v, "@") {
		parts := strings.SplitN(v, "@", 2)
		name := parts[0]
		if len(name) <= 2 {
			return name[:1] + "***@" + parts[1]
		}
		return name[:2] + "***@" + parts[1]
	}
	if len(v) <= 4 {
		return v[:1] + "***"
	}
	return v[:2] + "***" + v[len(v)-2:]
}

func badRequest(reason string, message string) error {
	return infraerrors.New(http.StatusBadRequest, reason, message)
}

func internalError(message string) error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", message)
}
