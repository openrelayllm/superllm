package adminplus

import (
	"context"
	"net/http"
	"testing"

	actionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/actions"
	sub2apiapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sub2api"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSub2APIHandlerApplyLocalAccountOpsActionRecordsLocalAccountScheduleDisableExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	routing := &sub2APIActionRoutingPort{}
	actionRepo := newSub2APIActionRepository(&adminplusdomain.ActionRecommendation{
		ID:         11,
		SupplierID: 7,
		Type:       adminplusdomain.ActionTypeLocalAccountScheduleDisable,
		Status:     adminplusdomain.ActionStatusApproved,
		Signals:    []string{"local_sub2api_account_id=42"},
	})
	handler := NewSub2APIHandlerWithAccountTestAndActions(
		sub2apiapp.NewService(nil, nil).WithRoutingPort(routing),
		nil,
		actionsapp.NewService(actionRepo),
	)
	router := gin.New()
	router.POST("/local-account-ops/apply", handler.ApplyLocalAccountOpsAction)

	rec := performJSONWithHeaders(t, router, http.MethodPost, "/local-account-ops/apply", `{
		"action": "set_schedulable",
		"account_ids": [42],
		"schedulable": false,
		"action_id": 11,
		"scheduler_run_id": "routing-capacity-watch-1",
		"scheduler_step_id": 99,
		"reason": "action_recommendation"
	}`, map[string]string{"Idempotency-Key": "disable-account-42-action-11"})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.True(t, routing.applyCalled)
	require.Equal(t, []int64{42}, routing.applyInput.AccountIDs)
	require.NotNil(t, routing.applyInput.Schedulable)
	require.False(t, *routing.applyInput.Schedulable)
	require.Equal(t, int64(11), routing.applyInput.ActionRecommendationID)
	require.Len(t, actionRepo.executions, 1)
	require.Equal(t, adminplusdomain.ActionExecutionStatusSucceeded, actionRepo.executions[0].Status)
	require.Equal(t, adminplusdomain.ActionTypeLocalAccountScheduleDisable, actionRepo.executions[0].ActionType)
	require.Equal(t, "routing-capacity-watch-1", actionRepo.executions[0].SchedulerRunID)
	require.Equal(t, int64(99), actionRepo.executions[0].SchedulerStepID)
	require.NotEmpty(t, actionRepo.executions[0].IdempotencyKeyHash)
	require.Equal(t, "routing-capacity-watch-1", actionRepo.executions[0].RequestPayload["scheduler_run_id"])
	require.Equal(t, int64(99), actionRepo.executions[0].RequestPayload["scheduler_step_id"])
	require.Equal(t, "local_account_ops_before", actionRepo.executions[0].BeforeSnapshot["mode"])
	require.Equal(t, "local_account_ops_after", actionRepo.executions[0].AfterSnapshot["mode"])
	require.Equal(t, adminplusdomain.ActionStatusExecuted, actionRepo.actions[11].Status)
}

