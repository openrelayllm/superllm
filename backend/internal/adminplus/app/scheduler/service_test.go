package scheduler

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"testing"
	"time"

	actionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/actions"
	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	costsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/costs"
	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	healthapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/health"
	purityapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/purity"
	ratesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/rates"
	sessionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sessions"
	sub2apiapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sub2api"
	suppliergroupsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliergroups"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	usagecostsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/usagecosts"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServiceRunCreatesDurableExtensionTasksAndDeduplicates(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(supplierService, extensionService)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	supplier := createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:                 "relay-a",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:        adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:         adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:         "https://relay-a.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: "ops@example.com",
		BalanceCents:         500_00,
		BalanceCurrency:      "CNY",
	})

	first, err := service.Run(context.Background(), RunInput{
		Mode:      "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeCaptureSession},
	})
	require.NoError(t, err)
	require.Equal(t, 1, first.CreatedCount)
	require.Equal(t, 0, first.SkippedCount)
	require.Len(t, first.Items, 1)
	require.NotEmpty(t, first.Items[0].ScheduleKey)

	tasks, err := extensionService.ListTasks(context.Background(), extensionapp.TaskFilter{SupplierID: supplier.ID, Limit: 20})
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Contains(t, tasks[0].Payload, "schedule_key")

	second, err := service.Run(context.Background(), RunInput{
		Mode:      "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeCaptureSession},
	})
	require.NoError(t, err)
	require.Equal(t, 0, second.CreatedCount)
	require.Equal(t, 1, second.SkippedCount)
	require.Len(t, second.Items, 1)
	for _, item := range second.Items {
		require.False(t, item.Created)
		require.Equal(t, "duplicate", item.Reason)
		require.NotZero(t, item.TaskID)
	}
}

func TestServiceRunDirectSyncTasksDoNotCreateExtensionTasks(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	groupSyncer := &stubGroupSyncer{total: 2}
	rateSyncer := &stubRateSyncer{total: 3}
	balanceSyncer := &stubBalanceSyncer{total: 1}
	healthSyncer := &stubHealthSyncer{total: 1}
	usageCostSyncer := &stubUsageCostSyncer{total: 4}
	service := NewServiceWithDependencies(supplierService, extensionService, groupSyncer, rateSyncer, balanceSyncer, healthSyncer, usageCostSyncer, nil)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	supplier := createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:            "relay-a",
		Kind:            adminplusdomain.SupplierKindRelay,
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:    "https://relay-a.example.com",
		BalanceCents:    500_00,
		BalanceCurrency: "CNY",
	})

	run, err := service.Run(context.Background(), RunInput{
		Mode: "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{
			adminplusdomain.ExtensionTaskTypeFetchGroups,
			adminplusdomain.ExtensionTaskTypeFetchRates,
			adminplusdomain.ExtensionTaskTypeFetchBalance,
			adminplusdomain.ExtensionTaskTypeFetchHealth,
			adminplusdomain.ExtensionTaskTypeFetchUsageCosts,
		},
	})
	require.NoError(t, err)
	require.Equal(t, 0, run.CreatedCount)
	require.Equal(t, 5, run.EligibleCount)
	require.Equal(t, 0, run.SkippedCount)
	require.Len(t, run.Items, 5)
	for _, item := range run.Items {
		require.Equal(t, actionDirectSync, item.Action)
		require.True(t, item.Synced)
		require.NotZero(t, item.Total)
		require.Zero(t, item.TaskID)
		require.Empty(t, item.Reason)
	}
	require.Equal(t, 1, groupSyncer.calls)
	require.Equal(t, 1, rateSyncer.calls)
	require.Equal(t, 1, balanceSyncer.calls)
	require.Equal(t, 1, healthSyncer.calls)
	require.Equal(t, 1, usageCostSyncer.calls)
	require.Equal(t, time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC), usageCostSyncer.startedAt)
	require.Equal(t, time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC), usageCostSyncer.endedAt)

	tasks, err := extensionService.ListTasks(context.Background(), extensionapp.TaskFilter{SupplierID: supplier.ID, Limit: 20})
	require.NoError(t, err)
	require.Empty(t, tasks)
}

func TestServiceRunPurityCheckUsesRequestSnapshot(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	purityChecker := &stubPurityChecker{}
	service := NewServiceWithDependencies(supplierService, extensionService, nil, nil, nil, nil, nil, nil).
		WithPurityChecker(purityChecker)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	supplier := createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:          "relay-purity-direct",
		Kind:          adminplusdomain.SupplierKindRelay,
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
	})
	_, err := supplierService.CreateAccount(context.Background(), suppliersapp.CreateSupplierAccountInput{
		SupplierID:            supplier.ID,
		LocalSub2APIAccountID: 1,
		BalanceCurrency:       "USD",
	})
	require.NoError(t, err)

	run, err := service.Run(context.Background(), RunInput{
		Mode:      "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeRunPurityCheck},
		Request:   map[string]any{"model": "gpt-direct"},
	})

	require.NoError(t, err)
	require.Equal(t, 1, run.EligibleCount)
	require.Len(t, run.Items, 1)
	require.True(t, run.Items[0].Synced)
	require.Equal(t, "gpt-direct", run.Items[0].Request["model"])
	require.Equal(t, "purity-report-1", run.Items[0].Result["report_id"])
	require.Equal(t, "gpt-direct", purityChecker.input.ModelID)
}

func TestServiceRunDryRunExplainsEligibleTasksWithoutWritingQueue(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(supplierService, extensionService)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	supplier := createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:                 "relay-a",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:        adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:         adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:         "https://relay-a.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: "ops@example.com",
		BalanceCents:         500_00,
		BalanceCurrency:      "CNY",
	})

	run, err := service.Run(context.Background(), RunInput{
		Mode:      "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeFetchGroups},
		DryRun:    true,
	})

	require.NoError(t, err)
	require.True(t, run.DryRun)
	require.Equal(t, 0, run.CreatedCount)
	require.Equal(t, 1, run.EligibleCount)
	require.Equal(t, 0, run.SkippedCount)
	require.Len(t, run.Items, 1)
	require.Equal(t, adminplusdomain.ExtensionTaskTypeFetchGroups, run.Items[0].TaskType)
	require.Equal(t, actionDirectSync, run.Items[0].Action)
	require.NotEmpty(t, run.Items[0].ScheduleKey)

	tasks, err := extensionService.ListTasks(context.Background(), extensionapp.TaskFilter{SupplierID: supplier.ID, Limit: 20})
	require.NoError(t, err)
	require.Empty(t, tasks)
}

func TestServiceRunDefaultsToBalanceRefreshTask(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	balanceSyncer := &stubBalanceSyncer{total: 1}
	service := NewServiceWithDependencies(supplierService, extensionService, nil, nil, balanceSyncer, nil, nil, nil)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	supplier := createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:                 "relay-a",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:        adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:         adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:         "https://relay-a.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: "ops@example.com",
		BalanceCents:         500_00,
		BalanceCurrency:      "CNY",
	})

	run, err := service.Run(context.Background(), RunInput{Mode: "manual"})

	require.NoError(t, err)
	require.Equal(t, []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeFetchBalance}, run.TaskTypes)
	require.Equal(t, 0, run.CreatedCount)
	require.Equal(t, 1, run.EligibleCount)
	require.Len(t, run.Items, 1)
	require.Equal(t, adminplusdomain.ExtensionTaskTypeFetchBalance, run.Items[0].TaskType)
	require.Equal(t, actionDirectSync, run.Items[0].Action)
	require.True(t, run.Items[0].Synced)
	require.Equal(t, 1, balanceSyncer.calls)

	tasks, err := extensionService.ListTasks(context.Background(), extensionapp.TaskFilter{SupplierID: supplier.ID, Limit: 20})
	require.NoError(t, err)
	require.Empty(t, tasks)
}

func TestServiceRunFiltersRemovedAnnouncementTask(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewServiceWithDependencies(
		supplierService,
		extensionService,
		nil,
		nil,
		&stubBalanceSyncer{total: 1},
		nil,
		nil,
		nil,
	)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:            "relay-a",
		Kind:            adminplusdomain.SupplierKindRelay,
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:    "https://relay-a.example.com",
		BalanceCents:    500_00,
		BalanceCurrency: "CNY",
	})

	run, err := service.Run(context.Background(), RunInput{
		Mode: "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{
			adminplusdomain.ExtensionTaskTypeFetchAnnouncements,
			adminplusdomain.ExtensionTaskTypeFetchBalance,
		},
	})

	require.NoError(t, err)
	require.Len(t, run.Items, 1)
	require.Equal(t, adminplusdomain.ExtensionTaskTypeFetchBalance, run.Items[0].TaskType)
}

func TestServiceCenterSurfacesPlansSupplierStatusesAndActions(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(supplierService, extensionService)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:          "needs-config",
		Kind:          adminplusdomain.SupplierKindRelay,
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
		BalanceCents:  100_00,
	})

	status := service.CenterStatus(context.Background())
	require.True(t, status.Enabled)
	require.Equal(t, "admin_plus_scheduler_runs", status.Queue)
	require.NotNil(t, status.NextRunAt)

	plans := service.ListPlans(context.Background())
	require.NotEmpty(t, plans)
	require.Contains(t, collectPlanTaskTypes(plans), "supplier.balance.sync")
	require.Contains(t, collectPlanTaskTypes(plans), "supplier.costs.reconcile")

	supplierStatuses, err := service.ListSupplierStatuses(context.Background())
	require.NoError(t, err)
	require.Len(t, supplierStatuses, 1)
	require.Equal(t, "missing_url", supplierStatuses[0].SessionStatus)
	require.NotEmpty(t, supplierStatuses[0].RecommendedAction)

	checklist, err := service.GetSupplierChecklist(context.Background(), supplierStatuses[0].SupplierID)
	require.NoError(t, err)
	require.Equal(t, supplierStatuses[0].SupplierID, checklist.SupplierID)
	require.NotEmpty(t, checklist.Items)
	require.Equal(t, "missing_url", checklistItemStatus(checklist.Items, "url"))
	require.Equal(t, "ready", checklistItemStatus(checklist.Items, "basic"))

	actions := service.ListActions(context.Background())
	require.Len(t, actions, 1)
	require.Equal(t, "supplier.configure_url", actions[0].Type)
	require.Equal(t, "critical", actions[0].Severity)
}

