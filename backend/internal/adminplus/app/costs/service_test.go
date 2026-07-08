package costs

import (
	"context"
	"net/http"
	"testing"
	"time"

	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	usagecostsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/usagecosts"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServiceSyncRecordsFundingEntitlementsAndIdempotentLedger(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	paidAt := now.Add(-2 * time.Hour)
	manualRedeemedAt := now.Add(-time.Hour)
	session := &stubCostSessionReader{input: ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     "https://relay.example.com",
		APIBaseURL: "https://relay.example.com/api/v1",
		Bundle:     map[string]any{"access_token": "browser-token"},
	}}
	funding := &stubCostFundingReader{result: &ports.ReadFundingTransactionsResult{
		SupplierID:   7,
		ProviderType: "sub2api",
		SystemType:   "sub2api",
		Origin:       "https://relay.example.com",
		APIBaseURL:   "https://relay.example.com/api/v1",
		CapturedAt:   now,
		Items: []ports.ProviderFundingTransaction{
			{
				ExternalID:      "order-1",
				OutTradeNo:      "out-1",
				PaymentType:     "stripe",
				OrderType:       "balance",
				Status:          "paid",
				Currency:        "usd",
				AmountCents:     10000,
				CashAmountCents: 9500,
				PaidAt:          &paidAt,
				RawPayload:      map[string]any{"id": "order-1"},
			},
		},
	}}
	entitlements := &stubCostEntitlementReader{result: &ports.ReadEntitlementTransactionsResult{
		SupplierID:   7,
		ProviderType: "sub2api",
		SystemType:   "sub2api",
		Origin:       "https://relay.example.com",
		APIBaseURL:   "https://relay.example.com/api/v1",
		CapturedAt:   now,
		Items: []ports.ProviderEntitlementTransaction{
			{
				ExternalID:   "redeem-pay-1",
				SourceFamily: "payment_auto_redeem",
				Type:         "balance",
				Status:       "used",
				Currency:     "usd",
				ValueCents:   10000,
				RawValue:     100,
				UsedAt:       &manualRedeemedAt,
				CodeLast4:    "A001",
				RawPayload:   map[string]any{"code_last4": "A001"},
			},
			{
				ExternalID:   "redeem-manual-1",
				SourceFamily: "manual_redeem",
				Type:         "balance",
				Status:       "used",
				Currency:     "usd",
				ValueCents:   2500,
				RawValue:     25,
				UsedAt:       &manualRedeemedAt,
				CodeLast4:    "M001",
				RawPayload:   map[string]any{"code_last4": "M001"},
			},
			{
				ExternalID:   "redeem-concurrency-1",
				SourceFamily: "manual_redeem",
				Type:         "concurrency",
				Status:       "used",
				Currency:     "usd",
				ValueCents:   7000,
				RawValue:     70,
				UsedAt:       &manualRedeemedAt,
				CodeLast4:    "C001",
				RawPayload:   map[string]any{"code_last4": "C001"},
			},
		},
	}}
	svc := NewServiceWithDependencies(repo, session, funding, entitlements, nil, nil, nil)
	svc.now = func() time.Time { return now }

	result, err := svc.Sync(context.Background(), SyncInput{
		SupplierID:                     7,
		IncludeFundingTransactions:     true,
		IncludeEntitlementTransactions: true,
	})

	require.NoError(t, err)
	require.Equal(t, int64(7), session.supplierID)
	require.Equal(t, 1, result.FundingTransactions)
	require.Equal(t, 3, result.EntitlementTransactions)
	require.Equal(t, 2, result.LedgerEntries)
	require.NotNil(t, result.Snapshot)
	require.Equal(t, int64(10000), result.Snapshot.CompletedFundingAmountCents)
	require.Equal(t, int64(9500), result.Snapshot.CompletedFundingCashCents)
	require.Equal(t, int64(2500), result.Snapshot.EntitlementAmountCents)
	require.Equal(t, int64(12500), result.Snapshot.ExpectedBalanceCents)

	second, err := svc.Sync(context.Background(), SyncInput{
		SupplierID:                     7,
		IncludeFundingTransactions:     true,
		IncludeEntitlementTransactions: true,
	})

	require.NoError(t, err)
	require.Equal(t, 0, second.LedgerEntries)
	ledger, err := svc.ListLedgerEntries(context.Background(), LedgerFilter{SupplierID: 7, Limit: 10})
	require.NoError(t, err)
	require.Len(t, ledger, 2)
	require.ElementsMatch(t, []string{"funding_credit", "entitlement_credit"}, []string{ledger[0].EntryType, ledger[1].EntryType})

	entitlementItems, err := svc.ListEntitlementTransactions(context.Background(), TransactionFilter{SupplierID: 7, Limit: 10})
	require.NoError(t, err)
	require.Len(t, entitlementItems, 3)
	concurrency := findEntitlementByExternalID(t, entitlementItems, "redeem-concurrency-1")
	require.Equal(t, "concurrency", concurrency.Type)
	require.Equal(t, int64(0), concurrency.ValueCents)
	require.Equal(t, float64(70), concurrency.RawValue)
}

