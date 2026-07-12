package supplierkeys

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"sort"
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
	SyncProviderName           bool
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
	SupplierID               int64
	SyncProviderName         bool
	AllowPartial             bool
	SupplierGroupPriorityIDs []int64
	LocalAccountBaseURL      string
	LocalAccountConcurrency  int
	LocalAccountPriority     int
	LocalAccountGroupIDs     []int64
	RuntimeStatus            adminplusdomain.SupplierRuntimeStatus
	HealthStatus             adminplusdomain.SupplierHealthStatus
	BalanceThresholdCents    int64
	BalanceCents             int64
	BalanceCurrency          string
}

type EnsureGroupInput struct {
	EnsureAllInput
	SupplierGroupID int64
}

type ProvisionGroupPlan struct {
	SupplierGroupID               int64                             `json:"supplier_group_id"`
	ExternalGroupID               string                            `json:"external_group_id"`
	GroupName                     string                            `json:"group_name"`
	ProviderFamily                string                            `json:"provider_family"`
	RateMultiplier                float64                           `json:"rate_multiplier,omitempty"`
	EffectiveRateMultiplier       float64                           `json:"effective_rate_multiplier,omitempty"`
	Action                        string                            `json:"action"`
	Priority                      int                               `json:"priority,omitempty"`
	ExistingKeyID                 int64                             `json:"existing_key_id,omitempty"`
	ExistingKeyStatus             adminplusdomain.SupplierKeyStatus `json:"existing_key_status,omitempty"`
	ExistingLocalSub2APIAccountID int64                             `json:"existing_local_sub2api_account_id,omitempty"`
	ProviderExternalKeyID         string                            `json:"provider_external_key_id,omitempty"`
	ProviderKeyName               string                            `json:"provider_key_name,omitempty"`
	ProviderKeyStatus             string                            `json:"provider_key_status,omitempty"`
	GroupKeyLimitPolicy           string                            `json:"group_key_limit_policy,omitempty"`
	GroupKeyLimitValue            int                               `json:"group_key_limit_value,omitempty"`
	GroupActiveKeyCount           int                               `json:"group_active_key_count,omitempty"`
	GroupRemainingKeySlots        int                               `json:"group_remaining_key_slots,omitempty"`
	BlockedReason                 string                            `json:"blocked_reason,omitempty"`
}

type EnsureAllPlan struct {
	SupplierID        int64                `json:"supplier_id"`
	KeyLimitPolicy    string               `json:"key_limit_policy"`
	KeyLimitValue     int                  `json:"key_limit_value"`
	ActiveKeyCount    int                  `json:"active_key_count"`
	RemainingKeySlots int                  `json:"remaining_key_slots"`
	Total             int                  `json:"total"`
	ToCreate          int                  `json:"to_create"`
	AlreadySatisfied  int                  `json:"already_satisfied"`
	Blocked           int                  `json:"blocked"`
	Items             []ProvisionGroupPlan `json:"items"`
}

type providerKeyCapacitySnapshot struct {
	ActiveKeyCount             int
	ActiveByExternalGroup      map[string]ports.ProviderKeySnapshot
	ActiveCountByExternalGroup map[string]int
	Incomplete                 bool
}

type RepairBindingInput struct {
	SupplierID                 int64
	KeyID                      int64
	LocalSub2APIAccountID      int64
	ManualSecret               string
	LocalAccountPlatform       string
	LocalAccountName           string
	LocalAccountBaseURL        string
	LocalAccountPriority       int
	LocalAccountRateMultiplier *float64
	LocalAccountGroupIDs       []int64
	RuntimeStatus              adminplusdomain.SupplierRuntimeStatus
	HealthStatus               adminplusdomain.SupplierHealthStatus
	ConfiguredConcurrency      int
	BalanceThresholdCents      int64
	BalanceCents               int64
	BalanceCurrency            string
	SupplierAccountIdentifier  string
	SupplierAccountLabel       string
}

type ImportProviderProjectionInput struct {
	SupplierID      int64
	SupplierGroupID int64
	ExternalKeyID   string
}

type ImportProviderProjectionsInput struct {
	SupplierID int64
	Items      []ImportProviderProjectionInput
}

type ImportProviderProjectionsResult struct {
	SupplierID int64                                `json:"supplier_id"`
	Total      int                                  `json:"total"`
	Imported   int                                  `json:"imported"`
	Skipped    int                                  `json:"skipped"`
	Failed     int                                  `json:"failed"`
	Items      []ImportProviderProjectionResultItem `json:"items"`
}

type ImportProviderProjectionResultItem struct {
	SupplierGroupID int64                        `json:"supplier_group_id"`
	ExternalKeyID   string                       `json:"external_key_id,omitempty"`
	Action          string                       `json:"action"`
	Key             *adminplusdomain.SupplierKey `json:"key,omitempty"`
	ErrorCode       string                       `json:"error_code,omitempty"`
	ErrorMessage    string                       `json:"error_message,omitempty"`
}

type DisableLocalProjectionInput struct {
	SupplierID int64
	KeyID      int64
	Reason     string
}

type DisableProviderKeyInput struct {
	SupplierID int64
	KeyID      int64
	Reason     string
}

type DeleteProviderKeyInput struct {
	SupplierID int64
	KeyID      int64
	Reason     string
}

type StandardizeNamesInput struct {
	SupplierID       int64 `json:"supplier_id"`
	SyncProviderName bool  `json:"sync_provider_name"`
}

type ProvisionKeyResult struct {
	Key     *adminplusdomain.SupplierKey     `json:"key"`
	Binding *adminplusdomain.SupplierAccount `json:"binding"`
}

type StandardizeNamesResult struct {
	SupplierID       int64                        `json:"supplier_id"`
	SyncProviderName bool                         `json:"sync_provider_name"`
	Total            int                          `json:"total"`
	Updated          int                          `json:"updated"`
	Skipped          int                          `json:"skipped"`
	Failed           int                          `json:"failed"`
	Items            []StandardizeNamesResultItem `json:"items"`
}

