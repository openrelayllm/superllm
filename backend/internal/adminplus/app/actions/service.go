package actions

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/candidateeval"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type SupplierSignal struct {
	SupplierID         int64
	Name               string
	RuntimeStatus      adminplusdomain.SupplierRuntimeStatus
	HealthStatus       adminplusdomain.SupplierHealthStatus
	BalanceCents       int64
	Currency           string
	EffectiveCostCents int64
	KeyLimitPolicy     string
	KeyLimitValue      int
	ActiveKeyCount     int
	KeyCapacityStatus  string
}

type GenerateInput struct {
	Suppliers            []SupplierSignal
	CandidateEvaluations []CandidateSignal
	LocalGroupCapacity   []LocalGroupCapacitySignal
	LocalAccountSchedule []LocalAccountScheduleSignal
	BalanceEvents        []*adminplusdomain.BalanceEvent
	HealthEvents         []*adminplusdomain.HealthEvent
	KanbanEvents         []*adminplusdomain.KanbanEvent
	CostSnapshots        []*adminplusdomain.SupplierCostSnapshot
	MinProfitMargin      float64
}

type GenerateResult struct {
	Items []*adminplusdomain.ActionRecommendation `json:"items"`
	Total int                                     `json:"total"`
}

type RecommendationFilter struct {
	ID         int64
	SupplierID int64
	Status     adminplusdomain.ActionStatus
	Severity   adminplusdomain.ActionSeverity
	Type       adminplusdomain.ActionType
	Signal     string
	Limit      int
}

type ExecuteInput struct {
	OperatorUserID      int64
	SchedulerRunID      string
	SchedulerStepID     int64
	IdempotencyKeyHash  string
	IdempotencyReplayed bool
	BeforeSnapshot      map[string]any
	AfterSnapshot       map[string]any
	RequestPayload      map[string]any
}

type ManualExecutionInput struct {
	SupplierID         int64
	TargetSupplierID   *int64
	ActionType         adminplusdomain.ActionType
	Severity           adminplusdomain.ActionSeverity
	ReasonCode         string
	Title              string
	Description        string
	ExpectedImpact     string
	Signals            []string
	OperatorUserID     int64
	SchedulerRunID     string
	SchedulerStepID    int64
	IdempotencyKeyHash string
	BeforeSnapshot     map[string]any
	AfterSnapshot      map[string]any
	RequestPayload     map[string]any
	ResponsePayload    map[string]any
	Status             adminplusdomain.ActionExecutionStatus
	ErrorMessage       string
}

type LocalGroupCapacitySignal struct {
	LocalGroupID                   int64
	LocalGroupName                 string
	Platform                       string
	TotalAccounts                  int64
	SchedulableAccounts            int64
	ActiveAPIKeyCount              int64
	RateMultiplier                 float64
	LowCapacityThreshold           int64
	BestCandidateSupplierID        int64
	BestCandidateSupplierGroupID   int64
	BestCandidateLocalAccountID    int64
	BestCandidateRateMultiplier    float64
	BestCandidateCheckSource       string
	BestCandidateSupplierName      string
	BestCandidateSupplierGroupName string
}

type LocalAccountScheduleSignal struct {
	SupplierID              int64
	SupplierGroupID         int64
	LocalSub2APIAccountID   int64
	LocalAccountName        string
	SupplierName            string
	SupplierGroupName       string
	LocalGroupIDs           []int64
	LocalGroupNames         []string
	LocalAccountSchedulable bool
	CandidateStatus         string
	BlockedReason           string
	CheckSource             string
	BalanceStatus           string
	KeyCapacityStatus       string
	ChannelCheckStatus      string
	EffectiveRateMultiplier float64
}

type CandidateSignal struct {
	SupplierID              int64
	SupplierGroupID         int64
	LocalSub2APIAccountID   int64
	CandidateStatus         string
	BlockedReason           string
	CheckSource             string
	BalanceStatus           string
	KeyCapacityStatus       string
	EffectiveRateMultiplier float64
}

const costBalanceDeltaCriticalCents int64 = 1000

type Repository interface {
	CreateRecommendation(ctx context.Context, action *adminplusdomain.ActionRecommendation) (*adminplusdomain.ActionRecommendation, error)
	GetRecommendation(ctx context.Context, id int64) (*adminplusdomain.ActionRecommendation, error)
	ListRecommendations(ctx context.Context, filter RecommendationFilter) ([]*adminplusdomain.ActionRecommendation, error)
	UpdateRecommendationStatus(ctx context.Context, id int64, status adminplusdomain.ActionStatus) (*adminplusdomain.ActionRecommendation, error)
	CreateExecution(ctx context.Context, execution *adminplusdomain.ActionExecution) (*adminplusdomain.ActionExecution, error)
	GetExecution(ctx context.Context, id int64) (*adminplusdomain.ActionExecution, error)
	ListExecutions(ctx context.Context, recommendationID int64, limit int) ([]*adminplusdomain.ActionExecution, error)
	MarkExecutionIdempotencyReplayed(ctx context.Context, recommendationID int64, idempotencyKeyHash string) (*adminplusdomain.ActionExecution, error)
	MarkLatestExecutionIdempotencyReplayed(ctx context.Context, actionType adminplusdomain.ActionType, idempotencyKeyHash string) (*adminplusdomain.ActionExecution, error)
}

type SupplierStatusUpdater interface {
	UpdateStatus(ctx context.Context, id int64, in suppliersapp.UpdateSupplierStatusInput) (*adminplusdomain.Supplier, error)
}

type Service struct {
	repo            Repository
	supplierUpdater SupplierStatusUpdater
	now             func() time.Time
}

func NewService(repo Repository) *Service {
	return NewServiceWithDependencies(repo, nil)
}

func NewServiceWithDependencies(repo Repository, supplierUpdater SupplierStatusUpdater) *Service {
	return &Service{
		repo:            repo,
		supplierUpdater: supplierUpdater,
		now:             time.Now,
	}
}

func NewRuleService() *Service {
	return NewService(nil)
}

func (s *Service) Generate(ctx context.Context, in GenerateInput) (*GenerateResult, error) {
	if len(in.Suppliers) == 0 && len(in.CandidateEvaluations) == 0 && len(in.LocalGroupCapacity) == 0 && len(in.LocalAccountSchedule) == 0 && len(in.BalanceEvents) == 0 && len(in.HealthEvents) == 0 && len(in.KanbanEvents) == 0 && len(in.CostSnapshots) == 0 {
		return nil, badRequest("ACTION_SIGNALS_REQUIRED", "action signals are required")
	}
	now := s.now().UTC()
	suppliers := indexSuppliers(in.Suppliers)
	bestCandidate := findBestSwitchCandidate(in.Suppliers)
	items := make([]*adminplusdomain.ActionRecommendation, 0)
	items = append(items, s.actionsFromSuppliers(now, in.Suppliers, bestCandidate)...)
	items = append(items, s.actionsFromCandidateEvaluations(now, in.CandidateEvaluations)...)
	items = append(items, s.actionsFromLocalGroupCapacity(now, in.LocalGroupCapacity)...)
	items = append(items, s.actionsFromLocalAccountSchedule(now, in.LocalAccountSchedule)...)
	items = append(items, s.actionsFromBalanceEvents(now, suppliers, in.BalanceEvents, bestCandidate)...)
	items = append(items, s.actionsFromHealthEvents(now, suppliers, in.HealthEvents, bestCandidate)...)
	items = append(items, s.actionsFromKanbanEvents(now, suppliers, in.KanbanEvents, bestCandidate)...)
	items = append(items, s.actionsFromCostSnapshots(now, in.CostSnapshots)...)
	sort.SliceStable(items, func(i, j int) bool {
		return severityRank(items[i].Severity) > severityRank(items[j].Severity)
	})
	if s.repo != nil {
		stored := make([]*adminplusdomain.ActionRecommendation, 0, len(items))
		for _, item := range items {
			created, err := s.createOrReuseRecommendation(ctx, item)
			if err != nil {
				return nil, err
			}
			stored = append(stored, created)
		}
		items = stored
	}
	return &GenerateResult{Items: items, Total: len(items)}, nil
}

