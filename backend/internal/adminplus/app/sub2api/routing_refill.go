package sub2api

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/candidateeval"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	defaultRoutingRefillCooldown                = 3 * time.Minute
	defaultRoutingRefillFailedCandidateCooldown = 10 * time.Minute
)

type RoutingRefillInput struct {
	LocalGroupID                   int64   `json:"local_group_id"`
	Platform                       string  `json:"platform,omitempty"`
	ModelScope                     string  `json:"model_scope,omitempty"`
	MaxRateMultiplier              float64 `json:"max_rate_multiplier,omitempty"`
	Limit                          int     `json:"limit,omitempty"`
	DryRun                         bool    `json:"dry_run"`
	ActionRecommendationID         int64   `json:"action_id,omitempty"`
	Reason                         string  `json:"reason,omitempty"`
	TriggerType                    string  `json:"trigger_type,omitempty"`
	RequestedBy                    int64   `json:"requested_by,omitempty"`
	CooldownSeconds                int     `json:"cooldown_seconds,omitempty"`
	ConfirmWindowSecs              int     `json:"confirm_window_seconds,omitempty"`
	FailedCandidateCooldownSeconds int     `json:"failed_candidate_cooldown_seconds,omitempty"`
}

type RoutingRefillCandidate struct {
	LocalSub2APIAccountID   int64   `json:"local_sub2api_account_id"`
	LocalAccountName        string  `json:"local_account_name,omitempty"`
	LocalAccountPlatform    string  `json:"local_account_platform,omitempty"`
	SupplierID              int64   `json:"supplier_id,omitempty"`
	SupplierName            string  `json:"supplier_name,omitempty"`
	SupplierGroupID         int64   `json:"supplier_group_id,omitempty"`
	SupplierGroupName       string  `json:"supplier_group_name,omitempty"`
	SupplierKeyID           int64   `json:"supplier_key_id,omitempty"`
	CandidateStatus         string  `json:"candidate_status"`
	BlockedReason           string  `json:"blocked_reason,omitempty"`
	CheckSource             string  `json:"check_source,omitempty"`
	ModelScope              string  `json:"model_scope,omitempty"`
	ModelMatchStatus        string  `json:"model_match_status,omitempty"`
	EffectiveRateMultiplier float64 `json:"effective_rate_multiplier"`
}

type RoutingRefillResult struct {
	Action             string                    `json:"action"`
	DryRun             bool                      `json:"dry_run"`
	LocalGroupID       int64                     `json:"local_group_id"`
	Platform           string                    `json:"platform,omitempty"`
	ModelScope         string                    `json:"model_scope,omitempty"`
	AvailabilityBefore *RoutingGroupAvailability `json:"availability_before,omitempty"`
	AvailabilityAfter  *RoutingGroupAvailability `json:"availability_after,omitempty"`
	Candidate          *RoutingRefillCandidate   `json:"candidate,omitempty"`
	Account            *Sub2APIAccountSnapshot   `json:"account,omitempty"`
	SkippedReason      string                    `json:"skipped_reason,omitempty"`
}

