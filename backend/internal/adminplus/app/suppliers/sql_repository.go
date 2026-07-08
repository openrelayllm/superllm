package suppliers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

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

func supplierActiveKeyCountSQL(supplierIDExpr string) string {
	return fmt.Sprintf(`COALESCE((
		SELECT COUNT(*)::INT
		FROM admin_plus_supplier_keys sk
		WHERE sk.supplier_id = %s
			AND sk.status IN ('provisioning', 'bound', 'manual_secret_required')
	), 0)`, supplierIDExpr)
}

func supplierKeyCapacityStatusSQL(policyExpr string, valueExpr string, activeCountExpr string) string {
	return fmt.Sprintf(`CASE
		WHEN %s = 'unlimited' THEN 'available'
		WHEN %s = 'unsupported' THEN 'unsupported'
		WHEN %s = 'limited' AND %s > 0 AND %s >= %s THEN 'exhausted'
		WHEN %s = 'limited' AND %s > 0 AND %s >= (%s - 1) THEN 'limited'
		WHEN %s = 'limited' AND %s > 0 THEN 'available'
		ELSE 'unknown'
	END`, policyExpr, policyExpr, policyExpr, valueExpr, activeCountExpr, valueExpr, policyExpr, valueExpr, activeCountExpr, valueExpr, policyExpr, valueExpr)
}

