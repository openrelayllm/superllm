package costs

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

type MemoryRepository struct {
	mu           sync.Mutex
	nextFunding  int64
	nextEntitle  int64
	nextLedger   int64
	nextSnapshot int64
	funding      map[int64]*adminplusdomain.SupplierFundingTransaction
	entitlements map[int64]*adminplusdomain.SupplierEntitlementTransaction
	ledger       map[int64]*adminplusdomain.SupplierCostLedgerEntry
	snapshots    map[int64]*adminplusdomain.SupplierCostSnapshot
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		nextFunding:  1,
		nextEntitle:  1,
		nextLedger:   1,
		nextSnapshot: 1,
		funding:      make(map[int64]*adminplusdomain.SupplierFundingTransaction),
		entitlements: make(map[int64]*adminplusdomain.SupplierEntitlementTransaction),
		ledger:       make(map[int64]*adminplusdomain.SupplierCostLedgerEntry),
		snapshots:    make(map[int64]*adminplusdomain.SupplierCostSnapshot),
	}
}

func (r *MemoryRepository) UpsertFundingTransaction(_ context.Context, item *adminplusdomain.SupplierFundingTransaction) (*adminplusdomain.SupplierFundingTransaction, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, existing := range r.funding {
		if existing.SupplierID == item.SupplierID && existing.ProviderType == item.ProviderType && existing.ExternalID == item.ExternalID {
			cp := cloneFunding(item)
			cp.ID = id
			cp.CreatedAt = existing.CreatedAt
			cp.UpdatedAt = item.LastSeenAt
			r.funding[id] = cp
			return cloneFunding(cp), false, nil
		}
	}
	cp := cloneFunding(item)
	cp.ID = r.nextFunding
	r.nextFunding++
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = item.LastSeenAt
	}
	if cp.UpdatedAt.IsZero() {
		cp.UpdatedAt = item.LastSeenAt
	}
	r.funding[cp.ID] = cp
	return cloneFunding(cp), true, nil
}

func (r *MemoryRepository) UpsertEntitlementTransaction(_ context.Context, item *adminplusdomain.SupplierEntitlementTransaction) (*adminplusdomain.SupplierEntitlementTransaction, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, existing := range r.entitlements {
		if existing.SupplierID == item.SupplierID && existing.ProviderType == item.ProviderType && existing.ExternalID == item.ExternalID {
			cp := cloneEntitlement(item)
			cp.ID = id
			cp.CreatedAt = existing.CreatedAt
			cp.UpdatedAt = item.LastSeenAt
			r.entitlements[id] = cp
			return cloneEntitlement(cp), false, nil
		}
	}
	cp := cloneEntitlement(item)
	cp.ID = r.nextEntitle
	r.nextEntitle++
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = item.LastSeenAt
	}
	if cp.UpdatedAt.IsZero() {
		cp.UpdatedAt = item.LastSeenAt
	}
	r.entitlements[cp.ID] = cp
	return cloneEntitlement(cp), true, nil
}

func (r *MemoryRepository) UpsertLedgerEntry(_ context.Context, entry *adminplusdomain.SupplierCostLedgerEntry) (*adminplusdomain.SupplierCostLedgerEntry, bool, error) {
	if entry == nil {
		return nil, false, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.ledger {
		if existing.SupplierID == entry.SupplierID &&
			existing.ProviderType == entry.ProviderType &&
			existing.EntryType == entry.EntryType &&
			existing.SourceType == entry.SourceType &&
			existing.SourceID == entry.SourceID {
			cp := cloneLedger(entry)
			cp.ID = existing.ID
			cp.CreatedAt = existing.CreatedAt
			r.ledger[existing.ID] = cp
			return cloneLedger(cp), false, nil
		}
	}
	cp := cloneLedger(entry)
	cp.ID = r.nextLedger
	r.nextLedger++
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now().UTC()
	}
	r.ledger[cp.ID] = cp
	return cloneLedger(cp), true, nil
}

func (r *MemoryRepository) DeleteLedgerEntryForSource(_ context.Context, supplierID int64, providerType string, entryType string, sourceType string, sourceID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	providerType = normalizeProviderType(providerType)
	for id, existing := range r.ledger {
		if existing.SupplierID == supplierID &&
			existing.ProviderType == providerType &&
			existing.EntryType == entryType &&
			existing.SourceType == sourceType &&
			existing.SourceID == sourceID {
			delete(r.ledger, id)
		}
	}
	return nil
}

