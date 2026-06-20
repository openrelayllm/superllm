package adminplus

import (
	"time"

	sub2apiapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sub2api"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Sub2APIHandler struct {
	service *sub2apiapp.Service
}

func NewSub2APIHandler(service *sub2apiapp.Service) *Sub2APIHandler {
	return &Sub2APIHandler{service: service}
}

func (h *Sub2APIHandler) ListLocalUsageLines(c *gin.Context) {
	page := parsePagination(c)
	filter, ok := parseUsageFilter(c)
	if !ok {
		return
	}
	filter.Limit = fetchLimitForPagination(page)
	items, err := h.service.ListLocalUsageLines(c.Request.Context(), filter)
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *Sub2APIHandler) ListLocalUsageSummaries(c *gin.Context) {
	page := parsePagination(c)
	filter, ok := parseUsageFilter(c)
	if !ok {
		return
	}
	filter.Limit = fetchLimitForPagination(page)
	items, err := h.service.ListLocalUsageSummaries(c.Request.Context(), filter)
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *Sub2APIHandler) ListLocalAccountUsageSummaries(c *gin.Context) {
	page := parsePagination(c)
	filter, ok := parseUsageFilter(c)
	if !ok {
		return
	}
	filter.Limit = fetchLimitForPagination(page)
	items, err := h.service.ListLocalAccountUsageSummaries(c.Request.Context(), filter)
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *Sub2APIHandler) ListAccountRuntime(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListAccountRuntime(c.Request.Context(), sub2apiapp.RuntimeFilter{
		AccountID: parseInt64Query(c, "account_id"),
		Query:     c.Query("q"),
		Limit:     fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func parseUsageFilter(c *gin.Context) (sub2apiapp.UsageFilter, bool) {
	from, ok := parseOptionalQueryTime(c, "from")
	if !ok {
		return sub2apiapp.UsageFilter{}, false
	}
	to, ok := parseOptionalQueryTime(c, "to")
	if !ok {
		return sub2apiapp.UsageFilter{}, false
	}
	return sub2apiapp.UsageFilter{
		AccountID: parseInt64Query(c, "account_id"),
		Model:     c.Query("model"),
		From:      valueOrZero(from),
		To:        valueOrZero(to),
		Limit:     parseIntQuery(c, "limit"),
	}, true
}

func parseOptionalQueryTime(c *gin.Context, name string) (*time.Time, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		response.BadRequest(c, "invalid "+name+", expected RFC3339")
		return nil, false
	}
	return &t, true
}

func valueOrZero(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}