func TestServiceSupplierStatusesIncludeCandidateSummary(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	candidateReader := &stubCandidateSummaryReader{}
	service := NewService(supplierService, extensionService).WithCandidateSummaryReader(candidateReader)

	supplier := createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:          "lime",
		Kind:          adminplusdomain.SupplierKindRelay,
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:  "https://lime.example.com",
		BalanceCents:  100_00,
	})
	candidateReader.rows = []*adminplusdomain.LocalAccountOpsRow{{
		SupplierID:              supplier.ID,
		LocalSub2APIAccountID:   42,
		CandidateStatus:         "balance_blocked",
		BlockedReason:           "recharge_required",
		CheckSource:             "balance",
		EffectiveRateMultiplier: 0.2,
	}}

	statuses, err := service.ListSupplierStatuses(context.Background())

	require.NoError(t, err)
	require.Len(t, statuses, 1)
	require.Equal(t, supplier.ID, statuses[0].SupplierID)
	require.NotNil(t, statuses[0].CandidateSummary)
	require.Equal(t, "balance_blocked", statuses[0].CandidateSummary.CandidateStatus)
	require.Equal(t, "recharge_required", statuses[0].CandidateSummary.BlockedReason)
	require.Equal(t, 1, statuses[0].CandidateSummary.BalanceBlockedCount)
	require.Equal(t, "blocked", statuses[0].ScheduleStatus)
	require.Equal(t, "充值后重跑候选评估", statuses[0].RecommendedAction)

	checklist, err := service.GetSupplierChecklist(context.Background(), supplier.ID)
	require.NoError(t, err)
	require.NotNil(t, checklist.CandidateSummary)
	require.Equal(t, "blocked", checklistItemStatus(checklist.Items, "candidate_pool"))
}

func TestServiceListActionsIncludesLocalGroupCapacityWatch(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	candidateReader := &stubCandidateSummaryReader{
		groups: []*sub2apiapp.LocalSub2APIGroup{{
			ID:                  1001,
			Name:                "Lime",
			Platform:            "openai",
			ActiveAPIKeyCount:   2,
			SchedulableAccounts: 0,
			TotalAccounts:       1,
		}},
		rows: []*adminplusdomain.LocalAccountOpsRow{{
			LocalSub2APIAccountID:   42,
			LocalAccountPlatform:    "openai",
			LocalAccountGroupIDs:    []int64{2002},
			SupplierName:            "cheap",
			CandidateStatus:         "available",
			EffectiveRateMultiplier: 0.12,
		}},
	}
	service := NewService(supplierService, extensionService).WithCandidateSummaryReader(candidateReader)

	actions := service.ListActions(context.Background())

	require.Len(t, actions, 1)
	require.Equal(t, "local_group.routing.refill", actions[0].Type)
	require.Equal(t, "critical", actions[0].Severity)
	require.Equal(t, "Lime", actions[0].SupplierName)
	require.Contains(t, actions[0].Reason, "账号 #42")
	require.Equal(t, "预览补池", actions[0].RecommendedOperation)
}

func TestServiceListActionsUsesConfiguredRoutingLowCapacityThreshold(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	repo := newFakeSchedulerRepository()
	candidateReader := &stubCandidateSummaryReader{
		groups: []*sub2apiapp.LocalSub2APIGroup{{
			ID:                  1001,
			Name:                "Lime",
			Platform:            "openai",
			ActiveAPIKeyCount:   3,
			SchedulableAccounts: 2,
			TotalAccounts:       2,
		}},
		rows: []*adminplusdomain.LocalAccountOpsRow{{
			LocalSub2APIAccountID:   42,
			LocalAccountPlatform:    "openai",
			LocalAccountGroupIDs:    []int64{2002},
			SupplierName:            "cheap",
			CandidateStatus:         "available",
			EffectiveRateMultiplier: 0.12,
		}},
	}
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, nil, nil, nil, nil).
		WithCandidateSummaryReader(candidateReader)
	_, err := service.UpdateSettings(context.Background(), adminplusdomain.SchedulerSettings{
		Enabled:                           true,
		RoutingRefillLowCapacityThreshold: 2,
		RoutingRefillCooldownSeconds:      180,
	})
	require.NoError(t, err)

	actions := service.ListActions(context.Background())

	require.Len(t, actions, 1)
	require.Equal(t, "local_group.routing.low_capacity", actions[0].Type)
	require.Equal(t, "warning", actions[0].Severity)
	require.Contains(t, actions[0].Reason, "可调度账号数为 2")
}

func TestServiceProcessNextRoutingCapacityWatchGeneratesActionsWhenAutoDisabled(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	repo := newFakeSchedulerRepository()
	candidateReader := &stubCandidateSummaryReader{
		groups: []*sub2apiapp.LocalSub2APIGroup{{
			ID:                  1001,
			Name:                "Lime",
			Platform:            "openai",
			ActiveAPIKeyCount:   2,
			SchedulableAccounts: 0,
		}},
		rows: []*adminplusdomain.LocalAccountOpsRow{{
			LocalSub2APIAccountID:   42,
			LocalAccountPlatform:    "openai",
			SupplierName:            "cheap",
			CandidateStatus:         "available",
			EffectiveRateMultiplier: 0.12,
		}},
	}
	refiller := &stubRoutingRefiller{}
	actionSyncer := &stubActionRecommendationSyncer{}
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, nil, nil, nil, nil).
		WithCandidateSummaryReader(candidateReader).
		WithRoutingRefiller(refiller).
		WithActionRecommendationSyncer(actionSyncer)
	service.now = func() time.Time {
		return time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	}

	summary, err := service.EnqueueRun(context.Background(), RunInput{
		Mode:      "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeRoutingCapacityWatch},
	})
	require.NoError(t, err)
	require.Equal(t, "queued", summary.Status)
	require.Len(t, repo.steps, 1)
	require.Equal(t, int64(0), repo.steps[0].SupplierID)
	require.Equal(t, "本地调度", repo.steps[0].SupplierName)
	require.Equal(t, adminplusdomain.ExtensionTaskTypeRoutingCapacityWatch, repo.steps[0].TaskType)
	require.Equal(t, actionDirectSync, repo.steps[0].Action)

	processed, err := service.ProcessNext(context.Background(), "worker-test")

	require.NoError(t, err)
	require.True(t, processed)
	require.Empty(t, refiller.calls)
	require.Equal(t, "succeeded", repo.steps[0].Status)
	require.Equal(t, 1, repo.steps[0].ResultCount)
	require.Equal(t, false, repo.steps[0].ResultSnapshot["auto_execute_enabled"])
	require.Equal(t, 1, repo.steps[0].ResultSnapshot["actions_generated"])
	actionLinks, ok := repo.steps[0].ResultSnapshot["actions"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, actionLinks, 1)
	require.Equal(t, "local_group.routing.refill", actionLinks[0]["type"])
	require.Equal(t, "routing_refill", actionLinks[0]["recommendation_type"])
	require.Equal(t, int64(1001), actionLinks[0]["local_group_id"])
	require.Len(t, repo.actions, 1)
	require.Equal(t, "local_group.routing.refill", repo.actions[0].Type)
	require.Len(t, actionSyncer.inputs, 1)
	require.Len(t, actionSyncer.inputs[0].LocalGroupCapacity, 1)
	require.Equal(t, int64(1001), actionSyncer.inputs[0].LocalGroupCapacity[0].LocalGroupID)
}

func TestServiceProcessNextRoutingCapacityWatchAutoRefillsEmptyGroupsOnly(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	repo := newFakeSchedulerRepository()
	candidateReader := &stubCandidateSummaryReader{
		groups: []*sub2apiapp.LocalSub2APIGroup{
			{
				ID:                  1001,
				Name:                "Lime Empty",
				Platform:            "openai",
				ActiveAPIKeyCount:   2,
				SchedulableAccounts: 0,
			},
			{
				ID:                  1002,
				Name:                "Lime Low",
				Platform:            "openai",
				ActiveAPIKeyCount:   2,
				SchedulableAccounts: 1,
			},
			{
				ID:                  1003,
				Name:                "Unused",
				Platform:            "openai",
				ActiveAPIKeyCount:   0,
				SchedulableAccounts: 0,
			},
		},
		rows: []*adminplusdomain.LocalAccountOpsRow{{
			LocalSub2APIAccountID:   42,
			LocalAccountPlatform:    "openai",
			SupplierID:              7,
			SupplierName:            "cheap",
			CandidateStatus:         "available",
			EffectiveRateMultiplier: 0.12,
		}},
	}
	refiller := &stubRoutingRefiller{
		results: map[int64]*sub2apiapp.RoutingRefillResult{
			1001: {
				Action:       "refill_local_group",
				LocalGroupID: 1001,
				Platform:     "openai",
				AvailabilityBefore: &sub2apiapp.RoutingGroupAvailability{
					GroupID:             1001,
					GroupName:           "Lime Empty",
					Platform:            "openai",
					SchedulableAccounts: 0,
					ActiveAPIKeyCount:   2,
				},
				Candidate: &sub2apiapp.RoutingRefillCandidate{
					LocalSub2APIAccountID:   42,
					SupplierID:              7,
					EffectiveRateMultiplier: 0.12,
				},
			},
		},
	}
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, nil, nil, nil, nil).
		WithCandidateSummaryReader(candidateReader).
		WithRoutingRefiller(refiller)
	service.now = func() time.Time {
		return time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	}
	_, err := service.UpdateSettings(context.Background(), adminplusdomain.SchedulerSettings{
		Enabled:                           true,
		RoutingRefillAutoExecuteEnabled:   true,
		RoutingRefillLowCapacityThreshold: 1,
		RoutingRefillCooldownSeconds:      240,
		RoutingRefillConfirmWindowSeconds: 30,
		RoutingRefillMaxRateMultiplier:    0.5,
	})
	require.NoError(t, err)

	summary, err := service.EnqueueRun(context.Background(), RunInput{
		Mode:      "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeRoutingCapacityWatch},
	})
	require.NoError(t, err)
	require.Equal(t, "queued", summary.Status)

	processed, err := service.ProcessNext(context.Background(), "worker-test")

	require.NoError(t, err)
	require.True(t, processed)
	require.Len(t, refiller.calls, 1)
	require.Equal(t, int64(1001), refiller.calls[0].LocalGroupID)
	require.Equal(t, "openai", refiller.calls[0].Platform)
	require.Equal(t, 0.5, refiller.calls[0].MaxRateMultiplier)
	require.Equal(t, 240, refiller.calls[0].CooldownSeconds)
	require.Equal(t, 30, refiller.calls[0].ConfirmWindowSecs)
	require.False(t, refiller.calls[0].DryRun)
	require.Equal(t, "auto_scheduler_capacity_watch", refiller.calls[0].Reason)
	require.Equal(t, "scheduler:auto", refiller.calls[0].TriggerType)
	require.Equal(t, "succeeded", repo.steps[0].Status)
	require.Equal(t, 1, repo.steps[0].ResultCount)
	require.Equal(t, true, repo.steps[0].ResultSnapshot["auto_execute_enabled"])
	require.Equal(t, 3, repo.steps[0].ResultSnapshot["groups_scanned"])
	require.Equal(t, 1, repo.steps[0].ResultSnapshot["eligible_groups"])
	require.Equal(t, 1, repo.steps[0].ResultSnapshot["attempted"])
	require.Equal(t, 1, repo.steps[0].ResultSnapshot["succeeded"])
}

