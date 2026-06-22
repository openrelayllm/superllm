package adminplus

import (
	"net/http"
	"strconv"

	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type SupplierHandler struct {
	service *suppliersapp.Service
}

func NewSupplierHandler(service *suppliersapp.Service) *SupplierHandler {
	return &SupplierHandler{service: service}
}

type createSupplierRequest struct {
	Name                  string `json:"name" binding:"required"`
	Kind                  string `json:"kind" binding:"required"`
	Type                  string `json:"type" binding:"required"`
	RuntimeStatus         string `json:"runtime_status"`
	HealthStatus          string `json:"health_status"`
	DashboardURL          string `json:"dashboard_url"`
	APIBaseURL            string `json:"api_base_url"`
	ThirdPartyRechargeURL string `json:"third_party_recharge_url"`
	LocalRechargeURL      string `json:"local_recharge_url"`
	Contact               string `json:"contact"`
	Notes                 string `json:"notes"`
	PostgresReadDSN       string `json:"postgres_read_dsn"`
	RedisReadDSN          string `json:"redis_read_dsn"`
	BrowserLoginEnabled   bool   `json:"browser_login_enabled"`
	BrowserLoginUsername  string `json:"browser_login_username"`
	BrowserLoginPassword  string `json:"browser_login_password"`
	BrowserLoginToken     string `json:"browser_login_token"`
	BalanceCents          int64  `json:"balance_cents"`
	BalanceCurrency       string `json:"balance_currency"`
}

type createSupplierAccountRequest struct {
	LocalSub2APIAccountID     int64  `json:"local_sub2api_account_id" binding:"required"`
	SupplierAccountIdentifier string `json:"supplier_account_identifier"`
	SupplierAccountLabel      string `json:"supplier_account_label"`
	OrganizationID            string `json:"organization_id"`
	ProjectID                 string `json:"project_id"`
	RateProfile               string `json:"rate_profile"`
	ConfiguredConcurrency     int    `json:"configured_concurrency"`
	ObservedMaxConcurrency    int    `json:"observed_max_concurrency"`
	BalanceThresholdCents     int64  `json:"balance_threshold_cents"`
	BalanceCents              int64  `json:"balance_cents"`
	BalanceCurrency           string `json:"balance_currency"`
	RuntimeStatus             string `json:"runtime_status"`
	HealthStatus              string `json:"health_status"`
}

type updateSupplierAccountRequest struct {
	SupplierAccountIdentifier string `json:"supplier_account_identifier"`
	SupplierAccountLabel      string `json:"supplier_account_label"`
	OrganizationID            string `json:"organization_id"`
	ProjectID                 string `json:"project_id"`
	RateProfile               string `json:"rate_profile"`
	ConfiguredConcurrency     int    `json:"configured_concurrency"`
	ObservedMaxConcurrency    int    `json:"observed_max_concurrency"`
	BalanceThresholdCents     int64  `json:"balance_threshold_cents"`
	BalanceCents              int64  `json:"balance_cents"`
	BalanceCurrency           string `json:"balance_currency"`
	RuntimeStatus             string `json:"runtime_status"`
	HealthStatus              string `json:"health_status"`
}

type updateSupplierStatusRequest struct {
	RuntimeStatus string `json:"runtime_status" binding:"required"`
	HealthStatus  string `json:"health_status" binding:"required"`
}

type createSupplierFromSiteCandidateRequest struct {
	Name                  string         `json:"name"`
	Type                  string         `json:"type"`
	DashboardURL          string         `json:"dashboard_url" binding:"required"`
	APIBaseURL            string         `json:"api_base_url"`
	ThirdPartyRechargeURL string         `json:"third_party_recharge_url"`
	LocalRechargeURL      string         `json:"local_recharge_url"`
	SourceHost            string         `json:"source_host"`
	SourceURL             string         `json:"source_url"`
	PageContext           map[string]any `json:"page_context"`
}

type supplierSiteMatchRequest struct {
	URL    string `json:"url"`
	Origin string `json:"origin"`
	Host   string `json:"host"`
}

func (h *SupplierHandler) List(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.List(c.Request.Context(), suppliersapp.SupplierFilter{
		Kind:          adminplusdomain.NormalizeSupplierKind(c.Query("kind")),
		Type:          adminplusdomain.NormalizeSupplierType(c.Query("type")),
		RuntimeStatus: adminplusdomain.NormalizeSupplierRuntimeStatus(c.Query("runtime_status")),
		HealthStatus:  adminplusdomain.NormalizeSupplierHealthStatus(c.Query("health_status")),
		Query:         c.Query("q"),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *SupplierHandler) Create(c *gin.Context) {
	var req createSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	supplier, err := h.service.Create(c.Request.Context(), suppliersapp.CreateSupplierInput{
		Name:                  req.Name,
		Kind:                  adminplusdomain.NormalizeSupplierKind(req.Kind),
		Type:                  adminplusdomain.NormalizeSupplierType(req.Type),
		RuntimeStatus:         adminplusdomain.NormalizeSupplierRuntimeStatus(req.RuntimeStatus),
		HealthStatus:          adminplusdomain.NormalizeSupplierHealthStatus(req.HealthStatus),
		DashboardURL:          req.DashboardURL,
		APIBaseURL:            req.APIBaseURL,
		ThirdPartyRechargeURL: req.ThirdPartyRechargeURL,
		LocalRechargeURL:      req.LocalRechargeURL,
		Contact:               req.Contact,
		Notes:                 req.Notes,
		PostgresReadDSN:       req.PostgresReadDSN,
		RedisReadDSN:          req.RedisReadDSN,
		BrowserLoginEnabled:   req.BrowserLoginEnabled,
		BrowserLoginUsername:  req.BrowserLoginUsername,
		BrowserLoginPassword:  req.BrowserLoginPassword,
		BrowserLoginToken:     req.BrowserLoginToken,
		BalanceCents:          req.BalanceCents,
		BalanceCurrency:       req.BalanceCurrency,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, supplier)
}

func (h *SupplierHandler) CreateFromSiteCandidate(c *gin.Context) {
	var req createSupplierFromSiteCandidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	title, _ := req.PageContext["title"].(string)
	supplier, err := h.service.CreateFromSiteCandidate(c.Request.Context(), suppliersapp.CreateFromSiteCandidateInput{
		Name:                  req.Name,
		Type:                  adminplusdomain.NormalizeSupplierType(req.Type),
		DashboardURL:          req.DashboardURL,
		APIBaseURL:            req.APIBaseURL,
		ThirdPartyRechargeURL: req.ThirdPartyRechargeURL,
		LocalRechargeURL:      req.LocalRechargeURL,
		SourceHost:            req.SourceHost,
		SourceURL:             req.SourceURL,
		Title:                 title,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, supplier)
}

func (h *SupplierHandler) MatchSite(c *gin.Context) {
	var req supplierSiteMatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	result, err := h.service.MatchSite(c.Request.Context(), suppliersapp.SiteMatchInput{
		URL:    req.URL,
		Origin: req.Origin,
		Host:   req.Host,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *SupplierHandler) Update(c *gin.Context) {
	id, ok := parseSupplierID(c)
	if !ok {
		return
	}
	var req createSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	supplier, err := h.service.Update(c.Request.Context(), id, suppliersapp.UpdateSupplierInput{
		Name:                  req.Name,
		Kind:                  adminplusdomain.NormalizeSupplierKind(req.Kind),
		Type:                  adminplusdomain.NormalizeSupplierType(req.Type),
		RuntimeStatus:         adminplusdomain.NormalizeSupplierRuntimeStatus(req.RuntimeStatus),
		HealthStatus:          adminplusdomain.NormalizeSupplierHealthStatus(req.HealthStatus),
		DashboardURL:          req.DashboardURL,
		APIBaseURL:            req.APIBaseURL,
		ThirdPartyRechargeURL: req.ThirdPartyRechargeURL,
		LocalRechargeURL:      req.LocalRechargeURL,
		Contact:               req.Contact,
		Notes:                 req.Notes,
		PostgresReadDSN:       req.PostgresReadDSN,
		RedisReadDSN:          req.RedisReadDSN,
		BrowserLoginEnabled:   req.BrowserLoginEnabled,
		BrowserLoginUsername:  req.BrowserLoginUsername,
		BrowserLoginPassword:  req.BrowserLoginPassword,
		BrowserLoginToken:     req.BrowserLoginToken,
		BalanceCents:          req.BalanceCents,
		BalanceCurrency:       req.BalanceCurrency,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, supplier)
}

func (h *SupplierHandler) Get(c *gin.Context) {
	id, ok := parseSupplierID(c)
	if !ok {
		return
	}
	supplier, err := h.service.Get(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, supplier)
}

func (h *SupplierHandler) Delete(c *gin.Context) {
	id, ok := parseSupplierID(c)
	if !ok {
		return
	}
	if response.ErrorFrom(c, h.service.Delete(c.Request.Context(), id)) {
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *SupplierHandler) UpdateStatus(c *gin.Context) {
	id, ok := parseSupplierID(c)
	if !ok {
		return
	}

	var req updateSupplierStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	supplier, err := h.service.UpdateStatus(c.Request.Context(), id, suppliersapp.UpdateSupplierStatusInput{
		RuntimeStatus: adminplusdomain.NormalizeSupplierRuntimeStatus(req.RuntimeStatus),
		HealthStatus:  adminplusdomain.NormalizeSupplierHealthStatus(req.HealthStatus),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, supplier)
}

func (h *SupplierHandler) ListAccounts(c *gin.Context) {
	page := parsePagination(c)
	id, ok := parseSupplierID(c)
	if !ok {
		return
	}
	items, err := h.service.ListAccounts(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *SupplierHandler) CreateAccount(c *gin.Context) {
	id, ok := parseSupplierID(c)
	if !ok {
		return
	}
	var req createSupplierAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	account, err := h.service.CreateAccount(c.Request.Context(), suppliersapp.CreateSupplierAccountInput{
		SupplierID:                id,
		LocalSub2APIAccountID:     req.LocalSub2APIAccountID,
		SupplierAccountIdentifier: req.SupplierAccountIdentifier,
		SupplierAccountLabel:      req.SupplierAccountLabel,
		OrganizationID:            req.OrganizationID,
		ProjectID:                 req.ProjectID,
		RateProfile:               req.RateProfile,
		ConfiguredConcurrency:     req.ConfiguredConcurrency,
		BalanceThresholdCents:     req.BalanceThresholdCents,
		BalanceCents:              req.BalanceCents,
		BalanceCurrency:           req.BalanceCurrency,
		RuntimeStatus:             adminplusdomain.NormalizeSupplierRuntimeStatus(req.RuntimeStatus),
		HealthStatus:              adminplusdomain.NormalizeSupplierHealthStatus(req.HealthStatus),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, account)
}

func (h *SupplierHandler) UpdateAccount(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	accountID, err := strconv.ParseInt(c.Param("accountID"), 10, 64)
	if err != nil || accountID <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid supplier account id")
		return
	}
	var req updateSupplierAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	account, err := h.service.UpdateAccount(c.Request.Context(), suppliersapp.UpdateSupplierAccountInput{
		SupplierID:                supplierID,
		AccountID:                 accountID,
		SupplierAccountIdentifier: req.SupplierAccountIdentifier,
		SupplierAccountLabel:      req.SupplierAccountLabel,
		OrganizationID:            req.OrganizationID,
		ProjectID:                 req.ProjectID,
		RateProfile:               req.RateProfile,
		ConfiguredConcurrency:     req.ConfiguredConcurrency,
		ObservedMaxConcurrency:    req.ObservedMaxConcurrency,
		BalanceThresholdCents:     req.BalanceThresholdCents,
		BalanceCents:              req.BalanceCents,
		BalanceCurrency:           req.BalanceCurrency,
		RuntimeStatus:             adminplusdomain.NormalizeSupplierRuntimeStatus(req.RuntimeStatus),
		HealthStatus:              adminplusdomain.NormalizeSupplierHealthStatus(req.HealthStatus),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, account)
}

func (h *SupplierHandler) DeleteAccount(c *gin.Context) {
	supplierID, ok := parseSupplierID(c)
	if !ok {
		return
	}
	accountID, err := strconv.ParseInt(c.Param("accountID"), 10, 64)
	if err != nil || accountID <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid supplier account id")
		return
	}
	if response.ErrorFrom(c, h.service.DeleteAccount(c.Request.Context(), supplierID, accountID)) {
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *SupplierHandler) ListLocalAccounts(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListLocalAccounts(c.Request.Context(), c.Query("q"), page.Offset+page.PageSize)
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func parseSupplierID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid supplier id")
		return 0, false
	}
	return id, true
}