func (s *Service) RefillLocalGroup(ctx context.Context, in RoutingRefillInput) (result *RoutingRefillResult, err error) {
	defer func() {
		s.recordRoutingRefillRun(ctx, in, result, err)
	}()
	if s == nil || s.repo == nil || s.routing == nil {
		return nil, internalError("sub2api routing service is not configured")
	}
	if in.LocalGroupID <= 0 {
		return nil, badRequest("ROUTING_GROUP_ID_INVALID", "invalid group id")
	}
	platform := strings.TrimSpace(in.Platform)
	modelScope := strings.TrimSpace(in.ModelScope)
	result = &RoutingRefillResult{
		Action:       "refill_local_group",
		DryRun:       in.DryRun,
		LocalGroupID: in.LocalGroupID,
		Platform:     platform,
		ModelScope:   modelScope,
	}
	if !in.DryRun {
		unlock, acquired, lockErr := s.tryRoutingRefillLock(ctx, in.LocalGroupID)
		if lockErr != nil {
			return nil, lockErr
		}
		if !acquired {
			result.SkippedReason = "refill_locked"
			return result, nil
		}
		defer func() {
			if unlock != nil {
				_ = unlock()
			}
		}()
		inCooldown, err := s.routingRefillInCooldown(ctx, in.LocalGroupID, routingRefillCooldown(in.CooldownSeconds))
		if err != nil {
			return nil, err
		}
		if inCooldown {
			result.SkippedReason = "refill_cooldown"
			return result, nil
		}
		confirmed, err := s.routingRefillRecentlyConfirmed(ctx, in.LocalGroupID, routingRefillConfirmWindow(in.ConfirmWindowSecs))
		if err != nil {
			return nil, err
		}
		if !confirmed {
			result.SkippedReason = "refill_confirmation_required"
			return result, nil
		}
	}

	before, err := s.routing.GetGroupAvailability(ctx, in.LocalGroupID, platform)
	if err != nil {
		return nil, err
	}
	result.AvailabilityBefore = before
	if before != nil && before.SchedulableAccounts > 0 {
		result.SkippedReason = "group_has_schedulable_accounts"
		return result, nil
	}

	rows, err := s.ListLocalAccountOps(ctx, LocalAccountOpsFilter{
		MaxRateMultiplier: in.MaxRateMultiplier,
		ModelScope:        modelScope,
		Limit:             normalizeRoutingRefillLimit(in.Limit),
	})
	if err != nil {
		return nil, err
	}
	excludedAccounts := map[int64]struct{}{}
	if !in.DryRun {
		excludedAccounts, err = s.routingRefillRecentlyFailedAccounts(ctx, in.LocalGroupID, routingRefillFailedCandidateCooldown(in.FailedCandidateCooldownSeconds))
		if err != nil {
			return nil, err
		}
	}
	candidate := bestRoutingRefillCandidate(rows, in.LocalGroupID, platform, modelScope, excludedAccounts)
	if candidate == nil {
		if len(excludedAccounts) > 0 && routingRefillHasUsableCandidate(rows, in.LocalGroupID, platform, modelScope) {
			result.SkippedReason = "candidate_suppressed_after_failure"
		} else {
			result.SkippedReason = "candidate_not_found"
		}
		return result, nil
	}
	result.Candidate = routingRefillCandidateFromRow(candidate)
	if in.DryRun {
		return result, nil
	}

	latest, err := s.routing.GetGroupAvailability(ctx, in.LocalGroupID, platform)
	if err != nil {
		return nil, err
	}
	if latest != nil && latest.SchedulableAccounts > 0 {
		result.AvailabilityAfter = latest
		result.SkippedReason = "group_recovered_before_write"
		return result, nil
	}
	account, err := s.routing.EnsureAccountInGroup(ctx, candidate.LocalSub2APIAccountID, in.LocalGroupID)
	if err != nil {
		return nil, err
	}
	result.Account = account
	after, err := s.routing.GetGroupAvailability(ctx, in.LocalGroupID, platform)
	if err != nil {
		return nil, err
	}
	result.AvailabilityAfter = after
	return result, nil
}

func (s *Service) tryRoutingRefillLock(ctx context.Context, groupID int64) (RoutingRefillUnlockFunc, bool, error) {
	locker, ok := s.repo.(RoutingRefillLocker)
	if !ok {
		return func() error { return nil }, true, nil
	}
	return locker.TryRoutingRefillLock(ctx, groupID)
}

func (s *Service) routingRefillInCooldown(ctx context.Context, groupID int64, cooldown time.Duration) (bool, error) {
	if cooldown <= 0 {
		return false, nil
	}
	runs, err := s.repo.ListRoutingRefillRuns(ctx, RoutingRefillRunFilter{
		LocalGroupID: groupID,
		Status:       "succeeded",
		Limit:        1,
	})
	if err != nil {
		return false, err
	}
	if len(runs) == 0 || runs[0] == nil || runs[0].CreatedAt.IsZero() {
		return false, nil
	}
	now := time.Now()
	if s != nil && s.now != nil {
		now = s.now()
	}
	return runs[0].CreatedAt.After(now.Add(-cooldown)), nil
}

