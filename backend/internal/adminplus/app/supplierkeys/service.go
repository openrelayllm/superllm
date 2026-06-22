package supplierkeys

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type ProvisionKeyInput struct {
	SupplierID                 int64
	SupplierGroupID            int64
	Name                       string
	QuotaUSD                   float64
	ExpiresInDays              *int
	LocalAccountPlatform       string
	LocalAccountName           string
	LocalAccountBaseURL        string
	LocalAccountConcurrency    int
	LocalAccountPriority       int
	LocalAccountRateMultiplier *float64
	LocalAccountGroupIDs       []int64
	RuntimeStatus              adminplusdomain.SupplierRuntimeStatus
	HealthStatus               adminplusdomain.SupplierHealthStatus
	BalanceThresholdCents      int64
	BalanceCents               int64
	BalanceCurrency            string
}

type EnsureAllInput struct {
	SupplierID              int64
	LocalAccountBaseURL     string
	LocalAccountConcurrency int
	LocalAccountPriority    int
	RuntimeStatus           adminplusdomain.SupplierRuntimeStatus
	HealthStatus            adminplusdomain.SupplierHealthStatus
	BalanceThresholdCents   int64
	BalanceCents            int64
	BalanceCurrency         string
}

type EnsureGroupInput struct {
	EnsureAllInput
	SupplierGroupID int64
}

type ProvisionGroupPlan struct {
	SupplierGroupID int64  `json:"supplier_group_id"`
	ExternalGroupID string `json:"external_group_id"`
	GroupName       string `json:"group_name"`
	ProviderFamily  string `json:"provider_family"`
}

type RepairBindingInput struct {
	SupplierID                int64
	KeyID                     int64
	LocalSub2APIAccountID     int64
	RuntimeStatus             adminplusdomain.SupplierRuntimeStatus
	HealthStatus              adminplusdomain.SupplierHealthStatus
	ConfiguredConcurrency     int
	BalanceThresholdCents     int64
	BalanceCents              int64
	BalanceCurrency           string
	SupplierAccountIdentifier string
	SupplierAccountLabel      string
}

type ProvisionKeyResult struct {
	Key     *adminplusdomain.SupplierKey     `json:"key"`
	Binding *adminplusdomain.SupplierAccount `json:"binding"`
}

type EnsureAllResult struct {
	SupplierID         int64                 `json:"supplier_id"`
	Total              int                   `json:"total"`
	Created            int                   `json:"created"`
	Skipped            int                   `json:"skipped"`
	Failed             int                   `json:"failed"`
	LocalGroupsCreated int                   `json:"local_groups_created"`
	LocalAccountsBound int                   `json:"local_accounts_bound"`
	Items              []EnsureAllResultItem `json:"items"`
}

type EnsureAllResultItem struct {
	SupplierGroupID        int64                            `json:"supplier_group_id"`
	ExternalGroupID        string                           `json:"external_group_id"`
	GroupName              string                           `json:"group_name"`
	Action                 string                           `json:"action"`
	Key                    *adminplusdomain.SupplierKey     `json:"key,omitempty"`
	Binding                *adminplusdomain.SupplierAccount `json:"binding,omitempty"`
	LocalSub2APIGroupID    int64                            `json:"local_sub2api_group_id,omitempty"`
	LocalSub2APIGroupName  string                           `json:"local_sub2api_group_name,omitempty"`
	LocalGroupCreated      bool                             `json:"local_group_created,omitempty"`
	LocalAccountGroupBound bool                             `json:"local_account_group_bound,omitempty"`
	ErrorCode              string                           `json:"error_code,omitempty"`
	ErrorMessage           string                           `json:"error_message,omitempty"`
}

type ListFilter struct {
	SupplierID int64
	Status     adminplusdomain.SupplierKeyStatus
	Query      string
	Limit      int
}

type Repository interface {
	GetSupplier(ctx context.Context, supplierID int64) (*adminplusdomain.Supplier, error)
	GetGroup(ctx context.Context, supplierID int64, groupID int64) (*adminplusdomain.SupplierGroup, error)
	GetKey(ctx context.Context, supplierID int64, keyID int64) (*adminplusdomain.SupplierKey, error)
	ListGroups(ctx context.Context, supplierID int64) ([]*adminplusdomain.SupplierGroup, error)
	FindActiveByGroup(ctx context.Context, supplierID int64, groupID int64) (*adminplusdomain.SupplierKey, error)
	CreateKey(ctx context.Context, key *adminplusdomain.SupplierKey) (*adminplusdomain.SupplierKey, error)
	UpdateKeyAfterLocalBind(ctx context.Context, keyID int64, localAccount *service.Account, status adminplusdomain.SupplierKeyStatus, errorCode string, errorMessage string) (*adminplusdomain.SupplierKey, error)
	CreateBinding(ctx context.Context, account *adminplusdomain.SupplierAccount) (*adminplusdomain.SupplierAccount, error)
	UpsertBinding(ctx context.Context, account *adminplusdomain.SupplierAccount) (*adminplusdomain.SupplierAccount, error)
	List(ctx context.Context, filter ListFilter) ([]*adminplusdomain.SupplierKey, error)
}

type SessionReader interface {
	DecryptedProbeInput(ctx context.Context, supplierID int64) (ports.SessionProbeInput, error)
}

type Sub2APIGateway interface {
	CreateAccount(ctx context.Context, input *service.CreateAccountInput) (*service.Account, error)
	GetAccount(ctx context.Context, id int64) (*service.Account, error)
	UpdateAccount(ctx context.Context, id int64, input *service.UpdateAccountInput) (*service.Account, error)
}

type LocalAccountService = Sub2APIGateway

type Sub2APIAccountFinder interface {
	FindAccount(ctx context.Context, input Sub2APIAccountLookupInput) (*service.Account, error)
}

type LocalAccountFinder = Sub2APIAccountFinder

type Sub2APIAccountLookupInput struct {
	SupplierID             int64
	SupplierGroupID        int64
	SupplierKeyID          int64
	ExternalGroupID        string
	LocalAccountName       string
	LocalAccountPlatform   string
	SupplierKeyExternalID  string
	SupplierKeyFingerprint string
	SupplierKeyLast4       string
}

type LocalAccountLookupInput = Sub2APIAccountLookupInput

type Service struct {
	repo           Repository
	session        SessionReader
	keyAdapter     ports.SessionKeyAdapter
	sub2apiGateway Sub2APIGateway
	legacyAccounts LegacyAccountService
	now            func() time.Time
}

