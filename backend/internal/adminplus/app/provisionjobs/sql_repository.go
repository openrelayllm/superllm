package provisionjobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
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

func findActiveGroupJob(ctx context.Context, tx *sql.Tx, supplierID int64, supplierGroupID int64, stepType adminplusdomain.SupplierProvisionStepType) (*adminplusdomain.SupplierProvisionJob, error) {
	job, err := scanProvisionJob(tx.QueryRowContext(ctx, provisionJobSelectSQL()+`
		WHERE id = (
			SELECT job_id
			FROM supplier_provision_steps
			WHERE supplier_id = $1
				AND supplier_group_id = $2
				AND step_type = $3
				AND status IN ('queued', 'running', 'retryable_failed')
			ORDER BY id DESC
			LIMIT 1
		)
	`, supplierID, supplierGroupID, string(stepType)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return job, err
}

func findActiveSupplierJob(ctx context.Context, tx *sql.Tx, supplierID int64, jobType adminplusdomain.SupplierProvisionJobType) (*adminplusdomain.SupplierProvisionJob, error) {
	job, err := scanProvisionJob(tx.QueryRowContext(ctx, provisionJobSelectSQL()+`
		WHERE supplier_id = $1
			AND job_type = $2
			AND status IN ('queued', 'running', 'retryable_failed')
		ORDER BY id DESC
		LIMIT 1
	`, supplierID, string(jobType)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return job, err
}

func (r *SQLRepository) CreateJobWithSteps(ctx context.Context, job *adminplusdomain.SupplierProvisionJob, steps []*adminplusdomain.SupplierProvisionStep, eventType string) (*adminplusdomain.SupplierProvisionJob, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, dbNotConfigured()
	}
	if job == nil {
		return nil, false, infraerrors.New(http.StatusBadRequest, "SUPPLIER_PROVISION_JOB_INVALID", "supplier provision job is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if job.JobType == adminplusdomain.SupplierProvisionJobTypeProvisionAllGroupKeys {
		if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", activeStepLockID(job.SupplierID, 0, adminplusdomain.SupplierProvisionStepProvisionAllGroupKeys)); err != nil {
			return nil, false, err
		}
		activeJob, err := findActiveSupplierJob(ctx, tx, job.SupplierID, job.JobType)
		if err != nil {
			return nil, false, err
		}
		if activeJob != nil {
			if err := tx.Commit(); err != nil {
				return nil, false, err
			}
			committed = true
			steps, err := r.listSteps(ctx, activeJob.ID)
			if err != nil {
				return nil, false, err
			}
			activeJob.Steps = steps
			return activeJob, true, nil
		}
	}
	if len(steps) == 1 && steps[0] != nil && steps[0].SupplierGroupID > 0 {
		step := steps[0]
		if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", activeStepLockID(step.SupplierID, step.SupplierGroupID, step.StepType)); err != nil {
			return nil, false, err
		}
		activeJob, err := findActiveGroupJob(ctx, tx, step.SupplierID, step.SupplierGroupID, step.StepType)
		if err != nil {
			return nil, false, err
		}
		if activeJob != nil {
			if err := tx.Commit(); err != nil {
				return nil, false, err
			}
			committed = true
			steps, err := r.listSteps(ctx, activeJob.ID)
			if err != nil {
				return nil, false, err
			}
			activeJob.Steps = steps
			return activeJob, true, nil
		}
	}

	requestJSON, err := marshalMap(job.RequestSnapshot)
	if err != nil {
		return nil, false, err
	}
	var existingID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO supplier_provision_jobs (
			job_type, supplier_id, status, idempotency_key_hash, requested_by,
			request_snapshot, result_snapshot, total_steps, succeeded_steps, failed_steps,
			manual_required_steps, attempts, max_attempts, next_run_at, locked_by,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, '{}'::jsonb, $7, 0, 0, 0, 0, $8, $9, '', $10, $11)
		ON CONFLICT (job_type, supplier_id, idempotency_key_hash)
		WHERE idempotency_key_hash <> ''
		DO NOTHING
		RETURNING id
	`,
		string(job.JobType),
		job.SupplierID,
		string(job.Status),
		job.IdempotencyKeyHash,
		job.RequestedBy,
		requestJSON,
		job.TotalSteps,
		job.MaxAttempts,
		job.NextRunAt,
		job.CreatedAt,
		job.UpdatedAt,
	).Scan(&existingID)
	replayed := false
	if errors.Is(err, sql.ErrNoRows) {
		replayed = true
		row := tx.QueryRowContext(ctx, `
			SELECT id
			FROM supplier_provision_jobs
			WHERE job_type = $1 AND supplier_id = $2 AND idempotency_key_hash = $3
			LIMIT 1
		`, string(job.JobType), job.SupplierID, job.IdempotencyKeyHash)
		if err := row.Scan(&existingID); err != nil {
			return nil, false, err
		}
	} else if err != nil {
		return nil, false, err
	}

	if !replayed {
		insertedSteps := 0
		for _, step := range steps {
			if step == nil {
				continue
			}
			step.JobID = existingID
			stepJSON, err := marshalMap(step.RequestSnapshot)
			if err != nil {
				return nil, false, err
			}
			res, err := tx.ExecContext(ctx, `
				INSERT INTO supplier_provision_steps (
					job_id, supplier_id, supplier_group_id, step_type, status,
					idempotency_key, external_resource_type, external_resource_id,
					attempts, max_attempts, next_run_at, locked_by,
					request_snapshot, result_snapshot, created_at, updated_at
				)
				VALUES ($1, $2, $3, $4, $5, $6, '', '', 0, $7, $8, '', $9, '{}'::jsonb, $10, $11)
				ON CONFLICT DO NOTHING
			`,
				step.JobID,
				step.SupplierID,
				step.SupplierGroupID,
				string(step.StepType),
				string(step.Status),
				step.IdempotencyKey,
				step.MaxAttempts,
				step.NextRunAt,
				stepJSON,
				step.CreatedAt,
				step.UpdatedAt,
			)
			if err != nil {
				return nil, false, err
			}
			if affected, err := res.RowsAffected(); err == nil {
				insertedSteps += int(affected)
			}
		}
		if insertedSteps != job.TotalSteps {
			if _, err := tx.ExecContext(ctx, `
				UPDATE supplier_provision_jobs
				SET total_steps = $1,
					updated_at = $2
				WHERE id = $3
			`, insertedSteps, job.UpdatedAt, existingID); err != nil {
				return nil, false, err
			}
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO admin_plus_outbox_events (
				event_id, event_type, aggregate_type, aggregate_id, payload,
				status, attempts, available_at, created_at, updated_at
			)
			VALUES ($1, $2, 'supplier_provision_job', $3, $4, 'pending', 0, $5, $6, $7)
			ON CONFLICT (event_id) DO NOTHING
		`,
			eventID(existingID, eventType),
			eventType,
			existingID,
			mustMarshalMap(outboxPayload(existingID, job.SupplierID, job.JobType)),
			job.NextRunAt,
			job.CreatedAt,
			job.UpdatedAt,
		); err != nil {
			return nil, false, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	committed = true
	created, err := r.GetJob(ctx, existingID)
	return created, replayed, err
}

func (r *SQLRepository) GetJob(ctx context.Context, id int64) (*adminplusdomain.SupplierProvisionJob, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	job, err := scanProvisionJob(r.db.QueryRowContext(ctx, provisionJobSelectSQL()+` WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_PROVISION_JOB_NOT_FOUND", "supplier provision job not found")
	}
	if err != nil {
		return nil, err
	}
	steps, err := r.listSteps(ctx, job.ID)
	if err != nil {
		return nil, err
	}
	job.Steps = steps
	return job, nil
}

func (r *SQLRepository) ListJobs(ctx context.Context, filter ListFilter) ([]*adminplusdomain.SupplierProvisionJob, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	where := make([]string, 0, 2)
	args := make([]any, 0, 3)
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
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	query := provisionJobSelectSQL()
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY created_at DESC, id DESC LIMIT " + addArg(filter.Limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	jobs := make([]*adminplusdomain.SupplierProvisionJob, 0)
	for rows.Next() {
		job, err := scanProvisionJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, job := range jobs {
		steps, err := r.listSteps(ctx, job.ID)
		if err != nil {
			return nil, err
		}
		job.Steps = steps
	}
	return jobs, nil
}

func (r *SQLRepository) ClaimNextJob(ctx context.Context, workerID string, now time.Time, lease time.Duration) (*adminplusdomain.SupplierProvisionJob, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	if lease <= 0 {
		lease = defaultLease
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
	job, err := scanProvisionJob(tx.QueryRowContext(ctx, provisionJobSelectSQL()+`
		WHERE id = (
			SELECT id
			FROM supplier_provision_jobs
			WHERE status IN ('queued', 'retryable_failed')
				AND next_run_at <= $1
				AND (locked_until IS NULL OR locked_until <= $1)
			ORDER BY next_run_at ASC, id ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
	`, now))
	if errors.Is(err, sql.ErrNoRows) {
		committed = true
		_ = tx.Commit()
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE supplier_provision_jobs
		SET locked_by = $2,
			locked_until = $3,
			updated_at = $1
		WHERE id = $4
	`, now, workerID, now.Add(lease), job.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	job.LockedBy = workerID
	lockedUntil := now.Add(lease)
	job.LockedUntil = &lockedUntil
	return job, nil
}

func (r *SQLRepository) MarkJobRunning(ctx context.Context, jobID int64, workerID string, now time.Time) error {
	return r.exec(ctx, `
		UPDATE supplier_provision_jobs
		SET status = 'running',
			attempts = attempts + 1,
			locked_by = $2,
			locked_until = $3,
			error_code = '',
			error_message = '',
			updated_at = $1
		WHERE id = $4
	`, now, workerID, now.Add(defaultLease), jobID)
}

func (r *SQLRepository) MarkJobSucceeded(ctx context.Context, jobID int64, result map[string]any, now time.Time) error {
	payload, err := marshalMap(result)
	if err != nil {
		return err
	}
	return r.exec(ctx, `
		UPDATE supplier_provision_jobs
		SET status = 'succeeded',
			result_snapshot = $2,
			succeeded_steps = total_steps,
			failed_steps = 0,
			manual_required_steps = 0,
			locked_by = '',
			locked_until = NULL,
			error_code = '',
			error_message = '',
			finished_at = $1,
			updated_at = $1
		WHERE id = $3
	`, now, payload, jobID)
}

func (r *SQLRepository) MarkJobFailed(ctx context.Context, jobID int64, status adminplusdomain.SupplierProvisionStatus, errorCode string, errorMessage string, nextRunAt time.Time, now time.Time) error {
	return r.exec(ctx, `
		UPDATE supplier_provision_jobs
		SET status = $2,
			succeeded_steps = (
				SELECT COUNT(*)::int
				FROM supplier_provision_steps
				WHERE job_id = $6 AND status = 'succeeded'
			),
			failed_steps = (
				SELECT COUNT(*)::int
				FROM supplier_provision_steps
				WHERE job_id = $6 AND status IN ('retryable_failed', 'dead')
			),
			manual_required_steps = (
				SELECT COUNT(*)::int
				FROM supplier_provision_steps
				WHERE job_id = $6 AND status = 'manual_required'
			),
			next_run_at = $3,
			locked_by = '',
			locked_until = NULL,
			error_code = $4,
			error_message = $5,
			finished_at = CASE WHEN $2 IN ('dead', 'cancelled', 'manual_required', 'partial_succeeded') THEN $1 ELSE finished_at END,
			updated_at = $1
		WHERE id = $6
	`, now, string(status), nextRunAt, errorCode, errorMessage, jobID)
}

func (r *SQLRepository) ListRunnableSteps(ctx context.Context, jobID int64, now time.Time) ([]*adminplusdomain.SupplierProvisionStep, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	rows, err := r.db.QueryContext(ctx, provisionStepSelectSQL()+`
		WHERE job_id = $1
			AND status IN ('queued', 'retryable_failed')
			AND next_run_at <= $2
			AND (locked_until IS NULL OR locked_until <= $2)
		ORDER BY id ASC
	`, jobID, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	steps := make([]*adminplusdomain.SupplierProvisionStep, 0)
	for rows.Next() {
		step, err := scanProvisionStep(rows)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (r *SQLRepository) ListJobSteps(ctx context.Context, jobID int64) ([]*adminplusdomain.SupplierProvisionStep, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	return r.listSteps(ctx, jobID)
}

func (r *SQLRepository) MarkStepRunning(ctx context.Context, stepID int64, workerID string, now time.Time) error {
	return r.exec(ctx, `
		UPDATE supplier_provision_steps
		SET status = 'running',
			attempts = attempts + 1,
			locked_by = $2,
			locked_until = $3,
			error_code = '',
			error_message = '',
			updated_at = $1
		WHERE id = $4
	`, now, workerID, now.Add(defaultLease), stepID)
}

func (r *SQLRepository) MarkStepSucceeded(ctx context.Context, stepID int64, result map[string]any, now time.Time) error {
	payload, err := marshalMap(result)
	if err != nil {
		return err
	}
	return r.exec(ctx, `
		UPDATE supplier_provision_steps
		SET status = 'succeeded',
			result_snapshot = $2,
			locked_by = '',
			locked_until = NULL,
			error_code = '',
			error_message = '',
			finished_at = $1,
			updated_at = $1
		WHERE id = $3
	`, now, payload, stepID)
}

func (r *SQLRepository) MarkStepFailed(ctx context.Context, stepID int64, status adminplusdomain.SupplierProvisionStatus, errorCode string, errorMessage string, nextRunAt time.Time, now time.Time) error {
	return r.exec(ctx, `
		UPDATE supplier_provision_steps
		SET status = $2,
			next_run_at = $3,
			locked_by = '',
			locked_until = NULL,
			error_code = $4,
			error_message = $5,
			finished_at = CASE WHEN $2 IN ('dead', 'manual_required', 'skipped') THEN $1 ELSE finished_at END,
			updated_at = $1
		WHERE id = $6
	`, now, string(status), nextRunAt, errorCode, errorMessage, stepID)
}

func (r *SQLRepository) RecordAttempt(ctx context.Context, attempt Attempt) error {
	if r == nil || r.db == nil {
		return dbNotConfigured()
	}
	requestJSON, err := marshalMap(attempt.RequestSnapshot)
	if err != nil {
		return err
	}
	responseJSON, err := marshalMap(attempt.ResponseSnapshot)
	if err != nil {
		return err
	}
	var stepID any
	if attempt.StepID > 0 {
		stepID = attempt.StepID
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO supplier_provision_attempts (
			job_id, step_id, supplier_id, supplier_group_id,
			operation, status, request_snapshot, response_snapshot,
			error_code, error_message, duration_ms
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, attempt.JobID, stepID, attempt.SupplierID, attempt.SupplierGroupID, attempt.Operation, attempt.Status, requestJSON, responseJSON, attempt.ErrorCode, attempt.ErrorMessage, attempt.DurationMS)
	return err
}

func (r *SQLRepository) ListPendingOutboxEvents(ctx context.Context, limit int, now time.Time) ([]OutboxEvent, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT event_id, event_type, aggregate_type, aggregate_id, payload
		FROM admin_plus_outbox_events
		WHERE status IN ('pending', 'failed')
			AND available_at <= $1
		ORDER BY available_at ASC, created_at ASC
		LIMIT $2
	`, now, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	events := make([]OutboxEvent, 0)
	for rows.Next() {
		event, err := scanOutboxEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *SQLRepository) MarkOutboxPublished(ctx context.Context, eventID string, now time.Time) error {
	return r.exec(ctx, `
		UPDATE admin_plus_outbox_events
		SET status = 'published',
			published_at = $2,
			updated_at = $2
		WHERE event_id = $1
	`, eventID, now)
}

func (r *SQLRepository) MarkOutboxFailed(ctx context.Context, eventID string, now time.Time, err error) error {
	return r.exec(ctx, `
		UPDATE admin_plus_outbox_events
		SET status = 'failed',
			attempts = attempts + 1,
			available_at = $2,
			updated_at = $3
		WHERE event_id = $1
	`, eventID, now.Add(30*time.Second), now)
}

func (r *SQLRepository) listSteps(ctx context.Context, jobID int64) ([]*adminplusdomain.SupplierProvisionStep, error) {
	rows, err := r.db.QueryContext(ctx, provisionStepSelectSQL()+`
		WHERE job_id = $1
		ORDER BY id ASC
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	steps := make([]*adminplusdomain.SupplierProvisionStep, 0)
	for rows.Next() {
		step, err := scanProvisionStep(rows)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (r *SQLRepository) exec(ctx context.Context, query string, args ...any) error {
	if r == nil || r.db == nil {
		return dbNotConfigured()
	}
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func provisionJobSelectSQL() string {
	return `
		SELECT id, job_type, supplier_id, status, idempotency_key_hash, requested_by,
			request_snapshot, result_snapshot, total_steps, succeeded_steps, failed_steps,
			manual_required_steps, attempts, max_attempts, next_run_at, locked_by,
			locked_until, error_code, error_message, created_at, updated_at, finished_at
		FROM supplier_provision_jobs
	`
}

func provisionStepSelectSQL() string {
	return `
		SELECT id, job_id, supplier_id, supplier_group_id, step_type, status,
			idempotency_key, external_resource_type, external_resource_id,
			attempts, max_attempts, next_run_at, locked_by, locked_until,
			error_code, error_message, request_snapshot, result_snapshot,
			created_at, updated_at, finished_at
		FROM supplier_provision_steps
	`
}

func scanProvisionJob(scanner rowScanner) (*adminplusdomain.SupplierProvisionJob, error) {
	var job adminplusdomain.SupplierProvisionJob
	var jobType, status string
	var requestRaw, resultRaw []byte
	var lockedUntil sql.NullTime
	var finishedAt sql.NullTime
	err := scanner.Scan(
		&job.ID,
		&jobType,
		&job.SupplierID,
		&status,
		&job.IdempotencyKeyHash,
		&job.RequestedBy,
		&requestRaw,
		&resultRaw,
		&job.TotalSteps,
		&job.SucceededSteps,
		&job.FailedSteps,
		&job.ManualRequiredSteps,
		&job.Attempts,
		&job.MaxAttempts,
		&job.NextRunAt,
		&job.LockedBy,
		&lockedUntil,
		&job.ErrorCode,
		&job.ErrorMessage,
		&job.CreatedAt,
		&job.UpdatedAt,
		&finishedAt,
	)
	if err != nil {
		return nil, err
	}
	job.JobType = adminplusdomain.SupplierProvisionJobType(jobType)
	job.Status = adminplusdomain.SupplierProvisionStatus(status)
	job.RequestSnapshot = unmarshalMap(requestRaw)
	job.ResultSnapshot = unmarshalMap(resultRaw)
	if lockedUntil.Valid {
		value := lockedUntil.Time
		job.LockedUntil = &value
	}
	if finishedAt.Valid {
		value := finishedAt.Time
		job.FinishedAt = &value
	}
	return &job, nil
}

func scanProvisionStep(scanner rowScanner) (*adminplusdomain.SupplierProvisionStep, error) {
	var step adminplusdomain.SupplierProvisionStep
	var stepType, status string
	var requestRaw, resultRaw []byte
	var lockedUntil sql.NullTime
	var finishedAt sql.NullTime
	err := scanner.Scan(
		&step.ID,
		&step.JobID,
		&step.SupplierID,
		&step.SupplierGroupID,
		&stepType,
		&status,
		&step.IdempotencyKey,
		&step.ExternalResourceType,
		&step.ExternalResourceID,
		&step.Attempts,
		&step.MaxAttempts,
		&step.NextRunAt,
		&step.LockedBy,
		&lockedUntil,
		&step.ErrorCode,
		&step.ErrorMessage,
		&requestRaw,
		&resultRaw,
		&step.CreatedAt,
		&step.UpdatedAt,
		&finishedAt,
	)
	if err != nil {
		return nil, err
	}
	step.StepType = adminplusdomain.SupplierProvisionStepType(stepType)
	step.Status = adminplusdomain.SupplierProvisionStatus(status)
	step.RequestSnapshot = unmarshalMap(requestRaw)
	step.ResultSnapshot = unmarshalMap(resultRaw)
	if lockedUntil.Valid {
		value := lockedUntil.Time
		step.LockedUntil = &value
	}
	if finishedAt.Valid {
		value := finishedAt.Time
		step.FinishedAt = &value
	}
	return &step, nil
}

func scanOutboxEvent(scanner rowScanner) (OutboxEvent, error) {
	var event OutboxEvent
	var raw []byte
	err := scanner.Scan(&event.EventID, &event.EventType, &event.AggregateType, &event.AggregateID, &raw)
	if err != nil {
		return OutboxEvent{}, err
	}
	event.Payload = unmarshalMap(raw)
	return event, nil
}

func marshalMap(value map[string]any) ([]byte, error) {
	if len(value) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(value)
}

func mustMarshalMap(value map[string]any) []byte {
	payload, err := marshalMap(value)
	if err != nil {
		return []byte("{}")
	}
	return payload
}

func unmarshalMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&out); err != nil {
		return map[string]any{}
	}
	return out
}

func eventID(jobID int64, eventType string) string {
	return fmt.Sprintf("supplier-provision:%s:%d", eventType, jobID)
}

func outboxPayload(jobID int64, supplierID int64, jobType adminplusdomain.SupplierProvisionJobType) map[string]any {
	return map[string]any{
		"job_id":      jobID,
		"supplier_id": supplierID,
		"job_type":    string(jobType),
	}
}

func activeStepLockID(supplierID int64, supplierGroupID int64, stepType adminplusdomain.SupplierProvisionStepType) int64 {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "supplier-provision:%d:%d:%s", supplierID, supplierGroupID, stepType)
	return int64(h.Sum64())
}

func dbNotConfigured() error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_DB_NOT_CONFIGURED", "admin plus database is not configured")
}