type StandardizeNamesResultItem struct {
	KeyID              int64  `json:"key_id"`
	SupplierGroupID    int64  `json:"supplier_group_id"`
	ExternalKeyID      string `json:"external_key_id,omitempty"`
	LocalName          string `json:"local_name"`
	TargetLocalName    string `json:"target_local_name"`
	TargetProviderName string `json:"target_provider_name,omitempty"`
	LocalUpdated       bool   `json:"local_updated,omitempty"`
	ProviderUpdated    bool   `json:"provider_updated,omitempty"`
	Action             string `json:"action"`
	ErrorCode          string `json:"error_code,omitempty"`
	ErrorMessage       string `json:"error_message,omitempty"`
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
	UpdateKeyManualSecret(ctx context.Context, keyID int64, fingerprint string, last4 string) (*adminplusdomain.SupplierKey, error)
	UpdateKeyAfterLocalBind(ctx context.Context, keyID int64, localAccount *service.Account, status adminplusdomain.SupplierKeyStatus, errorCode string, errorMessage string) (*adminplusdomain.SupplierKey, error)
	UpdateKeyName(ctx context.Context, supplierID int64, keyID int64, name string) (*adminplusdomain.SupplierKey, error)
	DisableLocalProjection(ctx context.Context, supplierID int64, keyID int64, reason string) (*adminplusdomain.SupplierKey, error)
	MarkKeyDisabled(ctx context.Context, supplierID int64, keyID int64, errorCode string, errorMessage string) (*adminplusdomain.SupplierKey, error)
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
	targetLocalName := standardKeyNameForGroup(supplier, group, normalized.Name)
	providerCreateName := normalized.Name
	if providerCreateName == "" {
		providerCreateName = targetLocalName
	}
	existing, err := s.repo.FindActiveByGroup(ctx, normalized.SupplierID, group.ID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_GROUP_KEY_ALREADY_BOUND", "supplier group already has a bound or provisioning key")
	}
	if err := s.ensureSupplierKeyCapacityForCreate(ctx, supplier, group); err != nil {
		return nil, err
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
		Name:            providerCreateName,
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
		Name:            trimLimit(targetLocalName, 160),
		KeyFingerprint:  fingerprintSecret(created.Secret),
		KeyLast4:        lastN(created.Secret, 4),
		Status:          adminplusdomain.SupplierKeyStatusProvisioning,
		ProviderFamily:  normalizeProviderFamily(group.ProviderFamily),
		ProvisionRequest: map[string]any{
			"name":                 targetLocalName,
			"requested_name":       in.Name,
			"provider_create_name": providerCreateName,
			"sync_provider_name":   normalized.SyncProviderName,
			"external_group_id":    group.ExternalGroupID,
			"quota_usd":            normalized.QuotaUSD,
			"expires_in_days":      normalized.ExpiresInDays,
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
		Name:           firstNonEmpty(normalized.LocalAccountName, targetLocalName),
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
	if normalized.SyncProviderName {
		if _, err := s.renameProviderKey(ctx, input, savedKey, group); err != nil {
			return nil, err
		}
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
	plan, err := s.planEnsureAll(ctx, normalized, supplier)
	if err != nil {
		return nil, err
	}
	if err := ensureAllPlanCanApply(plan, normalized.AllowPartial); err != nil {
		return nil, err
	}
	result := &EnsureAllResult{
		SupplierID: normalized.SupplierID,
		Items:      make([]EnsureAllResultItem, 0, len(plan.Items)),
	}
	for _, planItem := range plan.Items {
		if planItem.Action != "create" && planItem.Action != "skipped_existing" {
			continue
		}
		group, err := s.repo.GetGroup(ctx, normalized.SupplierID, planItem.SupplierGroupID)
		if err != nil {
			result.Total++
			result.Failed++
			result.Items = append(result.Items, EnsureAllResultItem{
				SupplierGroupID: planItem.SupplierGroupID,
				ExternalGroupID: planItem.ExternalGroupID,
				GroupName:       planItem.GroupName,
				Action:          "failed",
				ErrorCode:       infraerrors.Reason(err),
				ErrorMessage:    infraerrors.Message(err),
			})
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

func (s *Service) PlanEnsureAll(ctx context.Context, in EnsureAllInput) (*EnsureAllPlan, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier key service is not configured")
	}
	normalized, supplier, err := s.normalizeEnsureAllInput(ctx, in)
	if err != nil {
		return nil, err
	}
	return s.planEnsureAll(ctx, normalized, supplier)
}

func (s *Service) planEnsureAll(ctx context.Context, normalized EnsureAllInput, supplier *adminplusdomain.Supplier) (*EnsureAllPlan, error) {
	if supplier == nil {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found")
	}
	groups, err := s.repo.ListGroups(ctx, normalized.SupplierID)
	if err != nil {
		return nil, err
	}
	keys, err := s.repo.List(ctx, ListFilter{SupplierID: normalized.SupplierID, Limit: 5000})
	if err != nil {
		return nil, err
	}
	activeKeysByGroup := make(map[int64]*adminplusdomain.SupplierKey)
	activeKeyCountsByGroup := make(map[int64]int)
	localActiveKeyCount := 0
	for _, key := range keys {
		if key == nil || !isBlockingKeyStatus(key.Status) {
			continue
		}
		localActiveKeyCount++
		activeKeyCountsByGroup[key.SupplierGroupID]++
		if current := activeKeysByGroup[key.SupplierGroupID]; current == nil || key.ID > current.ID {
			activeKeysByGroup[key.SupplierGroupID] = key
		}
	}
	activeKeyCount := localActiveKeyCount
	providerSnapshot, hasProviderSnapshot := s.readProviderKeyCapacitySnapshot(ctx, supplier)
	if hasProviderSnapshot {
		activeKeyCount = maxInt(localActiveKeyCount, providerSnapshot.ActiveKeyCount)
	}
	policy := normalizeKeyLimitPolicy(supplier.KeyLimitPolicy)
	remaining := remainingKeySlots(policy, supplier.KeyLimitValue, activeKeyCount)
	plan := &EnsureAllPlan{
		SupplierID:        normalized.SupplierID,
		KeyLimitPolicy:    policy,
		KeyLimitValue:     supplier.KeyLimitValue,
		ActiveKeyCount:    activeKeyCount,
		RemainingKeySlots: remaining,
		Items:             make([]ProvisionGroupPlan, 0, len(groups)),
	}
	createCandidates := make([]ProvisionGroupPlan, 0, len(groups))
	for _, group := range groups {
		if group == nil || group.Status != adminplusdomain.SupplierGroupStatusActive {
			continue
		}
		plan.Total++
		item := provisionGroupPlanFromGroup(group)
		if existing := activeKeysByGroup[group.ID]; existing != nil {
			item.Action = "skipped_existing"
			item.ExistingKeyID = existing.ID
			item.ExistingKeyStatus = existing.Status
			item.ExistingLocalSub2APIAccountID = existing.LocalSub2APIAccountID
			plan.AlreadySatisfied++
			plan.Items = append(plan.Items, item)
			continue
		}
		if hasProviderSnapshot {
			providerKey, hasProviderKey := providerSnapshot.activeExternalGroup(group.ExternalGroupID)
			if hasProviderKey {
				item.ProviderExternalKeyID = providerKey.ExternalKeyID
				item.ProviderKeyName = providerKey.Name
				item.ProviderKeyStatus = providerKey.Status
				item.Action = "blocked"
				item.BlockedReason = "provider_key_exists_unbound"
				plan.Blocked++
				plan.Items = append(plan.Items, item)
				continue
			}
		}
		if hasProviderSnapshot && providerSnapshot.Incomplete {
			item.Action = "blocked"
			item.BlockedReason = "provider_key_capacity_incomplete"
			plan.Blocked++
			plan.Items = append(plan.Items, item)
			continue
		}
		groupActiveCount := activeKeyCountsByGroup[group.ID]
		if hasProviderSnapshot {
			groupActiveCount = maxInt(groupActiveCount, providerSnapshot.activeExternalGroupCount(group.ExternalGroupID))
		}
		item.GroupActiveKeyCount = groupActiveCount
		item.GroupRemainingKeySlots = remainingGroupKeySlots(item.GroupKeyLimitPolicy, item.GroupKeyLimitValue, groupActiveCount)
		if blockReason := groupKeyCapacityBlockReason(item.GroupKeyLimitPolicy, item.GroupKeyLimitValue, groupActiveCount); blockReason != "" {
			item.Action = "blocked"
			item.BlockedReason = blockReason
			plan.Blocked++
			plan.Items = append(plan.Items, item)
			continue
		}
		switch policy {
		case adminplusdomain.SupplierKeyLimitPolicyUnlimited:
			item.Action = "create"
			createCandidates = append(createCandidates, item)
		case adminplusdomain.SupplierKeyLimitPolicyLimited:
			item.Action = "create"
			createCandidates = append(createCandidates, item)
		case adminplusdomain.SupplierKeyLimitPolicyUnsupported:
			item.Action = "blocked"
			item.BlockedReason = "key_provisioning_unsupported"
			plan.Blocked++
			plan.Items = append(plan.Items, item)
		default:
			item.Action = "blocked"
			item.BlockedReason = "key_capacity_unknown"
			plan.Blocked++
			plan.Items = append(plan.Items, item)
		}
	}
	sortProvisionGroupPlansByPriority(createCandidates, normalized.SupplierGroupPriorityIDs)
	for index := range createCandidates {
		createCandidates[index].Priority = index + 1
	}
	for _, item := range createCandidates {
		if policy == adminplusdomain.SupplierKeyLimitPolicyLimited && (remaining <= 0 || plan.ToCreate >= remaining) {
			item.Action = "blocked"
			item.BlockedReason = "key_capacity_exhausted"
			plan.Blocked++
			plan.Items = append(plan.Items, item)
			continue
		}
		plan.ToCreate++
		plan.Items = append(plan.Items, item)
	}
	sortProvisionGroupPlans(plan.Items)
	return plan, nil
}

func ensureAllPlanCanApply(plan *EnsureAllPlan, allowPartial bool) error {
	if plan == nil {
		return badRequest("SUPPLIER_KEY_PLAN_INVALID", "supplier key provision plan is required")
	}
	if plan.Blocked == 0 {
		return nil
	}
	if allowPartial && planHasOnlyCapacityExhaustedBlocks(plan) && planHasActionableItems(plan) {
		return nil
	}
	reason := "SUPPLIER_KEY_PLAN_BLOCKED"
	message := "supplier key provision plan has blocked groups"
	for _, item := range plan.Items {
		if item.Action != "blocked" {
			continue
		}
		switch item.BlockedReason {
		case "key_capacity_unknown":
			reason = "SUPPLIER_KEY_CAPACITY_UNKNOWN"
			message = "supplier key capacity is unknown; configure key limit policy before provisioning all groups"
		case "key_capacity_exhausted":
			reason = "SUPPLIER_KEY_CAPACITY_EXHAUSTED"
			message = "supplier key capacity is exhausted; provisioning all groups would be incomplete"
		case "group_key_capacity_unknown":
			reason = "SUPPLIER_GROUP_KEY_CAPACITY_UNKNOWN"
			message = "supplier group key capacity is unknown; configure group key limit policy before provisioning"
		case "group_key_capacity_exhausted":
			reason = "SUPPLIER_GROUP_KEY_CAPACITY_EXHAUSTED"
			message = "supplier group key capacity is exhausted; provisioning all groups would be incomplete"
		case "group_key_provisioning_unsupported":
			reason = "SUPPLIER_GROUP_KEY_PROVISIONING_UNSUPPORTED"
			message = "supplier group does not support automatic key provisioning"
		case "key_provisioning_unsupported":
			reason = "SUPPLIER_KEY_PROVIDER_UNSUPPORTED"
			message = "supplier does not support automatic key provisioning"
		case "provider_key_exists_unbound":
			reason = "SUPPLIER_PROVIDER_KEY_UNBOUND"
			message = "third-party provider already has keys that are not bound in SuperLLM"
		case "provider_key_capacity_incomplete":
			reason = "SUPPLIER_PROVIDER_KEY_CAPACITY_INCOMPLETE"
			message = "third-party provider key list was not fully read; retry sync before provisioning"
		}
		break
	}
	return infraerrors.New(http.StatusConflict, reason, message)
}

func planHasOnlyCapacityExhaustedBlocks(plan *EnsureAllPlan) bool {
	if plan == nil || plan.Blocked == 0 {
		return false
	}
	for _, item := range plan.Items {
		if item.Action == "blocked" && !isCapacityExhaustedBlockReason(item.BlockedReason) {
			return false
		}
	}
	return true
}

func isCapacityExhaustedBlockReason(reason string) bool {
	return reason == "key_capacity_exhausted" || reason == "group_key_capacity_exhausted"
}

func planHasActionableItems(plan *EnsureAllPlan) bool {
	if plan == nil {
		return false
	}
	for _, item := range plan.Items {
		if item.Action == "create" || item.Action == "skipped_existing" {
			return true
		}
	}
	return false
}

func provisionGroupPlanFromGroup(group *adminplusdomain.SupplierGroup) ProvisionGroupPlan {
	if group == nil {
		return ProvisionGroupPlan{}
	}
	effectiveRate := group.EffectiveRateMultiplier
	if effectiveRate <= 0 {
		effectiveRate = group.RateMultiplier
	}
	return ProvisionGroupPlan{
		SupplierGroupID:         group.ID,
		ExternalGroupID:         group.ExternalGroupID,
		GroupName:               group.Name,
		ProviderFamily:          group.ProviderFamily,
		RateMultiplier:          group.RateMultiplier,
		EffectiveRateMultiplier: effectiveRate,
		GroupKeyLimitPolicy:     normalizeGroupKeyLimitPolicy(group.KeyLimitPolicy),
		GroupKeyLimitValue:      group.KeyLimitValue,
		GroupActiveKeyCount:     group.ActiveKeyCount,
		GroupRemainingKeySlots:  remainingGroupKeySlots(group.KeyLimitPolicy, group.KeyLimitValue, group.ActiveKeyCount),
	}
}

func sortProvisionGroupPlans(items []ProvisionGroupPlan) {
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left.Action != right.Action {
			return provisionPlanActionRank(left.Action) < provisionPlanActionRank(right.Action)
		}
		if left.Priority > 0 || right.Priority > 0 {
			if left.Priority <= 0 {
				return false
			}
			if right.Priority <= 0 {
				return true
			}
			if left.Priority != right.Priority {
				return left.Priority < right.Priority
			}
		}
		if left.EffectiveRateMultiplier != right.EffectiveRateMultiplier {
			if left.EffectiveRateMultiplier <= 0 {
				return false
			}
			if right.EffectiveRateMultiplier <= 0 {
				return true
			}
			return left.EffectiveRateMultiplier < right.EffectiveRateMultiplier
		}
		return left.SupplierGroupID < right.SupplierGroupID
	})
}

func sortProvisionGroupPlansByPriority(items []ProvisionGroupPlan, priorityIDs []int64) {
	ranks := supplierGroupPriorityRanks(priorityIDs)
	if len(ranks) == 0 {
		sortProvisionGroupPlans(items)
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		leftRank, leftPinned := ranks[left.SupplierGroupID]
		rightRank, rightPinned := ranks[right.SupplierGroupID]
		if leftPinned || rightPinned {
			if !leftPinned {
				return false
			}
			if !rightPinned {
				return true
			}
			if leftRank != rightRank {
				return leftRank < rightRank
			}
		}
		if left.EffectiveRateMultiplier != right.EffectiveRateMultiplier {
			if left.EffectiveRateMultiplier <= 0 {
				return false
			}
			if right.EffectiveRateMultiplier <= 0 {
				return true
			}
			return left.EffectiveRateMultiplier < right.EffectiveRateMultiplier
		}
		return left.SupplierGroupID < right.SupplierGroupID
	})
}

func supplierGroupPriorityRanks(ids []int64) map[int64]int {
	if len(ids) == 0 {
		return nil
	}
	ranks := make(map[int64]int, len(ids))
	for index, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := ranks[id]; exists {
			continue
		}
		ranks[id] = index + 1
	}
	return ranks
}

func provisionPlanActionRank(action string) int {
	switch action {
	case "create":
		return 0
	case "skipped_existing":
		return 1
	case "blocked":
		return 2
	default:
		return 3
	}
}

func normalizeKeyLimitPolicy(value string) string {
	switch strings.TrimSpace(value) {
	case adminplusdomain.SupplierKeyLimitPolicyUnlimited:
		return adminplusdomain.SupplierKeyLimitPolicyUnlimited
	case adminplusdomain.SupplierKeyLimitPolicyLimited:
		return adminplusdomain.SupplierKeyLimitPolicyLimited
	case adminplusdomain.SupplierKeyLimitPolicyUnsupported:
		return adminplusdomain.SupplierKeyLimitPolicyUnsupported
	default:
		return adminplusdomain.SupplierKeyLimitPolicyUnknown
	}
}

func normalizeGroupKeyLimitPolicy(value string) string {
	switch strings.TrimSpace(value) {
	case adminplusdomain.SupplierGroupKeyLimitPolicyUnknown:
		return adminplusdomain.SupplierGroupKeyLimitPolicyUnknown
	case adminplusdomain.SupplierGroupKeyLimitPolicyUnlimited:
		return adminplusdomain.SupplierGroupKeyLimitPolicyUnlimited
	case adminplusdomain.SupplierGroupKeyLimitPolicyLimited:
		return adminplusdomain.SupplierGroupKeyLimitPolicyLimited
	case adminplusdomain.SupplierGroupKeyLimitPolicyUnsupported:
		return adminplusdomain.SupplierGroupKeyLimitPolicyUnsupported
	default:
		return adminplusdomain.SupplierGroupKeyLimitPolicyInherit
	}
}

func remainingGroupKeySlots(policy string, limit int, activeCount int) int {
	switch normalizeGroupKeyLimitPolicy(policy) {
	case adminplusdomain.SupplierGroupKeyLimitPolicyUnlimited, adminplusdomain.SupplierGroupKeyLimitPolicyInherit:
		return -1
	case adminplusdomain.SupplierGroupKeyLimitPolicyLimited:
		if limit <= activeCount {
			return 0
		}
		return limit - activeCount
	default:
		return 0
	}
}

func groupKeyCapacityBlockReason(policy string, limit int, activeCount int) string {
	switch normalizeGroupKeyLimitPolicy(policy) {
	case adminplusdomain.SupplierGroupKeyLimitPolicyInherit, adminplusdomain.SupplierGroupKeyLimitPolicyUnlimited:
		return ""
	case adminplusdomain.SupplierGroupKeyLimitPolicyLimited:
		if limit <= activeCount {
			return "group_key_capacity_exhausted"
		}
		return ""
	case adminplusdomain.SupplierGroupKeyLimitPolicyUnsupported:
		return "group_key_provisioning_unsupported"
	default:
		return "group_key_capacity_unknown"
	}
}

func groupKeyCapacityStatus(policy string, limit int, activeCount int) string {
	switch normalizeGroupKeyLimitPolicy(policy) {
	case adminplusdomain.SupplierGroupKeyLimitPolicyInherit:
		return adminplusdomain.SupplierGroupKeyLimitPolicyInherit
	case adminplusdomain.SupplierGroupKeyLimitPolicyUnlimited:
		return adminplusdomain.SupplierKeyCapacityAvailable
	case adminplusdomain.SupplierGroupKeyLimitPolicyLimited:
		if limit <= activeCount {
			return adminplusdomain.SupplierKeyCapacityExhausted
		}
		if limit-activeCount <= 2 {
			return adminplusdomain.SupplierKeyCapacityLimited
		}
		return adminplusdomain.SupplierKeyCapacityAvailable
	case adminplusdomain.SupplierGroupKeyLimitPolicyUnsupported:
		return adminplusdomain.SupplierKeyCapacityUnsupported
	default:
		return adminplusdomain.SupplierKeyCapacityUnknown
	}
}

func remainingKeySlots(policy string, limit int, activeCount int) int {
	switch policy {
	case adminplusdomain.SupplierKeyLimitPolicyUnlimited:
		return -1
	case adminplusdomain.SupplierKeyLimitPolicyLimited:
		if limit <= activeCount {
			return 0
		}
		return limit - activeCount
	default:
		return 0
	}
}

func keyCapacityStatus(policy string, limit int, activeCount int) string {
	switch normalizeKeyLimitPolicy(policy) {
	case adminplusdomain.SupplierKeyLimitPolicyUnlimited:
		return adminplusdomain.SupplierKeyCapacityAvailable
	case adminplusdomain.SupplierKeyLimitPolicyLimited:
		if limit <= activeCount {
			return adminplusdomain.SupplierKeyCapacityExhausted
		}
		if limit-activeCount <= 2 {
			return adminplusdomain.SupplierKeyCapacityLimited
		}
		return adminplusdomain.SupplierKeyCapacityAvailable
	case adminplusdomain.SupplierKeyLimitPolicyUnsupported:
		return adminplusdomain.SupplierKeyCapacityUnsupported
	default:
		return adminplusdomain.SupplierKeyCapacityUnknown
	}
}

func (s *Service) ensureSupplierKeyCapacityForCreate(ctx context.Context, supplier *adminplusdomain.Supplier, group *adminplusdomain.SupplierGroup) error {
	if supplier == nil {
		return infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found")
	}
	snapshot, hasProviderSnapshot := s.readProviderKeyCapacitySnapshot(ctx, supplier)
	if hasProviderSnapshot && group != nil && snapshot.hasActiveExternalGroup(group.ExternalGroupID) {
		return infraerrors.New(http.StatusConflict, "SUPPLIER_PROVIDER_KEY_UNBOUND", "third-party provider already has an active key for this group; import or release it before provisioning")
	}
	if hasProviderSnapshot && snapshot.Incomplete {
		return infraerrors.New(http.StatusConflict, "SUPPLIER_PROVIDER_KEY_CAPACITY_INCOMPLETE", "third-party provider key list was not fully read; retry sync before provisioning")
	}
	blockReason, err := s.groupKeyCapacityBlockReasonForCreate(ctx, supplier, group, snapshot, hasProviderSnapshot)
	if err != nil {
		return err
	}
	if blockReason != "" {
		switch blockReason {
		case "group_key_capacity_exhausted":
			return infraerrors.New(http.StatusConflict, "SUPPLIER_GROUP_KEY_CAPACITY_EXHAUSTED", "supplier group key capacity is exhausted")
		case "group_key_provisioning_unsupported":
			return infraerrors.New(http.StatusConflict, "SUPPLIER_GROUP_KEY_PROVISIONING_UNSUPPORTED", "supplier group does not support automatic key provisioning")
		default:
			return infraerrors.New(http.StatusConflict, "SUPPLIER_GROUP_KEY_CAPACITY_UNKNOWN", "supplier group key capacity is unknown")
		}
	}
	switch normalizeKeyLimitPolicy(supplier.KeyLimitPolicy) {
	case adminplusdomain.SupplierKeyLimitPolicyUnsupported:
		return infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_PROVISIONING_UNSUPPORTED", "supplier does not support automatic key provisioning")
	case adminplusdomain.SupplierKeyLimitPolicyLimited:
		activeCount, err := s.activeKeyCountForCapacity(ctx, supplier, snapshot, hasProviderSnapshot)
		if err != nil {
			return err
		}
		if supplier.KeyLimitValue <= activeCount {
			return infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_CAPACITY_EXHAUSTED", "supplier key capacity is exhausted")
		}
	}
	return nil
}

func (s *Service) groupKeyCapacityBlockReasonForCreate(ctx context.Context, supplier *adminplusdomain.Supplier, group *adminplusdomain.SupplierGroup, snapshot *providerKeyCapacitySnapshot, hasProviderSnapshot bool) (string, error) {
	if supplier == nil || group == nil {
		return "", nil
	}
	policy := normalizeGroupKeyLimitPolicy(group.KeyLimitPolicy)
	if policy == adminplusdomain.SupplierGroupKeyLimitPolicyInherit || policy == adminplusdomain.SupplierGroupKeyLimitPolicyUnlimited {
		return "", nil
	}
	if blockReason := groupKeyCapacityBlockReason(policy, group.KeyLimitValue, 0); blockReason != "" && policy != adminplusdomain.SupplierGroupKeyLimitPolicyLimited {
		return blockReason, nil
	}
	localCount := 0
	keys, err := s.repo.List(ctx, ListFilter{SupplierID: supplier.ID, Limit: 5000})
	if err != nil {
		return "", err
	}
	for _, key := range keys {
		if key != nil && key.SupplierGroupID == group.ID && isBlockingKeyStatus(key.Status) {
			localCount++
		}
	}
	if hasProviderSnapshot && snapshot != nil {
		localCount = maxInt(localCount, snapshot.activeExternalGroupCount(group.ExternalGroupID))
	}
	return groupKeyCapacityBlockReason(policy, group.KeyLimitValue, localCount), nil
}

func (s *Service) activeKeyCountForCapacity(ctx context.Context, supplier *adminplusdomain.Supplier, snapshot *providerKeyCapacitySnapshot, hasProviderSnapshot bool) (int, error) {
	if supplier == nil {
		return 0, infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found")
	}
	localCount, err := s.countActiveSupplierKeys(ctx, supplier.ID)
	if err != nil {
		return 0, err
	}
	if hasProviderSnapshot && snapshot != nil {
		return maxInt(localCount, snapshot.ActiveKeyCount), nil
	}
	return localCount, nil
}

func (s *Service) countActiveSupplierKeys(ctx context.Context, supplierID int64) (int, error) {
	keys, err := s.repo.List(ctx, ListFilter{SupplierID: supplierID, Limit: 5000})
	if err != nil {
		return 0, err
	}
	count := 0
	for _, key := range keys {
		if key != nil && isBlockingKeyStatus(key.Status) {
			count++
		}
	}
	return count, nil
}

func (s *Service) readProviderKeyCapacitySnapshot(ctx context.Context, supplier *adminplusdomain.Supplier) (*providerKeyCapacitySnapshot, bool) {
	if s == nil || s.session == nil || s.keyAdapter == nil || supplier == nil || !supplierSupportsKeyProvisioning(supplier.Type) {
		return nil, false
	}
	input, err := s.session.DecryptedProbeInput(ctx, supplier.ID)
	if err != nil {
		return nil, false
	}
	capacity, err := s.keyAdapter.ReadKeyCapacity(ctx, input, ports.ReadProviderKeyCapacityInput{
		SupplierID: supplier.ID,
		Limit:      5000,
	})
	if err == nil && capacity != nil {
		return providerKeyCapacitySnapshotFromCapacity(capacity), true
	}
	result, listErr := s.keyAdapter.ListKeys(ctx, input, ports.ListProviderKeysInput{
		SupplierID: supplier.ID,
		Limit:      5000,
	})
	if listErr != nil || result == nil {
		return nil, false
	}
	return providerKeyCapacitySnapshotFromList(result.Keys), true
}

func (s *Service) readProviderKeyCapacitySnapshotRequired(ctx context.Context, supplier *adminplusdomain.Supplier) (*providerKeyCapacitySnapshot, error) {
	if s == nil || s.session == nil || s.keyAdapter == nil || supplier == nil || !supplierSupportsKeyProvisioning(supplier.Type) {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_PROVIDER_UNSUPPORTED", "supplier key provider adapter is not available")
	}
	input, err := s.session.DecryptedProbeInput(ctx, supplier.ID)
	if err != nil {
		return nil, err
	}
	capacity, err := s.keyAdapter.ReadKeyCapacity(ctx, input, ports.ReadProviderKeyCapacityInput{
		SupplierID: supplier.ID,
		Limit:      5000,
	})
	if err == nil && capacity != nil {
		return providerKeyCapacitySnapshotFromCapacity(capacity), nil
	}
	result, listErr := s.keyAdapter.ListKeys(ctx, input, ports.ListProviderKeysInput{
		SupplierID: supplier.ID,
		Limit:      5000,
	})
	if listErr != nil {
		return nil, listErr
	}
	if result == nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_PROVIDER_KEY_LIST_EMPTY", "third-party provider key list is empty")
	}
	return providerKeyCapacitySnapshotFromList(result.Keys), nil
}

