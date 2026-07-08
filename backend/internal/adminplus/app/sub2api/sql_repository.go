package sub2api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"sort"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
	"github.com/lib/pq"
)

type SQLRepository struct {
	db *sql.DB
}

const routingImpactRecentWindow = 24 * time.Hour

const maxRoutingSensitiveFieldValueLength = 4000

var routingSensitiveRedactExtraKeys = []string{
	"authorization",
	"api_key",
	"apikey",
	"key",
	"token",
	"secret",
	"cookie",
	"cookies",
	"set_cookie",
	"set-cookie",
	"password",
}

type localAccountOpsExec interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func NewSQLRepository(db ReadDB) *SQLRepository {
	return &SQLRepository{db: db.DB}
}

func (r *SQLRepository) ListLocalUsageLines(ctx context.Context, filter UsageFilter) ([]*adminplusdomain.LocalUsageLine, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	where, args := buildUsageWhere(filter)
	limitRef := addSQLArg(&args, filter.Limit)
	query := `
		SELECT
			ul.id,
			ul.account_id,
			COALESCE(a.name, ''),
			COALESCE(a.platform, ''),
			COALESCE(ul.request_id, ''),
			COALESCE(NULLIF(ul.requested_model, ''), NULLIF(ul.model, ''), ''),
			COALESCE(ul.input_tokens, 0)::BIGINT,
			COALESCE(ul.output_tokens, 0)::BIGINT,
			ROUND((COALESCE(ul.actual_cost, ul.total_cost, 0)::NUMERIC) * 100)::BIGINT,
			ul.created_at
		FROM usage_logs ul
		LEFT JOIN accounts a ON a.id = ul.account_id
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY ul.created_at DESC, ul.id DESC
		LIMIT ` + limitRef

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	items := make([]*adminplusdomain.LocalUsageLine, 0)
	for rows.Next() {
		var item adminplusdomain.LocalUsageLine
		if err := rows.Scan(
			&item.ID,
			&item.AccountID,
			&item.AccountName,
			&item.AccountPlatform,
			&item.ExternalRequestID,
			&item.Model,
			&item.InputTokens,
			&item.OutputTokens,
			&item.RevenueCents,
			&item.StartedAt,
		); err != nil {
			return nil, err
		}
		item.Currency = "USD"
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SQLRepository) ListLocalUsageSummaries(ctx context.Context, filter UsageFilter) ([]*adminplusdomain.LocalUsageSummary, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	where, args := buildUsageWhere(filter)
	limitRef := addSQLArg(&args, filter.Limit)
	query := `
		SELECT
			ul.account_id,
			COALESCE(a.name, ''),
			COALESCE(a.platform, ''),
			COALESCE(NULLIF(ul.requested_model, ''), NULLIF(ul.model, ''), ''),
			COUNT(*)::BIGINT AS request_count,
			COALESCE(SUM(ul.input_tokens), 0)::BIGINT,
			COALESCE(SUM(ul.output_tokens), 0)::BIGINT,
			ROUND((COALESCE(SUM(ul.actual_cost), 0)::NUMERIC) * 100)::BIGINT,
			ROUND((COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0)::NUMERIC) * 100)::BIGINT,
			ROUND((COALESCE(SUM(ul.total_cost), 0)::NUMERIC) * 100)::BIGINT,
			COALESCE(ROUND(AVG(ul.first_token_ms))::BIGINT, 0),
			COALESCE(ROUND(AVG(ul.duration_ms))::BIGINT, 0),
			MIN(ul.created_at),
			MAX(ul.created_at),
			MAX(ul.created_at) AS last_request_created_at
		FROM usage_logs ul
		LEFT JOIN accounts a ON a.id = ul.account_id
		WHERE ` + strings.Join(where, " AND ") + `
		GROUP BY ul.account_id, a.name, a.platform, COALESCE(NULLIF(ul.requested_model, ''), NULLIF(ul.model, ''), '')
		ORDER BY request_count DESC, last_request_created_at DESC
		LIMIT ` + limitRef

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	items := make([]*adminplusdomain.LocalUsageSummary, 0)
	for rows.Next() {
		var item adminplusdomain.LocalUsageSummary
		if err := rows.Scan(
			&item.AccountID,
			&item.AccountName,
			&item.AccountPlatform,
			&item.Model,
			&item.RequestCount,
			&item.InputTokens,
			&item.OutputTokens,
			&item.RevenueCents,
			&item.AccountCostCents,
			&item.OriginalCostCents,
			&item.AvgFirstTokenMs,
			&item.AvgTotalLatencyMs,
			&item.WindowStart,
			&item.WindowEnd,
			&item.LastRequestCreatedAt,
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

func (r *SQLRepository) ListLocalAccountUsageSummaries(ctx context.Context, filter UsageFilter) ([]*adminplusdomain.LocalAccountUsageSummary, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	where, args := buildUsageWhere(filter)
	limitRef := addSQLArg(&args, filter.Limit)
	query := `
		SELECT
			ul.account_id,
			COALESCE(a.name, ''),
			COALESCE(a.platform, ''),
			COUNT(*)::BIGINT AS request_count,
			COALESCE(SUM(ul.input_tokens), 0)::BIGINT,
			COALESCE(SUM(ul.output_tokens), 0)::BIGINT,
			COALESCE(SUM(ul.input_tokens), 0)::BIGINT + COALESCE(SUM(ul.output_tokens), 0)::BIGINT,
			ROUND((COALESCE(SUM(ul.actual_cost), 0)::NUMERIC) * 100)::BIGINT,
			ROUND((COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0)::NUMERIC) * 100)::BIGINT,
			ROUND((COALESCE(SUM(ul.total_cost), 0)::NUMERIC) * 100)::BIGINT,
			COALESCE(ROUND(AVG(ul.first_token_ms))::BIGINT, 0),
			COALESCE(ROUND(AVG(ul.duration_ms))::BIGINT, 0),
			MIN(ul.created_at),
			MAX(ul.created_at),
			MAX(ul.created_at) AS last_request_created_at
		FROM usage_logs ul
		LEFT JOIN accounts a ON a.id = ul.account_id
		WHERE ` + strings.Join(where, " AND ") + `
		GROUP BY ul.account_id, a.name, a.platform
		ORDER BY request_count DESC, last_request_created_at DESC
		LIMIT ` + limitRef

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	items := make([]*adminplusdomain.LocalAccountUsageSummary, 0)
	for rows.Next() {
		var item adminplusdomain.LocalAccountUsageSummary
		if err := rows.Scan(
			&item.AccountID,
			&item.AccountName,
			&item.AccountPlatform,
			&item.RequestCount,
			&item.InputTokens,
			&item.OutputTokens,
			&item.TotalTokens,
			&item.RevenueCents,
			&item.AccountCostCents,
			&item.OriginalCostCents,
			&item.AvgFirstTokenMs,
			&item.AvgTotalLatencyMs,
			&item.WindowStart,
			&item.WindowEnd,
			&item.LastRequestCreatedAt,
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

func (r *SQLRepository) ListLocalAccountOps(ctx context.Context, filter LocalAccountOpsFilter) ([]*adminplusdomain.LocalAccountOpsRow, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	where, args := buildLocalAccountOpsWhere(filter)
	limitRef := addSQLArg(&args, filter.Limit)
	query := `
		WITH local_groups AS (
			SELECT
				ag.account_id,
				COALESCE(array_agg(g.id ORDER BY ag.priority, g.id) FILTER (WHERE g.id IS NOT NULL), ARRAY[]::BIGINT[]) AS group_ids,
				COALESCE(array_agg(g.name ORDER BY ag.priority, ag.group_id) FILTER (WHERE g.name IS NOT NULL), ARRAY[]::TEXT[]) AS group_names
			FROM account_groups ag
			INNER JOIN groups g ON g.id = ag.group_id AND g.deleted_at IS NULL
			GROUP BY ag.account_id
		),
		supplier_key_counts AS (
			SELECT
				supplier_id,
				COUNT(*)::INT AS active_key_count
			FROM admin_plus_supplier_keys
			WHERE status IN ('provisioning', 'bound', 'manual_secret_required')
			GROUP BY supplier_id
		)
		SELECT
			a.id,
			COALESCE(a.name, ''),
			COALESCE(a.platform, ''),
			COALESCE(a.type, ''),
			COALESCE(a.status, ''),
			COALESCE(a.error_message, ''),
			COALESCE(a.schedulable, false),
			COALESCE(a.concurrency, 0),
			COALESCE(a.priority, 0),
			COALESCE(a.rate_multiplier, 1),
			a.rate_limited_at,
			a.rate_limit_reset_at,
			a.overload_until,
			a.temp_unschedulable_until,
			COALESCE(a.temp_unschedulable_reason, ''),
			a.updated_at,
			COALESCE(lg.group_ids, ARRAY[]::BIGINT[]),
			COALESCE(lg.group_names, ARRAY[]::TEXT[]),
			COALESCE(asa.id, 0),
			COALESCE(s.id, 0),
			COALESCE(s.name, ''),
			COALESCE(s.type, ''),
			COALESCE(s.runtime_status, ''),
			COALESCE(s.health_status, ''),
			COALESCE(asa.runtime_status, ''),
			COALESCE(asa.health_status, ''),
			COALESCE(asa.balance_threshold_cents, 0),
			COALESCE(asa.balance_cents, 0),
			COALESCE(asa.balance_currency, ''),
			COALESCE(asa.has_usable_balance, false),
			COALESCE(sg.id, 0),
			COALESCE(sg.external_group_id, ''),
			COALESCE(sg.name, ''),
			COALESCE(sg.provider_family, ''),
			COALESCE(sg.model_family, ''),
			COALESCE(sg.model_spec, ''),
			COALESCE(sg.status, ''),
			COALESCE(sg.effective_rate_multiplier, a.rate_multiplier, 1),
			COALESCE(sk.id, 0),
			COALESCE(sk.name, ''),
			COALESCE(sk.key_last4, ''),
			COALESCE(sk.status, ''),
			CASE
				WHEN s.id IS NULL THEN 'unknown'
				WHEN COALESCE(s.key_limit_policy, 'unknown') = 'unlimited' THEN 'available'
				WHEN COALESCE(s.key_limit_policy, 'unknown') = 'unsupported' THEN 'unsupported'
				WHEN COALESCE(s.key_limit_policy, 'unknown') = 'limited'
					AND COALESCE(s.key_limit_value, 0) > 0
					AND COALESCE(skc.active_key_count, 0) >= COALESCE(s.key_limit_value, 0) THEN 'exhausted'
				WHEN COALESCE(s.key_limit_policy, 'unknown') = 'limited'
					AND COALESCE(s.key_limit_value, 0) > 0
					AND COALESCE(skc.active_key_count, 0) >= COALESCE(s.key_limit_value, 0) - 1 THEN 'limited'
				WHEN COALESCE(s.key_limit_policy, 'unknown') = 'limited'
					AND COALESCE(s.key_limit_value, 0) > 0 THEN 'available'
				ELSE 'unknown'
			END AS key_capacity_status,
			COALESCE(lc.probe_status, 'untested'),
			COALESCE(lc.remote_status, ''),
			COALESCE(lc.recommended, false),
			COALESCE(lc.status_code, 0),
			COALESCE(lc.error_class, ''),
			COALESCE(lc.error_message, ''),
			lc.captured_at,
			CASE
				WHEN asa.id IS NULL THEN 'unbound'
				WHEN COALESCE(asa.has_usable_balance, false) THEN 'usable'
				WHEN COALESCE(asa.balance_cents, 0) <= COALESCE(asa.balance_threshold_cents, 0) THEN 'insufficient'
				ELSE 'unknown'
			END AS balance_status,
			CASE
				WHEN asa.id IS NULL THEN 'unbound'
				WHEN COALESCE(lss.drift_status, 'synced') = 'pending' THEN 'local_account_state_drift'
				WHEN COALESCE(s.runtime_status, '') = 'disabled' THEN 'supplier_disabled'
				WHEN COALESCE(asa.runtime_status, '') = 'disabled' THEN 'binding_disabled'
				WHEN sk.id IS NULL THEN 'missing_key'
				WHEN sk.local_sub2api_account_id > 0 AND sk.local_sub2api_account_id <> a.id THEN 'key_local_account_mismatch'
				WHEN sg.id IS NULL THEN 'missing_group'
				WHEN sg.status = 'missing' THEN 'group_missing'
				WHEN sg.status = 'disabled' THEN 'group_disabled'
				WHEN COALESCE(asa.local_account_name, '') <> COALESCE(a.name, '')
					OR COALESCE(asa.local_account_platform, '') <> COALESCE(a.platform, '')
					OR COALESCE(asa.local_account_type, '') <> COALESCE(a.type, '') THEN 'local_account_metadata_drift'
				ELSE 'synced'
			END AS drift_status,
			CASE
				WHEN asa.id IS NULL THEN NULL
				ELSE GREATEST(
					COALESCE(asa.updated_at, 'epoch'::TIMESTAMPTZ),
					COALESCE(sk.updated_at, 'epoch'::TIMESTAMPTZ),
					COALESCE(sg.updated_at, 'epoch'::TIMESTAMPTZ),
					COALESCE(lss.last_checked_at, 'epoch'::TIMESTAMPTZ)
				)
			END AS last_local_sync_at
		FROM accounts a
		LEFT JOIN local_groups lg ON lg.account_id = a.id
		LEFT JOIN admin_plus_supplier_accounts asa ON asa.local_sub2api_account_id = a.id
		LEFT JOIN admin_plus_suppliers s ON s.id = asa.supplier_id
		LEFT JOIN supplier_key_counts skc ON skc.supplier_id = s.id
		LEFT JOIN admin_plus_supplier_keys sk ON sk.id = asa.supplier_key_id AND sk.supplier_id = asa.supplier_id
		LEFT JOIN admin_plus_supplier_groups sg ON sg.id = sk.supplier_group_id AND sg.supplier_id = asa.supplier_id
		LEFT JOIN admin_plus_local_account_state_snapshots lss ON lss.local_sub2api_account_id = a.id
		LEFT JOIN LATERAL (
			SELECT c.*
			FROM admin_plus_supplier_channel_check_snapshots c
			WHERE asa.id IS NOT NULL
				AND c.supplier_id = asa.supplier_id
				AND (
					c.supplier_account_id = asa.id
					OR (
						c.local_sub2api_account_id = a.id
						AND (sg.id IS NULL OR c.supplier_group_id = sg.id)
					)
					OR (sg.id IS NOT NULL AND c.supplier_group_id = sg.id)
				)
			ORDER BY
				CASE
					WHEN c.supplier_account_id = asa.id THEN 0
					WHEN sg.id IS NOT NULL AND c.local_sub2api_account_id = a.id AND c.supplier_group_id = sg.id THEN 1
					WHEN c.local_sub2api_account_id = a.id THEN 2
					ELSE 3
				END,
				c.captured_at DESC,
				c.id DESC
			LIMIT 1
		) lc ON TRUE
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY a.id DESC, asa.id DESC NULLS LAST
		LIMIT ` + limitRef

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	items := make([]*adminplusdomain.LocalAccountOpsRow, 0)
	for rows.Next() {
		item, err := scanLocalAccountOpsRow(rows)
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

func (r *SQLRepository) PreviewLocalAccountOpsAction(ctx context.Context, input LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	return r.buildLocalAccountOpsActionPlan(ctx, r.db, input, true)
}

func (r *SQLRepository) ApplyLocalAccountOpsAction(ctx context.Context, input LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := r.syncLocalAccountState(ctx, tx, LocalAccountStateSyncInput{AccountIDs: input.AccountIDs, Limit: len(input.AccountIDs)}, false); err != nil {
		return nil, err
	}
	if pending, err := listPendingLocalAccountStateDrift(ctx, tx, input.AccountIDs); err != nil {
		return nil, err
	} else if len(pending) > 0 {
		return &adminplusdomain.LocalAccountOpsActionResult{
			Action:        input.Action,
			AccountIDs:    append([]int64(nil), input.AccountIDs...),
			GroupIDs:      append([]int64(nil), input.GroupIDs...),
			Blocked:       true,
			BlockedReason: "LOCAL_ACCOUNT_STATE_DRIFT_PENDING",
			Warnings:      []string{"检测到 Sub2API 原后台手工改动，请先同步并采纳或恢复本地状态"},
		}, nil
	}

	plan, err := r.buildLocalAccountOpsActionPlan(ctx, tx, input, false)
	if err != nil {
		return nil, err
	}
	if plan.Blocked && !input.AllowEmptyPool {
		return plan, nil
	}
	if err := r.applyLocalAccountOpsAction(ctx, tx, input, plan); err != nil {
		return nil, err
	}
	if err := acceptLocalAccountStateSnapshots(ctx, tx, input.AccountIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	plan.Blocked = false
	plan.BlockedReason = ""
	return plan, nil
}

func (r *SQLRepository) ListLocalGroups(ctx context.Context, limit int) ([]*LocalSub2APIGroup, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	if limit <= 0 {
		limit = 1000
	}
	if limit > 5000 {
		limit = 5000
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			g.id,
			COALESCE(g.name, ''),
			COALESCE(g.platform, ''),
			COALESCE(g.status, ''),
			COALESCE(g.rate_multiplier, 1)::DOUBLE PRECISION,
			COALESCE(g.is_exclusive, false),
			COUNT(DISTINCT a.id) FILTER (WHERE a.deleted_at IS NULL)::BIGINT AS total_accounts,
			COUNT(DISTINCT a.id) FILTER (
				WHERE a.deleted_at IS NULL
					AND a.status = 'active'
					AND COALESCE(a.schedulable, false)
					AND (a.expires_at IS NULL OR a.expires_at > NOW() OR COALESCE(a.auto_pause_on_expired, TRUE) = FALSE)
					AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at <= NOW())
					AND (a.overload_until IS NULL OR a.overload_until <= NOW())
					AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= NOW())
			)::BIGINT AS schedulable_accounts,
			COUNT(DISTINCT ak.id) FILTER (WHERE ak.status = 'active' AND ak.deleted_at IS NULL)::BIGINT AS active_api_key_count
		FROM groups g
		LEFT JOIN account_groups ag ON ag.group_id = g.id
		LEFT JOIN accounts a ON a.id = ag.account_id
		LEFT JOIN api_keys ak ON ak.group_id = g.id
		WHERE g.deleted_at IS NULL
		GROUP BY g.id, g.name, g.platform, g.status, g.rate_multiplier, g.is_exclusive
		ORDER BY LOWER(g.name), g.id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	items := make([]*LocalSub2APIGroup, 0)
	for rows.Next() {
		var item LocalSub2APIGroup
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Platform,
			&item.Status,
			&item.RateMultiplier,
			&item.IsExclusive,
			&item.TotalAccounts,
			&item.SchedulableAccounts,
			&item.ActiveAPIKeyCount,
		); err != nil {
			return nil, err
		}
		item.WouldEmptySchedulablePool = item.ActiveAPIKeyCount > 0 && item.SchedulableAccounts == 0
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SQLRepository) CreateRoutingRefillRun(ctx context.Context, run *RoutingRefillRun) (*RoutingRefillRun, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	if run == nil {
		return nil, infraerrors.New(http.StatusBadRequest, "ROUTING_REFILL_RUN_INVALID", "routing refill run is required")
	}
	requestSnapshot, err := json.Marshal(nonNilMap(run.RequestSnapshot))
	if err != nil {
		return nil, err
	}
	resultSnapshot, err := json.Marshal(nonNilMap(run.ResultSnapshot))
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_routing_refill_runs (
			run_id, sub2api_instance_id, local_group_id, local_group_name, platform, model_scope,
			trigger_type, dry_run, status, reason, skipped_reason,
			before_total_accounts, before_schedulable_accounts, before_active_api_key_count,
			after_total_accounts, after_schedulable_accounts, after_active_api_key_count,
			selected_supplier_id, selected_supplier_group_id, selected_supplier_key_id,
			selected_local_account_id, selected_effective_rate_multiplier,
			requested_by, error_code, error_message, request_snapshot, result_snapshot
		)
		VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14,
			$15, $16, $17,
			$18, $19, $20,
			$21, $22,
			$23, $24, $25, $26::jsonb, $27::jsonb
		)
		RETURNING
			id, run_id, sub2api_instance_id, local_group_id, local_group_name, platform, model_scope,
			trigger_type, dry_run, status, reason, skipped_reason,
			before_total_accounts, before_schedulable_accounts, before_active_api_key_count,
			after_total_accounts, after_schedulable_accounts, after_active_api_key_count,
			selected_supplier_id, selected_supplier_group_id, selected_supplier_key_id,
			selected_local_account_id, selected_effective_rate_multiplier,
			requested_by, error_code, error_message, request_snapshot, result_snapshot, created_at, updated_at
	`,
		run.RunID,
		firstNonEmpty(run.Sub2APIInstanceID, "local"),
		run.LocalGroupID,
		run.LocalGroupName,
		run.Platform,
		run.ModelScope,
		firstNonEmpty(run.TriggerType, "manual"),
		run.DryRun,
		firstNonEmpty(run.Status, "succeeded"),
		run.Reason,
		run.SkippedReason,
		run.BeforeTotalAccounts,
		run.BeforeSchedulableAccounts,
		run.BeforeActiveAPIKeyCount,
		run.AfterTotalAccounts,
		run.AfterSchedulableAccounts,
		run.AfterActiveAPIKeyCount,
		run.SelectedSupplierID,
		run.SelectedSupplierGroupID,
		run.SelectedSupplierKeyID,
		run.SelectedLocalAccountID,
		run.SelectedEffectiveRateMultiplier,
		run.RequestedBy,
		run.ErrorCode,
		run.ErrorMessage,
		string(requestSnapshot),
		string(resultSnapshot),
	)
	return scanRoutingRefillRun(row)
}

func (r *SQLRepository) ListRoutingRefillRuns(ctx context.Context, filter RoutingRefillRunFilter) ([]*RoutingRefillRun, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	where := []string{"1=1"}
	args := make([]any, 0, 3)
	if filter.LocalGroupID > 0 {
		where = append(where, "local_group_id = "+addSQLArg(&args, filter.LocalGroupID))
	}
	if strings.TrimSpace(filter.Status) != "" {
		where = append(where, "status = "+addSQLArg(&args, strings.TrimSpace(filter.Status)))
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	limitRef := addSQLArg(&args, limit)
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id, run_id, sub2api_instance_id, local_group_id, local_group_name, platform, model_scope,
			trigger_type, dry_run, status, reason, skipped_reason,
			before_total_accounts, before_schedulable_accounts, before_active_api_key_count,
			after_total_accounts, after_schedulable_accounts, after_active_api_key_count,
			selected_supplier_id, selected_supplier_group_id, selected_supplier_key_id,
			selected_local_account_id, selected_effective_rate_multiplier,
			requested_by, error_code, error_message, request_snapshot, result_snapshot, created_at, updated_at
		FROM admin_plus_routing_refill_runs
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY created_at DESC, id DESC
		LIMIT `+limitRef,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	items := make([]*RoutingRefillRun, 0)
	for rows.Next() {
		item, err := scanRoutingRefillRun(rows)
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

func (r *SQLRepository) TryRoutingRefillLock(ctx context.Context, groupID int64) (RoutingRefillUnlockFunc, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	if groupID <= 0 {
		return nil, false, infraerrors.New(http.StatusBadRequest, "ROUTING_GROUP_ID_INVALID", "invalid group id")
	}
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return nil, false, err
	}
	lockID := routingRefillLockID(groupID)
	var acquired bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired); err != nil {
		_ = conn.Close()
		return nil, false, err
	}
	if !acquired {
		_ = conn.Close()
		return nil, false, nil
	}
	return func() error {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		defer func() {
			_ = conn.Close()
		}()
		var released bool
		if err := conn.QueryRowContext(releaseCtx, "SELECT pg_advisory_unlock($1)", lockID).Scan(&released); err != nil {
			return err
		}
		return nil
	}, true, nil
}

func routingRefillLockID(groupID int64) int64 {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "admin-plus-routing-refill:%d", groupID)
	return int64(h.Sum64())
}

func (r *SQLRepository) GetGroupAvailability(ctx context.Context, groupID int64, platform string) (*RoutingGroupAvailability, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	if groupID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "ROUTING_GROUP_ID_INVALID", "invalid group id")
	}
	platform = strings.TrimSpace(platform)
	var item RoutingGroupAvailability
	err := r.db.QueryRowContext(ctx, `
		SELECT
			g.id,
			COALESCE(g.name, ''),
			COUNT(DISTINCT a.id) FILTER (
				WHERE a.deleted_at IS NULL
					AND ($2 = '' OR LOWER(COALESCE(a.platform, '')) = LOWER($2))
			)::BIGINT AS total_accounts,
			COUNT(DISTINCT a.id) FILTER (
				WHERE a.deleted_at IS NULL
					AND ($2 = '' OR LOWER(COALESCE(a.platform, '')) = LOWER($2))
					AND a.status = 'active'
					AND COALESCE(a.schedulable, false)
					AND (a.expires_at IS NULL OR a.expires_at > NOW() OR COALESCE(a.auto_pause_on_expired, TRUE) = FALSE)
					AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at <= NOW())
					AND (a.overload_until IS NULL OR a.overload_until <= NOW())
					AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= NOW())
			)::BIGINT AS schedulable_accounts,
			COUNT(DISTINCT ak.id) FILTER (WHERE ak.status = 'active' AND ak.deleted_at IS NULL)::BIGINT AS active_api_key_count
		FROM groups g
		LEFT JOIN account_groups ag ON ag.group_id = g.id
		LEFT JOIN accounts a ON a.id = ag.account_id
		LEFT JOIN api_keys ak ON ak.group_id = g.id
		WHERE g.deleted_at IS NULL AND g.id = $1
		GROUP BY g.id, g.name
	`, groupID, platform).Scan(&item.GroupID, &item.GroupName, &item.TotalAccounts, &item.SchedulableAccounts, &item.ActiveAPIKeyCount)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "ROUTING_GROUP_NOT_FOUND", "group not found")
	}
	if err != nil {
		return nil, err
	}
	item.Platform = platform
	item.WouldEmptySchedulablePool = item.ActiveAPIKeyCount > 0 && item.SchedulableAccounts == 0
	if item.ActiveAPIKeyCount > 0 {
		if err := r.applyRoutingGroupRecentImpact(ctx, groupID, &item); err != nil {
			return nil, err
		}
		if item.RecentErrorRequestCount > 0 {
			failures, truncated, err := r.listRoutingRecentFailureRequests(ctx, groupID, 5)
			if err != nil {
				return nil, err
			}
			item.RecentFailureRequests = failures
			item.RecentFailuresTruncated = truncated
		}
		keys, truncated, err := r.listRoutingImpactedAPIKeys(ctx, groupID, 20)
		if err != nil {
			return nil, err
		}
		item.ImpactedAPIKeys = keys
		item.ImpactedAPIKeysTruncated = truncated
	}
	return &item, nil
}

func (r *SQLRepository) applyRoutingGroupRecentImpact(ctx context.Context, groupID int64, item *RoutingGroupAvailability) error {
	if item == nil {
		return nil
	}
	windowSeconds := int64(routingImpactRecentWindow / time.Second)
	var lastRequestAt sql.NullTime
	var lastErrorAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		WITH usage_agg AS (
			SELECT
				COUNT(*)::BIGINT AS success_request_count,
				COALESCE(SUM(
					COALESCE(ul.input_tokens, 0)
					+ COALESCE(ul.output_tokens, 0)
					+ COALESCE(ul.cache_creation_tokens, 0)
					+ COALESCE(ul.cache_read_tokens, 0)
					+ COALESCE(ul.image_output_tokens, 0)
				), 0)::BIGINT AS token_count,
				MAX(ul.created_at) AS last_request_at
			FROM usage_logs ul
			WHERE ul.group_id = $1
				AND ul.created_at >= NOW() - ($2::BIGINT * INTERVAL '1 second')
		),
		error_agg AS (
			SELECT
				COUNT(*) FILTER (WHERE COALESCE(e.status_code, 0) >= 400)::BIGINT AS error_request_count,
				COUNT(*) FILTER (
					WHERE COALESCE(e.error_owner, '') = 'provider'
						AND NOT COALESCE(e.is_business_limited, false)
						AND COALESCE(e.upstream_status_code, e.status_code, 0) = 429
				)::BIGINT AS upstream_429_count,
				MAX(e.created_at) FILTER (WHERE COALESCE(e.status_code, 0) >= 400) AS last_error_at
			FROM ops_error_logs e
			WHERE e.group_id = $1
				AND e.created_at >= NOW() - ($2::BIGINT * INTERVAL '1 second')
				AND NOT COALESCE(e.is_count_tokens, false)
		)
		SELECT
			usage_agg.success_request_count,
			usage_agg.token_count,
			usage_agg.last_request_at,
			error_agg.error_request_count,
			error_agg.upstream_429_count,
			error_agg.last_error_at
		FROM usage_agg, error_agg
	`, groupID, windowSeconds).Scan(
		&item.RecentSuccessRequestCount,
		&item.RecentTokenCount,
		&lastRequestAt,
		&item.RecentErrorRequestCount,
		&item.RecentUpstream429Count,
		&lastErrorAt,
	)
	if err != nil {
		return err
	}
	item.RecentWindowSeconds = windowSeconds
	if lastRequestAt.Valid {
		value := lastRequestAt.Time
		item.RecentLastRequestAt = &value
	}
	if lastErrorAt.Valid {
		value := lastErrorAt.Time
		item.RecentLastErrorAt = &value
	}
	return nil
}

func (r *SQLRepository) ListRoutingImpactAPIKeys(ctx context.Context, filter RoutingImpactFilter) ([]RoutingImpactedAPIKey, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	if filter.LocalGroupID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "ROUTING_GROUP_ID_INVALID", "invalid group id")
	}
	items, _, err := r.listRoutingImpactedAPIKeys(ctx, filter.LocalGroupID, filter.Limit)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SQLRepository) ListRoutingImpactFailureRequests(ctx context.Context, filter RoutingImpactFilter) ([]RoutingFailureRequest, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	if filter.LocalGroupID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "ROUTING_GROUP_ID_INVALID", "invalid group id")
	}
	items, _, err := r.listRoutingRecentFailureRequests(ctx, filter.LocalGroupID, filter.Limit)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SQLRepository) GetRoutingFailureSensitiveDetail(ctx context.Context, input RoutingSensitiveFailureDetailInput) (*RoutingSensitiveFailureDetail, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	if input.FailureID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "ROUTING_FAILURE_ID_INVALID", "invalid failure id")
	}
	if input.LocalGroupID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "ROUTING_GROUP_ID_INVALID", "invalid group id")
	}
	raw := routingSensitiveFailureRawFields{}
	detail := &RoutingSensitiveFailureDetail{}
	err := r.db.QueryRowContext(ctx, `
		SELECT
			e.id,
			COALESCE(e.group_id, 0)::BIGINT,
			COALESCE(e.request_id, ''),
			COALESCE(e.api_key_id, 0)::BIGINT,
			COALESCE(ak.name, ''),
			CASE
				WHEN LENGTH(COALESCE(ak.key, '')) <= 12 THEN COALESCE(ak.key, '')
				ELSE CONCAT(LEFT(ak.key, 6), '...', RIGHT(ak.key, 4))
			END AS key_preview,
			COALESCE(e.user_id, 0)::BIGINT,
			COALESCE(e.account_id, 0)::BIGINT,
			COALESCE(e.model, ''),
			COALESCE(e.status_code, 0)::INT,
			COALESCE(e.upstream_status_code, 0)::INT,
			COALESCE(e.error_owner, ''),
			COALESCE(e.error_type, ''),
			e.created_at,
			COALESCE(e.error_message, ''),
			COALESCE(e.error_body, ''),
			COALESCE(e.upstream_error_message, ''),
			COALESCE(e.upstream_error_detail, ''),
			COALESCE(e.provider_error_code, ''),
			COALESCE(e.provider_error_type, ''),
			COALESCE(e.network_error_type, ''),
			COALESCE(e.error_source, ''),
			COALESCE(e.inbound_endpoint, ''),
			COALESCE(e.upstream_endpoint, ''),
			COALESCE(e.requested_model, ''),
			COALESCE(e.upstream_model, ''),
			COALESCE(e.retry_after_seconds, 0)::INT
		FROM ops_error_logs e
		LEFT JOIN api_keys ak ON ak.id = e.api_key_id
		WHERE e.id = $1
			AND e.group_id = $2
			AND COALESCE(e.status_code, 0) >= 400
			AND NOT COALESCE(e.is_count_tokens, false)
	`, input.FailureID, input.LocalGroupID).Scan(
		&detail.ID,
		&detail.LocalGroupID,
		&detail.RequestID,
		&detail.APIKeyID,
		&detail.APIKeyName,
		&detail.APIKeyPreview,
		&detail.UserID,
		&detail.AccountID,
		&detail.Model,
		&detail.StatusCode,
		&detail.UpstreamStatusCode,
		&detail.ErrorOwner,
		&detail.ErrorType,
		&detail.CreatedAt,
		&raw.ErrorMessage,
		&raw.ErrorBody,
		&raw.UpstreamErrorMessage,
		&raw.UpstreamErrorDetail,
		&raw.ProviderErrorCode,
		&raw.ProviderErrorType,
		&raw.NetworkErrorType,
		&raw.ErrorSource,
		&raw.InboundEndpoint,
		&raw.UpstreamEndpoint,
		&raw.RequestedModel,
		&raw.UpstreamModel,
		&raw.RetryAfterSeconds,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, infraerrors.New(http.StatusNotFound, "ROUTING_FAILURE_REQUEST_NOT_FOUND", "routing failure request not found")
		}
		return nil, err
	}
	detail.Fields = routingSensitiveFailureFields(input.Fields, raw)
	for _, field := range detail.Fields {
		if field.Available {
			detail.Available = true
			break
		}
	}
	if !detail.Available {
		detail.UnavailableReason = routingSensitiveUnavailableNotRecorded
	}
	return detail, nil
}

type routingSensitiveFailureRawFields struct {
	ErrorMessage         string
	ErrorBody            string
	UpstreamErrorMessage string
	UpstreamErrorDetail  string
	ProviderErrorCode    string
	ProviderErrorType    string
	NetworkErrorType     string
	ErrorSource          string
	InboundEndpoint      string
	UpstreamEndpoint     string
	RequestedModel       string
	UpstreamModel        string
	RetryAfterSeconds    int
}

func routingSensitiveFailureFields(fields []string, raw routingSensitiveFailureRawFields) []RoutingSensitiveFailureField {
	values := map[string]string{
		"error_message":          raw.ErrorMessage,
		"error_body":             raw.ErrorBody,
		"upstream_error_message": raw.UpstreamErrorMessage,
		"upstream_error_detail":  raw.UpstreamErrorDetail,
		"provider_error_code":    raw.ProviderErrorCode,
		"provider_error_type":    raw.ProviderErrorType,
		"network_error_type":     raw.NetworkErrorType,
		"error_source":           raw.ErrorSource,
		"inbound_endpoint":       raw.InboundEndpoint,
		"upstream_endpoint":      raw.UpstreamEndpoint,
		"requested_model":        raw.RequestedModel,
		"upstream_model":         raw.UpstreamModel,
	}
	if raw.RetryAfterSeconds > 0 {
		values["retry_after_seconds"] = fmt.Sprintf("%d", raw.RetryAfterSeconds)
	}
	out := make([]RoutingSensitiveFailureField, 0, len(fields))
	for _, name := range fields {
		value := strings.TrimSpace(values[name])
		if value == "" {
			out = append(out, RoutingSensitiveFailureField{
				Name:              name,
				UnavailableReason: routingSensitiveUnavailableNotRecorded,
			})
			continue
		}
		redacted, truncated := redactRoutingSensitiveFieldValue(value)
		out = append(out, RoutingSensitiveFailureField{
			Name:      name,
			Available: true,
			Value:     redacted,
			Redacted:  true,
			Truncated: truncated,
		})
	}
	return out
}

func redactRoutingSensitiveFieldValue(value string) (string, bool) {
	redacted := logredact.RedactText(value, routingSensitiveRedactExtraKeys...)
	if len(redacted) <= maxRoutingSensitiveFieldValueLength {
		return redacted, false
	}
	runes := []rune(redacted)
	if len(runes) <= maxRoutingSensitiveFieldValueLength {
		return redacted, false
	}
	return string(runes[:maxRoutingSensitiveFieldValueLength]), true
}

func (r *SQLRepository) listRoutingRecentFailureRequests(ctx context.Context, groupID int64, limit int) ([]RoutingFailureRequest, bool, error) {
	if limit <= 0 {
		limit = 5
	}
	windowSeconds := int64(routingImpactRecentWindow / time.Second)
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			e.id,
			COALESCE(e.request_id, ''),
			COALESCE(e.api_key_id, 0)::BIGINT,
			COALESCE(ak.name, ''),
			CASE
				WHEN LENGTH(COALESCE(ak.key, '')) <= 12 THEN COALESCE(ak.key, '')
				ELSE CONCAT(LEFT(ak.key, 6), '...', RIGHT(ak.key, 4))
			END AS key_preview,
			COALESCE(e.user_id, 0)::BIGINT,
			COALESCE(e.account_id, 0)::BIGINT,
			COALESCE(e.model, ''),
			COALESCE(e.status_code, 0)::INT,
			COALESCE(e.upstream_status_code, 0)::INT,
			COALESCE(e.error_owner, ''),
			COALESCE(e.error_type, ''),
			LEFT(COALESCE(NULLIF(e.upstream_error_message, ''), NULLIF(e.error_message, ''), ''), 240),
			e.created_at
		FROM ops_error_logs e
		LEFT JOIN api_keys ak ON ak.id = e.api_key_id
		WHERE e.group_id = $1
			AND e.created_at >= NOW() - ($2::BIGINT * INTERVAL '1 second')
			AND COALESCE(e.status_code, 0) >= 400
			AND NOT COALESCE(e.is_count_tokens, false)
		ORDER BY e.created_at DESC, e.id DESC
		LIMIT $3
	`, groupID, windowSeconds, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		_ = rows.Close()
	}()
	items := make([]RoutingFailureRequest, 0, limit)
	for rows.Next() {
		var item RoutingFailureRequest
		if err := rows.Scan(
			&item.ID,
			&item.RequestID,
			&item.APIKeyID,
			&item.APIKeyName,
			&item.APIKeyPreview,
			&item.UserID,
			&item.AccountID,
			&item.Model,
			&item.StatusCode,
			&item.UpstreamStatusCode,
			&item.ErrorOwner,
			&item.ErrorType,
			&item.ErrorMessage,
			&item.CreatedAt,
		); err != nil {
			return nil, false, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	truncated := len(items) > limit
	if truncated {
		items = items[:limit]
	}
	return items, truncated, nil
}

func (r *SQLRepository) listRoutingImpactedAPIKeys(ctx context.Context, groupID int64, limit int) ([]RoutingImpactedAPIKey, bool, error) {
	if limit <= 0 {
		limit = 20
	}
	windowSeconds := int64(routingImpactRecentWindow / time.Second)
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			ak.id,
			ak.user_id,
			COALESCE(ak.name, ''),
			CASE
				WHEN LENGTH(COALESCE(ak.key, '')) <= 12 THEN COALESCE(ak.key, '')
				ELSE CONCAT(LEFT(ak.key, 6), '...', RIGHT(ak.key, 4))
			END AS key_preview,
			COALESCE(ak.status, ''),
			ak.last_used_at,
			COALESCE(usage_recent.success_request_count, 0)::BIGINT,
			COALESCE(usage_recent.token_count, 0)::BIGINT,
			usage_recent.last_request_at,
			COALESCE(error_recent.error_request_count, 0)::BIGINT,
			COALESCE(error_recent.upstream_429_count, 0)::BIGINT,
			error_recent.last_error_at
		FROM api_keys ak
		LEFT JOIN LATERAL (
			SELECT
				COUNT(*)::BIGINT AS success_request_count,
				COALESCE(SUM(
					COALESCE(ul.input_tokens, 0)
					+ COALESCE(ul.output_tokens, 0)
					+ COALESCE(ul.cache_creation_tokens, 0)
					+ COALESCE(ul.cache_read_tokens, 0)
					+ COALESCE(ul.image_output_tokens, 0)
				), 0)::BIGINT AS token_count,
				MAX(ul.created_at) AS last_request_at
			FROM usage_logs ul
			WHERE ul.api_key_id = ak.id
				AND ul.group_id = $1
				AND ul.created_at >= NOW() - ($2::BIGINT * INTERVAL '1 second')
		) usage_recent ON TRUE
		LEFT JOIN LATERAL (
			SELECT
				COUNT(*) FILTER (WHERE COALESCE(e.status_code, 0) >= 400)::BIGINT AS error_request_count,
				COUNT(*) FILTER (
					WHERE COALESCE(e.error_owner, '') = 'provider'
						AND NOT COALESCE(e.is_business_limited, false)
						AND COALESCE(e.upstream_status_code, e.status_code, 0) = 429
				)::BIGINT AS upstream_429_count,
				MAX(e.created_at) FILTER (WHERE COALESCE(e.status_code, 0) >= 400) AS last_error_at
			FROM ops_error_logs e
			WHERE e.api_key_id = ak.id
				AND e.group_id = $1
				AND e.created_at >= NOW() - ($2::BIGINT * INTERVAL '1 second')
				AND NOT COALESCE(e.is_count_tokens, false)
		) error_recent ON TRUE
		WHERE ak.deleted_at IS NULL
			AND ak.status = 'active'
			AND ak.group_id = $1
		ORDER BY ak.last_used_at DESC NULLS LAST, ak.id DESC
		LIMIT $3
	`, groupID, windowSeconds, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		_ = rows.Close()
	}()
	items := make([]RoutingImpactedAPIKey, 0, limit)
	for rows.Next() {
		var item RoutingImpactedAPIKey
		var lastUsedAt sql.NullTime
		var recentLastRequestAt sql.NullTime
		var recentLastErrorAt sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Name,
			&item.KeyPreview,
			&item.Status,
			&lastUsedAt,
			&item.RecentSuccessRequestCount,
			&item.RecentTokenCount,
			&recentLastRequestAt,
			&item.RecentErrorRequestCount,
			&item.RecentUpstream429Count,
			&recentLastErrorAt,
		); err != nil {
			return nil, false, err
		}
		if lastUsedAt.Valid {
			value := lastUsedAt.Time
			item.LastUsedAt = &value
		}
		if recentLastRequestAt.Valid {
			value := recentLastRequestAt.Time
			item.RecentLastRequestAt = &value
		}
		if recentLastErrorAt.Valid {
			value := recentLastErrorAt.Time
			item.RecentLastErrorAt = &value
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	truncated := len(items) > limit
	if truncated {
		items = items[:limit]
	}
	return items, truncated, nil
}

func (r *SQLRepository) GetAccount(ctx context.Context, accountID int64) (*Sub2APIAccountSnapshot, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	if accountID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "ROUTING_ACCOUNT_ID_INVALID", "invalid account id")
	}
	var item Sub2APIAccountSnapshot
	var groupIDs pq.Int64Array
	err := r.db.QueryRowContext(ctx, `
		SELECT
			a.id,
			COALESCE(a.name, ''),
			COALESCE(a.platform, ''),
			COALESCE(a.type, ''),
			COALESCE(a.status, ''),
			COALESCE(a.schedulable, false),
			COALESCE(array_agg(ag.group_id ORDER BY ag.group_id) FILTER (WHERE ag.group_id IS NOT NULL), ARRAY[]::BIGINT[]) AS group_ids
		FROM accounts a
		LEFT JOIN account_groups ag ON ag.account_id = a.id
		WHERE a.deleted_at IS NULL AND a.id = $1
		GROUP BY a.id, a.name, a.platform, a.type, a.status, a.schedulable
	`, accountID).Scan(&item.AccountID, &item.Name, &item.Platform, &item.Type, &item.Status, &item.Schedulable, &groupIDs)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "ROUTING_ACCOUNT_NOT_FOUND", "account not found")
	}
	if err != nil {
		return nil, err
	}
	item.GroupIDs = append([]int64(nil), groupIDs...)
	return &item, nil
}

func (r *SQLRepository) EnsureAccountInGroup(ctx context.Context, accountID int64, groupID int64) (*Sub2APIAccountSnapshot, error) {
	if accountID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "ROUTING_ACCOUNT_ID_INVALID", "invalid account id")
	}
	if groupID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "ROUTING_GROUP_ID_INVALID", "invalid group id")
	}
	result, err := r.ApplyLocalAccountOpsAction(ctx, LocalAccountOpsActionInput{
		Action:         adminplusdomain.LocalAccountOpsActionAddToGroups,
		AccountIDs:     []int64{accountID},
		GroupIDs:       []int64{groupID},
		AllowEmptyPool: true,
	})
	if err != nil {
		return nil, err
	}
	if result != nil && result.Blocked {
		return nil, localAccountOpsBlockedError(result)
	}
	return r.GetAccount(ctx, accountID)
}

func (r *SQLRepository) SetAccountSchedulable(ctx context.Context, accountID int64, schedulable bool, reason string) (*Sub2APIAccountSnapshot, error) {
	if accountID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "ROUTING_ACCOUNT_ID_INVALID", "invalid account id")
	}
	result, err := r.ApplyLocalAccountOpsAction(ctx, LocalAccountOpsActionInput{
		Action:      adminplusdomain.LocalAccountOpsActionSetSchedulable,
		AccountIDs:  []int64{accountID},
		Schedulable: &schedulable,
		Reason:      strings.TrimSpace(reason),
	})
	if err != nil {
		return nil, err
	}
	if result != nil && result.Blocked {
		return nil, localAccountOpsBlockedError(result)
	}
	return r.GetAccount(ctx, accountID)
}

func (r *SQLRepository) HasSupplierUsageSince(ctx context.Context, supplierID int64, since time.Time) (bool, error) {
	if r == nil || r.db == nil {
		return false, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM usage_logs ul
			INNER JOIN admin_plus_supplier_accounts asa
				ON asa.local_sub2api_account_id = ul.account_id
			WHERE asa.supplier_id = $1
				AND ul.created_at >= $2
			LIMIT 1
		)
	`, supplierID, since.UTC()).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *SQLRepository) SyncLocalAccountState(ctx context.Context, input LocalAccountStateSyncInput) (*adminplusdomain.LocalAccountStateSyncResult, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := r.syncLocalAccountState(ctx, tx, input, true); err != nil {
		return nil, err
	}
	result, err := localAccountStateSyncResult(ctx, tx, input)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *SQLRepository) ResolveLocalAccountState(ctx context.Context, input LocalAccountStateResolutionInput) (*adminplusdomain.LocalAccountStateResolutionResult, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_READ_DB_NOT_CONFIGURED", "sub2api read database is not configured")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	syncInput := LocalAccountStateSyncInput{AccountIDs: input.AccountIDs, Limit: len(input.AccountIDs)}
	if err := r.syncLocalAccountState(ctx, tx, syncInput, true); err != nil {
		return nil, err
	}
	pendingAccountIDs, err := listPendingLocalAccountStateDrift(ctx, tx, input.AccountIDs)
	if err != nil {
		return nil, err
	}

	var resolvedAccounts int64
	var restoredAccounts int64
	warnings := make([]string, 0)
	if len(pendingAccountIDs) == 0 {
		warnings = append(warnings, "未发现待处理的本地状态变更")
	} else {
		switch input.Action {
		case adminplusdomain.LocalAccountStateResolutionAcceptObserved:
			resolvedAccounts, err = acceptObservedLocalAccountStateSnapshots(ctx, tx, input.AccountIDs)
			if err != nil {
				return nil, err
			}
			if err := markLocalAccountDriftEvents(ctx, tx, input.AccountIDs, "accepted"); err != nil {
				return nil, err
			}
		case adminplusdomain.LocalAccountStateResolutionRestoreAccepted:
			missingGroupIDs, err := listMissingAcceptedLocalGroups(ctx, tx, input.AccountIDs)
			if err != nil {
				return nil, err
			}
			if len(missingGroupIDs) > 0 {
				return nil, infraerrors.New(http.StatusConflict, "LOCAL_ACCOUNT_STATE_RESTORE_GROUPS_MISSING", fmt.Sprintf("accepted local groups are missing: %v", missingGroupIDs))
			}
			impactedGroupIDs, err := listLocalAccountStateRestoreGroupIDs(ctx, tx, input.AccountIDs)
			if err != nil {
				return nil, err
			}
			if err := restoreAcceptedLocalAccountState(ctx, tx, input.AccountIDs); err != nil {
				return nil, err
			}
			if err := enqueueLocalAccountStateRestoreSchedulerOutbox(ctx, tx, input, impactedGroupIDs); err != nil {
				return nil, err
			}
			if err := acceptLocalAccountStateSnapshots(ctx, tx, input.AccountIDs); err != nil {
				return nil, err
			}
			if err := markLocalAccountDriftEvents(ctx, tx, input.AccountIDs, "restored"); err != nil {
				return nil, err
			}
			restoredAccounts = int64(len(pendingAccountIDs))
			resolvedAccounts = restoredAccounts
		}
	}

	result, err := localAccountStateResolutionResult(ctx, tx, input, resolvedAccounts, restoredAccounts, warnings)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func buildUsageWhere(filter UsageFilter) ([]string, []any) {
	where := []string{"ul.created_at >= $1", "ul.created_at < $2"}
	args := []any{filter.From, filter.To}
	if filter.AccountID > 0 {
		where = append(where, "ul.account_id = "+addSQLArg(&args, filter.AccountID))
	}
	if strings.TrimSpace(filter.Model) != "" {
		where = append(where, "LOWER(COALESCE(NULLIF(ul.requested_model, ''), NULLIF(ul.model, ''), '')) = "+addSQLArg(&args, strings.ToLower(strings.TrimSpace(filter.Model))))
	}
	return where, args
}

func buildLocalAccountOpsWhere(filter LocalAccountOpsFilter) ([]string, []any) {
	where := []string{"a.deleted_at IS NULL"}
	args := make([]any, 0, 5)
	if strings.TrimSpace(filter.Query) != "" {
		query := strings.TrimSpace(filter.Query)
		like := "%" + strings.ToLower(query) + "%"
		likeRef := addSQLArg(&args, like)
		idRef := addSQLArg(&args, query)
		where = append(where, `(LOWER(COALESCE(a.name, '')) LIKE `+likeRef+`
			OR LOWER(COALESCE(a.platform, '')) LIKE `+likeRef+`
			OR LOWER(COALESCE(a.type, '')) LIKE `+likeRef+`
			OR LOWER(COALESCE(a.status, '')) LIKE `+likeRef+`
			OR CAST(a.id AS TEXT) = `+idRef+`
			OR LOWER(COALESCE(s.name, '')) LIKE `+likeRef+`
			OR LOWER(COALESCE(sg.name, '')) LIKE `+likeRef+`
			OR LOWER(COALESCE(sk.name, '')) LIKE `+likeRef+`
			OR EXISTS (
				SELECT 1
				FROM account_groups agq
				INNER JOIN groups gq ON gq.id = agq.group_id AND gq.deleted_at IS NULL
				WHERE agq.account_id = a.id AND LOWER(gq.name) LIKE `+likeRef+`
			))`)
	}
	if filter.SupplierID > 0 {
		where = append(where, "asa.supplier_id = "+addSQLArg(&args, filter.SupplierID))
	}
	if filter.SupplierGroupID > 0 {
		where = append(where, "sg.id = "+addSQLArg(&args, filter.SupplierGroupID))
	}
	if filter.LocalGroupID > 0 {
		where = append(where, `EXISTS (
			SELECT 1
			FROM account_groups agf
			WHERE agf.account_id = a.id AND agf.group_id = `+addSQLArg(&args, filter.LocalGroupID)+`
		)`)
	}
	if filter.MaxRateMultiplier > 0 {
		where = append(where, "COALESCE(sg.effective_rate_multiplier, a.rate_multiplier, 1) <= "+addSQLArg(&args, filter.MaxRateMultiplier))
	}
	if filter.BalanceStatus != "" {
		where = append(where, `(CASE
			WHEN asa.id IS NULL THEN 'unbound'
			WHEN COALESCE(asa.has_usable_balance, false) THEN 'usable'
			WHEN COALESCE(asa.balance_cents, 0) <= COALESCE(asa.balance_threshold_cents, 0) THEN 'insufficient'
			ELSE 'unknown'
		END) = `+addSQLArg(&args, filter.BalanceStatus))
	}
	if filter.ChannelCheckStatus != "" {
		where = append(where, "COALESCE(lc.probe_status, 'untested') = "+addSQLArg(&args, filter.ChannelCheckStatus))
	}
	if filter.Schedulable != nil {
		where = append(where, "COALESCE(a.schedulable, false) = "+addSQLArg(&args, *filter.Schedulable))
	}
	return where, args
}

func (r *SQLRepository) buildLocalAccountOpsActionPlan(ctx context.Context, exec localAccountOpsExec, input LocalAccountOpsActionInput, dryRun bool) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	existingAccountIDs, err := listExistingLocalAccountIDs(ctx, exec, input.AccountIDs)
	if err != nil {
		return nil, err
	}
	if len(existingAccountIDs) == 0 {
		return nil, infraerrors.New(http.StatusNotFound, "LOCAL_ACCOUNT_OPS_ACCOUNTS_NOT_FOUND", "local accounts not found")
	}
	if len(existingAccountIDs) != len(input.AccountIDs) {
		return nil, infraerrors.New(http.StatusBadRequest, "LOCAL_ACCOUNT_OPS_ACCOUNTS_PARTIAL_NOT_FOUND", "some local accounts were not found")
	}

	groupIDs, err := r.impactedLocalGroupIDs(ctx, exec, input)
	if err != nil {
		return nil, err
	}
	impacts, err := listLocalGroupImpacts(ctx, exec, groupIDs, input.Action, input.AccountIDs)
	if err != nil {
		return nil, err
	}

	result := &adminplusdomain.LocalAccountOpsActionResult{
		Action:       input.Action,
		DryRun:       dryRun,
		AccountIDs:   append([]int64(nil), input.AccountIDs...),
		GroupIDs:     append([]int64(nil), input.GroupIDs...),
		GroupImpacts: impacts,
		Warnings:     localAccountOpsWarnings(input, impacts),
	}
	for _, impact := range impacts {
		if impact.WouldEmptySchedulablePool {
			result.Blocked = true
			result.BlockedReason = "LOCAL_GROUP_SCHEDULABLE_POOL_WOULD_BE_EMPTY"
			break
		}
	}
	return result, nil
}

func (r *SQLRepository) impactedLocalGroupIDs(ctx context.Context, exec localAccountOpsExec, input LocalAccountOpsActionInput) ([]int64, error) {
	switch input.Action {
	case adminplusdomain.LocalAccountOpsActionAddToGroups, adminplusdomain.LocalAccountOpsActionRemoveFromGroups:
		groupIDs, err := listExistingLocalGroupIDs(ctx, exec, input.GroupIDs)
		if err != nil {
			return nil, err
		}
		if len(groupIDs) == 0 {
			return nil, infraerrors.New(http.StatusNotFound, "LOCAL_ACCOUNT_OPS_GROUPS_NOT_FOUND", "local groups not found")
		}
		if len(groupIDs) != len(input.GroupIDs) {
			return nil, infraerrors.New(http.StatusBadRequest, "LOCAL_ACCOUNT_OPS_GROUPS_PARTIAL_NOT_FOUND", "some local groups were not found")
		}
		return groupIDs, nil
	case adminplusdomain.LocalAccountOpsActionSetSchedulable:
		if input.Schedulable == nil || *input.Schedulable {
			return nil, nil
		}
		return listAccountLocalGroupIDs(ctx, exec, input.AccountIDs)
	default:
		return nil, nil
	}
}

func (r *SQLRepository) applyLocalAccountOpsAction(ctx context.Context, tx *sql.Tx, input LocalAccountOpsActionInput, result *adminplusdomain.LocalAccountOpsActionResult) error {
	switch input.Action {
	case adminplusdomain.LocalAccountOpsActionSetSchedulable:
		updated, err := updateLocalAccountsSchedulable(ctx, tx, input.AccountIDs, *input.Schedulable)
		if err != nil {
			return err
		}
		result.UpdatedAccounts = updated
	case adminplusdomain.LocalAccountOpsActionAddToGroups:
		added, err := addLocalAccountsToGroups(ctx, tx, input.AccountIDs, input.GroupIDs)
		if err != nil {
			return err
		}
		result.AddedBindings = added
	case adminplusdomain.LocalAccountOpsActionRemoveFromGroups:
		removed, err := removeLocalAccountsFromGroups(ctx, tx, input.AccountIDs, input.GroupIDs)
		if err != nil {
			return err
		}
		result.RemovedBindings = removed
	}
	if err := enqueueLocalAccountOpsSchedulerOutbox(ctx, tx, input); err != nil {
		return err
	}
	return nil
}

func listExistingLocalAccountIDs(ctx context.Context, exec localAccountOpsExec, accountIDs []int64) ([]int64, error) {
	rows, err := exec.QueryContext(ctx, `
		SELECT id
		FROM accounts
		WHERE deleted_at IS NULL AND id = ANY($1)
		ORDER BY id
	`, pq.Array(accountIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanInt64Rows(rows)
}

func listExistingLocalGroupIDs(ctx context.Context, exec localAccountOpsExec, groupIDs []int64) ([]int64, error) {
	rows, err := exec.QueryContext(ctx, `
		SELECT id
		FROM groups
		WHERE deleted_at IS NULL AND id = ANY($1)
		ORDER BY id
	`, pq.Array(groupIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanInt64Rows(rows)
}

func listAccountLocalGroupIDs(ctx context.Context, exec localAccountOpsExec, accountIDs []int64) ([]int64, error) {
	rows, err := exec.QueryContext(ctx, `
		SELECT DISTINCT ag.group_id
		FROM account_groups ag
		INNER JOIN groups g ON g.id = ag.group_id AND g.deleted_at IS NULL
		WHERE ag.account_id = ANY($1)
		ORDER BY ag.group_id
	`, pq.Array(accountIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanInt64Rows(rows)
}

func listLocalGroupImpacts(ctx context.Context, exec localAccountOpsExec, groupIDs []int64, action adminplusdomain.LocalAccountOpsAction, accountIDs []int64) ([]adminplusdomain.LocalAccountOpsGroupImpact, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	rows, err := exec.QueryContext(ctx, `
		SELECT
			g.id,
			g.name,
			COUNT(DISTINCT ak.id) FILTER (WHERE ak.status = 'active' AND ak.deleted_at IS NULL)::BIGINT AS active_api_key_count,
			COUNT(DISTINCT a.id) FILTER (
				WHERE a.deleted_at IS NULL
					AND a.status = 'active'
					AND COALESCE(a.schedulable, false)
					AND (a.expires_at IS NULL OR a.expires_at > NOW() OR COALESCE(a.auto_pause_on_expired, TRUE) = FALSE)
					AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at <= NOW())
					AND (a.overload_until IS NULL OR a.overload_until <= NOW())
					AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= NOW())
			)::BIGINT AS before_schedulable_accounts,
			COUNT(DISTINCT a.id) FILTER (
				WHERE a.deleted_at IS NULL
					AND a.status = 'active'
					AND CASE
						WHEN $3 = 'set_schedulable' AND a.id = ANY($2) THEN FALSE
						WHEN $3 = 'remove_from_groups' AND a.id = ANY($2) THEN FALSE
						ELSE COALESCE(a.schedulable, false)
					END
					AND (a.expires_at IS NULL OR a.expires_at > NOW() OR COALESCE(a.auto_pause_on_expired, TRUE) = FALSE)
					AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at <= NOW())
					AND (a.overload_until IS NULL OR a.overload_until <= NOW())
					AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= NOW())
			)::BIGINT AS after_schedulable_accounts
		FROM groups g
		LEFT JOIN account_groups ag ON ag.group_id = g.id
		LEFT JOIN accounts a ON a.id = ag.account_id
		LEFT JOIN api_keys ak ON ak.group_id = g.id
		WHERE g.deleted_at IS NULL AND g.id = ANY($1)
		GROUP BY g.id, g.name
		ORDER BY g.name, g.id
	`, pq.Array(groupIDs), pq.Array(accountIDs), string(action))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	impacts := make([]adminplusdomain.LocalAccountOpsGroupImpact, 0, len(groupIDs))
	for rows.Next() {
		var impact adminplusdomain.LocalAccountOpsGroupImpact
		if err := rows.Scan(
			&impact.GroupID,
			&impact.GroupName,
			&impact.ActiveAPIKeyCount,
			&impact.BeforeSchedulableAccounts,
			&impact.AfterSchedulableAccounts,
		); err != nil {
			return nil, err
		}
		impact.WouldEmptySchedulablePool = impact.ActiveAPIKeyCount > 0 &&
			impact.BeforeSchedulableAccounts > 0 &&
			impact.AfterSchedulableAccounts == 0
		impacts = append(impacts, impact)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return impacts, nil
}

func updateLocalAccountsSchedulable(ctx context.Context, exec localAccountOpsExec, accountIDs []int64, schedulable bool) (int64, error) {
	result, err := exec.ExecContext(ctx, `
		UPDATE accounts
		SET schedulable = $2, updated_at = NOW()
		WHERE deleted_at IS NULL AND id = ANY($1) AND COALESCE(schedulable, false) <> $2
	`, pq.Array(accountIDs), schedulable)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func addLocalAccountsToGroups(ctx context.Context, exec localAccountOpsExec, accountIDs []int64, groupIDs []int64) (int64, error) {
	result, err := exec.ExecContext(ctx, `
		INSERT INTO account_groups (account_id, group_id, priority, created_at)
		SELECT account_id, group_id, 50, NOW()
		FROM unnest($1::BIGINT[]) AS account_id
		CROSS JOIN unnest($2::BIGINT[]) AS group_id
		ON CONFLICT (account_id, group_id) DO NOTHING
	`, pq.Array(accountIDs), pq.Array(groupIDs))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func removeLocalAccountsFromGroups(ctx context.Context, exec localAccountOpsExec, accountIDs []int64, groupIDs []int64) (int64, error) {
	result, err := exec.ExecContext(ctx, `
		DELETE FROM account_groups
		WHERE account_id = ANY($1) AND group_id = ANY($2)
	`, pq.Array(accountIDs), pq.Array(groupIDs))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func enqueueLocalAccountOpsSchedulerOutbox(ctx context.Context, exec localAccountOpsExec, input LocalAccountOpsActionInput) error {
	payload := map[string]any{
		"account_ids": input.AccountIDs,
		"source":      "admin_plus.local_account_ops",
		"action":      string(input.Action),
	}
	if len(input.GroupIDs) > 0 {
		payload["group_ids"] = input.GroupIDs
	}
	if input.Schedulable != nil {
		payload["schedulable"] = *input.Schedulable
	}
	if input.RequestedBy > 0 {
		payload["requested_by"] = input.RequestedBy
	}
	if input.ActionRecommendationID > 0 {
		payload["action_id"] = input.ActionRecommendationID
	}
	if strings.TrimSpace(input.Reason) != "" {
		payload["reason"] = strings.TrimSpace(input.Reason)
	}
	if err := insertSchedulerOutbox(ctx, exec, service.SchedulerOutboxEventAccountBulkChanged, nil, nil, payload); err != nil {
		return err
	}
	for _, groupID := range input.GroupIDs {
		id := groupID
		if err := insertSchedulerOutbox(ctx, exec, service.SchedulerOutboxEventGroupChanged, nil, &id, payload); err != nil {
			return err
		}
	}
	if input.Action == adminplusdomain.LocalAccountOpsActionSetSchedulable && input.Schedulable != nil && !*input.Schedulable {
		groupIDs, err := listAccountLocalGroupIDs(ctx, exec, input.AccountIDs)
		if err != nil {
			return err
		}
		for _, groupID := range groupIDs {
			id := groupID
			if err := insertSchedulerOutbox(ctx, exec, service.SchedulerOutboxEventGroupChanged, nil, &id, payload); err != nil {
				return err
			}
		}
	}
	return nil
}

func insertSchedulerOutbox(ctx context.Context, exec localAccountOpsExec, eventType string, accountID *int64, groupID *int64, payload any) error {
	var payloadArg any
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		payloadArg = raw
	}
	_, err := exec.ExecContext(ctx, `
		INSERT INTO scheduler_outbox (event_type, account_id, group_id, payload)
		VALUES ($1, $2, $3, $4)
	`, eventType, accountID, groupID, payloadArg)
	return err
}

func (r *SQLRepository) syncLocalAccountState(ctx context.Context, exec localAccountOpsExec, input LocalAccountStateSyncInput, recordEvents bool) error {
	if input.Limit <= 0 {
		input.Limit = 500
	}
	_, err := exec.ExecContext(ctx, `
		WITH current_state AS (
			SELECT
				a.id AS account_id,
				COALESCE(a.name, '') AS account_name,
				COALESCE(a.platform, '') AS account_platform,
				COALESCE(a.type, '') AS account_type,
				COALESCE(a.schedulable, false) AS schedulable,
				COALESCE(array_agg(ag.group_id ORDER BY ag.group_id) FILTER (WHERE ag.group_id IS NOT NULL), ARRAY[]::BIGINT[]) AS group_ids
			FROM accounts a
			LEFT JOIN account_groups ag ON ag.account_id = a.id
			WHERE a.deleted_at IS NULL
				AND (COALESCE(array_length($1::BIGINT[], 1), 0) = 0 OR a.id = ANY($1))
			GROUP BY a.id, a.name, a.platform, a.type, a.schedulable
			ORDER BY a.id DESC
			LIMIT $2
		)
		INSERT INTO admin_plus_local_account_state_snapshots (
			local_sub2api_account_id,
			accepted_account_name, accepted_account_platform, accepted_account_type, accepted_schedulable, accepted_group_ids,
			observed_account_name, observed_account_platform, observed_account_type, observed_schedulable, observed_group_ids,
			drift_status, first_drift_detected_at, last_checked_at, accepted_at, updated_at
		)
		SELECT
			account_id,
			account_name, account_platform, account_type, schedulable, group_ids,
			account_name, account_platform, account_type, schedulable, group_ids,
			'synced', NULL, NOW(), NOW(), NOW()
		FROM current_state
		ON CONFLICT (local_sub2api_account_id) DO UPDATE
		SET observed_account_name = EXCLUDED.observed_account_name,
			observed_account_platform = EXCLUDED.observed_account_platform,
			observed_account_type = EXCLUDED.observed_account_type,
			observed_schedulable = EXCLUDED.observed_schedulable,
			observed_group_ids = EXCLUDED.observed_group_ids,
			drift_status = CASE
				WHEN admin_plus_local_account_state_snapshots.accepted_account_name <> EXCLUDED.observed_account_name
					OR admin_plus_local_account_state_snapshots.accepted_account_platform <> EXCLUDED.observed_account_platform
					OR admin_plus_local_account_state_snapshots.accepted_account_type <> EXCLUDED.observed_account_type
					OR admin_plus_local_account_state_snapshots.accepted_schedulable <> EXCLUDED.observed_schedulable
					OR admin_plus_local_account_state_snapshots.accepted_group_ids <> EXCLUDED.observed_group_ids
				THEN 'pending'
				ELSE 'synced'
			END,
			first_drift_detected_at = CASE
				WHEN admin_plus_local_account_state_snapshots.accepted_account_name <> EXCLUDED.observed_account_name
					OR admin_plus_local_account_state_snapshots.accepted_account_platform <> EXCLUDED.observed_account_platform
					OR admin_plus_local_account_state_snapshots.accepted_account_type <> EXCLUDED.observed_account_type
					OR admin_plus_local_account_state_snapshots.accepted_schedulable <> EXCLUDED.observed_schedulable
					OR admin_plus_local_account_state_snapshots.accepted_group_ids <> EXCLUDED.observed_group_ids
				THEN COALESCE(admin_plus_local_account_state_snapshots.first_drift_detected_at, NOW())
				ELSE NULL
			END,
			last_checked_at = NOW(),
			updated_at = NOW()
	`, pq.Array(input.AccountIDs), input.Limit)
	if err != nil {
		return err
	}
	if _, err := exec.ExecContext(ctx, `
		UPDATE admin_plus_supplier_accounts asa
		SET local_account_name = COALESCE(a.name, ''),
			local_account_platform = COALESCE(a.platform, ''),
			local_account_type = COALESCE(a.type, ''),
			updated_at = NOW()
		FROM accounts a
		WHERE a.deleted_at IS NULL
			AND asa.local_sub2api_account_id = a.id
			AND (COALESCE(array_length($1::BIGINT[], 1), 0) = 0 OR a.id = ANY($1))
	`, pq.Array(input.AccountIDs)); err != nil {
		return err
	}
	if recordEvents {
		return insertLocalAccountDriftEvents(ctx, exec, input.AccountIDs)
	}
	return nil
}

func insertLocalAccountDriftEvents(ctx context.Context, exec localAccountOpsExec, accountIDs []int64) error {
	_, err := exec.ExecContext(ctx, `
		INSERT INTO admin_plus_local_account_drift_events (
			local_sub2api_account_id, drift_type, old_snapshot, new_snapshot, status, detected_at
		)
		SELECT
			lss.local_sub2api_account_id,
			'local_state',
			jsonb_build_object(
				'name', lss.accepted_account_name,
				'platform', lss.accepted_account_platform,
				'type', lss.accepted_account_type,
				'schedulable', lss.accepted_schedulable,
				'group_ids', lss.accepted_group_ids
			),
			jsonb_build_object(
				'name', lss.observed_account_name,
				'platform', lss.observed_account_platform,
				'type', lss.observed_account_type,
				'schedulable', lss.observed_schedulable,
				'group_ids', lss.observed_group_ids
			),
			'detected',
			COALESCE(lss.first_drift_detected_at, NOW())
		FROM admin_plus_local_account_state_snapshots lss
		WHERE lss.drift_status = 'pending'
			AND (COALESCE(array_length($1::BIGINT[], 1), 0) = 0 OR lss.local_sub2api_account_id = ANY($1))
			AND NOT EXISTS (
				SELECT 1
				FROM admin_plus_local_account_drift_events e
				WHERE e.local_sub2api_account_id = lss.local_sub2api_account_id
					AND e.status = 'detected'
					AND e.old_snapshot = jsonb_build_object(
						'name', lss.accepted_account_name,
						'platform', lss.accepted_account_platform,
						'type', lss.accepted_account_type,
						'schedulable', lss.accepted_schedulable,
						'group_ids', lss.accepted_group_ids
					)
					AND e.new_snapshot = jsonb_build_object(
						'name', lss.observed_account_name,
						'platform', lss.observed_account_platform,
						'type', lss.observed_account_type,
						'schedulable', lss.observed_schedulable,
						'group_ids', lss.observed_group_ids
					)
			)
	`, pq.Array(accountIDs))
	return err
}

func listPendingLocalAccountStateDrift(ctx context.Context, exec localAccountOpsExec, accountIDs []int64) ([]int64, error) {
	rows, err := exec.QueryContext(ctx, `
		SELECT local_sub2api_account_id
		FROM admin_plus_local_account_state_snapshots
		WHERE drift_status = 'pending'
			AND local_sub2api_account_id = ANY($1)
		ORDER BY local_sub2api_account_id
	`, pq.Array(accountIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanInt64Rows(rows)
}

func acceptLocalAccountStateSnapshots(ctx context.Context, exec localAccountOpsExec, accountIDs []int64) error {
	_, err := exec.ExecContext(ctx, `
		WITH current_state AS (
			SELECT
				a.id AS account_id,
				COALESCE(a.name, '') AS account_name,
				COALESCE(a.platform, '') AS account_platform,
				COALESCE(a.type, '') AS account_type,
				COALESCE(a.schedulable, false) AS schedulable,
				COALESCE(array_agg(ag.group_id ORDER BY ag.group_id) FILTER (WHERE ag.group_id IS NOT NULL), ARRAY[]::BIGINT[]) AS group_ids
			FROM accounts a
			LEFT JOIN account_groups ag ON ag.account_id = a.id
			WHERE a.deleted_at IS NULL AND a.id = ANY($1)
			GROUP BY a.id, a.name, a.platform, a.type, a.schedulable
		)
		INSERT INTO admin_plus_local_account_state_snapshots (
			local_sub2api_account_id,
			accepted_account_name, accepted_account_platform, accepted_account_type, accepted_schedulable, accepted_group_ids,
			observed_account_name, observed_account_platform, observed_account_type, observed_schedulable, observed_group_ids,
			drift_status, first_drift_detected_at, last_checked_at, accepted_at, updated_at
		)
		SELECT
			account_id,
			account_name, account_platform, account_type, schedulable, group_ids,
			account_name, account_platform, account_type, schedulable, group_ids,
			'synced', NULL, NOW(), NOW(), NOW()
		FROM current_state
		ON CONFLICT (local_sub2api_account_id) DO UPDATE
		SET accepted_account_name = EXCLUDED.accepted_account_name,
			accepted_account_platform = EXCLUDED.accepted_account_platform,
			accepted_account_type = EXCLUDED.accepted_account_type,
			accepted_schedulable = EXCLUDED.accepted_schedulable,
			accepted_group_ids = EXCLUDED.accepted_group_ids,
			observed_account_name = EXCLUDED.observed_account_name,
			observed_account_platform = EXCLUDED.observed_account_platform,
			observed_account_type = EXCLUDED.observed_account_type,
			observed_schedulable = EXCLUDED.observed_schedulable,
			observed_group_ids = EXCLUDED.observed_group_ids,
			drift_status = 'synced',
			first_drift_detected_at = NULL,
			last_checked_at = NOW(),
			accepted_at = NOW(),
			updated_at = NOW()
	`, pq.Array(accountIDs))
	return err
}

func acceptObservedLocalAccountStateSnapshots(ctx context.Context, exec localAccountOpsExec, accountIDs []int64) (int64, error) {
	result, err := exec.ExecContext(ctx, `
		UPDATE admin_plus_local_account_state_snapshots
		SET accepted_account_name = observed_account_name,
			accepted_account_platform = observed_account_platform,
			accepted_account_type = observed_account_type,
			accepted_schedulable = observed_schedulable,
			accepted_group_ids = observed_group_ids,
			drift_status = 'synced',
			first_drift_detected_at = NULL,
			accepted_at = NOW(),
			updated_at = NOW()
		WHERE drift_status = 'pending'
			AND local_sub2api_account_id = ANY($1)
	`, pq.Array(accountIDs))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func markLocalAccountDriftEvents(ctx context.Context, exec localAccountOpsExec, accountIDs []int64, status string) error {
	_, err := exec.ExecContext(ctx, `
		UPDATE admin_plus_local_account_drift_events
		SET status = $2,
			resolved_at = NOW()
		WHERE status = 'detected'
			AND local_sub2api_account_id = ANY($1)
	`, pq.Array(accountIDs), status)
	return err
}

func listMissingAcceptedLocalGroups(ctx context.Context, exec localAccountOpsExec, accountIDs []int64) ([]int64, error) {
	rows, err := exec.QueryContext(ctx, `
		SELECT DISTINCT accepted_group_id
		FROM admin_plus_local_account_state_snapshots lss
		CROSS JOIN LATERAL unnest(lss.accepted_group_ids) AS accepted_group_id
		LEFT JOIN groups g ON g.id = accepted_group_id AND g.deleted_at IS NULL
		WHERE lss.drift_status = 'pending'
			AND lss.local_sub2api_account_id = ANY($1)
			AND g.id IS NULL
		ORDER BY accepted_group_id
	`, pq.Array(accountIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanInt64Rows(rows)
}

func listLocalAccountStateRestoreGroupIDs(ctx context.Context, exec localAccountOpsExec, accountIDs []int64) ([]int64, error) {
	rows, err := exec.QueryContext(ctx, `
		SELECT DISTINCT group_id
		FROM (
			SELECT unnest(lss.accepted_group_ids || lss.observed_group_ids) AS group_id
			FROM admin_plus_local_account_state_snapshots lss
			WHERE lss.drift_status = 'pending'
				AND lss.local_sub2api_account_id = ANY($1)
		) restored_groups
		ORDER BY group_id
	`, pq.Array(accountIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanInt64Rows(rows)
}

func restoreAcceptedLocalAccountState(ctx context.Context, exec localAccountOpsExec, accountIDs []int64) error {
	if _, err := exec.ExecContext(ctx, `
		UPDATE accounts a
		SET name = lss.accepted_account_name,
			platform = lss.accepted_account_platform,
			type = lss.accepted_account_type,
			schedulable = lss.accepted_schedulable,
			updated_at = NOW()
		FROM admin_plus_local_account_state_snapshots lss
		WHERE a.deleted_at IS NULL
			AND a.id = lss.local_sub2api_account_id
			AND lss.drift_status = 'pending'
			AND lss.local_sub2api_account_id = ANY($1)
	`, pq.Array(accountIDs)); err != nil {
		return err
	}
	if _, err := exec.ExecContext(ctx, `
		DELETE FROM account_groups ag
		USING admin_plus_local_account_state_snapshots lss
		WHERE ag.account_id = lss.local_sub2api_account_id
			AND lss.drift_status = 'pending'
			AND lss.local_sub2api_account_id = ANY($1)
			AND NOT (ag.group_id = ANY(lss.accepted_group_ids))
	`, pq.Array(accountIDs)); err != nil {
		return err
	}
	_, err := exec.ExecContext(ctx, `
		INSERT INTO account_groups (account_id, group_id, priority, created_at)
		SELECT lss.local_sub2api_account_id, accepted_group_id, 50, NOW()
		FROM admin_plus_local_account_state_snapshots lss
		CROSS JOIN LATERAL unnest(lss.accepted_group_ids) AS accepted_group_id
		INNER JOIN groups g ON g.id = accepted_group_id AND g.deleted_at IS NULL
		WHERE lss.drift_status = 'pending'
			AND lss.local_sub2api_account_id = ANY($1)
		ON CONFLICT (account_id, group_id) DO NOTHING
	`, pq.Array(accountIDs))
	return err
}

func enqueueLocalAccountStateRestoreSchedulerOutbox(ctx context.Context, exec localAccountOpsExec, input LocalAccountStateResolutionInput, groupIDs []int64) error {
	payload := map[string]any{
		"account_ids": input.AccountIDs,
		"source":      "admin_plus.local_account_state",
		"action":      string(input.Action),
	}
	if input.RequestedBy > 0 {
		payload["requested_by"] = input.RequestedBy
	}
	if err := insertSchedulerOutbox(ctx, exec, service.SchedulerOutboxEventAccountBulkChanged, nil, nil, payload); err != nil {
		return err
	}
	for _, groupID := range groupIDs {
		id := groupID
		if err := insertSchedulerOutbox(ctx, exec, service.SchedulerOutboxEventGroupChanged, nil, &id, payload); err != nil {
			return err
		}
	}
	return nil
}

func localAccountStateResolutionResult(ctx context.Context, exec localAccountOpsExec, input LocalAccountStateResolutionInput, resolvedAccounts int64, restoredAccounts int64, warnings []string) (*adminplusdomain.LocalAccountStateResolutionResult, error) {
	syncResult, err := localAccountStateSyncResult(ctx, exec, LocalAccountStateSyncInput{
		AccountIDs: input.AccountIDs,
		Limit:      len(input.AccountIDs),
	})
	if err != nil {
		return nil, err
	}
	return &adminplusdomain.LocalAccountStateResolutionResult{
		Action:               input.Action,
		AccountIDs:           append([]int64(nil), input.AccountIDs...),
		ResolvedAccounts:     resolvedAccounts,
		RestoredAccounts:     restoredAccounts,
		PendingDriftAccounts: syncResult.PendingDriftAccounts,
		Items:                syncResult.Items,
		Warnings:             warnings,
	}, nil
}

func localAccountStateSyncResult(ctx context.Context, exec localAccountOpsExec, input LocalAccountStateSyncInput) (*adminplusdomain.LocalAccountStateSyncResult, error) {
	rows, err := exec.QueryContext(ctx, `
		SELECT
			lss.local_sub2api_account_id,
			COALESCE(a.name, ''),
			lss.accepted_account_name,
			lss.accepted_account_platform,
			lss.accepted_account_type,
			lss.accepted_schedulable,
			lss.accepted_group_ids,
			lss.observed_account_name,
			lss.observed_account_platform,
			lss.observed_account_type,
			lss.observed_schedulable,
			lss.observed_group_ids,
			CASE
				WHEN lss.accepted_account_name <> lss.observed_account_name THEN ARRAY['name']::TEXT[] ELSE ARRAY[]::TEXT[] END
				|| CASE WHEN lss.accepted_account_platform <> lss.observed_account_platform THEN ARRAY['platform']::TEXT[] ELSE ARRAY[]::TEXT[] END
				|| CASE WHEN lss.accepted_account_type <> lss.observed_account_type THEN ARRAY['type']::TEXT[] ELSE ARRAY[]::TEXT[] END
				|| CASE WHEN lss.accepted_schedulable <> lss.observed_schedulable THEN ARRAY['schedulable']::TEXT[] ELSE ARRAY[]::TEXT[] END
				|| CASE WHEN lss.accepted_group_ids <> lss.observed_group_ids THEN ARRAY['groups']::TEXT[] ELSE ARRAY[]::TEXT[] END AS drift_fields,
			lss.first_drift_detected_at,
			lss.last_checked_at,
			lss.drift_status
		FROM admin_plus_local_account_state_snapshots lss
		LEFT JOIN accounts a ON a.id = lss.local_sub2api_account_id
		WHERE (COALESCE(array_length($1::BIGINT[], 1), 0) = 0 OR lss.local_sub2api_account_id = ANY($1))
		ORDER BY CASE WHEN lss.drift_status = 'pending' THEN 0 ELSE 1 END, lss.updated_at DESC, lss.local_sub2api_account_id DESC
		LIMIT $2
	`, pq.Array(input.AccountIDs), input.Limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := &adminplusdomain.LocalAccountStateSyncResult{}
	for rows.Next() {
		var item adminplusdomain.LocalAccountStateDriftSummary
		var acceptedGroupIDs pq.Int64Array
		var observedGroupIDs pq.Int64Array
		var driftFields pq.StringArray
		var firstDetectedAt sql.NullTime
		var driftStatus string
		if err := rows.Scan(
			&item.LocalSub2APIAccountID,
			&item.AccountName,
			&item.Accepted.Name,
			&item.Accepted.Platform,
			&item.Accepted.Type,
			&item.Accepted.Schedulable,
			&acceptedGroupIDs,
			&item.Observed.Name,
			&item.Observed.Platform,
			&item.Observed.Type,
			&item.Observed.Schedulable,
			&observedGroupIDs,
			&driftFields,
			&firstDetectedAt,
			&item.LastCheckedAt,
			&driftStatus,
		); err != nil {
			return nil, err
		}
		result.CheckedAccounts++
		if driftStatus == "pending" {
			result.DriftedAccounts++
			result.PendingDriftAccounts++
			item.Accepted.GroupIDs = append([]int64(nil), acceptedGroupIDs...)
			item.Observed.GroupIDs = append([]int64(nil), observedGroupIDs...)
			item.DriftFields = append([]string(nil), driftFields...)
			if firstDetectedAt.Valid {
				item.FirstDetectedAt = &firstDetectedAt.Time
			}
			result.Items = append(result.Items, item)
		} else {
			result.SyncedAccounts++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func localAccountOpsWarnings(input LocalAccountOpsActionInput, impacts []adminplusdomain.LocalAccountOpsGroupImpact) []string {
	warnings := make([]string, 0)
	for _, impact := range impacts {
		if impact.ActiveAPIKeyCount > 0 && impact.AfterSchedulableAccounts == 0 {
			warnings = append(warnings, fmt.Sprintf("本地分组 %s 将没有可调度账号，但仍有 %d 个启用 API Key", impact.GroupName, impact.ActiveAPIKeyCount))
		}
	}
	if input.Action == adminplusdomain.LocalAccountOpsActionRemoveFromGroups && len(impacts) > 0 {
		warnings = append(warnings, "移出本地分组会影响绑定该分组的用户 API Key 调度池")
	}
	return warnings
}

func localAccountOpsBlockedError(result *adminplusdomain.LocalAccountOpsActionResult) error {
	code := "LOCAL_ACCOUNT_OPS_BLOCKED"
	if result != nil && strings.TrimSpace(result.BlockedReason) != "" {
		code = result.BlockedReason
	}
	return infraerrors.New(http.StatusConflict, code, "local account operation blocked")
}

func scanInt64Rows(rows *sql.Rows) ([]int64, error) {
	out := make([]int64, 0)
	for rows.Next() {
		var value int64
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func scanLocalAccountOpsRow(scanner interface{ Scan(dest ...any) error }) (*adminplusdomain.LocalAccountOpsRow, error) {
	var item adminplusdomain.LocalAccountOpsRow
	var groupIDs pq.Int64Array
	var groupNames pq.StringArray
	var rateLimitedAt sql.NullTime
	var rateLimitResetAt sql.NullTime
	var overloadUntil sql.NullTime
	var tempUnschedulableUntil sql.NullTime
	var lastChannelCheckAt sql.NullTime
	var lastLocalSyncAt sql.NullTime
	if err := scanner.Scan(
		&item.LocalSub2APIAccountID,
		&item.LocalAccountName,
		&item.LocalAccountPlatform,
		&item.LocalAccountType,
		&item.LocalAccountStatus,
		&item.LocalAccountErrorMessage,
		&item.LocalAccountSchedulable,
		&item.LocalAccountConcurrency,
		&item.LocalAccountPriority,
		&item.LocalAccountRateMultiplier,
		&rateLimitedAt,
		&rateLimitResetAt,
		&overloadUntil,
		&tempUnschedulableUntil,
		&item.LocalAccountTempUnschedReason,
		&item.LocalAccountUpdatedAt,
		&groupIDs,
		&groupNames,
		&item.SupplierAccountID,
		&item.SupplierID,
		&item.SupplierName,
		&item.SupplierType,
		&item.SupplierRuntimeStatus,
		&item.SupplierHealthStatus,
		&item.SupplierAccountRuntimeStatus,
		&item.SupplierAccountHealthStatus,
		&item.BalanceThresholdCents,
		&item.BalanceCents,
		&item.BalanceCurrency,
		&item.HasUsableBalance,
		&item.SupplierGroupID,
		&item.SupplierExternalGroupID,
		&item.SupplierGroupName,
		&item.SupplierGroupProvider,
		&item.SupplierGroupModelFamily,
		&item.SupplierGroupModelSpec,
		&item.SupplierGroupStatus,
		&item.EffectiveRateMultiplier,
		&item.SupplierKeyID,
		&item.SupplierKeyName,
		&item.SupplierKeyLast4,
		&item.SupplierKeyStatus,
		&item.KeyCapacityStatus,
		&item.ChannelCheckStatus,
		&item.ChannelRemoteStatus,
		&item.ChannelRecommended,
		&item.ChannelStatusCode,
		&item.ChannelErrorClass,
		&item.ChannelErrorMessage,
		&lastChannelCheckAt,
		&item.BalanceStatus,
		&item.DriftStatus,
		&lastLocalSyncAt,
	); err != nil {
		return nil, err
	}
	item.LocalAccountGroupIDs = append([]int64(nil), groupIDs...)
	item.LocalAccountGroupNames = append([]string(nil), groupNames...)
	if rateLimitedAt.Valid {
		item.LocalAccountRateLimitedAt = &rateLimitedAt.Time
	}
	if rateLimitResetAt.Valid {
		item.LocalAccountRateLimitResetAt = &rateLimitResetAt.Time
	}
	if overloadUntil.Valid {
		item.LocalAccountOverloadUntil = &overloadUntil.Time
	}
	if tempUnschedulableUntil.Valid {
		item.LocalAccountTempUnschedAt = &tempUnschedulableUntil.Time
	}
	if lastChannelCheckAt.Valid {
		item.LastChannelCheckAt = &lastChannelCheckAt.Time
	}
	if lastLocalSyncAt.Valid {
		item.LastLocalSyncAt = &lastLocalSyncAt.Time
	}
	if item.BalanceCurrency == "" {
		item.BalanceCurrency = "USD"
	}
	if item.ChannelCheckStatus == "" {
		item.ChannelCheckStatus = "untested"
	}
	if item.DriftStatus == "" {
		item.DriftStatus = "unknown"
	}
	if item.BalanceStatus == "" {
		item.BalanceStatus = "unknown"
	}
	return &item, nil
}

func scanRoutingRefillRun(scanner interface{ Scan(dest ...any) error }) (*RoutingRefillRun, error) {
	var item RoutingRefillRun
	var requestSnapshot []byte
	var resultSnapshot []byte
	if err := scanner.Scan(
		&item.ID,
		&item.RunID,
		&item.Sub2APIInstanceID,
		&item.LocalGroupID,
		&item.LocalGroupName,
		&item.Platform,
		&item.ModelScope,
		&item.TriggerType,
		&item.DryRun,
		&item.Status,
		&item.Reason,
		&item.SkippedReason,
		&item.BeforeTotalAccounts,
		&item.BeforeSchedulableAccounts,
		&item.BeforeActiveAPIKeyCount,
		&item.AfterTotalAccounts,
		&item.AfterSchedulableAccounts,
		&item.AfterActiveAPIKeyCount,
		&item.SelectedSupplierID,
		&item.SelectedSupplierGroupID,
		&item.SelectedSupplierKeyID,
		&item.SelectedLocalAccountID,
		&item.SelectedEffectiveRateMultiplier,
		&item.RequestedBy,
		&item.ErrorCode,
		&item.ErrorMessage,
		&requestSnapshot,
		&resultSnapshot,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.RequestSnapshot = decodeJSONMap(requestSnapshot)
	item.ResultSnapshot = decodeJSONMap(resultSnapshot)
	return &item, nil
}

func nonNilMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func decodeJSONMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func addSQLArg(args *[]any, value any) string {
	*args = append(*args, value)
	return fmt.Sprintf("$%d", len(*args))
}
