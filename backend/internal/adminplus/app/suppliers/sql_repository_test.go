package suppliers

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func newSupplierSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return db, mock
}

func TestSQLRepositoryCreatePersistsSupplier(t *testing.T) {
	db, mock := newSupplierSQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	balanceUpdatedAt := now

	mock.ExpectQuery(`INSERT INTO admin_plus_suppliers`).
		WithArgs(
			"Primary Relay",
			"relay",
			"sub2api",
			"candidate",
			"normal",
			"https://relay.example.com/admin",
			"https://relay.example.com",
			"ops@example.com",
			"primary upstream",
			true,
			true,
			true,
			true,
			true,
			false,
			"op***@example.com",
			int64(5000),
			"USD",
			&balanceUpdatedAt,
			now,
			now,
		).
		WillReturnRows(newSupplierRows().AddRow(
			int64(7),
			"Primary Relay",
			"relay",
			"sub2api",
			"candidate",
			"normal",
			"https://relay.example.com/admin",
			"https://relay.example.com",
			"ops@example.com",
			"primary upstream",
			true,
			true,
			true,
			true,
			true,
			false,
			"op***@example.com",
			int64(5000),
			"USD",
			balanceUpdatedAt,
			now,
			now,
		))

	got, err := repo.Create(context.Background(), &adminplusdomain.Supplier{
		Name:          "Primary Relay",
		Kind:          adminplusdomain.SupplierKindRelay,
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusCandidate,
		HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:  "https://relay.example.com/admin",
		APIBaseURL:    "https://relay.example.com",
		Contact:       "ops@example.com",
		Notes:         "primary upstream",
		Credential: adminplusdomain.SupplierCredentialStatus{
			PostgresConfigured:             true,
			RedisConfigured:                true,
			BrowserLoginEnabled:            true,
			BrowserLoginUsernameConfigured: true,
			BrowserLoginPasswordConfigured: true,
			MaskedBrowserLoginUsername:     "op***@example.com",
		},
		BalanceCents:     5000,
		BalanceCurrency:  "USD",
		BalanceUpdatedAt: &balanceUpdatedAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	})

	require.NoError(t, err)
	require.Equal(t, int64(7), got.ID)
	require.Equal(t, adminplusdomain.SupplierRuntimeStatusCandidate, got.RuntimeStatus)
	require.True(t, got.Credential.PostgresConfigured)
}

func TestSQLRepositoryListFiltersWithParameterizedQuery(t *testing.T) {
	db, mock := newSupplierSQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM admin_plus_suppliers\s+WHERE 1=1 AND kind = \$1 AND type = \$2 AND runtime_status = \$3 AND health_status = \$4 AND \(LOWER\(name\) LIKE \$5 OR LOWER\(contact\) LIKE \$6 OR LOWER\(notes\) LIKE \$7\)`).
		WithArgs("relay", "sub2api", "monitor_only", "normal", "%relay%", "%relay%", "%relay%").
		WillReturnRows(newSupplierRows().AddRow(
			int64(1),
			"Relay",
			"relay",
			"sub2api",
			"monitor_only",
			"normal",
			"",
			"",
			"",
			"",
			false,
			false,
			false,
			false,
			false,
			false,
			"",
			int64(0),
			"USD",
			nil,
			now,
			now,
		))

	items, err := repo.List(context.Background(), SupplierFilter{
		Kind:          adminplusdomain.SupplierKindRelay,
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
		Query:         "Relay",
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "Relay", items[0].Name)
}

func TestSQLRepositoryUpdateStatus(t *testing.T) {
	db, mock := newSupplierSQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`UPDATE admin_plus_suppliers`).
		WithArgs(int64(9), "disabled", "paused").
		WillReturnRows(newSupplierRows().AddRow(
			int64(9),
			"Paused Relay",
			"relay",
			"sub2api",
			"disabled",
			"paused",
			"",
			"",
			"",
			"",
			false,
			false,
			false,
			false,
			false,
			false,
			"",
			int64(0),
			"USD",
			nil,
			now,
			now,
		))

	got, err := repo.UpdateStatus(context.Background(), 9, adminplusdomain.SupplierRuntimeStatusDisabled, adminplusdomain.SupplierHealthStatusPaused)

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierRuntimeStatusDisabled, got.RuntimeStatus)
	require.Equal(t, adminplusdomain.SupplierHealthStatusPaused, got.HealthStatus)
}

func TestSQLRepositoryGetNotFound(t *testing.T) {
	db, mock := newSupplierSQLMock(t)
	repo := NewSQLRepository(db)

	mock.ExpectQuery(`FROM admin_plus_suppliers\s+WHERE id = \$1`).
		WithArgs(int64(404)).
		WillReturnError(sql.ErrNoRows)

	_, err := repo.Get(context.Background(), 404)

	require.Error(t, err)
	require.Contains(t, err.Error(), "SUPPLIER_NOT_FOUND")
}

func newSupplierRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"name",
		"kind",
		"type",
		"runtime_status",
		"health_status",
		"dashboard_url",
		"api_base_url",
		"contact",
		"notes",
		"postgres_configured",
		"redis_configured",
		"browser_login_enabled",
		"browser_login_username_configured",
		"browser_login_password_configured",
		"browser_login_token_configured",
		"masked_browser_login_username",
		"balance_cents",
		"balance_currency",
		"balance_updated_at",
		"created_at",
		"updated_at",
	})
}