func TestServiceSettingsNormalizeRoutingRefillPolicy(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	repo := newFakeSchedulerRepository()
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, nil, nil, nil, nil)

	defaults := service.Settings(context.Background())
	require.False(t, defaults.RoutingRefillAutoExecuteEnabled)
	require.EqualValues(t, 1, defaults.RoutingRefillLowCapacityThreshold)
	require.Equal(t, 180, defaults.RoutingRefillCooldownSeconds)
	require.Equal(t, 0, defaults.RoutingRefillConfirmWindowSeconds)
	require.Zero(t, defaults.RoutingRefillMaxRateMultiplier)

	updated, err := service.UpdateSettings(context.Background(), adminplusdomain.SchedulerSettings{
		RoutingRefillAutoExecuteEnabled:   true,
		RoutingRefillLowCapacityThreshold: 0,
		RoutingRefillCooldownSeconds:      90000,
		RoutingRefillConfirmWindowSeconds: -1,
		RoutingRefillMaxRateMultiplier:    -0.25,
	})
	require.NoError(t, err)
	require.True(t, updated.RoutingRefillAutoExecuteEnabled)
	require.EqualValues(t, 1, updated.RoutingRefillLowCapacityThreshold)
	require.Equal(t, 86400, updated.RoutingRefillCooldownSeconds)
	require.Equal(t, 0, updated.RoutingRefillConfirmWindowSeconds)
	require.Zero(t, updated.RoutingRefillMaxRateMultiplier)
}

func TestServiceListActionsIncludesBadLocalAccountScheduleDisableSuggestion(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	candidateReader := &stubCandidateSummaryReader{
		rows: []*adminplusdomain.LocalAccountOpsRow{
			{
				LocalSub2APIAccountID:   42,
				LocalAccountName:        "bad-channel",
				LocalAccountPlatform:    "openai",
				LocalAccountStatus:      "active",
				LocalAccountSchedulable: true,
				LocalAccountGroupIDs:    []int64{1001},
				LocalAccountGroupNames:  []string{"Lime"},
				SupplierID:              7,
				SupplierName:            "cheap",
				SupplierRuntimeStatus:   "active",
				SupplierHealthStatus:    "normal",
				SupplierAccountID:       7001,
				SupplierGroupID:         7002,
				SupplierGroupStatus:     "active",
				SupplierKeyID:           7003,
				SupplierKeyStatus:       "bound",
				HasUsableBalance:        true,
				BalanceStatus:           "usable",
				ChannelCheckStatus:      "remote_unavailable",
				DriftStatus:             "synced",
				EffectiveRateMultiplier: 0.12,
			},
			{
				LocalSub2APIAccountID:   43,
				LocalAccountStatus:      "active",
				LocalAccountSchedulable: true,
				LocalAccountGroupIDs:    []int64{1001},
				CandidateStatus:         "balance_blocked",
				BlockedReason:           "recharge_required",
			},
			{
				LocalSub2APIAccountID:   44,
				LocalAccountStatus:      "active",
				LocalAccountSchedulable: false,
				LocalAccountGroupIDs:    []int64{1001},
				CandidateStatus:         "blocked",
				BlockedReason:           "channel_monitor_failed",
			},
		},
	}
	service := NewService(supplierService, extensionService).WithCandidateSummaryReader(candidateReader)

	actions := service.ListActions(context.Background())

	require.Len(t, actions, 1)
	require.Equal(t, "local_account.schedule.disable", actions[0].Type)
	require.Equal(t, "local_account.schedule.disable:42", actions[0].ID)
	require.Equal(t, "warning", actions[0].Severity)
	require.Equal(t, int64(7), actions[0].SupplierID)
	require.Contains(t, actions[0].Reason, "#42")
	require.Contains(t, actions[0].Reason, "channel_monitor_failed")
	require.Equal(t, "预览关闭调度", actions[0].RecommendedOperation)
}

func TestServiceListActionsSyncsLocalRoutingRecommendations(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	actionSyncer := &stubActionRecommendationSyncer{}
	candidateReader := &stubCandidateSummaryReader{
		groups: []*sub2apiapp.LocalSub2APIGroup{{
			ID:                  1001,
			Name:                "Lime",
			Platform:            "openai",
			ActiveAPIKeyCount:   2,
			SchedulableAccounts: 0,
			TotalAccounts:       1,
		}},
		rows: []*adminplusdomain.LocalAccountOpsRow{
			{
				LocalSub2APIAccountID:   42,
				LocalAccountPlatform:    "openai",
				LocalAccountGroupIDs:    []int64{2002},
				SupplierID:              7,
				SupplierName:            "cheap",
				SupplierGroupID:         7002,
				SupplierGroupName:       "cheap-low",
				CandidateStatus:         "available",
				CheckSource:             "channel_monitor",
				EffectiveRateMultiplier: 0.12,
			},
			{
				LocalSub2APIAccountID:   43,
				LocalAccountName:        "bad-channel",
				LocalAccountPlatform:    "openai",
				LocalAccountSchedulable: true,
				LocalAccountGroupIDs:    []int64{1001},
				LocalAccountGroupNames:  []string{"Lime"},
				SupplierID:              8,
				SupplierName:            "failing",
				SupplierGroupID:         8002,
				SupplierGroupName:       "failing-group",
				CandidateStatus:         "blocked",
				BlockedReason:           "channel_monitor_failed",
				CheckSource:             "channel_monitor",
				ChannelCheckStatus:      "request_error",
				EffectiveRateMultiplier: 0.2,
			},
		},
	}
	service := NewService(supplierService, extensionService).
		WithCandidateSummaryReader(candidateReader).
		WithActionRecommendationSyncer(actionSyncer)

	actions := service.ListActions(context.Background())

	require.Len(t, actions, 2)
	require.Len(t, actionSyncer.inputs, 1)
	require.Len(t, actionSyncer.inputs[0].LocalGroupCapacity, 1)
	require.Len(t, actionSyncer.inputs[0].LocalAccountSchedule, 1)
	require.Equal(t, int64(1001), actionSyncer.inputs[0].LocalGroupCapacity[0].LocalGroupID)
	require.Equal(t, int64(42), actionSyncer.inputs[0].LocalGroupCapacity[0].BestCandidateLocalAccountID)
	require.Equal(t, int64(43), actionSyncer.inputs[0].LocalAccountSchedule[0].LocalSub2APIAccountID)
}

func TestServiceUpdatePlanConfigPersistsUserSchedule(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	repo := newFakeSchedulerRepository()
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, nil, nil, nil, nil)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	require.NotEmpty(t, service.ListPlans(context.Background()))
	updated, err := service.UpdatePlanConfig(context.Background(), "supplier.balance.sync", adminplusdomain.SchedulerPlanConfig{
		Status:            "enabled",
		Scope:             "全部启用供应商",
		IntervalSeconds:   int64((15 * time.Minute).Seconds()),
		WindowMinutes:     15,
		MisfirePolicy:     "backfill",
		ConcurrencyPolicy: "allow",
	})

	require.NoError(t, err)
	require.Equal(t, int64(900), updated.IntervalSeconds)
	require.Equal(t, 15, updated.WindowMinutes)
	require.Equal(t, "15 分钟", updated.FrequencyLabel)
	require.NotNil(t, updated.NextRunAt)

	plans := service.ListPlans(context.Background())
	plan := requirePlan(t, plans, "supplier.balance.sync")
	require.Equal(t, int64(900), plan.IntervalSeconds)
	require.Equal(t, 15, plan.WindowMinutes)
	require.Equal(t, "backfill", plan.MisfirePolicy)
	require.Equal(t, "allow", plan.ConcurrencyPolicy)
}

func TestServicePlanStatsOnlyCountsIssuesAfterLastSuccess(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	repo := newFakeSchedulerRepository()
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, nil, nil, nil, nil)
	beforeSuccess := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	lastSuccess := time.Date(2026, 6, 20, 10, 5, 0, 0, time.UTC)
	afterSuccess := time.Date(2026, 6, 20, 10, 8, 0, 0, time.UTC)
	repo.steps = []adminplusdomain.SchedulerStepRecord{
		{TaskType: adminplusdomain.ExtensionTaskTypeFetchBalance, Status: "retryable_failed", Reason: "old_failure", FinishedAt: &beforeSuccess},
		{TaskType: adminplusdomain.ExtensionTaskTypeFetchBalance, Status: "succeeded", FinishedAt: &lastSuccess},
		{TaskType: adminplusdomain.ExtensionTaskTypeFetchBalance, Status: "manual_required", Reason: "new_failure", FinishedAt: &afterSuccess},
	}

	plans := service.ListPlans(context.Background())
	plan := requirePlan(t, plans, "supplier.balance.sync")

	require.Equal(t, &lastSuccess, plan.LastSuccessAt)
	require.Equal(t, 1, plan.IssueCount)
	require.Equal(t, "new_failure", plan.LastIssue)
	require.Equal(t, &afterSuccess, plan.LastIssueAt)
}

