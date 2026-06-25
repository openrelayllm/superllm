package mailverification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type SecretCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

type SQLRepository struct {
	db     *sql.DB
	cipher SecretCipher
}

func NewSQLRepositoryWithCipher(db *sql.DB, cipher SecretCipher) *SQLRepository {
	return &SQLRepository{db: db, cipher: cipher}
}

func (r *SQLRepository) SaveCredential(ctx context.Context, credential *Credential) (*Credential, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.InternalServer("MAIL_VERIFICATION_DB_NOT_CONFIGURED", "mail verification database is not configured")
	}
	if credential == nil {
		return nil, infraerrors.BadRequest("MAIL_CREDENTIAL_INVALID", "mail credential is required")
	}
	accessToken, err := r.encryptRequired(credential.AccessToken)
	if err != nil {
		return nil, err
	}
	refreshToken, err := r.encryptOptional(credential.RefreshToken)
	if err != nil {
		return nil, err
	}
	metadata, err := marshalMetadata(credential.Metadata)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_mail_credentials (
			provider,
			name,
			email,
			email_masked,
			access_token_ciphertext,
			refresh_token_ciphertext,
			scopes,
			token_type,
			expires_at,
			metadata,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)
		ON CONFLICT (provider, email) WHERE email <> '' DO UPDATE SET
			name = EXCLUDED.name,
			email_masked = EXCLUDED.email_masked,
			access_token_ciphertext = EXCLUDED.access_token_ciphertext,
			refresh_token_ciphertext = CASE
				WHEN EXCLUDED.refresh_token_ciphertext <> '' THEN EXCLUDED.refresh_token_ciphertext
				ELSE admin_plus_mail_credentials.refresh_token_ciphertext
			END,
			scopes = EXCLUDED.scopes,
			token_type = EXCLUDED.token_type,
			expires_at = EXCLUDED.expires_at,
			metadata = EXCLUDED.metadata,
			last_error_code = '',
			updated_at = EXCLUDED.updated_at
		RETURNING id, provider, name, email, email_masked,
			access_token_ciphertext, refresh_token_ciphertext, scopes, token_type, expires_at, metadata,
			last_checked_at, last_error_code, created_at, updated_at
	`,
		string(credential.Provider),
		credential.Name,
		credential.Email,
		firstNonEmpty(credential.EmailMasked, maskEmail(credential.Email)),
		accessToken,
		refreshToken,
		scopesToString(credential.Scopes),
		credential.TokenType,
		credential.ExpiresAt,
		metadata,
		now,
	)
	return r.scanCredential(row)
}

func (r *SQLRepository) GetCredential(ctx context.Context, id int64) (*Credential, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.InternalServer("MAIL_VERIFICATION_DB_NOT_CONFIGURED", "mail verification database is not configured")
	}
	if id <= 0 {
		return nil, infraerrors.BadRequest("MAIL_CREDENTIAL_ID_INVALID", "invalid mail credential id")
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, provider, name, email, email_masked,
			access_token_ciphertext, refresh_token_ciphertext, scopes, token_type, expires_at, metadata,
			last_checked_at, last_error_code, created_at, updated_at
		FROM admin_plus_mail_credentials
		WHERE id = $1
	`, id)
	return r.scanCredential(row)
}

func (r *SQLRepository) ListCredentials(ctx context.Context, filter CredentialFilter) ([]*adminplusdomain.MailVerificationCredential, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.InternalServer("MAIL_VERIFICATION_DB_NOT_CONFIGURED", "mail verification database is not configured")
	}
	query := `
		SELECT id, provider, name, email_masked, scopes, token_type, expires_at, metadata,
			last_checked_at, last_error_code, created_at, updated_at
		FROM admin_plus_mail_credentials
		WHERE 1=1`
	args := make([]any, 0, 1)
	if filter.Provider != "" {
		args = append(args, string(filter.Provider))
		query += " AND provider = $1"
	}
	query += " ORDER BY updated_at DESC, id DESC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]*adminplusdomain.MailVerificationCredential, 0)
	for rows.Next() {
		item, err := scanPublicCredential(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SQLRepository) ListCredentialRecords(ctx context.Context, filter CredentialFilter) ([]*Credential, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.InternalServer("MAIL_VERIFICATION_DB_NOT_CONFIGURED", "mail verification database is not configured")
	}
	query := `
		SELECT id, provider, name, email, email_masked,
			access_token_ciphertext, refresh_token_ciphertext, scopes, token_type, expires_at, metadata,
			last_checked_at, last_error_code, created_at, updated_at
		FROM admin_plus_mail_credentials
		WHERE 1=1`
	args := make([]any, 0, 1)
	if filter.Provider != "" {
		args = append(args, string(filter.Provider))
		query += " AND provider = $1"
	}
	query += " ORDER BY updated_at DESC, id DESC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]*Credential, 0)
	for rows.Next() {
		item, err := r.scanCredential(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SQLRepository) UpdateTokens(ctx context.Context, id int64, update TokenUpdate) (*Credential, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.InternalServer("MAIL_VERIFICATION_DB_NOT_CONFIGURED", "mail verification database is not configured")
	}
	accessToken, err := r.encryptRequired(update.AccessToken)
	if err != nil {
		return nil, err
	}
	refreshToken, err := r.encryptOptional(update.RefreshToken)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_mail_credentials
		SET access_token_ciphertext = $2,
			refresh_token_ciphertext = CASE WHEN $3 <> '' THEN $3 ELSE refresh_token_ciphertext END,
			scopes = CASE WHEN $4 <> '' THEN $4 ELSE scopes END,
			token_type = CASE WHEN $5 <> '' THEN $5 ELSE token_type END,
			expires_at = $6,
			last_error_code = '',
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, provider, name, email, email_masked,
			access_token_ciphertext, refresh_token_ciphertext, scopes, token_type, expires_at, metadata,
			last_checked_at, last_error_code, created_at, updated_at
	`, id, accessToken, refreshToken, scopesToString(update.Scopes), update.TokenType, update.ExpiresAt)
	return r.scanCredential(row)
}

func (r *SQLRepository) RecordCredentialCheck(ctx context.Context, id int64, checkedAt time.Time, errorCode string) error {
	if r == nil || r.db == nil {
		return infraerrors.InternalServer("MAIL_VERIFICATION_DB_NOT_CONFIGURED", "mail verification database is not configured")
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE admin_plus_mail_credentials
		SET last_checked_at = $2,
			last_error_code = $3,
			updated_at = NOW()
		WHERE id = $1
	`, id, checkedAt.UTC(), trimLimit(errorCode, 80))
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return infraerrors.NotFound("MAIL_CREDENTIAL_NOT_FOUND", "mail credential not found")
	}
	return nil
}

