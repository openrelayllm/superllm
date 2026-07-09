package actions

import (
	"context"
	"testing"

	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func TestServiceGenerateSwitchesFromDepletedActiveSupplierToEligibleCandidate(t *testing.T) {
	svc := NewRuleService()

	result, err := svc.Generate(context.Background(), GenerateInput{
		Suppliers: []SupplierSignal{
			{
				SupplierID:         1,
				Name:               "active",
				RuntimeStatus:      adminplusdomain.SupplierRuntimeStatusActive,
				HealthStatus:       adminplusdomain.SupplierHealthStatusNormal,
				BalanceCents:       0,
				EffectiveCostCents: 100,
			},
			{
				SupplierID:         2,
				Name:               "candidate",
				RuntimeStatus:      adminplusdomain.SupplierRuntimeStatusCandidate,
				HealthStatus:       adminplusdomain.SupplierHealthStatusNormal,
				BalanceCents:       5000,
				EffectiveCostCents: 80,
			},
		},
	})

	require.NoError(t, err)
	requireAction(t, result.Items, adminplusdomain.ActionTypePauseSupplier, "active_supplier_depleted")
	switchAction := requireAction(t, result.Items, adminplusdomain.ActionTypeSwitchSupplier, "switch_from_depleted_supplier")
	require.NotNil(t, switchAction.TargetSupplierID)
	require.Equal(t, int64(2), *switchAction.TargetSupplierID)
}

func TestServiceGeneratePausesAndSwitchesFromFailingActiveSupplier(t *testing.T) {
	svc := NewRuleService()

	result, err := svc.Generate(context.Background(), GenerateInput{
		Suppliers: []SupplierSignal{
			{
				SupplierID:         1,
				RuntimeStatus:      adminplusdomain.SupplierRuntimeStatusActive,
				HealthStatus:       adminplusdomain.SupplierHealthStatusNormal,
				BalanceCents:       5000,
				EffectiveCostCents: 100,
			},
			{
				SupplierID:         2,
				RuntimeStatus:      adminplusdomain.SupplierRuntimeStatusCandidate,
				HealthStatus:       adminplusdomain.SupplierHealthStatusNormal,
				BalanceCents:       5000,
				EffectiveCostCents: 90,
			},
		},
		HealthEvents: []*adminplusdomain.HealthEvent{
			{
				SupplierID: 1,
				Type:       adminplusdomain.HealthEventTypeRequestError,
				Status:     adminplusdomain.HealthEventStatusOpen,
			},
		},
	})

	require.NoError(t, err)
	requireAction(t, result.Items, adminplusdomain.ActionTypePauseSupplier, "supplier_request_errors")
	requireAction(t, result.Items, adminplusdomain.ActionTypeSwitchSupplier, "switch_from_failing_supplier")
}

func TestServiceGenerateDegradesSlowOrSaturatedSupplier(t *testing.T) {
	svc := NewRuleService()

	result, err := svc.Generate(context.Background(), GenerateInput{
		Suppliers: []SupplierSignal{
			{
				SupplierID:    1,
				RuntimeStatus: adminplusdomain.SupplierRuntimeStatusActive,
				HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
				BalanceCents:  5000,
			},
		},
		HealthEvents: []*adminplusdomain.HealthEvent{
			{
				SupplierID: 1,
				Type:       adminplusdomain.HealthEventTypeSlowFirstToken,
				Status:     adminplusdomain.HealthEventStatusOpen,
			},
		},
	})

	require.NoError(t, err)
	requireAction(t, result.Items, adminplusdomain.ActionTypeDegradeSupplier, "supplier_performance_degraded")
}

func TestServiceGeneratePausesAndSwitchesFromCriticalKanbanSupplierRisk(t *testing.T) {
	svc := NewRuleService()

	result, err := svc.Generate(context.Background(), GenerateInput{
		Suppliers: []SupplierSignal{
			{
				SupplierID:         1,
				RuntimeStatus:      adminplusdomain.SupplierRuntimeStatusActive,
				HealthStatus:       adminplusdomain.SupplierHealthStatusNormal,
				BalanceCents:       5000,
				EffectiveCostCents: 100,
			},
			{
				SupplierID:         2,
				RuntimeStatus:      adminplusdomain.SupplierRuntimeStatusCandidate,
				HealthStatus:       adminplusdomain.SupplierHealthStatusNormal,
				BalanceCents:       5000,
				EffectiveCostCents: 90,
			},
		},
		KanbanEvents: []*adminplusdomain.KanbanEvent{
			{
				ID:         31,
				EventType:  "supply_quality_risk",
				Severity:   "critical",
				Status:     "open",
				Model:      "gpt-risk",
				SourceType: "supplier",
				SourceID:   1,
				Title:      "供应质量风险",
			},
		},
	})

	require.NoError(t, err)
	requireAction(t, result.Items, adminplusdomain.ActionTypePauseSupplier, "kanban_supply_quality_blocked")
	switchAction := requireAction(t, result.Items, adminplusdomain.ActionTypeSwitchSupplier, "switch_from_supply_quality_risk")
	require.NotNil(t, switchAction.TargetSupplierID)
	require.Equal(t, int64(2), *switchAction.TargetSupplierID)
	require.Contains(t, switchAction.Signals, "kanban_event_id=31")
}

func TestServiceGeneratePricingReviewFromKanbanEventWithoutSuppliers(t *testing.T) {
	svc := NewRuleService()

	result, err := svc.Generate(context.Background(), GenerateInput{
		KanbanEvents: []*adminplusdomain.KanbanEvent{
			{
				ID:        41,
				EventType: "market_price_drop",
				Severity:  "warning",
				Status:    "open",
				Model:     "gpt-price",
				Title:     "市场价格下降",
			},
		},
	})

	require.NoError(t, err)
	action := requireAction(t, result.Items, adminplusdomain.ActionTypeInvestigateProfit, "kanban_pricing_review")
	require.Equal(t, int64(0), action.SupplierID)
	require.Contains(t, action.Signals, "model=gpt-price")
}

func TestServiceGenerateCandidateEvaluationPrioritizesRechargeOverChannelFailure(t *testing.T) {
	svc := NewRuleService()

	result, err := svc.Generate(context.Background(), GenerateInput{
		CandidateEvaluations: []CandidateSignal{{
			SupplierID:              7,
			SupplierGroupID:         1001,
			LocalSub2APIAccountID:   42,
			CandidateStatus:         "balance_blocked",
			BlockedReason:           "recharge_required",
			CheckSource:             "balance",
			BalanceStatus:           "recharge_required",
			KeyCapacityStatus:       "unknown",
			EffectiveRateMultiplier: 0.2,
		}},
	})

	require.NoError(t, err)
	action := requireAction(t, result.Items, adminplusdomain.ActionTypeRechargeSupplier, "candidate_balance_recharge_required")
	require.Equal(t, int64(7), action.SupplierID)
	require.Contains(t, action.Signals, "blocked_reason=recharge_required")
	require.Contains(t, action.Signals, "effective_rate_multiplier=0.2")
	require.NotContains(t, action.Signals, "channel_monitor_failed")
}

func TestServiceGenerateBalanceRecoveredCreatesRecheckAction(t *testing.T) {
	svc := NewRuleService()
	oldBalance := int64(0)

	result, err := svc.Generate(context.Background(), GenerateInput{
		BalanceEvents: []*adminplusdomain.BalanceEvent{
			{
				ID:                       31,
				SupplierID:               7,
				Type:                     adminplusdomain.BalanceEventTypeRecovered,
				OldBalanceCents:          &oldBalance,
				NewBalanceCents:          5000,
				LowBalanceThresholdCents: 1000,
				SwitchEligible:           true,
				Status:                   adminplusdomain.BalanceEventStatusOpen,
			},
		},
	})

	require.NoError(t, err)
	action := requireAction(t, result.Items, adminplusdomain.ActionTypeReviewCredential, "supplier_balance_recovered_recheck")
	require.Equal(t, int64(7), action.SupplierID)
	require.Equal(t, adminplusdomain.ActionSeverityInfo, action.Severity)
	require.Contains(t, action.Signals, "balance_event_id=31")
	require.Contains(t, action.Signals, "balance_event_type=recovered")
	require.Contains(t, action.Signals, "old_balance_cents=0")
	require.Contains(t, action.Signals, "balance_cents=5000")
	require.Contains(t, action.Signals, "switch_eligible=true")
}

func TestServiceGenerateCostSnapshotBalanceDeltaCreatesReconcileAction(t *testing.T) {
	svc := NewRuleService()
	actualBalance := int64(5000)
	balanceDelta := int64(-1500)

	result, err := svc.Generate(context.Background(), GenerateInput{
		CostSnapshots: []*adminplusdomain.SupplierCostSnapshot{
			{
				ID:                          41,
				SupplierID:                  7,
				Currency:                    "USD",
				CompletedFundingAmountCents: 10000,
				EntitlementAmountCents:      2000,
				UsageCostCents:              5500,
				ExpectedBalanceCents:        6500,
				ActualBalanceCents:          &actualBalance,
				BalanceDeltaCents:           &balanceDelta,
			},
		},
	})

	require.NoError(t, err)
	action := requireAction(t, result.Items, adminplusdomain.ActionTypeSupplierCostReconcileAdjustment, "supplier_cost_balance_reconcile_anomaly")
	require.Equal(t, int64(7), action.SupplierID)
	require.Equal(t, adminplusdomain.ActionSeverityCritical, action.Severity)
	require.Contains(t, action.Signals, "cost_snapshot_id=41")
	require.Contains(t, action.Signals, "expected_balance_cents=6500")
	require.Contains(t, action.Signals, "actual_balance_cents=5000")
	require.Contains(t, action.Signals, "balance_delta_cents=-1500")
}

func TestServiceGenerateCandidateEvaluationCreatesKeyCapacityAction(t *testing.T) {
	svc := NewRuleService()

	result, err := svc.Generate(context.Background(), GenerateInput{
		CandidateEvaluations: []CandidateSignal{{
			SupplierID:              7,
			SupplierGroupID:         1001,
			CandidateStatus:         "capacity_blocked",
			BlockedReason:           "key_capacity_exhausted",
			CheckSource:             "key_capacity",
			KeyCapacityStatus:       "exhausted",
			EffectiveRateMultiplier: 0.1,
		}},
	})

	require.NoError(t, err)
	action := requireAction(t, result.Items, adminplusdomain.ActionTypeReviewCredential, "candidate_key_capacity_exhausted")
	require.Equal(t, adminplusdomain.ActionSeverityWarning, action.Severity)
	require.Contains(t, action.Signals, "supplier_group_id=1001")
}

func TestServiceGenerateCandidateEvaluationCreatesPurityActions(t *testing.T) {
	svc := NewRuleService()

	result, err := svc.Generate(context.Background(), GenerateInput{
		CandidateEvaluations: []CandidateSignal{
			{
				SupplierID:              7,
				SupplierGroupID:         1001,
				LocalSub2APIAccountID:   42,
				CandidateStatus:         "blocked",
				BlockedReason:           "purity_failed",
				CheckSource:             "purity",
				EffectiveRateMultiplier: 0.1,
			},
			{
				SupplierID:              8,
				SupplierGroupID:         1002,
				LocalSub2APIAccountID:   43,
				CandidateStatus:         "degraded",
				BlockedReason:           "purity_risk",
				CheckSource:             "purity",
				EffectiveRateMultiplier: 0.15,
			},
			{
				SupplierID:              9,
				SupplierGroupID:         1003,
				LocalSub2APIAccountID:   44,
				CandidateStatus:         "unknown",
				BlockedReason:           "purity_stale",
				CheckSource:             "purity",
				PurityFreshness:         "stale",
				EffectiveRateMultiplier: 0.12,
			},
		},
	})

	require.NoError(t, err)
	failed := requireAction(t, result.Items, adminplusdomain.ActionTypeReviewCredential, "candidate_purity_failed")
	require.Equal(t, adminplusdomain.ActionSeverityWarning, failed.Severity)
	require.Contains(t, failed.Signals, "blocked_reason=purity_failed")
	require.Contains(t, failed.Signals, "check_source=purity")
	risk := requireAction(t, result.Items, adminplusdomain.ActionTypeReviewCredential, "candidate_purity_risk")
	require.Equal(t, adminplusdomain.ActionSeverityInfo, risk.Severity)
	require.Contains(t, risk.Signals, "blocked_reason=purity_risk")
	stale := requireAction(t, result.Items, adminplusdomain.ActionTypeReviewCredential, "candidate_purity_stale")
	require.Equal(t, adminplusdomain.ActionSeverityWarning, stale.Severity)
	require.Contains(t, stale.Signals, "blocked_reason=purity_stale")
	require.Contains(t, stale.Signals, "purity_freshness_status=stale")
}

func TestServiceGenerateCandidateEvaluationCreatesProxyAction(t *testing.T) {
	svc := NewRuleService()

	result, err := svc.Generate(context.Background(), GenerateInput{
		CandidateEvaluations: []CandidateSignal{{
			SupplierID:              7,
			SupplierGroupID:         1001,
			LocalSub2APIAccountID:   42,
			CandidateStatus:         "blocked",
			BlockedReason:           "proxy_expired",
			CheckSource:             "proxy",
			EffectiveRateMultiplier: 0.1,
		}},
	})

	require.NoError(t, err)
	action := requireAction(t, result.Items, adminplusdomain.ActionTypeReviewCredential, "candidate_proxy_unavailable")
	require.Equal(t, adminplusdomain.ActionSeverityWarning, action.Severity)
	require.Contains(t, action.Signals, "blocked_reason=proxy_expired")
	require.Contains(t, action.Signals, "check_source=proxy")
}

func TestServiceGenerateSupplierKeyCapacityActions(t *testing.T) {
	svc := NewRuleService()

	result, err := svc.Generate(context.Background(), GenerateInput{
		Suppliers: []SupplierSignal{
			{
				SupplierID:        7,
				RuntimeStatus:     adminplusdomain.SupplierRuntimeStatusCandidate,
				HealthStatus:      adminplusdomain.SupplierHealthStatusNormal,
				BalanceCents:      5000,
				KeyLimitPolicy:    adminplusdomain.SupplierKeyLimitPolicyLimited,
				KeyLimitValue:     10,
				ActiveKeyCount:    10,
				KeyCapacityStatus: adminplusdomain.SupplierKeyCapacityExhausted,
			},
			{
				SupplierID:        8,
				RuntimeStatus:     adminplusdomain.SupplierRuntimeStatusCandidate,
				HealthStatus:      adminplusdomain.SupplierHealthStatusNormal,
				BalanceCents:      5000,
				KeyLimitPolicy:    adminplusdomain.SupplierKeyLimitPolicyUnknown,
				KeyCapacityStatus: adminplusdomain.SupplierKeyCapacityUnknown,
			},
		},
	})

	require.NoError(t, err)
	exhausted := requireAction(t, result.Items, adminplusdomain.ActionTypeReviewCredential, "supplier_key_capacity_exhausted")
	require.Equal(t, int64(7), exhausted.SupplierID)
	require.Equal(t, adminplusdomain.ActionSeverityWarning, exhausted.Severity)
	require.Contains(t, exhausted.Signals, "key_limit_value=10")
	require.Contains(t, exhausted.Signals, "active_key_count=10")
	unknown := requireAction(t, result.Items, adminplusdomain.ActionTypeReviewCredential, "supplier_key_capacity_unknown")
	require.Equal(t, int64(8), unknown.SupplierID)
	require.Equal(t, adminplusdomain.ActionSeverityInfo, unknown.Severity)
}

func TestServiceGenerateLocalGroupCapacityCreatesRoutingRefillAction(t *testing.T) {
	svc := NewRuleService()

	result, err := svc.Generate(context.Background(), GenerateInput{
		LocalGroupCapacity: []LocalGroupCapacitySignal{
			{
				LocalGroupID:                   9,
				LocalGroupName:                 "Lime",
				Platform:                       "openai",
				TotalAccounts:                  2,
				SchedulableAccounts:            0,
				ActiveAPIKeyCount:              3,
				LowCapacityThreshold:           1,
				BestCandidateSupplierID:        7,
				BestCandidateSupplierGroupID:   1001,
				BestCandidateLocalAccountID:    42,
				BestCandidateRateMultiplier:    0.2,
				BestCandidateCheckSource:       "channel_monitor",
				BestCandidateSupplierName:      "supplier-a",
				BestCandidateSupplierGroupName: "group-a",
			},
		},
	})

	require.NoError(t, err)
	action := requireAction(t, result.Items, adminplusdomain.ActionTypeRoutingRefill, "local_group_routing_refill_required")
	require.Equal(t, adminplusdomain.ActionSeverityCritical, action.Severity)
	require.Equal(t, int64(7), action.SupplierID)
	require.Contains(t, action.Signals, "local_group_id=9")
	require.Contains(t, action.Signals, "active_api_key_count=3")
	require.Contains(t, action.Signals, "candidate_local_account_id=42")
	require.Contains(t, action.Signals, "candidate_effective_rate_multiplier=0.2")
}

func TestServiceGenerateLocalAccountScheduleCreatesDisableAction(t *testing.T) {
	svc := NewRuleService()

	result, err := svc.Generate(context.Background(), GenerateInput{
		LocalAccountSchedule: []LocalAccountScheduleSignal{
			{
				SupplierID:              7,
				SupplierGroupID:         1001,
				LocalSub2APIAccountID:   42,
				LocalAccountName:        "lime-account",
				SupplierName:            "supplier-a",
				SupplierGroupName:       "group-a",
				LocalGroupIDs:           []int64{9},
				LocalGroupNames:         []string{"Lime"},
				LocalAccountSchedulable: true,
				CandidateStatus:         "blocked",
				BlockedReason:           "channel_monitor_failed",
				CheckSource:             "channel_monitor",
				ChannelCheckStatus:      "request_error",
				EffectiveRateMultiplier: 0.2,
			},
		},
	})

	require.NoError(t, err)
	action := requireAction(t, result.Items, adminplusdomain.ActionTypeLocalAccountScheduleDisable, "local_account_schedule_disable_required")
	require.Equal(t, adminplusdomain.ActionSeverityWarning, action.Severity)
	require.Equal(t, int64(7), action.SupplierID)
	require.Contains(t, action.Signals, "local_sub2api_account_id=42")
	require.Contains(t, action.Signals, "blocked_reason=channel_monitor_failed")
	require.Contains(t, action.Signals, "local_group_ids=9")
	require.Contains(t, action.Signals, "effective_rate_multiplier=0.2")
}

func TestServiceGenerateLocalAccountScheduleIgnoresBalanceBlockedAccount(t *testing.T) {
	svc := NewRuleService()

	result, err := svc.Generate(context.Background(), GenerateInput{
		LocalAccountSchedule: []LocalAccountScheduleSignal{
			{
				SupplierID:              7,
				LocalSub2APIAccountID:   42,
				LocalGroupIDs:           []int64{9},
				LocalAccountSchedulable: true,
				CandidateStatus:         "balance_blocked",
				BlockedReason:           "recharge_required",
				CheckSource:             "balance",
			},
		},
	})

	require.NoError(t, err)
	require.Empty(t, result.Items)
}

func TestServiceGenerateReusesEquivalentOpenRoutingRefillAction(t *testing.T) {
	repo := newFakeActionRepository(&adminplusdomain.ActionRecommendation{
		ID:         10,
		Type:       adminplusdomain.ActionTypeRoutingRefill,
		Status:     adminplusdomain.ActionStatusOpen,
		SupplierID: 7,
		ReasonCode: "local_group_routing_refill_required",
		Signals:    []string{"local_group_id=9"},
	})
	svc := NewService(repo)

	result, err := svc.Generate(context.Background(), GenerateInput{
		LocalGroupCapacity: []LocalGroupCapacitySignal{
			{
				LocalGroupID:                9,
				SchedulableAccounts:         0,
				ActiveAPIKeyCount:           3,
				BestCandidateSupplierID:     7,
				BestCandidateLocalAccountID: 42,
			},
		},
	})

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, int64(10), result.Items[0].ID)
	require.Len(t, repo.actions, 1)
}

