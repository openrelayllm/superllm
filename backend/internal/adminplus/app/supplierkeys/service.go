package supplierkeys

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
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
	FindActiveByGroup(ctx context.Context, supplierID int64, groupID int64) (*adminplusdomain.SupplierKey, error)
	CreateKey(ctx context.Context, key *adminplusdomain.SupplierKey) (*adminplusdomain.SupplierKey, error)
	UpdateKeyAfterLocalBind(ctx context.Context, keyID int64, localAccount *service.Account, status adminplusdomain.SupplierKeyStatus, errorCode string, errorMessage string) (*adminplusdomain.SupplierKey, error)
	CreateBinding(ctx context.Context, account *adminplusdomain.SupplierAccount) (*adminplusdomain.SupplierAccount, error)
	List(ctx context.Context, filter ListFilter) ([]*adminplusdomain.SupplierKey, error)
}

type SessionReader interface {
	DecryptedProbeInput(ctx context.Context, supplierID int64) (ports.SessionProbeInput, error)
}

type LocalAccountService interface {
	CreateAccount(ctx context.Context, input *service.CreateAccountInput) (*service.Account, error)
	GetAccount(ctx context.Context, id int64) (*service.Account, error)
}

type Service struct {
	repo          Repository
	session       SessionReader
	keyAdapter    ports.SessionKeyAdapter
	localAccounts LocalAccountService
	now           func() time.Time
}

func NewService(repo Repository, session SessionReader, keyAdapter ports.SessionKeyAdapter, localAccounts LocalAccountService) *Service {
	return &Service{
		repo:          repo,
		session:       session,
		keyAdapter:    keyAdapter,
		localAccounts: localAccounts,
		now:           time.Now,
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
	if s.localAccounts == nil {
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
	if supplier.Type != adminplusdomain.SupplierTypeSub2API {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_PROVIDER_UNSUPPORTED", "only Sub2API supplier key provisioning is supported")
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

	localAccount, localErr := s.localAccounts.CreateAccount(ctx, &service.CreateAccountInput{
		Name:     normalized.LocalAccountName,
		Platform: normalized.LocalAccountPlatform,
		Type:     service.AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  created.Secret,
			"base_url": normalized.LocalAccountBaseURL,
		},
		Extra: map[string]any{
			"admin_plus_supplier_id":       normalized.SupplierID,
			"admin_plus_supplier_group_id": group.ID,
			"admin_plus_supplier_key_id":   savedKey.ID,
			"supplier_external_group_id":   group.ExternalGroupID,
		},
		Concurrency:           normalized.LocalAccountConcurrency,
		Priority:              normalized.LocalAccountPriority,
		RateMultiplier:        normalized.LocalAccountRateMultiplier,
		GroupIDs:              normalized.LocalAccountGroupIDs,
		SkipMixedChannelCheck: true,
	})
	if localErr != nil {
		_, _ = s.repo.UpdateKeyAfterLocalBind(ctx, savedKey.ID, nil, adminplusdomain.SupplierKeyStatusFailed, "LOCAL_ACCOUNT_CREATE_FAILED", localErr.Error())
		return nil, infraerrors.New(http.StatusBadGateway, "LOCAL_SUB2API_ACCOUNT_CREATE_FAILED", "failed to create local Sub2API account").WithCause(localErr)
	}

	binding := &adminplusdomain.SupplierAccount{
		SupplierID:                normalized.SupplierID,
		SupplierKeyID:             savedKey.ID,
		LocalSub2APIAccountID:     localAccount.ID,
		LocalAccountName:          localAccount.Name,
		LocalAccountPlatform:      localAccount.Platform,
		LocalAccountType:          localAccount.Type,
		SupplierAccountIdentifier: savedKey.ExternalKeyID,
		SupplierAccountLabel:      savedKey.Name,
		RateProfile:               group.ProviderFamily,
		ConfiguredConcurrency:     normalized.LocalAccountConcurrency,
		BalanceThresholdCents:     normalized.BalanceThresholdCents,
		BalanceCents:              normalized.BalanceCents,
		BalanceCurrency:           normalized.BalanceCurrency,
		HasUsableBalance:          normalized.BalanceCents > 0,
		RuntimeStatus:             normalized.RuntimeStatus,
		HealthStatus:              normalized.HealthStatus,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	savedBinding, err := s.repo.CreateBinding(ctx, binding)
	if err != nil {
		_, _ = s.repo.UpdateKeyAfterLocalBind(ctx, savedKey.ID, localAccount, adminplusdomain.SupplierKeyStatusFailed, "SUPPLIER_ACCOUNT_BIND_FAILED", err.Error())
		return nil, err
	}
	savedKey, err = s.repo.UpdateKeyAfterLocalBind(ctx, savedKey.ID, localAccount, adminplusdomain.SupplierKeyStatusBound, "", "")
	if err != nil {
		return nil, err
	}
	return &ProvisionKeyResult{Key: savedKey, Binding: savedBinding}, nil
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
	if s.localAccounts == nil {
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
	localAccount, err := s.localAccounts.GetAccount(ctx, normalized.LocalSub2APIAccountID)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	binding := &adminplusdomain.SupplierAccount{
		SupplierID:                normalized.SupplierID,
		SupplierKeyID:             key.ID,
		LocalSub2APIAccountID:     localAccount.ID,
		LocalAccountName:          localAccount.Name,
		LocalAccountPlatform:      localAccount.Platform,
		LocalAccountType:          localAccount.Type,
		SupplierAccountIdentifier: firstNonEmpty(normalized.SupplierAccountIdentifier, key.ExternalKeyID),
		SupplierAccountLabel:      firstNonEmpty(normalized.SupplierAccountLabel, key.Name),
		RateProfile:               key.ProviderFamily,
		ConfiguredConcurrency:     normalized.ConfiguredConcurrency,
		BalanceThresholdCents:     normalized.BalanceThresholdCents,
		BalanceCents:              normalized.BalanceCents,
		BalanceCurrency:           normalized.BalanceCurrency,
		HasUsableBalance:          normalized.BalanceCents > 0,
		RuntimeStatus:             normalized.RuntimeStatus,
		HealthStatus:              normalized.HealthStatus,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
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

func validLocalPlatform(platform string) bool {
	switch platform {
	case service.PlatformOpenAI, service.PlatformAnthropic, service.PlatformGemini, service.PlatformAntigravity:
		return true
	default:
		return false
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

func badRequest(reason string, message string) error {
	return infraerrors.New(http.StatusBadRequest, reason, message)
}

func internalError(message string) error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", message)
}
