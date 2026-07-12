package adminplus

import (
	"net/http"
	"strconv"
	"strings"

	actionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/actions"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type ActionHandler struct {
	service *actionsapp.Service
}

func NewActionHandler(service *actionsapp.Service) *ActionHandler {
	return &ActionHandler{service: service}
}

type generateActionsRequest struct {
	Suppliers            []supplierSignalDTO                     `json:"suppliers"`
	CandidateEvaluations []candidateEvaluationDTO                `json:"candidate_evaluations"`
	LocalGroupCapacity   []localGroupCapacityDTO                 `json:"local_group_capacity"`
	LocalAccountSchedule []localAccountScheduleDTO               `json:"local_account_schedule"`
	BalanceEvents        []*adminplusdomain.BalanceEvent         `json:"balance_events"`
	HealthEvents         []*adminplusdomain.HealthEvent          `json:"health_events"`
	CostSnapshots        []*adminplusdomain.SupplierCostSnapshot `json:"cost_snapshots"`
	MinProfitMargin      float64                                 `json:"min_profit_margin"`
}

type supplierSignalDTO struct {
	SupplierID         int64  `json:"supplier_id" binding:"required"`
	Name               string `json:"name"`
	RuntimeStatus      string `json:"runtime_status"`
	HealthStatus       string `json:"health_status"`
	BalanceCents       int64  `json:"balance_cents"`
	Currency           string `json:"currency"`
	EffectiveCostCents int64  `json:"effective_cost_cents"`
	KeyLimitPolicy     string `json:"key_limit_policy"`
	KeyLimitValue      int    `json:"key_limit_value"`
	ActiveKeyCount     int    `json:"active_key_count"`
	KeyCapacityStatus  string `json:"key_capacity_status"`
}

type candidateEvaluationDTO struct {
	SupplierID              int64   `json:"supplier_id"`
	SupplierGroupID         int64   `json:"supplier_group_id"`
	LocalSub2APIAccountID   int64   `json:"local_sub2api_account_id"`
	CandidateStatus         string  `json:"candidate_status"`
	BlockedReason           string  `json:"blocked_reason"`
	CheckSource             string  `json:"check_source"`
	BalanceStatus           string  `json:"balance_status"`
	KeyCapacityStatus       string  `json:"key_capacity_status"`
	PurityFreshness         string  `json:"purity_freshness_status"`
	EffectiveRateMultiplier float64 `json:"effective_rate_multiplier"`
}

type localGroupCapacityDTO struct {
	LocalGroupID                   int64   `json:"local_group_id"`
	LocalGroupName                 string  `json:"local_group_name"`
	Platform                       string  `json:"platform"`
	TotalAccounts                  int64   `json:"total_accounts"`
	SchedulableAccounts            int64   `json:"schedulable_accounts"`
	ActiveAPIKeyCount              int64   `json:"active_api_key_count"`
	RateMultiplier                 float64 `json:"rate_multiplier"`
	LowCapacityThreshold           int64   `json:"low_capacity_threshold"`
	BestCandidateSupplierID        int64   `json:"best_candidate_supplier_id"`
	BestCandidateSupplierGroupID   int64   `json:"best_candidate_supplier_group_id"`
	BestCandidateLocalAccountID    int64   `json:"best_candidate_local_account_id"`
	BestCandidateRateMultiplier    float64 `json:"best_candidate_rate_multiplier"`
	BestCandidateCheckSource       string  `json:"best_candidate_check_source"`
	BestCandidateSupplierName      string  `json:"best_candidate_supplier_name"`
	BestCandidateSupplierGroupName string  `json:"best_candidate_supplier_group_name"`
}

