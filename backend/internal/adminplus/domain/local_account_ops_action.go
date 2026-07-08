package domain

type LocalAccountOpsAction string

const (
	LocalAccountOpsActionSetSchedulable   LocalAccountOpsAction = "set_schedulable"
	LocalAccountOpsActionAddToGroups      LocalAccountOpsAction = "add_to_groups"
	LocalAccountOpsActionRemoveFromGroups LocalAccountOpsAction = "remove_from_groups"
)

type LocalAccountOpsGroupImpact struct {
	GroupID                   int64  `json:"group_id"`
	GroupName                 string `json:"group_name"`
	ActiveAPIKeyCount         int64  `json:"active_api_key_count"`
	BeforeSchedulableAccounts int64  `json:"before_schedulable_accounts"`
	AfterSchedulableAccounts  int64  `json:"after_schedulable_accounts"`
	WouldEmptySchedulablePool bool   `json:"would_empty_schedulable_pool"`
}

type LocalAccountOpsActionResult struct {
	Action                 LocalAccountOpsAction        `json:"action"`
	DryRun                 bool                         `json:"dry_run"`
	Blocked                bool                         `json:"blocked"`
	BlockedReason          string                       `json:"blocked_reason,omitempty"`
	AccountIDs             []int64                      `json:"account_ids"`
	GroupIDs               []int64                      `json:"group_ids,omitempty"`
	UpdatedAccounts        int64                        `json:"updated_accounts"`
	AddedBindings          int64                        `json:"added_bindings"`
	RemovedBindings        int64                        `json:"removed_bindings"`
	GroupImpacts           []LocalAccountOpsGroupImpact `json:"group_impacts,omitempty"`
	Warnings               []string                     `json:"warnings,omitempty"`
	ActionRecommendationID int64                        `json:"action_recommendation_id,omitempty"`
	ActionExecutionID      int64                        `json:"action_execution_id,omitempty"`
}

func (a LocalAccountOpsAction) Valid() bool {
	switch a {
	case LocalAccountOpsActionSetSchedulable, LocalAccountOpsActionAddToGroups, LocalAccountOpsActionRemoveFromGroups:
		return true
	default:
		return false
	}
}
