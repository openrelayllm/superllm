package costs

import (
	"context"
	"testing"
	"time"

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
	require.Equal(t, 2, result.EntitlementTransactions)
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

var _ Repository = (*MemoryRepository)(nil)
var _ SessionReader = (*stubCostSessionReader)(nil)
var _ ports.SessionFundingAdapter = (*stubCostFundingReader)(nil)
var _ ports.SessionEntitlementAdapter = (*stubCostEntitlementReader)(nil)