func (r *MemoryRepository) RefreshSnapshot(_ context.Context, supplierID int64, currency string, capturedAt time.Time) (*adminplusdomain.SupplierCostSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	currency = normalizeCurrency(currency)
	var fundingAmount, fundingCash, fundingActualPayment, entitlementAmount, refundAmount, adjustmentAmount int64
	var autoRedeemEntitlementAmount int64
	for _, entry := range r.ledger {
		if entry.SupplierID != supplierID || normalizeCurrency(entry.Currency) != currency {
			continue
		}
		switch entry.EntryType {
		case "funding_credit":
			fundingAmount += entry.AmountCents
			fundingCash += entry.CashAmountCents
			fundingActualPayment += entry.ActualPaymentCents
		case "entitlement_credit":
			entitlementAmount += entry.AmountCents
		case "refund_debit":
			refundAmount += -entry.AmountCents
		case "manual_adjustment", "reversal":
			adjustmentAmount += entry.AmountCents
		}
	}
	for _, item := range r.entitlements {
		if item.SupplierID != supplierID || normalizeCurrency(item.Currency) != currency {
			continue
		}
		if strings.EqualFold(item.SourceFamily, "payment_auto_redeem") &&
			strings.EqualFold(item.Type, "balance") &&
			strings.EqualFold(item.Status, "used") {
			autoRedeemEntitlementAmount += item.ValueCents
		}
	}
	if autoRedeemFallback := autoRedeemEntitlementAmount - fundingAmount; autoRedeemFallback > 0 {
		entitlementAmount += autoRedeemFallback
	}
	if fundingActualPayment == 0 && fundingAmount > 0 {
		if fundingCash > 0 {
			fundingActualPayment = fundingCash
		} else {
			fundingActualPayment = fundingAmount
		}
	}
	snapshot := &adminplusdomain.SupplierCostSnapshot{
		ID:                          r.nextSnapshot,
		SupplierID:                  supplierID,
		Currency:                    currency,
		CompletedFundingAmountCents: fundingAmount,
		CompletedFundingCashCents:   fundingCash,
		RechargeActualPaymentCents:  fundingActualPayment + entitlementAmount,
		EntitlementAmountCents:      entitlementAmount,
		RefundAmountCents:           refundAmount,
		AdjustmentAmountCents:       adjustmentAmount,
		ExpectedBalanceCents:        fundingAmount + entitlementAmount - refundAmount + adjustmentAmount,
		CapturedAt:                  capturedAt.UTC(),
		CreatedAt:                   capturedAt.UTC(),
	}
	r.nextSnapshot++
	r.snapshots[snapshot.ID] = snapshot
	return cloneSnapshot(snapshot), nil
}

func (r *MemoryRepository) GetSnapshot(_ context.Context, id int64) (*adminplusdomain.SupplierCostSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneSnapshot(r.snapshots[id]), nil
}

func (r *MemoryRepository) ListSnapshots(_ context.Context, filter SummaryFilter) ([]*adminplusdomain.SupplierCostSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]*adminplusdomain.SupplierCostSnapshot, 0)
	for _, snapshot := range r.snapshots {
		if filter.SupplierID > 0 && snapshot.SupplierID != filter.SupplierID {
			continue
		}
		if strings.ToUpper(strings.TrimSpace(snapshot.Currency)) != "USD" {
			continue
		}
		items = append(items, cloneSnapshot(snapshot))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CapturedAt.Equal(items[j].CapturedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CapturedAt.After(items[j].CapturedAt)
	})
	return limitSnapshots(items, filter.Limit), nil
}

func (r *MemoryRepository) GetLedgerOverview(_ context.Context) (*adminplusdomain.SupplierCostLedgerOverview, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	latest := make(map[string]*adminplusdomain.SupplierCostSnapshot)
	for _, snapshot := range r.snapshots {
		if snapshot == nil {
			continue
		}
		if strings.ToUpper(strings.TrimSpace(snapshot.Currency)) != "USD" {
			continue
		}
		key := snapshotKey(snapshot.SupplierID, snapshot.Currency)
		current := latest[key]
		if current == nil || snapshot.CapturedAt.After(current.CapturedAt) || (snapshot.CapturedAt.Equal(current.CapturedAt) && snapshot.ID > current.ID) {
			cp := cloneSnapshot(snapshot)
			normalizeCostSnapshotDerivedAmounts(cp)
			latest[key] = cp
		}
	}
	items := make([]*adminplusdomain.SupplierCostSnapshot, 0, len(latest))
	for _, snapshot := range latest {
		items = append(items, snapshot)
	}
	return buildLedgerOverview(items, time.Now()), nil
}