type localAccountScheduleDTO struct {
	SupplierID              int64    `json:"supplier_id"`
	SupplierGroupID         int64    `json:"supplier_group_id"`
	LocalSub2APIAccountID   int64    `json:"local_sub2api_account_id"`
	LocalAccountName        string   `json:"local_account_name"`
	SupplierName            string   `json:"supplier_name"`
	SupplierGroupName       string   `json:"supplier_group_name"`
	LocalGroupIDs           []int64  `json:"local_group_ids"`
	LocalGroupNames         []string `json:"local_group_names"`
	LocalAccountSchedulable bool     `json:"local_account_schedulable"`
	CandidateStatus         string   `json:"candidate_status"`
	BlockedReason           string   `json:"blocked_reason"`
	CheckSource             string   `json:"check_source"`
	BalanceStatus           string   `json:"balance_status"`
	KeyCapacityStatus       string   `json:"key_capacity_status"`
	ChannelCheckStatus      string   `json:"channel_check_status"`
	EffectiveRateMultiplier float64  `json:"effective_rate_multiplier"`
}

type updateActionStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type executeActionRequest struct {
	OperatorUserID  int64          `json:"operator_user_id"`
	SchedulerRunID  string         `json:"scheduler_run_id"`
	SchedulerStepID int64          `json:"scheduler_step_id"`
	RequestPayload  map[string]any `json:"request_payload"`
}

