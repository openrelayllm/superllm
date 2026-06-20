package adminplus

import (
	reconciliationapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/reconciliation"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type ReconciliationHandler struct {
	service *reconciliationapp.Service
}

func NewReconciliationHandler(service *reconciliationapp.Service) *ReconciliationHandler {
	return &ReconciliationHandler{service: service}
}

type runReconciliationRequest struct {
	SupplierBills        []reconciliationSupplierBillDTO `json:"supplier_bills"`
	LocalUsages          []reconciliationLocalUsageDTO   `json:"local_usages"`
	TimeToleranceSeconds int64                           `json:"time_tolerance_seconds"`
	CostMismatchCents    int64                           `json:"cost_mismatch_cents"`
}

type reconciliationSupplierBillDTO struct {
	ID                int64  `json:"id"`
	SupplierID        int64  `json:"supplier_id"`
	ExternalBillID    string `json:"external_bill_id"`
	ExternalRequestID string `json:"external_request_id"`
	Model             string `json:"model"`
	Currency          string `json:"currency"`
	CostCents         int64  `json:"cost_cents"`
	InputTokens       int64  `json:"input_tokens"`
	OutputTokens      int64  `json:"output_tokens"`
	StartedAt         string `json:"started_at" binding:"required"`
}

type reconciliationLocalUsageDTO struct {
	ID                int64  `json:"id"`
	ExternalRequestID string `json:"external_request_id"`
	Model             string `json:"model"`
	Currency          string `json:"currency"`
	RevenueCents      int64  `json:"revenue_cents"`
	InputTokens       int64  `json:"input_tokens"`
	OutputTokens      int64  `json:"output_tokens"`
	StartedAt         string `json:"started_at" binding:"required"`
}

func (h *ReconciliationHandler) Run(c *gin.Context) {
	var req runReconciliationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	supplierBills := make([]*adminplusdomain.SupplierBillLine, 0, len(req.SupplierBills))
	for _, line := range req.SupplierBills {
		startedAt, ok := parseRequiredTime(c, "supplier_bills.started_at", line.StartedAt)
		if !ok {
			return
		}
		supplierBills = append(supplierBills, &adminplusdomain.SupplierBillLine{
			ID:                line.ID,
			SupplierID:        line.SupplierID,
			ExternalBillID:    line.ExternalBillID,
			ExternalRequestID: line.ExternalRequestID,
			Model:             line.Model,
			Currency:          line.Currency,
			CostCents:         line.CostCents,
			InputTokens:       line.InputTokens,
			OutputTokens:      line.OutputTokens,
			StartedAt:         *startedAt,
		})
	}

	localUsages := make([]*adminplusdomain.LocalUsageLine, 0, len(req.LocalUsages))
	for _, line := range req.LocalUsages {
		startedAt, ok := parseRequiredTime(c, "local_usages.started_at", line.StartedAt)
		if !ok {
			return
		}
		localUsages = append(localUsages, &adminplusdomain.LocalUsageLine{
			ID:                line.ID,
			ExternalRequestID: line.ExternalRequestID,
			Model:             line.Model,
			Currency:          line.Currency,
			RevenueCents:      line.RevenueCents,
			InputTokens:       line.InputTokens,
			OutputTokens:      line.OutputTokens,
			StartedAt:         *startedAt,
		})
	}

	result, err := h.service.Run(c.Request.Context(), reconciliationapp.RunInput{
		SupplierBills:     supplierBills,
		LocalUsages:       localUsages,
		TimeTolerance:     secondsDuration(req.TimeToleranceSeconds),
		CostMismatchCents: req.CostMismatchCents,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}
