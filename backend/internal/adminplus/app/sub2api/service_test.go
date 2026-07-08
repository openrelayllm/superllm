package sub2api

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type stubUsageRepository struct {
	filter          UsageFilter
	opsFilter       LocalAccountOpsFilter
	actionInput     LocalAccountOpsActionInput
	syncInput       LocalAccountStateSyncInput
	resolveInput    LocalAccountStateResolutionInput
	opsRows         []*adminplusdomain.LocalAccountOpsRow
	applyResult     *adminplusdomain.LocalAccountOpsActionResult
	syncResult      *adminplusdomain.LocalAccountStateSyncResult
	resolveResult   *adminplusdomain.LocalAccountStateResolutionResult
	refillFilter    RoutingRefillRunFilter
	refillFilters   []RoutingRefillRunFilter
	refillRuns      []*RoutingRefillRun
	impactFilter    RoutingImpactFilter
	sensitiveInput  RoutingSensitiveFailureDetailInput
	sensitiveDetail *RoutingSensitiveFailureDetail
	impactKeys      []RoutingImpactedAPIKey
	impactFailures  []RoutingFailureRequest
}

type stubRuntimeRepository struct {
	filter RuntimeFilter
}

type stubRoutingPort struct {
	previewInput   LocalAccountOpsActionInput
	applyInput     LocalAccountOpsActionInput
	resolveInput   LocalAccountStateResolutionInput
	groupID        int64
	platform       string
	accountID      int64
	groupAccount   int64
	groupTarget    int64
	schedulable    bool
	reason         string
	previewCalled  bool
	applyCalled    bool
	resolveCalled  bool
	groupCalled    bool
	accountCalled  bool
	ensureCalled   bool
	scheduleCalled bool
	previewResult  *adminplusdomain.LocalAccountOpsActionResult
	applyResult    *adminplusdomain.LocalAccountOpsActionResult
	resolveResult  *adminplusdomain.LocalAccountStateResolutionResult
	availabilities []*RoutingGroupAvailability
	availabilityN  int
}

func (r *stubUsageRepository) ListLocalUsageLines(_ context.Context, filter UsageFilter) ([]*adminplusdomain.LocalUsageLine, error) {
	r.filter = filter
	return []*adminplusdomain.LocalUsageLine{}, nil
}

func (r *stubUsageRepository) ListLocalUsageSummaries(_ context.Context, filter UsageFilter) ([]*adminplusdomain.LocalUsageSummary, error) {
	r.filter = filter
	return []*adminplusdomain.LocalUsageSummary{}, nil
}

func (r *stubUsageRepository) ListLocalAccountUsageSummaries(_ context.Context, filter UsageFilter) ([]*adminplusdomain.LocalAccountUsageSummary, error) {
	r.filter = filter
	return []*adminplusdomain.LocalAccountUsageSummary{}, nil
}

func (r *stubUsageRepository) ListLocalGroups(_ context.Context, _ int) ([]*LocalSub2APIGroup, error) {
	return []*LocalSub2APIGroup{}, nil
}

func (r *stubUsageRepository) ListLocalAccountOps(_ context.Context, filter LocalAccountOpsFilter) ([]*adminplusdomain.LocalAccountOpsRow, error) {
	r.opsFilter = filter
	if r.opsRows != nil {
		return r.opsRows, nil
	}
	return []*adminplusdomain.LocalAccountOpsRow{}, nil
}

func (r *stubUsageRepository) ListRoutingImpactAPIKeys(_ context.Context, filter RoutingImpactFilter) ([]RoutingImpactedAPIKey, error) {
	r.impactFilter = filter
	return r.impactKeys, nil
}

func (r *stubUsageRepository) ListRoutingImpactFailureRequests(_ context.Context, filter RoutingImpactFilter) ([]RoutingFailureRequest, error) {
	r.impactFilter = filter
	return r.impactFailures, nil
}

