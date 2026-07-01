package adminplus

import (
	"time"

	costsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/costs"
	provisionjobsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/provisionjobs"
	schedulerapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/scheduler"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type CostHandler struct {
	service       *costsapp.Service
	provisionJobs *provisionjobsapp.Service
	scheduler     *schedulerapp.Service
}

func NewCostHandler(service *costsapp.Service) *CostHandler {
	return &CostHandler{service: service}
}

func NewCostHandlerWithProvisionJobs(service *costsapp.Service, provisionJobs *provisionjobsapp.Service) *CostHandler {
	return &CostHandler{service: service, provisionJobs: provisionJobs}
}

func NewCostHandlerWithProvisionJobsAndScheduler(service *costsapp.Service, provisionJobs *provisionjobsapp.Service, scheduler *schedulerapp.Service) *CostHandler {
	return &CostHandler{service: service, provisionJobs: provisionJobs, scheduler: scheduler}
}

type syncSupplierCostsRequest struct {
	StartedAt                      string `json:"started_at"`
	EndedAt                        string `json:"ended_at"`
	IncludeFundingTransactions     *bool  `json:"include_funding_transactions"`
	IncludeEntitlementTransactions *bool  `json:"include_entitlement_transactions"`
	IncludeUsageCostLines          *bool  `json:"include_usage_cost_lines"`
	IncludeBalanceSnapshot         *bool  `json:"include_balance_snapshot"`
	LowBalanceThresholdCents       int64  `json:"low_balance_threshold_cents"`
}

type backfillSupplierCostsRequest struct {
	SupplierID                     int64  `json:"supplier_id"`
	StartedAt                      string `json:"started_at"`
	EndedAt                        string `json:"ended_at"`
	IncludeFundingTransactions     *bool  `json:"include_funding_transactions"`
	IncludeEntitlementTransactions *bool  `json:"include_entitlement_transactions"`
	IncludeUsageCostLines          *bool  `json:"include_usage_cost_lines"`
	IncludeBalanceSnapshot         *bool  `json:"include_balance_snapshot"`
	LowBalanceThresholdCents       int64  `json:"low_balance_threshold_cents"`
}

func (h *CostHandler) SyncSupplierCosts(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	var req syncSupplierCostsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	startedAt, ok := parseOptionalNamedTime(c, "started_at", req.StartedAt)
	if !ok {
		return
	}
	endedAt, ok := parseOptionalNamedTime(c, "ended_at", req.EndedAt)
	if !ok {
		return
	}
	if h.provisionJobs == nil {
		err := infraerrors.New(500, "ADMIN_PLUS_INTERNAL_ERROR", "supplier cost sync job service is not configured")
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.provisionJobs.Submit(c.Request.Context(), provisionjobsapp.SubmitInput{
		JobType:        adminplusdomain.SupplierProvisionJobTypeSyncSupplierCosts,
		SupplierID:     supplierID,
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
		RequestedBy:    currentAdminUserID(c),
		Request: map[string]any{
			"started_at":                       timePtrRFC3339(startedAt),
			"ended_at":                         timePtrRFC3339(endedAt),
			"include_funding_transactions":     boolDefault(req.IncludeFundingTransactions, true),
			"include_entitlement_transactions": boolDefault(req.IncludeEntitlementTransactions, true),
			"include_usage_cost_lines":         boolDefault(req.IncludeUsageCostLines, true),
			"include_balance_snapshot":         boolDefault(req.IncludeBalanceSnapshot, true),
			"low_balance_threshold_cents":      req.LowBalanceThresholdCents,
		},
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Accepted(c, result)
}

func (h *CostHandler) BackfillSupplierCosts(c *gin.Context) {
	var req backfillSupplierCostsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	startedAt, ok := parseOptionalNamedTime(c, "started_at", req.StartedAt)
	if !ok {
		return
	}
	endedAt, ok := parseOptionalNamedTime(c, "ended_at", req.EndedAt)
	if !ok {
		return
	}
	if h.scheduler == nil {
		err := infraerrors.New(500, "ADMIN_PLUS_INTERNAL_ERROR", "scheduler service is not configured")
		response.ErrorFrom(c, err)
		return
	}
	run, err := h.scheduler.EnqueueCostHistoryBackfill(c.Request.Context(), schedulerapp.CostBackfillInput{
		Mode:                           "manual:cost-history-backfill",
		SupplierID:                     req.SupplierID,
		StartedAt:                      startedAt,
		EndedAt:                        endedAt,
		IncludeFundingTransactions:     boolDefault(req.IncludeFundingTransactions, true),
		IncludeEntitlementTransactions: boolDefault(req.IncludeEntitlementTransactions, true),
		IncludeUsageCostLines:          boolDefault(req.IncludeUsageCostLines, true),
		IncludeBalanceSnapshot:         boolDefault(req.IncludeBalanceSnapshot, true),
		LowBalanceThresholdCents:       req.LowBalanceThresholdCents,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Accepted(c, run)
}

func (h *CostHandler) ListSupplierSummaries(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListSnapshots(c.Request.Context(), costsapp.SummaryFilter{
		SupplierID: parseInt64Query(c, "supplier_id"),
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *CostHandler) GetLedgerOverview(c *gin.Context) {
	overview, err := h.service.GetLedgerOverview(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, overview)
}

func (h *CostHandler) GetSupplierSummary(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	items, err := h.service.ListSnapshots(c.Request.Context(), costsapp.SummaryFilter{
		SupplierID: supplierID,
		Limit:      10,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

func (h *CostHandler) ListFundingTransactions(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	page := parsePagination(c)
	items, err := h.service.ListFundingTransactions(c.Request.Context(), costsapp.TransactionFilter{
		SupplierID: supplierID,
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *CostHandler) ListEntitlementTransactions(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	page := parsePagination(c)
	items, err := h.service.ListEntitlementTransactions(c.Request.Context(), costsapp.TransactionFilter{
		SupplierID: supplierID,
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *CostHandler) ListLedgerEntries(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	page := parsePagination(c)
	items, err := h.service.ListLedgerEntries(c.Request.Context(), costsapp.LedgerFilter{
		SupplierID: supplierID,
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func boolDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func timePtrRFC3339(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