func TestSub2APIHandlerApplyLocalAccountOpsActionRecordsManualExecutionAndReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	idempotencyRepo := newOperationsIdempotencyRepository()
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(idempotencyRepo, service.DefaultIdempotencyConfig()))
	t.Cleanup(func() {
		service.SetDefaultIdempotencyCoordinator(nil)
	})
	routing := &sub2APIActionRoutingPort{}
	actionRepo := newSub2APIActionRepository()
	handler := NewSub2APIHandlerWithAccountTestAndActions(
		sub2apiapp.NewService(nil, nil).WithRoutingPort(routing),
		nil,
		actionsapp.NewService(actionRepo),
	)
	router := gin.New()
	router.POST("/local-account-ops/apply", handler.ApplyLocalAccountOpsAction)
	body := `{
		"action": "add_to_groups",
		"account_ids": [42],
		"group_ids": [1001],
		"reason": "manual_ops_test"
	}`
	headers := map[string]string{"Idempotency-Key": "manual-add-account-42-group-1001"}

	first := performJSONWithHeaders(t, router, http.MethodPost, "/local-account-ops/apply", body, headers)
	second := performJSONWithHeaders(t, router, http.MethodPost, "/local-account-ops/apply", body, headers)

	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	require.Equal(t, 1, routing.applyCalls)
	require.Len(t, actionRepo.actions, 1)
	require.Equal(t, adminplusdomain.ActionTypeLocalAccountManualOps, actionRepo.actions[1].Type)
	require.Equal(t, adminplusdomain.ActionStatusExecuted, actionRepo.actions[1].Status)
	require.False(t, actionRepo.actions[1].RequiresApproval)
	require.Contains(t, actionRepo.actions[1].Signals, "local_sub2api_account_id=42")
	require.Contains(t, actionRepo.actions[1].Signals, "local_group_id=1001")
	require.Len(t, actionRepo.executions, 1)
	require.Equal(t, adminplusdomain.ActionTypeLocalAccountManualOps, actionRepo.executions[0].ActionType)
	require.Equal(t, adminplusdomain.ActionExecutionStatusSucceeded, actionRepo.executions[0].Status)
	require.Equal(t, "local_account_manual_ops_apply", actionRepo.executions[0].RequestPayload["mode"])
	require.True(t, actionRepo.executions[0].IdempotencyReplayed)
	require.Equal(t, "true", second.Header().Get("X-Idempotency-Replayed"))
}

func TestSub2APIHandlerApplyLocalAccountOpsActionMarksExecutionReplayedOnIdempotencyReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	idempotencyRepo := newOperationsIdempotencyRepository()
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(idempotencyRepo, service.DefaultIdempotencyConfig()))
	t.Cleanup(func() {
		service.SetDefaultIdempotencyCoordinator(nil)
	})
	routing := &sub2APIActionRoutingPort{}
	actionRepo := newSub2APIActionRepository(&adminplusdomain.ActionRecommendation{
		ID:         12,
		SupplierID: 7,
		Type:       adminplusdomain.ActionTypeLocalAccountScheduleDisable,
		Status:     adminplusdomain.ActionStatusApproved,
		Signals:    []string{"local_sub2api_account_id=42"},
	})
	handler := NewSub2APIHandlerWithAccountTestAndActions(
		sub2apiapp.NewService(nil, nil).WithRoutingPort(routing),
		nil,
		actionsapp.NewService(actionRepo),
	)
	router := gin.New()
	router.POST("/local-account-ops/apply", handler.ApplyLocalAccountOpsAction)
	body := `{
		"action": "set_schedulable",
		"account_ids": [42],
		"schedulable": false,
		"action_id": 12,
		"reason": "action_recommendation"
	}`
	headers := map[string]string{"Idempotency-Key": "disable-account-42-action-12"}

	first := performJSONWithHeaders(t, router, http.MethodPost, "/local-account-ops/apply", body, headers)
	second := performJSONWithHeaders(t, router, http.MethodPost, "/local-account-ops/apply", body, headers)

	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	require.Empty(t, first.Header().Get("X-Idempotency-Replayed"))
	require.Equal(t, "true", second.Header().Get("X-Idempotency-Replayed"))
	require.Equal(t, 1, routing.applyCalls)
	require.Len(t, actionRepo.executions, 1)
	require.True(t, actionRepo.executions[0].IdempotencyReplayed)
}

