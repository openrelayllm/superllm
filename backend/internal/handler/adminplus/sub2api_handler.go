package adminplus

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	actionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/actions"
	sub2apiapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sub2api"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type Sub2APIHandler struct {
	service            *sub2apiapp.Service
	accountTestService *service.AccountTestService
	actions            *actionsapp.Service
}

func NewSub2APIHandler(service *sub2apiapp.Service) *Sub2APIHandler {
	return &Sub2APIHandler{service: service}
}

func NewSub2APIHandlerWithAccountTest(service *sub2apiapp.Service, accountTestService *service.AccountTestService) *Sub2APIHandler {
	return &Sub2APIHandler{
		service:            service,
		accountTestService: accountTestService,
	}
}

func NewSub2APIHandlerWithAccountTestAndActions(service *sub2apiapp.Service, accountTestService *service.AccountTestService, actions *actionsapp.Service) *Sub2APIHandler {
	return &Sub2APIHandler{
		service:            service,
		accountTestService: accountTestService,
		actions:            actions,
	}
}

type testLocalAccountRequest struct {
	ModelID string `json:"model_id"`
	Prompt  string `json:"prompt"`
	Mode    string `json:"mode"`
}

type localAccountOpsActionRequest struct {
	Action          string  `json:"action"`
	AccountIDs      []int64 `json:"account_ids"`
	GroupIDs        []int64 `json:"group_ids"`
	Schedulable     *bool   `json:"schedulable"`
	AllowEmptyPool  bool    `json:"allow_empty_pool"`
	ActionID        int64   `json:"action_id"`
	SchedulerRunID  string  `json:"scheduler_run_id"`
	SchedulerStepID int64   `json:"scheduler_step_id"`
	Reason          string  `json:"reason"`
}

type localAccountStateSyncRequest struct {
	AccountIDs []int64 `json:"account_ids"`
	Limit      int     `json:"limit"`
}

type localAccountStateResolutionRequest struct {
	AccountIDs []int64 `json:"account_ids"`
}

type routingRefillRequest struct {
	LocalGroupID      int64   `json:"local_group_id"`
	Platform          string  `json:"platform"`
	ModelScope        string  `json:"model_scope"`
	MaxRateMultiplier float64 `json:"max_rate_multiplier"`
	Limit             int     `json:"limit"`
	DryRun            bool    `json:"dry_run"`
	ActionID          int64   `json:"action_id"`
	SchedulerRunID    string  `json:"scheduler_run_id"`
	SchedulerStepID   int64   `json:"scheduler_step_id"`
	Reason            string  `json:"reason"`
	TriggerType       string  `json:"trigger_type"`
	CooldownSeconds   int     `json:"cooldown_seconds"`
	ConfirmWindowSecs int     `json:"confirm_window_seconds"`
}

type routingSensitiveFailureDetailRequest struct {
	LocalGroupID int64    `json:"local_group_id"`
	Reason       string   `json:"reason"`
	Fields       []string `json:"fields"`
}

func (h *Sub2APIHandler) ListLocalAccountModels(c *gin.Context) {
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	if h.accountTestService == nil {
		response.InternalError(c, "account test service is not configured")
		return
	}

	models, err := h.accountTestService.GetAvailableModels(c.Request.Context(), accountID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, models)
}

func (h *Sub2APIHandler) TestLocalAccount(c *gin.Context) {
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	if h.accountTestService == nil {
		response.InternalError(c, "account test service is not configured")
		return
	}

	var req testLocalAccountRequest
	_ = c.ShouldBindJSON(&req)
	_ = h.accountTestService.TestAccountConnection(c, accountID, req.ModelID, req.Prompt, req.Mode)
}