func (s *Service) createOrReuseRecommendation(ctx context.Context, item *adminplusdomain.ActionRecommendation) (*adminplusdomain.ActionRecommendation, error) {
	if existing, err := s.findEquivalentActiveRecommendation(ctx, item); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}
	return s.repo.CreateRecommendation(ctx, item)
}

func (s *Service) findEquivalentActiveRecommendation(ctx context.Context, item *adminplusdomain.ActionRecommendation) (*adminplusdomain.ActionRecommendation, error) {
	for _, status := range []adminplusdomain.ActionStatus{
		adminplusdomain.ActionStatusOpen,
		adminplusdomain.ActionStatusAcknowledged,
		adminplusdomain.ActionStatusApproved,
	} {
		items, err := s.repo.ListRecommendations(ctx, RecommendationFilter{
			SupplierID: item.SupplierID,
			Status:     status,
			Type:       item.Type,
			Limit:      1000,
		})
		if err != nil {
			return nil, err
		}
		for _, existing := range items {
			if equivalentRecommendation(existing, item) {
				return existing, nil
			}
		}
	}
	return nil, nil
}

func equivalentRecommendation(left *adminplusdomain.ActionRecommendation, right *adminplusdomain.ActionRecommendation) bool {
	if left == nil || right == nil {
		return false
	}
	if left.Type != right.Type || left.ReasonCode != right.ReasonCode || left.SupplierID != right.SupplierID {
		return false
	}
	if !sameOptionalInt64(left.TargetSupplierID, right.TargetSupplierID) {
		return false
	}
	return recommendationIdentity(left) == recommendationIdentity(right)
}

func recommendationIdentity(action *adminplusdomain.ActionRecommendation) string {
	if action == nil {
		return ""
	}
	for _, key := range []string{
		"local_group_id",
		"local_sub2api_account_id",
		"supplier_group_id",
		"balance_event_id",
		"cost_snapshot_id",
		"kanban_event_id",
	} {
		if value := signalValue(action.Signals, key); value != "" {
			return key + "=" + value
		}
	}
	return ""
}

func signalValue(signals []string, key string) string {
	prefix := strings.TrimSpace(key) + "="
	for _, signal := range signals {
		trimmed := strings.TrimSpace(signal)
		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		}
	}
	return ""
}

func sameOptionalInt64(left *int64, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func positiveInt64(value int64) int64 {
	if value <= 0 {
		return 0
	}
	return value
}

func (s *Service) ListRecommendations(ctx context.Context, filter RecommendationFilter) ([]*adminplusdomain.ActionRecommendation, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("action repository is not configured")
	}
	if filter.SupplierID < 0 {
		return nil, badRequest("ACTION_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if filter.ID < 0 {
		return nil, badRequest("ACTION_RECOMMENDATION_ID_INVALID", "invalid action recommendation id")
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, badRequest("ACTION_STATUS_INVALID", "invalid action status")
	}
	if filter.Severity != "" && !filter.Severity.Valid() {
		return nil, badRequest("ACTION_SEVERITY_INVALID", "invalid action severity")
	}
	if filter.Type != "" && !filter.Type.Valid() {
		return nil, badRequest("ACTION_TYPE_INVALID", "invalid action type")
	}
	filter.Signal = strings.TrimSpace(filter.Signal)
	if filter.Signal != "" && !strings.Contains(filter.Signal, "=") {
		return nil, badRequest("ACTION_SIGNAL_FILTER_INVALID", "invalid action signal filter")
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListRecommendations(ctx, filter)
}

func (s *Service) UpdateRecommendationStatus(ctx context.Context, id int64, status adminplusdomain.ActionStatus) (*adminplusdomain.ActionRecommendation, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("action repository is not configured")
	}
	if id <= 0 {
		return nil, badRequest("ACTION_RECOMMENDATION_ID_INVALID", "invalid action recommendation id")
	}
	if !status.Valid() {
		return nil, badRequest("ACTION_STATUS_INVALID", "invalid action status")
	}
	return s.repo.UpdateRecommendationStatus(ctx, id, status)
}

func (s *Service) GetRecommendation(ctx context.Context, id int64) (*adminplusdomain.ActionRecommendation, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("action repository is not configured")
	}
	if id <= 0 {
		return nil, badRequest("ACTION_RECOMMENDATION_ID_INVALID", "invalid action recommendation id")
	}
	action, err := s.repo.GetRecommendation(ctx, id)
	if err != nil {
		return nil, err
	}
	if action == nil {
		return nil, infraerrors.New(http.StatusNotFound, "ACTION_RECOMMENDATION_NOT_FOUND", "action recommendation not found")
	}
	return action, nil
}

func (s *Service) ExecuteApprovedRecommendation(ctx context.Context, id int64, in ExecuteInput) (*adminplusdomain.ActionExecution, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("action repository is not configured")
	}
	if id <= 0 {
		return nil, badRequest("ACTION_RECOMMENDATION_ID_INVALID", "invalid action recommendation id")
	}
	action, err := s.repo.GetRecommendation(ctx, id)
	if err != nil {
		return nil, err
	}
	if action.Status != adminplusdomain.ActionStatusApproved {
		return nil, infraerrors.New(http.StatusConflict, "ACTION_RECOMMENDATION_NOT_APPROVED", "action recommendation must be approved before execution")
	}
	execution := s.executionFromRecommendation(ctx, action, in, s.now().UTC())
	created, err := s.repo.CreateExecution(ctx, execution)
	if err != nil {
		return nil, err
	}
	if created.Status == adminplusdomain.ActionExecutionStatusSucceeded {
		if _, err := s.repo.UpdateRecommendationStatus(ctx, id, adminplusdomain.ActionStatusExecuted); err != nil {
			return nil, err
		}
	}
	return created, nil
}

func (s *Service) RequireApprovedRecommendation(ctx context.Context, id int64, actionType adminplusdomain.ActionType) (*adminplusdomain.ActionRecommendation, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("action repository is not configured")
	}
	if id <= 0 {
		return nil, badRequest("ACTION_RECOMMENDATION_ID_INVALID", "invalid action recommendation id")
	}
	if !actionType.Valid() {
		return nil, badRequest("ACTION_TYPE_INVALID", "invalid action type")
	}
	action, err := s.repo.GetRecommendation(ctx, id)
	if err != nil {
		return nil, err
	}
	if action.Type != actionType {
		return nil, infraerrors.New(http.StatusConflict, "ACTION_RECOMMENDATION_TYPE_MISMATCH", "action recommendation type does not match the requested workflow")
	}
	if action.Status != adminplusdomain.ActionStatusApproved {
		return nil, infraerrors.New(http.StatusConflict, "ACTION_RECOMMENDATION_NOT_APPROVED", "action recommendation must be approved before execution")
	}
	return action, nil
}

