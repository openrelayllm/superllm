package provisionjobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/costs"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliergroups"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/supplierkeys"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	defaultMaxAttempts = 3
	defaultLease       = 2 * time.Minute
)

type SubmitInput struct {
	JobType         adminplusdomain.SupplierProvisionJobType
	SupplierID      int64
	SupplierGroupID int64
	IdempotencyKey  string
	RequestedBy     int64
	Request         map[string]any
}

type SubmitResult struct {
	JobID           int64                                    `json:"job_id"`
	Status          adminplusdomain.SupplierProvisionStatus  `json:"status"`
	JobType         adminplusdomain.SupplierProvisionJobType `json:"job_type"`
	SupplierID      int64                                    `json:"supplier_id"`
	SupplierGroupID int64                                    `json:"supplier_group_id,omitempty"`
	PollURL         string                                   `json:"poll_url"`
	Mode            string                                   `json:"mode"`
	Replayed        bool                                     `json:"replayed,omitempty"`
}

type Repository interface {
	CreateJobWithSteps(ctx context.Context, job *adminplusdomain.SupplierProvisionJob, steps []*adminplusdomain.SupplierProvisionStep, eventType string) (*adminplusdomain.SupplierProvisionJob, bool, error)
	GetJob(ctx context.Context, id int64) (*adminplusdomain.SupplierProvisionJob, error)
	ListJobs(ctx context.Context, filter ListFilter) ([]*adminplusdomain.SupplierProvisionJob, error)
	ClaimNextJob(ctx context.Context, workerID string, now time.Time, lease time.Duration) (*adminplusdomain.SupplierProvisionJob, error)
	MarkJobRunning(ctx context.Context, jobID int64, workerID string, now time.Time) error
	MarkJobSucceeded(ctx context.Context, jobID int64, result map[string]any, now time.Time) error
	MarkJobFailed(ctx context.Context, jobID int64, status adminplusdomain.SupplierProvisionStatus, errorCode string, errorMessage string, nextRunAt time.Time, now time.Time) error
	ListRunnableSteps(ctx context.Context, jobID int64, now time.Time) ([]*adminplusdomain.SupplierProvisionStep, error)
	ListJobSteps(ctx context.Context, jobID int64) ([]*adminplusdomain.SupplierProvisionStep, error)
	MarkStepRunning(ctx context.Context, stepID int64, workerID string, now time.Time) error
	MarkStepSucceeded(ctx context.Context, stepID int64, result map[string]any, now time.Time) error
	MarkStepFailed(ctx context.Context, stepID int64, status adminplusdomain.SupplierProvisionStatus, errorCode string, errorMessage string, nextRunAt time.Time, now time.Time) error
	RecordAttempt(ctx context.Context, attempt Attempt) error
	ListPendingOutboxEvents(ctx context.Context, limit int, now time.Time) ([]OutboxEvent, error)
	MarkOutboxPublished(ctx context.Context, eventID string, now time.Time) error
	MarkOutboxFailed(ctx context.Context, eventID string, now time.Time, err error) error
}

type ListFilter struct {
	SupplierID int64
	Status     adminplusdomain.SupplierProvisionStatus
	Limit      int
}

type Attempt struct {
	JobID            int64
	StepID           int64
	SupplierID       int64
	SupplierGroupID  int64
	Operation        string
	Status           string
	RequestSnapshot  map[string]any
	ResponseSnapshot map[string]any
	ErrorCode        string
	ErrorMessage     string
	DurationMS       int64
}

type OutboxEvent struct {
	EventID       string
	EventType     string
	AggregateType string
	AggregateID   int64
	Payload       map[string]any
}

type RedisPublisher interface {
	PublishProvisionEvent(ctx context.Context, event OutboxEvent) error
}

type RedisEventWaiter interface {
	WaitProvisionEvents(ctx context.Context, consumerID string, block time.Duration) (int, error)
}

type GroupSyncer interface {
	Sync(ctx context.Context, supplierID int64) (*suppliergroups.SyncResult, error)
}

type KeyProvisioner interface {
	Provision(ctx context.Context, in supplierkeys.ProvisionKeyInput) (*supplierkeys.ProvisionKeyResult, error)
	PlanEnsureAll(ctx context.Context, in supplierkeys.EnsureAllInput) ([]supplierkeys.ProvisionGroupPlan, error)
	EnsureGroup(ctx context.Context, in supplierkeys.EnsureGroupInput) (*supplierkeys.EnsureAllResultItem, error)
}

