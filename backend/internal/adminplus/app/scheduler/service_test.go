package scheduler

import (
	"context"
	"testing"
	"time"

	announcementsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/announcements"
	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	healthapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/health"
	ratesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/rates"
	suppliergroupsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliergroups"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	usagecostsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/usagecosts"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
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
	announcementSyncer := &stubAnnouncementSyncer{total: 5}
	healthSyncer := &stubHealthSyncer{total: 1}
	usageCostSyncer := &stubUsageCostSyncer{total: 4}
	service := NewServiceWithDependencies(supplierService, extensionService, groupSyncer, rateSyncer, balanceSyncer, announcementSyncer, healthSyncer, usageCostSyncer)
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
			adminplusdomain.ExtensionTaskTypeFetchAnnouncements,
			adminplusdomain.ExtensionTaskTypeFetchHealth,
			adminplusdomain.ExtensionTaskTypeFetchUsageCosts,
		},
	})
	require.NoError(t, err)
	require.Equal(t, 0, run.CreatedCount)
	require.Equal(t, 6, run.EligibleCount)
	require.Equal(t, 0, run.SkippedCount)
	require.Len(t, run.Items, 6)
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
	require.Equal(t, 1, announcementSyncer.calls)
	require.Equal(t, 1, healthSyncer.calls)
	require.Equal(t, 1, usageCostSyncer.calls)
	require.Equal(t, time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC), usageCostSyncer.startedAt)
	require.Equal(t, time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC), usageCostSyncer.endedAt)

	tasks, err := extensionService.ListTasks(context.Background(), extensionapp.TaskFilter{SupplierID: supplier.ID, Limit: 20})
	require.NoError(t, err)
	require.Empty(t, tasks)
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

func TestServiceRunKeepsNoBalanceSupplierOutOfSwitchOnlyTasks(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewServiceWithDependencies(
		supplierService,
		extensionService,
		&stubGroupSyncer{total: 1},
		&stubRateSyncer{total: 1},
		&stubBalanceSyncer{total: 1},
		&stubAnnouncementSyncer{total: 1},
		&stubHealthSyncer{total: 1},
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
			adminplusdomain.ExtensionTaskTypeFetchAnnouncements,
			adminplusdomain.ExtensionTaskTypeFetchHealth,
			adminplusdomain.ExtensionTaskTypeFetchUsageCosts,
		},
	})
	require.NoError(t, err)
	require.Equal(t, 0, run.CreatedCount)
	require.Equal(t, 4, run.EligibleCount)
	require.Equal(t, 2, run.SkippedCount)

	reasons := make(map[adminplusdomain.ExtensionTaskType]string)
	for _, item := range run.Items {
		reasons[item.TaskType] = item.Reason
	}
	require.Empty(t, reasons[adminplusdomain.ExtensionTaskTypeFetchGroups])
	require.Empty(t, reasons[adminplusdomain.ExtensionTaskTypeFetchRates])
	require.Empty(t, reasons[adminplusdomain.ExtensionTaskTypeFetchBalance])
	require.Empty(t, reasons[adminplusdomain.ExtensionTaskTypeFetchAnnouncements])
	require.Equal(t, "not_switch_eligible", reasons[adminplusdomain.ExtensionTaskTypeFetchHealth])
	require.Equal(t, "not_switch_eligible", reasons[adminplusdomain.ExtensionTaskTypeFetchUsageCosts])
	for _, item := range run.Items {
		if item.TaskType == adminplusdomain.ExtensionTaskTypeFetchGroups || item.TaskType == adminplusdomain.ExtensionTaskTypeFetchRates || item.TaskType == adminplusdomain.ExtensionTaskTypeFetchBalance || item.TaskType == adminplusdomain.ExtensionTaskTypeFetchAnnouncements {
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
	calls int
	total int
}

func (s *stubBalanceSyncer) SyncFromSession(_ context.Context, in balancesapp.SyncFromSessionInput) (*balancesapp.SyncFromSessionResult, error) {
	s.calls++
	result := &balancesapp.SyncFromSessionResult{SupplierID: in.SupplierID}
	if s.total > 0 {
		result.Snapshot = &adminplusdomain.BalanceSnapshot{SupplierID: in.SupplierID}
	}
	return result, nil
}

type stubAnnouncementSyncer struct {
	calls int
	total int
}

func (s *stubAnnouncementSyncer) SyncFromSession(_ context.Context, in announcementsapp.SyncFromSessionInput) (*announcementsapp.SyncFromSessionResult, error) {
	s.calls++
	return &announcementsapp.SyncFromSessionResult{SupplierID: in.SupplierID, Total: s.total}, nil
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

func createSchedulerSupplier(t *testing.T, service *suppliersapp.Service, in suppliersapp.CreateSupplierInput) *adminplusdomain.Supplier {
	t.Helper()
	supplier, err := service.Create(context.Background(), in)
	require.NoError(t, err)
	require.NotZero(t, supplier.ID)
	return supplier
}