func (s *Service) routingRefillRecentlyConfirmed(ctx context.Context, groupID int64, window time.Duration) (bool, error) {
	if window <= 0 {
		return true, nil
	}
	runs, err := s.repo.ListRoutingRefillRuns(ctx, RoutingRefillRunFilter{
		LocalGroupID: groupID,
		Status:       "previewed",
		Limit:        1,
	})
	if err != nil {
		return false, err
	}
	if len(runs) == 0 || runs[0] == nil || runs[0].CreatedAt.IsZero() {
		return false, nil
	}
	now := time.Now()
	if s != nil && s.now != nil {
		now = s.now()
	}
	return runs[0].CreatedAt.After(now.Add(-window)), nil
}

func (s *Service) routingRefillRecentlyFailedAccounts(ctx context.Context, groupID int64, window time.Duration) (map[int64]struct{}, error) {
	out := make(map[int64]struct{})
	if window <= 0 {
		return out, nil
	}
	runs, err := s.repo.ListRoutingRefillRuns(ctx, RoutingRefillRunFilter{
		LocalGroupID: groupID,
		Status:       "failed",
		Limit:        100,
	})
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if s != nil && s.now != nil {
		now = s.now()
	}
	cutoff := now.Add(-window)
	for _, run := range runs {
		if run == nil || run.SelectedLocalAccountID <= 0 || run.CreatedAt.IsZero() {
			continue
		}
		if run.CreatedAt.Before(cutoff) {
			continue
		}
		out[run.SelectedLocalAccountID] = struct{}{}
	}
	return out, nil
}

func (s *Service) ListRoutingRefillRuns(ctx context.Context, filter RoutingRefillRunFilter) ([]*RoutingRefillRun, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("sub2api service is not configured")
	}
	if filter.LocalGroupID < 0 {
		return nil, badRequest("ROUTING_GROUP_ID_INVALID", "invalid group id")
	}
	filter.Status = strings.TrimSpace(filter.Status)
	if filter.Status != "" && !validRoutingRefillRunStatus(filter.Status) {
		return nil, badRequest("ROUTING_REFILL_RUN_STATUS_INVALID", "invalid routing refill run status")
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 500 {
		filter.Limit = 500
	}
	return s.repo.ListRoutingRefillRuns(ctx, filter)
}

func (s *Service) recordRoutingRefillRun(ctx context.Context, in RoutingRefillInput, result *RoutingRefillResult, runErr error) {
	if s == nil || s.repo == nil {
		return
	}
	run := routingRefillRunFromResult(in, result, runErr)
	if run == nil {
		return
	}
	_, _ = s.repo.CreateRoutingRefillRun(ctx, run)
}

