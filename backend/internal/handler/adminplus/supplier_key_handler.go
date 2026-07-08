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
	SyncProviderName           bool     `json:"sync_provider_name"`
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
	SyncProviderName         bool    `json:"sync_provider_name"`
	AllowPartial             bool    `json:"allow_partial"`
	SupplierGroupPriorityIDs []int64 `json:"supplier_group_priority_ids"`
	LocalAccountBaseURL      string  `json:"local_account_base_url"`
	LocalAccountConcurrency  int     `json:"local_account_concurrency"`
	LocalAccountPriority     int     `json:"local_account_priority"`
	LocalAccountGroupIDs     []int64 `json:"local_account_group_ids"`
	RuntimeStatus            string  `json:"runtime_status"`
	HealthStatus             string  `json:"health_status"`
	BalanceThresholdCents    int64   `json:"balance_threshold_cents"`
	BalanceCents             int64   `json:"balance_cents"`
	BalanceCurrency          string  `json:"balance_currency"`
}

type repairSupplierKeyBindingRequest struct {
	LocalSub2APIAccountID      int64    `json:"local_sub2api_account_id"`
	ManualSecret               string   `json:"manual_secret"`
	LocalAccountPlatform       string   `json:"local_account_platform"`
	LocalAccountName           string   `json:"local_account_name"`
	LocalAccountBaseURL        string   `json:"local_account_base_url"`
	LocalAccountPriority       int      `json:"local_account_priority"`
	LocalAccountRateMultiplier *float64 `json:"local_account_rate_multiplier"`
	LocalAccountGroupIDs       []int64  `json:"local_account_group_ids"`
	RuntimeStatus              string   `json:"runtime_status"`
	HealthStatus               string   `json:"health_status"`
	ConfiguredConcurrency      int      `json:"configured_concurrency"`
	BalanceThresholdCents      int64    `json:"balance_threshold_cents"`
	BalanceCents               int64    `json:"balance_cents"`
	BalanceCurrency            string   `json:"balance_currency"`
	SupplierAccountIdentifier  string   `json:"supplier_account_identifier"`
	SupplierAccountLabel       string   `json:"supplier_account_label"`
}

type importProviderKeyProjectionRequest struct {
	SupplierGroupID int64  `json:"supplier_group_id" binding:"required"`
	ExternalKeyID   string `json:"external_key_id"`
}

type importProviderKeyProjectionsRequest struct {
	Items []importProviderKeyProjectionRequest `json:"items" binding:"required"`
}

type disableSupplierKeyLocalProjectionRequest struct {
	Reason string `json:"reason"`
}

type providerSupplierKeyOperationRequest struct {
	Reason string `json:"reason"`
}

type standardizeSupplierKeyNamesRequest struct {
	SyncProviderName bool `json:"sync_provider_name"`
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
			SyncProviderName:           req.SyncProviderName,
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
		input := ensureSupplierKeysInput(supplierID, req)
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

func ensureSupplierKeysInput(supplierID int64, req ensureSupplierKeysRequest) supplierkeysapp.EnsureAllInput {
	return supplierkeysapp.EnsureAllInput{
		SupplierID:               supplierID,
		SyncProviderName:         req.SyncProviderName,
		AllowPartial:             req.AllowPartial,
		SupplierGroupPriorityIDs: req.SupplierGroupPriorityIDs,
		LocalAccountBaseURL:      req.LocalAccountBaseURL,
		LocalAccountConcurrency:  req.LocalAccountConcurrency,
		LocalAccountPriority:     req.LocalAccountPriority,
		LocalAccountGroupIDs:     req.LocalAccountGroupIDs,
		RuntimeStatus:            adminplusdomain.NormalizeSupplierRuntimeStatus(req.RuntimeStatus),
		HealthStatus:             adminplusdomain.NormalizeSupplierHealthStatus(req.HealthStatus),
		BalanceThresholdCents:    req.BalanceThresholdCents,
		BalanceCents:             req.BalanceCents,
		BalanceCurrency:          req.BalanceCurrency,
	}
}

func (h *SupplierKeyHandler) EnsureAllPlan(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	var req ensureSupplierKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	result, err := h.service.PlanEnsureAll(c.Request.Context(), ensureSupplierKeysInput(supplierID, req))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *SupplierKeyHandler) StandardizeNames(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	var req standardizeSupplierKeyNamesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	input := supplierkeysapp.StandardizeNamesInput{
		SupplierID:       supplierID,
		SyncProviderName: req.SyncProviderName,
	}
	executeAdminPlusIdempotentJSON(c, "admin-plus.supplier-key.standardize-names", input, 0, http.StatusOK, func(ctx context.Context) (any, error) {
		return h.service.StandardizeNames(ctx, input)
	})
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
		SupplierID:                 supplierID,
		KeyID:                      keyID,
		LocalSub2APIAccountID:      req.LocalSub2APIAccountID,
		ManualSecret:               req.ManualSecret,
		LocalAccountPlatform:       req.LocalAccountPlatform,
		LocalAccountName:           req.LocalAccountName,
		LocalAccountBaseURL:        req.LocalAccountBaseURL,
		LocalAccountPriority:       req.LocalAccountPriority,
		LocalAccountRateMultiplier: req.LocalAccountRateMultiplier,
		LocalAccountGroupIDs:       req.LocalAccountGroupIDs,
		RuntimeStatus:              adminplusdomain.NormalizeSupplierRuntimeStatus(req.RuntimeStatus),
		HealthStatus:               adminplusdomain.NormalizeSupplierHealthStatus(req.HealthStatus),
		ConfiguredConcurrency:      req.ConfiguredConcurrency,
		BalanceThresholdCents:      req.BalanceThresholdCents,
		BalanceCents:               req.BalanceCents,
		BalanceCurrency:            req.BalanceCurrency,
		SupplierAccountIdentifier:  req.SupplierAccountIdentifier,
		SupplierAccountLabel:       req.SupplierAccountLabel,
	}
	executeAdminPlusIdempotentJSON(c, "admin-plus.supplier-key.repair-binding", input, 0, http.StatusOK, func(ctx context.Context) (any, error) {
		return h.service.RepairBinding(ctx, input)
	})
}

