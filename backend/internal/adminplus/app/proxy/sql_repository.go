package proxy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/lib/pq"
)

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) CenterStatus(ctx context.Context) (*adminplusdomain.ProxyCenterStatus, error) {
	if r == nil || r.db == nil {
		return nil, internalError("proxy repository is not configured")
	}
	var out adminplusdomain.ProxyCenterStatus
	row := r.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM admin_plus_proxy_subscriptions),
			(SELECT COUNT(*) FROM admin_plus_proxy_subscriptions WHERE enabled),
			(SELECT COUNT(*) FROM admin_plus_proxy_nodes),
			(SELECT COUNT(*) FROM admin_plus_proxy_nodes WHERE health_status = 'healthy'),
			(SELECT COUNT(*) FROM admin_plus_proxy_policies),
			(SELECT COUNT(*) FROM admin_plus_proxy_target_policies WHERE enabled),
			(SELECT COUNT(*) FROM admin_plus_proxy_runtime_slots),
			(SELECT COUNT(*) FROM admin_plus_proxy_runtime_slots WHERE status = 'assigned'),
			(SELECT COUNT(*) FROM admin_plus_proxy_assignments WHERE status = 'active'),
			(SELECT COUNT(*) FROM admin_plus_proxy_audit_events WHERE level = 'error' AND created_at >= NOW() - INTERVAL '24 hours')
	`)
	err := row.Scan(
		&out.SubscriptionsTotal,
		&out.SubscriptionsActive,
		&out.NodesTotal,
		&out.HealthyNodes,
		&out.PoliciesTotal,
		&out.TargetsTotal,
		&out.SlotsTotal,
		&out.SlotsAssigned,
		&out.AssignmentsActive,
		&out.RecentErrors,
	)
	return &out, err
}

func (r *SQLRepository) CreateSubscription(ctx context.Context, subscription *adminplusdomain.ProxySubscription, urlCiphertext string) (*adminplusdomain.ProxySubscription, error) {
	if r == nil || r.db == nil {
		return nil, internalError("proxy repository is not configured")
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_proxy_subscriptions (
			name, subscription_type, url_ciphertext, url_hash, enabled,
			refresh_interval_seconds, last_refresh_status, created_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, 0))
		RETURNING `+subscriptionColumns(),
		subscription.Name,
		string(subscription.SubscriptionType),
		urlCiphertext,
		subscription.URLHash,
		subscription.Enabled,
		subscription.RefreshIntervalSeconds,
		string(subscription.LastRefreshStatus),
		subscription.CreatedBy,
	)
	return scanSubscription(row)
}

func (r *SQLRepository) UpdateSubscription(ctx context.Context, id int64, input UpdateSubscriptionInput) (*adminplusdomain.ProxySubscription, error) {
	current, secret, err := r.GetSubscriptionSecret(ctx, id)
	if err != nil {
		return nil, err
	}
	name := current.Name
	if input.Name != nil {
		name = strings.TrimSpace(*input.Name)
	}
	subscriptionType := current.SubscriptionType
	if input.SubscriptionType != nil {
		subscriptionType = *input.SubscriptionType
	}
	enabled := current.Enabled
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	refreshInterval := current.RefreshIntervalSeconds
	if input.RefreshIntervalSeconds != nil {
		refreshInterval = *input.RefreshIntervalSeconds
	}
	urlHash := current.URLHash
	if input.SubscriptionURLHash != nil {
		urlHash = *input.SubscriptionURLHash
	}
	if input.SubscriptionURLCiphertext != nil {
		secret = *input.SubscriptionURLCiphertext
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_proxy_subscriptions
		SET name = $2,
			subscription_type = $3,
			url_ciphertext = $4,
			url_hash = $5,
			enabled = $6,
			refresh_interval_seconds = $7,
			updated_at = NOW()
		WHERE id = $1
		RETURNING `+subscriptionColumns(),
		id, name, string(subscriptionType), secret, urlHash, enabled, refreshInterval,
	)
	return scanSubscription(row)
}

func (r *SQLRepository) DeleteSubscription(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return internalError("proxy repository is not configured")
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM admin_plus_proxy_subscriptions WHERE id = $1`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return notFound("PROXY_SUBSCRIPTION_NOT_FOUND", "proxy subscription not found")
	}
	return nil
}

func (r *SQLRepository) GetSubscription(ctx context.Context, id int64) (*adminplusdomain.ProxySubscription, error) {
	if r == nil || r.db == nil {
		return nil, internalError("proxy repository is not configured")
	}
	row := r.db.QueryRowContext(ctx, `SELECT `+subscriptionColumns()+` FROM admin_plus_proxy_subscriptions WHERE id = $1`, id)
	return scanSubscription(row)
}

func (r *SQLRepository) GetSubscriptionSecret(ctx context.Context, id int64) (*adminplusdomain.ProxySubscription, string, error) {
	if r == nil || r.db == nil {
		return nil, "", internalError("proxy repository is not configured")
	}
	row := r.db.QueryRowContext(ctx, `SELECT `+subscriptionColumns()+`, url_ciphertext FROM admin_plus_proxy_subscriptions WHERE id = $1`, id)
	subscription, secret, err := scanSubscriptionWithSecret(row)
	return subscription, secret, err
}

