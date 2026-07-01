package domain

import "time"

type SchedulerRun struct {
	RunID         string              `json:"run_id"`
	Mode          string              `json:"mode"`
	DryRun        bool                `json:"dry_run"`
	RequestedAt   time.Time           `json:"requested_at"`
	TaskTypes     []ExtensionTaskType `json:"task_types"`
	CreatedCount  int                 `json:"created_count"`
	SkippedCount  int                 `json:"skipped_count"`
	EligibleCount int                 `json:"eligible_count"`
	Items         []ScheduledTask     `json:"items"`
}

type SchedulerStatus struct {
	Enabled         bool   `json:"enabled"`
	IntervalSeconds int64  `json:"interval_seconds"`
	Queue           string `json:"queue"`
}

type SchedulerCenterStatus struct {
	Enabled         bool       `json:"enabled"`
	WorkerStatus    string     `json:"worker_status"`
	Queue           string     `json:"queue"`
	IntervalSeconds int64      `json:"interval_seconds"`
	RunningSteps    int        `json:"running_steps"`
	QueuedSteps     int        `json:"queued_steps"`
	FailedSteps     int        `json:"failed_steps"`
	OverduePlans    int        `json:"overdue_plans"`
	OpenActions     int        `json:"open_actions"`
	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
	NextRunAt       *time.Time `json:"next_run_at,omitempty"`
}

type SchedulerPlan struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	TaskType          string     `json:"task_type"`
	TaskTypes         []string   `json:"task_types,omitempty"`
	Status            string     `json:"status"`
	Scope             string     `json:"scope"`
	FrequencyLabel    string     `json:"frequency_label"`
	IntervalSeconds   int64      `json:"interval_seconds"`
	WindowMinutes     int        `json:"window_minutes"`
	MisfirePolicy     string     `json:"misfire_policy"`
	ConcurrencyPolicy string     `json:"concurrency_policy"`
	HighCost          bool       `json:"high_cost"`
	Description       string     `json:"description"`
	LastRunAt         *time.Time `json:"last_run_at,omitempty"`
	LastSuccessAt     *time.Time `json:"last_success_at,omitempty"`
	IssueCount        int        `json:"issue_count"`
	LastIssueAt       *time.Time `json:"last_issue_at,omitempty"`
	LastIssue         string     `json:"last_issue,omitempty"`
	NextRunAt         *time.Time `json:"next_run_at,omitempty"`
}

type SchedulerPlanConfig struct {
	Status            string `json:"status"`
	Scope             string `json:"scope"`
	IntervalSeconds   int64  `json:"interval_seconds"`
	WindowMinutes     int    `json:"window_minutes"`
	MisfirePolicy     string `json:"misfire_policy"`
	ConcurrencyPolicy string `json:"concurrency_policy"`
}

type SchedulerPlanStats struct {
	PlanID        string     `json:"plan_id"`
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
	IssueCount    int        `json:"issue_count"`
	LastIssueAt   *time.Time `json:"last_issue_at,omitempty"`
	LastIssue     string     `json:"last_issue,omitempty"`
}

type SchedulerRunSummary struct {
	ID              string         `json:"id"`
	LegacyRunID     string         `json:"legacy_run_id,omitempty"`
	TriggerType     string         `json:"trigger_type"`
	TaskType        string         `json:"task_type"`
	Status          string         `json:"status"`
	RequestedAt     time.Time      `json:"requested_at"`
	StartedAt       *time.Time     `json:"started_at,omitempty"`
	FinishedAt      *time.Time     `json:"finished_at,omitempty"`
	SupplierCount   int            `json:"supplier_count"`
	TotalSteps      int            `json:"total_steps"`
	SucceededSteps  int            `json:"succeeded_steps"`
	FailedSteps     int            `json:"failed_steps"`
	SkippedSteps    int            `json:"skipped_steps"`
	DurationMS      int64          `json:"duration_ms"`
	ErrorCode       string         `json:"error_code,omitempty"`
	ErrorMessage    string         `json:"error_message,omitempty"`
	RequestSnapshot map[string]any `json:"request_snapshot,omitempty"`
	ResultSnapshot  map[string]any `json:"result_snapshot,omitempty"`
}

type SchedulerRunDetail struct {
	Run   SchedulerRunSummary   `json:"run"`
	Steps []SchedulerStepRecord `json:"steps"`
}

type SchedulerAttemptRecord struct {
	ID               int64             `json:"id"`
	StepID           int64             `json:"step_id"`
	RunID            string            `json:"run_id"`
	SupplierID       int64             `json:"supplier_id"`
	TaskType         ExtensionTaskType `json:"task_type"`
	Status           string            `json:"status"`
	WorkerID         string            `json:"worker_id,omitempty"`
	AttemptNo        int               `json:"attempt_no"`
	StartedAt        *time.Time        `json:"started_at,omitempty"`
	FinishedAt       time.Time         `json:"finished_at"`
	DurationMS       int64             `json:"duration_ms"`
	ErrorCode        string            `json:"error_code,omitempty"`
	ErrorMessage     string            `json:"error_message,omitempty"`
	RequestSnapshot  map[string]any    `json:"request_snapshot,omitempty"`
	ResponseSnapshot map[string]any    `json:"response_snapshot,omitempty"`
}

