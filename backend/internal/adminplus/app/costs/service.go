package costs

import (
	"context"
	"strings"
	"time"

	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	usagecostsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/usagecosts"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
)

type SyncInput struct {
	SupplierID                     int64
	StartedAt                      *time.Time
	EndedAt                        *time.Time
	IncludeFundingTransactions     bool
	IncludeEntitlementTransactions bool
	IncludeUsageCostLines          bool
	IncludeBalanceSnapshot         bool
	LowBalanceThresholdCents       int64
}

type SyncResult struct {
	SupplierID              int64                                 `json:"supplier_id"`
	ProviderType            string                                `json:"provider_type"`
	SystemType              string                                `json:"system_type"`
	Origin                  string                                `json:"origin"`
	APIBaseURL              string                                `json:"api_base_url"`
	SyncedAt                time.Time                             `json:"synced_at"`
	FundingTransactions     int                                   `json:"funding_transactions"`
	EntitlementTransactions int                                   `json:"entitlement_transactions"`
	UsageCostLines          int                                   `json:"usage_cost_lines"`
	LedgerEntries           int                                   `json:"ledger_entries"`
	Snapshot                *adminplusdomain.SupplierCostSnapshot `json:"snapshot,omitempty"`
	Capabilities            map[string]bool                       `json:"capabilities"`
	Diagnostics             map[string]string                     `json:"diagnostics,omitempty"`
}

type SummaryFilter struct {
	SupplierID int64
	Limit      int
}

type TransactionFilter struct {
	SupplierID int64
	Limit      int
}

type LedgerFilter struct {
	SupplierID int64
	Limit      int
}

type Repository interface {
	UpsertFundingTransaction(ctx context.Context, item *adminplusdomain.SupplierFundingTransaction) (*adminplusdomain.SupplierFundingTransaction, bool, error)
	UpsertEntitlementTransaction(ctx context.Context, item *adminplusdomain.SupplierEntitlementTransaction) (*adminplusdomain.SupplierEntitlementTransaction, bool, error)
	UpsertLedgerEntry(ctx context.Context, entry *adminplusdomain.SupplierCostLedgerEntry) (*adminplusdomain.SupplierCostLedgerEntry, bool, error)
	RefreshSnapshot(ctx context.Context, supplierID int64, currency string, capturedAt time.Time) (*adminplusdomain.SupplierCostSnapshot, error)
	ListSnapshots(ctx context.Context, filter SummaryFilter) ([]*adminplusdomain.SupplierCostSnapshot, error)
	ListFundingTransactions(ctx context.Context, filter TransactionFilter) ([]*adminplusdomain.SupplierFundingTransaction, error)
	ListEntitlementTransactions(ctx context.Context, filter TransactionFilter) ([]*adminplusdomain.SupplierEntitlementTransaction, error)
	ListLedgerEntries(ctx context.Context, filter LedgerFilter) ([]*adminplusdomain.SupplierCostLedgerEntry, error)
}

type SessionReader interface {
	DecryptedProbeInput(ctx context.Context, supplierID int64) (ports.SessionProbeInput, error)
}

type UsageCostSyncer interface {
	SyncFromSession(ctx context.Context, in usagecostsapp.SyncFromSessionInput) (*usagecostsapp.SyncFromSessionResult, error)
}

type BalanceSyncer interface {
	SyncFromSession(ctx context.Context, in balancesapp.SyncFromSessionInput) (*balancesapp.SyncFromSessionResult, error)
}