func (h *ActionHandler) Generate(c *gin.Context) {
	var req generateActionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	suppliers := make([]actionsapp.SupplierSignal, 0, len(req.Suppliers))
	for _, supplier := range req.Suppliers {
		suppliers = append(suppliers, actionsapp.SupplierSignal{
			SupplierID:         supplier.SupplierID,
			Name:               supplier.Name,
			RuntimeStatus:      adminplusdomain.NormalizeSupplierRuntimeStatus(supplier.RuntimeStatus),
			HealthStatus:       adminplusdomain.NormalizeSupplierHealthStatus(supplier.HealthStatus),
			BalanceCents:       supplier.BalanceCents,
			Currency:           supplier.Currency,
			EffectiveCostCents: supplier.EffectiveCostCents,
			KeyLimitPolicy:     supplier.KeyLimitPolicy,
			KeyLimitValue:      supplier.KeyLimitValue,
			ActiveKeyCount:     supplier.ActiveKeyCount,
			KeyCapacityStatus:  supplier.KeyCapacityStatus,
		})
	}
	candidateEvaluations := make([]actionsapp.CandidateSignal, 0, len(req.CandidateEvaluations))
	for _, candidate := range req.CandidateEvaluations {
		candidateEvaluations = append(candidateEvaluations, actionsapp.CandidateSignal{
			SupplierID:              candidate.SupplierID,
			SupplierGroupID:         candidate.SupplierGroupID,
			LocalSub2APIAccountID:   candidate.LocalSub2APIAccountID,
			CandidateStatus:         candidate.CandidateStatus,
			BlockedReason:           candidate.BlockedReason,
			CheckSource:             candidate.CheckSource,
			BalanceStatus:           candidate.BalanceStatus,
			KeyCapacityStatus:       candidate.KeyCapacityStatus,
			PurityFreshness:         candidate.PurityFreshness,
			EffectiveRateMultiplier: candidate.EffectiveRateMultiplier,
		})
	}
	localGroupCapacity := make([]actionsapp.LocalGroupCapacitySignal, 0, len(req.LocalGroupCapacity))
	for _, group := range req.LocalGroupCapacity {
		localGroupCapacity = append(localGroupCapacity, actionsapp.LocalGroupCapacitySignal{
			LocalGroupID:                   group.LocalGroupID,
			LocalGroupName:                 group.LocalGroupName,
			Platform:                       group.Platform,
			TotalAccounts:                  group.TotalAccounts,
			SchedulableAccounts:            group.SchedulableAccounts,
			ActiveAPIKeyCount:              group.ActiveAPIKeyCount,
			RateMultiplier:                 group.RateMultiplier,
			LowCapacityThreshold:           group.LowCapacityThreshold,
			BestCandidateSupplierID:        group.BestCandidateSupplierID,
			BestCandidateSupplierGroupID:   group.BestCandidateSupplierGroupID,
			BestCandidateLocalAccountID:    group.BestCandidateLocalAccountID,
			BestCandidateRateMultiplier:    group.BestCandidateRateMultiplier,
			BestCandidateCheckSource:       group.BestCandidateCheckSource,
			BestCandidateSupplierName:      group.BestCandidateSupplierName,
			BestCandidateSupplierGroupName: group.BestCandidateSupplierGroupName,
		})
	}
	localAccountSchedule := make([]actionsapp.LocalAccountScheduleSignal, 0, len(req.LocalAccountSchedule))
	for _, account := range req.LocalAccountSchedule {
		localAccountSchedule = append(localAccountSchedule, actionsapp.LocalAccountScheduleSignal{
			SupplierID:              account.SupplierID,
			SupplierGroupID:         account.SupplierGroupID,
			LocalSub2APIAccountID:   account.LocalSub2APIAccountID,
			LocalAccountName:        account.LocalAccountName,
			SupplierName:            account.SupplierName,
			SupplierGroupName:       account.SupplierGroupName,
			LocalGroupIDs:           account.LocalGroupIDs,
			LocalGroupNames:         account.LocalGroupNames,
			LocalAccountSchedulable: account.LocalAccountSchedulable,
			CandidateStatus:         account.CandidateStatus,
			BlockedReason:           account.BlockedReason,
			CheckSource:             account.CheckSource,
			BalanceStatus:           account.BalanceStatus,
			KeyCapacityStatus:       account.KeyCapacityStatus,
			ChannelCheckStatus:      account.ChannelCheckStatus,
			EffectiveRateMultiplier: account.EffectiveRateMultiplier,
		})
	}
	result, err := h.service.Generate(c.Request.Context(), actionsapp.GenerateInput{
		Suppliers:            suppliers,
		CandidateEvaluations: candidateEvaluations,
		LocalGroupCapacity:   localGroupCapacity,
		LocalAccountSchedule: localAccountSchedule,
		BalanceEvents:        req.BalanceEvents,
		HealthEvents:         req.HealthEvents,
		CostSnapshots:        req.CostSnapshots,
		MinProfitMargin:      req.MinProfitMargin,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *ActionHandler) ListRecommendations(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListRecommendations(c.Request.Context(), actionsapp.RecommendationFilter{
		ID:         parseInt64Query(c, "recommendation_id"),
		SupplierID: parseInt64Query(c, "supplier_id"),
		Status:     adminplusdomain.ActionStatus(c.Query("status")),
		Severity:   adminplusdomain.ActionSeverity(c.Query("severity")),
		Type:       adminplusdomain.ActionType(c.Query("type")),
		Signal:     actionRecommendationSignalQuery(c),
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func actionRecommendationSignalQuery(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if signal := strings.TrimSpace(c.Query("signal")); signal != "" {
		return signal
	}
	if id := parseInt64Query(c, "local_group_id"); id > 0 {
		return "local_group_id=" + strconv.FormatInt(id, 10)
	}
	if id := parseInt64Query(c, "local_sub2api_account_id"); id > 0 {
		return "local_sub2api_account_id=" + strconv.FormatInt(id, 10)
	}
	return ""
}

func (h *ActionHandler) UpdateRecommendationStatus(c *gin.Context) {
	id, ok := parseActionRecommendationID(c)
	if !ok {
		return
	}
	var req updateActionStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	item, err := h.service.UpdateRecommendationStatus(c.Request.Context(), id, adminplusdomain.ActionStatus(req.Status))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, item)
}

func (h *ActionHandler) ExecuteRecommendation(c *gin.Context) {
	id, ok := parseActionRecommendationID(c)
	if !ok {
		return
	}
	var req executeActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	item, err := h.service.ExecuteApprovedRecommendation(c.Request.Context(), id, actionsapp.ExecuteInput{
		OperatorUserID:     req.OperatorUserID,
		SchedulerRunID:     req.SchedulerRunID,
		SchedulerStepID:    req.SchedulerStepID,
		IdempotencyKeyHash: adminPlusIdempotencyKeyHash(c),
		RequestPayload:     req.RequestPayload,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, item)
}

func (h *ActionHandler) ListExecutions(c *gin.Context) {
	id, ok := parseActionRecommendationID(c)
	if !ok {
		return
	}
	page := parsePagination(c)
	items, err := h.service.ListExecutions(c.Request.Context(), id, fetchLimitForPagination(page))
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func parseActionRecommendationID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid action recommendation id")
		return 0, false
	}
	return id, true
}
