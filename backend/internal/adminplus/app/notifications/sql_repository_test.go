package notifications

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func newNotificationSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return db, mock
}

func TestSQLRepositoryCreateDelivery(t *testing.T) {
	db, mock := newNotificationSQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`INSERT INTO admin_plus_notification_deliveries`).
		WithArgs("feishu", "balance.low_balance", int64(21), int64(7), "feishu:balance.low_balance:21", "sending", sqlmock.AnyArg()).
		WillReturnRows(newNotificationDeliveryRows().AddRow(
			int64(1),
			"feishu",
			"balance.low_balance",
			int64(21),
			int64(7),
			"feishu:balance.low_balance:21",
			"sending",
			1,
			"",
			[]byte(`{"msg_type":"text"}`),
			nil,
			now,
			now,
		))

	got, created, err := repo.CreateDelivery(context.Background(), &adminplusdomain.NotificationDelivery{
		Channel:    adminplusdomain.NotificationChannelFeishu,
		EventType:  "balance.low_balance",
		EventID:    21,
		SupplierID: 7,
		DedupeKey:  "feishu:balance.low_balance:21",
		Payload:    map[string]any{"msg_type": "text"},
	})

	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, int64(1), got.ID)
	require.Equal(t, adminplusdomain.NotificationStatusSending, got.Status)
	require.Equal(t, "text", got.Payload["msg_type"])
}

func TestSQLRepositoryMarkDeliverySucceeded(t *testing.T) {
	db, mock := newNotificationSQLMock(t)
	repo := NewSQLRepository(db)

	mock.ExpectExec(`UPDATE admin_plus_notification_deliveries`).
		WithArgs(int64(1), "succeeded").
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.MarkDeliverySucceeded(context.Background(), 1))
}

func TestSQLRepositoryMarkDeliveryFailed(t *testing.T) {
	db, mock := newNotificationSQLMock(t)
	repo := NewSQLRepository(db)

	mock.ExpectExec(`UPDATE admin_plus_notification_deliveries`).
		WithArgs(int64(1), "failed", "webhook failed").
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.MarkDeliveryFailed(context.Background(), 1, "webhook failed"))
}

func TestSQLRepositoryListDeliveriesFilters(t *testing.T) {
	db, mock := newNotificationSQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM admin_plus_notification_deliveries\s+WHERE 1=1 AND supplier_id = \$1 AND channel = \$2 AND status = \$3 AND event_type = \$4 ORDER BY created_at DESC, id DESC LIMIT \$5`).
		WithArgs(int64(7), "feishu", "failed", "health.request_error", 20).
		WillReturnRows(newNotificationDeliveryRows().AddRow(
			int64(3),
			"feishu",
			"health.request_error",
			int64(31),
			int64(7),
			"feishu:health.request_error:31",
			"failed",
			1,
			"webhook failed",
			[]byte(`{"msg_type":"text"}`),
			nil,
			now,
			now,
		))

	items, err := repo.ListDeliveries(context.Background(), DeliveryFilter{
		SupplierID: 7,
		Channel:    adminplusdomain.NotificationChannelFeishu,
		Status:     adminplusdomain.NotificationStatusFailed,
		EventType:  "health.request_error",
		Limit:      20,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "webhook failed", items[0].LastError)
}

func newNotificationDeliveryRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"channel",
		"event_type",
		"event_id",
		"supplier_id",
		"dedupe_key",
		"status",
		"attempts",
		"last_error",
		"payload",
		"sent_at",
		"created_at",
		"updated_at",
	})
}
