package provisionjobs

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/costs"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/supplierkeys"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServiceRunOnceExecutesSupplierCostSyncJob(t *testing.T) {
	repo := newProvisionJobsMemoryRepository()
	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	syncer := &stubCostSyncer{result: &costs.SyncResult{
		SupplierID:              7,
		ProviderType:            "sub2api",
		SystemType:              "sub2api",
		SyncedAt:                now,
		FundingTransactions:     4,
		EntitlementTransactions: 10,
		UsageCostLines:          100,
		LedgerEntries:           14,
		Capabilities:            map[string]bool{"funding_transactions": true},
	}}
	service := NewServiceWithCostSyncer(repo, nil, nil, nil, syncer)
	service.now = func() time.Time { return now }

	submitted, err := service.Submit(context.Background(), SubmitInput{
		JobType:    adminplusdomain.SupplierProvisionJobTypeSyncSupplierCosts,
		SupplierID: 7,
		Request: map[string]any{
			"started_at":                       "2026-06-14T00:00:00Z",
			"ended_at":                         "2026-06-21T00:00:00Z",
			"include_funding_transactions":     true,
			"include_entitlement_transactions": true,
			"include_usage_cost_lines":         true,
			"include_balance_snapshot":         true,
		},
	})
	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierProvisionJobTypeSyncSupplierCosts, submitted.JobType)

	job, err := service.RunOnce(context.Background(), "worker-1")
	require.NoError(t, err)
	require.NotNil(t, job)
	require.Equal(t, int64(7), syncer.input.SupplierID)
	require.NotNil(t, syncer.input.StartedAt)
	require.NotNil(t, syncer.input.EndedAt)
	require.True(t, syncer.input.IncludeFundingTransactions)

	stored, err := service.Get(context.Background(), submitted.JobID)
	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierProvisionStatusSucceeded, stored.Status)
	require.Equal(t, 4, stored.ResultSnapshot["funding_transactions"])
	require.Equal(t, "sub2api", stored.ResultSnapshot["provider_type"])
}

func TestServiceProvisionAllCreatesOneStepPerSupplierGroup(t *testing.T) {
	repo := newProvisionJobsMemoryRepository()
	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	keyProvisioner := &stubKeyProvisioner{
		plan: &supplierkeys.EnsureAllPlan{
			Items: []supplierkeys.ProvisionGroupPlan{
				{SupplierGroupID: 10, ExternalGroupID: "g10", GroupName: "Fast", ProviderFamily: "openai", Action: "create"},
				{SupplierGroupID: 20, ExternalGroupID: "g20", GroupName: "Cheap", ProviderFamily: "openai", Action: "create"},
			},
		},
	}
	service := NewService(repo, nil, nil, keyProvisioner)
	service.now = func() time.Time { return now }

	submitted, err := service.Submit(context.Background(), SubmitInput{
		JobType:        adminplusdomain.SupplierProvisionJobTypeProvisionAllGroupKeys,
		SupplierID:     7,
		IdempotencyKey: "supplier-7-all",
		Request: map[string]any{
			"local_account_base_url": "https://relay.example.com/v1",
			"runtime_status":         string(adminplusdomain.SupplierRuntimeStatusMonitorOnly),
			"health_status":          string(adminplusdomain.SupplierHealthStatusNormal),
		},
	})
	require.NoError(t, err)
	require.Equal(t, int64(7), keyProvisioner.planned.SupplierID)
	require.Equal(t, adminplusdomain.SupplierProvisionJobTypeProvisionAllGroupKeys, submitted.JobType)

	stored, err := service.Get(context.Background(), submitted.JobID)
	require.NoError(t, err)
	require.Len(t, stored.Steps, 2)
	require.Equal(t, 2, stored.TotalSteps)
	require.Equal(t, int64(10), stored.Steps[0].SupplierGroupID)
	require.Equal(t, adminplusdomain.SupplierProvisionStepEnsureThirdPartyKey, stored.Steps[0].StepType)
	require.Equal(t, "supplier-7-all:group:10", stored.Steps[0].IdempotencyKey)

	job, err := service.RunOnce(context.Background(), "worker-1")
	require.NoError(t, err)
	require.NotNil(t, job)
	require.Len(t, keyProvisioner.ensureCalls, 2)
	require.Equal(t, int64(10), keyProvisioner.ensureCalls[0].SupplierGroupID)
	require.Equal(t, "https://relay.example.com/v1", keyProvisioner.ensureCalls[0].LocalAccountBaseURL)
	require.Equal(t, int64(20), keyProvisioner.ensureCalls[1].SupplierGroupID)

	stored, err = service.Get(context.Background(), submitted.JobID)
	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierProvisionStatusSucceeded, stored.Status)
	require.Equal(t, 2, stored.SucceededSteps)
	require.Equal(t, 2, stored.ResultSnapshot["total"])
	require.Equal(t, 2, stored.ResultSnapshot["created"])
	require.Equal(t, 2, stored.ResultSnapshot["local_groups_created"])
}

