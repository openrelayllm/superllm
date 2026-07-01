package scheduler

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func TestSQLRepositorySaveRunPersistsRunAndSteps(t *testing.T) {
	db, mock := newSchedulerSQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	run := adminplusdomain.SchedulerRunSummary{
		ID:             "manual-1",
		LegacyRunID:    "manual:1",
		TriggerType:    "manual",
		TaskType:       "supplier.balance.sync",
		Status:         "succeeded",
		RequestedAt:    now,
		StartedAt:      &now,
		FinishedAt:     &now,
		SupplierCount:  1,
		TotalSteps:     1,
		SucceededSteps: 1,
		DurationMS:     12,
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO admin_plus_scheduler_runs").
		WithArgs(run.ID, run.LegacyRunID, run.TriggerType, run.TaskType, run.Status, run.RequestedAt, sqlmock.AnyArg(), sqlmock.AnyArg(), run.SupplierCount, run.TotalSteps, run.SucceededSteps, run.FailedSteps, run.SkippedSteps, run.DurationMS, run.ErrorCode, run.ErrorMessage, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM admin_plus_scheduler_steps").
		WithArgs(run.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO admin_plus_scheduler_steps").
		WithArgs(run.ID, int64(7), "relay-a", "fetch_balance", "direct_sync", "succeeded", "scheduler:fetch_balance:supplier:7:202606231200", int64(0), 1, "", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := repo.SaveRun(context.Background(), run, []adminplusdomain.ScheduledTask{
		{
			SupplierID:   7,
			SupplierName: "relay-a",
			TaskType:     adminplusdomain.ExtensionTaskTypeFetchBalance,
			Action:       actionDirectSync,
			ScheduleKey:  "scheduler:fetch_balance:supplier:7:202606231200",
			Synced:       true,
			Total:        1,
		},
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepositoryListRuns(t *testing.T) {
	db, mock := newSchedulerSQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "legacy_run_id", "trigger_type", "task_type", "status", "requested_at", "started_at", "finished_at",
		"supplier_count", "total_steps", "succeeded_steps", "failed_steps", "skipped_steps", "duration_ms",
		"error_code", "error_message", "request_snapshot", "result_snapshot",
	}).AddRow("manual-1", "manual:1", "manual", "supplier.balance.sync", "succeeded", now, now, now, 1, 1, 1, 0, 0, int64(12), "", "", nil, nil)
	mock.ExpectQuery("SELECT id, legacy_run_id, trigger_type").
		WithArgs(20).
		WillReturnRows(rows)

	runs, err := repo.ListRuns(context.Background(), 20)

	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, "manual-1", runs[0].ID)
	require.Equal(t, "supplier.balance.sync", runs[0].TaskType)
	require.NotNil(t, runs[0].StartedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepositoryGetRunAndListSteps(t *testing.T) {
	db, mock := newSchedulerSQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	runRows := sqlmock.NewRows([]string{
		"id", "legacy_run_id", "trigger_type", "task_type", "status", "requested_at", "started_at", "finished_at",
		"supplier_count", "total_steps", "succeeded_steps", "failed_steps", "skipped_steps", "duration_ms",
		"error_code", "error_message", "request_snapshot", "result_snapshot",
	}).AddRow("manual-1", "manual:1", "manual", "supplier.balance.sync", "retryable_failed", now, now, now, 1, 1, 0, 1, 0, int64(12), "SCHEDULER_STEP_FAILED", "upstream_500", nil, nil)
	mock.ExpectQuery("FROM admin_plus_scheduler_runs").
		WithArgs("manual-1").
		WillReturnRows(runRows)

	run, err := repo.GetRun(context.Background(), "manual-1")

	require.NoError(t, err)
	require.Equal(t, "manual-1", run.ID)
	require.Equal(t, "retryable_failed", run.Status)

	nextAttemptAt := now.Add(5 * time.Minute)
	stepRows := sqlmock.NewRows([]string{
		"id", "run_id", "supplier_id", "supplier_name", "task_type", "action", "status", "schedule_key",
		"extension_task_id", "result_count", "reason", "attempts", "max_attempts", "next_attempt_at",
		"locked_by", "locked_until", "request_snapshot", "result_snapshot", "started_at", "finished_at",
	}).AddRow(int64(9), "manual-1", int64(7), "relay-a", "fetch_balance", "direct_sync", "retryable_failed", "scheduler:fetch_balance:supplier:7:202606231200", int64(0), 0, "upstream_500", 2, 3, nextAttemptAt, "", nil, nil, nil, now, now)
	mock.ExpectQuery("FROM admin_plus_scheduler_steps").
		WithArgs("manual-1", 200).
		WillReturnRows(stepRows)

	steps, err := repo.ListSteps(context.Background(), "manual-1", 200)

	require.NoError(t, err)
	require.Len(t, steps, 1)
	require.Equal(t, int64(9), steps[0].ID)
	require.Equal(t, adminplusdomain.ExtensionTaskTypeFetchBalance, steps[0].TaskType)
	require.Equal(t, 2, steps[0].Attempts)
	require.NotNil(t, steps[0].NextAttemptAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepositoryStepStats(t *testing.T) {
	db, mock := newSchedulerSQLMock(t)
	repo := NewSQLRepository(db)
	rows := sqlmock.NewRows([]string{"running", "queued", "failed"}).AddRow(2, 3, 1)
	mock.ExpectQuery("SELECT").
		WillReturnRows(rows)

	running, queued, failed, err := repo.StepStats(context.Background())

	require.NoError(t, err)
	require.Equal(t, 2, running)
	require.Equal(t, 3, queued)
	require.Equal(t, 1, failed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepositoryRetryStepRequeuesRetryableStep(t *testing.T) {
	db, mock := newSchedulerSQLMock(t)
	repo := NewSQLRepository(db)
	retryAt := time.Date(2026, 6, 23, 12, 5, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "run_id", "supplier_id", "supplier_name", "task_type", "action", "status", "schedule_key",
		"extension_task_id", "result_count", "reason", "attempts", "max_attempts", "next_attempt_at",
		"locked_by", "locked_until", "request_snapshot", "result_snapshot", "started_at", "finished_at",
	}).AddRow(int64(9), "manual-1", int64(7), "relay-a", "fetch_balance", "direct_sync", "queued", "scheduler:fetch_balance:supplier:7:202606231200", int64(0), 0, "", 0, 3, retryAt, "", nil, nil, nil, nil, nil)
	mock.ExpectQuery("UPDATE admin_plus_scheduler_steps").
		WithArgs(int64(9), retryAt).
		WillReturnRows(rows)

	step, err := repo.RetryStep(context.Background(), 9, retryAt)

	require.NoError(t, err)
	require.Equal(t, "queued", step.Status)
	require.Empty(t, step.Reason)
	require.Equal(t, 0, step.Attempts)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepositoryCancelStep(t *testing.T) {
	db, mock := newSchedulerSQLMock(t)
	repo := NewSQLRepository(db)
	cancelledAt := time.Date(2026, 6, 23, 12, 6, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "run_id", "supplier_id", "supplier_name", "task_type", "action", "status", "schedule_key",
		"extension_task_id", "result_count", "reason", "attempts", "max_attempts", "next_attempt_at",
		"locked_by", "locked_until", "request_snapshot", "result_snapshot", "started_at", "finished_at",
	}).AddRow(int64(9), "manual-1", int64(7), "relay-a", "fetch_balance", "direct_sync", "cancelled", "scheduler:fetch_balance:supplier:7:202606231200", int64(0), 0, "manual_cancelled", 1, 3, nil, "", nil, nil, nil, nil, cancelledAt)
	mock.ExpectQuery("UPDATE admin_plus_scheduler_steps").
		WithArgs(int64(9), cancelledAt).
		WillReturnRows(rows)

	step, err := repo.CancelStep(context.Background(), 9, cancelledAt)

	require.NoError(t, err)
	require.Equal(t, "cancelled", step.Status)
	require.Equal(t, "manual_cancelled", step.Reason)
	require.NotNil(t, step.FinishedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepositoryCancelRun(t *testing.T) {
	db, mock := newSchedulerSQLMock(t)
	repo := NewSQLRepository(db)
	cancelledAt := time.Date(2026, 6, 23, 12, 6, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "legacy_run_id", "trigger_type", "task_type", "status", "requested_at", "started_at", "finished_at",
		"supplier_count", "total_steps", "succeeded_steps", "failed_steps", "skipped_steps", "duration_ms",
		"error_code", "error_message", "request_snapshot", "result_snapshot",
	}).AddRow("manual-1", "manual:1", "manual", "supplier.balance.sync", "cancelled", cancelledAt, cancelledAt, cancelledAt, 1, 1, 0, 0, 0, int64(12), "", "manual_cancelled", nil, nil)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE admin_plus_scheduler_steps").
		WithArgs("manual-1", cancelledAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("UPDATE admin_plus_scheduler_runs").
		WithArgs("manual-1", cancelledAt).
		WillReturnRows(rows)
	mock.ExpectCommit()

	run, err := repo.CancelRun(context.Background(), "manual-1", cancelledAt)

	require.NoError(t, err)
	require.Equal(t, "cancelled", run.Status)
	require.Equal(t, "manual_cancelled", run.ErrorMessage)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepositoryRetryFailedSteps(t *testing.T) {
	db, mock := newSchedulerSQLMock(t)
	repo := NewSQLRepository(db)
	retryAt := time.Date(2026, 6, 23, 12, 7, 0, 0, time.UTC)
	mock.ExpectExec("UPDATE admin_plus_scheduler_steps").
		WithArgs("manual-1", retryAt).
		WillReturnResult(sqlmock.NewResult(0, 2))

	affected, err := repo.RetryFailedSteps(context.Background(), "manual-1", retryAt)

	require.NoError(t, err)
	require.Equal(t, 2, affected)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepositoryRefreshRunStatus(t *testing.T) {
	db, mock := newSchedulerSQLMock(t)
	repo := NewSQLRepository(db)
	finishedAt := time.Date(2026, 6, 23, 12, 8, 0, 0, time.UTC)
	mock.ExpectExec("WITH counts AS").
		WithArgs("manual-1", finishedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.RefreshRunStatus(context.Background(), "manual-1", finishedAt)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepositoryCompleteStepWritesAttempt(t *testing.T) {
	db, mock := newSchedulerSQLMock(t)
	repo := NewSQLRepository(db)
	finishedAt := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	mock.ExpectExec("WITH current_step AS").
		WithArgs(int64(9), "succeeded", 1, "", finishedAt, "", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.CompleteStep(context.Background(), 9, "succeeded", 1, "", map[string]any{"result": 1}, finishedAt)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepositoryPlans(t *testing.T) {
	db, mock := newSchedulerSQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	next := now.Add(10 * time.Minute)
	plan := adminplusdomain.SchedulerPlan{
		ID:                "supplier.balance.sync",
		Name:              "余额同步",
		TaskType:          "supplier.balance.sync",
		TaskTypes:         []string{"fetch_balance"},
		Status:            "enabled",
		Scope:             "全部启用供应商",
		FrequencyLabel:    "10 分钟",
		IntervalSeconds:   600,
		WindowMinutes:     10,
		MisfirePolicy:     "fire_once",
		ConcurrencyPolicy: "forbid",
		Description:       "读取余额",
		NextRunAt:         &next,
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO admin_plus_scheduler_plans").
		WithArgs(plan.ID, plan.Name, plan.TaskType, sqlmock.AnyArg(), plan.Status, plan.Scope, plan.FrequencyLabel, plan.IntervalSeconds, plan.WindowMinutes, plan.MisfirePolicy, plan.ConcurrencyPolicy, plan.HighCost, plan.Description, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, repo.SavePlans(context.Background(), []adminplusdomain.SchedulerPlan{plan}))

	rows := sqlmock.NewRows([]string{
		"id", "name", "task_type", "task_types", "status", "scope", "frequency_label", "interval_seconds",
		"window_minutes", "misfire_policy", "concurrency_policy", "high_cost", "description", "last_run_at", "next_run_at",
	}).AddRow(plan.ID, plan.Name, plan.TaskType, []byte("{fetch_balance}"), plan.Status, plan.Scope, plan.FrequencyLabel, plan.IntervalSeconds, plan.WindowMinutes, plan.MisfirePolicy, plan.ConcurrencyPolicy, plan.HighCost, plan.Description, nil, next)
	mock.ExpectQuery("SELECT id, name, task_type, task_types").
		WillReturnRows(rows)

	plans, err := repo.ListPlans(context.Background())

	require.NoError(t, err)
	require.Len(t, plans, 1)
	require.Equal(t, plan.ID, plans[0].ID)
	require.Equal(t, []string{"fetch_balance"}, plans[0].TaskTypes)
	require.NotNil(t, plans[0].NextRunAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepositoryUpdatePlanStatus(t *testing.T) {
	db, mock := newSchedulerSQLMock(t)
	repo := NewSQLRepository(db)
	next := time.Date(2026, 6, 23, 12, 10, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "name", "task_type", "task_types", "status", "scope", "frequency_label", "interval_seconds",
		"window_minutes", "misfire_policy", "concurrency_policy", "high_cost", "description", "last_run_at", "next_run_at",
	}).AddRow("supplier.balance.sync", "余额同步", "supplier.balance.sync", []byte("{fetch_balance}"), "paused", "全部启用供应商", "10 分钟", int64(600), 10, "fire_once", "forbid", false, "读取余额", nil, next)
	mock.ExpectQuery("UPDATE admin_plus_scheduler_plans").
		WithArgs("supplier.balance.sync", "paused").
		WillReturnRows(rows)

	plan, err := repo.UpdatePlanStatus(context.Background(), "supplier.balance.sync", "paused")

	require.NoError(t, err)
	require.Equal(t, "paused", plan.Status)
	require.Equal(t, []string{"fetch_balance"}, plan.TaskTypes)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepositorySettings(t *testing.T) {
	db, mock := newSchedulerSQLMock(t)
	repo := NewSQLRepository(db)
	settings := adminplusdomain.SchedulerSettings{
		Enabled:                       true,
		DefaultSupplierConcurrency:    2,
		ChannelChecksEnabled:          true,
		ChannelCheckDailyBudgetTokens: 1000,
		FirstTokenSlowThresholdMS:     3000,
		TotalLatencySlowThresholdMS:   15000,
		DefaultEnabledTaskTypes:       []string{"supplier.balance.sync"},
		HighCostTaskTypes:             []string{"supplier.channels.check"},
	}

	mock.ExpectExec("INSERT INTO admin_plus_scheduler_settings").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.SaveSettings(context.Background(), settings))

	rows := sqlmock.NewRows([]string{"value"}).AddRow([]byte(`{"enabled":true,"default_supplier_concurrency":2,"channel_checks_enabled":true,"channel_check_daily_budget_tokens":1000,"first_token_slow_threshold_ms":3000,"total_latency_slow_threshold_ms":15000,"default_enabled_task_types":["supplier.balance.sync"],"high_cost_task_types":["supplier.channels.check"]}`))
	mock.ExpectQuery("SELECT value FROM admin_plus_scheduler_settings").
		WillReturnRows(rows)

	stored, err := repo.LoadSettings(context.Background())

	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, settings.DefaultSupplierConcurrency, stored.DefaultSupplierConcurrency)
	require.Equal(t, settings.DefaultEnabledTaskTypes, stored.DefaultEnabledTaskTypes)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepositoryActions(t *testing.T) {
	db, mock := newSchedulerSQLMock(t)
	repo := NewSQLRepository(db)
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	action := adminplusdomain.SchedulerAction{
		ID:                   "supplier.recharge:7",
		SupplierID:           7,
		SupplierName:         "relay-a",
		Severity:             "warning",
		Status:               "open",
		Type:                 "supplier.recharge",
		Title:                "供应商余额不足",
		Reason:               "余额不可用",
		RecommendedOperation: "刷新余额或充值",
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO admin_plus_scheduler_actions").
		WithArgs(action.ID, action.SupplierID, action.SupplierName, action.Severity, action.Status, action.Type, action.Title, action.Reason, action.RecommendedOperation, action.CreatedAt, action.UpdatedAt, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, repo.UpsertActions(context.Background(), []adminplusdomain.SchedulerAction{action}))

	rows := sqlmock.NewRows([]string{
		"id", "supplier_id", "supplier_name", "severity", "status", "type", "title", "reason",
		"recommended_operation", "created_at", "updated_at", "resolved_at",
	}).AddRow(action.ID, action.SupplierID, action.SupplierName, action.Severity, action.Status, action.Type, action.Title, action.Reason, action.RecommendedOperation, action.CreatedAt, action.UpdatedAt, nil)
	mock.ExpectQuery("SELECT id, supplier_id, supplier_name, severity").
		WillReturnRows(rows)

	actions, err := repo.ListActions(context.Background())

	require.NoError(t, err)
	require.Len(t, actions, 1)
	require.Equal(t, action.ID, actions[0].ID)

	resolvedAt := now.Add(time.Minute)
	updateRows := sqlmock.NewRows([]string{
		"id", "supplier_id", "supplier_name", "severity", "status", "type", "title", "reason",
		"recommended_operation", "created_at", "updated_at", "resolved_at",
	}).AddRow(action.ID, action.SupplierID, action.SupplierName, action.Severity, "resolved", action.Type, action.Title, action.Reason, action.RecommendedOperation, action.CreatedAt, resolvedAt, resolvedAt)
	mock.ExpectQuery("UPDATE admin_plus_scheduler_actions").
		WithArgs(action.ID, "resolved", sqlmock.AnyArg()).
		WillReturnRows(updateRows)

	updated, err := repo.UpdateActionStatus(context.Background(), action.ID, "resolved", &resolvedAt)

	require.NoError(t, err)
	require.Equal(t, "resolved", updated.Status)
	require.NotNil(t, updated.ResolvedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func newSchedulerSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}
