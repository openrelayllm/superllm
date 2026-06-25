package extension

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
	"github.com/lib/pq"
)

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) CreateTask(ctx context.Context, task *adminplusdomain.ExtensionTask) (*adminplusdomain.ExtensionTask, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	payload, err := marshalJSONMap(task.Payload)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_extension_tasks (
			supplier_id, type, schedule_key, status, priority, attempts, max_attempts,
			device_id, lease_token, lease_expires_at, last_heartbeat_at,
			available_after, payload, result, error_code, error_message,
			created_at, updated_at, finished_at
		)
		VALUES (NULLIF($1, 0), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, '{}'::jsonb, $14, $15, $16, $17, $18)
		RETURNING id, supplier_id, type, schedule_key, status, priority, attempts, max_attempts,
			device_id, lease_token, lease_expires_at, last_heartbeat_at,
			available_after, payload, result, error_code, error_message,
			created_at, updated_at, finished_at
	`,
		task.SupplierID,
		string(task.Type),
		task.ScheduleKey,
		string(task.Status),
		task.Priority,
		task.Attempts,
		task.MaxAttempts,
		task.DeviceID,
		task.LeaseToken,
		nullableTime(task.LeaseExpiresAt),
		nullableTime(task.LastHeartbeatAt),
		task.AvailableAfter,
		payload,
		task.ErrorCode,
		task.ErrorMessage,
		task.CreatedAt,
		task.UpdatedAt,
		nullableTime(task.FinishedAt),
	)
	return scanExtensionTask(row)
}

func (r *SQLRepository) CreateTaskIfAbsent(ctx context.Context, task *adminplusdomain.ExtensionTask) (*adminplusdomain.ExtensionTask, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, dbNotConfigured()
	}
	payload, err := marshalJSONMap(task.Payload)
	if err != nil {
		return nil, false, err
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_extension_tasks (
			supplier_id, type, schedule_key, status, priority, attempts, max_attempts,
			device_id, lease_token, lease_expires_at, last_heartbeat_at,
			available_after, payload, result, error_code, error_message,
			created_at, updated_at, finished_at
		)
		VALUES (NULLIF($1, 0), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, '{}'::jsonb, $14, $15, $16, $17, $18)
		ON CONFLICT (schedule_key) WHERE schedule_key <> '' DO NOTHING
		RETURNING id, supplier_id, type, schedule_key, status, priority, attempts, max_attempts,
			device_id, lease_token, lease_expires_at, last_heartbeat_at,
			available_after, payload, result, error_code, error_message,
			created_at, updated_at, finished_at
	`,
		task.SupplierID,
		string(task.Type),
		task.ScheduleKey,
		string(task.Status),
		task.Priority,
		task.Attempts,
		task.MaxAttempts,
		task.DeviceID,
		task.LeaseToken,
		nullableTime(task.LeaseExpiresAt),
		nullableTime(task.LastHeartbeatAt),
		task.AvailableAfter,
		payload,
		task.ErrorCode,
		task.ErrorMessage,
		task.CreatedAt,
		task.UpdatedAt,
		nullableTime(task.FinishedAt),
	)
	created, err := scanExtensionTask(row)
	if err == nil {
		return created, true, nil
	}
	if err != sql.ErrNoRows {
		return nil, false, err
	}
	existing, err := r.GetTaskByScheduleKey(ctx, task.ScheduleKey)
	return existing, false, err
}

func (r *SQLRepository) ClaimNextTask(ctx context.Context, now time.Time, types []adminplusdomain.ExtensionTaskType, lease Lease) (*adminplusdomain.ExtensionTask, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	typeValues := make([]string, 0, len(types))
	for _, taskType := range types {
		typeValues = append(typeValues, string(taskType))
	}
	row := r.db.QueryRowContext(ctx, `
		WITH next_task AS (
			SELECT id
			FROM admin_plus_extension_tasks
			WHERE status = 'pending'
				AND attempts < max_attempts
				AND available_after <= $1
				AND (COALESCE(array_length($2::text[], 1), 0) = 0 OR type = ANY($2::text[]))
			ORDER BY priority DESC, created_at ASC, id ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE admin_plus_extension_tasks t
		SET status = 'claimed',
			device_id = $3,
			lease_token = $4,
			lease_expires_at = $5,
			last_heartbeat_at = $1,
			attempts = attempts + 1,
			updated_at = $1
		FROM next_task
		WHERE t.id = next_task.id
		RETURNING t.id, t.supplier_id, t.type, t.schedule_key, t.status, t.priority, t.attempts, t.max_attempts,
			t.device_id, t.lease_token, t.lease_expires_at, t.last_heartbeat_at,
			t.available_after, t.payload, t.result, t.error_code, t.error_message,
			t.created_at, t.updated_at, t.finished_at
	`, now, pq.Array(typeValues), lease.DeviceID, lease.Token, lease.ExpiresAt)
	task, err := scanExtensionTask(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return task, err
}

func (r *SQLRepository) UpdateTask(ctx context.Context, task *adminplusdomain.ExtensionTask) (*adminplusdomain.ExtensionTask, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	payload, err := marshalJSONMap(task.Payload)
	if err != nil {
		return nil, err
	}
	result, err := marshalJSONMap(task.Result)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_extension_tasks
		SET status = $2,
			priority = $3,
			attempts = $4,
			max_attempts = $5,
			device_id = $6,
			lease_token = $7,
			lease_expires_at = $8,
			last_heartbeat_at = $9,
			available_after = $10,
			payload = $11,
			result = $12,
			error_code = $13,
			error_message = $14,
			updated_at = $15,
			finished_at = $16
		WHERE id = $1
		RETURNING id, supplier_id, type, schedule_key, status, priority, attempts, max_attempts,
			device_id, lease_token, lease_expires_at, last_heartbeat_at,
			available_after, payload, result, error_code, error_message,
			created_at, updated_at, finished_at
	`,
		task.ID,
		string(task.Status),
		task.Priority,
		task.Attempts,
		task.MaxAttempts,
		task.DeviceID,
		task.LeaseToken,
		nullableTime(task.LeaseExpiresAt),
		nullableTime(task.LastHeartbeatAt),
		task.AvailableAfter,
		payload,
		result,
		task.ErrorCode,
		task.ErrorMessage,
		task.UpdatedAt,
		nullableTime(task.FinishedAt),
	)
	updated, err := scanExtensionTask(row)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "EXTENSION_TASK_NOT_FOUND", "extension task not found")
	}
	return updated, err
}

func (r *SQLRepository) GetTask(ctx context.Context, id int64) (*adminplusdomain.ExtensionTask, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, supplier_id, type, schedule_key, status, priority, attempts, max_attempts,
			device_id, lease_token, lease_expires_at, last_heartbeat_at,
			available_after, payload, result, error_code, error_message,
			created_at, updated_at, finished_at
		FROM admin_plus_extension_tasks
		WHERE id = $1
	`, id)
	task, err := scanExtensionTask(row)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "EXTENSION_TASK_NOT_FOUND", "extension task not found")
	}
	return task, err
}

