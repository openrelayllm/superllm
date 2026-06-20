package adminplus

import (
	"net/http"
	"strconv"
	"time"

	ratesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/rates"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type RateHandler struct {
	service *ratesapp.Service
}

func NewRateHandler(service *ratesapp.Service) *RateHandler {
	return &RateHandler{service: service}
}

type recordRateSnapshotRequest struct {
	SupplierID       int64                  `json:"supplier_id" binding:"required"`
	Source           string                 `json:"source"`
	CapturedAt       string                 `json:"captured_at"`
	ThresholdPercent float64                `json:"threshold_percent"`
	Entries          []rateSnapshotEntryDTO `json:"entries" binding:"required"`
}

type rateSnapshotEntryDTO struct {
	Model       string         `json:"model" binding:"required"`
	BillingMode string         `json:"billing_mode" binding:"required"`
	PriceItem   string         `json:"price_item" binding:"required"`
	Unit        string         `json:"unit" binding:"required"`
	Currency    string         `json:"currency"`
	PriceMicros int64          `json:"price_micros"`
	RawPayload  map[string]any `json:"raw_payload"`
}

func (h *RateHandler) RecordSnapshot(c *gin.Context) {
	var req recordRateSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	capturedAt, ok := parseOptionalTime(c, req.CapturedAt)
	if !ok {
		return
	}

	entries := make([]ratesapp.RateEntryInput, 0, len(req.Entries))
	for _, entry := range req.Entries {
		entries = append(entries, ratesapp.RateEntryInput{
			Model:       entry.Model,
			BillingMode: entry.BillingMode,
			PriceItem:   entry.PriceItem,
			Unit:        entry.Unit,
			Currency:    entry.Currency,
			PriceMicros: entry.PriceMicros,
			RawPayload:  entry.RawPayload,
		})
	}

	result, err := h.service.RecordSnapshot(c.Request.Context(), ratesapp.RecordSnapshotInput{
		SupplierID:       req.SupplierID,
		Source:           req.Source,
		CapturedAt:       capturedAt,
		ThresholdPercent: req.ThresholdPercent,
		Entries:          entries,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, result)
}

func (h *RateHandler) ListSnapshots(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListSnapshots(c.Request.Context(), ratesapp.SnapshotFilter{
		SupplierID: parseInt64Query(c, "supplier_id"),
		Model:      c.Query("model"),
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *RateHandler) ListEvents(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListChangeEvents(c.Request.Context(), ratesapp.EventFilter{
		SupplierID: parseInt64Query(c, "supplier_id"),
		Status:     adminplusdomain.RateChangeStatus(c.Query("status")),
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *RateHandler) AcknowledgeEvent(c *gin.Context) {
	id, ok := parseRateEventID(c)
	if !ok {
		return
	}
	event, err := h.service.AcknowledgeChangeEvent(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, event)
}

func parseRateEventID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid rate event id")
		return 0, false
	}
	return id, true
}

func parseOptionalTime(c *gin.Context, value string) (*time.Time, bool) {
	if value == "" {
		return nil, true
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid captured_at, expected RFC3339")
		return nil, false
	}
	return &t, true
}

func parseInt64Query(c *gin.Context, name string) int64 {
	raw := c.Query(name)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func parseIntQuery(c *gin.Context, name string) int {
	raw := c.Query(name)
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return value
}