func (h *SupplierKeyHandler) ImportProviderProjection(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	var req importProviderKeyProjectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	input := supplierkeysapp.ImportProviderProjectionInput{
		SupplierID:      supplierID,
		SupplierGroupID: req.SupplierGroupID,
		ExternalKeyID:   req.ExternalKeyID,
	}
	executeAdminPlusIdempotentJSON(c, "admin-plus.supplier-key.import-provider-projection", input, 0, http.StatusCreated, func(ctx context.Context) (any, error) {
		return h.service.ImportProviderProjection(ctx, input)
	})
}

func (h *SupplierKeyHandler) ImportProviderProjections(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	var req importProviderKeyProjectionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	items := make([]supplierkeysapp.ImportProviderProjectionInput, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, supplierkeysapp.ImportProviderProjectionInput{
			SupplierID:      supplierID,
			SupplierGroupID: item.SupplierGroupID,
			ExternalKeyID:   item.ExternalKeyID,
		})
	}
	input := supplierkeysapp.ImportProviderProjectionsInput{
		SupplierID: supplierID,
		Items:      items,
	}
	executeAdminPlusIdempotentJSON(c, "admin-plus.supplier-key.import-provider-projections", input, 0, http.StatusCreated, func(ctx context.Context) (any, error) {
		return h.service.ImportProviderProjections(ctx, input)
	})
}

func (h *SupplierKeyHandler) DisableLocalProjection(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	keyID, ok := parseSupplierKeyID(c)
	if !ok {
		return
	}
	var req disableSupplierKeyLocalProjectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	input := supplierkeysapp.DisableLocalProjectionInput{
		SupplierID: supplierID,
		KeyID:      keyID,
		Reason:     req.Reason,
	}
	executeAdminPlusIdempotentJSON(c, "admin-plus.supplier-key.disable-local-projection", input, 0, http.StatusOK, func(ctx context.Context) (any, error) {
		return h.service.DisableLocalProjection(ctx, input)
	})
}

func (h *SupplierKeyHandler) DisableProviderKey(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	keyID, ok := parseSupplierKeyID(c)
	if !ok {
		return
	}
	var req providerSupplierKeyOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	input := supplierkeysapp.DisableProviderKeyInput{
		SupplierID: supplierID,
		KeyID:      keyID,
		Reason:     req.Reason,
	}
	executeAdminPlusIdempotentJSON(c, "admin-plus.supplier-key.disable-provider", input, 0, http.StatusOK, func(ctx context.Context) (any, error) {
		return h.service.DisableProviderKey(ctx, input)
	})
}

func (h *SupplierKeyHandler) DeleteProviderKey(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	keyID, ok := parseSupplierKeyID(c)
	if !ok {
		return
	}
	var req providerSupplierKeyOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	input := supplierkeysapp.DeleteProviderKeyInput{
		SupplierID: supplierID,
		KeyID:      keyID,
		Reason:     req.Reason,
	}
	executeAdminPlusIdempotentJSON(c, "admin-plus.supplier-key.delete-provider", input, 0, http.StatusOK, func(ctx context.Context) (any, error) {
		return h.service.DeleteProviderKey(ctx, input)
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
		"sync_provider_name":            req.SyncProviderName,
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
		"sync_provider_name":          req.SyncProviderName,
		"allow_partial":               req.AllowPartial,
		"supplier_group_priority_ids": req.SupplierGroupPriorityIDs,
		"local_account_base_url":      req.LocalAccountBaseURL,
		"local_account_concurrency":   req.LocalAccountConcurrency,
		"local_account_priority":      req.LocalAccountPriority,
		"local_account_group_ids":     req.LocalAccountGroupIDs,
		"runtime_status":              req.RuntimeStatus,
		"health_status":               req.HealthStatus,
		"balance_threshold_cents":     req.BalanceThresholdCents,
		"balance_cents":               req.BalanceCents,
		"balance_currency":            req.BalanceCurrency,
	}
}
