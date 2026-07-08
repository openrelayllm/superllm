package sub2api

import (
	"context"
	"testing"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func TestServiceRefillLocalGroupDryRunSelectsLowestAvailableCandidate(t *testing.T) {
	balanceBlocked := refillOpsRow(43, 8, 0.1, "balance_blocked", nil)
	balanceBlocked.HasUsableBalance = false
	balanceBlocked.BalanceStatus = "insufficient"
	repo := &stubUsageRepository{
		opsRows: []*adminplusdomain.LocalAccountOpsRow{
			refillOpsRow(42, 7, 0.3, "available", nil),
			balanceBlocked,
			refillOpsRow(44, 9, 0.2, "available", []int64{1001}),
			refillOpsRow(45, 10, 0.12, "available", nil),
		},
	}
	routing := &stubRoutingPort{
		availabilities: []*RoutingGroupAvailability{{
			SchedulableAccounts: 0,
			ActiveAPIKeyCount:   2,
		}},
	}
	svc := NewService(repo, &stubRuntimeRepository{}).WithRoutingPort(routing)

	result, err := svc.RefillLocalGroup(context.Background(), RoutingRefillInput{
		LocalGroupID:      1001,
		Platform:          "openai",
		MaxRateMultiplier: 0.5,
		DryRun:            true,
	})

	require.NoError(t, err)
	require.True(t, result.DryRun)
	require.Empty(t, result.SkippedReason)
	require.NotNil(t, result.Candidate)
	require.Equal(t, int64(45), result.Candidate.LocalSub2APIAccountID)
	require.Equal(t, 0.12, result.Candidate.EffectiveRateMultiplier)
	require.False(t, routing.ensureCalled)
	require.Equal(t, 1000, repo.opsFilter.Limit)
	require.Equal(t, 0.5, repo.opsFilter.MaxRateMultiplier)
	require.Len(t, repo.refillRuns, 1)
	require.Equal(t, "previewed", repo.refillRuns[0].Status)
	require.True(t, repo.refillRuns[0].DryRun)
	require.Equal(t, int64(45), repo.refillRuns[0].SelectedLocalAccountID)
}

func TestServiceRefillLocalGroupDryRunSelectsLowestModelMatchedCandidate(t *testing.T) {
	lowOpenAI := refillOpsRow(45, 10, 0.12, "available", nil)
	lowOpenAI.SupplierGroupModelFamily = "OpenAI"
	lowOpenAI.SupplierGroupModelSpec = "GPT-4o"
	claude := refillOpsRow(46, 11, 0.2, "available", nil)
	claude.SupplierGroupModelFamily = "Claude"
	claude.SupplierGroupModelSpec = "Sonnet"
	repo := &stubUsageRepository{
		opsRows: []*adminplusdomain.LocalAccountOpsRow{lowOpenAI, claude},
	}
	routing := &stubRoutingPort{
		availabilities: []*RoutingGroupAvailability{{
			SchedulableAccounts: 0,
			ActiveAPIKeyCount:   2,
		}},
	}
	svc := NewService(repo, &stubRuntimeRepository{}).WithRoutingPort(routing)

	result, err := svc.RefillLocalGroup(context.Background(), RoutingRefillInput{
		LocalGroupID: 1001,
		Platform:     "openai",
		ModelScope:   "claude-3-5-sonnet",
		DryRun:       true,
	})

	require.NoError(t, err)
	require.Empty(t, result.SkippedReason)
	require.Equal(t, "claude-3-5-sonnet", result.ModelScope)
	require.NotNil(t, result.Candidate)
	require.Equal(t, int64(46), result.Candidate.LocalSub2APIAccountID)
	require.Equal(t, "Claude / Sonnet", result.Candidate.ModelScope)
	require.Equal(t, "supported", result.Candidate.ModelMatchStatus)
	require.Equal(t, "claude-3-5-sonnet", repo.opsFilter.ModelScope)
	require.Len(t, repo.refillRuns, 1)
	require.Equal(t, "claude-3-5-sonnet", repo.refillRuns[0].ModelScope)
}

func TestServiceRefillLocalGroupAppliesCandidateAfterSecondAvailabilityCheck(t *testing.T) {
	repo := &stubUsageRepository{
		opsRows: []*adminplusdomain.LocalAccountOpsRow{
			refillOpsRow(45, 10, 0.12, "available", nil),
		},
	}
	routing := &stubRoutingPort{
		availabilities: []*RoutingGroupAvailability{
			{SchedulableAccounts: 0, ActiveAPIKeyCount: 2},
			{SchedulableAccounts: 0, ActiveAPIKeyCount: 2},
			{SchedulableAccounts: 1, ActiveAPIKeyCount: 2},
		},
	}
	svc := NewService(repo, &stubRuntimeRepository{}).WithRoutingPort(routing)

	result, err := svc.RefillLocalGroup(context.Background(), RoutingRefillInput{LocalGroupID: 1001, Platform: "openai"})

	require.NoError(t, err)
	require.Empty(t, result.SkippedReason)
	require.True(t, routing.ensureCalled)
	require.Equal(t, int64(45), routing.groupAccount)
	require.Equal(t, int64(1001), routing.groupTarget)
	require.NotNil(t, result.AvailabilityAfter)
	require.Equal(t, int64(1), result.AvailabilityAfter.SchedulableAccounts)
	require.Len(t, repo.refillRuns, 1)
	require.Equal(t, "succeeded", repo.refillRuns[0].Status)
	require.Equal(t, int64(45), repo.refillRuns[0].SelectedLocalAccountID)
	require.Equal(t, int64(1001), repo.refillRuns[0].LocalGroupID)
	require.Equal(t, int64(1), repo.refillRuns[0].AfterSchedulableAccounts)
}

func TestServiceRefillLocalGroupSkipsWhenLockBusy(t *testing.T) {
	base := &stubUsageRepository{
		opsRows: []*adminplusdomain.LocalAccountOpsRow{
			refillOpsRow(45, 10, 0.12, "available", nil),
		},
	}
	repo := &stubRoutingRefillLockRepository{stubUsageRepository: base}
	routing := &stubRoutingPort{
		availabilities: []*RoutingGroupAvailability{{SchedulableAccounts: 0, ActiveAPIKeyCount: 2}},
	}
	svc := NewService(repo, &stubRuntimeRepository{}).WithRoutingPort(routing)

	result, err := svc.RefillLocalGroup(context.Background(), RoutingRefillInput{LocalGroupID: 1001, Platform: "openai"})

	require.NoError(t, err)
	require.Equal(t, "refill_locked", result.SkippedReason)
	require.True(t, repo.lockRequested)
	require.Equal(t, int64(1001), repo.lockGroupID)
	require.False(t, repo.unlockCalled)
	require.False(t, routing.groupCalled)
	require.False(t, routing.ensureCalled)
	require.Len(t, base.refillRuns, 1)
	require.Equal(t, "skipped", base.refillRuns[0].Status)
	require.Equal(t, "refill_locked", base.refillRuns[0].SkippedReason)
}

func TestServiceRefillLocalGroupSkipsWhenRecentSuccessInCooldown(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	repo := &stubUsageRepository{
		opsRows: []*adminplusdomain.LocalAccountOpsRow{
			refillOpsRow(45, 10, 0.12, "available", nil),
		},
		refillRuns: []*RoutingRefillRun{{
			ID:           99,
			LocalGroupID: 1001,
			Status:       "succeeded",
			CreatedAt:    now.Add(-time.Minute),
		}},
	}
	routing := &stubRoutingPort{
		availabilities: []*RoutingGroupAvailability{{SchedulableAccounts: 0, ActiveAPIKeyCount: 2}},
	}
	svc := NewService(repo, &stubRuntimeRepository{}).WithRoutingPort(routing)
	svc.now = func() time.Time { return now }

	result, err := svc.RefillLocalGroup(context.Background(), RoutingRefillInput{LocalGroupID: 1001, Platform: "openai"})

	require.NoError(t, err)
	require.Equal(t, "refill_cooldown", result.SkippedReason)
	require.False(t, routing.groupCalled)
	require.False(t, routing.ensureCalled)
	require.Zero(t, repo.opsFilter.Limit)
	requireRoutingRefillFilter(t, repo, RoutingRefillRunFilter{LocalGroupID: 1001, Status: "succeeded", Limit: 1})
	require.Len(t, repo.refillRuns, 2)
	require.Equal(t, "skipped", repo.refillRuns[1].Status)
	require.Equal(t, "refill_cooldown", repo.refillRuns[1].SkippedReason)
}

func TestServiceRefillLocalGroupUsesConfiguredCooldown(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	repo := &stubUsageRepository{
		opsRows: []*adminplusdomain.LocalAccountOpsRow{
			refillOpsRow(45, 10, 0.12, "available", nil),
		},
		refillRuns: []*RoutingRefillRun{{
			ID:           99,
			LocalGroupID: 1001,
			Status:       "succeeded",
			CreatedAt:    now.Add(-time.Minute),
		}},
	}
	routing := &stubRoutingPort{
		availabilities: []*RoutingGroupAvailability{
			{SchedulableAccounts: 0, ActiveAPIKeyCount: 2},
			{SchedulableAccounts: 0, ActiveAPIKeyCount: 2},
			{SchedulableAccounts: 1, ActiveAPIKeyCount: 2},
		},
	}
	svc := NewService(repo, &stubRuntimeRepository{}).WithRoutingPort(routing)
	svc.now = func() time.Time { return now }

	result, err := svc.RefillLocalGroup(context.Background(), RoutingRefillInput{
		LocalGroupID:    1001,
		Platform:        "openai",
		CooldownSeconds: 30,
	})

	require.NoError(t, err)
	require.Empty(t, result.SkippedReason)
	require.True(t, routing.ensureCalled)
	requireRoutingRefillFilter(t, repo, RoutingRefillRunFilter{LocalGroupID: 1001, Status: "succeeded", Limit: 1})
	require.Len(t, repo.refillRuns, 2)
	require.Equal(t, "succeeded", repo.refillRuns[1].Status)
	require.Equal(t, float64(30), repo.refillRuns[1].RequestSnapshot["cooldown_seconds"])
}

func TestServiceRefillLocalGroupRequiresRecentPreviewWhenConfirmWindowConfigured(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	repo := &stubUsageRepository{
		opsRows: []*adminplusdomain.LocalAccountOpsRow{
			refillOpsRow(45, 10, 0.12, "available", nil),
		},
	}
	routing := &stubRoutingPort{
		availabilities: []*RoutingGroupAvailability{{SchedulableAccounts: 0, ActiveAPIKeyCount: 2}},
	}
	svc := NewService(repo, &stubRuntimeRepository{}).WithRoutingPort(routing)
	svc.now = func() time.Time { return now }

	result, err := svc.RefillLocalGroup(context.Background(), RoutingRefillInput{
		LocalGroupID:      1001,
		Platform:          "openai",
		ConfirmWindowSecs: 300,
		CooldownSeconds:   -1,
	})

	require.NoError(t, err)
	require.Equal(t, "refill_confirmation_required", result.SkippedReason)
	require.False(t, routing.groupCalled)
	require.False(t, routing.ensureCalled)
	requireRoutingRefillFilter(t, repo, RoutingRefillRunFilter{LocalGroupID: 1001, Status: "previewed", Limit: 1})
	require.Len(t, repo.refillRuns, 1)
	require.Equal(t, "skipped", repo.refillRuns[0].Status)
	require.Equal(t, "refill_confirmation_required", repo.refillRuns[0].SkippedReason)
	require.Equal(t, float64(300), repo.refillRuns[0].RequestSnapshot["confirm_window_seconds"])
}

func TestServiceRefillLocalGroupAcceptsRecentPreviewConfirmation(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	repo := &stubUsageRepository{
		opsRows: []*adminplusdomain.LocalAccountOpsRow{
			refillOpsRow(45, 10, 0.12, "available", nil),
		},
		refillRuns: []*RoutingRefillRun{{
			ID:           99,
			LocalGroupID: 1001,
			Status:       "previewed",
			DryRun:       true,
			CreatedAt:    now.Add(-time.Minute),
		}},
	}
	routing := &stubRoutingPort{
		availabilities: []*RoutingGroupAvailability{
			{SchedulableAccounts: 0, ActiveAPIKeyCount: 2},
			{SchedulableAccounts: 0, ActiveAPIKeyCount: 2},
			{SchedulableAccounts: 1, ActiveAPIKeyCount: 2},
		},
	}
	svc := NewService(repo, &stubRuntimeRepository{}).WithRoutingPort(routing)
	svc.now = func() time.Time { return now }

	result, err := svc.RefillLocalGroup(context.Background(), RoutingRefillInput{
		LocalGroupID:      1001,
		Platform:          "openai",
		ConfirmWindowSecs: 300,
		CooldownSeconds:   -1,
	})

	require.NoError(t, err)
	require.Empty(t, result.SkippedReason)
	require.True(t, routing.ensureCalled)
	requireRoutingRefillFilter(t, repo, RoutingRefillRunFilter{LocalGroupID: 1001, Status: "previewed", Limit: 1})
	require.Len(t, repo.refillRuns, 2)
	require.Equal(t, "succeeded", repo.refillRuns[1].Status)
}

func TestServiceRefillLocalGroupDryRunIgnoresRecentSuccessCooldown(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	repo := &stubUsageRepository{
		opsRows: []*adminplusdomain.LocalAccountOpsRow{
			refillOpsRow(45, 10, 0.12, "available", nil),
		},
		refillRuns: []*RoutingRefillRun{{
			ID:           99,
			LocalGroupID: 1001,
			Status:       "succeeded",
			CreatedAt:    now.Add(-time.Minute),
		}},
	}
	routing := &stubRoutingPort{
		availabilities: []*RoutingGroupAvailability{{SchedulableAccounts: 0, ActiveAPIKeyCount: 2}},
	}
	svc := NewService(repo, &stubRuntimeRepository{}).WithRoutingPort(routing)
	svc.now = func() time.Time { return now }

	result, err := svc.RefillLocalGroup(context.Background(), RoutingRefillInput{LocalGroupID: 1001, Platform: "openai", DryRun: true})

	require.NoError(t, err)
	require.Empty(t, result.SkippedReason)
	require.NotNil(t, result.Candidate)
	require.Equal(t, int64(45), result.Candidate.LocalSub2APIAccountID)
	require.True(t, routing.groupCalled)
	require.False(t, routing.ensureCalled)
	require.Equal(t, RoutingRefillRunFilter{}, repo.refillFilter)
	require.Len(t, repo.refillRuns, 2)
	require.Equal(t, "previewed", repo.refillRuns[1].Status)
}

func TestServiceRefillLocalGroupReleasesLockAfterApply(t *testing.T) {
	base := &stubUsageRepository{
		opsRows: []*adminplusdomain.LocalAccountOpsRow{
			refillOpsRow(45, 10, 0.12, "available", nil),
		},
	}
	repo := &stubRoutingRefillLockRepository{
		stubUsageRepository: base,
		acquired:            true,
	}
	routing := &stubRoutingPort{
		availabilities: []*RoutingGroupAvailability{
			{SchedulableAccounts: 0, ActiveAPIKeyCount: 2},
			{SchedulableAccounts: 0, ActiveAPIKeyCount: 2},
			{SchedulableAccounts: 1, ActiveAPIKeyCount: 2},
		},
	}
	svc := NewService(repo, &stubRuntimeRepository{}).WithRoutingPort(routing)

	result, err := svc.RefillLocalGroup(context.Background(), RoutingRefillInput{LocalGroupID: 1001, Platform: "openai"})

	require.NoError(t, err)
	require.Empty(t, result.SkippedReason)
	require.True(t, repo.unlockCalled)
	require.True(t, routing.ensureCalled)
}

func TestServiceRefillLocalGroupSkipsWhenGroupAlreadyHasSchedulableAccounts(t *testing.T) {
	repo := &stubUsageRepository{
		opsRows: []*adminplusdomain.LocalAccountOpsRow{
			refillOpsRow(45, 10, 0.12, "available", nil),
		},
	}
	routing := &stubRoutingPort{
		availabilities: []*RoutingGroupAvailability{{SchedulableAccounts: 1}},
	}
	svc := NewService(repo, &stubRuntimeRepository{}).WithRoutingPort(routing)

	result, err := svc.RefillLocalGroup(context.Background(), RoutingRefillInput{LocalGroupID: 1001})

	require.NoError(t, err)
	require.Equal(t, "group_has_schedulable_accounts", result.SkippedReason)
	require.False(t, routing.ensureCalled)
	require.Zero(t, repo.opsFilter.Limit)
	require.Len(t, repo.refillRuns, 1)
	require.Equal(t, "skipped", repo.refillRuns[0].Status)
	require.Equal(t, "group_has_schedulable_accounts", repo.refillRuns[0].SkippedReason)
}

func TestServiceRefillLocalGroupSuppressesRecentlyFailedCandidate(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	repo := &stubUsageRepository{
		opsRows: []*adminplusdomain.LocalAccountOpsRow{
			refillOpsRow(45, 10, 0.12, "available", nil),
			refillOpsRow(46, 11, 0.16, "available", nil),
		},
		refillRuns: []*RoutingRefillRun{{
			ID:                     99,
			LocalGroupID:           1001,
			Status:                 "failed",
			SelectedLocalAccountID: 45,
			CreatedAt:              now.Add(-time.Minute),
		}},
	}
	routing := &stubRoutingPort{
		availabilities: []*RoutingGroupAvailability{
			{SchedulableAccounts: 0, ActiveAPIKeyCount: 2},
			{SchedulableAccounts: 0, ActiveAPIKeyCount: 2},
			{SchedulableAccounts: 1, ActiveAPIKeyCount: 2},
		},
	}
	svc := NewService(repo, &stubRuntimeRepository{}).WithRoutingPort(routing)
	svc.now = func() time.Time { return now }

	result, err := svc.RefillLocalGroup(context.Background(), RoutingRefillInput{
		LocalGroupID:    1001,
		Platform:        "openai",
		CooldownSeconds: -1,
	})

	require.NoError(t, err)
	require.Empty(t, result.SkippedReason)
	require.NotNil(t, result.Candidate)
	require.Equal(t, int64(46), result.Candidate.LocalSub2APIAccountID)
	require.Equal(t, int64(46), routing.groupAccount)
	requireRoutingRefillFilter(t, repo, RoutingRefillRunFilter{LocalGroupID: 1001, Status: "failed", Limit: 100})
	require.Len(t, repo.refillRuns, 2)
	require.Equal(t, "succeeded", repo.refillRuns[1].Status)
	require.Equal(t, int64(46), repo.refillRuns[1].SelectedLocalAccountID)
}

func TestServiceRefillLocalGroupSkipsWhenOnlyCandidateRecentlyFailed(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	repo := &stubUsageRepository{
		opsRows: []*adminplusdomain.LocalAccountOpsRow{
			refillOpsRow(45, 10, 0.12, "available", nil),
		},
		refillRuns: []*RoutingRefillRun{{
			ID:                     99,
			LocalGroupID:           1001,
			Status:                 "failed",
			SelectedLocalAccountID: 45,
			CreatedAt:              now.Add(-time.Minute),
		}},
	}
	routing := &stubRoutingPort{
		availabilities: []*RoutingGroupAvailability{{SchedulableAccounts: 0, ActiveAPIKeyCount: 2}},
	}
	svc := NewService(repo, &stubRuntimeRepository{}).WithRoutingPort(routing)
	svc.now = func() time.Time { return now }

	result, err := svc.RefillLocalGroup(context.Background(), RoutingRefillInput{
		LocalGroupID:    1001,
		Platform:        "openai",
		CooldownSeconds: -1,
	})

	require.NoError(t, err)
	require.Equal(t, "candidate_suppressed_after_failure", result.SkippedReason)
	require.False(t, routing.ensureCalled)
	require.Len(t, repo.refillRuns, 2)
	require.Equal(t, "skipped", repo.refillRuns[1].Status)
	require.Equal(t, "candidate_suppressed_after_failure", repo.refillRuns[1].SkippedReason)
}

type stubRoutingRefillLockRepository struct {
	*stubUsageRepository
	acquired      bool
	lockRequested bool
	unlockCalled  bool
	lockGroupID   int64
}

func (r *stubRoutingRefillLockRepository) TryRoutingRefillLock(_ context.Context, groupID int64) (RoutingRefillUnlockFunc, bool, error) {
	r.lockRequested = true
	r.lockGroupID = groupID
	if !r.acquired {
		return nil, false, nil
	}
	return func() error {
		r.unlockCalled = true
		return nil
	}, true, nil
}

func refillOpsRow(accountID int64, supplierID int64, rate float64, candidateStatus string, groupIDs []int64) *adminplusdomain.LocalAccountOpsRow {
	return &adminplusdomain.LocalAccountOpsRow{
		LocalSub2APIAccountID:    accountID,
		LocalAccountName:         "account",
		LocalAccountPlatform:     "openai",
		LocalAccountStatus:       "active",
		LocalAccountSchedulable:  true,
		LocalAccountGroupIDs:     append([]int64(nil), groupIDs...),
		SupplierID:               supplierID,
		SupplierName:             "supplier",
		SupplierAccountID:        accountID + 1000,
		SupplierRuntimeStatus:    "active",
		SupplierHealthStatus:     "normal",
		SupplierGroupID:          accountID + 2000,
		SupplierGroupStatus:      "active",
		SupplierGroupModelFamily: "OpenAI",
		SupplierGroupModelSpec:   "GPT-4o",
		SupplierKeyID:            accountID + 3000,
		SupplierKeyStatus:        "bound",
		HasUsableBalance:         true,
		BalanceStatus:            "usable",
		ChannelCheckStatus:       "available",
		DriftStatus:              "synced",
		CandidateStatus:          candidateStatus,
		CheckSource:              "channel_monitor",
		EffectiveRateMultiplier:  rate,
	}
}

func requireRoutingRefillFilter(t *testing.T, repo *stubUsageRepository, expected RoutingRefillRunFilter) {
	t.Helper()
	for _, filter := range repo.refillFilters {
		if filter == expected {
			return
		}
	}
	require.Failf(t, "routing refill filter not found", "expected %+v in %+v", expected, repo.refillFilters)
}
