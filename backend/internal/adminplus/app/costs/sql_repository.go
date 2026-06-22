package costs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) UpsertFundingTransaction(ctx context.Context, item *adminplusdomain.SupplierFundingTransaction) (*adminplusdomain.SupplierFundingTransaction, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, dbNotConfigured()
	}
	rawPayload, err := marshalRawPayload(item.RawPayload)
	if err != nil {
		return nil, false, err
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_supplier_funding_transactions (
			supplier_id, provider_type, external_id, out_trade_no, payment_trade_no,
			payment_type, order_type, status, currency, amount_cents, cash_amount_cents,
			refund_amount_cents, fee_rate, created_at_external, paid_at, completed_at,
			raw_payload, last_seen_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		ON CONFLICT (supplier_id, provider_type, external_id)
		DO UPDATE SET
			out_trade_no = EXCLUDED.out_trade_no,
			payment_trade_no = EXCLUDED.payment_trade_no,
			payment_type = EXCLUDED.payment_type,
			order_type = EXCLUDED.order_type,
			status = EXCLUDED.status,
			currency = EXCLUDED.currency,
			amount_cents = EXCLUDED.amount_cents,
			cash_amount_cents = EXCLUDED.cash_amount_cents,
			refund_amount_cents = EXCLUDED.refund_amount_cents,
			fee_rate = EXCLUDED.fee_rate,
			created_at_external = EXCLUDED.created_at_external,
			paid_at = EXCLUDED.paid_at,
			completed_at = EXCLUDED.completed_at,
			raw_payload = EXCLUDED.raw_payload,
			last_seen_at = EXCLUDED.last_seen_at,
			updated_at = NOW()
		RETURNING id, supplier_id, provider_type, external_id, out_trade_no, payment_trade_no,
			payment_type, order_type, status, currency, amount_cents, cash_amount_cents,
			refund_amount_cents, fee_rate, created_at_external, paid_at, completed_at,
			raw_payload, last_seen_at, created_at, updated_at,
			(xmax = 0) AS inserted
	`,
		item.SupplierID,
		item.ProviderType,
		item.ExternalID,
		item.OutTradeNo,
		item.PaymentTradeNo,
		item.PaymentType,
		item.OrderType,
		item.Status,
		item.Currency,
		item.AmountCents,
		item.CashAmountCents,
		item.RefundAmountCents,
		nullableFloat64(item.FeeRate),
		nullableTime(item.CreatedAtExternal),
		nullableTime(item.PaidAt),
		nullableTime(item.CompletedAt),
		rawPayload,
		item.LastSeenAt,
	)
	return scanFundingTransactionWithInserted(row)
}

func (r *SQLRepository) UpsertEntitlementTransaction(ctx context.Context, item *adminplusdomain.SupplierEntitlementTransaction) (*adminplusdomain.SupplierEntitlementTransaction, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, dbNotConfigured()
	}
	rawPayload, err := marshalRawPayload(item.RawPayload)
	if err != nil {
		return nil, false, err
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_supplier_entitlement_transactions (
			supplier_id, provider_type, external_id, code_fingerprint, code_last4,
			source_family, type, status, currency, value_cents, raw_value, group_id,
			validity_days, used_at, created_at_external, raw_payload, last_seen_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (supplier_id, provider_type, external_id)
		DO UPDATE SET
			code_fingerprint = EXCLUDED.code_fingerprint,
			code_last4 = EXCLUDED.code_last4,
			source_family = EXCLUDED.source_family,
			type = EXCLUDED.type,
			status = EXCLUDED.status,
			currency = EXCLUDED.currency,
			value_cents = EXCLUDED.value_cents,
			raw_value = EXCLUDED.raw_value,
			group_id = EXCLUDED.group_id,
			validity_days = EXCLUDED.validity_days,
			used_at = EXCLUDED.used_at,
			created_at_external = EXCLUDED.created_at_external,
			raw_payload = EXCLUDED.raw_payload,
			last_seen_at = EXCLUDED.last_seen_at,
			updated_at = NOW()
		RETURNING id, supplier_id, provider_type, external_id, code_fingerprint, code_last4,
			source_family, type, status, currency, value_cents, raw_value, group_id,
			validity_days, used_at, created_at_external, raw_payload, last_seen_at,
			created_at, updated_at,
			(xmax = 0) AS inserted
	`,
		item.SupplierID,
		item.ProviderType,
		item.ExternalID,
		item.CodeFingerprint,
		item.CodeLast4,
		item.SourceFamily,
		item.Type,
		item.Status,
		item.Currency,
		item.ValueCents,
		item.RawValue,
		item.GroupID,
		item.ValidityDays,
		nullableTime(item.UsedAt),
		nullableTime(item.CreatedAtExternal),
		rawPayload,
		item.LastSeenAt,
	)
	return scanEntitlementTransactionWithInserted(row)
}