func (s *Service) RecordExternalExecution(ctx context.Context, id int64, in ExecuteInput, status adminplusdomain.ActionExecutionStatus, responsePayload map[string]any, errorMessage string) (*adminplusdomain.ActionExecution, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("action repository is not configured")
	}
	if id <= 0 {
		return nil, badRequest("ACTION_RECOMMENDATION_ID_INVALID", "invalid action recommendation id")
	}
	if !status.Valid() {
		return nil, badRequest("ACTION_EXECUTION_STATUS_INVALID", "invalid action execution status")
	}
	action, err := s.repo.GetRecommendation(ctx, id)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	created, err := s.repo.CreateExecution(ctx, &adminplusdomain.ActionExecution{
		RecommendationID:    action.ID,
		ActionType:          action.Type,
		SupplierID:          action.SupplierID,
		TargetSupplierID:    cloneInt64(action.TargetSupplierID),
		Status:              status,
		RequestPayload:      clonePayload(in.RequestPayload),
		ResponsePayload:     clonePayload(responsePayload),
		ErrorMessage:        strings.TrimSpace(errorMessage),
		OperatorUserID:      in.OperatorUserID,
		SchedulerRunID:      strings.TrimSpace(in.SchedulerRunID),
		SchedulerStepID:     positiveInt64(in.SchedulerStepID),
		IdempotencyKeyHash:  strings.TrimSpace(in.IdempotencyKeyHash),
		IdempotencyReplayed: in.IdempotencyReplayed,
		BeforeSnapshot:      clonePayload(in.BeforeSnapshot),
		AfterSnapshot:       clonePayload(in.AfterSnapshot),
		CreatedAt:           now,
		UpdatedAt:           now,
	})
	if err != nil {
		return nil, err
	}
	if created.Status == adminplusdomain.ActionExecutionStatusSucceeded {
		if _, err := s.repo.UpdateRecommendationStatus(ctx, id, adminplusdomain.ActionStatusExecuted); err != nil {
			return nil, err
		}
	}
	return created, nil
}

func (s *Service) RecordManualExecution(ctx context.Context, in ManualExecutionInput) (*adminplusdomain.ActionExecution, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("action repository is not configured")
	}
	if !in.ActionType.Valid() {
		return nil, badRequest("ACTION_TYPE_INVALID", "invalid action type")
	}
	severity := in.Severity
	if severity == "" {
		severity = adminplusdomain.ActionSeverityInfo
	}
	if !severity.Valid() {
		return nil, badRequest("ACTION_SEVERITY_INVALID", "invalid action severity")
	}
	if !in.Status.Valid() {
		return nil, badRequest("ACTION_EXECUTION_STATUS_INVALID", "invalid action execution status")
	}
	now := s.now().UTC()
	reasonCode := strings.TrimSpace(in.ReasonCode)
	if reasonCode == "" {
		reasonCode = "manual_execution_recorded"
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = "Manual action recorded"
	}
	action, err := s.repo.CreateRecommendation(ctx, &adminplusdomain.ActionRecommendation{
		SupplierID:       in.SupplierID,
		TargetSupplierID: cloneInt64(in.TargetSupplierID),
		Type:             in.ActionType,
		Severity:         severity,
		Status:           adminplusdomain.ActionStatusExecuted,
		ReasonCode:       reasonCode,
		Title:            title,
		Description:      strings.TrimSpace(in.Description),
		ExpectedImpact:   strings.TrimSpace(in.ExpectedImpact),
		RequiresApproval: false,
		Signals:          normalizeSignals(in.Signals),
		CreatedAt:        now,
	})
	if err != nil {
		return nil, err
	}
	return s.repo.CreateExecution(ctx, &adminplusdomain.ActionExecution{
		RecommendationID:   action.ID,
		ActionType:         action.Type,
		SupplierID:         action.SupplierID,
		TargetSupplierID:   cloneInt64(action.TargetSupplierID),
		Status:             in.Status,
		RequestPayload:     clonePayload(in.RequestPayload),
		ResponsePayload:    clonePayload(in.ResponsePayload),
		ErrorMessage:       strings.TrimSpace(in.ErrorMessage),
		OperatorUserID:     in.OperatorUserID,
		SchedulerRunID:     strings.TrimSpace(in.SchedulerRunID),
		SchedulerStepID:    positiveInt64(in.SchedulerStepID),
		IdempotencyKeyHash: strings.TrimSpace(in.IdempotencyKeyHash),
		BeforeSnapshot:     clonePayload(in.BeforeSnapshot),
		AfterSnapshot:      clonePayload(in.AfterSnapshot),
		CreatedAt:          now,
		UpdatedAt:          now,
	})
}

func (s *Service) ListExecutions(ctx context.Context, recommendationID int64, limit int) ([]*adminplusdomain.ActionExecution, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("action repository is not configured")
	}
	if recommendationID <= 0 {
		return nil, badRequest("ACTION_RECOMMENDATION_ID_INVALID", "invalid action recommendation id")
	}
	return s.repo.ListExecutions(ctx, recommendationID, normalizeLimit(limit))
}

func (s *Service) GetRecommendationExecution(ctx context.Context, recommendationID int64, executionID int64) (*adminplusdomain.ActionExecution, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("action repository is not configured")
	}
	if recommendationID <= 0 {
		return nil, badRequest("ACTION_RECOMMENDATION_ID_INVALID", "invalid action recommendation id")
	}
	if executionID <= 0 {
		return nil, badRequest("ACTION_EXECUTION_ID_INVALID", "invalid action execution id")
	}
	execution, err := s.repo.GetExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}
	if execution == nil {
		return nil, infraerrors.New(http.StatusNotFound, "ACTION_EXECUTION_NOT_FOUND", "action execution not found")
	}
	if execution.RecommendationID != recommendationID {
		return nil, infraerrors.New(http.StatusConflict, "ACTION_EXECUTION_RECOMMENDATION_MISMATCH", "action execution does not belong to the recommendation")
	}
	return execution, nil
}

func (s *Service) MarkExecutionIdempotencyReplayed(ctx context.Context, recommendationID int64, idempotencyKeyHash string) (*adminplusdomain.ActionExecution, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("action repository is not configured")
	}
	if recommendationID <= 0 {
		return nil, badRequest("ACTION_RECOMMENDATION_ID_INVALID", "invalid action recommendation id")
	}
	idempotencyKeyHash = strings.TrimSpace(idempotencyKeyHash)
	if idempotencyKeyHash == "" {
		return nil, badRequest("ACTION_EXECUTION_IDEMPOTENCY_KEY_HASH_REQUIRED", "action execution idempotency key hash is required")
	}
	return s.repo.MarkExecutionIdempotencyReplayed(ctx, recommendationID, idempotencyKeyHash)
}

func (s *Service) MarkLatestExecutionIdempotencyReplayed(ctx context.Context, actionType adminplusdomain.ActionType, idempotencyKeyHash string) (*adminplusdomain.ActionExecution, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("action repository is not configured")
	}
	if !actionType.Valid() {
		return nil, badRequest("ACTION_TYPE_INVALID", "invalid action type")
	}
	idempotencyKeyHash = strings.TrimSpace(idempotencyKeyHash)
	if idempotencyKeyHash == "" {
		return nil, badRequest("ACTION_EXECUTION_IDEMPOTENCY_KEY_HASH_REQUIRED", "action execution idempotency key hash is required")
	}
	return s.repo.MarkLatestExecutionIdempotencyReplayed(ctx, actionType, idempotencyKeyHash)
}