func (r *SQLRepository) GetTaskByScheduleKey(ctx context.Context, scheduleKey string) (*adminplusdomain.ExtensionTask, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, supplier_id, type, schedule_key, status, priority, attempts, max_attempts,
			device_id, lease_token, lease_expires_at, last_heartbeat_at,
			available_after, payload, result, error_code, error_message,
			created_at, updated_at, finished_at
		FROM admin_plus_extension_tasks
		WHERE schedule_key = $1 AND schedule_key <> ''
	`, scheduleKey)
	task, err := scanExtensionTask(row)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "EXTENSION_TASK_NOT_FOUND", "extension task not found")
	}
	return task, err
}

func (r *SQLRepository) ListTasks(ctx context.Context, filter TaskFilter) ([]*adminplusdomain.ExtensionTask, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	where := []string{"1=1"}
	args := make([]any, 0, 4)
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
	if filter.Type != "" {
		where = append(where, "type = "+addArg(string(filter.Type)))
	}
	limitRef := addArg(filter.Limit)
	query := `
		SELECT id, supplier_id, type, schedule_key, status, priority, attempts, max_attempts,
			device_id, lease_token, lease_expires_at, last_heartbeat_at,
			available_after, payload, result, error_code, error_message,
			created_at, updated_at, finished_at
		FROM admin_plus_extension_tasks
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY created_at DESC, id DESC
		LIMIT ` + limitRef

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]*adminplusdomain.ExtensionTask, 0)
	for rows.Next() {
		item, err := scanExtensionTask(rows)
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

type extensionTaskScanner interface {
	Scan(dest ...any) error
}

func scanExtensionTask(scanner extensionTaskScanner) (*adminplusdomain.ExtensionTask, error) {
	var task adminplusdomain.ExtensionTask
	var supplierID sql.NullInt64
	var taskType, scheduleKey, status string
	var leaseExpiresAt, lastHeartbeatAt, finishedAt sql.NullTime
	var payload, result []byte
	err := scanner.Scan(
		&task.ID,
		&supplierID,
		&taskType,
		&scheduleKey,
		&status,
		&task.Priority,
		&task.Attempts,
		&task.MaxAttempts,
		&task.DeviceID,
		&task.LeaseToken,
		&leaseExpiresAt,
		&lastHeartbeatAt,
		&task.AvailableAfter,
		&payload,
		&result,
		&task.ErrorCode,
		&task.ErrorMessage,
		&task.CreatedAt,
		&task.UpdatedAt,
		&finishedAt,
	)
	if err != nil {
		return nil, err
	}
	if supplierID.Valid {
		task.SupplierID = supplierID.Int64
	}
	task.Type = adminplusdomain.ExtensionTaskType(taskType)
	task.ScheduleKey = scheduleKey
	task.Status = adminplusdomain.ExtensionTaskStatus(status)
	if leaseExpiresAt.Valid {
		t := leaseExpiresAt.Time
		task.LeaseExpiresAt = &t
	}
	if lastHeartbeatAt.Valid {
		t := lastHeartbeatAt.Time
		task.LastHeartbeatAt = &t
	}
	if finishedAt.Valid {
		t := finishedAt.Time
		task.FinishedAt = &t
	}
	if len(payload) > 0 {
		var decoded map[string]any
		if err := json.Unmarshal(payload, &decoded); err != nil {
			return nil, err
		}
		task.Payload = decoded
	}
	if len(result) > 0 {
		var decoded map[string]any
		if err := json.Unmarshal(result, &decoded); err != nil {
			return nil, err
		}
		task.Result = decoded
	}
	return &task, nil
}

func marshalJSONMap(value map[string]any) ([]byte, error) {
	if len(value) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(value)
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