func (r *SQLRepository) UpsertLedgerEntry(ctx context.Context, entry *adminplusdomain.SupplierCostLedgerEntry) (*adminplusdomain.SupplierCostLedgerEntry, bool, error) {
	if entry == nil {
		return nil, false, nil
	}
	if r == nil || r.db == nil {
		return nil, false, dbNotConfigured()
	}
	rawPayload, err := marshalRawPayload(entry.RawPayload)
	if err != nil {
		return nil, false, err
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_supplier_cost_ledger_entries (
			supplier_id, provider_type, entry_type, source_type, source_id,
			source_external_id, currency, amount_cents, cash_amount_cents,
			occurred_at, raw_payload
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (supplier_id, provider_type, entry_type, source_type, source_id)
		DO UPDATE SET
			source_external_id = EXCLUDED.source_external_id,
			currency = EXCLUDED.currency,
			amount_cents = EXCLUDED.amount_cents,
			cash_amount_cents = EXCLUDED.cash_amount_cents,
			occurred_at = EXCLUDED.occurred_at,
			raw_payload = EXCLUDED.raw_payload
		RETURNING id, supplier_id, provider_type, entry_type, source_type, source_id,
			source_external_id, currency, amount_cents, cash_amount_cents,
			occurred_at, raw_payload, created_at,
			(xmax = 0) AS inserted
	`,
		entry.SupplierID,
		entry.ProviderType,
		entry.EntryType,
		entry.SourceType,
		entry.SourceID,
		entry.SourceExternalID,
		entry.Currency,
		entry.AmountCents,
		entry.CashAmountCents,
		entry.OccurredAt,
		rawPayload,
	)
	return scanLedgerEntryWithInserted(row)
}

func (r *SQLRepository) DeleteLedgerEntryForSource(ctx context.Context, supplierID int64, providerType string, entryType string, sourceType string, sourceID int64) error {
	if r == nil || r.db == nil {
		return dbNotConfigured()
	}
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM admin_plus_supplier_cost_ledger_entries
		WHERE supplier_id = $1
			AND provider_type = $2
			AND entry_type = $3
			AND source_type = $4
			AND source_id = $5
	`, supplierID, normalizeProviderType(providerType), entryType, sourceType, sourceID)
	return err
}

func (r *SQLRepository) RefreshSnapshot(ctx context.Context, supplierID int64, currency string, capturedAt time.Time) (*adminplusdomain.SupplierCostSnapshot, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		WITH ledger AS (
			SELECT
				COALESCE(SUM(CASE WHEN entry_type = 'funding_credit' THEN amount_cents ELSE 0 END), 0) AS funding_amount,
				COALESCE(SUM(CASE WHEN entry_type = 'funding_credit' THEN cash_amount_cents ELSE 0 END), 0) AS funding_cash,
				COALESCE(SUM(CASE WHEN entry_type = 'entitlement_credit' THEN amount_cents ELSE 0 END), 0) AS entitlement_amount,
				COALESCE(SUM(CASE WHEN entry_type = 'refund_debit' THEN -amount_cents ELSE 0 END), 0) AS refund_amount,
				COALESCE(SUM(CASE WHEN entry_type IN ('manual_adjustment', 'reversal') THEN amount_cents ELSE 0 END), 0) AS adjustment_amount
			FROM admin_plus_supplier_cost_ledger_entries
			WHERE supplier_id = $1 AND currency = $2
		),
		auto_redeem_entitlement AS (
			SELECT COALESCE(SUM(value_cents), 0) AS amount
			FROM admin_plus_supplier_entitlement_transactions
			WHERE supplier_id = $1
				AND currency = $2
				AND LOWER(source_family) = 'payment_auto_redeem'
				AND LOWER(type) = 'balance'
				AND LOWER(status) = 'used'
		),
		usage AS (
			SELECT COALESCE(SUM(cost_cents), 0) AS usage_cost
			FROM admin_plus_supplier_usage_cost_lines
			WHERE supplier_id = $1 AND currency = $2
		),
		balance AS (
			SELECT balance_cents
			FROM admin_plus_balance_snapshots
			WHERE supplier_id = $1 AND currency = $2
			ORDER BY captured_at DESC, id DESC
			LIMIT 1
		),
		raw_totals AS (
			SELECT
				ledger.funding_amount,
				ledger.funding_cash,
				ledger.entitlement_amount + GREATEST(auto_redeem_entitlement.amount - ledger.funding_amount, 0) AS entitlement_amount,
				usage.usage_cost AS raw_usage_cost,
				ledger.refund_amount,
				ledger.adjustment_amount,
				(
					ledger.funding_amount
					+ ledger.entitlement_amount
					+ GREATEST(auto_redeem_entitlement.amount - ledger.funding_amount, 0)
					- ledger.refund_amount
					+ ledger.adjustment_amount
				) AS balance_before_usage,
				balance.balance_cents AS actual_balance
			FROM ledger
			CROSS JOIN auto_redeem_entitlement
			CROSS JOIN usage
			LEFT JOIN balance ON TRUE
		),
		totals AS (
			SELECT
				funding_amount,
				funding_cash,
				entitlement_amount,
				CASE
					WHEN actual_balance IS NULL THEN raw_usage_cost
					ELSE GREATEST(balance_before_usage - actual_balance, 0)
				END AS usage_cost,
				refund_amount,
				adjustment_amount,
				balance_before_usage - CASE
					WHEN actual_balance IS NULL THEN raw_usage_cost
					ELSE GREATEST(balance_before_usage - actual_balance, 0)
				END AS expected_balance,
				actual_balance
			FROM raw_totals
		)
		INSERT INTO admin_plus_supplier_cost_snapshots (
			supplier_id, currency, completed_funding_amount_cents, completed_funding_cash_cents,
			entitlement_amount_cents, usage_cost_cents, refund_amount_cents,
			adjustment_amount_cents, expected_balance_cents, actual_balance_cents,
			balance_delta_cents, captured_at
		)
		SELECT
			$1, $2, funding_amount, funding_cash, entitlement_amount, usage_cost,
			refund_amount, adjustment_amount, expected_balance, actual_balance,
			CASE WHEN actual_balance IS NULL THEN NULL ELSE actual_balance - expected_balance END,
			$3
		FROM totals
		RETURNING id, supplier_id, currency, completed_funding_amount_cents,
			completed_funding_cash_cents, entitlement_amount_cents, usage_cost_cents,
			refund_amount_cents, adjustment_amount_cents, expected_balance_cents,
			actual_balance_cents, balance_delta_cents, captured_at, created_at
	`, supplierID, strings.ToUpper(currency), capturedAt)
	return scanCostSnapshot(row)
}

func (r *SQLRepository) ListSnapshots(ctx context.Context, filter SummaryFilter) ([]*adminplusdomain.SupplierCostSnapshot, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	where, args := supplierWhere(filter.SupplierID)
	return r.listSnapshots(ctx, where, args, filter.Limit)
}

func (r *SQLRepository) listSnapshots(ctx context.Context, where []string, args []any, limit int) ([]*adminplusdomain.SupplierCostSnapshot, error) {
	args = append(args, limit)
	query := `
		SELECT id, supplier_id, currency, completed_funding_amount_cents,
			completed_funding_cash_cents, entitlement_amount_cents, usage_cost_cents,
			refund_amount_cents, adjustment_amount_cents, expected_balance_cents,
			actual_balance_cents, balance_delta_cents, captured_at, created_at
		FROM (
			SELECT DISTINCT ON (supplier_id, currency)
				id, supplier_id, currency, completed_funding_amount_cents,
				completed_funding_cash_cents, entitlement_amount_cents, usage_cost_cents,
				refund_amount_cents, adjustment_amount_cents, expected_balance_cents,
				actual_balance_cents, balance_delta_cents, captured_at, created_at
			FROM admin_plus_supplier_cost_snapshots
			WHERE ` + strings.Join(where, " AND ") + `
			ORDER BY supplier_id, currency, captured_at DESC, id DESC
		) latest_snapshots
		ORDER BY captured_at DESC, id DESC
		LIMIT $` + fmt.Sprintf("%d", len(args))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]*adminplusdomain.SupplierCostSnapshot, 0)
	for rows.Next() {
		item, err := scanCostSnapshot(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) ListFundingTransactions(ctx context.Context, filter TransactionFilter) ([]*adminplusdomain.SupplierFundingTransaction, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	where, args := supplierWhere(filter.SupplierID)
	args = append(args, filter.Limit)
	query := `
		SELECT id, supplier_id, provider_type, external_id, out_trade_no, payment_trade_no,
			payment_type, order_type, status, currency, amount_cents, cash_amount_cents,
			refund_amount_cents, fee_rate, created_at_external, paid_at, completed_at,
			raw_payload, last_seen_at, created_at, updated_at
		FROM admin_plus_supplier_funding_transactions
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY COALESCE(completed_at, paid_at, created_at_external, last_seen_at) DESC, id DESC
		LIMIT $` + fmt.Sprintf("%d", len(args))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]*adminplusdomain.SupplierFundingTransaction, 0)
	for rows.Next() {
		item, err := scanFundingTransaction(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) ListEntitlementTransactions(ctx context.Context, filter TransactionFilter) ([]*adminplusdomain.SupplierEntitlementTransaction, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	where, args := supplierWhere(filter.SupplierID)
	args = append(args, filter.Limit)
	query := `
		SELECT id, supplier_id, provider_type, external_id, code_fingerprint, code_last4,
			source_family, type, status, currency, value_cents, raw_value, group_id,
			validity_days, used_at, created_at_external, raw_payload, last_seen_at,
			created_at, updated_at
		FROM admin_plus_supplier_entitlement_transactions
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY COALESCE(used_at, created_at_external, last_seen_at) DESC, id DESC
		LIMIT $` + fmt.Sprintf("%d", len(args))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]*adminplusdomain.SupplierEntitlementTransaction, 0)
	for rows.Next() {
		item, err := scanEntitlementTransaction(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) ListLedgerEntries(ctx context.Context, filter LedgerFilter) ([]*adminplusdomain.SupplierCostLedgerEntry, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	where, args := supplierWhere(filter.SupplierID)
	args = append(args, filter.Limit)
	query := `
		SELECT id, supplier_id, provider_type, entry_type, source_type, source_id,
			source_external_id, currency, amount_cents, cash_amount_cents,
			occurred_at, raw_payload, created_at
		FROM admin_plus_supplier_cost_ledger_entries
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY occurred_at DESC, id DESC
		LIMIT $` + fmt.Sprintf("%d", len(args))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]*adminplusdomain.SupplierCostLedgerEntry, 0)
	for rows.Next() {
		item, err := scanLedgerEntry(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func supplierWhere(supplierID int64) ([]string, []any) {
	where := []string{"1=1"}
	args := make([]any, 0, 2)
	if supplierID > 0 {
		args = append(args, supplierID)
		where = append(where, fmt.Sprintf("supplier_id = $%d", len(args)))
	}
	return where, args
}

type scanner interface {
	Scan(dest ...any) error
}

func scanFundingTransactionWithInserted(s scanner) (*adminplusdomain.SupplierFundingTransaction, bool, error) {
	item, inserted, err := scanFundingTransactionInternal(s, true)
	return item, inserted, err
}

func scanFundingTransaction(s scanner) (*adminplusdomain.SupplierFundingTransaction, error) {
	item, _, err := scanFundingTransactionInternal(s, false)
	return item, err
}

func scanFundingTransactionInternal(s scanner, withInserted bool) (*adminplusdomain.SupplierFundingTransaction, bool, error) {
	var item adminplusdomain.SupplierFundingTransaction
	var feeRate sql.NullFloat64
	var createdAtExternal, paidAt, completedAt sql.NullTime
	var rawPayload []byte
	var inserted bool
	dest := []any{
		&item.ID, &item.SupplierID, &item.ProviderType, &item.ExternalID, &item.OutTradeNo,
		&item.PaymentTradeNo, &item.PaymentType, &item.OrderType, &item.Status, &item.Currency,
		&item.AmountCents, &item.CashAmountCents, &item.RefundAmountCents, &feeRate,
		&createdAtExternal, &paidAt, &completedAt, &rawPayload, &item.LastSeenAt,
		&item.CreatedAt, &item.UpdatedAt,
	}
	if withInserted {
		dest = append(dest, &inserted)
	}
	if err := s.Scan(dest...); err != nil {
		return nil, false, err
	}
	if feeRate.Valid {
		item.FeeRate = &feeRate.Float64
	}
	item.CreatedAtExternal = nullableTimePtr(createdAtExternal)
	item.PaidAt = nullableTimePtr(paidAt)
	item.CompletedAt = nullableTimePtr(completedAt)
	item.RawPayload = unmarshalPayload(rawPayload)
	return &item, inserted, nil
}

func scanEntitlementTransactionWithInserted(s scanner) (*adminplusdomain.SupplierEntitlementTransaction, bool, error) {
	item, inserted, err := scanEntitlementTransactionInternal(s, true)
	return item, inserted, err
}

func scanEntitlementTransaction(s scanner) (*adminplusdomain.SupplierEntitlementTransaction, error) {
	item, _, err := scanEntitlementTransactionInternal(s, false)
	return item, err
}

func scanEntitlementTransactionInternal(s scanner, withInserted bool) (*adminplusdomain.SupplierEntitlementTransaction, bool, error) {
	var item adminplusdomain.SupplierEntitlementTransaction
	var usedAt, createdAtExternal sql.NullTime
	var rawPayload []byte
	var inserted bool
	dest := []any{
		&item.ID, &item.SupplierID, &item.ProviderType, &item.ExternalID, &item.CodeFingerprint,
		&item.CodeLast4, &item.SourceFamily, &item.Type, &item.Status, &item.Currency,
		&item.ValueCents, &item.RawValue, &item.GroupID, &item.ValidityDays, &usedAt,
		&createdAtExternal, &rawPayload, &item.LastSeenAt, &item.CreatedAt, &item.UpdatedAt,
	}
	if withInserted {
		dest = append(dest, &inserted)
	}
	if err := s.Scan(dest...); err != nil {
		return nil, false, err
	}
	item.UsedAt = nullableTimePtr(usedAt)
	item.CreatedAtExternal = nullableTimePtr(createdAtExternal)
	item.RawPayload = unmarshalPayload(rawPayload)
	return &item, inserted, nil
}

func scanLedgerEntry(s scanner) (*adminplusdomain.SupplierCostLedgerEntry, error) {
	item, _, err := scanLedgerEntryInternal(s, false)
	return item, err
}

func scanLedgerEntryWithInserted(s scanner) (*adminplusdomain.SupplierCostLedgerEntry, bool, error) {
	return scanLedgerEntryInternal(s, true)
}

func scanLedgerEntryInternal(s scanner, withInserted bool) (*adminplusdomain.SupplierCostLedgerEntry, bool, error) {
	var item adminplusdomain.SupplierCostLedgerEntry
	var rawPayload []byte
	var inserted bool
	dest := []any{
		&item.ID, &item.SupplierID, &item.ProviderType, &item.EntryType, &item.SourceType,
		&item.SourceID, &item.SourceExternalID, &item.Currency, &item.AmountCents,
		&item.CashAmountCents, &item.OccurredAt, &rawPayload, &item.CreatedAt,
	}
	if withInserted {
		dest = append(dest, &inserted)
	}
	if err := s.Scan(dest...); err != nil {
		return nil, false, err
	}
	item.RawPayload = unmarshalPayload(rawPayload)
	return &item, inserted, nil
}

func scanCostSnapshot(s scanner) (*adminplusdomain.SupplierCostSnapshot, error) {
	var item adminplusdomain.SupplierCostSnapshot
	var actual, delta sql.NullInt64
	err := s.Scan(
		&item.ID, &item.SupplierID, &item.Currency, &item.CompletedFundingAmountCents,
		&item.CompletedFundingCashCents, &item.EntitlementAmountCents, &item.UsageCostCents,
		&item.RefundAmountCents, &item.AdjustmentAmountCents, &item.ExpectedBalanceCents,
		&actual, &delta, &item.CapturedAt, &item.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if actual.Valid {
		item.ActualBalanceCents = &actual.Int64
	}
	if delta.Valid {
		item.BalanceDeltaCents = &delta.Int64
	}
	normalizeCostSnapshotDerivedAmounts(&item)
	return &item, nil
}

func normalizeCostSnapshotDerivedAmounts(item *adminplusdomain.SupplierCostSnapshot) {
	if item == nil || item.ActualBalanceCents == nil {
		return
	}
	balanceBeforeUsage := item.CompletedFundingAmountCents +
		item.EntitlementAmountCents -
		item.RefundAmountCents +
		item.AdjustmentAmountCents
	usageCost := balanceBeforeUsage - *item.ActualBalanceCents
	if usageCost < 0 {
		usageCost = 0
	}
	expectedBalance := balanceBeforeUsage - usageCost
	balanceDelta := *item.ActualBalanceCents - expectedBalance
	item.UsageCostCents = usageCost
	item.ExpectedBalanceCents = expectedBalance
	item.BalanceDeltaCents = &balanceDelta
}

func marshalRawPayload(payload map[string]any) ([]byte, error) {
	if len(payload) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(payload)
}

func unmarshalPayload(data []byte) map[string]any {
	if len(data) == 0 {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}
	return payload
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func nullableFloat64(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}