func TestActionHandlerExecuteRecommendationRecordsSchedulerSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	actionRepo := newSub2APIActionRepository(&adminplusdomain.ActionRecommendation{
		ID:         21,
		SupplierID: 7,
		Type:       adminplusdomain.ActionTypeReviewCredential,
		Status:     adminplusdomain.ActionStatusApproved,
		ReasonCode: "credential_invalid",
	})
	handler := NewActionHandler(actionsapp.NewService(actionRepo))
	router := gin.New()
	router.POST("/actions/recommendations/:id/execute", handler.ExecuteRecommendation)

	rec := performJSONWithHeaders(t, router, http.MethodPost, "/actions/recommendations/21/execute", `{
		"scheduler_run_id": "routing-capacity-watch-2",
		"scheduler_step_id": 101,
		"request_payload": {
			"source": "test"
		}
	}`, map[string]string{"Idempotency-Key": "generic-action-21"})

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Len(t, actionRepo.executions, 1)
	require.Equal(t, "routing-capacity-watch-2", actionRepo.executions[0].SchedulerRunID)
	require.Equal(t, int64(101), actionRepo.executions[0].SchedulerStepID)
	require.NotEmpty(t, actionRepo.executions[0].IdempotencyKeyHash)
	require.Equal(t, adminplusdomain.ActionStatusExecuted, actionRepo.actions[21].Status)
}

func TestSub2APIHandlerRetryActionExecutionReplaysFailedLocalAccountScheduleDisableSafely(t *testing.T) {
	gin.SetMode(gin.TestMode)
	routing := &sub2APIActionRoutingPort{}
	actionRepo := newSub2APIActionRepository(&adminplusdomain.ActionRecommendation{
		ID:         31,
		SupplierID: 7,
		Type:       adminplusdomain.ActionTypeLocalAccountScheduleDisable,
		Status:     adminplusdomain.ActionStatusApproved,
		Signals:    []string{"local_sub2api_account_id=42"},
	})
	actionRepo.executions = append(actionRepo.executions, &adminplusdomain.ActionExecution{
		ID:               41,
		RecommendationID: 31,
		ActionType:       adminplusdomain.ActionTypeLocalAccountScheduleDisable,
		SupplierID:       7,
		Status:           adminplusdomain.ActionExecutionStatusFailed,
		RequestPayload: map[string]any{
			"mode":              "local_account_ops_apply",
			"action":            string(adminplusdomain.LocalAccountOpsActionSetSchedulable),
			"account_ids":       []any{float64(42)},
			"schedulable":       false,
			"action_id":         float64(31),
			"scheduler_run_id":  "routing-capacity-watch-3",
			"scheduler_step_id": float64(201),
			"reason":            "action_recommendation",
		},
		ErrorMessage:    "LOCAL_ACCOUNT_STATE_DRIFT_PENDING",
		SchedulerRunID:  "routing-capacity-watch-3",
		SchedulerStepID: 201,
	})
	handler := NewSub2APIHandlerWithAccountTestAndActions(
		sub2apiapp.NewService(nil, nil).WithRoutingPort(routing),
		nil,
		actionsapp.NewService(actionRepo),
	)
	router := gin.New()
	router.POST("/actions/recommendations/:id/executions/:executionID/retry", handler.RetryActionExecution)

	rec := performJSONWithHeaders(t, router, http.MethodPost, "/actions/recommendations/31/executions/41/retry", `{}`, map[string]string{
		"Idempotency-Key": "retry-disable-account-42-action-31",
	})

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.True(t, routing.applyCalled)
	require.Equal(t, []int64{42}, routing.applyInput.AccountIDs)
	require.NotNil(t, routing.applyInput.Schedulable)
	require.False(t, *routing.applyInput.Schedulable)
	require.Equal(t, int64(31), routing.applyInput.ActionRecommendationID)
	require.Len(t, actionRepo.executions, 2)
	retried := actionRepo.executions[1]
	require.Equal(t, adminplusdomain.ActionExecutionStatusSucceeded, retried.Status)
	require.Equal(t, int64(41), retried.RequestPayload["retry_source_execution_id"])
	require.Equal(t, "routing-capacity-watch-3", retried.SchedulerRunID)
	require.Equal(t, int64(201), retried.SchedulerStepID)
	require.NotEmpty(t, retried.IdempotencyKeyHash)
	require.Equal(t, adminplusdomain.ActionStatusExecuted, actionRepo.actions[31].Status)
}

