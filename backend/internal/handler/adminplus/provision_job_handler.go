package adminplus

import (
	"net/http"
	"strconv"

	provisionjobsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/provisionjobs"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type ProvisionJobHandler struct {
	service *provisionjobsapp.Service
}

func NewProvisionJobHandler(service *provisionjobsapp.Service) *ProvisionJobHandler {
	return &ProvisionJobHandler{service: service}
}

func (h *ProvisionJobHandler) Get(c *gin.Context) {
	jobID, ok := parseProvisionJobID(c)
	if !ok {
		return
	}
	job, err := h.service.Get(c.Request.Context(), jobID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, job)
}

func (h *ProvisionJobHandler) List(c *gin.Context) {
	page := parsePagination(c)
	supplierID := int64(0)
	if raw := c.Query("supplier_id"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			response.Error(c, http.StatusBadRequest, "invalid supplier id")
			return
		}
		supplierID = value
	}
	items, err := h.service.List(c.Request.Context(), provisionjobsapp.ListFilter{
		SupplierID: supplierID,
		Status:     adminplusdomain.SupplierProvisionStatus(c.Query("status")),
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func parseProvisionJobID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("jobID"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid supplier provision job id")
		return 0, false
	}
	return id, true
}