type LegacyAccountService interface {
	GetAccount(ctx context.Context, id int64) (*service.Account, error)
}

type localAccountEnsureInput struct {
	Supplier       *adminplusdomain.Supplier
	Group          *adminplusdomain.SupplierGroup
	Key            *adminplusdomain.SupplierKey
	Secret         string
	BaseURL        string
	Platform       string
	Name           string
	Concurrency    int
	Priority       int
	RateMultiplier *float64
	GroupIDs       []int64
}

func NewService(repo Repository, session SessionReader, keyAdapter ports.SessionKeyAdapter, gateway Sub2APIGateway) *Service {
	return NewServiceWithLegacy(repo, session, keyAdapter, gateway, nil)
}

func NewServiceWithLegacy(repo Repository, session SessionReader, keyAdapter ports.SessionKeyAdapter, gateway Sub2APIGateway, legacyAccounts LegacyAccountService) *Service {
	return &Service{
		repo:           repo,
		session:        session,
		keyAdapter:     keyAdapter,
		sub2apiGateway: gateway,
		legacyAccounts: legacyAccounts,
		now:            time.Now,
	}
}

func (s *Service) Provision(ctx context.Context, in ProvisionKeyInput) (*ProvisionKeyResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier key service is not configured")
	}
	if s.session == nil {
		return nil, internalError("supplier browser session service is not configured")
	}
	if s.keyAdapter == nil {
		return nil, internalError("supplier key provider adapter is not configured")
	}
	if s.sub2apiGateway == nil {
		return nil, internalError("local Sub2API account service is not configured")
	}
	normalized, err := s.normalizeProvisionInput(in)
	if err != nil {
		return nil, err
	}
	supplier, err := s.repo.GetSupplier(ctx, normalized.SupplierID)
	if err != nil {
		return nil, err
	}
	if !supplierSupportsKeyProvisioning(supplier.Type) {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_PROVIDER_UNSUPPORTED", "only Sub2API or New API supplier key provisioning is supported")
	}
	group, err := s.repo.GetGroup(ctx, normalized.SupplierID, normalized.SupplierGroupID)
	if err != nil {
		return nil, err
	}
	if group.Status != adminplusdomain.SupplierGroupStatusActive {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_GROUP_NOT_ACTIVE", "supplier group is not active")
	}
	existing, err := s.repo.FindActiveByGroup(ctx, normalized.SupplierID, group.ID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_GROUP_KEY_ALREADY_BOUND", "supplier group already has a bound or provisioning key")
	}
	if err := s.ensureLocalAccountGatewayReady(ctx); err != nil {
		return nil, err
	}
	input, err := s.session.DecryptedProbeInput(ctx, normalized.SupplierID)
	if err != nil {
		return nil, err
	}
	created, err := s.keyAdapter.CreateKey(ctx, input, ports.CreateProviderKeyInput{
		SupplierID:      normalized.SupplierID,
		ExternalGroupID: group.ExternalGroupID,
		Name:            normalized.Name,
		QuotaUSD:        normalized.QuotaUSD,
		ExpiresInDays:   normalized.ExpiresInDays,
		Metadata: map[string]any{
			"supplier_group_id": group.ID,
			"provider_family":   group.ProviderFamily,
		},
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(created.Secret) == "" {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_SECRET_REQUIRED", "supplier did not return created key secret")
	}
	now := s.now().UTC()
	key := &adminplusdomain.SupplierKey{
		SupplierID:      normalized.SupplierID,
		SupplierGroupID: group.ID,
		ExternalGroupID: firstNonEmpty(created.ExternalGroupID, group.ExternalGroupID),
		ExternalKeyID:   trimLimit(created.ExternalKeyID, 160),
		Name:            trimLimit(firstNonEmpty(created.Name, normalized.Name), 160),
		KeyFingerprint:  fingerprintSecret(created.Secret),
		KeyLast4:        lastN(created.Secret, 4),
		Status:          adminplusdomain.SupplierKeyStatusProvisioning,
		ProviderFamily:  normalizeProviderFamily(group.ProviderFamily),
		ProvisionRequest: map[string]any{
			"name":              normalized.Name,
			"external_group_id": group.ExternalGroupID,
			"quota_usd":         normalized.QuotaUSD,
			"expires_in_days":   normalized.ExpiresInDays,
		},
		ProvisionResponse: cloneMap(created.RawPayload),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	savedKey, err := s.repo.CreateKey(ctx, key)
	if err != nil {
		return nil, err
	}

	localAccount, _, _, updatedKey, localErr := s.ensureLocalAccountForKey(ctx, localAccountEnsureInput{
		Supplier:       supplier,
		Group:          group,
		Key:            savedKey,
		Secret:         created.Secret,
		BaseURL:        normalized.LocalAccountBaseURL,
		Platform:       normalized.LocalAccountPlatform,
		Name:           normalized.LocalAccountName,
		Concurrency:    normalized.LocalAccountConcurrency,
		Priority:       normalized.LocalAccountPriority,
		RateMultiplier: normalized.LocalAccountRateMultiplier,
		GroupIDs:       normalized.LocalAccountGroupIDs,
	})
	if localErr != nil {
		_, _ = s.repo.UpdateKeyAfterLocalBind(ctx, savedKey.ID, nil, adminplusdomain.SupplierKeyStatusFailed, "LOCAL_ACCOUNT_CREATE_FAILED", localErr.Error())
		return nil, infraerrors.New(http.StatusBadGateway, "LOCAL_SUB2API_ACCOUNT_CREATE_FAILED", "failed to create local Sub2API account").WithCause(localErr)
	}
	if updatedKey != nil {
		savedKey = updatedKey
	}

	binding := s.supplierAccountForLocalAccount(normalized, savedKey, group, localAccount)
	binding.CreatedAt = now
	binding.UpdatedAt = now
	savedBinding, err := s.repo.UpsertBinding(ctx, binding)
	if err != nil {
		_, _ = s.repo.UpdateKeyAfterLocalBind(ctx, savedKey.ID, localAccount, adminplusdomain.SupplierKeyStatusFailed, "SUPPLIER_ACCOUNT_BIND_FAILED", err.Error())
		return nil, err
	}
	return &ProvisionKeyResult{Key: savedKey, Binding: savedBinding}, nil
}

func (s *Service) EnsureAll(ctx context.Context, in EnsureAllInput) (*EnsureAllResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier key service is not configured")
	}
	if s.sub2apiGateway == nil {
		return nil, internalError("local Sub2API account service is not configured")
	}
	normalized, supplier, err := s.normalizeEnsureAllInput(ctx, in)
	if err != nil {
		return nil, err
	}
	groups, err := s.repo.ListGroups(ctx, normalized.SupplierID)
	if err != nil {
		return nil, err
	}
	result := &EnsureAllResult{
		SupplierID: normalized.SupplierID,
		Items:      make([]EnsureAllResultItem, 0, len(groups)),
	}
	for _, group := range groups {
		if group == nil || group.Status != adminplusdomain.SupplierGroupStatusActive {
			continue
		}
		result.Total++
		item, err := s.ensureGroup(ctx, normalized, supplier, group)
		if err != nil {
			result.Failed++
			result.Items = append(result.Items, *item)
			continue
		}
		switch item.Action {
		case "created":
			result.Created++
			if item.LocalAccountGroupBound {
				result.LocalAccountsBound++
			}
		case "skipped":
			result.Skipped++
			if item.LocalAccountGroupBound {
				result.LocalAccountsBound++
			}
		}
		if item.LocalGroupCreated {
			result.LocalGroupsCreated++
		}
		result.Items = append(result.Items, *item)
	}
	return result, nil
}

func (s *Service) PlanEnsureAll(ctx context.Context, in EnsureAllInput) ([]ProvisionGroupPlan, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier key service is not configured")
	}
	normalized, _, err := s.normalizeEnsureAllInput(ctx, in)
	if err != nil {
		return nil, err
	}
	groups, err := s.repo.ListGroups(ctx, normalized.SupplierID)
	if err != nil {
		return nil, err
	}
	plans := make([]ProvisionGroupPlan, 0, len(groups))
	for _, group := range groups {
		if group == nil || group.Status != adminplusdomain.SupplierGroupStatusActive {
			continue
		}
		plans = append(plans, ProvisionGroupPlan{
			SupplierGroupID: group.ID,
			ExternalGroupID: group.ExternalGroupID,
			GroupName:       group.Name,
			ProviderFamily:  group.ProviderFamily,
		})
	}
	return plans, nil
}

