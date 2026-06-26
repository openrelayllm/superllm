package balances

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func newBalanceSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return db, mock
}

const updateSupplierBalanceSQLPattern = `UPDATE admin_plus_suppliers\s+SET balance_cents = \$2::BIGINT,\s+balance_currency = \$3::TEXT,\s+balance_updated_at = \$4::TIMESTAMPTZ,\s+runtime_status = CASE\s+WHEN \$2::BIGINT <= 0 AND runtime_status IN \('candidate', 'active'\) THEN 'monitor_only'\s+ELSE runtime_status\s+END,\s+updated_at = NOW\(\)\s+WHERE id = \$1::BIGINT`

func TestSQLRepositoryCreateBalanceSnapshot(t *testing.T) {
	db, mock := newBalanceSQLMock(t)
	repo := NewSQLRepository(db)
	capturedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	createdAt := capturedAt.Add(time.Second)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO admin_plus_balance_snapshots`).
		WithArgs(int64(7), "manual", "candidate", int64(5000), "USD", true, sqlmock.AnyArg(), capturedAt).
		WillReturnRows(newBalanceSnapshotRows().AddRow(
			int64(11), int64(7), "manual", "candidate", int64(5000), "USD", true, []byte(`{"source":"manual"}`), capturedAt, createdAt,
		))
	mock.ExpectExec(updateSupplierBalanceSQLPattern).
		WithArgs(int64(7), int64(5000), "USD", capturedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	got, err := repo.CreateSnapshot(context.Background(), &adminplusdomain.BalanceSnapshot{
		SupplierID:     7,
		Source:         "manual",
		RuntimeStatus:  adminplusdomain.SupplierRuntimeStatusCandidate,
		BalanceCents:   5000,
		Currency:       "USD",
		SwitchEligible: true,
		RawPayload:     map[string]any{"source": "manual"},
		CapturedAt:     capturedAt,
	})

	require.NoError(t, err)
	require.Equal(t, int64(11), got.ID)
	require.Equal(t, adminplusdomain.SupplierRuntimeStatusCandidate, got.RuntimeStatus)
	require.Equal(t, "manual", got.RawPayload["source"])
}

func TestSQLRepositoryCreateBalanceSnapshotRollsBackWhenSupplierUpdateFails(t *testing.T) {
	db, mock := newBalanceSQLMock(t)
	repo := NewSQLRepository(db)
	capturedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	createdAt := capturedAt.Add(time.Second)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO admin_plus_balance_snapshots`).
		WithArgs(int64(7), "provider_session", "monitor_only", int64(0), "USD", false, sqlmock.AnyArg(), capturedAt).
		WillReturnRows(newBalanceSnapshotRows().AddRow(
			int64(11), int64(7), "provider_session", "monitor_only", int64(0), "USD", false, []byte(`{"source":"probe"}`), capturedAt, createdAt,
		))
	mock.ExpectExec(updateSupplierBalanceSQLPattern).
		WithArgs(int64(7), int64(0), "USD", capturedAt).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	got, err := repo.CreateSnapshot(context.Background(), &adminplusdomain.BalanceSnapshot{
		SupplierID:     7,
		Source:         "provider_session",
		RuntimeStatus:  adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		BalanceCents:   0,
		Currency:       "USD",
		SwitchEligible: false,
		RawPayload:     map[string]any{"source": "probe"},
		CapturedAt:     capturedAt,
	})

	require.Nil(t, got)
	require.Error(t, err)
	require.Contains(t, err.Error(), "BALANCE_STORE_FAILED")
	require.Contains(t, err.Error(), "update_supplier_balance")
}