func supplierSelectColumns(supplierIDExpr string) string {
	activeCount := supplierActiveKeyCountSQL(supplierIDExpr)
	return `id, name, kind, type, runtime_status, health_status,
		dashboard_url, api_base_url, third_party_recharge_url, local_recharge_url, contact, notes,
		postgres_configured, redis_configured, browser_login_enabled,
		browser_login_username_configured, browser_login_password_configured, browser_login_token_configured, masked_browser_login_username,
		balance_cents, balance_currency, balance_updated_at, recharge_multiplier,
		key_limit_policy, key_limit_value,
		` + activeCount + `,
		` + supplierKeyCapacityStatusSQL("key_limit_policy", "key_limit_value", activeCount) + `,
		created_at, updated_at`
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
			dashboard_url, api_base_url, third_party_recharge_url, local_recharge_url, contact, notes,
			postgres_configured, redis_configured, browser_login_enabled,
			browser_login_username_configured, browser_login_password_configured, browser_login_token_configured, masked_browser_login_username,
			browser_login_username_ciphertext, browser_login_password_ciphertext, browser_login_token_ciphertext,
			balance_cents, balance_currency, balance_updated_at, recharge_multiplier,
			key_limit_policy, key_limit_value, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29)
		RETURNING `+supplierSelectColumns("admin_plus_suppliers.id")+`
	`,
		supplier.Name,
		string(supplier.Kind),
		string(supplier.Type),
		string(supplier.RuntimeStatus),
		string(supplier.HealthStatus),
		supplier.DashboardURL,
		supplier.APIBaseURL,
		supplier.ThirdPartyRechargeURL,
		supplier.LocalRechargeURL,
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
		supplier.RechargeMultiplier,
		supplier.KeyLimitPolicy,
		supplier.KeyLimitValue,
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
		SELECT `+supplierSelectColumns("admin_plus_suppliers.id")+`
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
		SELECT ` + supplierSelectColumns("admin_plus_suppliers.id") + `
		FROM admin_plus_suppliers
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY created_at DESC, id DESC
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
			third_party_recharge_url = $9,
			local_recharge_url = $10,
			contact = $11,
			notes = $12,
			postgres_configured = $13,
			redis_configured = $14,
			browser_login_enabled = $15,
			browser_login_username_configured = $16,
			browser_login_password_configured = $17,
			browser_login_token_configured = $18,
			masked_browser_login_username = $19,
			browser_login_username_ciphertext = CASE WHEN $20 <> '' THEN $20 ELSE browser_login_username_ciphertext END,
			browser_login_password_ciphertext = CASE WHEN $21 <> '' THEN $21 ELSE browser_login_password_ciphertext END,
			browser_login_token_ciphertext = CASE WHEN $22 <> '' THEN $22 ELSE browser_login_token_ciphertext END,
			balance_cents = $23,
			balance_currency = $24,
			balance_updated_at = $25,
			recharge_multiplier = $26,
			key_limit_policy = $27,
			key_limit_value = $28,
			updated_at = $29
		WHERE id = $1
		RETURNING `+supplierSelectColumns("admin_plus_suppliers.id")+`
	`,
		supplier.ID,
		supplier.Name,
		string(supplier.Kind),
		string(supplier.Type),
		string(supplier.RuntimeStatus),
		string(supplier.HealthStatus),
		supplier.DashboardURL,
		supplier.APIBaseURL,
		supplier.ThirdPartyRechargeURL,
		supplier.LocalRechargeURL,
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
		supplier.RechargeMultiplier,
		supplier.KeyLimitPolicy,
		supplier.KeyLimitValue,
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
		RETURNING `+supplierSelectColumns("admin_plus_suppliers.id")+`
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
		SELECT asa.id, asa.supplier_id, asa.supplier_key_id, asa.local_sub2api_account_id,
			asa.local_account_name, asa.local_account_platform, asa.local_account_type,
			asa.supplier_account_identifier, asa.supplier_account_label, asa.organization_id, asa.project_id, asa.rate_profile,
			asa.configured_concurrency, asa.observed_max_concurrency,
			asa.balance_threshold_cents, asa.balance_cents, asa.balance_currency, asa.has_usable_balance,
			asa.runtime_status, asa.health_status, asa.created_at, asa.updated_at,
			COALESCE(sg.id, 0), COALESCE(sg.external_group_id, ''), COALESCE(sg.name, ''),
			COALESCE(sg.provider_family, ''), COALESCE(sg.effective_rate_multiplier, 0),
			COALESCE(sk.name, ''), COALESCE(sk.external_key_id, ''), COALESCE(sk.key_last4, '')
		FROM admin_plus_supplier_accounts asa
		LEFT JOIN admin_plus_supplier_keys sk ON sk.id = asa.supplier_key_id
		LEFT JOIN admin_plus_supplier_groups sg ON sg.id = sk.supplier_group_id
		WHERE asa.supplier_id = $1
		ORDER BY asa.created_at DESC, asa.id DESC
	`, supplierID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	items := make([]*adminplusdomain.SupplierAccount, 0)
	for rows.Next() {
		item, err := scanSupplierAccountWithGroup(rows)
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
		RETURNING id, supplier_id, supplier_key_id, local_sub2api_account_id,
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
		RETURNING id, supplier_id, supplier_key_id, local_sub2api_account_id,
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
	where := []string{"a.deleted_at IS NULL"}
	args := make([]any, 0, 2)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if strings.TrimSpace(query) != "" {
		like := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
		groupLike := addArg(like)
		where = append(where, "(LOWER(a.name) LIKE "+addArg(like)+" OR LOWER(a.platform) LIKE "+addArg(like)+" OR LOWER(a.type) LIKE "+addArg(like)+" OR EXISTS (SELECT 1 FROM account_groups agq INNER JOIN groups gq ON gq.id = agq.group_id AND gq.deleted_at IS NULL WHERE agq.account_id = a.id AND LOWER(gq.name) LIKE "+groupLike+"))")
	}
	limitRef := addArg(limit)
	rows, err := r.sub2apiDB.QueryContext(ctx, `
			SELECT
				a.id,
				a.name,
				COALESCE(a.notes, ''),
				a.platform,
				a.type,
				a.status,
				COALESCE(a.error_message, ''),
				a.schedulable,
				a.concurrency,
				a.load_factor,
				a.priority,
				COALESCE(a.rate_multiplier, 1),
				a.last_used_at,
				a.expires_at,
				COALESCE(a.auto_pause_on_expired, TRUE),
				a.rate_limited_at,
				a.rate_limit_reset_at,
				a.overload_until,
				a.temp_unschedulable_until,
				COALESCE(a.temp_unschedulable_reason, ''),
				a.session_window_start,
				a.session_window_end,
				COALESCE(a.session_window_status, ''),
				a.created_at,
				a.updated_at,
				COALESCE(array_agg(g.id ORDER BY ag.priority, g.id) FILTER (WHERE g.id IS NOT NULL), ARRAY[]::BIGINT[]),
				COALESCE(array_agg(g.name ORDER BY ag.priority, ag.group_id) FILTER (WHERE g.name IS NOT NULL), ARRAY[]::TEXT[])
			FROM accounts a
			LEFT JOIN account_groups ag ON ag.account_id = a.id
			LEFT JOIN groups g ON g.id = ag.group_id AND g.deleted_at IS NULL
			WHERE `+strings.Join(where, " AND ")+`
			GROUP BY a.id, a.name, a.notes, a.platform, a.type, a.status, a.error_message,
				a.schedulable, a.concurrency, a.load_factor, a.priority, a.rate_multiplier,
				a.last_used_at, a.expires_at, a.auto_pause_on_expired, a.rate_limited_at,
				a.rate_limit_reset_at, a.overload_until, a.temp_unschedulable_until,
				a.temp_unschedulable_reason, a.session_window_start, a.session_window_end,
				a.session_window_status, a.created_at, a.updated_at
			ORDER BY a.id DESC
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
		item, err := scanLocalSub2APIAccount(rows)
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

