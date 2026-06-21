package supplierkeys

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) GetSupplier(ctx context.Context, supplierID int64) (*adminplusdomain.Supplier, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, kind, type, runtime_status, health_status,
			dashboard_url, api_base_url, contact, notes,
			postgres_configured, redis_configured, browser_login_enabled,
			browser_login_username_configured, browser_login_password_configured, browser_login_token_configured, masked_browser_login_username,
			balance_cents, balance_currency, balance_updated_at, created_at, updated_at
		FROM admin_plus_suppliers
		WHERE id = $1
	`, supplierID)
	return scanSupplier(row)
}

func (r *SQLRepository) GetGroup(ctx context.Context, supplierID int64, groupID int64) (*adminplusdomain.SupplierGroup, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, supplier_id, external_group_id, name, description, provider_family,
			rate_multiplier, user_rate_multiplier, effective_rate_multiplier,
			rpm_limit, daily_limit_usd, weekly_limit_usd, monthly_limit_usd,
			allow_image_generation, is_private, status, raw_payload,
			last_seen_at, created_at, updated_at
		FROM admin_plus_supplier_groups
		WHERE supplier_id = $1 AND id = $2
	`, supplierID, groupID)
	return scanSupplierGroup(row)
}

func (r *SQLRepository) GetKey(ctx context.Context, supplierID int64, keyID int64) (*adminplusdomain.SupplierKey, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, supplier_id, supplier_group_id, external_group_id, external_key_id,
			name, key_fingerprint, key_last4, status, provider_family,
			local_sub2api_account_id, local_account_name, local_account_platform, local_account_type,
			provision_request, provision_response, error_code, error_message,
			created_at, updated_at
		FROM admin_plus_supplier_keys
		WHERE supplier_id = $1 AND id = $2
	`, supplierID, keyID)
	return scanSupplierKey(row)
}

func (r *SQLRepository) ListGroups(ctx context.Context, supplierID int64) ([]*adminplusdomain.SupplierGroup, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, supplier_id, external_group_id, name, description, provider_family,
			rate_multiplier, user_rate_multiplier, effective_rate_multiplier,
			rpm_limit, daily_limit_usd, weekly_limit_usd, monthly_limit_usd,
			allow_image_generation, is_private, status, raw_payload,
			last_seen_at, created_at, updated_at
		FROM admin_plus_supplier_groups
		WHERE supplier_id = $1 AND status = 'active'
		ORDER BY created_at DESC, id DESC
	`, supplierID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
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

func (r *SQLRepository) FindActiveByGroup(ctx context.Context, supplierID int64, groupID int64) (*adminplusdomain.SupplierKey, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, supplier_id, supplier_group_id, external_group_id, external_key_id,
			name, key_fingerprint, key_last4, status, provider_family,
			local_sub2api_account_id, local_account_name, local_account_platform, local_account_type,
			provision_request, provision_response, error_code, error_message,
			created_at, updated_at
		FROM admin_plus_supplier_keys
		WHERE supplier_id = $1
			AND supplier_group_id = $2
			AND status IN ('provisioning', 'bound', 'manual_secret_required')
		ORDER BY id DESC
		LIMIT 1
	`, supplierID, groupID)
	item, err := scanSupplierKey(row)
	if infraerrors.Reason(err) == "SUPPLIER_KEY_NOT_FOUND" {
		return nil, nil
	}
	return item, err
}

func (r *SQLRepository) CreateKey(ctx context.Context, key *adminplusdomain.SupplierKey) (*adminplusdomain.SupplierKey, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	requestJSON, err := marshalJSON(key.ProvisionRequest)
	if err != nil {
		return nil, err
	}
	responseJSON, err := marshalJSON(key.ProvisionResponse)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_supplier_keys (
			supplier_id, supplier_group_id, external_group_id, external_key_id,
			name, key_fingerprint, key_last4, status, provider_family,
			local_sub2api_account_id, local_account_name, local_account_platform, local_account_type,
			provision_request, provision_response, error_code, error_message,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 0, '', '', '', $10, $11, '', '', $12, $13)
		RETURNING id, supplier_id, supplier_group_id, external_group_id, external_key_id,
			name, key_fingerprint, key_last4, status, provider_family,
			local_sub2api_account_id, local_account_name, local_account_platform, local_account_type,
			provision_request, provision_response, error_code, error_message,
			created_at, updated_at
	`,
		key.SupplierID,
		key.SupplierGroupID,
		key.ExternalGroupID,
		key.ExternalKeyID,
		key.Name,
		key.KeyFingerprint,
		key.KeyLast4,
		string(key.Status),
		key.ProviderFamily,
		requestJSON,
		responseJSON,
		key.CreatedAt,
		key.UpdatedAt,
	)
	item, err := scanSupplierKey(row)
	if err != nil {
		return nil, translateKeyCreateError(err)
	}
	return item, nil
}