func TestSQLRepositoryFindLatestBalanceSnapshotNoRows(t *testing.T) {
	db, mock := newBalanceSQLMock(t)
	repo := NewSQLRepository(db)
	capturedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM admin_plus_balance_snapshots\s+WHERE supplier_id = \$1`).
		WithArgs(int64(7), "USD", capturedAt).
		WillReturnError(sql.ErrNoRows)

	got, err := repo.FindLatestSnapshot(context.Background(), 7, "USD", capturedAt)

	require.NoError(t, err)
	require.Nil(t, got)
}

func TestSQLRepositoryCreateBalanceEvent(t *testing.T) {
	db, mock := newBalanceSQLMock(t)
	repo := NewSQLRepository(db)
	createdAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	oldBalance := int64(3000)

	mock.ExpectQuery(`INSERT INTO admin_plus_balance_events`).
		WithArgs(
			int64(7),
			int64(11),
			"low_balance",
			"candidate",
			oldBalance,
			int64(500),
			int64(1000),
			"USD",
			true,
			"open",
		).
		WillReturnRows(newBalanceEventRows().AddRow(
			int64(21),
			int64(7),
			int64(11),
			"low_balance",
			"candidate",
			oldBalance,
			int64(500),
			int64(1000),
			"USD",
			true,
			"open",
			createdAt,
			nil,
		))

	got, err := repo.CreateEvent(context.Background(), &adminplusdomain.BalanceEvent{
		SupplierID:               7,
		SnapshotID:               11,
		Type:                     adminplusdomain.BalanceEventTypeLowBalance,
		RuntimeStatus:            adminplusdomain.SupplierRuntimeStatusCandidate,
		OldBalanceCents:          &oldBalance,
		NewBalanceCents:          500,
		LowBalanceThresholdCents: 1000,
		Currency:                 "USD",
		SwitchEligible:           true,
		Status:                   adminplusdomain.BalanceEventStatusOpen,
	})

	require.NoError(t, err)
	require.Equal(t, int64(21), got.ID)
	require.Equal(t, adminplusdomain.BalanceEventTypeLowBalance, got.Type)
	require.NotNil(t, got.OldBalanceCents)
}

func TestSQLRepositoryListBalanceEventsFiltersWithParameterizedQuery(t *testing.T) {
	db, mock := newBalanceSQLMock(t)
	repo := NewSQLRepository(db)
	createdAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM admin_plus_balance_events\s+WHERE 1=1 AND supplier_id = \$1 AND status = \$2\s+ORDER BY created_at DESC, id DESC\s+LIMIT \$3`).
		WithArgs(int64(7), "open", 50).
		WillReturnRows(newBalanceEventRows().AddRow(
			int64(21), int64(7), int64(11), "depleted", "active", nil, int64(0), int64(1000), "USD", false, "open", createdAt, nil,
		))

	items, err := repo.ListEvents(context.Background(), EventFilter{
		SupplierID: 7,
		Status:     adminplusdomain.BalanceEventStatusOpen,
		Limit:      50,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, adminplusdomain.BalanceEventTypeDepleted, items[0].Type)
}

func TestSQLRepositoryUpdateBalanceEventStatusNotFound(t *testing.T) {
	db, mock := newBalanceSQLMock(t)
	repo := NewSQLRepository(db)

	mock.ExpectQuery(`UPDATE admin_plus_balance_events`).
		WithArgs(int64(404), "acknowledged").
		WillReturnError(sql.ErrNoRows)

	_, err := repo.UpdateEventStatus(context.Background(), 404, adminplusdomain.BalanceEventStatusAcknowledged)

	require.Error(t, err)
	require.Contains(t, err.Error(), "BALANCE_EVENT_NOT_FOUND")
}

func newBalanceSnapshotRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"supplier_id",
		"source",
		"runtime_status",
		"balance_cents",
		"currency",
		"switch_eligible",
		"raw_payload",
		"captured_at",
		"created_at",
	})
}

func newBalanceEventRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"supplier_id",
		"snapshot_id",
		"type",
		"runtime_status",
		"old_balance_cents",
		"new_balance_cents",
		"low_balance_threshold_cents",
		"currency",
		"switch_eligible",
		"status",
		"created_at",
		"acknowledged_at",
	})
}