func (h *Sub2APIHandler) ListLocalUsageLines(c *gin.Context) {
	page := parsePagination(c)
	filter, ok := parseUsageFilter(c)
	if !ok {
		return
	}
	filter.Limit = fetchLimitForPagination(page)
	items, err := h.service.ListLocalUsageLines(c.Request.Context(), filter)
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *Sub2APIHandler) ListLocalUsageSummaries(c *gin.Context) {
	page := parsePagination(c)
	filter, ok := parseUsageFilter(c)
	if !ok {
		return
	}
	filter.Limit = fetchLimitForPagination(page)
	items, err := h.service.ListLocalUsageSummaries(c.Request.Context(), filter)
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *Sub2APIHandler) ListLocalAccountUsageSummaries(c *gin.Context) {
	page := parsePagination(c)
	filter, ok := parseUsageFilter(c)
	if !ok {
		return
	}
	filter.Limit = fetchLimitForPagination(page)
	items, err := h.service.ListLocalAccountUsageSummaries(c.Request.Context(), filter)
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *Sub2APIHandler) ListLocalAccountOps(c *gin.Context) {
	page := parsePagination(c)
	maxRateMultiplier, ok := parseFloat64Query(c, "max_rate_multiplier")
	if !ok {
		return
	}
	items, err := h.service.ListLocalAccountOps(c.Request.Context(), sub2apiapp.LocalAccountOpsFilter{
		Query:              c.Query("q"),
		SupplierID:         parseInt64Query(c, "supplier_id"),
		LocalGroupID:       parseInt64Query(c, "local_group_id"),
		SupplierGroupID:    parseInt64Query(c, "supplier_group_id"),
		MaxRateMultiplier:  maxRateMultiplier,
		ModelScope:         c.Query("model_scope"),
		BalanceStatus:      c.Query("balance_status"),
		ChannelCheckStatus: c.Query("channel_check_status"),
		Schedulable:        parseOptionalBoolQuery(c, "schedulable"),
		Limit:              fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *Sub2APIHandler) ListLocalGroups(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListLocalGroups(c.Request.Context(), fetchLimitForPagination(page))
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *Sub2APIHandler) PreviewLocalAccountOpsAction(c *gin.Context) {
	var req localAccountOpsActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	result, err := h.service.PreviewLocalAccountOpsAction(c.Request.Context(), localAccountOpsInputFromRequest(req, currentAdminUserID(c)))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *Sub2APIHandler) ApplyLocalAccountOpsAction(c *gin.Context) {
	var req localAccountOpsActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	input := localAccountOpsInputFromRequest(req, currentAdminUserID(c))
	if h.actions == nil {
		response.InternalError(c, "action service is not configured")
		return
	}
	idempotencyKeyHash := adminPlusIdempotencyKeyHash(c)
	replayHook := h.actionExecutionReplayHook(req.ActionID, idempotencyKeyHash)
	if req.ActionID <= 0 {
		replayHook = h.latestActionExecutionReplayHook(adminplusdomain.ActionTypeLocalAccountManualOps, idempotencyKeyHash)
	}
	executeAdminPlusIdempotentJSONWithReplay(c, "admin-plus.local-account-ops.apply", input, 0, http.StatusOK, func(ctx context.Context) (any, error) {
		if req.ActionID > 0 {
			return h.applyActionBackedLocalAccountOps(ctx, req.ActionID, currentAdminUserID(c), idempotencyKeyHash, req.SchedulerRunID, req.SchedulerStepID, input, 0)
		}
		return h.applyManualLocalAccountOps(ctx, currentAdminUserID(c), idempotencyKeyHash, input)
	}, replayHook)
}

func (h *Sub2APIHandler) RefillLocalGroup(c *gin.Context) {
	var req routingRefillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	input := sub2apiapp.RoutingRefillInput{
		LocalGroupID:           req.LocalGroupID,
		Platform:               req.Platform,
		ModelScope:             req.ModelScope,
		MaxRateMultiplier:      req.MaxRateMultiplier,
		Limit:                  req.Limit,
		DryRun:                 req.DryRun,
		ActionRecommendationID: req.ActionID,
		Reason:                 req.Reason,
		TriggerType:            req.TriggerType,
		RequestedBy:            currentAdminUserID(c),
		CooldownSeconds:        req.CooldownSeconds,
		ConfirmWindowSecs:      req.ConfirmWindowSecs,
	}
	if input.DryRun {
		result, err := h.service.RefillLocalGroup(c.Request.Context(), input)
		if response.ErrorFrom(c, err) {
			return
		}
		response.Success(c, result)
		return
	}
	if req.ActionID > 0 {
		if h.actions == nil {
			response.InternalError(c, "action service is not configured")
			return
		}
	}
	idempotencyKeyHash := adminPlusIdempotencyKeyHash(c)
	executeAdminPlusIdempotentJSONWithReplay(c, "admin-plus.routing.refill-local-group", input, 0, http.StatusOK, func(ctx context.Context) (any, error) {
		if req.ActionID > 0 {
			return h.applyActionBackedRoutingRefill(ctx, req.ActionID, currentAdminUserID(c), idempotencyKeyHash, req.SchedulerRunID, req.SchedulerStepID, input, 0)
		}
		return h.service.RefillLocalGroup(ctx, input)
	}, h.actionExecutionReplayHook(req.ActionID, idempotencyKeyHash))
}

func (h *Sub2APIHandler) RetryActionExecution(c *gin.Context) {
	recommendationID, ok := parseActionRecommendationID(c)
	if !ok {
		return
	}
	executionID, ok := parseActionExecutionID(c)
	if !ok {
		return
	}
	if h.actions == nil {
		response.InternalError(c, "action service is not configured")
		return
	}
	if h.service == nil {
		response.InternalError(c, "sub2api service is not configured")
		return
	}
	idempotencyKeyHash := adminPlusIdempotencyKeyHash(c)
	executeAdminPlusIdempotentJSONWithReplay(c, "admin-plus.actions.execution.retry", map[string]any{
		"recommendation_id": recommendationID,
		"execution_id":      executionID,
	}, 0, http.StatusCreated, func(ctx context.Context) (any, error) {
		execution, err := h.actions.GetRecommendationExecution(ctx, recommendationID, executionID)
		if err != nil {
			return nil, err
		}
		if execution.Status != adminplusdomain.ActionExecutionStatusFailed {
			return nil, infraerrors.New(http.StatusConflict, "ACTION_EXECUTION_RETRY_STATUS_UNSUPPORTED", "only failed action executions can be retried")
		}
		switch execution.ActionType {
		case adminplusdomain.ActionTypeRoutingRefill:
			return h.retryRoutingRefillAction(ctx, recommendationID, execution, currentAdminUserID(c), idempotencyKeyHash)
		case adminplusdomain.ActionTypeLocalAccountScheduleDisable:
			return h.retryLocalAccountScheduleDisableAction(ctx, recommendationID, execution, currentAdminUserID(c), idempotencyKeyHash)
		default:
			return nil, infraerrors.New(http.StatusConflict, "ACTION_EXECUTION_RETRY_ACTION_UNSUPPORTED", "action execution type is not supported by safe retry")
		}
	}, h.actionExecutionReplayHook(recommendationID, idempotencyKeyHash))
}

func (h *Sub2APIHandler) RollbackActionExecution(c *gin.Context) {
	recommendationID, ok := parseActionRecommendationID(c)
	if !ok {
		return
	}
	executionID, ok := parseActionExecutionID(c)
	if !ok {
		return
	}
	if h.actions == nil {
		response.InternalError(c, "action service is not configured")
		return
	}
	if h.service == nil {
		response.InternalError(c, "sub2api service is not configured")
		return
	}
	idempotencyKeyHash := adminPlusIdempotencyKeyHash(c)
	executeAdminPlusIdempotentJSONWithReplay(c, "admin-plus.actions.execution.rollback", map[string]any{
		"recommendation_id": recommendationID,
		"execution_id":      executionID,
	}, 0, http.StatusCreated, func(ctx context.Context) (any, error) {
		execution, err := h.actions.GetRecommendationExecution(ctx, recommendationID, executionID)
		if err != nil {
			return nil, err
		}
		if execution.Status != adminplusdomain.ActionExecutionStatusSucceeded {
			return nil, infraerrors.New(http.StatusConflict, "ACTION_EXECUTION_ROLLBACK_STATUS_UNSUPPORTED", "only succeeded action executions can be rolled back")
		}
		if err := h.ensureExecutionNotRolledBack(ctx, recommendationID, execution.ID); err != nil {
			return nil, err
		}
		switch execution.ActionType {
		case adminplusdomain.ActionTypeRoutingRefill:
			return h.rollbackRoutingRefillAction(ctx, recommendationID, execution, currentAdminUserID(c), idempotencyKeyHash)
		case adminplusdomain.ActionTypeLocalAccountScheduleDisable:
			return h.rollbackLocalAccountScheduleDisableAction(ctx, recommendationID, execution, currentAdminUserID(c), idempotencyKeyHash)
		default:
			return nil, infraerrors.New(http.StatusConflict, "ACTION_EXECUTION_ROLLBACK_ACTION_UNSUPPORTED", "action execution type is not supported by safe rollback")
		}
	}, h.actionExecutionReplayHook(recommendationID, idempotencyKeyHash))
}

func (h *Sub2APIHandler) actionExecutionReplayHook(recommendationID int64, idempotencyKeyHash string) func(context.Context) error {
	if h == nil || h.actions == nil || recommendationID <= 0 {
		return nil
	}
	idempotencyKeyHash = strings.TrimSpace(idempotencyKeyHash)
	if idempotencyKeyHash == "" {
		return nil
	}
	return func(ctx context.Context) error {
		_, err := h.actions.MarkExecutionIdempotencyReplayed(ctx, recommendationID, idempotencyKeyHash)
		return err
	}
}

func (h *Sub2APIHandler) latestActionExecutionReplayHook(actionType adminplusdomain.ActionType, idempotencyKeyHash string) func(context.Context) error {
	if h == nil || h.actions == nil || !actionType.Valid() {
		return nil
	}
	idempotencyKeyHash = strings.TrimSpace(idempotencyKeyHash)
	if idempotencyKeyHash == "" {
		return nil
	}
	return func(ctx context.Context) error {
		_, err := h.actions.MarkLatestExecutionIdempotencyReplayed(ctx, actionType, idempotencyKeyHash)
		return err
	}
}

func (h *Sub2APIHandler) retryLocalAccountScheduleDisableAction(ctx context.Context, recommendationID int64, execution *adminplusdomain.ActionExecution, operatorID int64, idempotencyKeyHash string) (*adminplusdomain.ActionExecution, error) {
	var req localAccountOpsActionRequest
	if err := decodeActionExecutionRequestPayload(execution.RequestPayload, &req); err != nil {
		return nil, err
	}
	req.ActionID = recommendationID
	if req.SchedulerRunID == "" {
		req.SchedulerRunID = execution.SchedulerRunID
	}
	if req.SchedulerStepID <= 0 {
		req.SchedulerStepID = execution.SchedulerStepID
	}
	input := localAccountOpsInputFromRequest(req, operatorID)
	input.ActionRecommendationID = recommendationID
	_, err := h.applyActionBackedLocalAccountOps(ctx, recommendationID, operatorID, idempotencyKeyHash, req.SchedulerRunID, req.SchedulerStepID, input, execution.ID)
	if err != nil {
		return nil, err
	}
	return latestCreatedExecution(h.actions, ctx, recommendationID)
}

func (h *Sub2APIHandler) retryRoutingRefillAction(ctx context.Context, recommendationID int64, execution *adminplusdomain.ActionExecution, operatorID int64, idempotencyKeyHash string) (*adminplusdomain.ActionExecution, error) {
	var req routingRefillRequest
	if err := decodeActionExecutionRequestPayload(execution.RequestPayload, &req); err != nil {
		return nil, err
	}
	if req.DryRun {
		return nil, infraerrors.New(http.StatusConflict, "ACTION_EXECUTION_RETRY_PAYLOAD_UNSUPPORTED", "dry-run action execution payload cannot be retried as an apply")
	}
	req.ActionID = recommendationID
	if req.SchedulerRunID == "" {
		req.SchedulerRunID = execution.SchedulerRunID
	}
	if req.SchedulerStepID <= 0 {
		req.SchedulerStepID = execution.SchedulerStepID
	}
	input := sub2apiapp.RoutingRefillInput{
		LocalGroupID:           req.LocalGroupID,
		Platform:               req.Platform,
		ModelScope:             req.ModelScope,
		MaxRateMultiplier:      req.MaxRateMultiplier,
		Limit:                  req.Limit,
		DryRun:                 false,
		ActionRecommendationID: recommendationID,
		Reason:                 firstNonEmpty(req.Reason, "retry_action_execution"),
		TriggerType:            req.TriggerType,
		RequestedBy:            operatorID,
		CooldownSeconds:        req.CooldownSeconds,
		ConfirmWindowSecs:      req.ConfirmWindowSecs,
	}
	_, err := h.applyActionBackedRoutingRefill(ctx, recommendationID, operatorID, idempotencyKeyHash, req.SchedulerRunID, req.SchedulerStepID, input, execution.ID)
	if err != nil {
		return nil, err
	}
	return latestCreatedExecution(h.actions, ctx, recommendationID)
}

func (h *Sub2APIHandler) rollbackLocalAccountScheduleDisableAction(ctx context.Context, recommendationID int64, execution *adminplusdomain.ActionExecution, operatorID int64, idempotencyKeyHash string) (*adminplusdomain.ActionExecution, error) {
	action, err := h.actions.GetRecommendation(ctx, recommendationID)
	if err != nil {
		return nil, err
	}
	if action.Type != adminplusdomain.ActionTypeLocalAccountScheduleDisable {
		return nil, infraerrors.New(http.StatusConflict, "ACTION_RECOMMENDATION_TYPE_MISMATCH", "action recommendation type does not match the requested workflow")
	}
	var sourceReq localAccountOpsActionRequest
	if err := decodeActionExecutionRequestPayload(execution.RequestPayload, &sourceReq); err != nil {
		return nil, err
	}
	sourceInput := localAccountOpsInputFromRequest(sourceReq, operatorID)
	if err := validateLocalAccountScheduleDisableAction(action, sourceInput); err != nil {
		return nil, err
	}
	schedulable := true
	rollbackInput := sub2apiapp.LocalAccountOpsActionInput{
		Action:                 adminplusdomain.LocalAccountOpsActionSetSchedulable,
		AccountIDs:             sourceInput.AccountIDs,
		Schedulable:            &schedulable,
		RequestedBy:            operatorID,
		Reason:                 "rollback_action_execution",
		ActionRecommendationID: recommendationID,
	}
	_, err = h.applyRollbackLocalAccountOps(ctx, recommendationID, operatorID, idempotencyKeyHash, execution.SchedulerRunID, execution.SchedulerStepID, rollbackInput, execution.ID, "local_account_schedule_disable_rollback")
	if err != nil {
		return nil, err
	}
	return latestCreatedExecution(h.actions, ctx, recommendationID)
}

func (h *Sub2APIHandler) rollbackRoutingRefillAction(ctx context.Context, recommendationID int64, execution *adminplusdomain.ActionExecution, operatorID int64, idempotencyKeyHash string) (*adminplusdomain.ActionExecution, error) {
	action, err := h.actions.GetRecommendation(ctx, recommendationID)
	if err != nil {
		return nil, err
	}
	var sourceReq routingRefillRequest
	if err := decodeActionExecutionRequestPayload(execution.RequestPayload, &sourceReq); err != nil {
		return nil, err
	}
	if sourceReq.DryRun {
		return nil, infraerrors.New(http.StatusConflict, "ACTION_EXECUTION_ROLLBACK_PAYLOAD_UNSUPPORTED", "dry-run action execution payload cannot be rolled back")
	}
	if action.Type != adminplusdomain.ActionTypeRoutingRefill {
		return nil, infraerrors.New(http.StatusConflict, "ACTION_RECOMMENDATION_TYPE_MISMATCH", "action recommendation type does not match the requested workflow")
	}
	if actionLocalGroupID := int64Signal(action.Signals, "local_group_id"); actionLocalGroupID > 0 && actionLocalGroupID != sourceReq.LocalGroupID {
		return nil, infraerrors.New(http.StatusConflict, "ACTION_ROUTING_REFILL_GROUP_MISMATCH", "action recommendation local group does not match request")
	}
	accountID := payloadInt64(execution.ResponsePayload, "account_id")
	if accountID <= 0 {
		accountID = payloadInt64(execution.ResponsePayload, "candidate_local_account_id")
	}
	if accountID <= 0 {
		return nil, infraerrors.New(http.StatusConflict, "ACTION_ROUTING_REFILL_ROLLBACK_ACCOUNT_MISSING", "routing refill execution does not include the account to remove")
	}
	rollbackInput := sub2apiapp.LocalAccountOpsActionInput{
		Action:                 adminplusdomain.LocalAccountOpsActionRemoveFromGroups,
		AccountIDs:             []int64{accountID},
		GroupIDs:               []int64{sourceReq.LocalGroupID},
		RequestedBy:            operatorID,
		Reason:                 "rollback_action_execution",
		ActionRecommendationID: recommendationID,
	}
	_, err = h.applyRollbackLocalAccountOps(ctx, recommendationID, operatorID, idempotencyKeyHash, execution.SchedulerRunID, execution.SchedulerStepID, rollbackInput, execution.ID, "routing_refill_rollback")
	if err != nil {
		return nil, err
	}
	return latestCreatedExecution(h.actions, ctx, recommendationID)
}

func (h *Sub2APIHandler) applyActionBackedLocalAccountOps(ctx context.Context, actionID int64, operatorID int64, idempotencyKeyHash string, schedulerRunID string, schedulerStepID int64, input sub2apiapp.LocalAccountOpsActionInput, retrySourceExecutionID int64) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	if h.actions == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ACTION_SERVICE_NOT_CONFIGURED", "action service is not configured")
	}
	action, err := h.actions.RequireApprovedRecommendation(ctx, actionID, adminplusdomain.ActionTypeLocalAccountScheduleDisable)
	if err != nil {
		return nil, err
	}
	if err := validateLocalAccountScheduleDisableAction(action, input); err != nil {
		return nil, err
	}
	result, err := h.service.ApplyLocalAccountOpsAction(ctx, input)
	status := adminplusdomain.ActionExecutionStatusSucceeded
	errorMessage := ""
	if err != nil {
		status = adminplusdomain.ActionExecutionStatusFailed
		errorMessage = err.Error()
	} else if result != nil && result.Blocked {
		status = adminplusdomain.ActionExecutionStatusFailed
		errorMessage = result.BlockedReason
	}
	requestPayload := localAccountOpsActionRequestPayload(input, schedulerRunID, schedulerStepID)
	responsePayload := localAccountOpsActionResponsePayload(result)
	addRetrySourcePayload(requestPayload, retrySourceExecutionID)
	addRetrySourcePayload(responsePayload, retrySourceExecutionID)
	_, recordErr := h.actions.RecordExternalExecution(ctx, actionID, actionsapp.ExecuteInput{
		OperatorUserID:     operatorID,
		SchedulerRunID:     schedulerRunID,
		SchedulerStepID:    schedulerStepID,
		IdempotencyKeyHash: idempotencyKeyHash,
		BeforeSnapshot:     localAccountOpsBeforeSnapshot(result),
		AfterSnapshot:      localAccountOpsAfterSnapshot(result),
		RequestPayload:     requestPayload,
	}, status, responsePayload, errorMessage)
	if err == nil && recordErr != nil {
		err = recordErr
	}
	return result, err
}

