package adminplus

import (
	"net/http"
	"strconv"
	"strings"

	channelchecksapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/channelchecks"
	provisionjobsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/provisionjobs"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type ChannelCheckHandler struct {
	service       *channelchecksapp.Service
	provisionJobs *provisionjobsapp.Service
}

func NewChannelCheckHandler(service *channelchecksapp.Service) *ChannelCheckHandler {
	return &ChannelCheckHandler{service: service}
}

func NewChannelCheckHandlerWithProvisionJobs(service *channelchecksapp.Service, provisionJobs *provisionjobsapp.Service) *ChannelCheckHandler {
	return &ChannelCheckHandler{service: service, provisionJobs: provisionJobs}
}

type probeSupplierChannelRequest struct {
	SupplierGroupID         int64  `json:"supplier_group_id"`
	AutoPauseOnFailure      *bool  `json:"auto_pause_on_failure"`
	ProbeModel              string `json:"probe_model"`
	FirstTokenThresholdMS   int64  `json:"first_token_threshold_ms"`
	TotalLatencyThresholdMS int64  `json:"total_latency_threshold_ms"`
}

type syncSupplierChannelsRequest struct {
	CandidateLimit          int    `json:"candidate_limit"`
	AutoPauseOnFailure      *bool  `json:"auto_pause_on_failure"`
	ProbeModel              string `json:"probe_model"`
	FirstTokenThresholdMS   int64  `json:"first_token_threshold_ms"`
	TotalLatencyThresholdMS int64  `json:"total_latency_threshold_ms"`
}

type setChannelSchedulingRequest struct {
	SupplierGroupID int64 `json:"supplier_group_id"`
}

func (h *ChannelCheckHandler) ListBest(c *gin.Context) {
	ids := parseSupplierIDsQuery(c.Query("supplier_ids"))
	items, err := h.service.ListBest(c.Request.Context(), ids)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

func (h *ChannelCheckHandler) List(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	page := parsePagination(c)
	items, err := h.service.ListLatest(c.Request.Context(), supplierID, fetchLimitForPagination(page))
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *ChannelCheckHandler) Probe(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	var req probeSupplierChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	result, err := h.service.Check(c.Request.Context(), channelchecksapp.CheckInput{
		SupplierID:              supplierID,
		SupplierGroupID:         req.SupplierGroupID,
		AutoPauseOnFailure:      boolDefault(req.AutoPauseOnFailure, true),
		ProbeModel:              strings.TrimSpace(req.ProbeModel),
		FirstTokenThresholdMS:   req.FirstTokenThresholdMS,
		TotalLatencyThresholdMS: req.TotalLatencyThresholdMS,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, result)
}

func (h *ChannelCheckHandler) Sync(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	var req syncSupplierChannelsRequest
	_ = c.ShouldBindJSON(&req)
	if h.provisionJobs == nil {
		response.Error(c, http.StatusInternalServerError, "supplier provision job service is not configured")
		return
	}
	result, err := h.provisionJobs.Submit(c.Request.Context(), provisionjobsapp.SubmitInput{
		JobType:        adminplusdomain.SupplierProvisionJobTypeCheckSupplierChannels,
		SupplierID:     supplierID,
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
		RequestedBy:    currentAdminUserID(c),
		Request: map[string]any{
			"candidate_limit":            req.CandidateLimit,
			"auto_pause_on_failure":      boolDefault(req.AutoPauseOnFailure, true),
			"probe_model":                strings.TrimSpace(req.ProbeModel),
			"first_token_threshold_ms":   req.FirstTokenThresholdMS,
			"total_latency_threshold_ms": req.TotalLatencyThresholdMS,
		},
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Accepted(c, result)
}

func (h *ChannelCheckHandler) EnableScheduling(c *gin.Context) {
	h.setScheduling(c, true)
}

func (h *ChannelCheckHandler) PauseScheduling(c *gin.Context) {
	h.setScheduling(c, false)
}

func (h *ChannelCheckHandler) setScheduling(c *gin.Context, schedulable bool) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	var req setChannelSchedulingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	snapshot, err := h.service.SetScheduling(c.Request.Context(), supplierID, req.SupplierGroupID, schedulable)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, snapshot)
}

func parseSupplierIDsQuery(raw string) []int64 {
	parts := strings.Split(raw, ",")
	out := make([]int64, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err == nil && id > 0 {
			out = append(out, id)
		}
	}
	return out
}