func TestServiceProvisionAllRejectsBlockedCapacityPlan(t *testing.T) {
	repo := newProvisionJobsMemoryRepository()
	keyProvisioner := &stubKeyProvisioner{
		plan: &supplierkeys.EnsureAllPlan{
			SupplierID:        7,
			KeyLimitPolicy:    adminplusdomain.SupplierKeyLimitPolicyLimited,
			KeyLimitValue:     1,
			ActiveKeyCount:    1,
			RemainingKeySlots: 0,
			Blocked:           1,
			Items: []supplierkeys.ProvisionGroupPlan{{
				SupplierGroupID: 20,
				ExternalGroupID: "g20",
				GroupName:       "Cheap",
				ProviderFamily:  "openai",
				Action:          "blocked",
				BlockedReason:   "key_capacity_exhausted",
			}},
		},
	}
	service := NewService(repo, nil, nil, keyProvisioner)

	submitted, err := service.Submit(context.Background(), SubmitInput{
		JobType:        adminplusdomain.SupplierProvisionJobTypeProvisionAllGroupKeys,
		SupplierID:     7,
		IdempotencyKey: "supplier-7-all",
		Request: map[string]any{
			"local_account_base_url": "https://relay.example.com/v1",
			"runtime_status":         string(adminplusdomain.SupplierRuntimeStatusMonitorOnly),
			"health_status":          string(adminplusdomain.SupplierHealthStatusNormal),
		},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "SUPPLIER_KEY_CAPACITY_EXHAUSTED")
	require.Nil(t, submitted)
}

func TestServiceProvisionAllAllowsExplicitPartialCapacityPlan(t *testing.T) {
	repo := newProvisionJobsMemoryRepository()
	keyProvisioner := &stubKeyProvisioner{
		plan: &supplierkeys.EnsureAllPlan{
			SupplierID:        7,
			KeyLimitPolicy:    adminplusdomain.SupplierKeyLimitPolicyLimited,
			KeyLimitValue:     2,
			ActiveKeyCount:    1,
			RemainingKeySlots: 1,
			ToCreate:          1,
			Blocked:           1,
			Items: []supplierkeys.ProvisionGroupPlan{
				{SupplierGroupID: 10, ExternalGroupID: "g10", GroupName: "Lowest", ProviderFamily: "openai", Action: "create"},
				{SupplierGroupID: 20, ExternalGroupID: "g20", GroupName: "High", ProviderFamily: "openai", Action: "blocked", BlockedReason: "key_capacity_exhausted"},
			},
		},
	}
	service := NewService(repo, nil, nil, keyProvisioner)

	submitted, err := service.Submit(context.Background(), SubmitInput{
		JobType:        adminplusdomain.SupplierProvisionJobTypeProvisionAllGroupKeys,
		SupplierID:     7,
		IdempotencyKey: "supplier-7-partial",
		Request: map[string]any{
			"allow_partial":          true,
			"local_account_base_url": "https://relay.example.com/v1",
			"runtime_status":         string(adminplusdomain.SupplierRuntimeStatusMonitorOnly),
			"health_status":          string(adminplusdomain.SupplierHealthStatusNormal),
		},
	})

	require.NoError(t, err)
	require.NotNil(t, submitted)
	stored, err := service.Get(context.Background(), submitted.JobID)
	require.NoError(t, err)
	require.Len(t, stored.Steps, 1)
	require.Equal(t, int64(10), stored.Steps[0].SupplierGroupID)
	require.Equal(t, true, stored.Steps[0].RequestSnapshot["allow_partial"])
}

func TestServiceProvisionAllAllowsExplicitPartialUnboundProviderKeyPlan(t *testing.T) {
	repo := newProvisionJobsMemoryRepository()
	keyProvisioner := &stubKeyProvisioner{
		plan: &supplierkeys.EnsureAllPlan{
			SupplierID: 7,
			ToCreate:   1,
			Blocked:    1,
			Items: []supplierkeys.ProvisionGroupPlan{
				{SupplierGroupID: 10, ExternalGroupID: "g10", GroupName: "Lowest", ProviderFamily: "openai", Action: "create"},
				{SupplierGroupID: 20, ExternalGroupID: "g20", GroupName: "Existing", ProviderFamily: "openai", Action: "blocked", BlockedReason: "provider_key_exists_unbound", ProviderExternalKeyID: "3603"},
			},
		},
	}
	service := NewService(repo, nil, nil, keyProvisioner)

	submitted, err := service.Submit(context.Background(), SubmitInput{
		JobType:        adminplusdomain.SupplierProvisionJobTypeProvisionAllGroupKeys,
		SupplierID:     7,
		IdempotencyKey: "supplier-7-partial-unbound",
		Request: map[string]any{
			"allow_partial":          true,
			"local_account_base_url": "https://relay.example.com/v1",
			"runtime_status":         string(adminplusdomain.SupplierRuntimeStatusMonitorOnly),
			"health_status":          string(adminplusdomain.SupplierHealthStatusNormal),
		},
	})

	require.NoError(t, err)
	require.NotNil(t, submitted)
	stored, err := service.Get(context.Background(), submitted.JobID)
	require.NoError(t, err)
	require.Len(t, stored.Steps, 1)
	require.Equal(t, int64(10), stored.Steps[0].SupplierGroupID)
}

func TestServiceProvisionAllRetriesOnlyFailedSupplierGroup(t *testing.T) {
	repo := newProvisionJobsMemoryRepository()
	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	keyProvisioner := &stubKeyProvisioner{
		plan: &supplierkeys.EnsureAllPlan{
			Items: []supplierkeys.ProvisionGroupPlan{
				{SupplierGroupID: 10, ExternalGroupID: "g10", GroupName: "Fast", ProviderFamily: "openai", Action: "create"},
				{SupplierGroupID: 20, ExternalGroupID: "g20", GroupName: "Cheap", ProviderFamily: "openai", Action: "create"},
			},
		},
		failGroupID: 20,
	}
	service := NewService(repo, nil, nil, keyProvisioner)
	service.now = func() time.Time { return now }

	submitted, err := service.Submit(context.Background(), SubmitInput{
		JobType:        adminplusdomain.SupplierProvisionJobTypeProvisionAllGroupKeys,
		SupplierID:     7,
		IdempotencyKey: "supplier-7-all",
		Request: map[string]any{
			"local_account_base_url": "https://relay.example.com/v1",
			"runtime_status":         string(adminplusdomain.SupplierRuntimeStatusMonitorOnly),
			"health_status":          string(adminplusdomain.SupplierHealthStatusNormal),
		},
	})
	require.NoError(t, err)

	_, err = service.RunOnce(context.Background(), "worker-1")
	require.NoError(t, err)
	stored, err := service.Get(context.Background(), submitted.JobID)
	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierProvisionStatusRetryableFailed, stored.Status)
	require.Equal(t, 1, stored.SucceededSteps)
	require.Equal(t, 1, stored.FailedSteps)
	require.Equal(t, []int64{10, 20}, keyProvisioner.ensureGroupIDs())

	keyProvisioner.failGroupID = 0
	now = now.Add(3 * time.Second)
	_, err = service.RunOnce(context.Background(), "worker-1")
	require.NoError(t, err)

	stored, err = service.Get(context.Background(), submitted.JobID)
	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierProvisionStatusSucceeded, stored.Status)
	require.Equal(t, 2, stored.SucceededSteps)
	require.Equal(t, []int64{10, 20, 20}, keyProvisioner.ensureGroupIDs())
}