func (s *Service) actionsFromSuppliers(now time.Time, suppliers []SupplierSignal, bestCandidate *SupplierSignal) []*adminplusdomain.ActionRecommendation {
	items := make([]*adminplusdomain.ActionRecommendation, 0)
	for _, supplier := range suppliers {
		if supplier.SupplierID <= 0 {
			continue
		}
		if supplier.RuntimeStatus == adminplusdomain.SupplierRuntimeStatusActive && supplier.BalanceCents <= 0 {
			items = append(items, newAction(now, supplier.SupplierID, nil, adminplusdomain.ActionTypePauseSupplier, adminplusdomain.ActionSeverityCritical, "active_supplier_depleted", "Pause depleted active supplier", "Active supplier has no balance and must not receive traffic.", "prevent failed upstream calls", []string{"balance_cents=0"}))
			if bestCandidate != nil {
				items = append(items, newAction(now, supplier.SupplierID, &bestCandidate.SupplierID, adminplusdomain.ActionTypeSwitchSupplier, adminplusdomain.ActionSeverityCritical, "switch_from_depleted_supplier", "Switch away from depleted supplier", "A cheaper or available candidate exists while the active supplier is depleted.", "restore traffic with available balance", []string{"balance_cents=0", "candidate_available"}))
			}
		}
		if supplier.HealthStatus == adminplusdomain.SupplierHealthStatusCredentialInvalid {
			items = append(items, newAction(now, supplier.SupplierID, nil, adminplusdomain.ActionTypeReviewCredential, adminplusdomain.ActionSeverityCritical, "credential_invalid", "Review supplier credential", "Supplier credential is invalid and browser/API collection may fail.", "restore monitoring and routing eligibility", []string{"credential_invalid"}))
		}
		keyCapacityStatus := strings.ToLower(strings.TrimSpace(supplier.KeyCapacityStatus))
		switch keyCapacityStatus {
		case adminplusdomain.SupplierKeyCapacityExhausted:
			items = append(items, newAction(now, supplier.SupplierID, nil, adminplusdomain.ActionTypeReviewCredential, adminplusdomain.ActionSeverityWarning, "supplier_key_capacity_exhausted", "Review supplier key capacity", "Supplier key capacity is exhausted. Batch provisioning may only create part of the plan.", "free stale keys, raise the key limit, or explicitly run the partial provisioning plan", supplierKeyCapacitySignals(supplier)))
		case adminplusdomain.SupplierKeyCapacityUnknown:
			items = append(items, newAction(now, supplier.SupplierID, nil, adminplusdomain.ActionTypeReviewCredential, adminplusdomain.ActionSeverityInfo, "supplier_key_capacity_unknown", "Configure supplier key capacity", "Supplier key capacity is unknown. Admin Plus will block blind batch provisioning until the policy is configured or refreshed.", "make provisioning plans predictable before creating keys", supplierKeyCapacitySignals(supplier)))
		case adminplusdomain.SupplierKeyCapacityUnsupported:
			items = append(items, newAction(now, supplier.SupplierID, nil, adminplusdomain.ActionTypeReviewCredential, adminplusdomain.ActionSeverityInfo, "supplier_key_provisioning_unsupported", "Review manual key provisioning", "Supplier does not support automatic key provisioning. Use manual key handling for blocked groups.", "avoid failed automatic key creation attempts", supplierKeyCapacitySignals(supplier)))
		}
	}
	return items
}

func (s *Service) actionsFromBalanceEvents(now time.Time, suppliers map[int64]SupplierSignal, events []*adminplusdomain.BalanceEvent, bestCandidate *SupplierSignal) []*adminplusdomain.ActionRecommendation {
	items := make([]*adminplusdomain.ActionRecommendation, 0, len(events))
	for _, event := range events {
		if event == nil || event.Status != adminplusdomain.BalanceEventStatusOpen {
			continue
		}
		supplier := suppliers[event.SupplierID]
		switch event.Type {
		case adminplusdomain.BalanceEventTypeDepleted:
			items = append(items, newAction(now, event.SupplierID, nil, adminplusdomain.ActionTypeRechargeSupplier, adminplusdomain.ActionSeverityCritical, "supplier_balance_depleted", "Recharge depleted supplier", "Supplier balance is depleted. It may still be monitored, but must not be selected for switching.", "restore candidate eligibility after recharge", []string{"balance_depleted"}))
			if bestCandidate != nil && supplier.RuntimeStatus == adminplusdomain.SupplierRuntimeStatusActive {
				items = append(items, newAction(now, event.SupplierID, &bestCandidate.SupplierID, adminplusdomain.ActionTypeSwitchSupplier, adminplusdomain.ActionSeverityCritical, "switch_from_depleted_supplier", "Switch to available supplier", "Active supplier balance is depleted and another eligible supplier is available.", "keep customer traffic stable", []string{"balance_depleted", "candidate_available"}))
			}
		case adminplusdomain.BalanceEventTypeLowBalance:
			items = append(items, newAction(now, event.SupplierID, nil, adminplusdomain.ActionTypeRechargeSupplier, adminplusdomain.ActionSeverityWarning, "supplier_balance_low", "Recharge low-balance supplier", "Supplier balance is below configured threshold.", "avoid emergency traffic switch", []string{"balance_low"}))
		case adminplusdomain.BalanceEventTypeRecovered:
			items = append(items, newAction(now, event.SupplierID, nil, adminplusdomain.ActionTypeReviewCredential, adminplusdomain.ActionSeverityInfo, "supplier_balance_recovered_recheck", "Recheck recovered supplier", "Supplier balance has recovered from a low or depleted state. Refresh candidate evaluation before putting it back into routing.", "promote recovered low-rate supply back to routing review without default token probes", balanceEventSignals(event)))
		}
	}
	return items
}

func (s *Service) actionsFromCandidateEvaluations(now time.Time, evaluations []CandidateSignal) []*adminplusdomain.ActionRecommendation {
	items := make([]*adminplusdomain.ActionRecommendation, 0, len(evaluations))
	seen := make(map[string]struct{}, len(evaluations))
	for _, evaluation := range evaluations {
		if evaluation.SupplierID <= 0 {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(evaluation.CandidateStatus))
		reason := strings.ToLower(strings.TrimSpace(evaluation.BlockedReason))
		source := strings.ToLower(strings.TrimSpace(evaluation.CheckSource))
		key := strings.Join([]string{strconv.FormatInt(evaluation.SupplierID, 10), strconv.FormatInt(evaluation.SupplierGroupID, 10), strconv.FormatInt(evaluation.LocalSub2APIAccountID, 10), status, reason, source}, ":")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		signals := candidateEvaluationSignals(evaluation)
		switch {
		case status == candidateeval.StatusBalanceBlocked || reason == candidateeval.BalanceRechargeNeeded:
			items = append(items, newAction(now, evaluation.SupplierID, nil, adminplusdomain.ActionTypeRechargeSupplier, adminplusdomain.ActionSeverityWarning, "candidate_balance_recharge_required", "Recharge low-rate candidate supplier", "A candidate is blocked by balance. Keep it as a low-rate opportunity and rerun checks after recharge.", "restore low-rate routing opportunity after recharge", signals))
		case status == candidateeval.StatusCapacityBlocked || reason == "key_capacity_exhausted":
			items = append(items, newAction(now, evaluation.SupplierID, nil, adminplusdomain.ActionTypeReviewCredential, adminplusdomain.ActionSeverityWarning, "candidate_key_capacity_exhausted", "Review supplier key capacity", "A low-rate candidate cannot be provisioned because key capacity is exhausted.", "decide whether to reuse keys, delete stale keys, or prioritize groups", signals))
		case status == candidateeval.StatusNeedsProvision:
			items = append(items, newAction(now, evaluation.SupplierID, nil, adminplusdomain.ActionTypeReviewCredential, adminplusdomain.ActionSeverityInfo, "candidate_needs_provisioning", "Provision candidate binding", "A candidate needs key or local-account provisioning before it can join routing.", "prepare candidate for future routing refill", signals))
		case status == candidateeval.StatusLocalBlocked:
			items = append(items, newAction(now, evaluation.SupplierID, nil, adminplusdomain.ActionTypeReviewCredential, adminplusdomain.ActionSeverityWarning, "candidate_local_state_blocked", "Review candidate local state", "A candidate is blocked by local Sub2API state or drift.", "resolve local account state before automated refill", signals))
		case reason == "channel_monitor_failed":
			items = append(items, newAction(now, evaluation.SupplierID, nil, adminplusdomain.ActionTypeReviewCredential, adminplusdomain.ActionSeverityWarning, "candidate_channel_monitor_failed", "Review candidate channel monitor", "A candidate is blocked by channel monitor status; no active token probe is triggered by this recommendation.", "avoid spending probe tokens until low-cost monitor facts are resolved", signals))
		}
	}
	return items
}

