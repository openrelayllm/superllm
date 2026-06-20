package sessions

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) Upsert(ctx context.Context, session *adminplusdomain.SupplierBrowserSession) (*adminplusdomain.SupplierBrowserSession, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	summary, err := marshalMap(session.SessionSummary)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_supplier_browser_sessions (
			supplier_id, origin, api_base_url, session_summary,
			session_bundle_ciphertext, captured_at, expires_at,
			source_extension_task_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, 0), $9, $10)
		ON CONFLICT (supplier_id) DO UPDATE
		SET origin = EXCLUDED.origin,
			api_base_url = EXCLUDED.api_base_url,
			session_summary = EXCLUDED.session_summary,
			session_bundle_ciphertext = EXCLUDED.session_bundle_ciphertext,
			captured_at = EXCLUDED.captured_at,
			expires_at = EXCLUDED.expires_at,
			source_extension_task_id = EXCLUDED.source_extension_task_id,
			updated_at = EXCLUDED.updated_at
		RETURNING supplier_id, origin, api_base_url, session_summary,
			session_bundle_ciphertext, captured_at, expires_at,
			source_extension_task_id, created_at, updated_at
	`,
		session.SupplierID,
		session.Origin,
		session.APIBaseURL,
		summary,
		session.SessionBundleCiphertext,
		session.CapturedAt,
		nullableTime(session.ExpiresAt),
		session.SourceExtensionTaskID,
		session.CreatedAt,
		session.UpdatedAt,
	)
	return scanSupplierBrowserSession(row)
}

func (r *SQLRepository) Get(ctx context.Context, supplierID int64) (*adminplusdomain.SupplierBrowserSession, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT supplier_id, origin, api_base_url, session_summary,
			session_bundle_ciphertext, captured_at, expires_at,
			source_extension_task_id, created_at, updated_at
		FROM admin_plus_supplier_browser_sessions
		WHERE supplier_id = $1
	`, supplierID)
	session, err := scanSupplierBrowserSession(row)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_SESSION_NOT_FOUND", "supplier browser session not found")
	}
	return session, err
}

type supplierBrowserSessionScanner interface {
	Scan(dest ...any) error
}

func scanSupplierBrowserSession(scanner supplierBrowserSessionScanner) (*adminplusdomain.SupplierBrowserSession, error) {
	var session adminplusdomain.SupplierBrowserSession
	var summary []byte
	var expiresAt sql.NullTime
	var sourceTaskID sql.NullInt64
	err := scanner.Scan(
		&session.SupplierID,
		&session.Origin,
		&session.APIBaseURL,
		&summary,
		&session.SessionBundleCiphertext,
		&session.CapturedAt,
		&expiresAt,
		&sourceTaskID,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(summary) > 0 {
		if err := json.Unmarshal(summary, &session.SessionSummary); err != nil {
			return nil, err
		}
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		session.ExpiresAt = &t
	}
	if sourceTaskID.Valid {
		session.SourceExtensionTaskID = sourceTaskID.Int64
	}
	return &session, nil
}

func marshalMap(value map[string]any) ([]byte, error) {
	if len(value) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(value)
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func dbNotConfigured() error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
}
