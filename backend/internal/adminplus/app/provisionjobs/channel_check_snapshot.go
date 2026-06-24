package provisionjobs

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/channelchecks"
)

func channelCheckInputFromSnapshot(supplierID int64, snapshot map[string]any) channelchecks.CheckInput {
	return channelchecks.CheckInput{
		SupplierID:              supplierID,
		SupplierGroupID:         int64Value(snapshot["supplier_group_id"]),
		CandidateLimit:          intValue(snapshot["candidate_limit"]),
		AutoPauseOnFailure:      boolValue(snapshot["auto_pause_on_failure"], true),
		ProbeModel:              stringValue(snapshot["probe_model"]),
		FirstTokenThresholdMS:   int64Value(snapshot["first_token_threshold_ms"]),
		TotalLatencyThresholdMS: int64Value(snapshot["total_latency_threshold_ms"]),
	}
}

func channelCheckResultSnapshot(result *channelchecks.CheckResult) map[string]any {
	if result == nil {
		return map[string]any{}
	}
	out := map[string]any{
		"supplier_id": result.SupplierID,
		"checked_at":  result.CheckedAt.Format(time.RFC3339),
		"total":       result.Total,
	}
	if result.Best != nil {
		out["best_supplier_group_id"] = result.Best.SupplierGroupID
		out["best_group_name"] = result.Best.GroupName
		out["best_effective_rate_multiplier"] = result.Best.EffectiveRateMultiplier
		out["best_first_token_ms"] = result.Best.FirstTokenMS
		out["best_duration_ms"] = result.Best.DurationMS
	}
	return out
}