func TestSub2APIHandlerRollbackActionExecutionRestoresLocalAccountScheduleDisable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	routing := &sub2APIActionRoutingPort{}
	actionRepo := newSub2APIActionRepository(&adminplusdomain.ActionRecommendation{
		ID:         51,
		SupplierID: 7,
		Type:       adminplusdomain.ActionTypeLocalAccountScheduleDisable,
		Status:     adminplusdomain.ActionStatusExecuted,
		Signals:    []string{"local_sub2api_account_id=42"},
	})
	actionRepo.executions = append(actionRepo.executions, &adminplusdomain.ActionExecution{
		ID:               61,
		RecommendationID: 51,
		ActionType:       adminplusdomain.ActionTypeLocalAccountScheduleDisable,
		SupplierID:       7,
		Status:           adminplusdomain.ActionExecutionStatusSucceeded,
		RequestPayload: map[string]any{
			"mode":        "local_account_ops_apply",
			"action":      string(adminplusdomain.LocalAccountOpsActionSetSchedulable),
			"account_ids": []any{float64(42)},
			"schedulable": false,
			"action_id":   float64(51),
			"reason":      "action_recommendation",
		},
	})
	handler := NewSub2APIHandlerWithAccountTestAndActions(
		sub2apiapp.NewService(nil, nil).WithRoutingPort(routing),
		nil,
		actionsapp.NewService(actionRepo),
	)
	router := gin.New()
	router.POST("/actions/recommendations/:id/executions/:executionID/rollback", handler.RollbackActionExecution)

	rec := performJSONWithHeaders(t, router, http.MethodPost, "/actions/recommendations/51/executions/61/rollback", `{}`, map[string]string{
		"Idempotency-Key": "rollback-disable-account-42-action-51",
	})

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.True(t, routing.applyCalled)
	require.Equal(t, adminplusdomain.LocalAccountOpsActionSetSchedulable, routing.applyInput.Action)
	require.Equal(t, []int64{42}, routing.applyInput.AccountIDs)
	require.NotNil(t, routing.applyInput.Schedulable)
	require.True(t, *routing.applyInput.Schedulable)
	require.Len(t, actionRepo.executions, 2)
	rollback := actionRepo.executions[1]
	require.Equal(t, adminplusdomain.ActionExecutionStatusSucceeded, rollback.Status)
	require.Equal(t, adminplusdomain.ActionTypeLocalAccountScheduleDisable, rollback.ActionType)
	require.Equal(t, "local_account_schedule_disable_rollback", rollback.RequestPayload["mode"])
	require.Equal(t, int64(61), rollback.RequestPayload["rollback_source_execution_id"])
}

