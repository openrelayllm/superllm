package extension

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	defaultLeaseTTL    = 5 * time.Minute
	defaultMaxAttempts = 3
)

type CreateTaskInput struct {
	SupplierID     int64
	Type           adminplusdomain.ExtensionTaskType
	ScheduleKey    string
	Priority       int
	MaxAttempts    int
	AvailableAfter *time.Time
	Payload        map[string]any
}

type ClaimTaskInput struct {
	DeviceID string
	Types    []adminplusdomain.ExtensionTaskType
	LeaseTTL time.Duration
}

type CreateLeasedTaskInput struct {
	SupplierID int64
	Type       adminplusdomain.ExtensionTaskType
	DeviceID   string
	Payload    map[string]any
	LeaseTTL   time.Duration
}

type HeartbeatInput struct {
	TaskID     int64
	DeviceID   string
	LeaseToken string
	LeaseTTL   time.Duration
}

type CompleteTaskInput struct {
	TaskID     int64
	DeviceID   string
	LeaseToken string
	Result     map[string]any
}

type FailTaskInput struct {
	TaskID       int64
	DeviceID     string
	LeaseToken   string
	ErrorCode    string
	ErrorMessage string
	RetryAfter   *time.Time
}

type BrowserCredentialInput struct {
	TaskID     int64
	DeviceID   string
	LeaseToken string
}

type CancelTaskInput struct {
	TaskID int64
	Reason string
}

type TaskFilter struct {
	SupplierID int64
	Status     adminplusdomain.ExtensionTaskStatus
	Type       adminplusdomain.ExtensionTaskType
	Limit      int
}

type BrowserCredentialProvider interface {
	GetBrowserCredential(ctx context.Context, supplierID int64) (*adminplusdomain.SupplierBrowserCredential, error)
}

type FailureProcessor interface {
	ProcessTaskFailure(ctx context.Context, task *adminplusdomain.ExtensionTask, errorCode string, errorMessage string) (map[string]any, error)
}

type Repository interface {
	CreateTask(ctx context.Context, task *adminplusdomain.ExtensionTask) (*adminplusdomain.ExtensionTask, error)
	CreateTaskIfAbsent(ctx context.Context, task *adminplusdomain.ExtensionTask) (*adminplusdomain.ExtensionTask, bool, error)
	ClaimNextTask(ctx context.Context, now time.Time, types []adminplusdomain.ExtensionTaskType, lease Lease) (*adminplusdomain.ExtensionTask, error)
	UpdateTask(ctx context.Context, task *adminplusdomain.ExtensionTask) (*adminplusdomain.ExtensionTask, error)
	GetTask(ctx context.Context, id int64) (*adminplusdomain.ExtensionTask, error)
	ListTasks(ctx context.Context, filter TaskFilter) ([]*adminplusdomain.ExtensionTask, error)
}

type Lease struct {
	DeviceID  string
	Token     string
	ExpiresAt time.Time
}

type Service struct {
	repo            Repository
	resultProcessor ResultProcessor
	credentials     BrowserCredentialProvider
	bizlog          *bizlogs.Recorder
	now             func() time.Time
	newToken        func() (string, error)
}

func NewService(repo Repository) *Service {
	return &Service{
		repo:     repo,
		now:      time.Now,
		newToken: randomToken,
	}
}

func NewServiceWithResultProcessor(repo Repository, processor ResultProcessor) *Service {
	service := NewService(repo)
	service.resultProcessor = processor
	return service
}

func NewServiceWithDependencies(repo Repository, processor ResultProcessor, credentials BrowserCredentialProvider) *Service {
	service := NewServiceWithResultProcessor(repo, processor)
	service.credentials = credentials
	return service
}

func (s *Service) WithDiagnostics(recorder *bizlogs.Recorder) *Service {
	if s != nil {
		s.bizlog = recorder
	}
	return s
}

func (s *Service) CreateTask(ctx context.Context, in CreateTaskInput) (*adminplusdomain.ExtensionTask, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("extension task service is not configured")
	}
	task, err := s.buildTask(in)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateTask(ctx, task)
}