func (r *SQLRepository) ListSubscriptions(ctx context.Context, filter SubscriptionFilter) ([]*adminplusdomain.ProxySubscription, error) {
	if r == nil || r.db == nil {
		return nil, internalError("proxy repository is not configured")
	}
	args := make([]any, 0, 2)
	where := []string{"1=1"}
	if filter.Enabled != nil {
		args = append(args, *filter.Enabled)
		where = append(where, fmt.Sprintf("enabled = $%d", len(args)))
	}
	limit := normalizedLimit(filter.Limit, 500)
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+subscriptionColumns()+`
		FROM admin_plus_proxy_subscriptions
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY updated_at DESC, id DESC
		LIMIT $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubscriptions(rows)
}

func (r *SQLRepository) SaveConfigVersion(ctx context.Context, subscriptionID int64, configVersion string, mihomoYAML []byte, generatedAt time.Time) error {
	if r == nil || r.db == nil {
		return internalError("proxy repository is not configured")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO admin_plus_proxy_config_versions (
			subscription_id, config_version, mihomo_yaml, generated_at
		)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (subscription_id, config_version) DO UPDATE
		SET mihomo_yaml = EXCLUDED.mihomo_yaml,
			generated_at = EXCLUDED.generated_at
	`, subscriptionID, configVersion, string(mihomoYAML), generatedAt)
	return err
}

func (r *SQLRepository) GetConfigVersion(ctx context.Context, subscriptionID int64, configVersion string) ([]byte, error) {
	if r == nil || r.db == nil {
		return nil, internalError("proxy repository is not configured")
	}
	var raw string
	err := r.db.QueryRowContext(ctx, `
		SELECT mihomo_yaml
		FROM admin_plus_proxy_config_versions
		WHERE subscription_id = $1 AND config_version = $2
	`, subscriptionID, configVersion).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, notFound("PROXY_CONFIG_VERSION_NOT_FOUND", "proxy config version not found")
	}
	return []byte(raw), err
}

func (r *SQLRepository) ReplaceNodes(ctx context.Context, subscriptionID int64, configVersion string, nodes []*adminplusdomain.ProxyNode) error {
	if r == nil || r.db == nil {
		return internalError("proxy repository is not configured")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, `DELETE FROM admin_plus_proxy_nodes WHERE subscription_id = $1`, subscriptionID); err != nil {
		return err
	}
	for _, node := range nodes {
		raw, marshalErr := json.Marshal(node.RawMetadata)
		if marshalErr != nil {
			err = marshalErr
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO admin_plus_proxy_nodes (
				subscription_id, config_version, node_key, display_name, protocol,
				region, server_hash, health_status, raw_metadata
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
		`, subscriptionID, configVersion, node.NodeKey, node.DisplayName, node.Protocol, node.Region, node.ServerHash, string(node.HealthStatus), string(raw))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *SQLRepository) UpdateSubscriptionRefresh(ctx context.Context, id int64, status adminplusdomain.ProxyRefreshStatus, refreshError string, activeConfigVersion string, nodeCount int, refreshedAt *time.Time) (*adminplusdomain.ProxySubscription, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_proxy_subscriptions
		SET last_refresh_status = $2,
			last_refresh_error = $3,
			active_config_version = $4,
			node_count = $5,
			last_refreshed_at = $6,
			updated_at = NOW()
		WHERE id = $1
		RETURNING `+subscriptionColumns(),
		id, string(status), refreshError, activeConfigVersion, nodeCount, nullableTime(refreshedAt),
	)
	return scanSubscription(row)
}

func (r *SQLRepository) ListNodes(ctx context.Context, filter NodeFilter) ([]*adminplusdomain.ProxyNode, error) {
	if r == nil || r.db == nil {
		return nil, internalError("proxy repository is not configured")
	}
	args := make([]any, 0, 6)
	where := []string{"1=1"}
	if filter.SubscriptionID > 0 {
		args = append(args, filter.SubscriptionID)
		where = append(where, fmt.Sprintf("subscription_id = $%d", len(args)))
	}
	if len(filter.SubscriptionIDs) > 0 {
		args = append(args, pq.Array(filter.SubscriptionIDs))
		where = append(where, fmt.Sprintf("subscription_id = ANY($%d)", len(args)))
	}
	if filter.HealthStatus != "" {
		args = append(args, string(filter.HealthStatus))
		where = append(where, fmt.Sprintf("health_status = $%d", len(args)))
	}
	if !filter.IncludeDisabled {
		where = append(where, "health_status <> 'disabled'")
	}
	if strings.TrimSpace(filter.Query) != "" {
		args = append(args, "%"+strings.TrimSpace(filter.Query)+"%")
		where = append(where, fmt.Sprintf("(display_name ILIKE $%d OR region ILIKE $%d OR protocol ILIKE $%d)", len(args), len(args), len(args)))
	}
	limit := normalizedLimit(filter.Limit, 1000)
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+nodeColumns()+`
		FROM admin_plus_proxy_nodes
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY
			CASE health_status
				WHEN 'healthy' THEN 1
				WHEN 'unknown' THEN 2
				WHEN 'degraded' THEN 3
				ELSE 4
			END,
			COALESCE(last_latency_ms, 2147483647),
			updated_at DESC,
			id DESC
		LIMIT $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

func (r *SQLRepository) GetNode(ctx context.Context, id int64) (*adminplusdomain.ProxyNode, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+nodeColumns()+` FROM admin_plus_proxy_nodes WHERE id = $1`, id)
	return scanNode(row)
}

func (r *SQLRepository) UpdateNodeHealth(ctx context.Context, id int64, status adminplusdomain.ProxyNodeHealthStatus, latencyMS *int, egressIP string, errorCode string, errorMessage string, checkedAt *time.Time) (*adminplusdomain.ProxyNode, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_proxy_nodes
		SET health_status = $2,
			last_latency_ms = $3,
			last_egress_ip = $4,
			last_error_code = $5,
			last_error_message = $6,
			last_checked_at = $7,
			updated_at = NOW()
		WHERE id = $1
		RETURNING `+nodeColumns(),
		id, string(status), nullableInt(latencyMS), egressIP, errorCode, errorMessage, nullableTime(checkedAt),
	)
	return scanNode(row)
}

func (r *SQLRepository) UpdateNodeDisabled(ctx context.Context, id int64, disabled bool, reason string) (*adminplusdomain.ProxyNode, error) {
	status := adminplusdomain.ProxyNodeHealthHealthy
	if disabled {
		status = adminplusdomain.ProxyNodeHealthDisabled
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_proxy_nodes
		SET health_status = $2,
			disabled_reason = $3,
			updated_at = NOW()
		WHERE id = $1
		RETURNING `+nodeColumns(), id, string(status), reason)
	return scanNode(row)
}

func (r *SQLRepository) RecordHealthCheck(ctx context.Context, check *adminplusdomain.ProxyHealthCheck) (*adminplusdomain.ProxyHealthCheck, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_proxy_health_checks (
			node_id, check_type, status, latency_ms, egress_ip,
			target_host, error_code, error_message, checked_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, node_id, check_type, status, latency_ms, egress_ip,
			target_host, error_code, error_message, checked_at, created_at
	`, check.NodeID, check.CheckType, check.Status, nullableInt(check.LatencyMS), check.EgressIP, check.TargetHost, check.ErrorCode, check.ErrorMessage, check.CheckedAt)
	var out adminplusdomain.ProxyHealthCheck
	var latency sql.NullInt64
	err := row.Scan(&out.ID, &out.NodeID, &out.CheckType, &out.Status, &latency, &out.EgressIP, &out.TargetHost, &out.ErrorCode, &out.ErrorMessage, &out.CheckedAt, &out.CreatedAt)
	if latency.Valid {
		v := int(latency.Int64)
		out.LatencyMS = &v
	}
	return &out, err
}

func (r *SQLRepository) ListPolicies(ctx context.Context, filter PolicyFilter) ([]*adminplusdomain.ProxyPolicy, error) {
	args := make([]any, 0, 2)
	where := []string{"1=1"}
	if filter.Enabled != nil {
		args = append(args, *filter.Enabled)
		where = append(where, fmt.Sprintf("enabled = $%d", len(args)))
	}
	limit := normalizedLimit(filter.Limit, 500)
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+policyColumns()+`,
			(SELECT COUNT(*) FROM admin_plus_proxy_target_policies t WHERE t.policy_id = p.id AND t.enabled) AS enabled_targets,
			(SELECT COUNT(*) FROM admin_plus_proxy_nodes n WHERE n.health_status = 'healthy') AS healthy_nodes_available
		FROM admin_plus_proxy_policies p
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY updated_at DESC, id DESC
		LIMIT $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPolicies(rows)
}

func (r *SQLRepository) GetPolicy(ctx context.Context, id int64) (*adminplusdomain.ProxyPolicy, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+policyColumns()+`,
			(SELECT COUNT(*) FROM admin_plus_proxy_target_policies t WHERE t.policy_id = p.id AND t.enabled) AS enabled_targets,
			(SELECT COUNT(*) FROM admin_plus_proxy_nodes n WHERE n.health_status = 'healthy') AS healthy_nodes_available
		FROM admin_plus_proxy_policies p
		WHERE p.id = $1
	`, id)
	return scanPolicy(row)
}

func (r *SQLRepository) CreatePolicy(ctx context.Context, policy *adminplusdomain.ProxyPolicy) (*adminplusdomain.ProxyPolicy, error) {
	subscriptions, _ := json.Marshal(policy.SubscriptionIDs)
	regions, _ := json.Marshal(policy.PreferredRegions)
	configJSON, _ := json.Marshal(nilSafeMap(policy.Config))
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_proxy_policies (
			name, enabled, subscription_ids, preferred_regions,
			max_concurrency, max_switches_per_task, connect_timeout_ms,
			request_timeout_ms, config
		)
		VALUES ($1, $2, $3::jsonb, $4::jsonb, $5, $6, $7, $8, $9::jsonb)
		RETURNING `+policyColumns()+`, 0, 0
	`, policy.Name, policy.Enabled, string(subscriptions), string(regions), policy.MaxConcurrency, policy.MaxSwitchesPerTask, policy.ConnectTimeoutMS, policy.RequestTimeoutMS, string(configJSON))
	return scanPolicy(row)
}