func providerKeyCapacitySnapshotFromCapacity(capacity *ports.ProviderKeyCapacityResult) *providerKeyCapacitySnapshot {
	if capacity == nil {
		return nil
	}
	snapshot := providerKeyCapacitySnapshotFromList(capacity.Keys)
	if capacity.ActiveKeyCount > snapshot.ActiveKeyCount {
		snapshot.ActiveKeyCount = capacity.ActiveKeyCount
	}
	if capacity.Diagnostics != nil {
		if truncated, ok := capacity.Diagnostics["truncated"].(bool); ok && truncated {
			snapshot.Incomplete = true
		}
	}
	return snapshot
}

func providerKeyCapacitySnapshotFromList(keys []ports.ProviderKeySnapshot) *providerKeyCapacitySnapshot {
	snapshot := &providerKeyCapacitySnapshot{
		ActiveByExternalGroup:      make(map[string]ports.ProviderKeySnapshot),
		ActiveCountByExternalGroup: make(map[string]int),
	}
	for _, key := range keys {
		if !providerKeyStatusOccupiesCapacity(key.Status) {
			continue
		}
		snapshot.ActiveKeyCount++
		groupID := strings.TrimSpace(key.ExternalGroupID)
		if groupID == "" {
			continue
		}
		snapshot.ActiveCountByExternalGroup[groupID]++
		if _, exists := snapshot.ActiveByExternalGroup[groupID]; !exists {
			snapshot.ActiveByExternalGroup[groupID] = key
		}
	}
	return snapshot
}

