package adminplus

import (
	"context"
	"net/http"
	"strconv"

	provisionjobsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/provisionjobs"
	supplierkeysapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/supplierkeys"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type SupplierKeyHandler struct {
	service       *supplierkeysapp.Service
	provisionJobs *provisionjobsapp.Service
}

func NewSupplierKeyHandler(service *supplierkeysapp.Service) *SupplierKeyHandler {
	return &SupplierKeyHandler{service: service}
}

func NewSupplierKeyHandlerWithProvisionJobs(service *supplierkeysapp.Service, provisionJobs *provisionjobsapp.Service) *SupplierKeyHandler {
	return &SupplierKeyHandler{service: service, provisionJobs: provisionJobs}
}

type provisionSupplierKeyRequest struct {
	SupplierGroupID            int64    `json:"supplier_group_id" binding:"required"`
	Name                       string   `json:"name"`
	QuotaUSD                   float64  `json:"quota_usd"`
	ExpiresInDays              *int     `json:"expires_in_days"`
	LocalAccountPlatform       string   `json:"local_account_platform"`
	LocalAccountName           string   `json:"local_account_name"`
	LocalAccountBaseURL        string   `json:"local_account_base_url"`
	LocalAccountConcurrency    int      `json:"local_account_concurrency"`
	LocalAccountPriority       int      `json:"local_account_priority"`
	LocalAccountRateMultiplier *float64 `json:"local_account_rate_multiplier"`
	LocalAccountGroupIDs       []int64  `json:"local_account_group_ids"`
	RuntimeStatus              string   `json:"runtime_status"`
	HealthStatus               string   `json:"health_status"`
	BalanceThresholdCents      int64    `json:"balance_threshold_cents"`
	BalanceCents               int64    `json:"balance_cents"`
	BalanceCurrency            string   `json:"balance_currency"`
}

type ensureSupplierKeysRequest struct {
	LocalAccountBaseURL     string `json:"local_account_base_url"`
	LocalAccountConcurrency int    `json:"local_account_concurrency"`
	LocalAccountPriority    int    `json:"local_account_priority"`
	RuntimeStatus           string `json:"runtime_status"`
	HealthStatus            string `json:"health_status"`
	BalanceThresholdCents   int64  `json:"balance_threshold_cents"`
	BalanceCents            int64  `json:"balance_cents"`
	BalanceCurrency         string `json:"balance_currency"`
}

type repairSupplierKeyBindingRequest struct {
	LocalSub2APIAccountID     int64  `json:"local_sub2api_account_id" binding:"required"`
	RuntimeStatus             string `json:"runtime_status"`
	HealthStatus              string `json:"health_status"`
	ConfiguredConcurrency     int    `json:"configured_concurrency"`
	BalanceThresholdCents     int64  `json:"balance_threshold_cents"`
	BalanceCents              int64  `json:"balance_cents"`
	BalanceCurrency           string `json:"balance_currency"`
	SupplierAccountIdentifier string `json:"supplier_account_identifier"`
	SupplierAccountLabel      string `json:"supplier_account_label"`
}