func TestSub2APIHandlerRollbackActionExecutionRemovesRoutingRefillAccountFromGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	routing := &sub2APIActionRoutingPort{}
	actionRepo := newSub2APIActionRepository(&adminplusdomain.ActionRecommendation{
		ID:         71,
		SupplierID: 7,
		Type:       adminplusdomain.ActionTypeRoutingRefill,
		Status:     adminplusdomain.ActionStatusExecuted,
		Signals:    []string{"local_group_id=1001"},
	})
	actionRepo.executions = append(actionRepo.executions, &adminplusdomain.ActionExecution{
		ID:               81,
		RecommendationID: 71,
		ActionType:       adminplusdomain.ActionTypeRoutingRefill,
		SupplierID:       7,
		Status:           adminplusdomain.ActionExecutionStatusSucceeded,
		RequestPayload: map[string]any{
			"mode":           "routing_refill_apply",
			"local_group_id": float64(1001),
			"platform":       "openai",
			"dry_run":        false,
			"action_id":      float64(71),
			"reason":         "action_recommendation",
		},
		ResponsePayload: map[string]any{
			"mode":       "routing_refill_apply",
			"account_id": float64(42),
		},
	})
	handler := NewSub2APIHandlerWithAccountTestAndActions(
		sub2apiapp.NewService(nil, nil).WithRoutingPort(routing),
		nil,
		actionsapp.NewService(actionRepo),
	)
	router := gin.New()
	router.POST("/actions/recommendations/:id/executions/:executionID/rollback", handler.RollbackActionExecution)

	rec := performJSONWithHeaders(t, router, http.MethodPost, "/actions/recommendations/71/executions/81/rollback", `{}`, map[string]string{
		"Idempotency-Key": "rollback-routing-refill-account-42-group-1001",
	})

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.True(t, routing.applyCalled)
	require.Equal(t, adminplusdomain.LocalAccountOpsActionRemoveFromGroups, routing.applyInput.Action)
	require.Equal(t, []int64{42}, routing.applyInput.AccountIDs)
	require.Equal(t, []int64{1001}, routing.applyInput.GroupIDs)
	require.Len(t, actionRepo.executions, 2)
	rollback := actionRepo.executions[1]
	require.Equal(t, adminplusdomain.ActionExecutionStatusSucceeded, rollback.Status)
	require.Equal(t, adminplusdomain.ActionTypeRoutingRefill, rollback.ActionType)
	require.Equal(t, "routing_refill_rollback", rollback.RequestPayload["mode"])
	require.Equal(t, int64(81), rollback.RequestPayload["rollback_source_execution_id"])
}

type sub2APIActionRoutingPort struct {
	applyCalled bool
	applyCalls  int
	applyInput  sub2apiapp.LocalAccountOpsActionInput
	applyResult *adminplusdomain.LocalAccountOpsActionResult
}

func (r *sub2APIActionRoutingPort) GetGroupAvailability(_ context.Context, groupID int64, platform string) (*sub2apiapp.RoutingGroupAvailability, error) {
	return &sub2apiapp.RoutingGroupAvailability{GroupID: groupID, Platform: platform}, nil
}

func (r *sub2APIActionRoutingPort) GetAccount(_ context.Context, accountID int64) (*sub2apiapp.Sub2APIAccountSnapshot, error) {
	return &sub2apiapp.Sub2APIAccountSnapshot{AccountID: accountID, Schedulable: true}, nil
}

func (r *sub2APIActionRoutingPort) EnsureAccountInGroup(_ context.Context, accountID int64, groupID int64) (*sub2apiapp.Sub2APIAccountSnapshot, error) {
	return &sub2apiapp.Sub2APIAccountSnapshot{AccountID: accountID, GroupIDs: []int64{groupID}}, nil
}

func (r *sub2APIActionRoutingPort) SetAccountSchedulable(_ context.Context, accountID int64, schedulable bool, _ string) (*sub2apiapp.Sub2APIAccountSnapshot, error) {
	return &sub2apiapp.Sub2APIAccountSnapshot{AccountID: accountID, Schedulable: schedulable}, nil
}

func (r *sub2APIActionRoutingPort) PreviewLocalAccountOpsAction(_ context.Context, input sub2apiapp.LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	return &adminplusdomain.LocalAccountOpsActionResult{Action: input.Action, DryRun: true, AccountIDs: input.AccountIDs}, nil
}

func (r *sub2APIActionRoutingPort) ApplyLocalAccountOpsAction(_ context.Context, input sub2apiapp.LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	r.applyCalled = true
	r.applyCalls++
	r.applyInput = input
	if r.applyResult != nil {
		return r.applyResult, nil
	}
	return &adminplusdomain.LocalAccountOpsActionResult{
		Action:          input.Action,
		AccountIDs:      input.AccountIDs,
		UpdatedAccounts: int64(len(input.AccountIDs)),
	}, nil
}