func (r *SQLRepository) UpdatePolicy(ctx context.Context, id int64, input UpdatePolicyInput) (*adminplusdomain.ProxyPolicy, error) {
	current, err := r.GetPolicy(ctx, id)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		current.Name = strings.TrimSpace(*input.Name)
	}
	if input.Enabled != nil {
		current.Enabled = *input.Enabled
	}
	if input.SubscriptionIDs != nil {
		current.SubscriptionIDs = dedupeInt64(input.SubscriptionIDs)
	}
	if input.PreferredRegions != nil {
		current.PreferredRegions = dedupeStrings(input.PreferredRegions)
	}
	if input.MaxConcurrency != nil {
		current.MaxConcurrency = *input.MaxConcurrency
	}
	if input.MaxSwitchesPerTask != nil {
		current.MaxSwitchesPerTask = *input.MaxSwitchesPerTask
	}
	if input.ConnectTimeoutMS != nil {
		current.ConnectTimeoutMS = *input.ConnectTimeoutMS
	}
	if input.RequestTimeoutMS != nil {
		current.RequestTimeoutMS = *input.RequestTimeoutMS
	}
	if input.Config != nil {
		current.Config = input.Config
	}
	subscriptions, _ := json.Marshal(current.SubscriptionIDs)
	regions, _ := json.Marshal(current.PreferredRegions)
	configJSON, _ := json.Marshal(nilSafeMap(current.Config))
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_proxy_policies
		SET name = $2,
			enabled = $3,
			subscription_ids = $4::jsonb,
			preferred_regions = $5::jsonb,
			max_concurrency = $6,
			max_switches_per_task = $7,
			connect_timeout_ms = $8,
			request_timeout_ms = $9,
			config = $10::jsonb,
			updated_at = NOW()
		WHERE id = $1
		RETURNING `+policyColumns()+`,
			(SELECT COUNT(*) FROM admin_plus_proxy_target_policies t WHERE t.policy_id = admin_plus_proxy_policies.id AND t.enabled),
			(SELECT COUNT(*) FROM admin_plus_proxy_nodes n WHERE n.health_status = 'healthy')
	`, id, current.Name, current.Enabled, string(subscriptions), string(regions), current.MaxConcurrency, current.MaxSwitchesPerTask, current.ConnectTimeoutMS, current.RequestTimeoutMS, string(configJSON))
	return scanPolicy(row)
}

func (r *SQLRepository) DeletePolicy(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return internalError("proxy repository is not configured")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, `DELETE FROM admin_plus_proxy_target_policies WHERE policy_id = $1`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE admin_plus_proxy_policies
		SET enabled = FALSE,
			subscription_ids = '[]'::jsonb,
			preferred_regions = '[]'::jsonb,
			updated_at = NOW()
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
		err = notFound("PROXY_POLICY_NOT_FOUND", "proxy policy not found")
		return err
	}
	return tx.Commit()
}

func (r *SQLRepository) ListTargets(ctx context.Context, filter TargetFilter) ([]*adminplusdomain.ProxyTargetPolicy, error) {
	args := make([]any, 0, 4)
	where := []string{"1=1"}
	if filter.PolicyID > 0 {
		args = append(args, filter.PolicyID)
		where = append(where, fmt.Sprintf("policy_id = $%d", len(args)))
	}
	if filter.Purpose != "" {
		args = append(args, string(filter.Purpose))
		where = append(where, fmt.Sprintf("purpose = $%d", len(args)))
	}
	if filter.Enabled != nil {
		args = append(args, *filter.Enabled)
		where = append(where, fmt.Sprintf("enabled = $%d", len(args)))
	}
	limit := normalizedLimit(filter.Limit, 1000)
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+targetColumns()+`
		FROM admin_plus_proxy_target_policies
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY updated_at DESC, id DESC
		LIMIT $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTargets(rows)
}