func (s *Service) EnsureGroup(ctx context.Context, in EnsureGroupInput) (*EnsureAllResultItem, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier key service is not configured")
	}
	if s.sub2apiGateway == nil {
		return nil, internalError("local Sub2API account service is not configured")
	}
	if in.SupplierGroupID <= 0 {
		return nil, badRequest("SUPPLIER_GROUP_ID_INVALID", "invalid supplier group id")
	}
	normalized, supplier, err := s.normalizeEnsureAllInput(ctx, in.EnsureAllInput)
	if err != nil {
		return nil, err
	}
	group, err := s.repo.GetGroup(ctx, normalized.SupplierID, in.SupplierGroupID)
	if err != nil {
		return nil, err
	}
	return s.ensureGroup(ctx, normalized, supplier, group)
}

func (s *Service) ensureGroup(ctx context.Context, normalized EnsureAllInput, supplier *adminplusdomain.Supplier, group *adminplusdomain.SupplierGroup) (*EnsureAllResultItem, error) {
	item := &EnsureAllResultItem{}
	if group != nil {
		item.SupplierGroupID = group.ID
		item.ExternalGroupID = group.ExternalGroupID
		item.GroupName = group.Name
	}
	fail := func(err error) (*EnsureAllResultItem, error) {
		item.Action = "failed"
		item.ErrorCode = infraerrors.Reason(err)
		item.ErrorMessage = infraerrors.Message(err)
		return item, err
	}
	if group == nil || group.Status != adminplusdomain.SupplierGroupStatusActive {
		item.Action = "skipped"
		return item, nil
	}
	existing, err := s.repo.FindActiveByGroup(ctx, normalized.SupplierID, group.ID)
	if err != nil {
		return fail(err)
	}
	if existing != nil {
		localAccountInput := localAccountInputForKey(normalized, supplier, group, existing, nil)
		ensuredAccount, accountCreated, bound, updated, err := s.ensureLocalAccountForKey(ctx, localAccountInput)
		if err != nil {
			item.Key = existing
			return fail(err)
		}
		if accountCreated || bound {
			item.LocalAccountGroupBound = true
		}
		if updated != nil {
			existing = updated
		}
		if ensuredAccount != nil {
			binding := s.supplierAccountForLocalAccount(provisionInputFromEnsureAll(normalized, group), existing, group, ensuredAccount)
			savedBinding, bindErr := s.repo.UpsertBinding(ctx, binding)
			if bindErr != nil {
				item.Key = existing
				return fail(bindErr)
			}
			item.Binding = savedBinding
		}
		item.Action = "skipped"
		item.Key = existing
		return item, nil
	}
	rate := group.EffectiveRateMultiplier
	if rate <= 0 {
		rate = group.RateMultiplier
	}
	input := ProvisionKeyInput{
		SupplierID:                 normalized.SupplierID,
		SupplierGroupID:            group.ID,
		Name:                       defaultProvisionName(group),
		QuotaUSD:                   0,
		LocalAccountPlatform:       normalizeLocalPlatform(group.ProviderFamily),
		LocalAccountName:           defaultLocalAccountName(supplier, group),
		LocalAccountBaseURL:        normalized.LocalAccountBaseURL,
		LocalAccountConcurrency:    normalized.LocalAccountConcurrency,
		LocalAccountPriority:       normalized.LocalAccountPriority,
		LocalAccountRateMultiplier: &rate,
		RuntimeStatus:              normalized.RuntimeStatus,
		HealthStatus:               normalized.HealthStatus,
		BalanceThresholdCents:      normalized.BalanceThresholdCents,
		BalanceCents:               normalized.BalanceCents,
		BalanceCurrency:            normalized.BalanceCurrency,
	}
	created, err := s.Provision(ctx, input)
	if err != nil {
		return fail(err)
	}
	item.Action = "created"
	item.Key = created.Key
	item.Binding = created.Binding
	return item, nil
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]*adminplusdomain.SupplierKey, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier key service is not configured")
	}
	if filter.SupplierID <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, badRequest("SUPPLIER_KEY_STATUS_INVALID", "invalid supplier key status")
	}
	filter.Query = normalizeQuery(filter.Query)
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.List(ctx, filter)
}

