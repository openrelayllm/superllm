package suppliers

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type CreateSupplierInput struct {
	Name                 string
	Kind                 adminplusdomain.SupplierKind
	Type                 adminplusdomain.SupplierType
	RuntimeStatus        adminplusdomain.SupplierRuntimeStatus
	HealthStatus         adminplusdomain.SupplierHealthStatus
	DashboardURL         string
	APIBaseURL           string
	Contact              string
	Notes                string
	PostgresReadDSN      string
	RedisReadDSN         string
	BrowserLoginEnabled  bool
	BrowserLoginUsername string
	BrowserLoginPassword string
	BrowserLoginToken    string
	BalanceCents         int64
	BalanceCurrency      string
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

type UpdateSupplierStatusInput struct {
	RuntimeStatus adminplusdomain.SupplierRuntimeStatus
	HealthStatus  adminplusdomain.SupplierHealthStatus
}

type SupplierFilter struct {
	Kind          adminplusdomain.SupplierKind
	Type          adminplusdomain.SupplierType
	RuntimeStatus adminplusdomain.SupplierRuntimeStatus
	HealthStatus  adminplusdomain.SupplierHealthStatus
	Query         string
}

type Repository interface {
	Create(ctx context.Context, supplier *adminplusdomain.Supplier) (*adminplusdomain.Supplier, error)
	Get(ctx context.Context, id int64) (*adminplusdomain.Supplier, error)
	List(ctx context.Context, filter SupplierFilter) ([]*adminplusdomain.Supplier, error)
	UpdateStatus(ctx context.Context, id int64, runtimeStatus adminplusdomain.SupplierRuntimeStatus, healthStatus adminplusdomain.SupplierHealthStatus) (*adminplusdomain.Supplier, error)
	ListAccounts(ctx context.Context, supplierID int64) ([]*adminplusdomain.SupplierAccount, error)
	CreateAccount(ctx context.Context, account *adminplusdomain.SupplierAccount) (*adminplusdomain.SupplierAccount, error)
	DeleteAccount(ctx context.Context, supplierID int64, accountID int64) error
	ListLocalAccounts(ctx context.Context, query string, limit int) ([]*adminplusdomain.LocalSub2APIAccount, error)
}

type Service struct {
	repo Repository
	now  func() time.Time
}

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
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, badRequest("SUPPLIER_NAME_REQUIRED", "supplier name is required")
	}
	if len(name) > 80 {
		return nil, badRequest("SUPPLIER_NAME_TOO_LONG", "supplier name must be 80 characters or less")
	}
	if !in.Kind.Valid() {
		return nil, badRequest("SUPPLIER_KIND_INVALID", "invalid supplier kind")
	}
	if !in.Type.Valid() {
		return nil, badRequest("SUPPLIER_TYPE_INVALID", "invalid supplier type")
	}
	runtimeStatus := in.RuntimeStatus
	if runtimeStatus == "" {
		runtimeStatus = adminplusdomain.SupplierRuntimeStatusMonitorOnly
	}
	if !runtimeStatus.Valid() {
		return nil, badRequest("SUPPLIER_RUNTIME_STATUS_INVALID", "invalid supplier runtime status")
	}
	healthStatus := in.HealthStatus
	if healthStatus == "" {
		healthStatus = adminplusdomain.SupplierHealthStatusNormal
	}
	if !healthStatus.Valid() {
		return nil, badRequest("SUPPLIER_HEALTH_STATUS_INVALID", "invalid supplier health status")
	}
	if runtimeStatus == adminplusdomain.SupplierRuntimeStatusCandidate && in.BalanceCents <= 0 {
		return nil, badRequest("SUPPLIER_BALANCE_REQUIRED_FOR_CANDIDATE", "candidate supplier must have positive balance")
	}
	if runtimeStatus == adminplusdomain.SupplierRuntimeStatusActive && in.BalanceCents <= 0 {
		return nil, badRequest("SUPPLIER_BALANCE_REQUIRED_FOR_ACTIVE", "active supplier must have positive balance")
	}
	dashboardURL, err := normalizeOptionalURL(in.DashboardURL, "SUPPLIER_DASHBOARD_URL_INVALID")
	if err != nil {
		return nil, err
	}
	apiBaseURL, err := normalizeOptionalURL(in.APIBaseURL, "SUPPLIER_API_BASE_URL_INVALID")
	if err != nil {
		return nil, err
	}

	now := s.now().UTC()
	supplier := &adminplusdomain.Supplier{
		Name:                 name,
		Kind:                 in.Kind,
		Type:                 in.Type,
		RuntimeStatus:        runtimeStatus,
		HealthStatus:         healthStatus,
		DashboardURL:         dashboardURL,
		APIBaseURL:           apiBaseURL,
		Contact:              trimLimit(in.Contact, 120),
		Notes:                trimLimit(in.Notes, 500),
		BrowserLoginUsername: strings.TrimSpace(in.BrowserLoginUsername),
		BrowserLoginPassword: strings.TrimSpace(in.BrowserLoginPassword),
		BrowserLoginToken:    strings.TrimSpace(in.BrowserLoginToken),
		Credential: adminplusdomain.SupplierCredentialStatus{
			PostgresConfigured:             strings.TrimSpace(in.PostgresReadDSN) != "",
			RedisConfigured:                strings.TrimSpace(in.RedisReadDSN) != "",
			BrowserLoginEnabled:            in.BrowserLoginEnabled,
			BrowserLoginUsernameConfigured: strings.TrimSpace(in.BrowserLoginUsername) != "",
			BrowserLoginPasswordConfigured: strings.TrimSpace(in.BrowserLoginPassword) != "",
			BrowserLoginTokenConfigured:    strings.TrimSpace(in.BrowserLoginToken) != "",
			MaskedBrowserLoginUsername:     maskUsername(in.BrowserLoginUsername),
		},
		BalanceCents:    in.BalanceCents,
		BalanceCurrency: normalizeCurrency(in.BalanceCurrency),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if in.BalanceCents > 0 {
		t := now
		supplier.BalanceUpdatedAt = &t
	}
	return s.repo.Create(ctx, supplier)
}

func (s *Service) Get(ctx context.Context, id int64) (*adminplusdomain.Supplier, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	return s.repo.Get(ctx, id)
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
	return s.repo.List(ctx, filter)
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
	return s.repo.UpdateStatus(ctx, id, in.RuntimeStatus, in.HealthStatus)
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

func trimLimit(value string, limit int) string {
	v := strings.TrimSpace(value)
	if len(v) <= limit {
		return v
	}
	return v[:limit]
}

func normalizeCurrency(value string) string {
	v := strings.ToUpper(strings.TrimSpace(value))
	if v == "" {
		return "USD"
	}
	if len(v) != 3 {
		return "USD"
	}
	return v
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