func TestServiceSyncCountsAutoRedeemEntitlementsWhenFundingIsMissing(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 6, 22, 1, 30, 0, 0, time.UTC)
	usedAt := now.Add(-30 * time.Minute)
	session := &stubCostSessionReader{}
	entitlements := &stubCostEntitlementReader{result: &ports.ReadEntitlementTransactionsResult{
		SupplierID:   7,
		ProviderType: "sub2api",
		Items: []ports.ProviderEntitlementTransaction{
			{
				ExternalID:   "auto-redeem-20",
				SourceFamily: "payment_auto_redeem",
				Type:         "balance",
				Status:       "used",
				Currency:     "usd",
				ValueCents:   2000,
				RawValue:     20,
				UsedAt:       &usedAt,
			},
			{
				ExternalID:   "auto-redeem-30",
				SourceFamily: "payment_auto_redeem",
				Type:         "balance",
				Status:       "used",
				Currency:     "usd",
				ValueCents:   3000,
				RawValue:     30,
				UsedAt:       &usedAt,
			},
			{
				ExternalID:   "auto-redeem-concurrency",
				SourceFamily: "manual_redeem",
				Type:         "concurrency",
				Status:       "used",
				Currency:     "usd",
				ValueCents:   7000,
				RawValue:     70,
				UsedAt:       &usedAt,
			},
		},
	}}
	svc := NewServiceWithDependencies(repo, session, nil, entitlements, nil, nil, nil)
	svc.now = func() time.Time { return now }

	result, err := svc.Sync(context.Background(), SyncInput{
		SupplierID:                     7,
		IncludeEntitlementTransactions: true,
	})

	require.NoError(t, err)
	require.Equal(t, 3, result.EntitlementTransactions)
	require.Equal(t, 0, result.LedgerEntries)
	require.NotNil(t, result.Snapshot)
	require.Equal(t, int64(0), result.Snapshot.CompletedFundingAmountCents)
	require.Equal(t, int64(5000), result.Snapshot.EntitlementAmountCents)
	require.Equal(t, int64(5000), result.Snapshot.ExpectedBalanceCents)
}

func TestServiceSyncKeepsGoingWhenOptionalCostCapabilitiesAreMissing(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 7, 1, 11, 0, 0, 0, time.UTC)
	balanceCents := int64(12000)
	session := &stubCostSessionReader{input: ports.SessionProbeInput{
		SupplierID: 7,
		Bundle: map[string]any{
			"provider_type": "new_api",
			"system_type":   "new_api",
		},
	}}
	funding := &stubCostFundingReader{err: infraerrors.New(http.StatusConflict, "SUPPLIER_FUNDING_CAPABILITY_MISSING", "supplier session cannot read funding transactions")}
	entitlements := &stubCostEntitlementReader{err: infraerrors.New(http.StatusConflict, "SUPPLIER_ENTITLEMENT_CAPABILITY_MISSING", "supplier session cannot read entitlement transactions")}
	balance := &stubCostBalanceSyncer{result: &balancesapp.SyncFromSessionResult{
		SystemType: "new_api",
		Snapshot: &adminplusdomain.BalanceSnapshot{
			SupplierID:    7,
			BalanceCents:  balanceCents,
			Currency:      "USD",
			CapturedAt:    now,
			RuntimeStatus: adminplusdomain.SupplierRuntimeStatusActive,
		},
	}}
	svc := NewServiceWithDependencies(repo, session, funding, entitlements, nil, balance, nil)
	svc.now = func() time.Time { return now }

	result, err := svc.Sync(context.Background(), SyncInput{
		SupplierID:                     7,
		IncludeFundingTransactions:     true,
		IncludeEntitlementTransactions: true,
		IncludeBalanceSnapshot:         true,
	})

	require.NoError(t, err)
	require.Equal(t, "new_api", result.ProviderType)
	require.Equal(t, "new_api", result.SystemType)
	require.Equal(t, "supplier session cannot read funding transactions", result.Diagnostics["funding_transactions"])
	require.Equal(t, "supplier session cannot read entitlement transactions", result.Diagnostics["entitlement_transactions"])
	require.True(t, result.Capabilities["balance_snapshot"])
	require.Equal(t, int64(7), balance.request.SupplierID)
}

