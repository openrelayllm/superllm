package domain

import "time"

type SupplierProvisionJobType string

const (
	SupplierProvisionJobTypeSyncGroups            SupplierProvisionJobType = "sync_groups"
	SupplierProvisionJobTypeProvisionGroupKey     SupplierProvisionJobType = "provision_group_key"
	SupplierProvisionJobTypeProvisionAllGroupKeys SupplierProvisionJobType = "provision_all_group_keys"
	SupplierProvisionJobTypeRepairBinding         SupplierProvisionJobType = "repair_binding"
	SupplierProvisionJobTypeSyncSupplierCosts     SupplierProvisionJobType = "sync_supplier_costs"
)

type SupplierProvisionStatus string

const (
	SupplierProvisionStatusQueued           SupplierProvisionStatus = "queued"
	SupplierProvisionStatusRunning          SupplierProvisionStatus = "running"
	SupplierProvisionStatusSucceeded        SupplierProvisionStatus = "succeeded"
	SupplierProvisionStatusPartialSucceeded SupplierProvisionStatus = "partial_succeeded"
	SupplierProvisionStatusRetryableFailed  SupplierProvisionStatus = "retryable_failed"
	SupplierProvisionStatusManualRequired   SupplierProvisionStatus = "manual_required"
	SupplierProvisionStatusDead             SupplierProvisionStatus = "dead"
	SupplierProvisionStatusCancelled        SupplierProvisionStatus = "cancelled"
)

type SupplierProvisionStepType string

const (
	SupplierProvisionStepEnsureSupplierSession    SupplierProvisionStepType = "ensure_supplier_session"
	SupplierProvisionStepSyncSupplierGroup        SupplierProvisionStepType = "sync_supplier_group"
	SupplierProvisionStepEnsureThirdPartyKey      SupplierProvisionStepType = "ensure_third_party_key"
	SupplierProvisionStepEnsureSub2APIGroup       SupplierProvisionStepType = "ensure_sub2api_group"
	SupplierProvisionStepEnsureSub2APIAccount     SupplierProvisionStepType = "ensure_sub2api_account"
	SupplierProvisionStepUpsertAdminPlusBinding   SupplierProvisionStepType = "upsert_admin_plus_binding"
	SupplierProvisionStepEnqueueInitialCollection SupplierProvisionStepType = "enqueue_initial_collection"
	SupplierProvisionStepProvisionAllGroupKeys    SupplierProvisionStepType = "provision_all_group_keys"
	SupplierProvisionStepRepairBinding            SupplierProvisionStepType = "repair_binding"
	SupplierProvisionStepSyncSupplierCosts        SupplierProvisionStepType = "sync_supplier_costs"
)

type SupplierProvisionJob struct {
	ID                  int64                    `json:"id"`
	JobType             SupplierProvisionJobType `json:"job_type"`
	SupplierID          int64                    `json:"supplier_id"`
	Status              SupplierProvisionStatus  `json:"status"`
	IdempotencyKeyHash  string                   `json:"idempotency_key_hash,omitempty"`
	RequestedBy         int64                    `json:"requested_by,omitempty"`
	RequestSnapshot     map[string]any           `json:"request_snapshot,omitempty"`
	ResultSnapshot      map[string]any           `json:"result_snapshot,omitempty"`
	TotalSteps          int                      `json:"total_steps"`
	SucceededSteps      int                      `json:"succeeded_steps"`
	FailedSteps         int                      `json:"failed_steps"`
	ManualRequiredSteps int                      `json:"manual_required_steps"`
	Attempts            int                      `json:"attempts"`
	MaxAttempts         int                      `json:"max_attempts"`
	NextRunAt           time.Time                `json:"next_run_at"`
	LockedBy            string                   `json:"locked_by,omitempty"`
	LockedUntil         *time.Time               `json:"locked_until,omitempty"`
	ErrorCode           string                   `json:"error_code,omitempty"`
	ErrorMessage        string                   `json:"error_message,omitempty"`
	CreatedAt           time.Time                `json:"created_at"`
	UpdatedAt           time.Time                `json:"updated_at"`
	FinishedAt          *time.Time               `json:"finished_at,omitempty"`
	Steps               []*SupplierProvisionStep `json:"steps,omitempty"`
}

type SupplierProvisionStep struct {
	ID                   int64                     `json:"id"`
	JobID                int64                     `json:"job_id"`
	SupplierID           int64                     `json:"supplier_id"`
	SupplierGroupID      int64                     `json:"supplier_group_id,omitempty"`
	StepType             SupplierProvisionStepType `json:"step_type"`
	Status               SupplierProvisionStatus   `json:"status"`
	IdempotencyKey       string                    `json:"idempotency_key,omitempty"`
	ExternalResourceType string                    `json:"external_resource_type,omitempty"`
	ExternalResourceID   string                    `json:"external_resource_id,omitempty"`
	Attempts             int                       `json:"attempts"`
	MaxAttempts          int                       `json:"max_attempts"`
	NextRunAt            time.Time                 `json:"next_run_at"`
	LockedBy             string                    `json:"locked_by,omitempty"`
	LockedUntil          *time.Time                `json:"locked_until,omitempty"`
	ErrorCode            string                    `json:"error_code,omitempty"`
	ErrorMessage         string                    `json:"error_message,omitempty"`
	RequestSnapshot      map[string]any            `json:"request_snapshot,omitempty"`
	ResultSnapshot       map[string]any            `json:"result_snapshot,omitempty"`
	CreatedAt            time.Time                 `json:"created_at"`
	UpdatedAt            time.Time                 `json:"updated_at"`
	FinishedAt           *time.Time                `json:"finished_at,omitempty"`
}
