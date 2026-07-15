package health

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	sub2apiapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sub2api"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func newHealthSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return db, mock
}

func TestSQLRepositoryCreateHealthSample(t *testing.T) {
	db, mock := newHealthSQLMock(t)
	repo := NewSQLRepository(db)
	capturedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	createdAt := capturedAt.Add(time.Second)
	available := 1
	limit := 10

	mock.ExpectQuery(`INSERT INTO admin_plus_health_samples`).
		WithArgs(
			int64(7),
			"manual",
			"gpt-4o-mini",
			int64(1200),
			int64(5000),
			200,
			"",
			3,
			available,
			limit,
			sqlmock.AnyArg(),
			capturedAt,
		).
		WillReturnRows(newHealthSampleRows().AddRow(
			int64(11),
			int64(7),
			"manual",
			"gpt-4o-mini",
			int64(1200),
			int64(5000),
			200,
			"",
			3,
			available,
			limit,
			[]byte(`{"probe":"ok"}`),
			capturedAt,
			createdAt,
		))

	got, err := repo.CreateSample(context.Background(), &adminplusdomain.HealthSample{
		SupplierID:           7,
		Source:               "manual",
		Model:                "gpt-4o-mini",
		FirstTokenLatencyMS:  1200,
		TotalLatencyMS:       5000,
		StatusCode:           200,
		ObservedConcurrency:  3,
		AvailableConcurrency: &available,
		ConcurrencyLimit:     &limit,
		RawPayload:           map[string]any{"probe": "ok"},
		CapturedAt:           capturedAt,
	})

	require.NoError(t, err)
	require.Equal(t, int64(11), got.ID)
	require.NotNil(t, got.AvailableConcurrency)
	require.Equal(t, "ok", got.RawPayload["probe"])
}

func TestSQLRepositoryCreateHealthEvent(t *testing.T) {
	db, mock := newHealthSQLMock(t)
	repo := NewSQLRepository(db)
	createdAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`INSERT INTO admin_plus_health_events`).
		WithArgs(
			int64(7),
			int64(11),
			"slow_first_token",
			"gpt-4o-mini",
			int64(5000),
			int64(3000),
			200,
			"",
			"open",
		).
		WillReturnRows(newHealthEventRows().AddRow(
			int64(21),
			int64(7),
			int64(11),
			"slow_first_token",
			"gpt-4o-mini",
			int64(5000),
			int64(3000),
			200,
			"",
			"open",
			createdAt,
			nil,
		))

	got, err := repo.CreateEvent(context.Background(), &adminplusdomain.HealthEvent{
		SupplierID:     7,
		SampleID:       11,
		Type:           adminplusdomain.HealthEventTypeSlowFirstToken,
		Model:          "gpt-4o-mini",
		ObservedValue:  5000,
		ThresholdValue: 3000,
		StatusCode:     200,
		Status:         adminplusdomain.HealthEventStatusOpen,
	})

	require.NoError(t, err)
	require.Equal(t, int64(21), got.ID)
	require.Equal(t, adminplusdomain.HealthEventTypeSlowFirstToken, got.Type)
}

func TestSQLRepositoryListHealthEventsFiltersWithParameterizedQuery(t *testing.T) {
	db, mock := newHealthSQLMock(t)
	repo := NewSQLRepository(db)
	createdAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM admin_plus_health_events\s+WHERE 1=1 AND supplier_id = \$1 AND status = \$2 AND type = \$3\s+ORDER BY created_at DESC, id DESC\s+LIMIT \$4`).
		WithArgs(int64(7), "open", "request_error", 50).
		WillReturnRows(newHealthEventRows().AddRow(
			int64(21), int64(7), int64(11), "request_error", "gpt-4o-mini", int64(502), int64(0), 502, "bad_gateway", "open", createdAt, nil,
		))

	items, err := repo.ListEvents(context.Background(), EventFilter{
		SupplierID: 7,
		Status:     adminplusdomain.HealthEventStatusOpen,
		Type:       adminplusdomain.HealthEventTypeRequestError,
		Limit:      50,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "bad_gateway", items[0].ErrorClass)
}

func TestSQLRepositoryUpdateHealthEventStatusNotFound(t *testing.T) {
	db, mock := newHealthSQLMock(t)
	repo := NewSQLRepository(db)

	mock.ExpectQuery(`UPDATE admin_plus_health_events`).
		WithArgs(int64(404), "acknowledged").
		WillReturnError(sql.ErrNoRows)

	_, err := repo.UpdateEventStatus(context.Background(), 404, adminplusdomain.HealthEventStatusAcknowledged)

	require.Error(t, err)
	require.Contains(t, err.Error(), "HEALTH_EVENT_NOT_FOUND")
}

func TestSQLRepositoryGetProbeTargetReadsAccountFromSub2APIDatabase(t *testing.T) {
	primaryDB, primaryMock := newHealthSQLMock(t)
	accountDB, accountMock := newHealthSQLMock(t)
	repo := NewSQLRepositoryWithReadDB(primaryDB, sub2apiapp.ReadDB{DB: accountDB, Configured: true})

	primaryMock.ExpectQuery(`FROM admin_plus_supplier_accounts asa\s+INNER JOIN admin_plus_suppliers s`).
		WithArgs(int64(7), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{
			"supplier_id", "supplier_name", "api_base_url", "supplier_account_id",
			"local_account_id", "local_account_name", "local_account_platform", "local_account_type",
		}).AddRow(int64(7), "Relay", "https://supplier.example", int64(11), int64(101), "stale", "openai", "apikey"))
	accountMock.ExpectQuery(`FROM accounts\s+WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs(int64(101)).
		WillReturnRows(sqlmock.NewRows([]string{
			"name", "platform", "type", "status", "schedulable", "concurrency", "credentials",
		}).AddRow("Sub2API account", "openai", "apikey", "active", true, 8, []byte(`{"api_key":"sk-test","base_url":"https://relay.example/v1"}`)))

	target, err := repo.GetProbeTarget(context.Background(), 7, 11)

	require.NoError(t, err)
	require.Equal(t, int64(101), target.LocalAccountID)
	require.Equal(t, "Sub2API account", target.LocalAccountName)
	require.Equal(t, "sk-test", target.APIKey)
	require.Equal(t, "https://relay.example/v1", target.AccountBaseURL)
	require.Equal(t, 8, target.LocalAccountConcurrency)
}

func newHealthSampleRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"supplier_id",
		"source",
		"model",
		"first_token_latency_ms",
		"total_latency_ms",
		"status_code",
		"error_class",
		"observed_concurrency",
		"available_concurrency",
		"concurrency_limit",
		"raw_payload",
		"captured_at",
		"created_at",
	})
}

func newHealthEventRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"supplier_id",
		"sample_id",
		"type",
		"model",
		"observed_value",
		"threshold_value",
		"status_code",
		"error_class",
		"status",
		"created_at",
		"acknowledged_at",
	})
}