func TestServiceProvisionAllStopsAfterProviderKeyLimitReached(t *testing.T) {
	repo := newProvisionJobsMemoryRepository()
	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	keyProvisioner := &stubKeyProvisioner{
		plan: &supplierkeys.EnsureAllPlan{Items: []supplierkeys.ProvisionGroupPlan{
			{SupplierGroupID: 10, ExternalGroupID: "g10", GroupName: "First", ProviderFamily: "openai", Action: "create"},
			{SupplierGroupID: 20, ExternalGroupID: "g20", GroupName: "Second", ProviderFamily: "openai", Action: "create"},
			{SupplierGroupID: 30, ExternalGroupID: "g30", GroupName: "Third", ProviderFamily: "openai", Action: "create"},
		}},
		failGroupID: 20,
		failCode:    "SUPPLIER_KEY_LIMIT_REACHED",
	}
	service := NewService(repo, nil, nil, keyProvisioner)
	service.now = func() time.Time { return now }

	submitted, err := service.Submit(context.Background(), SubmitInput{
		JobType: adminplusdomain.SupplierProvisionJobTypeProvisionAllGroupKeys, SupplierID: 7, IdempotencyKey: "supplier-7-limit",
		Request: map[string]any{"local_account_base_url": "https://relay.example.com/v1", "runtime_status": string(adminplusdomain.SupplierRuntimeStatusMonitorOnly), "health_status": string(adminplusdomain.SupplierHealthStatusNormal)},
	})
	require.NoError(t, err)

	_, err = service.RunOnce(context.Background(), "worker-1")
	require.NoError(t, err)
	stored, err := service.Get(context.Background(), submitted.JobID)
	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierProvisionStatusPartialSucceeded, stored.Status)
	require.Equal(t, []int64{10, 20}, keyProvisioner.ensureGroupIDs())
	require.Equal(t, adminplusdomain.SupplierProvisionStatusDead, stored.Steps[1].Status)
	require.Equal(t, adminplusdomain.SupplierProvisionStatusSkipped, stored.Steps[2].Status)
	require.Equal(t, "SUPPLIER_KEY_LIMIT_REACHED", stored.Steps[2].ErrorCode)
}

