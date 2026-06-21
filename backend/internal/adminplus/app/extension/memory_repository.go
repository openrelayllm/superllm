package extension

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type MemoryRepository struct {
	mu     sync.Mutex
	nextID int64
	tasks  map[int64]*adminplusdomain.ExtensionTask
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		nextID: 1,
		tasks:  make(map[int64]*adminplusdomain.ExtensionTask),
	}
}

func (r *MemoryRepository) CreateTask(_ context.Context, task *adminplusdomain.ExtensionTask) (*adminplusdomain.ExtensionTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.createTaskLocked(task), nil
}

func (r *MemoryRepository) CreateTaskIfAbsent(_ context.Context, task *adminplusdomain.ExtensionTask) (*adminplusdomain.ExtensionTask, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.tasks {
		if task.ScheduleKey != "" && existing.ScheduleKey == task.ScheduleKey {
			return cloneMemoryExtensionTask(existing), false, nil
		}
	}
	return r.createTaskLocked(task), true, nil
}

func (r *MemoryRepository) createTaskLocked(task *adminplusdomain.ExtensionTask) *adminplusdomain.ExtensionTask {
	cp := cloneMemoryExtensionTask(task)
	cp.ID = r.nextID
	r.nextID++
	r.tasks[cp.ID] = cp
	return cloneMemoryExtensionTask(cp)
}

func (r *MemoryRepository) ClaimNextTask(_ context.Context, now time.Time, types []adminplusdomain.ExtensionTaskType, lease Lease) (*adminplusdomain.ExtensionTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	allowedTypes := make(map[adminplusdomain.ExtensionTaskType]struct{}, len(types))
	for _, taskType := range types {
		allowedTypes[taskType] = struct{}{}
	}
	var candidate *adminplusdomain.ExtensionTask
	for _, task := range r.tasks {
		if task.Status != adminplusdomain.ExtensionTaskStatusPending {
			continue
		}
		if task.Attempts >= task.MaxAttempts {
			continue
		}
		if task.AvailableAfter.After(now) {
			continue
		}
		if len(allowedTypes) > 0 {
			if _, ok := allowedTypes[task.Type]; !ok {
				continue
			}
		}
		if candidate == nil || task.Priority > candidate.Priority || (task.Priority == candidate.Priority && task.CreatedAt.Before(candidate.CreatedAt)) {
			candidate = task
		}
	}
	if candidate == nil {
		return nil, nil
	}
	candidate.Status = adminplusdomain.ExtensionTaskStatusClaimed
	candidate.DeviceID = lease.DeviceID
	candidate.LeaseToken = lease.Token
	candidate.LeaseExpiresAt = &lease.ExpiresAt
	candidate.LastHeartbeatAt = &now
	candidate.Attempts++
	candidate.UpdatedAt = now
	return cloneMemoryExtensionTask(candidate), nil
}

func (r *MemoryRepository) UpdateTask(_ context.Context, task *adminplusdomain.ExtensionTask) (*adminplusdomain.ExtensionTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.tasks[task.ID]; !ok {
		return nil, infraerrors.New(http.StatusNotFound, "EXTENSION_TASK_NOT_FOUND", "extension task not found")
	}
	cp := cloneMemoryExtensionTask(task)
	r.tasks[cp.ID] = cp
	return cloneMemoryExtensionTask(cp), nil
}

func (r *MemoryRepository) GetTask(_ context.Context, id int64) (*adminplusdomain.ExtensionTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, ok := r.tasks[id]
	if !ok {
		return nil, infraerrors.New(http.StatusNotFound, "EXTENSION_TASK_NOT_FOUND", "extension task not found")
	}
	return cloneMemoryExtensionTask(task), nil
}

func (r *MemoryRepository) ListTasks(_ context.Context, filter TaskFilter) ([]*adminplusdomain.ExtensionTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]*adminplusdomain.ExtensionTask, 0, len(r.tasks))
	for _, task := range r.tasks {
		if filter.SupplierID > 0 && task.SupplierID != filter.SupplierID {
			continue
		}
		if filter.Status != "" && task.Status != filter.Status {
			continue
		}
		if filter.Type != "" && task.Type != filter.Type {
			continue
		}
		items = append(items, cloneMemoryExtensionTask(task))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if filter.Limit > 0 && len(items) > filter.Limit {
		items = items[:filter.Limit]
	}
	return items, nil
}

func cloneMemoryExtensionTask(in *adminplusdomain.ExtensionTask) *adminplusdomain.ExtensionTask {
	if in == nil {
		return nil
	}
	out := *in
	if in.LeaseExpiresAt != nil {
		t := *in.LeaseExpiresAt
		out.LeaseExpiresAt = &t
	}
	if in.LastHeartbeatAt != nil {
		t := *in.LastHeartbeatAt
		out.LastHeartbeatAt = &t
	}
	if in.FinishedAt != nil {
		t := *in.FinishedAt
		out.FinishedAt = &t
	}
	if in.Payload != nil {
		out.Payload = make(map[string]any, len(in.Payload))
		for k, v := range in.Payload {
			out.Payload[k] = v
		}
	}
	if in.Result != nil {
		out.Result = make(map[string]any, len(in.Result))
		for k, v := range in.Result {
			out.Result[k] = v
		}
	}
	return &out
}
