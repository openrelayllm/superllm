package suppliers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	sub2apiapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sub2api"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/lib/pq"
)

type SQLRepository struct {
	db        *sql.DB
	sub2apiDB *sql.DB
	cipher    CredentialCipher
}

type CredentialCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

func NewSQLRepository(db *sql.DB, sub2apiReadDB sub2apiapp.ReadDB) *SQLRepository {
	return NewSQLRepositoryWithCipher(db, sub2apiReadDB, nil)
}

func NewSQLRepositoryWithCipher(db *sql.DB, sub2apiReadDB sub2apiapp.ReadDB, cipher CredentialCipher) *SQLRepository {
	readDB := sub2apiReadDB.DB
	if readDB == nil {
		readDB = db
	}
	return &SQLRepository{
		db:        db,
		sub2apiDB: readDB,
		cipher:    cipher,
	}
}

func (r *SQLRepository) Create(ctx context.Context, supplier *adminplusdomain.Supplier) (*adminplusdomain.Supplier, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
	}
	usernameCiphertext, err := r.encryptOptional(supplier.BrowserLoginUsername)
	if err != nil {
		return nil, err
	}
	passwordCiphertext, err := r.encryptOptional(supplier.BrowserLoginPassword)
	if err != nil {
		return nil, err
	}
	tokenCiphertext, err := r.encryptOptional(supplier.BrowserLoginToken)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_suppliers (
			name, kind, type, runtime_status, health_status,
			dashboard_url, api_base_url, contact, notes,
			postgres_configured, redis_configured, browser_login_enabled,
			browser_login_username_configured, browser_login_password_configured, browser_login_token_configured, masked_browser_login_username,
			browser_login_username_ciphertext, browser_login_password_ciphertext, browser_login_token_ciphertext,
			balance_cents, balance_currency, balance_updated_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
		RETURNING id, name, kind, type, runtime_status, health_status,
			dashboard_url, api_base_url, contact, notes,
			postgres_configured, redis_configured, browser_login_enabled,
			browser_login_username_configured, browser_login_password_configured, browser_login_token_configured, masked_browser_login_username,
			balance_cents, balance_currency, balance_updated_at, created_at, updated_at
	`,
		supplier.Name,
		string(supplier.Kind),
		string(supplier.Type),
		string(supplier.RuntimeStatus),
		string(supplier.HealthStatus),
		supplier.DashboardURL,
		supplier.APIBaseURL,
		supplier.Contact,
		supplier.Notes,
		supplier.Credential.PostgresConfigured,
		supplier.Credential.RedisConfigured,
		supplier.Credential.BrowserLoginEnabled,
		supplier.Credential.BrowserLoginUsernameConfigured,
		supplier.Credential.BrowserLoginPasswordConfigured,
		supplier.Credential.BrowserLoginTokenConfigured,
		supplier.Credential.MaskedBrowserLoginUsername,
		usernameCiphertext,
		passwordCiphertext,
		tokenCiphertext,
		supplier.BalanceCents,
		supplier.BalanceCurrency,
		supplier.BalanceUpdatedAt,
		supplier.CreatedAt,
		supplier.UpdatedAt,
	)
	return scanSupplier(row)
}

func (r *SQLRepository) GetBrowserCredential(ctx context.Context, id int64) (*adminplusdomain.SupplierBrowserCredential, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, kind, type, dashboard_url, api_base_url,
			browser_login_enabled,
			browser_login_username_ciphertext,
			browser_login_password_ciphertext,
			browser_login_token_ciphertext
		FROM admin_plus_suppliers
		WHERE id = $1
	`, id)
	var credential adminplusdomain.SupplierBrowserCredential
	var kind, supplierType string
	var loginEnabled bool
	var usernameCiphertext, passwordCiphertext, tokenCiphertext string
	err := row.Scan(
		&credential.SupplierID,
		&credential.SupplierName,
		&kind,
		&supplierType,
		&credential.DashboardURL,
		&credential.APIBaseURL,
		&loginEnabled,
		&usernameCiphertext,
		&passwordCiphertext,
		&tokenCiphertext,
	)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found")
	}
	if err != nil {
		return nil, err
	}
	if !loginEnabled {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_BROWSER_LOGIN_DISABLED", "supplier browser login is disabled")
	}
	credential.Kind = adminplusdomain.SupplierKind(kind)
	credential.Type = adminplusdomain.SupplierType(supplierType)
	if credential.Username, err = r.decryptOptional(usernameCiphertext); err != nil {
		return nil, err
	}
	if credential.Password, err = r.decryptOptional(passwordCiphertext); err != nil {
		return nil, err
	}
	if credential.Token, err = r.decryptOptional(tokenCiphertext); err != nil {
		return nil, err
	}
	return &credential, nil
}

