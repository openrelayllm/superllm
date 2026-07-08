package provisionjobs

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/costs"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/supplierkeys"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

func provisionInputFromSnapshot(supplierID int64, step *adminplusdomain.SupplierProvisionStep, snapshot map[string]any) (supplierkeys.ProvisionKeyInput, error) {
	groupID := int64Value(snapshot["supplier_group_id"])
	if groupID <= 0 && step != nil {
		groupID = step.SupplierGroupID
	}
	if groupID <= 0 {
		return supplierkeys.ProvisionKeyInput{}, badRequest("SUPPLIER_GROUP_ID_INVALID", "invalid supplier group id")
	}
	return supplierkeys.ProvisionKeyInput{
		SupplierID:                 supplierID,
		SupplierGroupID:            groupID,
		Name:                       stringValue(snapshot["name"]),
		SyncProviderName:           boolValue(snapshot["sync_provider_name"], false),
		QuotaUSD:                   float64Value(snapshot["quota_usd"]),
		ExpiresInDays:              intPtrValue(snapshot["expires_in_days"]),
		LocalAccountPlatform:       stringValue(snapshot["local_account_platform"]),
		LocalAccountName:           stringValue(snapshot["local_account_name"]),
		LocalAccountBaseURL:        stringValue(snapshot["local_account_base_url"]),
		LocalAccountConcurrency:    intValue(snapshot["local_account_concurrency"]),
		LocalAccountPriority:       intValue(snapshot["local_account_priority"]),
		LocalAccountRateMultiplier: float64PtrValue(snapshot["local_account_rate_multiplier"]),
		LocalAccountGroupIDs:       int64SliceValue(snapshot["local_account_group_ids"]),
		RuntimeStatus:              adminplusdomain.NormalizeSupplierRuntimeStatus(stringValue(snapshot["runtime_status"])),
		HealthStatus:               adminplusdomain.NormalizeSupplierHealthStatus(stringValue(snapshot["health_status"])),
		BalanceThresholdCents:      int64Value(snapshot["balance_threshold_cents"]),
		BalanceCents:               int64Value(snapshot["balance_cents"]),
		BalanceCurrency:            stringValue(snapshot["balance_currency"]),
	}, nil
}

func ensureAllInputFromSnapshot(supplierID int64, snapshot map[string]any) supplierkeys.EnsureAllInput {
	return supplierkeys.EnsureAllInput{
		SupplierID:               supplierID,
		SyncProviderName:         boolValue(snapshot["sync_provider_name"], false),
		AllowPartial:             boolValue(snapshot["allow_partial"], false),
		SupplierGroupPriorityIDs: int64SliceValue(snapshot["supplier_group_priority_ids"]),
		LocalAccountBaseURL:      stringValue(snapshot["local_account_base_url"]),
		LocalAccountConcurrency:  intValue(snapshot["local_account_concurrency"]),
		LocalAccountPriority:     intValue(snapshot["local_account_priority"]),
		LocalAccountGroupIDs:     int64SliceValue(snapshot["local_account_group_ids"]),
		RuntimeStatus:            adminplusdomain.NormalizeSupplierRuntimeStatus(stringValue(snapshot["runtime_status"])),
		HealthStatus:             adminplusdomain.NormalizeSupplierHealthStatus(stringValue(snapshot["health_status"])),
		BalanceThresholdCents:    int64Value(snapshot["balance_threshold_cents"]),
		BalanceCents:             int64Value(snapshot["balance_cents"]),
		BalanceCurrency:          stringValue(snapshot["balance_currency"]),
	}
}

func ensureGroupInputFromSnapshot(supplierID int64, step *adminplusdomain.SupplierProvisionStep) supplierkeys.EnsureGroupInput {
	snapshot := map[string]any{}
	groupID := int64(0)
	if step != nil {
		snapshot = step.RequestSnapshot
		groupID = step.SupplierGroupID
	}
	if groupID <= 0 {
		groupID = int64Value(snapshot["supplier_group_id"])
	}
	return supplierkeys.EnsureGroupInput{
		EnsureAllInput:  ensureAllInputFromSnapshot(supplierID, snapshot),
		SupplierGroupID: groupID,
	}
}

