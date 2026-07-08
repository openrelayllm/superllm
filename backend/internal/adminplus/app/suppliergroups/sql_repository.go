package suppliergroups

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/lib/pq"
)

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func supplierGroupActiveKeyCountSQL(groupIDExpr string) string {
	return `(SELECT COUNT(*)::int
		FROM admin_plus_supplier_keys sk
		WHERE sk.supplier_group_id = ` + groupIDExpr + `
			AND sk.status IN ('provisioning', 'bound', 'manual_secret_required'))`
}

func (r *SQLRepository) GetSupplierName(ctx context.Context, supplierID int64) (string, error) {
	if r == nil || r.db == nil {
		return "", dbNotConfigured()
	}
	var name string
	if err := r.db.QueryRowContext(ctx, `
		SELECT name
		FROM admin_plus_suppliers
		WHERE id = $1
	`, supplierID).Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			return "", infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found")
		}
		return "", err
	}
	return name, nil
}

func (r *SQLRepository) UpsertMany(ctx context.Context, supplierID int64, groups []*adminplusdomain.SupplierGroup, seenAt time.Time) ([]*adminplusdomain.SupplierGroup, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	seenIDs := make([]string, 0, len(groups))
	saved := make([]*adminplusdomain.SupplierGroup, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		item, err := upsertGroup(ctx, tx, group)
		if err != nil {
			return nil, err
		}
		saved = append(saved, item)
		seenIDs = append(seenIDs, group.ExternalGroupID)
	}
	if len(seenIDs) > 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE admin_plus_supplier_groups
			SET status = 'missing',
				updated_at = $2
			WHERE supplier_id = $1
				AND external_group_id <> ALL($3)
				AND status <> 'missing'
		`, supplierID, seenAt, pq.Array(seenIDs)); err != nil {
			return nil, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE admin_plus_supplier_groups
			SET status = 'missing',
				updated_at = $2
			WHERE supplier_id = $1
				AND status <> 'missing'
		`, supplierID, seenAt); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return saved, nil
}

func upsertGroup(ctx context.Context, tx *sql.Tx, group *adminplusdomain.SupplierGroup) (*adminplusdomain.SupplierGroup, error) {
	rawPayload, err := marshalRawPayload(group.RawPayload)
	if err != nil {
		return nil, err
	}
	row := tx.QueryRowContext(ctx, `
		INSERT INTO admin_plus_supplier_groups (
			supplier_id, external_group_id, name, description, provider_family,
			official_name, model_family, model_spec, standard_key_name,
			rate_multiplier, user_rate_multiplier, effective_rate_multiplier,
			rpm_limit, daily_limit_usd, weekly_limit_usd, monthly_limit_usd,
			allow_image_generation, is_private, status, raw_payload,
			last_seen_at, naming_updated_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
		ON CONFLICT (supplier_id, external_group_id) DO UPDATE
		SET name = EXCLUDED.name,
			description = EXCLUDED.description,
			provider_family = EXCLUDED.provider_family,
			official_name = EXCLUDED.official_name,
			model_family = EXCLUDED.model_family,
			model_spec = EXCLUDED.model_spec,
			standard_key_name = EXCLUDED.standard_key_name,
			rate_multiplier = EXCLUDED.rate_multiplier,
			user_rate_multiplier = EXCLUDED.user_rate_multiplier,
			effective_rate_multiplier = EXCLUDED.effective_rate_multiplier,
			rpm_limit = EXCLUDED.rpm_limit,
			daily_limit_usd = EXCLUDED.daily_limit_usd,
			weekly_limit_usd = EXCLUDED.weekly_limit_usd,
			monthly_limit_usd = EXCLUDED.monthly_limit_usd,
			allow_image_generation = EXCLUDED.allow_image_generation,
			is_private = EXCLUDED.is_private,
			status = EXCLUDED.status,
			raw_payload = EXCLUDED.raw_payload,
			last_seen_at = EXCLUDED.last_seen_at,
			naming_updated_at = EXCLUDED.naming_updated_at,
			updated_at = EXCLUDED.updated_at
		RETURNING id, supplier_id, external_group_id, name, description, provider_family,
			official_name, model_family, model_spec, standard_key_name,
			rate_multiplier, user_rate_multiplier, effective_rate_multiplier,
			rpm_limit, daily_limit_usd, weekly_limit_usd, monthly_limit_usd,
			allow_image_generation, is_private,
			COALESCE(key_limit_policy, 'inherit'), COALESCE(key_limit_value, 0),
			`+supplierGroupActiveKeyCountSQL("admin_plus_supplier_groups.id")+`,
			status, raw_payload,
			last_seen_at, naming_updated_at, created_at, updated_at
	`,
		group.SupplierID,
		group.ExternalGroupID,
		group.Name,
		group.Description,
		group.ProviderFamily,
		group.OfficialName,
		group.ModelFamily,
		group.ModelSpec,
		group.StandardKeyName,
		group.RateMultiplier,
		nullableFloat64(group.UserRateMultiplier),
		group.EffectiveRateMultiplier,
		nullableInt64(group.RPMLimit),
		nullableFloat64(group.DailyLimitUSD),
		nullableFloat64(group.WeeklyLimitUSD),
		nullableFloat64(group.MonthlyLimitUSD),
		group.AllowImageGeneration,
		group.IsPrivate,
		string(group.Status),
		rawPayload,
		group.LastSeenAt,
		nullableTime(group.NamingUpdatedAt),
		group.CreatedAt,
		group.UpdatedAt,
	)
	return scanSupplierGroup(row)
}

