package sub2api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type SQLRepository struct {
	db *sql.DB
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

func addSQLArg(args *[]any, value any) string {
	*args = append(*args, value)
	return fmt.Sprintf("$%d", len(*args))
}