func (h *Sub2APIHandler) applyManualLocalAccountOps(ctx context.Context, operatorID int64, idempotencyKeyHash string, input sub2apiapp.LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	if h.actions == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ACTION_SERVICE_NOT_CONFIGURED", "action service is not configured")
	}
	result, err := h.service.ApplyLocalAccountOpsAction(ctx, input)
	status := adminplusdomain.ActionExecutionStatusSucceeded
	errorMessage := ""
	if err != nil {
		status = adminplusdomain.ActionExecutionStatusFailed
		errorMessage = err.Error()
	} else if result != nil && result.Blocked {
		status = adminplusdomain.ActionExecutionStatusFailed
		errorMessage = result.BlockedReason
	}
	requestPayload := localAccountOpsActionRequestPayload(input, "", 0)
	responsePayload := localAccountOpsActionResponsePayload(result)
	requestPayload["mode"] = "local_account_manual_ops_apply"
	responsePayload["mode"] = "local_account_manual_ops_apply"
	execution, recordErr := h.actions.RecordManualExecution(ctx, actionsapp.ManualExecutionInput{
		ActionType:         adminplusdomain.ActionTypeLocalAccountManualOps,
		Severity:           adminplusdomain.ActionSeverityInfo,
		ReasonCode:         "local_account_manual_ops_apply",
		Title:              manualLocalAccountOpsTitle(input),
		Description:        "Manual local account operation executed from Admin Plus local account operations.",
		ExpectedImpact:     "keep manual local routing changes in unified action execution history",
		Signals:            manualLocalAccountOpsSignals(input),
		OperatorUserID:     operatorID,
		IdempotencyKeyHash: strings.TrimSpace(idempotencyKeyHash),
		BeforeSnapshot:     localAccountOpsBeforeSnapshot(result),
		AfterSnapshot:      localAccountOpsAfterSnapshot(result),
		RequestPayload:     requestPayload,
		ResponsePayload:    responsePayload,
		Status:             status,
		ErrorMessage:       errorMessage,
	})
	if recordErr == nil && result != nil && execution != nil {
		result.ActionRecommendationID = execution.RecommendationID
		result.ActionExecutionID = execution.ID
	}
	if err == nil && recordErr != nil {
		err = recordErr
	}
	return result, err
}

