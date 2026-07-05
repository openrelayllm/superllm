package kanban

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

type usageDerivedRow struct {
	supplyType             string
	supplierID             int64
	accountID              int64
	model                  string
	requestCount           int64
	accountCount           int64
	inputTokens            int64
	outputTokens           int64
	cacheReadTokens        int64
	cacheWriteTokens       int64
	revenueCents           int64
	avgTTFTMS              sql.NullInt64
	avgTotalLatencyMS      sql.NullInt64
	modelComparableCount   int64
	modelMismatchCount     int64
	degradedAccountCount   int64
	usableBalanceCount     int64
	balanceCents           int64
	balanceThresholdCents  int64
	configuredConcurrency  int64
	observedMaxConcurrency int64
	observedAt             time.Time
	errorCount             int64
}

func (r *SQLRepository) ListUsageDerivedSnapshots(ctx context.Context, filter UsageDerivedFilter) (*UsageDerivedSnapshots, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	if filter.Since.IsZero() || filter.Until.IsZero() || !filter.Since.Before(filter.Until) {
		return &UsageDerivedSnapshots{}, nil
	}
	supplyType := normalizeSupplyTypeFilter(filter.SupplyType)
	if supplyType == "competitor" || supplyType == "custom" {
		return &UsageDerivedSnapshots{}, nil
	}

	args := []any{filter.Since.UTC(), filter.Until.UTC()}
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	usageWhere := []string{
		"ul.created_at >= $1",
		"ul.created_at < $2",
		"COALESCE(NULLIF(ul.requested_model, ''), NULLIF(ul.model, ''), '') <> ''",
	}
	errorWhere := []string{
		"oel.created_at >= $1",
		"oel.created_at < $2",
		"oel.account_id IS NOT NULL",
		"COALESCE(NULLIF(oel.requested_model, ''), NULLIF(oel.model, ''), '') <> ''",
		"NOT COALESCE(oel.is_business_limited, false)",
	}
	if strings.TrimSpace(filter.Model) != "" {
		ref := addArg(strings.TrimSpace(filter.Model))
		usageWhere = append(usageWhere, "COALESCE(NULLIF(ul.requested_model, ''), NULLIF(ul.model, ''), '') = "+ref)
		errorWhere = append(errorWhere, "COALESCE(NULLIF(oel.requested_model, ''), NULLIF(oel.model, ''), '') = "+ref)
	}
	if supplyType != "" {
		ref := addArg(supplyType)
		usageWhere = append(usageWhere, "CASE WHEN asa.supplier_id IS NULL THEN 'own_pool' ELSE 'supplier' END = "+ref)
		errorWhere = append(errorWhere, "CASE WHEN asa.supplier_id IS NULL THEN 'own_pool' ELSE 'supplier' END = "+ref)
	}
	if filter.SupplierID > 0 {
		ref := addArg(filter.SupplierID)
		usageWhere = append(usageWhere, "asa.supplier_id = "+ref)
		errorWhere = append(errorWhere, "asa.supplier_id = "+ref)
	}
	if filter.LocalSub2APIAccountID > 0 {
		ref := addArg(filter.LocalSub2APIAccountID)
		usageWhere = append(usageWhere, "ul.account_id = "+ref)
		errorWhere = append(errorWhere, "oel.account_id = "+ref)
	}

	accountSelect := "0::BIGINT"
	accountGroup := ""
	accountJoin := ""
	if filter.LocalSub2APIAccountID > 0 {
		accountSelect = "account_id"
		accountGroup = ", account_id"
		accountJoin = " AND eg.account_id = ug.account_id"
	}
	limitRef := addArg(normalizeLimit(filter.Limit))

	rows, err := r.db.QueryContext(ctx, `
		WITH usage_base AS (
			SELECT
				CASE WHEN asa.supplier_id IS NULL THEN 'own_pool' ELSE 'supplier' END AS supply_type,
				COALESCE(asa.supplier_id, 0)::BIGINT AS supplier_id,
				ul.account_id::BIGINT AS account_id,
				COALESCE(NULLIF(ul.requested_model, ''), NULLIF(ul.model, ''), '') AS model,
				COALESCE(ul.input_tokens, 0)::BIGINT AS input_tokens,
				COALESCE(ul.output_tokens, 0)::BIGINT AS output_tokens,
				COALESCE(ul.cache_read_tokens, 0)::BIGINT AS cache_read_tokens,
				GREATEST(
					COALESCE(ul.cache_creation_tokens, 0),
					COALESCE(ul.cache_creation_5m_tokens, 0) + COALESCE(ul.cache_creation_1h_tokens, 0)
				)::BIGINT AS cache_write_tokens,
				COALESCE(ul.actual_cost, ul.total_cost, 0)::NUMERIC AS revenue_usd,
				ul.first_token_ms,
				ul.duration_ms,
				COALESCE(NULLIF(ul.upstream_model, ''), '') AS upstream_model,
				CASE
					WHEN COALESCE(a.status, 'active') <> 'active'
						OR COALESCE(asa.health_status, 'normal') <> 'normal'
						OR COALESCE(s.health_status, 'normal') <> 'normal'
						OR COALESCE(asa.runtime_status, 'active') = 'disabled'
						OR COALESCE(s.runtime_status, 'active') = 'disabled'
					THEN TRUE ELSE FALSE
				END AS degraded_account,
				CASE
					WHEN asa.supplier_id IS NULL THEN TRUE
					WHEN COALESCE(asa.has_usable_balance, false) THEN TRUE
					WHEN COALESCE(asa.balance_cents, 0) > COALESCE(asa.balance_threshold_cents, 0) THEN TRUE
					WHEN COALESCE(s.balance_cents, 0) > 0 THEN TRUE
					ELSE FALSE
				END AS has_usable_balance,
				GREATEST(COALESCE(asa.balance_cents, 0), COALESCE(s.balance_cents, 0))::BIGINT AS balance_cents,
				COALESCE(asa.balance_threshold_cents, 0)::BIGINT AS balance_threshold_cents,
				COALESCE(NULLIF(asa.configured_concurrency, 0), a.concurrency, 0)::BIGINT AS configured_concurrency,
				COALESCE(asa.observed_max_concurrency, 0)::BIGINT AS observed_max_concurrency,
				ul.created_at
			FROM usage_logs ul
			LEFT JOIN accounts a ON a.id = ul.account_id
			LEFT JOIN admin_plus_supplier_accounts asa ON asa.local_sub2api_account_id = ul.account_id
			LEFT JOIN admin_plus_suppliers s ON s.id = asa.supplier_id
			WHERE `+strings.Join(usageWhere, " AND ")+`
		),
		usage_group AS (
			SELECT
				supply_type,
				supplier_id,
				`+accountSelect+` AS account_id,
				model,
				COUNT(*)::BIGINT AS request_count,
				COUNT(DISTINCT account_id)::BIGINT AS account_count,
				COALESCE(SUM(input_tokens), 0)::BIGINT AS input_tokens,
				COALESCE(SUM(output_tokens), 0)::BIGINT AS output_tokens,
				COALESCE(SUM(cache_read_tokens), 0)::BIGINT AS cache_read_tokens,
				COALESCE(SUM(cache_write_tokens), 0)::BIGINT AS cache_write_tokens,
				ROUND(COALESCE(SUM(revenue_usd), 0) * 100)::BIGINT AS revenue_cents,
				ROUND(AVG(NULLIF(first_token_ms, 0)))::BIGINT AS avg_ttft_ms,
				ROUND(AVG(NULLIF(duration_ms, 0)))::BIGINT AS avg_total_latency_ms,
				COUNT(*) FILTER (WHERE upstream_model <> '')::BIGINT AS model_comparable_count,
				COUNT(*) FILTER (WHERE upstream_model <> '' AND LOWER(upstream_model) <> LOWER(model))::BIGINT AS model_mismatch_count,
				COUNT(DISTINCT account_id) FILTER (WHERE degraded_account)::BIGINT AS degraded_account_count,
				COUNT(DISTINCT account_id) FILTER (WHERE has_usable_balance)::BIGINT AS usable_balance_count,
				MAX(balance_cents)::BIGINT AS balance_cents,
				MAX(balance_threshold_cents)::BIGINT AS balance_threshold_cents,
				MAX(configured_concurrency)::BIGINT AS configured_concurrency,
				MAX(observed_max_concurrency)::BIGINT AS observed_max_concurrency,
				MAX(created_at) AS observed_at
			FROM usage_base
			GROUP BY supply_type, supplier_id, model`+accountGroup+`
		),
		error_base AS (
			SELECT
				CASE WHEN asa.supplier_id IS NULL THEN 'own_pool' ELSE 'supplier' END AS supply_type,
				COALESCE(asa.supplier_id, 0)::BIGINT AS supplier_id,
				oel.account_id::BIGINT AS account_id,
				COALESCE(NULLIF(oel.requested_model, ''), NULLIF(oel.model, ''), '') AS model,
				oel.created_at
			FROM ops_error_logs oel
			LEFT JOIN admin_plus_supplier_accounts asa ON asa.local_sub2api_account_id = oel.account_id
			WHERE `+strings.Join(errorWhere, " AND ")+`
		),
		error_group AS (
			SELECT
				supply_type,
				supplier_id,
				`+accountSelect+` AS account_id,
				model,
				COUNT(*)::BIGINT AS error_count,
				COUNT(DISTINCT account_id)::BIGINT AS error_account_count,
				MAX(created_at) AS observed_at
			FROM error_base
			GROUP BY supply_type, supplier_id, model`+accountGroup+`
		)
		SELECT
			COALESCE(ug.supply_type, eg.supply_type) AS supply_type,
			COALESCE(ug.supplier_id, eg.supplier_id, 0)::BIGINT AS supplier_id,
			COALESCE(ug.account_id, eg.account_id, 0)::BIGINT AS account_id,
			COALESCE(ug.model, eg.model, '') AS model,
			COALESCE(ug.request_count, 0)::BIGINT AS request_count,
			CASE
				WHEN COALESCE(ug.account_count, 0) > 0 THEN ug.account_count
				ELSE COALESCE(eg.error_account_count, 0)
			END::BIGINT AS account_count,
			COALESCE(ug.input_tokens, 0)::BIGINT AS input_tokens,
			COALESCE(ug.output_tokens, 0)::BIGINT AS output_tokens,
			COALESCE(ug.cache_read_tokens, 0)::BIGINT AS cache_read_tokens,
			COALESCE(ug.cache_write_tokens, 0)::BIGINT AS cache_write_tokens,
			COALESCE(ug.revenue_cents, 0)::BIGINT AS revenue_cents,
			ug.avg_ttft_ms,
			ug.avg_total_latency_ms,
			COALESCE(ug.model_comparable_count, 0)::BIGINT AS model_comparable_count,
			COALESCE(ug.model_mismatch_count, 0)::BIGINT AS model_mismatch_count,
			COALESCE(ug.degraded_account_count, 0)::BIGINT AS degraded_account_count,
			COALESCE(ug.usable_balance_count, 0)::BIGINT AS usable_balance_count,
			COALESCE(ug.balance_cents, 0)::BIGINT AS balance_cents,
			COALESCE(ug.balance_threshold_cents, 0)::BIGINT AS balance_threshold_cents,
			COALESCE(ug.configured_concurrency, 0)::BIGINT AS configured_concurrency,
			COALESCE(ug.observed_max_concurrency, 0)::BIGINT AS observed_max_concurrency,
			CASE
				WHEN ug.observed_at IS NULL THEN eg.observed_at
				WHEN eg.observed_at IS NULL THEN ug.observed_at
				WHEN ug.observed_at >= eg.observed_at THEN ug.observed_at
				ELSE eg.observed_at
			END AS observed_at,
			COALESCE(eg.error_count, 0)::BIGINT AS error_count
		FROM usage_group ug
		FULL OUTER JOIN error_group eg
			ON ug.supply_type = eg.supply_type
			AND ug.supplier_id = eg.supplier_id
			AND ug.model = eg.model`+accountJoin+`
		ORDER BY
			CASE
				WHEN ug.observed_at IS NULL THEN eg.observed_at
				WHEN eg.observed_at IS NULL THEN ug.observed_at
				WHEN ug.observed_at >= eg.observed_at THEN ug.observed_at
				ELSE eg.observed_at
			END DESC,
			COALESCE(ug.request_count, 0) DESC
		LIMIT `+limitRef, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := &UsageDerivedSnapshots{}
	var idx int64
	for rows.Next() {
		idx++
		var row usageDerivedRow
		if err := rows.Scan(
			&row.supplyType,
			&row.supplierID,
			&row.accountID,
			&row.model,
			&row.requestCount,
			&row.accountCount,
			&row.inputTokens,
			&row.outputTokens,
			&row.cacheReadTokens,
			&row.cacheWriteTokens,
			&row.revenueCents,
			&row.avgTTFTMS,
			&row.avgTotalLatencyMS,
			&row.modelComparableCount,
			&row.modelMismatchCount,
			&row.degradedAccountCount,
			&row.usableBalanceCount,
			&row.balanceCents,
			&row.balanceThresholdCents,
			&row.configuredConcurrency,
			&row.observedMaxConcurrency,
			&row.observedAt,
			&row.errorCount,
		); err != nil {
			return nil, err
		}
		if row.requestCount > 0 {
			result.Cache = append(result.Cache, cacheSnapshotFromUsageDerivedRow(row, -idx))
		}
		result.Quality = append(result.Quality, qualitySnapshotFromUsageDerivedRow(row, -100000-idx))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func cacheSnapshotFromUsageDerivedRow(row usageDerivedRow, id int64) *adminplusdomain.CacheEfficiencySnapshot {
	denominator := row.cacheReadTokens + row.cacheWriteTokens + row.inputTokens
	hitRatio := 0.0
	if denominator > 0 {
		hitRatio = float64(row.cacheReadTokens) / float64(denominator)
	}
	duplicateInputTokens := int64(0)
	if row.cacheReadTokens+row.cacheWriteTokens > 0 && row.inputTokens > row.cacheReadTokens {
		duplicateInputTokens = row.inputTokens - row.cacheReadTokens
	}
	totalTokens := row.inputTokens + row.outputTokens + row.cacheReadTokens + row.cacheWriteTokens
	estimatedWasteCents := int64(0)
	if duplicateInputTokens > 0 && totalTokens > 0 && row.revenueCents > 0 {
		estimatedWasteCents = (row.revenueCents*duplicateInputTokens + totalTokens/2) / totalTokens
	}
	status := statusFromCacheHitRatio(hitRatio)
	if status == "unknown" && row.accountCount > 1 && row.requestCount > 1 && row.inputTokens > 0 {
		status = "bad"
	}
	routingStrategy := "fixed_account"
	if row.accountCount > 1 {
		routingStrategy = "round_robin"
	}
	return &adminplusdomain.CacheEfficiencySnapshot{
		ID:                    id,
		SupplyType:            row.supplyType,
		SupplierID:            row.supplierID,
		LocalSub2APIAccountID: row.accountID,
		Model:                 row.model,
		RoutingStrategy:       routingStrategy,
		StickyScope:           "none",
		SampleRequests:        int(clampInt64ToInt(row.requestCount)),
		CacheReadTokens:       row.cacheReadTokens,
		CacheWriteTokens:      row.cacheWriteTokens,
		InputTokens:           row.inputTokens,
		OutputTokens:          row.outputTokens,
		CacheHitRatio:         hitRatio,
		DuplicateInputTokens:  duplicateInputTokens,
		EstimatedWasteCents:   estimatedWasteCents,
		AvgTTFTMS:             nullInt64Ptr(row.avgTTFTMS),
		AvgTotalLatencyMS:     nullInt64Ptr(row.avgTotalLatencyMS),
		Status:                status,
		Notes:                 usageDerivedNotes(row),
		ObservedAt:            row.observedAt.UTC(),
		RawPayload:            usageDerivedPayload(row),
		CreatedAt:             row.observedAt.UTC(),
	}
}

func qualitySnapshotFromUsageDerivedRow(row usageDerivedRow, id int64) *adminplusdomain.SupplyQualitySnapshot {
	totalAttempts := row.requestCount + row.errorCount
	availabilityRatio := 1.0
	errorRatio := 0.0
	if totalAttempts > 0 {
		availabilityRatio = float64(row.requestCount) / float64(totalAttempts)
		errorRatio = float64(row.errorCount) / float64(totalAttempts)
	}
	if row.degradedAccountCount > 0 && availabilityRatio > 0.85 {
		availabilityRatio = 0.85
	}
	cacheRatio := cacheHitRatioFromUsageRow(row)
	purityScore := 0.0
	if row.modelComparableCount > 0 {
		purityScore = (1 - float64(row.modelMismatchCount)/float64(row.modelComparableCount)) * 100
	}
	usageTrustScore := 90.0
	if totalAttempts > 0 {
		usageTrustScore = (1 - errorRatio) * 100
	}
	balanceRiskScore := balanceRiskScoreFromUsageRow(row)
	concurrencyScore := concurrencyScoreFromUsageRow(row)
	input := SupplyQualityInput{
		SupplyType:            row.supplyType,
		SupplierID:            row.supplierID,
		Model:                 row.model,
		AvailabilityRatio:     availabilityRatio,
		ErrorRatio:            errorRatio,
		AvgTTFTMS:             nullInt64Ptr(row.avgTTFTMS),
		AvgTotalLatencyMS:     nullInt64Ptr(row.avgTotalLatencyMS),
		CacheHitRatio:         cacheRatio,
		PurityScore:           purityScore,
		UsageTrustScore:       usageTrustScore,
		BalanceRiskScore:      balanceRiskScore,
		ConcurrencyScore:      concurrencyScore,
		LocalSub2APIAccountID: row.accountID,
	}
	qualityScore := deriveQualityScore(input)
	decision := decisionFromQualityScore(qualityScore, input)
	return &adminplusdomain.SupplyQualitySnapshot{
		ID:                    id,
		SupplyType:            row.supplyType,
		SupplierID:            row.supplierID,
		LocalSub2APIAccountID: row.accountID,
		Model:                 row.model,
		AvailabilityRatio:     availabilityRatio,
		ErrorRatio:            errorRatio,
		AvgTTFTMS:             nullInt64Ptr(row.avgTTFTMS),
		AvgTotalLatencyMS:     nullInt64Ptr(row.avgTotalLatencyMS),
		CacheHitRatio:         cacheRatio,
		PurityScore:           purityScore,
		UsageTrustScore:       usageTrustScore,
		BalanceRiskScore:      balanceRiskScore,
		ConcurrencyScore:      concurrencyScore,
		QualityScore:          qualityScore,
		Decision:              decision,
		Notes:                 usageDerivedNotes(row),
		ObservedAt:            row.observedAt.UTC(),
		RawPayload:            usageDerivedPayload(row),
		CreatedAt:             row.observedAt.UTC(),
	}
}

func cacheHitRatioFromUsageRow(row usageDerivedRow) float64 {
	denominator := row.cacheReadTokens + row.cacheWriteTokens + row.inputTokens
	if denominator <= 0 {
		return 0
	}
	return float64(row.cacheReadTokens) / float64(denominator)
}

func balanceRiskScoreFromUsageRow(row usageDerivedRow) float64 {
	if row.supplyType == "own_pool" {
		if row.degradedAccountCount > 0 {
			return 70
		}
		return 10
	}
	if row.usableBalanceCount == 0 {
		return 90
	}
	if row.balanceThresholdCents > 0 && row.balanceCents <= row.balanceThresholdCents {
		return 70
	}
	return 15
}

func concurrencyScoreFromUsageRow(row usageDerivedRow) float64 {
	if row.configuredConcurrency <= 0 {
		return 60
	}
	if row.observedMaxConcurrency > 0 && row.observedMaxConcurrency >= row.configuredConcurrency {
		return 60
	}
	if row.accountCount > 1 {
		return 80
	}
	return 90
}

func usageDerivedNotes(row usageDerivedRow) string {
	return fmt.Sprintf("由近 7 天真实 usage/错误日志自动派生；账号数 %d，请求 %d，错误 %d。", row.accountCount, row.requestCount, row.errorCount)
}

func usageDerivedPayload(row usageDerivedRow) map[string]any {
	return map[string]any{
		"source":                   "usage_logs",
		"error_source":             "ops_error_logs",
		"derived":                  true,
		"request_count":            row.requestCount,
		"account_count":            row.accountCount,
		"error_count":              row.errorCount,
		"model_comparable_count":   row.modelComparableCount,
		"model_mismatch_count":     row.modelMismatchCount,
		"degraded_account_count":   row.degradedAccountCount,
		"usable_balance_count":     row.usableBalanceCount,
		"balance_cents":            row.balanceCents,
		"balance_threshold_cents":  row.balanceThresholdCents,
		"configured_concurrency":   row.configuredConcurrency,
		"observed_max_concurrency": row.observedMaxConcurrency,
	}
}

func nullInt64Ptr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	v := value.Int64
	return &v
}

func clampInt64ToInt(value int64) int {
	if value <= 0 {
		return 0
	}
	if value > int64(^uint(0)>>1) {
		return int(^uint(0) >> 1)
	}
	return int(value)
}