func (s *providerKeyCapacitySnapshot) hasActiveExternalGroup(externalGroupID string) bool {
	if s == nil || len(s.ActiveByExternalGroup) == 0 {
		return false
	}
	_, ok := s.ActiveByExternalGroup[strings.TrimSpace(externalGroupID)]
	return ok
}

func (s *providerKeyCapacitySnapshot) activeExternalGroup(externalGroupID string) (ports.ProviderKeySnapshot, bool) {
	if s == nil || len(s.ActiveByExternalGroup) == 0 {
		return ports.ProviderKeySnapshot{}, false
	}
	key, ok := s.ActiveByExternalGroup[strings.TrimSpace(externalGroupID)]
	return key, ok
}

func (s *providerKeyCapacitySnapshot) activeExternalGroupCount(externalGroupID string) int {
	if s == nil || len(s.ActiveCountByExternalGroup) == 0 {
		return 0
	}
	return s.ActiveCountByExternalGroup[strings.TrimSpace(externalGroupID)]
}

func providerKeyStatusOccupiesCapacity(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "active", "enabled", "enable":
		return true
	case "disabled", "inactive", "deleted":
		return false
	default:
		return false
	}
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
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
		SyncProviderName:           normalized.SyncProviderName,
		QuotaUSD:                   0,
		LocalAccountPlatform:       normalizeLocalPlatform(group.ProviderFamily),
		LocalAccountName:           defaultLocalAccountName(supplier, group),
		LocalAccountBaseURL:        normalized.LocalAccountBaseURL,
		LocalAccountConcurrency:    normalized.LocalAccountConcurrency,
		LocalAccountPriority:       normalized.LocalAccountPriority,
		LocalAccountRateMultiplier: &rate,
		LocalAccountGroupIDs:       normalized.LocalAccountGroupIDs,
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