func (r *stubUsageRepository) GetRoutingFailureSensitiveDetail(_ context.Context, input RoutingSensitiveFailureDetailInput) (*RoutingSensitiveFailureDetail, error) {
	r.sensitiveInput = input
	if r.sensitiveDetail != nil {
		return r.sensitiveDetail, nil
	}
	return &RoutingSensitiveFailureDetail{
		ID:           input.FailureID,
		LocalGroupID: input.LocalGroupID,
		Available:    true,
		Fields: []RoutingSensitiveFailureField{{
			Name:      "error_body",
			Available: true,
			Value:     `{"message":"redacted"}`,
			Redacted:  true,
		}},
	}, nil
}

func (r *stubUsageRepository) CreateRoutingRefillRun(_ context.Context, run *RoutingRefillRun) (*RoutingRefillRun, error) {
	if run.ID == 0 {
		run.ID = int64(len(r.refillRuns) + 1)
	}
	r.refillRuns = append(r.refillRuns, run)
	return run, nil
}

func (r *stubUsageRepository) ListRoutingRefillRuns(_ context.Context, filter RoutingRefillRunFilter) ([]*RoutingRefillRun, error) {
	r.refillFilter = filter
	r.refillFilters = append(r.refillFilters, filter)
	items := make([]*RoutingRefillRun, 0, len(r.refillRuns))
	for _, run := range r.refillRuns {
		if run == nil {
			continue
		}
		if filter.LocalGroupID > 0 && run.LocalGroupID != filter.LocalGroupID {
			continue
		}
		if filter.Status != "" && run.Status != filter.Status {
			continue
		}
		items = append(items, run)
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.After(right.CreatedAt)
		}
		return left.ID > right.ID
	})
	if filter.Limit > 0 && len(items) > filter.Limit {
		items = items[:filter.Limit]
	}
	return items, nil
}

func (r *stubUsageRepository) GetGroupAvailability(_ context.Context, groupID int64, platform string) (*RoutingGroupAvailability, error) {
	return &RoutingGroupAvailability{GroupID: groupID, Platform: platform, SchedulableAccounts: 1}, nil
}

func (r *stubUsageRepository) GetAccount(_ context.Context, accountID int64) (*Sub2APIAccountSnapshot, error) {
	return &Sub2APIAccountSnapshot{AccountID: accountID, Schedulable: true}, nil
}

func (r *stubUsageRepository) EnsureAccountInGroup(_ context.Context, accountID int64, groupID int64) (*Sub2APIAccountSnapshot, error) {
	return &Sub2APIAccountSnapshot{AccountID: accountID, GroupIDs: []int64{groupID}}, nil
}

func (r *stubUsageRepository) SetAccountSchedulable(_ context.Context, accountID int64, schedulable bool, _ string) (*Sub2APIAccountSnapshot, error) {
	return &Sub2APIAccountSnapshot{AccountID: accountID, Schedulable: schedulable}, nil
}

func (r *stubUsageRepository) PreviewLocalAccountOpsAction(_ context.Context, input LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	r.actionInput = input
	return &adminplusdomain.LocalAccountOpsActionResult{Action: input.Action, DryRun: true, AccountIDs: input.AccountIDs}, nil
}

func (r *stubUsageRepository) ApplyLocalAccountOpsAction(_ context.Context, input LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	r.actionInput = input
	if r.applyResult != nil {
		return r.applyResult, nil
	}
	return &adminplusdomain.LocalAccountOpsActionResult{Action: input.Action, AccountIDs: input.AccountIDs}, nil
}

func (r *stubUsageRepository) SyncLocalAccountState(_ context.Context, input LocalAccountStateSyncInput) (*adminplusdomain.LocalAccountStateSyncResult, error) {
	r.syncInput = input
	if r.syncResult != nil {
		return r.syncResult, nil
	}
	return &adminplusdomain.LocalAccountStateSyncResult{CheckedAccounts: int64(len(input.AccountIDs))}, nil
}