func (r *SQLRepository) getLocalAccount(ctx context.Context, id int64) (*adminplusdomain.LocalSub2APIAccount, error) {
	if r == nil || r.sub2apiDB == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	row := r.sub2apiDB.QueryRowContext(ctx, `
		SELECT
			a.id,
			a.name,
			COALESCE(a.notes, ''),
			a.platform,
			a.type,
			a.status,
			COALESCE(a.error_message, ''),
			a.schedulable,
			a.concurrency,
			a.load_factor,
			a.priority,
			COALESCE(a.rate_multiplier, 1),
			a.last_used_at,
			a.expires_at,
			COALESCE(a.auto_pause_on_expired, TRUE),
			a.rate_limited_at,
			a.rate_limit_reset_at,
			a.overload_until,
			a.temp_unschedulable_until,
			COALESCE(a.temp_unschedulable_reason, ''),
			a.session_window_start,
			a.session_window_end,
			COALESCE(a.session_window_status, ''),
			a.created_at,
			a.updated_at,
			COALESCE(array_agg(g.id ORDER BY ag.priority, g.id) FILTER (WHERE g.id IS NOT NULL), ARRAY[]::BIGINT[]),
			COALESCE(array_agg(g.name ORDER BY ag.priority, ag.group_id) FILTER (WHERE g.name IS NOT NULL), ARRAY[]::TEXT[])
		FROM accounts a
		LEFT JOIN account_groups ag ON ag.account_id = a.id
		LEFT JOIN groups g ON g.id = ag.group_id AND g.deleted_at IS NULL
		WHERE a.id = $1 AND a.deleted_at IS NULL
		GROUP BY a.id, a.name, a.notes, a.platform, a.type, a.status, a.error_message,
			a.schedulable, a.concurrency, a.load_factor, a.priority, a.rate_multiplier,
			a.last_used_at, a.expires_at, a.auto_pause_on_expired, a.rate_limited_at,
			a.rate_limit_reset_at, a.overload_until, a.temp_unschedulable_until,
			a.temp_unschedulable_reason, a.session_window_start, a.session_window_end,
			a.session_window_status, a.created_at, a.updated_at
	`, id)
	item, err := scanLocalSub2APIAccount(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, infraerrors.New(http.StatusNotFound, "LOCAL_ACCOUNT_NOT_FOUND", "local Sub2API account not found")
		}
		return nil, err
	}
	return item, nil
}

func scanLocalSub2APIAccount(scanner supplierScanner) (*adminplusdomain.LocalSub2APIAccount, error) {
	var item adminplusdomain.LocalSub2APIAccount
	var groupIDs pq.Int64Array
	var groupNames pq.StringArray
	var loadFactor sql.NullInt64
	if err := scanner.Scan(
		&item.ID,
		&item.Name,
		&item.Notes,
		&item.Platform,
		&item.Type,
		&item.Status,
		&item.ErrorMessage,
		&item.Schedulable,
		&item.Concurrency,
		&loadFactor,
		&item.Priority,
		&item.RateMultiplier,
		&item.LastUsedAt,
		&item.ExpiresAt,
		&item.AutoPauseOnExpired,
		&item.RateLimitedAt,
		&item.RateLimitResetAt,
		&item.OverloadUntil,
		&item.TempUnschedulableUntil,
		&item.TempUnschedulableReason,
		&item.SessionWindowStart,
		&item.SessionWindowEnd,
		&item.SessionWindowStatus,
		&item.CreatedAt,
		&item.UpdatedAt,
		&groupIDs,
		&groupNames,
	); err != nil {
		return nil, err
	}
	if loadFactor.Valid {
		value := int(loadFactor.Int64)
		item.LoadFactor = &value
	}
	item.GroupIDs = append([]int64(nil), groupIDs...)
	item.GroupNames = append([]string(nil), groupNames...)
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
		&supplier.ThirdPartyRechargeURL,
		&supplier.LocalRechargeURL,
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
		&supplier.RechargeMultiplier,
		&supplier.KeyLimitPolicy,
		&supplier.KeyLimitValue,
		&supplier.ActiveKeyCount,
		&supplier.KeyCapacityStatus,
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
		&account.SupplierKeyID,
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

func scanSupplierAccountWithGroup(scanner supplierScanner) (*adminplusdomain.SupplierAccount, error) {
	var account adminplusdomain.SupplierAccount
	var runtimeStatus, healthStatus string
	err := scanner.Scan(
		&account.ID,
		&account.SupplierID,
		&account.SupplierKeyID,
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
		&account.SupplierGroupID,
		&account.SupplierExternalGroupID,
		&account.SupplierGroupName,
		&account.SupplierGroupProvider,
		&account.SupplierGroupRate,
		&account.SupplierKeyName,
		&account.SupplierKeyExternalID,
		&account.SupplierKeyLast4,
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