func TestServiceCenterStatusDoesNotExposePastNextRunAt(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	repo := newFakeSchedulerRepository()
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, nil, nil, nil, nil)
	now := time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	service.now = func() time.Time {
		return now
	}
	past := now.Add(-time.Hour)
	repo.plans = []adminplusdomain.SchedulerPlan{
		{
			ID:              "supplier.balance.sync",
			Name:            "余额同步",
			TaskType:        "supplier.balance.sync",
			TaskTypes:       []string{"fetch_balance"},
			Status:          "enabled",
			IntervalSeconds: 600,
			WindowMinutes:   10,
			NextRunAt:       &past,
		},
	}

	status := service.CenterStatus(context.Background())

	require.Equal(t, 1, status.OverduePlans)
	require.NotNil(t, status.NextRunAt)
	require.False(t, status.NextRunAt.Before(now))
}

func TestServiceEnqueueRunDefersExecutionUntilWorkerClaimsStep(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	balanceSyncer := &stubBalanceSyncer{total: 1}
	repo := newFakeSchedulerRepository()
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, balanceSyncer, nil, nil, nil)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:            "relay-a",
		Kind:            adminplusdomain.SupplierKindRelay,
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:    "https://relay-a.example.com",
		BalanceCents:    500_00,
		BalanceCurrency: "USD",
	})

	summary, err := service.EnqueueRun(context.Background(), RunInput{
		Mode:      "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeFetchBalance},
	})
	require.NoError(t, err)
	require.Equal(t, "queued", summary.Status)
	require.Equal(t, 0, balanceSyncer.calls)
	require.Len(t, repo.steps, 1)
	require.Equal(t, "queued", repo.steps[0].Status)

	processed, err := service.ProcessNext(context.Background(), "worker-test")
	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, 1, balanceSyncer.calls)
	require.Equal(t, "succeeded", repo.steps[0].Status)
	require.Equal(t, 1, repo.steps[0].ResultCount)
	require.Equal(t, "succeeded", repo.runs[0].Status)
}

func TestServiceProcessNextRunsPurityCheckFromQueuedStep(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	repo := newFakeSchedulerRepository()
	purityChecker := &stubPurityChecker{}
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, nil, nil, nil, nil).
		WithPurityChecker(purityChecker)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	supplier := createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:          "relay-purity",
		Kind:          adminplusdomain.SupplierKindRelay,
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
	})
	_, err := supplierService.CreateAccount(context.Background(), suppliersapp.CreateSupplierAccountInput{
		SupplierID:            supplier.ID,
		LocalSub2APIAccountID: 1,
		BalanceCurrency:       "USD",
	})
	require.NoError(t, err)

	summary, err := service.EnqueueRun(context.Background(), RunInput{
		Mode:      "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeRunPurityCheck},
		Request:   map[string]any{"model": "gpt-acceptance"},
	})
	require.NoError(t, err)
	require.Equal(t, "queued", summary.Status)
	require.Len(t, repo.steps, 1)
	require.Equal(t, adminplusdomain.ExtensionTaskTypeRunPurityCheck, repo.steps[0].TaskType)
	require.Equal(t, "gpt-acceptance", repo.steps[0].RequestSnapshot["model"])

	processed, err := service.ProcessNext(context.Background(), "worker-test")

	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, 1, purityChecker.calls)
	require.Equal(t, int64(1), purityChecker.input.AccountID)
	require.Equal(t, purityapp.ProviderOpenAI, purityChecker.input.Provider)
	require.Equal(t, "gpt-acceptance", purityChecker.input.ModelID)
	require.Equal(t, "succeeded", repo.steps[0].Status)
	require.Equal(t, 1, repo.steps[0].ResultCount)
	require.Equal(t, "purity-report-1", repo.steps[0].ResultSnapshot["report_id"])
	require.Equal(t, "gpt-acceptance", repo.steps[0].ResultSnapshot["model"])
}

func TestServiceListStepsIncludesAttemptLogs(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	repo := newFakeSchedulerRepository()
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, nil, nil, nil, nil)
	finishedAt := time.Date(2026, 6, 20, 10, 5, 0, 0, time.UTC)
	repo.steps = []adminplusdomain.SchedulerStepRecord{
		{
			ID:           33192,
			RunID:        "plan-supplier.costs.reconcile-test",
			SupplierID:   12,
			SupplierName: "登录 - 何意味",
			TaskType:     adminplusdomain.ExtensionTaskTypeReconcileCosts,
			Action:       "sync_costs",
			Status:       "retryable_failed",
			MaxAttempts:  3,
		},
	}
	repo.attempts = []adminplusdomain.SchedulerAttemptRecord{
		{
			ID:           1,
			StepID:       33192,
			RunID:        "plan-supplier.costs.reconcile-test",
			SupplierID:   12,
			TaskType:     adminplusdomain.ExtensionTaskTypeReconcileCosts,
			Status:       "retryable_failed",
			AttemptNo:    1,
			FinishedAt:   finishedAt,
			ErrorCode:    "SUPPLIER_SESSION_BAD_STATUS",
			ErrorMessage: "supplier session endpoint returned non-success status",
		},
	}

	steps, err := service.ListSteps(context.Background(), "plan-supplier.costs.reconcile-test", 100, 0)

	require.NoError(t, err)
	require.Len(t, steps, 1)
	require.Len(t, steps[0].OperationLogs, 1)
	require.Equal(t, "SUPPLIER_SESSION_BAD_STATUS", steps[0].OperationLogs[0].ErrorCode)
}

func TestServiceEnqueueCostHistoryBackfillUsesStepSnapshot(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	repo := newFakeSchedulerRepository()
	costSyncer := &stubCostSyncer{}
	service := ProvideService(repo, supplierService, extensionService, nil, nil, nil, nil, nil, costSyncer, nil, nil, nil, nil, nil, nil)
	now := time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	service.now = func() time.Time {
		return now
	}
	startedAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endedAt := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)

	createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:          "relay-a",
		Kind:          adminplusdomain.SupplierKindRelay,
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:  "https://relay-a.example.com",
		BalanceCents:  500_00,
	})

	summary, err := service.EnqueueCostHistoryBackfill(context.Background(), CostBackfillInput{
		StartedAt:                      &startedAt,
		EndedAt:                        &endedAt,
		IncludeFundingTransactions:     true,
		IncludeEntitlementTransactions: true,
		IncludeUsageCostLines:          true,
		IncludeBalanceSnapshot:         true,
	})
	require.NoError(t, err)
	require.Equal(t, "queued", summary.Status)
	require.Equal(t, "supplier.costs.reconcile", summary.TaskType)
	require.Len(t, repo.steps, 1)
	require.Equal(t, adminplusdomain.ExtensionTaskTypeReconcileCosts, repo.steps[0].TaskType)
	require.Equal(t, "2025-01-01T00:00:00Z", repo.steps[0].RequestSnapshot["started_at"])

	processed, err := service.ProcessNext(context.Background(), "worker-test")

	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, 1, costSyncer.calls)
	require.Equal(t, startedAt, *costSyncer.input.StartedAt)
	require.Equal(t, endedAt, *costSyncer.input.EndedAt)
	require.True(t, costSyncer.input.IncludeFundingTransactions)
	require.Equal(t, "succeeded", repo.steps[0].Status)
	require.Equal(t, 14, repo.steps[0].ResultCount)
	require.Equal(t, 2, repo.steps[0].ResultSnapshot["funding_transactions"])
}

func TestServiceProcessNextRefreshesMissingSessionBeforeCostBackfill(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	repo := newFakeSchedulerRepository()
	costSyncer := &stubCostSyncer{}
	sessionRefresher := &stubSessionRefresher{
		probeErrors: []error{infraerrors.New(http.StatusNotFound, "SUPPLIER_SESSION_NOT_FOUND", "supplier browser session not found")},
		loginResult: &sessionsapp.LoginResult{Session: &adminplusdomain.SupplierBrowserSession{
			SupplierID:     1,
			SessionSource:  adminplusdomain.SupplierSessionSourceDirectLogin,
			Origin:         "https://relay-a.example.com",
			APIBaseURL:     "https://relay-a.example.com",
			SessionSummary: map[string]any{"session_source": "direct_login"},
			CapturedAt:     time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC),
		}},
	}
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, nil, nil, nil, nil).
		WithSessionRefresher(sessionRefresher).
		WithCostSyncer(costSyncer)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}
	startedAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endedAt := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)

	createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:                 "relay-a",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:        adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:         adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:         "https://relay-a.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: "ops@example.com",
		BrowserLoginPassword: "secret",
		BalanceCents:         500_00,
		BalanceCurrency:      "USD",
	})
	_, err := service.EnqueueCostHistoryBackfill(context.Background(), CostBackfillInput{
		StartedAt:              &startedAt,
		EndedAt:                &endedAt,
		IncludeUsageCostLines:  true,
		IncludeBalanceSnapshot: true,
	})
	require.NoError(t, err)

	processed, err := service.ProcessNext(context.Background(), "worker-test")

	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, 1, sessionRefresher.loginCalls)
	require.NotContains(t, sessionRefresher.lastLoginInput.LoginContext, "require_admin_session")
	require.NotContains(t, sessionRefresher.lastLoginInput.LoginContext, "required_role")
	require.Equal(t, 1, costSyncer.calls)
	require.Equal(t, "succeeded", repo.steps[0].Status)
	require.Empty(t, repo.steps[0].Reason)
}

