package usagecosts

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

func (r *SQLRepository) CreateUsageCostLine(ctx context.Context, line *adminplusdomain.SupplierUsageCostLine) (*adminplusdomain.SupplierUsageCostLine, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	rawPayload, err := marshalRawPayload(line.RawPayload)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_supplier_usage_cost_lines (
			supplier_id, source, external_usage_cost_id, external_request_id,
			api_key_name, model, endpoint, request_type, billing_mode,
			reasoning_effort, currency, cost_cents, input_tokens, output_tokens,
			cache_read_tokens, total_tokens, first_token_ms, duration_ms,
			user_agent, started_at, ended_at, raw_payload, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23)
		RETURNING id, supplier_id, source, external_usage_cost_id, external_request_id,
			api_key_name, model, endpoint, request_type, billing_mode,
			reasoning_effort, currency, cost_cents, input_tokens, output_tokens,
			cache_read_tokens, total_tokens, first_token_ms, duration_ms,
			user_agent, started_at, ended_at, raw_payload, created_at
	`,
		line.SupplierID,
		line.Source,
		line.ExternalUsageCostID,
		line.ExternalRequestID,
		line.APIKeyName,
		line.Model,
		line.Endpoint,
		line.RequestType,
		line.BillingMode,
		line.ReasoningEffort,
		line.Currency,
		line.CostCents,
		line.InputTokens,
		line.OutputTokens,
		line.CacheReadTokens,
		line.TotalTokens,
		line.FirstTokenMS,
		line.DurationMS,
		line.UserAgent,
		line.StartedAt,
		nullableTime(line.EndedAt),
		rawPayload,
		line.CreatedAt,
	)
	return scanSupplierUsageCostLine(row)
}

func (r *SQLRepository) ListUsageCostLines(ctx context.Context, filter UsageCostLineFilter) ([]*adminplusdomain.SupplierUsageCostLine, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	where := []string{"1=1"}
	args := make([]any, 0, 2)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if filter.SupplierID > 0 {
		where = append(where, "supplier_id = "+addArg(filter.SupplierID))
	}
	limitRef := addArg(filter.Limit)
	query := `
		SELECT id, supplier_id, source, external_usage_cost_id, external_request_id,
			api_key_name, model, endpoint, request_type, billing_mode,
			reasoning_effort, currency, cost_cents, input_tokens, output_tokens,
			cache_read_tokens, total_tokens, first_token_ms, duration_ms,
			user_agent, started_at, ended_at, raw_payload, created_at
		FROM admin_plus_supplier_usage_cost_lines
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY started_at DESC, id DESC
		LIMIT ` + limitRef

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]*adminplusdomain.SupplierUsageCostLine, 0)
	for rows.Next() {
		item, err := scanSupplierUsageCostLine(rows)
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

type usageCostLineScanner interface {
	Scan(dest ...any) error
}

func scanSupplierUsageCostLine(scanner usageCostLineScanner) (*adminplusdomain.SupplierUsageCostLine, error) {
	var line adminplusdomain.SupplierUsageCostLine
	var endedAt sql.NullTime
	var rawPayload []byte
	err := scanner.Scan(
		&line.ID,
		&line.SupplierID,
		&line.Source,
		&line.ExternalUsageCostID,
		&line.ExternalRequestID,
		&line.APIKeyName,
		&line.Model,
		&line.Endpoint,
		&line.RequestType,
		&line.BillingMode,
		&line.ReasoningEffort,
		&line.Currency,
		&line.CostCents,
		&line.InputTokens,
		&line.OutputTokens,
		&line.CacheReadTokens,
		&line.TotalTokens,
		&line.FirstTokenMS,
		&line.DurationMS,
		&line.UserAgent,
		&line.StartedAt,
		&endedAt,
		&rawPayload,
		&line.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if endedAt.Valid {
		t := endedAt.Time
		line.EndedAt = &t
	}
	if len(rawPayload) > 0 {
		var payload map[string]any
		if err := json.Unmarshal(rawPayload, &payload); err != nil {
			return nil, err
		}
		line.RawPayload = payload
	}
	return &line, nil
}

func marshalRawPayload(payload map[string]any) ([]byte, error) {
	if len(payload) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(payload)
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