func (s *Service) RepairBinding(ctx context.Context, in RepairBindingInput) (*ProvisionKeyResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier key service is not configured")
	}
	if s.sub2apiGateway == nil {
		return nil, internalError("local Sub2API account service is not configured")
	}
	normalized, err := s.normalizeRepairBindingInput(in)
	if err != nil {
		return nil, err
	}
	key, err := s.repo.GetKey(ctx, normalized.SupplierID, normalized.KeyID)
	if err != nil {
		return nil, err
	}
	if key.Status != adminplusdomain.SupplierKeyStatusFailed {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_REPAIR_NOT_ALLOWED", "only failed supplier key can be repaired")
	}
	if !repairableKeyError(key.ErrorCode) {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_REPAIR_NOT_ALLOWED", "supplier key failure is not a local binding failure")
	}
	existing, err := s.repo.FindActiveByGroup(ctx, normalized.SupplierID, key.SupplierGroupID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.ID != key.ID {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_GROUP_KEY_ALREADY_BOUND", "supplier group already has a bound or provisioning key")
	}
	localAccount, err := s.sub2apiGateway.GetAccount(ctx, normalized.LocalSub2APIAccountID)
	if err != nil {
		return nil, err
	}
	group, err := s.repo.GetGroup(ctx, normalized.SupplierID, key.SupplierGroupID)
	if err != nil {
		return nil, err
	}
	if _, _, err := s.ensureLocalAccountStateForGroups(ctx, localAccount, nil, "", key, group); err != nil {
		return nil, err
	}
	now := s.now().UTC()
	provisionInput := ProvisionKeyInput{
		SupplierID:              normalized.SupplierID,
		LocalAccountConcurrency: normalized.ConfiguredConcurrency,
		RuntimeStatus:           normalized.RuntimeStatus,
		HealthStatus:            normalized.HealthStatus,
		BalanceThresholdCents:   normalized.BalanceThresholdCents,
		BalanceCents:            normalized.BalanceCents,
		BalanceCurrency:         normalized.BalanceCurrency,
	}
	binding := s.supplierAccountForLocalAccount(provisionInput, key, group, localAccount)
	binding.SupplierAccountIdentifier = firstNonEmpty(normalized.SupplierAccountIdentifier, binding.SupplierAccountIdentifier)
	binding.SupplierAccountLabel = firstNonEmpty(normalized.SupplierAccountLabel, binding.SupplierAccountLabel)
	binding.CreatedAt = now
	binding.UpdatedAt = now
	savedBinding, err := s.repo.CreateBinding(ctx, binding)
	if err != nil {
		return nil, err
	}
	savedKey, err := s.repo.UpdateKeyAfterLocalBind(ctx, key.ID, localAccount, adminplusdomain.SupplierKeyStatusBound, "", "")
	if err != nil {
		return nil, err
	}
	return &ProvisionKeyResult{Key: savedKey, Binding: savedBinding}, nil
}

