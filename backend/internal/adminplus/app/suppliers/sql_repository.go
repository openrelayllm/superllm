package suppliers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) Create(ctx context.Context, supplier *adminplusdomain.Supplier) (*adminplusdomain.Supplier, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_suppliers (
			name, kind, type, runtime_status, health_status,
			dashboard_url, api_base_url, contact, notes,
			postgres_configured, redis_configured, browser_login_enabled,
			browser_login_username_configured, browser_login_password_configured, browser_login_token_configured, masked_browser_login_username,
			balance_cents, balance_currency, balance_updated_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
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
		supplier.BalanceCents,
		supplier.BalanceCurrency,
		supplier.BalanceUpdatedAt,
		supplier.CreatedAt,
		supplier.UpdatedAt,
	)
	return scanSupplier(row)
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
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_supplier_accounts (
			supplier_id, local_sub2api_account_id,
			local_account_name, local_account_platform, local_account_type,
			supplier_account_identifier, supplier_account_label, organization_id, project_id, rate_profile,
			configured_concurrency, observed_max_concurrency,
			balance_threshold_cents, balance_cents, balance_currency, has_usable_balance,
			runtime_status, health_status, created_at, updated_at
		)
		SELECT
			$1, a.id, a.name, a.platform, a.type,
			$3, $4, $5, $6, $7,
			$8, $9,
			$10, $11, $12, $13,
			$14, $15, $16, $17
		FROM accounts a
		WHERE a.id = $2 AND a.deleted_at IS NULL
		RETURNING id, supplier_id, local_sub2api_account_id,
			local_account_name, local_account_platform, local_account_type,
			supplier_account_identifier, supplier_account_label, organization_id, project_id, rate_profile,
			configured_concurrency, observed_max_concurrency,
			balance_threshold_cents, balance_cents, balance_currency, has_usable_balance,
			runtime_status, health_status, created_at, updated_at
	`,
		account.SupplierID,
		account.LocalSub2APIAccountID,
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, infraerrors.New(http.StatusNotFound, "LOCAL_ACCOUNT_NOT_FOUND", "local Sub2API account not found")
		}
		return nil, translateSupplierAccountCreateError(err)
	}
	return created, nil
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
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
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
	rows, err := r.db.QueryContext(ctx, `
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

func translateSupplierAccountCreateError(err error) error {
	message := err.Error()
	if strings.Contains(message, "admin_plus_supplier_accounts_unique_local_account") ||
		strings.Contains(message, "admin_plus_supplier_accounts_supplier_id_local_sub2api_account_id_key") {
		return infraerrors.New(http.StatusConflict, "SUPPLIER_ACCOUNT_ALREADY_BOUND", "local Sub2API account is already bound to this supplier")
	}
	return err
}
