package domain

import "time"

type MarketPriceSnapshot struct {
	ID                int64          `json:"id"`
	SourceType        string         `json:"source_type"`
	SourceName        string         `json:"source_name"`
	SourceURL         string         `json:"source_url,omitempty"`
	SiteID            int64          `json:"site_id,omitempty"`
	SupplierID        int64          `json:"supplier_id,omitempty"`
	Model             string         `json:"model"`
	BillingMode       string         `json:"billing_mode"`
	PriceItem         string         `json:"price_item"`
	Unit              string         `json:"unit"`
	Currency          string         `json:"currency"`
	PriceMicros       int64          `json:"price_micros"`
	PackageLabel      string         `json:"package_label,omitempty"`
	PackagePriceCents *int64         `json:"package_price_cents,omitempty"`
	PackageQuota      string         `json:"package_quota,omitempty"`
	RateMultiplier    *float64       `json:"rate_multiplier,omitempty"`
	MinRechargeCents  *int64         `json:"min_recharge_cents,omitempty"`
	BonusPercent      *float64       `json:"bonus_percent,omitempty"`
	Confidence        float64        `json:"confidence"`
	ObservedAt        time.Time      `json:"observed_at"`
	RawPayload        map[string]any `json:"raw_payload,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
}

type CacheEfficiencySnapshot struct {
	ID                    int64          `json:"id"`
	SupplyType            string         `json:"supply_type"`
	SupplierID            int64          `json:"supplier_id,omitempty"`
	LocalSub2APIAccountID int64          `json:"local_sub2api_account_id,omitempty"`
	Model                 string         `json:"model"`
	RoutingStrategy       string         `json:"routing_strategy"`
	StickyScope           string         `json:"sticky_scope"`
	SampleRequests        int            `json:"sample_requests"`
	CacheReadTokens       int64          `json:"cache_read_tokens"`
	CacheWriteTokens      int64          `json:"cache_write_tokens"`
	InputTokens           int64          `json:"input_tokens"`
	OutputTokens          int64          `json:"output_tokens"`
	CacheHitRatio         float64        `json:"cache_hit_ratio"`
	DuplicateInputTokens  int64          `json:"duplicate_input_tokens"`
	EstimatedWasteCents   int64          `json:"estimated_waste_cents"`
	AvgTTFTMS             *int64         `json:"avg_ttft_ms,omitempty"`
	AvgTotalLatencyMS     *int64         `json:"avg_total_latency_ms,omitempty"`
	Status                string         `json:"status"`
	Notes                 string         `json:"notes,omitempty"`
	ObservedAt            time.Time      `json:"observed_at"`
	RawPayload            map[string]any `json:"raw_payload,omitempty"`
	CreatedAt             time.Time      `json:"created_at"`
}

type SupplyQualitySnapshot struct {
	ID                    int64          `json:"id"`
	SupplyType            string         `json:"supply_type"`
	SupplierID            int64          `json:"supplier_id,omitempty"`
	LocalSub2APIAccountID int64          `json:"local_sub2api_account_id,omitempty"`
	Model                 string         `json:"model,omitempty"`
	AvailabilityRatio     float64        `json:"availability_ratio"`
	ErrorRatio            float64        `json:"error_ratio"`
	AvgTTFTMS             *int64         `json:"avg_ttft_ms,omitempty"`
	AvgTotalLatencyMS     *int64         `json:"avg_total_latency_ms,omitempty"`
	CacheHitRatio         float64        `json:"cache_hit_ratio"`
	PurityScore           float64        `json:"purity_score"`
	UsageTrustScore       float64        `json:"usage_trust_score"`
	BalanceRiskScore      float64        `json:"balance_risk_score"`
	ConcurrencyScore      float64        `json:"concurrency_score"`
	QualityScore          float64        `json:"quality_score"`
	Decision              string         `json:"decision"`
	Notes                 string         `json:"notes,omitempty"`
	ObservedAt            time.Time      `json:"observed_at"`
	RawPayload            map[string]any `json:"raw_payload,omitempty"`
	CreatedAt             time.Time      `json:"created_at"`
}

type AcceptanceReport struct {
	ID                    int64          `json:"id"`
	SupplyType            string         `json:"supply_type"`
	SupplierID            int64          `json:"supplier_id,omitempty"`
	LocalSub2APIAccountID int64          `json:"local_sub2api_account_id,omitempty"`
	Model                 string         `json:"model,omitempty"`
	Status                string         `json:"status"`
	ConnectivityStatus    string         `json:"connectivity_status"`
	ModelListStatus       string         `json:"model_list_status"`
	PurityStatus          string         `json:"purity_status"`
	TrialCallStatus       string         `json:"trial_call_status"`
	UsageMeteringStatus   string         `json:"usage_metering_status"`
	CacheAuditStatus      string         `json:"cache_audit_status"`
	BalanceStatus         string         `json:"balance_status"`
	ConcurrencyStatus     string         `json:"concurrency_status"`
	FailureReason         string         `json:"failure_reason,omitempty"`
	Recommendation        string         `json:"recommendation,omitempty"`
	ReportPayload         map[string]any `json:"report_payload,omitempty"`
	ObservedAt            time.Time      `json:"observed_at"`
	CreatedAt             time.Time      `json:"created_at"`
}

type KanbanModelMarginRow struct {
	Model                    string     `json:"model"`
	Currency                 string     `json:"currency"`
	MarketLowPriceMicros     *int64     `json:"market_low_price_micros,omitempty"`
	MarketMedianPriceMicros  *int64     `json:"market_median_price_micros,omitempty"`
	MarketHighPriceMicros    *int64     `json:"market_high_price_micros,omitempty"`
	MarketSampleCount        int        `json:"market_sample_count"`
	BestSupplierCostMicros   *int64     `json:"best_supplier_cost_micros,omitempty"`
	CacheAdjustedCostMicros  *int64     `json:"cache_adjusted_cost_micros,omitempty"`
	CacheHitRatio            *float64   `json:"cache_hit_ratio,omitempty"`
	CacheStatus              string     `json:"cache_status,omitempty"`
	QualityScore             *float64   `json:"quality_score,omitempty"`
	QualityDecision          string     `json:"quality_decision,omitempty"`
	AcceptanceStatus         string     `json:"acceptance_status,omitempty"`
	SuggestedPriceMicros     *int64     `json:"suggested_price_micros,omitempty"`
	GrossMarginPercent       *float64   `json:"gross_margin_percent,omitempty"`
	RequiredMarginPercent    float64    `json:"required_margin_percent"`
	MarginGapPercent         *float64   `json:"margin_gap_percent,omitempty"`
	SuggestedVsMarketPercent *float64   `json:"suggested_vs_market_percent,omitempty"`
	RiskLevel                string     `json:"risk_level"`
	Recommendation           string     `json:"recommendation"`
	LatestMarketObservedAt   *time.Time `json:"latest_market_observed_at,omitempty"`
	LatestCacheObservedAt    *time.Time `json:"latest_cache_observed_at,omitempty"`
	LatestSupplierCapturedAt *time.Time `json:"latest_supplier_captured_at,omitempty"`
}

type KanbanEvent struct {
	ID                  int64          `json:"id"`
	EventType           string         `json:"event_type"`
	Severity            string         `json:"severity"`
	Status              string         `json:"status"`
	Model               string         `json:"model"`
	SourceType          string         `json:"source_type,omitempty"`
	SourceID            int64          `json:"source_id,omitempty"`
	RelatedSnapshotType string         `json:"related_snapshot_type,omitempty"`
	RelatedSnapshotID   int64          `json:"related_snapshot_id,omitempty"`
	Title               string         `json:"title"`
	Description         string         `json:"description,omitempty"`
	Recommendation      string         `json:"recommendation,omitempty"`
	Payload             map[string]any `json:"payload,omitempty"`
	OccurredAt          time.Time      `json:"occurred_at"`
	CreatedAt           time.Time      `json:"created_at"`
}

type KanbanOverview struct {
	GeneratedAt             time.Time                  `json:"generated_at"`
	MarketSnapshotCount     int                        `json:"market_snapshot_count"`
	CacheSnapshotCount      int                        `json:"cache_snapshot_count"`
	QualitySnapshotCount    int                        `json:"quality_snapshot_count"`
	AcceptanceReportCount   int                        `json:"acceptance_report_count"`
	OpenEventCount          int                        `json:"open_event_count"`
	CriticalEventCount      int                        `json:"critical_event_count"`
	ModelCount              int                        `json:"model_count"`
	RiskyCacheModelCount    int                        `json:"risky_cache_model_count"`
	RiskyQualityModelCount  int                        `json:"risky_quality_model_count"`
	BlockedAcceptanceCount  int                        `json:"blocked_acceptance_count"`
	UnprofitableModelCount  int                        `json:"unprofitable_model_count"`
	ModelMargins            []KanbanModelMarginRow     `json:"model_margins"`
	RecentEvents            []*KanbanEvent             `json:"recent_events"`
	RecentMarketSnapshots   []*MarketPriceSnapshot     `json:"recent_market_snapshots"`
	RecentCacheSnapshots    []*CacheEfficiencySnapshot `json:"recent_cache_snapshots"`
	RecentQualitySnapshots  []*SupplyQualitySnapshot   `json:"recent_quality_snapshots"`
	RecentAcceptanceReports []*AcceptanceReport        `json:"recent_acceptance_reports"`
	AcceptanceStepSummaries []AcceptanceStepSummary    `json:"acceptance_step_summaries"`
}

type AcceptanceStepSummary struct {
	Step         string `json:"step"`
	TotalCount   int    `json:"total_count"`
	PassCount    int    `json:"pass_count"`
	WarnCount    int    `json:"warn_count"`
	FailCount    int    `json:"fail_count"`
	UnknownCount int    `json:"unknown_count"`
	RiskLevel    string `json:"risk_level"`
}
