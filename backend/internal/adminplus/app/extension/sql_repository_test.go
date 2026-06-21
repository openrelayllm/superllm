package extension

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func newExtensionSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return db, mock
}

func TestSQLRepositoryCreateExtensionTask(t *testing.T) {
	db, mock := newExtensionSQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`INSERT INTO admin_plus_extension_tasks`).
		WithArgs(
			int64(7),
			"fetch_rates",
			"",
			"pending",
			10,
			0,
			3,
			"",
			"",
			nil,
			nil,
			now,
			sqlmock.AnyArg(),
			"",
			"",
			now,
			now,
			nil,
		).
		WillReturnRows(newExtensionTaskRows().AddRow(
			int64(11),
			int64(7),
			"fetch_rates",
			"",
			"pending",
			10,
			0,
			3,
			"",
			"",
			nil,
			nil,
			now,
			[]byte(`{"page":"rates"}`),
			[]byte(`{}`),
			"",
			"",
			now,
			now,
			nil,
		))

	got, err := repo.CreateTask(context.Background(), &adminplusdomain.ExtensionTask{
		SupplierID:     7,
		Type:           adminplusdomain.ExtensionTaskTypeFetchRates,
		Status:         adminplusdomain.ExtensionTaskStatusPending,
		Priority:       10,
		MaxAttempts:    3,
		AvailableAfter: now,
		Payload:        map[string]any{"page": "rates"},
		CreatedAt:      now,
		UpdatedAt:      now,
	})

	require.NoError(t, err)
	require.Equal(t, int64(11), got.ID)
	require.Equal(t, "rates", got.Payload["page"])
}

func TestSQLRepositoryCreateExtensionTaskIfAbsentCreatesTask(t *testing.T) {
	db, mock := newExtensionSQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	scheduleKey := "scheduler:fetch_rates:supplier:7:202606201000"

	mock.ExpectQuery(`INSERT INTO admin_plus_extension_tasks[\s\S]+ON CONFLICT \(schedule_key\)`).
		WithArgs(
			int64(7),
			"fetch_rates",
			scheduleKey,
			"pending",
			80,
			0,
			3,
			"",
			"",
			nil,
			nil,
			now,
			sqlmock.AnyArg(),
			"",
			"",
			now,
			now,
			nil,
		).
		WillReturnRows(newExtensionTaskRows().AddRow(
			int64(11),
			int64(7),
			"fetch_rates",
			scheduleKey,
			"pending",
			80,
			0,
			3,
			"",
			"",
			nil,
			nil,
			now,
			[]byte(`{"source":"scheduler"}`),
			[]byte(`{}`),
			"",
			"",
			now,
			now,
			nil,
		))

	got, created, err := repo.CreateTaskIfAbsent(context.Background(), &adminplusdomain.ExtensionTask{
		SupplierID:     7,
		Type:           adminplusdomain.ExtensionTaskTypeFetchRates,
		ScheduleKey:    scheduleKey,
		Status:         adminplusdomain.ExtensionTaskStatusPending,
		Priority:       80,
		MaxAttempts:    3,
		AvailableAfter: now,
		Payload:        map[string]any{"source": "scheduler"},
		CreatedAt:      now,
		UpdatedAt:      now,
	})

	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, int64(11), got.ID)
	require.Equal(t, scheduleKey, got.ScheduleKey)
}

func TestSQLRepositoryCreateExtensionTaskIfAbsentReturnsExistingTask(t *testing.T) {
	db, mock := newExtensionSQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	scheduleKey := "scheduler:fetch_rates:supplier:7:202606201000"

	mock.ExpectQuery(`INSERT INTO admin_plus_extension_tasks[\s\S]+ON CONFLICT \(schedule_key\)`).
		WithArgs(
			int64(7),
			"fetch_rates",
			scheduleKey,
			"pending",
			80,
			0,
			3,
			"",
			"",
			nil,
			nil,
			now,
			sqlmock.AnyArg(),
			"",
			"",
			now,
			now,
			nil,
		).
		WillReturnError(sql.ErrNoRows)

	mock.ExpectQuery(`SELECT id, supplier_id, type, schedule_key`).
		WithArgs(scheduleKey).
		WillReturnRows(newExtensionTaskRows().AddRow(
			int64(11),
			int64(7),
			"fetch_rates",
			scheduleKey,
			"pending",
			80,
			0,
			3,
			"",
			"",
			nil,
			nil,
			now,
			[]byte(`{"source":"scheduler"}`),
			[]byte(`{}`),
			"",
			"",
			now,
			now,
			nil,
		))

	got, created, err := repo.CreateTaskIfAbsent(context.Background(), &adminplusdomain.ExtensionTask{
		SupplierID:     7,
		Type:           adminplusdomain.ExtensionTaskTypeFetchRates,
		ScheduleKey:    scheduleKey,
		Status:         adminplusdomain.ExtensionTaskStatusPending,
		Priority:       80,
		MaxAttempts:    3,
		AvailableAfter: now,
		Payload:        map[string]any{"source": "scheduler"},
		CreatedAt:      now,
		UpdatedAt:      now,
	})

	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, int64(11), got.ID)
	require.Equal(t, scheduleKey, got.ScheduleKey)
}

