package actions

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/lib/pq"
)

type ActionFilter = RecommendationFilter

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) CreateRecommendation(ctx context.Context, action *adminplusdomain.ActionRecommendation) (*adminplusdomain.ActionRecommendation, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_action_recommendations (
			supplier_id, target_supplier_id, type, severity, status, reason_code,
			title, description, expected_impact, requires_approval, signals, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, supplier_id, target_supplier_id, type, severity, status,
			reason_code, title, description, expected_impact, requires_approval,
			signals, created_at
	`,
		action.SupplierID,
		nullableInt64(action.TargetSupplierID),
		string(action.Type),
		string(action.Severity),
		string(action.Status),
		action.ReasonCode,
		action.Title,
		action.Description,
		action.ExpectedImpact,
		action.RequiresApproval,
		pq.Array(action.Signals),
		action.CreatedAt,
	)
	return scanActionRecommendation(row)
}

func (r *SQLRepository) GetRecommendation(ctx context.Context, id int64) (*adminplusdomain.ActionRecommendation, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, supplier_id, target_supplier_id, type, severity, status,
			reason_code, title, description, expected_impact, requires_approval,
			signals, created_at
		FROM admin_plus_action_recommendations
		WHERE id = $1
	`, id)
	action, err := scanActionRecommendation(row)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "ACTION_RECOMMENDATION_NOT_FOUND", "action recommendation not found")
	}
	return action, err
}

func (r *SQLRepository) ListRecommendations(ctx context.Context, filter ActionFilter) ([]*adminplusdomain.ActionRecommendation, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	where := []string{"1=1"}
	args := make([]any, 0, 5)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if filter.ID > 0 {
		where = append(where, "id = "+addArg(filter.ID))
	}
	if filter.SupplierID > 0 {
		where = append(where, "supplier_id = "+addArg(filter.SupplierID))
	}
	if filter.Status != "" {
		where = append(where, "status = "+addArg(string(filter.Status)))
	}
	if filter.Severity != "" {
		where = append(where, "severity = "+addArg(string(filter.Severity)))
	}
	if filter.Type != "" {
		where = append(where, "type = "+addArg(string(filter.Type)))
	}
	if strings.TrimSpace(filter.Signal) != "" {
		where = append(where, "signals @> ARRAY["+addArg(strings.TrimSpace(filter.Signal))+"]::text[]")
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	limitRef := addArg(limit)
	query := `
		SELECT id, supplier_id, target_supplier_id, type, severity, status,
			reason_code, title, description, expected_impact, requires_approval,
			signals, created_at
		FROM admin_plus_action_recommendations
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY created_at DESC, id DESC
		LIMIT ` + limitRef

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]*adminplusdomain.ActionRecommendation, 0)
	for rows.Next() {
		item, err := scanActionRecommendation(rows)
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

func (r *SQLRepository) UpdateRecommendationStatus(ctx context.Context, id int64, status adminplusdomain.ActionStatus) (*adminplusdomain.ActionRecommendation, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_action_recommendations
		SET status = $2
		WHERE id = $1
		RETURNING id, supplier_id, target_supplier_id, type, severity, status,
			reason_code, title, description, expected_impact, requires_approval,
			signals, created_at
	`, id, string(status))
	action, err := scanActionRecommendation(row)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "ACTION_RECOMMENDATION_NOT_FOUND", "action recommendation not found")
	}
	return action, err
}

func (r *SQLRepository) CreateExecution(ctx context.Context, execution *adminplusdomain.ActionExecution) (*adminplusdomain.ActionExecution, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	requestPayload, err := json.Marshal(nonNilPayload(execution.RequestPayload))
	if err != nil {
		return nil, err
	}
	responsePayload, err := json.Marshal(nonNilPayload(execution.ResponsePayload))
	if err != nil {
		return nil, err
	}
	beforeSnapshot, err := json.Marshal(nonNilPayload(execution.BeforeSnapshot))
	if err != nil {
		return nil, err
	}
	afterSnapshot, err := json.Marshal(nonNilPayload(execution.AfterSnapshot))
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_action_executions (
			recommendation_id, action_type, supplier_id, target_supplier_id, status,
			request_payload, response_payload, error_message, operator_user_id,
			scheduler_run_id, scheduler_step_id, idempotency_key_hash, idempotency_replayed,
			before_snapshot, after_snapshot, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8, $9, $10, $11, $12, $13, $14::jsonb, $15::jsonb, $16, $17)
		RETURNING id, recommendation_id, action_type, supplier_id, target_supplier_id,
			status, request_payload, response_payload, error_message, operator_user_id,
			scheduler_run_id, scheduler_step_id, idempotency_key_hash, idempotency_replayed,
			before_snapshot, after_snapshot, created_at, updated_at
	`,
		execution.RecommendationID,
		string(execution.ActionType),
		execution.SupplierID,
		nullableInt64(execution.TargetSupplierID),
		string(execution.Status),
		string(requestPayload),
		string(responsePayload),
		execution.ErrorMessage,
		execution.OperatorUserID,
		execution.SchedulerRunID,
		execution.SchedulerStepID,
		execution.IdempotencyKeyHash,
		execution.IdempotencyReplayed,
		string(beforeSnapshot),
		string(afterSnapshot),
		execution.CreatedAt,
		execution.UpdatedAt,
	)
	return scanActionExecution(row)
}

