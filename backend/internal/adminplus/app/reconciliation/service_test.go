package reconciliation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func TestServiceRunMatchesByExternalRequestIDAndCalculatesProfit(t *testing.T) {
	svc := NewService()
	startedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	result, err := svc.Run(context.Background(), RunInput{
		SupplierBills: []*adminplusdomain.SupplierBillLine{
			{
				ID:                11,
				SupplierID:        7,
				ExternalRequestID: "req-1",
				Model:             "gpt-4o-mini",
				Currency:          "USD",
				CostCents:         100,
				StartedAt:         startedAt,
			},
		},
		LocalUsages: []*adminplusdomain.LocalUsageLine{
			{
				ID:                21,
				ExternalRequestID: "req-1",
				Model:             "gpt-4o-mini",
				Currency:          "USD",
				RevenueCents:      150,
				StartedAt:         startedAt,
			},
		},
	})

	require.NoError(t, err)
	require.Len(t, result.Lines, 1)
	line := result.Lines[0]
	require.Equal(t, adminplusdomain.ReconciliationStatusMatched, line.Status)
	require.Equal(t, int64(100), line.CostCents)
	require.Equal(t, int64(150), line.RevenueCents)
	require.Equal(t, int64(50), line.ProfitCents)
	require.NotNil(t, line.ProfitMargin)
	require.InDelta(t, 0.3333, *line.ProfitMargin, 0.001)
	require.Equal(t, int64(50), result.Summary.ProfitCents)
}

func TestServiceRunMatchesByModelAndTimeWindow(t *testing.T) {
	svc := NewService()
	startedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	result, err := svc.Run(context.Background(), RunInput{
		TimeTolerance: 2 * time.Minute,
		SupplierBills: []*adminplusdomain.SupplierBillLine{
			{
				ID:         11,
				SupplierID: 7,
				Model:      "claude-sonnet-4",
				Currency:   "USD",
				CostCents:  200,
				StartedAt:  startedAt,
			},
		},
		LocalUsages: []*adminplusdomain.LocalUsageLine{
			{
				ID:           21,
				Model:        "claude-sonnet-4",
				Currency:     "USD",
				RevenueCents: 320,
				StartedAt:    startedAt.Add(time.Minute),
			},
		},
	})

	require.NoError(t, err)
	require.Len(t, result.Lines, 1)
	require.Equal(t, adminplusdomain.ReconciliationStatusMatched, result.Lines[0].Status)
}

func TestServiceRunEmitsSupplierOnlyAndLocalOnlyLines(t *testing.T) {
	svc := NewService()
	startedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	result, err := svc.Run(context.Background(), RunInput{
		SupplierBills: []*adminplusdomain.SupplierBillLine{
			{ID: 11, SupplierID: 7, Model: "gpt-4o-mini", Currency: "USD", CostCents: 100, StartedAt: startedAt},
		},
		LocalUsages: []*adminplusdomain.LocalUsageLine{
			{ID: 21, Model: "gemini-2.5-pro", Currency: "USD", RevenueCents: 200, StartedAt: startedAt},
		},
	})

	require.NoError(t, err)
	require.Len(t, result.Lines, 2)
	require.Equal(t, adminplusdomain.ReconciliationStatusSupplierOnly, result.Lines[0].Status)
	require.Equal(t, adminplusdomain.ReconciliationStatusLocalOnly, result.Lines[1].Status)
	require.Equal(t, int64(100), result.Summary.CostCents)
	require.Equal(t, int64(200), result.Summary.RevenueCents)
	require.Equal(t, int64(100), result.Summary.ProfitCents)
}

