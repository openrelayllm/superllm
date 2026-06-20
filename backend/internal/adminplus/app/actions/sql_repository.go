package actions

import (
	"context"
	"database/sql"
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

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func dbNotConfigured() error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
}
