package adminplus

import (
	"net/http"
	"strconv"

	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type BalanceHandler struct {
	service *balancesapp.Service
}

func NewBalanceHandler(service *balancesapp.Service) *BalanceHandler {
	return &BalanceHandler{service: service}
}

type recordBalanceSnapshotRequest struct {
	SupplierID               int64          `json:"supplier_id" binding:"required"`
	Source                   string         `json:"source"`
	RuntimeStatus            string         `json:"runtime_status"`
	BalanceCents             int64          `json:"balance_cents"`
	Currency                 string         `json:"currency"`
	LowBalanceThresholdCents int64          `json:"low_balance_threshold_cents"`
	RawPayload               map[string]any `json:"raw_payload"`
	CapturedAt               string         `json:"captured_at"`
}

func (h *BalanceHandler) RecordSnapshot(c *gin.Context) {
	var req recordBalanceSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	capturedAt, ok := parseOptionalTime(c, req.CapturedAt)
	if !ok {
		return
	}
	event, snapshot, err := h.service.RecordSnapshot(c.Request.Context(), balancesapp.RecordSnapshotInput{
		SupplierID:               req.SupplierID,
		Source:                   req.Source,
		RuntimeStatus:            adminplusdomain.NormalizeSupplierRuntimeStatus(req.RuntimeStatus),
		BalanceCents:             req.BalanceCents,
		Currency:                 req.Currency,
		LowBalanceThresholdCents: req.LowBalanceThresholdCents,
		RawPayload:               req.RawPayload,
		CapturedAt:               capturedAt,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, gin.H{"snapshot": snapshot, "event": event})
}

func (h *BalanceHandler) ListSnapshots(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListSnapshots(c.Request.Context(), balancesapp.SnapshotFilter{
		SupplierID: parseInt64Query(c, "supplier_id"),
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *BalanceHandler) ListEvents(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListEvents(c.Request.Context(), balancesapp.EventFilter{
		SupplierID: parseInt64Query(c, "supplier_id"),
		Status:     adminplusdomain.BalanceEventStatus(c.Query("status")),
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *BalanceHandler) AcknowledgeEvent(c *gin.Context) {
	id, ok := parseBalanceEventID(c)
	if !ok {
		return
	}
	event, err := h.service.AcknowledgeEvent(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, event)
}

func parseBalanceEventID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid balance event id")
		return 0, false
	}
	return id, true
}
