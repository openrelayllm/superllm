package balances

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) CreateSnapshot(ctx context.Context, snapshot *adminplusdomain.BalanceSnapshot) (*adminplusdomain.BalanceSnapshot, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	rawPayload, err := marshalRawPayload(snapshot.RawPayload)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, balanceStoreError("begin_transaction", err, snapshot.SupplierID)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	row := tx.QueryRowContext(ctx, `
		INSERT INTO admin_plus_balance_snapshots (
			supplier_id, source, runtime_status, balance_cents, currency,
			switch_eligible, raw_payload, captured_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, supplier_id, source, runtime_status, balance_cents,
			currency, switch_eligible, raw_payload, captured_at, created_at
	`,
		snapshot.SupplierID,
		snapshot.Source,
		string(snapshot.RuntimeStatus),
		snapshot.BalanceCents,
		snapshot.Currency,
		snapshot.SwitchEligible,
		rawPayload,
		snapshot.CapturedAt,
	)
	created, err := scanBalanceSnapshot(row)
	if err != nil {
		return nil, balanceStoreError("insert_snapshot", err, snapshot.SupplierID)
	}
	result, err := tx.ExecContext(ctx, `
			UPDATE admin_plus_suppliers
			SET balance_cents = $2::BIGINT,
				balance_currency = $3::TEXT,
				balance_updated_at = $4::TIMESTAMPTZ,
				runtime_status = CASE
					WHEN $2::BIGINT <= 0 AND runtime_status IN ('candidate', 'active') THEN 'monitor_only'
					ELSE runtime_status
				END,
				updated_at = NOW()
			WHERE id = $1::BIGINT
			`, created.SupplierID, created.BalanceCents, created.Currency, created.CapturedAt)
	if err != nil {
		return nil, balanceStoreError("update_supplier_balance", err, created.SupplierID)
	}
	if rows, err := result.RowsAffected(); err == nil && rows == 0 {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found while recording balance snapshot").
			WithMetadata(map[string]string{
				"store_step":  "update_supplier_balance",
				"supplier_id": strconv.FormatInt(created.SupplierID, 10),
			})
	}
	if err := tx.Commit(); err != nil {
		return nil, balanceStoreError("commit_transaction", err, created.SupplierID)
	}
	committed = true
	return created, nil
}

func (r *SQLRepository) FindLatestSnapshot(ctx context.Context, supplierID int64, currency string, capturedAt time.Time) (*adminplusdomain.BalanceSnapshot, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, supplier_id, source, runtime_status, balance_cents,
			currency, switch_eligible, raw_payload, captured_at, created_at
		FROM admin_plus_balance_snapshots
		WHERE supplier_id = $1
			AND currency = $2
			AND captured_at <= $3
		ORDER BY captured_at DESC, id DESC
		LIMIT 1
	`, supplierID, currency, capturedAt)
	item, err := scanBalanceSnapshot(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (r *SQLRepository) CreateEvent(ctx context.Context, event *adminplusdomain.BalanceEvent) (*adminplusdomain.BalanceEvent, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_balance_events (
			supplier_id, snapshot_id, type, runtime_status, old_balance_cents,
			new_balance_cents, low_balance_threshold_cents, currency, switch_eligible, status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, supplier_id, snapshot_id, type, runtime_status, old_balance_cents,
			new_balance_cents, low_balance_threshold_cents, currency, switch_eligible,
			status, created_at, acknowledged_at
	`,
		event.SupplierID,
		event.SnapshotID,
		string(event.Type),
		string(event.RuntimeStatus),
		nullableInt64(event.OldBalanceCents),
		event.NewBalanceCents,
		event.LowBalanceThresholdCents,
		event.Currency,
		event.SwitchEligible,
		string(event.Status),
	)
	return scanBalanceEvent(row)
}