func TestServiceProcessNextRefreshesMissingSessionBeforeBalanceSync(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	balanceSyncer := &stubBalanceSyncer{total: 1}
	sessionRefresher := &stubSessionRefresher{
		probeErrors: []error{infraerrors.New(http.StatusNotFound, "SUPPLIER_SESSION_NOT_FOUND", "supplier browser session not found")},
		loginResult: &sessionsapp.LoginResult{Session: &adminplusdomain.SupplierBrowserSession{
			SupplierID:     1,
			SessionSource:  adminplusdomain.SupplierSessionSourceDirectLogin,
			Origin:         "https://relay-a.example.com",
			APIBaseURL:     "https://relay-a.example.com",
			SessionSummary: map[string]any{"session_source": "direct_login"},
			CapturedAt:     time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC),
		}},
	}
	repo := newFakeSchedulerRepository()
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, balanceSyncer, nil, nil, nil).
		WithSessionRefresher(sessionRefresher)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:                 "relay-a",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:        adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:         adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:         "https://relay-a.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: "ops@example.com",
		BrowserLoginPassword: "secret",
		BalanceCents:         500_00,
		BalanceCurrency:      "USD",
	})
	_, err := service.EnqueueRun(context.Background(), RunInput{
		Mode:      "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeFetchBalance},
	})
	require.NoError(t, err)

	processed, err := service.ProcessNext(context.Background(), "worker-test")

	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, 1, sessionRefresher.loginCalls)
	require.NotContains(t, sessionRefresher.lastLoginInput.LoginContext, "require_admin_session")
	require.Equal(t, 1, balanceSyncer.calls)
	require.Equal(t, "succeeded", repo.steps[0].Status)
	require.Empty(t, repo.steps[0].Reason)
}

func TestServiceProcessNextMarksManualRequiredWhenAutoLoginNeedsBrowser(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	balanceSyncer := &stubBalanceSyncer{total: 1}
	sessionRefresher := &stubSessionRefresher{
		probeErrors: []error{infraerrors.New(http.StatusNotFound, "SUPPLIER_SESSION_NOT_FOUND", "supplier browser session not found")},
		loginErr:    infraerrors.New(http.StatusConflict, "BROWSER_CHALLENGE_REQUIRED", "supplier direct login requires browser verification"),
	}
	repo := newFakeSchedulerRepository()
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, balanceSyncer, nil, nil, nil).
		WithSessionRefresher(sessionRefresher)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:                 "relay-browser",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:        adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:         adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:         "https://relay-browser.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: "ops@example.com",
		BrowserLoginPassword: "secret",
		BalanceCents:         500_00,
		BalanceCurrency:      "USD",
	})
	_, err := service.EnqueueRun(context.Background(), RunInput{
		Mode:      "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeFetchBalance},
	})
	require.NoError(t, err)

	processed, err := service.ProcessNext(context.Background(), "worker-test")

	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, 1, sessionRefresher.loginCalls)
	require.Equal(t, 0, balanceSyncer.calls)
	require.Equal(t, "manual_required", repo.steps[0].Status)
	require.Contains(t, repo.steps[0].Reason, "BROWSER_CHALLENGE_REQUIRED")
	require.Contains(t, repo.steps[0].Reason, "插件采集会话")
}

func TestServiceProcessNextKeepsAutoLoginUpstreamFailureRetryable(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	balanceSyncer := &stubBalanceSyncer{total: 1}
	sessionRefresher := &stubSessionRefresher{
		probeErrors: []error{infraerrors.New(http.StatusNotFound, "SUPPLIER_SESSION_NOT_FOUND", "supplier browser session not found")},
		loginErr:    infraerrors.New(http.StatusConflict, "SUPPLIER_DIRECT_LOGIN_UPSTREAM_ORIGIN_ERROR", "supplier upstream origin is unavailable"),
	}
	repo := newFakeSchedulerRepository()
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, balanceSyncer, nil, nil, nil).
		WithSessionRefresher(sessionRefresher)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:                 "relay-upstream",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:        adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:         adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:         "https://relay-upstream.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: "ops@example.com",
		BrowserLoginPassword: "secret",
		BalanceCents:         500_00,
		BalanceCurrency:      "USD",
	})
	_, err := service.EnqueueRun(context.Background(), RunInput{
		Mode:      "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeFetchBalance},
	})
	require.NoError(t, err)

	processed, err := service.ProcessNext(context.Background(), "worker-test")

	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, 1, sessionRefresher.loginCalls)
	require.Equal(t, 0, balanceSyncer.calls)
	require.Equal(t, "retryable_failed", repo.steps[0].Status)
	require.Contains(t, repo.steps[0].Reason, "SUPPLIER_DIRECT_LOGIN_UPSTREAM_ORIGIN_ERROR")
	require.Contains(t, repo.steps[0].Reason, `"login_attempted":"true"`)
}

func TestServiceProcessNextRefreshesSessionOnceAfterSyncExpired(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	balanceSyncer := &stubBalanceSyncer{
		total:  1,
		errors: []error{infraerrors.New(http.StatusConflict, "SUPPLIER_SESSION_EXPIRED", "supplier browser session is expired")},
	}
	sessionRefresher := &stubSessionRefresher{
		loginResult: &sessionsapp.LoginResult{Session: &adminplusdomain.SupplierBrowserSession{
			SupplierID:     1,
			SessionSource:  adminplusdomain.SupplierSessionSourceDirectLogin,
			Origin:         "https://relay-a.example.com",
			APIBaseURL:     "https://relay-a.example.com",
			SessionSummary: map[string]any{"session_source": "direct_login"},
			CapturedAt:     time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC),
		}},
	}
	repo := newFakeSchedulerRepository()
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, balanceSyncer, nil, nil, nil).
		WithSessionRefresher(sessionRefresher)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:                 "relay-a",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:        adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:         adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:         "https://relay-a.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: "ops@example.com",
		BrowserLoginPassword: "secret",
		BalanceCents:         500_00,
		BalanceCurrency:      "USD",
	})
	_, err := service.EnqueueRun(context.Background(), RunInput{
		Mode:      "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeFetchBalance},
	})
	require.NoError(t, err)

	processed, err := service.ProcessNext(context.Background(), "worker-test")

	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, 1, sessionRefresher.loginCalls)
	require.Equal(t, 2, balanceSyncer.calls)
	require.Equal(t, "succeeded", repo.steps[0].Status)
	require.Empty(t, repo.steps[0].Reason)
}

func TestServiceProcessNextDoesNotDirectLoginAfterNewAPIAdminPermissionFailure(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	balanceSyncer := &stubBalanceSyncer{
		total: 1,
		errors: []error{infraerrors.New(
			http.StatusForbidden,
			"SUPPLIER_NEW_API_ADMIN_SESSION_REQUIRED",
			"new api session cannot access requested endpoint",
		)},
	}
	sessionRefresher := &stubSessionRefresher{}
	repo := newFakeSchedulerRepository()
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, balanceSyncer, nil, nil, nil).
		WithSessionRefresher(sessionRefresher)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:                 "new-api-user-session",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeNewAPI,
		RuntimeStatus:        adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:         adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:         "https://new-api.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: "user@example.com",
		BrowserLoginPassword: "secret",
		BalanceCents:         500_00,
		BalanceCurrency:      "USD",
	})
	_, err := service.EnqueueRun(context.Background(), RunInput{
		Mode:      "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeFetchBalance},
	})
	require.NoError(t, err)

	processed, err := service.ProcessNext(context.Background(), "worker-test")

	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, 0, sessionRefresher.loginCalls)
	require.Equal(t, 1, balanceSyncer.calls)
	require.Equal(t, "manual_required", repo.steps[0].Status)
	require.Contains(t, repo.steps[0].Reason, "SUPPLIER_NEW_API_ADMIN_SESSION_REQUIRED")
	require.NotContains(t, repo.steps[0].Reason, "SUPPLIER_DIRECT_LOGIN_ADMIN_REQUIRED")
	require.NotContains(t, repo.steps[0].Reason, `"login_attempted":"true"`)
}

func TestServiceRetryStepRequeuesFailedStep(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	repo := newFakeSchedulerRepository()
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, &stubBalanceSyncer{total: 1}, nil, nil, nil)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:            "relay-a",
		Kind:            adminplusdomain.SupplierKindRelay,
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:    "https://relay-a.example.com",
		BalanceCents:    500_00,
		BalanceCurrency: "USD",
	})
	summary, err := service.EnqueueRun(context.Background(), RunInput{
		Mode:      "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeFetchBalance},
	})
	require.NoError(t, err)
	require.Len(t, repo.steps, 1)
	repo.steps[0].Status = "running"
	require.NoError(t, repo.CompleteStep(context.Background(), repo.steps[0].ID, "retryable_failed", 0, "upstream_500", nil, service.now()))
	require.NoError(t, repo.RefreshRunStatus(context.Background(), summary.ID, service.now()))
	require.Equal(t, "retryable_failed", repo.runs[0].Status)

	step, err := service.RetryStep(context.Background(), repo.steps[0].ID)

	require.NoError(t, err)
	require.Equal(t, "queued", step.Status)
	require.Empty(t, step.Reason)
	require.Equal(t, "running", repo.runs[0].Status)
}

