package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	channelchecksapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/channelchecks"
	costsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/costs"
	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	healthapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/health"
	purityapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/purity"
	ratesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/rates"
	sessionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sessions"
	suppliergroupsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliergroups"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	usagecostsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/usagecosts"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	defaultWindowMinutes = 5
	dailyBucketLayout    = "20060102"
	windowBucketLayout   = "200601021504"

	actionDirectSync    = "direct_sync"
	actionExtensionTask = "extension_task"
	actionCompatTask    = "compat_task"
)

type RunInput struct {
	Mode          string
	SupplierID    int64
	TaskTypes     []adminplusdomain.ExtensionTaskType
	Request       map[string]any
	WindowMinutes int
	DryRun        bool
	Now           time.Time
}

type CostBackfillInput struct {
	Mode                           string
	SupplierID                     int64
	StartedAt                      *time.Time
	EndedAt                        *time.Time
	IncludeFundingTransactions     bool
	IncludeEntitlementTransactions bool
	IncludeUsageCostLines          bool
	IncludeBalanceSnapshot         bool
	LowBalanceThresholdCents       int64
	Now                            time.Time
}

type Service struct {
	repo             Repository
	supplierService  *suppliersapp.Service
	extensionService *extensionapp.Service
	groupSyncer      GroupSyncer
	rateSyncer       RateSyncer
	balanceSyncer    BalanceSyncer
	healthSyncer     HealthSyncer
	usageCostSyncer  UsageCostSyncer
	costSyncer       CostSyncer
	channelChecker   ChannelChecker
	purityChecker    PurityChecker
	sessionRefresher SessionRefresher
	runObserver      RunStatusObserver
	now              func() time.Time
	recentRunsMu     sync.Mutex
	recentRuns       []adminplusdomain.SchedulerRunSummary
}

type Repository interface {
	SaveRun(ctx context.Context, run adminplusdomain.SchedulerRunSummary, steps []adminplusdomain.ScheduledTask) error
	ListRuns(ctx context.Context, limit int, offset int, taskType string) ([]adminplusdomain.SchedulerRunSummary, error)
	GetRun(ctx context.Context, runID string) (*adminplusdomain.SchedulerRunSummary, error)
	ListSteps(ctx context.Context, runID string, limit int, offset int) ([]adminplusdomain.SchedulerStepRecord, error)
	ListAttempts(ctx context.Context, runID string, limit int) ([]adminplusdomain.SchedulerAttemptRecord, error)
	DeleteRun(ctx context.Context, runID string) (*adminplusdomain.SchedulerCleanupResult, error)
	DeleteRuns(ctx context.Context, taskType string) (*adminplusdomain.SchedulerCleanupResult, error)
	RetryStep(ctx context.Context, stepID int64, retryAt time.Time) (*adminplusdomain.SchedulerStepRecord, error)
	CancelStep(ctx context.Context, stepID int64, cancelledAt time.Time) (*adminplusdomain.SchedulerStepRecord, error)
	CancelRun(ctx context.Context, runID string, cancelledAt time.Time) (*adminplusdomain.SchedulerRunSummary, error)
	RetryFailedSteps(ctx context.Context, runID string, retryAt time.Time) (int, error)
	SavePlans(ctx context.Context, plans []adminplusdomain.SchedulerPlan) error
	ListPlans(ctx context.Context) ([]adminplusdomain.SchedulerPlan, error)
	UpdatePlanStatus(ctx context.Context, planID, status string) (*adminplusdomain.SchedulerPlan, error)
	UpdatePlanConfig(ctx context.Context, planID string, config adminplusdomain.SchedulerPlanConfig) (*adminplusdomain.SchedulerPlan, error)
	PlanStats(ctx context.Context, plans []adminplusdomain.SchedulerPlan) (map[string]adminplusdomain.SchedulerPlanStats, error)
	SupplierLatestSteps(ctx context.Context) (map[int64]map[adminplusdomain.ExtensionTaskType]adminplusdomain.SchedulerStepRecord, error)
	ClaimDuePlan(ctx context.Context, now time.Time) (*adminplusdomain.SchedulerPlan, error)
	SaveSettings(ctx context.Context, settings adminplusdomain.SchedulerSettings) error
	LoadSettings(ctx context.Context) (*adminplusdomain.SchedulerSettings, error)
	UpsertActions(ctx context.Context, actions []adminplusdomain.SchedulerAction) error
	ListActions(ctx context.Context) ([]adminplusdomain.SchedulerAction, error)
	UpdateActionStatus(ctx context.Context, actionID, status string, resolvedAt *time.Time) (*adminplusdomain.SchedulerAction, error)
	StepStats(ctx context.Context) (running int, queued int, failed int, err error)
	ClaimStep(ctx context.Context, workerID string, lease time.Duration) (*adminplusdomain.SchedulerStepRecord, error)
	CompleteStep(ctx context.Context, stepID int64, status string, resultCount int, reason string, result map[string]any, finishedAt time.Time) error
	RefreshRunStatus(ctx context.Context, runID string, finishedAt time.Time) error
}

type GroupSyncer interface {
	Sync(ctx context.Context, supplierID int64) (*suppliergroupsapp.SyncResult, error)
}

type RateSyncer interface {
	SyncFromSession(ctx context.Context, in ratesapp.SyncFromSessionInput) (*ratesapp.SyncFromSessionResult, error)
}

type BalanceSyncer interface {
	SyncFromSession(ctx context.Context, in balancesapp.SyncFromSessionInput) (*balancesapp.SyncFromSessionResult, error)
}

type HealthSyncer interface {
	SyncFromSession(ctx context.Context, in healthapp.SyncFromSessionInput) (*healthapp.SyncFromSessionResult, error)
}

type UsageCostSyncer interface {
	SyncFromSession(ctx context.Context, in usagecostsapp.SyncFromSessionInput) (*usagecostsapp.SyncFromSessionResult, error)
}

type CostSyncer interface {
	Sync(ctx context.Context, in costsapp.SyncInput) (*costsapp.SyncResult, error)
}

type ChannelChecker interface {
	Check(ctx context.Context, in channelchecksapp.CheckInput) (*channelchecksapp.CheckResult, error)
}

type PurityChecker interface {
	RunAccountCheck(ctx context.Context, in purityapp.AccountCheckInput) (*purityapp.PublicReport, error)
}

type SessionRefresher interface {
	DecryptedProbeInput(ctx context.Context, supplierID int64) (ports.SessionProbeInput, error)
	Login(ctx context.Context, in sessionsapp.LoginInput) (*sessionsapp.LoginResult, error)
}

type RunStatusObserver interface {
	OnSchedulerRunStatusRefreshed(ctx context.Context, runID string) error
}