func routingRefillRunFromResult(in RoutingRefillInput, result *RoutingRefillResult, runErr error) *RoutingRefillRun {
	if in.LocalGroupID <= 0 && result == nil {
		return nil
	}
	run := &RoutingRefillRun{
		Sub2APIInstanceID: "local",
		LocalGroupID:      in.LocalGroupID,
		Platform:          strings.TrimSpace(in.Platform),
		ModelScope:        strings.TrimSpace(in.ModelScope),
		TriggerType:       normalizedRoutingRefillTriggerType(in.TriggerType),
		DryRun:            in.DryRun,
		Reason:            strings.TrimSpace(in.Reason),
		RequestedBy:       in.RequestedBy,
		RequestSnapshot: map[string]any{
			"local_group_id":                    in.LocalGroupID,
			"platform":                          strings.TrimSpace(in.Platform),
			"model_scope":                       strings.TrimSpace(in.ModelScope),
			"max_rate_multiplier":               in.MaxRateMultiplier,
			"limit":                             in.Limit,
			"dry_run":                           in.DryRun,
			"action_id":                         in.ActionRecommendationID,
			"reason":                            strings.TrimSpace(in.Reason),
			"trigger_type":                      normalizedRoutingRefillTriggerType(in.TriggerType),
			"cooldown_seconds":                  routingRefillCooldown(in.CooldownSeconds).Seconds(),
			"confirm_window_seconds":            routingRefillConfirmWindow(in.ConfirmWindowSecs).Seconds(),
			"failed_candidate_cooldown_seconds": routingRefillFailedCandidateCooldown(in.FailedCandidateCooldownSeconds).Seconds(),
		},
	}
	if result != nil {
		run.LocalGroupID = result.LocalGroupID
		run.Platform = result.Platform
		run.SkippedReason = result.SkippedReason
		run.ResultSnapshot = routingRefillResultSnapshot(result)
		applyRoutingRefillRunAvailability(run, result.AvailabilityBefore, false)
		applyRoutingRefillRunAvailability(run, result.AvailabilityAfter, true)
		if result.Candidate != nil {
			run.SelectedSupplierID = result.Candidate.SupplierID
			run.SelectedSupplierGroupID = result.Candidate.SupplierGroupID
			run.SelectedSupplierKeyID = result.Candidate.SupplierKeyID
			run.SelectedLocalAccountID = result.Candidate.LocalSub2APIAccountID
			run.SelectedEffectiveRateMultiplier = result.Candidate.EffectiveRateMultiplier
		}
	}
	if runErr != nil {
		run.Status = "failed"
		run.ErrorCode = infraerrors.Reason(runErr)
		run.ErrorMessage = runErr.Error()
		return run
	}
	if result != nil && result.SkippedReason != "" {
		run.Status = "skipped"
		return run
	}
	if in.DryRun {
		run.Status = "previewed"
		return run
	}
	run.Status = "succeeded"
	return run
}

func applyRoutingRefillRunAvailability(run *RoutingRefillRun, availability *RoutingGroupAvailability, after bool) {
	if run == nil || availability == nil {
		return
	}
	if run.LocalGroupName == "" {
		run.LocalGroupName = availability.GroupName
	}
	if run.Platform == "" {
		run.Platform = availability.Platform
	}
	if after {
		run.AfterTotalAccounts = availability.TotalAccounts
		run.AfterSchedulableAccounts = availability.SchedulableAccounts
		run.AfterActiveAPIKeyCount = availability.ActiveAPIKeyCount
		return
	}
	run.BeforeTotalAccounts = availability.TotalAccounts
	run.BeforeSchedulableAccounts = availability.SchedulableAccounts
	run.BeforeActiveAPIKeyCount = availability.ActiveAPIKeyCount
}

func routingRefillResultSnapshot(result *RoutingRefillResult) map[string]any {
	if result == nil {
		return map[string]any{}
	}
	return map[string]any{
		"action":              result.Action,
		"dry_run":             result.DryRun,
		"local_group_id":      result.LocalGroupID,
		"platform":            result.Platform,
		"model_scope":         result.ModelScope,
		"availability_before": result.AvailabilityBefore,
		"availability_after":  result.AvailabilityAfter,
		"candidate":           result.Candidate,
		"account":             result.Account,
		"skipped_reason":      result.SkippedReason,
	}
}

func normalizedRoutingRefillTriggerType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "manual"
	}
	return value
}

func validRoutingRefillRunStatus(value string) bool {
	switch value {
	case "previewed", "succeeded", "skipped", "failed":
		return true
	default:
		return false
	}
}

func normalizeRoutingRefillLimit(limit int) int {
	if limit <= 0 {
		return 1000
	}
	if limit > 5000 {
		return 5000
	}
	return limit
}

func routingRefillCooldown(seconds int) time.Duration {
	if seconds < 0 {
		return 0
	}
	if seconds == 0 {
		return defaultRoutingRefillCooldown
	}
	if seconds > int((24 * time.Hour).Seconds()) {
		seconds = int((24 * time.Hour).Seconds())
	}
	return time.Duration(seconds) * time.Second
}