func TestServiceCancelAndRetryFailedRunSteps(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	repo := newFakeSchedulerRepository()
	service := NewServiceWithDependenciesAndRepository(repo, supplierService, extensionService, nil, nil, &stubBalanceSyncer{total: 1}, nil, nil, nil)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:            "relay-a",
		Kind:            adminplusdomain.SupplierKindRelay,
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:    "https://relay-a.example.com",
		BalanceCents:    500_00,
		BalanceCurrency: "USD",
	})
	summary, err := service.EnqueueRun(context.Background(), RunInput{
		Mode:      "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeFetchBalance},
	})
	require.NoError(t, err)
	require.Len(t, repo.steps, 1)

	cancelledStep, err := service.CancelStep(context.Background(), repo.steps[0].ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", cancelledStep.Status)

	detail, err := service.RetryFailedSteps(context.Background(), summary.ID)
	require.NoError(t, err)
	require.Equal(t, "running", detail.Run.Status)
	require.Equal(t, "queued", repo.steps[0].Status)

	cancelledRun, err := service.CancelRun(context.Background(), summary.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", cancelledRun.Status)
	require.Equal(t, "cancelled", repo.steps[0].Status)

	processed, err := service.ProcessNext(context.Background(), "worker-test")
	require.NoError(t, err)
	require.False(t, processed)
}

func TestServiceRunKeepsNoBalanceSupplierOutOfSwitchOnlyTasks(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewServiceWithDependencies(
		supplierService,
		extensionService,
		&stubGroupSyncer{total: 1},
		&stubRateSyncer{total: 1},
		&stubBalanceSyncer{total: 1},
		&stubHealthSyncer{total: 1},
		nil,
		nil,
	)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	supplier := createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:                 "announcement-only",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:        adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:         adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:         "https://announcement-only.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: "ops@example.com",
		BalanceCents:         0,
		BalanceCurrency:      "CNY",
	})

	run, err := service.Run(context.Background(), RunInput{
		Mode: "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{
			adminplusdomain.ExtensionTaskTypeFetchRates,
			adminplusdomain.ExtensionTaskTypeFetchGroups,
			adminplusdomain.ExtensionTaskTypeFetchBalance,
			adminplusdomain.ExtensionTaskTypeFetchHealth,
			adminplusdomain.ExtensionTaskTypeFetchUsageCosts,
		},
	})
	require.NoError(t, err)
	require.Equal(t, 0, run.CreatedCount)
	require.Equal(t, 3, run.EligibleCount)
	require.Equal(t, 2, run.SkippedCount)

	reasons := make(map[adminplusdomain.ExtensionTaskType]string)
	for _, item := range run.Items {
		reasons[item.TaskType] = item.Reason
	}
	require.Empty(t, reasons[adminplusdomain.ExtensionTaskTypeFetchGroups])
	require.Empty(t, reasons[adminplusdomain.ExtensionTaskTypeFetchRates])
	require.Empty(t, reasons[adminplusdomain.ExtensionTaskTypeFetchBalance])
	require.Equal(t, "not_switch_eligible", reasons[adminplusdomain.ExtensionTaskTypeFetchHealth])
	require.Equal(t, "not_switch_eligible", reasons[adminplusdomain.ExtensionTaskTypeFetchUsageCosts])
	for _, item := range run.Items {
		if item.TaskType == adminplusdomain.ExtensionTaskTypeFetchGroups || item.TaskType == adminplusdomain.ExtensionTaskTypeFetchRates || item.TaskType == adminplusdomain.ExtensionTaskTypeFetchBalance {
			require.Equal(t, actionDirectSync, item.Action)
			require.True(t, item.Synced)
		}
	}

	tasks, err := extensionService.ListTasks(context.Background(), extensionapp.TaskFilter{SupplierID: supplier.ID, Limit: 20})
	require.NoError(t, err)
	require.Empty(t, tasks)
}

func TestServiceRunSkipsExtensionTaskWithoutBrowserCredential(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(supplierService, extensionService)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:                 "missing-browser",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:        adminplusdomain.SupplierRuntimeStatusActive,
		HealthStatus:         adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:         "https://missing-browser.example.com",
		BrowserLoginEnabled:  false,
		BrowserLoginUsername: "ops@example.com",
		BalanceCents:         500_00,
		BalanceCurrency:      "CNY",
	})

	run, err := service.Run(context.Background(), RunInput{
		Mode:      "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeCaptureSession},
	})
	require.NoError(t, err)
	require.Equal(t, 0, run.CreatedCount)
	require.Equal(t, 1, run.SkippedCount)
	require.Equal(t, "browser_login_disabled", run.Items[0].Reason)
}

type stubGroupSyncer struct {
	calls int
	total int
}

func (s *stubGroupSyncer) Sync(_ context.Context, supplierID int64) (*suppliergroupsapp.SyncResult, error) {
	s.calls++
	return &suppliergroupsapp.SyncResult{SupplierID: supplierID, Total: s.total}, nil
}

type stubRateSyncer struct {
	calls int
	total int
}

func (s *stubRateSyncer) SyncFromSession(_ context.Context, in ratesapp.SyncFromSessionInput) (*ratesapp.SyncFromSessionResult, error) {
	s.calls++
	return &ratesapp.SyncFromSessionResult{SupplierID: in.SupplierID, Total: s.total}, nil
}

type stubBalanceSyncer struct {
	calls  int
	total  int
	errors []error
}

func (s *stubBalanceSyncer) SyncFromSession(_ context.Context, in balancesapp.SyncFromSessionInput) (*balancesapp.SyncFromSessionResult, error) {
	s.calls++
	if len(s.errors) > 0 {
		err := s.errors[0]
		s.errors = s.errors[1:]
		return nil, err
	}
	result := &balancesapp.SyncFromSessionResult{SupplierID: in.SupplierID}
	if s.total > 0 {
		result.Snapshot = &adminplusdomain.BalanceSnapshot{SupplierID: in.SupplierID}
	}
	return result, nil
}

type stubSessionRefresher struct {
	probeCalls     int
	loginCalls     int
	lastLoginInput sessionsapp.LoginInput
	probeErrors    []error
	loginResult    *sessionsapp.LoginResult
	loginErr       error
}

func (s *stubSessionRefresher) DecryptedProbeInput(_ context.Context, supplierID int64) (ports.SessionProbeInput, error) {
	s.probeCalls++
	if len(s.probeErrors) > 0 {
		err := s.probeErrors[0]
		s.probeErrors = s.probeErrors[1:]
		return ports.SessionProbeInput{}, err
	}
	return ports.SessionProbeInput{SupplierID: supplierID}, nil
}

func (s *stubSessionRefresher) Login(_ context.Context, in sessionsapp.LoginInput) (*sessionsapp.LoginResult, error) {
	s.loginCalls++
	s.lastLoginInput = in
	if s.loginErr != nil {
		return nil, s.loginErr
	}
	if s.loginResult != nil {
		return s.loginResult, nil
	}
	return &sessionsapp.LoginResult{Session: &adminplusdomain.SupplierBrowserSession{}}, nil
}

type stubCandidateSummaryReader struct {
	rows   []*adminplusdomain.LocalAccountOpsRow
	groups []*sub2apiapp.LocalSub2APIGroup
}

func (s *stubCandidateSummaryReader) ListLocalGroups(_ context.Context, _ int) ([]*sub2apiapp.LocalSub2APIGroup, error) {
	return s.groups, nil
}

func (s *stubCandidateSummaryReader) ListLocalAccountOps(_ context.Context, _ sub2apiapp.LocalAccountOpsFilter) ([]*adminplusdomain.LocalAccountOpsRow, error) {
	return s.rows, nil
}

type stubRoutingRefiller struct {
	calls   []sub2apiapp.RoutingRefillInput
	results map[int64]*sub2apiapp.RoutingRefillResult
	errs    map[int64]error
}

func (s *stubRoutingRefiller) RefillLocalGroup(_ context.Context, in sub2apiapp.RoutingRefillInput) (*sub2apiapp.RoutingRefillResult, error) {
	s.calls = append(s.calls, in)
	if s.errs != nil {
		if err := s.errs[in.LocalGroupID]; err != nil {
			return nil, err
		}
	}
	if s.results != nil {
		if result := s.results[in.LocalGroupID]; result != nil {
			return result, nil
		}
	}
	return &sub2apiapp.RoutingRefillResult{
		Action:       "refill_local_group",
		LocalGroupID: in.LocalGroupID,
		Platform:     in.Platform,
	}, nil
}

type stubActionRecommendationSyncer struct {
	inputs []actionsapp.GenerateInput
	err    error
}

func (s *stubActionRecommendationSyncer) Generate(_ context.Context, in actionsapp.GenerateInput) (*actionsapp.GenerateResult, error) {
	s.inputs = append(s.inputs, in)
	if s.err != nil {
		return nil, s.err
	}
	total := len(in.LocalGroupCapacity) + len(in.LocalAccountSchedule)
	return &actionsapp.GenerateResult{Total: total}, nil
}

type stubHealthSyncer struct {
	calls int
	total int
}

func (s *stubHealthSyncer) SyncFromSession(_ context.Context, in healthapp.SyncFromSessionInput) (*healthapp.SyncFromSessionResult, error) {
	s.calls++
	return &healthapp.SyncFromSessionResult{SupplierID: in.SupplierID, Total: s.total}, nil
}

type stubUsageCostSyncer struct {
	calls     int
	total     int
	startedAt time.Time
	endedAt   time.Time
}

func (s *stubUsageCostSyncer) SyncFromSession(_ context.Context, in usagecostsapp.SyncFromSessionInput) (*usagecostsapp.SyncFromSessionResult, error) {
	s.calls++
	s.startedAt = in.StartedAt
	s.endedAt = in.EndedAt
	return &usagecostsapp.SyncFromSessionResult{SupplierID: in.SupplierID, Total: s.total}, nil
}

type stubCostSyncer struct {
	calls  int
	input  costsapp.SyncInput
	errors []error
}

func (s *stubCostSyncer) Sync(_ context.Context, in costsapp.SyncInput) (*costsapp.SyncResult, error) {
	s.calls++
	s.input = in
	if len(s.errors) > 0 {
		err := s.errors[0]
		s.errors = s.errors[1:]
		return nil, err
	}
	return &costsapp.SyncResult{
		SupplierID:              in.SupplierID,
		ProviderType:            "sub2api",
		SyncedAt:                time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC),
		FundingTransactions:     2,
		EntitlementTransactions: 3,
		UsageCostLines:          4,
		LedgerEntries:           5,
		Capabilities:            map[string]bool{"funding_transactions": in.IncludeFundingTransactions},
	}, nil
}

type stubPurityChecker struct {
	calls int
	input purityapp.AccountCheckInput
}

func (s *stubPurityChecker) RunAccountCheck(_ context.Context, in purityapp.AccountCheckInput) (*purityapp.PublicReport, error) {
	s.calls++
	s.input = in
	return &purityapp.PublicReport{
		Provider:           in.Provider,
		ReportID:           "purity-report-1",
		ModelID:            in.ModelID,
		Status:             purityapp.RunStatusDone,
		Verdict:            purityapp.VerdictOfficialOpenAI,
		Score:              90,
		Total:              100,
		OfficialScore:      80,
		CompatibilityScore: 10,
		Summary:            "ok",
		CheckedAt:          time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC),
		Metrics: purityapp.PublicCheckMetrics{
			Usage: &purityapp.TokenUsage{
				InputTokens:  100,
				OutputTokens: 20,
				CachedTokens: 40,
			},
		},
	}, nil
}

func createSchedulerSupplier(t *testing.T, service *suppliersapp.Service, in suppliersapp.CreateSupplierInput) *adminplusdomain.Supplier {
	t.Helper()
	supplier, err := service.Create(context.Background(), in)
	require.NoError(t, err)
	require.NotZero(t, supplier.ID)
	return supplier
}

