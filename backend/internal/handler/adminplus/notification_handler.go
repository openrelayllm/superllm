package adminplus

import (
	notificationsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	repo notificationsapp.Repository
}

func NewNotificationHandler(repo notificationsapp.Repository) *NotificationHandler {
	return &NotificationHandler{repo: repo}
}

func (h *NotificationHandler) ListDeliveries(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.repo.ListDeliveries(c.Request.Context(), notificationsapp.DeliveryFilter{
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