func (s *Service) CreateTaskIfAbsent(ctx context.Context, in CreateTaskInput) (*adminplusdomain.ExtensionTask, bool, error) {
	if s == nil || s.repo == nil {
		return nil, false, internalError("extension task service is not configured")
	}
	task, err := s.buildTask(in)
	if err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(task.ScheduleKey) == "" {
		return nil, false, badRequest("EXTENSION_TASK_SCHEDULE_KEY_REQUIRED", "schedule key is required")
	}
	return s.repo.CreateTaskIfAbsent(ctx, task)
}

func (s *Service) CreateLeasedTask(ctx context.Context, in CreateLeasedTaskInput) (*adminplusdomain.ExtensionTask, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("extension task service is not configured")
	}
	deviceID := strings.TrimSpace(in.DeviceID)
	if deviceID == "" {
		return nil, badRequest("EXTENSION_DEVICE_ID_REQUIRED", "extension device id is required")
	}
	task, err := s.buildTask(CreateTaskInput{
		SupplierID:  in.SupplierID,
		Type:        in.Type,
		MaxAttempts: 1,
		Payload:     in.Payload,
	})
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	token, err := s.newToken()
	if err != nil {
		return nil, internalError("failed to generate extension task lease token")
	}
	expiresAt := now.Add(normalizeLeaseTTL(in.LeaseTTL))
	task.Status = adminplusdomain.ExtensionTaskStatusClaimed
	task.DeviceID = deviceID
	task.LeaseToken = token
	task.LeaseExpiresAt = &expiresAt
	task.LastHeartbeatAt = &now
	task.Attempts = 1
	task.UpdatedAt = now
	return s.repo.CreateTask(ctx, task)
}

func (s *Service) ClaimTask(ctx context.Context, in ClaimTaskInput) (*adminplusdomain.ExtensionTask, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("extension task service is not configured")
	}
	deviceID := strings.TrimSpace(in.DeviceID)
	if deviceID == "" {
		return nil, badRequest("EXTENSION_DEVICE_ID_REQUIRED", "extension device id is required")
	}
	for _, taskType := range in.Types {
		if !taskType.Valid() {
			return nil, badRequest("EXTENSION_TASK_TYPE_INVALID", "invalid extension task type")
		}
	}
	now := s.now().UTC()
	token, err := s.newToken()
	if err != nil {
		return nil, internalError("failed to generate extension task lease token")
	}
	task, err := s.repo.ClaimNextTask(ctx, now, in.Types, Lease{
		DeviceID:  deviceID,
		Token:     token,
		ExpiresAt: now.Add(normalizeLeaseTTL(in.LeaseTTL)),
	})
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, infraerrors.New(http.StatusNotFound, "EXTENSION_TASK_NOT_AVAILABLE", "no claimable extension task")
	}
	return task, nil
}

func (s *Service) buildTask(in CreateTaskInput) (*adminplusdomain.ExtensionTask, error) {
	if in.SupplierID <= 0 && in.Type != adminplusdomain.ExtensionTaskTypeRegisterSupplier {
		return nil, badRequest("EXTENSION_TASK_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if !in.Type.Valid() {
		return nil, badRequest("EXTENSION_TASK_TYPE_INVALID", "invalid extension task type")
	}
	maxAttempts := in.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}
	availableAfter := s.now().UTC()
	if in.AvailableAfter != nil {
		availableAfter = in.AvailableAfter.UTC()
	}
	now := s.now().UTC()
	return &adminplusdomain.ExtensionTask{
		SupplierID:     in.SupplierID,
		Type:           in.Type,
		ScheduleKey:    trimLimit(in.ScheduleKey, 200),
		Status:         adminplusdomain.ExtensionTaskStatusPending,
		Priority:       in.Priority,
		MaxAttempts:    maxAttempts,
		AvailableAfter: availableAfter,
		Payload:        in.Payload,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func (s *Service) Heartbeat(ctx context.Context, in HeartbeatInput) (*adminplusdomain.ExtensionTask, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("extension task service is not configured")
	}
	task, err := s.requireLeasedTask(ctx, in.TaskID, in.DeviceID, in.LeaseToken)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	task.Status = adminplusdomain.ExtensionTaskStatusRunning
	task.LastHeartbeatAt = &now
	task.UpdatedAt = now
	expiresAt := now.Add(normalizeLeaseTTL(in.LeaseTTL))
	task.LeaseExpiresAt = &expiresAt
	return s.repo.UpdateTask(ctx, task)
}

func (s *Service) CompleteTask(ctx context.Context, in CompleteTaskInput) (*adminplusdomain.ExtensionTask, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("extension task service is not configured")
	}
	task, err := s.requireLeasedTask(ctx, in.TaskID, in.DeviceID, in.LeaseToken)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	result := in.Result
	if s.resultProcessor != nil {
		ingest, err := s.resultProcessor.ProcessTaskResult(ctx, task, result)
		if err != nil {
			return nil, ingestError(err)
		}
		result = mergeIngestResult(result, ingest)
	}
	task.Status = adminplusdomain.ExtensionTaskStatusSucceeded
	task.Result = result
	task.UpdatedAt = now
	task.FinishedAt = &now
	updated, err := s.repo.UpdateTask(ctx, task)
	if err != nil {
		s.recordTaskFailure(ctx, task, "complete_task", err, "")
		return nil, err
	}
	s.recordTaskSuccess(ctx, updated, "complete_task")
	return updated, nil
}