func (r *SQLRepository) CreateTarget(ctx context.Context, target *adminplusdomain.ProxyTargetPolicy) (*adminplusdomain.ProxyTargetPolicy, error) {
	methods, _ := json.Marshal(target.AllowedMethods)
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_proxy_target_policies (
			policy_id, target_host, purpose, allowed_methods,
			rate_limit_per_minute, enabled, authorization_note
		)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7)
		ON CONFLICT (policy_id, target_host, purpose) DO UPDATE
		SET allowed_methods = EXCLUDED.allowed_methods,
			rate_limit_per_minute = EXCLUDED.rate_limit_per_minute,
			enabled = EXCLUDED.enabled,
			authorization_note = EXCLUDED.authorization_note,
			updated_at = NOW()
		RETURNING `+targetColumns(),
		target.PolicyID, target.TargetHost, string(target.Purpose), string(methods), target.RateLimitPerMinute, target.Enabled, target.AuthorizationNote,
	)
	return scanTarget(row)
}

func (r *SQLRepository) UpdateTarget(ctx context.Context, policyID int64, targetID int64, input UpdateTargetInput) (*adminplusdomain.ProxyTargetPolicy, error) {
	current, err := r.getTarget(ctx, policyID, targetID)
	if err != nil {
		return nil, err
	}
	if input.TargetHost != nil {
		current.TargetHost = *input.TargetHost
	}
	if input.Purpose != nil {
		current.Purpose = *input.Purpose
	}
	if input.AllowedMethods != nil {
		current.AllowedMethods = input.AllowedMethods
	}
	if input.RateLimitPerMinute != nil {
		current.RateLimitPerMinute = *input.RateLimitPerMinute
	}
	if input.Enabled != nil {
		current.Enabled = *input.Enabled
	}
	if input.AuthorizationNote != nil {
		current.AuthorizationNote = *input.AuthorizationNote
	}
	methods, _ := json.Marshal(current.AllowedMethods)
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_proxy_target_policies
		SET target_host = $3,
			purpose = $4,
			allowed_methods = $5::jsonb,
			rate_limit_per_minute = $6,
			enabled = $7,
			authorization_note = $8,
			updated_at = NOW()
		WHERE policy_id = $1 AND id = $2
		RETURNING `+targetColumns(),
		policyID, targetID, current.TargetHost, string(current.Purpose), string(methods),
		current.RateLimitPerMinute, current.Enabled, current.AuthorizationNote,
	)
	return scanTarget(row)
}

func (r *SQLRepository) DeleteTarget(ctx context.Context, policyID int64, targetID int64) error {
	if r == nil || r.db == nil {
		return internalError("proxy repository is not configured")
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM admin_plus_proxy_target_policies WHERE policy_id = $1 AND id = $2`, policyID, targetID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return notFound("PROXY_TARGET_POLICY_NOT_FOUND", "proxy target policy not found")
	}
	return nil
}

func (r *SQLRepository) getTarget(ctx context.Context, policyID int64, targetID int64) (*adminplusdomain.ProxyTargetPolicy, error) {
	if r == nil || r.db == nil {
		return nil, internalError("proxy repository is not configured")
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+targetColumns()+`
		FROM admin_plus_proxy_target_policies
		WHERE policy_id = $1 AND id = $2
	`, policyID, targetID)
	return scanTarget(row)
}

func (r *SQLRepository) ListRuntimeSlots(ctx context.Context, filter RuntimeSlotFilter) ([]*adminplusdomain.ProxyRuntimeSlot, error) {
	args := make([]any, 0, 2)
	where := []string{"1=1"}
	if filter.Status != "" {
		args = append(args, string(filter.Status))
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	limit := normalizedLimit(filter.Limit, 1000)
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+slotColumns()+`
		FROM admin_plus_proxy_runtime_slots
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id ASC
		LIMIT $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSlots(rows)
}

func (r *SQLRepository) CreateRuntimeSlot(ctx context.Context, slot *adminplusdomain.ProxyRuntimeSlot, controllerSecretCiphertext string) (*adminplusdomain.ProxyRuntimeSlot, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_proxy_runtime_slots (
			slot_key, status, mixed_port, controller_port, controller_secret_ciphertext
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+slotColumns(),
		slot.SlotKey, string(slot.Status), slot.MixedPort, slot.ControllerPort, controllerSecretCiphertext,
	)
	return scanSlot(row)
}

func (r *SQLRepository) GetRuntimeSlotSecret(ctx context.Context, id int64) (*adminplusdomain.ProxyRuntimeSlot, string, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+slotColumns()+`, controller_secret_ciphertext FROM admin_plus_proxy_runtime_slots WHERE id = $1`, id)
	return scanSlotWithSecret(row)
}

