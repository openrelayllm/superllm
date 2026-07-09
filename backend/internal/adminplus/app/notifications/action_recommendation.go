package notifications

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

func (s *Service) DispatchActionRecommendation(ctx context.Context, action *adminplusdomain.ActionRecommendation) error {
	if action == nil || !shouldNotifyActionRecommendation(action) {
		return nil
	}
	eventType := actionRecommendationEventType(action)
	if eventType == "" {
		return nil
	}
	return s.Dispatch(ctx, DispatchInput{
		Type:        eventType,
		ID:          action.ID,
		SupplierID:  action.SupplierID,
		DedupeKey:   actionRecommendationDedupeKey(eventType, action),
		ThrottleKey: actionRecommendationThrottleKey(eventType, action),
		Text:        actionRecommendationText(action),
		Payload: map[string]any{
			"action_recommendation_id": action.ID,
			"action_type":              string(action.Type),
			"severity":                 string(action.Severity),
			"reason_code":              action.ReasonCode,
			"supplier_id":              action.SupplierID,
			"target_supplier_id":       nullableActionTargetID(action.TargetSupplierID),
			"signals":                  append([]string(nil), action.Signals...),
		},
	})
}

func shouldNotifyActionRecommendation(action *adminplusdomain.ActionRecommendation) bool {
	if action.Status != "" && action.Status != adminplusdomain.ActionStatusOpen {
		return false
	}
	switch action.Severity {
	case adminplusdomain.ActionSeverityCritical, adminplusdomain.ActionSeverityWarning:
		return true
	default:
		return false
	}
}

func actionRecommendationEventType(action *adminplusdomain.ActionRecommendation) string {
	reason := strings.ToLower(strings.TrimSpace(action.ReasonCode))
	switch {
	case strings.Contains(reason, "balance"), strings.Contains(reason, "depleted"), action.Type == adminplusdomain.ActionTypeRechargeSupplier:
		return "action.balance_required"
	case strings.Contains(reason, "key_capacity"), strings.Contains(reason, "key_provisioning"):
		return "action.key_capacity"
	case strings.Contains(reason, "routing_refill"), strings.Contains(reason, "routing_low_capacity"), action.Type == adminplusdomain.ActionTypeRoutingRefill:
		return "action.routing_capacity"
	case strings.Contains(reason, "local_state"), strings.Contains(reason, "drift"):
		return "action.local_state"
	case strings.Contains(reason, "channel_monitor"), strings.Contains(reason, "active_probe"), action.Type == adminplusdomain.ActionTypeLocalAccountScheduleDisable:
		return "action.channel_failure"
	case strings.Contains(reason, "proxy"):
		return "action.proxy_review"
	case strings.Contains(reason, "purity"):
		return "action.purity_review"
	case strings.Contains(reason, "credential"):
		return "action.credential_review"
	case strings.Contains(reason, "cost"), strings.Contains(reason, "reconcile"), action.Type == adminplusdomain.ActionTypeSupplierCostReconcileAdjustment:
		return "action.cost_reconcile"
	case strings.Contains(reason, "kanban"), strings.Contains(reason, "profit"), action.Type == adminplusdomain.ActionTypeInvestigateProfit:
		return "action.profit_review"
	default:
		return "action.operational_review"
	}
}

func actionRecommendationDedupeKey(eventType string, action *adminplusdomain.ActionRecommendation) string {
	return strings.Join([]string{
		"action",
		eventType,
		string(action.Type),
		strings.TrimSpace(action.ReasonCode),
		strconv.FormatInt(action.SupplierID, 10),
		actionRecommendationScopeSignal(action),
	}, ":")
}

func actionRecommendationThrottleKey(eventType string, action *adminplusdomain.ActionRecommendation) string {
	return strings.Join([]string{
		"action",
		eventType,
		strings.TrimSpace(action.ReasonCode),
		strconv.FormatInt(action.SupplierID, 10),
		actionRecommendationScopeSignal(action),
	}, ":")
}

func actionRecommendationScopeSignal(action *adminplusdomain.ActionRecommendation) string {
	for _, key := range []string{"local_group_id", "local_sub2api_account_id", "supplier_group_id", "candidate_local_account_id", "cost_snapshot_id"} {
		if value := signalValue(action.Signals, key); value != "" {
			return key + "=" + value
		}
	}
	if action.TargetSupplierID != nil && *action.TargetSupplierID > 0 {
		return "target_supplier_id=" + strconv.FormatInt(*action.TargetSupplierID, 10)
	}
	if action.ID > 0 {
		return "action_id=" + strconv.FormatInt(action.ID, 10)
	}
	return "global"
}

func signalValue(signals []string, key string) string {
	prefix := key + "="
	for _, signal := range signals {
		if strings.HasPrefix(signal, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(signal, prefix))
		}
	}
	return ""
}

func actionRecommendationText(action *adminplusdomain.ActionRecommendation) string {
	parts := []string{
		fmt.Sprintf("Sub2API Admin Plus 动作建议：%s", strings.TrimSpace(action.Title)),
		fmt.Sprintf("级别：%s", actionSeverityText(action.Severity)),
		fmt.Sprintf("原因：%s", strings.TrimSpace(action.ReasonCode)),
	}
	if action.SupplierID > 0 {
		parts = append(parts, fmt.Sprintf("供应商 ID：%d", action.SupplierID))
	}
	if action.Description != "" {
		parts = append(parts, "说明："+strings.TrimSpace(action.Description))
	}
	if action.ExpectedImpact != "" {
		parts = append(parts, "影响："+strings.TrimSpace(action.ExpectedImpact))
	}
	if len(action.Signals) > 0 {
		parts = append(parts, "信号："+strings.Join(firstSignals(action.Signals, 6), "；"))
	}
	return strings.Join(parts, "\n")
}

func actionSeverityText(severity adminplusdomain.ActionSeverity) string {
	switch severity {
	case adminplusdomain.ActionSeverityCritical:
		return "严重"
	case adminplusdomain.ActionSeverityWarning:
		return "警告"
	default:
		return string(severity)
	}
}

func firstSignals(signals []string, limit int) []string {
	if len(signals) <= limit {
		return append([]string(nil), signals...)
	}
	return append([]string(nil), signals[:limit]...)
}

func nullableActionTargetID(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}
