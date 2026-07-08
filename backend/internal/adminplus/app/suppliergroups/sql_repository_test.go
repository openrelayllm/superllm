package suppliergroups

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func newSupplierGroupSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return db, mock
}

func TestSQLRepositoryCreateChangeEvents(t *testing.T) {
	db, mock := newSupplierGroupSQLMock(t)
	repo := NewSQLRepository(db)
	createdAt := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)
	oldRate := 0.13
	changePercent := 100.0

	mock.ExpectQuery(`INSERT INTO admin_plus_supplier_group_change_events`).
		WithArgs(
			int64(7),
			int64(11),
			"gpt-low",
			"gpt-low",
			"openai",
			"increase",
			oldRate,
			0.26,
			changePercent,
			true,
			createdAt,
		).
		WillReturnRows(newSupplierGroupChangeEventRows().AddRow(
			int64(21),
			int64(7),
			int64(11),
			"gpt-low",
			"gpt-low",
			"openai",
			"increase",
			oldRate,
			0.26,
			changePercent,
			true,
			createdAt,
		))

	items, err := repo.CreateChangeEvents(context.Background(), []*adminplusdomain.SupplierGroupChangeEvent{
		{
			SupplierID:                 7,
			SupplierGroupID:            11,
			ExternalGroupID:            "gpt-low",
			GroupName:                  "gpt-low",
			ProviderFamily:             "openai",
			Direction:                  adminplusdomain.SupplierGroupChangeDirectionIncrease,
			OldEffectiveRateMultiplier: &oldRate,
			NewEffectiveRateMultiplier: 0.26,
			ChangePercent:              &changePercent,
			LowRate:                    true,
			CreatedAt:                  createdAt,
		},
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(21), items[0].ID)
	require.Equal(t, adminplusdomain.SupplierGroupChangeDirectionIncrease, items[0].Direction)
}

func TestSQLRepositoryListChangeEvents(t *testing.T) {
	db, mock := newSupplierGroupSQLMock(t)
	repo := NewSQLRepository(db)
	createdAt := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM admin_plus_supplier_group_change_events\s+WHERE supplier_id = \$1 AND direction = \$2 AND low_rate = \$3\s+ORDER BY created_at DESC, id DESC\s+LIMIT \$4`).
		WithArgs(int64(7), "new", true, 20).
		WillReturnRows(newSupplierGroupChangeEventRows().AddRow(
			int64(21),
			int64(7),
			int64(11),
			"gpt-low",
			"gpt-low",
			"openai",
			"new",
			nil,
			0.05,
			nil,
			true,
			createdAt,
		))

	lowRate := true
	items, err := repo.ListChangeEvents(context.Background(), EventFilter{
		SupplierID: 7,
		Direction:  adminplusdomain.SupplierGroupChangeDirectionNew,
		LowRate:    &lowRate,
		Limit:      20,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.True(t, items[0].LowRate)
	require.Nil(t, items[0].OldEffectiveRateMultiplier)
}

func TestSQLRepositoryListSupportsGlobalSupplierGroupLookup(t *testing.T) {
	db, mock := newSupplierGroupSQLMock(t)
	repo := NewSQLRepository(db)
	lastSeenAt := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 7, 8, 11, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM admin_plus_supplier_groups\s+WHERE status = \$1 AND \(LOWER\(name\) LIKE \$2`).
		WithArgs(
			"active",
			"%gpt%",
			"%gpt%",
			"%gpt%",
			"%gpt%",
			"%gpt%",
			"%gpt%",
			"%gpt%",
			"%gpt%",
			50,
		).
		WillReturnRows(newSupplierGroupRows().AddRow(
			int64(101),
			int64(7),
			"gpt-low",
			"GPT Low",
			"low rate",
			"openai",
			"gpt",
			"gpt",
			"gpt-4o",
			"lime-openai-gpt-low",
			1.0,
			nil,
			0.05,
			nil,
			nil,
			nil,
			nil,
			true,
			false,
			"limited",
			2,
			1,
			"active",
			[]byte(`{"id":"gpt-low"}`),
			lastSeenAt,
			nil,
			createdAt,
			updatedAt,
		))

	items, err := repo.List(context.Background(), ListFilter{
		Status: adminplusdomain.SupplierGroupStatusActive,
		Query:  "gpt",
		Limit:  50,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(101), items[0].ID)
	require.Equal(t, int64(7), items[0].SupplierID)
	require.Equal(t, "GPT Low", items[0].Name)
	require.Equal(t, 0.05, items[0].EffectiveRateMultiplier)
}

func newSupplierGroupChangeEventRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"supplier_id",
		"supplier_group_id",
		"external_group_id",
		"group_name",
		"provider_family",
		"direction",
		"old_effective_rate_multiplier",
		"new_effective_rate_multiplier",
		"change_percent",
		"low_rate",
		"created_at",
	})
}

func newSupplierGroupRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"supplier_id",
		"external_group_id",
		"name",
		"description",
		"provider_family",
		"official_name",
		"model_family",
		"model_spec",
		"standard_key_name",
		"rate_multiplier",
		"user_rate_multiplier",
		"effective_rate_multiplier",
		"rpm_limit",
		"daily_limit_usd",
		"weekly_limit_usd",
		"monthly_limit_usd",
		"allow_image_generation",
		"is_private",
		"key_limit_policy",
		"key_limit_value",
		"active_key_count",
		"status",
		"raw_payload",
		"last_seen_at",
		"naming_updated_at",
		"created_at",
		"updated_at",
	})
}