func (r *SQLRepository) UpdateRuntimeSlot(ctx context.Context, id int64, input UpdateRuntimeSlotInput) (*adminplusdomain.ProxyRuntimeSlot, error) {
	slot, secret, err := r.GetRuntimeSlotSecret(ctx, id)
	if err != nil {
		return nil, err
	}
	if input.Status != nil {
		slot.Status = *input.Status
	}
	if input.MixedPort != nil {
		slot.MixedPort = *input.MixedPort
	}
	if input.ControllerPort != nil {
		slot.ControllerPort = *input.ControllerPort
	}
	if input.ControllerSecretCiphertext != nil {
		secret = *input.ControllerSecretCiphertext
	}
	if input.ProcessID != nil {
		slot.ProcessID = *input.ProcessID
	}
	if input.ConfigPath != nil {
		slot.ConfigPath = *input.ConfigPath
	}
	if input.AssignedTaskType != nil {
		slot.AssignedTaskType = *input.AssignedTaskType
	}
	if input.AssignedTaskID != nil {
		slot.AssignedTaskID = *input.AssignedTaskID
	}
	if input.SelectedNodeID != nil {
		slot.SelectedNodeID = *input.SelectedNodeID
	}
	if input.LastStartedAt != nil {
		slot.LastStartedAt = *input.LastStartedAt
	}
	if input.LastHeartbeatAt != nil {
		slot.LastHeartbeatAt = *input.LastHeartbeatAt
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_proxy_runtime_slots
		SET status = $2,
			mixed_port = $3,
			controller_port = $4,
			controller_secret_ciphertext = $5,
			process_id = $6,
			config_path = $7,
			assigned_task_type = $8,
			assigned_task_id = $9,
			selected_node_id = NULLIF($10, 0),
			last_started_at = $11,
			last_heartbeat_at = $12,
			updated_at = NOW()
		WHERE id = $1
		RETURNING `+slotColumns(),
		id, string(slot.Status), slot.MixedPort, slot.ControllerPort, secret, nullableInt(slot.ProcessID),
		slot.ConfigPath, slot.AssignedTaskType, slot.AssignedTaskID, slot.SelectedNodeID,
		nullableTime(slot.LastStartedAt), nullableTime(slot.LastHeartbeatAt),
	)
	return scanSlot(row)
}

func (r *SQLRepository) CreateAssignment(ctx context.Context, assignment *adminplusdomain.ProxyAssignment) (*adminplusdomain.ProxyAssignment, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_proxy_assignments (
			task_type, task_id, policy_id, slot_id, node_id, target_host,
			egress_ip, status, switch_count, started_at
		)
		VALUES ($1, $2, $3, $4, NULLIF($5, 0), $6, $7, $8, $9, $10)
		RETURNING id`,
		assignment.TaskType, assignment.TaskID, assignment.PolicyID, assignment.SlotID, assignment.NodeID,
		assignment.TargetHost, assignment.EgressIP, string(assignment.Status), assignment.SwitchCount, assignment.StartedAt,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.GetAssignment(ctx, id)
}

func (r *SQLRepository) GetAssignment(ctx context.Context, id int64) (*adminplusdomain.ProxyAssignment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+assignmentColumns()+`
		FROM admin_plus_proxy_assignments a
		LEFT JOIN admin_plus_proxy_runtime_slots s ON s.id = a.slot_id
		WHERE a.id = $1
	`, id)
	return scanAssignment(row)
}

