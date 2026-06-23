package suppliers

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	sub2apiapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sub2api"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/lib/pq"
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
	repo := NewSQLRepository(db, sub2apiapp.ReadDB{DB: db})
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
			"https://relay.example.com/custom/topup",
			"https://sub2apiplus.example.com/custom/topup",
			"ops@example.com",
			"primary upstream",
			true,
			true,
			true,
			true,
			true,
			false,
			"op***@example.com",
			"ops@example.com",
			"secret",
			"",
			int64(5000),
			"USD",
			&balanceUpdatedAt,
			float64(1),
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
			"https://relay.example.com/custom/topup",
			"https://sub2apiplus.example.com/custom/topup",
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
			float64(1),
			now,
			now,
		))

	got, err := repo.Create(context.Background(), &adminplusdomain.Supplier{
		Name:                  "Primary Relay",
		Kind:                  adminplusdomain.SupplierKindRelay,
		Type:                  adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:         adminplusdomain.SupplierRuntimeStatusCandidate,
		HealthStatus:          adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:          "https://relay.example.com/admin",
		APIBaseURL:            "https://relay.example.com",
		ThirdPartyRechargeURL: "https://relay.example.com/custom/topup",
		LocalRechargeURL:      "https://sub2apiplus.example.com/custom/topup",
		Contact:               "ops@example.com",
		Notes:                 "primary upstream",
		BrowserLoginUsername:  "ops@example.com",
		BrowserLoginPassword:  "secret",
		Credential: adminplusdomain.SupplierCredentialStatus{
			PostgresConfigured:             true,
			RedisConfigured:                true,
			BrowserLoginEnabled:            true,
			BrowserLoginUsernameConfigured: true,
			BrowserLoginPasswordConfigured: true,
			MaskedBrowserLoginUsername:     "op***@example.com",
		},
		BalanceCents:       5000,
		BalanceCurrency:    "USD",
		BalanceUpdatedAt:   &balanceUpdatedAt,
		RechargeMultiplier: 1,
		CreatedAt:          now,
		UpdatedAt:          now,
	})

	require.NoError(t, err)
	require.Equal(t, int64(7), got.ID)
	require.Equal(t, adminplusdomain.SupplierRuntimeStatusCandidate, got.RuntimeStatus)
	require.Equal(t, "https://relay.example.com/custom/topup", got.ThirdPartyRechargeURL)
	require.Equal(t, "https://sub2apiplus.example.com/custom/topup", got.LocalRechargeURL)
	require.True(t, got.Credential.PostgresConfigured)
}

func TestSQLRepositoryGetBrowserCredential(t *testing.T) {
	db, mock := newSupplierSQLMock(t)
	repo := NewSQLRepository(db, sub2apiapp.ReadDB{DB: db})

	mock.ExpectQuery(`FROM admin_plus_suppliers\s+WHERE id = \$1`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"name",
			"kind",
			"type",
			"dashboard_url",
			"api_base_url",
			"browser_login_enabled",
			"browser_login_username_ciphertext",
			"browser_login_password_ciphertext",
			"browser_login_token_ciphertext",
		}).AddRow(
			int64(7),
			"Primary Relay",
			"relay",
			"sub2api",
			"https://relay.example.com/admin",
			"https://relay.example.com/api/v1",
			true,
			"ops@example.com",
			"secret",
			"session-token",
		))

	got, err := repo.GetBrowserCredential(context.Background(), 7)

	require.NoError(t, err)
	require.Equal(t, int64(7), got.SupplierID)
	require.Equal(t, adminplusdomain.SupplierTypeSub2API, got.Type)
	require.Equal(t, "https://relay.example.com/admin", got.DashboardURL)
	require.Equal(t, "ops@example.com", got.Username)
	require.Equal(t, "secret", got.Password)
	require.Equal(t, "session-token", got.Token)
}

func TestSQLRepositoryListFiltersWithParameterizedQuery(t *testing.T) {
	db, mock := newSupplierSQLMock(t)
	repo := NewSQLRepository(db, sub2apiapp.ReadDB{DB: db})
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
			float64(1),
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
	repo := NewSQLRepository(db, sub2apiapp.ReadDB{DB: db})
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
			float64(1),
			now,
			now,
		))

	got, err := repo.UpdateStatus(context.Background(), 9, adminplusdomain.SupplierRuntimeStatusDisabled, adminplusdomain.SupplierHealthStatusPaused)

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierRuntimeStatusDisabled, got.RuntimeStatus)
	require.Equal(t, adminplusdomain.SupplierHealthStatusPaused, got.HealthStatus)
}

