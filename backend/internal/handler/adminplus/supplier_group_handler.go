package adminplus

import (
	"net/http"
	"strconv"

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

type updateSupplierGroupKeyCapacityRequest struct {
	KeyLimitPolicy string `json:"key_limit_policy"`
	KeyLimitValue  int    `json:"key_limit_value"`
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

func (h *SupplierGroupHandler) ListAll(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.List(c.Request.Context(), suppliergroupsapp.ListFilter{
		SupplierID: parseInt64Query(c, "supplier_id"),
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

func (h *SupplierGroupHandler) UpdateKeyCapacity(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	groupID, err := strconv.ParseInt(c.Param("groupID"), 10, 64)
	if err != nil || groupID <= 0 {
		response.ErrorWithDetails(c, http.StatusBadRequest, "invalid supplier group id", "SUPPLIER_GROUP_ID_INVALID", nil)
		return
	}
	var req updateSupplierGroupKeyCapacityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithDetails(c, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST_BODY", nil)
		return
	}
	item, err := h.service.UpdateKeyCapacity(c.Request.Context(), suppliergroupsapp.UpdateKeyCapacityInput{
		SupplierID:      supplierID,
		SupplierGroupID: groupID,
		KeyLimitPolicy:  req.KeyLimitPolicy,
		KeyLimitValue:   req.KeyLimitValue,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, item)
}

func (h *SupplierGroupHandler) ListEvents(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	page := parsePagination(c)
	lowRate := parseOptionalBoolQuery(c, "low_rate")
	items, err := h.service.ListChangeEvents(c.Request.Context(), suppliergroupsapp.EventFilter{
		SupplierID: supplierID,
		Direction:  adminplusdomain.SupplierGroupChangeDirection(c.Query("direction")),
		LowRate:    lowRate,
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