func (r *SQLRepository) List(ctx context.Context, filter ListFilter) ([]*adminplusdomain.SupplierGroup, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	where := make([]string, 0, 3)
	args := make([]any, 0, 4)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if filter.SupplierID > 0 {
		where = append(where, "supplier_id = "+addArg(filter.SupplierID))
	}
	if filter.Status != "" {
		where = append(where, "status = "+addArg(string(filter.Status)))
	}
	if filter.Query != "" {
		needle := "%" + strings.ToLower(filter.Query) + "%"
		where = append(where, "(LOWER(name) LIKE "+addArg(needle)+" OR LOWER(description) LIKE "+addArg(needle)+" OR LOWER(provider_family) LIKE "+addArg(needle)+" OR LOWER(external_group_id) LIKE "+addArg(needle)+" OR LOWER(official_name) LIKE "+addArg(needle)+" OR LOWER(model_family) LIKE "+addArg(needle)+" OR LOWER(model_spec) LIKE "+addArg(needle)+" OR LOWER(standard_key_name) LIKE "+addArg(needle)+")")
	}
	limitRef := addArg(filter.Limit)
	query := `
		SELECT id, supplier_id, external_group_id, name, description, provider_family,
			official_name, model_family, model_spec, standard_key_name,
			rate_multiplier, user_rate_multiplier, effective_rate_multiplier,
			rpm_limit, daily_limit_usd, weekly_limit_usd, monthly_limit_usd,
			allow_image_generation, is_private,
			COALESCE(key_limit_policy, 'inherit'), COALESCE(key_limit_value, 0),
			` + supplierGroupActiveKeyCountSQL("admin_plus_supplier_groups.id") + `,
			status, raw_payload,
			last_seen_at, naming_updated_at, created_at, updated_at
		FROM admin_plus_supplier_groups
		` + supplierGroupWhereClause(where) + `
		ORDER BY last_seen_at DESC, id DESC
		LIMIT ` + limitRef
	rows, err := r.db.QueryContext(ctx, query, args...)
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

func (r *SQLRepository) UpdateKeyCapacity(ctx context.Context, in UpdateKeyCapacityInput) (*adminplusdomain.SupplierGroup, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_supplier_groups
		SET key_limit_policy = $3,
			key_limit_value = $4,
			updated_at = NOW()
		WHERE supplier_id = $1 AND id = $2
		RETURNING id, supplier_id, external_group_id, name, description, provider_family,
			official_name, model_family, model_spec, standard_key_name,
			rate_multiplier, user_rate_multiplier, effective_rate_multiplier,
			rpm_limit, daily_limit_usd, weekly_limit_usd, monthly_limit_usd,
			allow_image_generation, is_private,
			COALESCE(key_limit_policy, 'inherit'), COALESCE(key_limit_value, 0),
			`+supplierGroupActiveKeyCountSQL("admin_plus_supplier_groups.id")+`,
			status, raw_payload,
			last_seen_at, naming_updated_at, created_at, updated_at
	`, in.SupplierID, in.SupplierGroupID, in.KeyLimitPolicy, in.KeyLimitValue)
	return scanSupplierGroup(row)
}

func supplierGroupWhereClause(where []string) string {
	if len(where) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(where, " AND ")
}

func (r *SQLRepository) CreateChangeEvents(ctx context.Context, events []*adminplusdomain.SupplierGroupChangeEvent) ([]*adminplusdomain.SupplierGroupChangeEvent, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	out := make([]*adminplusdomain.SupplierGroupChangeEvent, 0, len(events))
	for _, event := range events {
		if event == nil {
			continue
		}
		row := r.db.QueryRowContext(ctx, `
			INSERT INTO admin_plus_supplier_group_change_events (
				supplier_id, supplier_group_id, external_group_id, group_name,
				provider_family, direction, old_effective_rate_multiplier,
				new_effective_rate_multiplier, change_percent, low_rate, created_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING id, supplier_id, supplier_group_id, external_group_id, group_name,
				provider_family, direction, old_effective_rate_multiplier,
				new_effective_rate_multiplier, change_percent, low_rate, created_at
		`,
			event.SupplierID,
			event.SupplierGroupID,
			event.ExternalGroupID,
			event.GroupName,
			event.ProviderFamily,
			string(event.Direction),
			nullableFloat64(event.OldEffectiveRateMultiplier),
			event.NewEffectiveRateMultiplier,
			nullableFloat64(event.ChangePercent),
			event.LowRate,
			event.CreatedAt,
		)
		created, err := scanSupplierGroupChangeEvent(row)
		if err != nil {
			return nil, err
		}
		out = append(out, created)
	}
	return out, nil
}

func (r *SQLRepository) ListChangeEvents(ctx context.Context, filter EventFilter) ([]*adminplusdomain.SupplierGroupChangeEvent, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	where := []string{"supplier_id = $1"}
	args := []any{filter.SupplierID}
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if filter.Direction != "" {
		where = append(where, "direction = "+addArg(string(filter.Direction)))
	}
	if filter.LowRate != nil {
		where = append(where, "low_rate = "+addArg(*filter.LowRate))
	}
	limitRef := addArg(filter.Limit)
	query := `
		SELECT id, supplier_id, supplier_group_id, external_group_id, group_name,
			provider_family, direction, old_effective_rate_multiplier,
			new_effective_rate_multiplier, change_percent, low_rate, created_at
		FROM admin_plus_supplier_group_change_events
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY created_at DESC, id DESC
		LIMIT ` + limitRef
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]*adminplusdomain.SupplierGroupChangeEvent, 0)
	for rows.Next() {
		item, err := scanSupplierGroupChangeEvent(rows)
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

type supplierGroupScanner interface {
	Scan(dest ...any) error
}

func scanSupplierGroup(scanner supplierGroupScanner) (*adminplusdomain.SupplierGroup, error) {
	var group adminplusdomain.SupplierGroup
	var status string
	var rawPayload []byte
	var userRate sql.NullFloat64
	var rpmLimit sql.NullInt64
	var dailyLimit sql.NullFloat64
	var weeklyLimit sql.NullFloat64
	var monthlyLimit sql.NullFloat64
	var namingUpdatedAt sql.NullTime
	err := scanner.Scan(
		&group.ID,
		&group.SupplierID,
		&group.ExternalGroupID,
		&group.Name,
		&group.Description,
		&group.ProviderFamily,
		&group.OfficialName,
		&group.ModelFamily,
		&group.ModelSpec,
		&group.StandardKeyName,
		&group.RateMultiplier,
		&userRate,
		&group.EffectiveRateMultiplier,
		&rpmLimit,
		&dailyLimit,
		&weeklyLimit,
		&monthlyLimit,
		&group.AllowImageGeneration,
		&group.IsPrivate,
		&group.KeyLimitPolicy,
		&group.KeyLimitValue,
		&group.ActiveKeyCount,
		&status,
		&rawPayload,
		&group.LastSeenAt,
		&namingUpdatedAt,
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
	group.KeyLimitPolicy = normalizeGroupKeyLimitPolicy(group.KeyLimitPolicy)
	group.KeyCapacityStatus = groupKeyCapacityStatus(group.KeyLimitPolicy, group.KeyLimitValue, group.ActiveKeyCount)
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
	if namingUpdatedAt.Valid {
		value := namingUpdatedAt.Time
		group.NamingUpdatedAt = &value
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

func groupKeyCapacityStatus(policy string, limit int, activeCount int) string {
	switch normalizeGroupKeyLimitPolicy(policy) {
	case adminplusdomain.SupplierGroupKeyLimitPolicyInherit:
		return adminplusdomain.SupplierGroupKeyLimitPolicyInherit
	case adminplusdomain.SupplierGroupKeyLimitPolicyUnlimited:
		return adminplusdomain.SupplierKeyCapacityAvailable
	case adminplusdomain.SupplierGroupKeyLimitPolicyLimited:
		if limit <= activeCount {
			return adminplusdomain.SupplierKeyCapacityExhausted
		}
		if limit-activeCount <= 2 {
			return adminplusdomain.SupplierKeyCapacityLimited
		}
		return adminplusdomain.SupplierKeyCapacityAvailable
	case adminplusdomain.SupplierGroupKeyLimitPolicyUnsupported:
		return adminplusdomain.SupplierKeyCapacityUnsupported
	default:
		return adminplusdomain.SupplierKeyCapacityUnknown
	}
}

func scanSupplierGroupChangeEvent(scanner supplierGroupScanner) (*adminplusdomain.SupplierGroupChangeEvent, error) {
	var event adminplusdomain.SupplierGroupChangeEvent
	var direction string
	var oldRate sql.NullFloat64
	var changePercent sql.NullFloat64
	err := scanner.Scan(
		&event.ID,
		&event.SupplierID,
		&event.SupplierGroupID,
		&event.ExternalGroupID,
		&event.GroupName,
		&event.ProviderFamily,
		&direction,
		&oldRate,
		&event.NewEffectiveRateMultiplier,
		&changePercent,
		&event.LowRate,
		&event.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	event.Direction = adminplusdomain.SupplierGroupChangeDirection(direction)
	if oldRate.Valid {
		value := oldRate.Float64
		event.OldEffectiveRateMultiplier = &value
	}
	if changePercent.Valid {
		value := changePercent.Float64
		event.ChangePercent = &value
	}
	return &event, nil
}

func marshalRawPayload(value map[string]any) ([]byte, error) {
	if len(value) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(value)
}

func nullableFloat64(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
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