func (r *SQLRepository) ListAssignments(ctx context.Context, filter AssignmentFilter) ([]*adminplusdomain.ProxyAssignment, error) {
	args := make([]any, 0, 4)
	where := []string{"1=1"}
	if filter.TaskType != "" {
		args = append(args, filter.TaskType)
		where = append(where, fmt.Sprintf("a.task_type = $%d", len(args)))
	}
	if filter.TaskID != "" {
		args = append(args, filter.TaskID)
		where = append(where, fmt.Sprintf("a.task_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, string(filter.Status))
		where = append(where, fmt.Sprintf("a.status = $%d", len(args)))
	}
	limit := normalizedLimit(filter.Limit, 1000)
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+assignmentColumns()+`
		FROM admin_plus_proxy_assignments a
		LEFT JOIN admin_plus_proxy_runtime_slots s ON s.id = a.slot_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAssignments(rows)
}

func (r *SQLRepository) UpdateAssignment(ctx context.Context, id int64, input UpdateAssignmentInput) (*adminplusdomain.ProxyAssignment, error) {
	assignment, err := r.GetAssignment(ctx, id)
	if err != nil {
		return nil, err
	}
	if input.NodeID != nil {
		assignment.NodeID = *input.NodeID
	}
	if input.EgressIP != nil {
		assignment.EgressIP = *input.EgressIP
	}
	if input.Status != nil {
		assignment.Status = *input.Status
	}
	if input.SwitchCount != nil {
		assignment.SwitchCount = *input.SwitchCount
	}
	if input.ErrorCode != nil {
		assignment.ErrorCode = *input.ErrorCode
	}
	if input.ErrorMessage != nil {
		assignment.ErrorMessage = *input.ErrorMessage
	}
	if input.ReleasedAt != nil {
		assignment.ReleasedAt = *input.ReleasedAt
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE admin_plus_proxy_assignments
		SET node_id = NULLIF($2, 0),
			egress_ip = $3,
			status = $4,
			switch_count = $5,
			error_code = $6,
			error_message = $7,
			released_at = $8
		WHERE id = $1`,
		id, assignment.NodeID, assignment.EgressIP, string(assignment.Status), assignment.SwitchCount,
		assignment.ErrorCode, assignment.ErrorMessage, nullableTime(assignment.ReleasedAt),
	)
	if err != nil {
		return nil, err
	}
	return r.GetAssignment(ctx, id)
}

func (r *SQLRepository) CountPoliciesUsingSubscription(ctx context.Context, subscriptionID int64) (int, error) {
	if r == nil || r.db == nil {
		return 0, internalError("proxy repository is not configured")
	}
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM admin_plus_proxy_policies
		WHERE enabled
		  AND EXISTS (
			SELECT 1
			FROM jsonb_array_elements_text(subscription_ids) AS value
			WHERE value::bigint = $1
		  )
	`, subscriptionID).Scan(&count)
	return count, err
}

func (r *SQLRepository) CountActiveAssignmentsForSubscription(ctx context.Context, subscriptionID int64) (int, error) {
	if r == nil || r.db == nil {
		return 0, internalError("proxy repository is not configured")
	}
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM admin_plus_proxy_assignments a
		JOIN admin_plus_proxy_nodes n ON n.id = a.node_id
		WHERE a.status = 'active' AND n.subscription_id = $1
	`, subscriptionID).Scan(&count)
	return count, err
}

func (r *SQLRepository) CountActiveAssignmentsForPolicy(ctx context.Context, policyID int64) (int, error) {
	if r == nil || r.db == nil {
		return 0, internalError("proxy repository is not configured")
	}
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM admin_plus_proxy_assignments
		WHERE status = 'active' AND policy_id = $1
	`, policyID).Scan(&count)
	return count, err
}

func (r *SQLRepository) CreateAuditEvent(ctx context.Context, event *adminplusdomain.ProxyAuditEvent) (*adminplusdomain.ProxyAuditEvent, error) {
	payload, _ := json.Marshal(nilSafeMap(event.Payload))
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_proxy_audit_events (
			event_type, actor_id, task_type, task_id, policy_id, slot_id,
			node_id, subscription_id, target_host, level, message, payload
		)
		VALUES ($1, NULLIF($2, 0), $3, $4, NULLIF($5, 0), NULLIF($6, 0),
			NULLIF($7, 0), NULLIF($8, 0), $9, $10, $11, $12::jsonb)
		RETURNING `+auditColumns(),
		event.EventType, event.ActorID, event.TaskType, event.TaskID, event.PolicyID, event.SlotID,
		event.NodeID, event.SubscriptionID, event.TargetHost, string(event.Level), event.Message, string(payload),
	)
	return scanAuditEvent(row)
}