func ensureGroupResultSnapshot(item *supplierkeys.EnsureAllResultItem) map[string]any {
	if item == nil {
		return map[string]any{}
	}
	out := map[string]any{
		"supplier_group_id":         item.SupplierGroupID,
		"external_group_id":         item.ExternalGroupID,
		"group_name":                item.GroupName,
		"action":                    item.Action,
		"local_sub2api_group_id":    item.LocalSub2APIGroupID,
		"local_sub2api_group_name":  item.LocalSub2APIGroupName,
		"local_group_created":       item.LocalGroupCreated,
		"local_account_group_bound": item.LocalAccountGroupBound,
		"local_sub2api_account_id":  int64(0),
		"supplier_key_id":           int64(0),
		"supplier_key_status":       "",
		"supplier_binding_id":       int64(0),
	}
	if item.Key != nil {
		out["supplier_key_id"] = item.Key.ID
		out["supplier_key_status"] = string(item.Key.Status)
		out["local_sub2api_account_id"] = item.Key.LocalSub2APIAccountID
	}
	if item.Binding != nil {
		out["supplier_binding_id"] = item.Binding.ID
		out["local_sub2api_account_id"] = item.Binding.LocalSub2APIAccountID
	}
	if strings.TrimSpace(item.ErrorCode) != "" {
		out["error_code"] = item.ErrorCode
	}
	if strings.TrimSpace(item.ErrorMessage) != "" {
		out["error_message"] = item.ErrorMessage
	}
	return out
}

func costSyncInputFromSnapshot(supplierID int64, snapshot map[string]any) costs.SyncInput {
	return costs.SyncInput{
		SupplierID:                     supplierID,
		StartedAt:                      timePtrValue(snapshot["started_at"]),
		EndedAt:                        timePtrValue(snapshot["ended_at"]),
		IncludeFundingTransactions:     boolValue(snapshot["include_funding_transactions"], true),
		IncludeEntitlementTransactions: boolValue(snapshot["include_entitlement_transactions"], true),
		IncludeUsageCostLines:          boolValue(snapshot["include_usage_cost_lines"], true),
		IncludeBalanceSnapshot:         boolValue(snapshot["include_balance_snapshot"], true),
		LowBalanceThresholdCents:       int64Value(snapshot["low_balance_threshold_cents"]),
	}
}

func costSyncResultSnapshot(result *costs.SyncResult) map[string]any {
	if result == nil {
		return map[string]any{}
	}
	out := map[string]any{
		"supplier_id":              result.SupplierID,
		"provider_type":            result.ProviderType,
		"system_type":              result.SystemType,
		"origin":                   result.Origin,
		"api_base_url":             result.APIBaseURL,
		"synced_at":                result.SyncedAt,
		"funding_transactions":     result.FundingTransactions,
		"entitlement_transactions": result.EntitlementTransactions,
		"usage_cost_lines":         result.UsageCostLines,
		"ledger_entries":           result.LedgerEntries,
		"capabilities":             result.Capabilities,
	}
	if len(result.Diagnostics) > 0 {
		out["diagnostics"] = result.Diagnostics
	}
	if result.Snapshot != nil {
		out["snapshot_id"] = result.Snapshot.ID
		out["currency"] = result.Snapshot.Currency
		out["usage_cost_cents"] = result.Snapshot.UsageCostCents
		out["expected_balance_cents"] = result.Snapshot.ExpectedBalanceCents
		out["actual_balance_cents"] = result.Snapshot.ActualBalanceCents
		out["balance_delta_cents"] = result.Snapshot.BalanceDeltaCents
	}
	return out
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func intValue(value any) int {
	return int(int64Value(value))
}

func int64Value(value any) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case json.Number:
		n, _ := strconv.ParseInt(v.String(), 10, 64)
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return n
	default:
		return 0
	}
}

func float64Value(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		n, _ := strconv.ParseFloat(v.String(), 64)
		return n
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return n
	default:
		return 0
	}
}

func boolValue(value any, fallback bool) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		default:
			return fallback
		}
	case nil:
		return fallback
	default:
		return fallback
	}
}

func timePtrValue(value any) *time.Time {
	raw := stringValue(value)
	if raw == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	utc := parsed.UTC()
	return &utc
}

func intPtrValue(value any) *int {
	if value == nil {
		return nil
	}
	n := intValue(value)
	return &n
}

func float64PtrValue(value any) *float64 {
	if value == nil {
		return nil
	}
	n := float64Value(value)
	return &n
}

func int64SliceValue(value any) []int64 {
	switch v := value.(type) {
	case []int64:
		return append([]int64(nil), v...)
	case []int:
		out := make([]int64, 0, len(v))
		for _, item := range v {
			out = append(out, int64(item))
		}
		return out
	case []any:
		out := make([]int64, 0, len(v))
		for _, item := range v {
			if n := int64Value(item); n > 0 {
				out = append(out, n)
			}
		}
		return out
	default:
		return nil
	}
}