func (h *Sub2APIHandler) applyRollbackLocalAccountOps(ctx context.Context, actionID int64, operatorID int64, idempotencyKeyHash string, schedulerRunID string, schedulerStepID int64, input sub2apiapp.LocalAccountOpsActionInput, rollbackSourceExecutionID int64, mode string) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	result, err := h.service.ApplyLocalAccountOpsAction(ctx, input)
	status := adminplusdomain.ActionExecutionStatusSucceeded
	errorMessage := ""
	if err != nil {
		status = adminplusdomain.ActionExecutionStatusFailed
		errorMessage = err.Error()
	} else if result != nil && result.Blocked {
		status = adminplusdomain.ActionExecutionStatusFailed
		errorMessage = result.BlockedReason
	}
	requestPayload := localAccountOpsActionRequestPayload(input, schedulerRunID, schedulerStepID)
	responsePayload := localAccountOpsActionResponsePayload(result)
	if strings.TrimSpace(mode) != "" {
		requestPayload["mode"] = strings.TrimSpace(mode)
		responsePayload["mode"] = strings.TrimSpace(mode)
	}
	addRollbackSourcePayload(requestPayload, rollbackSourceExecutionID)
	addRollbackSourcePayload(responsePayload, rollbackSourceExecutionID)
	_, recordErr := h.actions.RecordExternalExecution(ctx, actionID, actionsapp.ExecuteInput{
		OperatorUserID:     operatorID,
		SchedulerRunID:     schedulerRunID,
		SchedulerStepID:    schedulerStepID,
		IdempotencyKeyHash: idempotencyKeyHash,
		BeforeSnapshot:     localAccountOpsBeforeSnapshot(result),
		AfterSnapshot:      localAccountOpsAfterSnapshot(result),
		RequestPayload:     requestPayload,
	}, status, responsePayload, errorMessage)
	if err == nil && recordErr != nil {
		err = recordErr
	}
	return result, err
}