func (s *Service) DisableLocalProjection(ctx context.Context, in DisableLocalProjectionInput) (*adminplusdomain.SupplierKey, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier key service is not configured")
	}
	if in.SupplierID <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if in.KeyID <= 0 {
		return nil, badRequest("SUPPLIER_KEY_ID_INVALID", "invalid supplier key id")
	}
	key, err := s.repo.GetKey(ctx, in.SupplierID, in.KeyID)
	if err != nil {
		return nil, err
	}
	if key.Status == adminplusdomain.SupplierKeyStatusDisabled {
		return key, nil
	}
	if !isBlockingKeyStatus(key.Status) {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_LOCAL_PROJECTION_RELEASE_NOT_ALLOWED", "only active supplier key projection can be released")
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		reason = "released from SuperLLM local projection; third-party key is not deleted"
	}
	return s.repo.DisableLocalProjection(ctx, in.SupplierID, in.KeyID, reason)
}

func (s *Service) DisableProviderKey(ctx context.Context, in DisableProviderKeyInput) (*adminplusdomain.SupplierKey, error) {
	return s.applyProviderKeyTerminalOperation(ctx, providerKeyTerminalOperationInput{
		SupplierID:    in.SupplierID,
		KeyID:         in.KeyID,
		Reason:        in.Reason,
		ErrorCode:     "PROVIDER_KEY_DISABLED",
		DefaultReason: "third-party provider key disabled by SuperLLM; local Sub2API account scheduling is not changed",
		Apply: func(ctx context.Context, input ports.SessionProbeInput, key *adminplusdomain.SupplierKey) (*ports.ProviderKeyResult, error) {
			return s.keyAdapter.DisableKey(ctx, input, ports.DisableProviderKeyInput{
				SupplierID:      key.SupplierID,
				ExternalKeyID:   key.ExternalKeyID,
				ExternalGroupID: key.ExternalGroupID,
				Name:            key.Name,
				Metadata:        providerKeyOperationMetadata(key),
			})
		},
	})
}

func (s *Service) DeleteProviderKey(ctx context.Context, in DeleteProviderKeyInput) (*adminplusdomain.SupplierKey, error) {
	return s.applyProviderKeyTerminalOperation(ctx, providerKeyTerminalOperationInput{
		SupplierID:    in.SupplierID,
		KeyID:         in.KeyID,
		Reason:        in.Reason,
		ErrorCode:     "PROVIDER_KEY_DELETED",
		DefaultReason: "third-party provider key deleted by SuperLLM; local Sub2API account scheduling is not changed",
		Apply: func(ctx context.Context, input ports.SessionProbeInput, key *adminplusdomain.SupplierKey) (*ports.ProviderKeyResult, error) {
			return s.keyAdapter.DeleteKey(ctx, input, ports.DeleteProviderKeyInput{
				SupplierID:      key.SupplierID,
				ExternalKeyID:   key.ExternalKeyID,
				ExternalGroupID: key.ExternalGroupID,
				Name:            key.Name,
				Metadata:        providerKeyOperationMetadata(key),
			})
		},
	})
}

type providerKeyTerminalOperationInput struct {
	SupplierID    int64
	KeyID         int64
	Reason        string
	ErrorCode     string
	DefaultReason string
	Apply         func(context.Context, ports.SessionProbeInput, *adminplusdomain.SupplierKey) (*ports.ProviderKeyResult, error)
}

func (s *Service) applyProviderKeyTerminalOperation(ctx context.Context, in providerKeyTerminalOperationInput) (*adminplusdomain.SupplierKey, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier key service is not configured")
	}
	if s.session == nil {
		return nil, internalError("supplier browser session service is not configured")
	}
	if s.keyAdapter == nil {
		return nil, internalError("supplier key provider adapter is not configured")
	}
	if in.SupplierID <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if in.KeyID <= 0 {
		return nil, badRequest("SUPPLIER_KEY_ID_INVALID", "invalid supplier key id")
	}
	if in.Apply == nil {
		return nil, internalError("supplier key provider operation is not configured")
	}
	supplier, err := s.repo.GetSupplier(ctx, in.SupplierID)
	if err != nil {
		return nil, err
	}
	if !supplierSupportsKeyProvisioning(supplier.Type) {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_PROVIDER_UNSUPPORTED", "only Sub2API or New API supplier key operations are supported")
	}
	key, err := s.repo.GetKey(ctx, in.SupplierID, in.KeyID)
	if err != nil {
		return nil, err
	}
	if key.Status == adminplusdomain.SupplierKeyStatusDisabled {
		return key, nil
	}
	if strings.TrimSpace(key.ExternalKeyID) == "" {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_EXTERNAL_ID_REQUIRED", "supplier key external id is required for provider operation")
	}
	input, err := s.session.DecryptedProbeInput(ctx, in.SupplierID)
	if err != nil {
		return nil, err
	}
	if _, err := in.Apply(ctx, input, key); err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		reason = in.DefaultReason
	}
	return s.repo.MarkKeyDisabled(ctx, in.SupplierID, in.KeyID, in.ErrorCode, reason)
}

func providerKeyOperationMetadata(key *adminplusdomain.SupplierKey) map[string]any {
	if key == nil {
		return map[string]any{}
	}
	return map[string]any{
		"supplier_key_id":   key.ID,
		"supplier_group_id": key.SupplierGroupID,
		"provider_family":   key.ProviderFamily,
	}
}