func TestSQLRepositoryClaimNextTaskUsesAtomicUpdate(t *testing.T) {
	db, mock := newExtensionSQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Minute)

	mock.ExpectQuery(`WITH next_task AS \(\s+SELECT id\s+FROM admin_plus_extension_tasks[\s\S]+FOR UPDATE SKIP LOCKED[\s\S]+UPDATE admin_plus_extension_tasks`).
		WithArgs(now, sqlmock.AnyArg(), "chrome-1", "lease-token", expiresAt).
		WillReturnRows(newExtensionTaskRows().AddRow(
			int64(11),
			int64(7),
			"fetch_usage_costs",
			"",
			"claimed",
			10,
			1,
			3,
			"chrome-1",
			"lease-token",
			expiresAt,
			now,
			now,
			[]byte(`{}`),
			[]byte(`{}`),
			"",
			"",
			now,
			now,
			nil,
		))

	got, err := repo.ClaimNextTask(context.Background(), now, []adminplusdomain.ExtensionTaskType{
		adminplusdomain.ExtensionTaskTypeFetchUsageCosts,
	}, Lease{
		DeviceID:  "chrome-1",
		Token:     "lease-token",
		ExpiresAt: expiresAt,
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ExtensionTaskStatusClaimed, got.Status)
	require.Equal(t, "chrome-1", got.DeviceID)
	require.Equal(t, 1, got.Attempts)
}

func TestSQLRepositoryClaimNextTaskNoRows(t *testing.T) {
	db, mock := newExtensionSQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`WITH next_task AS`).
		WithArgs(now, sqlmock.AnyArg(), "chrome-1", "lease-token", now.Add(time.Minute)).
		WillReturnError(sql.ErrNoRows)

	got, err := repo.ClaimNextTask(context.Background(), now, nil, Lease{
		DeviceID:  "chrome-1",
		Token:     "lease-token",
		ExpiresAt: now.Add(time.Minute),
	})

	require.NoError(t, err)
	require.Nil(t, got)
}

func TestSQLRepositoryUpdateExtensionTaskNotFound(t *testing.T) {
	db, mock := newExtensionSQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`UPDATE admin_plus_extension_tasks`).
		WithArgs(
			int64(404),
			"succeeded",
			0,
			1,
			3,
			"chrome-1",
			"lease-token",
			nil,
			nil,
			now,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			"",
			"",
			now,
			now,
		).
		WillReturnError(sql.ErrNoRows)

	_, err := repo.UpdateTask(context.Background(), &adminplusdomain.ExtensionTask{
		ID:             404,
		Status:         adminplusdomain.ExtensionTaskStatusSucceeded,
		Attempts:       1,
		MaxAttempts:    3,
		DeviceID:       "chrome-1",
		LeaseToken:     "lease-token",
		AvailableAfter: now,
		UpdatedAt:      now,
		FinishedAt:     &now,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "EXTENSION_TASK_NOT_FOUND")
}

func TestSQLRepositoryListExtensionTasksFiltersWithParameterizedQuery(t *testing.T) {
	db, mock := newExtensionSQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM admin_plus_extension_tasks\s+WHERE 1=1 AND supplier_id = \$1 AND status = \$2 AND type = \$3\s+ORDER BY created_at DESC, id DESC\s+LIMIT \$4`).
		WithArgs(int64(7), "pending", "fetch_rates", 50).
		WillReturnRows(newExtensionTaskRows().AddRow(
			int64(11),
			int64(7),
			"fetch_rates",
			"",
			"pending",
			10,
			0,
			3,
			"",
			"",
			nil,
			nil,
			now,
			[]byte(`{}`),
			[]byte(`{}`),
			"",
			"",
			now,
			now,
			nil,
		))

	items, err := repo.ListTasks(context.Background(), TaskFilter{
		SupplierID: 7,
		Status:     adminplusdomain.ExtensionTaskStatusPending,
		Type:       adminplusdomain.ExtensionTaskTypeFetchRates,
		Limit:      50,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, adminplusdomain.ExtensionTaskTypeFetchRates, items[0].Type)
}

func newExtensionTaskRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"supplier_id",
		"type",
		"schedule_key",
		"status",
		"priority",
		"attempts",
		"max_attempts",
		"device_id",
		"lease_token",
		"lease_expires_at",
		"last_heartbeat_at",
		"available_after",
		"payload",
		"result",
		"error_code",
		"error_message",
		"created_at",
		"updated_at",
		"finished_at",
	})
}
