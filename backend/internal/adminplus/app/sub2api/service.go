package sub2api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/candidateeval"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type UsageFilter struct {
	AccountID int64
	Model     string
	From      time.Time
	To        time.Time
	Limit     int
}

type LocalAccountOpsFilter struct {
	Query              string
	SupplierID         int64
	LocalGroupID       int64
	SupplierGroupID    int64
	MaxRateMultiplier  float64
	ModelScope         string
	BalanceStatus      string
	ChannelCheckStatus string
	Schedulable        *bool
	Limit              int
}

type LocalAccountOpsActionInput struct {
	Action                 adminplusdomain.LocalAccountOpsAction
	AccountIDs             []int64
	GroupIDs               []int64
	Schedulable            *bool
	DryRun                 bool
	AllowEmptyPool         bool
	RequestedBy            int64
	Reason                 string
	ActionRecommendationID int64
}

type LocalAccountStateSyncInput struct {
	AccountIDs  []int64
	Limit       int
	RequestedBy int64
}

type LocalAccountStateResolutionInput struct {
	Action      adminplusdomain.LocalAccountStateResolutionAction
	AccountIDs  []int64
	RequestedBy int64
}

type RoutingGroupAvailability struct {
	GroupID                   int64                   `json:"group_id"`
	GroupName                 string                  `json:"group_name"`
	Platform                  string                  `json:"platform,omitempty"`
	TotalAccounts             int64                   `json:"total_accounts"`
	SchedulableAccounts       int64                   `json:"schedulable_accounts"`
	ActiveAPIKeyCount         int64                   `json:"active_api_key_count"`
	WouldEmptySchedulablePool bool                    `json:"would_empty_schedulable_pool"`
	RecentWindowSeconds       int64                   `json:"recent_window_seconds,omitempty"`
	RecentSuccessRequestCount int64                   `json:"recent_success_request_count,omitempty"`
	RecentErrorRequestCount   int64                   `json:"recent_error_request_count,omitempty"`
	RecentUpstream429Count    int64                   `json:"recent_upstream_429_count,omitempty"`
	RecentTokenCount          int64                   `json:"recent_token_count,omitempty"`
	RecentLastRequestAt       *time.Time              `json:"recent_last_request_at,omitempty"`
	RecentLastErrorAt         *time.Time              `json:"recent_last_error_at,omitempty"`
	ImpactedAPIKeys           []RoutingImpactedAPIKey `json:"impacted_api_keys,omitempty"`
	ImpactedAPIKeysTruncated  bool                    `json:"impacted_api_keys_truncated,omitempty"`
	RecentFailureRequests     []RoutingFailureRequest `json:"recent_failure_requests,omitempty"`
	RecentFailuresTruncated   bool                    `json:"recent_failures_truncated,omitempty"`
}

type RoutingImpactedAPIKey struct {
	ID                        int64      `json:"id"`
	UserID                    int64      `json:"user_id"`
	Name                      string     `json:"name"`
	KeyPreview                string     `json:"key_preview,omitempty"`
	Status                    string     `json:"status"`
	LastUsedAt                *time.Time `json:"last_used_at,omitempty"`
	RecentSuccessRequestCount int64      `json:"recent_success_request_count,omitempty"`
	RecentErrorRequestCount   int64      `json:"recent_error_request_count,omitempty"`
	RecentUpstream429Count    int64      `json:"recent_upstream_429_count,omitempty"`
	RecentTokenCount          int64      `json:"recent_token_count,omitempty"`
	RecentLastRequestAt       *time.Time `json:"recent_last_request_at,omitempty"`
	RecentLastErrorAt         *time.Time `json:"recent_last_error_at,omitempty"`
}

