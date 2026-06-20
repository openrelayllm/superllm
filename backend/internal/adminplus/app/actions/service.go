package actions

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type SupplierSignal struct {
	SupplierID         int64
	Name               string
	RuntimeStatus      adminplusdomain.SupplierRuntimeStatus
	HealthStatus       adminplusdomain.SupplierHealthStatus
	BalanceCents       int64
	Currency           string
	EffectiveCostCents int64
}

type GenerateInput struct {
	Suppliers       []SupplierSignal
	BalanceEvents   []*adminplusdomain.BalanceEvent
	PromotionEvents []*adminplusdomain.PromotionEvent
	HealthEvents    []*adminplusdomain.HealthEvent
	Reconciliation  adminplusdomain.ReconciliationSummary
	MinProfitMargin float64
}

type GenerateResult struct {
	Items []*adminplusdomain.ActionRecommendation `json:"items"`
	Total int                                     `json:"total"`
}

type RecommendationFilter struct {
	SupplierID int64
	Status     adminplusdomain.ActionStatus
	Severity   adminplusdomain.ActionSeverity
	Type       adminplusdomain.ActionType
	Limit      int
}

type Repository interface {
	CreateRecommendation(ctx context.Context, action *adminplusdomain.ActionRecommendation) (*adminplusdomain.ActionRecommendation, error)
	ListRecommendations(ctx context.Context, filter RecommendationFilter) ([]*adminplusdomain.ActionRecommendation, error)
	UpdateRecommendationStatus(ctx context.Context, id int64, status adminplusdomain.ActionStatus) (*adminplusdomain.ActionRecommendation, error)
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  time.Now,
	}
}

func NewRuleService() *Service {
	return NewService(nil)
}

func (s *Service) Generate(ctx context.Context, in GenerateInput) (*GenerateResult, error) {
	if len(in.Suppliers) == 0 {
		return nil, badRequest("ACTION_SUPPLIERS_REQUIRED", "supplier signals are required")
	}
	now := s.now().UTC()
	suppliers := indexSuppliers(in.Suppliers)
	bestCandidate := findBestSwitchCandidate(in.Suppliers)
	items := make([]*adminplusdomain.ActionRecommendation, 0)
	items = append(items, s.actionsFromSuppliers(now, in.Suppliers, bestCandidate)...)
	items = append(items, s.actionsFromBalanceEvents(now, suppliers, in.BalanceEvents, bestCandidate)...)
	items = append(items, s.actionsFromPromotionEvents(now, suppliers, in.PromotionEvents)...)
	items = append(items, s.actionsFromHealthEvents(now, suppliers, in.HealthEvents, bestCandidate)...)
	if item := s.actionFromReconciliation(now, in.Reconciliation, in.MinProfitMargin); item != nil {
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return severityRank(items[i].Severity) > severityRank(items[j].Severity)
	})
	if s.repo != nil {
		stored := make([]*adminplusdomain.ActionRecommendation, 0, len(items))
		for _, item := range items {
			created, err := s.repo.CreateRecommendation(ctx, item)
			if err != nil {
				return nil, err
			}
			stored = append(stored, created)
		}
		items = stored
	}
	return &GenerateResult{Items: items, Total: len(items)}, nil
}

func (s *Service) ListRecommendations(ctx context.Context, filter RecommendationFilter) ([]*adminplusdomain.ActionRecommendation, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("action repository is not configured")
	}
	if filter.SupplierID < 0 {
		return nil, badRequest("ACTION_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, badRequest("ACTION_STATUS_INVALID", "invalid action status")
	}
	if filter.Severity != "" && !filter.Severity.Valid() {
		return nil, badRequest("ACTION_SEVERITY_INVALID", "invalid action severity")
	}
	if filter.Type != "" && !filter.Type.Valid() {
		return nil, badRequest("ACTION_TYPE_INVALID", "invalid action type")
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListRecommendations(ctx, filter)
}

func (s *Service) UpdateRecommendationStatus(ctx context.Context, id int64, status adminplusdomain.ActionStatus) (*adminplusdomain.ActionRecommendation, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("action repository is not configured")
	}
	if id <= 0 {
		return nil, badRequest("ACTION_RECOMMENDATION_ID_INVALID", "invalid action recommendation id")
	}
	if !status.Valid() {
		return nil, badRequest("ACTION_STATUS_INVALID", "invalid action status")
	}
	return s.repo.UpdateRecommendationStatus(ctx, id, status)
}