type CostSyncer interface {
	Sync(ctx context.Context, in costs.SyncInput) (*costs.SyncResult, error)
}

type Service struct {
	repo           Repository
	publisher      RedisPublisher
	groupSyncer    GroupSyncer
	keyProvisioner KeyProvisioner
	costSyncer     CostSyncer
	now            func() time.Time
}

func NewService(repo Repository, publisher RedisPublisher, groupSyncer GroupSyncer, keyProvisioner KeyProvisioner) *Service {
	return &Service{
		repo:           repo,
		publisher:      publisher,
		groupSyncer:    groupSyncer,
		keyProvisioner: keyProvisioner,
		now:            time.Now,
	}
}

func NewServiceWithCostSyncer(repo Repository, publisher RedisPublisher, groupSyncer GroupSyncer, keyProvisioner KeyProvisioner, costSyncer CostSyncer) *Service {
	service := NewService(repo, publisher, groupSyncer, keyProvisioner)
	service.costSyncer = costSyncer
	return service
}

func (s *Service) Submit(ctx context.Context, in SubmitInput) (*SubmitResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier provision job service is not configured")
	}
	normalized, err := normalizeSubmitInput(in)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	steps, err := s.stepsForSubmit(ctx, normalized, now)
	if err != nil {
		return nil, err
	}
	job := &adminplusdomain.SupplierProvisionJob{
		JobType:            normalized.JobType,
		SupplierID:         normalized.SupplierID,
		Status:             adminplusdomain.SupplierProvisionStatusQueued,
		IdempotencyKeyHash: hashIdempotencyKey(normalized.IdempotencyKey),
		RequestedBy:        normalized.RequestedBy,
		RequestSnapshot:    cloneMap(normalized.Request),
		TotalSteps:         len(steps),
		MaxAttempts:        defaultMaxAttempts,
		NextRunAt:          now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	created, replayed, err := s.repo.CreateJobWithSteps(ctx, job, steps, eventTypeForJob(normalized.JobType))
	if err != nil {
		return nil, err
	}
	if !replayed {
		s.publishPendingOutboxBestEffort(10)
	}
	return submitResult(created, normalized.SupplierGroupID, replayed), nil
}

func (s *Service) Get(ctx context.Context, id int64) (*adminplusdomain.SupplierProvisionJob, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier provision job service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("SUPPLIER_PROVISION_JOB_ID_INVALID", "invalid supplier provision job id")
	}
	return s.repo.GetJob(ctx, id)
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]*adminplusdomain.SupplierProvisionJob, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier provision job service is not configured")
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	return s.repo.ListJobs(ctx, filter)
}

func (s *Service) PublishPendingOutbox(ctx context.Context, limit int) error {
	if s == nil || s.repo == nil || s.publisher == nil {
		return nil
	}
	if limit <= 0 {
		limit = 100
	}
	now := s.now().UTC()
	events, err := s.repo.ListPendingOutboxEvents(ctx, limit, now)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := s.publisher.PublishProvisionEvent(ctx, event); err != nil {
			_ = s.repo.MarkOutboxFailed(ctx, event.EventID, now, err)
			continue
		}
		_ = s.repo.MarkOutboxPublished(ctx, event.EventID, now)
	}
	return nil
}

func (s *Service) publishPendingOutboxBestEffort(limit int) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.PublishPendingOutbox(ctx, limit)
	}()
}

func (s *Service) stepsForSubmit(ctx context.Context, in SubmitInput, now time.Time) ([]*adminplusdomain.SupplierProvisionStep, error) {
	if in.JobType != adminplusdomain.SupplierProvisionJobTypeProvisionAllGroupKeys {
		return []*adminplusdomain.SupplierProvisionStep{
			newSubmitStep(in.SupplierID, in.SupplierGroupID, stepTypeForJob(in.JobType), in.IdempotencyKey, in.Request, now),
		}, nil
	}
	if s.keyProvisioner == nil {
		return nil, internalError("supplier key provisioner is not configured")
	}
	plans, err := s.keyProvisioner.PlanEnsureAll(ctx, ensureAllInputFromSnapshot(in.SupplierID, in.Request))
	if err != nil {
		return nil, err
	}
	steps := make([]*adminplusdomain.SupplierProvisionStep, 0, len(plans))
	for _, plan := range plans {
		if plan.SupplierGroupID <= 0 {
			continue
		}
		request := cloneMap(in.Request)
		request["supplier_group_id"] = plan.SupplierGroupID
		request["external_group_id"] = plan.ExternalGroupID
		request["group_name"] = plan.GroupName
		request["provider_family"] = plan.ProviderFamily
		steps = append(steps, newSubmitStep(
			in.SupplierID,
			plan.SupplierGroupID,
			adminplusdomain.SupplierProvisionStepEnsureThirdPartyKey,
			stepIdempotencyKey(in.IdempotencyKey, plan.SupplierGroupID),
			request,
			now,
		))
	}
	return steps, nil
}