func (h *Sub2APIHandler) applyActionBackedRoutingRefill(ctx context.Context, actionID int64, operatorID int64, idempotencyKeyHash string, schedulerRunID string, schedulerStepID int64, input sub2apiapp.RoutingRefillInput, retrySourceExecutionID int64) (*sub2apiapp.RoutingRefillResult, error) {
	if h.actions == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ACTION_SERVICE_NOT_CONFIGURED", "action service is not configured")
	}
	action, err := h.actions.RequireApprovedRecommendation(ctx, actionID, adminplusdomain.ActionTypeRoutingRefill)
	if err != nil {
		return nil, err
	}
	if actionLocalGroupID := int64Signal(action.Signals, "local_group_id"); actionLocalGroupID > 0 && actionLocalGroupID != input.LocalGroupID {
		return nil, infraerrors.New(http.StatusConflict, "ACTION_ROUTING_REFILL_GROUP_MISMATCH", "action recommendation local group does not match request")
	}
	result, err := h.service.RefillLocalGroup(ctx, input)
	status := adminplusdomain.ActionExecutionStatusSucceeded
	errorMessage := ""
	if err != nil {
		status = adminplusdomain.ActionExecutionStatusFailed
		errorMessage = err.Error()
	} else if result != nil && result.SkippedReason != "" {
		status = adminplusdomain.ActionExecutionStatusFailed
		errorMessage = result.SkippedReason
	}
	requestPayload := routingRefillActionRequestPayload(input, schedulerRunID, schedulerStepID)
	responsePayload := routingRefillActionResponsePayload(result)
	addRetrySourcePayload(requestPayload, retrySourceExecutionID)
	addRetrySourcePayload(responsePayload, retrySourceExecutionID)
	_, recordErr := h.actions.RecordExternalExecution(ctx, actionID, actionsapp.ExecuteInput{
		OperatorUserID:     operatorID,
		SchedulerRunID:     schedulerRunID,
		SchedulerStepID:    schedulerStepID,
		IdempotencyKeyHash: idempotencyKeyHash,
		BeforeSnapshot:     routingRefillBeforeSnapshot(result),
		AfterSnapshot:      routingRefillAfterSnapshot(result),
		RequestPayload:     requestPayload,
	}, status, responsePayload, errorMessage)
	if err == nil && recordErr != nil {
		err = recordErr
	}
	return result, err
}

func (h *Sub2APIHandler) ensureExecutionNotRolledBack(ctx context.Context, recommendationID int64, executionID int64) error {
	items, err := h.actions.ListExecutions(ctx, recommendationID, 1000)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item == nil || item.Status != adminplusdomain.ActionExecutionStatusSucceeded {
			continue
		}
		if payloadInt64(item.RequestPayload, "rollback_source_execution_id") == executionID || payloadInt64(item.ResponsePayload, "rollback_source_execution_id") == executionID {
			return infraerrors.New(http.StatusConflict, "ACTION_EXECUTION_ALREADY_ROLLED_BACK", "action execution has already been rolled back")
		}
	}
	return nil
}

func latestCreatedExecution(service *actionsapp.Service, ctx context.Context, recommendationID int64) (*adminplusdomain.ActionExecution, error) {
	items, err := service.ListExecutions(ctx, recommendationID, 1)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 || items[0] == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ACTION_EXECUTION_RETRY_NOT_RECORDED", "action execution retry was not recorded")
	}
	return items[0], nil
}

