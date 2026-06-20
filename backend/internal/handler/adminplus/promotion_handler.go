package adminplus

import (
	"net/http"
	"strconv"

	promotionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/promotions"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type PromotionHandler struct {
	service *promotionsapp.Service
}

func NewPromotionHandler(service *promotionsapp.Service) *PromotionHandler {
	return &PromotionHandler{service: service}
}

type recordPromotionRequest struct {
	SupplierID       int64          `json:"supplier_id" binding:"required"`
	Source           string         `json:"source"`
	Type             string         `json:"type" binding:"required"`
	Title            string         `json:"title" binding:"required"`
	Description      string         `json:"description"`
	Currency         string         `json:"currency"`
	MinRechargeCents int64          `json:"min_recharge_cents"`
	BonusPercent     *float64       `json:"bonus_percent"`
	DiscountPercent  *float64       `json:"discount_percent"`
	RuntimeStatus    string         `json:"runtime_status"`
	BalanceCents     int64          `json:"balance_cents"`
	StartsAt         string         `json:"starts_at"`
	EndsAt           string         `json:"ends_at"`
	CapturedAt       string         `json:"captured_at"`
	RawPayload       map[string]any `json:"raw_payload"`
}

func (h *PromotionHandler) RecordPromotion(c *gin.Context) {
	var req recordPromotionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	startsAt, ok := parseOptionalTime(c, req.StartsAt)
	if !ok {
		return
	}
	endsAt, ok := parseOptionalTime(c, req.EndsAt)
	if !ok {
		return
	}
	capturedAt, ok := parseOptionalTime(c, req.CapturedAt)
	if !ok {
		return
	}
	event, err := h.service.RecordPromotion(c.Request.Context(), promotionsapp.RecordPromotionInput{
		SupplierID:       req.SupplierID,
		Source:           req.Source,
		Type:             adminplusdomain.PromotionType(req.Type),
		Title:            req.Title,
		Description:      req.Description,
		Currency:         req.Currency,
		MinRechargeCents: req.MinRechargeCents,
		BonusPercent:     req.BonusPercent,
		DiscountPercent:  req.DiscountPercent,
		RuntimeStatus:    adminplusdomain.NormalizeSupplierRuntimeStatus(req.RuntimeStatus),
		BalanceCents:     req.BalanceCents,
		StartsAt:         startsAt,
		EndsAt:           endsAt,
		CapturedAt:       capturedAt,
		RawPayload:       req.RawPayload,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, event)
}

func (h *PromotionHandler) ListEvents(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListEvents(c.Request.Context(), promotionsapp.EventFilter{
		SupplierID:     parseInt64Query(c, "supplier_id"),
		Status:         adminplusdomain.PromotionStatus(c.Query("status")),
		Recommendation: adminplusdomain.PromotionRecommendation(c.Query("recommendation")),
		Limit:          fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *PromotionHandler) AcknowledgeEvent(c *gin.Context) {
	id, ok := parsePromotionEventID(c)
	if !ok {
		return
	}
	event, err := h.service.AcknowledgeEvent(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, event)
}

func parsePromotionEventID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid promotion event id")
		return 0, false
	}
	return id, true
}