func TestProvisionJobErrorMessagePreservesNonSensitiveCause(t *testing.T) {
	err := infraerrors.New(500, "ADMIN_PLUS_INTERNAL_ERROR", infraerrors.UnknownMessage).
		WithCause(errors.New("decrypt: message authentication failed"))

	require.Equal(t, "decrypt: message authentication failed", provisionJobErrorMessage(err))
}

func TestProvisionJobErrorMessageRedactsSensitiveCause(t *testing.T) {
	err := infraerrors.New(500, "ADMIN_PLUS_INTERNAL_ERROR", infraerrors.UnknownMessage).
		WithCause(errors.New("request failed with access_token=secret"))

	require.Equal(t, "error detail redacted because it contains sensitive fields", provisionJobErrorMessage(err))
}

type stubCostSyncer struct {
	input  costs.SyncInput
	result *costs.SyncResult
}

func (s *stubCostSyncer) Sync(_ context.Context, in costs.SyncInput) (*costs.SyncResult, error) {
	s.input = in
	return s.result, nil
}

type stubKeyProvisioner struct {
	planned     supplierkeys.EnsureAllInput
	plan        *supplierkeys.EnsureAllPlan
	ensureCalls []supplierkeys.EnsureGroupInput
	failGroupID int64
	failCode    string
}