func (s *Service) actionsFromSuppliers(now time.Time, suppliers []SupplierSignal, bestCandidate *SupplierSignal) []*adminplusdomain.ActionRecommendation {
	items := make([]*adminplusdomain.ActionRecommendation, 0)
	for _, supplier := range suppliers {
		if supplier.SupplierID <= 0 {
			continue
		}
		if supplier.RuntimeStatus == adminplusdomain.SupplierRuntimeStatusActive && supplier.BalanceCents <= 0 {
			items = append(items, newAction(now, supplier.SupplierID, nil, adminplusdomain.ActionTypePauseSupplier, adminplusdomain.ActionSeverityCritical, "active_supplier_depleted", "Pause depleted active supplier", "Active supplier has no balance and must not receive traffic.", "prevent failed upstream calls", []string{"balance_cents=0"}))
			if bestCandidate != nil {
				items = append(items, newAction(now, supplier.SupplierID, &bestCandidate.SupplierID, adminplusdomain.ActionTypeSwitchSupplier, adminplusdomain.ActionSeverityCritical, "switch_from_depleted_supplier", "Switch away from depleted supplier", "A cheaper or available candidate exists while the active supplier is depleted.", "restore traffic with available balance", []string{"balance_cents=0", "candidate_available"}))
			}
		}
		if supplier.HealthStatus == adminplusdomain.SupplierHealthStatusCredentialInvalid {
			items = append(items, newAction(now, supplier.SupplierID, nil, adminplusdomain.ActionTypeReviewCredential, adminplusdomain.ActionSeverityCritical, "credential_invalid", "Review supplier credential", "Supplier credential is invalid and browser/API collection may fail.", "restore monitoring and routing eligibility", []string{"credential_invalid"}))
		}
	}
	return items
}

func (s *Service) actionsFromBalanceEvents(now time.Time, suppliers map[int64]SupplierSignal, events []*adminplusdomain.BalanceEvent, bestCandidate *SupplierSignal) []*adminplusdomain.ActionRecommendation {
	items := make([]*adminplusdomain.ActionRecommendation, 0, len(events))
	for _, event := range events {
		if event == nil || event.Status != adminplusdomain.BalanceEventStatusOpen {
			continue
		}
		supplier := suppliers[event.SupplierID]
		switch event.Type {
		case adminplusdomain.BalanceEventTypeDepleted:
			items = append(items, newAction(now, event.SupplierID, nil, adminplusdomain.ActionTypeRechargeSupplier, adminplusdomain.ActionSeverityCritical, "supplier_balance_depleted", "Recharge depleted supplier", "Supplier balance is depleted. It may still be monitored, but must not be selected for switching.", "restore candidate eligibility after recharge", []string{"balance_depleted"}))
			if bestCandidate != nil && supplier.RuntimeStatus == adminplusdomain.SupplierRuntimeStatusActive {
				items = append(items, newAction(now, event.SupplierID, &bestCandidate.SupplierID, adminplusdomain.ActionTypeSwitchSupplier, adminplusdomain.ActionSeverityCritical, "switch_from_depleted_supplier", "Switch to available supplier", "Active supplier balance is depleted and another eligible supplier is available.", "keep customer traffic stable", []string{"balance_depleted", "candidate_available"}))
			}
		case adminplusdomain.BalanceEventTypeLowBalance:
			items = append(items, newAction(now, event.SupplierID, nil, adminplusdomain.ActionTypeRechargeSupplier, adminplusdomain.ActionSeverityWarning, "supplier_balance_low", "Recharge low-balance supplier", "Supplier balance is below configured threshold.", "avoid emergency traffic switch", []string{"balance_low"}))
		}
	}
	return items
}

func (s *Service) actionsFromPromotionEvents(now time.Time, suppliers map[int64]SupplierSignal, events []*adminplusdomain.PromotionEvent) []*adminplusdomain.ActionRecommendation {
	items := make([]*adminplusdomain.ActionRecommendation, 0, len(events))
	for _, event := range events {
		if event == nil || event.Status != adminplusdomain.PromotionStatusOpen {
			continue
		}
		switch event.Recommendation {
		case adminplusdomain.PromotionRecommendationRechargeToUnlock:
			items = append(items, newAction(now, event.SupplierID, nil, adminplusdomain.ActionTypeRechargeSupplier, adminplusdomain.ActionSeverityInfo, "promotion_recharge_to_unlock", "Recharge to unlock promotion", "Supplier has a promotion but no usable balance. Recharge may unlock lower cost, but it must not be used for switching until balance is positive.", "potentially lower future API cost", []string{"promotion", "no_balance"}))
		case adminplusdomain.PromotionRecommendationSwitchCandidate:
			supplier := suppliers[event.SupplierID]
			if supplier.BalanceCents > 0 && adminplusdomain.IsSwitchableSupplierStatus(supplier.RuntimeStatus) {
				items = append(items, newAction(now, event.SupplierID, nil, adminplusdomain.ActionTypeIncreaseWeight, adminplusdomain.ActionSeverityInfo, "promotion_switch_candidate", "Review promoted supplier weight", "Supplier has a usable promotion and positive balance.", "route more traffic to lower-cost supplier after approval", []string{"promotion", "switch_eligible"}))
			}
		}
	}
	return items
}

