package adminplus

import (
	"net/http"
	"strconv"

	healthapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/health"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	service *healthapp.Service
}

func NewHealthHandler(service *healthapp.Service) *HealthHandler {
	return &HealthHandler{service: service}
}

type recordHealthSampleRequest struct {
	SupplierID                   int64          `json:"supplier_id" binding:"required"`
	Source                       string         `json:"source"`
	Model                        string         `json:"model" binding:"required"`
	FirstTokenLatencyMS          int64          `json:"first_token_latency_ms"`
	TotalLatencyMS               int64          `json:"total_latency_ms"`
	StatusCode                   int            `json:"status_code"`
	ErrorClass                   string         `json:"error_class"`
	ObservedConcurrency          int            `json:"observed_concurrency"`
	AvailableConcurrency         *int           `json:"available_concurrency"`
	ConcurrencyLimit             *int           `json:"concurrency_limit"`
	FirstTokenThresholdMS        int64          `json:"first_token_threshold_ms"`
	TotalLatencyThresholdMS      int64          `json:"total_latency_threshold_ms"`
	ConcurrencySaturationPercent float64        `json:"concurrency_saturation_percent"`
	RawPayload                   map[string]any `json:"raw_payload"`
	CapturedAt                   string         `json:"captured_at"`
}

type probeOpenAIResponsesRequest struct {
	SupplierID                   int64   `json:"supplier_id" binding:"required"`
	SupplierAccountID            int64   `json:"supplier_account_id"`
	Model                        string  `json:"model"`
	Prompt                       string  `json:"prompt"`
	FirstTokenThresholdMS        int64   `json:"first_token_threshold_ms"`
	TotalLatencyThresholdMS      int64   `json:"total_latency_threshold_ms"`
	ConcurrencySaturationPercent float64 `json:"concurrency_saturation_percent"`
}

func (h *HealthHandler) RecordSample(c *gin.Context) {
	var req recordHealthSampleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	capturedAt, ok := parseOptionalTime(c, req.CapturedAt)
	if !ok {
		return
	}
	result, err := h.service.RecordSample(c.Request.Context(), healthapp.RecordSampleInput{
		SupplierID:                   req.SupplierID,
		Source:                       req.Source,
		Model:                        req.Model,
		FirstTokenLatencyMS:          req.FirstTokenLatencyMS,
		TotalLatencyMS:               req.TotalLatencyMS,
		StatusCode:                   req.StatusCode,
		ErrorClass:                   req.ErrorClass,
		ObservedConcurrency:          req.ObservedConcurrency,
		AvailableConcurrency:         req.AvailableConcurrency,
		ConcurrencyLimit:             req.ConcurrencyLimit,
		FirstTokenThresholdMS:        req.FirstTokenThresholdMS,
		TotalLatencyThresholdMS:      req.TotalLatencyThresholdMS,
		ConcurrencySaturationPercent: req.ConcurrencySaturationPercent,
		RawPayload:                   req.RawPayload,
		CapturedAt:                   capturedAt,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, result)
}

func (h *HealthHandler) ProbeOpenAIResponses(c *gin.Context) {
	var req probeOpenAIResponsesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	result, err := h.service.ProbeOpenAIResponses(c.Request.Context(), healthapp.ProbeInput{
		SupplierID:                   req.SupplierID,
		SupplierAccountID:            req.SupplierAccountID,
		Model:                        req.Model,
		Prompt:                       req.Prompt,
		FirstTokenThresholdMS:        req.FirstTokenThresholdMS,
		TotalLatencyThresholdMS:      req.TotalLatencyThresholdMS,
		ConcurrencySaturationPercent: req.ConcurrencySaturationPercent,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, result)
}

func (h *HealthHandler) ListSamples(c *gin.Context) {
	items, err := h.service.ListSamples(c.Request.Context(), healthapp.SampleFilter{
		SupplierID: parseInt64Query(c, "supplier_id"),
		Model:      c.Query("model"),
		Limit:      parseIntQuery(c, "limit"),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

func (h *HealthHandler) ListEvents(c *gin.Context) {
	items, err := h.service.ListEvents(c.Request.Context(), healthapp.EventFilter{
		SupplierID: parseInt64Query(c, "supplier_id"),
		Status:     adminplusdomain.HealthEventStatus(c.Query("status")),
		Type:       adminplusdomain.HealthEventType(c.Query("type")),
		Limit:      parseIntQuery(c, "limit"),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

func (h *HealthHandler) AcknowledgeEvent(c *gin.Context) {
	id, ok := parseHealthEventID(c)
	if !ok {
		return
	}
	event, err := h.service.AcknowledgeEvent(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, event)
}

func parseHealthEventID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid health event id")
		return 0, false
	}
	return id, true
}
