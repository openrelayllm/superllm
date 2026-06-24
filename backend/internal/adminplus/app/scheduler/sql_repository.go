package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
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

func (r *SQLRepository) SaveRun(ctx context.Context, run adminplusdomain.SchedulerRunSummary, steps []adminplusdomain.ScheduledTask) error {
	if r == nil || r.db == nil {
		return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
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

	_, err = tx.ExecContext(ctx, `
		INSERT INTO admin_plus_scheduler_runs (
			id, legacy_run_id, trigger_type, task_type, status, requested_at, started_at, finished_at,
			supplier_count, total_steps, succeeded_steps, failed_steps, skipped_steps, duration_ms,
			error_code, error_message, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, NOW())
		ON CONFLICT (id) DO UPDATE SET
			legacy_run_id = EXCLUDED.legacy_run_id,
			trigger_type = EXCLUDED.trigger_type,
			task_type = EXCLUDED.task_type,
			status = EXCLUDED.status,
			requested_at = EXCLUDED.requested_at,
			started_at = EXCLUDED.started_at,
			finished_at = EXCLUDED.finished_at,
			supplier_count = EXCLUDED.supplier_count,
			total_steps = EXCLUDED.total_steps,
			succeeded_steps = EXCLUDED.succeeded_steps,
			failed_steps = EXCLUDED.failed_steps,
			skipped_steps = EXCLUDED.skipped_steps,
			duration_ms = EXCLUDED.duration_ms,
			error_code = EXCLUDED.error_code,
			error_message = EXCLUDED.error_message,
			updated_at = NOW()
	`, run.ID, run.LegacyRunID, run.TriggerType, run.TaskType, run.Status, run.RequestedAt, nullableTime(run.StartedAt), nullableTime(run.FinishedAt),
		run.SupplierCount, run.TotalSteps, run.SucceededSteps, run.FailedSteps, run.SkippedSteps, run.DurationMS, run.ErrorCode, run.ErrorMessage)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM admin_plus_scheduler_steps WHERE run_id = $1`, run.ID)
	if err != nil {
		return err
	}
	for _, step := range steps {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO admin_plus_scheduler_steps (
				run_id, supplier_id, supplier_name, task_type, action, status, schedule_key,
				extension_task_id, result_count, reason, started_at, finished_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, run.ID, step.SupplierID, step.SupplierName, string(step.TaskType), step.Action, schedulerStepStatus(run.Status, step), step.ScheduleKey, step.TaskID, step.Total, step.Reason, nullableTime(run.StartedAt), nullableTime(run.FinishedAt))
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	return err
}

func (r *SQLRepository) ListRuns(ctx context.Context, limit int) ([]adminplusdomain.SchedulerRunSummary, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, legacy_run_id, trigger_type, task_type, status, requested_at, started_at, finished_at,
			supplier_count, total_steps, succeeded_steps, failed_steps, skipped_steps, duration_ms,
			error_code, error_message
		FROM admin_plus_scheduler_runs
		ORDER BY requested_at DESC, id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]adminplusdomain.SchedulerRunSummary, 0)
	for rows.Next() {
		run, err := scanRunSummary(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *run)
	}
	return out, rows.Err()
}

func (r *SQLRepository) GetRun(ctx context.Context, runID string) (*adminplusdomain.SchedulerRunSummary, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, legacy_run_id, trigger_type, task_type, status, requested_at, started_at, finished_at,
			supplier_count, total_steps, succeeded_steps, failed_steps, skipped_steps, duration_ms,
			error_code, error_message
		FROM admin_plus_scheduler_runs
		WHERE id = $1
	`, runID)
	return scanRunSummary(row)
}