func TestServiceSyncAppliesSupplierRechargeMultiplierToFundingCost(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 6, 22, 8, 0, 0, 0, time.UTC)
	session := &stubCostSessionReader{}
	funding := &stubCostFundingReader{result: &ports.ReadFundingTransactionsResult{
		SupplierID:   7,
		ProviderType: "sub2api",
		Items: []ports.ProviderFundingTransaction{
			{
				ExternalID:  "order-100",
				Status:      "paid",
				Currency:    "USD",
				AmountCents: 10000,
			},
		},
	}}
	supplierLookup := &stubCostSupplierLookup{
		supplier: &adminplusdomain.Supplier{
			ID:                 7,
			RechargeMultiplier: 10,
		},
	}
	svc := NewServiceWithDependencies(repo, session, funding, nil, nil, nil, supplierLookup)
	svc.now = func() time.Time { return now }

	result, err := svc.Sync(context.Background(), SyncInput{
		SupplierID:                 7,
		IncludeFundingTransactions: true,
	})

	require.NoError(t, err)
	require.NotNil(t, result.Snapshot)
	require.Equal(t, int64(10000), result.Snapshot.CompletedFundingAmountCents)
	require.Equal(t, int64(1000), result.Snapshot.RechargeActualPaymentCents)
	items, err := svc.ListFundingTransactions(context.Background(), TransactionFilter{SupplierID: 7, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, float64(10), items[0].RechargeMultiplier)
	require.Equal(t, int64(1000), items[0].ActualPaymentCents)
	ledger, err := svc.ListLedgerEntries(context.Background(), LedgerFilter{SupplierID: 7, Limit: 10})
	require.NoError(t, err)
	require.Len(t, ledger, 1)
	require.Equal(t, int64(1000), ledger[0].ActualPaymentCents)
}

func TestServiceApplyReconcileAdjustmentRecordsManualLedgerEntry(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	_, _, err := repo.UpsertLedgerEntry(context.Background(), &adminplusdomain.SupplierCostLedgerEntry{
		SupplierID:         7,
		ProviderType:       "sub2api",
		EntryType:          "funding_credit",
		SourceType:         "funding_transaction",
		SourceID:           11,
		SourceExternalID:   "order-11",
		Currency:           "USD",
		AmountCents:        10000,
		CashAmountCents:    10000,
		ActualPaymentCents: 10000,
		OccurredAt:         now.Add(-time.Hour),
	})
	require.NoError(t, err)
	before, err := repo.RefreshSnapshot(context.Background(), 7, "USD", now.Add(-30*time.Minute))
	require.NoError(t, err)
	actual := int64(12000)
	delta := int64(2000)
	repo.snapshots[before.ID].ActualBalanceCents = &actual
	repo.snapshots[before.ID].BalanceDeltaCents = &delta
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyReconcileAdjustment(context.Background(), ReconcileAdjustmentInput{
		SupplierID:             7,
		SnapshotID:             before.ID,
		ActionRecommendationID: 99,
		AdjustmentAmountCents:  ptrInt64(2000),
		Reason:                 "reviewed missing balance credit",
	})

	require.NoError(t, err)
	require.Equal(t, int64(2000), result.AdjustmentAmountCents)
	require.NotNil(t, result.LedgerEntry)
	require.Equal(t, "manual_adjustment", result.LedgerEntry.EntryType)
	require.Equal(t, "action_recommendation", result.LedgerEntry.SourceType)
	require.Equal(t, int64(99), result.LedgerEntry.SourceID)
	require.Equal(t, int64(12000), result.AfterSnapshot.ExpectedBalanceCents)
	require.Equal(t, int64(2000), result.AfterSnapshot.AdjustmentAmountCents)
	ledger, err := svc.ListLedgerEntries(context.Background(), LedgerFilter{SupplierID: 7, Limit: 10})
	require.NoError(t, err)
	require.Len(t, ledger, 2)
}