func (s *Service) actionsFromLocalGroupCapacity(now time.Time, groups []LocalGroupCapacitySignal) []*adminplusdomain.ActionRecommendation {
	items := make([]*adminplusdomain.ActionRecommendation, 0, len(groups))
	seen := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		if group.LocalGroupID <= 0 || group.ActiveAPIKeyCount <= 0 {
			continue
		}
		threshold := group.LowCapacityThreshold
		if threshold <= 0 {
			threshold = 1
		}
		severity := adminplusdomain.ActionSeverity("")
		reason := ""
		title := ""
		description := ""
		impact := ""
		switch {
		case group.SchedulableAccounts <= 0:
			severity = adminplusdomain.ActionSeverityCritical
			reason = "local_group_routing_refill_required"
			title = "Refill empty local routing group"
			description = "A local Sub2API group has enabled user API keys but no schedulable account. Refill the lowest-rate available candidate before customer traffic keeps failing."
			impact = "restore schedulable supply for impacted user API keys"
		case group.SchedulableAccounts <= threshold:
			severity = adminplusdomain.ActionSeverityWarning
			reason = "local_group_routing_low_capacity"
			title = "Review low-capacity local routing group"
			description = "A local Sub2API group is below the configured schedulable-account threshold. Preview routing refill before it becomes an empty pool."
			impact = "increase fallback capacity while preserving low-rate priority"
		default:
			continue
		}
		key := strconv.FormatInt(group.LocalGroupID, 10) + ":" + reason
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, newAction(now, group.BestCandidateSupplierID, nil, adminplusdomain.ActionTypeRoutingRefill, severity, reason, title, description, impact, localGroupCapacitySignals(group, threshold)))
	}
	return items
}

func (s *Service) actionsFromLocalAccountSchedule(now time.Time, accounts []LocalAccountScheduleSignal) []*adminplusdomain.ActionRecommendation {
	items := make([]*adminplusdomain.ActionRecommendation, 0, len(accounts))
	seen := make(map[int64]struct{}, len(accounts))
	for _, account := range accounts {
		if account.LocalSub2APIAccountID <= 0 || !account.LocalAccountSchedulable || len(account.LocalGroupIDs) == 0 {
			continue
		}
		if _, ok := seen[account.LocalSub2APIAccountID]; ok {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(account.CandidateStatus))
		reason := strings.ToLower(strings.TrimSpace(account.BlockedReason))
		if status != candidateeval.StatusBlocked || !localAccountScheduleDisableReason(reason) {
			continue
		}
		seen[account.LocalSub2APIAccountID] = struct{}{}
		items = append(items, newAction(
			now,
			account.SupplierID,
			nil,
			adminplusdomain.ActionTypeLocalAccountScheduleDisable,
			adminplusdomain.ActionSeverityWarning,
			"local_account_schedule_disable_required",
			"Disable failed local account scheduling",
			"A schedulable local Sub2API account is still in routing groups while channel checks mark it blocked. Preview the disable action before customer traffic keeps retrying a failing account.",
			"remove a failing local account from scheduling while preserving group safety guards",
			localAccountScheduleSignals(account),
		))
	}
	return items
}

func (s *Service) actionsFromHealthEvents(now time.Time, suppliers map[int64]SupplierSignal, events []*adminplusdomain.HealthEvent, bestCandidate *SupplierSignal) []*adminplusdomain.ActionRecommendation {
	items := make([]*adminplusdomain.ActionRecommendation, 0, len(events))
	for _, event := range events {
		if event == nil || event.Status != adminplusdomain.HealthEventStatusOpen {
			continue
		}
		supplier := suppliers[event.SupplierID]
		switch event.Type {
		case adminplusdomain.HealthEventTypeRequestError:
			items = append(items, newAction(now, event.SupplierID, nil, adminplusdomain.ActionTypePauseSupplier, adminplusdomain.ActionSeverityCritical, "supplier_request_errors", "Pause failing supplier", "Supplier returned request errors and may impact customer traffic.", "stop routing traffic to failing node", []string{"request_error"}))
			if bestCandidate != nil && supplier.RuntimeStatus == adminplusdomain.SupplierRuntimeStatusActive {
				items = append(items, newAction(now, event.SupplierID, &bestCandidate.SupplierID, adminplusdomain.ActionTypeSwitchSupplier, adminplusdomain.ActionSeverityCritical, "switch_from_failing_supplier", "Switch from failing supplier", "Active supplier is failing and another eligible supplier is available.", "restore stable API responses", []string{"request_error", "candidate_available"}))
			}
		case adminplusdomain.HealthEventTypeSlowFirstToken, adminplusdomain.HealthEventTypeSlowTotal, adminplusdomain.HealthEventTypeConcurrencyFull:
			items = append(items, newAction(now, event.SupplierID, nil, adminplusdomain.ActionTypeDegradeSupplier, adminplusdomain.ActionSeverityWarning, "supplier_performance_degraded", "Degrade supplier weight", "Supplier performance is degraded or concurrency is saturated.", "reduce slow responses while preserving fallback capacity", []string{string(event.Type)}))
		}
	}
	return items
}