func (s *Service) FailTask(ctx context.Context, in FailTaskInput) (*adminplusdomain.ExtensionTask, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("extension task service is not configured")
	}
	task, err := s.requireLeasedTask(ctx, in.TaskID, in.DeviceID, in.LeaseToken)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	task.ErrorCode = trimLimit(in.ErrorCode, 80)
	task.ErrorMessage = trimLimit(in.ErrorMessage, 1000)
	if processor, ok := s.resultProcessor.(FailureProcessor); ok {
		ingest, err := processor.ProcessTaskFailure(ctx, task, task.ErrorCode, task.ErrorMessage)
		if err != nil {
			return nil, ingestError(err)
		}
		task.Result = mergeIngestResult(task.Result, ingest)
	}
	task.UpdatedAt = now
	if task.Attempts >= task.MaxAttempts {
		task.Status = adminplusdomain.ExtensionTaskStatusFailed
		task.FinishedAt = &now
		updated, err := s.repo.UpdateTask(ctx, task)
		if err != nil {
			s.recordTaskFailure(ctx, task, "fail_task", err, task.ErrorCode)
			return nil, err
		}
		s.recordTaskReportedFailure(ctx, updated, true)
		return updated, nil
	}
	task.Status = adminplusdomain.ExtensionTaskStatusPending
	task.DeviceID = ""
	task.LeaseToken = ""
	task.LeaseExpiresAt = nil
	task.LastHeartbeatAt = nil
	task.AvailableAfter = now
	if in.RetryAfter != nil {
		task.AvailableAfter = in.RetryAfter.UTC()
	}
	updated, err := s.repo.UpdateTask(ctx, task)
	if err != nil {
		s.recordTaskFailure(ctx, task, "fail_task", err, task.ErrorCode)
		return nil, err
	}
	s.recordTaskReportedFailure(ctx, updated, false)
	return updated, nil
}

func (s *Service) CancelTask(ctx context.Context, in CancelTaskInput) (*adminplusdomain.ExtensionTask, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("extension task service is not configured")
	}
	if in.TaskID <= 0 {
		return nil, badRequest("EXTENSION_TASK_ID_INVALID", "invalid extension task id")
	}
	task, err := s.repo.GetTask(ctx, in.TaskID)
	if err != nil {
		return nil, err
	}
	if task.Status == adminplusdomain.ExtensionTaskStatusSucceeded {
		return nil, badRequest("EXTENSION_TASK_ALREADY_SUCCEEDED", "succeeded extension task cannot be cancelled")
	}
	if task.Status == adminplusdomain.ExtensionTaskStatusCancelled {
		return task, nil
	}
	now := s.now().UTC()
	task.Status = adminplusdomain.ExtensionTaskStatusCancelled
	task.LeaseToken = ""
	task.LeaseExpiresAt = nil
	task.LastHeartbeatAt = nil
	task.ErrorCode = firstNonEmpty(trimLimit(in.Reason, 80), "TASK_CANCELLED")
	task.UpdatedAt = now
	task.FinishedAt = &now
	updated, err := s.repo.UpdateTask(ctx, task)
	if err != nil {
		s.recordTaskFailure(ctx, task, "cancel_task", err, task.ErrorCode)
		return nil, err
	}
	s.recordTaskCancelled(ctx, updated)
	return updated, nil
}