func (r *SQLRepository) UpdateKeyAfterLocalBind(ctx context.Context, keyID int64, localAccount *service.Account, status adminplusdomain.SupplierKeyStatus, errorCode string, errorMessage string) (*adminplusdomain.SupplierKey, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	localID := int64(0)
	localName := ""
	localPlatform := ""
	localType := ""
	if localAccount != nil {
		localID = localAccount.ID
		localName = localAccount.Name
		localPlatform = localAccount.Platform
		localType = localAccount.Type
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_supplier_keys
		SET local_sub2api_account_id = $2,
			local_account_name = $3,
			local_account_platform = $4,
			local_account_type = $5,
			status = $6,
			error_code = $7,
			error_message = $8,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, supplier_id, supplier_group_id, external_group_id, external_key_id,
			name, key_fingerprint, key_last4, status, provider_family,
			local_sub2api_account_id, local_account_name, local_account_platform, local_account_type,
			provision_request, provision_response, error_code, error_message,
			created_at, updated_at
	`,
		keyID,
		localID,
		localName,
		localPlatform,
		localType,
		string(status),
		errorCode,
		errorMessage,
	)
	item, err := scanSupplierKey(row)
	if err != nil {
		return nil, translateKeyCreateError(err)
	}
	return item, nil
}

func (r *SQLRepository) CreateBinding(ctx context.Context, account *adminplusdomain.SupplierAccount) (*adminplusdomain.SupplierAccount, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_supplier_accounts (
			supplier_id, supplier_key_id, local_sub2api_account_id,
			local_account_name, local_account_platform, local_account_type,
			supplier_account_identifier, supplier_account_label, organization_id, project_id, rate_profile,
			configured_concurrency, observed_max_concurrency,
			balance_threshold_cents, balance_cents, balance_currency, has_usable_balance,
			runtime_status, health_status, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13,
			$14, $15, $16, $17,
			$18, $19, $20, $21
		)
		RETURNING id
	`,
		account.SupplierID,
		account.SupplierKeyID,
		account.LocalSub2APIAccountID,
		account.LocalAccountName,
		account.LocalAccountPlatform,
		account.LocalAccountType,
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
	var createdID int64
	if err := row.Scan(&createdID); err != nil {
		return nil, translateBindingCreateError(err)
	}
	item, err := r.getBinding(ctx, account.SupplierID, createdID)
	if err != nil {
		return nil, translateBindingCreateError(err)
	}
	return item, nil
}

func (r *SQLRepository) UpsertBinding(ctx context.Context, account *adminplusdomain.SupplierAccount) (*adminplusdomain.SupplierAccount, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	if account == nil {
		return nil, infraerrors.New(http.StatusBadRequest, "SUPPLIER_ACCOUNT_INVALID", "supplier account binding is required")
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_supplier_accounts (
			supplier_id, supplier_key_id, local_sub2api_account_id,
			local_account_name, local_account_platform, local_account_type,
			supplier_account_identifier, supplier_account_label, organization_id, project_id, rate_profile,
			configured_concurrency, observed_max_concurrency,
			balance_threshold_cents, balance_cents, balance_currency, has_usable_balance,
			runtime_status, health_status, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13,
			$14, $15, $16, $17,
			$18, $19, $20, $21
		)
		ON CONFLICT (supplier_id, supplier_key_id)
		WHERE supplier_key_id > 0
		DO UPDATE SET
			local_sub2api_account_id = EXCLUDED.local_sub2api_account_id,
			local_account_name = EXCLUDED.local_account_name,
			local_account_platform = EXCLUDED.local_account_platform,
			local_account_type = EXCLUDED.local_account_type,
			supplier_account_identifier = EXCLUDED.supplier_account_identifier,
			supplier_account_label = EXCLUDED.supplier_account_label,
			organization_id = EXCLUDED.organization_id,
			project_id = EXCLUDED.project_id,
			rate_profile = EXCLUDED.rate_profile,
			configured_concurrency = EXCLUDED.configured_concurrency,
			observed_max_concurrency = EXCLUDED.observed_max_concurrency,
			balance_threshold_cents = EXCLUDED.balance_threshold_cents,
			balance_cents = EXCLUDED.balance_cents,
			balance_currency = EXCLUDED.balance_currency,
			has_usable_balance = EXCLUDED.has_usable_balance,
			runtime_status = EXCLUDED.runtime_status,
			health_status = EXCLUDED.health_status,
			updated_at = EXCLUDED.updated_at
		RETURNING id
	`,
		account.SupplierID,
		account.SupplierKeyID,
		account.LocalSub2APIAccountID,
		account.LocalAccountName,
		account.LocalAccountPlatform,
		account.LocalAccountType,
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
	var bindingID int64
	if err := row.Scan(&bindingID); err != nil {
		return nil, translateBindingCreateError(err)
	}
	return r.getBinding(ctx, account.SupplierID, bindingID)
}

func (r *SQLRepository) getBinding(ctx context.Context, supplierID int64, bindingID int64) (*adminplusdomain.SupplierAccount, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT asa.id, asa.supplier_id, asa.supplier_key_id, asa.local_sub2api_account_id,
			asa.local_account_name, asa.local_account_platform, asa.local_account_type,
			asa.supplier_account_identifier, asa.supplier_account_label, asa.organization_id, asa.project_id, asa.rate_profile,
			asa.configured_concurrency, asa.observed_max_concurrency,
			asa.balance_threshold_cents, asa.balance_cents, asa.balance_currency, asa.has_usable_balance,
			asa.runtime_status, asa.health_status, asa.created_at, asa.updated_at,
			COALESCE(sg.id, 0), COALESCE(sg.external_group_id, ''), COALESCE(sg.name, ''),
			COALESCE(sg.provider_family, ''), COALESCE(sg.effective_rate_multiplier, 0)
		FROM admin_plus_supplier_accounts asa
		LEFT JOIN admin_plus_supplier_keys sk ON sk.id = asa.supplier_key_id
		LEFT JOIN admin_plus_supplier_groups sg ON sg.id = sk.supplier_group_id
		WHERE asa.supplier_id = $1 AND asa.id = $2
	`, supplierID, bindingID)
	return scanSupplierAccountWithGroup(row)
}

func (r *SQLRepository) List(ctx context.Context, filter ListFilter) ([]*adminplusdomain.SupplierKey, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	where := []string{"supplier_id = $1"}
	args := []any{filter.SupplierID}
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if filter.Status != "" {
		where = append(where, "status = "+addArg(string(filter.Status)))
	}
	if filter.Query != "" {
		needle := "%" + strings.ToLower(filter.Query) + "%"
		where = append(where, "(LOWER(name) LIKE "+addArg(needle)+" OR LOWER(external_key_id) LIKE "+addArg(needle)+" OR LOWER(external_group_id) LIKE "+addArg(needle)+" OR key_last4 LIKE "+addArg(needle)+")")
	}
	limitRef := addArg(filter.Limit)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, supplier_id, supplier_group_id, external_group_id, external_key_id,
			name, key_fingerprint, key_last4, status, provider_family,
			local_sub2api_account_id, local_account_name, local_account_platform, local_account_type,
			provision_request, provision_response, error_code, error_message,
			created_at, updated_at
		FROM admin_plus_supplier_keys
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC
		LIMIT `+limitRef, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]*adminplusdomain.SupplierKey, 0)
	for rows.Next() {
		item, err := scanSupplierKey(rows)
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

type scanner interface {
	Scan(dest ...any) error
}

func scanSupplier(scanner scanner) (*adminplusdomain.Supplier, error) {
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
	if errors.Is(err, sql.ErrNoRows) {
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

func scanSupplierGroup(scanner scanner) (*adminplusdomain.SupplierGroup, error) {
	var group adminplusdomain.SupplierGroup
	var status string
	var rawPayload []byte
	var userRate sql.NullFloat64
	var rpmLimit sql.NullInt64
	var dailyLimit sql.NullFloat64
	var weeklyLimit sql.NullFloat64
	var monthlyLimit sql.NullFloat64
	err := scanner.Scan(
		&group.ID,
		&group.SupplierID,
		&group.ExternalGroupID,
		&group.Name,
		&group.Description,
		&group.ProviderFamily,
		&group.RateMultiplier,
		&userRate,
		&group.EffectiveRateMultiplier,
		&rpmLimit,
		&dailyLimit,
		&weeklyLimit,
		&monthlyLimit,
		&group.AllowImageGeneration,
		&group.IsPrivate,
		&status,
		&rawPayload,
		&group.LastSeenAt,
		&group.CreatedAt,
		&group.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_GROUP_NOT_FOUND", "supplier group not found")
	}
	if err != nil {
		return nil, err
	}
	group.Status = adminplusdomain.SupplierGroupStatus(status)
	if userRate.Valid {
		value := userRate.Float64
		group.UserRateMultiplier = &value
	}
	if rpmLimit.Valid {
		value := rpmLimit.Int64
		group.RPMLimit = &value
	}
	if dailyLimit.Valid {
		value := dailyLimit.Float64
		group.DailyLimitUSD = &value
	}
	if weeklyLimit.Valid {
		value := weeklyLimit.Float64
		group.WeeklyLimitUSD = &value
	}
	if monthlyLimit.Valid {
		value := monthlyLimit.Float64
		group.MonthlyLimitUSD = &value
	}
	if len(rawPayload) > 0 {
		var payload map[string]any
		if err := json.Unmarshal(rawPayload, &payload); err != nil {
			return nil, err
		}
		group.RawPayload = payload
	}
	return &group, nil
}

func scanSupplierKey(scanner scanner) (*adminplusdomain.SupplierKey, error) {
	var key adminplusdomain.SupplierKey
	var status string
	var requestJSON, responseJSON []byte
	err := scanner.Scan(
		&key.ID,
		&key.SupplierID,
		&key.SupplierGroupID,
		&key.ExternalGroupID,
		&key.ExternalKeyID,
		&key.Name,
		&key.KeyFingerprint,
		&key.KeyLast4,
		&status,
		&key.ProviderFamily,
		&key.LocalSub2APIAccountID,
		&key.LocalAccountName,
		&key.LocalAccountPlatform,
		&key.LocalAccountType,
		&requestJSON,
		&responseJSON,
		&key.ErrorCode,
		&key.ErrorMessage,
		&key.CreatedAt,
		&key.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_KEY_NOT_FOUND", "supplier key not found")
	}
	if err != nil {
		return nil, err
	}
	key.Status = adminplusdomain.SupplierKeyStatus(status)
	key.ProvisionRequest = unmarshalJSONMap(requestJSON)
	key.ProvisionResponse = unmarshalJSONMap(responseJSON)
	return &key, nil
}

func scanSupplierAccountWithGroup(scanner scanner) (*adminplusdomain.SupplierAccount, error) {
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
	)
	if err != nil {
		return nil, err
	}
	account.RuntimeStatus = adminplusdomain.SupplierRuntimeStatus(runtimeStatus)
	account.HealthStatus = adminplusdomain.SupplierHealthStatus(healthStatus)
	return &account, nil
}

func marshalJSON(value map[string]any) ([]byte, error) {
	if len(value) == 0 {
		return []byte(`{}`), nil
	}
	return json.Marshal(value)
}

func unmarshalJSONMap(data []byte) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func translateBindingCreateError(err error) error {
	message := err.Error()
	if strings.Contains(message, "admin_plus_supplier_accounts_unique_local_account") ||
		strings.Contains(message, "admin_plus_supplier_accounts_supplier_id_local_sub2api_account_id_key") {
		return infraerrors.New(http.StatusConflict, "SUPPLIER_ACCOUNT_ALREADY_BOUND", "local Sub2API account is already bound to this supplier")
	}
	return err
}

func translateKeyCreateError(err error) error {
	message := err.Error()
	if strings.Contains(message, "idx_admin_plus_supplier_keys_one_active_group") {
		return infraerrors.New(http.StatusConflict, "SUPPLIER_GROUP_KEY_ALREADY_BOUND", "supplier group already has a bound or provisioning key")
	}
	if strings.Contains(message, "idx_admin_plus_supplier_keys_fingerprint") {
		return infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_ALREADY_EXISTS", "supplier key already exists for this supplier")
	}
	return err
}

func dbNotConfigured() error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
}
