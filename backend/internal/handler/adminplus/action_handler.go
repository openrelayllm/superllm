package adminplus

import (
	"net/http"
	"strconv"

	actionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/actions"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type ActionHandler struct {
	service *actionsapp.Service
}

func NewActionHandler(service *actionsapp.Service) *ActionHandler {
	return &ActionHandler{service: service}
}

type generateActionsRequest struct {
	Suppliers       []supplierSignalDTO                   `json:"suppliers" binding:"required"`
	BalanceEvents   []*adminplusdomain.BalanceEvent       `json:"balance_events"`
	PromotionEvents []*adminplusdomain.PromotionEvent     `json:"promotion_events"`
	HealthEvents    []*adminplusdomain.HealthEvent        `json:"health_events"`
	Reconciliation  adminplusdomain.ReconciliationSummary `json:"reconciliation"`
	MinProfitMargin float64                               `json:"min_profit_margin"`
}

type supplierSignalDTO struct {
	SupplierID         int64  `json:"supplier_id" binding:"required"`
	Name               string `json:"name"`
	RuntimeStatus      string `json:"runtime_status"`
	HealthStatus       string `json:"health_status"`
	BalanceCents       int64  `json:"balance_cents"`
	Currency           string `json:"currency"`
	EffectiveCostCents int64  `json:"effective_cost_cents"`
}

type updateActionStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func (h *ActionHandler) Generate(c *gin.Context) {
	var req generateActionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	suppliers := make([]actionsapp.SupplierSignal, 0, len(req.Suppliers))
	for _, supplier := range req.Suppliers {
		suppliers = append(suppliers, actionsapp.SupplierSignal{
			SupplierID:         supplier.SupplierID,
			Name:               supplier.Name,
			RuntimeStatus:      adminplusdomain.NormalizeSupplierRuntimeStatus(supplier.RuntimeStatus),
			HealthStatus:       adminplusdomain.NormalizeSupplierHealthStatus(supplier.HealthStatus),
			BalanceCents:       supplier.BalanceCents,
			Currency:           supplier.Currency,
			EffectiveCostCents: supplier.EffectiveCostCents,
		})
	}
	result, err := h.service.Generate(c.Request.Context(), actionsapp.GenerateInput{
		Suppliers:       suppliers,
		BalanceEvents:   req.BalanceEvents,
		PromotionEvents: req.PromotionEvents,
		HealthEvents:    req.HealthEvents,
		Reconciliation:  req.Reconciliation,
		MinProfitMargin: req.MinProfitMargin,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *ActionHandler) ListRecommendations(c *gin.Context) {
	items, err := h.service.ListRecommendations(c.Request.Context(), actionsapp.RecommendationFilter{
		SupplierID: parseInt64Query(c, "supplier_id"),
		Status:     adminplusdomain.ActionStatus(c.Query("status")),
		Severity:   adminplusdomain.ActionSeverity(c.Query("severity")),
		Type:       adminplusdomain.ActionType(c.Query("type")),
		Limit:      parseIntQuery(c, "limit"),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

func (h *ActionHandler) UpdateRecommendationStatus(c *gin.Context) {
	id, ok := parseActionRecommendationID(c)
	if !ok {
		return
	}
	var req updateActionStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	item, err := h.service.UpdateRecommendationStatus(c.Request.Context(), id, adminplusdomain.ActionStatus(req.Status))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, item)
}

func parseActionRecommendationID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid action recommendation id")
		return 0, false
	}
	return id, true
}
