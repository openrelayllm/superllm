package domain

import "time"

type ExtensionTaskType string

const (
	ExtensionTaskTypeFetchRates         ExtensionTaskType = "fetch_rates"
	ExtensionTaskTypeFetchGroups        ExtensionTaskType = "fetch_groups"
	ExtensionTaskTypeFetchBalance       ExtensionTaskType = "fetch_balance"
	ExtensionTaskTypeFetchAnnouncements ExtensionTaskType = "fetch_announcements"
	ExtensionTaskTypeExportBills        ExtensionTaskType = "export_bills"
	ExtensionTaskTypeFetchUsageCosts    ExtensionTaskType = "fetch_usage_costs"
	ExtensionTaskTypeReconcileCosts     ExtensionTaskType = "reconcile_supplier_costs"
	ExtensionTaskTypeFetchHealth        ExtensionTaskType = "fetch_health"
	ExtensionTaskTypeCheckChannels      ExtensionTaskType = "check_supplier_channels"
	ExtensionTaskTypeCaptureSession     ExtensionTaskType = "capture_supplier_session"
	ExtensionTaskTypeRegisterSupplier   ExtensionTaskType = "register_supplier_account"
)

func (t ExtensionTaskType) Valid() bool {
	switch t {
	case ExtensionTaskTypeFetchRates, ExtensionTaskTypeFetchGroups, ExtensionTaskTypeFetchBalance, ExtensionTaskTypeFetchAnnouncements, ExtensionTaskTypeExportBills, ExtensionTaskTypeFetchUsageCosts, ExtensionTaskTypeReconcileCosts, ExtensionTaskTypeFetchHealth, ExtensionTaskTypeCheckChannels, ExtensionTaskTypeCaptureSession, ExtensionTaskTypeRegisterSupplier:
		return true
	default:
		return false
	}
}

type ExtensionTaskStatus string

const (
	ExtensionTaskStatusPending   ExtensionTaskStatus = "pending"
	ExtensionTaskStatusClaimed   ExtensionTaskStatus = "claimed"
	ExtensionTaskStatusRunning   ExtensionTaskStatus = "running"
	ExtensionTaskStatusSucceeded ExtensionTaskStatus = "succeeded"
	ExtensionTaskStatusFailed    ExtensionTaskStatus = "failed"
	ExtensionTaskStatusCancelled ExtensionTaskStatus = "cancelled"
)

func (s ExtensionTaskStatus) Valid() bool {
	switch s {
	case ExtensionTaskStatusPending, ExtensionTaskStatusClaimed, ExtensionTaskStatusRunning, ExtensionTaskStatusSucceeded, ExtensionTaskStatusFailed, ExtensionTaskStatusCancelled:
		return true
	default:
		return false
	}
}

type ExtensionTask struct {
	ID              int64               `json:"id"`
	SupplierID      int64               `json:"supplier_id"`
	Type            ExtensionTaskType   `json:"type"`
	ScheduleKey     string              `json:"schedule_key,omitempty"`
	Status          ExtensionTaskStatus `json:"status"`
	Priority        int                 `json:"priority"`
	Attempts        int                 `json:"attempts"`
	MaxAttempts     int                 `json:"max_attempts"`
	DeviceID        string              `json:"device_id,omitempty"`
	LeaseToken      string              `json:"lease_token,omitempty"`
	LeaseExpiresAt  *time.Time          `json:"lease_expires_at,omitempty"`
	LastHeartbeatAt *time.Time          `json:"last_heartbeat_at,omitempty"`
	AvailableAfter  time.Time           `json:"available_after"`
	Payload         map[string]any      `json:"payload,omitempty"`
	Result          map[string]any      `json:"result,omitempty"`
	ErrorCode       string              `json:"error_code,omitempty"`
	ErrorMessage    string              `json:"error_message,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
	FinishedAt      *time.Time          `json:"finished_at,omitempty"`
}