func (r *SQLRepository) ListAuditEvents(ctx context.Context, filter AuditFilter) ([]*adminplusdomain.ProxyAuditEvent, error) {
	args := make([]any, 0, 6)
	where := []string{"1=1"}
	if filter.EventType != "" {
		args = append(args, filter.EventType)
		where = append(where, fmt.Sprintf("event_type = $%d", len(args)))
	}
	if filter.TaskType != "" {
		args = append(args, filter.TaskType)
		where = append(where, fmt.Sprintf("task_type = $%d", len(args)))
	}
	if filter.TaskID != "" {
		args = append(args, filter.TaskID)
		where = append(where, fmt.Sprintf("task_id = $%d", len(args)))
	}
	if filter.Level != "" {
		args = append(args, string(filter.Level))
		where = append(where, fmt.Sprintf("level = $%d", len(args)))
	}
	if filter.TargetHost != "" {
		args = append(args, "%"+canonicalHost(filter.TargetHost)+"%")
		where = append(where, fmt.Sprintf("target_host ILIKE $%d", len(args)))
	}
	limit := normalizedLimit(filter.Limit, 1000)
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+auditColumns()+`
		FROM admin_plus_proxy_audit_events
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY created_at DESC, id DESC
		LIMIT $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuditEvents(rows)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func subscriptionColumns() string {
	return `id, name, subscription_type, (url_ciphertext <> '') AS url_configured, url_hash,
		enabled, refresh_interval_seconds, last_refresh_status, last_refresh_error,
		active_config_version, node_count, COALESCE(created_by, 0), created_at,
		updated_at, last_refreshed_at`
}

func scanSubscription(row rowScanner) (*adminplusdomain.ProxySubscription, error) {
	var out adminplusdomain.ProxySubscription
	var typ string
	var status string
	var lastRefreshed sql.NullTime
	err := row.Scan(
		&out.ID, &out.Name, &typ, &out.URLConfigured, &out.URLHash,
		&out.Enabled, &out.RefreshIntervalSeconds, &status, &out.LastRefreshError,
		&out.ActiveConfigVersion, &out.NodeCount, &out.CreatedBy, &out.CreatedAt,
		&out.UpdatedAt, &lastRefreshed,
	)
	if err == sql.ErrNoRows {
		return nil, notFound("PROXY_SUBSCRIPTION_NOT_FOUND", "proxy subscription not found")
	}
	if err != nil {
		return nil, err
	}
	out.SubscriptionType = adminplusdomain.ProxySubscriptionType(typ)
	out.LastRefreshStatus = adminplusdomain.ProxyRefreshStatus(status)
	if lastRefreshed.Valid {
		out.LastRefreshedAt = &lastRefreshed.Time
	}
	return &out, nil
}

func scanSubscriptionWithSecret(row rowScanner) (*adminplusdomain.ProxySubscription, string, error) {
	var out adminplusdomain.ProxySubscription
	var typ string
	var status string
	var lastRefreshed sql.NullTime
	var secret string
	dest := []any{
		&out.ID, &out.Name, &typ, &out.URLConfigured, &out.URLHash,
		&out.Enabled, &out.RefreshIntervalSeconds, &status, &out.LastRefreshError,
		&out.ActiveConfigVersion, &out.NodeCount, &out.CreatedBy, &out.CreatedAt,
		&out.UpdatedAt, &lastRefreshed,
	}
	if err := row.Scan(append(dest, &secret)...); err != nil {
		if err == sql.ErrNoRows {
			return nil, "", notFound("PROXY_SUBSCRIPTION_NOT_FOUND", "proxy subscription not found")
		}
		return nil, "", err
	}
	out.SubscriptionType = adminplusdomain.ProxySubscriptionType(typ)
	out.LastRefreshStatus = adminplusdomain.ProxyRefreshStatus(status)
	if lastRefreshed.Valid {
		out.LastRefreshedAt = &lastRefreshed.Time
	}
	return &out, secret, nil
}

func scanSubscriptions(rows *sql.Rows) ([]*adminplusdomain.ProxySubscription, error) {
	out := []*adminplusdomain.ProxySubscription{}
	for rows.Next() {
		item, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func nodeColumns() string {
	return `id, subscription_id, config_version, node_key, display_name, protocol,
		region, server_hash, health_status, last_latency_ms, last_egress_ip,
		last_error_code, last_error_message, last_checked_at, disabled_reason,
		raw_metadata, created_at, updated_at`
}

func scanNode(row rowScanner) (*adminplusdomain.ProxyNode, error) {
	var out adminplusdomain.ProxyNode
	var health string
	var latency sql.NullInt64
	var checked sql.NullTime
	var raw []byte
	err := row.Scan(&out.ID, &out.SubscriptionID, &out.ConfigVersion, &out.NodeKey, &out.DisplayName, &out.Protocol,
		&out.Region, &out.ServerHash, &health, &latency, &out.LastEgressIP,
		&out.LastErrorCode, &out.LastErrorMessage, &checked, &out.DisabledReason,
		&raw, &out.CreatedAt, &out.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, notFound("PROXY_NODE_NOT_FOUND", "proxy node not found")
	}
	if err != nil {
		return nil, err
	}
	out.HealthStatus = adminplusdomain.ProxyNodeHealthStatus(health)
	if latency.Valid {
		v := int(latency.Int64)
		out.LastLatencyMS = &v
	}
	if checked.Valid {
		out.LastCheckedAt = &checked.Time
	}
	_ = json.Unmarshal(raw, &out.RawMetadata)
	return &out, nil
}

func scanNodes(rows *sql.Rows) ([]*adminplusdomain.ProxyNode, error) {
	out := []*adminplusdomain.ProxyNode{}
	for rows.Next() {
		item, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func policyColumns() string {
	return `id, name, enabled, subscription_ids, preferred_regions,
		max_concurrency, max_switches_per_task, connect_timeout_ms,
		request_timeout_ms, config, created_at, updated_at`
}

func scanPolicy(row rowScanner) (*adminplusdomain.ProxyPolicy, error) {
	var out adminplusdomain.ProxyPolicy
	var subscriptions []byte
	var regions []byte
	var configJSON []byte
	err := row.Scan(&out.ID, &out.Name, &out.Enabled, &subscriptions, &regions,
		&out.MaxConcurrency, &out.MaxSwitchesPerTask, &out.ConnectTimeoutMS,
		&out.RequestTimeoutMS, &configJSON, &out.CreatedAt, &out.UpdatedAt,
		&out.EnabledTargets, &out.HealthyNodesAvailable)
	if err == sql.ErrNoRows {
		return nil, notFound("PROXY_POLICY_NOT_FOUND", "proxy policy not found")
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(subscriptions, &out.SubscriptionIDs)
	_ = json.Unmarshal(regions, &out.PreferredRegions)
	_ = json.Unmarshal(configJSON, &out.Config)
	return &out, nil
}

func scanPolicies(rows *sql.Rows) ([]*adminplusdomain.ProxyPolicy, error) {
	out := []*adminplusdomain.ProxyPolicy{}
	for rows.Next() {
		item, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func targetColumns() string {
	return `id, policy_id, target_host, purpose, allowed_methods,
		rate_limit_per_minute, enabled, authorization_note, created_at, updated_at`
}

func scanTarget(row rowScanner) (*adminplusdomain.ProxyTargetPolicy, error) {
	var out adminplusdomain.ProxyTargetPolicy
	var purpose string
	var methods []byte
	err := row.Scan(&out.ID, &out.PolicyID, &out.TargetHost, &purpose, &methods,
		&out.RateLimitPerMinute, &out.Enabled, &out.AuthorizationNote, &out.CreatedAt, &out.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, notFound("PROXY_TARGET_POLICY_NOT_FOUND", "proxy target policy not found")
	}
	if err != nil {
		return nil, err
	}
	out.Purpose = adminplusdomain.ProxyTaskPurpose(purpose)
	_ = json.Unmarshal(methods, &out.AllowedMethods)
	return &out, nil
}

func scanTargets(rows *sql.Rows) ([]*adminplusdomain.ProxyTargetPolicy, error) {
	out := []*adminplusdomain.ProxyTargetPolicy{}
	for rows.Next() {
		item, err := scanTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func slotColumns() string {
	return `id, slot_key, status, mixed_port, controller_port,
		(controller_secret_ciphertext <> '') AS controller_secret_configured,
		process_id, config_path, assigned_task_type, assigned_task_id,
		COALESCE(selected_node_id, 0), last_started_at, last_heartbeat_at,
		created_at, updated_at`
}

func scanSlot(row rowScanner) (*adminplusdomain.ProxyRuntimeSlot, error) {
	var out adminplusdomain.ProxyRuntimeSlot
	var status string
	var processID sql.NullInt64
	var started sql.NullTime
	var heartbeat sql.NullTime
	err := row.Scan(
		&out.ID, &out.SlotKey, &status, &out.MixedPort, &out.ControllerPort,
		&out.ControllerSecretConfigured, &processID, &out.ConfigPath,
		&out.AssignedTaskType, &out.AssignedTaskID, &out.SelectedNodeID,
		&started, &heartbeat, &out.CreatedAt, &out.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, notFound("PROXY_RUNTIME_SLOT_NOT_FOUND", "proxy runtime slot not found")
	}
	if err != nil {
		return nil, err
	}
	out.Status = adminplusdomain.ProxyRuntimeSlotStatus(status)
	if processID.Valid {
		v := int(processID.Int64)
		out.ProcessID = &v
	}
	if started.Valid {
		out.LastStartedAt = &started.Time
	}
	if heartbeat.Valid {
		out.LastHeartbeatAt = &heartbeat.Time
	}
	return &out, nil
}

func scanSlotWithSecret(row rowScanner) (*adminplusdomain.ProxyRuntimeSlot, string, error) {
	var out adminplusdomain.ProxyRuntimeSlot
	var status string
	var processID sql.NullInt64
	var started sql.NullTime
	var heartbeat sql.NullTime
	var secret string
	dest := []any{
		&out.ID, &out.SlotKey, &status, &out.MixedPort, &out.ControllerPort,
		&out.ControllerSecretConfigured, &processID, &out.ConfigPath,
		&out.AssignedTaskType, &out.AssignedTaskID, &out.SelectedNodeID,
		&started, &heartbeat, &out.CreatedAt, &out.UpdatedAt,
	}
	err := row.Scan(append(dest, &secret)...)
	if err == sql.ErrNoRows {
		return nil, "", notFound("PROXY_RUNTIME_SLOT_NOT_FOUND", "proxy runtime slot not found")
	}
	if err != nil {
		return nil, "", err
	}
	out.Status = adminplusdomain.ProxyRuntimeSlotStatus(status)
	if processID.Valid {
		v := int(processID.Int64)
		out.ProcessID = &v
	}
	if started.Valid {
		out.LastStartedAt = &started.Time
	}
	if heartbeat.Valid {
		out.LastHeartbeatAt = &heartbeat.Time
	}
	return &out, secret, nil
}

func scanSlots(rows *sql.Rows) ([]*adminplusdomain.ProxyRuntimeSlot, error) {
	out := []*adminplusdomain.ProxyRuntimeSlot{}
	for rows.Next() {
		item, err := scanSlot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func assignmentColumns() string {
	return `a.id, a.task_type, a.task_id, a.policy_id, a.slot_id, COALESCE(a.node_id, 0),
		COALESCE(s.mixed_port, 0),
		a.target_host, a.egress_ip, a.status, a.switch_count, a.error_code,
		a.error_message, a.started_at, a.released_at, a.created_at`
}

func scanAssignment(row rowScanner) (*adminplusdomain.ProxyAssignment, error) {
	var out adminplusdomain.ProxyAssignment
	var status string
	var released sql.NullTime
	err := row.Scan(&out.ID, &out.TaskType, &out.TaskID, &out.PolicyID, &out.SlotID, &out.NodeID, &out.MixedPort,
		&out.TargetHost, &out.EgressIP, &status, &out.SwitchCount, &out.ErrorCode,
		&out.ErrorMessage, &out.StartedAt, &released, &out.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, notFound("PROXY_ASSIGNMENT_NOT_FOUND", "proxy assignment not found")
	}
	if err != nil {
		return nil, err
	}
	out.Status = adminplusdomain.ProxyAssignmentStatus(status)
	if released.Valid {
		out.ReleasedAt = &released.Time
	}
	return &out, nil
}

func scanAssignments(rows *sql.Rows) ([]*adminplusdomain.ProxyAssignment, error) {
	out := []*adminplusdomain.ProxyAssignment{}
	for rows.Next() {
		item, err := scanAssignment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func auditColumns() string {
	return `id, event_type, COALESCE(actor_id, 0), task_type, task_id,
		COALESCE(policy_id, 0), COALESCE(slot_id, 0), COALESCE(node_id, 0),
		COALESCE(subscription_id, 0), target_host, level, message, payload, created_at`
}

func scanAuditEvent(row rowScanner) (*adminplusdomain.ProxyAuditEvent, error) {
	var out adminplusdomain.ProxyAuditEvent
	var level string
	var payload []byte
	err := row.Scan(&out.ID, &out.EventType, &out.ActorID, &out.TaskType, &out.TaskID,
		&out.PolicyID, &out.SlotID, &out.NodeID, &out.SubscriptionID, &out.TargetHost,
		&level, &out.Message, &payload, &out.CreatedAt)
	if err != nil {
		return nil, err
	}
	out.Level = adminplusdomain.ProxyAuditLevel(level)
	_ = json.Unmarshal(payload, &out.Payload)
	return &out, nil
}

func scanAuditEvents(rows *sql.Rows) ([]*adminplusdomain.ProxyAuditEvent, error) {
	out := []*adminplusdomain.ProxyAuditEvent{}
	for rows.Next() {
		item, err := scanAuditEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nilSafeMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func normalizedLimit(limit int, fallback int) int {
	if limit <= 0 {
		return fallback
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}
