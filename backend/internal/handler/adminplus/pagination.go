package adminplus

import (
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	defaultAdminPlusPageSize = 20
	maxAdminPlusPageSize     = 1000
)

type paginationRequest struct {
	Page     int
	PageSize int
	Limit    int
	Offset   int
}

func parsePagination(c *gin.Context) paginationRequest {
	page := parsePositiveIntQuery(c, "page", 1)
	pageSize := parsePositiveIntQuery(c, "page_size", 0)
	if pageSize == 0 {
		pageSize = parsePositiveIntQuery(c, "limit", defaultAdminPlusPageSize)
	}
	if pageSize <= 0 {
		pageSize = defaultAdminPlusPageSize
	}
	if pageSize > maxAdminPlusPageSize {
		pageSize = maxAdminPlusPageSize
	}
	return paginationRequest{
		Page:     page,
		PageSize: pageSize,
		Limit:    pageSize,
		Offset:   (page - 1) * pageSize,
	}
}

func parsePositiveIntQuery(c *gin.Context, name string, fallback int) int {
	raw := c.Query(name)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func paginatedData(items any, total int, page paginationRequest) gin.H {
	pages := 0
	if page.PageSize > 0 && total > 0 {
		pages = int(math.Ceil(float64(total) / float64(page.PageSize)))
	}
	return gin.H{
		"items":     items,
		"total":     total,
		"page":      page.Page,
		"page_size": page.PageSize,
		"pages":     pages,
	}
}

func fetchLimitForPagination(page paginationRequest) int {
	limit := page.Offset + page.PageSize
	if limit < maxAdminPlusPageSize {
		return maxAdminPlusPageSize
	}
	return limit
}

func paginateSlice[T any](items []T, page paginationRequest) ([]T, int) {
	total := len(items)
	if page.Offset >= total {
		return []T{}, total
	}
	end := page.Offset + page.PageSize
	if end > total {
		end = total
	}
	return items[page.Offset:end], total
}