func (h *SupplierKeyHandler) Provision(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	var req provisionSupplierKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if h.provisionJobs == nil {
		input := supplierkeysapp.ProvisionKeyInput{
			SupplierID:                 supplierID,
			SupplierGroupID:            req.SupplierGroupID,
			Name:                       req.Name,
			QuotaUSD:                   req.QuotaUSD,
			ExpiresInDays:              req.ExpiresInDays,
			LocalAccountPlatform:       req.LocalAccountPlatform,
			LocalAccountName:           req.LocalAccountName,
			LocalAccountBaseURL:        req.LocalAccountBaseURL,
			LocalAccountConcurrency:    req.LocalAccountConcurrency,
			LocalAccountPriority:       req.LocalAccountPriority,
			LocalAccountRateMultiplier: req.LocalAccountRateMultiplier,
			LocalAccountGroupIDs:       req.LocalAccountGroupIDs,
			RuntimeStatus:              adminplusdomain.NormalizeSupplierRuntimeStatus(req.RuntimeStatus),
			HealthStatus:               adminplusdomain.NormalizeSupplierHealthStatus(req.HealthStatus),
			BalanceThresholdCents:      req.BalanceThresholdCents,
			BalanceCents:               req.BalanceCents,
			BalanceCurrency:            req.BalanceCurrency,
		}
		executeAdminPlusIdempotentJSON(c, "admin-plus.supplier-key.provision", input, 0, http.StatusCreated, func(ctx context.Context) (any, error) {
			return h.service.Provision(ctx, input)
		})
		return
	}
	result, err := h.provisionJobs.Submit(c.Request.Context(), provisionjobsapp.SubmitInput{
		JobType:         adminplusdomain.SupplierProvisionJobTypeProvisionGroupKey,
		SupplierID:      supplierID,
		SupplierGroupID: req.SupplierGroupID,
		IdempotencyKey:  c.GetHeader("Idempotency-Key"),
		RequestedBy:     currentAdminUserID(c),
		Request:         provisionRequestSnapshot(req),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Accepted(c, result)
}

func (h *SupplierKeyHandler) EnsureAll(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	var req ensureSupplierKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if h.provisionJobs == nil {
		input := supplierkeysapp.EnsureAllInput{
			SupplierID:              supplierID,
			LocalAccountBaseURL:     req.LocalAccountBaseURL,
			LocalAccountConcurrency: req.LocalAccountConcurrency,
			LocalAccountPriority:    req.LocalAccountPriority,
			RuntimeStatus:           adminplusdomain.NormalizeSupplierRuntimeStatus(req.RuntimeStatus),
			HealthStatus:            adminplusdomain.NormalizeSupplierHealthStatus(req.HealthStatus),
			BalanceThresholdCents:   req.BalanceThresholdCents,
			BalanceCents:            req.BalanceCents,
			BalanceCurrency:         req.BalanceCurrency,
		}
		executeAdminPlusIdempotentJSON(c, "admin-plus.supplier-key.ensure-all", input, 0, http.StatusCreated, func(ctx context.Context) (any, error) {
			return h.service.EnsureAll(ctx, input)
		})
		return
	}
	result, err := h.provisionJobs.Submit(c.Request.Context(), provisionjobsapp.SubmitInput{
		JobType:        adminplusdomain.SupplierProvisionJobTypeProvisionAllGroupKeys,
		SupplierID:     supplierID,
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
		RequestedBy:    currentAdminUserID(c),
		Request:        ensureAllRequestSnapshot(req),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Accepted(c, result)
}

func (h *SupplierKeyHandler) RepairBinding(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	keyID, ok := parseSupplierKeyID(c)
	if !ok {
		return
	}
	var req repairSupplierKeyBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	input := supplierkeysapp.RepairBindingInput{
		SupplierID:                supplierID,
		KeyID:                     keyID,
		LocalSub2APIAccountID:     req.LocalSub2APIAccountID,
		RuntimeStatus:             adminplusdomain.NormalizeSupplierRuntimeStatus(req.RuntimeStatus),
		HealthStatus:              adminplusdomain.NormalizeSupplierHealthStatus(req.HealthStatus),
		ConfiguredConcurrency:     req.ConfiguredConcurrency,
		BalanceThresholdCents:     req.BalanceThresholdCents,
		BalanceCents:              req.BalanceCents,
		BalanceCurrency:           req.BalanceCurrency,
		SupplierAccountIdentifier: req.SupplierAccountIdentifier,
		SupplierAccountLabel:      req.SupplierAccountLabel,
	}
	executeAdminPlusIdempotentJSON(c, "admin-plus.supplier-key.repair-binding", input, 0, http.StatusOK, func(ctx context.Context) (any, error) {
		return h.service.RepairBinding(ctx, input)
	})
}

func (h *SupplierKeyHandler) List(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	page := parsePagination(c)
	items, err := h.service.List(c.Request.Context(), supplierkeysapp.ListFilter{
		SupplierID: supplierID,
		Status:     adminplusdomain.NormalizeSupplierKeyStatus(c.Query("status")),
		Query:      c.Query("q"),
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func parseSupplierKeyID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("keyID"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid supplier key id")
		return 0, false
	}
	return id, true
}

func provisionRequestSnapshot(req provisionSupplierKeyRequest) map[string]any {
	return map[string]any{
		"supplier_group_id":             req.SupplierGroupID,
		"name":                          req.Name,
		"quota_usd":                     req.QuotaUSD,
		"expires_in_days":               req.ExpiresInDays,
		"local_account_platform":        req.LocalAccountPlatform,
		"local_account_name":            req.LocalAccountName,
		"local_account_base_url":        req.LocalAccountBaseURL,
		"local_account_concurrency":     req.LocalAccountConcurrency,
		"local_account_priority":        req.LocalAccountPriority,
		"local_account_rate_multiplier": req.LocalAccountRateMultiplier,
		"local_account_group_ids":       req.LocalAccountGroupIDs,
		"runtime_status":                req.RuntimeStatus,
		"health_status":                 req.HealthStatus,
		"balance_threshold_cents":       req.BalanceThresholdCents,
		"balance_cents":                 req.BalanceCents,
		"balance_currency":              req.BalanceCurrency,
	}
}

func ensureAllRequestSnapshot(req ensureSupplierKeysRequest) map[string]any {
	return map[string]any{
		"local_account_base_url":    req.LocalAccountBaseURL,
		"local_account_concurrency": req.LocalAccountConcurrency,
		"local_account_priority":    req.LocalAccountPriority,
		"runtime_status":            req.RuntimeStatus,
		"health_status":             req.HealthStatus,
		"balance_threshold_cents":   req.BalanceThresholdCents,
		"balance_cents":             req.BalanceCents,
		"balance_currency":          req.BalanceCurrency,
	}
}