func (r *sub2APIActionRoutingPort) ResolveLocalAccountState(_ context.Context, input sub2apiapp.LocalAccountStateResolutionInput) (*adminplusdomain.LocalAccountStateResolutionResult, error) {
	return &adminplusdomain.LocalAccountStateResolutionResult{Action: input.Action, AccountIDs: input.AccountIDs}, nil
}

type sub2APIActionRepository struct {
	actions    map[int64]*adminplusdomain.ActionRecommendation
	executions []*adminplusdomain.ActionExecution
}

func newSub2APIActionRepository(actions ...*adminplusdomain.ActionRecommendation) *sub2APIActionRepository {
	repo := &sub2APIActionRepository{actions: map[int64]*adminplusdomain.ActionRecommendation{}}
	for _, action := range actions {
		repo.actions[action.ID] = action
	}
	return repo
}

func (r *sub2APIActionRepository) CreateRecommendation(_ context.Context, action *adminplusdomain.ActionRecommendation) (*adminplusdomain.ActionRecommendation, error) {
	if action.ID == 0 {
		action.ID = int64(len(r.actions) + 1)
	}
	r.actions[action.ID] = action
	return action, nil
}

func (r *sub2APIActionRepository) GetRecommendation(_ context.Context, id int64) (*adminplusdomain.ActionRecommendation, error) {
	return r.actions[id], nil
}

func (r *sub2APIActionRepository) ListRecommendations(_ context.Context, _ actionsapp.RecommendationFilter) ([]*adminplusdomain.ActionRecommendation, error) {
	items := make([]*adminplusdomain.ActionRecommendation, 0, len(r.actions))
	for _, action := range r.actions {
		items = append(items, action)
	}
	return items, nil
}

func (r *sub2APIActionRepository) UpdateRecommendationStatus(_ context.Context, id int64, status adminplusdomain.ActionStatus) (*adminplusdomain.ActionRecommendation, error) {
	action := r.actions[id]
	action.Status = status
	return action, nil
}

func (r *sub2APIActionRepository) CreateExecution(_ context.Context, execution *adminplusdomain.ActionExecution) (*adminplusdomain.ActionExecution, error) {
	if execution.ID == 0 {
		execution.ID = int64(len(r.executions) + 1)
	}
	r.executions = append(r.executions, execution)
	return execution, nil
}

func (r *sub2APIActionRepository) GetExecution(_ context.Context, id int64) (*adminplusdomain.ActionExecution, error) {
	for _, execution := range r.executions {
		if execution.ID == id {
			return execution, nil
		}
	}
	return nil, nil
}

func (r *sub2APIActionRepository) ListExecutions(_ context.Context, recommendationID int64, _ int) ([]*adminplusdomain.ActionExecution, error) {
	items := make([]*adminplusdomain.ActionExecution, 0, len(r.executions))
	for i := len(r.executions) - 1; i >= 0; i-- {
		execution := r.executions[i]
		if execution.RecommendationID == recommendationID {
			items = append(items, execution)
		}
	}
	return items, nil
}

func (r *sub2APIActionRepository) MarkExecutionIdempotencyReplayed(_ context.Context, recommendationID int64, idempotencyKeyHash string) (*adminplusdomain.ActionExecution, error) {
	for i := len(r.executions) - 1; i >= 0; i-- {
		execution := r.executions[i]
		if execution.RecommendationID == recommendationID && execution.IdempotencyKeyHash == idempotencyKeyHash {
			execution.IdempotencyReplayed = true
			return execution, nil
		}
	}
	return nil, nil
}

func (r *sub2APIActionRepository) MarkLatestExecutionIdempotencyReplayed(_ context.Context, actionType adminplusdomain.ActionType, idempotencyKeyHash string) (*adminplusdomain.ActionExecution, error) {
	for i := len(r.executions) - 1; i >= 0; i-- {
		execution := r.executions[i]
		if execution.ActionType == actionType && execution.IdempotencyKeyHash == idempotencyKeyHash {
			execution.IdempotencyReplayed = true
			return execution, nil
		}
	}
	return nil, nil
}