func TestServiceApplyReconcileDetailRepairRecordsFundingCredit(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 7, 9, 11, 0, 0, 0, time.UTC)
	before := seedReconcileSnapshot(t, repo, now, 10000, 12500, 2500)
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyReconcileDetailRepair(context.Background(), ReconcileDetailRepairInput{
		SupplierID:             7,
		SnapshotID:             before.ID,
		ActionRecommendationID: 101,
		DetailType:             "funding_credit",
		Reason:                 "missing recharge",
	})

	require.NoError(t, err)
	require.Equal(t, "funding_credit", result.DetailType)
	require.Equal(t, int64(2500), result.AmountCents)
	require.NotNil(t, result.FundingTransaction)
	require.Equal(t, "COMPLETED", result.FundingTransaction.Status)
	require.NotNil(t, result.LedgerEntry)
	require.Equal(t, "funding_credit", result.LedgerEntry.EntryType)
	require.Equal(t, int64(12500), result.AfterSnapshot.ExpectedBalanceCents)
	items, err := svc.ListFundingTransactions(context.Background(), TransactionFilter{SupplierID: 7, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
}

func TestServiceApplyReconcileDetailRepairRecordsEntitlementCredit(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 7, 9, 11, 15, 0, 0, time.UTC)
	before := seedReconcileSnapshot(t, repo, now, 10000, 13000, 3000)
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyReconcileDetailRepair(context.Background(), ReconcileDetailRepairInput{
		SupplierID:             7,
		SnapshotID:             before.ID,
		ActionRecommendationID: 102,
		DetailType:             "entitlement_credit",
		ExternalID:             "redeem-manual-102",
	})

	require.NoError(t, err)
	require.Equal(t, "entitlement_credit", result.DetailType)
	require.NotNil(t, result.EntitlementTransaction)
	require.Equal(t, "manual_redeem", result.EntitlementTransaction.SourceFamily)
	require.NotNil(t, result.LedgerEntry)
	require.Equal(t, "entitlement_credit", result.LedgerEntry.EntryType)
	require.Equal(t, int64(13000), result.AfterSnapshot.ExpectedBalanceCents)
	items, err := svc.ListEntitlementTransactions(context.Background(), TransactionFilter{SupplierID: 7, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
}

func TestServiceApplyReconcileDetailRepairRecordsRefundDebit(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 7, 9, 11, 30, 0, 0, time.UTC)
	before := seedReconcileSnapshot(t, repo, now, 10000, 8000, -2000)
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	result, err := svc.ApplyReconcileDetailRepair(context.Background(), ReconcileDetailRepairInput{
		SupplierID:             7,
		SnapshotID:             before.ID,
		ActionRecommendationID: 103,
		DetailType:             "refund_debit",
	})

	require.NoError(t, err)
	require.Equal(t, "refund_debit", result.DetailType)
	require.NotNil(t, result.FundingTransaction)
	require.Equal(t, "REFUNDED", result.FundingTransaction.Status)
	require.NotNil(t, result.LedgerEntry)
	require.Equal(t, "refund_debit", result.LedgerEntry.EntryType)
	require.Equal(t, int64(-2000), result.LedgerEntry.AmountCents)
	require.Equal(t, int64(8000), result.AfterSnapshot.ExpectedBalanceCents)
}

func TestServiceApplyReconcileDetailRepairImportsUsageCost(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 7, 9, 11, 45, 0, 0, time.UTC)
	before := seedReconcileSnapshot(t, repo, now, 10000, 9000, -1000)
	importer := &stubCostUsageImporter{}
	svc := NewService(repo)
	svc.now = func() time.Time { return now }
	svc.usageCostImporter = importer

	result, err := svc.ApplyReconcileDetailRepair(context.Background(), ReconcileDetailRepairInput{
		SupplierID:             7,
		SnapshotID:             before.ID,
		ActionRecommendationID: 104,
		DetailType:             "usage_cost",
		Model:                  "gpt-4.1-mini",
	})

	require.NoError(t, err)
	require.Equal(t, "usage_cost", result.DetailType)
	require.NotNil(t, result.UsageCostLine)
	require.Equal(t, int64(1000), result.UsageCostLine.CostCents)
	require.Equal(t, "manual_reconcile", result.UsageCostLine.Source)
	require.Len(t, importer.inputs, 1)
	require.Equal(t, "gpt-4.1-mini", importer.inputs[0].Model)
}

func TestServiceApplyReconcileDetailRepairRejectsDirectionMismatch(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	before := seedReconcileSnapshot(t, repo, now, 10000, 12500, 2500)
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	_, err := svc.ApplyReconcileDetailRepair(context.Background(), ReconcileDetailRepairInput{
		SupplierID:             7,
		SnapshotID:             before.ID,
		ActionRecommendationID: 105,
		DetailType:             "usage_cost",
	})

	require.Error(t, err)
	require.Equal(t, "COST_RECONCILE_DETAIL_DIRECTION_MISMATCH", infraerrors.Reason(err))
}

func TestServiceApplyReconcileDetailRepairRejectsAmountMismatch(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 7, 9, 12, 15, 0, 0, time.UTC)
	before := seedReconcileSnapshot(t, repo, now, 10000, 12500, 2500)
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	_, err := svc.ApplyReconcileDetailRepair(context.Background(), ReconcileDetailRepairInput{
		SupplierID:             7,
		SnapshotID:             before.ID,
		ActionRecommendationID: 106,
		DetailType:             "funding_credit",
		AmountCents:            ptrInt64(1000),
	})

	require.Error(t, err)
	require.Equal(t, "COST_RECONCILE_DETAIL_AMOUNT_MISMATCH", infraerrors.Reason(err))
}