func (s *stubKeyProvisioner) Provision(context.Context, supplierkeys.ProvisionKeyInput) (*supplierkeys.ProvisionKeyResult, error) {
	return nil, nil
}

func (s *stubKeyProvisioner) PlanEnsureAll(_ context.Context, in supplierkeys.EnsureAllInput) (*supplierkeys.EnsureAllPlan, error) {
	s.planned = in
	if s.plan != nil {
		cp := *s.plan
		cp.Items = append([]supplierkeys.ProvisionGroupPlan(nil), s.plan.Items...)
		return &cp, nil
	}
	return &supplierkeys.EnsureAllPlan{}, nil
}

func (s *stubKeyProvisioner) EnsureGroup(_ context.Context, in supplierkeys.EnsureGroupInput) (*supplierkeys.EnsureAllResultItem, error) {
	s.ensureCalls = append(s.ensureCalls, in)
	if s.failGroupID == in.SupplierGroupID {
		code := s.failCode
		if code == "" {
			code = "TEST_GROUP_FAILED"
		}
		return &supplierkeys.EnsureAllResultItem{
			SupplierGroupID: in.SupplierGroupID,
			Action:          "failed",
			ErrorCode:       code,
			ErrorMessage:    "group failed",
		}, infraerrors.New(409, code, "group failed")
	}
	return &supplierkeys.EnsureAllResultItem{
		SupplierGroupID:        in.SupplierGroupID,
		ExternalGroupID:        "external",
		GroupName:              "group",
		Action:                 "created",
		LocalGroupCreated:      true,
		LocalAccountGroupBound: true,
		LocalSub2APIGroupID:    in.SupplierGroupID + 1000,
		LocalSub2APIGroupName:  "local-group",
	}, nil
}

func (s *stubKeyProvisioner) ensureGroupIDs() []int64 {
	out := make([]int64, 0, len(s.ensureCalls))
	for _, call := range s.ensureCalls {
		out = append(out, call.SupplierGroupID)
	}
	return out
}

type provisionJobsMemoryRepository struct {
	nextJobID  int64
	nextStepID int64
	jobs       map[int64]*adminplusdomain.SupplierProvisionJob
	steps      map[int64]*adminplusdomain.SupplierProvisionStep
	jobSteps   map[int64][]int64
}

func newProvisionJobsMemoryRepository() *provisionJobsMemoryRepository {
	return &provisionJobsMemoryRepository{
		nextJobID:  1,
		nextStepID: 1,
		jobs:       make(map[int64]*adminplusdomain.SupplierProvisionJob),
		steps:      make(map[int64]*adminplusdomain.SupplierProvisionStep),
		jobSteps:   make(map[int64][]int64),
	}
}

func (r *provisionJobsMemoryRepository) CreateJobWithSteps(_ context.Context, job *adminplusdomain.SupplierProvisionJob, steps []*adminplusdomain.SupplierProvisionStep, _ string) (*adminplusdomain.SupplierProvisionJob, bool, error) {
	job.ID = r.nextJobID
	r.nextJobID++
	job.TotalSteps = len(steps)
	r.jobs[job.ID] = cloneProvisionJob(job)
	for _, step := range steps {
		if step == nil {
			continue
		}
		step.ID = r.nextStepID
		r.nextStepID++
		step.JobID = job.ID
		r.steps[step.ID] = cloneProvisionStep(step)
		r.jobSteps[job.ID] = append(r.jobSteps[job.ID], step.ID)
	}
	stored := cloneProvisionJob(r.jobs[job.ID])
	for _, stepID := range r.jobSteps[job.ID] {
		stored.Steps = append(stored.Steps, cloneProvisionStep(r.steps[stepID]))
	}
	return stored, false, nil
}

func (r *provisionJobsMemoryRepository) GetJob(_ context.Context, id int64) (*adminplusdomain.SupplierProvisionJob, error) {
	job := cloneProvisionJob(r.jobs[id])
	if job == nil {
		return nil, sql.ErrNoRows
	}
	for _, stepID := range r.jobSteps[id] {
		job.Steps = append(job.Steps, cloneProvisionStep(r.steps[stepID]))
	}
	return job, nil
}

