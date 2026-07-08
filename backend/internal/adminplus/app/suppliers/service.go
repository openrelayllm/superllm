package suppliers

import (
	"context"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
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
	KeyLimitPolicy        string
	KeyLimitValue         int
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
	KeyLimitPolicy        string
	KeyLimitValue         int
}

type UpdateSupplierStatusInput struct {
	RuntimeStatus adminplusdomain.SupplierRuntimeStatus
	HealthStatus  adminplusdomain.SupplierHealthStatus
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
		KeyLimitPolicy:        in.KeyLimitPolicy,
		KeyLimitValue:         in.KeyLimitValue,
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
		Credential:            supplierCredentialStatusFromCreateInput(in),
		BalanceCents:          normalized.BalanceCents,
		BalanceCurrency:       normalized.BalanceCurrency,
		RechargeMultiplier:    normalized.RechargeMultiplier,
		KeyLimitPolicy:        normalized.KeyLimitPolicy,
		KeyLimitValue:         normalized.KeyLimitValue,
		KeyCapacityStatus:     supplierKeyCapacityStatus(normalized.KeyLimitPolicy, normalized.KeyLimitValue, 0),
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if normalized.BalanceCents > 0 {
		t := now
		supplier.BalanceUpdatedAt = &t
	}
	created, err := s.repo.Create(ctx, supplier)
	if err != nil {
		return nil, err
	}
	return normalizeSupplierForRead(created), nil
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

func (s *Service) List(ctx context.Context, filter SupplierFilter) ([]*adminplusdomain.Supplier, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	filter, err := normalizeSupplierFilter(filter)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]*adminplusdomain.Supplier, 0, len(items))
	for _, supplier := range items {
		normalized := normalizeSupplierForRead(supplier)
		if !supplierMatchesCapabilityStatus(normalized, filter.CapabilityStatus) {
			continue
		}
		if !supplierMatchesIntegrationProtocol(normalized, filter.IntegrationProtocol) {
			continue
		}
		if !supplierMatchesPlatformHint(normalized, filter.PlatformHint) {
			continue
		}
		if !supplierMatchesPlatformFamily(normalized, filter.PlatformFamily) {
			continue
		}
		out = append(out, normalized)
	}
	return out, nil
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
		KeyLimitPolicy:        in.KeyLimitPolicy,
		KeyLimitValue:         in.KeyLimitValue,
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
	applySupplierCredentialUpdate(&updated, existing, in)
	if existing.BalanceCents != normalized.BalanceCents || existing.BalanceCurrency != normalized.BalanceCurrency || existing.BalanceUpdatedAt == nil {
		t := now
		updated.BalanceUpdatedAt = &t
	}
	updated.BalanceCents = normalized.BalanceCents
	updated.BalanceCurrency = normalized.BalanceCurrency
	updated.RechargeMultiplier = normalized.RechargeMultiplier
	updated.KeyLimitPolicy = normalized.KeyLimitPolicy
	updated.KeyLimitValue = normalized.KeyLimitValue
	updated.KeyCapacityStatus = supplierKeyCapacityStatus(updated.KeyLimitPolicy, updated.KeyLimitValue, updated.ActiveKeyCount)
	updated.UpdatedAt = now
	supplier, err := s.repo.Update(ctx, &updated)
	if err != nil {
		return nil, err
	}
	return normalizeSupplierForRead(supplier), nil
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
