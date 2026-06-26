package adminplus

import (
	"strconv"
	"strings"

	proxyapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/proxy"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type ProxyHandler struct {
	service *proxyapp.Service
}

func NewProxyHandler(service *proxyapp.Service) *ProxyHandler {
	return &ProxyHandler{service: service}
}

type createProxySubscriptionRequest struct {
	Name                   string `json:"name"`
	SubscriptionType       string `json:"subscription_type"`
	SubscriptionURL        string `json:"subscription_url"`
	Enabled                *bool  `json:"enabled"`
	RefreshIntervalSeconds int    `json:"refresh_interval_seconds"`
	RefreshNow             *bool  `json:"refresh_now"`
}

type updateProxySubscriptionRequest struct {
	Name                   *string `json:"name"`
	SubscriptionType       *string `json:"subscription_type"`
	SubscriptionURL        *string `json:"subscription_url"`
	Enabled                *bool   `json:"enabled"`
	RefreshIntervalSeconds *int    `json:"refresh_interval_seconds"`
}

type updateProxyNodeStatusRequest struct {
	Reason string `json:"reason"`
}

type createProxyPolicyRequest struct {
	Name               string         `json:"name"`
	Enabled            *bool          `json:"enabled"`
	SubscriptionIDs    []int64        `json:"subscription_ids"`
	PreferredRegions   []string       `json:"preferred_regions"`
	MaxConcurrency     int            `json:"max_concurrency"`
	MaxSwitchesPerTask int            `json:"max_switches_per_task"`
	ConnectTimeoutMS   int            `json:"connect_timeout_ms"`
	RequestTimeoutMS   int            `json:"request_timeout_ms"`
	Config             map[string]any `json:"config"`
}

type updateProxyPolicyRequest struct {
	Name               *string        `json:"name"`
	Enabled            *bool          `json:"enabled"`
	SubscriptionIDs    []int64        `json:"subscription_ids"`
	PreferredRegions   []string       `json:"preferred_regions"`
	MaxConcurrency     *int           `json:"max_concurrency"`
	MaxSwitchesPerTask *int           `json:"max_switches_per_task"`
	ConnectTimeoutMS   *int           `json:"connect_timeout_ms"`
	RequestTimeoutMS   *int           `json:"request_timeout_ms"`
	Config             map[string]any `json:"config"`
}

type createProxyTargetRequest struct {
	TargetHost         string   `json:"target_host"`
	Purpose            string   `json:"purpose"`
	AllowedMethods     []string `json:"allowed_methods"`
	RateLimitPerMinute int      `json:"rate_limit_per_minute"`
	Enabled            *bool    `json:"enabled"`
	AuthorizationNote  string   `json:"authorization_note"`
}

type updateProxyTargetRequest struct {
	TargetHost         *string  `json:"target_host"`
	Purpose            *string  `json:"purpose"`
	AllowedMethods     []string `json:"allowed_methods"`
	RateLimitPerMinute *int     `json:"rate_limit_per_minute"`
	Enabled            *bool    `json:"enabled"`
	AuthorizationNote  *string  `json:"authorization_note"`
}

type createProxyAssignmentRequest struct {
	TaskType   string `json:"task_type"`
	TaskID     string `json:"task_id"`
	PolicyID   int64  `json:"policy_id"`
	TargetHost string `json:"target_host"`
	Purpose    string `json:"purpose"`
	Method     string `json:"method"`
}