func (s *Service) StandardizeNames(ctx context.Context, in StandardizeNamesInput) (*StandardizeNamesResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier key service is not configured")
	}
	if in.SupplierID <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if in.SyncProviderName {
		if s.session == nil {
			return nil, internalError("supplier browser session service is not configured")
		}
		if s.keyAdapter == nil {
			return nil, internalError("supplier key provider adapter is not configured")
		}
	}
	supplier, err := s.repo.GetSupplier(ctx, in.SupplierID)
	if err != nil {
		return nil, err
	}
	if !supplierSupportsKeyProvisioning(supplier.Type) {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_PROVIDER_UNSUPPORTED", "only Sub2API or New API supplier key provisioning is supported")
	}
	keys, err := s.repo.List(ctx, ListFilter{SupplierID: in.SupplierID, Limit: 1000})
	if err != nil {
		return nil, err
	}
	result := &StandardizeNamesResult{
		SupplierID:       in.SupplierID,
		SyncProviderName: in.SyncProviderName,
		Items:            make([]StandardizeNamesResultItem, 0, len(keys)),
	}
	var probeInput ports.SessionProbeInput
	if in.SyncProviderName && len(keys) > 0 {
		probeInput, err = s.session.DecryptedProbeInput(ctx, in.SupplierID)
		if err != nil {
			return nil, err
		}
	}
	for _, key := range keys {
		if key == nil || !standardizableKeyStatus(key.Status) {
			continue
		}
		result.Total++
		item := StandardizeNamesResultItem{
			KeyID:           key.ID,
			SupplierGroupID: key.SupplierGroupID,
			ExternalKeyID:   key.ExternalKeyID,
			LocalName:       key.Name,
			Action:          "skipped",
		}
		group, err := s.repo.GetGroup(ctx, in.SupplierID, key.SupplierGroupID)
		if err != nil {
			item.Action = "failed"
			item.ErrorCode = infraerrors.Reason(err)
			item.ErrorMessage = infraerrors.Message(err)
			result.Failed++
			result.Items = append(result.Items, item)
			continue
		}
		item.TargetLocalName = standardKeyNameForGroup(supplier, group, key.Name)
		item.TargetProviderName = providerStableKeyName(key, group)
		if strings.TrimSpace(item.TargetLocalName) == "" {
			item.Action = "failed"
			item.ErrorCode = "SUPPLIER_KEY_NAME_INVALID"
			item.ErrorMessage = "target key name is empty"
			result.Failed++
			result.Items = append(result.Items, item)
			continue
		}
		changed := false
		if key.Name != item.TargetLocalName {
			updated, err := s.repo.UpdateKeyName(ctx, in.SupplierID, key.ID, item.TargetLocalName)
			if err != nil {
				item.Action = "failed"
				item.ErrorCode = infraerrors.Reason(err)
				item.ErrorMessage = infraerrors.Message(err)
				result.Failed++
				result.Items = append(result.Items, item)
				continue
			}
			key = updated
			item.LocalUpdated = true
			changed = true
		}
		if in.SyncProviderName {
			if _, err := s.renameProviderKey(ctx, probeInput, key, group); err != nil {
				item.Action = "failed"
				item.ErrorCode = infraerrors.Reason(err)
				item.ErrorMessage = infraerrors.Message(err)
				result.Failed++
				result.Items = append(result.Items, item)
				continue
			}
			item.ProviderUpdated = true
			changed = true
		}
		if changed {
			item.Action = "updated"
			result.Updated++
		} else {
			result.Skipped++
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func (s *Service) ImportProviderProjection(ctx context.Context, in ImportProviderProjectionInput) (*adminplusdomain.SupplierKey, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier key service is not configured")
	}
	if s.session == nil {
		return nil, internalError("supplier browser session service is not configured")
	}
	if s.keyAdapter == nil {
		return nil, internalError("supplier key provider adapter is not configured")
	}
	normalized, err := normalizeImportProviderProjectionInput(in)
	if err != nil {
		return nil, err
	}
	supplier, err := s.repo.GetSupplier(ctx, normalized.SupplierID)
	if err != nil {
		return nil, err
	}
	if !supplierSupportsKeyProvisioning(supplier.Type) {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_PROVIDER_UNSUPPORTED", "only Sub2API or New API supplier key operations are supported")
	}
	var snapshot *providerKeyCapacitySnapshot
	var snapshotErr error
	snapshotLoaded := false
	snapshotGetter := func() (*providerKeyCapacitySnapshot, error) {
		if !snapshotLoaded {
			snapshot, snapshotErr = s.readProviderKeyCapacitySnapshotRequired(ctx, supplier)
			snapshotLoaded = true
		}
		return snapshot, snapshotErr
	}
	key, _, err := s.importProviderProjectionWithSnapshot(ctx, normalized, supplier, snapshotGetter)
	return key, err
}

func (s *Service) ImportProviderProjections(ctx context.Context, in ImportProviderProjectionsInput) (*ImportProviderProjectionsResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier key service is not configured")
	}
	if s.session == nil {
		return nil, internalError("supplier browser session service is not configured")
	}
	if s.keyAdapter == nil {
		return nil, internalError("supplier key provider adapter is not configured")
	}
	normalized, err := normalizeImportProviderProjectionsInput(in)
	if err != nil {
		return nil, err
	}
	supplier, err := s.repo.GetSupplier(ctx, normalized.SupplierID)
	if err != nil {
		return nil, err
	}
	if !supplierSupportsKeyProvisioning(supplier.Type) {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_PROVIDER_UNSUPPORTED", "only Sub2API or New API supplier key operations are supported")
	}
	var snapshot *providerKeyCapacitySnapshot
	var snapshotErr error
	snapshotLoaded := false
	snapshotGetter := func() (*providerKeyCapacitySnapshot, error) {
		if !snapshotLoaded {
			snapshot, snapshotErr = s.readProviderKeyCapacitySnapshotRequired(ctx, supplier)
			snapshotLoaded = true
		}
		return snapshot, snapshotErr
	}
	result := &ImportProviderProjectionsResult{
		SupplierID: normalized.SupplierID,
		Total:      len(normalized.Items),
		Items:      make([]ImportProviderProjectionResultItem, 0, len(normalized.Items)),
	}
	for _, itemInput := range normalized.Items {
		item := ImportProviderProjectionResultItem{
			SupplierGroupID: itemInput.SupplierGroupID,
			ExternalKeyID:   itemInput.ExternalKeyID,
		}
		key, created, err := s.importProviderProjectionWithSnapshot(ctx, itemInput, supplier, snapshotGetter)
		if err != nil {
			item.Action = "failed"
			item.ErrorCode = infraerrors.Reason(err)
			item.ErrorMessage = infraerrors.Message(err)
			result.Failed++
			result.Items = append(result.Items, item)
			continue
		}
		item.Key = key
		if created {
			item.Action = "imported"
			result.Imported++
		} else {
			item.Action = "skipped_existing"
			result.Skipped++
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func (s *Service) importProviderProjectionWithSnapshot(ctx context.Context, normalized ImportProviderProjectionInput, supplier *adminplusdomain.Supplier, snapshotGetter func() (*providerKeyCapacitySnapshot, error)) (*adminplusdomain.SupplierKey, bool, error) {
	group, err := s.repo.GetGroup(ctx, normalized.SupplierID, normalized.SupplierGroupID)
	if err != nil {
		return nil, false, err
	}
	if group.Status != adminplusdomain.SupplierGroupStatusActive {
		return nil, false, infraerrors.New(http.StatusConflict, "SUPPLIER_GROUP_NOT_ACTIVE", "supplier group is not active")
	}
	existing, err := s.repo.FindActiveByGroup(ctx, normalized.SupplierID, group.ID)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		if existing.Status == adminplusdomain.SupplierKeyStatusManualSecretRequired && importProjectionMatchesExisting(normalized, existing) {
			return existing, false, nil
		}
		return nil, false, infraerrors.New(http.StatusConflict, "SUPPLIER_GROUP_KEY_ALREADY_BOUND", "supplier group already has a bound or provisioning key")
	}
	if snapshotGetter == nil {
		return nil, false, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_PROVIDER_UNSUPPORTED", "supplier key provider adapter is not available")
	}
	snapshot, err := snapshotGetter()
	if err != nil {
		return nil, false, err
	}
	providerKey, ok := snapshot.activeExternalGroup(group.ExternalGroupID)
	if !ok {
		return nil, false, infraerrors.New(http.StatusConflict, "SUPPLIER_PROVIDER_KEY_NOT_FOUND", "third-party provider active key was not found for this group")
	}
	if normalized.ExternalKeyID != "" && strings.TrimSpace(providerKey.ExternalKeyID) != "" && normalized.ExternalKeyID != strings.TrimSpace(providerKey.ExternalKeyID) {
		return nil, false, infraerrors.New(http.StatusConflict, "SUPPLIER_PROVIDER_KEY_MISMATCH", "third-party provider active key does not match requested external key id")
	}
	now := s.now().UTC()
	key := &adminplusdomain.SupplierKey{
		SupplierID:      normalized.SupplierID,
		SupplierGroupID: group.ID,
		ExternalGroupID: firstNonEmpty(providerKey.ExternalGroupID, group.ExternalGroupID),
		ExternalKeyID:   trimLimit(providerKey.ExternalKeyID, 160),
		Name:            trimLimit(firstNonEmpty(providerKey.Name, standardKeyNameForGroup(supplier, group, "")), 160),
		Status:          adminplusdomain.SupplierKeyStatusManualSecretRequired,
		ProviderFamily:  normalizeProviderFamily(group.ProviderFamily),
		ProvisionRequest: map[string]any{
			"source":                   "provider_key_list_import",
			"supplier_group_id":        group.ID,
			"external_group_id":        group.ExternalGroupID,
			"requested_external_key":   normalized.ExternalKeyID,
			"provider_external_key_id": providerKey.ExternalKeyID,
			"provider_key_name":        providerKey.Name,
			"provider_key_status":      providerKey.Status,
			"manual_secret_required":   true,
		},
		ProvisionResponse: sanitizeProviderKeyProjectionPayload(providerKey.RawPayload),
		ErrorCode:         "SUPPLIER_KEY_SECRET_REQUIRED",
		ErrorMessage:      "third-party key projection imported from provider list; paste the key secret to create the local Sub2API account",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	created, err := s.repo.CreateKey(ctx, key)
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
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
	if !repairableKeyStatus(key.Status) {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_REPAIR_NOT_ALLOWED", "only failed supplier key can be repaired")
	}
	if !repairableKeyFailure(key) {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_REPAIR_NOT_ALLOWED", "supplier key failure is not a local binding failure")
	}
	existing, err := s.repo.FindActiveByGroup(ctx, normalized.SupplierID, key.SupplierGroupID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.ID != key.ID {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_GROUP_KEY_ALREADY_BOUND", "supplier group already has a bound or provisioning key")
	}
	supplier, err := s.repo.GetSupplier(ctx, normalized.SupplierID)
	if err != nil {
		return nil, err
	}
	group, err := s.repo.GetGroup(ctx, normalized.SupplierID, key.SupplierGroupID)
	if err != nil {
		return nil, err
	}

	localAccount, key, err := s.repairLocalAccountForKey(ctx, normalized, supplier, group, key)
	if err != nil {
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
	savedBinding, err := s.repo.UpsertBinding(ctx, binding)
	if err != nil {
		return nil, err
	}
	savedKey, err := s.repo.UpdateKeyAfterLocalBind(ctx, key.ID, localAccount, adminplusdomain.SupplierKeyStatusBound, "", "")
	if err != nil {
		return nil, err
	}
	return &ProvisionKeyResult{Key: savedKey, Binding: savedBinding}, nil
}

func (s *Service) repairLocalAccountForKey(ctx context.Context, in RepairBindingInput, supplier *adminplusdomain.Supplier, group *adminplusdomain.SupplierGroup, key *adminplusdomain.SupplierKey) (*service.Account, *adminplusdomain.SupplierKey, error) {
	secret := strings.TrimSpace(in.ManualSecret)
	if secret != "" {
		updatedKey, err := s.repo.UpdateKeyManualSecret(ctx, key.ID, fingerprintSecret(secret), lastN(secret, 4))
		if err != nil {
			return nil, key, err
		}
		key = updatedKey
		localAccount, _, _, boundKey, err := s.ensureLocalAccountForKey(ctx, localAccountEnsureInput{
			Supplier:       supplier,
			Group:          group,
			Key:            key,
			Secret:         secret,
			BaseURL:        in.LocalAccountBaseURL,
			Platform:       in.LocalAccountPlatform,
			Name:           firstNonEmpty(in.LocalAccountName, key.LocalAccountName),
			Concurrency:    in.ConfiguredConcurrency,
			Priority:       in.LocalAccountPriority,
			RateMultiplier: in.LocalAccountRateMultiplier,
			GroupIDs:       in.LocalAccountGroupIDs,
		})
		if err != nil {
			return nil, key, err
		}
		if boundKey != nil {
			key = boundKey
		}
		return localAccount, key, nil
	}
	localAccount, err := s.sub2apiGateway.GetAccount(ctx, in.LocalSub2APIAccountID)
	if err != nil {
		return nil, key, err
	}
	if _, syncedAccount, err := s.ensureLocalAccountStateForGroups(ctx, localAccount, in.LocalAccountGroupIDs, in.LocalAccountBaseURL, key, group); err != nil {
		return nil, key, err
	} else if syncedAccount != nil {
		localAccount = syncedAccount
	}
	return localAccount, key, nil
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
	if strings.TrimSpace(in.LocalAccountName) != "" {
		in.LocalAccountName = trimLimit(in.LocalAccountName, 160)
	}
	in.LocalAccountBaseURL = baseURL
	in.LocalAccountGroupIDs = mergeInt64IDs(nil, in.LocalAccountGroupIDs...)
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
	in.SupplierGroupPriorityIDs = mergeInt64IDs(nil, in.SupplierGroupPriorityIDs...)
	in.LocalAccountGroupIDs = mergeInt64IDs(nil, in.LocalAccountGroupIDs...)
	in.RuntimeStatus = runtimeStatus
	in.HealthStatus = healthStatus
	in.BalanceCurrency = normalizeCurrency(in.BalanceCurrency)
	return in, supplier, nil
}

func standardizableKeyStatus(status adminplusdomain.SupplierKeyStatus) bool {
	switch status {
	case adminplusdomain.SupplierKeyStatusProvisioning, adminplusdomain.SupplierKeyStatusBound, adminplusdomain.SupplierKeyStatusManualSecretRequired:
		return true
	default:
		return false
	}
}

func (s *Service) renameProviderKey(ctx context.Context, input ports.SessionProbeInput, key *adminplusdomain.SupplierKey, group *adminplusdomain.SupplierGroup) (*ports.ProviderKeyResult, error) {
	if s == nil || s.keyAdapter == nil {
		return nil, internalError("supplier key provider adapter is not configured")
	}
	if key == nil || key.ID <= 0 {
		return nil, badRequest("SUPPLIER_KEY_ID_INVALID", "invalid supplier key id")
	}
	if strings.TrimSpace(key.ExternalKeyID) == "" {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_EXTERNAL_ID_REQUIRED", "supplier key external id is required for provider rename")
	}
	target := providerStableKeyName(key, group)
	if target == "" {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_PROVIDER_NAME_INVALID", "target provider key name is empty")
	}
	return s.keyAdapter.RenameKey(ctx, input, ports.RenameProviderKeyInput{
		SupplierID:      key.SupplierID,
		ExternalKeyID:   key.ExternalKeyID,
		ExternalGroupID: firstNonEmpty(key.ExternalGroupID, groupExternalID(group)),
		Name:            target,
		Metadata: map[string]any{
			"supplier_key_id":   key.ID,
			"supplier_group_id": key.SupplierGroupID,
			"provider_family":   providerFamilyForAlias(group),
		},
	})
}

func (s *Service) normalizeRepairBindingInput(in RepairBindingInput) (RepairBindingInput, error) {
	if in.SupplierID <= 0 {
		return RepairBindingInput{}, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if in.KeyID <= 0 {
		return RepairBindingInput{}, badRequest("SUPPLIER_KEY_ID_INVALID", "invalid supplier key id")
	}
	manualSecret := strings.TrimSpace(in.ManualSecret)
	if manualSecret == "" && in.LocalSub2APIAccountID <= 0 {
		return RepairBindingInput{}, badRequest("LOCAL_ACCOUNT_ID_INVALID", "invalid local Sub2API account id")
	}
	if manualSecret != "" && strings.TrimSpace(in.LocalAccountBaseURL) == "" {
		return RepairBindingInput{}, badRequest("LOCAL_ACCOUNT_BASE_URL_REQUIRED", "local account base url is required")
	}
	platform := strings.ToLower(strings.TrimSpace(in.LocalAccountPlatform))
	if platform != "" && !validLocalPlatform(platform) {
		return RepairBindingInput{}, badRequest("LOCAL_ACCOUNT_PLATFORM_INVALID", "invalid local account platform")
	}
	if in.ConfiguredConcurrency < 0 {
		return RepairBindingInput{}, badRequest("SUPPLIER_ACCOUNT_CONCURRENCY_INVALID", "configured concurrency cannot be negative")
	}
	if in.LocalAccountPriority < 0 {
		return RepairBindingInput{}, badRequest("LOCAL_ACCOUNT_PRIORITY_INVALID", "local account priority cannot be negative")
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
	in.ManualSecret = manualSecret
	in.LocalAccountPlatform = platform
	in.LocalAccountName = trimLimit(in.LocalAccountName, 160)
	in.LocalAccountBaseURL = strings.TrimSpace(in.LocalAccountBaseURL)
	in.LocalAccountGroupIDs = mergeInt64IDs(nil, in.LocalAccountGroupIDs...)
	in.RuntimeStatus = runtimeStatus
	in.HealthStatus = healthStatus
	in.BalanceCurrency = normalizeCurrency(in.BalanceCurrency)
	in.SupplierAccountIdentifier = trimLimit(in.SupplierAccountIdentifier, 160)
	in.SupplierAccountLabel = trimLimit(in.SupplierAccountLabel, 160)
	return in, nil
}

func normalizeImportProviderProjectionInput(in ImportProviderProjectionInput) (ImportProviderProjectionInput, error) {
	if in.SupplierID <= 0 {
		return ImportProviderProjectionInput{}, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if in.SupplierGroupID <= 0 {
		return ImportProviderProjectionInput{}, badRequest("SUPPLIER_GROUP_ID_INVALID", "invalid supplier group id")
	}
	in.ExternalKeyID = trimLimit(in.ExternalKeyID, 160)
	return in, nil
}

func normalizeImportProviderProjectionsInput(in ImportProviderProjectionsInput) (ImportProviderProjectionsInput, error) {
	if in.SupplierID <= 0 {
		return ImportProviderProjectionsInput{}, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if len(in.Items) == 0 {
		return ImportProviderProjectionsInput{}, badRequest("SUPPLIER_PROVIDER_KEY_IMPORT_ITEMS_REQUIRED", "provider key import items are required")
	}
	if len(in.Items) > 200 {
		return ImportProviderProjectionsInput{}, badRequest("SUPPLIER_PROVIDER_KEY_IMPORT_ITEMS_LIMIT_EXCEEDED", "provider key import items exceed the maximum batch size")
	}
	out := ImportProviderProjectionsInput{
		SupplierID: in.SupplierID,
		Items:      make([]ImportProviderProjectionInput, 0, len(in.Items)),
	}
	for _, item := range in.Items {
		item.SupplierID = in.SupplierID
		normalized, err := normalizeImportProviderProjectionInput(item)
		if err != nil {
			return ImportProviderProjectionsInput{}, err
		}
		out.Items = append(out.Items, normalized)
	}
	return out, nil
}

func importProjectionMatchesExisting(in ImportProviderProjectionInput, key *adminplusdomain.SupplierKey) bool {
	if key == nil {
		return false
	}
	if key.SupplierID != in.SupplierID || key.SupplierGroupID != in.SupplierGroupID {
		return false
	}
	if in.ExternalKeyID == "" {
		return true
	}
	return strings.TrimSpace(key.ExternalKeyID) == in.ExternalKeyID
}

func repairableKeyStatus(status adminplusdomain.SupplierKeyStatus) bool {
	switch status {
	case adminplusdomain.SupplierKeyStatusFailed, adminplusdomain.SupplierKeyStatusManualSecretRequired:
		return true
	default:
		return false
	}
}

func repairableKeyFailure(key *adminplusdomain.SupplierKey) bool {
	if key == nil {
		return false
	}
	if key.Status == adminplusdomain.SupplierKeyStatusManualSecretRequired {
		return true
	}
	return repairableKeyError(key.ErrorCode)
}

func repairableKeyError(errorCode string) bool {
	switch strings.TrimSpace(errorCode) {
	case "LOCAL_ACCOUNT_CREATE_FAILED", "SUPPLIER_ACCOUNT_BIND_FAILED", "LOCAL_SUB2API_ACCOUNT_SECRET_UNAVAILABLE", "SUPPLIER_KEY_SECRET_REQUIRED":
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
	groupIDs = mergeInt64IDs(groupIDs, in.LocalAccountGroupIDs...)
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
		LocalAccountGroupIDs:       in.LocalAccountGroupIDs,
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

func standardKeyNameForGroup(supplier *adminplusdomain.Supplier, group *adminplusdomain.SupplierGroup, fallback string) string {
	if group == nil {
		return trimLimit(firstNonEmpty(fallback, "AdminPlus-key"), 160)
	}
	if strings.TrimSpace(group.StandardKeyName) != "" {
		return trimLimit(group.StandardKeyName, 160)
	}
	naming := adminplusdomain.BuildSupplierGroupNaming(adminplusdomain.SupplierGroupNamingInput{
		SupplierName:   supplierNameForNaming(supplier, group),
		OfficialName:   firstNonEmpty(group.OfficialName, group.Name),
		GroupName:      group.Name,
		Description:    group.Description,
		ProviderFamily: group.ProviderFamily,
		RawPayload:     group.RawPayload,
		RateMultiplier: effectiveGroupRate(group),
	})
	return trimLimit(firstNonEmpty(naming.StandardKeyName, fallback, "AdminPlus-key"), 160)
}

func supplierNameForNaming(supplier *adminplusdomain.Supplier, group *adminplusdomain.SupplierGroup) string {
	if supplier != nil && strings.TrimSpace(supplier.Name) != "" {
		return supplier.Name
	}
	if group != nil && group.SupplierID > 0 {
		return "supplier-" + strconv.FormatInt(group.SupplierID, 10)
	}
	return "supplier"
}

func effectiveGroupRate(group *adminplusdomain.SupplierGroup) float64 {
	if group == nil {
		return 1
	}
	if group.EffectiveRateMultiplier > 0 {
		return group.EffectiveRateMultiplier
	}
	if group.UserRateMultiplier != nil && *group.UserRateMultiplier > 0 {
		return *group.UserRateMultiplier
	}
	if group.RateMultiplier > 0 {
		return group.RateMultiplier
	}
	return 1
}

func providerStableKeyName(key *adminplusdomain.SupplierKey, group *adminplusdomain.SupplierGroup) string {
	if key == nil || key.ID <= 0 {
		return ""
	}
	alias := providerAliasPart(providerFamilyForAlias(group))
	if alias == "" {
		alias = "Model"
	}
	return trimLimit("#"+strconv.FormatInt(key.ID, 10)+"-"+alias, 80)
}

func providerFamilyForAlias(group *adminplusdomain.SupplierGroup) string {
	if group == nil {
		return ""
	}
	if strings.TrimSpace(group.ModelFamily) != "" {
		return group.ModelFamily
	}
	naming := adminplusdomain.BuildSupplierGroupNaming(adminplusdomain.SupplierGroupNamingInput{
		OfficialName:   firstNonEmpty(group.OfficialName, group.Name),
		GroupName:      group.Name,
		Description:    group.Description,
		ProviderFamily: group.ProviderFamily,
		RawPayload:     group.RawPayload,
		RateMultiplier: effectiveGroupRate(group),
	})
	return firstNonEmpty(naming.ModelFamily, group.ProviderFamily, group.Name)
}

func providerAliasPart(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.ReplaceAll(value, "\\", "-")
	value = strings.Join(strings.Fields(value), "-")
	value = strings.Trim(value, "-._")
	return trimLimit(value, 40)
}

func groupExternalID(group *adminplusdomain.SupplierGroup) string {
	if group == nil {
		return ""
	}
	return group.ExternalGroupID
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
	if infraerrors.Code(err) == http.StatusNotFound {
		return true
	}
	reason := strings.ToUpper(strings.TrimSpace(infraerrors.Reason(err)))
	if strings.Contains(reason, "NOT_FOUND") {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "not found") ||
		strings.Contains(message, "not exist") ||
		strings.Contains(message, "no rows") ||
		strings.Contains(message, "404") ||
		strings.Contains(message, "不存在") ||
		strings.Contains(message, "未找到") ||
		strings.Contains(message, "已删除")
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
	if value == "" || value == "QTA" || value == "CNY" {
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

func sanitizeProviderKeyProjectionPayload(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		if providerKeyProjectionSensitiveField(key) {
			out[key] = "[redacted]"
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			out[key] = sanitizeProviderKeyProjectionPayload(nested)
			continue
		}
		out[key] = value
	}
	return out
}

func providerKeyProjectionSensitiveField(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
	switch normalized {
	case "key", "api_key", "apikey", "token", "secret", "access_token", "refresh_token":
		return true
	default:
		return strings.Contains(normalized, "secret") || strings.Contains(normalized, "token")
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