func (s *Service) actionsFromKanbanEvents(now time.Time, suppliers map[int64]SupplierSignal, events []*adminplusdomain.KanbanEvent, bestCandidate *SupplierSignal) []*adminplusdomain.ActionRecommendation {
	items := make([]*adminplusdomain.ActionRecommendation, 0, len(events))
	for _, event := range events {
		if event == nil || event.Status != "open" {
			continue
		}
		supplierID := supplierIDFromKanbanEvent(event)
		severity := actionSeverityFromKanbanEvent(event.Severity)
		signals := kanbanEventSignals(event)
		supplier := suppliers[supplierID]
		switch event.EventType {
		case "supply_quality_risk":
			if supplierID <= 0 {
				items = append(items, newAction(now, 0, nil, adminplusdomain.ActionTypeInvestigateProfit, severity, "kanban_supply_quality_risk", "Review supply quality risk", eventDescription(event, "Supply quality risk needs manual review."), "prevent low-quality supply from entering production", signals))
				continue
			}
			actionType := adminplusdomain.ActionTypeDegradeSupplier
			reason := "kanban_supply_quality_risk"
			title := "Degrade risky supplier"
			impact := "reduce exposure to low-quality supply while keeping fallback capacity"
			if severity == adminplusdomain.ActionSeverityCritical {
				actionType = adminplusdomain.ActionTypePauseSupplier
				reason = "kanban_supply_quality_blocked"
				title = "Pause blocked supplier"
				impact = "stop routing production traffic to failed quality source"
			}
			items = append(items, newAction(now, supplierID, nil, actionType, severity, reason, title, eventDescription(event, "Supply quality is below production standard."), impact, signals))
			items = append(items, switchActionFromKanban(now, supplier, supplierID, bestCandidate, severity, "switch_from_supply_quality_risk", "Switch from risky supplier", eventDescription(event, "Active supplier has quality risk and another candidate is available."), "restore stable traffic with a safer supplier", signals)...)
		case "acceptance_risk":
			if supplierID <= 0 {
				items = append(items, newAction(now, 0, nil, adminplusdomain.ActionTypeReviewCredential, severity, "kanban_acceptance_risk", "Review acceptance risk", eventDescription(event, "Acceptance failed or is not production-ready."), "keep unaccepted supply out of production candidates", signals))
				continue
			}
			actionType := adminplusdomain.ActionTypeReviewCredential
			reason := "kanban_acceptance_risk"
			title := "Review supplier acceptance"
			impact := "keep unaccepted supply out of production candidates"
			if severity == adminplusdomain.ActionSeverityCritical && supplier.RuntimeStatus == adminplusdomain.SupplierRuntimeStatusActive {
				actionType = adminplusdomain.ActionTypePauseSupplier
				reason = "kanban_acceptance_blocked_active_supplier"
				title = "Pause supplier with failed acceptance"
				impact = "stop routing production traffic to a supplier that failed acceptance"
			}
			items = append(items, newAction(now, supplierID, nil, actionType, severity, reason, title, eventDescription(event, "Acceptance failed or is not production-ready."), impact, signals))
			items = append(items, switchActionFromKanban(now, supplier, supplierID, bestCandidate, severity, "switch_from_acceptance_risk", "Switch from failed-acceptance supplier", eventDescription(event, "Active supplier failed acceptance and another candidate is available."), "restore traffic with an accepted supplier candidate", signals)...)
		case "cache_efficiency_risk":
			if supplierID > 0 {
				items = append(items, newAction(now, supplierID, nil, adminplusdomain.ActionTypeDegradeSupplier, severity, "kanban_cache_efficiency_risk", "Degrade low-cache supplier", eventDescription(event, "Cache efficiency risk increases real cost."), "reduce repeated-input cost caused by low cache hit rate", signals))
			} else {
				items = append(items, newAction(now, 0, nil, adminplusdomain.ActionTypeInvestigateProfit, severity, "kanban_cache_efficiency_risk", "Review cache-adjusted cost", eventDescription(event, "Cache efficiency risk increases real cost."), "correct pricing and routing assumptions for low cache hit rate", signals))
			}
		case "market_price_drop", "market_price_anomaly", "market_model_added", "market_model_removed", "market_promotion", "unprofitable_model", "pricing_recommendation":
			items = append(items, newAction(now, 0, nil, adminplusdomain.ActionTypeInvestigateProfit, severity, "kanban_pricing_review", "Review model pricing", eventDescription(event, "Market pricing or margin signal needs review."), "avoid following unsustainable prices without quality and cache evidence", signals))
		}
	}
	return items
}

func (s *Service) actionsFromCostSnapshots(now time.Time, snapshots []*adminplusdomain.SupplierCostSnapshot) []*adminplusdomain.ActionRecommendation {
	items := make([]*adminplusdomain.ActionRecommendation, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot == nil || snapshot.SupplierID <= 0 || snapshot.ActualBalanceCents == nil || snapshot.BalanceDeltaCents == nil {
			continue
		}
		delta := *snapshot.BalanceDeltaCents
		if delta == 0 {
			continue
		}
		severity := adminplusdomain.ActionSeverityWarning
		if absInt64(delta) >= costBalanceDeltaCriticalCents {
			severity = adminplusdomain.ActionSeverityCritical
		}
		items = append(items, newAction(
			now,
			snapshot.SupplierID,
			nil,
			adminplusdomain.ActionTypeSupplierCostReconcileAdjustment,
			severity,
			"supplier_cost_balance_reconcile_anomaly",
			"Repair supplier balance ledger",
			"Supplier funding, entitlement, usage-cost ledger does not match the current balance snapshot.",
			"record a reviewed ledger adjustment before routing decisions rely on the corrected balance",
			costSnapshotSignals(snapshot),
		))
	}
	return items
}

func indexSuppliers(items []SupplierSignal) map[int64]SupplierSignal {
	out := make(map[int64]SupplierSignal, len(items))
	for _, item := range items {
		if item.SupplierID > 0 {
			out[item.SupplierID] = item
		}
	}
	return out
}

func findBestSwitchCandidate(items []SupplierSignal) *SupplierSignal {
	var best *SupplierSignal
	for i := range items {
		item := items[i]
		if !adminplusdomain.CanUseSupplierForSwitching(item.RuntimeStatus, item.BalanceCents) {
			continue
		}
		if item.HealthStatus != "" && item.HealthStatus != adminplusdomain.SupplierHealthStatusNormal {
			continue
		}
		if best == nil || item.EffectiveCostCents < best.EffectiveCostCents {
			cp := item
			best = &cp
		}
	}
	return best
}

func switchActionFromKanban(now time.Time, supplier SupplierSignal, supplierID int64, bestCandidate *SupplierSignal, severity adminplusdomain.ActionSeverity, reason string, title string, description string, impact string, signals []string) []*adminplusdomain.ActionRecommendation {
	if bestCandidate == nil || bestCandidate.SupplierID <= 0 || bestCandidate.SupplierID == supplierID {
		return nil
	}
	if supplier.RuntimeStatus != adminplusdomain.SupplierRuntimeStatusActive {
		return nil
	}
	if severity != adminplusdomain.ActionSeverityCritical {
		return nil
	}
	return []*adminplusdomain.ActionRecommendation{
		newAction(now, supplierID, &bestCandidate.SupplierID, adminplusdomain.ActionTypeSwitchSupplier, severity, reason, title, description, impact, append(normalizeSignals(signals), "candidate_available")),
	}
}

func supplierIDFromKanbanEvent(event *adminplusdomain.KanbanEvent) int64 {
	if event == nil {
		return 0
	}
	if event.SourceType == "supplier" && event.SourceID > 0 {
		return event.SourceID
	}
	if event.Payload != nil {
		return int64FromPayload(event.Payload, "supplier_id")
	}
	return 0
}