func (r *SQLRepository) Get(ctx context.Context, id int64) (*adminplusdomain.Supplier, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, kind, type, runtime_status, health_status,
			dashboard_url, api_base_url, contact, notes,
			postgres_configured, redis_configured, browser_login_enabled,
			browser_login_username_configured, browser_login_password_configured, browser_login_token_configured, masked_browser_login_username,
			balance_cents, balance_currency, balance_updated_at, created_at, updated_at
		FROM admin_plus_suppliers
		WHERE id = $1
	`, id)
	return scanSupplier(row)
}

func (r *SQLRepository) List(ctx context.Context, filter SupplierFilter) ([]*adminplusdomain.Supplier, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
	}
	where := []string{"1=1"}
	args := make([]any, 0, 5)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	if filter.Kind != "" {
		where = append(where, "kind = "+addArg(string(filter.Kind)))
	}
	if filter.Type != "" {
		where = append(where, "type = "+addArg(string(filter.Type)))
	}
	if filter.RuntimeStatus != "" {
		where = append(where, "runtime_status = "+addArg(string(filter.RuntimeStatus)))
	}
	if filter.HealthStatus != "" {
		where = append(where, "health_status = "+addArg(string(filter.HealthStatus)))
	}
	if filter.Query != "" {
		where = append(where, "(LOWER(name) LIKE "+addArg("%"+strings.ToLower(filter.Query)+"%")+" OR LOWER(contact) LIKE "+addArg("%"+strings.ToLower(filter.Query)+"%")+" OR LOWER(notes) LIKE "+addArg("%"+strings.ToLower(filter.Query)+"%")+")")
	}

	query := `
		SELECT id, name, kind, type, runtime_status, health_status,
			dashboard_url, api_base_url, contact, notes,
			postgres_configured, redis_configured, browser_login_enabled,
			browser_login_username_configured, browser_login_password_configured, browser_login_token_configured, masked_browser_login_username,
			balance_cents, balance_currency, balance_updated_at, created_at, updated_at
		FROM admin_plus_suppliers
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY id ASC
	`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	items := make([]*adminplusdomain.Supplier, 0)
	for rows.Next() {
		supplier, err := scanSupplier(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, supplier)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SQLRepository) Update(ctx context.Context, supplier *adminplusdomain.Supplier) (*adminplusdomain.Supplier, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
	}
	usernameCiphertext, err := r.encryptOptional(supplier.BrowserLoginUsername)
	if err != nil {
		return nil, err
	}
	passwordCiphertext, err := r.encryptOptional(supplier.BrowserLoginPassword)
	if err != nil {
		return nil, err
	}
	tokenCiphertext, err := r.encryptOptional(supplier.BrowserLoginToken)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_suppliers
		SET name = $2,
			kind = $3,
			type = $4,
			runtime_status = $5,
			health_status = $6,
			dashboard_url = $7,
			api_base_url = $8,
			contact = $9,
			notes = $10,
			postgres_configured = $11,
			redis_configured = $12,
			browser_login_enabled = $13,
			browser_login_username_configured = $14,
			browser_login_password_configured = $15,
			browser_login_token_configured = $16,
			masked_browser_login_username = $17,
			browser_login_username_ciphertext = CASE WHEN $18 <> '' THEN $18 ELSE browser_login_username_ciphertext END,
			browser_login_password_ciphertext = CASE WHEN $19 <> '' THEN $19 ELSE browser_login_password_ciphertext END,
			browser_login_token_ciphertext = CASE WHEN $20 <> '' THEN $20 ELSE browser_login_token_ciphertext END,
			balance_cents = $21,
			balance_currency = $22,
			balance_updated_at = $23,
			updated_at = $24
		WHERE id = $1
		RETURNING id, name, kind, type, runtime_status, health_status,
			dashboard_url, api_base_url, contact, notes,
			postgres_configured, redis_configured, browser_login_enabled,
			browser_login_username_configured, browser_login_password_configured, browser_login_token_configured, masked_browser_login_username,
			balance_cents, balance_currency, balance_updated_at, created_at, updated_at
	`,
		supplier.ID,
		supplier.Name,
		string(supplier.Kind),
		string(supplier.Type),
		string(supplier.RuntimeStatus),
		string(supplier.HealthStatus),
		supplier.DashboardURL,
		supplier.APIBaseURL,
		supplier.Contact,
		supplier.Notes,
		supplier.Credential.PostgresConfigured,
		supplier.Credential.RedisConfigured,
		supplier.Credential.BrowserLoginEnabled,
		supplier.Credential.BrowserLoginUsernameConfigured,
		supplier.Credential.BrowserLoginPasswordConfigured,
		supplier.Credential.BrowserLoginTokenConfigured,
		supplier.Credential.MaskedBrowserLoginUsername,
		usernameCiphertext,
		passwordCiphertext,
		tokenCiphertext,
		supplier.BalanceCents,
		supplier.BalanceCurrency,
		supplier.BalanceUpdatedAt,
		supplier.UpdatedAt,
	)
	return scanSupplier(row)
}