func TestServiceSyncDerivesRechargeMultiplierFromFundingCashAmount(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 6, 22, 8, 30, 0, 0, time.UTC)
	session := &stubCostSessionReader{}
	funding := &stubCostFundingReader{result: &ports.ReadFundingTransactionsResult{
		SupplierID:   7,
		ProviderType: "new_api",
		Items: []ports.ProviderFundingTransaction{
			{
				ExternalID:      "new-api-order-100",
				Status:          "success",
				Currency:        "USD",
				AmountCents:     10000,
				CashAmountCents: 1000,
			},
		},
	}}
	svc := NewServiceWithDependencies(repo, session, funding, nil, nil, nil, nil)
	svc.now = func() time.Time { return now }

	result, err := svc.Sync(context.Background(), SyncInput{
		SupplierID:                 7,
		IncludeFundingTransactions: true,
	})

	require.NoError(t, err)
	require.NotNil(t, result.Snapshot)
	require.Equal(t, int64(10000), result.Snapshot.CompletedFundingAmountCents)
	require.Equal(t, int64(1000), result.Snapshot.CompletedFundingCashCents)
	require.Equal(t, int64(1000), result.Snapshot.RechargeActualPaymentCents)
	items, err := svc.ListFundingTransactions(context.Background(), TransactionFilter{SupplierID: 7, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, float64(10), items[0].RechargeMultiplier)
	require.Equal(t, int64(1000), items[0].ActualPaymentCents)
}

func TestServiceSyncUsesProviderTypeFromSessionBundleForBalanceOnlyNewAPI(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 6, 22, 3, 30, 0, 0, time.UTC)
	session := &stubCostSessionReader{input: ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     "https://www.codexapis.com",
		APIBaseURL: "https://www.codexapis.com",
		Bundle: map[string]any{
			"provider_type": "new_api",
			"context": map[string]any{
				"system_type": "new_api",
			},
		},
	}}
	balance := &stubCostBalanceSyncer{result: &balancesapp.SyncFromSessionResult{
		SupplierID: 7,
		SystemType: "new_api",
		Origin:     "https://www.codexapis.com",
		APIBaseURL: "https://www.codexapis.com",
		SyncedAt:   now,
		Snapshot: &adminplusdomain.BalanceSnapshot{
			SupplierID:   7,
			BalanceCents: 1234500,
			Currency:     "USD",
			CapturedAt:   now,
		},
	}}
	svc := NewServiceWithDependencies(repo, session, nil, nil, nil, balance, nil)
	svc.now = func() time.Time { return now }

	result, err := svc.Sync(context.Background(), SyncInput{
		SupplierID:                     7,
		IncludeFundingTransactions:     false,
		IncludeEntitlementTransactions: false,
		IncludeUsageCostLines:          false,
		IncludeBalanceSnapshot:         true,
	})

	require.NoError(t, err)
	require.Equal(t, "new_api", result.ProviderType)
	require.Equal(t, "new_api", result.SystemType)
	require.Equal(t, "https://www.codexapis.com", result.APIBaseURL)
	require.Equal(t, true, result.Capabilities["balance_snapshot"])
	require.NotNil(t, result.Snapshot)
	require.Equal(t, "USD", result.Snapshot.Currency)
}

func TestServiceSyncRecordsCostReconcileAnomalyWithoutNotification(t *testing.T) {
	now := time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)
	repo := &anomalyCostRepository{
		MemoryRepository: NewMemoryRepository(),
		deltaCents:       ptrInt64(-250),
	}
	session := &stubCostSessionReader{}
	funding := &stubCostFundingReader{result: &ports.ReadFundingTransactionsResult{
		SupplierID:   7,
		ProviderType: "sub2api",
		Items: []ports.ProviderFundingTransaction{
			{
				ExternalID:  "order-1",
				Status:      "paid",
				Currency:    "USD",
				AmountCents: 10000,
			},
		},
	}}
	notifier := &stubCostNotifier{}
	svc := NewServiceWithDependenciesAndNotifier(repo, notifier, session, funding, nil, nil, nil, nil)
	svc.now = func() time.Time { return now }

	result, err := svc.Sync(context.Background(), SyncInput{
		SupplierID:                 7,
		IncludeFundingTransactions: true,
	})

	require.NoError(t, err)
	require.NotNil(t, result.Snapshot)
	require.NotNil(t, result.Snapshot.BalanceDeltaCents)
	require.Equal(t, int64(-250), *result.Snapshot.BalanceDeltaCents)
	require.Empty(t, notifier.snapshots)
}