func (s *Service) normalizeProvisionInput(in ProvisionKeyInput) (ProvisionKeyInput, error) {
	if in.SupplierID <= 0 {
		return ProvisionKeyInput{}, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if in.SupplierGroupID <= 0 {
		return ProvisionKeyInput{}, badRequest("SUPPLIER_GROUP_ID_INVALID", "invalid supplier group id")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = "admin-plus-key-" + time.Now().UTC().Format("20060102150405")
	}
	if len(name) > 120 {
		name = name[:120]
	}
	if in.QuotaUSD < 0 {
		return ProvisionKeyInput{}, badRequest("SUPPLIER_KEY_QUOTA_INVALID", "quota cannot be negative")
	}
	if in.ExpiresInDays != nil && *in.ExpiresInDays < 0 {
		return ProvisionKeyInput{}, badRequest("SUPPLIER_KEY_EXPIRES_INVALID", "expires_in_days cannot be negative")
	}
	platform := strings.ToLower(strings.TrimSpace(in.LocalAccountPlatform))
	if platform == "" {
		platform = service.PlatformOpenAI
	}
	if !validLocalPlatform(platform) {
		return ProvisionKeyInput{}, badRequest("LOCAL_ACCOUNT_PLATFORM_INVALID", "invalid local account platform")
	}
	baseURL := strings.TrimSpace(in.LocalAccountBaseURL)
	if baseURL == "" {
		return ProvisionKeyInput{}, badRequest("LOCAL_ACCOUNT_BASE_URL_REQUIRED", "local account base url is required")
	}
	concurrency := in.LocalAccountConcurrency
	if concurrency < 0 {
		return ProvisionKeyInput{}, badRequest("LOCAL_ACCOUNT_CONCURRENCY_INVALID", "local account concurrency cannot be negative")
	}
	runtimeStatus := in.RuntimeStatus
	if runtimeStatus == "" {
		runtimeStatus = adminplusdomain.SupplierRuntimeStatusMonitorOnly
	}
	if !runtimeStatus.Valid() {
		return ProvisionKeyInput{}, badRequest("SUPPLIER_ACCOUNT_RUNTIME_STATUS_INVALID", "invalid supplier account runtime status")
	}
	healthStatus := in.HealthStatus
	if healthStatus == "" {
		healthStatus = adminplusdomain.SupplierHealthStatusNormal
	}
	if !healthStatus.Valid() {
		return ProvisionKeyInput{}, badRequest("SUPPLIER_ACCOUNT_HEALTH_STATUS_INVALID", "invalid supplier account health status")
	}
	if adminplusdomain.IsSwitchableSupplierStatus(runtimeStatus) && in.BalanceCents <= 0 {
		return ProvisionKeyInput{}, badRequest("SUPPLIER_ACCOUNT_BALANCE_REQUIRED", "switchable supplier account must have positive balance")
	}
	if in.BalanceThresholdCents < 0 || in.BalanceCents < 0 {
		return ProvisionKeyInput{}, badRequest("SUPPLIER_ACCOUNT_BALANCE_INVALID", "balance values cannot be negative")
	}
	in.Name = name
	in.LocalAccountPlatform = platform
	if strings.TrimSpace(in.LocalAccountName) == "" {
		in.LocalAccountName = name
	} else {
		in.LocalAccountName = trimLimit(in.LocalAccountName, 160)
	}
	in.LocalAccountBaseURL = baseURL
	in.LocalAccountConcurrency = concurrency
	in.RuntimeStatus = runtimeStatus
	in.HealthStatus = healthStatus
	in.BalanceCurrency = normalizeCurrency(in.BalanceCurrency)
	return in, nil
}

func (s *Service) normalizeEnsureAllInput(ctx context.Context, in EnsureAllInput) (EnsureAllInput, *adminplusdomain.Supplier, error) {
	if in.SupplierID <= 0 {
		return EnsureAllInput{}, nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	supplier, err := s.repo.GetSupplier(ctx, in.SupplierID)
	if err != nil {
		return EnsureAllInput{}, nil, err
	}
	if !supplierSupportsKeyProvisioning(supplier.Type) {
		return EnsureAllInput{}, nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_PROVIDER_UNSUPPORTED", "only Sub2API or New API supplier key provisioning is supported")
	}
	baseURL := strings.TrimSpace(in.LocalAccountBaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL(supplier)
	}
	if baseURL == "" {
		return EnsureAllInput{}, nil, badRequest("LOCAL_ACCOUNT_BASE_URL_REQUIRED", "local account base url is required")
	}
	if in.LocalAccountConcurrency < 0 {
		return EnsureAllInput{}, nil, badRequest("LOCAL_ACCOUNT_CONCURRENCY_INVALID", "local account concurrency cannot be negative")
	}
	runtimeStatus := in.RuntimeStatus
	if runtimeStatus == "" {
		runtimeStatus = adminplusdomain.SupplierRuntimeStatusMonitorOnly
	}
	if !runtimeStatus.Valid() {
		return EnsureAllInput{}, nil, badRequest("SUPPLIER_ACCOUNT_RUNTIME_STATUS_INVALID", "invalid supplier account runtime status")
	}
	healthStatus := in.HealthStatus
	if healthStatus == "" {
		healthStatus = adminplusdomain.SupplierHealthStatusNormal
	}
	if !healthStatus.Valid() {
		return EnsureAllInput{}, nil, badRequest("SUPPLIER_ACCOUNT_HEALTH_STATUS_INVALID", "invalid supplier account health status")
	}
	if adminplusdomain.IsSwitchableSupplierStatus(runtimeStatus) && in.BalanceCents <= 0 {
		return EnsureAllInput{}, nil, badRequest("SUPPLIER_ACCOUNT_BALANCE_REQUIRED", "switchable supplier account must have positive balance")
	}
	if in.BalanceThresholdCents < 0 || in.BalanceCents < 0 {
		return EnsureAllInput{}, nil, badRequest("SUPPLIER_ACCOUNT_BALANCE_INVALID", "balance values cannot be negative")
	}
	in.LocalAccountBaseURL = baseURL
	in.RuntimeStatus = runtimeStatus
	in.HealthStatus = healthStatus
	in.BalanceCurrency = normalizeCurrency(in.BalanceCurrency)
	return in, supplier, nil
}

func (s *Service) normalizeRepairBindingInput(in RepairBindingInput) (RepairBindingInput, error) {
	if in.SupplierID <= 0 {
		return RepairBindingInput{}, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if in.KeyID <= 0 {
		return RepairBindingInput{}, badRequest("SUPPLIER_KEY_ID_INVALID", "invalid supplier key id")
	}
	if in.LocalSub2APIAccountID <= 0 {
		return RepairBindingInput{}, badRequest("LOCAL_ACCOUNT_ID_INVALID", "invalid local Sub2API account id")
	}
	if in.ConfiguredConcurrency < 0 {
		return RepairBindingInput{}, badRequest("SUPPLIER_ACCOUNT_CONCURRENCY_INVALID", "configured concurrency cannot be negative")
	}
	runtimeStatus := in.RuntimeStatus
	if runtimeStatus == "" {
		runtimeStatus = adminplusdomain.SupplierRuntimeStatusMonitorOnly
	}
	if !runtimeStatus.Valid() {
		return RepairBindingInput{}, badRequest("SUPPLIER_ACCOUNT_RUNTIME_STATUS_INVALID", "invalid supplier account runtime status")
	}
	healthStatus := in.HealthStatus
	if healthStatus == "" {
		healthStatus = adminplusdomain.SupplierHealthStatusNormal
	}
	if !healthStatus.Valid() {
		return RepairBindingInput{}, badRequest("SUPPLIER_ACCOUNT_HEALTH_STATUS_INVALID", "invalid supplier account health status")
	}
	if adminplusdomain.IsSwitchableSupplierStatus(runtimeStatus) && in.BalanceCents <= 0 {
		return RepairBindingInput{}, badRequest("SUPPLIER_ACCOUNT_BALANCE_REQUIRED", "switchable supplier account must have positive balance")
	}
	if in.BalanceThresholdCents < 0 || in.BalanceCents < 0 {
		return RepairBindingInput{}, badRequest("SUPPLIER_ACCOUNT_BALANCE_INVALID", "balance values cannot be negative")
	}
	in.RuntimeStatus = runtimeStatus
	in.HealthStatus = healthStatus
	in.BalanceCurrency = normalizeCurrency(in.BalanceCurrency)
	in.SupplierAccountIdentifier = trimLimit(in.SupplierAccountIdentifier, 160)
	in.SupplierAccountLabel = trimLimit(in.SupplierAccountLabel, 160)
	return in, nil
}

func repairableKeyError(errorCode string) bool {
	switch strings.TrimSpace(errorCode) {
	case "LOCAL_ACCOUNT_CREATE_FAILED", "SUPPLIER_ACCOUNT_BIND_FAILED":
		return true
	default:
		return false
	}
}

func supplierSupportsKeyProvisioning(supplierType adminplusdomain.SupplierType) bool {
	switch supplierType {
	case adminplusdomain.SupplierTypeSub2API, adminplusdomain.SupplierTypeNewAPI:
		return true
	default:
		return false
	}
}

func validLocalPlatform(platform string) bool {
	switch platform {
	case service.PlatformOpenAI, service.PlatformAnthropic, service.PlatformGemini, service.PlatformAntigravity:
		return true
	default:
		return false
	}
}

func (s *Service) ensureLocalAccountGatewayReady(ctx context.Context) error {
	finder, ok := s.sub2apiGateway.(Sub2APIAccountFinder)
	if !ok {
		return nil
	}
	_, err := finder.FindAccount(ctx, Sub2APIAccountLookupInput{
		LocalAccountName:     "__admin_plus_gateway_preflight__",
		LocalAccountPlatform: service.PlatformOpenAI,
	})
	if err != nil && !isLocalAccountNotFound(err) {
		return localGatewayError("LOCAL_SUB2API_ACCOUNT_LOOKUP_FAILED", "failed to lookup local Sub2API account", err)
	}
	return nil
}

func (s *Service) ensureLocalAccountStateForGroups(ctx context.Context, localAccount *service.Account, localGroupIDs []int64, baseURL string, key *adminplusdomain.SupplierKey, group *adminplusdomain.SupplierGroup) (bool, *service.Account, error) {
	if localAccount == nil || localAccount.ID <= 0 {
		return false, localAccount, nil
	}
	groupIDs := mergeInt64IDs(localAccount.GroupIDs, localGroupIDs...)
	groupChanged := len(localGroupIDs) > 0 && len(groupIDs) != len(localAccount.GroupIDs)
	credentialsPatch := missingLocalAccountCredentialDefaults(localAccount, baseURL)
	extraPatch := missingLocalAccountExtraDefaults(localAccount, key, group)
	if !groupChanged && len(credentialsPatch) == 0 && len(extraPatch) == 0 {
		return false, localAccount, nil
	}
	input := &service.UpdateAccountInput{
		SkipMixedChannelCheck: true,
	}
	if groupChanged {
		input.GroupIDs = &groupIDs
	}
	if len(credentialsPatch) > 0 {
		input.Credentials = credentialsPatch
	}
	if len(extraPatch) > 0 {
		input.Extra = mergeAnyMap(localAccount.Extra, extraPatch)
	}
	updated, err := s.sub2apiGateway.UpdateAccount(ctx, localAccount.ID, input)
	if err != nil {
		return false, nil, localGatewayError("LOCAL_SUB2API_ACCOUNT_STATE_SYNC_FAILED", "failed to sync local Sub2API account state", err)
	}
	return groupChanged, updated, nil
}

func localAccountInputForKey(in EnsureAllInput, supplier *adminplusdomain.Supplier, group *adminplusdomain.SupplierGroup, key *adminplusdomain.SupplierKey, groupIDs []int64) localAccountEnsureInput {
	rate := group.EffectiveRateMultiplier
	if rate <= 0 {
		rate = group.RateMultiplier
	}
	platform := normalizeLocalPlatform(group.ProviderFamily)
	name := defaultLocalAccountName(supplier, group)
	if key != nil {
		platform = firstNonEmpty(key.LocalAccountPlatform, platform)
		name = firstNonEmpty(key.LocalAccountName, name)
	}
	return localAccountEnsureInput{
		Supplier:       supplier,
		Group:          group,
		Key:            key,
		BaseURL:        in.LocalAccountBaseURL,
		Platform:       platform,
		Name:           name,
		Concurrency:    in.LocalAccountConcurrency,
		Priority:       in.LocalAccountPriority,
		RateMultiplier: &rate,
		GroupIDs:       groupIDs,
	}
}

func (s *Service) ensureLocalAccountForKey(ctx context.Context, in localAccountEnsureInput) (*service.Account, bool, bool, *adminplusdomain.SupplierKey, error) {
	if s == nil || s.sub2apiGateway == nil {
		return nil, false, false, in.Key, internalError("local Sub2API account service is not configured")
	}
	if in.Group == nil || in.Key == nil {
		return nil, false, false, in.Key, badRequest("SUPPLIER_GROUP_ID_INVALID", "invalid supplier group id")
	}
	platform := strings.ToLower(strings.TrimSpace(in.Platform))
	if platform == "" {
		platform = normalizeLocalPlatform(in.Group.ProviderFamily)
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = defaultLocalAccountName(in.Supplier, in.Group)
	}
	groupIDs := mergeInt64IDs(nil, in.GroupIDs...)

	account, err := s.findLocalAccountForKey(ctx, in, name, platform)
	if err != nil {
		return nil, false, false, in.Key, err
	}
	created := false
	if account == nil {
		schedulable := false
		secret := strings.TrimSpace(in.Secret)
		if secret == "" {
			secret = s.recoverLegacyLocalAccountSecret(ctx, in.Key.LocalSub2APIAccountID)
		}
		if secret == "" {
			return nil, false, false, in.Key, infraerrors.New(http.StatusConflict, "LOCAL_SUB2API_ACCOUNT_SECRET_UNAVAILABLE", "existing supplier key has no recoverable secret for creating the real Sub2API account")
		}
		createdAccount, createErr := s.sub2apiGateway.CreateAccount(ctx, &service.CreateAccountInput{
			Name:                  name,
			Platform:              platform,
			Type:                  service.AccountTypeAPIKey,
			Credentials:           defaultLocalAccountCredentials(platform, in.BaseURL, secret),
			Extra:                 defaultLocalAccountExtra(platform, in.SupplierID(), in.Group, in.Key.ID),
			Concurrency:           in.Concurrency,
			Priority:              in.Priority,
			RateMultiplier:        in.RateMultiplier,
			Schedulable:           &schedulable,
			GroupIDs:              groupIDs,
			SkipDefaultGroupBind:  true,
			SkipMixedChannelCheck: true,
		})
		if createErr != nil {
			return nil, false, false, in.Key, createErr
		}
		account = createdAccount
		created = true
	}
	bound, syncedAccount, err := s.ensureLocalAccountStateForGroups(ctx, account, groupIDs, in.BaseURL, in.Key, in.Group)
	if err != nil {
		return account, created, bound, in.Key, err
	}
	if syncedAccount != nil {
		account = syncedAccount
	}
	updatedKey, err := s.repo.UpdateKeyAfterLocalBind(ctx, in.Key.ID, account, adminplusdomain.SupplierKeyStatusBound, "", "")
	if err != nil {
		return account, created, bound, in.Key, err
	}
	return account, created, bound, updatedKey, nil
}

func (in localAccountEnsureInput) SupplierID() int64 {
	if in.Supplier != nil {
		return in.Supplier.ID
	}
	if in.Group != nil {
		return in.Group.SupplierID
	}
	if in.Key != nil {
		return in.Key.SupplierID
	}
	return 0
}

func (s *Service) findLocalAccountForKey(ctx context.Context, in localAccountEnsureInput, name string, platform string) (*service.Account, error) {
	lookup := Sub2APIAccountLookupInput{
		SupplierID:             in.SupplierID(),
		SupplierGroupID:        in.Group.ID,
		SupplierKeyID:          in.Key.ID,
		ExternalGroupID:        firstNonEmpty(in.Key.ExternalGroupID, in.Group.ExternalGroupID),
		LocalAccountName:       name,
		LocalAccountPlatform:   platform,
		SupplierKeyExternalID:  in.Key.ExternalKeyID,
		SupplierKeyFingerprint: in.Key.KeyFingerprint,
		SupplierKeyLast4:       in.Key.KeyLast4,
	}
	if in.Key.LocalSub2APIAccountID > 0 {
		account, err := s.sub2apiGateway.GetAccount(ctx, in.Key.LocalSub2APIAccountID)
		if err == nil && localAccountMatchesLookup(account, lookup) {
			return account, nil
		}
		if err != nil && !isLocalAccountNotFound(err) {
			return nil, infraerrors.New(http.StatusBadGateway, "LOCAL_SUB2API_ACCOUNT_GET_FAILED", "failed to get local Sub2API account").WithCause(err)
		}
	}
	finder, ok := s.sub2apiGateway.(Sub2APIAccountFinder)
	if !ok {
		return nil, nil
	}
	account, err := finder.FindAccount(ctx, lookup)
	if err != nil {
		if isLocalAccountNotFound(err) {
			return nil, nil
		}
		return nil, infraerrors.New(http.StatusBadGateway, "LOCAL_SUB2API_ACCOUNT_LOOKUP_FAILED", "failed to lookup local Sub2API account").WithCause(err)
	}
	if account != nil && !localAccountMatchesLookup(account, lookup) {
		return nil, nil
	}
	return account, nil
}

func (s *Service) recoverLegacyLocalAccountSecret(ctx context.Context, localAccountID int64) string {
	if s == nil || s.legacyAccounts == nil || localAccountID <= 0 {
		return ""
	}
	account, err := s.legacyAccounts.GetAccount(ctx, localAccountID)
	if err != nil || account == nil {
		return ""
	}
	return strings.TrimSpace(stringFromMap(account.Credentials, "api_key"))
}

func provisionInputFromEnsureAll(in EnsureAllInput, group *adminplusdomain.SupplierGroup) ProvisionKeyInput {
	rate := 1.0
	if group != nil {
		rate = group.EffectiveRateMultiplier
		if rate <= 0 {
			rate = group.RateMultiplier
		}
		if rate <= 0 {
			rate = 1
		}
	}
	return ProvisionKeyInput{
		SupplierID:                 in.SupplierID,
		LocalAccountConcurrency:    in.LocalAccountConcurrency,
		LocalAccountPriority:       in.LocalAccountPriority,
		LocalAccountRateMultiplier: &rate,
		RuntimeStatus:              in.RuntimeStatus,
		HealthStatus:               in.HealthStatus,
		BalanceThresholdCents:      in.BalanceThresholdCents,
		BalanceCents:               in.BalanceCents,
		BalanceCurrency:            in.BalanceCurrency,
	}
}

func (s *Service) supplierAccountForLocalAccount(in ProvisionKeyInput, key *adminplusdomain.SupplierKey, group *adminplusdomain.SupplierGroup, localAccount *service.Account) *adminplusdomain.SupplierAccount {
	now := s.now().UTC()
	return &adminplusdomain.SupplierAccount{
		SupplierID:                in.SupplierID,
		SupplierKeyID:             key.ID,
		LocalSub2APIAccountID:     localAccount.ID,
		LocalAccountName:          localAccount.Name,
		LocalAccountPlatform:      localAccount.Platform,
		LocalAccountType:          localAccount.Type,
		SupplierAccountIdentifier: key.ExternalKeyID,
		SupplierAccountLabel:      key.Name,
		RateProfile:               group.ProviderFamily,
		ConfiguredConcurrency:     in.LocalAccountConcurrency,
		BalanceThresholdCents:     in.BalanceThresholdCents,
		BalanceCents:              in.BalanceCents,
		BalanceCurrency:           in.BalanceCurrency,
		HasUsableBalance:          in.BalanceCents > 0,
		RuntimeStatus:             in.RuntimeStatus,
		HealthStatus:              in.HealthStatus,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
}

func mergeInt64IDs(existing []int64, ids ...int64) []int64 {
	out := make([]int64, 0, len(existing)+len(ids))
	seen := make(map[int64]struct{}, len(existing)+len(ids))
	for _, id := range existing {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func defaultLocalAccountCredentials(platform string, baseURL string, secret string) map[string]any {
	credentials := map[string]any{
		"pool_mode": true,
	}
	if strings.TrimSpace(secret) != "" {
		credentials["api_key"] = strings.TrimSpace(secret)
	}
	if normalized := strings.TrimRight(strings.TrimSpace(baseURL), "/"); normalized != "" {
		credentials["base_url"] = normalized
	}
	return credentials
}

func defaultLocalAccountExtra(platform string, supplierID int64, group *adminplusdomain.SupplierGroup, keyID int64) map[string]any {
	extra := map[string]any{
		"admin_plus_supplier_id":  supplierID,
		"admin_plus_supplier_key": keyID,
	}
	if group != nil {
		extra["admin_plus_supplier_group_id"] = group.ID
		if strings.TrimSpace(group.ExternalGroupID) != "" {
			extra["admin_plus_external_group_id"] = strings.TrimSpace(group.ExternalGroupID)
		}
		if strings.TrimSpace(group.ProviderFamily) != "" {
			extra["admin_plus_provider_family"] = normalizeProviderFamily(group.ProviderFamily)
		}
	}
	if strings.TrimSpace(platform) != "" {
		extra["admin_plus_local_platform"] = strings.TrimSpace(platform)
	}
	return extra
}

func missingLocalAccountCredentialDefaults(account *service.Account, baseURL string) map[string]any {
	if account == nil {
		return nil
	}
	patch := map[string]any{}
	if account.GetCredential("base_url") == "" {
		if normalized := strings.TrimRight(strings.TrimSpace(baseURL), "/"); normalized != "" {
			patch["base_url"] = normalized
		}
	}
	if len(patch) == 0 {
		return nil
	}
	return patch
}

func missingLocalAccountExtraDefaults(account *service.Account, key *adminplusdomain.SupplierKey, group *adminplusdomain.SupplierGroup) map[string]any {
	if account == nil {
		return nil
	}
	patch := map[string]any{}
	if key != nil {
		if account.GetExtraString("admin_plus_supplier_id") == "" && key.SupplierID > 0 {
			patch["admin_plus_supplier_id"] = key.SupplierID
		}
		if account.GetExtraString("admin_plus_supplier_key") == "" && key.ID > 0 {
			patch["admin_plus_supplier_key"] = key.ID
		}
		if account.GetExtraString("admin_plus_local_platform") == "" && strings.TrimSpace(key.LocalAccountPlatform) != "" {
			patch["admin_plus_local_platform"] = strings.TrimSpace(key.LocalAccountPlatform)
		}
	}
	if group != nil {
		if account.GetExtraString("admin_plus_supplier_group_id") == "" && group.ID > 0 {
			patch["admin_plus_supplier_group_id"] = group.ID
		}
		if account.GetExtraString("admin_plus_external_group_id") == "" && strings.TrimSpace(group.ExternalGroupID) != "" {
			patch["admin_plus_external_group_id"] = strings.TrimSpace(group.ExternalGroupID)
		}
		if account.GetExtraString("admin_plus_provider_family") == "" && strings.TrimSpace(group.ProviderFamily) != "" {
			patch["admin_plus_provider_family"] = normalizeProviderFamily(group.ProviderFamily)
		}
	}
	if len(patch) == 0 {
		return nil
	}
	return patch
}

func mergeAnyMap(existing map[string]any, patch map[string]any) map[string]any {
	if len(existing) == 0 && len(patch) == 0 {
		return nil
	}
	out := make(map[string]any, len(existing)+len(patch))
	for key, value := range existing {
		out[key] = value
	}
	for key, value := range patch {
		out[key] = value
	}
	return out
}

func localAccountMatchesLookup(account *service.Account, lookup Sub2APIAccountLookupInput) bool {
	if account == nil {
		return false
	}
	if strings.TrimSpace(lookup.LocalAccountName) != "" && strings.EqualFold(strings.TrimSpace(account.Name), strings.TrimSpace(lookup.LocalAccountName)) {
		return true
	}
	if int64FromMap(account.Extra, "admin_plus_supplier_id") == lookup.SupplierID && lookup.SupplierID > 0 {
		if lookup.SupplierKeyID > 0 && int64FromMap(account.Extra, "admin_plus_supplier_key") == lookup.SupplierKeyID {
			return true
		}
		if lookup.SupplierGroupID > 0 && int64FromMap(account.Extra, "admin_plus_supplier_group_id") == lookup.SupplierGroupID {
			return true
		}
		if strings.TrimSpace(lookup.ExternalGroupID) != "" && strings.EqualFold(stringFromMap(account.Extra, "admin_plus_external_group_id"), lookup.ExternalGroupID) {
			return true
		}
	}
	return false
}

func isLocalAccountNotFound(err error) bool {
	if err == nil {
		return false
	}
	reason := strings.ToUpper(strings.TrimSpace(infraerrors.Reason(err)))
	if strings.Contains(reason, "NOT_FOUND") {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "not found") || strings.Contains(message, "404")
}

func stringFromMap(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	switch v := values[key].(type) {
	case string:
		return strings.TrimSpace(v)
	case []byte:
		return strings.TrimSpace(string(v))
	default:
		return ""
	}
}

func int64FromMap(values map[string]any, key string) int64 {
	if len(values) == 0 {
		return 0
	}
	switch v := values[key].(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return n
	default:
		return 0
	}
}

func fingerprintSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func lastN(value string, n int) string {
	value = strings.TrimSpace(value)
	if len(value) <= n {
		return value
	}
	return value[len(value)-n:]
}

func normalizeProviderFamily(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "mixed"
	}
	return trimLimit(value, 60)
}

func normalizeLocalPlatform(providerFamily string) string {
	value := strings.ToLower(strings.TrimSpace(providerFamily))
	if strings.Contains(value, "anthropic") || strings.Contains(value, "claude") {
		return service.PlatformAnthropic
	}
	if strings.Contains(value, "gemini") || strings.Contains(value, "google") {
		return service.PlatformGemini
	}
	if strings.Contains(value, "antigravity") {
		return service.PlatformAntigravity
	}
	return service.PlatformOpenAI
}

func normalizeCurrency(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "USD"
	}
	return trimLimit(value, 12)
}

func normalizeQuery(value string) string {
	return trimLimit(strings.ToLower(strings.TrimSpace(value)), 120)
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func trimLimit(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit]
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func defaultProvisionName(group *adminplusdomain.SupplierGroup) string {
	if group == nil {
		return "AdminPlus-group"
	}
	externalID := strings.TrimSpace(group.ExternalGroupID)
	if externalID == "" {
		externalID = "group-" + time.Now().UTC().Format("20060102150405")
	}
	name := trimLimit(group.Name, 120)
	if name == "" {
		name = "unnamed"
	}
	return trimLimit("AdminPlus-"+externalID+"-"+name, 160)
}

func defaultLocalAccountName(supplier *adminplusdomain.Supplier, group *adminplusdomain.SupplierGroup) string {
	parts := []string{}
	if supplier != nil && strings.TrimSpace(supplier.Name) != "" {
		parts = append(parts, strings.TrimSpace(supplier.Name))
	}
	if group != nil && strings.TrimSpace(group.Name) != "" {
		parts = append(parts, strings.TrimSpace(group.Name))
	}
	if len(parts) == 0 {
		return "AdminPlus supplier account"
	}
	return trimLimit(strings.Join(parts, " / "), 160)
}

func defaultBaseURL(supplier *adminplusdomain.Supplier) string {
	if supplier == nil {
		return ""
	}
	for _, raw := range []string{supplier.APIBaseURL, supplier.DashboardURL} {
		value := strings.TrimRight(strings.TrimSpace(raw), "/")
		if value == "" {
			continue
		}
		if strings.HasSuffix(value, "/api/v1") {
			return strings.TrimSuffix(value, "/api/v1") + "/v1"
		}
		if strings.HasSuffix(value, "/v1") {
			return value
		}
		return value + "/v1"
	}
	return ""
}

func badRequest(reason string, message string) error {
	return infraerrors.New(http.StatusBadRequest, reason, message)
}

func internalError(message string) error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", message)
}

func localGatewayError(reason string, message string, cause error) error {
	detail := localGatewayErrorDetail(cause)
	if detail != "" {
		message = message + ": " + detail
	}
	if cause != nil && supplierKeysContainsSensitiveMarker(cause.Error()) {
		cause = errors.New("error detail redacted because it contains sensitive fields")
	}
	return infraerrors.New(http.StatusBadGateway, reason, message).WithCause(cause)
}

func localGatewayErrorDetail(err error) string {
	if err == nil {
		return ""
	}
	appErr := infraerrors.FromError(err)
	parts := make([]string, 0, 2)
	if strings.TrimSpace(appErr.Reason) != "" {
		parts = append(parts, strings.TrimSpace(appErr.Reason))
	}
	if strings.TrimSpace(appErr.Message) != "" && strings.TrimSpace(appErr.Message) != infraerrors.UnknownMessage {
		parts = append(parts, strings.TrimSpace(appErr.Message))
	}
	detail := strings.Join(parts, ": ")
	if detail == "" {
		detail = strings.TrimSpace(err.Error())
	}
	if detail == "" {
		return ""
	}
	if supplierKeysContainsSensitiveMarker(detail) {
		return "error detail redacted because it contains sensitive fields"
	}
	return trimLimit(detail, 240)
}

func supplierKeysContainsSensitiveMarker(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{
		"access_token",
		"refresh_token",
		"authorization",
		"bearer ",
		"cookie",
		"password",
		"secret",
		"session_bundle",
		"api_key",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
