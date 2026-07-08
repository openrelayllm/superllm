package adminplus

import (
	"context"
	"net/http"
	"time"

	actionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/actions"
	costsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/costs"
	provisionjobsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/provisionjobs"
	schedulerapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/scheduler"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type CostHandler struct {
	service       *costsapp.Service
	provisionJobs *provisionjobsapp.Service
	scheduler     *schedulerapp.Service
	actions       *actionsapp.Service
}

func NewCostHandler(service *costsapp.Service) *CostHandler {
	return &CostHandler{service: service}
}

func NewCostHandlerWithProvisionJobs(service *costsapp.Service, provisionJobs *provisionjobsapp.Service) *CostHandler {
	return &CostHandler{service: service, provisionJobs: provisionJobs}
}

func NewCostHandlerWithProvisionJobsAndScheduler(service *costsapp.Service, provisionJobs *provisionjobsapp.Service, scheduler *schedulerapp.Service) *CostHandler {
	return &CostHandler{service: service, provisionJobs: provisionJobs, scheduler: scheduler}
}

func NewCostHandlerWithProvisionJobsSchedulerAndActions(service *costsapp.Service, provisionJobs *provisionjobsapp.Service, scheduler *schedulerapp.Service, actions *actionsapp.Service) *CostHandler {
	return &CostHandler{service: service, provisionJobs: provisionJobs, scheduler: scheduler, actions: actions}
}

type syncSupplierCostsRequest struct {
	StartedAt                      string `json:"started_at"`
	EndedAt                        string `json:"ended_at"`
	IncludeFundingTransactions     *bool  `json:"include_funding_transactions"`
	IncludeEntitlementTransactions *bool  `json:"include_entitlement_transactions"`
	IncludeUsageCostLines          *bool  `json:"include_usage_cost_lines"`
	IncludeBalanceSnapshot         *bool  `json:"include_balance_snapshot"`
	LowBalanceThresholdCents       int64  `json:"low_balance_threshold_cents"`
}

type backfillSupplierCostsRequest struct {
	SupplierID                     int64  `json:"supplier_id"`
	StartedAt                      string `json:"started_at"`
	EndedAt                        string `json:"ended_at"`
	IncludeFundingTransactions     *bool  `json:"include_funding_transactions"`
	IncludeEntitlementTransactions *bool  `json:"include_entitlement_transactions"`
	IncludeUsageCostLines          *bool  `json:"include_usage_cost_lines"`
	IncludeBalanceSnapshot         *bool  `json:"include_balance_snapshot"`
	LowBalanceThresholdCents       int64  `json:"low_balance_threshold_cents"`
}

type applyCostReconcileAdjustmentRequest struct {
	SnapshotID            int64  `json:"snapshot_id"`
	AdjustmentAmountCents *int64 `json:"adjustment_amount_cents"`
	Reason                string `json:"reason"`
	OccurredAt            string `json:"occurred_at"`
	SchedulerRunID        string `json:"scheduler_run_id"`
	SchedulerStepID       int64  `json:"scheduler_step_id"`
}

type applyCostReconcileDetailRepairRequest struct {
	SnapshotID      int64  `json:"snapshot_id"`
	DetailType      string `json:"detail_type"`
	AmountCents     *int64 `json:"amount_cents"`
	ExternalID      string `json:"external_id"`
	Model           string `json:"model"`
	Reason          string `json:"reason"`
	OccurredAt      string `json:"occurred_at"`
	SchedulerRunID  string `json:"scheduler_run_id"`
	SchedulerStepID int64  `json:"scheduler_step_id"`
}

