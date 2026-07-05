package kanban

import (
	"context"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

type AcceptanceReportGenerateInput struct {
	SupplyType            string
	SupplierID            int64
	LocalSub2APIAccountID int64
	Model                 string
	EnqueueEvidenceTasks  bool
	ObservedAt            *time.Time
}

func (s *Service) GenerateAcceptanceReport(ctx context.Context, in AcceptanceReportGenerateInput) (*adminplusdomain.AcceptanceReport, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("kanban service is not configured")
	}
	model := strings.TrimSpace(in.Model)
	supplierID := positiveID(in.SupplierID)
	accountID := positiveID(in.LocalSub2APIAccountID)
	if model == "" && supplierID == 0 && accountID == 0 {
		return nil, badRequest("KANBAN_ACCEPTANCE_TARGET_REQUIRED", "model, supplier_id or local_sub2api_account_id is required")
	}
	supplyType := normalizeSupplyType(in.SupplyType)
	quality, err := s.latestSupplyQuality(ctx, model, supplyType, supplierID, accountID)
	if err != nil {
		return nil, err
	}
	cache, err := s.latestCacheEfficiency(ctx, model, supplyType, supplierID, accountID)
	if err != nil {
		return nil, err
	}
	var schedulerRun *adminplusdomain.SchedulerRunSummary
	if in.EnqueueEvidenceTasks {
		schedulerRun, err = s.enqueueAcceptanceEvidenceTasks(ctx, supplyType, supplierID, accountID, model)
		if err != nil {
			return nil, err
		}
	}
	failureReasons := make([]string, 0)
	input := AcceptanceReportInput{
		SupplyType:            supplyType,
		SupplierID:            supplierID,
		LocalSub2APIAccountID: accountID,
		Model:                 model,
		ObservedAt:            in.ObservedAt,
		ConnectivityStatus:    statusFromAvailability(quality),
		ModelListStatus:       "unknown",
		PurityStatus:          statusFromScore(scorePtrFromQuality(quality, "purity"), 50, 80),
		TrialCallStatus:       statusFromTrialCall(quality),
		UsageMeteringStatus:   statusFromScore(scorePtrFromQuality(quality, "usage"), 50, 80),
		CacheAuditStatus:      statusFromCacheEvidence(cache, quality),
		BalanceStatus:         statusFromRiskScore(riskScorePtrFromQuality(quality, "balance"), 80, 40),
		ConcurrencyStatus:     statusFromScore(scorePtrFromQuality(quality, "concurrency"), 50, 80),
		ReportPayload: map[string]any{
			"quality_snapshot_id":  snapshotID(quality),
			"cache_snapshot_id":    snapshotID(cache),
			"generator":            "acceptance_from_quality_cache_v1",
			"evidence_model":       firstNonEmptyString(modelFromQuality(quality), modelFromCache(cache), model),
			"evidence_supply_type": firstNonEmptyString(supplyTypeFromQuality(quality), supplyTypeFromCache(cache), supplyType),
		},
	}
	if schedulerRun != nil {
		input.ReportPayload["evidence_scheduler_run_id"] = schedulerRun.ID
		input.ReportPayload["evidence_scheduler_status"] = schedulerRun.Status
		input.ReportPayload["evidence_scheduler_task_type"] = schedulerRun.TaskType
		input.ReportPayload["evidence_scheduler_total_steps"] = schedulerRun.TotalSteps
		input.ReportPayload["evidence_scheduler_requested_at"] = schedulerRun.RequestedAt.UTC().Format(time.RFC3339)
	}
	collectAcceptanceFailureReasons(&failureReasons, input)
	input.FailureReason = strings.Join(failureReasons, "; ")
	if input.Model == "" {
		input.Model = firstNonEmptyString(modelFromQuality(quality), modelFromCache(cache))
	}
	return s.RecordAcceptanceReport(ctx, input)
}

func (s *Service) latestSupplyQuality(ctx context.Context, model string, supplyType string, supplierID int64, accountID int64) (*adminplusdomain.SupplyQualitySnapshot, error) {
	items, err := s.ListSupplyQuality(ctx, SupplyQualityFilter{Model: model, SupplyType: supplyType, SupplierID: supplierID, LocalSub2APIAccountID: accountID, Limit: 20})
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item != nil {
			return item, nil
		}
	}
	return nil, nil
}

func (s *Service) latestCacheEfficiency(ctx context.Context, model string, supplyType string, supplierID int64, accountID int64) (*adminplusdomain.CacheEfficiencySnapshot, error) {
	items, err := s.ListCacheEfficiency(ctx, CacheEfficiencyFilter{Model: model, SupplyType: supplyType, SupplierID: supplierID, LocalSub2APIAccountID: accountID, Limit: 20})
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item != nil {
			return item, nil
		}
	}
	return nil, nil
}

func statusFromAvailability(quality *adminplusdomain.SupplyQualitySnapshot) string {
	if quality == nil || quality.AvailabilityRatio <= 0 {
		return "unknown"
	}
	switch {
	case quality.AvailabilityRatio < 0.8 || quality.ErrorRatio > 0.2:
		return "fail"
	case quality.AvailabilityRatio < 0.95 || quality.ErrorRatio > 0.05:
		return "warn"
	default:
		return "pass"
	}
}