func (r *stubUsageRepository) ResolveLocalAccountState(_ context.Context, input LocalAccountStateResolutionInput) (*adminplusdomain.LocalAccountStateResolutionResult, error) {
	r.resolveInput = input
	if r.resolveResult != nil {
		return r.resolveResult, nil
	}
	return &adminplusdomain.LocalAccountStateResolutionResult{
		Action:           input.Action,
		AccountIDs:       input.AccountIDs,
		ResolvedAccounts: int64(len(input.AccountIDs)),
	}, nil
}

func (r *stubRuntimeRepository) ListAccountRuntime(_ context.Context, filter RuntimeFilter) ([]*adminplusdomain.LocalAccountRuntime, error) {
	r.filter = filter
	return []*adminplusdomain.LocalAccountRuntime{}, nil
}

func (r *stubRoutingPort) GetGroupAvailability(_ context.Context, groupID int64, platform string) (*RoutingGroupAvailability, error) {
	r.groupCalled = true
	r.groupID = groupID
	r.platform = platform
	if r.availabilityN < len(r.availabilities) {
		item := *r.availabilities[r.availabilityN]
		r.availabilityN++
		item.GroupID = groupID
		item.Platform = platform
		return &item, nil
	}
	return &RoutingGroupAvailability{GroupID: groupID, Platform: platform, SchedulableAccounts: 1}, nil
}

func (r *stubRoutingPort) GetAccount(_ context.Context, accountID int64) (*Sub2APIAccountSnapshot, error) {
	r.accountCalled = true
	r.accountID = accountID
	return &Sub2APIAccountSnapshot{AccountID: accountID, Schedulable: true}, nil
}

func (r *stubRoutingPort) EnsureAccountInGroup(_ context.Context, accountID int64, groupID int64) (*Sub2APIAccountSnapshot, error) {
	r.ensureCalled = true
	r.groupAccount = accountID
	r.groupTarget = groupID
	return &Sub2APIAccountSnapshot{AccountID: accountID, GroupIDs: []int64{groupID}}, nil
}

func (r *stubRoutingPort) SetAccountSchedulable(_ context.Context, accountID int64, schedulable bool, reason string) (*Sub2APIAccountSnapshot, error) {
	r.scheduleCalled = true
	r.accountID = accountID
	r.schedulable = schedulable
	r.reason = reason
	return &Sub2APIAccountSnapshot{AccountID: accountID, Schedulable: schedulable}, nil
}

func (r *stubRoutingPort) PreviewLocalAccountOpsAction(_ context.Context, input LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	r.previewCalled = true
	r.previewInput = input
	if r.previewResult != nil {
		return r.previewResult, nil
	}
	return &adminplusdomain.LocalAccountOpsActionResult{Action: input.Action, DryRun: true, AccountIDs: input.AccountIDs}, nil
}

func (r *stubRoutingPort) ApplyLocalAccountOpsAction(_ context.Context, input LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	r.applyCalled = true
	r.applyInput = input
	if r.applyResult != nil {
		return r.applyResult, nil
	}
	return &adminplusdomain.LocalAccountOpsActionResult{Action: input.Action, AccountIDs: input.AccountIDs}, nil
}

func (r *stubRoutingPort) ResolveLocalAccountState(_ context.Context, input LocalAccountStateResolutionInput) (*adminplusdomain.LocalAccountStateResolutionResult, error) {
	r.resolveCalled = true
	r.resolveInput = input
	if r.resolveResult != nil {
		return r.resolveResult, nil
	}
	return &adminplusdomain.LocalAccountStateResolutionResult{
		Action:           input.Action,
		AccountIDs:       input.AccountIDs,
		ResolvedAccounts: int64(len(input.AccountIDs)),
	}, nil
}

type stubBizlogWriter struct {
	inputs []*service.OpsInsertSystemLogInput
}