func collectPlanTaskTypes(plans []adminplusdomain.SchedulerPlan) []string {
	out := make([]string, 0, len(plans))
	for _, plan := range plans {
		out = append(out, plan.TaskType)
	}
	return out
}

func requirePlan(t *testing.T, plans []adminplusdomain.SchedulerPlan, id string) adminplusdomain.SchedulerPlan {
	t.Helper()
	for _, plan := range plans {
		if plan.ID == id {
			return plan
		}
	}
	require.Failf(t, "plan not found", "plan %s was not found", id)
	return adminplusdomain.SchedulerPlan{}
}

func checklistItemStatus(items []adminplusdomain.SchedulerSupplierChecklistItem, key string) string {
	for _, item := range items {
		if item.Key == key {
			return item.Status
		}
	}
	return ""
}

type fakeSchedulerRepository struct {
	runs     []adminplusdomain.SchedulerRunSummary
	steps    []adminplusdomain.SchedulerStepRecord
	attempts []adminplusdomain.SchedulerAttemptRecord
	plans    []adminplusdomain.SchedulerPlan
	settings *adminplusdomain.SchedulerSettings
	actions  []adminplusdomain.SchedulerAction
}

func newFakeSchedulerRepository() *fakeSchedulerRepository {
	return &fakeSchedulerRepository{
		runs:     make([]adminplusdomain.SchedulerRunSummary, 0),
		steps:    make([]adminplusdomain.SchedulerStepRecord, 0),
		attempts: make([]adminplusdomain.SchedulerAttemptRecord, 0),
	}
}

func (r *fakeSchedulerRepository) SaveRun(_ context.Context, run adminplusdomain.SchedulerRunSummary, steps []adminplusdomain.ScheduledTask) error {
	r.runs = append(r.runs, run)
	for _, step := range steps {
		status := "queued"
		if step.Reason != "" {
			status = "skipped"
		}
		r.steps = append(r.steps, adminplusdomain.SchedulerStepRecord{
			ID:              int64(len(r.steps) + 1),
			RunID:           run.ID,
			SupplierID:      step.SupplierID,
			SupplierName:    step.SupplierName,
			TaskType:        step.TaskType,
			Action:          step.Action,
			Status:          status,
			ScheduleKey:     step.ScheduleKey,
			Reason:          step.Reason,
			RequestSnapshot: cloneTestMap(step.Request),
			ResultSnapshot:  cloneTestMap(step.Result),
		})
	}
	return nil
}

func cloneTestMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (r *fakeSchedulerRepository) ListRuns(_ context.Context, limit int, offset int, taskType string) ([]adminplusdomain.SchedulerRunSummary, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > len(r.runs) {
		limit = len(r.runs)
	}
	if offset >= len(r.runs) {
		return []adminplusdomain.SchedulerRunSummary{}, nil
	}
	taskType = strings.TrimSpace(taskType)
	out := make([]adminplusdomain.SchedulerRunSummary, 0, limit)
	skipped := 0
	for i := len(r.runs) - 1; i >= 0 && len(out) < limit; i-- {
		if taskType != "" && r.runs[i].TaskType != taskType {
			continue
		}
		if skipped < offset {
			skipped++
			continue
		}
		out = append(out, r.runs[i])
	}
	return out, nil
}