func (s *Service) recordTaskSuccess(ctx context.Context, task *adminplusdomain.ExtensionTask, action string) {
	if s == nil || s.bizlog == nil || task == nil {
		return
	}
	s.bizlog.Record(ctx, bizlogs.Event{
		Level:      bizlogs.LevelInfo,
		Category:   bizlogs.CategoryExtension,
		Action:     action,
		Outcome:    bizlogs.OutcomeSucceeded,
		Message:    "extension task completed",
		SupplierID: task.SupplierID,
		Metadata:   taskLogMetadata(task),
	})
}

func (s *Service) recordTaskCancelled(ctx context.Context, task *adminplusdomain.ExtensionTask) {
	if s == nil || s.bizlog == nil || task == nil {
		return
	}
	metadata := taskLogMetadata(task)
	metadata["error_code"] = task.ErrorCode
	s.bizlog.Record(ctx, bizlogs.Event{
		Level:      bizlogs.LevelInfo,
		Category:   bizlogs.CategoryExtension,
		Action:     "cancel_task",
		Outcome:    "cancelled",
		Message:    "extension task cancelled",
		SupplierID: task.SupplierID,
		Reason:     task.ErrorCode,
		Metadata:   metadata,
	})
}

func (s *Service) recordTaskReportedFailure(ctx context.Context, task *adminplusdomain.ExtensionTask, final bool) {
	if s == nil || s.bizlog == nil || task == nil {
		return
	}
	metadata := taskLogMetadata(task)
	metadata["final_failure"] = final
	metadata["error_code"] = task.ErrorCode
	metadata["error_message"] = task.ErrorMessage
	outcome := bizlogs.OutcomeFailed
	if !final {
		outcome = "retry_scheduled"
	}
	s.bizlog.Record(ctx, bizlogs.Event{
		Level:      bizlogs.LevelWarn,
		Category:   bizlogs.CategoryExtension,
		Action:     "task_reported_failure",
		Outcome:    outcome,
		Message:    "extension task reported failure",
		SupplierID: task.SupplierID,
		Reason:     task.ErrorCode,
		Metadata:   metadata,
	})
}

func (s *Service) recordTaskFailure(ctx context.Context, task *adminplusdomain.ExtensionTask, action string, err error, reason string) {
	if s == nil || s.bizlog == nil {
		return
	}
	metadata := taskLogMetadata(task)
	if reason != "" {
		metadata["error_code"] = reason
	}
	event := bizlogs.EventFromError(bizlogs.Event{
		Level:      bizlogs.LevelWarn,
		Category:   bizlogs.CategoryExtension,
		Action:     action,
		Outcome:    bizlogs.OutcomeFailed,
		Message:    "extension task operation failed",
		SupplierID: taskSupplierID(task),
		Reason:     reason,
		Metadata:   metadata,
	}, err)
	s.bizlog.Record(ctx, event)
}

func taskLogMetadata(task *adminplusdomain.ExtensionTask) map[string]any {
	if task == nil {
		return map[string]any{}
	}
	out := map[string]any{
		"task_id":      task.ID,
		"supplier_id":  task.SupplierID,
		"type":         string(task.Type),
		"status":       string(task.Status),
		"attempts":     task.Attempts,
		"max_attempts": task.MaxAttempts,
		"device_id":    task.DeviceID,
	}
	putPayloadInt64(out, task.Payload, "discovery_id")
	putPayloadInt64(out, task.Payload, "registration_id")
	if task.FinishedAt != nil {
		out["finished_at"] = task.FinishedAt.UTC().Format(time.RFC3339)
	}
	if task.AvailableAfter.After(time.Time{}) {
		out["available_after"] = task.AvailableAfter.UTC().Format(time.RFC3339)
	}
	return out
}