func (r *SQLRepository) ListSnapshots(ctx context.Context, filter SnapshotFilter) ([]*adminplusdomain.BalanceSnapshot, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	where := []string{"1=1"}
	args := make([]any, 0, 2)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if filter.SupplierID > 0 {
		where = append(where, "supplier_id = "+addArg(filter.SupplierID))
	}
	limitRef := addArg(filter.Limit)
	query := `
		SELECT id, supplier_id, source, runtime_status, balance_cents,
			currency, switch_eligible, raw_payload, captured_at, created_at
		FROM admin_plus_balance_snapshots
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY captured_at DESC, id DESC
		LIMIT ` + limitRef

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]*adminplusdomain.BalanceSnapshot, 0)
	for rows.Next() {
		item, err := scanBalanceSnapshot(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SQLRepository) ListEvents(ctx context.Context, filter EventFilter) ([]*adminplusdomain.BalanceEvent, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	where := []string{"1=1"}
	args := make([]any, 0, 3)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if filter.SupplierID > 0 {
		where = append(where, "supplier_id = "+addArg(filter.SupplierID))
	}
	if filter.Status != "" {
		where = append(where, "status = "+addArg(string(filter.Status)))
	}
	limitRef := addArg(filter.Limit)
	query := `
		SELECT id, supplier_id, snapshot_id, type, runtime_status, old_balance_cents,
			new_balance_cents, low_balance_threshold_cents, currency, switch_eligible,
			status, created_at, acknowledged_at
		FROM admin_plus_balance_events
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY created_at DESC, id DESC
		LIMIT ` + limitRef

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]*adminplusdomain.BalanceEvent, 0)
	for rows.Next() {
		item, err := scanBalanceEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SQLRepository) UpdateEventStatus(ctx context.Context, id int64, status adminplusdomain.BalanceEventStatus) (*adminplusdomain.BalanceEvent, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_balance_events
		SET status = $2,
			acknowledged_at = CASE WHEN $2 = 'acknowledged' THEN NOW() ELSE NULL END
		WHERE id = $1
		RETURNING id, supplier_id, snapshot_id, type, runtime_status, old_balance_cents,
			new_balance_cents, low_balance_threshold_cents, currency, switch_eligible,
			status, created_at, acknowledged_at
	`, id, string(status))
	event, err := scanBalanceEvent(row)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "BALANCE_EVENT_NOT_FOUND", "balance event not found")
	}
	return event, err
}

type balanceScanner interface {
	Scan(dest ...any) error
}

func scanBalanceSnapshot(scanner balanceScanner) (*adminplusdomain.BalanceSnapshot, error) {
	var snapshot adminplusdomain.BalanceSnapshot
	var rawPayload []byte
	var runtimeStatus string
	err := scanner.Scan(
		&snapshot.ID,
		&snapshot.SupplierID,
		&snapshot.Source,
		&runtimeStatus,
		&snapshot.BalanceCents,
		&snapshot.Currency,
		&snapshot.SwitchEligible,
		&rawPayload,
		&snapshot.CapturedAt,
		&snapshot.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	snapshot.RuntimeStatus = adminplusdomain.SupplierRuntimeStatus(runtimeStatus)
	if len(rawPayload) > 0 {
		var payload map[string]any
		if err := json.Unmarshal(rawPayload, &payload); err != nil {
			return nil, err
		}
		snapshot.RawPayload = payload
	}
	return &snapshot, nil
}

func scanBalanceEvent(scanner balanceScanner) (*adminplusdomain.BalanceEvent, error) {
	var event adminplusdomain.BalanceEvent
	var eventType, runtimeStatus, status string
	var oldBalance sql.NullInt64
	var acknowledgedAt sql.NullTime
	err := scanner.Scan(
		&event.ID,
		&event.SupplierID,
		&event.SnapshotID,
		&eventType,
		&runtimeStatus,
		&oldBalance,
		&event.NewBalanceCents,
		&event.LowBalanceThresholdCents,
		&event.Currency,
		&event.SwitchEligible,
		&status,
		&event.CreatedAt,
		&acknowledgedAt,
	)
	if err != nil {
		return nil, err
	}
	event.Type = adminplusdomain.BalanceEventType(eventType)
	event.RuntimeStatus = adminplusdomain.SupplierRuntimeStatus(runtimeStatus)
	event.Status = adminplusdomain.BalanceEventStatus(status)
	if oldBalance.Valid {
		v := oldBalance.Int64
		event.OldBalanceCents = &v
	}
	if acknowledgedAt.Valid {
		t := acknowledgedAt.Time
		event.AcknowledgedAt = &t
	}
	return &event, nil
}

func marshalRawPayload(payload map[string]any) ([]byte, error) {
	if len(payload) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(payload)
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func dbNotConfigured() error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
}

func balanceStoreError(step string, err error, supplierID int64) error {
	if err == nil {
		return nil
	}
	return infraerrors.New(http.StatusInternalServerError, "BALANCE_STORE_FAILED", "failed to record supplier balance snapshot").
		WithCause(err).
		WithMetadata(map[string]string{
			"store_step":    step,
			"supplier_id":   strconv.FormatInt(supplierID, 10),
			"error_message": err.Error(),
		})
}
