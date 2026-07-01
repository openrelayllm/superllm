package sitecatalog

import (
	"context"
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func newSiteCatalogSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return db, mock
}

func TestSQLRepositoryDeleteSiteClearsDiscoveryLinksAndDeletesSite(t *testing.T) {
	db, mock := newSiteCatalogSQLMock(t)
	repo := NewSQLRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id\s+FROM admin_plus_site_catalog_sites\s+WHERE id = \$1\s+FOR UPDATE`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectExec(`UPDATE admin_plus_site_discoveries\s+SET catalog_site_id = NULL,\s+process_status = CASE\s+WHEN process_status = 'added_to_catalog' THEN 'unprocessed'\s+ELSE process_status\s+END,\s+updated_at = NOW\(\)\s+WHERE catalog_site_id = \$1`).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`DELETE FROM admin_plus_site_catalog_sites\s+WHERE id = \$1`).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, repo.DeleteSite(context.Background(), 7))
}

func TestSQLRepositoryDeleteSiteReturnsNotFound(t *testing.T) {
	db, mock := newSiteCatalogSQLMock(t)
	repo := NewSQLRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id\s+FROM admin_plus_site_catalog_sites\s+WHERE id = \$1\s+FOR UPDATE`).
		WithArgs(int64(404)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	err := repo.DeleteSite(context.Background(), 404)
	require.Error(t, err)
	require.Contains(t, err.Error(), "SITE_CATALOG_SITE_NOT_FOUND")
}