func newSubmitStep(supplierID int64, supplierGroupID int64, stepType adminplusdomain.SupplierProvisionStepType, idempotencyKey string, request map[string]any, now time.Time) *adminplusdomain.SupplierProvisionStep {
	return &adminplusdomain.SupplierProvisionStep{
		SupplierID:      supplierID,
		SupplierGroupID: supplierGroupID,
		StepType:        stepType,
		Status:          adminplusdomain.SupplierProvisionStatusQueued,
		IdempotencyKey:  idempotencyKey,
		RequestSnapshot: cloneMap(request),
		MaxAttempts:     defaultMaxAttempts,
		NextRunAt:       now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func stepIdempotencyKey(jobKey string, supplierGroupID int64) string {
	key := strings.TrimSpace(jobKey)
	if key == "" || supplierGroupID <= 0 {
		return key
	}
	return fmt.Sprintf("%s:group:%d", key, supplierGroupID)
}

func (s *Service) RunOnce(ctx context.Context, workerID string) (*adminplusdomain.SupplierProvisionJob, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier provision job service is not configured")
	}
	if workerID == "" {
		workerID = defaultWorkerID()
	}
	now := s.now().UTC()
	job, err := s.repo.ClaimNextJob(ctx, workerID, now, defaultLease)
	if err != nil || job == nil {
		return job, err
	}
	if err := s.executeJob(ctx, workerID, job); err != nil {
		return job, err
	}
	return job, nil
}

func (s *Service) executeJob(ctx context.Context, workerID string, job *adminplusdomain.SupplierProvisionJob) error {
	now := s.now().UTC()
	if err := s.repo.MarkJobRunning(ctx, job.ID, workerID, now); err != nil {
		return err
	}
	steps, err := s.repo.ListRunnableSteps(ctx, job.ID, now)
	if err != nil {
		_ = s.repo.MarkJobFailed(ctx, job.ID, adminplusdomain.SupplierProvisionStatusRetryableFailed, infraerrors.Reason(err), infraerrors.Message(err), retryAt(now, 1), now)
		return err
	}
	if len(steps) == 0 {
		allSteps, listErr := s.repo.ListJobSteps(ctx, job.ID)
		if listErr != nil {
			_ = s.repo.MarkJobFailed(ctx, job.ID, adminplusdomain.SupplierProvisionStatusRetryableFailed, infraerrors.Reason(listErr), infraerrors.Message(listErr), retryAt(now, 1), now)
			return listErr
		}
		return s.finishJobFromSteps(ctx, job, allSteps, nil, now)
	}
	for _, step := range steps {
		if step == nil {
			continue
		}
		if err := s.repo.MarkStepRunning(ctx, step.ID, workerID, now); err != nil {
			return err
		}
		started := time.Now()
		result, execErr := s.executeJobPayload(ctx, job, step)
		durationMS := time.Since(started).Milliseconds()
		if execErr != nil {
			code := firstNonEmpty(infraerrors.Reason(execErr), "SUPPLIER_PROVISION_JOB_FAILED")
			message := provisionJobErrorMessage(execErr)
			next := retryAt(now, step.Attempts+1)
			status := adminplusdomain.SupplierProvisionStatusRetryableFailed
			if step.Attempts+1 >= step.MaxAttempts {
				status = adminplusdomain.SupplierProvisionStatusDead
			}
			_ = s.repo.MarkStepFailed(ctx, step.ID, status, code, message, next, now)
			_ = s.repo.RecordAttempt(ctx, Attempt{
				JobID: job.ID, StepID: step.ID, SupplierID: job.SupplierID, SupplierGroupID: step.SupplierGroupID,
				Operation: string(step.StepType), Status: string(status), RequestSnapshot: step.RequestSnapshot,
				ErrorCode: code, ErrorMessage: message, DurationMS: durationMS,
			})
			continue
		}
		if err := s.repo.MarkStepSucceeded(ctx, step.ID, result, now); err != nil {
			return err
		}
		_ = s.repo.RecordAttempt(ctx, Attempt{
			JobID: job.ID, StepID: step.ID, SupplierID: job.SupplierID, SupplierGroupID: step.SupplierGroupID,
			Operation: string(step.StepType), Status: string(adminplusdomain.SupplierProvisionStatusSucceeded),
			RequestSnapshot: step.RequestSnapshot, ResponseSnapshot: result, DurationMS: durationMS,
		})
	}
	allSteps, err := s.repo.ListJobSteps(ctx, job.ID)
	if err != nil {
		_ = s.repo.MarkJobFailed(ctx, job.ID, adminplusdomain.SupplierProvisionStatusRetryableFailed, infraerrors.Reason(err), infraerrors.Message(err), retryAt(now, 1), now)
		return err
	}
	return s.finishJobFromSteps(ctx, job, allSteps, nil, now)
}

func (s *Service) executeJobPayload(ctx context.Context, job *adminplusdomain.SupplierProvisionJob, step *adminplusdomain.SupplierProvisionStep) (map[string]any, error) {
	switch job.JobType {
	case adminplusdomain.SupplierProvisionJobTypeSyncGroups:
		if s.groupSyncer == nil {
			return nil, internalError("supplier group syncer is not configured")
		}
		result, err := s.groupSyncer.Sync(ctx, job.SupplierID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"total": result.Total, "synced_at": result.SyncedAt}, nil
	case adminplusdomain.SupplierProvisionJobTypeProvisionGroupKey:
		if s.keyProvisioner == nil {
			return nil, internalError("supplier key provisioner is not configured")
		}
		input, err := provisionInputFromSnapshot(job.SupplierID, step, job.RequestSnapshot)
		if err != nil {
			return nil, err
		}
		result, err := s.keyProvisioner.Provision(ctx, input)
		if err != nil {
			return nil, err
		}
		out := map[string]any{}
		if result != nil && result.Key != nil {
			out["supplier_key_id"] = result.Key.ID
			out["supplier_key_status"] = result.Key.Status
			out["local_sub2api_account_id"] = result.Key.LocalSub2APIAccountID
		}
		if result != nil && result.Binding != nil {
			out["binding_id"] = result.Binding.ID
		}
		return out, nil
	case adminplusdomain.SupplierProvisionJobTypeProvisionAllGroupKeys:
		if s.keyProvisioner == nil {
			return nil, internalError("supplier key provisioner is not configured")
		}
		input := ensureGroupInputFromSnapshot(job.SupplierID, step)
		result, err := s.keyProvisioner.EnsureGroup(ctx, input)
		if err != nil {
			return ensureGroupResultSnapshot(result), err
		}
		return ensureGroupResultSnapshot(result), nil
	case adminplusdomain.SupplierProvisionJobTypeSyncSupplierCosts:
		if s.costSyncer == nil {
			return nil, internalError("supplier cost syncer is not configured")
		}
		input := costSyncInputFromSnapshot(job.SupplierID, job.RequestSnapshot)
		result, err := s.costSyncer.Sync(ctx, input)
		if err != nil {
			return nil, err
		}
		return costSyncResultSnapshot(result), nil
	default:
		return nil, badRequest("SUPPLIER_PROVISION_JOB_TYPE_UNSUPPORTED", "unsupported supplier provision job type")
	}
}

func (s *Service) finishJobFromSteps(ctx context.Context, job *adminplusdomain.SupplierProvisionJob, steps []*adminplusdomain.SupplierProvisionStep, fallbackErr error, now time.Time) error {
	result := jobResultSnapshot(job, steps)
	status, code, message, next := jobStatusFromSteps(steps, fallbackErr, now)
	if status == adminplusdomain.SupplierProvisionStatusSucceeded {
		return s.repo.MarkJobSucceeded(ctx, job.ID, result, now)
	}
	return s.repo.MarkJobFailed(ctx, job.ID, status, code, message, next, now)
}

func jobResultSnapshot(job *adminplusdomain.SupplierProvisionJob, steps []*adminplusdomain.SupplierProvisionStep) map[string]any {
	if job == nil {
		return map[string]any{}
	}
	if job.JobType != adminplusdomain.SupplierProvisionJobTypeProvisionAllGroupKeys {
		if len(steps) == 1 {
			return cloneMap(steps[0].ResultSnapshot)
		}
		return map[string]any{"total_steps": len(steps)}
	}
	out := map[string]any{
		"total":                len(steps),
		"created":              0,
		"skipped":              0,
		"failed":               0,
		"local_groups_created": 0,
		"local_accounts_bound": 0,
	}
	for _, step := range steps {
		if step == nil {
			continue
		}
		switch stringValue(step.ResultSnapshot["action"]) {
		case "created":
			out["created"] = intValue(out["created"]) + 1
		case "skipped":
			out["skipped"] = intValue(out["skipped"]) + 1
		}
		if boolValue(step.ResultSnapshot["local_group_created"], false) {
			out["local_groups_created"] = intValue(out["local_groups_created"]) + 1
		}
		if boolValue(step.ResultSnapshot["local_account_group_bound"], false) {
			out["local_accounts_bound"] = intValue(out["local_accounts_bound"]) + 1
		}
		if step.Status == adminplusdomain.SupplierProvisionStatusRetryableFailed || step.Status == adminplusdomain.SupplierProvisionStatusDead || step.Status == adminplusdomain.SupplierProvisionStatusManualRequired {
			out["failed"] = intValue(out["failed"]) + 1
		}
	}
	return out
}

func jobStatusFromSteps(steps []*adminplusdomain.SupplierProvisionStep, fallbackErr error, now time.Time) (adminplusdomain.SupplierProvisionStatus, string, string, time.Time) {
	if len(steps) == 0 {
		return adminplusdomain.SupplierProvisionStatusSucceeded, "", "", now
	}
	succeeded := 0
	retryable := 0
	dead := 0
	manual := 0
	next := time.Time{}
	var firstFailed *adminplusdomain.SupplierProvisionStep
	for _, step := range steps {
		if step == nil {
			continue
		}
		switch step.Status {
		case adminplusdomain.SupplierProvisionStatusSucceeded:
			succeeded++
		case adminplusdomain.SupplierProvisionStatusManualRequired:
			manual++
			if firstFailed == nil {
				firstFailed = step
			}
		case adminplusdomain.SupplierProvisionStatusDead:
			dead++
			if firstFailed == nil {
				firstFailed = step
			}
		case adminplusdomain.SupplierProvisionStatusRetryableFailed:
			retryable++
			if firstFailed == nil {
				firstFailed = step
			}
			if next.IsZero() || step.NextRunAt.Before(next) {
				next = step.NextRunAt
			}
		default:
			retryable++
			if firstFailed == nil {
				firstFailed = step
			}
			if next.IsZero() || step.NextRunAt.Before(next) {
				next = step.NextRunAt
			}
		}
	}
	if succeeded == len(steps) {
		return adminplusdomain.SupplierProvisionStatusSucceeded, "", "", now
	}
	if next.IsZero() {
		next = retryAt(now, 1)
	}
	code, message := failedStepError(firstFailed, fallbackErr)
	if retryable > 0 {
		return adminplusdomain.SupplierProvisionStatusRetryableFailed, code, message, next
	}
	if succeeded > 0 && (dead > 0 || manual > 0) {
		return adminplusdomain.SupplierProvisionStatusPartialSucceeded, code, message, now
	}
	if manual > 0 {
		return adminplusdomain.SupplierProvisionStatusManualRequired, code, message, now
	}
	return adminplusdomain.SupplierProvisionStatusDead, code, message, now
}

func failedStepError(step *adminplusdomain.SupplierProvisionStep, fallbackErr error) (string, string) {
	if step != nil {
		return firstNonEmpty(step.ErrorCode, infraerrors.Reason(fallbackErr), "SUPPLIER_PROVISION_JOB_FAILED"),
			firstNonEmpty(step.ErrorMessage, provisionJobErrorMessage(fallbackErr))
	}
	return firstNonEmpty(infraerrors.Reason(fallbackErr), "SUPPLIER_PROVISION_JOB_FAILED"), provisionJobErrorMessage(fallbackErr)
}

func provisionJobErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	message := infraerrors.Message(err)
	if message != infraerrors.UnknownMessage {
		return trimErrorMessage(message)
	}
	cause := err
	var appErr *infraerrors.ApplicationError
	if errors.As(err, &appErr) {
		if unwrapped := errors.Unwrap(appErr); unwrapped != nil {
			cause = unwrapped
		}
	}
	return trimErrorMessage(cause.Error())
}

