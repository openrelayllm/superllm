package actions

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func newActionSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return db, mock
}

func TestSQLRepositoryCreateActionRecommendation(t *testing.T) {
	db, mock := newActionSQLMock(t)
	repo := NewSQLRepository(db)
	createdAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	targetSupplierID := int64(9)

	mock.ExpectQuery(`INSERT INTO admin_plus_action_recommendations`).
		WithArgs(
			int64(7),
			targetSupplierID,
			"switch_supplier",
			"critical",
			"open",
			"switch_from_depleted_supplier",
			"Switch supplier",
			"depleted",
			"restore traffic",
			true,
			sqlmock.AnyArg(),
			createdAt,
		).
		WillReturnRows(newActionRows().AddRow(
			int64(21),
			int64(7),
			targetSupplierID,
			"switch_supplier",
			"critical",
			"open",
			"switch_from_depleted_supplier",
			"Switch supplier",
			"depleted",
			"restore traffic",
			true,
			pq.StringArray{"balance_depleted", "candidate_available"},
			createdAt,
		))

	got, err := repo.CreateRecommendation(context.Background(), &adminplusdomain.ActionRecommendation{
		SupplierID:       7,
		TargetSupplierID: &targetSupplierID,
		Type:             adminplusdomain.ActionTypeSwitchSupplier,
		Severity:         adminplusdomain.ActionSeverityCritical,
		Status:           adminplusdomain.ActionStatusOpen,
		ReasonCode:       "switch_from_depleted_supplier",
		Title:            "Switch supplier",
		Description:      "depleted",
		ExpectedImpact:   "restore traffic",
		RequiresApproval: true,
		Signals:          []string{"balance_depleted", "candidate_available"},
		CreatedAt:        createdAt,
	})

	require.NoError(t, err)
	require.Equal(t, int64(21), got.ID)
	require.NotNil(t, got.TargetSupplierID)
	require.Equal(t, adminplusdomain.ActionSeverityCritical, got.Severity)
	require.Equal(t, []string{"balance_depleted", "candidate_available"}, got.Signals)
}

func TestSQLRepositoryListActionRecommendationsFiltersWithParameterizedQuery(t *testing.T) {
	db, mock := newActionSQLMock(t)
	repo := NewSQLRepository(db)
	createdAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM admin_plus_action_recommendations\s+WHERE 1=1 AND supplier_id = \$1 AND status = \$2 AND severity = \$3 AND type = \$4\s+ORDER BY created_at DESC, id DESC\s+LIMIT \$5`).
		WithArgs(int64(7), "open", "warning", "degrade_supplier", 50).
		WillReturnRows(newActionRows().AddRow(
			int64(21),
			int64(7),
			nil,
			"degrade_supplier",
			"warning",
			"open",
			"supplier_performance_degraded",
			"Degrade supplier",
			"slow",
			"reduce slow responses",
			true,
			pq.StringArray{"slow_first_token"},
			createdAt,
		))

	items, err := repo.ListRecommendations(context.Background(), ActionFilter{
		SupplierID: 7,
		Status:     adminplusdomain.ActionStatusOpen,
		Severity:   adminplusdomain.ActionSeverityWarning,
		Type:       adminplusdomain.ActionTypeDegradeSupplier,
		Limit:      50,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, adminplusdomain.ActionTypeDegradeSupplier, items[0].Type)
	require.Equal(t, []string{"slow_first_token"}, items[0].Signals)
}

func TestSQLRepositoryListActionRecommendationsFiltersBySignal(t *testing.T) {
	db, mock := newActionSQLMock(t)
	repo := NewSQLRepository(db)
	createdAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM admin_plus_action_recommendations\s+WHERE 1=1 AND type = \$1 AND signals @> ARRAY\[\$2\]::text\[\]\s+ORDER BY created_at DESC, id DESC\s+LIMIT \$3`).
		WithArgs("routing_refill", "local_group_id=1001", 50).
		WillReturnRows(newActionRows().AddRow(
			int64(22),
			int64(7),
			nil,
			"routing_refill",
			"critical",
			"open",
			"local_group_routing_refill_required",
			"Refill",
			"empty",
			"restore capacity",
			true,
			pq.StringArray{"local_group_id=1001"},
			createdAt,
		))

	items, err := repo.ListRecommendations(context.Background(), ActionFilter{
		Type:   adminplusdomain.ActionTypeRoutingRefill,
		Signal: "local_group_id=1001",
		Limit:  50,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(22), items[0].ID)
}