func TestServiceSyncDoesNotNotifyCostReconcileAnomalyBelowThreshold(t *testing.T) {
	now := time.Date(2026, 6, 22, 9, 30, 0, 0, time.UTC)
	repo := &anomalyCostRepository{
		MemoryRepository: NewMemoryRepository(),
		deltaCents:       ptrInt64(50),
	}
	session := &stubCostSessionReader{}
	funding := &stubCostFundingReader{result: &ports.ReadFundingTransactionsResult{
		SupplierID:   7,
		ProviderType: "sub2api",
		Items: []ports.ProviderFundingTransaction{
			{
				ExternalID:  "order-1",
				Status:      "paid",
				Currency:    "USD",
				AmountCents: 10000,
			},
		},
	}}
	notifier := &stubCostNotifier{}
	svc := NewServiceWithDependenciesAndNotifier(repo, notifier, session, funding, nil, nil, nil, nil)
	svc.now = func() time.Time { return now }

	result, err := svc.Sync(context.Background(), SyncInput{
		SupplierID:                 7,
		IncludeFundingTransactions: true,
	})

	require.NoError(t, err)
	require.NotNil(t, result.Snapshot)
	require.Empty(t, notifier.snapshots)
}

func TestServiceSyncTurnsRefundedFundingIntoDebit(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	session := &stubCostSessionReader{}
	funding := &stubCostFundingReader{result: &ports.ReadFundingTransactionsResult{
		SupplierID:   7,
		ProviderType: "sub2api",
		Items: []ports.ProviderFundingTransaction{
			{
				ExternalID:        "refund-1",
				Status:            "refunded",
				Currency:          "USD",
				AmountCents:       5000,
				RefundAmountCents: 5000,
			},
		},
	}}
	svc := NewServiceWithDependencies(repo, session, funding, nil, nil, nil, nil)
	svc.now = func() time.Time { return now }

	result, err := svc.Sync(context.Background(), SyncInput{
		SupplierID:                     7,
		IncludeFundingTransactions:     true,
		IncludeBalanceSnapshot:         false,
		IncludeUsageCostLines:          false,
		IncludeEntitlementTransactions: false,
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.LedgerEntries)
	ledger, err := svc.ListLedgerEntries(context.Background(), LedgerFilter{SupplierID: 7, Limit: 10})
	require.NoError(t, err)
	require.Len(t, ledger, 1)
	require.Equal(t, "refund_debit", ledger[0].EntryType)
	require.Equal(t, int64(-5000), ledger[0].AmountCents)
	require.Equal(t, int64(-5000), result.Snapshot.ExpectedBalanceCents)
}

func TestServiceSyncReconcilesEntitlementTypeCorrection(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	usedAt := now.Add(-time.Hour)
	session := &stubCostSessionReader{}
	entitlements := &stubCostEntitlementReader{result: &ports.ReadEntitlementTransactionsResult{
		SupplierID:   7,
		ProviderType: "sub2api",
		Items: []ports.ProviderEntitlementTransaction{
			{
				ExternalID:   "redeem-1",
				SourceFamily: "manual_redeem",
				Type:         "balance",
				Status:       "used",
				Currency:     "usd",
				ValueCents:   7000,
				RawValue:     70,
				UsedAt:       &usedAt,
			},
		},
	}}
	svc := NewServiceWithDependencies(repo, session, nil, entitlements, nil, nil, nil)
	svc.now = func() time.Time { return now }

	first, err := svc.Sync(context.Background(), SyncInput{
		SupplierID:                     7,
		IncludeEntitlementTransactions: true,
	})

	require.NoError(t, err)
	require.Equal(t, 1, first.LedgerEntries)
	require.NotNil(t, first.Snapshot)
	require.Equal(t, int64(7000), first.Snapshot.EntitlementAmountCents)
	require.Equal(t, int64(7000), first.Snapshot.ExpectedBalanceCents)

	entitlements.result.Items[0].Type = "concurrency"
	entitlements.result.Items[0].ValueCents = 7000
	entitlements.result.Items[0].RawValue = 70

	second, err := svc.Sync(context.Background(), SyncInput{
		SupplierID:                     7,
		IncludeEntitlementTransactions: true,
	})

	require.NoError(t, err)
	require.Equal(t, 0, second.LedgerEntries)
	require.NotNil(t, second.Snapshot)
	require.Equal(t, int64(0), second.Snapshot.EntitlementAmountCents)
	require.Equal(t, int64(0), second.Snapshot.ExpectedBalanceCents)

	ledger, err := svc.ListLedgerEntries(context.Background(), LedgerFilter{SupplierID: 7, Limit: 10})
	require.NoError(t, err)
	require.Empty(t, ledger)

	items, err := svc.ListEntitlementTransactions(context.Background(), TransactionFilter{SupplierID: 7, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "concurrency", items[0].Type)
	require.Equal(t, int64(0), items[0].ValueCents)
	require.Equal(t, float64(70), items[0].RawValue)
}

func TestServiceGetLedgerOverviewAggregatesLatestSnapshotsByCurrency(t *testing.T) {
	repo := NewMemoryRepository()
	older := time.Date(2026, 6, 21, 8, 0, 0, 0, time.UTC)
	latest := older.Add(time.Hour)
	actualA := int64(4000)
	deltaA := int64(-500)
	actualB := int64(2500)
	deltaB := int64(0)
	repo.snapshots[1] = &adminplusdomain.SupplierCostSnapshot{
		ID:                          1,
		SupplierID:                  1,
		Currency:                    "USD",
		CompletedFundingAmountCents: 10000,
		EntitlementAmountCents:      1000,
		UsageCostCents:              100,
		ExpectedBalanceCents:        10900,
		CapturedAt:                  older,
	}
	repo.snapshots[2] = &adminplusdomain.SupplierCostSnapshot{
		ID:                          2,
		SupplierID:                  1,
		Currency:                    "USD",
		CompletedFundingAmountCents: 12000,
		CompletedFundingCashCents:   11500,
		EntitlementAmountCents:      3000,
		UsageCostCents:              11000,
		ExpectedBalanceCents:        4000,
		ActualBalanceCents:          &actualA,
		BalanceDeltaCents:           &deltaA,
		CapturedAt:                  latest,
	}
	repo.snapshots[3] = &adminplusdomain.SupplierCostSnapshot{
		ID:                          3,
		SupplierID:                  2,
		Currency:                    "USD",
		CompletedFundingAmountCents: 5000,
		EntitlementAmountCents:      0,
		UsageCostCents:              2500,
		ExpectedBalanceCents:        2500,
		ActualBalanceCents:          &actualB,
		BalanceDeltaCents:           &deltaB,
		CapturedAt:                  older,
	}
	repo.snapshots[4] = &adminplusdomain.SupplierCostSnapshot{
		ID:                          4,
		SupplierID:                  3,
		Currency:                    "CNY",
		CompletedFundingAmountCents: 5000,
		EntitlementAmountCents:      0,
		UsageCostCents:              0,
		ExpectedBalanceCents:        5000,
		CapturedAt:                  latest,
	}
	svc := NewService(repo)

	overview, err := svc.GetLedgerOverview(context.Background())

	require.NoError(t, err)
	require.NotNil(t, overview)
	require.Len(t, overview.Items, 1)
	require.Equal(t, "USD", overview.Items[0].Currency)
	require.Equal(t, 2, overview.Items[0].SupplierCount)
	require.Equal(t, 2, overview.Items[0].SnapshotCount)
	require.Equal(t, 2, overview.Items[0].ActualBalanceAvailableCount)
	require.Equal(t, int64(17000), overview.Items[0].CompletedFundingAmountCents)
	require.Equal(t, int64(3000), overview.Items[0].EntitlementAmountCents)
	require.Equal(t, int64(20000), overview.Items[0].RechargeTotalCents)
	require.Equal(t, int64(13500), overview.Items[0].UsageCostCents)
	require.NotNil(t, overview.Items[0].ActualBalanceCents)
	require.Equal(t, int64(6500), *overview.Items[0].ActualBalanceCents)
	require.NotNil(t, overview.Items[0].BalanceDeltaCents)
	require.Equal(t, int64(0), *overview.Items[0].BalanceDeltaCents)
}

type stubCostSessionReader struct {
	input      ports.SessionProbeInput
	supplierID int64
}

func (r *stubCostSessionReader) DecryptedProbeInput(_ context.Context, supplierID int64) (ports.SessionProbeInput, error) {
	r.supplierID = supplierID
	if r.input.SupplierID == 0 {
		r.input.SupplierID = supplierID
	}
	return r.input, nil
}

type stubCostFundingReader struct {
	result  *ports.ReadFundingTransactionsResult
	request ports.ReadFundingTransactionsInput
	err     error
}

func (r *stubCostFundingReader) ReadFundingTransactions(_ context.Context, _ ports.SessionProbeInput, request ports.ReadFundingTransactionsInput) (*ports.ReadFundingTransactionsResult, error) {
	r.request = request
	if r.err != nil {
		return nil, r.err
	}
	return r.result, nil
}

type stubCostEntitlementReader struct {
	result  *ports.ReadEntitlementTransactionsResult
	request ports.ReadEntitlementTransactionsInput
	err     error
}

func (r *stubCostEntitlementReader) ReadEntitlementTransactions(_ context.Context, _ ports.SessionProbeInput, request ports.ReadEntitlementTransactionsInput) (*ports.ReadEntitlementTransactionsResult, error) {
	r.request = request
	if r.err != nil {
		return nil, r.err
	}
	return r.result, nil
}

type stubCostBalanceSyncer struct {
	result  *balancesapp.SyncFromSessionResult
	request balancesapp.SyncFromSessionInput
}

func (s *stubCostBalanceSyncer) SyncFromSession(_ context.Context, in balancesapp.SyncFromSessionInput) (*balancesapp.SyncFromSessionResult, error) {
	s.request = in
	return s.result, nil
}

type stubCostUsageImporter struct {
	inputs []usagecostsapp.ImportUsageCostLineInput
}

func (s *stubCostUsageImporter) ImportUsageCostLines(_ context.Context, lines []usagecostsapp.ImportUsageCostLineInput) ([]*adminplusdomain.SupplierUsageCostLine, error) {
	s.inputs = append(s.inputs, lines...)
	items := make([]*adminplusdomain.SupplierUsageCostLine, 0, len(lines))
	for i, line := range lines {
		items = append(items, &adminplusdomain.SupplierUsageCostLine{
			ID:                  int64(i + 1),
			SupplierID:          line.SupplierID,
			Source:              line.Source,
			ExternalUsageCostID: line.ExternalUsageCostID,
			Model:               line.Model,
			Endpoint:            line.Endpoint,
			Currency:            line.Currency,
			CostCents:           line.CostCents,
			StartedAt:           line.StartedAt,
			RawPayload:          line.RawPayload,
		})
	}
	return items, nil
}

type stubCostSupplierLookup struct {
	supplier *adminplusdomain.Supplier
}

func (s *stubCostSupplierLookup) Get(_ context.Context, id int64) (*adminplusdomain.Supplier, error) {
	if s.supplier == nil {
		return &adminplusdomain.Supplier{ID: id, RechargeMultiplier: 1}, nil
	}
	out := *s.supplier
	if out.ID == 0 {
		out.ID = id
	}
	return &out, nil
}

type stubCostNotifier struct {
	snapshots []*adminplusdomain.SupplierCostSnapshot
}

func (n *stubCostNotifier) NotifyCostReconcileAnomaly(_ context.Context, snapshot *adminplusdomain.SupplierCostSnapshot) error {
	n.snapshots = append(n.snapshots, cloneSnapshot(snapshot))
	return nil
}

type anomalyCostRepository struct {
	*MemoryRepository
	deltaCents *int64
}

func (r *anomalyCostRepository) RefreshSnapshot(ctx context.Context, supplierID int64, currency string, capturedAt time.Time) (*adminplusdomain.SupplierCostSnapshot, error) {
	snapshot, err := r.MemoryRepository.RefreshSnapshot(ctx, supplierID, currency, capturedAt)
	if err != nil || snapshot == nil || r.deltaCents == nil {
		return snapshot, err
	}
	delta := *r.deltaCents
	actual := snapshot.ExpectedBalanceCents + delta
	snapshot.ActualBalanceCents = &actual
	snapshot.BalanceDeltaCents = &delta
	return cloneSnapshot(snapshot), nil
}

func ptrInt64(value int64) *int64 {
	return &value
}

func seedReconcileSnapshot(t *testing.T, repo *MemoryRepository, now time.Time, expected int64, actual int64, delta int64) *adminplusdomain.SupplierCostSnapshot {
	t.Helper()
	_, _, err := repo.UpsertLedgerEntry(context.Background(), &adminplusdomain.SupplierCostLedgerEntry{
		SupplierID:         7,
		ProviderType:       "sub2api",
		EntryType:          "funding_credit",
		SourceType:         "funding_transaction",
		SourceID:           1,
		SourceExternalID:   "seed-funding",
		Currency:           "USD",
		AmountCents:        expected,
		CashAmountCents:    expected,
		ActualPaymentCents: expected,
		OccurredAt:         now.Add(-time.Hour),
	})
	require.NoError(t, err)
	before, err := repo.RefreshSnapshot(context.Background(), 7, "USD", now.Add(-30*time.Minute))
	require.NoError(t, err)
	repo.snapshots[before.ID].ActualBalanceCents = &actual
	repo.snapshots[before.ID].BalanceDeltaCents = &delta
	return before
}

func findEntitlementByExternalID(t *testing.T, items []*adminplusdomain.SupplierEntitlementTransaction, externalID string) *adminplusdomain.SupplierEntitlementTransaction {
	t.Helper()
	for _, item := range items {
		if item.ExternalID == externalID {
			return item
		}
	}
	require.Failf(t, "entitlement transaction not found", "external_id=%s", externalID)
	return nil
}

var _ Repository = (*MemoryRepository)(nil)
var _ Repository = (*anomalyCostRepository)(nil)
var _ SessionReader = (*stubCostSessionReader)(nil)
var _ ports.SessionFundingAdapter = (*stubCostFundingReader)(nil)
var _ ports.SessionEntitlementAdapter = (*stubCostEntitlementReader)(nil)
var _ UsageCostImporter = (*stubCostUsageImporter)(nil)
var _ SupplierLookup = (*stubCostSupplierLookup)(nil)
var _ Notifier = (*stubCostNotifier)(nil)