func TestServiceRunMarksCurrencyMismatch(t *testing.T) {
	svc := NewService()
	startedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	result, err := svc.Run(context.Background(), RunInput{
		SupplierBills: []*adminplusdomain.SupplierBillLine{
			{ID: 11, SupplierID: 7, ExternalRequestID: "req-1", Model: "gpt-4o-mini", Currency: "USD", CostCents: 100, StartedAt: startedAt},
		},
		LocalUsages: []*adminplusdomain.LocalUsageLine{
			{ID: 21, ExternalRequestID: "req-1", Model: "gpt-4o-mini", Currency: "CNY", RevenueCents: 200, StartedAt: startedAt},
		},
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ReconciliationStatusCurrencyMismatch, result.Lines[0].Status)
}

func TestServiceRunMarksRevenueBelowCostAsCostMismatch(t *testing.T) {
	svc := NewService()
	startedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	result, err := svc.Run(context.Background(), RunInput{
		SupplierBills: []*adminplusdomain.SupplierBillLine{
			{ID: 11, SupplierID: 7, ExternalRequestID: "req-1", Model: "gpt-4o-mini", Currency: "USD", CostCents: 200, StartedAt: startedAt},
		},
		LocalUsages: []*adminplusdomain.LocalUsageLine{
			{ID: 21, ExternalRequestID: "req-1", Model: "gpt-4o-mini", Currency: "USD", RevenueCents: 150, StartedAt: startedAt},
		},
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ReconciliationStatusCostMismatch, result.Lines[0].Status)
	require.Equal(t, int64(-50), result.Lines[0].ProfitCents)
}

func TestServiceRunDoesNotNotifyWhenReconciliationHasNoAnomaly(t *testing.T) {
	requests := withFeishuWebhookServer(t, http.StatusOK)
	repo := notifications.NewMemoryRepository()
	svc := NewServiceWithNotifier(NewFeishuNotifierFromEnv(repo))
	startedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	result, err := svc.Run(context.Background(), RunInput{
		SupplierBills: []*adminplusdomain.SupplierBillLine{
			{ID: 11, SupplierID: 7, ExternalRequestID: "req-1", Model: "gpt-5.5", Currency: "USD", CostCents: 100, StartedAt: startedAt},
		},
		LocalUsages: []*adminplusdomain.LocalUsageLine{
			{ID: 21, ExternalRequestID: "req-1", Model: "gpt-5.5", Currency: "USD", RevenueCents: 150, StartedAt: startedAt},
		},
	})

	require.NoError(t, err)
	require.False(t, hasReconciliationAnomaly(result))
	require.Equal(t, 0, *requests)
	deliveries, err := repo.ListDeliveries(context.Background(), notifications.DeliveryFilter{Limit: 10})
	require.NoError(t, err)
	require.Empty(t, deliveries)
}

func TestServiceRunNotifiesReconciliationAnomalyOnce(t *testing.T) {
	requests := withFeishuWebhookServer(t, http.StatusOK)
	repo := notifications.NewMemoryRepository()
	svc := NewServiceWithNotifier(NewFeishuNotifierFromEnv(repo))
	startedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	input := RunInput{
		SupplierBills: []*adminplusdomain.SupplierBillLine{
			{ID: 11, SupplierID: 7, ExternalRequestID: "req-1", Model: "gpt-5.5", Currency: "USD", CostCents: 200, StartedAt: startedAt},
		},
		LocalUsages: []*adminplusdomain.LocalUsageLine{
			{ID: 21, ExternalRequestID: "req-1", Model: "gpt-5.5", Currency: "USD", RevenueCents: 150, StartedAt: startedAt},
		},
	}

	result, err := svc.Run(context.Background(), input)
	require.NoError(t, err)
	require.True(t, hasReconciliationAnomaly(result))

	_, err = svc.Run(context.Background(), input)
	require.NoError(t, err)

	require.Equal(t, 1, *requests)
	deliveries, err := repo.ListDeliveries(context.Background(), notifications.DeliveryFilter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, deliveries, 1)
	require.Equal(t, "reconciliation.anomaly", deliveries[0].EventType)
	require.Equal(t, int64(7), deliveries[0].SupplierID)
	require.Equal(t, adminplusdomain.NotificationStatusSucceeded, deliveries[0].Status)
	require.Contains(t, deliveries[0].DedupeKey, "feishu:reconciliation.anomaly:")
}

func TestBuildFeishuReconciliationTextIncludesAnomalyCounts(t *testing.T) {
	profitMargin := -0.25
	text := buildFeishuReconciliationText(&RunResult{
		Lines: []*adminplusdomain.ReconciliationLine{
			{Status: adminplusdomain.ReconciliationStatusSupplierOnly, Currency: "USD", CostCents: 200, ProfitCents: -200},
			{Status: adminplusdomain.ReconciliationStatusLocalOnly, Currency: "USD", RevenueCents: 150, ProfitCents: 150},
			{Status: adminplusdomain.ReconciliationStatusCostMismatch, Currency: "USD", CostCents: 200, RevenueCents: 150, ProfitCents: -50},
		},
		Summary: adminplusdomain.ReconciliationSummary{
			TotalSupplierLines: 2,
			TotalLocalLines:    2,
			CostCents:          400,
			RevenueCents:       300,
			ProfitCents:        -100,
			ProfitMargin:       &profitMargin,
		},
	})

	require.Contains(t, text, "供应商单边：1")
	require.Contains(t, text, "本地单边：1")
	require.Contains(t, text, "成本异常：1")
	require.Contains(t, text, "利润：-1.00 USD")
	require.Contains(t, text, "利润率：-25.00%")
}

func withFeishuWebhookServer(t *testing.T, statusCode int) *int {
	t.Helper()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(statusCode)
	}))
	t.Cleanup(server.Close)
	t.Setenv("ADMIN_PLUS_FEISHU_WEBHOOK_URL", server.URL)
	t.Setenv("ADMIN_PLUS_FEISHU_WEBHOOK_SECRET", "")
	t.Setenv("ADMIN_PLUS_FEISHU_BALANCE_WEBHOOK_URL", "")
	t.Setenv("ADMIN_PLUS_FEISHU_BALANCE_WEBHOOK_SECRET", "")
	return &requests
}