func statusFromTrialCall(quality *adminplusdomain.SupplyQualitySnapshot) string {
	if quality == nil {
		return "unknown"
	}
	if quality.AvailabilityRatio > 0 && (quality.AvailabilityRatio < 0.8 || quality.ErrorRatio > 0.2) {
		return "fail"
	}
	if quality.AvgTTFTMS != nil && *quality.AvgTTFTMS > 8000 {
		return "fail"
	}
	if quality.AvgTotalLatencyMS != nil && *quality.AvgTotalLatencyMS > 60000 {
		return "fail"
	}
	if quality.AvailabilityRatio > 0 && (quality.AvailabilityRatio < 0.95 || quality.ErrorRatio > 0.05) {
		return "warn"
	}
	if quality.AvgTTFTMS != nil && *quality.AvgTTFTMS > 4000 {
		return "warn"
	}
	if quality.AvgTotalLatencyMS != nil && *quality.AvgTotalLatencyMS > 30000 {
		return "warn"
	}
	if quality.AvailabilityRatio > 0 || quality.AvgTTFTMS != nil || quality.AvgTotalLatencyMS != nil {
		return "pass"
	}
	return "unknown"
}

func statusFromCacheEvidence(cache *adminplusdomain.CacheEfficiencySnapshot, quality *adminplusdomain.SupplyQualitySnapshot) string {
	if cache != nil {
		if cache.Status == "bad" || cache.CacheHitRatio < defaultCacheRiskHitRatio {
			return "fail"
		}
		if cache.Status == "risky" || cache.Status == "watching" || cache.CacheHitRatio < 0.65 {
			return "warn"
		}
		if cache.Status == "healthy" || cache.CacheHitRatio >= 0.65 {
			return "pass"
		}
	}
	if quality != nil && quality.CacheHitRatio > 0 {
		if quality.CacheHitRatio < defaultCacheRiskHitRatio {
			return "fail"
		}
		if quality.CacheHitRatio < 0.65 {
			return "warn"
		}
		return "pass"
	}
	return "unknown"
}

func statusFromScore(score *float64, failBelow float64, warnBelow float64) string {
	if score == nil || *score <= 0 {
		return "unknown"
	}
	if *score < failBelow {
		return "fail"
	}
	if *score < warnBelow {
		return "warn"
	}
	return "pass"
}

func statusFromRiskScore(score *float64, failAbove float64, warnAbove float64) string {
	if score == nil || *score < 0 {
		return "unknown"
	}
	if *score > failAbove {
		return "fail"
	}
	if *score > warnAbove {
		return "warn"
	}
	return "pass"
}

func scorePtrFromQuality(quality *adminplusdomain.SupplyQualitySnapshot, key string) *float64 {
	if quality == nil {
		return nil
	}
	var value float64
	switch key {
	case "purity":
		value = quality.PurityScore
	case "usage":
		value = quality.UsageTrustScore
	case "concurrency":
		value = quality.ConcurrencyScore
	default:
		return nil
	}
	return &value
}

func riskScorePtrFromQuality(quality *adminplusdomain.SupplyQualitySnapshot, key string) *float64 {
	if quality == nil || key != "balance" {
		return nil
	}
	value := quality.BalanceRiskScore
	return &value
}

func collectAcceptanceFailureReasons(out *[]string, input AcceptanceReportInput) {
	appendReason := func(status string, reason string) {
		if status == "fail" || status == "warn" || status == "unknown" {
			*out = append(*out, reason+":"+status)
		}
	}
	appendReason(input.ConnectivityStatus, "connectivity")
	appendReason(input.ModelListStatus, "model_list")
	appendReason(input.PurityStatus, "purity")
	appendReason(input.TrialCallStatus, "trial_call")
	appendReason(input.UsageMeteringStatus, "usage_metering")
	appendReason(input.CacheAuditStatus, "cache_audit")
	appendReason(input.BalanceStatus, "balance")
	appendReason(input.ConcurrencyStatus, "concurrency")
}

func snapshotID(item any) int64 {
	switch v := item.(type) {
	case *adminplusdomain.SupplyQualitySnapshot:
		if v != nil {
			return v.ID
		}
	case *adminplusdomain.CacheEfficiencySnapshot:
		if v != nil {
			return v.ID
		}
	}
	return 0
}

func modelFromQuality(item *adminplusdomain.SupplyQualitySnapshot) string {
	if item == nil {
		return ""
	}
	return item.Model
}

func modelFromCache(item *adminplusdomain.CacheEfficiencySnapshot) string {
	if item == nil {
		return ""
	}
	return item.Model
}

func supplyTypeFromQuality(item *adminplusdomain.SupplyQualitySnapshot) string {
	if item == nil {
		return ""
	}
	return item.SupplyType
}

func supplyTypeFromCache(item *adminplusdomain.CacheEfficiencySnapshot) string {
	if item == nil {
		return ""
	}
	return item.SupplyType
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
