package adminplus

import (
	"net/http"

	provisionjobsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/provisionjobs"
	suppliergroupsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliergroups"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

type SupplierGroupHandler struct {
	service       *suppliergroupsapp.Service
	provisionJobs *provisionjobsapp.Service
}

func NewSupplierGroupHandler(service *suppliergroupsapp.Service) *SupplierGroupHandler {
	return &SupplierGroupHandler{service: service}
}

func NewSupplierGroupHandlerWithProvisionJobs(service *suppliergroupsapp.Service, provisionJobs *provisionjobsapp.Service) *SupplierGroupHandler {
	return &SupplierGroupHandler{service: service, provisionJobs: provisionJobs}
}

func (h *SupplierGroupHandler) Sync(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	if h.provisionJobs == nil {
		result, err := h.service.Sync(c.Request.Context(), supplierID)
		if response.ErrorFrom(c, err) {
			return
		}
		c.JSON(http.StatusCreated, response.Response{Code: 0, Message: "success", Data: result})
		return
	}
	result, err := h.provisionJobs.Submit(c.Request.Context(), provisionjobsapp.SubmitInput{
		JobType:        adminplusdomain.SupplierProvisionJobTypeSyncGroups,
		SupplierID:     supplierID,
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
		RequestedBy:    currentAdminUserID(c),
		Request:        map[string]any{},
	})
	if response.ErrorFrom(c, err) {
		return
	}
	c.JSON(http.StatusAccepted, response.Response{Code: 0, Message: "accepted", Data: result})
}

func (h *SupplierGroupHandler) List(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	page := parsePagination(c)
	items, err := h.service.List(c.Request.Context(), suppliergroupsapp.ListFilter{
		SupplierID: supplierID,
		Status:     adminplusdomain.NormalizeSupplierGroupStatus(c.Query("status")),
		Query:      c.Query("q"),
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func currentAdminUserID(c *gin.Context) int64 {
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		return subject.UserID
	}
	return 0
}