type stepFailureReason struct {
	Stage        string            `json:"stage"`
	Code         string            `json:"code,omitempty"`
	Message      string            `json:"message,omitempty"`
	Action       string            `json:"action,omitempty"`
	Outcome      string            `json:"outcome,omitempty"`
	LoginCode    string            `json:"login_code,omitempty"`
	LoginMessage string            `json:"login_message,omitempty"`
	Suggestion   string            `json:"suggestion,omitempty"`
	RawError     string            `json:"raw_error,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type stepFailureInput struct {
	TaskType adminplusdomain.ExtensionTaskType
	Stage    string
	Action   string
	Err      error
	Metadata map[string]string
}

func ProvideService(
	repo Repository,
	supplierService *suppliersapp.Service,
	extensionService *extensionapp.Service,
	groupSyncer GroupSyncer,
	rateSyncer RateSyncer,
	balanceSyncer BalanceSyncer,
	healthSyncer HealthSyncer,
	usageCostSyncer UsageCostSyncer,
	costSyncer CostSyncer,
	channelChecker ChannelChecker,
	purityChecker PurityChecker,
	sessionRefresher SessionRefresher,
) *Service {
	return NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, groupSyncer, rateSyncer, balanceSyncer, healthSyncer, usageCostSyncer, channelChecker).
		WithCostSyncer(costSyncer).
		WithPurityChecker(purityChecker).
		WithSessionRefresher(sessionRefresher)
}

func NewService(supplierService *suppliersapp.Service, extensionService *extensionapp.Service) *Service {
	return &Service{
		supplierService:  supplierService,
		extensionService: extensionService,
		now:              time.Now,
	}
}

func NewServiceWithDependencies(
	supplierService *suppliersapp.Service,
	extensionService *extensionapp.Service,
	groupSyncer GroupSyncer,
	rateSyncer RateSyncer,
	balanceSyncer BalanceSyncer,
	healthSyncer HealthSyncer,
	usageCostSyncer UsageCostSyncer,
	channelChecker ChannelChecker,
) *Service {
	service := NewService(supplierService, extensionService)
	service.groupSyncer = groupSyncer
	service.rateSyncer = rateSyncer
	service.balanceSyncer = balanceSyncer
	service.healthSyncer = healthSyncer
	service.usageCostSyncer = usageCostSyncer
	service.channelChecker = channelChecker
	return service
}

func (s *Service) WithSessionRefresher(refresher SessionRefresher) *Service {
	if s != nil {
		s.sessionRefresher = refresher
	}
	return s
}

func (s *Service) WithCostSyncer(syncer CostSyncer) *Service {
	if s != nil {
		s.costSyncer = syncer
	}
	return s
}

func (s *Service) WithPurityChecker(checker PurityChecker) *Service {
	if s != nil {
		s.purityChecker = checker
	}
	return s
}

func (s *Service) WithRunStatusObserver(observer RunStatusObserver) *Service {
	if s != nil {
		s.runObserver = observer
	}
	return s
}

func NewServiceWithDependenciesAndRepository(
	repo Repository,
	supplierService *suppliersapp.Service,
	extensionService *extensionapp.Service,
	groupSyncer GroupSyncer,
	rateSyncer RateSyncer,
	balanceSyncer BalanceSyncer,
	healthSyncer HealthSyncer,
	usageCostSyncer UsageCostSyncer,
	channelChecker ChannelChecker,
) *Service {
	service := NewServiceWithDependencies(supplierService, extensionService, groupSyncer, rateSyncer, balanceSyncer, healthSyncer, usageCostSyncer, channelChecker)
	service.repo = repo
	return service
}

func (s *Service) EnqueueCostHistoryBackfill(ctx context.Context, in CostBackfillInput) (*adminplusdomain.SchedulerRunSummary, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.New(http.StatusConflict, "ADMIN_PLUS_SCHEDULER_REPOSITORY_REQUIRED", "cost history backfill requires persistent scheduler repository")
	}
	if s.costSyncer == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_COST_SYNCER_NOT_CONFIGURED", "supplier cost syncer is not configured")
	}
	if in.LowBalanceThresholdCents < 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_COST_BACKFILL_THRESHOLD_INVALID", "low balance threshold must be non-negative")
	}
	if !in.IncludeFundingTransactions && !in.IncludeEntitlementTransactions && !in.IncludeUsageCostLines && !in.IncludeBalanceSnapshot {
		in.IncludeFundingTransactions = true
		in.IncludeEntitlementTransactions = true
		in.IncludeUsageCostLines = true
		in.IncludeBalanceSnapshot = true
	}
	if in.Now.IsZero() {
		in.Now = s.now().UTC()
	} else {
		in.Now = in.Now.UTC()
	}
	mode := strings.TrimSpace(in.Mode)
	if mode == "" {
		mode = "manual:cost-history-backfill"
	}
	suppliers, err := s.supplierService.List(ctx, suppliersapp.SupplierFilter{})
	if err != nil {
		return nil, err
	}
	runID := strings.ReplaceAll(fmt.Sprintf("%s-%d", mode, in.Now.UnixNano()), ":", "-")
	request := costBackfillRequestSnapshot(in)
	items := make([]adminplusdomain.ScheduledTask, 0, len(suppliers))
	skipped := 0
	for _, supplier := range suppliers {
		if supplier == nil {
			continue
		}
		if in.SupplierID > 0 && supplier.ID != in.SupplierID {
			continue
		}
		item := adminplusdomain.ScheduledTask{
			SupplierID:   supplier.ID,
			SupplierName: supplier.Name,
			TaskType:     adminplusdomain.ExtensionTaskTypeReconcileCosts,
			Action:       actionDirectSync,
			ScheduleKey:  fmt.Sprintf("scheduler:%s:supplier:%d:%s", adminplusdomain.ExtensionTaskTypeReconcileCosts, supplier.ID, in.Now.Format(windowBucketLayout)),
			Request:      cloneMap(request),
		}
		if reason := ineligibleReason(supplier, item.TaskType); reason != "" {
			item.Reason = reason
			skipped++
		}
		items = append(items, item)
	}
	requestedAt := in.Now.UTC()
	summary := adminplusdomain.SchedulerRunSummary{
		ID:              runID,
		LegacyRunID:     fmt.Sprintf("%s:%d", mode, in.Now.UnixNano()),
		TriggerType:     mode,
		TaskType:        schedulerTaskTypeLabel(adminplusdomain.ExtensionTaskTypeReconcileCosts),
		Status:          "queued",
		RequestedAt:     requestedAt,
		SupplierCount:   schedulerRunSupplierCount(items),
		TotalSteps:      len(items),
		SkippedSteps:    skipped,
		RequestSnapshot: cloneMap(request),
	}
	if len(items) == 0 {
		summary.Status = "skipped"
		summary.FinishedAt = &requestedAt
	}
	if err := s.repo.SaveRun(ctx, summary, items); err != nil {
		return nil, err
	}
	s.recentRunsMu.Lock()
	s.recentRuns = append(s.recentRuns, summary)
	if len(s.recentRuns) > 100 {
		s.recentRuns = append([]adminplusdomain.SchedulerRunSummary{}, s.recentRuns[len(s.recentRuns)-100:]...)
	}
	s.recentRunsMu.Unlock()
	return &summary, nil
}

func (s *Service) Run(ctx context.Context, in RunInput) (*adminplusdomain.SchedulerRun, error) {
	if in.Now.IsZero() {
		in.Now = s.now().UTC()
	} else {
		in.Now = in.Now.UTC()
	}
	mode := strings.TrimSpace(in.Mode)
	if mode == "" {
		mode = "manual"
	}
	windowMinutes := in.WindowMinutes
	if windowMinutes <= 0 {
		windowMinutes = defaultWindowMinutes
	}
	taskTypes := normalizeTaskTypes(in.TaskTypes)
	run := &adminplusdomain.SchedulerRun{
		RunID:       fmt.Sprintf("%s-%d", mode, in.Now.UnixNano()),
		Mode:        mode,
		DryRun:      in.DryRun,
		RequestedAt: in.Now,
		TaskTypes:   taskTypes,
		Items:       make([]adminplusdomain.ScheduledTask, 0),
	}

	suppliers, err := s.supplierService.List(ctx, suppliersapp.SupplierFilter{})
	if err != nil {
		return nil, err
	}
	for _, supplier := range suppliers {
		if supplier == nil {
			continue
		}
		if in.SupplierID > 0 && supplier.ID != in.SupplierID {
			continue
		}
		for _, taskType := range taskTypes {
			item := s.scheduleSupplierTask(ctx, supplier, taskType, mode, windowMinutes, in.Now, in.DryRun, in.Request)
			if item.Created {
				run.CreatedCount++
			} else if item.Synced {
				run.EligibleCount++
			} else if item.Reason == "" {
				run.EligibleCount++
			} else {
				run.SkippedCount++
			}
			run.Items = append(run.Items, item)
		}
	}
	if err := s.rememberRun(ctx, run); err != nil {
		return nil, err
	}
	return run, nil
}

func (s *Service) EnqueueRun(ctx context.Context, in RunInput) (*adminplusdomain.SchedulerRunSummary, error) {
	if in.Now.IsZero() {
		in.Now = s.now().UTC()
	} else {
		in.Now = in.Now.UTC()
	}
	mode := strings.TrimSpace(in.Mode)
	if mode == "" {
		mode = "manual"
	}
	windowMinutes := in.WindowMinutes
	if windowMinutes <= 0 {
		windowMinutes = defaultWindowMinutes
	}
	taskTypes := normalizeTaskTypes(in.TaskTypes)
	run := &adminplusdomain.SchedulerRun{
		RunID:       fmt.Sprintf("%s-%d", mode, in.Now.UnixNano()),
		Mode:        mode,
		DryRun:      in.DryRun,
		RequestedAt: in.Now,
		TaskTypes:   taskTypes,
		Items:       make([]adminplusdomain.ScheduledTask, 0),
	}

	suppliers, err := s.supplierService.List(ctx, suppliersapp.SupplierFilter{})
	if err != nil {
		return nil, err
	}
	for _, supplier := range suppliers {
		if supplier == nil {
			continue
		}
		if in.SupplierID > 0 && supplier.ID != in.SupplierID {
			continue
		}
		for _, taskType := range taskTypes {
			item := s.planSupplierTask(supplier, taskType, windowMinutes, in.Now)
			item.Request = cloneMap(in.Request)
			if item.Reason != "" {
				run.SkippedCount++
			} else {
				run.EligibleCount++
			}
			run.Items = append(run.Items, item)
		}
	}
	requestedAt := in.Now.UTC()
	summary := adminplusdomain.SchedulerRunSummary{
		ID:              strings.ReplaceAll(run.RunID, ":", "-"),
		LegacyRunID:     run.RunID,
		TriggerType:     mode,
		TaskType:        schedulerRunTaskLabel(taskTypes),
		Status:          "queued",
		RequestedAt:     requestedAt,
		SupplierCount:   schedulerRunSupplierCount(run.Items),
		TotalSteps:      len(run.Items),
		SucceededSteps:  0,
		FailedSteps:     0,
		SkippedSteps:    run.SkippedCount,
		RequestSnapshot: cloneMap(in.Request),
	}
	if len(run.Items) == 0 {
		summary.Status = "skipped"
		finishedAt := requestedAt
		summary.FinishedAt = &finishedAt
	}
	if s.repo != nil {
		if err := s.repo.SaveRun(ctx, summary, run.Items); err != nil {
			return nil, err
		}
	}
	s.recentRunsMu.Lock()
	s.recentRuns = append(s.recentRuns, summary)
	if len(s.recentRuns) > 100 {
		s.recentRuns = append([]adminplusdomain.SchedulerRunSummary{}, s.recentRuns[len(s.recentRuns)-100:]...)
	}
	s.recentRunsMu.Unlock()
	return &summary, nil
}

func (s *Service) Status() adminplusdomain.SchedulerStatus {
	return adminplusdomain.SchedulerStatus{
		Enabled:         schedulerEnabled(),
		IntervalSeconds: int64(schedulerInterval().Seconds()),
		Queue:           "admin_plus_extension_tasks",
	}
}

func (s *Service) CenterStatus(ctx context.Context) adminplusdomain.SchedulerCenterStatus {
	settings := s.Settings(ctx)
	runs := s.ListRuns(ctx, 20, 0, "")
	plans := s.ListPlans(ctx)
	now := s.now().UTC()
	var lastRunAt *time.Time
	var nextRunAt *time.Time
	failedSteps := 0
	runningSteps := 0
	queuedSteps := 0
	overduePlans := 0
	for idx := range runs {
		run := runs[idx]
		if lastRunAt == nil || run.RequestedAt.After(*lastRunAt) {
			value := run.RequestedAt
			lastRunAt = &value
		}
		failedSteps += run.FailedSteps
	}
	for idx := range plans {
		plan := plans[idx]
		if plan.Status != "enabled" || plan.NextRunAt == nil {
			continue
		}
		candidate := *plan.NextRunAt
		if !candidate.After(now) {
			overduePlans++
			candidate = now
		}
		if nextRunAt == nil || candidate.Before(*nextRunAt) {
			value := candidate
			nextRunAt = &value
		}
	}
	if s.repo != nil {
		if running, queued, failed, err := s.repo.StepStats(ctx); err == nil {
			runningSteps = running
			queuedSteps = queued
			failedSteps = failed
		}
	}
	if nextRunAt == nil {
		value := now.Add(schedulerInterval())
		nextRunAt = &value
	}
	workerStatus := "running"
	if !schedulerWorkerEnabled() {
		workerStatus = "down"
	} else if s.repo == nil {
		workerStatus = "degraded"
	}
	return adminplusdomain.SchedulerCenterStatus{
		Enabled:         settings.Enabled,
		WorkerStatus:    workerStatus,
		Queue:           "admin_plus_scheduler_runs",
		IntervalSeconds: int64(schedulerInterval().Seconds()),
		RunningSteps:    runningSteps,
		QueuedSteps:     queuedSteps,
		FailedSteps:     failedSteps,
		OverduePlans:    overduePlans,
		OpenActions:     len(s.ListActions(ctx)),
		LastRunAt:       lastRunAt,
		NextRunAt:       nextRunAt,
	}
}

func (s *Service) ListPlans(ctx context.Context) []adminplusdomain.SchedulerPlan {
	plans := s.defaultPlans()
	if s.repo != nil {
		_ = s.repo.SavePlans(ctx, plans)
		if stored, err := s.repo.ListPlans(ctx); err == nil && len(stored) > 0 {
			return s.attachPlanStats(ctx, stored)
		}
	}
	return s.attachPlanStats(ctx, plans)
}

func (s *Service) UpdatePlanStatus(ctx context.Context, planID, status string) (*adminplusdomain.SchedulerPlan, error) {
	planID = strings.TrimSpace(planID)
	status = strings.TrimSpace(status)
	if planID == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_PLAN_ID_REQUIRED", "scheduler plan id is required")
	}
	if status != "enabled" && status != "paused" && status != "disabled" {
		return nil, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_PLAN_STATUS_INVALID", "scheduler plan status is invalid")
	}
	if s.repo == nil {
		for _, plan := range s.defaultPlans() {
			if plan.ID == planID {
				plan.Status = status
				return &plan, nil
			}
		}
		return nil, infraerrors.New(http.StatusNotFound, "ADMIN_PLUS_SCHEDULER_PLAN_NOT_FOUND", "scheduler plan not found")
	}
	_ = s.repo.SavePlans(ctx, s.defaultPlans())
	plan, err := s.repo.UpdatePlanStatus(ctx, planID, status)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "ADMIN_PLUS_SCHEDULER_PLAN_NOT_FOUND", "scheduler plan not found")
	}
	return plan, err
}

func (s *Service) UpdatePlanConfig(ctx context.Context, planID string, config adminplusdomain.SchedulerPlanConfig) (*adminplusdomain.SchedulerPlan, error) {
	planID = strings.TrimSpace(planID)
	if planID == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_PLAN_ID_REQUIRED", "scheduler plan id is required")
	}
	normalized, err := normalizePlanConfig(config)
	if err != nil {
		return nil, err
	}
	if s.repo == nil {
		for _, plan := range s.defaultPlans() {
			if plan.ID != planID {
				continue
			}
			applyPlanConfig(&plan, normalized, s.now().UTC())
			return &plan, nil
		}
		return nil, infraerrors.New(http.StatusNotFound, "ADMIN_PLUS_SCHEDULER_PLAN_NOT_FOUND", "scheduler plan not found")
	}
	_ = s.repo.SavePlans(ctx, s.defaultPlans())
	plan, err := s.repo.UpdatePlanConfig(ctx, planID, normalized)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "ADMIN_PLUS_SCHEDULER_PLAN_NOT_FOUND", "scheduler plan not found")
	}
	if err != nil {
		return nil, err
	}
	enriched := s.attachPlanStats(ctx, []adminplusdomain.SchedulerPlan{*plan})
	return &enriched[0], nil
}

func (s *Service) ListRuns(ctx context.Context, limit int, offset int, taskType string) []adminplusdomain.SchedulerRunSummary {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	taskType = strings.TrimSpace(taskType)
	if s.repo != nil {
		runs, err := s.repo.ListRuns(ctx, limit, offset, taskType)
		if err == nil {
			return runs
		}
	}
	s.recentRunsMu.Lock()
	defer s.recentRunsMu.Unlock()
	if len(s.recentRuns) == 0 {
		return []adminplusdomain.SchedulerRunSummary{}
	}
	if offset >= len(s.recentRuns) {
		return []adminplusdomain.SchedulerRunSummary{}
	}
	out := make([]adminplusdomain.SchedulerRunSummary, 0, limit)
	skipped := 0
	for i := len(s.recentRuns) - 1; i >= 0 && len(out) < limit; i-- {
		if taskType != "" && s.recentRuns[i].TaskType != taskType {
			continue
		}
		if skipped < offset {
			skipped++
			continue
		}
		out = append(out, s.recentRuns[i])
	}
	return out
}

func (s *Service) GetRunDetail(ctx context.Context, runID string) (*adminplusdomain.SchedulerRunDetail, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_RUN_ID_REQUIRED", "scheduler run id is required")
	}
	if s.repo != nil {
		run, err := s.repo.GetRun(ctx, runID)
		if err == sql.ErrNoRows {
			return nil, infraerrors.New(http.StatusNotFound, "ADMIN_PLUS_SCHEDULER_RUN_NOT_FOUND", "scheduler run not found")
		}
		if err != nil {
			return nil, err
		}
		steps, err := s.repo.ListSteps(ctx, runID, 100, 0)
		if err != nil {
			return nil, err
		}
		attempts, err := s.repo.ListAttempts(ctx, runID, 1000)
		if err != nil {
			return nil, err
		}
		attachStepAttempts(steps, attempts)
		return &adminplusdomain.SchedulerRunDetail{Run: *run, Steps: steps}, nil
	}
	for _, run := range s.ListRuns(ctx, 100, 0, "") {
		if run.ID == runID {
			return &adminplusdomain.SchedulerRunDetail{Run: run, Steps: []adminplusdomain.SchedulerStepRecord{}}, nil
		}
	}
	return nil, infraerrors.New(http.StatusNotFound, "ADMIN_PLUS_SCHEDULER_RUN_NOT_FOUND", "scheduler run not found")
}

func (s *Service) ListSteps(ctx context.Context, runID string, limit int, offset int) ([]adminplusdomain.SchedulerStepRecord, error) {
	if s.repo == nil {
		return []adminplusdomain.SchedulerStepRecord{}, nil
	}
	runID = strings.TrimSpace(runID)
	steps, err := s.repo.ListSteps(ctx, runID, limit, offset)
	if err != nil || runID == "" || len(steps) == 0 {
		return steps, err
	}
	attempts, err := s.repo.ListAttempts(ctx, runID, 2000)
	if err != nil {
		return nil, err
	}
	attachStepAttempts(steps, attempts)
	return steps, nil
}

func (s *Service) DeleteRun(ctx context.Context, runID string) (*adminplusdomain.SchedulerCleanupResult, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_RUN_ID_REQUIRED", "scheduler run id is required")
	}
	if s.repo == nil {
		return nil, infraerrors.New(http.StatusConflict, "ADMIN_PLUS_SCHEDULER_REPOSITORY_REQUIRED", "scheduler run cleanup requires persistent scheduler repository")
	}
	result, err := s.repo.DeleteRun(ctx, runID)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "ADMIN_PLUS_SCHEDULER_RUN_NOT_FOUND", "scheduler run not found")
	}
	return result, err
}

func (s *Service) DeleteRuns(ctx context.Context, taskType string) (*adminplusdomain.SchedulerCleanupResult, error) {
	if s.repo == nil {
		return nil, infraerrors.New(http.StatusConflict, "ADMIN_PLUS_SCHEDULER_REPOSITORY_REQUIRED", "scheduler run cleanup requires persistent scheduler repository")
	}
	return s.repo.DeleteRuns(ctx, strings.TrimSpace(taskType))
}

func attachStepAttempts(steps []adminplusdomain.SchedulerStepRecord, attempts []adminplusdomain.SchedulerAttemptRecord) {
	if len(steps) == 0 || len(attempts) == 0 {
		return
	}
	indexByStepID := make(map[int64]int, len(steps))
	for index := range steps {
		indexByStepID[steps[index].ID] = index
	}
	for _, attempt := range attempts {
		index, ok := indexByStepID[attempt.StepID]
		if !ok {
			continue
		}
		steps[index].OperationLogs = append(steps[index].OperationLogs, attempt)
	}
}

func (s *Service) RetryStep(ctx context.Context, stepID int64) (*adminplusdomain.SchedulerStepRecord, error) {
	if stepID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_STEP_ID_REQUIRED", "scheduler step id is required")
	}
	if s.repo == nil {
		return nil, infraerrors.New(http.StatusConflict, "ADMIN_PLUS_SCHEDULER_REPOSITORY_REQUIRED", "scheduler step retry requires persistent scheduler repository")
	}
	retryAt := s.now().UTC()
	step, err := s.repo.RetryStep(ctx, stepID, retryAt)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusConflict, "ADMIN_PLUS_SCHEDULER_STEP_NOT_RETRYABLE", "scheduler step is not retryable")
	}
	if err != nil {
		return nil, err
	}
	if err := s.repo.RefreshRunStatus(ctx, step.RunID, retryAt); err != nil {
		return nil, err
	}
	return step, nil
}

func (s *Service) CancelStep(ctx context.Context, stepID int64) (*adminplusdomain.SchedulerStepRecord, error) {
	if stepID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_STEP_ID_REQUIRED", "scheduler step id is required")
	}
	if s.repo == nil {
		return nil, infraerrors.New(http.StatusConflict, "ADMIN_PLUS_SCHEDULER_REPOSITORY_REQUIRED", "scheduler step cancel requires persistent scheduler repository")
	}
	cancelledAt := s.now().UTC()
	step, err := s.repo.CancelStep(ctx, stepID, cancelledAt)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusConflict, "ADMIN_PLUS_SCHEDULER_STEP_NOT_CANCELLABLE", "scheduler step is not cancellable")
	}
	if err != nil {
		return nil, err
	}
	if err := s.repo.RefreshRunStatus(ctx, step.RunID, cancelledAt); err != nil {
		return nil, err
	}
	return step, nil
}

func (s *Service) CancelRun(ctx context.Context, runID string) (*adminplusdomain.SchedulerRunSummary, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_RUN_ID_REQUIRED", "scheduler run id is required")
	}
	if s.repo == nil {
		return nil, infraerrors.New(http.StatusConflict, "ADMIN_PLUS_SCHEDULER_REPOSITORY_REQUIRED", "scheduler run cancel requires persistent scheduler repository")
	}
	run, err := s.repo.CancelRun(ctx, runID, s.now().UTC())
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusConflict, "ADMIN_PLUS_SCHEDULER_RUN_NOT_CANCELLABLE", "scheduler run is not cancellable")
	}
	return run, err
}

func (s *Service) RetryFailedSteps(ctx context.Context, runID string) (*adminplusdomain.SchedulerRunDetail, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_RUN_ID_REQUIRED", "scheduler run id is required")
	}
	if s.repo == nil {
		return nil, infraerrors.New(http.StatusConflict, "ADMIN_PLUS_SCHEDULER_REPOSITORY_REQUIRED", "scheduler retry failed requires persistent scheduler repository")
	}
	retryAt := s.now().UTC()
	affected, err := s.repo.RetryFailedSteps(ctx, runID, retryAt)
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, infraerrors.New(http.StatusConflict, "ADMIN_PLUS_SCHEDULER_RUN_NO_RETRYABLE_STEPS", "scheduler run has no retryable steps")
	}
	if err := s.repo.RefreshRunStatus(ctx, runID, retryAt); err != nil {
		return nil, err
	}
	return s.GetRunDetail(ctx, runID)
}

func (s *Service) ListSupplierStatuses(ctx context.Context) ([]adminplusdomain.SchedulerSupplierStatus, error) {
	suppliers, err := s.supplierService.List(ctx, suppliersapp.SupplierFilter{})
	if err != nil {
		return nil, err
	}
	latestSteps := map[int64]map[adminplusdomain.ExtensionTaskType]adminplusdomain.SchedulerStepRecord{}
	if s.repo != nil {
		if values, err := s.repo.SupplierLatestSteps(ctx); err == nil {
			latestSteps = values
		}
	}
	out := make([]adminplusdomain.SchedulerSupplierStatus, 0, len(suppliers))
	for _, supplier := range suppliers {
		if supplier == nil {
			continue
		}
		out = append(out, schedulerSupplierStatus(supplier, latestSteps[supplier.ID]))
	}
	return out, nil
}

func (s *Service) GetSupplierChecklist(ctx context.Context, supplierID int64) (*adminplusdomain.SchedulerSupplierChecklist, error) {
	if supplierID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_SUPPLIER_ID_REQUIRED", "scheduler supplier id is required")
	}
	supplier, err := s.supplierService.Get(ctx, supplierID)
	if err != nil {
		return nil, err
	}
	latestSteps := map[adminplusdomain.ExtensionTaskType]adminplusdomain.SchedulerStepRecord{}
	if s.repo != nil {
		if values, err := s.repo.SupplierLatestSteps(ctx); err == nil {
			latestSteps = values[supplier.ID]
		}
	}
	status := schedulerSupplierStatus(supplier, latestSteps)
	items := schedulerSupplierChecklistItems(supplier, status)
	return &adminplusdomain.SchedulerSupplierChecklist{
		SupplierID:        supplier.ID,
		SupplierName:      supplier.Name,
		SupplierType:      string(supplier.Type),
		CompletionPercent: schedulerChecklistCompletionPercent(items),
		RecommendedAction: status.RecommendedAction,
		Items:             items,
	}, nil
}

func (s *Service) ListActions(ctx context.Context) []adminplusdomain.SchedulerAction {
	generated := s.generateActions(ctx)
	if s.repo != nil {
		_ = s.repo.UpsertActions(ctx, generated)
		if stored, err := s.repo.ListActions(ctx); err == nil {
			return stored
		}
	}
	return generated
}

func (s *Service) ResolveAction(ctx context.Context, actionID, status string) (*adminplusdomain.SchedulerAction, error) {
	actionID = strings.TrimSpace(actionID)
	status = strings.TrimSpace(status)
	if actionID == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_ACTION_ID_REQUIRED", "scheduler action id is required")
	}
	if status != "resolved" && status != "ignored" && status != "investigating" {
		return nil, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_ACTION_STATUS_INVALID", "scheduler action status is invalid")
	}
	now := s.now().UTC()
	var resolvedAt *time.Time
	if status == "resolved" || status == "ignored" {
		resolvedAt = &now
	}
	if s.repo != nil {
		_ = s.repo.UpsertActions(ctx, s.generateActions(ctx))
		action, err := s.repo.UpdateActionStatus(ctx, actionID, status, resolvedAt)
		if err == sql.ErrNoRows {
			return nil, infraerrors.New(http.StatusNotFound, "ADMIN_PLUS_SCHEDULER_ACTION_NOT_FOUND", "scheduler action not found")
		}
		return action, err
	}
	for _, action := range s.generateActions(ctx) {
		if action.ID == actionID {
			action.Status = status
			action.UpdatedAt = now
			action.ResolvedAt = resolvedAt
			return &action, nil
		}
	}
	return nil, infraerrors.New(http.StatusNotFound, "ADMIN_PLUS_SCHEDULER_ACTION_NOT_FOUND", "scheduler action not found")
}

func (s *Service) generateActions(ctx context.Context) []adminplusdomain.SchedulerAction {
	suppliers, err := s.supplierService.List(ctx, suppliersapp.SupplierFilter{})
	if err != nil {
		return []adminplusdomain.SchedulerAction{}
	}
	now := s.now().UTC()
	actions := make([]adminplusdomain.SchedulerAction, 0)
	for _, supplier := range suppliers {
		if supplier == nil {
			continue
		}
		if supplier.HealthStatus == adminplusdomain.SupplierHealthStatusCredentialInvalid {
			actions = append(actions, schedulerAction(now, supplier, "critical", "supplier.session.refresh", "会话凭据失效", "供应商凭据失效，账单、余额和分组采集会失败。", "刷新会话"))
			continue
		}
		if supplier.RuntimeStatus == adminplusdomain.SupplierRuntimeStatusDisabled || supplier.HealthStatus == adminplusdomain.SupplierHealthStatusPaused {
			actions = append(actions, schedulerAction(now, supplier, "warning", "supplier.resume_or_review", "供应商未参与自动化", "供应商已停用或暂停，调度中心会跳过相关任务。", "检查供应商状态"))
			continue
		}
		if strings.TrimSpace(supplier.DashboardURL) == "" && strings.TrimSpace(supplier.APIBaseURL) == "" {
			actions = append(actions, schedulerAction(now, supplier, "critical", "supplier.configure_url", "缺少供应商地址", "缺少 Dashboard/API URL，Provider Adapter 无法执行采集。", "补充供应商地址"))
			continue
		}
		if supplier.BalanceCents <= 0 {
			actions = append(actions, schedulerAction(now, supplier, "warning", "supplier.recharge", "供应商余额不足", "余额不可用时不会进入自动切换候选。", "刷新余额或充值"))
		}
	}
	return actions
}

func (s *Service) Settings(ctx context.Context) adminplusdomain.SchedulerSettings {
	defaults := s.defaultSettings()
	if s.repo != nil {
		if stored, err := s.repo.LoadSettings(ctx); err == nil && stored != nil {
			return normalizeSettings(*stored, defaults)
		}
		_ = s.repo.SaveSettings(ctx, defaults)
	}
	return defaults
}

func (s *Service) UpdateSettings(ctx context.Context, settings adminplusdomain.SchedulerSettings) (adminplusdomain.SchedulerSettings, error) {
	normalized := normalizeSettings(settings, s.defaultSettings())
	if s.repo != nil {
		if err := s.repo.SaveSettings(ctx, normalized); err != nil {
			return normalized, err
		}
	}
	return normalized, nil
}

func (s *Service) defaultSettings() adminplusdomain.SchedulerSettings {
	return adminplusdomain.SchedulerSettings{
		Enabled:                       schedulerEnabled(),
		DefaultSupplierConcurrency:    1,
		ChannelChecksEnabled:          channelChecksSchedulerEnabled(),
		ChannelCheckDailyBudgetTokens: 0,
		FirstTokenSlowThresholdMS:     3000,
		TotalLatencySlowThresholdMS:   15000,
		DefaultEnabledTaskTypes: []string{
			"supplier.balance.sync",
			"supplier.groups.sync",
			"supplier.rates.sync",
			"supplier.funding_orders.sync",
			"supplier.redeem_orders.sync",
			"supplier.usage_costs.sync",
			"supplier.session.probe",
		},
		HighCostTaskTypes: []string{
			"supplier.channels.check",
			"supplier.purity.check",
			"local.sub2api.schedule.ensure",
			"local.sub2api.schedule.remove_invalid",
		},
	}
}

func normalizeSettings(settings, defaults adminplusdomain.SchedulerSettings) adminplusdomain.SchedulerSettings {
	if settings.DefaultSupplierConcurrency <= 0 {
		settings.DefaultSupplierConcurrency = defaults.DefaultSupplierConcurrency
	}
	if settings.FirstTokenSlowThresholdMS <= 0 {
		settings.FirstTokenSlowThresholdMS = defaults.FirstTokenSlowThresholdMS
	}
	if settings.TotalLatencySlowThresholdMS <= 0 {
		settings.TotalLatencySlowThresholdMS = defaults.TotalLatencySlowThresholdMS
	}
	if len(settings.DefaultEnabledTaskTypes) == 0 {
		settings.DefaultEnabledTaskTypes = defaults.DefaultEnabledTaskTypes
	}
	if len(settings.HighCostTaskTypes) == 0 {
		settings.HighCostTaskTypes = defaults.HighCostTaskTypes
	}
	return settings
}

func costBackfillRequestSnapshot(in CostBackfillInput) map[string]any {
	return map[string]any{
		"started_at":                       timePtrRFC3339(in.StartedAt),
		"ended_at":                         timePtrRFC3339(in.EndedAt),
		"include_funding_transactions":     in.IncludeFundingTransactions,
		"include_entitlement_transactions": in.IncludeEntitlementTransactions,
		"include_usage_cost_lines":         in.IncludeUsageCostLines,
		"include_balance_snapshot":         in.IncludeBalanceSnapshot,
		"low_balance_threshold_cents":      in.LowBalanceThresholdCents,
		"resource_mode":                    "scheduler_serial",
	}
}

func costSyncInputFromSnapshot(supplierID int64, snapshot map[string]any) costsapp.SyncInput {
	return costsapp.SyncInput{
		SupplierID:                     supplierID,
		StartedAt:                      timePtrFromSnapshot(snapshot, "started_at"),
		EndedAt:                        timePtrFromSnapshot(snapshot, "ended_at"),
		IncludeFundingTransactions:     boolValue(snapshot, "include_funding_transactions", true),
		IncludeEntitlementTransactions: boolValue(snapshot, "include_entitlement_transactions", true),
		IncludeUsageCostLines:          boolValue(snapshot, "include_usage_cost_lines", true),
		IncludeBalanceSnapshot:         boolValue(snapshot, "include_balance_snapshot", true),
		LowBalanceThresholdCents:       int64Value(snapshot, "low_balance_threshold_cents"),
	}
}

func costSyncResultSnapshot(result *costsapp.SyncResult) map[string]any {
	if result == nil {
		return map[string]any{}
	}
	out := map[string]any{
		"supplier_id":              result.SupplierID,
		"provider_type":            result.ProviderType,
		"system_type":              result.SystemType,
		"origin":                   result.Origin,
		"api_base_url":             result.APIBaseURL,
		"synced_at":                result.SyncedAt.UTC().Format(time.RFC3339),
		"funding_transactions":     result.FundingTransactions,
		"entitlement_transactions": result.EntitlementTransactions,
		"usage_cost_lines":         result.UsageCostLines,
		"ledger_entries":           result.LedgerEntries,
		"capabilities":             result.Capabilities,
		"diagnostics":              result.Diagnostics,
	}
	if result.Snapshot != nil {
		out["snapshot_id"] = result.Snapshot.ID
		out["currency"] = result.Snapshot.Currency
		out["captured_at"] = result.Snapshot.CapturedAt.UTC().Format(time.RFC3339)
	}
	return out
}

func costSyncResultCount(result *costsapp.SyncResult) int {
	if result == nil {
		return 0
	}
	count := result.FundingTransactions + result.EntitlementTransactions + result.UsageCostLines + result.LedgerEntries
	if result.Snapshot != nil {
		count++
	}
	return count
}

func (s *Service) purityAccountForSupplier(ctx context.Context, supplier *adminplusdomain.Supplier, request map[string]any) (int64, string, error) {
	accountID := int64Value(request, "local_sub2api_account_id")
	if accountID > 0 {
		return accountID, purityProviderFromSupplier(supplier), nil
	}
	accounts, err := s.supplierService.ListAccounts(ctx, supplier.ID)
	if err != nil {
		return 0, "", err
	}
	for _, account := range accounts {
		if account != nil && account.LocalSub2APIAccountID > 0 {
			return account.LocalSub2APIAccountID, purityProviderFromSupplier(supplier), nil
		}
	}
	return 0, "", infraerrors.New(http.StatusConflict, "ADMIN_PLUS_SCHEDULER_PURITY_ACCOUNT_MISSING", "supplier has no linked local account for purity check")
}

func purityProviderFromSupplier(supplier *adminplusdomain.Supplier) string {
	if supplier == nil {
		return ""
	}
	switch supplier.Type {
	case adminplusdomain.SupplierTypeAnthropic:
		return purityapp.ProviderAnthropic
	case adminplusdomain.SupplierTypeGemini:
		return purityapp.ProviderGemini
	default:
		return purityapp.ProviderOpenAI
	}
}

func purityReportSnapshot(report *purityapp.PublicReport, accountID int64, provider string) map[string]any {
	if report == nil {
		return map[string]any{
			"local_sub2api_account_id": accountID,
			"provider":                 provider,
		}
	}
	out := map[string]any{
		"local_sub2api_account_id": accountID,
		"provider":                 provider,
		"report_id":                report.ReportID,
		"model":                    report.ModelID,
		"status":                   report.Status,
		"verdict":                  report.Verdict,
		"score":                    report.Score,
		"total":                    report.Total,
		"official_score":           report.OfficialScore,
		"compatibility_score":      report.CompatibilityScore,
		"summary":                  report.Summary,
		"checked_at":               report.CheckedAt.UTC().Format(time.RFC3339),
	}
	if report.Metrics.Usage != nil {
		out["input_tokens"] = report.Metrics.Usage.InputTokens
		out["output_tokens"] = report.Metrics.Usage.OutputTokens
		out["cached_tokens"] = report.Metrics.Usage.CachedTokens
	}
	if report.TokenAudit != nil {
		out["token_audit_status"] = report.TokenAudit.Status
		out["token_audit_summary"] = report.TokenAudit.Summary
	}
	return out
}

func timePtrRFC3339(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func timePtrFromSnapshot(snapshot map[string]any, key string) *time.Time {
	raw := strings.TrimSpace(stringValueAny(snapshot[key]))
	if raw == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	utc := parsed.UTC()
	return &utc
}

func boolValue(snapshot map[string]any, key string, fallback bool) bool {
	value, ok := snapshot[key]
	if !ok || value == nil {
		return fallback
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func int64Value(snapshot map[string]any, key string) int64 {
	value := snapshot[key]
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		parsed, _ := v.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return parsed
	default:
		return 0
	}
}

func stringValue(snapshot map[string]any, key string) string {
	return strings.TrimSpace(stringValueAny(snapshot[key]))
}

func stringValueAny(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
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

func (s *Service) ProcessNext(ctx context.Context, workerID string) (bool, error) {
	if s.repo == nil {
		return false, nil
	}
	if strings.TrimSpace(workerID) == "" {
		workerID = defaultWorkerID()
	}
	step, err := s.repo.ClaimStep(ctx, workerID, 5*time.Minute)
	if err != nil || step == nil {
		return false, err
	}
	finishedAt := s.now().UTC()
	stepCtx, cancelStep := context.WithCancel(ctx)
	monitorDone := make(chan struct{})
	go s.cancelClaimedStepWhenRunStops(stepCtx, step.RunID, cancelStep, monitorDone)
	status, total, reason, result := s.executeClaimedStep(stepCtx, step, finishedAt)
	close(monitorDone)
	cancelStep()
	if err := s.repo.CompleteStep(ctx, step.ID, status, total, reason, result, finishedAt); err != nil {
		return true, err
	}
	if err := s.repo.RefreshRunStatus(ctx, step.RunID, finishedAt); err != nil {
		return true, err
	}
	if err := s.notifyRunStatusObserver(ctx, step.RunID); err != nil {
		return true, err
	}
	return true, nil
}

func (s *Service) notifyRunStatusObserver(ctx context.Context, runID string) error {
	if s == nil || s.runObserver == nil {
		return nil
	}
	return s.runObserver.OnSchedulerRunStatusRefreshed(ctx, runID)
}

func (s *Service) cancelClaimedStepWhenRunStops(ctx context.Context, runID string, cancel context.CancelFunc, done <-chan struct{}) {
	if s == nil || s.repo == nil || strings.TrimSpace(runID) == "" || cancel == nil {
		return
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			run, err := s.repo.GetRun(ctx, runID)
			if err != nil || run == nil {
				continue
			}
			if schedulerRunAllowsClaimedStep(run.Status) {
				continue
			}
			cancel()
			return
		}
	}
}

func schedulerRunAllowsClaimedStep(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "running", "retryable_failed", "partial_succeeded", "manual_required":
		return true
	default:
		return false
	}
}

func (s *Service) EnqueueDuePlan(ctx context.Context) (bool, error) {
	if s.repo == nil {
		return false, nil
	}
	_ = s.repo.SavePlans(ctx, s.defaultPlans())
	plan, err := s.repo.ClaimDuePlan(ctx, s.now().UTC())
	if err != nil || plan == nil {
		return false, err
	}
	taskTypes := schedulerPlanTaskTypes(*plan)
	if len(taskTypes) == 0 {
		return false, nil
	}
	_, err = s.EnqueueRun(ctx, RunInput{
		Mode:          "plan:" + plan.ID,
		TaskTypes:     taskTypes,
		WindowMinutes: plan.WindowMinutes,
	})
	if err != nil {
		return true, err
	}
	return true, nil
}

func (s *Service) executeClaimedStep(ctx context.Context, step *adminplusdomain.SchedulerStepRecord, now time.Time) (string, int, string, map[string]any) {
	if step == nil {
		return "dead", 0, "step_missing", nil
	}
	supplier, err := s.supplierService.Get(ctx, step.SupplierID)
	if err != nil {
		return "retryable_failed", 0, err.Error(), nil
	}
	item := adminplusdomain.ScheduledTask{
		SupplierID:   step.SupplierID,
		SupplierName: step.SupplierName,
		TaskType:     step.TaskType,
		Action:       step.Action,
		ScheduleKey:  step.ScheduleKey,
		TaskID:       step.ExtensionTaskID,
		Request:      cloneMap(step.RequestSnapshot),
	}
	if reason := ineligibleReason(supplier, step.TaskType); reason != "" {
		return "skipped", 0, reason, nil
	}
	if item.Action == actionDirectSync {
		s.syncSupplierTaskWithSessionRefresh(ctx, supplier, step.TaskType, now, &item)
	} else {
		task, created, err := s.extensionService.CreateTaskIfAbsent(ctx, extensionapp.CreateTaskInput{
			SupplierID:  supplier.ID,
			Type:        step.TaskType,
			ScheduleKey: step.ScheduleKey,
			Priority:    taskPriority(step.TaskType),
			MaxAttempts: 3,
			Payload: map[string]any{
				"source":        "scheduler",
				"mode":          "worker",
				"task_type":     string(step.TaskType),
				"schedule_key":  step.ScheduleKey,
				"supplier_id":   supplier.ID,
				"supplier_name": supplier.Name,
				"supplier_type": string(supplier.Type),
				"dashboard_url": supplier.DashboardURL,
				"api_base_url":  supplier.APIBaseURL,
			},
		})
		if err != nil {
			item.Reason = err.Error()
		} else if task != nil {
			item.TaskID = task.ID
			item.Created = created
			if !created {
				item.Reason = "duplicate"
			}
		}
	}
	if item.Reason != "" {
		if item.Reason == "duplicate" {
			return "skipped", item.Total, item.Reason, item.Result
		}
		return stepFailureStatus(item.Reason), item.Total, item.Reason, item.Result
	}
	return "succeeded", item.Total, "", item.Result
}

func (s *Service) scheduleSupplierTask(ctx context.Context, supplier *adminplusdomain.Supplier, taskType adminplusdomain.ExtensionTaskType, mode string, windowMinutes int, now time.Time, dryRun bool, request map[string]any) adminplusdomain.ScheduledTask {
	item := adminplusdomain.ScheduledTask{
		SupplierID:   supplier.ID,
		SupplierName: supplier.Name,
		TaskType:     taskType,
		Action:       actionForTaskType(taskType),
		Request:      cloneMap(request),
	}
	if reason := ineligibleReason(supplier, taskType); reason != "" {
		item.Reason = reason
		return item
	}
	bucket := scheduleBucket(taskType, now, windowMinutes)
	item.ScheduleKey = fmt.Sprintf("scheduler:%s:supplier:%d:%s", taskType, supplier.ID, bucket)
	if dryRun {
		return item
	}
	if item.Action == actionDirectSync {
		s.syncSupplierTaskWithSessionRefresh(ctx, supplier, taskType, now, &item)
		return item
	}
	task, created, err := s.extensionService.CreateTaskIfAbsent(ctx, extensionapp.CreateTaskInput{
		SupplierID:  supplier.ID,
		Type:        taskType,
		ScheduleKey: item.ScheduleKey,
		Priority:    taskPriority(taskType),
		MaxAttempts: 3,
		Payload: map[string]any{
			"source":          "scheduler",
			"mode":            mode,
			"task_type":       string(taskType),
			"schedule_key":    item.ScheduleKey,
			"schedule_bucket": bucket,
			"supplier_id":     supplier.ID,
			"supplier_name":   supplier.Name,
			"supplier_type":   string(supplier.Type),
			"dashboard_url":   supplier.DashboardURL,
			"api_base_url":    supplier.APIBaseURL,
		},
	})
	if err != nil {
		item.Reason = err.Error()
		return item
	}
	if task != nil {
		item.TaskID = task.ID
	}
	item.Created = created
	if !created {
		item.Reason = "duplicate"
	}
	return item
}

func (s *Service) planSupplierTask(supplier *adminplusdomain.Supplier, taskType adminplusdomain.ExtensionTaskType, windowMinutes int, now time.Time) adminplusdomain.ScheduledTask {
	item := adminplusdomain.ScheduledTask{
		SupplierID:   supplier.ID,
		SupplierName: supplier.Name,
		TaskType:     taskType,
		Action:       actionForTaskType(taskType),
	}
	if reason := ineligibleReason(supplier, taskType); reason != "" {
		item.Reason = reason
		return item
	}
	bucket := scheduleBucket(taskType, now, windowMinutes)
	item.ScheduleKey = fmt.Sprintf("scheduler:%s:supplier:%d:%s", taskType, supplier.ID, bucket)
	return item
}

func (s *Service) syncSupplierTaskWithSessionRefresh(ctx context.Context, supplier *adminplusdomain.Supplier, taskType adminplusdomain.ExtensionTaskType, now time.Time, item *adminplusdomain.ScheduledTask) {
	if !taskNeedsSession(taskType) || s.sessionRefresher == nil {
		s.syncSupplierTask(ctx, supplier, taskType, now, item)
		return
	}
	ok, refreshed := s.ensureSupplierSession(ctx, supplier, taskType, item)
	if !ok {
		return
	}
	s.syncSupplierTask(ctx, supplier, taskType, now, item)
	if item.Reason != "" && refreshed {
		item.Reason = withStepFailureMetadata(item.Reason, map[string]string{"login_attempted": "true"})
	}
	if item.Reason == "" || refreshed || !isSessionRefreshableReason(item.Reason) {
		return
	}
	previousReason := item.Reason
	item.Reason = ""
	item.Synced = false
	item.Total = 0
	if s.refreshSupplierSession(ctx, supplier, taskType, item, "session_refresh_after_sync", previousReason) {
		s.syncSupplierTask(ctx, supplier, taskType, now, item)
		if item.Reason != "" {
			item.Reason = withStepFailureMetadata(item.Reason, map[string]string{"login_attempted": "true"})
		}
	}
}

func (s *Service) ensureSupplierSession(ctx context.Context, supplier *adminplusdomain.Supplier, taskType adminplusdomain.ExtensionTaskType, item *adminplusdomain.ScheduledTask) (bool, bool) {
	_, err := s.sessionRefresher.DecryptedProbeInput(ctx, supplier.ID)
	if err == nil {
		return true, false
	}
	if !isSessionRefreshableError(err) {
		return true, false
	}
	return s.refreshSupplierSession(ctx, supplier, taskType, item, "session_precheck", err.Error()), true
}

func (s *Service) refreshSupplierSession(ctx context.Context, supplier *adminplusdomain.Supplier, taskType adminplusdomain.ExtensionTaskType, item *adminplusdomain.ScheduledTask, stage string, previousReason string) bool {
	if !supplier.Credential.BrowserLoginEnabled || (!supplier.Credential.BrowserLoginUsernameConfigured && !supplier.Credential.BrowserLoginTokenConfigured) {
		item.Reason = encodeStepFailure(stepFailureReason{
			Stage:      stage,
			Code:       reasonCodeFromText(previousReason),
			Message:    "supplier session is unavailable",
			Action:     "direct_login",
			Outcome:    "manual_required",
			Suggestion: "开启供应商登录并配置账号密码或 access token 后重试。",
			RawError:   trimLimit(previousReason, 900),
			Metadata: map[string]string{
				"login_attempted": "false",
			},
		})
		return false
	}
	loginContext := map[string]any{
		"source":          "scheduler",
		"task_type":       string(taskType),
		"scheduler_stage": stage,
	}
	if taskRequiresAdminSession(taskType) {
		loginContext["require_admin_session"] = true
		loginContext["required_role"] = "10"
	}
	login, loginErr := s.sessionRefresher.Login(ctx, sessionsapp.LoginInput{
		SupplierID:   supplier.ID,
		LoginContext: loginContext,
	})
	if loginErr != nil {
		item.Reason = encodeStepFailure(stepFailureReason{
			Stage:        stage,
			Code:         reasonCodeFromText(previousReason),
			Message:      "supplier session is unavailable",
			Action:       "direct_login",
			Outcome:      loginFailureOutcome(loginErr),
			LoginCode:    infraerrors.Reason(loginErr),
			LoginMessage: firstNonEmpty(infraerrors.Message(loginErr), "supplier direct login failed"),
			Suggestion:   loginFailureSuggestion(loginErr),
			RawError:     trimLimit(loginErr.Error(), 900),
			Metadata:     mergeMetadata(errorMetadata(loginErr), map[string]string{"login_attempted": "true"}),
		})
		return false
	}
	if login == nil || login.Session == nil {
		item.Reason = encodeStepFailure(stepFailureReason{
			Stage:      stage,
			Code:       reasonCodeFromText(previousReason),
			Message:    "supplier session is unavailable",
			Action:     "direct_login",
			Outcome:    "failed",
			Suggestion: "自动登录没有返回可用会话，请手动一键登录或使用插件采集会话。",
			Metadata: map[string]string{
				"login_attempted": "true",
			},
		})
		return false
	}
	return true
}

func taskRequiresAdminSession(taskType adminplusdomain.ExtensionTaskType) bool {
	switch taskType {
	case adminplusdomain.ExtensionTaskTypeFetchUsageCosts, adminplusdomain.ExtensionTaskTypeReconcileCosts:
		return true
	default:
		return false
	}
}

func (s *Service) syncSupplierTask(ctx context.Context, supplier *adminplusdomain.Supplier, taskType adminplusdomain.ExtensionTaskType, now time.Time, item *adminplusdomain.ScheduledTask) {
	switch taskType {
	case adminplusdomain.ExtensionTaskTypeFetchGroups:
		if s.groupSyncer == nil {
			item.Reason = "group_syncer_missing"
			return
		}
		result, err := s.groupSyncer.Sync(ctx, supplier.ID)
		if err != nil {
			item.Reason = encodeSyncFailure(stepFailureInput{TaskType: taskType, Stage: "supplier_groups_sync", Action: "sync_groups", Err: err})
			return
		}
		if result != nil {
			item.Total = result.Total
		}
	case adminplusdomain.ExtensionTaskTypeFetchRates:
		if s.rateSyncer == nil {
			item.Reason = "rate_syncer_missing"
			return
		}
		result, err := s.rateSyncer.SyncFromSession(ctx, ratesapp.SyncFromSessionInput{SupplierID: supplier.ID})
		if err != nil {
			item.Reason = encodeSyncFailure(stepFailureInput{TaskType: taskType, Stage: "supplier_rates_sync", Action: "sync_rates", Err: err})
			return
		}
		if result != nil {
			item.Total = result.Total
		}
	case adminplusdomain.ExtensionTaskTypeFetchBalance:
		if s.balanceSyncer == nil {
			item.Reason = "balance_syncer_missing"
			return
		}
		result, err := s.balanceSyncer.SyncFromSession(ctx, balancesapp.SyncFromSessionInput{SupplierID: supplier.ID})
		if err != nil {
			item.Reason = encodeSyncFailure(stepFailureInput{TaskType: taskType, Stage: "supplier_balance_sync", Action: "sync_balance", Err: err})
			return
		}
		if result != nil && result.Snapshot != nil {
			item.Total = 1
		}
	case adminplusdomain.ExtensionTaskTypeFetchAnnouncements:
		item.Reason = "announcement_sync_removed"
		return
	case adminplusdomain.ExtensionTaskTypeFetchHealth:
		if s.healthSyncer == nil {
			item.Reason = "health_syncer_missing"
			return
		}
		result, err := s.healthSyncer.SyncFromSession(ctx, healthapp.SyncFromSessionInput{SupplierID: supplier.ID})
		if err != nil {
			item.Reason = encodeSyncFailure(stepFailureInput{TaskType: taskType, Stage: "supplier_health_sync", Action: "sync_health", Err: err})
			return
		}
		if result != nil {
			item.Total = result.Total
		}
	case adminplusdomain.ExtensionTaskTypeFetchUsageCosts:
		if s.usageCostSyncer == nil {
			item.Reason = "usage_cost_syncer_missing"
			return
		}
		startedAt, endedAt := usageCostWindow(now)
		result, err := s.usageCostSyncer.SyncFromSession(ctx, usagecostsapp.SyncFromSessionInput{
			SupplierID: supplier.ID,
			StartedAt:  startedAt,
			EndedAt:    endedAt,
		})
		if err != nil {
			item.Reason = encodeSyncFailure(stepFailureInput{TaskType: taskType, Stage: "supplier_usage_costs_sync", Action: "sync_usage_costs", Err: err})
			return
		}
		if result != nil {
			item.Total = result.Total
		}
	case adminplusdomain.ExtensionTaskTypeReconcileCosts:
		if s.costSyncer == nil {
			item.Reason = "cost_syncer_missing"
			return
		}
		result, err := s.costSyncer.Sync(ctx, costSyncInputFromSnapshot(supplier.ID, item.Request))
		if err != nil {
			item.Reason = encodeSyncFailure(stepFailureInput{TaskType: taskType, Stage: "supplier_costs_reconcile", Action: "sync_costs", Err: err})
			return
		}
		item.Result = costSyncResultSnapshot(result)
		item.Total = costSyncResultCount(result)
	case adminplusdomain.ExtensionTaskTypeCheckChannels:
		if s.channelChecker == nil {
			item.Reason = "channel_checker_missing"
			return
		}
		result, err := s.channelChecker.Check(ctx, channelchecksapp.CheckInput{
			SupplierID:         supplier.ID,
			AutoPauseOnFailure: true,
		})
		if err != nil {
			item.Reason = encodeSyncFailure(stepFailureInput{TaskType: taskType, Stage: "supplier_channel_check", Action: "check_channels", Err: err})
			return
		}
		if result != nil {
			item.Total = result.Total
		}
	case adminplusdomain.ExtensionTaskTypeRunPurityCheck:
		if s.purityChecker == nil {
			item.Reason = "purity_checker_missing"
			return
		}
		accountID, provider, err := s.purityAccountForSupplier(ctx, supplier, item.Request)
		if err != nil {
			item.Reason = encodeSyncFailure(stepFailureInput{TaskType: taskType, Stage: "supplier_purity_check", Action: "resolve_account", Err: err})
			return
		}
		report, err := s.purityChecker.RunAccountCheck(ctx, purityapp.AccountCheckInput{
			AccountID: accountID,
			Provider:  provider,
			ModelID:   stringValue(item.Request, "model"),
		})
		if err != nil {
			item.Reason = encodeSyncFailure(stepFailureInput{TaskType: taskType, Stage: "supplier_purity_check", Action: "run_account_check", Err: err})
			return
		}
		item.Total = 1
		item.Result = purityReportSnapshot(report, accountID, provider)
	default:
		item.Reason = "direct_sync_not_supported"
		return
	}
	item.Synced = true
}

func (s *Service) rememberRun(ctx context.Context, run *adminplusdomain.SchedulerRun) error {
	if run == nil {
		return nil
	}
	requestedAt := run.RequestedAt.UTC()
	finishedAt := s.now().UTC()
	summary := adminplusdomain.SchedulerRunSummary{
		ID:             strings.ReplaceAll(run.RunID, ":", "-"),
		LegacyRunID:    run.RunID,
		TriggerType:    run.Mode,
		TaskType:       schedulerRunTaskLabel(run.TaskTypes),
		Status:         schedulerRunStatus(run),
		RequestedAt:    requestedAt,
		StartedAt:      &requestedAt,
		FinishedAt:     &finishedAt,
		SupplierCount:  schedulerRunSupplierCount(run.Items),
		TotalSteps:     len(run.Items),
		SucceededSteps: schedulerRunSucceededSteps(run.Items),
		FailedSteps:    schedulerRunFailedSteps(run.Items),
		SkippedSteps:   run.SkippedCount,
		DurationMS:     finishedAt.Sub(requestedAt).Milliseconds(),
	}
	if summary.FailedSteps > 0 {
		summary.ErrorCode = "SCHEDULER_STEP_FAILED"
		summary.ErrorMessage = "存在失败或跳过的调度步骤"
	}
	if s.repo != nil {
		if err := s.repo.SaveRun(ctx, summary, run.Items); err != nil {
			return err
		}
	}
	s.recentRunsMu.Lock()
	defer s.recentRunsMu.Unlock()
	s.recentRuns = append(s.recentRuns, summary)
	if len(s.recentRuns) > 100 {
		s.recentRuns = append([]adminplusdomain.SchedulerRunSummary{}, s.recentRuns[len(s.recentRuns)-100:]...)
	}
	return nil
}

func (s *Service) defaultPlans() []adminplusdomain.SchedulerPlan {
	now := s.now().UTC()
	return []adminplusdomain.SchedulerPlan{
		plan("supplier.balance.sync", "余额同步", "supplier.balance.sync", []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeFetchBalance}, "enabled", "全部启用供应商", 10*time.Minute, 10, "fire_once", "forbid", false, "读取供应商用户侧余额并刷新余额快照", now),
		plan("supplier.groups.sync", "分组同步", "supplier.groups.sync", []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeFetchGroups}, "enabled", "全部启用供应商", time.Hour, 10, "fire_once", "forbid", false, "同步供应商分组、渠道和协议投影", now),
		plan("supplier.rates.sync", "倍率同步", "supplier.rates.sync", []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeFetchRates}, "enabled", "全部启用供应商", time.Hour, 10, "fire_once", "forbid", false, "同步使用倍率、充值倍率和有效倍率", now),
		plan("supplier.costs.reconcile", "成本对账", "supplier.costs.reconcile", []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeReconcileCosts}, "enabled", "全部启用供应商", time.Hour, 60, "backfill", "forbid", false, "分批采集充值、兑换、usage 和余额并刷新成本台账", now),
		plan("supplier.session.probe", "会话探测", "supplier.session.probe", []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeFetchHealth}, "enabled", "全部启用供应商", 30*time.Minute, 30, "fire_once", "forbid", false, "探测供应商会话和只读 capability", now),
		plan("supplier.channels.check", "渠道健康检测", "supplier.channels.check", []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeCheckChannels}, "paused", "按供应商启用", 0, 10, "skip", "forbid", true, "使用真实模型请求检测渠道可用性、首 token 和总耗时", now),
		plan("supplier.purity.check", "模型纯度检测", "supplier.purity.check", []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeRunPurityCheck}, "paused", "按供应商启用", 0, 10, "skip", "forbid", true, "使用本地账号 API Key 执行模型身份、协议和 usage 纯度检测", now),
		plan("local.sub2api.schedule.ensure", "加入本地调度", "local.sub2api.schedule.ensure", nil, "paused", "智能动作触发", 0, 10, "skip", "forbid", true, "将低倍率且可用的供应商渠道加入本地 Lime/Sub2API 调度", now),
	}
}

func (s *Service) attachPlanStats(ctx context.Context, plans []adminplusdomain.SchedulerPlan) []adminplusdomain.SchedulerPlan {
	if len(plans) == 0 {
		return plans
	}
	if s.repo != nil {
		if stats, err := s.repo.PlanStats(ctx, plans); err == nil {
			for idx := range plans {
				stat, ok := stats[plans[idx].ID]
				if !ok {
					continue
				}
				plans[idx].LastSuccessAt = stat.LastSuccessAt
				plans[idx].IssueCount = stat.IssueCount
				plans[idx].LastIssueAt = stat.LastIssueAt
				plans[idx].LastIssue = stat.LastIssue
			}
			return plans
		}
	}
	runs := s.ListRuns(context.Background(), 100, 0, "")
	for idx := range plans {
		for runIdx := range runs {
			run := runs[runIdx]
			if run.TaskType != plans[idx].TaskType && !strings.Contains(run.TaskType, plans[idx].TaskType) {
				continue
			}
			value := run.RequestedAt
			plans[idx].LastRunAt = &value
			if run.Status == "succeeded" {
				plans[idx].LastSuccessAt = &value
			} else if run.FailedSteps > 0 {
				plans[idx].IssueCount += run.FailedSteps
				plans[idx].LastIssue = run.ErrorMessage
				plans[idx].LastIssueAt = &value
			}
			break
		}
	}
	return plans
}

func plan(id, name, taskType string, taskTypes []adminplusdomain.ExtensionTaskType, status, scope string, interval time.Duration, windowMinutes int, misfire, concurrency string, highCost bool, description string, now time.Time) adminplusdomain.SchedulerPlan {
	taskTypeValues := make([]string, 0, len(taskTypes))
	for _, value := range taskTypes {
		taskTypeValues = append(taskTypeValues, string(value))
	}
	var nextRunAt *time.Time
	if status == "enabled" && interval > 0 {
		value := now.Add(interval)
		nextRunAt = &value
	}
	return adminplusdomain.SchedulerPlan{
		ID:                id,
		Name:              name,
		TaskType:          taskType,
		TaskTypes:         taskTypeValues,
		Status:            status,
		Scope:             scope,
		FrequencyLabel:    frequencyLabel(interval),
		IntervalSeconds:   int64(interval.Seconds()),
		WindowMinutes:     windowMinutes,
		MisfirePolicy:     misfire,
		ConcurrencyPolicy: concurrency,
		HighCost:          highCost,
		Description:       description,
		NextRunAt:         nextRunAt,
	}
}

func schedulerPlanTaskTypes(plan adminplusdomain.SchedulerPlan) []adminplusdomain.ExtensionTaskType {
	if len(plan.TaskTypes) == 0 {
		return nil
	}
	values := make([]adminplusdomain.ExtensionTaskType, 0, len(plan.TaskTypes))
	for _, raw := range plan.TaskTypes {
		taskType := adminplusdomain.ExtensionTaskType(strings.TrimSpace(raw))
		if taskType.Valid() {
			values = append(values, taskType)
		}
	}
	return normalizeTaskTypes(values)
}

func frequencyLabel(interval time.Duration) string {
	if interval <= 0 {
		return "手动"
	}
	if interval%time.Hour == 0 {
		return fmt.Sprintf("%d 小时", int(interval/time.Hour))
	}
	if interval%time.Minute == 0 {
		return fmt.Sprintf("%d 分钟", int(interval/time.Minute))
	}
	return fmt.Sprintf("%d 秒", int(interval/time.Second))
}

func normalizePlanConfig(config adminplusdomain.SchedulerPlanConfig) (adminplusdomain.SchedulerPlanConfig, error) {
	config.Status = strings.TrimSpace(config.Status)
	config.Scope = strings.TrimSpace(config.Scope)
	config.MisfirePolicy = strings.TrimSpace(config.MisfirePolicy)
	config.ConcurrencyPolicy = strings.TrimSpace(config.ConcurrencyPolicy)
	if config.Status != "enabled" && config.Status != "paused" && config.Status != "disabled" {
		return config, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_PLAN_STATUS_INVALID", "scheduler plan status is invalid")
	}
	if config.IntervalSeconds < 0 || config.IntervalSeconds > int64((30*24*time.Hour).Seconds()) {
		return config, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_PLAN_INTERVAL_INVALID", "scheduler plan interval is invalid")
	}
	if config.IntervalSeconds > 0 && config.IntervalSeconds < int64(time.Minute.Seconds()) {
		return config, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_PLAN_INTERVAL_INVALID", "scheduler plan interval must be at least one minute")
	}
	if config.WindowMinutes <= 0 || config.WindowMinutes > 24*60 {
		return config, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_PLAN_WINDOW_INVALID", "scheduler plan window is invalid")
	}
	switch config.MisfirePolicy {
	case "fire_once", "backfill", "skip":
	default:
		return config, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_PLAN_MISFIRE_INVALID", "scheduler plan misfire policy is invalid")
	}
	switch config.ConcurrencyPolicy {
	case "forbid", "allow":
	default:
		return config, infraerrors.New(http.StatusBadRequest, "ADMIN_PLUS_SCHEDULER_PLAN_CONCURRENCY_INVALID", "scheduler plan concurrency policy is invalid")
	}
	if config.Scope == "" {
		config.Scope = "全部启用供应商"
	}
	return config, nil
}

func applyPlanConfig(plan *adminplusdomain.SchedulerPlan, config adminplusdomain.SchedulerPlanConfig, now time.Time) {
	plan.Status = config.Status
	plan.Scope = config.Scope
	plan.IntervalSeconds = config.IntervalSeconds
	plan.WindowMinutes = config.WindowMinutes
	plan.MisfirePolicy = config.MisfirePolicy
	plan.ConcurrencyPolicy = config.ConcurrencyPolicy
	plan.FrequencyLabel = frequencyLabel(time.Duration(config.IntervalSeconds) * time.Second)
	if config.Status == "enabled" && config.IntervalSeconds > 0 {
		next := now.Add(time.Duration(config.IntervalSeconds) * time.Second)
		plan.NextRunAt = &next
	} else {
		plan.NextRunAt = nil
	}
}

func schedulerSupplierStatus(supplier *adminplusdomain.Supplier, latestSteps map[adminplusdomain.ExtensionTaskType]adminplusdomain.SchedulerStepRecord) adminplusdomain.SchedulerSupplierStatus {
	sessionStatus := taskStatusFromLatest(latestSteps, adminplusdomain.ExtensionTaskTypeFetchHealth, "not_checked")
	lastError := ""
	recommendedAction := ""
	if supplier.HealthStatus == adminplusdomain.SupplierHealthStatusCredentialInvalid {
		sessionStatus = "failed"
		lastError = "credential_invalid"
		recommendedAction = "刷新供应商会话"
	} else if strings.TrimSpace(supplier.DashboardURL) == "" && strings.TrimSpace(supplier.APIBaseURL) == "" {
		sessionStatus = "missing_url"
		lastError = "supplier_url_missing"
		recommendedAction = "补充供应商地址"
	}
	balanceStatus := taskStatusFromLatest(latestSteps, adminplusdomain.ExtensionTaskTypeFetchBalance, "not_checked")
	if balanceStatus == "not_checked" && supplier.BalanceUpdatedAt != nil {
		balanceStatus = "ok"
	}
	if supplier.BalanceCents <= 0 {
		balanceStatus = "empty"
		if recommendedAction == "" {
			recommendedAction = "刷新余额或充值"
		}
	}
	groupStatus := taskStatusFromLatest(latestSteps, adminplusdomain.ExtensionTaskTypeFetchGroups, "not_checked")
	rateStatus := taskStatusFromLatest(latestSteps, adminplusdomain.ExtensionTaskTypeFetchRates, "not_checked")
	billingStatus := billingStatusFromLatest(latestSteps)
	channelStatus := taskStatusFromLatest(latestSteps, adminplusdomain.ExtensionTaskTypeCheckChannels, "not_checked")
	scheduleStatus := "manual"
	if supplier.RuntimeStatus == adminplusdomain.SupplierRuntimeStatusDisabled {
		groupStatus = "skipped"
		rateStatus = "skipped"
		billingStatus = "skipped"
		channelStatus = "skipped"
		scheduleStatus = "paused"
		lastError = "supplier_disabled"
		recommendedAction = "恢复供应商后再启用自动化"
	}
	out := adminplusdomain.SchedulerSupplierStatus{
		SupplierID:        supplier.ID,
		SupplierName:      supplier.Name,
		SupplierType:      string(supplier.Type),
		RuntimeStatus:     string(supplier.RuntimeStatus),
		HealthStatus:      string(supplier.HealthStatus),
		BalanceCents:      supplier.BalanceCents,
		BalanceCurrency:   supplier.BalanceCurrency,
		SessionStatus:     sessionStatus,
		BalanceStatus:     balanceStatus,
		GroupStatus:       groupStatus,
		RateStatus:        rateStatus,
		BillingStatus:     billingStatus,
		ChannelStatus:     channelStatus,
		ScheduleStatus:    scheduleStatus,
		LastError:         lastError,
		RecommendedAction: recommendedAction,
	}
	out.CompletionPercent = schedulerChecklistCompletionPercent(schedulerSupplierChecklistItems(supplier, out))
	return out
}

func taskStatusFromLatest(latestSteps map[adminplusdomain.ExtensionTaskType]adminplusdomain.SchedulerStepRecord, taskType adminplusdomain.ExtensionTaskType, fallback string) string {
	if len(latestSteps) == 0 {
		return fallback
	}
	step, ok := latestSteps[taskType]
	if !ok {
		return fallback
	}
	switch step.Status {
	case "succeeded", "skipped":
		return "ready"
	case "queued", "running":
		return step.Status
	case "retryable_failed", "manual_required", "dead":
		return "failed"
	default:
		return string(step.Status)
	}
}

func billingStatusFromLatest(latestSteps map[adminplusdomain.ExtensionTaskType]adminplusdomain.SchedulerStepRecord) string {
	usage := taskStatusFromLatest(latestSteps, adminplusdomain.ExtensionTaskTypeFetchUsageCosts, "not_checked")
	if usage == "failed" {
		return "failed"
	}
	if usage == "ready" {
		return "ready"
	}
	if usage == "running" {
		return "running"
	}
	if usage == "queued" {
		return "queued"
	}
	return "not_checked"
}

func schedulerSupplierChecklistItems(supplier *adminplusdomain.Supplier, status adminplusdomain.SchedulerSupplierStatus) []adminplusdomain.SchedulerSupplierChecklistItem {
	updatedAt := supplier.UpdatedAt
	items := []adminplusdomain.SchedulerSupplierChecklistItem{
		checklistItem("basic", "基础信息", checklistStatus(supplier.Name != "" && supplier.Type != ""), "供应商名称和类型已配置", supplier.Name, "补充供应商基础信息", &updatedAt),
		checklistItem("url", "Dashboard/API URL", checklistURLStatus(supplier), "Provider Adapter 需要可访问地址执行采集", firstNonEmpty(supplier.DashboardURL, supplier.APIBaseURL), "补充供应商地址", &updatedAt),
		checklistItem("session", "会话可用", status.SessionStatus, "会话状态决定余额、分组和账单采集是否可执行", string(supplier.HealthStatus), "刷新供应商会话", &updatedAt),
		checklistItem("balance", "余额可采集", status.BalanceStatus, "余额影响是否进入自动切换候选", formatSupplierBalance(supplier.BalanceCents, supplier.BalanceCurrency), "刷新余额或充值", supplier.BalanceUpdatedAt),
		checklistItem("recharge_rate", "充值倍率", checklistStatus(supplier.RechargeMultiplier > 0), "用于将显示额度换算为实际支付成本", fmt.Sprintf("%.4g", supplier.RechargeMultiplier), "设置或重新采集充值倍率", &updatedAt),
		checklistItem("recharge_entry", "充值入口", checklistStatus(strings.TrimSpace(supplier.ThirdPartyRechargeURL) != "" || strings.TrimSpace(supplier.LocalRechargeURL) != ""), "对账需要保存第三方和本地充值入口", firstNonEmpty(supplier.ThirdPartyRechargeURL, supplier.LocalRechargeURL), "采集公告/充值入口", &updatedAt),
		checklistItem("groups", "分组同步", status.GroupStatus, "同步供应商分组、渠道和协议投影", status.GroupStatus, "同步分组", &updatedAt),
		checklistItem("rates", "使用倍率", status.RateStatus, "同步渠道使用倍率并计算有效倍率", status.RateStatus, "同步倍率", &updatedAt),
		checklistItem("billing", "账务采集", status.BillingStatus, "充值、兑换和 usage 采集为成本对账提供事实", status.BillingStatus, "运行成本对账", &updatedAt),
		checklistItem("channels", "渠道检测", status.ChannelStatus, "渠道检测会消耗 token，默认需要人工启用", status.ChannelStatus, "开启渠道检测或单次检测", &updatedAt),
		checklistItem("schedule", "Lime/OpenAI 调度", status.ScheduleStatus, "可用低倍率渠道应能加入本地调度分组", status.ScheduleStatus, "加入本地调度", &updatedAt),
	}
	if supplier.RuntimeStatus == adminplusdomain.SupplierRuntimeStatusDisabled {
		for idx := range items {
			if items[idx].Status == "ready" || items[idx].Status == "ok" {
				items[idx].Status = "skipped"
				items[idx].RecommendedAction = "恢复供应商后再检查"
			}
		}
	}
	return items
}

func schedulerChecklistCompletionPercent(items []adminplusdomain.SchedulerSupplierChecklistItem) int {
	if len(items) == 0 {
		return 0
	}
	completed := 0
	for _, item := range items {
		if item.Status == "ready" || item.Status == "ok" || item.Status == "enabled" {
			completed++
		}
	}
	return completed * 100 / len(items)
}

func checklistItem(key, label, status, description, evidence, action string, checkedAt *time.Time) adminplusdomain.SchedulerSupplierChecklistItem {
	return adminplusdomain.SchedulerSupplierChecklistItem{
		Key:               key,
		Label:             label,
		Status:            status,
		Description:       description,
		Evidence:          strings.TrimSpace(evidence),
		RecommendedAction: action,
		LastCheckedAt:     checkedAt,
	}
}

func checklistStatus(ok bool) string {
	if ok {
		return "ready"
	}
	return "missing"
}

func checklistURLStatus(supplier *adminplusdomain.Supplier) string {
	if strings.TrimSpace(supplier.DashboardURL) != "" || strings.TrimSpace(supplier.APIBaseURL) != "" {
		return "ready"
	}
	return "missing_url"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func taskNeedsSession(taskType adminplusdomain.ExtensionTaskType) bool {
	switch taskType {
	case adminplusdomain.ExtensionTaskTypeFetchGroups,
		adminplusdomain.ExtensionTaskTypeFetchRates,
		adminplusdomain.ExtensionTaskTypeFetchBalance,
		adminplusdomain.ExtensionTaskTypeFetchHealth,
		adminplusdomain.ExtensionTaskTypeFetchUsageCosts,
		adminplusdomain.ExtensionTaskTypeReconcileCosts,
		adminplusdomain.ExtensionTaskTypeCheckChannels:
		return true
	default:
		return false
	}
}

func stepFailureStatus(reason string) string {
	if stepFailureNeedsManual(reason) {
		return "manual_required"
	}
	return "retryable_failed"
}

func stepFailureNeedsManual(reason string) bool {
	var payload stepFailureReason
	if json.Unmarshal([]byte(strings.TrimSpace(reason)), &payload) == nil {
		if payload.Outcome == "manual_required" {
			return true
		}
		return isManualRequiredLoginCode(payload.LoginCode)
	}
	return isManualRequiredLoginCode(reasonCodeFromText(reason))
}

func encodeStepFailure(reason stepFailureReason) string {
	raw, err := json.Marshal(reason)
	if err == nil {
		return string(raw)
	}
	return strings.TrimSpace(firstNonEmpty(reason.LoginMessage, reason.Message, reason.RawError, reason.Code))
}

func encodeSyncFailure(in stepFailureInput) string {
	err := in.Err
	if err == nil {
		return ""
	}
	code := infraerrors.Reason(err)
	message := infraerrors.Message(err)
	metadata := cloneStringMap(in.Metadata)
	if appErr := infraerrors.FromError(err); appErr != nil {
		if appErr.Reason != "" {
			code = appErr.Reason
		}
		if appErr.Message != "" && appErr.Message != infraerrors.UnknownMessage {
			message = appErr.Message
		}
		metadata = mergeMetadata(metadata, errorMetadata(err))
	}
	if strings.TrimSpace(code) == "" {
		code = firstNonEmpty(reasonCodeFromText(err.Error()), "SCHEDULER_STEP_FAILED")
	}
	if strings.TrimSpace(message) == "" || message == infraerrors.UnknownMessage {
		message = syncFailureDefaultMessage(in.TaskType)
	}
	return encodeStepFailure(stepFailureReason{
		Stage:      firstNonEmpty(in.Stage, "supplier_sync"),
		Code:       code,
		Message:    message,
		Action:     firstNonEmpty(in.Action, "sync"),
		Outcome:    syncFailureOutcome(code),
		Suggestion: syncFailureSuggestion(code),
		RawError:   trimLimit(err.Error(), 900),
		Metadata:   metadata,
	})
}

func errorMetadata(err error) map[string]string {
	appErr := infraerrors.FromError(err)
	if appErr == nil || len(appErr.Metadata) == 0 {
		return nil
	}
	out := make(map[string]string, len(appErr.Metadata))
	for key, value := range appErr.Metadata {
		out[key] = value
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func mergeMetadata(base map[string]string, extra map[string]string) map[string]string {
	if len(extra) == 0 {
		return cloneStringMap(base)
	}
	out := cloneStringMap(base)
	if out == nil {
		out = make(map[string]string, len(extra))
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}

func withStepFailureMetadata(reason string, metadata map[string]string) string {
	if strings.TrimSpace(reason) == "" || len(metadata) == 0 {
		return reason
	}
	var payload stepFailureReason
	if json.Unmarshal([]byte(strings.TrimSpace(reason)), &payload) != nil {
		return reason
	}
	payload.Metadata = mergeMetadata(payload.Metadata, metadata)
	return encodeStepFailure(payload)
}

func syncFailureDefaultMessage(taskType adminplusdomain.ExtensionTaskType) string {
	switch taskType {
	case adminplusdomain.ExtensionTaskTypeFetchBalance:
		return "supplier balance sync failed"
	case adminplusdomain.ExtensionTaskTypeFetchGroups:
		return "supplier groups sync failed"
	case adminplusdomain.ExtensionTaskTypeFetchRates:
		return "supplier rates sync failed"
	case adminplusdomain.ExtensionTaskTypeFetchUsageCosts:
		return "supplier usage costs sync failed"
	case adminplusdomain.ExtensionTaskTypeReconcileCosts:
		return "supplier costs reconcile failed"
	case adminplusdomain.ExtensionTaskTypeFetchHealth:
		return "supplier health sync failed"
	case adminplusdomain.ExtensionTaskTypeCheckChannels:
		return "supplier channel check failed"
	default:
		return "supplier task sync failed"
	}
}

func syncFailureOutcome(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "LOGIN_CREDENTIAL_INVALID",
		"LOGIN_CAPTCHA_REQUIRED",
		"LOGIN_MFA_REQUIRED",
		"BROWSER_FALLBACK_REQUIRED",
		"BROWSER_CHALLENGE_REQUIRED",
		"SUPPLIER_SESSION_PERMISSION_DENIED",
		"SUPPLIER_NEW_API_ADMIN_SESSION_REQUIRED":
		return "manual_required"
	default:
		return "failed"
	}
}

func syncFailureSuggestion(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "SUPPLIER_SESSION_NOT_FOUND", "SUPPLIER_SESSION_EXPIRED", "SUPPLIER_SESSION_DECRYPT_FAILED":
		return "重新一键登录或使用 Chrome 插件采集会话后重试。"
	case "SUPPLIER_SESSION_PERMISSION_DENIED":
		return "供应商会话无权读取该接口，请重新登录、检查账号权限或改用插件采集最新会话。"
	case "SUPPLIER_NEW_API_ADMIN_SESSION_REQUIRED":
		return "new-api 历史全量需要管理员/root 会话，请先执行一键登录并使用管理员账号重新登录后重试。"
	case "SUPPLIER_SESSION_PROBE_FAILED":
		return "供应商接口超时或不可达，请检查供应商地址、网络出口和前置防护后重试。"
	case "SUPPLIER_SESSION_PROBE_HTML", "SUPPLIER_SESSION_PROBE_BAD_STATUS", "SUPPLIER_DIRECT_LOGIN_UPSTREAM_HTML", "SUPPLIER_DIRECT_LOGIN_UPSTREAM_ORIGIN_ERROR":
		return "供应商前置层返回了 HTML/源站异常，请检查 Cloudflare/Nginx/风控策略或改用浏览器会话。"
	case "LOGIN_CREDENTIAL_INVALID":
		return "供应商登录凭据无效，请更新账号密码或 token 后重试。"
	case "LOGIN_CAPTCHA_REQUIRED", "BROWSER_CHALLENGE_REQUIRED", "BROWSER_FALLBACK_REQUIRED":
		return "供应商要求浏览器验证，请人工完成浏览器验证或使用插件采集会话。"
	case "LOGIN_MFA_REQUIRED":
		return "供应商要求二次验证，请人工完成登录或使用插件采集会话。"
	default:
		return "查看供应商地址、登录凭据、会话权限和上游防护策略后重试。"
	}
}

func isSessionRefreshableError(err error) bool {
	if err == nil {
		return false
	}
	return isSessionRefreshableCode(infraerrors.Reason(err)) || isSessionRefreshableReason(err.Error())
}

func isSessionRefreshableReason(reason string) bool {
	return isSessionRefreshableCode(reasonCodeFromText(reason))
}

func isSessionRefreshableCode(code string) bool {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "SUPPLIER_SESSION_NOT_FOUND",
		"SUPPLIER_SESSION_EXPIRED",
		"SUPPLIER_SESSION_DECRYPT_FAILED",
		"SUPPLIER_SESSION_PERMISSION_DENIED",
		"SUPPLIER_NEW_API_ADMIN_SESSION_REQUIRED":
		return true
	default:
		return false
	}
}

func loginFailureOutcome(err error) string {
	if isManualRequiredLoginCode(infraerrors.Reason(err)) {
		return "manual_required"
	}
	if code := infraerrors.Code(err); code == http.StatusUnauthorized || code == http.StatusForbidden {
		return "manual_required"
	}
	return "failed"
}

func isManualRequiredLoginCode(code string) bool {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED",
		"SUPPLIER_DIRECT_LOGIN_API_BASE_URL_REQUIRED",
		"SUPPLIER_DIRECT_LOGIN_ADMIN_REQUIRED",
		"SUPPLIER_NEW_API_ADMIN_SESSION_REQUIRED",
		"SUPPLIER_DIRECT_LOGIN_UNSUPPORTED",
		"LOGIN_CREDENTIAL_INVALID",
		"LOGIN_CAPTCHA_REQUIRED",
		"LOGIN_MFA_REQUIRED",
		"BROWSER_FALLBACK_REQUIRED",
		"BROWSER_CHALLENGE_REQUIRED",
		"PASSWORD_LOGIN_DISABLED":
		return true
	default:
		return false
	}
}

func loginFailureSuggestion(err error) string {
	switch strings.ToUpper(strings.TrimSpace(infraerrors.Reason(err))) {
	case "SUPPLIER_DIRECT_LOGIN_API_BASE_URL_REQUIRED":
		return "补充供应商 API 地址后重试。"
	case "SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED":
		return "补充供应商登录账号密码或 access token 后重试。"
	case "SUPPLIER_DIRECT_LOGIN_ADMIN_REQUIRED":
		return "供应商后台模式需要管理员账号，请换用管理员凭据后重试。"
	case "SUPPLIER_DIRECT_LOGIN_UNSUPPORTED":
		return "该供应商类型暂不支持后端直登，请使用插件采集会话后重试。"
	case "LOGIN_CREDENTIAL_INVALID":
		return "供应商登录凭据无效，请更新账号密码或 token 后重试。"
	case "LOGIN_CAPTCHA_REQUIRED", "BROWSER_CHALLENGE_REQUIRED", "BROWSER_FALLBACK_REQUIRED":
		return "供应商要求浏览器验证，请人工完成浏览器验证或使用插件采集会话后重试。"
	case "LOGIN_MFA_REQUIRED":
		return "供应商要求二次验证，请人工完成登录或使用插件采集会话。"
	case "PASSWORD_LOGIN_DISABLED":
		return "供应商关闭密码登录，请改用 token 或插件采集会话。"
	default:
		return "自动登录失败，请检查供应商地址、凭据和防护策略后重试。"
	}
}

func reasonCodeFromText(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ""
	}
	var payload stepFailureReason
	if json.Unmarshal([]byte(reason), &payload) == nil {
		return firstNonEmpty(payload.LoginCode, payload.Code)
	}
	upper := strings.ToUpper(reason)
	for _, marker := range []string{
		"SUPPLIER_SESSION_NOT_FOUND",
		"SUPPLIER_SESSION_EXPIRED",
		"SUPPLIER_SESSION_DECRYPT_FAILED",
		"SUPPLIER_SESSION_PERMISSION_DENIED",
		"SUPPLIER_SESSION_PROBE_FAILED",
		"SUPPLIER_SESSION_PROBE_HTML",
		"SUPPLIER_SESSION_PROBE_BAD_STATUS",
		"SUPPLIER_SESSION_PROFILE_INVALID",
		"SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED",
		"SUPPLIER_DIRECT_LOGIN_UPSTREAM_HTML",
		"SUPPLIER_DIRECT_LOGIN_UPSTREAM_ORIGIN_ERROR",
		"SUPPLIER_DIRECT_LOGIN_FAILED",
		"LOGIN_CREDENTIAL_INVALID",
		"LOGIN_CAPTCHA_REQUIRED",
		"LOGIN_MFA_REQUIRED",
		"BROWSER_FALLBACK_REQUIRED",
		"BROWSER_CHALLENGE_REQUIRED",
		"PASSWORD_LOGIN_DISABLED",
	} {
		if strings.Contains(upper, marker) {
			return marker
		}
	}
	return ""
}

func trimLimit(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}

func formatSupplierBalance(cents int64, currency string) string {
	if currency == "" {
		currency = "USD"
	}
	return fmt.Sprintf("%.2f %s", float64(cents)/100, currency)
}

func schedulerAction(now time.Time, supplier *adminplusdomain.Supplier, severity, actionType, title, reason, operation string) adminplusdomain.SchedulerAction {
	return adminplusdomain.SchedulerAction{
		ID:                   fmt.Sprintf("%s:%d", actionType, supplier.ID),
		SupplierID:           supplier.ID,
		SupplierName:         supplier.Name,
		Severity:             severity,
		Status:               "open",
		Type:                 actionType,
		Title:                title,
		Reason:               reason,
		RecommendedOperation: operation,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func schedulerRunTaskLabel(taskTypes []adminplusdomain.ExtensionTaskType) string {
	if len(taskTypes) == 1 {
		return schedulerTaskTypeLabel(taskTypes[0])
	}
	if len(taskTypes) == 0 {
		return "supplier.balance.sync"
	}
	return "mixed"
}

func schedulerTaskTypeLabel(taskType adminplusdomain.ExtensionTaskType) string {
	switch taskType {
	case adminplusdomain.ExtensionTaskTypeFetchGroups:
		return "supplier.groups.sync"
	case adminplusdomain.ExtensionTaskTypeFetchRates:
		return "supplier.rates.sync"
	case adminplusdomain.ExtensionTaskTypeFetchBalance:
		return "supplier.balance.sync"
	case adminplusdomain.ExtensionTaskTypeFetchHealth:
		return "supplier.session.probe"
	case adminplusdomain.ExtensionTaskTypeFetchUsageCosts:
		return "supplier.usage_costs.sync"
	case adminplusdomain.ExtensionTaskTypeReconcileCosts:
		return "supplier.costs.reconcile"
	case adminplusdomain.ExtensionTaskTypeCheckChannels:
		return "supplier.channels.check"
	case adminplusdomain.ExtensionTaskTypeRunPurityCheck:
		return "supplier.purity.check"
	case adminplusdomain.ExtensionTaskTypeCaptureSession:
		return "supplier.session.capture"
	default:
		return string(taskType)
	}
}

func schedulerRunStatus(run *adminplusdomain.SchedulerRun) string {
	if run == nil {
		return "unknown"
	}
	if len(run.Items) == 0 {
		return "skipped"
	}
	if schedulerRunSucceededSteps(run.Items) == len(run.Items) {
		return "succeeded"
	}
	if schedulerRunSucceededSteps(run.Items) > 0 {
		return "partial_succeeded"
	}
	if run.DryRun {
		return "succeeded"
	}
	return "retryable_failed"
}

func schedulerRunSupplierCount(items []adminplusdomain.ScheduledTask) int {
	seen := map[int64]struct{}{}
	for _, item := range items {
		seen[item.SupplierID] = struct{}{}
	}
	return len(seen)
}

func schedulerRunSucceededSteps(items []adminplusdomain.ScheduledTask) int {
	count := 0
	for _, item := range items {
		if item.Synced || (!item.Synced && item.Reason == "") {
			count++
		}
	}
	return count
}

func schedulerRunFailedSteps(items []adminplusdomain.ScheduledTask) int {
	count := 0
	for _, item := range items {
		if item.Reason != "" && item.Reason != "duplicate" {
			count++
		}
	}
	return count
}

func normalizeTaskTypes(input []adminplusdomain.ExtensionTaskType) []adminplusdomain.ExtensionTaskType {
	if len(input) == 0 {
		out := []adminplusdomain.ExtensionTaskType{
			adminplusdomain.ExtensionTaskTypeFetchBalance,
		}
		if channelChecksSchedulerEnabled() {
			out = append(out, adminplusdomain.ExtensionTaskTypeCheckChannels)
		}
		return out
	}
	out := make([]adminplusdomain.ExtensionTaskType, 0, len(input))
	seen := make(map[adminplusdomain.ExtensionTaskType]struct{}, len(input))
	for _, taskType := range input {
		if !taskType.Valid() {
			continue
		}
		if taskType == adminplusdomain.ExtensionTaskTypeFetchAnnouncements {
			continue
		}
		if _, ok := seen[taskType]; ok {
			continue
		}
		seen[taskType] = struct{}{}
		out = append(out, taskType)
	}
	return out
}

func ineligibleReason(supplier *adminplusdomain.Supplier, taskType adminplusdomain.ExtensionTaskType) string {
	if supplier.RuntimeStatus == adminplusdomain.SupplierRuntimeStatusDisabled {
		return "supplier_disabled"
	}
	if supplier.HealthStatus == adminplusdomain.SupplierHealthStatusPaused {
		return "supplier_paused"
	}
	if supplier.HealthStatus == adminplusdomain.SupplierHealthStatusCredentialInvalid {
		return "credential_invalid"
	}
	if actionForTaskType(taskType) == actionDirectSync && taskType != adminplusdomain.ExtensionTaskTypeRunPurityCheck && supplier.DashboardURL == "" && supplier.APIBaseURL == "" {
		return "supplier_url_missing"
	}
	if actionForTaskType(taskType) != actionDirectSync && supplier.DashboardURL == "" {
		return "dashboard_url_missing"
	}
	if actionForTaskType(taskType) != actionDirectSync {
		if !supplier.Credential.BrowserLoginEnabled {
			return "browser_login_disabled"
		}
		if !supplier.Credential.BrowserLoginUsernameConfigured && !supplier.Credential.BrowserLoginTokenConfigured {
			return "browser_login_credential_missing"
		}
	}
	switch taskType {
	case adminplusdomain.ExtensionTaskTypeFetchHealth, adminplusdomain.ExtensionTaskTypeFetchUsageCosts, adminplusdomain.ExtensionTaskTypeCheckChannels:
		if !adminplusdomain.CanUseSupplierForSwitching(supplier.RuntimeStatus, supplier.BalanceCents) {
			return "not_switch_eligible"
		}
	}
	return ""
}

func actionForTaskType(taskType adminplusdomain.ExtensionTaskType) string {
	switch taskType {
	case adminplusdomain.ExtensionTaskTypeFetchGroups,
		adminplusdomain.ExtensionTaskTypeFetchRates,
		adminplusdomain.ExtensionTaskTypeFetchBalance,
		adminplusdomain.ExtensionTaskTypeFetchHealth,
		adminplusdomain.ExtensionTaskTypeFetchUsageCosts,
		adminplusdomain.ExtensionTaskTypeReconcileCosts,
		adminplusdomain.ExtensionTaskTypeCheckChannels,
		adminplusdomain.ExtensionTaskTypeRunPurityCheck:
		return actionDirectSync
	case adminplusdomain.ExtensionTaskTypeCaptureSession:
		return actionExtensionTask
	default:
		return actionCompatTask
	}
}

func usageCostWindow(now time.Time) (time.Time, time.Time) {
	startedAt := now.UTC().Truncate(24 * time.Hour)
	return startedAt, startedAt.Add(24 * time.Hour)
}

func scheduleBucket(taskType adminplusdomain.ExtensionTaskType, now time.Time, windowMinutes int) string {
	if taskType == adminplusdomain.ExtensionTaskTypeFetchUsageCosts || taskType == adminplusdomain.ExtensionTaskTypeReconcileCosts {
		return now.Format(dailyBucketLayout)
	}
	window := time.Duration(windowMinutes) * time.Minute
	return now.Truncate(window).Format(windowBucketLayout)
}

func taskPriority(taskType adminplusdomain.ExtensionTaskType) int {
	switch taskType {
	case adminplusdomain.ExtensionTaskTypeCaptureSession:
		return 95
	case adminplusdomain.ExtensionTaskTypeFetchGroups:
		return 85
	case adminplusdomain.ExtensionTaskTypeFetchBalance:
		return 90
	case adminplusdomain.ExtensionTaskTypeFetchRates:
		return 80
	case adminplusdomain.ExtensionTaskTypeFetchHealth:
		return 60
	case adminplusdomain.ExtensionTaskTypeCheckChannels:
		return 55
	case adminplusdomain.ExtensionTaskTypeRunPurityCheck:
		return 50
	case adminplusdomain.ExtensionTaskTypeFetchUsageCosts:
		return 40
	case adminplusdomain.ExtensionTaskTypeReconcileCosts:
		return 35
	default:
		return 10
	}
}

type Worker struct {
	service *Service
	stop    chan struct{}
	done    chan struct{}
	once    sync.Once
	started bool
}

func NewWorker(service *Service) *Worker {
	return &Worker{
		service: service,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

func ProvideWorker(service *Service) *Worker {
	worker := NewWorker(service)
	if schedulerWorkerEnabled() {
		worker.Start(schedulerInterval())
	}
	return worker
}

func (w *Worker) Start(interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	w.started = true
	go func() {
		defer close(w.done)
		timer := time.NewTimer(2 * time.Second)
		nextPeriodic := time.Now().Add(10 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				processed, _ := w.service.ProcessNext(context.Background(), defaultWorkerID())
				if !processed && time.Now().After(nextPeriodic) && w.service.automaticSchedulingEnabled(context.Background()) {
					if w.service.repo != nil {
						processed, _ = w.service.EnqueueDuePlan(context.Background())
						if processed {
							nextPeriodic = time.Now().Add(500 * time.Millisecond)
						} else {
							nextPeriodic = time.Now().Add(interval)
						}
					} else {
						_, _ = w.service.Run(context.Background(), RunInput{Mode: "periodic"})
						processed = true
						nextPeriodic = time.Now().Add(interval)
					}
				}
				if processed {
					timer.Reset(500 * time.Millisecond)
				} else {
					timer.Reset(5 * time.Second)
				}
			case <-w.stop:
				return
			}
		}
	}()
}

func (w *Worker) Stop() {
	w.once.Do(func() {
		if !w.started {
			return
		}
		close(w.stop)
		<-w.done
	})
}

func (s *Service) automaticSchedulingEnabled(ctx context.Context) bool {
	return s.Settings(ctx).Enabled
}

func schedulerEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_PLUS_SCHEDULER_ENABLED")))
	return value == "" || value == "1" || value == "true" || value == "yes"
}

func schedulerWorkerEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_PLUS_SCHEDULER_WORKER_ENABLED")))
	return value == "" || value == "1" || value == "true" || value == "yes"
}

func schedulerInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv("ADMIN_PLUS_SCHEDULER_INTERVAL_SECONDS"))
	if raw == "" {
		return 10 * time.Minute
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return 10 * time.Minute
	}
	return time.Duration(seconds) * time.Second
}

func channelChecksSchedulerEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_PLUS_CHANNEL_CHECKS_SCHEDULER_ENABLED")))
	return value == "1" || value == "true" || value == "yes"
}

func defaultWorkerID() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "scheduler-worker"
	}
	return fmt.Sprintf("%s-%d", hostname, os.Getpid())
}
