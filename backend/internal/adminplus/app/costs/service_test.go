package costs

import (
	"context"
	"testing"
	"time"

	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
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
	svc := NewServiceWithDependencies(repo, session, funding, entitlements, nil, nil)
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
	svc := NewServiceWithDependencies(repo, session, nil, entitlements, nil, nil)
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
			Currency:     "QTA",
			CapturedAt:   now,
		},
	}}
	svc := NewServiceWithDependencies(repo, session, nil, nil, nil, balance)
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
	require.Equal(t, "QTA", result.Snapshot.Currency)
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
	svc := NewServiceWithDependencies(repo, session, funding, nil, nil, nil)
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
	svc := NewServiceWithDependencies(repo, session, nil, entitlements, nil, nil)
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
}

func (r *stubCostFundingReader) ReadFundingTransactions(_ context.Context, _ ports.SessionProbeInput, request ports.ReadFundingTransactionsInput) (*ports.ReadFundingTransactionsResult, error) {
	r.request = request
	return r.result, nil
}

type stubCostEntitlementReader struct {
	result  *ports.ReadEntitlementTransactionsResult
	request ports.ReadEntitlementTransactionsInput
}

func (r *stubCostEntitlementReader) ReadEntitlementTransactions(_ context.Context, _ ports.SessionProbeInput, request ports.ReadEntitlementTransactionsInput) (*ports.ReadEntitlementTransactionsResult, error) {
	r.request = request
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
var _ SessionReader = (*stubCostSessionReader)(nil)
var _ ports.SessionFundingAdapter = (*stubCostFundingReader)(nil)
var _ ports.SessionEntitlementAdapter = (*stubCostEntitlementReader)(nil)