func TestSQLRepositoryCreateAccountReadsLocalAccountFromSub2APIReadDB(t *testing.T) {
	adminDB, adminMock := newSupplierSQLMock(t)
	readDB, readMock := newSupplierSQLMock(t)
	repo := NewSQLRepository(adminDB, sub2apiapp.ReadDB{DB: readDB})
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	readMock.ExpectQuery(`FROM accounts a\s+LEFT JOIN account_groups ag ON ag\.account_id = a\.id\s+LEFT JOIN groups g ON g\.id = ag\.group_id AND g\.deleted_at IS NULL\s+WHERE a\.id = \$1 AND a\.deleted_at IS NULL`).
		WithArgs(int64(42)).
		WillReturnRows(newLocalSub2APIAccountRows().AddRow(
			int64(42),
			"Real Sub2API Account",
			"openai",
			"api_key",
			"active",
			true,
			8,
			10,
			1.0,
			pq.Int64Array{1001, 1002},
			pq.StringArray{"Lime", "Gemini"},
		))

	adminMock.ExpectQuery(`INSERT INTO admin_plus_supplier_accounts`).
		WithArgs(
			int64(7),
			int64(42),
			"Real Sub2API Account",
			"openai",
			"api_key",
			"upstream-key",
			"Primary",
			"org-1",
			"project-1",
			"default",
			8,
			0,
			int64(1000),
			int64(2000),
			"USD",
			true,
			"candidate",
			"normal",
			now,
			now,
		).
		WillReturnRows(newSupplierAccountRows().AddRow(
			int64(100),
			int64(7),
			int64(0),
			int64(42),
			"Real Sub2API Account",
			"openai",
			"api_key",
			"upstream-key",
			"Primary",
			"org-1",
			"project-1",
			"default",
			8,
			0,
			int64(1000),
			int64(2000),
			"USD",
			true,
			"candidate",
			"normal",
			now,
			now,
		))

	got, err := repo.CreateAccount(context.Background(), &adminplusdomain.SupplierAccount{
		SupplierID:                7,
		LocalSub2APIAccountID:     42,
		SupplierAccountIdentifier: "upstream-key",
		SupplierAccountLabel:      "Primary",
		OrganizationID:            "org-1",
		ProjectID:                 "project-1",
		RateProfile:               "default",
		ConfiguredConcurrency:     8,
		BalanceThresholdCents:     1000,
		BalanceCents:              2000,
		BalanceCurrency:           "USD",
		HasUsableBalance:          true,
		RuntimeStatus:             adminplusdomain.SupplierRuntimeStatusCandidate,
		HealthStatus:              adminplusdomain.SupplierHealthStatusNormal,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	})

	require.NoError(t, err)
	require.Equal(t, int64(100), got.ID)
	require.Equal(t, "Real Sub2API Account", got.LocalAccountName)
}

func TestSQLRepositoryListLocalAccountsReturnsGroups(t *testing.T) {
	db, mock := newSupplierSQLMock(t)
	repo := NewSQLRepository(db, sub2apiapp.ReadDB{DB: db})

	mock.ExpectQuery(`FROM accounts a\s+LEFT JOIN account_groups ag ON ag\.account_id = a\.id\s+LEFT JOIN groups g ON g\.id = ag\.group_id AND g\.deleted_at IS NULL`).
		WithArgs(100).
		WillReturnRows(newLocalSub2APIAccountRows().AddRow(
			int64(42),
			"Real Sub2API Account",
			"openai",
			"api_key",
			"active",
			true,
			8,
			10,
			1.0,
			pq.Int64Array{1001, 1002},
			pq.StringArray{"Lime", "Gemini"},
		))

	items, err := repo.ListLocalAccounts(context.Background(), "", 100)

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, []int64{1001, 1002}, items[0].GroupIDs)
	require.Equal(t, []string{"Lime", "Gemini"}, items[0].GroupNames)
}

func TestSQLRepositoryGetNotFound(t *testing.T) {
	db, mock := newSupplierSQLMock(t)
	repo := NewSQLRepository(db, sub2apiapp.ReadDB{DB: db})

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
		"third_party_recharge_url",
		"local_recharge_url",
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
		"recharge_multiplier",
		"created_at",
		"updated_at",
	})
}

func newSupplierAccountRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"supplier_id",
		"supplier_key_id",
		"local_sub2api_account_id",
		"local_account_name",
		"local_account_platform",
		"local_account_type",
		"supplier_account_identifier",
		"supplier_account_label",
		"organization_id",
		"project_id",
		"rate_profile",
		"configured_concurrency",
		"observed_max_concurrency",
		"balance_threshold_cents",
		"balance_cents",
		"balance_currency",
		"has_usable_balance",
		"runtime_status",
		"health_status",
		"created_at",
		"updated_at",
	})
}

func newLocalSub2APIAccountRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"name",
		"platform",
		"type",
		"status",
		"schedulable",
		"concurrency",
		"priority",
		"rate_multiplier",
		"group_ids",
		"group_names",
	})
}