func (r *SQLRepository) ListSteps(ctx context.Context, runID string, limit int) ([]adminplusdomain.SchedulerStepRecord, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, run_id, supplier_id, supplier_name, task_type, action, status, schedule_key,
			extension_task_id, result_count, reason, attempts, max_attempts, next_attempt_at,
			locked_by, locked_until, started_at, finished_at
		FROM admin_plus_scheduler_steps
		WHERE ($1 = '' OR run_id = $1)
		ORDER BY id ASC
		LIMIT $2
	`, runID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]adminplusdomain.SchedulerStepRecord, 0)
	for rows.Next() {
		step, err := scanStepRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *step)
	}
	return out, rows.Err()
}

func (r *SQLRepository) ListAttempts(ctx context.Context, runID string, limit int) ([]adminplusdomain.SchedulerAttemptRecord, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return []adminplusdomain.SchedulerAttemptRecord{}, nil
	}
	if limit <= 0 || limit > 2000 {
		limit = 1000
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, step_id, run_id, supplier_id, task_type, status, worker_id, attempt_no,
			started_at, finished_at, duration_ms, error_code, error_message, request_snapshot, response_snapshot
		FROM admin_plus_scheduler_attempts
		WHERE run_id = $1
		ORDER BY step_id ASC, attempt_no ASC, id ASC
		LIMIT $2
	`, runID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]adminplusdomain.SchedulerAttemptRecord, 0)
	for rows.Next() {
		attempt, err := scanAttemptRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *attempt)
	}
	return out, rows.Err()
}