func (r *SQLRepository) UpdateStatus(ctx context.Context, id int64, runtimeStatus adminplusdomain.SupplierRuntimeStatus, healthStatus adminplusdomain.SupplierHealthStatus) (*adminplusdomain.Supplier, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_suppliers
		SET runtime_status = $2, health_status = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, kind, type, runtime_status, health_status,
			dashboard_url, api_base_url, contact, notes,
			postgres_configured, redis_configured, browser_login_enabled,
			browser_login_username_configured, browser_login_password_configured, browser_login_token_configured, masked_browser_login_username,
			balance_cents, balance_currency, balance_updated_at, created_at, updated_at
	`, id, string(runtimeStatus), string(healthStatus))
	return scanSupplier(row)
}

func (r *SQLRepository) Delete(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
	}
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM admin_plus_suppliers
		WHERE id = $1
	`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found")
	}
	return nil
}

func (r *SQLRepository) ListAccounts(ctx context.Context, supplierID int64) ([]*adminplusdomain.SupplierAccount, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, supplier_id, local_sub2api_account_id,
			local_account_name, local_account_platform, local_account_type,
			supplier_account_identifier, supplier_account_label, organization_id, project_id, rate_profile,
			configured_concurrency, observed_max_concurrency,
			balance_threshold_cents, balance_cents, balance_currency, has_usable_balance,
			runtime_status, health_status, created_at, updated_at
		FROM admin_plus_supplier_accounts
		WHERE supplier_id = $1
		ORDER BY id ASC
	`, supplierID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	items := make([]*adminplusdomain.SupplierAccount, 0)
	for rows.Next() {
		item, err := scanSupplierAccount(rows)
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

func (r *SQLRepository) CreateAccount(ctx context.Context, account *adminplusdomain.SupplierAccount) (*adminplusdomain.SupplierAccount, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
	}
	localAccount, err := r.getLocalAccount(ctx, account.LocalSub2APIAccountID)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_supplier_accounts (
			supplier_id, local_sub2api_account_id,
			local_account_name, local_account_platform, local_account_type,
			supplier_account_identifier, supplier_account_label, organization_id, project_id, rate_profile,
			configured_concurrency, observed_max_concurrency,
			balance_threshold_cents, balance_cents, balance_currency, has_usable_balance,
			runtime_status, health_status, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12,
			$13, $14, $15, $16,
			$17, $18, $19, $20
		)
		RETURNING id, supplier_id, local_sub2api_account_id,
			local_account_name, local_account_platform, local_account_type,
			supplier_account_identifier, supplier_account_label, organization_id, project_id, rate_profile,
			configured_concurrency, observed_max_concurrency,
			balance_threshold_cents, balance_cents, balance_currency, has_usable_balance,
			runtime_status, health_status, created_at, updated_at
	`,
		account.SupplierID,
		account.LocalSub2APIAccountID,
		localAccount.Name,
		localAccount.Platform,
		localAccount.Type,
		account.SupplierAccountIdentifier,
		account.SupplierAccountLabel,
		account.OrganizationID,
		account.ProjectID,
		account.RateProfile,
		account.ConfiguredConcurrency,
		account.ObservedMaxConcurrency,
		account.BalanceThresholdCents,
		account.BalanceCents,
		account.BalanceCurrency,
		account.HasUsableBalance,
		string(account.RuntimeStatus),
		string(account.HealthStatus),
		account.CreatedAt,
		account.UpdatedAt,
	)
	created, err := scanSupplierAccount(row)
	if err != nil {
		return nil, translateSupplierAccountCreateError(err)
	}
	return created, nil
}