func (w *stubBizlogWriter) BatchInsertSystemLogs(_ context.Context, inputs []*service.OpsInsertSystemLogInput) (int64, error) {
	w.inputs = append(w.inputs, inputs...)
	return int64(len(inputs)), nil
}

func TestServiceListLocalUsageLinesDefaultsToLast24Hours(t *testing.T) {
	repo := &stubUsageRepository{}
	svc := NewService(repo, &stubRuntimeRepository{})
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	_, err := svc.ListLocalUsageLines(context.Background(), UsageFilter{})

	require.NoError(t, err)
	require.Equal(t, now.Add(-24*time.Hour), repo.filter.From)
	require.Equal(t, now, repo.filter.To)
	require.Equal(t, 200, repo.filter.Limit)
}

func TestServiceRejectsTooLargeUsageRange(t *testing.T) {
	svc := NewService(&stubUsageRepository{}, &stubRuntimeRepository{})
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(32 * 24 * time.Hour)

	_, err := svc.ListLocalUsageSummaries(context.Background(), UsageFilter{From: from, To: to})

	require.Error(t, err)
	require.Contains(t, err.Error(), "LOCAL_USAGE_TIME_RANGE_TOO_LARGE")
}

func TestServiceListAccountRuntimeRejectsNegativeAccountID(t *testing.T) {
	svc := NewService(&stubUsageRepository{}, &stubRuntimeRepository{})

	_, err := svc.ListAccountRuntime(context.Background(), RuntimeFilter{AccountID: -1})

	require.Error(t, err)
	require.Contains(t, err.Error(), "ACCOUNT_RUNTIME_ACCOUNT_ID_INVALID")
}

func TestServiceListAccountRuntimePassesFilter(t *testing.T) {
	repo := &stubRuntimeRepository{}
	svc := NewService(&stubUsageRepository{}, repo)

	_, err := svc.ListAccountRuntime(context.Background(), RuntimeFilter{AccountID: 7, Query: "prod", Limit: 20})

	require.NoError(t, err)
	require.Equal(t, int64(7), repo.filter.AccountID)
	require.Equal(t, "prod", repo.filter.Query)
	require.Equal(t, 20, repo.filter.Limit)
}

func TestServiceListLocalAccountOpsNormalizesFilter(t *testing.T) {
	repo := &stubUsageRepository{}
	svc := NewService(repo, &stubRuntimeRepository{})
	schedulable := true

	_, err := svc.ListLocalAccountOps(context.Background(), LocalAccountOpsFilter{
		Query:              "  lime  ",
		SupplierID:         7,
		LocalGroupID:       9,
		SupplierGroupID:    11,
		MaxRateMultiplier:  0.2,
		BalanceStatus:      " Usable ",
		ChannelCheckStatus: " AVAILABLE ",
		Schedulable:        &schedulable,
	})

	require.NoError(t, err)
	require.Equal(t, "lime", repo.opsFilter.Query)
	require.Equal(t, int64(7), repo.opsFilter.SupplierID)
	require.Equal(t, int64(9), repo.opsFilter.LocalGroupID)
	require.Equal(t, int64(11), repo.opsFilter.SupplierGroupID)
	require.Equal(t, 0.2, repo.opsFilter.MaxRateMultiplier)
	require.Equal(t, "usable", repo.opsFilter.BalanceStatus)
	require.Equal(t, "available", repo.opsFilter.ChannelCheckStatus)
	require.Equal(t, &schedulable, repo.opsFilter.Schedulable)
	require.Equal(t, 200, repo.opsFilter.Limit)
}