func routingRefillConfirmWindow(seconds int) time.Duration {
	if seconds <= 0 {
		return 0
	}
	if seconds > int((24 * time.Hour).Seconds()) {
		seconds = int((24 * time.Hour).Seconds())
	}
	return time.Duration(seconds) * time.Second
}

func routingRefillFailedCandidateCooldown(seconds int) time.Duration {
	if seconds < 0 {
		return 0
	}
	if seconds == 0 {
		return defaultRoutingRefillFailedCandidateCooldown
	}
	if seconds > int((24 * time.Hour).Seconds()) {
		seconds = int((24 * time.Hour).Seconds())
	}
	return time.Duration(seconds) * time.Second
}

func bestRoutingRefillCandidate(rows []*adminplusdomain.LocalAccountOpsRow, targetGroupID int64, platform string, modelScope string, excludedAccountIDs map[int64]struct{}) *adminplusdomain.LocalAccountOpsRow {
	candidates := make([]*adminplusdomain.LocalAccountOpsRow, 0, len(rows))
	for _, row := range rows {
		if !routingRefillCandidateUsable(row, targetGroupID, platform, modelScope, excludedAccountIDs) {
			continue
		}
		candidates = append(candidates, row)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if left.EffectiveRateMultiplier != right.EffectiveRateMultiplier {
			return left.EffectiveRateMultiplier < right.EffectiveRateMultiplier
		}
		if left.SupplierID != right.SupplierID {
			return left.SupplierID < right.SupplierID
		}
		return left.LocalSub2APIAccountID < right.LocalSub2APIAccountID
	})
	if len(candidates) == 0 {
		return nil
	}
	return candidates[0]
}

func routingRefillHasUsableCandidate(rows []*adminplusdomain.LocalAccountOpsRow, targetGroupID int64, platform string, modelScope string) bool {
	for _, row := range rows {
		if routingRefillCandidateUsable(row, targetGroupID, platform, modelScope, nil) {
			return true
		}
	}
	return false
}

func routingRefillCandidateUsable(row *adminplusdomain.LocalAccountOpsRow, targetGroupID int64, platform string, modelScope string, excludedAccountIDs map[int64]struct{}) bool {
	if row == nil || row.LocalSub2APIAccountID <= 0 {
		return false
	}
	if _, ok := excludedAccountIDs[row.LocalSub2APIAccountID]; ok {
		return false
	}
	if platform != "" && !strings.EqualFold(strings.TrimSpace(row.LocalAccountPlatform), platform) {
		return false
	}
	if containsInt64(row.LocalAccountGroupIDs, targetGroupID) {
		return false
	}
	if row.CandidateStatus == "" || strings.TrimSpace(modelScope) != "" {
		candidateeval.ApplyToLocalAccountOpsRowForModel(row, modelScope)
	}
	return row.CandidateStatus == candidateeval.StatusAvailable
}

func containsInt64(values []int64, needle int64) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func routingRefillCandidateFromRow(row *adminplusdomain.LocalAccountOpsRow) *RoutingRefillCandidate {
	if row == nil {
		return nil
	}
	return &RoutingRefillCandidate{
		LocalSub2APIAccountID:   row.LocalSub2APIAccountID,
		LocalAccountName:        row.LocalAccountName,
		LocalAccountPlatform:    row.LocalAccountPlatform,
		SupplierID:              row.SupplierID,
		SupplierName:            row.SupplierName,
		SupplierGroupID:         row.SupplierGroupID,
		SupplierGroupName:       row.SupplierGroupName,
		SupplierKeyID:           row.SupplierKeyID,
		CandidateStatus:         row.CandidateStatus,
		BlockedReason:           row.BlockedReason,
		CheckSource:             row.CheckSource,
		ModelScope:              row.ModelScope,
		ModelMatchStatus:        row.ModelMatchStatus,
		EffectiveRateMultiplier: row.EffectiveRateMultiplier,
	}
}
