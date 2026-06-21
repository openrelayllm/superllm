package adminplus

import (
	"strconv"
	"time"

	sub2apiapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sub2api"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type Sub2APIHandler struct {
	service            *sub2apiapp.Service
	accountTestService *service.AccountTestService
}

func NewSub2APIHandler(service *sub2apiapp.Service) *Sub2APIHandler {
	return &Sub2APIHandler{service: service}
}

func NewSub2APIHandlerWithAccountTest(service *sub2apiapp.Service, accountTestService *service.AccountTestService) *Sub2APIHandler {
	return &Sub2APIHandler{
		service:            service,
		accountTestService: accountTestService,
	}
}

type testLocalAccountRequest struct {
	ModelID string `json:"model_id"`
	Prompt  string `json:"prompt"`
	Mode    string `json:"mode"`
}

func (h *Sub2APIHandler) ListLocalAccountModels(c *gin.Context) {
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	if h.accountTestService == nil {
		response.InternalError(c, "account test service is not configured")
		return
	}

	models, err := h.accountTestService.GetAvailableModels(c.Request.Context(), accountID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, models)
}

func (h *Sub2APIHandler) TestLocalAccount(c *gin.Context) {
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	if h.accountTestService == nil {
		response.InternalError(c, "account test service is not configured")
		return
	}

	var req testLocalAccountRequest
	_ = c.ShouldBindJSON(&req)
	_ = h.accountTestService.TestAccountConnection(c, accountID, req.ModelID, req.Prompt, req.Mode)
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

func parseAccountIDParam(c *gin.Context) (int64, bool) {
	accountID, err := strconv.ParseInt(c.Param("accountID"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "invalid account id")
		return 0, false
	}
	return accountID, true
}