func (r *SQLRepository) scanCredential(scanner interface{ Scan(dest ...any) error }) (*Credential, error) {
	var credential Credential
	var provider string
	var accessTokenCiphertext string
	var refreshTokenCiphertext string
	var scopes string
	var metadataRaw []byte
	err := scanner.Scan(
		&credential.ID,
		&provider,
		&credential.Name,
		&credential.Email,
		&credential.EmailMasked,
		&accessTokenCiphertext,
		&refreshTokenCiphertext,
		&scopes,
		&credential.TokenType,
		&credential.ExpiresAt,
		&metadataRaw,
		&credential.LastCheckedAt,
		&credential.LastErrorCode,
		&credential.CreatedAt,
		&credential.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, infraerrors.NotFound("MAIL_CREDENTIAL_NOT_FOUND", "mail credential not found")
	}
	if err != nil {
		return nil, err
	}
	credential.Provider = adminplusdomain.MailVerificationProvider(provider)
	credential.Scopes = normalizeScopes(nil, scopes)
	credential.Metadata = unmarshalMetadata(metadataRaw)
	if credential.AccessToken, err = r.decryptRequired(accessTokenCiphertext); err != nil {
		return nil, err
	}
	if credential.RefreshToken, err = r.decryptOptional(refreshTokenCiphertext); err != nil {
		return nil, err
	}
	return &credential, nil
}

func scanPublicCredential(scanner interface{ Scan(dest ...any) error }) (*adminplusdomain.MailVerificationCredential, error) {
	var out adminplusdomain.MailVerificationCredential
	var provider string
	var scopes string
	var metadataRaw []byte
	err := scanner.Scan(
		&out.ID,
		&provider,
		&out.Name,
		&out.EmailMasked,
		&scopes,
		&out.TokenType,
		&out.ExpiresAt,
		&metadataRaw,
		&out.LastCheckedAt,
		&out.LastErrorCode,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	out.Provider = adminplusdomain.MailVerificationProvider(provider)
	out.Scopes = normalizeScopes(nil, scopes)
	out.Metadata = unmarshalMetadata(metadataRaw)
	return &out, nil
}

func marshalMetadata(metadata map[string]string) ([]byte, error) {
	if metadata == nil {
		metadata = map[string]string{}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return nil, infraerrors.BadRequest("MAIL_CREDENTIAL_METADATA_INVALID", "mail credential metadata must be valid")
	}
	return raw, nil
}

func unmarshalMetadata(raw []byte) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	var metadata map[string]string
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil
	}
	return metadata
}

func (r *SQLRepository) encryptRequired(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", infraerrors.BadRequest("MAIL_CREDENTIAL_ACCESS_TOKEN_REQUIRED", "mail access token is required")
	}
	return r.encryptOptional(value)
}

func (r *SQLRepository) encryptOptional(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if r.cipher == nil {
		return value, nil
	}
	encrypted, err := r.cipher.Encrypt(value)
	if err != nil {
		return "", infraerrors.New(http.StatusInternalServerError, "MAIL_CREDENTIAL_ENCRYPT_FAILED", "failed to encrypt mail credential")
	}
	return encrypted, nil
}

func (r *SQLRepository) decryptRequired(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", infraerrors.InternalServer("MAIL_CREDENTIAL_ACCESS_TOKEN_MISSING", "mail access token is missing")
	}
	return r.decryptOptional(value)
}

func (r *SQLRepository) decryptOptional(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if r.cipher == nil {
		return value, nil
	}
	decrypted, err := r.cipher.Decrypt(value)
	if err != nil {
		return "", infraerrors.New(http.StatusInternalServerError, "MAIL_CREDENTIAL_DECRYPT_FAILED", "failed to decrypt mail credential")
	}
	return decrypted, nil
}