func (r *provisionJobsMemoryRepository) ListJobs(context.Context, ListFilter) ([]*adminplusdomain.SupplierProvisionJob, error) {
	return nil, nil
}

func (r *provisionJobsMemoryRepository) ClaimNextJob(_ context.Context, workerID string, now time.Time, lease time.Duration) (*adminplusdomain.SupplierProvisionJob, error) {
	for _, job := range r.jobs {
		if job.Status != adminplusdomain.SupplierProvisionStatusQueued && job.Status != adminplusdomain.SupplierProvisionStatusRetryableFailed {
			continue
		}
		if job.NextRunAt.After(now) {
			continue
		}
		job.LockedBy = workerID
		lockedUntil := now.Add(lease)
		job.LockedUntil = &lockedUntil
		return cloneProvisionJob(job), nil
	}
	return nil, nil
}

func (r *provisionJobsMemoryRepository) MarkJobRunning(_ context.Context, jobID int64, workerID string, now time.Time) error {
	job := r.jobs[jobID]
	job.Status = adminplusdomain.SupplierProvisionStatusRunning
	job.Attempts++
	job.LockedBy = workerID
	lockedUntil := now.Add(defaultLease)
	job.LockedUntil = &lockedUntil
	job.UpdatedAt = now
	return nil
}

func (r *provisionJobsMemoryRepository) MarkJobSucceeded(_ context.Context, jobID int64, result map[string]any, now time.Time) error {
	job := r.jobs[jobID]
	job.Status = adminplusdomain.SupplierProvisionStatusSucceeded
	job.ResultSnapshot = cloneProvisionMap(result)
	job.SucceededSteps = job.TotalSteps
	job.UpdatedAt = now
	job.FinishedAt = &now
	return nil
}

func (r *provisionJobsMemoryRepository) MarkJobFailed(_ context.Context, jobID int64, status adminplusdomain.SupplierProvisionStatus, errorCode string, errorMessage string, nextRunAt time.Time, now time.Time) error {
	job := r.jobs[jobID]
	job.Status = status
	job.ErrorCode = errorCode
	job.ErrorMessage = errorMessage
	job.NextRunAt = nextRunAt
	job.SucceededSteps = 0
	job.FailedSteps = 0
	job.ManualRequiredSteps = 0
	for _, stepID := range r.jobSteps[jobID] {
		switch r.steps[stepID].Status {
		case adminplusdomain.SupplierProvisionStatusSucceeded:
			job.SucceededSteps++
		case adminplusdomain.SupplierProvisionStatusRetryableFailed, adminplusdomain.SupplierProvisionStatusDead:
			job.FailedSteps++
		case adminplusdomain.SupplierProvisionStatusManualRequired:
			job.ManualRequiredSteps++
		}
	}
	job.UpdatedAt = now
	if status == adminplusdomain.SupplierProvisionStatusDead || status == adminplusdomain.SupplierProvisionStatusManualRequired || status == adminplusdomain.SupplierProvisionStatusPartialSucceeded {
		job.FinishedAt = &now
	}
	return nil
}

func (r *provisionJobsMemoryRepository) ListRunnableSteps(_ context.Context, jobID int64, now time.Time) ([]*adminplusdomain.SupplierProvisionStep, error) {
	out := make([]*adminplusdomain.SupplierProvisionStep, 0)
	for _, stepID := range r.jobSteps[jobID] {
		step := r.steps[stepID]
		if step == nil {
			continue
		}
		if step.Status != adminplusdomain.SupplierProvisionStatusQueued && step.Status != adminplusdomain.SupplierProvisionStatusRetryableFailed {
			continue
		}
		if step.NextRunAt.After(now) {
			continue
		}
		out = append(out, cloneProvisionStep(step))
	}
	return out, nil
}