func (r *SQLRepository) GetExecution(ctx context.Context, id int64) (*adminplusdomain.ActionExecution, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, recommendation_id, action_type, supplier_id, target_supplier_id,
			status, request_payload, response_payload, error_message, operator_user_id,
			scheduler_run_id, scheduler_step_id, idempotency_key_hash, idempotency_replayed,
			before_snapshot, after_snapshot, created_at, updated_at
		FROM admin_plus_action_executions
		WHERE id = $1
	`, id)
	execution, err := scanActionExecution(row)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "ACTION_EXECUTION_NOT_FOUND", "action execution not found")
	}
	return execution, err
}

func (r *SQLRepository) ListExecutions(ctx context.Context, recommendationID int64, limit int) ([]*adminplusdomain.ActionExecution, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, recommendation_id, action_type, supplier_id, target_supplier_id,
			status, request_payload, response_payload, error_message, operator_user_id,
			scheduler_run_id, scheduler_step_id, idempotency_key_hash, idempotency_replayed,
			before_snapshot, after_snapshot, created_at, updated_at
		FROM admin_plus_action_executions
		WHERE recommendation_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, recommendationID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]*adminplusdomain.ActionExecution, 0)
	for rows.Next() {
		item, err := scanActionExecution(rows)
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

func (r *SQLRepository) MarkExecutionIdempotencyReplayed(ctx context.Context, recommendationID int64, idempotencyKeyHash string) (*adminplusdomain.ActionExecution, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		WITH target AS (
			SELECT id
			FROM admin_plus_action_executions
			WHERE recommendation_id = $1 AND idempotency_key_hash = $2
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		)
		UPDATE admin_plus_action_executions AS e
		SET idempotency_replayed = TRUE, updated_at = NOW()
		FROM target
		WHERE e.id = target.id
		RETURNING e.id, e.recommendation_id, e.action_type, e.supplier_id, e.target_supplier_id,
			e.status, e.request_payload, e.response_payload, e.error_message, e.operator_user_id,
			e.scheduler_run_id, e.scheduler_step_id, e.idempotency_key_hash, e.idempotency_replayed,
			e.before_snapshot, e.after_snapshot, e.created_at, e.updated_at
	`, recommendationID, idempotencyKeyHash)
	execution, err := scanActionExecution(row)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "ACTION_EXECUTION_IDEMPOTENCY_RECORD_NOT_FOUND", "action execution idempotency record not found")
	}
	return execution, err
}

func (r *SQLRepository) MarkLatestExecutionIdempotencyReplayed(ctx context.Context, actionType adminplusdomain.ActionType, idempotencyKeyHash string) (*adminplusdomain.ActionExecution, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		WITH target AS (
			SELECT id
			FROM admin_plus_action_executions
			WHERE action_type = $1 AND idempotency_key_hash = $2
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		)
		UPDATE admin_plus_action_executions AS e
		SET idempotency_replayed = TRUE, updated_at = NOW()
		FROM target
		WHERE e.id = target.id
		RETURNING e.id, e.recommendation_id, e.action_type, e.supplier_id, e.target_supplier_id,
			e.status, e.request_payload, e.response_payload, e.error_message, e.operator_user_id,
			e.scheduler_run_id, e.scheduler_step_id, e.idempotency_key_hash, e.idempotency_replayed,
			e.before_snapshot, e.after_snapshot, e.created_at, e.updated_at
	`, string(actionType), idempotencyKeyHash)
	execution, err := scanActionExecution(row)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "ACTION_EXECUTION_IDEMPOTENCY_RECORD_NOT_FOUND", "action execution idempotency record not found")
	}
	return execution, err
}

type actionScanner interface {
	Scan(dest ...any) error
}

func scanActionRecommendation(scanner actionScanner) (*adminplusdomain.ActionRecommendation, error) {
	var action adminplusdomain.ActionRecommendation
	var targetSupplierID sql.NullInt64
	var actionType, severity, status string
	var signals []string
	err := scanner.Scan(
		&action.ID,
		&action.SupplierID,
		&targetSupplierID,
		&actionType,
		&severity,
		&status,
		&action.ReasonCode,
		&action.Title,
		&action.Description,
		&action.ExpectedImpact,
		&action.RequiresApproval,
		pq.Array(&signals),
		&action.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if targetSupplierID.Valid {
		v := targetSupplierID.Int64
		action.TargetSupplierID = &v
	}
	action.Type = adminplusdomain.ActionType(actionType)
	action.Severity = adminplusdomain.ActionSeverity(severity)
	action.Status = adminplusdomain.ActionStatus(status)
	action.Signals = signals
	return &action, nil
}

func scanActionExecution(scanner actionScanner) (*adminplusdomain.ActionExecution, error) {
	var execution adminplusdomain.ActionExecution
	var targetSupplierID sql.NullInt64
	var actionType, status string
	var requestPayload []byte
	var responsePayload []byte
	var beforeSnapshot []byte
	var afterSnapshot []byte
	err := scanner.Scan(
		&execution.ID,
		&execution.RecommendationID,
		&actionType,
		&execution.SupplierID,
		&targetSupplierID,
		&status,
		&requestPayload,
		&responsePayload,
		&execution.ErrorMessage,
		&execution.OperatorUserID,
		&execution.SchedulerRunID,
		&execution.SchedulerStepID,
		&execution.IdempotencyKeyHash,
		&execution.IdempotencyReplayed,
		&beforeSnapshot,
		&afterSnapshot,
		&execution.CreatedAt,
		&execution.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if targetSupplierID.Valid {
		v := targetSupplierID.Int64
		execution.TargetSupplierID = &v
	}
	execution.ActionType = adminplusdomain.ActionType(actionType)
	execution.Status = adminplusdomain.ActionExecutionStatus(status)
	execution.RequestPayload = decodePayload(requestPayload)
	execution.ResponsePayload = decodePayload(responsePayload)
	execution.BeforeSnapshot = decodePayload(beforeSnapshot)
	execution.AfterSnapshot = decodePayload(afterSnapshot)
	return &execution, nil
}

func decodePayload(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func nonNilPayload(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	return payload
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func dbNotConfigured() error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
}
