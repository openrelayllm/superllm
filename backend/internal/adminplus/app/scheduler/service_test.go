package scheduler

import (
	"context"
	"testing"
	"time"

	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
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
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeFetchRates, adminplusdomain.ExtensionTaskTypeFetchHealth},
	})
	require.NoError(t, err)
	require.Equal(t, 2, first.CreatedCount)
	require.Equal(t, 0, first.SkippedCount)
	require.Len(t, first.Items, 2)
	require.NotEmpty(t, first.Items[0].ScheduleKey)

	tasks, err := extensionService.ListTasks(context.Background(), extensionapp.TaskFilter{SupplierID: supplier.ID, Limit: 20})
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	require.Contains(t, tasks[0].Payload, "schedule_key")

	second, err := service.Run(context.Background(), RunInput{
		Mode:      "manual",
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeFetchRates, adminplusdomain.ExtensionTaskTypeFetchHealth},
	})
	require.NoError(t, err)
	require.Equal(t, 0, second.CreatedCount)
	require.Equal(t, 2, second.SkippedCount)
	require.Len(t, second.Items, 2)
	for _, item := range second.Items {
		require.False(t, item.Created)
		require.Equal(t, "duplicate", item.Reason)
		require.NotZero(t, item.TaskID)
	}
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
	require.NotEmpty(t, run.Items[0].ScheduleKey)

	tasks, err := extensionService.ListTasks(context.Background(), extensionapp.TaskFilter{SupplierID: supplier.ID, Limit: 20})
	require.NoError(t, err)
	require.Empty(t, tasks)
}

func TestServiceRunKeepsNoBalanceSupplierOutOfSwitchOnlyTasks(t *testing.T) {
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(supplierService, extensionService)
	service.now = func() time.Time {
		return time.Date(2026, 6, 20, 10, 4, 0, 0, time.UTC)
	}

	supplier := createSchedulerSupplier(t, supplierService, suppliersapp.CreateSupplierInput{
		Name:                 "promotion-only",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:        adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:         adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:         "https://promotion-only.example.com",
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
			adminplusdomain.ExtensionTaskTypeFetchPromotions,
			adminplusdomain.ExtensionTaskTypeFetchHealth,
			adminplusdomain.ExtensionTaskTypeExportBills,
		},
	})
	require.NoError(t, err)
	require.Equal(t, 4, run.CreatedCount)
	require.Equal(t, 2, run.SkippedCount)

	reasons := make(map[adminplusdomain.ExtensionTaskType]string)
	for _, item := range run.Items {
		reasons[item.TaskType] = item.Reason
	}
	require.Empty(t, reasons[adminplusdomain.ExtensionTaskTypeFetchRates])
	require.Empty(t, reasons[adminplusdomain.ExtensionTaskTypeFetchGroups])
	require.Empty(t, reasons[adminplusdomain.ExtensionTaskTypeFetchBalance])
	require.Empty(t, reasons[adminplusdomain.ExtensionTaskTypeFetchPromotions])
	require.Equal(t, "not_switch_eligible", reasons[adminplusdomain.ExtensionTaskTypeFetchHealth])
	require.Equal(t, "not_switch_eligible", reasons[adminplusdomain.ExtensionTaskTypeExportBills])

	tasks, err := extensionService.ListTasks(context.Background(), extensionapp.TaskFilter{SupplierID: supplier.ID, Limit: 20})
	require.NoError(t, err)
	require.Len(t, tasks, 4)
	for _, task := range tasks {
		require.NotEqual(t, adminplusdomain.ExtensionTaskTypeFetchHealth, task.Type)
		require.NotEqual(t, adminplusdomain.ExtensionTaskTypeExportBills, task.Type)
	}
}

func TestServiceRunSkipsSupplierWithoutBrowserCredential(t *testing.T) {
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
		TaskTypes: []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeFetchRates},
	})
	require.NoError(t, err)
	require.Equal(t, 0, run.CreatedCount)
	require.Equal(t, 1, run.SkippedCount)
	require.Equal(t, "browser_login_disabled", run.Items[0].Reason)
}

func createSchedulerSupplier(t *testing.T, service *suppliersapp.Service, in suppliersapp.CreateSupplierInput) *adminplusdomain.Supplier {
	t.Helper()
	supplier, err := service.Create(context.Background(), in)
	require.NoError(t, err)
	require.NotZero(t, supplier.ID)
	return supplier
}