func (r *MemoryRepository) ListFundingTransactions(_ context.Context, filter TransactionFilter) ([]*adminplusdomain.SupplierFundingTransaction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]*adminplusdomain.SupplierFundingTransaction, 0)
	for _, item := range r.funding {
		if filter.SupplierID > 0 && item.SupplierID != filter.SupplierID {
			continue
		}
		items = append(items, cloneFunding(item))
	}
	sort.Slice(items, func(i, j int) bool { return eventTime(items[i]).After(eventTime(items[j])) })
	return limitFunding(items, filter.Limit), nil
}

func (r *MemoryRepository) ListEntitlementTransactions(_ context.Context, filter TransactionFilter) ([]*adminplusdomain.SupplierEntitlementTransaction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]*adminplusdomain.SupplierEntitlementTransaction, 0)
	for _, item := range r.entitlements {
		if filter.SupplierID > 0 && item.SupplierID != filter.SupplierID {
			continue
		}
		items = append(items, cloneEntitlement(item))
	}
	sort.Slice(items, func(i, j int) bool { return entitlementTime(items[i]).After(entitlementTime(items[j])) })
	return limitEntitlements(items, filter.Limit), nil
}

func (r *MemoryRepository) ListLedgerEntries(_ context.Context, filter LedgerFilter) ([]*adminplusdomain.SupplierCostLedgerEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]*adminplusdomain.SupplierCostLedgerEntry, 0)
	for _, item := range r.ledger {
		if filter.SupplierID > 0 && item.SupplierID != filter.SupplierID {
			continue
		}
		items = append(items, cloneLedger(item))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].OccurredAt.After(items[j].OccurredAt) })
	return limitLedger(items, filter.Limit), nil
}

func cloneFunding(in *adminplusdomain.SupplierFundingTransaction) *adminplusdomain.SupplierFundingTransaction {
	if in == nil {
		return nil
	}
	out := *in
	out.FeeRate = cloneFloat64(in.FeeRate)
	out.CreatedAtExternal = cloneTime(in.CreatedAtExternal)
	out.PaidAt = cloneTime(in.PaidAt)
	out.CompletedAt = cloneTime(in.CompletedAt)
	out.RawPayload = cloneMap(in.RawPayload)
	return &out
}

func cloneEntitlement(in *adminplusdomain.SupplierEntitlementTransaction) *adminplusdomain.SupplierEntitlementTransaction {
	if in == nil {
		return nil
	}
	out := *in
	out.UsedAt = cloneTime(in.UsedAt)
	out.CreatedAtExternal = cloneTime(in.CreatedAtExternal)
	out.RawPayload = cloneMap(in.RawPayload)
	return &out
}

func cloneLedger(in *adminplusdomain.SupplierCostLedgerEntry) *adminplusdomain.SupplierCostLedgerEntry {
	if in == nil {
		return nil
	}
	out := *in
	out.RawPayload = cloneMap(in.RawPayload)
	return &out
}

func cloneSnapshot(in *adminplusdomain.SupplierCostSnapshot) *adminplusdomain.SupplierCostSnapshot {
	if in == nil {
		return nil
	}
	out := *in
	out.ActualBalanceCents = cloneInt64(in.ActualBalanceCents)
	out.BalanceDeltaCents = cloneInt64(in.BalanceDeltaCents)
	return &out
}

func cloneFloat64(in *float64) *float64 {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneInt64(in *int64) *int64 {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		if strings.TrimSpace(key) == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func snapshotKey(supplierID int64, currency string) string {
	return normalizeCurrency(currency) + ":" + strconv.FormatInt(supplierID, 10)
}

func eventTime(item *adminplusdomain.SupplierFundingTransaction) time.Time {
	return firstTime(item.CompletedAt, item.PaidAt, item.CreatedAtExternal, item.LastSeenAt)
}

func entitlementTime(item *adminplusdomain.SupplierEntitlementTransaction) time.Time {
	return firstTime(item.UsedAt, item.CreatedAtExternal, item.LastSeenAt)
}

func limitSnapshots(items []*adminplusdomain.SupplierCostSnapshot, limit int) []*adminplusdomain.SupplierCostSnapshot {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func limitFunding(items []*adminplusdomain.SupplierFundingTransaction, limit int) []*adminplusdomain.SupplierFundingTransaction {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func limitEntitlements(items []*adminplusdomain.SupplierEntitlementTransaction, limit int) []*adminplusdomain.SupplierEntitlementTransaction {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func limitLedger(items []*adminplusdomain.SupplierCostLedgerEntry, limit int) []*adminplusdomain.SupplierCostLedgerEntry {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}