func TestServiceGenerateDispatchesNotificationForStoredRecommendation(t *testing.T) {
	repo := newFakeActionRepository()
	notifier := &fakeActionNotificationDispatcher{}
	svc := NewServiceWithDependencies(repo, nil, notifier)

	result, err := svc.Generate(context.Background(), GenerateInput{
		LocalGroupCapacity: []LocalGroupCapacitySignal{{
			LocalGroupID:                9,
			SchedulableAccounts:         0,
			ActiveAPIKeyCount:           3,
			BestCandidateSupplierID:     7,
			BestCandidateLocalAccountID: 42,
		}},
	})

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Len(t, notifier.actions, 1)
	require.Equal(t, result.Items[0].ID, notifier.actions[0].ID)
	require.Equal(t, "local_group_routing_refill_required", notifier.actions[0].ReasonCode)
}

func TestServiceGenerateCreatesNewRecommendationWhenEquivalentWasExecuted(t *testing.T) {
	repo := newFakeActionRepository(&adminplusdomain.ActionRecommendation{
		ID:         10,
		Type:       adminplusdomain.ActionTypeRoutingRefill,
		Status:     adminplusdomain.ActionStatusExecuted,
		SupplierID: 7,
		ReasonCode: "local_group_routing_refill_required",
		Signals:    []string{"local_group_id=9"},
	})
	svc := NewService(repo)

	result, err := svc.Generate(context.Background(), GenerateInput{
		LocalGroupCapacity: []LocalGroupCapacitySignal{
			{
				LocalGroupID:                9,
				SchedulableAccounts:         0,
				ActiveAPIKeyCount:           3,
				BestCandidateSupplierID:     7,
				BestCandidateLocalAccountID: 42,
			},
		},
	})

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.NotEqual(t, int64(10), result.Items[0].ID)
	require.Equal(t, adminplusdomain.ActionStatusOpen, result.Items[0].Status)
	require.Len(t, repo.actions, 2)
}