func (s *Service) actionsFromHealthEvents(now time.Time, suppliers map[int64]SupplierSignal, events []*adminplusdomain.HealthEvent, bestCandidate *SupplierSignal) []*adminplusdomain.ActionRecommendation {
	items := make([]*adminplusdomain.ActionRecommendation, 0, len(events))
	for _, event := range events {
		if event == nil || event.Status != adminplusdomain.HealthEventStatusOpen {
			continue
		}
		supplier := suppliers[event.SupplierID]
		switch event.Type {
		case adminplusdomain.HealthEventTypeRequestError:
			items = append(items, newAction(now, event.SupplierID, nil, adminplusdomain.ActionTypePauseSupplier, adminplusdomain.ActionSeverityCritical, "supplier_request_errors", "Pause failing supplier", "Supplier returned request errors and may impact customer traffic.", "stop routing traffic to failing node", []string{"request_error"}))
			if bestCandidate != nil && supplier.RuntimeStatus == adminplusdomain.SupplierRuntimeStatusActive {
				items = append(items, newAction(now, event.SupplierID, &bestCandidate.SupplierID, adminplusdomain.ActionTypeSwitchSupplier, adminplusdomain.ActionSeverityCritical, "switch_from_failing_supplier", "Switch from failing supplier", "Active supplier is failing and another eligible supplier is available.", "restore stable API responses", []string{"request_error", "candidate_available"}))
			}
		case adminplusdomain.HealthEventTypeSlowFirstToken, adminplusdomain.HealthEventTypeSlowTotal, adminplusdomain.HealthEventTypeConcurrencyFull:
			items = append(items, newAction(now, event.SupplierID, nil, adminplusdomain.ActionTypeDegradeSupplier, adminplusdomain.ActionSeverityWarning, "supplier_performance_degraded", "Degrade supplier weight", "Supplier performance is degraded or concurrency is saturated.", "reduce slow responses while preserving fallback capacity", []string{string(event.Type)}))
		}
	}
	return items
}

func (s *Service) actionFromReconciliation(now time.Time, summary adminplusdomain.ReconciliationSummary, minMargin float64) *adminplusdomain.ActionRecommendation {
	if summary.RevenueCents <= 0 {
		return nil
	}
	margin := float64(summary.ProfitCents) / float64(summary.RevenueCents)
	if minMargin <= 0 {
		minMargin = 0.1
	}
	if margin >= minMargin {
		return nil
	}
	return newAction(now, 0, nil, adminplusdomain.ActionTypeInvestigateProfit, adminplusdomain.ActionSeverityWarning, "profit_margin_below_threshold", "Investigate low profit margin", "Current reconciliation indicates margin is below the configured threshold.", "protect relay operator profitability", []string{"profit_margin_low"})
}

func indexSuppliers(items []SupplierSignal) map[int64]SupplierSignal {
	out := make(map[int64]SupplierSignal, len(items))
	for _, item := range items {
		if item.SupplierID > 0 {
			out[item.SupplierID] = item
		}
	}
	return out
}

func findBestSwitchCandidate(items []SupplierSignal) *SupplierSignal {
	var best *SupplierSignal
	for i := range items {
		item := items[i]
		if !adminplusdomain.CanUseSupplierForSwitching(item.RuntimeStatus, item.BalanceCents) {
			continue
		}
		if item.HealthStatus != "" && item.HealthStatus != adminplusdomain.SupplierHealthStatusNormal {
			continue
		}
		if best == nil || item.EffectiveCostCents < best.EffectiveCostCents {
			cp := item
			best = &cp
		}
	}
	return best
}

func newAction(now time.Time, supplierID int64, targetSupplierID *int64, actionType adminplusdomain.ActionType, severity adminplusdomain.ActionSeverity, reason string, title string, description string, impact string, signals []string) *adminplusdomain.ActionRecommendation {
	return &adminplusdomain.ActionRecommendation{
		SupplierID:       supplierID,
		TargetSupplierID: cloneInt64(targetSupplierID),
		Type:             actionType,
		Severity:         severity,
		Status:           adminplusdomain.ActionStatusOpen,
		ReasonCode:       reason,
		Title:            title,
		Description:      description,
		ExpectedImpact:   impact,
		RequiresApproval: true,
		Signals:          normalizeSignals(signals),
		CreatedAt:        now,
	}
}

func normalizeSignals(signals []string) []string {
	out := make([]string, 0, len(signals))
	for _, signal := range signals {
		v := strings.TrimSpace(signal)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func cloneInt64(in *int64) *int64 {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}

func severityRank(severity adminplusdomain.ActionSeverity) int {
	switch severity {
	case adminplusdomain.ActionSeverityCritical:
		return 3
	case adminplusdomain.ActionSeverityWarning:
		return 2
	case adminplusdomain.ActionSeverityInfo:
		return 1
	default:
		return 0
	}
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func badRequest(reason string, message string) error {
	return infraerrors.New(http.StatusBadRequest, reason, message)
}

func internalError(message string) error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", message)
}