func TestSQLRepositoryUpdateActionRecommendationStatusNotFound(t *testing.T) {
	db, mock := newActionSQLMock(t)
	repo := NewSQLRepository(db)

	mock.ExpectQuery(`UPDATE admin_plus_action_recommendations`).
		WithArgs(int64(404), "acknowledged").
		WillReturnError(sql.ErrNoRows)

	_, err := repo.UpdateRecommendationStatus(context.Background(), 404, adminplusdomain.ActionStatusAcknowledged)

	require.Error(t, err)
	require.Contains(t, err.Error(), "ACTION_RECOMMENDATION_NOT_FOUND")
}

func TestSQLRepositoryCreateExecutionPersistsSchedulerSource(t *testing.T) {
	db, mock := newActionSQLMock(t)
	repo := NewSQLRepository(db)
	createdAt := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Second)

	mock.ExpectQuery(`INSERT INTO admin_plus_action_executions`).
		WithArgs(
			int64(11),
			"routing_refill",
			int64(7),
			nil,
			"succeeded",
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			"",
			int64(99),
			"routing-capacity-watch-1",
			int64(123),
			"idempotency-hash-1",
			false,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			createdAt,
			updatedAt,
		).
		WillReturnRows(newActionExecutionRows().AddRow(
			int64(31),
			int64(11),
			"routing_refill",
			int64(7),
			nil,
			"succeeded",
			[]byte(`{"local_group_id":9}`),
			[]byte(`{"updated":true}`),
			"",
			int64(99),
			"routing-capacity-watch-1",
			int64(123),
			"idempotency-hash-1",
			false,
			[]byte(`{"schedulable_accounts":0}`),
			[]byte(`{"schedulable_accounts":1}`),
			createdAt,
			updatedAt,
		))

	got, err := repo.CreateExecution(context.Background(), &adminplusdomain.ActionExecution{
		RecommendationID:   11,
		ActionType:         adminplusdomain.ActionTypeRoutingRefill,
		SupplierID:         7,
		Status:             adminplusdomain.ActionExecutionStatusSucceeded,
		RequestPayload:     map[string]any{"local_group_id": int64(9)},
		ResponsePayload:    map[string]any{"updated": true},
		OperatorUserID:     99,
		SchedulerRunID:     "routing-capacity-watch-1",
		SchedulerStepID:    123,
		IdempotencyKeyHash: "idempotency-hash-1",
		BeforeSnapshot:     map[string]any{"schedulable_accounts": int64(0)},
		AfterSnapshot:      map[string]any{"schedulable_accounts": int64(1)},
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	})

	require.NoError(t, err)
	require.Equal(t, int64(31), got.ID)
	require.Equal(t, "routing-capacity-watch-1", got.SchedulerRunID)
	require.Equal(t, int64(123), got.SchedulerStepID)
	require.Equal(t, "idempotency-hash-1", got.IdempotencyKeyHash)
	require.Equal(t, float64(0), got.BeforeSnapshot["schedulable_accounts"])
	require.Equal(t, float64(1), got.AfterSnapshot["schedulable_accounts"])
	require.Equal(t, adminplusdomain.ActionExecutionStatusSucceeded, got.Status)
}

func TestSQLRepositoryGetExecution(t *testing.T) {
	db, mock := newActionSQLMock(t)
	repo := NewSQLRepository(db)
	createdAt := time.Date(2026, 7, 9, 11, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Second)

	mock.ExpectQuery(`FROM admin_plus_action_executions\s+WHERE id = \$1`).
		WithArgs(int64(31)).
		WillReturnRows(newActionExecutionRows().AddRow(
			int64(31),
			int64(11),
			"routing_refill",
			int64(7),
			nil,
			"failed",
			[]byte(`{"local_group_id":9}`),
			[]byte(`{"skipped_reason":"candidate_not_found"}`),
			"candidate_not_found",
			int64(99),
			"routing-capacity-watch-1",
			int64(123),
			"idempotency-hash-1",
			false,
			[]byte(`{"schedulable_accounts":0}`),
			[]byte(`{"skipped_reason":"candidate_not_found"}`),
			createdAt,
			updatedAt,
		))

	got, err := repo.GetExecution(context.Background(), 31)

	require.NoError(t, err)
	require.Equal(t, int64(31), got.ID)
	require.Equal(t, int64(11), got.RecommendationID)
	require.Equal(t, adminplusdomain.ActionExecutionStatusFailed, got.Status)
	require.Equal(t, "candidate_not_found", got.ErrorMessage)
	require.Equal(t, float64(9), got.RequestPayload["local_group_id"])
}