func (r *SQLRepository) UpdateAccount(ctx context.Context, account *adminplusdomain.SupplierAccount) (*adminplusdomain.SupplierAccount, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_supplier_accounts
		SET supplier_account_identifier = $3,
			supplier_account_label = $4,
			organization_id = $5,
			project_id = $6,
			rate_profile = $7,
			configured_concurrency = $8,
			observed_max_concurrency = $9,
			balance_threshold_cents = $10,
			balance_cents = $11,
			balance_currency = $12,
			has_usable_balance = $13,
			runtime_status = $14,
			health_status = $15,
			updated_at = $16
		WHERE supplier_id = $1 AND id = $2
		RETURNING id, supplier_id, local_sub2api_account_id,
			local_account_name, local_account_platform, local_account_type,
			supplier_account_identifier, supplier_account_label, organization_id, project_id, rate_profile,
			configured_concurrency, observed_max_concurrency,
			balance_threshold_cents, balance_cents, balance_currency, has_usable_balance,
			runtime_status, health_status, created_at, updated_at
	`,
		account.SupplierID,
		account.ID,
		account.SupplierAccountIdentifier,
		account.SupplierAccountLabel,
		account.OrganizationID,
		account.ProjectID,
		account.RateProfile,
		account.ConfiguredConcurrency,
		account.ObservedMaxConcurrency,
		account.BalanceThresholdCents,
		account.BalanceCents,
		account.BalanceCurrency,
		account.HasUsableBalance,
		string(account.RuntimeStatus),
		string(account.HealthStatus),
		account.UpdatedAt,
	)
	updated, err := scanSupplierAccount(row)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_ACCOUNT_NOT_FOUND", "supplier account binding not found")
	}
	return updated, err
}

func (r *SQLRepository) DeleteAccount(ctx context.Context, supplierID int64, accountID int64) error {
	if r == nil || r.db == nil {
		return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
	}
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM admin_plus_supplier_accounts
		WHERE supplier_id = $1 AND id = $2
	`, supplierID, accountID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return infraerrors.New(http.StatusNotFound, "SUPPLIER_ACCOUNT_NOT_FOUND", "supplier account binding not found")
	}
	return nil
}