func (h *Sub2APIHandler) ListRoutingRefillRuns(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListRoutingRefillRuns(c.Request.Context(), sub2apiapp.RoutingRefillRunFilter{
		LocalGroupID: parseInt64Query(c, "local_group_id"),
		Status:       c.Query("status"),
		Limit:        fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *Sub2APIHandler) ListRoutingImpactAPIKeys(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListRoutingImpactAPIKeys(c.Request.Context(), sub2apiapp.RoutingImpactFilter{
		LocalGroupID: parseInt64Query(c, "local_group_id"),
		Limit:        fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *Sub2APIHandler) ListRoutingImpactFailureRequests(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListRoutingImpactFailureRequests(c.Request.Context(), sub2apiapp.RoutingImpactFilter{
		LocalGroupID: parseInt64Query(c, "local_group_id"),
		Limit:        fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *Sub2APIHandler) GetRoutingFailureSensitiveDetail(c *gin.Context) {
	failureID, ok := parseIDParam(c, "failureID")
	if !ok {
		return
	}
	var req routingSensitiveFailureDetailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	result, err := h.service.GetRoutingFailureSensitiveDetail(c.Request.Context(), sub2apiapp.RoutingSensitiveFailureDetailInput{
		FailureID:    failureID,
		LocalGroupID: req.LocalGroupID,
		Reason:       req.Reason,
		Fields:       req.Fields,
		RequestedBy:  currentAdminUserID(c),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *Sub2APIHandler) SyncLocalAccountState(c *gin.Context) {
	var req localAccountStateSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	result, err := h.service.SyncLocalAccountState(c.Request.Context(), sub2apiapp.LocalAccountStateSyncInput{
		AccountIDs:  req.AccountIDs,
		Limit:       req.Limit,
		RequestedBy: currentAdminUserID(c),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *Sub2APIHandler) AcceptLocalAccountState(c *gin.Context) {
	h.resolveLocalAccountState(c, adminplusdomain.LocalAccountStateResolutionAcceptObserved)
}

func (h *Sub2APIHandler) RestoreLocalAccountState(c *gin.Context) {
	h.resolveLocalAccountState(c, adminplusdomain.LocalAccountStateResolutionRestoreAccepted)
}

func (h *Sub2APIHandler) resolveLocalAccountState(c *gin.Context, action adminplusdomain.LocalAccountStateResolutionAction) {
	var req localAccountStateResolutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	input := sub2apiapp.LocalAccountStateResolutionInput{
		Action:      action,
		AccountIDs:  req.AccountIDs,
		RequestedBy: currentAdminUserID(c),
	}
	executeAdminPlusIdempotentJSON(c, "admin-plus.local-account-state."+string(action), input, 0, http.StatusOK, func(ctx context.Context) (any, error) {
		return h.service.ResolveLocalAccountState(ctx, input)
	})
}

func (h *Sub2APIHandler) ListAccountRuntime(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListAccountRuntime(c.Request.Context(), sub2apiapp.RuntimeFilter{
		AccountID: parseInt64Query(c, "account_id"),
		Query:     c.Query("q"),
		Limit:     fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func parseUsageFilter(c *gin.Context) (sub2apiapp.UsageFilter, bool) {
	from, ok := parseOptionalQueryTime(c, "from")
	if !ok {
		return sub2apiapp.UsageFilter{}, false
	}
	to, ok := parseOptionalQueryTime(c, "to")
	if !ok {
		return sub2apiapp.UsageFilter{}, false
	}
	return sub2apiapp.UsageFilter{
		AccountID: parseInt64Query(c, "account_id"),
		Model:     c.Query("model"),
		From:      valueOrZero(from),
		To:        valueOrZero(to),
		Limit:     parseIntQuery(c, "limit"),
	}, true
}

func int64Signal(signals []string, key string) int64 {
	prefix := strings.TrimSpace(key) + "="
	for _, signal := range signals {
		trimmed := strings.TrimSpace(signal)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		value := strings.TrimPrefix(trimmed, prefix)
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func parseActionExecutionID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("executionID"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid action execution id")
		return 0, false
	}
	return id, true
}

func decodeActionExecutionRequestPayload(payload map[string]any, dest any) error {
	if len(payload) == 0 {
		return infraerrors.New(http.StatusConflict, "ACTION_EXECUTION_RETRY_PAYLOAD_MISSING", "action execution request payload is missing")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return infraerrors.New(http.StatusConflict, "ACTION_EXECUTION_RETRY_PAYLOAD_INVALID", "action execution request payload is invalid")
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return infraerrors.New(http.StatusConflict, "ACTION_EXECUTION_RETRY_PAYLOAD_INVALID", "action execution request payload is invalid")
	}
	return nil
}

func routingRefillActionRequestPayload(input sub2apiapp.RoutingRefillInput, schedulerRunID string, schedulerStepID int64) map[string]any {
	payload := map[string]any{
		"mode":                   "routing_refill_apply",
		"local_group_id":         input.LocalGroupID,
		"platform":               input.Platform,
		"model_scope":            input.ModelScope,
		"max_rate_multiplier":    input.MaxRateMultiplier,
		"limit":                  input.Limit,
		"dry_run":                input.DryRun,
		"action_id":              input.ActionRecommendationID,
		"reason":                 input.Reason,
		"trigger_type":           input.TriggerType,
		"cooldown_seconds":       input.CooldownSeconds,
		"confirm_window_seconds": input.ConfirmWindowSecs,
	}
	addSchedulerSourcePayload(payload, schedulerRunID, schedulerStepID)
	return payload
}

func addRetrySourcePayload(payload map[string]any, executionID int64) {
	if payload == nil || executionID <= 0 {
		return
	}
	payload["retry_source_execution_id"] = executionID
}

func addRollbackSourcePayload(payload map[string]any, executionID int64) {
	if payload == nil || executionID <= 0 {
		return
	}
	payload["rollback_source_execution_id"] = executionID
}

func payloadInt64(payload map[string]any, key string) int64 {
	if payload == nil {
		return 0
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return int64(typed)
		}
	case int64:
		if typed > 0 {
			return typed
		}
	case int32:
		if typed > 0 {
			return int64(typed)
		}
	case float64:
		if typed > 0 {
			return int64(typed)
		}
	case float32:
		if typed > 0 {
			return int64(typed)
		}
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil && parsed > 0 {
			return parsed
		}
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return 0
}

func routingRefillActionResponsePayload(result *sub2apiapp.RoutingRefillResult) map[string]any {
	payload := map[string]any{"mode": "routing_refill_apply"}
	if result == nil {
		return payload
	}
	payload["local_group_id"] = result.LocalGroupID
	payload["platform"] = result.Platform
	payload["model_scope"] = result.ModelScope
	payload["dry_run"] = result.DryRun
	payload["skipped_reason"] = result.SkippedReason
	if result.AvailabilityBefore != nil {
		payload["before_schedulable_accounts"] = result.AvailabilityBefore.SchedulableAccounts
		payload["before_active_api_key_count"] = result.AvailabilityBefore.ActiveAPIKeyCount
	}
	if result.AvailabilityAfter != nil {
		payload["after_schedulable_accounts"] = result.AvailabilityAfter.SchedulableAccounts
		payload["after_active_api_key_count"] = result.AvailabilityAfter.ActiveAPIKeyCount
	}
	if result.Candidate != nil {
		payload["candidate_supplier_id"] = result.Candidate.SupplierID
		payload["candidate_supplier_group_id"] = result.Candidate.SupplierGroupID
		payload["candidate_local_account_id"] = result.Candidate.LocalSub2APIAccountID
		payload["candidate_model_scope"] = result.Candidate.ModelScope
		payload["candidate_model_match_status"] = result.Candidate.ModelMatchStatus
		payload["candidate_effective_rate_multiplier"] = result.Candidate.EffectiveRateMultiplier
	}
	if result.Account != nil {
		payload["account_id"] = result.Account.AccountID
		payload["account_schedulable"] = result.Account.Schedulable
	}
	return payload
}

func validateLocalAccountScheduleDisableAction(action *adminplusdomain.ActionRecommendation, input sub2apiapp.LocalAccountOpsActionInput) error {
	if input.Action != adminplusdomain.LocalAccountOpsActionSetSchedulable || input.Schedulable == nil || *input.Schedulable {
		return infraerrors.New(http.StatusConflict, "ACTION_LOCAL_ACCOUNT_DISABLE_OPERATION_MISMATCH", "action recommendation requires disabling local account scheduling")
	}
	if len(input.AccountIDs) != 1 {
		return infraerrors.New(http.StatusConflict, "ACTION_LOCAL_ACCOUNT_DISABLE_ACCOUNT_COUNT_MISMATCH", "action recommendation must disable exactly one local account")
	}
	actionAccountID := int64Signal(action.Signals, "local_sub2api_account_id")
	if actionAccountID <= 0 {
		return infraerrors.New(http.StatusConflict, "ACTION_LOCAL_ACCOUNT_DISABLE_ACCOUNT_SIGNAL_MISSING", "action recommendation local account signal is missing")
	}
	if actionAccountID != input.AccountIDs[0] {
		return infraerrors.New(http.StatusConflict, "ACTION_LOCAL_ACCOUNT_DISABLE_ACCOUNT_MISMATCH", "action recommendation local account does not match request")
	}
	return nil
}

func localAccountOpsActionRequestPayload(input sub2apiapp.LocalAccountOpsActionInput, schedulerRunID string, schedulerStepID int64) map[string]any {
	payload := map[string]any{
		"mode":        "local_account_ops_apply",
		"action":      string(input.Action),
		"account_ids": input.AccountIDs,
		"group_ids":   input.GroupIDs,
		"dry_run":     input.DryRun,
		"action_id":   input.ActionRecommendationID,
		"reason":      input.Reason,
	}
	if input.Schedulable != nil {
		payload["schedulable"] = *input.Schedulable
	}
	if input.AllowEmptyPool {
		payload["allow_empty_pool"] = true
	}
	addSchedulerSourcePayload(payload, schedulerRunID, schedulerStepID)
	return payload
}

func addSchedulerSourcePayload(payload map[string]any, schedulerRunID string, schedulerStepID int64) {
	if payload == nil {
		return
	}
	if trimmed := strings.TrimSpace(schedulerRunID); trimmed != "" {
		payload["scheduler_run_id"] = trimmed
	}
	if schedulerStepID > 0 {
		payload["scheduler_step_id"] = schedulerStepID
	}
}

func adminPlusIdempotencyKeyHash(c *gin.Context) string {
	if c == nil {
		return ""
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		return ""
	}
	return service.HashIdempotencyKey(key)
}

func localAccountOpsActionResponsePayload(result *adminplusdomain.LocalAccountOpsActionResult) map[string]any {
	payload := map[string]any{"mode": "local_account_ops_apply"}
	if result == nil {
		return payload
	}
	payload["action"] = string(result.Action)
	payload["dry_run"] = result.DryRun
	payload["blocked"] = result.Blocked
	payload["blocked_reason"] = result.BlockedReason
	payload["account_ids"] = result.AccountIDs
	payload["group_ids"] = result.GroupIDs
	payload["updated_accounts"] = result.UpdatedAccounts
	payload["added_bindings"] = result.AddedBindings
	payload["removed_bindings"] = result.RemovedBindings
	if len(result.GroupImpacts) > 0 {
		impacts := make([]map[string]any, 0, len(result.GroupImpacts))
		for _, impact := range result.GroupImpacts {
			impacts = append(impacts, map[string]any{
				"group_id":                     impact.GroupID,
				"active_api_key_count":         impact.ActiveAPIKeyCount,
				"before_schedulable_accounts":  impact.BeforeSchedulableAccounts,
				"after_schedulable_accounts":   impact.AfterSchedulableAccounts,
				"would_empty_schedulable_pool": impact.WouldEmptySchedulablePool,
			})
		}
		payload["group_impacts"] = impacts
	}
	if len(result.Warnings) > 0 {
		payload["warnings"] = result.Warnings
	}
	return payload
}

func routingRefillBeforeSnapshot(result *sub2apiapp.RoutingRefillResult) map[string]any {
	if result == nil || result.AvailabilityBefore == nil {
		return map[string]any{}
	}
	return routingAvailabilitySnapshot("routing_refill_before", result.LocalGroupID, result.Platform, result.ModelScope, result.AvailabilityBefore)
}

func routingRefillAfterSnapshot(result *sub2apiapp.RoutingRefillResult) map[string]any {
	if result == nil {
		return map[string]any{}
	}
	payload := map[string]any{
		"mode":           "routing_refill_after",
		"local_group_id": result.LocalGroupID,
		"platform":       result.Platform,
		"model_scope":    result.ModelScope,
		"skipped_reason": result.SkippedReason,
	}
	if result.AvailabilityAfter != nil {
		for key, value := range routingAvailabilitySnapshot("", result.LocalGroupID, result.Platform, result.ModelScope, result.AvailabilityAfter) {
			if key != "mode" {
				payload[key] = value
			}
		}
	}
	if result.Candidate != nil {
		payload["candidate_supplier_id"] = result.Candidate.SupplierID
		payload["candidate_supplier_group_id"] = result.Candidate.SupplierGroupID
		payload["candidate_local_account_id"] = result.Candidate.LocalSub2APIAccountID
		payload["candidate_model_scope"] = result.Candidate.ModelScope
		payload["candidate_model_match_status"] = result.Candidate.ModelMatchStatus
		payload["candidate_effective_rate_multiplier"] = result.Candidate.EffectiveRateMultiplier
	}
	if result.Account != nil {
		payload["account_id"] = result.Account.AccountID
		payload["account_schedulable"] = result.Account.Schedulable
	}
	return payload
}

func routingAvailabilitySnapshot(mode string, localGroupID int64, platform string, modelScope string, availability *sub2apiapp.RoutingGroupAvailability) map[string]any {
	if availability == nil {
		return map[string]any{}
	}
	payload := map[string]any{
		"mode":                         mode,
		"local_group_id":               localGroupID,
		"platform":                     platform,
		"model_scope":                  modelScope,
		"group_id":                     availability.GroupID,
		"group_name":                   availability.GroupName,
		"total_accounts":               availability.TotalAccounts,
		"schedulable_accounts":         availability.SchedulableAccounts,
		"active_api_key_count":         availability.ActiveAPIKeyCount,
		"recent_success_request_count": availability.RecentSuccessRequestCount,
		"recent_error_request_count":   availability.RecentErrorRequestCount,
		"recent_upstream_429_count":    availability.RecentUpstream429Count,
	}
	return payload
}

func localAccountOpsBeforeSnapshot(result *adminplusdomain.LocalAccountOpsActionResult) map[string]any {
	if result == nil {
		return map[string]any{}
	}
	return map[string]any{
		"mode":          "local_account_ops_before",
		"action":        string(result.Action),
		"account_ids":   result.AccountIDs,
		"group_impacts": localAccountOpsGroupImpactSnapshot(result.GroupImpacts, false),
	}
}

func localAccountOpsAfterSnapshot(result *adminplusdomain.LocalAccountOpsActionResult) map[string]any {
	if result == nil {
		return map[string]any{}
	}
	return map[string]any{
		"mode":             "local_account_ops_after",
		"action":           string(result.Action),
		"account_ids":      result.AccountIDs,
		"blocked":          result.Blocked,
		"blocked_reason":   result.BlockedReason,
		"updated_accounts": result.UpdatedAccounts,
		"added_bindings":   result.AddedBindings,
		"removed_bindings": result.RemovedBindings,
		"group_impacts":    localAccountOpsGroupImpactSnapshot(result.GroupImpacts, true),
	}
}

func manualLocalAccountOpsTitle(input sub2apiapp.LocalAccountOpsActionInput) string {
	switch input.Action {
	case adminplusdomain.LocalAccountOpsActionSetSchedulable:
		if input.Schedulable != nil && *input.Schedulable {
			return "Enable local account scheduling"
		}
		return "Disable local account scheduling"
	case adminplusdomain.LocalAccountOpsActionAddToGroups:
		return "Add local accounts to groups"
	case adminplusdomain.LocalAccountOpsActionRemoveFromGroups:
		return "Remove local accounts from groups"
	default:
		return "Manual local account operation"
	}
}

func manualLocalAccountOpsSignals(input sub2apiapp.LocalAccountOpsActionInput) []string {
	signals := []string{
		"action=" + string(input.Action),
		"manual=true",
	}
	if accountIDs := int64ListSignal(input.AccountIDs); accountIDs != "" {
		signals = append(signals, "local_sub2api_account_ids="+accountIDs)
		if len(input.AccountIDs) == 1 && input.AccountIDs[0] > 0 {
			signals = append(signals, "local_sub2api_account_id="+strconv.FormatInt(input.AccountIDs[0], 10))
		}
	}
	if groupIDs := int64ListSignal(input.GroupIDs); groupIDs != "" {
		signals = append(signals, "local_group_ids="+groupIDs)
		if len(input.GroupIDs) == 1 && input.GroupIDs[0] > 0 {
			signals = append(signals, "local_group_id="+strconv.FormatInt(input.GroupIDs[0], 10))
		}
	}
	if input.Schedulable != nil {
		signals = append(signals, "schedulable="+strconv.FormatBool(*input.Schedulable))
	}
	if input.AllowEmptyPool {
		signals = append(signals, "allow_empty_pool=true")
	}
	if reason := strings.TrimSpace(input.Reason); reason != "" {
		signals = append(signals, "reason="+reason)
	}
	return signals
}

func int64ListSignal(values []int64) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value > 0 {
			parts = append(parts, strconv.FormatInt(value, 10))
		}
	}
	return strings.Join(parts, ",")
}

func localAccountOpsGroupImpactSnapshot(impacts []adminplusdomain.LocalAccountOpsGroupImpact, after bool) []map[string]any {
	out := make([]map[string]any, 0, len(impacts))
	for _, impact := range impacts {
		item := map[string]any{
			"group_id":                     impact.GroupID,
			"group_name":                   impact.GroupName,
			"active_api_key_count":         impact.ActiveAPIKeyCount,
			"would_empty_schedulable_pool": impact.WouldEmptySchedulablePool,
		}
		if after {
			item["schedulable_accounts"] = impact.AfterSchedulableAccounts
		} else {
			item["schedulable_accounts"] = impact.BeforeSchedulableAccounts
		}
		out = append(out, item)
	}
	return out
}

func localAccountOpsInputFromRequest(req localAccountOpsActionRequest, requestedBy int64) sub2apiapp.LocalAccountOpsActionInput {
	return sub2apiapp.LocalAccountOpsActionInput{
		Action:                 adminplusdomain.LocalAccountOpsAction(req.Action),
		AccountIDs:             req.AccountIDs,
		GroupIDs:               req.GroupIDs,
		Schedulable:            req.Schedulable,
		AllowEmptyPool:         req.AllowEmptyPool,
		RequestedBy:            requestedBy,
		Reason:                 req.Reason,
		ActionRecommendationID: req.ActionID,
	}
}

func parseOptionalQueryTime(c *gin.Context, name string) (*time.Time, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		response.BadRequest(c, "invalid "+name+", expected RFC3339")
		return nil, false
	}
	return &t, true
}

func valueOrZero(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

func parseAccountIDParam(c *gin.Context) (int64, bool) {
	accountID, err := strconv.ParseInt(c.Param("accountID"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "invalid account id")
		return 0, false
	}
	return accountID, true
}
