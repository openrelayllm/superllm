package scheduler

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

const (
	defaultWindowMinutes = 10
	dailyBucketLayout    = "20060102"
	windowBucketLayout   = "200601021504"
)

type RunInput struct {
	Mode          string
	SupplierID    int64
	TaskTypes     []adminplusdomain.ExtensionTaskType
	WindowMinutes int
	DryRun        bool
	Now           time.Time
}

type Service struct {
	supplierService  *suppliersapp.Service
	extensionService *extensionapp.Service
	now              func() time.Time
}

func NewService(supplierService *suppliersapp.Service, extensionService *extensionapp.Service) *Service {
	return &Service{
		supplierService:  supplierService,
		extensionService: extensionService,
		now:              time.Now,
	}
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
			item := s.scheduleSupplierTask(ctx, supplier, taskType, mode, windowMinutes, in.Now, in.DryRun)
			if item.Created {
				run.CreatedCount++
			} else if item.Reason == "" {
				run.EligibleCount++
			} else {
				run.SkippedCount++
			}
			run.Items = append(run.Items, item)
		}
	}
	return run, nil
}

func (s *Service) Status() adminplusdomain.SchedulerStatus {
	return adminplusdomain.SchedulerStatus{
		Enabled:         schedulerEnabled(),
		IntervalSeconds: int64(schedulerInterval().Seconds()),
		Queue:           "admin_plus_extension_tasks",
	}
}

func (s *Service) scheduleSupplierTask(ctx context.Context, supplier *adminplusdomain.Supplier, taskType adminplusdomain.ExtensionTaskType, mode string, windowMinutes int, now time.Time, dryRun bool) adminplusdomain.ScheduledTask {
	item := adminplusdomain.ScheduledTask{
		SupplierID:   supplier.ID,
		SupplierName: supplier.Name,
		TaskType:     taskType,
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

func normalizeTaskTypes(input []adminplusdomain.ExtensionTaskType) []adminplusdomain.ExtensionTaskType {
	if len(input) == 0 {
		return []adminplusdomain.ExtensionTaskType{
			adminplusdomain.ExtensionTaskTypeFetchRates,
			adminplusdomain.ExtensionTaskTypeFetchGroups,
			adminplusdomain.ExtensionTaskTypeFetchBalance,
			adminplusdomain.ExtensionTaskTypeFetchPromotions,
			adminplusdomain.ExtensionTaskTypeFetchHealth,
			adminplusdomain.ExtensionTaskTypeExportBills,
		}
	}
	out := make([]adminplusdomain.ExtensionTaskType, 0, len(input))
	seen := make(map[adminplusdomain.ExtensionTaskType]struct{}, len(input))
	for _, taskType := range input {
		if !taskType.Valid() {
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
	if !supplier.Credential.BrowserLoginEnabled {
		return "browser_login_disabled"
	}
	if supplier.DashboardURL == "" {
		return "dashboard_url_missing"
	}
	if !supplier.Credential.BrowserLoginUsernameConfigured && !supplier.Credential.BrowserLoginTokenConfigured {
		return "browser_login_credential_missing"
	}
	switch taskType {
	case adminplusdomain.ExtensionTaskTypeFetchHealth, adminplusdomain.ExtensionTaskTypeExportBills:
		if !adminplusdomain.CanUseSupplierForSwitching(supplier.RuntimeStatus, supplier.BalanceCents) {
			return "not_switch_eligible"
		}
	}
	return ""
}

func scheduleBucket(taskType adminplusdomain.ExtensionTaskType, now time.Time, windowMinutes int) string {
	if taskType == adminplusdomain.ExtensionTaskTypeExportBills {
		return now.Format(dailyBucketLayout)
	}
	window := time.Duration(windowMinutes) * time.Minute
	return now.Truncate(window).Format(windowBucketLayout)
}

func taskPriority(taskType adminplusdomain.ExtensionTaskType) int {
	switch taskType {
	case adminplusdomain.ExtensionTaskTypeFetchGroups:
		return 85
	case adminplusdomain.ExtensionTaskTypeFetchBalance:
		return 90
	case adminplusdomain.ExtensionTaskTypeFetchRates:
		return 80
	case adminplusdomain.ExtensionTaskTypeFetchPromotions:
		return 70
	case adminplusdomain.ExtensionTaskTypeFetchHealth:
		return 60
	case adminplusdomain.ExtensionTaskTypeExportBills:
		return 40
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
	if schedulerEnabled() {
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
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				_, _ = w.service.Run(context.Background(), RunInput{Mode: "periodic"})
				timer.Reset(interval)
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

func schedulerEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_PLUS_SCHEDULER_ENABLED")))
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
