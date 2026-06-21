package supplierkeys

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func newSupplierKeySQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return db, mock
}

func TestSQLRepositoryFindActiveByGroupReturnsNilWhenMissing(t *testing.T) {
	db, mock := newSupplierKeySQLMock(t)
	repo := NewSQLRepository(db)

	mock.ExpectQuery(`FROM admin_plus_supplier_keys\s+WHERE supplier_id = \$1\s+AND supplier_group_id = \$2`).
		WithArgs(int64(24), int64(6)).
		WillReturnRows(newSupplierKeyRows())

	got, err := repo.FindActiveByGroup(context.Background(), 24, 6)

	require.NoError(t, err)
	require.Nil(t, got)
}

func TestSQLRepositoryFindActiveByGroupReturnsLatestBlockingKey(t *testing.T) {
	db, mock := newSupplierKeySQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM admin_plus_supplier_keys\s+WHERE supplier_id = \$1\s+AND supplier_group_id = \$2`).
		WithArgs(int64(24), int64(6)).
		WillReturnRows(newSupplierKeyRows().AddRow(
			int64(9),
			int64(24),
			int64(6),
			"1215",
			"99",
			"Lime",
			"fingerprint",
			"abcd",
			string(adminplusdomain.SupplierKeyStatusBound),
			"openai",
			int64(42),
			"AI Pixel / PLUS共享号池 / Lime",
			"openai",
			"api_key",
			[]byte(`{"name":"Lime"}`),
			[]byte(`{"id":99}`),
			"",
			"",
			now,
			now,
		))

	got, err := repo.FindActiveByGroup(context.Background(), 24, 6)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, int64(9), got.ID)
	require.Equal(t, adminplusdomain.SupplierKeyStatusBound, got.Status)
	require.Equal(t, "Lime", got.Name)
	require.Equal(t, int64(42), got.LocalSub2APIAccountID)
}

func newSupplierKeyRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"supplier_id",
		"supplier_group_id",
		"external_group_id",
		"external_key_id",
		"name",
		"key_fingerprint",
		"key_last4",
		"status",
		"provider_family",
		"local_sub2api_account_id",
		"local_account_name",
		"local_account_platform",
		"local_account_type",
		"provision_request",
		"provision_response",
		"error_code",
		"error_message",
		"created_at",
		"updated_at",
	})
}