func TestServiceListLocalAccountOpsAppliesCandidateEvaluation(t *testing.T) {
	repo := &stubUsageRepository{
		opsRows: []*adminplusdomain.LocalAccountOpsRow{{
			LocalSub2APIAccountID:        42,
			LocalAccountStatus:           "active",
			LocalAccountSchedulable:      true,
			SupplierID:                   7,
			SupplierRuntimeStatus:        "active",
			SupplierAccountID:            77,
			SupplierAccountRuntimeStatus: "active",
			SupplierGroupID:              1001,
			SupplierGroupStatus:          "active",
			SupplierKeyID:                9001,
			SupplierKeyStatus:            "bound",
			HasUsableBalance:             false,
			BalanceStatus:                "insufficient",
			ChannelCheckStatus:           "remote_unavailable",
			DriftStatus:                  "synced",
			EffectiveRateMultiplier:      0.2,
		}},
	}
	svc := NewService(repo, &stubRuntimeRepository{})

	rows, err := svc.ListLocalAccountOps(context.Background(), LocalAccountOpsFilter{})

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "balance_blocked", rows[0].CandidateStatus)
	require.Equal(t, "recharge_required", rows[0].BlockedReason)
	require.Equal(t, "balance", rows[0].CheckSource)
	require.Equal(t, "unknown", rows[0].KeyCapacityStatus)
}

