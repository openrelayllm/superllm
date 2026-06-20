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