func (r *SQLRepository) SavePlans(ctx context.Context, plans []adminplusdomain.SchedulerPlan) error {
	if r == nil || r.db == nil {
		return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
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
	for _, plan := range plans {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO admin_plus_scheduler_plans (
				id, name, task_type, task_types, status, scope, frequency_label, interval_seconds,
				window_minutes, misfire_policy, concurrency_policy, high_cost, description,
				last_run_at, next_run_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, NOW())
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				task_type = EXCLUDED.task_type,
				task_types = EXCLUDED.task_types,
				status = admin_plus_scheduler_plans.status,
				scope = admin_plus_scheduler_plans.scope,
				frequency_label = admin_plus_scheduler_plans.frequency_label,
				interval_seconds = admin_plus_scheduler_plans.interval_seconds,
				window_minutes = admin_plus_scheduler_plans.window_minutes,
				misfire_policy = admin_plus_scheduler_plans.misfire_policy,
				concurrency_policy = admin_plus_scheduler_plans.concurrency_policy,
				high_cost = EXCLUDED.high_cost,
				description = EXCLUDED.description,
				next_run_at = admin_plus_scheduler_plans.next_run_at,
				updated_at = NOW()
		`, plan.ID, plan.Name, plan.TaskType, pq.Array(plan.TaskTypes), plan.Status, plan.Scope, plan.FrequencyLabel,
			plan.IntervalSeconds, plan.WindowMinutes, plan.MisfirePolicy, plan.ConcurrencyPolicy, plan.HighCost,
			plan.Description, nullableTime(plan.LastRunAt), nullableTime(plan.NextRunAt))
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	return err
}

func (r *SQLRepository) ListPlans(ctx context.Context) ([]adminplusdomain.SchedulerPlan, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, task_type, task_types, status, scope, frequency_label, interval_seconds,
			window_minutes, misfire_policy, concurrency_policy, high_cost, description, last_run_at, next_run_at
		FROM admin_plus_scheduler_plans
		ORDER BY high_cost ASC, interval_seconds ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]adminplusdomain.SchedulerPlan, 0)
	for rows.Next() {
		var plan adminplusdomain.SchedulerPlan
		var taskTypes []string
		var lastRunAt, nextRunAt sql.NullTime
		if err := rows.Scan(
			&plan.ID,
			&plan.Name,
			&plan.TaskType,
			pq.Array(&taskTypes),
			&plan.Status,
			&plan.Scope,
			&plan.FrequencyLabel,
			&plan.IntervalSeconds,
			&plan.WindowMinutes,
			&plan.MisfirePolicy,
			&plan.ConcurrencyPolicy,
			&plan.HighCost,
			&plan.Description,
			&lastRunAt,
			&nextRunAt,
		); err != nil {
			return nil, err
		}
		plan.TaskTypes = taskTypes
		plan.LastRunAt = timePtr(lastRunAt)
		plan.NextRunAt = timePtr(nextRunAt)
		out = append(out, plan)
	}
	return out, rows.Err()
}

func (r *SQLRepository) UpdatePlanStatus(ctx context.Context, planID, status string) (*adminplusdomain.SchedulerPlan, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_scheduler_plans
		SET status = $2,
			next_run_at = CASE
				WHEN $2 = 'enabled' AND next_run_at IS NULL AND interval_seconds > 0 THEN NOW() + make_interval(secs => interval_seconds::int)
				WHEN $2 <> 'enabled' THEN NULL
				ELSE next_run_at
			END,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, task_type, task_types, status, scope, frequency_label, interval_seconds,
			window_minutes, misfire_policy, concurrency_policy, high_cost, description, last_run_at, next_run_at
	`, planID, status)
	return scanPlan(row)
}

func (r *SQLRepository) ClaimDuePlan(ctx context.Context, now time.Time) (*adminplusdomain.SchedulerPlan, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	row := r.db.QueryRowContext(ctx, `
		WITH picked AS (
			SELECT id
			FROM admin_plus_scheduler_plans
			WHERE status = 'enabled'
			  AND interval_seconds > 0
			  AND (next_run_at IS NULL OR next_run_at <= $1)
			ORDER BY COALESCE(next_run_at, $1) ASC, high_cost ASC, id ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		),
		updated AS (
			UPDATE admin_plus_scheduler_plans AS plan
			SET last_run_at = $1,
				next_run_at = $1 + make_interval(secs => plan.interval_seconds::int),
				updated_at = NOW()
			FROM picked
			WHERE plan.id = picked.id
			RETURNING plan.id, plan.name, plan.task_type, plan.task_types, plan.status, plan.scope, plan.frequency_label,
				plan.interval_seconds, plan.window_minutes, plan.misfire_policy, plan.concurrency_policy,
				plan.high_cost, plan.description, plan.last_run_at, plan.next_run_at
		)
		SELECT id, name, task_type, task_types, status, scope, frequency_label, interval_seconds,
			window_minutes, misfire_policy, concurrency_policy, high_cost, description, last_run_at, next_run_at
		FROM updated
	`, now)
	plan, err := scanPlan(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return plan, err
}

func (r *SQLRepository) SaveSettings(ctx context.Context, settings adminplusdomain.SchedulerSettings) error {
	if r == nil || r.db == nil {
		return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	payload, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO admin_plus_scheduler_settings (key, value, updated_at)
		VALUES ('global', $1::jsonb, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`, string(payload))
	return err
}

func (r *SQLRepository) LoadSettings(ctx context.Context) (*adminplusdomain.SchedulerSettings, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	var raw []byte
	if err := r.db.QueryRowContext(ctx, `SELECT value FROM admin_plus_scheduler_settings WHERE key = 'global'`).Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	var settings adminplusdomain.SchedulerSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *SQLRepository) StepStats(ctx context.Context) (running int, queued int, failed int, err error) {
	if r == nil || r.db == nil {
		err = infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
		return
	}
	err = r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'running')::int AS running,
			COUNT(*) FILTER (WHERE status IN ('queued', 'retryable_failed'))::int AS queued,
			COUNT(*) FILTER (WHERE status IN ('retryable_failed', 'manual_required', 'dead'))::int AS failed
		FROM admin_plus_scheduler_steps
	`).Scan(&running, &queued, &failed)
	return
}

func (r *SQLRepository) ClaimStep(ctx context.Context, workerID string, lease time.Duration) (*adminplusdomain.SchedulerStepRecord, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	if lease <= 0 {
		lease = 5 * time.Minute
	}
	row := r.db.QueryRowContext(ctx, `
		WITH picked AS (
			SELECT id
			FROM admin_plus_scheduler_steps
			WHERE (
				status IN ('queued', 'retryable_failed')
				OR (status = 'running' AND locked_until IS NOT NULL AND locked_until < NOW())
			)
			  AND next_attempt_at <= NOW()
			  AND attempts < max_attempts
			ORDER BY next_attempt_at ASC, id ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		),
		updated AS (
			UPDATE admin_plus_scheduler_steps AS step
			SET status = 'running',
				attempts = attempts + 1,
				locked_by = $1,
				locked_until = NOW() + make_interval(secs => $2),
				started_at = COALESCE(started_at, NOW()),
				updated_at = NOW()
			FROM picked
			WHERE step.id = picked.id
			RETURNING step.id, step.run_id, step.supplier_id, step.supplier_name, step.task_type, step.action,
				step.status, step.schedule_key, step.extension_task_id, step.result_count, step.reason,
				step.attempts, step.max_attempts, step.next_attempt_at, step.locked_by, step.locked_until,
				step.started_at, step.finished_at
		)
		SELECT id, run_id, supplier_id, supplier_name, task_type, action, status, schedule_key,
			extension_task_id, result_count, reason, attempts, max_attempts, next_attempt_at,
			locked_by, locked_until, started_at, finished_at
		FROM updated
	`, workerID, int64(lease.Seconds()))
	step, err := scanStepRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	_, _ = r.db.ExecContext(ctx, `UPDATE admin_plus_scheduler_runs SET status = 'running', started_at = COALESCE(started_at, NOW()), updated_at = NOW() WHERE id = $1 AND status = 'queued'`, step.RunID)
	return step, nil
}

func (r *SQLRepository) CompleteStep(ctx context.Context, stepID int64, status string, resultCount int, reason string, finishedAt time.Time) error {
	if r == nil || r.db == nil {
		return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	errorCode, errorMessage, responseSnapshot := schedulerAttemptDiagnostics(status, resultCount, reason)
	responsePayload, err := json.Marshal(responseSnapshot)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		WITH current_step AS (
			SELECT id, run_id, supplier_id, task_type, locked_by, attempts, started_at
			FROM admin_plus_scheduler_steps
			WHERE id = $1
		),
		updated_step AS (
			UPDATE admin_plus_scheduler_steps AS step
			SET status = $2,
				result_count = $3,
				reason = $4,
				finished_at = $5,
				locked_by = '',
				locked_until = NULL,
				next_attempt_at = CASE WHEN $2 = 'retryable_failed' THEN NOW() + INTERVAL '5 minutes' ELSE next_attempt_at END,
				updated_at = NOW()
			FROM current_step
			WHERE step.id = current_step.id
			RETURNING step.id
		)
		INSERT INTO admin_plus_scheduler_attempts (
			step_id, run_id, supplier_id, task_type, status, worker_id, attempt_no, started_at, finished_at,
			duration_ms, error_code, error_message, response_snapshot
		)
		SELECT current_step.id,
			current_step.run_id,
			current_step.supplier_id,
			current_step.task_type,
			$2,
			current_step.locked_by,
			current_step.attempts,
			current_step.started_at,
			$5,
			CASE WHEN current_step.started_at IS NULL THEN 0 ELSE EXTRACT(EPOCH FROM ($5 - current_step.started_at))::bigint * 1000 END,
			$6,
			$7,
			$8::jsonb
		FROM current_step
	`, stepID, status, resultCount, reason, finishedAt, errorCode, errorMessage, string(responsePayload))
	return err
}

func schedulerAttemptDiagnostics(status string, resultCount int, reason string) (string, string, map[string]any) {
	snapshot := map[string]any{
		"result_count": resultCount,
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", "", snapshot
	}
	snapshot["reason"] = trimLimit(reason, 1800)
	var parsed stepFailureReason
	if json.Unmarshal([]byte(reason), &parsed) == nil {
		snapshot["failure"] = parsed
		code := firstNonEmpty(parsed.LoginCode, parsed.Code)
		message := firstNonEmpty(parsed.LoginMessage, parsed.Message, parsed.RawError)
		return code, trimLimit(message, 900), snapshot
	}
	if status != "retryable_failed" && status != "manual_required" && status != "dead" {
		return "", "", snapshot
	}
	code := firstNonEmpty(reasonCodeFromText(reason), "SCHEDULER_STEP_FAILED")
	return code, trimLimit(reason, 900), snapshot
}

func (r *SQLRepository) RetryStep(ctx context.Context, stepID int64, retryAt time.Time) (*adminplusdomain.SchedulerStepRecord, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_scheduler_steps
		SET status = 'queued',
			reason = '',
			result_count = 0,
			attempts = 0,
			max_attempts = GREATEST(max_attempts, 3),
			next_attempt_at = $2,
			locked_by = '',
			locked_until = NULL,
			started_at = NULL,
			finished_at = NULL,
			updated_at = NOW()
		WHERE id = $1
		  AND status IN ('retryable_failed', 'manual_required', 'dead', 'skipped', 'cancelled')
		RETURNING id, run_id, supplier_id, supplier_name, task_type, action, status, schedule_key,
			extension_task_id, result_count, reason, attempts, max_attempts, next_attempt_at,
			locked_by, locked_until, started_at, finished_at
	`, stepID, retryAt)
	return scanStepRecord(row)
}

func (r *SQLRepository) CancelStep(ctx context.Context, stepID int64, cancelledAt time.Time) (*adminplusdomain.SchedulerStepRecord, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_scheduler_steps
		SET status = 'cancelled',
			reason = 'manual_cancelled',
			locked_by = '',
			locked_until = NULL,
			finished_at = $2,
			updated_at = NOW()
		WHERE id = $1
		  AND status IN ('queued', 'running', 'retryable_failed', 'manual_required')
		RETURNING id, run_id, supplier_id, supplier_name, task_type, action, status, schedule_key,
			extension_task_id, result_count, reason, attempts, max_attempts, next_attempt_at,
			locked_by, locked_until, started_at, finished_at
	`, stepID, cancelledAt)
	return scanStepRecord(row)
}

func (r *SQLRepository) CancelRun(ctx context.Context, runID string, cancelledAt time.Time) (*adminplusdomain.SchedulerRunSummary, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	_, err = tx.ExecContext(ctx, `
		UPDATE admin_plus_scheduler_steps
		SET status = 'cancelled',
			reason = CASE WHEN reason = '' THEN 'run_cancelled' ELSE reason END,
			locked_by = '',
			locked_until = NULL,
			finished_at = $2,
			updated_at = NOW()
		WHERE run_id = $1
		  AND status IN ('queued', 'running', 'retryable_failed', 'manual_required')
	`, runID, cancelledAt)
	if err != nil {
		return nil, err
	}
	row := tx.QueryRowContext(ctx, `
		UPDATE admin_plus_scheduler_runs
		SET status = 'cancelled',
			finished_at = $2,
			error_code = '',
			error_message = 'manual_cancelled',
			updated_at = NOW()
		WHERE id = $1
		  AND status IN ('queued', 'running', 'retryable_failed', 'partial_succeeded', 'manual_required')
		RETURNING id, legacy_run_id, trigger_type, task_type, status, requested_at, started_at, finished_at,
			supplier_count, total_steps, succeeded_steps, failed_steps, skipped_steps, duration_ms,
			error_code, error_message
	`, runID, cancelledAt)
	run, err := scanRunSummary(row)
	if err != nil {
		return nil, err
	}
	err = tx.Commit()
	return run, err
}

func (r *SQLRepository) RetryFailedSteps(ctx context.Context, runID string, retryAt time.Time) (int, error) {
	if r == nil || r.db == nil {
		return 0, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE admin_plus_scheduler_steps
		SET status = 'queued',
			reason = '',
			result_count = 0,
			attempts = 0,
			max_attempts = GREATEST(max_attempts, 3),
			next_attempt_at = $2,
			locked_by = '',
			locked_until = NULL,
			started_at = NULL,
			finished_at = NULL,
			updated_at = NOW()
		WHERE run_id = $1
		  AND status IN ('retryable_failed', 'manual_required', 'dead', 'skipped', 'cancelled')
	`, runID, retryAt)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

func (r *SQLRepository) RefreshRunStatus(ctx context.Context, runID string, finishedAt time.Time) error {
	if r == nil || r.db == nil {
		return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	_, err := r.db.ExecContext(ctx, `
		WITH counts AS (
			SELECT
				COUNT(*)::int AS total,
				COUNT(*) FILTER (WHERE status = 'succeeded')::int AS succeeded,
				COUNT(*) FILTER (WHERE status IN ('retryable_failed', 'manual_required', 'dead'))::int AS failed,
				COUNT(*) FILTER (WHERE status = 'skipped')::int AS skipped,
				COUNT(*) FILTER (WHERE status = 'cancelled')::int AS cancelled,
				COUNT(*) FILTER (WHERE status IN ('queued', 'running'))::int AS active
			FROM admin_plus_scheduler_steps
			WHERE run_id = $1
		)
		UPDATE admin_plus_scheduler_runs AS run
		SET total_steps = counts.total,
			succeeded_steps = counts.succeeded,
			failed_steps = counts.failed,
			skipped_steps = counts.skipped,
			status = CASE
				WHEN counts.total = 0 THEN run.status
				WHEN counts.active > 0 THEN 'running'
				WHEN counts.cancelled = counts.total THEN 'cancelled'
				WHEN counts.failed > 0 AND counts.succeeded > 0 THEN 'partial_succeeded'
				WHEN counts.failed > 0 THEN 'retryable_failed'
				WHEN counts.succeeded = counts.total THEN 'succeeded'
				WHEN counts.skipped = counts.total THEN 'skipped'
				WHEN counts.succeeded > 0 THEN 'partial_succeeded'
				WHEN counts.cancelled > 0 THEN 'cancelled'
				ELSE run.status
			END,
			finished_at = CASE WHEN counts.active = 0 THEN $2 ELSE run.finished_at END,
			duration_ms = CASE
				WHEN counts.active = 0 AND run.started_at IS NOT NULL THEN EXTRACT(EPOCH FROM ($2 - run.started_at))::bigint * 1000
				ELSE run.duration_ms
			END,
			error_code = CASE WHEN counts.failed > 0 THEN 'SCHEDULER_STEP_FAILED' ELSE '' END,
			error_message = CASE WHEN counts.failed > 0 THEN '存在失败或跳过的调度步骤' ELSE '' END,
			updated_at = NOW()
		FROM counts
		WHERE run.id = $1
	`, runID, finishedAt)
	return err
}

func (r *SQLRepository) UpsertActions(ctx context.Context, actions []adminplusdomain.SchedulerAction) error {
	if r == nil || r.db == nil {
		return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
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
	for _, action := range actions {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO admin_plus_scheduler_actions (
				id, supplier_id, supplier_name, severity, status, type, title, reason,
				recommended_operation, created_at, updated_at, resolved_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			ON CONFLICT (id) DO UPDATE SET
				supplier_id = EXCLUDED.supplier_id,
				supplier_name = EXCLUDED.supplier_name,
				severity = EXCLUDED.severity,
				type = EXCLUDED.type,
				title = EXCLUDED.title,
				reason = EXCLUDED.reason,
				recommended_operation = EXCLUDED.recommended_operation,
				updated_at = NOW()
			WHERE admin_plus_scheduler_actions.status NOT IN ('resolved', 'ignored')
		`, action.ID, action.SupplierID, action.SupplierName, action.Severity, action.Status, action.Type,
			action.Title, action.Reason, action.RecommendedOperation, action.CreatedAt, action.UpdatedAt, nullableTime(action.ResolvedAt))
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	return err
}

func (r *SQLRepository) ListActions(ctx context.Context) ([]adminplusdomain.SchedulerAction, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, supplier_id, supplier_name, severity, status, type, title, reason,
			recommended_operation, created_at, updated_at, resolved_at
		FROM admin_plus_scheduler_actions
		WHERE status IN ('open', 'investigating', 'ready_to_execute', 'executing', 'verifying')
		ORDER BY CASE severity WHEN 'critical' THEN 0 WHEN 'warning' THEN 1 ELSE 2 END, created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]adminplusdomain.SchedulerAction, 0)
	for rows.Next() {
		action, err := scanAction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *action)
	}
	return out, rows.Err()
}

func (r *SQLRepository) UpdateActionStatus(ctx context.Context, actionID, status string, resolvedAt *time.Time) (*adminplusdomain.SchedulerAction, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_scheduler_actions
		SET status = $2,
			resolved_at = $3,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, supplier_id, supplier_name, severity, status, type, title, reason,
			recommended_operation, created_at, updated_at, resolved_at
	`, actionID, status, nullableTime(resolvedAt))
	return scanAction(row)
}

func schedulerStepStatus(runStatus string, step adminplusdomain.ScheduledTask) string {
	if runStatus == "queued" {
		if step.Reason != "" {
			return "skipped"
		}
		return "queued"
	}
	if step.Synced {
		return "succeeded"
	}
	if step.Reason != "" {
		if step.Reason == "duplicate" {
			return "skipped"
		}
		return "retryable_failed"
	}
	return "succeeded"
}

func nullableTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return *value
}

func timePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRunSummary(row scanner) (*adminplusdomain.SchedulerRunSummary, error) {
	var run adminplusdomain.SchedulerRunSummary
	var startedAt, finishedAt sql.NullTime
	if err := row.Scan(
		&run.ID,
		&run.LegacyRunID,
		&run.TriggerType,
		&run.TaskType,
		&run.Status,
		&run.RequestedAt,
		&startedAt,
		&finishedAt,
		&run.SupplierCount,
		&run.TotalSteps,
		&run.SucceededSteps,
		&run.FailedSteps,
		&run.SkippedSteps,
		&run.DurationMS,
		&run.ErrorCode,
		&run.ErrorMessage,
	); err != nil {
		return nil, err
	}
	run.StartedAt = timePtr(startedAt)
	run.FinishedAt = timePtr(finishedAt)
	return &run, nil
}

func scanStepRecord(row scanner) (*adminplusdomain.SchedulerStepRecord, error) {
	var step adminplusdomain.SchedulerStepRecord
	var taskType string
	var nextAttemptAt, lockedUntil, startedAt, finishedAt sql.NullTime
	if err := row.Scan(
		&step.ID,
		&step.RunID,
		&step.SupplierID,
		&step.SupplierName,
		&taskType,
		&step.Action,
		&step.Status,
		&step.ScheduleKey,
		&step.ExtensionTaskID,
		&step.ResultCount,
		&step.Reason,
		&step.Attempts,
		&step.MaxAttempts,
		&nextAttemptAt,
		&step.LockedBy,
		&lockedUntil,
		&startedAt,
		&finishedAt,
	); err != nil {
		return nil, err
	}
	step.TaskType = adminplusdomain.ExtensionTaskType(taskType)
	step.NextAttemptAt = timePtr(nextAttemptAt)
	step.LockedUntil = timePtr(lockedUntil)
	step.StartedAt = timePtr(startedAt)
	step.FinishedAt = timePtr(finishedAt)
	return &step, nil
}

func scanAttemptRecord(row scanner) (*adminplusdomain.SchedulerAttemptRecord, error) {
	var attempt adminplusdomain.SchedulerAttemptRecord
	var taskType string
	var startedAt sql.NullTime
	var requestSnapshot, responseSnapshot []byte
	if err := row.Scan(
		&attempt.ID,
		&attempt.StepID,
		&attempt.RunID,
		&attempt.SupplierID,
		&taskType,
		&attempt.Status,
		&attempt.WorkerID,
		&attempt.AttemptNo,
		&startedAt,
		&attempt.FinishedAt,
		&attempt.DurationMS,
		&attempt.ErrorCode,
		&attempt.ErrorMessage,
		&requestSnapshot,
		&responseSnapshot,
	); err != nil {
		return nil, err
	}
	attempt.TaskType = adminplusdomain.ExtensionTaskType(taskType)
	attempt.StartedAt = timePtr(startedAt)
	if len(requestSnapshot) > 0 {
		var payload map[string]any
		if err := json.Unmarshal(requestSnapshot, &payload); err != nil {
			return nil, err
		}
		attempt.RequestSnapshot = payload
	}
	if len(responseSnapshot) > 0 {
		var payload map[string]any
		if err := json.Unmarshal(responseSnapshot, &payload); err != nil {
			return nil, err
		}
		attempt.ResponseSnapshot = payload
	}
	return &attempt, nil
}

func scanPlan(row scanner) (*adminplusdomain.SchedulerPlan, error) {
	var plan adminplusdomain.SchedulerPlan
	var taskTypes []string
	var lastRunAt, nextRunAt sql.NullTime
	if err := row.Scan(
		&plan.ID,
		&plan.Name,
		&plan.TaskType,
		pq.Array(&taskTypes),
		&plan.Status,
		&plan.Scope,
		&plan.FrequencyLabel,
		&plan.IntervalSeconds,
		&plan.WindowMinutes,
		&plan.MisfirePolicy,
		&plan.ConcurrencyPolicy,
		&plan.HighCost,
		&plan.Description,
		&lastRunAt,
		&nextRunAt,
	); err != nil {
		return nil, err
	}
	plan.TaskTypes = taskTypes
	plan.LastRunAt = timePtr(lastRunAt)
	plan.NextRunAt = timePtr(nextRunAt)
	return &plan, nil
}

func scanAction(row scanner) (*adminplusdomain.SchedulerAction, error) {
	var action adminplusdomain.SchedulerAction
	var resolvedAt sql.NullTime
	if err := row.Scan(
		&action.ID,
		&action.SupplierID,
		&action.SupplierName,
		&action.Severity,
		&action.Status,
		&action.Type,
		&action.Title,
		&action.Reason,
		&action.RecommendedOperation,
		&action.CreatedAt,
		&action.UpdatedAt,
		&resolvedAt,
	); err != nil {
		return nil, err
	}
	action.ResolvedAt = timePtr(resolvedAt)
	return &action, nil
}
