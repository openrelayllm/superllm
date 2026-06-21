package adminplus

import (
	"time"

	usagecostsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/usagecosts"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type UsageCostHandler struct {
	service *usagecostsapp.Service
}

func NewUsageCostHandler(service *usagecostsapp.Service) *UsageCostHandler {
	return &UsageCostHandler{service: service}
}

type importUsageCostLinesRequest struct {
	Lines []usageCostLineDTO `json:"lines" binding:"required"`
}

type syncSupplierUsageCostsRequest struct {
	StartedAt string `json:"started_at" binding:"required"`
	EndedAt   string `json:"ended_at" binding:"required"`
}

type usageCostLineDTO struct {
	SupplierID          int64          `json:"supplier_id" binding:"required"`
	Source              string         `json:"source"`
	ExternalUsageCostID string         `json:"external_usage_cost_id"`
	ExternalRequestID   string         `json:"external_request_id"`
	APIKeyName          string         `json:"api_key_name"`
	Model               string         `json:"model" binding:"required"`
	Endpoint            string         `json:"endpoint"`
	RequestType         string         `json:"request_type"`
	BillingMode         string         `json:"billing_mode"`
	ReasoningEffort     string         `json:"reasoning_effort"`
	Currency            string         `json:"currency"`
	CostCents           int64          `json:"cost_cents"`
	InputTokens         int64          `json:"input_tokens"`
	OutputTokens        int64          `json:"output_tokens"`
	CacheReadTokens     int64          `json:"cache_read_tokens"`
	TotalTokens         int64          `json:"total_tokens"`
	FirstTokenMS        int64          `json:"first_token_ms"`
	DurationMS          int64          `json:"duration_ms"`
	UserAgent           string         `json:"user_agent"`
	StartedAt           string         `json:"started_at" binding:"required"`
	EndedAt             string         `json:"ended_at"`
	RawPayload          map[string]any `json:"raw_payload"`
}

func (h *UsageCostHandler) ImportUsageCostLines(c *gin.Context) {
	var req importUsageCostLinesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	lines := make([]usagecostsapp.ImportUsageCostLineInput, 0, len(req.Lines))
	for _, line := range req.Lines {
		startedAt, ok := parseRequiredTime(c, "started_at", line.StartedAt)
		if !ok {
			return
		}
		endedAt, ok := parseOptionalNamedTime(c, "ended_at", line.EndedAt)
		if !ok {
			return
		}
		lines = append(lines, usagecostsapp.ImportUsageCostLineInput{
			SupplierID:          line.SupplierID,
			Source:              line.Source,
			ExternalUsageCostID: line.ExternalUsageCostID,
			ExternalRequestID:   line.ExternalRequestID,
			APIKeyName:          line.APIKeyName,
			Model:               line.Model,
			Endpoint:            line.Endpoint,
			RequestType:         line.RequestType,
			BillingMode:         line.BillingMode,
			ReasoningEffort:     line.ReasoningEffort,
			Currency:            line.Currency,
			CostCents:           line.CostCents,
			InputTokens:         line.InputTokens,
			OutputTokens:        line.OutputTokens,
			CacheReadTokens:     line.CacheReadTokens,
			TotalTokens:         line.TotalTokens,
			FirstTokenMS:        line.FirstTokenMS,
			DurationMS:          line.DurationMS,
			UserAgent:           line.UserAgent,
			StartedAt:           *startedAt,
			EndedAt:             endedAt,
			RawPayload:          line.RawPayload,
		})
	}

	items, err := h.service.ImportUsageCostLines(c.Request.Context(), lines)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, gin.H{"items": items, "total": len(items)})
}

func (h *UsageCostHandler) ListUsageCostLines(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListUsageCostLines(c.Request.Context(), usagecostsapp.UsageCostLineFilter{
		SupplierID: parseInt64Query(c, "supplier_id"),
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *UsageCostHandler) SyncSupplierUsageCosts(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	var req syncSupplierUsageCostsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	startedAt, ok := parseRequiredTime(c, "started_at", req.StartedAt)
	if !ok {
		return
	}
	endedAt, ok := parseRequiredTime(c, "ended_at", req.EndedAt)
	if !ok {
		return
	}
	result, err := h.service.SyncFromSession(c.Request.Context(), usagecostsapp.SyncFromSessionInput{
		SupplierID: supplierID,
		StartedAt:  *startedAt,
		EndedAt:    *endedAt,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, result)
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
