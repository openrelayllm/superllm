package domain

import "time"

type ActionType string

const (
	ActionTypeSwitchSupplier    ActionType = "switch_supplier"
	ActionTypePauseSupplier     ActionType = "pause_supplier"
	ActionTypeDegradeSupplier   ActionType = "degrade_supplier"
	ActionTypeIncreaseWeight    ActionType = "increase_weight"
	ActionTypeRechargeSupplier  ActionType = "recharge_supplier"
	ActionTypeInvestigateProfit ActionType = "investigate_profit"
	ActionTypeReviewCredential  ActionType = "review_credential"
)

type ActionSeverity string

const (
	ActionSeverityInfo     ActionSeverity = "info"
	ActionSeverityWarning  ActionSeverity = "warning"
	ActionSeverityCritical ActionSeverity = "critical"
)

type ActionStatus string

const (
	ActionStatusOpen         ActionStatus = "open"
	ActionStatusAcknowledged ActionStatus = "acknowledged"
	ActionStatusApproved     ActionStatus = "approved"
	ActionStatusExecuted     ActionStatus = "executed"
	ActionStatusRejected     ActionStatus = "rejected"
)

type ActionExecutionStatus string

const (
	ActionExecutionStatusRunning     ActionExecutionStatus = "running"
	ActionExecutionStatusSucceeded   ActionExecutionStatus = "succeeded"
	ActionExecutionStatusFailed      ActionExecutionStatus = "failed"
	ActionExecutionStatusUnsupported ActionExecutionStatus = "unsupported"
)

type ActionRecommendation struct {
	ID               int64          `json:"id"`
	SupplierID       int64          `json:"supplier_id"`
	TargetSupplierID *int64         `json:"target_supplier_id,omitempty"`
	Type             ActionType     `json:"type"`
	Severity         ActionSeverity `json:"severity"`
	Status           ActionStatus   `json:"status"`
	ReasonCode       string         `json:"reason_code"`
	Title            string         `json:"title"`
	Description      string         `json:"description"`
	ExpectedImpact   string         `json:"expected_impact,omitempty"`
	RequiresApproval bool           `json:"requires_approval"`
	Signals          []string       `json:"signals,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
}

type ActionExecution struct {
	ID               int64                 `json:"id"`
	RecommendationID int64                 `json:"recommendation_id"`
	ActionType       ActionType            `json:"action_type"`
	SupplierID       int64                 `json:"supplier_id"`
	TargetSupplierID *int64                `json:"target_supplier_id,omitempty"`
	Status           ActionExecutionStatus `json:"status"`
	RequestPayload   map[string]any        `json:"request_payload,omitempty"`
	ResponsePayload  map[string]any        `json:"response_payload,omitempty"`
	ErrorMessage     string                `json:"error_message,omitempty"`
	OperatorUserID   int64                 `json:"operator_user_id,omitempty"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

func (t ActionType) Valid() bool {
	switch t {
	case ActionTypeSwitchSupplier, ActionTypePauseSupplier, ActionTypeDegradeSupplier, ActionTypeIncreaseWeight, ActionTypeRechargeSupplier, ActionTypeInvestigateProfit, ActionTypeReviewCredential:
		return true
	default:
		return false
	}
}

func (s ActionSeverity) Valid() bool {
	switch s {
	case ActionSeverityInfo, ActionSeverityWarning, ActionSeverityCritical:
		return true
	default:
		return false
	}
}

func (s ActionStatus) Valid() bool {
	switch s {
	case ActionStatusOpen, ActionStatusAcknowledged, ActionStatusApproved, ActionStatusExecuted, ActionStatusRejected:
		return true
	default:
		return false
	}
}

func (s ActionExecutionStatus) Valid() bool {
	switch s {
	case ActionExecutionStatusRunning, ActionExecutionStatusSucceeded, ActionExecutionStatusFailed, ActionExecutionStatusUnsupported:
		return true
	default:
		return false
	}
}
