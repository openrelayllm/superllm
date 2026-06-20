package adminplus

import (
	"time"

	billingapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/billing"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type BillingHandler struct {
	service *billingapp.Service
}

func NewBillingHandler(service *billingapp.Service) *BillingHandler {
	return &BillingHandler{service: service}
}

type importBillLinesRequest struct {
	Lines []billLineDTO `json:"lines" binding:"required"`
}

type billLineDTO struct {
	SupplierID        int64          `json:"supplier_id" binding:"required"`
	Source            string         `json:"source"`
	ExternalBillID    string         `json:"external_bill_id"`
	ExternalRequestID string         `json:"external_request_id"`
	Model             string         `json:"model" binding:"required"`
	Currency          string         `json:"currency"`
	CostCents         int64          `json:"cost_cents"`
	InputTokens       int64          `json:"input_tokens"`
	OutputTokens      int64          `json:"output_tokens"`
	StartedAt         string         `json:"started_at" binding:"required"`
	EndedAt           string         `json:"ended_at"`
	RawPayload        map[string]any `json:"raw_payload"`
}

func (h *BillingHandler) ImportBillLines(c *gin.Context) {
	var req importBillLinesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	lines := make([]billingapp.ImportBillLineInput, 0, len(req.Lines))
	for _, line := range req.Lines {
		startedAt, ok := parseRequiredTime(c, "started_at", line.StartedAt)
		if !ok {
			return
		}
		endedAt, ok := parseOptionalNamedTime(c, "ended_at", line.EndedAt)
		if !ok {
			return
		}
		lines = append(lines, billingapp.ImportBillLineInput{
			SupplierID:        line.SupplierID,
			Source:            line.Source,
			ExternalBillID:    line.ExternalBillID,
			ExternalRequestID: line.ExternalRequestID,
			Model:             line.Model,
			Currency:          line.Currency,
			CostCents:         line.CostCents,
			InputTokens:       line.InputTokens,
			OutputTokens:      line.OutputTokens,
			StartedAt:         *startedAt,
			EndedAt:           endedAt,
			RawPayload:        line.RawPayload,
		})
	}

	items, err := h.service.ImportBillLines(c.Request.Context(), lines)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, gin.H{"items": items, "total": len(items)})
}

func (h *BillingHandler) ListBillLines(c *gin.Context) {
	items, err := h.service.ListBillLines(c.Request.Context(), billingapp.BillLineFilter{
		SupplierID: parseInt64Query(c, "supplier_id"),
		Limit:      parseIntQuery(c, "limit"),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

func parseRequiredTime(c *gin.Context, field string, value string) (*time.Time, bool) {
	if value == "" {
		response.BadRequest(c, field+" is required")
		return nil, false
	}
	return parseOptionalNamedTime(c, field, value)
}

func parseOptionalNamedTime(c *gin.Context, field string, value string) (*time.Time, bool) {
	if value == "" {
		return nil, true
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		response.BadRequest(c, "invalid "+field+", expected RFC3339")
		return nil, false
	}
	return &t, true
}
