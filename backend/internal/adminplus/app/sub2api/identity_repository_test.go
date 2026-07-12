package sub2api

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestIdentityRepositoryRequiresConfiguredReadonlyDatabase(t *testing.T) {
	repo := NewIdentityRepository(ReadDB{})

	_, err := repo.GetByEmail(context.Background(), "admin@example.com")

	require.ErrorIs(t, err, errIdentityDatabaseNotConfigured)
}

func TestIdentityRepositoryReadsSub2APIUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	now := time.Now().UTC()
	query := regexp.QuoteMeta(`SELECT id, email, password_hash, role, status, username, balance, concurrency,
                        totp_secret_encrypted, totp_enabled, totp_enabled_at,
                        created_at, updated_at, deleted_at
                   FROM users
                  WHERE LOWER(BTRIM(email)) = LOWER(BTRIM($1)) AND deleted_at IS NULL
                  LIMIT 1`)
	mock.ExpectQuery(query).
		WithArgs("ADMIN@example.com").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "password_hash", "role", "status", "username", "balance", "concurrency",
			"totp_secret_encrypted", "totp_enabled", "totp_enabled_at", "created_at", "updated_at", "deleted_at",
		}).AddRow(7, "admin@example.com", "$2a$hash", "admin", "active", "Admin", 0, 5, nil, false, nil, now, now, nil))
	mock.ExpectClose()
	repo := NewIdentityRepository(ReadDB{DB: db, Configured: true})

	user, err := repo.GetByEmail(context.Background(), " ADMIN@example.com ")

	require.NoError(t, err)
	require.Equal(t, int64(7), user.ID)
	require.Equal(t, "admin", user.Role)
	require.NoError(t, db.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}
