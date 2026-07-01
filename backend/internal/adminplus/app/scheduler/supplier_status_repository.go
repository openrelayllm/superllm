package scheduler

import (
	"context"
	"net/http"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (r *SQLRepository) SupplierLatestSteps(ctx context.Context) (map[int64]map[adminplusdomain.ExtensionTaskType]adminplusdomain.SchedulerStepRecord, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_SCHEDULER_DB_NOT_CONFIGURED", "admin plus scheduler database is not configured")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT ON (supplier_id, task_type)
			id, run_id, supplier_id, supplier_name, task_type, action, status, schedule_key,
			extension_task_id, result_count, reason, attempts, max_attempts, next_attempt_at,
			locked_by, locked_until, request_snapshot, result_snapshot, started_at, finished_at
		FROM admin_plus_scheduler_steps
		WHERE supplier_id > 0
		ORDER BY supplier_id, task_type, COALESCE(finished_at, started_at, next_attempt_at) DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[int64]map[adminplusdomain.ExtensionTaskType]adminplusdomain.SchedulerStepRecord)
	for rows.Next() {
		step, err := scanStepRecord(rows)
		if err != nil {
			return nil, err
		}
		if out[step.SupplierID] == nil {
			out[step.SupplierID] = make(map[adminplusdomain.ExtensionTaskType]adminplusdomain.SchedulerStepRecord)
		}
		out[step.SupplierID][step.TaskType] = *step
	}
	return out, rows.Err()
}