func TestServiceExecuteApprovedRecommendationRecordsSucceededReceipt(t *testing.T) {
	repo := newFakeActionRepository(&adminplusdomain.ActionRecommendation{
		ID:         1,
		Type:       adminplusdomain.ActionTypeReviewCredential,
		Status:     adminplusdomain.ActionStatusApproved,
		ReasonCode: "credential_invalid",
	})
	svc := NewService(repo)

	execution, err := svc.ExecuteApprovedRecommendation(context.Background(), 1, ExecuteInput{
		OperatorUserID:  99,
		SchedulerRunID:  "scheduler-run-1",
		SchedulerStepID: 123,
		RequestPayload:  map[string]any{"note": "reviewed"},
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ActionExecutionStatusSucceeded, execution.Status)
	require.Equal(t, int64(99), execution.OperatorUserID)
	require.Equal(t, "scheduler-run-1", execution.SchedulerRunID)
	require.Equal(t, int64(123), execution.SchedulerStepID)
	require.Equal(t, adminplusdomain.ActionStatusExecuted, repo.actions[1].Status)
	require.Len(t, repo.executions, 1)
}

func TestServiceExecuteApprovedRecommendationKeepsUnsupportedRoutingActionApproved(t *testing.T) {
	repo := newFakeActionRepository(&adminplusdomain.ActionRecommendation{
		ID:         2,
		Type:       adminplusdomain.ActionTypeSwitchSupplier,
		Status:     adminplusdomain.ActionStatusApproved,
		SupplierID: 7,
	})
	svc := NewService(repo)

	execution, err := svc.ExecuteApprovedRecommendation(context.Background(), 2, ExecuteInput{})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ActionExecutionStatusUnsupported, execution.Status)
	require.Equal(t, adminplusdomain.ActionStatusApproved, repo.actions[2].Status)
	require.Contains(t, execution.ResponsePayload["mode"], "unsupported")
}