func int64FromPayload(payload map[string]any, key string) int64 {
	switch value := payload[key].(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		return int64(value)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func actionSeverityFromKanbanEvent(severity string) adminplusdomain.ActionSeverity {
	switch adminplusdomain.ActionSeverity(strings.ToLower(strings.TrimSpace(severity))) {
	case adminplusdomain.ActionSeverityCritical:
		return adminplusdomain.ActionSeverityCritical
	case adminplusdomain.ActionSeverityWarning:
		return adminplusdomain.ActionSeverityWarning
	default:
		return adminplusdomain.ActionSeverityInfo
	}
}

func kanbanEventSignals(event *adminplusdomain.KanbanEvent) []string {
	if event == nil {
		return nil
	}
	signals := []string{"kanban_event", "kanban_event_id=" + strconv.FormatInt(event.ID, 10)}
	if event.EventType != "" {
		signals = append(signals, "kanban_event_type="+event.EventType)
	}
	if event.Model != "" {
		signals = append(signals, "model="+event.Model)
	}
	if event.RelatedSnapshotType != "" {
		signals = append(signals, "snapshot_type="+event.RelatedSnapshotType)
	}
	return signals
}

func candidateEvaluationSignals(evaluation CandidateSignal) []string {
	signals := []string{
		"candidate_status=" + strings.TrimSpace(evaluation.CandidateStatus),
		"blocked_reason=" + strings.TrimSpace(evaluation.BlockedReason),
		"check_source=" + strings.TrimSpace(evaluation.CheckSource),
		"balance_status=" + strings.TrimSpace(evaluation.BalanceStatus),
		"key_capacity_status=" + strings.TrimSpace(evaluation.KeyCapacityStatus),
	}
	if evaluation.SupplierGroupID > 0 {
		signals = append(signals, "supplier_group_id="+strconv.FormatInt(evaluation.SupplierGroupID, 10))
	}
	if evaluation.LocalSub2APIAccountID > 0 {
		signals = append(signals, "local_sub2api_account_id="+strconv.FormatInt(evaluation.LocalSub2APIAccountID, 10))
	}
	if evaluation.EffectiveRateMultiplier > 0 {
		signals = append(signals, "effective_rate_multiplier="+strconv.FormatFloat(evaluation.EffectiveRateMultiplier, 'f', -1, 64))
	}
	return signals
}

func balanceEventSignals(event *adminplusdomain.BalanceEvent) []string {
	if event == nil {
		return nil
	}
	signals := []string{
		"balance_event_type=" + string(event.Type),
		"balance_cents=" + strconv.FormatInt(event.NewBalanceCents, 10),
		"switch_eligible=" + strconv.FormatBool(event.SwitchEligible),
	}
	if event.ID > 0 {
		signals = append(signals, "balance_event_id="+strconv.FormatInt(event.ID, 10))
	}
	if event.OldBalanceCents != nil {
		signals = append(signals, "old_balance_cents="+strconv.FormatInt(*event.OldBalanceCents, 10))
	}
	if event.LowBalanceThresholdCents > 0 {
		signals = append(signals, "low_balance_threshold_cents="+strconv.FormatInt(event.LowBalanceThresholdCents, 10))
	}
	return signals
}

func costSnapshotSignals(snapshot *adminplusdomain.SupplierCostSnapshot) []string {
	if snapshot == nil {
		return nil
	}
	signals := []string{
		"cost_snapshot_id=" + strconv.FormatInt(snapshot.ID, 10),
		"currency=" + strings.TrimSpace(snapshot.Currency),
		"completed_funding_amount_cents=" + strconv.FormatInt(snapshot.CompletedFundingAmountCents, 10),
		"completed_funding_cash_cents=" + strconv.FormatInt(snapshot.CompletedFundingCashCents, 10),
		"recharge_actual_payment_cents=" + strconv.FormatInt(snapshot.RechargeActualPaymentCents, 10),
		"entitlement_amount_cents=" + strconv.FormatInt(snapshot.EntitlementAmountCents, 10),
		"usage_cost_cents=" + strconv.FormatInt(snapshot.UsageCostCents, 10),
		"refund_amount_cents=" + strconv.FormatInt(snapshot.RefundAmountCents, 10),
		"adjustment_amount_cents=" + strconv.FormatInt(snapshot.AdjustmentAmountCents, 10),
		"expected_balance_cents=" + strconv.FormatInt(snapshot.ExpectedBalanceCents, 10),
	}
	if snapshot.ActualBalanceCents != nil {
		signals = append(signals, "actual_balance_cents="+strconv.FormatInt(*snapshot.ActualBalanceCents, 10))
	}
	if snapshot.BalanceDeltaCents != nil {
		signals = append(signals, "balance_delta_cents="+strconv.FormatInt(*snapshot.BalanceDeltaCents, 10))
	}
	return signals
}

func localGroupCapacitySignals(group LocalGroupCapacitySignal, threshold int64) []string {
	signals := []string{
		"local_group_id=" + strconv.FormatInt(group.LocalGroupID, 10),
		"total_accounts=" + strconv.FormatInt(group.TotalAccounts, 10),
		"schedulable_accounts=" + strconv.FormatInt(group.SchedulableAccounts, 10),
		"active_api_key_count=" + strconv.FormatInt(group.ActiveAPIKeyCount, 10),
		"low_capacity_threshold=" + strconv.FormatInt(threshold, 10),
		"best_candidate_available=" + strconv.FormatBool(group.BestCandidateLocalAccountID > 0),
	}
	if strings.TrimSpace(group.LocalGroupName) != "" {
		signals = append(signals, "local_group_name="+strings.TrimSpace(group.LocalGroupName))
	}
	if strings.TrimSpace(group.Platform) != "" {
		signals = append(signals, "platform="+strings.TrimSpace(group.Platform))
	}
	if group.RateMultiplier > 0 {
		signals = append(signals, "local_group_rate_multiplier="+strconv.FormatFloat(group.RateMultiplier, 'f', -1, 64))
	}
	if group.BestCandidateSupplierID > 0 {
		signals = append(signals, "candidate_supplier_id="+strconv.FormatInt(group.BestCandidateSupplierID, 10))
	}
	if group.BestCandidateSupplierGroupID > 0 {
		signals = append(signals, "candidate_supplier_group_id="+strconv.FormatInt(group.BestCandidateSupplierGroupID, 10))
	}
	if group.BestCandidateLocalAccountID > 0 {
		signals = append(signals, "candidate_local_account_id="+strconv.FormatInt(group.BestCandidateLocalAccountID, 10))
	}
	if group.BestCandidateRateMultiplier > 0 {
		signals = append(signals, "candidate_effective_rate_multiplier="+strconv.FormatFloat(group.BestCandidateRateMultiplier, 'f', -1, 64))
	}
	if strings.TrimSpace(group.BestCandidateCheckSource) != "" {
		signals = append(signals, "candidate_check_source="+strings.TrimSpace(group.BestCandidateCheckSource))
	}
	if strings.TrimSpace(group.BestCandidateSupplierName) != "" {
		signals = append(signals, "candidate_supplier_name="+strings.TrimSpace(group.BestCandidateSupplierName))
	}
	if strings.TrimSpace(group.BestCandidateSupplierGroupName) != "" {
		signals = append(signals, "candidate_supplier_group_name="+strings.TrimSpace(group.BestCandidateSupplierGroupName))
	}
	return signals
}

func localAccountScheduleDisableReason(reason string) bool {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "channel_monitor_failed", "channel_active_probe_failed":
		return true
	default:
		return false
	}
}

func localAccountScheduleSignals(account LocalAccountScheduleSignal) []string {
	signals := []string{
		"local_sub2api_account_id=" + strconv.FormatInt(account.LocalSub2APIAccountID, 10),
		"local_account_schedulable=" + strconv.FormatBool(account.LocalAccountSchedulable),
		"candidate_status=" + strings.TrimSpace(account.CandidateStatus),
		"blocked_reason=" + strings.TrimSpace(account.BlockedReason),
		"check_source=" + strings.TrimSpace(account.CheckSource),
		"balance_status=" + strings.TrimSpace(account.BalanceStatus),
		"key_capacity_status=" + strings.TrimSpace(account.KeyCapacityStatus),
		"channel_check_status=" + strings.TrimSpace(account.ChannelCheckStatus),
	}
	if account.SupplierID > 0 {
		signals = append(signals, "supplier_id="+strconv.FormatInt(account.SupplierID, 10))
	}
	if account.SupplierGroupID > 0 {
		signals = append(signals, "supplier_group_id="+strconv.FormatInt(account.SupplierGroupID, 10))
	}
	if strings.TrimSpace(account.LocalAccountName) != "" {
		signals = append(signals, "local_account_name="+strings.TrimSpace(account.LocalAccountName))
	}
	if strings.TrimSpace(account.SupplierName) != "" {
		signals = append(signals, "supplier_name="+strings.TrimSpace(account.SupplierName))
	}
	if strings.TrimSpace(account.SupplierGroupName) != "" {
		signals = append(signals, "supplier_group_name="+strings.TrimSpace(account.SupplierGroupName))
	}
	if account.EffectiveRateMultiplier > 0 {
		signals = append(signals, "effective_rate_multiplier="+strconv.FormatFloat(account.EffectiveRateMultiplier, 'f', -1, 64))
	}
	groupIDs := uniquePositiveInt64Strings(account.LocalGroupIDs)
	if len(groupIDs) > 0 {
		signals = append(signals, "local_group_ids="+strings.Join(groupIDs, ","))
	}
	groupNames := normalizedNonEmptyStrings(account.LocalGroupNames)
	if len(groupNames) > 0 {
		signals = append(signals, "local_group_names="+strings.Join(groupNames, ","))
	}
	return signals
}

func uniquePositiveInt64Strings(values []int64) []string {
	seen := make(map[int64]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, strconv.FormatInt(value, 10))
	}
	return out
}

func normalizedNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func supplierKeyCapacitySignals(supplier SupplierSignal) []string {
	signals := []string{
		"key_capacity_status=" + strings.TrimSpace(supplier.KeyCapacityStatus),
		"key_limit_policy=" + strings.TrimSpace(supplier.KeyLimitPolicy),
		"active_key_count=" + strconv.Itoa(supplier.ActiveKeyCount),
	}
	if supplier.KeyLimitValue > 0 {
		signals = append(signals, "key_limit_value="+strconv.Itoa(supplier.KeyLimitValue))
	}
	return signals
}

func eventDescription(event *adminplusdomain.KanbanEvent, fallback string) string {
	if event == nil {
		return fallback
	}
	parts := make([]string, 0, 3)
	if strings.TrimSpace(event.Title) != "" {
		parts = append(parts, strings.TrimSpace(event.Title))
	}
	if strings.TrimSpace(event.Description) != "" {
		parts = append(parts, strings.TrimSpace(event.Description))
	}
	if strings.TrimSpace(event.Recommendation) != "" {
		parts = append(parts, strings.TrimSpace(event.Recommendation))
	}
	if len(parts) == 0 {
		return fallback
	}
	return strings.Join(parts, " ")
}

func newAction(now time.Time, supplierID int64, targetSupplierID *int64, actionType adminplusdomain.ActionType, severity adminplusdomain.ActionSeverity, reason string, title string, description string, impact string, signals []string) *adminplusdomain.ActionRecommendation {
	return &adminplusdomain.ActionRecommendation{
		SupplierID:       supplierID,
		TargetSupplierID: cloneInt64(targetSupplierID),
		Type:             actionType,
		Severity:         severity,
		Status:           adminplusdomain.ActionStatusOpen,
		ReasonCode:       reason,
		Title:            title,
		Description:      description,
		ExpectedImpact:   impact,
		RequiresApproval: true,
		Signals:          normalizeSignals(signals),
		CreatedAt:        now,
	}
}

func (s *Service) executionFromRecommendation(ctx context.Context, action *adminplusdomain.ActionRecommendation, in ExecuteInput, now time.Time) *adminplusdomain.ActionExecution {
	execution := &adminplusdomain.ActionExecution{
		RecommendationID:    action.ID,
		ActionType:          action.Type,
		SupplierID:          action.SupplierID,
		TargetSupplierID:    cloneInt64(action.TargetSupplierID),
		RequestPayload:      clonePayload(in.RequestPayload),
		OperatorUserID:      in.OperatorUserID,
		SchedulerRunID:      strings.TrimSpace(in.SchedulerRunID),
		SchedulerStepID:     positiveInt64(in.SchedulerStepID),
		IdempotencyKeyHash:  strings.TrimSpace(in.IdempotencyKeyHash),
		IdempotencyReplayed: in.IdempotencyReplayed,
		BeforeSnapshot:      clonePayload(in.BeforeSnapshot),
		AfterSnapshot:       clonePayload(in.AfterSnapshot),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	switch action.Type {
	case adminplusdomain.ActionTypeInvestigateProfit, adminplusdomain.ActionTypeReviewCredential:
		execution.Status = adminplusdomain.ActionExecutionStatusSucceeded
		execution.ResponsePayload = map[string]any{
			"mode":        "manual_workflow_recorded",
			"action_type": string(action.Type),
			"reason_code": action.ReasonCode,
		}
	case adminplusdomain.ActionTypeRoutingRefill:
		markUnsupportedExecution(execution, "routing_refill_requires_sub2api_refill_tool")
	case adminplusdomain.ActionTypeLocalAccountScheduleDisable:
		markUnsupportedExecution(execution, "local_account_schedule_disable_requires_local_account_ops_tool")
	case adminplusdomain.ActionTypePauseSupplier:
		s.executeSupplierStatusUpdate(ctx, execution, adminplusdomain.SupplierRuntimeStatusDisabled, adminplusdomain.SupplierHealthStatusPaused)
	case adminplusdomain.ActionTypeDegradeSupplier:
		s.executeSupplierStatusUpdate(ctx, execution, adminplusdomain.SupplierRuntimeStatusMonitorOnly, adminplusdomain.SupplierHealthStatusNormal)
	default:
		markUnsupportedExecution(execution, action.ReasonCode)
	}
	return execution
}

func (s *Service) executeSupplierStatusUpdate(ctx context.Context, execution *adminplusdomain.ActionExecution, runtimeStatus adminplusdomain.SupplierRuntimeStatus, healthStatus adminplusdomain.SupplierHealthStatus) {
	if execution.SupplierID <= 0 {
		execution.Status = adminplusdomain.ActionExecutionStatusFailed
		execution.ErrorMessage = "supplier_id is required for supplier status action"
		execution.ResponsePayload = map[string]any{
			"mode":        "supplier_status_update",
			"action_type": string(execution.ActionType),
			"error":       "supplier_id_required",
		}
		return
	}
	if s == nil || s.supplierUpdater == nil {
		markUnsupportedExecution(execution, "supplier_status_updater_missing")
		return
	}
	updated, err := s.supplierUpdater.UpdateStatus(ctx, execution.SupplierID, suppliersapp.UpdateSupplierStatusInput{
		RuntimeStatus: runtimeStatus,
		HealthStatus:  healthStatus,
	})
	if err != nil {
		execution.Status = adminplusdomain.ActionExecutionStatusFailed
		execution.ErrorMessage = err.Error()
		execution.ResponsePayload = map[string]any{
			"mode":                  "supplier_status_update",
			"action_type":           string(execution.ActionType),
			"target_runtime_status": string(runtimeStatus),
			"target_health_status":  string(healthStatus),
		}
		return
	}
	execution.Status = adminplusdomain.ActionExecutionStatusSucceeded
	execution.ResponsePayload = map[string]any{
		"mode":                  "supplier_status_update",
		"action_type":           string(execution.ActionType),
		"supplier_id":           updated.ID,
		"runtime_status":        string(updated.RuntimeStatus),
		"health_status":         string(updated.HealthStatus),
		"target_runtime_status": string(runtimeStatus),
		"target_health_status":  string(healthStatus),
	}
}

func markUnsupportedExecution(execution *adminplusdomain.ActionExecution, reasonCode string) {
	execution.Status = adminplusdomain.ActionExecutionStatusUnsupported
	execution.ErrorMessage = "automatic execution for this action type is not enabled; keep manual approval and execute in the owning system"
	execution.ResponsePayload = map[string]any{
		"mode":        "unsupported_without_routing_executor",
		"action_type": string(execution.ActionType),
		"reason_code": reasonCode,
	}
}

func clonePayload(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func normalizeSignals(signals []string) []string {
	out := make([]string, 0, len(signals))
	for _, signal := range signals {
		v := strings.TrimSpace(signal)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func cloneInt64(in *int64) *int64 {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}

func severityRank(severity adminplusdomain.ActionSeverity) int {
	switch severity {
	case adminplusdomain.ActionSeverityCritical:
		return 3
	case adminplusdomain.ActionSeverityWarning:
		return 2
	case adminplusdomain.ActionSeverityInfo:
		return 1
	default:
		return 0
	}
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

func badRequest(reason string, message string) error {
	return infraerrors.New(http.StatusBadRequest, reason, message)
}

func internalError(message string) error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", message)
}