type RoutingFailureRequest struct {
	ID                 int64     `json:"id"`
	RequestID          string    `json:"request_id,omitempty"`
	APIKeyID           int64     `json:"api_key_id,omitempty"`
	APIKeyName         string    `json:"api_key_name,omitempty"`
	APIKeyPreview      string    `json:"api_key_preview,omitempty"`
	UserID             int64     `json:"user_id,omitempty"`
	AccountID          int64     `json:"account_id,omitempty"`
	Model              string    `json:"model,omitempty"`
	StatusCode         int       `json:"status_code,omitempty"`
	UpstreamStatusCode int       `json:"upstream_status_code,omitempty"`
	ErrorOwner         string    `json:"error_owner,omitempty"`
	ErrorType          string    `json:"error_type,omitempty"`
	ErrorMessage       string    `json:"error_message,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

type RoutingImpactFilter struct {
	LocalGroupID int64
	Limit        int
}

type RoutingRefillRunFilter struct {
	LocalGroupID int64
	Status       string
	Limit        int
}

type RoutingRefillRun struct {
	ID                              int64          `json:"id"`
	RunID                           string         `json:"run_id,omitempty"`
	Sub2APIInstanceID               string         `json:"sub2api_instance_id"`
	LocalGroupID                    int64          `json:"local_group_id"`
	LocalGroupName                  string         `json:"local_group_name"`
	Platform                        string         `json:"platform,omitempty"`
	ModelScope                      string         `json:"model_scope,omitempty"`
	TriggerType                     string         `json:"trigger_type"`
	DryRun                          bool           `json:"dry_run"`
	Status                          string         `json:"status"`
	Reason                          string         `json:"reason,omitempty"`
	SkippedReason                   string         `json:"skipped_reason,omitempty"`
	BeforeTotalAccounts             int64          `json:"before_total_accounts"`
	BeforeSchedulableAccounts       int64          `json:"before_schedulable_accounts"`
	BeforeActiveAPIKeyCount         int64          `json:"before_active_api_key_count"`
	AfterTotalAccounts              int64          `json:"after_total_accounts"`
	AfterSchedulableAccounts        int64          `json:"after_schedulable_accounts"`
	AfterActiveAPIKeyCount          int64          `json:"after_active_api_key_count"`
	SelectedSupplierID              int64          `json:"selected_supplier_id,omitempty"`
	SelectedSupplierGroupID         int64          `json:"selected_supplier_group_id,omitempty"`
	SelectedSupplierKeyID           int64          `json:"selected_supplier_key_id,omitempty"`
	SelectedLocalAccountID          int64          `json:"selected_local_account_id,omitempty"`
	SelectedEffectiveRateMultiplier float64        `json:"selected_effective_rate_multiplier,omitempty"`
	RequestedBy                     int64          `json:"requested_by,omitempty"`
	ErrorCode                       string         `json:"error_code,omitempty"`
	ErrorMessage                    string         `json:"error_message,omitempty"`
	RequestSnapshot                 map[string]any `json:"request_snapshot,omitempty"`
	ResultSnapshot                  map[string]any `json:"result_snapshot,omitempty"`
	CreatedAt                       time.Time      `json:"created_at"`
	UpdatedAt                       time.Time      `json:"updated_at"`
}

type LocalSub2APIGroup struct {
	ID                        int64   `json:"id"`
	Name                      string  `json:"name"`
	Platform                  string  `json:"platform,omitempty"`
	Status                    string  `json:"status"`
	RateMultiplier            float64 `json:"rate_multiplier"`
	IsExclusive               bool    `json:"is_exclusive"`
	TotalAccounts             int64   `json:"total_accounts"`
	SchedulableAccounts       int64   `json:"schedulable_accounts"`
	ActiveAPIKeyCount         int64   `json:"active_api_key_count"`
	WouldEmptySchedulablePool bool    `json:"would_empty_schedulable_pool"`
}

type Sub2APIAccountSnapshot struct {
	AccountID   int64   `json:"account_id"`
	Name        string  `json:"name"`
	Platform    string  `json:"platform"`
	Type        string  `json:"type"`
	Status      string  `json:"status"`
	Schedulable bool    `json:"schedulable"`
	GroupIDs    []int64 `json:"group_ids"`
}

type Repository interface {
	ListLocalUsageLines(ctx context.Context, filter UsageFilter) ([]*adminplusdomain.LocalUsageLine, error)
	ListLocalUsageSummaries(ctx context.Context, filter UsageFilter) ([]*adminplusdomain.LocalUsageSummary, error)
	ListLocalAccountUsageSummaries(ctx context.Context, filter UsageFilter) ([]*adminplusdomain.LocalAccountUsageSummary, error)
	ListLocalGroups(ctx context.Context, limit int) ([]*LocalSub2APIGroup, error)
	ListLocalAccountOps(ctx context.Context, filter LocalAccountOpsFilter) ([]*adminplusdomain.LocalAccountOpsRow, error)
	ListRoutingImpactAPIKeys(ctx context.Context, filter RoutingImpactFilter) ([]RoutingImpactedAPIKey, error)
	ListRoutingImpactFailureRequests(ctx context.Context, filter RoutingImpactFilter) ([]RoutingFailureRequest, error)
	GetRoutingFailureSensitiveDetail(ctx context.Context, input RoutingSensitiveFailureDetailInput) (*RoutingSensitiveFailureDetail, error)
	CreateRoutingRefillRun(ctx context.Context, run *RoutingRefillRun) (*RoutingRefillRun, error)
	ListRoutingRefillRuns(ctx context.Context, filter RoutingRefillRunFilter) ([]*RoutingRefillRun, error)
	SyncLocalAccountState(ctx context.Context, input LocalAccountStateSyncInput) (*adminplusdomain.LocalAccountStateSyncResult, error)
}

type Sub2APIRoutingPort interface {
	GetGroupAvailability(ctx context.Context, groupID int64, platform string) (*RoutingGroupAvailability, error)
	GetAccount(ctx context.Context, accountID int64) (*Sub2APIAccountSnapshot, error)
	EnsureAccountInGroup(ctx context.Context, accountID int64, groupID int64) (*Sub2APIAccountSnapshot, error)
	SetAccountSchedulable(ctx context.Context, accountID int64, schedulable bool, reason string) (*Sub2APIAccountSnapshot, error)
	PreviewLocalAccountOpsAction(ctx context.Context, input LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error)
	ApplyLocalAccountOpsAction(ctx context.Context, input LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error)
	ResolveLocalAccountState(ctx context.Context, input LocalAccountStateResolutionInput) (*adminplusdomain.LocalAccountStateResolutionResult, error)
}

type RoutingRefillUnlockFunc func() error

type RoutingRefillLocker interface {
	TryRoutingRefillLock(ctx context.Context, groupID int64) (RoutingRefillUnlockFunc, bool, error)
}

type RuntimeReader interface {
	ListAccountRuntime(ctx context.Context, filter RuntimeFilter) ([]*adminplusdomain.LocalAccountRuntime, error)
}

type Service struct {
	repo        Repository
	routing     Sub2APIRoutingPort
	runtimeRepo RuntimeReader
	bizlog      *bizlogs.Recorder
	now         func() time.Time
}

func NewService(repo Repository, runtimeRepo RuntimeReader) *Service {
	routing, _ := repo.(Sub2APIRoutingPort)
	return &Service{
		repo:        repo,
		routing:     routing,
		runtimeRepo: runtimeRepo,
		now:         time.Now,
	}
}

func (s *Service) WithRoutingPort(routing Sub2APIRoutingPort) *Service {
	if s != nil {
		s.routing = routing
	}
	return s
}

func (s *Service) WithDiagnostics(recorder *bizlogs.Recorder) *Service {
	if s != nil {
		s.bizlog = recorder
	}
	return s
}

func (s *Service) ListLocalUsageLines(ctx context.Context, filter UsageFilter) ([]*adminplusdomain.LocalUsageLine, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("sub2api service is not configured")
	}
	normalized, err := s.normalizeFilter(filter)
	if err != nil {
		return nil, err
	}
	return s.repo.ListLocalUsageLines(ctx, normalized)
}

func (s *Service) ListLocalUsageSummaries(ctx context.Context, filter UsageFilter) ([]*adminplusdomain.LocalUsageSummary, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("sub2api service is not configured")
	}
	normalized, err := s.normalizeFilter(filter)
	if err != nil {
		return nil, err
	}
	return s.repo.ListLocalUsageSummaries(ctx, normalized)
}

func (s *Service) ListLocalAccountUsageSummaries(ctx context.Context, filter UsageFilter) ([]*adminplusdomain.LocalAccountUsageSummary, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("sub2api service is not configured")
	}
	normalized, err := s.normalizeFilter(filter)
	if err != nil {
		return nil, err
	}
	return s.repo.ListLocalAccountUsageSummaries(ctx, normalized)
}

func (s *Service) ListLocalAccountOps(ctx context.Context, filter LocalAccountOpsFilter) ([]*adminplusdomain.LocalAccountOpsRow, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("sub2api service is not configured")
	}
	normalized, err := normalizeLocalAccountOpsFilter(filter)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListLocalAccountOps(ctx, normalized)
	if err != nil {
		return nil, err
	}
	applyCandidateEvaluations(rows, normalized.ModelScope)
	return rows, nil
}

func (s *Service) ListLocalGroups(ctx context.Context, limit int) ([]*LocalSub2APIGroup, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("sub2api service is not configured")
	}
	if limit <= 0 {
		limit = 1000
	}
	if limit > 5000 {
		limit = 5000
	}
	return s.repo.ListLocalGroups(ctx, limit)
}

func (s *Service) ListRoutingImpactAPIKeys(ctx context.Context, filter RoutingImpactFilter) ([]RoutingImpactedAPIKey, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("sub2api service is not configured")
	}
	normalized, err := normalizeRoutingImpactFilter(filter)
	if err != nil {
		return nil, err
	}
	return s.repo.ListRoutingImpactAPIKeys(ctx, normalized)
}

func (s *Service) ListRoutingImpactFailureRequests(ctx context.Context, filter RoutingImpactFilter) ([]RoutingFailureRequest, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("sub2api service is not configured")
	}
	normalized, err := normalizeRoutingImpactFilter(filter)
	if err != nil {
		return nil, err
	}
	return s.repo.ListRoutingImpactFailureRequests(ctx, normalized)
}

func (s *Service) GetGroupAvailability(ctx context.Context, groupID int64, platform string) (*RoutingGroupAvailability, error) {
	if s == nil || s.routing == nil {
		return nil, internalError("sub2api routing port is not configured")
	}
	if groupID <= 0 {
		return nil, badRequest("ROUTING_GROUP_ID_INVALID", "invalid group id")
	}
	return s.routing.GetGroupAvailability(ctx, groupID, strings.TrimSpace(platform))
}

func (s *Service) GetAccount(ctx context.Context, accountID int64) (*Sub2APIAccountSnapshot, error) {
	if s == nil || s.routing == nil {
		return nil, internalError("sub2api routing port is not configured")
	}
	if accountID <= 0 {
		return nil, badRequest("ROUTING_ACCOUNT_ID_INVALID", "invalid account id")
	}
	return s.routing.GetAccount(ctx, accountID)
}

func (s *Service) EnsureAccountInGroup(ctx context.Context, accountID int64, groupID int64) (*Sub2APIAccountSnapshot, error) {
	if s == nil || s.routing == nil {
		return nil, internalError("sub2api routing port is not configured")
	}
	if accountID <= 0 {
		return nil, badRequest("ROUTING_ACCOUNT_ID_INVALID", "invalid account id")
	}
	if groupID <= 0 {
		return nil, badRequest("ROUTING_GROUP_ID_INVALID", "invalid group id")
	}
	return s.routing.EnsureAccountInGroup(ctx, accountID, groupID)
}

func (s *Service) SetAccountSchedulable(ctx context.Context, accountID int64, schedulable bool, reason string) (*Sub2APIAccountSnapshot, error) {
	if s == nil || s.routing == nil {
		return nil, internalError("sub2api routing port is not configured")
	}
	if accountID <= 0 {
		return nil, badRequest("ROUTING_ACCOUNT_ID_INVALID", "invalid account id")
	}
	return s.routing.SetAccountSchedulable(ctx, accountID, schedulable, strings.TrimSpace(reason))
}

func (s *Service) PreviewLocalAccountOpsAction(ctx context.Context, input LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	if s == nil || s.routing == nil {
		return nil, internalError("sub2api routing port is not configured")
	}
	normalized, err := normalizeLocalAccountOpsActionInput(input)
	if err != nil {
		return nil, err
	}
	normalized.DryRun = true
	return s.routing.PreviewLocalAccountOpsAction(ctx, normalized)
}

func (s *Service) ApplyLocalAccountOpsAction(ctx context.Context, input LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	if s == nil || s.routing == nil {
		return nil, internalError("sub2api routing port is not configured")
	}
	normalized, err := normalizeLocalAccountOpsActionInput(input)
	if err != nil {
		return nil, err
	}
	normalized.DryRun = false
	result, err := s.routing.ApplyLocalAccountOpsAction(ctx, normalized)
	if err != nil {
		s.recordLocalAccountOpsActionFailure(ctx, normalized, err)
		return nil, err
	}
	s.recordLocalAccountOpsActionResult(ctx, normalized, result)
	return result, nil
}

func (s *Service) SyncLocalAccountState(ctx context.Context, input LocalAccountStateSyncInput) (*adminplusdomain.LocalAccountStateSyncResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("sub2api service is not configured")
	}
	normalized, err := normalizeLocalAccountStateSyncInput(input)
	if err != nil {
		return nil, err
	}
	result, err := s.repo.SyncLocalAccountState(ctx, normalized)
	if err != nil {
		s.recordLocalAccountStateSyncFailure(ctx, normalized, err)
		return nil, err
	}
	s.recordLocalAccountStateSyncResult(ctx, normalized, result)
	return result, nil
}

func (s *Service) ResolveLocalAccountState(ctx context.Context, input LocalAccountStateResolutionInput) (*adminplusdomain.LocalAccountStateResolutionResult, error) {
	if s == nil || s.routing == nil {
		return nil, internalError("sub2api routing port is not configured")
	}
	normalized, err := normalizeLocalAccountStateResolutionInput(input)
	if err != nil {
		return nil, err
	}
	result, err := s.routing.ResolveLocalAccountState(ctx, normalized)
	if err != nil {
		s.recordLocalAccountStateResolutionFailure(ctx, normalized, err)
		return nil, err
	}
	s.recordLocalAccountStateResolutionResult(ctx, normalized, result)
	return result, nil
}

func (s *Service) ListAccountRuntime(ctx context.Context, filter RuntimeFilter) ([]*adminplusdomain.LocalAccountRuntime, error) {
	if s == nil || s.runtimeRepo == nil {
		return nil, internalError("sub2api runtime reader is not configured")
	}
	if filter.AccountID < 0 {
		return nil, badRequest("ACCOUNT_RUNTIME_ACCOUNT_ID_INVALID", "invalid account id")
	}
	return s.runtimeRepo.ListAccountRuntime(ctx, filter)
}

func (s *Service) normalizeFilter(filter UsageFilter) (UsageFilter, error) {
	if filter.AccountID < 0 {
		return UsageFilter{}, badRequest("LOCAL_USAGE_ACCOUNT_ID_INVALID", "invalid account id")
	}
	if s == nil {
		return UsageFilter{}, internalError("sub2api service is not configured")
	}
	to := filter.To.UTC()
	if to.IsZero() {
		to = s.now().UTC()
	}
	from := filter.From.UTC()
	if from.IsZero() {
		from = to.Add(-24 * time.Hour)
	}
	if !from.Before(to) {
		return UsageFilter{}, badRequest("LOCAL_USAGE_TIME_RANGE_INVALID", "from must be before to")
	}
	if to.Sub(from) > 31*24*time.Hour {
		return UsageFilter{}, badRequest("LOCAL_USAGE_TIME_RANGE_TOO_LARGE", "time range must be 31 days or less")
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	return UsageFilter{
		AccountID: filter.AccountID,
		Model:     strings.TrimSpace(filter.Model),
		From:      from,
		To:        to,
		Limit:     limit,
	}, nil
}

func normalizeLocalAccountOpsFilter(filter LocalAccountOpsFilter) (LocalAccountOpsFilter, error) {
	if filter.SupplierID < 0 {
		return LocalAccountOpsFilter{}, badRequest("LOCAL_ACCOUNT_OPS_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if filter.LocalGroupID < 0 {
		return LocalAccountOpsFilter{}, badRequest("LOCAL_ACCOUNT_OPS_GROUP_ID_INVALID", "invalid local group id")
	}
	if filter.SupplierGroupID < 0 {
		return LocalAccountOpsFilter{}, badRequest("LOCAL_ACCOUNT_OPS_SUPPLIER_GROUP_ID_INVALID", "invalid supplier group id")
	}
	if filter.MaxRateMultiplier < 0 {
		return LocalAccountOpsFilter{}, badRequest("LOCAL_ACCOUNT_OPS_MAX_RATE_INVALID", "invalid max rate multiplier")
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	balanceStatus := strings.ToLower(strings.TrimSpace(filter.BalanceStatus))
	if balanceStatus != "" && !validLocalAccountOpsBalanceStatus(balanceStatus) {
		return LocalAccountOpsFilter{}, badRequest("LOCAL_ACCOUNT_OPS_BALANCE_STATUS_INVALID", "invalid balance status")
	}
	channelStatus := strings.ToLower(strings.TrimSpace(filter.ChannelCheckStatus))
	if channelStatus != "" && !validLocalAccountOpsChannelStatus(channelStatus) {
		return LocalAccountOpsFilter{}, badRequest("LOCAL_ACCOUNT_OPS_CHANNEL_STATUS_INVALID", "invalid channel status")
	}
	return LocalAccountOpsFilter{
		Query:              strings.TrimSpace(filter.Query),
		SupplierID:         filter.SupplierID,
		LocalGroupID:       filter.LocalGroupID,
		SupplierGroupID:    filter.SupplierGroupID,
		MaxRateMultiplier:  filter.MaxRateMultiplier,
		ModelScope:         strings.TrimSpace(filter.ModelScope),
		BalanceStatus:      balanceStatus,
		ChannelCheckStatus: channelStatus,
		Schedulable:        filter.Schedulable,
		Limit:              limit,
	}, nil
}

func validLocalAccountOpsBalanceStatus(value string) bool {
	switch value {
	case "unbound", "usable", "insufficient", "unknown":
		return true
	default:
		return false
	}
}

func validLocalAccountOpsChannelStatus(value string) bool {
	switch value {
	case "untested", "available", "slow_first_token", "slow_total", "request_error", "remote_unavailable", "no_local_account", "probe_failed":
		return true
	default:
		return false
	}
}

func applyCandidateEvaluations(rows []*adminplusdomain.LocalAccountOpsRow, modelScope string) {
	for _, row := range rows {
		candidateeval.ApplyToLocalAccountOpsRowForModel(row, modelScope)
	}
}

func normalizeRoutingImpactFilter(filter RoutingImpactFilter) (RoutingImpactFilter, error) {
	if filter.LocalGroupID <= 0 {
		return RoutingImpactFilter{}, badRequest("ROUTING_GROUP_ID_INVALID", "invalid group id")
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 5000 {
		limit = 5000
	}
	return RoutingImpactFilter{
		LocalGroupID: filter.LocalGroupID,
		Limit:        limit,
	}, nil
}

func normalizeLocalAccountOpsActionInput(input LocalAccountOpsActionInput) (LocalAccountOpsActionInput, error) {
	if !input.Action.Valid() {
		return LocalAccountOpsActionInput{}, badRequest("LOCAL_ACCOUNT_OPS_ACTION_INVALID", "invalid local account operation")
	}
	if input.ActionRecommendationID < 0 {
		return LocalAccountOpsActionInput{}, badRequest("LOCAL_ACCOUNT_OPS_ACTION_ID_INVALID", "invalid action recommendation id")
	}
	accountIDs := uniquePositiveInt64s(input.AccountIDs)
	if len(accountIDs) == 0 {
		return LocalAccountOpsActionInput{}, badRequest("LOCAL_ACCOUNT_OPS_ACCOUNT_IDS_REQUIRED", "account_ids is required")
	}
	if len(accountIDs) > 500 {
		return LocalAccountOpsActionInput{}, badRequest("LOCAL_ACCOUNT_OPS_ACCOUNT_IDS_TOO_MANY", "account_ids must contain 500 items or less")
	}
	groupIDs := uniquePositiveInt64s(input.GroupIDs)
	switch input.Action {
	case adminplusdomain.LocalAccountOpsActionSetSchedulable:
		if input.Schedulable == nil {
			return LocalAccountOpsActionInput{}, badRequest("LOCAL_ACCOUNT_OPS_SCHEDULABLE_REQUIRED", "schedulable is required")
		}
	case adminplusdomain.LocalAccountOpsActionAddToGroups, adminplusdomain.LocalAccountOpsActionRemoveFromGroups:
		if len(groupIDs) == 0 {
			return LocalAccountOpsActionInput{}, badRequest("LOCAL_ACCOUNT_OPS_GROUP_IDS_REQUIRED", "group_ids is required")
		}
		if len(groupIDs) > 50 {
			return LocalAccountOpsActionInput{}, badRequest("LOCAL_ACCOUNT_OPS_GROUP_IDS_TOO_MANY", "group_ids must contain 50 items or less")
		}
	}
	return LocalAccountOpsActionInput{
		Action:                 input.Action,
		AccountIDs:             accountIDs,
		GroupIDs:               groupIDs,
		Schedulable:            input.Schedulable,
		DryRun:                 input.DryRun,
		AllowEmptyPool:         input.AllowEmptyPool,
		RequestedBy:            input.RequestedBy,
		Reason:                 strings.TrimSpace(input.Reason),
		ActionRecommendationID: input.ActionRecommendationID,
	}, nil
}

func normalizeLocalAccountStateSyncInput(input LocalAccountStateSyncInput) (LocalAccountStateSyncInput, error) {
	accountIDs := uniquePositiveInt64s(input.AccountIDs)
	if len(accountIDs) > 500 {
		return LocalAccountStateSyncInput{}, badRequest("LOCAL_ACCOUNT_STATE_SYNC_ACCOUNT_IDS_TOO_MANY", "account_ids must contain 500 items or less")
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 500
	}
	if limit > 1000 {
		limit = 1000
	}
	return LocalAccountStateSyncInput{
		AccountIDs:  accountIDs,
		Limit:       limit,
		RequestedBy: input.RequestedBy,
	}, nil
}

func normalizeLocalAccountStateResolutionInput(input LocalAccountStateResolutionInput) (LocalAccountStateResolutionInput, error) {
	if !input.Action.Valid() {
		return LocalAccountStateResolutionInput{}, badRequest("LOCAL_ACCOUNT_STATE_RESOLUTION_ACTION_INVALID", "invalid local account state resolution action")
	}
	accountIDs := uniquePositiveInt64s(input.AccountIDs)
	if len(accountIDs) == 0 {
		return LocalAccountStateResolutionInput{}, badRequest("LOCAL_ACCOUNT_STATE_RESOLUTION_ACCOUNT_IDS_REQUIRED", "account_ids is required")
	}
	if len(accountIDs) > 500 {
		return LocalAccountStateResolutionInput{}, badRequest("LOCAL_ACCOUNT_STATE_RESOLUTION_ACCOUNT_IDS_TOO_MANY", "account_ids must contain 500 items or less")
	}
	return LocalAccountStateResolutionInput{
		Action:      input.Action,
		AccountIDs:  accountIDs,
		RequestedBy: input.RequestedBy,
	}, nil
}

func (s *Service) recordLocalAccountOpsActionResult(ctx context.Context, input LocalAccountOpsActionInput, result *adminplusdomain.LocalAccountOpsActionResult) {
	if s == nil || s.bizlog == nil || result == nil {
		return
	}
	outcome := bizlogs.OutcomeSucceeded
	level := bizlogs.LevelInfo
	message := "local account operation applied"
	if result.Blocked {
		outcome = bizlogs.OutcomeBlocked
		level = bizlogs.LevelWarn
		message = "local account operation blocked by safety guard"
	}
	metadata := localAccountOpsActionLogMetadata(input, result)
	s.bizlog.Record(ctx, bizlogs.Event{
		Level:    level,
		Category: bizlogs.CategorySub2API,
		Action:   string(input.Action),
		Outcome:  outcome,
		Message:  message,
		Reason:   result.BlockedReason,
		Metadata: metadata,
	})
}

func (s *Service) recordLocalAccountOpsActionFailure(ctx context.Context, input LocalAccountOpsActionInput, err error) {
	if s == nil || s.bizlog == nil {
		return
	}
	event := bizlogs.EventFromError(bizlogs.Event{
		Level:    bizlogs.LevelWarn,
		Category: bizlogs.CategorySub2API,
		Action:   string(input.Action),
		Outcome:  bizlogs.OutcomeFailed,
		Message:  "local account operation failed",
		Metadata: localAccountOpsActionLogMetadata(input, nil),
	}, err)
	s.bizlog.Record(ctx, event)
}

func (s *Service) recordLocalAccountStateSyncResult(ctx context.Context, input LocalAccountStateSyncInput, result *adminplusdomain.LocalAccountStateSyncResult) {
	if s == nil || s.bizlog == nil || result == nil {
		return
	}
	level := bizlogs.LevelInfo
	outcome := bizlogs.OutcomeSucceeded
	message := "local account state synchronized"
	if result.PendingDriftAccounts > 0 {
		level = bizlogs.LevelWarn
		outcome = "drift_detected"
		message = "local account state drift detected"
	}
	s.bizlog.Record(ctx, bizlogs.Event{
		Level:    level,
		Category: bizlogs.CategorySub2API,
		Action:   "local_account_state_sync",
		Outcome:  outcome,
		Message:  message,
		Metadata: localAccountStateSyncLogMetadata(input, result),
	})
}

func (s *Service) recordLocalAccountStateSyncFailure(ctx context.Context, input LocalAccountStateSyncInput, err error) {
	if s == nil || s.bizlog == nil {
		return
	}
	event := bizlogs.EventFromError(bizlogs.Event{
		Level:    bizlogs.LevelWarn,
		Category: bizlogs.CategorySub2API,
		Action:   "local_account_state_sync",
		Outcome:  bizlogs.OutcomeFailed,
		Message:  "local account state sync failed",
		Metadata: localAccountStateSyncLogMetadata(input, nil),
	}, err)
	s.bizlog.Record(ctx, event)
}

func (s *Service) recordLocalAccountStateResolutionResult(ctx context.Context, input LocalAccountStateResolutionInput, result *adminplusdomain.LocalAccountStateResolutionResult) {
	if s == nil || s.bizlog == nil || result == nil {
		return
	}
	level := bizlogs.LevelInfo
	message := "local account state drift resolved"
	if result.PendingDriftAccounts > 0 {
		level = bizlogs.LevelWarn
		message = "local account state drift remains after resolution"
	}
	s.bizlog.Record(ctx, bizlogs.Event{
		Level:    level,
		Category: bizlogs.CategorySub2API,
		Action:   string(input.Action),
		Outcome:  bizlogs.OutcomeSucceeded,
		Message:  message,
		Metadata: localAccountStateResolutionLogMetadata(input, result),
	})
}

func (s *Service) recordLocalAccountStateResolutionFailure(ctx context.Context, input LocalAccountStateResolutionInput, err error) {
	if s == nil || s.bizlog == nil {
		return
	}
	event := bizlogs.EventFromError(bizlogs.Event{
		Level:    bizlogs.LevelWarn,
		Category: bizlogs.CategorySub2API,
		Action:   string(input.Action),
		Outcome:  bizlogs.OutcomeFailed,
		Message:  "local account state resolution failed",
		Metadata: localAccountStateResolutionLogMetadata(input, nil),
	}, err)
	s.bizlog.Record(ctx, event)
}

func localAccountStateResolutionLogMetadata(input LocalAccountStateResolutionInput, result *adminplusdomain.LocalAccountStateResolutionResult) map[string]any {
	metadata := map[string]any{
		"account_ids":   input.AccountIDs,
		"account_count": len(input.AccountIDs),
	}
	if input.RequestedBy > 0 {
		metadata["requested_by"] = input.RequestedBy
	}
	if result == nil {
		return metadata
	}
	metadata["resolved_accounts"] = result.ResolvedAccounts
	metadata["restored_accounts"] = result.RestoredAccounts
	metadata["pending_drift_accounts"] = result.PendingDriftAccounts
	if len(result.Warnings) > 0 {
		metadata["warnings"] = result.Warnings
	}
	if len(result.Items) > 0 {
		items := make([]map[string]any, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, map[string]any{
				"local_sub2api_account_id": item.LocalSub2APIAccountID,
				"drift_fields":             item.DriftFields,
			})
		}
		metadata["drift_items"] = items
	}
	return metadata
}

func localAccountStateSyncLogMetadata(input LocalAccountStateSyncInput, result *adminplusdomain.LocalAccountStateSyncResult) map[string]any {
	metadata := map[string]any{
		"account_ids":   input.AccountIDs,
		"account_count": len(input.AccountIDs),
		"limit":         input.Limit,
	}
	if input.RequestedBy > 0 {
		metadata["requested_by"] = input.RequestedBy
	}
	if result == nil {
		return metadata
	}
	metadata["checked_accounts"] = result.CheckedAccounts
	metadata["synced_accounts"] = result.SyncedAccounts
	metadata["drifted_accounts"] = result.DriftedAccounts
	metadata["pending_drift_accounts"] = result.PendingDriftAccounts
	if len(result.Items) > 0 {
		items := make([]map[string]any, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, map[string]any{
				"local_sub2api_account_id": item.LocalSub2APIAccountID,
				"drift_fields":             item.DriftFields,
			})
		}
		metadata["drift_items"] = items
	}
	return metadata
}

func localAccountOpsActionLogMetadata(input LocalAccountOpsActionInput, result *adminplusdomain.LocalAccountOpsActionResult) map[string]any {
	metadata := map[string]any{
		"account_ids":   input.AccountIDs,
		"account_count": len(input.AccountIDs),
		"group_ids":     input.GroupIDs,
		"group_count":   len(input.GroupIDs),
		"dry_run":       input.DryRun,
	}
	if input.Schedulable != nil {
		metadata["schedulable"] = *input.Schedulable
	}
	if input.RequestedBy > 0 {
		metadata["requested_by"] = input.RequestedBy
	}
	if input.ActionRecommendationID > 0 {
		metadata["action_id"] = input.ActionRecommendationID
	}
	if input.AllowEmptyPool {
		metadata["allow_empty_pool"] = true
	}
	if input.Reason != "" {
		metadata["reason"] = input.Reason
	}
	if result == nil {
		return metadata
	}
	metadata["blocked"] = result.Blocked
	metadata["updated_accounts"] = result.UpdatedAccounts
	metadata["added_bindings"] = result.AddedBindings
	metadata["removed_bindings"] = result.RemovedBindings
	metadata["group_impacts"] = localAccountOpsGroupImpactLogValues(result.GroupImpacts)
	if len(result.Warnings) > 0 {
		metadata["warnings"] = result.Warnings
	}
	if result.BlockedReason != "" {
		metadata["blocked_reason"] = result.BlockedReason
	}
	return metadata
}

func localAccountOpsGroupImpactLogValues(impacts []adminplusdomain.LocalAccountOpsGroupImpact) []map[string]any {
	if len(impacts) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(impacts))
	for _, impact := range impacts {
		out = append(out, map[string]any{
			"group_id":                     impact.GroupID,
			"group_name":                   impact.GroupName,
			"active_api_key_count":         impact.ActiveAPIKeyCount,
			"before_schedulable_accounts":  impact.BeforeSchedulableAccounts,
			"after_schedulable_accounts":   impact.AfterSchedulableAccounts,
			"would_empty_schedulable_pool": impact.WouldEmptySchedulablePool,
		})
	}
	return out
}

func uniquePositiveInt64s(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func badRequest(reason string, message string) error {
	return infraerrors.New(http.StatusBadRequest, reason, message)
}

func internalError(message string) error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", message)
}
