package sub2api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var errIdentityDatabaseNotConfigured = errors.New("Sub2API readonly database is required for authentication")

// IdentityRepository reads the authoritative users from the existing Sub2API database.
type IdentityRepository struct {
	db         *sql.DB
	configured bool
}

func NewIdentityRepository(readDB ReadDB) *IdentityRepository {
	return &IdentityRepository{db: readDB.DB, configured: readDB.Configured}
}

func (r *IdentityRepository) GetByID(ctx context.Context, id int64) (*service.User, error) {
	return r.get(ctx, "id = $1", id)
}

func (r *IdentityRepository) GetByEmail(ctx context.Context, email string) (*service.User, error) {
	return r.get(ctx, "LOWER(BTRIM(email)) = LOWER(BTRIM($1))", strings.TrimSpace(email))
}

func (r *IdentityRepository) GetFirstAdmin(ctx context.Context) (*service.User, error) {
	if r == nil || !r.configured || r.db == nil {
		return nil, fmt.Errorf("identity source unavailable: %w", errIdentityDatabaseNotConfigured)
	}
	return scanIdentityUser(r.db.QueryRowContext(ctx, `SELECT id, email, password_hash, role, status, username, balance, concurrency,
                        totp_secret_encrypted, totp_enabled, totp_enabled_at,
                        created_at, updated_at, deleted_at
                   FROM users
                  WHERE role = 'admin' AND status = 'active' AND deleted_at IS NULL
                  ORDER BY id
                  LIMIT 1`))
}

func (r *IdentityRepository) get(ctx context.Context, predicate string, arg any) (*service.User, error) {
	if r == nil || !r.configured || r.db == nil {
		return nil, fmt.Errorf("identity source unavailable: %w", errIdentityDatabaseNotConfigured)
	}

	query := `SELECT id, email, password_hash, role, status, username, balance, concurrency,
                        totp_secret_encrypted, totp_enabled, totp_enabled_at,
                        created_at, updated_at, deleted_at
                   FROM users
                  WHERE ` + predicate + ` AND deleted_at IS NULL
                  LIMIT 1`
	return scanIdentityUser(r.db.QueryRowContext(ctx, query, arg))
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanIdentityUser(row rowScanner) (*service.User, error) {
	user := &service.User{}
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.Status,
		&user.Username,
		&user.Balance,
		&user.Concurrency,
		&user.TotpSecretEncrypted,
		&user.TotpEnabled,
		&user.TotpEnabledAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read Sub2API user: %w", err)
	}
	return user, nil
}