type Service struct {
	repo              Repository
	session           SessionReader
	fundingReader     ports.SessionFundingAdapter
	entitlementReader ports.SessionEntitlementAdapter
	usageCostSyncer   UsageCostSyncer
	balanceSyncer     BalanceSyncer
	now               func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

func NewServiceWithDependencies(repo Repository, session SessionReader, fundingReader ports.SessionFundingAdapter, entitlementReader ports.SessionEntitlementAdapter, usageCostSyncer UsageCostSyncer, balanceSyncer BalanceSyncer) *Service {
	service := NewService(repo)
	service.session = session
	service.fundingReader = fundingReader
	service.entitlementReader = entitlementReader
	service.usageCostSyncer = usageCostSyncer
	service.balanceSyncer = balanceSyncer
	return service
}

func (s *Service) Sync(ctx context.Context, in SyncInput) (*SyncResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("cost service is not configured")
	}
	if s.session == nil {
		return nil, internalError("supplier browser session service is not configured")
	}
	if in.SupplierID <= 0 {
		return nil, badRequest("COST_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if in.LowBalanceThresholdCents < 0 {
		return nil, badRequest("COST_BALANCE_THRESHOLD_INVALID", "low balance threshold must be non-negative")
	}
	startedAt, endedAt, err := normalizeWindow(in.StartedAt, in.EndedAt, s.now)
	if err != nil {
		return nil, err
	}
	if !in.IncludeFundingTransactions && !in.IncludeEntitlementTransactions && !in.IncludeUsageCostLines && !in.IncludeBalanceSnapshot {
		in.IncludeFundingTransactions = true
		in.IncludeEntitlementTransactions = true
		in.IncludeUsageCostLines = true
		in.IncludeBalanceSnapshot = true
	}
	probeInput, err := s.session.DecryptedProbeInput(ctx, in.SupplierID)
	if err != nil {
		return nil, err
	}
	result := &SyncResult{
		SupplierID:   in.SupplierID,
		ProviderType: "sub2api",
		SyncedAt:     s.now().UTC(),
		Capabilities: map[string]bool{},
		Diagnostics:  map[string]string{},
	}
	currencies := map[string]struct{}{}
	ledgerEntries := 0

	if in.IncludeFundingTransactions {
		if s.fundingReader == nil {
			result.Diagnostics["funding_transactions"] = "adapter_missing"
		} else {
			read, err := s.fundingReader.ReadFundingTransactions(ctx, probeInput, ports.ReadFundingTransactionsInput{
				SupplierID: in.SupplierID,
				StartedAt:  startedAt,
				EndedAt:    endedAt,
			})
			if err != nil {
				return nil, err
			}
			result.ProviderType = firstNonEmpty(read.ProviderType, result.ProviderType)
			result.SystemType = read.SystemType
			result.Origin = read.Origin
			result.APIBaseURL = read.APIBaseURL
			result.Capabilities["funding_transactions"] = true
			for _, item := range read.Items {
				stored, _, err := s.repo.UpsertFundingTransaction(ctx, fundingFromProvider(in.SupplierID, read.ProviderType, item, result.SyncedAt))
				if err != nil {
					return nil, err
				}
				result.FundingTransactions++
				if stored != nil {
					if created, ok, err := s.repo.UpsertLedgerEntry(ctx, ledgerFromFunding(stored)); err != nil {
						return nil, err
					} else if ok && created != nil {
						ledgerEntries++
					}
					currencies[normalizeCurrency(stored.Currency)] = struct{}{}
				}
			}
		}
	}

	if in.IncludeEntitlementTransactions {
		if s.entitlementReader == nil {
			result.Diagnostics["entitlement_transactions"] = "adapter_missing"
		} else {
			read, err := s.entitlementReader.ReadEntitlementTransactions(ctx, probeInput, ports.ReadEntitlementTransactionsInput{
				SupplierID: in.SupplierID,
				StartedAt:  startedAt,
				EndedAt:    endedAt,
			})
			if err != nil {
				return nil, err
			}
			result.ProviderType = firstNonEmpty(read.ProviderType, result.ProviderType)
			result.SystemType = firstNonEmpty(read.SystemType, result.SystemType)
			result.Origin = firstNonEmpty(read.Origin, result.Origin)
			result.APIBaseURL = firstNonEmpty(read.APIBaseURL, result.APIBaseURL)
			result.Capabilities["entitlement_transactions"] = true
			for _, item := range read.Items {
				stored, _, err := s.repo.UpsertEntitlementTransaction(ctx, entitlementFromProvider(in.SupplierID, read.ProviderType, item, result.SyncedAt))
				if err != nil {
					return nil, err
				}
				result.EntitlementTransactions++
				if stored == nil || stored.SourceFamily == "payment_auto_redeem" {
					continue
				}
				if created, ok, err := s.repo.UpsertLedgerEntry(ctx, ledgerFromEntitlement(stored)); err != nil {
					return nil, err
				} else if ok && created != nil {
					ledgerEntries++
				}
				currencies[normalizeCurrency(stored.Currency)] = struct{}{}
			}
		}
	}

	if in.IncludeUsageCostLines && s.usageCostSyncer != nil {
		billing, err := s.usageCostSyncer.SyncFromSession(ctx, usagecostsapp.SyncFromSessionInput{
			SupplierID: in.SupplierID,
			StartedAt:  *startedAt,
			EndedAt:    *endedAt,
		})
		if err != nil {
			return nil, err
		}
		result.UsageCostLines = billing.Total
		result.Capabilities["usage_cost_lines"] = true
		result.SystemType = firstNonEmpty(billing.SystemType, result.SystemType)
		result.Origin = firstNonEmpty(billing.Origin, result.Origin)
		result.APIBaseURL = firstNonEmpty(billing.APIBaseURL, result.APIBaseURL)
		for _, line := range billing.Items {
			if line != nil {
				currencies[normalizeCurrency(line.Currency)] = struct{}{}
			}
		}
	}

	if in.IncludeBalanceSnapshot && s.balanceSyncer != nil {
		balance, err := s.balanceSyncer.SyncFromSession(ctx, balancesapp.SyncFromSessionInput{
			SupplierID:               in.SupplierID,
			LowBalanceThresholdCents: in.LowBalanceThresholdCents,
		})
		if err != nil {
			return nil, err
		}
		result.Capabilities["balance_snapshot"] = balance.Snapshot != nil
		result.SystemType = firstNonEmpty(balance.SystemType, result.SystemType)
		result.Origin = firstNonEmpty(balance.Origin, result.Origin)
		result.APIBaseURL = firstNonEmpty(balance.APIBaseURL, result.APIBaseURL)
		if balance.Snapshot != nil {
			currencies[normalizeCurrency(balance.Snapshot.Currency)] = struct{}{}
		}
	}

	for currency := range currencies {
		snapshot, err := s.repo.RefreshSnapshot(ctx, in.SupplierID, currency, result.SyncedAt)
		if err != nil {
			return nil, err
		}
		if result.Snapshot == nil {
			result.Snapshot = snapshot
		}
	}
	result.LedgerEntries = ledgerEntries
	if len(result.Diagnostics) == 0 {
		result.Diagnostics = nil
	}
	return result, nil
}

func (s *Service) ListSnapshots(ctx context.Context, filter SummaryFilter) ([]*adminplusdomain.SupplierCostSnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("cost service is not configured")
	}
	if filter.SupplierID < 0 {
		return nil, badRequest("COST_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListSnapshots(ctx, filter)
}

func (s *Service) ListFundingTransactions(ctx context.Context, filter TransactionFilter) ([]*adminplusdomain.SupplierFundingTransaction, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("cost service is not configured")
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListFundingTransactions(ctx, filter)
}

func (s *Service) ListEntitlementTransactions(ctx context.Context, filter TransactionFilter) ([]*adminplusdomain.SupplierEntitlementTransaction, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("cost service is not configured")
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListEntitlementTransactions(ctx, filter)
}

func (s *Service) ListLedgerEntries(ctx context.Context, filter LedgerFilter) ([]*adminplusdomain.SupplierCostLedgerEntry, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("cost service is not configured")
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListLedgerEntries(ctx, filter)
}

func normalizeWindow(startedAt *time.Time, endedAt *time.Time, now func() time.Time) (*time.Time, *time.Time, error) {
	end := time.Now().UTC()
	if now != nil {
		end = now().UTC()
	}
	if endedAt != nil {
		end = endedAt.UTC()
	}
	start := end.Add(-7 * 24 * time.Hour)
	if startedAt != nil {
		start = startedAt.UTC()
	}
	if !start.Before(end) {
		return nil, nil, badRequest("COST_TIME_RANGE_INVALID", "ended_at must be after started_at")
	}
	return &start, &end, nil
}

func fundingFromProvider(supplierID int64, providerType string, item ports.ProviderFundingTransaction, now time.Time) *adminplusdomain.SupplierFundingTransaction {
	return &adminplusdomain.SupplierFundingTransaction{
		SupplierID:        supplierID,
		ProviderType:      normalizeProviderType(providerType),
		ExternalID:        strings.TrimSpace(item.ExternalID),
		OutTradeNo:        trimLimit(item.OutTradeNo, 160),
		PaymentTradeNo:    trimLimit(item.PaymentTradeNo, 160),
		PaymentType:       trimLimit(item.PaymentType, 80),
		OrderType:         trimLimit(item.OrderType, 80),
		Status:            normalizeStatus(item.Status),
		Currency:          normalizeCurrency(item.Currency),
		AmountCents:       nonNegative(item.AmountCents),
		CashAmountCents:   nonNegative(item.CashAmountCents),
		RefundAmountCents: nonNegative(item.RefundAmountCents),
		FeeRate:           item.FeeRate,
		CreatedAtExternal: cloneTime(item.CreatedAtExternal),
		PaidAt:            cloneTime(item.PaidAt),
		CompletedAt:       cloneTime(item.CompletedAt),
		RawPayload:        item.RawPayload,
		LastSeenAt:        now.UTC(),
	}
}

func entitlementFromProvider(supplierID int64, providerType string, item ports.ProviderEntitlementTransaction, now time.Time) *adminplusdomain.SupplierEntitlementTransaction {
	return &adminplusdomain.SupplierEntitlementTransaction{
		SupplierID:        supplierID,
		ProviderType:      normalizeProviderType(providerType),
		ExternalID:        strings.TrimSpace(item.ExternalID),
		CodeFingerprint:   trimLimit(item.CodeFingerprint, 128),
		CodeLast4:         trimLimit(item.CodeLast4, 16),
		SourceFamily:      normalizeSourceFamily(item.SourceFamily),
		Type:              normalizeLower(item.Type, "balance"),
		Status:            normalizeLower(item.Status, "used"),
		Currency:          normalizeCurrency(item.Currency),
		ValueCents:        nonNegative(item.ValueCents),
		RawValue:          item.RawValue,
		GroupID:           item.GroupID,
		ValidityDays:      item.ValidityDays,
		UsedAt:            cloneTime(item.UsedAt),
		CreatedAtExternal: cloneTime(item.CreatedAtExternal),
		RawPayload:        item.RawPayload,
		LastSeenAt:        now.UTC(),
	}
}

func ledgerFromFunding(item *adminplusdomain.SupplierFundingTransaction) *adminplusdomain.SupplierCostLedgerEntry {
	if item == nil || !fundingStatusCounts(item.Status) {
		return nil
	}
	occurredAt := firstTime(item.CompletedAt, item.PaidAt, item.CreatedAtExternal, item.LastSeenAt)
	entryType := "funding_credit"
	amount := item.AmountCents
	if item.RefundAmountCents > 0 || strings.Contains(strings.ToUpper(item.Status), "REFUND") {
		entryType = "refund_debit"
		amount = -item.RefundAmountCents
		if amount == 0 {
			amount = -item.AmountCents
		}
	}
	return &adminplusdomain.SupplierCostLedgerEntry{
		SupplierID:       item.SupplierID,
		ProviderType:     item.ProviderType,
		EntryType:        entryType,
		SourceType:       "funding_transaction",
		SourceID:         item.ID,
		SourceExternalID: item.ExternalID,
		Currency:         item.Currency,
		AmountCents:      amount,
		CashAmountCents:  item.CashAmountCents,
		OccurredAt:       occurredAt,
		RawPayload: map[string]any{
			"status":       item.Status,
			"order_type":   item.OrderType,
			"out_trade_no": item.OutTradeNo,
		},
	}
}

func ledgerFromEntitlement(item *adminplusdomain.SupplierEntitlementTransaction) *adminplusdomain.SupplierCostLedgerEntry {
	if item == nil || item.Type != "balance" || item.Status != "used" {
		return nil
	}
	occurredAt := firstTime(item.UsedAt, item.CreatedAtExternal, item.LastSeenAt)
	return &adminplusdomain.SupplierCostLedgerEntry{
		SupplierID:       item.SupplierID,
		ProviderType:     item.ProviderType,
		EntryType:        "entitlement_credit",
		SourceType:       "entitlement_transaction",
		SourceID:         item.ID,
		SourceExternalID: item.ExternalID,
		Currency:         item.Currency,
		AmountCents:      item.ValueCents,
		OccurredAt:       occurredAt,
		RawPayload: map[string]any{
			"type":          item.Type,
			"source_family": item.SourceFamily,
			"status":        item.Status,
		},
	}
}

func fundingStatusCounts(status string) bool {
	status = strings.ToUpper(strings.TrimSpace(status))
	switch status {
	case "PAID", "RECHARGING", "COMPLETED", "REFUNDED", "PARTIALLY_REFUNDED":
		return true
	default:
		return false
	}
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func normalizeCurrency(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "USD"
	}
	return value
}

func normalizeProviderType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "sub2api"
	}
	return value
}

func normalizeStatus(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "UNKNOWN"
	}
	return value
}

func normalizeLower(value string, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	return value
}

func normalizeSourceFamily(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "manual_redeem"
	}
	return value
}

func nonNegative(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func trimLimit(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit > 0 && len(value) > limit {
		return value[:limit]
	}
	return value
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	t := value.UTC()
	return &t
}

func firstTime(values ...any) time.Time {
	for _, value := range values {
		switch v := value.(type) {
		case *time.Time:
			if v != nil && !v.IsZero() {
				return v.UTC()
			}
		case time.Time:
			if !v.IsZero() {
				return v.UTC()
			}
		}
	}
	return time.Now().UTC()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
