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

type ScheduledTask struct {
	SupplierID   int64             `json:"supplier_id"`
	SupplierName string            `json:"supplier_name"`
	TaskType     ExtensionTaskType `json:"task_type"`
	TaskID       int64             `json:"task_id,omitempty"`
	ScheduleKey  string            `json:"schedule_key"`
	Created      bool              `json:"created"`
	Reason       string            `json:"reason,omitempty"`
}