func normalizeSubmitInput(in SubmitInput) (SubmitInput, error) {
	if in.SupplierID <= 0 {
		return SubmitInput{}, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	switch in.JobType {
	case adminplusdomain.SupplierProvisionJobTypeSyncGroups, adminplusdomain.SupplierProvisionJobTypeProvisionAllGroupKeys, adminplusdomain.SupplierProvisionJobTypeSyncSupplierCosts:
	case adminplusdomain.SupplierProvisionJobTypeProvisionGroupKey:
		if in.SupplierGroupID <= 0 {
			return SubmitInput{}, badRequest("SUPPLIER_GROUP_ID_INVALID", "invalid supplier group id")
		}
	default:
		return SubmitInput{}, badRequest("SUPPLIER_PROVISION_JOB_TYPE_INVALID", "invalid supplier provision job type")
	}
	if strings.TrimSpace(in.IdempotencyKey) == "" {
		in.IdempotencyKey = fmt.Sprintf("%s:supplier:%d:group:%d:%d", in.JobType, in.SupplierID, in.SupplierGroupID, time.Now().UnixNano())
	}
	if in.Request == nil {
		in.Request = map[string]any{}
	}
	return in, nil
}

func submitResult(job *adminplusdomain.SupplierProvisionJob, supplierGroupID int64, replayed bool) *SubmitResult {
	return &SubmitResult{
		JobID:           job.ID,
		Status:          job.Status,
		JobType:         job.JobType,
		SupplierID:      job.SupplierID,
		SupplierGroupID: supplierGroupID,
		PollURL:         fmt.Sprintf("/api/v1/admin-plus/supplier-provision-jobs/%d", job.ID),
		Mode:            "async_job",
		Replayed:        replayed,
	}
}

func stepTypeForJob(jobType adminplusdomain.SupplierProvisionJobType) adminplusdomain.SupplierProvisionStepType {
	switch jobType {
	case adminplusdomain.SupplierProvisionJobTypeSyncGroups:
		return adminplusdomain.SupplierProvisionStepSyncSupplierGroup
	case adminplusdomain.SupplierProvisionJobTypeProvisionAllGroupKeys:
		return adminplusdomain.SupplierProvisionStepProvisionAllGroupKeys
	case adminplusdomain.SupplierProvisionJobTypeRepairBinding:
		return adminplusdomain.SupplierProvisionStepRepairBinding
	case adminplusdomain.SupplierProvisionJobTypeSyncSupplierCosts:
		return adminplusdomain.SupplierProvisionStepSyncSupplierCosts
	default:
		return adminplusdomain.SupplierProvisionStepEnsureThirdPartyKey
	}
}

func eventTypeForJob(jobType adminplusdomain.SupplierProvisionJobType) string {
	switch jobType {
	case adminplusdomain.SupplierProvisionJobTypeSyncGroups:
		return "supplier.groups.sync.requested"
	case adminplusdomain.SupplierProvisionJobTypeProvisionAllGroupKeys:
		return "supplier.keys.provision_all.requested"
	case adminplusdomain.SupplierProvisionJobTypeSyncSupplierCosts:
		return "supplier.costs.sync.requested"
	default:
		return "supplier.key.provision.requested"
	}
}

func hashIdempotencyKey(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func retryAt(now time.Time, attempts int) time.Time {
	delay := time.Duration(1<<min(attempts, 6)) * time.Second
	return now.Add(delay)
}

func defaultWorkerID() string {
	host, _ := os.Hostname()
	if host == "" {
		host = "local"
	}
	return "admin-plus-provision-" + host + "-" + strconv.Itoa(os.Getpid())
}

func badRequest(reason string, message string) error {
	return infraerrors.New(http.StatusBadRequest, reason, message)
}

func internalError(message string) error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", message)
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func trimErrorMessage(value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return infraerrors.UnknownMessage
	}
	if containsSensitiveMarker(v) {
		return "error detail redacted because it contains sensitive fields"
	}
	const limit = 240
	if len(v) > limit {
		return v[:limit] + "..."
	}
	return v
}

func containsSensitiveMarker(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{
		"access_token",
		"refresh_token",
		"authorization",
		"bearer ",
		"cookie",
		"password",
		"secret",
		"session_bundle",
		"api_key",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func stepID(step *adminplusdomain.SupplierProvisionStep) int64 {
	if step == nil {
		return 0
	}
	return step.ID
}

func supplierGroupID(step *adminplusdomain.SupplierProvisionStep) int64 {
	if step == nil {
		return 0
	}
	return step.SupplierGroupID
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