func TestServiceRecordExternalExecutionUpdatesSucceededRoutingRefill(t *testing.T) {
	repo := newFakeActionRepository(&adminplusdomain.ActionRecommendation{
		ID:         4,
		Type:       adminplusdomain.ActionTypeRoutingRefill,
		Status:     adminplusdomain.ActionStatusApproved,
		SupplierID: 7,
		Signals:    []string{"local_group_id=9"},
	})
	svc := NewService(repo)

	execution, err := svc.RecordExternalExecution(context.Background(), 4, ExecuteInput{
		OperatorUserID:     99,
		SchedulerRunID:     "routing_capacity_watch-1",
		SchedulerStepID:    456,
		IdempotencyKeyHash: "idempotency-hash-1",
		BeforeSnapshot:     map[string]any{"schedulable_accounts": int64(0)},
		AfterSnapshot:      map[string]any{"schedulable_accounts": int64(1)},
		RequestPayload:     map[string]any{"local_group_id": int64(9)},
	}, adminplusdomain.ActionExecutionStatusSucceeded, map[string]any{
		"mode":                                "routing_refill_apply",
		"candidate_local_account_id":          int64(42),
		"after_schedulable_accounts":          int64(1),
		"candidate_effective_rate_multiplier": 0.2,
	}, "")

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ActionTypeRoutingRefill, execution.ActionType)
	require.Equal(t, adminplusdomain.ActionExecutionStatusSucceeded, execution.Status)
	require.Equal(t, "routing_capacity_watch-1", execution.SchedulerRunID)
	require.Equal(t, int64(456), execution.SchedulerStepID)
	require.Equal(t, "idempotency-hash-1", execution.IdempotencyKeyHash)
	require.Equal(t, int64(0), execution.BeforeSnapshot["schedulable_accounts"])
	require.Equal(t, int64(1), execution.AfterSnapshot["schedulable_accounts"])
	require.Equal(t, adminplusdomain.ActionStatusExecuted, repo.actions[4].Status)
	require.Len(t, repo.executions, 1)
}