type SchedulerStepRecord struct {
	ID              int64                    `json:"id"`
	RunID           string                   `json:"run_id"`
	SupplierID      int64                    `json:"supplier_id"`
	SupplierName    string                   `json:"supplier_name"`
	TaskType        ExtensionTaskType        `json:"task_type"`
	Action          string                   `json:"action"`
	Status          string                   `json:"status"`
	ScheduleKey     string                   `json:"schedule_key"`
	ExtensionTaskID int64                    `json:"extension_task_id,omitempty"`
	ResultCount     int                      `json:"result_count"`
	Reason          string                   `json:"reason,omitempty"`
	Attempts        int                      `json:"attempts"`
	MaxAttempts     int                      `json:"max_attempts"`
	NextAttemptAt   *time.Time               `json:"next_attempt_at,omitempty"`
	LockedBy        string                   `json:"locked_by,omitempty"`
	LockedUntil     *time.Time               `json:"locked_until,omitempty"`
	RequestSnapshot map[string]any           `json:"request_snapshot,omitempty"`
	ResultSnapshot  map[string]any           `json:"result_snapshot,omitempty"`
	StartedAt       *time.Time               `json:"started_at,omitempty"`
	FinishedAt      *time.Time               `json:"finished_at,omitempty"`
	OperationLogs   []SchedulerAttemptRecord `json:"operation_logs,omitempty"`
}

type SchedulerSupplierStatus struct {
	SupplierID        int64  `json:"supplier_id"`
	SupplierName      string `json:"supplier_name"`
	SupplierType      string `json:"supplier_type"`
	RuntimeStatus     string `json:"runtime_status"`
	HealthStatus      string `json:"health_status"`
	BalanceCents      int64  `json:"balance_cents"`
	BalanceCurrency   string `json:"balance_currency"`
	CompletionPercent int    `json:"completion_percent"`
	SessionStatus     string `json:"session_status"`
	BalanceStatus     string `json:"balance_status"`
	GroupStatus       string `json:"group_status"`
	RateStatus        string `json:"rate_status"`
	BillingStatus     string `json:"billing_status"`
	ChannelStatus     string `json:"channel_status"`
	ScheduleStatus    string `json:"schedule_status"`
	LastError         string `json:"last_error,omitempty"`
	RecommendedAction string `json:"recommended_action,omitempty"`
}

type SchedulerSupplierChecklist struct {
	SupplierID        int64                            `json:"supplier_id"`
	SupplierName      string                           `json:"supplier_name"`
	SupplierType      string                           `json:"supplier_type"`
	CompletionPercent int                              `json:"completion_percent"`
	RecommendedAction string                           `json:"recommended_action,omitempty"`
	Items             []SchedulerSupplierChecklistItem `json:"items"`
}

type SchedulerSupplierChecklistItem struct {
	Key               string     `json:"key"`
	Label             string     `json:"label"`
	Status            string     `json:"status"`
	Description       string     `json:"description"`
	Evidence          string     `json:"evidence,omitempty"`
	RecommendedAction string     `json:"recommended_action,omitempty"`
	LastCheckedAt     *time.Time `json:"last_checked_at,omitempty"`
}

type SchedulerAction struct {
	ID                   string     `json:"id"`
	SupplierID           int64      `json:"supplier_id,omitempty"`
	SupplierName         string     `json:"supplier_name,omitempty"`
	Severity             string     `json:"severity"`
	Status               string     `json:"status"`
	Type                 string     `json:"type"`
	Title                string     `json:"title"`
	Reason               string     `json:"reason"`
	RecommendedOperation string     `json:"recommended_operation"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	ResolvedAt           *time.Time `json:"resolved_at,omitempty"`
}

type SchedulerSettings struct {
	Enabled                       bool     `json:"enabled"`
	DefaultSupplierConcurrency    int      `json:"default_supplier_concurrency"`
	ChannelChecksEnabled          bool     `json:"channel_checks_enabled"`
	ChannelCheckDailyBudgetTokens int      `json:"channel_check_daily_budget_tokens"`
	FirstTokenSlowThresholdMS     int64    `json:"first_token_slow_threshold_ms"`
	TotalLatencySlowThresholdMS   int64    `json:"total_latency_slow_threshold_ms"`
	DefaultEnabledTaskTypes       []string `json:"default_enabled_task_types"`
	HighCostTaskTypes             []string `json:"high_cost_task_types"`
}

type ScheduledTask struct {
	SupplierID   int64             `json:"supplier_id"`
	SupplierName string            `json:"supplier_name"`
	TaskType     ExtensionTaskType `json:"task_type"`
	Action       string            `json:"action"`
	TaskID       int64             `json:"task_id,omitempty"`
	ScheduleKey  string            `json:"schedule_key"`
	Request      map[string]any    `json:"request_snapshot,omitempty"`
	Result       map[string]any    `json:"result_snapshot,omitempty"`
	Created      bool              `json:"created"`
	Synced       bool              `json:"synced,omitempty"`
	Total        int               `json:"total,omitempty"`
	Reason       string            `json:"reason,omitempty"`
}