func (h *CostHandler) SyncSupplierCosts(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	var req syncSupplierCostsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	startedAt, ok := parseOptionalNamedTime(c, "started_at", req.StartedAt)
	if !ok {
		return
	}
	endedAt, ok := parseOptionalNamedTime(c, "ended_at", req.EndedAt)
	if !ok {
		return
	}
	if h.provisionJobs == nil {
		err := infraerrors.New(500, "ADMIN_PLUS_INTERNAL_ERROR", "supplier cost sync job service is not configured")
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.provisionJobs.Submit(c.Request.Context(), provisionjobsapp.SubmitInput{
		JobType:        adminplusdomain.SupplierProvisionJobTypeSyncSupplierCosts,
		SupplierID:     supplierID,
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
		RequestedBy:    currentAdminUserID(c),
		Request: map[string]any{
			"started_at":                       timePtrRFC3339(startedAt),
			"ended_at":                         timePtrRFC3339(endedAt),
			"include_funding_transactions":     boolDefault(req.IncludeFundingTransactions, true),
			"include_entitlement_transactions": boolDefault(req.IncludeEntitlementTransactions, true),
			"include_usage_cost_lines":         boolDefault(req.IncludeUsageCostLines, true),
			"include_balance_snapshot":         boolDefault(req.IncludeBalanceSnapshot, true),
			"low_balance_threshold_cents":      req.LowBalanceThresholdCents,
		},
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Accepted(c, result)
}

func (h *CostHandler) BackfillSupplierCosts(c *gin.Context) {
	var req backfillSupplierCostsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	startedAt, ok := parseOptionalNamedTime(c, "started_at", req.StartedAt)
	if !ok {
		return
	}
	endedAt, ok := parseOptionalNamedTime(c, "ended_at", req.EndedAt)
	if !ok {
		return
	}
	if h.scheduler == nil {
		err := infraerrors.New(500, "ADMIN_PLUS_INTERNAL_ERROR", "scheduler service is not configured")
		response.ErrorFrom(c, err)
		return
	}
	run, err := h.scheduler.EnqueueCostHistoryBackfill(c.Request.Context(), schedulerapp.CostBackfillInput{
		Mode:                           "manual:cost-history-backfill",
		SupplierID:                     req.SupplierID,
		StartedAt:                      startedAt,
		EndedAt:                        endedAt,
		IncludeFundingTransactions:     boolDefault(req.IncludeFundingTransactions, true),
		IncludeEntitlementTransactions: boolDefault(req.IncludeEntitlementTransactions, true),
		IncludeUsageCostLines:          boolDefault(req.IncludeUsageCostLines, true),
		IncludeBalanceSnapshot:         boolDefault(req.IncludeBalanceSnapshot, true),
		LowBalanceThresholdCents:       req.LowBalanceThresholdCents,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Accepted(c, run)
}

func (h *CostHandler) ListSupplierSummaries(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListSnapshots(c.Request.Context(), costsapp.SummaryFilter{
		SupplierID: parseInt64Query(c, "supplier_id"),
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *CostHandler) GetLedgerOverview(c *gin.Context) {
	overview, err := h.service.GetLedgerOverview(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, overview)
}

func (h *CostHandler) GetSupplierSummary(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	items, err := h.service.ListSnapshots(c.Request.Context(), costsapp.SummaryFilter{
		SupplierID: supplierID,
		Limit:      10,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

func (h *CostHandler) ListFundingTransactions(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	page := parsePagination(c)
	items, err := h.service.ListFundingTransactions(c.Request.Context(), costsapp.TransactionFilter{
		SupplierID: supplierID,
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *CostHandler) ListEntitlementTransactions(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	page := parsePagination(c)
	items, err := h.service.ListEntitlementTransactions(c.Request.Context(), costsapp.TransactionFilter{
		SupplierID: supplierID,
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *CostHandler) ListLedgerEntries(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	page := parsePagination(c)
	items, err := h.service.ListLedgerEntries(c.Request.Context(), costsapp.LedgerFilter{
		SupplierID: supplierID,
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *CostHandler) ApplyReconcileAdjustment(c *gin.Context) {
	recommendationID, ok := parseActionRecommendationID(c)
	if !ok {
		return
	}
	var req applyCostReconcileAdjustmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if h.actions == nil {
		response.InternalError(c, "action service is not configured")
		return
	}
	if h.service == nil {
		response.InternalError(c, "cost service is not configured")
		return
	}
	occurredAt, ok := parseOptionalNamedTime(c, "occurred_at", req.OccurredAt)
	if !ok {
		return
	}
	action, err := h.actions.RequireApprovedRecommendation(c.Request.Context(), recommendationID, adminplusdomain.ActionTypeSupplierCostReconcileAdjustment)
	if response.ErrorFrom(c, err) {
		return
	}
	snapshotID := req.SnapshotID
	actionSnapshotID := int64Signal(action.Signals, "cost_snapshot_id")
	if snapshotID <= 0 {
		snapshotID = actionSnapshotID
	}
	if actionSnapshotID > 0 && snapshotID != actionSnapshotID {
		err := infraerrors.New(http.StatusConflict, "COST_RECONCILE_SNAPSHOT_MISMATCH", "request snapshot does not match the action recommendation")
		response.ErrorFrom(c, err)
		return
	}
	idempotencyKeyHash := adminPlusIdempotencyKeyHash(c)
	payload := map[string]any{
		"recommendation_id": recommendationID,
		"supplier_id":       action.SupplierID,
		"snapshot_id":       snapshotID,
		"amount_cents":      req.AdjustmentAmountCents,
		"reason":            req.Reason,
	}
	executeAdminPlusIdempotentJSONWithReplay(c, "admin-plus.costs.reconcile-adjustment", payload, 0, http.StatusCreated, func(ctx context.Context) (any, error) {
		result, err := h.service.ApplyReconcileAdjustment(ctx, costsapp.ReconcileAdjustmentInput{
			SupplierID:             action.SupplierID,
			SnapshotID:             snapshotID,
			ActionRecommendationID: recommendationID,
			AdjustmentAmountCents:  req.AdjustmentAmountCents,
			Reason:                 req.Reason,
			OccurredAt:             occurredAt,
		})
		status := adminplusdomain.ActionExecutionStatusSucceeded
		errorMessage := ""
		if err != nil {
			status = adminplusdomain.ActionExecutionStatusFailed
			errorMessage = err.Error()
		}
		requestPayload := costReconcileActionRequestPayload(recommendationID, req, snapshotID)
		responsePayload := costReconcileActionResponsePayload(result)
		_, recordErr := h.actions.RecordExternalExecution(ctx, recommendationID, actionsapp.ExecuteInput{
			OperatorUserID:     currentAdminUserID(c),
			SchedulerRunID:     req.SchedulerRunID,
			SchedulerStepID:    req.SchedulerStepID,
			IdempotencyKeyHash: idempotencyKeyHash,
			BeforeSnapshot:     costSnapshotPayload(nilSafeBeforeSnapshot(result)),
			AfterSnapshot:      costSnapshotPayload(nilSafeAfterSnapshot(result)),
			RequestPayload:     requestPayload,
		}, status, responsePayload, errorMessage)
		if err == nil && recordErr != nil {
			err = recordErr
		}
		if err != nil {
			return nil, err
		}
		return result, nil
	}, func(ctx context.Context) error {
		_, err := h.actions.MarkExecutionIdempotencyReplayed(ctx, recommendationID, idempotencyKeyHash)
		return err
	})
}

func (h *CostHandler) ApplyReconcileDetailRepair(c *gin.Context) {
	recommendationID, ok := parseActionRecommendationID(c)
	if !ok {
		return
	}
	var req applyCostReconcileDetailRepairRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if h.actions == nil {
		response.InternalError(c, "action service is not configured")
		return
	}
	if h.service == nil {
		response.InternalError(c, "cost service is not configured")
		return
	}
	occurredAt, ok := parseOptionalNamedTime(c, "occurred_at", req.OccurredAt)
	if !ok {
		return
	}
	action, err := h.actions.RequireApprovedRecommendation(c.Request.Context(), recommendationID, adminplusdomain.ActionTypeSupplierCostReconcileAdjustment)
	if response.ErrorFrom(c, err) {
		return
	}
	snapshotID := req.SnapshotID
	actionSnapshotID := int64Signal(action.Signals, "cost_snapshot_id")
	if snapshotID <= 0 {
		snapshotID = actionSnapshotID
	}
	if actionSnapshotID > 0 && snapshotID != actionSnapshotID {
		err := infraerrors.New(http.StatusConflict, "COST_RECONCILE_SNAPSHOT_MISMATCH", "request snapshot does not match the action recommendation")
		response.ErrorFrom(c, err)
		return
	}
	idempotencyKeyHash := adminPlusIdempotencyKeyHash(c)
	payload := map[string]any{
		"recommendation_id": recommendationID,
		"supplier_id":       action.SupplierID,
		"snapshot_id":       snapshotID,
		"detail_type":       req.DetailType,
		"amount_cents":      req.AmountCents,
		"external_id":       req.ExternalID,
		"model":             req.Model,
		"reason":            req.Reason,
	}
	executeAdminPlusIdempotentJSONWithReplay(c, "admin-plus.costs.reconcile-detail-repair", payload, 0, http.StatusCreated, func(ctx context.Context) (any, error) {
		result, err := h.service.ApplyReconcileDetailRepair(ctx, costsapp.ReconcileDetailRepairInput{
			SupplierID:             action.SupplierID,
			SnapshotID:             snapshotID,
			ActionRecommendationID: recommendationID,
			DetailType:             req.DetailType,
			AmountCents:            req.AmountCents,
			ExternalID:             req.ExternalID,
			Model:                  req.Model,
			Reason:                 req.Reason,
			OccurredAt:             occurredAt,
		})
		status := adminplusdomain.ActionExecutionStatusSucceeded
		errorMessage := ""
		if err != nil {
			status = adminplusdomain.ActionExecutionStatusFailed
			errorMessage = err.Error()
		}
		requestPayload := costReconcileDetailRepairActionRequestPayload(recommendationID, req, snapshotID)
		responsePayload := costReconcileDetailRepairActionResponsePayload(result)
		_, recordErr := h.actions.RecordExternalExecution(ctx, recommendationID, actionsapp.ExecuteInput{
			OperatorUserID:     currentAdminUserID(c),
			SchedulerRunID:     req.SchedulerRunID,
			SchedulerStepID:    req.SchedulerStepID,
			IdempotencyKeyHash: idempotencyKeyHash,
			BeforeSnapshot:     costSnapshotPayload(detailRepairBeforeSnapshot(result)),
			AfterSnapshot:      costSnapshotPayload(detailRepairAfterSnapshot(result)),
			RequestPayload:     requestPayload,
		}, status, responsePayload, errorMessage)
		if err == nil && recordErr != nil {
			err = recordErr
		}
		if err != nil {
			return nil, err
		}
		return result, nil
	}, func(ctx context.Context) error {
		_, err := h.actions.MarkExecutionIdempotencyReplayed(ctx, recommendationID, idempotencyKeyHash)
		return err
	})
}

func boolDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func timePtrRFC3339(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func costReconcileActionRequestPayload(actionID int64, req applyCostReconcileAdjustmentRequest, snapshotID int64) map[string]any {
	payload := map[string]any{
		"mode":                    "supplier_cost_reconcile_adjustment",
		"action_id":               actionID,
		"snapshot_id":             snapshotID,
		"adjustment_amount_cents": req.AdjustmentAmountCents,
		"reason":                  req.Reason,
		"occurred_at":             req.OccurredAt,
	}
	addSchedulerSourcePayload(payload, req.SchedulerRunID, req.SchedulerStepID)
	return payload
}

func costReconcileActionResponsePayload(result *costsapp.ReconcileAdjustmentResult) map[string]any {
	if result == nil {
		return map[string]any{"mode": "supplier_cost_reconcile_adjustment"}
	}
	payload := map[string]any{
		"mode":                    "supplier_cost_reconcile_adjustment",
		"supplier_id":             result.SupplierID,
		"snapshot_id":             result.SnapshotID,
		"currency":                result.Currency,
		"adjustment_amount_cents": result.AdjustmentAmountCents,
	}
	if result.LedgerEntry != nil {
		payload["ledger_entry_id"] = result.LedgerEntry.ID
	}
	if result.AfterSnapshot != nil {
		payload["after_snapshot_id"] = result.AfterSnapshot.ID
		payload["after_balance_delta_cents"] = result.AfterSnapshot.BalanceDeltaCents
	}
	return payload
}

func costReconcileDetailRepairActionRequestPayload(actionID int64, req applyCostReconcileDetailRepairRequest, snapshotID int64) map[string]any {
	payload := map[string]any{
		"mode":         "supplier_cost_reconcile_detail_repair",
		"action_id":    actionID,
		"snapshot_id":  snapshotID,
		"detail_type":  req.DetailType,
		"amount_cents": req.AmountCents,
		"external_id":  req.ExternalID,
		"model":        req.Model,
		"reason":       req.Reason,
		"occurred_at":  req.OccurredAt,
	}
	addSchedulerSourcePayload(payload, req.SchedulerRunID, req.SchedulerStepID)
	return payload
}

func costReconcileDetailRepairActionResponsePayload(result *costsapp.ReconcileDetailRepairResult) map[string]any {
	if result == nil {
		return map[string]any{"mode": "supplier_cost_reconcile_detail_repair"}
	}
	payload := map[string]any{
		"mode":         "supplier_cost_reconcile_detail_repair",
		"supplier_id":  result.SupplierID,
		"snapshot_id":  result.SnapshotID,
		"currency":     result.Currency,
		"detail_type":  result.DetailType,
		"amount_cents": result.AmountCents,
	}
	if result.FundingTransaction != nil {
		payload["funding_transaction_id"] = result.FundingTransaction.ID
	}
	if result.EntitlementTransaction != nil {
		payload["entitlement_transaction_id"] = result.EntitlementTransaction.ID
	}
	if result.UsageCostLine != nil {
		payload["usage_cost_line_id"] = result.UsageCostLine.ID
	}
	if result.LedgerEntry != nil {
		payload["ledger_entry_id"] = result.LedgerEntry.ID
	}
	if result.AfterSnapshot != nil {
		payload["after_snapshot_id"] = result.AfterSnapshot.ID
		payload["after_balance_delta_cents"] = result.AfterSnapshot.BalanceDeltaCents
	}
	return payload
}

func nilSafeBeforeSnapshot(result *costsapp.ReconcileAdjustmentResult) *adminplusdomain.SupplierCostSnapshot {
	if result == nil {
		return nil
	}
	return result.BeforeSnapshot
}

func nilSafeAfterSnapshot(result *costsapp.ReconcileAdjustmentResult) *adminplusdomain.SupplierCostSnapshot {
	if result == nil {
		return nil
	}
	return result.AfterSnapshot
}

func detailRepairBeforeSnapshot(result *costsapp.ReconcileDetailRepairResult) *adminplusdomain.SupplierCostSnapshot {
	if result == nil {
		return nil
	}
	return result.BeforeSnapshot
}

func detailRepairAfterSnapshot(result *costsapp.ReconcileDetailRepairResult) *adminplusdomain.SupplierCostSnapshot {
	if result == nil {
		return nil
	}
	return result.AfterSnapshot
}

func costSnapshotPayload(snapshot *adminplusdomain.SupplierCostSnapshot) map[string]any {
	if snapshot == nil {
		return nil
	}
	out := map[string]any{
		"snapshot_id":                    snapshot.ID,
		"supplier_id":                    snapshot.SupplierID,
		"currency":                       snapshot.Currency,
		"completed_funding_amount_cents": snapshot.CompletedFundingAmountCents,
		"completed_funding_cash_cents":   snapshot.CompletedFundingCashCents,
		"recharge_actual_payment_cents":  snapshot.RechargeActualPaymentCents,
		"entitlement_amount_cents":       snapshot.EntitlementAmountCents,
		"usage_cost_cents":               snapshot.UsageCostCents,
		"refund_amount_cents":            snapshot.RefundAmountCents,
		"adjustment_amount_cents":        snapshot.AdjustmentAmountCents,
		"expected_balance_cents":         snapshot.ExpectedBalanceCents,
		"captured_at":                    snapshot.CapturedAt.UTC().Format(time.RFC3339),
	}
	if snapshot.ActualBalanceCents != nil {
		out["actual_balance_cents"] = *snapshot.ActualBalanceCents
	}
	if snapshot.BalanceDeltaCents != nil {
		out["balance_delta_cents"] = *snapshot.BalanceDeltaCents
	}
	return out
}