func TestServiceRecordExternalExecutionKeepsFailedRoutingRefillApproved(t *testing.T) {
	repo := newFakeActionRepository(&adminplusdomain.ActionRecommendation{
		ID:     5,
		Type:   adminplusdomain.ActionTypeRoutingRefill,
		Status: adminplusdomain.ActionStatusApproved,
	})
	svc := NewService(repo)

	execution, err := svc.RecordExternalExecution(context.Background(), 5, ExecuteInput{}, adminplusdomain.ActionExecutionStatusFailed, map[string]any{
		"mode":           "routing_refill_apply",
		"skipped_reason": "candidate_not_found",
	}, "candidate_not_found")

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ActionExecutionStatusFailed, execution.Status)
	require.Equal(t, "candidate_not_found", execution.ErrorMessage)
	require.Equal(t, adminplusdomain.ActionStatusApproved, repo.actions[5].Status)
}

func TestServiceGetRecommendationExecutionRejectsMismatchedRecommendation(t *testing.T) {
	repo := newFakeActionRepository(&adminplusdomain.ActionRecommendation{
		ID:     6,
		Type:   adminplusdomain.ActionTypeRoutingRefill,
		Status: adminplusdomain.ActionStatusApproved,
	})
	repo.executions = append(repo.executions, &adminplusdomain.ActionExecution{
		ID:               31,
		RecommendationID: 99,
		ActionType:       adminplusdomain.ActionTypeRoutingRefill,
		Status:           adminplusdomain.ActionExecutionStatusFailed,
	})
	svc := NewService(repo)

	_, err := svc.GetRecommendationExecution(context.Background(), 6, 31)

	require.Error(t, err)
	require.Contains(t, err.Error(), "ACTION_EXECUTION_RECOMMENDATION_MISMATCH")
}

