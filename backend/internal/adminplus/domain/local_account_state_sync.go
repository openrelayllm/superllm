package domain

import "time"

type LocalAccountStateSyncResult struct {
	CheckedAccounts      int64                           `json:"checked_accounts"`
	SyncedAccounts       int64                           `json:"synced_accounts"`
	DriftedAccounts      int64                           `json:"drifted_accounts"`
	PendingDriftAccounts int64                           `json:"pending_drift_accounts"`
	Items                []LocalAccountStateDriftSummary `json:"items,omitempty"`
}

type LocalAccountStateResolutionAction string

const (
	LocalAccountStateResolutionAcceptObserved  LocalAccountStateResolutionAction = "accept_observed"
	LocalAccountStateResolutionRestoreAccepted LocalAccountStateResolutionAction = "restore_accepted"
)

func (a LocalAccountStateResolutionAction) Valid() bool {
	switch a {
	case LocalAccountStateResolutionAcceptObserved, LocalAccountStateResolutionRestoreAccepted:
		return true
	default:
		return false
	}
}

type LocalAccountStateResolutionResult struct {
	Action               LocalAccountStateResolutionAction `json:"action"`
	AccountIDs           []int64                           `json:"account_ids"`
	ResolvedAccounts     int64                             `json:"resolved_accounts"`
	RestoredAccounts     int64                             `json:"restored_accounts"`
	PendingDriftAccounts int64                             `json:"pending_drift_accounts"`
	Items                []LocalAccountStateDriftSummary   `json:"items,omitempty"`
	Warnings             []string                          `json:"warnings,omitempty"`
}

type LocalAccountStateDriftSummary struct {
	LocalSub2APIAccountID int64                     `json:"local_sub2api_account_id"`
	AccountName           string                    `json:"account_name"`
	Accepted              LocalAccountStateSnapshot `json:"accepted"`
	Observed              LocalAccountStateSnapshot `json:"observed"`
	DriftFields           []string                  `json:"drift_fields"`
	FirstDetectedAt       *time.Time                `json:"first_detected_at,omitempty"`
	LastCheckedAt         time.Time                 `json:"last_checked_at"`
}

type LocalAccountStateSnapshot struct {
	Name        string  `json:"name"`
	Platform    string  `json:"platform"`
	Type        string  `json:"type"`
	Schedulable bool    `json:"schedulable"`
	GroupIDs    []int64 `json:"group_ids,omitempty"`
}