func (r *provisionJobsMemoryRepository) ListJobSteps(_ context.Context, jobID int64) ([]*adminplusdomain.SupplierProvisionStep, error) {
	out := make([]*adminplusdomain.SupplierProvisionStep, 0)
	for _, stepID := range r.jobSteps[jobID] {
		out = append(out, cloneProvisionStep(r.steps[stepID]))
	}
	return out, nil
}

func (r *provisionJobsMemoryRepository) MarkStepRunning(_ context.Context, stepID int64, workerID string, now time.Time) error {
	step := r.steps[stepID]
	step.Status = adminplusdomain.SupplierProvisionStatusRunning
	step.Attempts++
	step.LockedBy = workerID
	step.UpdatedAt = now
	return nil
}

func (r *provisionJobsMemoryRepository) MarkStepSucceeded(_ context.Context, stepID int64, result map[string]any, now time.Time) error {
	step := r.steps[stepID]
	step.Status = adminplusdomain.SupplierProvisionStatusSucceeded
	step.ResultSnapshot = cloneProvisionMap(result)
	step.UpdatedAt = now
	step.FinishedAt = &now
	return nil
}

func (r *provisionJobsMemoryRepository) MarkStepFailed(_ context.Context, stepID int64, status adminplusdomain.SupplierProvisionStatus, errorCode string, errorMessage string, nextRunAt time.Time, now time.Time) error {
	step := r.steps[stepID]
	step.Status = status
	step.ErrorCode = errorCode
	step.ErrorMessage = errorMessage
	step.NextRunAt = nextRunAt
	step.UpdatedAt = now
	return nil
}

func (r *provisionJobsMemoryRepository) SkipPendingSteps(_ context.Context, jobID int64, errorCode string, errorMessage string, now time.Time) error {
	for _, stepID := range r.jobSteps[jobID] {
		step := r.steps[stepID]
		if step == nil {
			continue
		}
		if step.Status != adminplusdomain.SupplierProvisionStatusQueued && step.Status != adminplusdomain.SupplierProvisionStatusRetryableFailed {
			continue
		}
		step.Status = adminplusdomain.SupplierProvisionStatusSkipped
		step.ErrorCode = errorCode
		step.ErrorMessage = errorMessage
		step.UpdatedAt = now
		step.FinishedAt = &now
	}
	return nil
}

func (r *provisionJobsMemoryRepository) RecordAttempt(context.Context, Attempt) error {
	return nil
}

func (r *provisionJobsMemoryRepository) ListPendingOutboxEvents(context.Context, int, time.Time) ([]OutboxEvent, error) {
	return nil, nil
}

func (r *provisionJobsMemoryRepository) MarkOutboxPublished(context.Context, string, time.Time) error {
	return nil
}

func (r *provisionJobsMemoryRepository) MarkOutboxFailed(context.Context, string, time.Time, error) error {
	return nil
}

func cloneProvisionJob(in *adminplusdomain.SupplierProvisionJob) *adminplusdomain.SupplierProvisionJob {
	if in == nil {
		return nil
	}
	out := *in
	out.RequestSnapshot = cloneProvisionMap(in.RequestSnapshot)
	out.ResultSnapshot = cloneProvisionMap(in.ResultSnapshot)
	if in.LockedUntil != nil {
		value := *in.LockedUntil
		out.LockedUntil = &value
	}
	if in.FinishedAt != nil {
		value := *in.FinishedAt
		out.FinishedAt = &value
	}
	out.Steps = nil
	return &out
}

func cloneProvisionStep(in *adminplusdomain.SupplierProvisionStep) *adminplusdomain.SupplierProvisionStep {
	if in == nil {
		return nil
	}
	out := *in
	out.RequestSnapshot = cloneProvisionMap(in.RequestSnapshot)
	out.ResultSnapshot = cloneProvisionMap(in.ResultSnapshot)
	if in.LockedUntil != nil {
		value := *in.LockedUntil
		out.LockedUntil = &value
	}
	if in.FinishedAt != nil {
		value := *in.FinishedAt
		out.FinishedAt = &value
	}
	return &out
}

func cloneProvisionMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

var _ Repository = (*provisionJobsMemoryRepository)(nil)