func TestServiceMarkExecutionIdempotencyReplayedUpdatesMatchingExecution(t *testing.T) {
	repo := newFakeActionRepository(&adminplusdomain.ActionRecommendation{
		ID:     7,
		Type:   adminplusdomain.ActionTypeRoutingRefill,
		Status: adminplusdomain.ActionStatusExecuted,
	})
	repo.executions = append(repo.executions, &adminplusdomain.ActionExecution{
		ID:                 41,
		RecommendationID:   7,
		ActionType:         adminplusdomain.ActionTypeRoutingRefill,
		Status:             adminplusdomain.ActionExecutionStatusSucceeded,
		IdempotencyKeyHash: "hash-1",
	})
	svc := NewService(repo)

	execution, err := svc.MarkExecutionIdempotencyReplayed(context.Background(), 7, " hash-1 ")

	require.NoError(t, err)
	require.Equal(t, int64(41), execution.ID)
	require.True(t, execution.IdempotencyReplayed)
	require.True(t, repo.executions[0].IdempotencyReplayed)
}

func TestServiceMarkLatestExecutionIdempotencyReplayedUpdatesMatchingActionType(t *testing.T) {
	repo := newFakeActionRepository(&adminplusdomain.ActionRecommendation{
		ID:     8,
		Type:   adminplusdomain.ActionTypeLocalAccountManualOps,
		Status: adminplusdomain.ActionStatusExecuted,
	})
	repo.executions = append(repo.executions, &adminplusdomain.ActionExecution{
		ID:                 51,
		RecommendationID:   8,
		ActionType:         adminplusdomain.ActionTypeLocalAccountManualOps,
		Status:             adminplusdomain.ActionExecutionStatusSucceeded,
		IdempotencyKeyHash: "hash-2",
	})
	svc := NewService(repo)

	execution, err := svc.MarkLatestExecutionIdempotencyReplayed(context.Background(), adminplusdomain.ActionTypeLocalAccountManualOps, " hash-2 ")

	require.NoError(t, err)
	require.Equal(t, int64(51), execution.ID)
	require.True(t, execution.IdempotencyReplayed)
}

