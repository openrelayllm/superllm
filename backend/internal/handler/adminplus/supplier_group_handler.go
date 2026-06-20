package adminplus

import (
	"net/http"

	suppliergroupsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliergroups"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type SupplierGroupHandler struct {
	service *suppliergroupsapp.Service
}

func NewSupplierGroupHandler(service *suppliergroupsapp.Service) *SupplierGroupHandler {
	return &SupplierGroupHandler{service: service}
}

func (h *SupplierGroupHandler) Sync(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	result, err := h.service.Sync(c.Request.Context(), supplierID)
	if response.ErrorFrom(c, err) {
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
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