func (r *fakeSchedulerRepository) GetRun(_ context.Context, runID string) (*adminplusdomain.SchedulerRunSummary, error) {
	for idx := range r.runs {
		if r.runs[idx].ID == runID {
			return &r.runs[idx], nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *fakeSchedulerRepository) ListSteps(_ context.Context, runID string, limit int, offset int) ([]adminplusdomain.SchedulerStepRecord, error) {
	out := make([]adminplusdomain.SchedulerStepRecord, 0)
	for _, step := range r.steps {
		if runID != "" && step.RunID != runID {
			continue
		}
		if offset > 0 {
			offset--
			continue
		}
		out = append(out, step)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (r *fakeSchedulerRepository) ListAttempts(_ context.Context, runID string, limit int) ([]adminplusdomain.SchedulerAttemptRecord, error) {
	out := make([]adminplusdomain.SchedulerAttemptRecord, 0)
	for _, attempt := range r.attempts {
		if runID != "" && attempt.RunID != runID {
			continue
		}
		out = append(out, attempt)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (r *fakeSchedulerRepository) DeleteRun(_ context.Context, runID string) (*adminplusdomain.SchedulerCleanupResult, error) {
	result := &adminplusdomain.SchedulerCleanupResult{RunID: runID}
	nextRuns := r.runs[:0]
	for _, run := range r.runs {
		if run.ID == runID {
			result.DeletedRuns++
			continue
		}
		nextRuns = append(nextRuns, run)
	}
	r.runs = nextRuns
	if result.DeletedRuns == 0 {
		return nil, sql.ErrNoRows
	}
	nextSteps := r.steps[:0]
	for _, step := range r.steps {
		if step.RunID == runID {
			result.DeletedSteps++
			continue
		}
		nextSteps = append(nextSteps, step)
	}
	r.steps = nextSteps
	return result, nil
}

func (r *fakeSchedulerRepository) DeleteRuns(_ context.Context, taskType string) (*adminplusdomain.SchedulerCleanupResult, error) {
	taskType = strings.TrimSpace(taskType)
	result := &adminplusdomain.SchedulerCleanupResult{RunID: "bulk"}
	if taskType != "" {
		result.RunID = "task_type:" + taskType
	}
	deletedRuns := map[string]struct{}{}
	nextRuns := r.runs[:0]
	for _, run := range r.runs {
		if (taskType == "" || run.TaskType == taskType) && run.Status != "queued" && run.Status != "running" {
			result.DeletedRuns++
			deletedRuns[run.ID] = struct{}{}
			continue
		}
		nextRuns = append(nextRuns, run)
	}
	r.runs = nextRuns
	nextSteps := r.steps[:0]
	for _, step := range r.steps {
		if _, ok := deletedRuns[step.RunID]; ok {
			result.DeletedSteps++
			continue
		}
		nextSteps = append(nextSteps, step)
	}
	r.steps = nextSteps
	return result, nil
}

func (r *fakeSchedulerRepository) RetryStep(_ context.Context, stepID int64, retryAt time.Time) (*adminplusdomain.SchedulerStepRecord, error) {
	for idx := range r.steps {
		if r.steps[idx].ID != stepID {
			continue
		}
		if r.steps[idx].Status != "retryable_failed" && r.steps[idx].Status != "manual_required" && r.steps[idx].Status != "dead" && r.steps[idx].Status != "skipped" && r.steps[idx].Status != "cancelled" {
			return nil, sql.ErrNoRows
		}
		r.steps[idx].Status = "queued"
		r.steps[idx].Reason = ""
		r.steps[idx].ResultCount = 0
		r.steps[idx].Attempts = 0
		r.steps[idx].NextAttemptAt = &retryAt
		r.steps[idx].StartedAt = nil
		r.steps[idx].FinishedAt = nil
		return &r.steps[idx], nil
	}
	return nil, sql.ErrNoRows
}

func (r *fakeSchedulerRepository) CancelStep(_ context.Context, stepID int64, cancelledAt time.Time) (*adminplusdomain.SchedulerStepRecord, error) {
	for idx := range r.steps {
		if r.steps[idx].ID != stepID {
			continue
		}
		if r.steps[idx].Status != "queued" && r.steps[idx].Status != "running" && r.steps[idx].Status != "retryable_failed" && r.steps[idx].Status != "manual_required" {
			return nil, sql.ErrNoRows
		}
		r.steps[idx].Status = "cancelled"
		r.steps[idx].Reason = "manual_cancelled"
		r.steps[idx].LockedBy = ""
		r.steps[idx].LockedUntil = nil
		r.steps[idx].FinishedAt = &cancelledAt
		return &r.steps[idx], nil
	}
	return nil, sql.ErrNoRows
}

func (r *fakeSchedulerRepository) CancelRun(_ context.Context, runID string, cancelledAt time.Time) (*adminplusdomain.SchedulerRunSummary, error) {
	for idx := range r.steps {
		if r.steps[idx].RunID != runID {
			continue
		}
		if r.steps[idx].Status == "queued" || r.steps[idx].Status == "running" || r.steps[idx].Status == "retryable_failed" || r.steps[idx].Status == "manual_required" {
			r.steps[idx].Status = "cancelled"
			r.steps[idx].Reason = "run_cancelled"
			r.steps[idx].FinishedAt = &cancelledAt
		}
	}
	for idx := range r.runs {
		if r.runs[idx].ID != runID {
			continue
		}
		if r.runs[idx].Status != "queued" && r.runs[idx].Status != "running" && r.runs[idx].Status != "retryable_failed" && r.runs[idx].Status != "partial_succeeded" && r.runs[idx].Status != "manual_required" {
			return nil, sql.ErrNoRows
		}
		r.runs[idx].Status = "cancelled"
		r.runs[idx].FinishedAt = &cancelledAt
		r.runs[idx].ErrorMessage = "manual_cancelled"
		return &r.runs[idx], nil
	}
	return nil, sql.ErrNoRows
}

func (r *fakeSchedulerRepository) RetryFailedSteps(_ context.Context, runID string, retryAt time.Time) (int, error) {
	affected := 0
	for idx := range r.steps {
		if r.steps[idx].RunID != runID {
			continue
		}
		if r.steps[idx].Status != "retryable_failed" && r.steps[idx].Status != "manual_required" && r.steps[idx].Status != "dead" && r.steps[idx].Status != "skipped" && r.steps[idx].Status != "cancelled" {
			continue
		}
		r.steps[idx].Status = "queued"
		r.steps[idx].Reason = ""
		r.steps[idx].ResultCount = 0
		r.steps[idx].Attempts = 0
		r.steps[idx].NextAttemptAt = &retryAt
		r.steps[idx].StartedAt = nil
		r.steps[idx].FinishedAt = nil
		affected++
	}
	return affected, nil
}

func (r *fakeSchedulerRepository) SavePlans(_ context.Context, plans []adminplusdomain.SchedulerPlan) error {
	for _, plan := range plans {
		found := false
		for idx := range r.plans {
			if r.plans[idx].ID != plan.ID {
				continue
			}
			stored := r.plans[idx]
			stored.Name = plan.Name
			stored.TaskType = plan.TaskType
			stored.TaskTypes = plan.TaskTypes
			stored.HighCost = plan.HighCost
			stored.Description = plan.Description
			r.plans[idx] = stored
			found = true
			break
		}
		if !found {
			r.plans = append(r.plans, plan)
		}
	}
	return nil
}

func (r *fakeSchedulerRepository) ListPlans(_ context.Context) ([]adminplusdomain.SchedulerPlan, error) {
	return append([]adminplusdomain.SchedulerPlan{}, r.plans...), nil
}

func (r *fakeSchedulerRepository) UpdatePlanStatus(_ context.Context, planID, status string) (*adminplusdomain.SchedulerPlan, error) {
	for idx := range r.plans {
		if r.plans[idx].ID != planID {
			continue
		}
		r.plans[idx].Status = status
		return &r.plans[idx], nil
	}
	return nil, sql.ErrNoRows
}

func (r *fakeSchedulerRepository) UpdatePlanConfig(_ context.Context, planID string, config adminplusdomain.SchedulerPlanConfig) (*adminplusdomain.SchedulerPlan, error) {
	for idx := range r.plans {
		if r.plans[idx].ID != planID {
			continue
		}
		applyPlanConfig(&r.plans[idx], config, time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC))
		return &r.plans[idx], nil
	}
	return nil, sql.ErrNoRows
}

func (r *fakeSchedulerRepository) PlanStats(_ context.Context, plans []adminplusdomain.SchedulerPlan) (map[string]adminplusdomain.SchedulerPlanStats, error) {
	out := make(map[string]adminplusdomain.SchedulerPlanStats, len(plans))
	for _, plan := range plans {
		stat := adminplusdomain.SchedulerPlanStats{PlanID: plan.ID}
		for idx := range r.steps {
			step := r.steps[idx]
			if !planHasTaskType(plan, string(step.TaskType)) {
				continue
			}
			if step.Status == "succeeded" && step.FinishedAt != nil {
				if stat.LastSuccessAt == nil || step.FinishedAt.After(*stat.LastSuccessAt) {
					value := *step.FinishedAt
					stat.LastSuccessAt = &value
				}
			}
		}
		for idx := range r.steps {
			step := r.steps[idx]
			if !planHasTaskType(plan, string(step.TaskType)) {
				continue
			}
			if step.Status != "retryable_failed" && step.Status != "manual_required" && step.Status != "dead" {
				continue
			}
			if stat.LastSuccessAt != nil && step.FinishedAt != nil && !step.FinishedAt.After(*stat.LastSuccessAt) {
				continue
			}
			stat.IssueCount++
			if step.FinishedAt != nil && (stat.LastIssueAt == nil || step.FinishedAt.After(*stat.LastIssueAt)) {
				value := *step.FinishedAt
				stat.LastIssueAt = &value
				stat.LastIssue = step.Reason
			}
		}
		out[plan.ID] = stat
	}
	return out, nil
}

func (r *fakeSchedulerRepository) SupplierLatestSteps(_ context.Context) (map[int64]map[adminplusdomain.ExtensionTaskType]adminplusdomain.SchedulerStepRecord, error) {
	out := make(map[int64]map[adminplusdomain.ExtensionTaskType]adminplusdomain.SchedulerStepRecord)
	for _, step := range r.steps {
		if step.SupplierID <= 0 {
			continue
		}
		if out[step.SupplierID] == nil {
			out[step.SupplierID] = make(map[adminplusdomain.ExtensionTaskType]adminplusdomain.SchedulerStepRecord)
		}
		current, ok := out[step.SupplierID][step.TaskType]
		if !ok || stepNewerThan(step, current) {
			out[step.SupplierID][step.TaskType] = step
		}
	}
	return out, nil
}

func stepNewerThan(next, current adminplusdomain.SchedulerStepRecord) bool {
	nextTime := stepComparableTime(next)
	currentTime := stepComparableTime(current)
	if nextTime.Equal(currentTime) {
		return next.ID > current.ID
	}
	return nextTime.After(currentTime)
}

func stepComparableTime(step adminplusdomain.SchedulerStepRecord) time.Time {
	if step.FinishedAt != nil {
		return *step.FinishedAt
	}
	if step.StartedAt != nil {
		return *step.StartedAt
	}
	if step.NextAttemptAt != nil {
		return *step.NextAttemptAt
	}
	return time.Time{}
}

func planHasTaskType(plan adminplusdomain.SchedulerPlan, taskType string) bool {
	for _, value := range plan.TaskTypes {
		if value == taskType {
			return true
		}
	}
	return plan.TaskType == taskType
}

func (r *fakeSchedulerRepository) ClaimDuePlan(_ context.Context, now time.Time) (*adminplusdomain.SchedulerPlan, error) {
	for idx := range r.plans {
		plan := &r.plans[idx]
		if plan.Status != "enabled" || plan.IntervalSeconds <= 0 {
			continue
		}
		if plan.NextRunAt != nil && plan.NextRunAt.After(now) {
			continue
		}
		plan.LastRunAt = &now
		next := now.Add(time.Duration(plan.IntervalSeconds) * time.Second)
		plan.NextRunAt = &next
		return plan, nil
	}
	return nil, nil
}

func (r *fakeSchedulerRepository) SaveSettings(_ context.Context, settings adminplusdomain.SchedulerSettings) error {
	r.settings = &settings
	return nil
}

func (r *fakeSchedulerRepository) LoadSettings(_ context.Context) (*adminplusdomain.SchedulerSettings, error) {
	return r.settings, nil
}

func (r *fakeSchedulerRepository) UpsertActions(_ context.Context, actions []adminplusdomain.SchedulerAction) error {
	for _, action := range actions {
		found := false
		for idx := range r.actions {
			if r.actions[idx].ID != action.ID {
				continue
			}
			if r.actions[idx].Status != "resolved" && r.actions[idx].Status != "ignored" {
				r.actions[idx] = action
			}
			found = true
			break
		}
		if !found {
			r.actions = append(r.actions, action)
		}
	}
	return nil
}

func (r *fakeSchedulerRepository) ListActions(_ context.Context) ([]adminplusdomain.SchedulerAction, error) {
	out := make([]adminplusdomain.SchedulerAction, 0, len(r.actions))
	for _, action := range r.actions {
		if action.Status == "resolved" || action.Status == "ignored" {
			continue
		}
		out = append(out, action)
	}
	return out, nil
}

func (r *fakeSchedulerRepository) UpdateActionStatus(_ context.Context, actionID, status string, resolvedAt *time.Time) (*adminplusdomain.SchedulerAction, error) {
	for idx := range r.actions {
		if r.actions[idx].ID != actionID {
			continue
		}
		r.actions[idx].Status = status
		r.actions[idx].ResolvedAt = resolvedAt
		return &r.actions[idx], nil
	}
	return nil, sql.ErrNoRows
}

func (r *fakeSchedulerRepository) StepStats(_ context.Context) (running int, queued int, failed int, err error) {
	for _, step := range r.steps {
		switch step.Status {
		case "running":
			running++
		case "queued", "retryable_failed":
			queued++
		case "manual_required", "dead":
			failed++
		}
	}
	return
}

func (r *fakeSchedulerRepository) ClaimStep(_ context.Context, _ string, _ time.Duration) (*adminplusdomain.SchedulerStepRecord, error) {
	for idx := range r.steps {
		if (r.steps[idx].Status == "queued" || r.steps[idx].Status == "retryable_failed") && r.runClaimable(r.steps[idx].RunID) {
			r.steps[idx].Status = "running"
			return &r.steps[idx], nil
		}
	}
	return nil, nil
}

func (r *fakeSchedulerRepository) CompleteStep(_ context.Context, stepID int64, status string, resultCount int, reason string, result map[string]any, finishedAt time.Time) error {
	for idx := range r.steps {
		if r.steps[idx].ID != stepID {
			continue
		}
		if r.steps[idx].Status != "running" {
			continue
		}
		r.steps[idx].Status = status
		r.steps[idx].ResultCount = resultCount
		r.steps[idx].Reason = reason
		r.steps[idx].ResultSnapshot = cloneTestMap(result)
		r.steps[idx].FinishedAt = &finishedAt
	}
	return nil
}

func (r *fakeSchedulerRepository) runClaimable(runID string) bool {
	for idx := range r.runs {
		if r.runs[idx].ID != runID {
			continue
		}
		return r.runs[idx].Status == "queued" ||
			r.runs[idx].Status == "running" ||
			r.runs[idx].Status == "retryable_failed" ||
			r.runs[idx].Status == "partial_succeeded" ||
			r.runs[idx].Status == "manual_required"
	}
	return false
}

func (r *fakeSchedulerRepository) RefreshRunStatus(_ context.Context, runID string, finishedAt time.Time) error {
	for idx := range r.runs {
		if r.runs[idx].ID != runID {
			continue
		}
		total := 0
		succeeded := 0
		failed := 0
		skipped := 0
		cancelled := 0
		active := 0
		for _, step := range r.steps {
			if step.RunID != runID {
				continue
			}
			total++
			switch step.Status {
			case "succeeded":
				succeeded++
			case "skipped":
				skipped++
			case "cancelled":
				cancelled++
			case "queued", "running":
				active++
			case "retryable_failed", "manual_required", "dead":
				failed++
			}
		}
		r.runs[idx].TotalSteps = total
		r.runs[idx].SucceededSteps = succeeded
		r.runs[idx].FailedSteps = failed
		r.runs[idx].SkippedSteps = skipped
		if total == 0 {
			// keep current run status
		} else if active > 0 {
			r.runs[idx].Status = "running"
		} else if cancelled == total {
			r.runs[idx].Status = "cancelled"
		} else if failed > 0 && succeeded > 0 {
			r.runs[idx].Status = "partial_succeeded"
		} else if failed > 0 {
			r.runs[idx].Status = "retryable_failed"
		} else if succeeded == total {
			r.runs[idx].Status = "succeeded"
		} else if skipped == total {
			r.runs[idx].Status = "skipped"
		} else if succeeded > 0 {
			r.runs[idx].Status = "partial_succeeded"
		} else if cancelled > 0 {
			r.runs[idx].Status = "cancelled"
		}
		r.runs[idx].FinishedAt = &finishedAt
	}
	return nil
}