func taskSupplierID(task *adminplusdomain.ExtensionTask) int64 {
	if task == nil {
		return 0
	}
	return task.SupplierID
}

func putPayloadInt64(out map[string]any, payload map[string]any, key string) {
	if out == nil || len(payload) == 0 || strings.TrimSpace(key) == "" {
		return
	}
	switch value := payload[key].(type) {
	case int64:
		if value > 0 {
			out[key] = value
		}
	case int:
		if value > 0 {
			out[key] = value
		}
	case float64:
		if value > 0 {
			out[key] = int64(value)
		}
	}
}

func (s *Service) ListTasks(ctx context.Context, filter TaskFilter) ([]*adminplusdomain.ExtensionTask, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("extension task service is not configured")
	}
	if filter.SupplierID < 0 {
		return nil, badRequest("EXTENSION_TASK_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, badRequest("EXTENSION_TASK_STATUS_INVALID", "invalid extension task status")
	}
	if filter.Type != "" && !filter.Type.Valid() {
		return nil, badRequest("EXTENSION_TASK_TYPE_INVALID", "invalid extension task type")
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListTasks(ctx, filter)
}

func (s *Service) GetTask(ctx context.Context, taskID int64) (*adminplusdomain.ExtensionTask, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("extension task service is not configured")
	}
	if taskID <= 0 {
		return nil, badRequest("EXTENSION_TASK_ID_INVALID", "invalid extension task id")
	}
	return s.repo.GetTask(ctx, taskID)
}

func (s *Service) GetBrowserCredential(ctx context.Context, in BrowserCredentialInput) (*adminplusdomain.SupplierBrowserCredential, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("extension task service is not configured")
	}
	if s.credentials == nil {
		return nil, internalError("supplier browser credential provider is not configured")
	}
	task, err := s.requireLeasedTask(ctx, in.TaskID, in.DeviceID, in.LeaseToken)
	if err != nil {
		return nil, err
	}
	return s.credentials.GetBrowserCredential(ctx, task.SupplierID)
}

func (s *Service) LeasedTask(ctx context.Context, taskID int64, deviceID string, leaseToken string) (*adminplusdomain.ExtensionTask, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("extension task service is not configured")
	}
	return s.requireLeasedTask(ctx, taskID, deviceID, leaseToken)
}

func (s *Service) requireLeasedTask(ctx context.Context, taskID int64, deviceID string, leaseToken string) (*adminplusdomain.ExtensionTask, error) {
	if taskID <= 0 {
		return nil, badRequest("EXTENSION_TASK_ID_INVALID", "invalid extension task id")
	}
	deviceID = strings.TrimSpace(deviceID)
	leaseToken = strings.TrimSpace(leaseToken)
	if deviceID == "" || leaseToken == "" {
		return nil, badRequest("EXTENSION_TASK_LEASE_REQUIRED", "extension task lease is required")
	}
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.DeviceID != deviceID || task.LeaseToken != leaseToken {
		return nil, infraerrors.New(http.StatusConflict, "EXTENSION_TASK_LEASE_MISMATCH", "extension task lease mismatch")
	}
	if task.LeaseExpiresAt == nil || task.LeaseExpiresAt.Before(s.now().UTC()) {
		return nil, infraerrors.New(http.StatusConflict, "EXTENSION_TASK_LEASE_EXPIRED", "extension task lease expired")
	}
	if task.Status != adminplusdomain.ExtensionTaskStatusClaimed && task.Status != adminplusdomain.ExtensionTaskStatusRunning {
		return nil, infraerrors.New(http.StatusConflict, "EXTENSION_TASK_NOT_RUNNING", "extension task is not running")
	}
	return task, nil
}

func normalizeLeaseTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return defaultLeaseTTL
	}
	if ttl > 30*time.Minute {
		return 30 * time.Minute
	}
	return ttl
}

func trimLimit(value string, limit int) string {
	v := strings.TrimSpace(value)
	if len(v) <= limit {
		return v
	}
	return v[:limit]
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func randomToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func badRequest(reason string, message string) error {
	return infraerrors.New(http.StatusBadRequest, reason, message)
}

func internalError(message string) error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", message)
}