func (r *SQLRepository) ListLocalAccounts(ctx context.Context, query string, limit int) ([]*adminplusdomain.LocalSub2APIAccount, error) {
	if r == nil || r.sub2apiDB == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	where := []string{"deleted_at IS NULL"}
	args := make([]any, 0, 2)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if strings.TrimSpace(query) != "" {
		like := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
		where = append(where, "(LOWER(name) LIKE "+addArg(like)+" OR LOWER(platform) LIKE "+addArg(like)+" OR LOWER(type) LIKE "+addArg(like)+")")
	}
	limitRef := addArg(limit)
	rows, err := r.sub2apiDB.QueryContext(ctx, `
		SELECT id, name, platform, type, status, schedulable, concurrency, priority, rate_multiplier
		FROM accounts
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id ASC
		LIMIT `+limitRef+`
	`, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	items := make([]*adminplusdomain.LocalSub2APIAccount, 0)
	for rows.Next() {
		var item adminplusdomain.LocalSub2APIAccount
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Platform,
			&item.Type,
			&item.Status,
			&item.Schedulable,
			&item.Concurrency,
			&item.Priority,
			&item.RateMultiplier,
		); err != nil {
			return nil, err
		}
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SQLRepository) UpsertGroups(ctx context.Context, groups []*adminplusdomain.SupplierGroup) ([]*adminplusdomain.SupplierGroup, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
	}
	items := make([]*adminplusdomain.SupplierGroup, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		rawPayload := group.RawPayload
		if rawPayload == nil {
			rawPayload = map[string]any{}
		}
		raw, err := json.Marshal(rawPayload)
		if err != nil {
			return nil, err
		}
		row := r.db.QueryRowContext(ctx, `
			INSERT INTO admin_plus_supplier_groups (
				supplier_id, external_group_id, name, description, rate_multiplier,
				is_private, provider_family, status, raw_payload, last_seen_at, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11, $12)
			ON CONFLICT (supplier_id, external_group_id)
			DO UPDATE SET
				name = EXCLUDED.name,
				description = EXCLUDED.description,
				rate_multiplier = EXCLUDED.rate_multiplier,
				is_private = EXCLUDED.is_private,
				provider_family = EXCLUDED.provider_family,
				status = EXCLUDED.status,
				raw_payload = EXCLUDED.raw_payload,
				last_seen_at = EXCLUDED.last_seen_at,
				updated_at = EXCLUDED.updated_at
			RETURNING id, supplier_id, external_group_id, name, description, rate_multiplier,
				is_private, provider_family, status, raw_payload, last_seen_at, created_at, updated_at
		`,
			group.SupplierID,
			group.ExternalGroupID,
			group.Name,
			group.Description,
			group.RateMultiplier,
			group.IsPrivate,
			group.ProviderFamily,
			string(group.Status),
			string(raw),
			group.LastSeenAt,
			group.CreatedAt,
			group.UpdatedAt,
		)
		item, err := scanSupplierGroup(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *SQLRepository) ListGroups(ctx context.Context, supplierID int64, status adminplusdomain.SupplierGroupStatus) ([]*adminplusdomain.SupplierGroup, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
	}
	where := []string{"supplier_id = $1"}
	args := []any{supplierID}
	if status != "" {
		args = append(args, string(status))
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, supplier_id, external_group_id, name, description, rate_multiplier,
			is_private, provider_family, status, raw_payload, last_seen_at, created_at, updated_at
		FROM admin_plus_supplier_groups
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY status ASC, id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	items := make([]*adminplusdomain.SupplierGroup, 0)
	for rows.Next() {
		item, err := scanSupplierGroup(rows)
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

func (r *SQLRepository) MarkMissingGroups(ctx context.Context, supplierID int64, seenExternalGroupIDs []string, missingAt time.Time) (int, error) {
	if r == nil || r.db == nil {
		return 0, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE admin_plus_supplier_groups
		SET status = 'missing', updated_at = $3
		WHERE supplier_id = $1
			AND status <> 'missing'
			AND NOT (external_group_id = ANY($2))
	`, supplierID, pq.Array(seenExternalGroupIDs), missingAt)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

func (r *SQLRepository) getLocalAccount(ctx context.Context, id int64) (*adminplusdomain.LocalSub2APIAccount, error) {
	if r == nil || r.sub2apiDB == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	row := r.sub2apiDB.QueryRowContext(ctx, `
		SELECT id, name, platform, type, status, schedulable, concurrency, priority, rate_multiplier
		FROM accounts
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	var item adminplusdomain.LocalSub2APIAccount
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.Platform,
		&item.Type,
		&item.Status,
		&item.Schedulable,
		&item.Concurrency,
		&item.Priority,
		&item.RateMultiplier,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, infraerrors.New(http.StatusNotFound, "LOCAL_ACCOUNT_NOT_FOUND", "local Sub2API account not found")
		}
		return nil, err
	}
	return &item, nil
}

type supplierScanner interface {
	Scan(dest ...any) error
}

func scanSupplier(scanner supplierScanner) (*adminplusdomain.Supplier, error) {
	var supplier adminplusdomain.Supplier
	var kind, supplierType, runtimeStatus, healthStatus string
	var balanceUpdatedAt sql.NullTime
	err := scanner.Scan(
		&supplier.ID,
		&supplier.Name,
		&kind,
		&supplierType,
		&runtimeStatus,
		&healthStatus,
		&supplier.DashboardURL,
		&supplier.APIBaseURL,
		&supplier.Contact,
		&supplier.Notes,
		&supplier.Credential.PostgresConfigured,
		&supplier.Credential.RedisConfigured,
		&supplier.Credential.BrowserLoginEnabled,
		&supplier.Credential.BrowserLoginUsernameConfigured,
		&supplier.Credential.BrowserLoginPasswordConfigured,
		&supplier.Credential.BrowserLoginTokenConfigured,
		&supplier.Credential.MaskedBrowserLoginUsername,
		&supplier.BalanceCents,
		&supplier.BalanceCurrency,
		&balanceUpdatedAt,
		&supplier.CreatedAt,
		&supplier.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found")
	}
	if err != nil {
		return nil, err
	}
	supplier.Kind = adminplusdomain.SupplierKind(kind)
	supplier.Type = adminplusdomain.SupplierType(supplierType)
	supplier.RuntimeStatus = adminplusdomain.SupplierRuntimeStatus(runtimeStatus)
	supplier.HealthStatus = adminplusdomain.SupplierHealthStatus(healthStatus)
	if balanceUpdatedAt.Valid {
		t := balanceUpdatedAt.Time
		supplier.BalanceUpdatedAt = &t
	}
	return &supplier, nil
}

func scanSupplierAccount(scanner supplierScanner) (*adminplusdomain.SupplierAccount, error) {
	var account adminplusdomain.SupplierAccount
	var runtimeStatus, healthStatus string
	err := scanner.Scan(
		&account.ID,
		&account.SupplierID,
		&account.LocalSub2APIAccountID,
		&account.LocalAccountName,
		&account.LocalAccountPlatform,
		&account.LocalAccountType,
		&account.SupplierAccountIdentifier,
		&account.SupplierAccountLabel,
		&account.OrganizationID,
		&account.ProjectID,
		&account.RateProfile,
		&account.ConfiguredConcurrency,
		&account.ObservedMaxConcurrency,
		&account.BalanceThresholdCents,
		&account.BalanceCents,
		&account.BalanceCurrency,
		&account.HasUsableBalance,
		&runtimeStatus,
		&healthStatus,
		&account.CreatedAt,
		&account.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	account.RuntimeStatus = adminplusdomain.SupplierRuntimeStatus(runtimeStatus)
	account.HealthStatus = adminplusdomain.SupplierHealthStatus(healthStatus)
	return &account, nil
}

func scanSupplierGroup(scanner supplierScanner) (*adminplusdomain.SupplierGroup, error) {
	var group adminplusdomain.SupplierGroup
	var status string
	var raw []byte
	err := scanner.Scan(
		&group.ID,
		&group.SupplierID,
		&group.ExternalGroupID,
		&group.Name,
		&group.Description,
		&group.RateMultiplier,
		&group.IsPrivate,
		&group.ProviderFamily,
		&status,
		&raw,
		&group.LastSeenAt,
		&group.CreatedAt,
		&group.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	group.Status = adminplusdomain.SupplierGroupStatus(status)
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &group.RawPayload); err != nil {
			return nil, err
		}
	}
	if group.RawPayload == nil {
		group.RawPayload = map[string]any{}
	}
	return &group, nil
}

func translateSupplierAccountCreateError(err error) error {
	message := err.Error()
	if strings.Contains(message, "admin_plus_supplier_accounts_unique_local_account") ||
		strings.Contains(message, "admin_plus_supplier_accounts_supplier_id_local_sub2api_account_id_key") {
		return infraerrors.New(http.StatusConflict, "SUPPLIER_ACCOUNT_ALREADY_BOUND", "local Sub2API account is already bound to this supplier")
	}
	return err
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
		return "", infraerrors.New(http.StatusInternalServerError, "SUPPLIER_BROWSER_CREDENTIAL_ENCRYPT_FAILED", "failed to encrypt supplier browser credential")
	}
	return encrypted, nil
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
		return "", infraerrors.New(http.StatusInternalServerError, "SUPPLIER_BROWSER_CREDENTIAL_DECRYPT_FAILED", "failed to decrypt supplier browser credential")
	}
	return decrypted, nil
}