func TestSQLRepositoryMarkExecutionIdempotencyReplayed(t *testing.T) {
	db, mock := newActionSQLMock(t)
	repo := NewSQLRepository(db)
	createdAt := time.Date(2026, 7, 9, 11, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)

	mock.ExpectQuery(`WITH target AS \(\s+SELECT id\s+FROM admin_plus_action_executions\s+WHERE recommendation_id = \$1 AND idempotency_key_hash = \$2\s+ORDER BY created_at DESC, id DESC\s+LIMIT 1\s+\)\s+UPDATE admin_plus_action_executions AS e\s+SET idempotency_replayed = TRUE, updated_at = NOW\(\)`).
		WithArgs(int64(11), "idempotency-hash-1").
		WillReturnRows(newActionExecutionRows().AddRow(
			int64(31),
			int64(11),
			"routing_refill",
			int64(7),
			nil,
			"succeeded",
			[]byte(`{"local_group_id":9}`),
			[]byte(`{"updated":true}`),
			"",
			int64(99),
			"routing-capacity-watch-1",
			int64(123),
			"idempotency-hash-1",
			true,
			[]byte(`{"schedulable_accounts":0}`),
			[]byte(`{"schedulable_accounts":1}`),
			createdAt,
			updatedAt,
		))

	got, err := repo.MarkExecutionIdempotencyReplayed(context.Background(), 11, "idempotency-hash-1")

	require.NoError(t, err)
	require.Equal(t, int64(31), got.ID)
	require.True(t, got.IdempotencyReplayed)
	require.Equal(t, "idempotency-hash-1", got.IdempotencyKeyHash)
}

func TestSQLRepositoryMarkLatestExecutionIdempotencyReplayed(t *testing.T) {
	db, mock := newActionSQLMock(t)
	repo := NewSQLRepository(db)
	createdAt := time.Date(2026, 7, 9, 11, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)

	mock.ExpectQuery(`WITH target AS \(\s+SELECT id\s+FROM admin_plus_action_executions\s+WHERE action_type = \$1 AND idempotency_key_hash = \$2\s+ORDER BY created_at DESC, id DESC\s+LIMIT 1\s+\)\s+UPDATE admin_plus_action_executions AS e\s+SET idempotency_replayed = TRUE, updated_at = NOW\(\)`).
		WithArgs("local_account_manual_ops", "manual-hash-1").
		WillReturnRows(newActionExecutionRows().AddRow(
			int64(41),
			int64(21),
			"local_account_manual_ops",
			int64(0),
			nil,
			"succeeded",
			[]byte(`{"action":"set_schedulable"}`),
			[]byte(`{"updated_accounts":1}`),
			"",
			int64(99),
			"",
			int64(0),
			"manual-hash-1",
			true,
			[]byte(`{"mode":"local_account_ops_before"}`),
			[]byte(`{"mode":"local_account_ops_after"}`),
			createdAt,
			updatedAt,
		))

	got, err := repo.MarkLatestExecutionIdempotencyReplayed(context.Background(), adminplusdomain.ActionTypeLocalAccountManualOps, "manual-hash-1")

	require.NoError(t, err)
	require.Equal(t, int64(41), got.ID)
	require.Equal(t, adminplusdomain.ActionTypeLocalAccountManualOps, got.ActionType)
	require.True(t, got.IdempotencyReplayed)
}

func newActionRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"supplier_id",
		"target_supplier_id",
		"type",
		"severity",
		"status",
		"reason_code",
		"title",
		"description",
		"expected_impact",
		"requires_approval",
		"signals",
		"created_at",
	})
}

func newActionExecutionRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"recommendation_id",
		"action_type",
		"supplier_id",
		"target_supplier_id",
		"status",
		"request_payload",
		"response_payload",
		"error_message",
		"operator_user_id",
		"scheduler_run_id",
		"scheduler_step_id",
		"idempotency_key_hash",
		"idempotency_replayed",
		"before_snapshot",
		"after_snapshot",
		"created_at",
		"updated_at",
	})
}
