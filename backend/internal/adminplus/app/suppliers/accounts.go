package suppliers

import (
	"context"
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

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

func (s *Service) ListLocalAccounts(ctx context.Context, query string, limit int) ([]*adminplusdomain.LocalSub2APIAccount, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repo.ListLocalAccounts(ctx, strings.TrimSpace(query), limit)
}