func TestServiceListLocalAccountOpsRejectsInvalidIDs(t *testing.T) {
	svc := NewService(&stubUsageRepository{}, &stubRuntimeRepository{})

	_, err := svc.ListLocalAccountOps(context.Background(), LocalAccountOpsFilter{SupplierID: -1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "LOCAL_ACCOUNT_OPS_SUPPLIER_ID_INVALID")

	_, err = svc.ListLocalAccountOps(context.Background(), LocalAccountOpsFilter{LocalGroupID: -1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "LOCAL_ACCOUNT_OPS_GROUP_ID_INVALID")

	_, err = svc.ListLocalAccountOps(context.Background(), LocalAccountOpsFilter{SupplierGroupID: -1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "LOCAL_ACCOUNT_OPS_SUPPLIER_GROUP_ID_INVALID")

	_, err = svc.ListLocalAccountOps(context.Background(), LocalAccountOpsFilter{MaxRateMultiplier: -0.1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "LOCAL_ACCOUNT_OPS_MAX_RATE_INVALID")

	_, err = svc.ListLocalAccountOps(context.Background(), LocalAccountOpsFilter{BalanceStatus: "bad"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "LOCAL_ACCOUNT_OPS_BALANCE_STATUS_INVALID")

	_, err = svc.ListLocalAccountOps(context.Background(), LocalAccountOpsFilter{ChannelCheckStatus: "bad"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "LOCAL_ACCOUNT_OPS_CHANNEL_STATUS_INVALID")
}

func TestServiceRoutingPortNormalizesSemanticInputs(t *testing.T) {
	routing := &stubRoutingPort{}
	svc := NewService(nil, &stubRuntimeRepository{}).WithRoutingPort(routing)

	availability, err := svc.GetGroupAvailability(context.Background(), 1001, " openai ")
	require.NoError(t, err)
	require.Equal(t, int64(1001), availability.GroupID)
	require.True(t, routing.groupCalled)
	require.Equal(t, "openai", routing.platform)

	account, err := svc.EnsureAccountInGroup(context.Background(), 42, 1001)
	require.NoError(t, err)
	require.Equal(t, []int64{1001}, account.GroupIDs)
	require.True(t, routing.ensureCalled)
	require.Equal(t, int64(42), routing.groupAccount)
	require.Equal(t, int64(1001), routing.groupTarget)

	account, err = svc.SetAccountSchedulable(context.Background(), 42, false, "  capacity_watch  ")
	require.NoError(t, err)
	require.False(t, account.Schedulable)
	require.True(t, routing.scheduleCalled)
	require.Equal(t, "capacity_watch", routing.reason)
}

func TestServicePreviewLocalAccountOpsActionNormalizesInput(t *testing.T) {
	repo := &stubUsageRepository{}
	svc := NewService(repo, &stubRuntimeRepository{})
	schedulable := false

	result, err := svc.PreviewLocalAccountOpsAction(context.Background(), LocalAccountOpsActionInput{
		Action:      adminplusdomain.LocalAccountOpsActionSetSchedulable,
		AccountIDs:  []int64{7, 7, -1, 8},
		Schedulable: &schedulable,
		RequestedBy: 99,
	})

	require.NoError(t, err)
	require.True(t, result.DryRun)
	require.Equal(t, []int64{7, 8}, repo.actionInput.AccountIDs)
	require.Equal(t, &schedulable, repo.actionInput.Schedulable)
	require.Equal(t, int64(99), repo.actionInput.RequestedBy)
}

func TestServiceApplyLocalAccountOpsActionRequiresGroupIDs(t *testing.T) {
	svc := NewService(&stubUsageRepository{}, &stubRuntimeRepository{})

	_, err := svc.ApplyLocalAccountOpsAction(context.Background(), LocalAccountOpsActionInput{
		Action:     adminplusdomain.LocalAccountOpsActionAddToGroups,
		AccountIDs: []int64{7},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "LOCAL_ACCOUNT_OPS_GROUP_IDS_REQUIRED")
}

func TestServiceApplyLocalAccountOpsActionUsesRoutingPort(t *testing.T) {
	routing := &stubRoutingPort{
		applyResult: &adminplusdomain.LocalAccountOpsActionResult{
			Action:          adminplusdomain.LocalAccountOpsActionSetSchedulable,
			AccountIDs:      []int64{7, 8},
			UpdatedAccounts: 2,
		},
	}
	svc := NewService(nil, &stubRuntimeRepository{}).WithRoutingPort(routing)
	schedulable := true

	result, err := svc.ApplyLocalAccountOpsAction(context.Background(), LocalAccountOpsActionInput{
		Action:      adminplusdomain.LocalAccountOpsActionSetSchedulable,
		AccountIDs:  []int64{7, 7, -1, 8},
		Schedulable: &schedulable,
		RequestedBy: 99,
		Reason:      "  capacity_watch  ",
	})

	require.NoError(t, err)
	require.True(t, routing.applyCalled)
	require.Equal(t, []int64{7, 8}, routing.applyInput.AccountIDs)
	require.Equal(t, &schedulable, routing.applyInput.Schedulable)
	require.Equal(t, int64(99), routing.applyInput.RequestedBy)
	require.Equal(t, "capacity_watch", routing.applyInput.Reason)
	require.False(t, routing.applyInput.DryRun)
	require.Equal(t, int64(2), result.UpdatedAccounts)
}

func TestServiceApplyLocalAccountOpsActionRecordsBusinessLog(t *testing.T) {
	repo := &stubUsageRepository{
		applyResult: &adminplusdomain.LocalAccountOpsActionResult{
			Action:          adminplusdomain.LocalAccountOpsActionSetSchedulable,
			AccountIDs:      []int64{7},
			UpdatedAccounts: 1,
		},
	}
	writer := &stubBizlogWriter{}
	svc := NewService(repo, &stubRuntimeRepository{}).WithDiagnostics(bizlogs.NewRecorder(writer))
	schedulable := true

	result, err := svc.ApplyLocalAccountOpsAction(context.Background(), LocalAccountOpsActionInput{
		Action:      adminplusdomain.LocalAccountOpsActionSetSchedulable,
		AccountIDs:  []int64{7},
		Schedulable: &schedulable,
		RequestedBy: 99,
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), result.UpdatedAccounts)
	require.Len(t, writer.inputs, 1)
	require.Equal(t, "admin_plus.sub2api", writer.inputs[0].Component)
	require.Equal(t, "info", writer.inputs[0].Level)
	require.Contains(t, writer.inputs[0].Message, "local account operation applied")

	var extra map[string]any
	require.NoError(t, json.Unmarshal([]byte(writer.inputs[0].ExtraJSON), &extra))
	require.Equal(t, "set_schedulable", extra["action"])
	require.Equal(t, "succeeded", extra["outcome"])
	require.Equal(t, float64(99), extra["requested_by"])
	require.Equal(t, true, extra["schedulable"])
	require.Equal(t, float64(1), extra["updated_accounts"])
}

func TestServiceApplyLocalAccountOpsActionRecordsBlockedBusinessLog(t *testing.T) {
	repo := &stubUsageRepository{
		applyResult: &adminplusdomain.LocalAccountOpsActionResult{
			Action:        adminplusdomain.LocalAccountOpsActionRemoveFromGroups,
			AccountIDs:    []int64{7},
			GroupIDs:      []int64{1001},
			Blocked:       true,
			BlockedReason: "LOCAL_GROUP_SCHEDULABLE_POOL_WOULD_BE_EMPTY",
			GroupImpacts: []adminplusdomain.LocalAccountOpsGroupImpact{{
				GroupID:                   1001,
				GroupName:                 "Lime",
				ActiveAPIKeyCount:         2,
				BeforeSchedulableAccounts: 1,
				AfterSchedulableAccounts:  0,
				WouldEmptySchedulablePool: true,
			}},
		},
	}
	writer := &stubBizlogWriter{}
	svc := NewService(repo, &stubRuntimeRepository{}).WithDiagnostics(bizlogs.NewRecorder(writer))

	result, err := svc.ApplyLocalAccountOpsAction(context.Background(), LocalAccountOpsActionInput{
		Action:     adminplusdomain.LocalAccountOpsActionRemoveFromGroups,
		AccountIDs: []int64{7},
		GroupIDs:   []int64{1001},
	})

	require.NoError(t, err)
	require.True(t, result.Blocked)
	require.Len(t, writer.inputs, 1)
	require.Equal(t, "warn", writer.inputs[0].Level)
	require.Contains(t, writer.inputs[0].Message, "blocked")

	var extra map[string]any
	require.NoError(t, json.Unmarshal([]byte(writer.inputs[0].ExtraJSON), &extra))
	require.Equal(t, "remove_from_groups", extra["action"])
	require.Equal(t, "blocked", extra["outcome"])
	require.Equal(t, "LOCAL_GROUP_SCHEDULABLE_POOL_WOULD_BE_EMPTY", extra["reason"])
	require.Equal(t, true, extra["blocked"])
	require.NotEmpty(t, extra["group_impacts"])
}

func TestServiceResolveLocalAccountStateNormalizesInput(t *testing.T) {
	repo := &stubUsageRepository{}
	svc := NewService(repo, &stubRuntimeRepository{})

	result, err := svc.ResolveLocalAccountState(context.Background(), LocalAccountStateResolutionInput{
		Action:      adminplusdomain.LocalAccountStateResolutionAcceptObserved,
		AccountIDs:  []int64{7, 7, -1, 8},
		RequestedBy: 99,
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.LocalAccountStateResolutionAcceptObserved, result.Action)
	require.Equal(t, []int64{7, 8}, repo.resolveInput.AccountIDs)
	require.Equal(t, int64(99), repo.resolveInput.RequestedBy)
}

func TestServiceResolveLocalAccountStateRejectsInvalidInput(t *testing.T) {
	svc := NewService(&stubUsageRepository{}, &stubRuntimeRepository{})

	_, err := svc.ResolveLocalAccountState(context.Background(), LocalAccountStateResolutionInput{
		Action:     adminplusdomain.LocalAccountStateResolutionAction("bad"),
		AccountIDs: []int64{7},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "LOCAL_ACCOUNT_STATE_RESOLUTION_ACTION_INVALID")

	_, err = svc.ResolveLocalAccountState(context.Background(), LocalAccountStateResolutionInput{
		Action: adminplusdomain.LocalAccountStateResolutionAcceptObserved,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "LOCAL_ACCOUNT_STATE_RESOLUTION_ACCOUNT_IDS_REQUIRED")
}

func TestServiceResolveLocalAccountStateRecordsBusinessLog(t *testing.T) {
	repo := &stubUsageRepository{
		resolveResult: &adminplusdomain.LocalAccountStateResolutionResult{
			Action:           adminplusdomain.LocalAccountStateResolutionRestoreAccepted,
			AccountIDs:       []int64{7},
			ResolvedAccounts: 1,
			RestoredAccounts: 1,
		},
	}
	writer := &stubBizlogWriter{}
	svc := NewService(repo, &stubRuntimeRepository{}).WithDiagnostics(bizlogs.NewRecorder(writer))

	_, err := svc.ResolveLocalAccountState(context.Background(), LocalAccountStateResolutionInput{
		Action:      adminplusdomain.LocalAccountStateResolutionRestoreAccepted,
		AccountIDs:  []int64{7},
		RequestedBy: 99,
	})

	require.NoError(t, err)
	require.Len(t, writer.inputs, 1)
	require.Equal(t, "admin_plus.sub2api", writer.inputs[0].Component)
	require.Equal(t, "info", writer.inputs[0].Level)

	var extra map[string]any
	require.NoError(t, json.Unmarshal([]byte(writer.inputs[0].ExtraJSON), &extra))
	require.Equal(t, "restore_accepted", extra["action"])
	require.Equal(t, "succeeded", extra["outcome"])
	require.Equal(t, float64(1), extra["resolved_accounts"])
	require.Equal(t, float64(1), extra["restored_accounts"])
	require.Equal(t, float64(99), extra["requested_by"])
}

func TestServiceGetRoutingFailureSensitiveDetailRequiresReason(t *testing.T) {
	svc := NewService(&stubUsageRepository{}, &stubRuntimeRepository{})

	_, err := svc.GetRoutingFailureSensitiveDetail(context.Background(), RoutingSensitiveFailureDetailInput{
		FailureID:    123,
		LocalGroupID: 1001,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "ROUTING_FAILURE_SENSITIVE_DETAIL_REASON_REQUIRED")
}

func TestServiceGetRoutingFailureSensitiveDetailNormalizesFieldsAndRecordsAudit(t *testing.T) {
	repo := &stubUsageRepository{}
	writer := &stubBizlogWriter{}
	svc := NewService(repo, &stubRuntimeRepository{}).WithDiagnostics(bizlogs.NewRecorder(writer))

	result, err := svc.GetRoutingFailureSensitiveDetail(context.Background(), RoutingSensitiveFailureDetailInput{
		FailureID:    123,
		LocalGroupID: 1001,
		Reason:       "investigate refill impact",
		Fields:       []string{"headers", "error_body", "error_body"},
		RequestedBy:  99,
	})

	require.NoError(t, err)
	require.Equal(t, int64(123), result.ID)
	require.Equal(t, []string{"request_headers", "error_body"}, repo.sensitiveInput.Fields)
	require.Equal(t, "investigate refill impact", repo.sensitiveInput.Reason)
	require.Len(t, writer.inputs, 1)
	require.Equal(t, "admin_plus.sub2api", writer.inputs[0].Component)
	require.Equal(t, "warn", writer.inputs[0].Level)
	require.NotContains(t, writer.inputs[0].ExtraJSON, "redacted")

	var extra map[string]any
	require.NoError(t, json.Unmarshal([]byte(writer.inputs[0].ExtraJSON), &extra))
	require.Equal(t, "routing_failure_sensitive_detail", extra["action"])
	require.Equal(t, "succeeded", extra["outcome"])
	require.Equal(t, "investigate refill impact", extra["reason"])
	require.Equal(t, float64(123), extra["failure_id"])
	require.Equal(t, float64(1001), extra["local_group_id"])
	require.Equal(t, float64(99), extra["requested_by"])
}
