package adminplus

import (
	"strconv"

	notificationsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	service *notificationsapp.Service
}

func NewNotificationHandler(service *notificationsapp.Service) *NotificationHandler {
	return &NotificationHandler{service: service}
}

func (h *NotificationHandler) CenterStatus(c *gin.Context) {
	response.Success(c, h.service.CenterStatus(c.Request.Context()))
}

func (h *NotificationHandler) Settings(c *gin.Context) {
	response.Success(c, h.service.Settings(c.Request.Context()))
}

func (h *NotificationHandler) UpdateSettings(c *gin.Context) {
	var req adminplusdomain.NotificationSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	settings, err := h.service.UpdateSettings(c.Request.Context(), req)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, settings)
}

func (h *NotificationHandler) Test(c *gin.Context) {
	var req notificationsapp.TestInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	delivery, err := h.service.Test(c.Request.Context(), req)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Accepted(c, delivery)
}

func (h *NotificationHandler) ListDeliveries(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListDeliveries(c.Request.Context(), notificationsapp.DeliveryFilter{
		SupplierID: parseInt64Query(c, "supplier_id"),
		Channel:    adminplusdomain.NotificationChannel(c.Query("channel")),
		Status:     adminplusdomain.NotificationStatus(c.Query("status")),
		EventType:  c.Query("event_type"),
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *NotificationHandler) RetryDelivery(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid notification delivery id")
		return
	}
	delivery, err := h.service.RetryDelivery(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Accepted(c, delivery)
}