func TestServiceRecordManualExecutionCreatesExecutedRecommendationAndExecution(t *testing.T) {
	repo := newFakeActionRepository()
	svc := NewService(repo)

	execution, err := svc.RecordManualExecution(context.Background(), ManualExecutionInput{
		ActionType:         adminplusdomain.ActionTypeLocalAccountManualOps,
		Severity:           adminplusdomain.ActionSeverityInfo,
		ReasonCode:         "local_account_manual_ops_apply",
		Title:              "Disable local account scheduling",
		Description:        "manual operation",
		ExpectedImpact:     "audit local routing change",
		Signals:            []string{"local_sub2api_account_id=42", "manual=true"},
		OperatorUserID:     99,
		IdempotencyKeyHash: "manual-hash-1",
		RequestPayload:     map[string]any{"action": "set_schedulable"},
		ResponsePayload:    map[string]any{"updated_accounts": int64(1)},
		BeforeSnapshot:     map[string]any{"mode": "before"},
		AfterSnapshot:      map[string]any{"mode": "after"},
		Status:             adminplusdomain.ActionExecutionStatusSucceeded,
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ActionTypeLocalAccountManualOps, execution.ActionType)
	require.Equal(t, adminplusdomain.ActionExecutionStatusSucceeded, execution.Status)
	require.Equal(t, "manual-hash-1", execution.IdempotencyKeyHash)
	require.Len(t, repo.actions, 1)
	require.Equal(t, adminplusdomain.ActionStatusExecuted, repo.actions[1].Status)
	require.False(t, repo.actions[1].RequiresApproval)
	require.Equal(t, []string{"local_sub2api_account_id=42", "manual=true"}, repo.actions[1].Signals)
}

func TestServiceRecordExternalExecutionUpdatesSucceededLocalAccountScheduleDisable(t *testing.T) {
	repo := newFakeActionRepository(&adminplusdomain.ActionRecommendation{
		ID:         6,
		Type:       adminplusdomain.ActionTypeLocalAccountScheduleDisable,
		Status:     adminplusdomain.ActionStatusApproved,
		SupplierID: 7,
		Signals:    []string{"local_sub2api_account_id=42"},
	})
	svc := NewService(repo)

	execution, err := svc.RecordExternalExecution(context.Background(), 6, ExecuteInput{
		OperatorUserID: 99,
		RequestPayload: map[string]any{"account_ids": []int64{42}, "schedulable": false},
	}, adminplusdomain.ActionExecutionStatusSucceeded, map[string]any{
		"mode":             "local_account_ops_apply",
		"updated_accounts": int64(1),
	}, "")

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ActionTypeLocalAccountScheduleDisable, execution.ActionType)
	require.Equal(t, adminplusdomain.ActionExecutionStatusSucceeded, execution.Status)
	require.Equal(t, adminplusdomain.ActionStatusExecuted, repo.actions[6].Status)
}

func TestServiceExecuteApprovedRecommendationUpdatesSupplierStatus(t *testing.T) {
	cases := []struct {
		name              string
		actionType        adminplusdomain.ActionType
		wantRuntimeStatus adminplusdomain.SupplierRuntimeStatus
		wantHealthStatus  adminplusdomain.SupplierHealthStatus
	}{
		{
			name:              "pause",
			actionType:        adminplusdomain.ActionTypePauseSupplier,
			wantRuntimeStatus: adminplusdomain.SupplierRuntimeStatusDisabled,
			wantHealthStatus:  adminplusdomain.SupplierHealthStatusPaused,
		},
		{
			name:              "degrade",
			actionType:        adminplusdomain.ActionTypeDegradeSupplier,
			wantRuntimeStatus: adminplusdomain.SupplierRuntimeStatusMonitorOnly,
			wantHealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeActionRepository(&adminplusdomain.ActionRecommendation{
				ID:         3,
				Type:       tc.actionType,
				Status:     adminplusdomain.ActionStatusApproved,
				SupplierID: 7,
				ReasonCode: string(tc.actionType),
			})
			updater := &fakeSupplierStatusUpdater{}
			svc := NewServiceWithDependencies(repo, updater)

			execution, err := svc.ExecuteApprovedRecommendation(context.Background(), 3, ExecuteInput{})

			require.NoError(t, err)
			require.Equal(t, adminplusdomain.ActionExecutionStatusSucceeded, execution.Status)
			require.Equal(t, 1, updater.calls)
			require.Equal(t, int64(7), updater.id)
			require.Equal(t, tc.wantRuntimeStatus, updater.input.RuntimeStatus)
			require.Equal(t, tc.wantHealthStatus, updater.input.HealthStatus)
			require.Equal(t, adminplusdomain.ActionStatusExecuted, repo.actions[3].Status)
			require.Equal(t, "supplier_status_update", execution.ResponsePayload["mode"])
		})
	}
}

func requireAction(t *testing.T, items []*adminplusdomain.ActionRecommendation, actionType adminplusdomain.ActionType, reason string) *adminplusdomain.ActionRecommendation {
	t.Helper()
	for _, item := range items {
		if item.Type == actionType && item.ReasonCode == reason {
			return item
		}
	}
	require.Failf(t, "missing action", "type=%s reason=%s items=%v", actionType, reason, items)
	return nil
}