type releaseProxyAssignmentRequest struct {
	Failed       bool   `json:"failed"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

type switchProxyAssignmentRequest struct {
	NodeID       int64  `json:"node_id"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

func (h *ProxyHandler) CenterStatus(c *gin.Context) {
	result, err := h.service.CenterStatus(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *ProxyHandler) ListSubscriptions(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListSubscriptions(c.Request.Context(), proxyapp.SubscriptionFilter{
		Enabled: parseOptionalBoolQuery(c, "enabled"),
		Limit:   fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *ProxyHandler) CreateSubscription(c *gin.Context) {
	var req createProxySubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	item, err := h.service.CreateSubscription(c.Request.Context(), proxyapp.CreateSubscriptionInput{
		Name:                   req.Name,
		SubscriptionType:       adminplusdomain.ProxySubscriptionType(strings.TrimSpace(req.SubscriptionType)),
		SubscriptionURL:        req.SubscriptionURL,
		Enabled:                boolPtrDefault(req.Enabled, true),
		RefreshIntervalSeconds: req.RefreshIntervalSeconds,
		RefreshNow:             boolPtrDefault(req.RefreshNow, true),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, item)
}

func (h *ProxyHandler) UpdateSubscription(c *gin.Context) {
	id, ok := parseProxyIDParam(c, "id")
	if !ok {
		return
	}
	var req updateProxySubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	var subscriptionType *adminplusdomain.ProxySubscriptionType
	if req.SubscriptionType != nil {
		value := adminplusdomain.ProxySubscriptionType(strings.TrimSpace(*req.SubscriptionType))
		subscriptionType = &value
	}
	item, err := h.service.UpdateSubscription(c.Request.Context(), id, proxyapp.UpdateSubscriptionInput{
		Name:                      req.Name,
		SubscriptionType:          subscriptionType,
		SubscriptionURLCiphertext: req.SubscriptionURL,
		Enabled:                   req.Enabled,
		RefreshIntervalSeconds:    req.RefreshIntervalSeconds,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, item)
}

func (h *ProxyHandler) RefreshSubscription(c *gin.Context) {
	id, ok := parseProxyIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.service.RefreshSubscription(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Accepted(c, item)
}

func (h *ProxyHandler) DeleteSubscription(c *gin.Context) {
	id, ok := parseProxyIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.service.DeleteSubscription(c.Request.Context(), id); response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *ProxyHandler) ListNodes(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListNodes(c.Request.Context(), proxyapp.NodeFilter{
		SubscriptionID:  parseInt64Query(c, "subscription_id"),
		HealthStatus:    adminplusdomain.ProxyNodeHealthStatus(strings.TrimSpace(c.Query("health_status"))),
		IncludeDisabled: c.Query("include_disabled") == "true",
		Query:           c.Query("q"),
		Limit:           fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *ProxyHandler) CheckNode(c *gin.Context) {
	id, ok := parseProxyIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.service.CheckNode(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Accepted(c, item)
}

func (h *ProxyHandler) DisableNode(c *gin.Context) {
	id, ok := parseProxyIDParam(c, "id")
	if !ok {
		return
	}
	var req updateProxyNodeStatusRequest
	_ = c.ShouldBindJSON(&req)
	item, err := h.service.UpdateNodeDisabled(c.Request.Context(), id, true, req.Reason)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, item)
}

func (h *ProxyHandler) EnableNode(c *gin.Context) {
	id, ok := parseProxyIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.service.UpdateNodeDisabled(c.Request.Context(), id, false, "")
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, item)
}

func (h *ProxyHandler) ListPolicies(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListPolicies(c.Request.Context(), proxyapp.PolicyFilter{
		Enabled: parseOptionalBoolQuery(c, "enabled"),
		Limit:   fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *ProxyHandler) CreatePolicy(c *gin.Context) {
	var req createProxyPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	item, err := h.service.CreatePolicy(c.Request.Context(), proxyapp.CreatePolicyInput{
		Name:               req.Name,
		Enabled:            boolPtrDefault(req.Enabled, true),
		SubscriptionIDs:    req.SubscriptionIDs,
		PreferredRegions:   req.PreferredRegions,
		MaxConcurrency:     req.MaxConcurrency,
		MaxSwitchesPerTask: req.MaxSwitchesPerTask,
		ConnectTimeoutMS:   req.ConnectTimeoutMS,
		RequestTimeoutMS:   req.RequestTimeoutMS,
		Config:             req.Config,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, item)
}

func (h *ProxyHandler) UpdatePolicy(c *gin.Context) {
	id, ok := parseProxyIDParam(c, "id")
	if !ok {
		return
	}
	var req updateProxyPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	item, err := h.service.UpdatePolicy(c.Request.Context(), id, proxyapp.UpdatePolicyInput{
		Name:               req.Name,
		Enabled:            req.Enabled,
		SubscriptionIDs:    req.SubscriptionIDs,
		PreferredRegions:   req.PreferredRegions,
		MaxConcurrency:     req.MaxConcurrency,
		MaxSwitchesPerTask: req.MaxSwitchesPerTask,
		ConnectTimeoutMS:   req.ConnectTimeoutMS,
		RequestTimeoutMS:   req.RequestTimeoutMS,
		Config:             req.Config,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, item)
}

func (h *ProxyHandler) DeletePolicy(c *gin.Context) {
	id, ok := parseProxyIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.service.DeletePolicy(c.Request.Context(), id); response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *ProxyHandler) ListTargets(c *gin.Context) {
	id, ok := parseProxyIDParam(c, "id")
	if !ok {
		return
	}
	page := parsePagination(c)
	items, err := h.service.ListTargets(c.Request.Context(), proxyapp.TargetFilter{
		PolicyID: id,
		Purpose:  adminplusdomain.ProxyTaskPurpose(strings.TrimSpace(c.Query("purpose"))),
		Enabled:  parseOptionalBoolQuery(c, "enabled"),
		Limit:    fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *ProxyHandler) CreateTarget(c *gin.Context) {
	id, ok := parseProxyIDParam(c, "id")
	if !ok {
		return
	}
	var req createProxyTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	item, err := h.service.CreateTarget(c.Request.Context(), proxyapp.CreateTargetInput{
		PolicyID:           id,
		TargetHost:         req.TargetHost,
		Purpose:            adminplusdomain.ProxyTaskPurpose(strings.TrimSpace(req.Purpose)),
		AllowedMethods:     req.AllowedMethods,
		RateLimitPerMinute: req.RateLimitPerMinute,
		Enabled:            boolPtrDefault(req.Enabled, true),
		AuthorizationNote:  req.AuthorizationNote,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, item)
}

func (h *ProxyHandler) UpdateTarget(c *gin.Context) {
	policyID, ok := parseProxyIDParam(c, "id")
	if !ok {
		return
	}
	targetID, ok := parseProxyIDParam(c, "targetID")
	if !ok {
		return
	}
	var req updateProxyTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	var purpose *adminplusdomain.ProxyTaskPurpose
	if req.Purpose != nil {
		value := adminplusdomain.ProxyTaskPurpose(strings.TrimSpace(*req.Purpose))
		purpose = &value
	}
	item, err := h.service.UpdateTarget(c.Request.Context(), policyID, targetID, proxyapp.UpdateTargetInput{
		TargetHost:         req.TargetHost,
		Purpose:            purpose,
		AllowedMethods:     req.AllowedMethods,
		RateLimitPerMinute: req.RateLimitPerMinute,
		Enabled:            req.Enabled,
		AuthorizationNote:  req.AuthorizationNote,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, item)
}

func (h *ProxyHandler) DeleteTarget(c *gin.Context) {
	policyID, ok := parseProxyIDParam(c, "id")
	if !ok {
		return
	}
	targetID, ok := parseProxyIDParam(c, "targetID")
	if !ok {
		return
	}
	if err := h.service.DeleteTarget(c.Request.Context(), policyID, targetID); response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *ProxyHandler) ListRuntimeSlots(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListRuntimeSlots(c.Request.Context(), proxyapp.RuntimeSlotFilter{
		Status: adminplusdomain.ProxyRuntimeSlotStatus(strings.TrimSpace(c.Query("status"))),
		Limit:  fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *ProxyHandler) RestartRuntimeSlot(c *gin.Context) {
	id, ok := parseProxyIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.service.RestartSlot(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Accepted(c, item)
}

func (h *ProxyHandler) CreateAssignment(c *gin.Context) {
	var req createProxyAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	item, err := h.service.RequestAssignment(c.Request.Context(), proxyapp.RequestAssignmentInput{
		TaskType:   req.TaskType,
		TaskID:     req.TaskID,
		PolicyID:   req.PolicyID,
		TargetHost: req.TargetHost,
		Purpose:    adminplusdomain.ProxyTaskPurpose(strings.TrimSpace(req.Purpose)),
		Method:     req.Method,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, item)
}

func (h *ProxyHandler) ListAssignments(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListAssignments(c.Request.Context(), proxyapp.AssignmentFilter{
		TaskType: c.Query("task_type"),
		TaskID:   c.Query("task_id"),
		Status:   adminplusdomain.ProxyAssignmentStatus(strings.TrimSpace(c.Query("status"))),
		Limit:    fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *ProxyHandler) ReleaseAssignment(c *gin.Context) {
	id, ok := parseProxyIDParam(c, "id")
	if !ok {
		return
	}
	var req releaseProxyAssignmentRequest
	_ = c.ShouldBindJSON(&req)
	item, err := h.service.ReleaseAssignment(c.Request.Context(), id, req.Failed, req.ErrorCode, req.ErrorMessage)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Accepted(c, item)
}

func (h *ProxyHandler) SwitchAssignment(c *gin.Context) {
	id, ok := parseProxyIDParam(c, "id")
	if !ok {
		return
	}
	var req switchProxyAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	item, err := h.service.SwitchAssignment(c.Request.Context(), id, proxyapp.SwitchAssignmentInput{
		NodeID:       req.NodeID,
		ErrorCode:    req.ErrorCode,
		ErrorMessage: req.ErrorMessage,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Accepted(c, item)
}

func (h *ProxyHandler) ListAuditEvents(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListAuditEvents(c.Request.Context(), proxyapp.AuditFilter{
		EventType:  c.Query("event_type"),
		TaskType:   c.Query("task_type"),
		TaskID:     c.Query("task_id"),
		Level:      adminplusdomain.ProxyAuditLevel(strings.TrimSpace(c.Query("level"))),
		TargetHost: c.Query("target_host"),
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func parseProxyIDParam(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid proxy id")
		return 0, false
	}
	return id, true
}

func parseOptionalBoolQuery(c *gin.Context, name string) *bool {
	value := strings.TrimSpace(c.Query(name))
	if value == "" {
		return nil
	}
	parsed := value == "true" || value == "1"
	return &parsed
}

func boolPtrDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}