type fakeActionRepository struct {
	actions    map[int64]*adminplusdomain.ActionRecommendation
	executions []*adminplusdomain.ActionExecution
}

func newFakeActionRepository(actions ...*adminplusdomain.ActionRecommendation) *fakeActionRepository {
	repo := &fakeActionRepository{actions: map[int64]*adminplusdomain.ActionRecommendation{}}
	for _, action := range actions {
		repo.actions[action.ID] = action
	}
	return repo
}

func (r *fakeActionRepository) CreateRecommendation(_ context.Context, action *adminplusdomain.ActionRecommendation) (*adminplusdomain.ActionRecommendation, error) {
	if action.ID == 0 {
		action.ID = int64(len(r.actions) + 1)
	}
	r.actions[action.ID] = action
	return action, nil
}

func (r *fakeActionRepository) GetRecommendation(_ context.Context, id int64) (*adminplusdomain.ActionRecommendation, error) {
	return r.actions[id], nil
}

func (r *fakeActionRepository) ListRecommendations(_ context.Context, filter RecommendationFilter) ([]*adminplusdomain.ActionRecommendation, error) {
	items := make([]*adminplusdomain.ActionRecommendation, 0, len(r.actions))
	for _, action := range r.actions {
		if filter.ID > 0 && action.ID != filter.ID {
			continue
		}
		if filter.SupplierID > 0 && action.SupplierID != filter.SupplierID {
			continue
		}
		if filter.Status != "" && action.Status != filter.Status {
			continue
		}
		if filter.Severity != "" && action.Severity != filter.Severity {
			continue
		}
		if filter.Type != "" && action.Type != filter.Type {
			continue
		}
		if filter.Signal != "" && !containsString(action.Signals, filter.Signal) {
			continue
		}
		items = append(items, action)
	}
	return items, nil
}

func (r *fakeActionRepository) UpdateRecommendationStatus(_ context.Context, id int64, status adminplusdomain.ActionStatus) (*adminplusdomain.ActionRecommendation, error) {
	action := r.actions[id]
	action.Status = status
	return action, nil
}

func (r *fakeActionRepository) CreateExecution(_ context.Context, execution *adminplusdomain.ActionExecution) (*adminplusdomain.ActionExecution, error) {
	if execution.ID == 0 {
		execution.ID = int64(len(r.executions) + 1)
	}
	r.executions = append(r.executions, execution)
	return execution, nil
}

func (r *fakeActionRepository) GetExecution(_ context.Context, id int64) (*adminplusdomain.ActionExecution, error) {
	for _, execution := range r.executions {
		if execution.ID == id {
			return execution, nil
		}
	}
	return nil, nil
}

func (r *fakeActionRepository) ListExecutions(_ context.Context, recommendationID int64, _ int) ([]*adminplusdomain.ActionExecution, error) {
	items := make([]*adminplusdomain.ActionExecution, 0, len(r.executions))
	for i := len(r.executions) - 1; i >= 0; i-- {
		execution := r.executions[i]
		if execution.RecommendationID == recommendationID {
			items = append(items, execution)
		}
	}
	return items, nil
}

func (r *fakeActionRepository) MarkExecutionIdempotencyReplayed(_ context.Context, recommendationID int64, idempotencyKeyHash string) (*adminplusdomain.ActionExecution, error) {
	for i := len(r.executions) - 1; i >= 0; i-- {
		execution := r.executions[i]
		if execution.RecommendationID == recommendationID && execution.IdempotencyKeyHash == idempotencyKeyHash {
			execution.IdempotencyReplayed = true
			return execution, nil
		}
	}
	return nil, nil
}

func (r *fakeActionRepository) MarkLatestExecutionIdempotencyReplayed(_ context.Context, actionType adminplusdomain.ActionType, idempotencyKeyHash string) (*adminplusdomain.ActionExecution, error) {
	for i := len(r.executions) - 1; i >= 0; i-- {
		execution := r.executions[i]
		if execution.ActionType == actionType && execution.IdempotencyKeyHash == idempotencyKeyHash {
			execution.IdempotencyReplayed = true
			return execution, nil
		}
	}
	return nil, nil
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

type fakeSupplierStatusUpdater struct {
	calls int
	id    int64
	input suppliersapp.UpdateSupplierStatusInput
}

func (u *fakeSupplierStatusUpdater) UpdateStatus(_ context.Context, id int64, in suppliersapp.UpdateSupplierStatusInput) (*adminplusdomain.Supplier, error) {
	u.calls++
	u.id = id
	u.input = in
	return &adminplusdomain.Supplier{
		ID:            id,
		RuntimeStatus: in.RuntimeStatus,
		HealthStatus:  in.HealthStatus,
	}, nil
}

type fakeActionNotificationDispatcher struct {
	actions []*adminplusdomain.ActionRecommendation
}

func (d *fakeActionNotificationDispatcher) DispatchActionRecommendation(_ context.Context, action *adminplusdomain.ActionRecommendation) error {
	d.actions = append(d.actions, action)
	return nil
}
