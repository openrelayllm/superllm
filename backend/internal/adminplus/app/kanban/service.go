package kanban

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

const (
	defaultListLimit            = 100
	maxListLimit                = 1000
	defaultTargetMarginPercent  = 25.0
	defaultRiskBufferPercent    = 8.0
	defaultPriceMovePercent     = 10.0
	defaultAnomalyDiscountRatio = 0.75
	defaultCacheRiskHitRatio    = 0.25
	defaultWatchingHitRatio     = 0.45
	defaultSuggestedPriceMicros = int64(0)
	defaultUsageDerivedWindow   = 7 * 24 * time.Hour
)

type MarketPriceInput struct {
	SourceType        string
	SourceName        string
	SourceURL         string
	SiteID            int64
	SupplierID        int64
	Model             string
	BillingMode       string
	PriceItem         string
	Unit              string
	Currency          string
	PriceMicros       int64
	PackageLabel      string
	PackagePriceCents *int64
	PackageQuota      string
	RateMultiplier    *float64
	MinRechargeCents  *int64
	BonusPercent      *float64
	Confidence        float64
	ObservedAt        *time.Time
	RawPayload        map[string]any
}

type CacheEfficiencyInput struct {
	SupplyType            string
	SupplierID            int64
	LocalSub2APIAccountID int64
	Model                 string
	RoutingStrategy       string
	StickyScope           string
	SampleRequests        int
	CacheReadTokens       int64
	CacheWriteTokens      int64
	InputTokens           int64
	OutputTokens          int64
	CacheHitRatio         *float64
	DuplicateInputTokens  int64
	EstimatedWasteCents   int64
	AvgTTFTMS             *int64
	AvgTotalLatencyMS     *int64
	Status                string
	Notes                 string
	ObservedAt            *time.Time
	RawPayload            map[string]any
}

type SupplyQualityInput struct {
	SupplyType            string
	SupplierID            int64
	LocalSub2APIAccountID int64
	Model                 string
	AvailabilityRatio     float64
	ErrorRatio            float64
	AvgTTFTMS             *int64
	AvgTotalLatencyMS     *int64
	CacheHitRatio         float64
	PurityScore           float64
	UsageTrustScore       float64
	BalanceRiskScore      float64
	ConcurrencyScore      float64
	QualityScore          float64
	Decision              string
	Notes                 string
	ObservedAt            *time.Time
	RawPayload            map[string]any
}

type AcceptanceReportInput struct {
	SupplyType            string
	SupplierID            int64
	LocalSub2APIAccountID int64
	Model                 string
	Status                string
	ConnectivityStatus    string
	ModelListStatus       string
	PurityStatus          string
	TrialCallStatus       string
	UsageMeteringStatus   string
	CacheAuditStatus      string
	BalanceStatus         string
	ConcurrencyStatus     string
	FailureReason         string
	Recommendation        string
	ReportPayload         map[string]any
	ObservedAt            *time.Time
}

type MarketPriceFilter struct {
	Model      string
	SourceType string
	SiteID     int64
	SupplierID int64
	Limit      int
}

type CacheEfficiencyFilter struct {
	Model                 string
	SupplyType            string
	SupplierID            int64
	LocalSub2APIAccountID int64
	Status                string
	Limit                 int
}

type SupplyQualityFilter struct {
	Model                 string
	SupplyType            string
	SupplierID            int64
	LocalSub2APIAccountID int64
	Decision              string
	Limit                 int
}

type AcceptanceReportFilter struct {
	Model                 string
	SupplyType            string
	SupplierID            int64
	LocalSub2APIAccountID int64
	Status                string
	Limit                 int
}

type KanbanEventFilter struct {
	Model     string
	EventType string
	Severity  string
	Status    string
	Limit     int
}

type OverviewFilter struct {
	Model               string
	TargetMarginPercent float64
	RiskBufferPercent   float64
	Limit               int
}

type UsageDerivedFilter struct {
	Model                 string
	SupplyType            string
	SupplierID            int64
	LocalSub2APIAccountID int64
	Since                 time.Time
	Until                 time.Time
	Limit                 int
}

type UsageDerivedSnapshots struct {
	Cache   []*adminplusdomain.CacheEfficiencySnapshot
	Quality []*adminplusdomain.SupplyQualitySnapshot
}

type Repository interface {
	CreateMarketPriceSnapshot(ctx context.Context, snapshot *adminplusdomain.MarketPriceSnapshot) (*adminplusdomain.MarketPriceSnapshot, error)
	ListMarketPriceSnapshots(ctx context.Context, filter MarketPriceFilter) ([]*adminplusdomain.MarketPriceSnapshot, error)
	CreateCacheEfficiencySnapshot(ctx context.Context, snapshot *adminplusdomain.CacheEfficiencySnapshot) (*adminplusdomain.CacheEfficiencySnapshot, error)
	ListCacheEfficiencySnapshots(ctx context.Context, filter CacheEfficiencyFilter) ([]*adminplusdomain.CacheEfficiencySnapshot, error)
	CreateSupplyQualitySnapshot(ctx context.Context, snapshot *adminplusdomain.SupplyQualitySnapshot) (*adminplusdomain.SupplyQualitySnapshot, error)
	ListSupplyQualitySnapshots(ctx context.Context, filter SupplyQualityFilter) ([]*adminplusdomain.SupplyQualitySnapshot, error)
	CreateAcceptanceReport(ctx context.Context, report *adminplusdomain.AcceptanceReport) (*adminplusdomain.AcceptanceReport, error)
	ListAcceptanceReports(ctx context.Context, filter AcceptanceReportFilter) ([]*adminplusdomain.AcceptanceReport, error)
	CreateKanbanEvent(ctx context.Context, event *adminplusdomain.KanbanEvent) (*adminplusdomain.KanbanEvent, error)
	ListKanbanEvents(ctx context.Context, filter KanbanEventFilter) ([]*adminplusdomain.KanbanEvent, error)
	UpdateKanbanEventStatus(ctx context.Context, id int64, status string) (*adminplusdomain.KanbanEvent, error)
	ListSupplierRateCosts(ctx context.Context, model string, limit int) ([]*SupplierRateCost, error)
	ListUsageDerivedSnapshots(ctx context.Context, filter UsageDerivedFilter) (*UsageDerivedSnapshots, error)
}

type SupplierRateCost struct {
	SupplierID  int64
	Model       string
	Currency    string
	PriceMicros int64
	CapturedAt  time.Time
}

type Service struct {
	repo              Repository
	siteCatalog       SiteCatalogReader
	evidenceScheduler AcceptanceEvidenceScheduler
	evidenceRunReader AcceptanceEvidenceRunReader
	marketPriceClient *http.Client
	now               func() time.Time
}

func NewService(repo Repository) *Service {
	return NewServiceWithDependencies(repo, nil)
}

func NewServiceWithDependencies(repo Repository, siteCatalog SiteCatalogReader) *Service {
	return NewServiceWithAllDependencies(repo, siteCatalog, nil)
}

func NewServiceWithAllDependencies(repo Repository, siteCatalog SiteCatalogReader, evidenceScheduler AcceptanceEvidenceScheduler) *Service {
	service := &Service{repo: repo, siteCatalog: siteCatalog, evidenceScheduler: evidenceScheduler, now: time.Now}
	if reader, ok := evidenceScheduler.(AcceptanceEvidenceRunReader); ok {
		service.evidenceRunReader = reader
	}
	if setter, ok := evidenceScheduler.(acceptanceEvidenceRunObserverSetter); ok {
		setter.WithRunStatusObserver(service)
	}
	return service
}

func (s *Service) WithMarketPriceHTTPClient(client *http.Client) *Service {
	if s != nil {
		s.marketPriceClient = client
	}
	return s
}

func (s *Service) RecordMarketPrice(ctx context.Context, in MarketPriceInput) (*adminplusdomain.MarketPriceSnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("kanban service is not configured")
	}
	snapshot, err := s.marketPriceFromInput(in)
	if err != nil {
		return nil, err
	}
	created, err := s.repo.CreateMarketPriceSnapshot(ctx, snapshot)
	if err != nil {
		return nil, err
	}
	s.recordMarketPriceEvents(ctx, created)
	return created, nil
}

func (s *Service) ListMarketPrices(ctx context.Context, filter MarketPriceFilter) ([]*adminplusdomain.MarketPriceSnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("kanban service is not configured")
	}
	filter.Model = strings.TrimSpace(filter.Model)
	filter.SourceType = normalizeSourceTypeFilter(filter.SourceType)
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListMarketPriceSnapshots(ctx, filter)
}

func (s *Service) RecordCacheEfficiency(ctx context.Context, in CacheEfficiencyInput) (*adminplusdomain.CacheEfficiencySnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("kanban service is not configured")
	}
	snapshot, err := s.cacheEfficiencyFromInput(in)
	if err != nil {
		return nil, err
	}
	created, err := s.repo.CreateCacheEfficiencySnapshot(ctx, snapshot)
	if err != nil {
		return nil, err
	}
	s.recordCacheEfficiencyEvents(ctx, created)
	return created, nil
}

func (s *Service) ListCacheEfficiency(ctx context.Context, filter CacheEfficiencyFilter) ([]*adminplusdomain.CacheEfficiencySnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("kanban service is not configured")
	}
	filter.Model = strings.TrimSpace(filter.Model)
	filter.SupplyType = normalizeSupplyTypeFilter(filter.SupplyType)
	filter.Status = normalizeCacheStatus(filter.Status)
	filter.Limit = normalizeLimit(filter.Limit)
	items, err := s.repo.ListCacheEfficiencySnapshots(ctx, filter)
	if err != nil {
		return nil, err
	}
	derived, err := s.usageDerivedSnapshots(ctx, usageDerivedFilterFromCache(filter))
	if err != nil {
		return nil, err
	}
	if derived != nil {
		for _, item := range derived.Cache {
			if item == nil {
				continue
			}
			if filter.Status != "" && item.Status != filter.Status {
				continue
			}
			items = append(items, item)
		}
	}
	sortCacheSnapshots(items)
	return firstCacheSnapshots(items, filter.Limit), nil
}

func (s *Service) RecordSupplyQuality(ctx context.Context, in SupplyQualityInput) (*adminplusdomain.SupplyQualitySnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("kanban service is not configured")
	}
	snapshot, err := s.supplyQualityFromInput(in)
	if err != nil {
		return nil, err
	}
	created, err := s.repo.CreateSupplyQualitySnapshot(ctx, snapshot)
	if err != nil {
		return nil, err
	}
	s.recordSupplyQualityEvents(ctx, created)
	return created, nil
}

func (s *Service) ListSupplyQuality(ctx context.Context, filter SupplyQualityFilter) ([]*adminplusdomain.SupplyQualitySnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("kanban service is not configured")
	}
	filter.Model = strings.TrimSpace(filter.Model)
	filter.SupplyType = normalizeSupplyTypeFilter(filter.SupplyType)
	filter.Decision = normalizeQualityDecision(filter.Decision)
	filter.Limit = normalizeLimit(filter.Limit)
	items, err := s.repo.ListSupplyQualitySnapshots(ctx, filter)
	if err != nil {
		return nil, err
	}
	derived, err := s.usageDerivedSnapshots(ctx, usageDerivedFilterFromQuality(filter))
	if err != nil {
		return nil, err
	}
	if derived != nil {
		for _, item := range derived.Quality {
			if item == nil {
				continue
			}
			if filter.Decision != "" && item.Decision != filter.Decision {
				continue
			}
			items = append(items, item)
		}
	}
	sortQualitySnapshots(items)
	return firstQualitySnapshots(items, filter.Limit), nil
}

func (s *Service) RecordAcceptanceReport(ctx context.Context, in AcceptanceReportInput) (*adminplusdomain.AcceptanceReport, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("kanban service is not configured")
	}
	report, err := s.acceptanceReportFromInput(in)
	if err != nil {
		return nil, err
	}
	created, err := s.repo.CreateAcceptanceReport(ctx, report)
	if err != nil {
		return nil, err
	}
	s.recordAcceptanceEvents(ctx, created)
	return created, nil
}

func (s *Service) ListAcceptanceReports(ctx context.Context, filter AcceptanceReportFilter) ([]*adminplusdomain.AcceptanceReport, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("kanban service is not configured")
	}
	filter.Model = strings.TrimSpace(filter.Model)
	filter.SupplyType = normalizeSupplyTypeFilter(filter.SupplyType)
	filter.Status = normalizeQualityDecision(filter.Status)
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListAcceptanceReports(ctx, filter)
}

func (s *Service) ListEvents(ctx context.Context, filter KanbanEventFilter) ([]*adminplusdomain.KanbanEvent, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("kanban service is not configured")
	}
	filter.Model = strings.TrimSpace(filter.Model)
	filter.EventType = normalizeEventType(filter.EventType)
	filter.Severity = normalizeEventSeverity(filter.Severity)
	filter.Status = normalizeEventStatus(filter.Status)
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListKanbanEvents(ctx, filter)
}

func (s *Service) UpdateEventStatus(ctx context.Context, id int64, status string) (*adminplusdomain.KanbanEvent, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("kanban service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("KANBAN_EVENT_ID_INVALID", "event id is required")
	}
	normalized := normalizeEventStatus(status)
	if normalized == "" {
		return nil, badRequest("KANBAN_EVENT_STATUS_INVALID", "event status is invalid")
	}
	return s.repo.UpdateKanbanEventStatus(ctx, id, normalized)
}

func (s *Service) Overview(ctx context.Context, filter OverviewFilter) (*adminplusdomain.KanbanOverview, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("kanban service is not configured")
	}
	limit := normalizeLimit(filter.Limit)
	if limit < 200 {
		limit = 200
	}
	market, err := s.repo.ListMarketPriceSnapshots(ctx, MarketPriceFilter{Model: strings.TrimSpace(filter.Model), Limit: limit})
	if err != nil {
		return nil, err
	}
	cache, err := s.ListCacheEfficiency(ctx, CacheEfficiencyFilter{Model: strings.TrimSpace(filter.Model), Limit: limit})
	if err != nil {
		return nil, err
	}
	quality, err := s.ListSupplyQuality(ctx, SupplyQualityFilter{Model: strings.TrimSpace(filter.Model), Limit: limit})
	if err != nil {
		return nil, err
	}
	acceptance, err := s.repo.ListAcceptanceReports(ctx, AcceptanceReportFilter{Model: strings.TrimSpace(filter.Model), Limit: limit})
	if err != nil {
		return nil, err
	}
	costs, err := s.repo.ListSupplierRateCosts(ctx, strings.TrimSpace(filter.Model), limit)
	if err != nil {
		return nil, err
	}
	events, err := s.repo.ListKanbanEvents(ctx, KanbanEventFilter{Model: strings.TrimSpace(filter.Model), Limit: maxListLimit})
	if err != nil {
		return nil, err
	}
	rows := buildModelMargins(market, cache, quality, acceptance, costs, filter)
	return &adminplusdomain.KanbanOverview{
		GeneratedAt:             s.now().UTC(),
		MarketSnapshotCount:     len(market),
		CacheSnapshotCount:      len(cache),
		QualitySnapshotCount:    len(quality),
		AcceptanceReportCount:   len(acceptance),
		OpenEventCount:          countOpenEvents(events),
		CriticalEventCount:      countCriticalEvents(events),
		ModelCount:              len(rows),
		RiskyCacheModelCount:    countRiskyCacheRows(rows),
		RiskyQualityModelCount:  countRiskyQualityRows(rows),
		BlockedAcceptanceCount:  countBlockedAcceptanceReports(acceptance),
		UnprofitableModelCount:  countUnprofitableRows(rows),
		ModelMargins:            rows,
		RecentEvents:            firstKanbanEvents(events, 10),
		RecentMarketSnapshots:   firstMarketSnapshots(market, 10),
		RecentCacheSnapshots:    firstCacheSnapshots(cache, 10),
		RecentQualitySnapshots:  firstQualitySnapshots(quality, 10),
		RecentAcceptanceReports: firstAcceptanceReports(acceptance, 10),
		AcceptanceStepSummaries: buildAcceptanceStepSummaries(acceptance),
	}, nil
}

func (s *Service) marketPriceFromInput(in MarketPriceInput) (*adminplusdomain.MarketPriceSnapshot, error) {
	model := strings.TrimSpace(in.Model)
	if model == "" {
		return nil, badRequest("KANBAN_MODEL_REQUIRED", "model is required")
	}
	if in.PriceMicros < 0 {
		return nil, badRequest("KANBAN_PRICE_INVALID", "price_micros must be non-negative")
	}
	confidence := in.Confidence
	if confidence <= 0 {
		confidence = 1
	}
	if confidence > 1 {
		return nil, badRequest("KANBAN_CONFIDENCE_INVALID", "confidence must be between 0 and 1")
	}
	observedAt := s.now().UTC()
	if in.ObservedAt != nil {
		observedAt = in.ObservedAt.UTC()
	}
	return &adminplusdomain.MarketPriceSnapshot{
		SourceType:        normalizeSourceType(in.SourceType),
		SourceName:        trimLimit(in.SourceName, 160),
		SourceURL:         strings.TrimSpace(in.SourceURL),
		SiteID:            positiveID(in.SiteID),
		SupplierID:        positiveID(in.SupplierID),
		Model:             model,
		BillingMode:       defaultLower(in.BillingMode, "tokens"),
		PriceItem:         defaultLower(in.PriceItem, "blended"),
		Unit:              defaultLower(in.Unit, "1m_tokens"),
		Currency:          normalizeCurrency(in.Currency),
		PriceMicros:       in.PriceMicros,
		PackageLabel:      trimLimit(in.PackageLabel, 160),
		PackagePriceCents: nonNegativePtr(in.PackagePriceCents),
		PackageQuota:      trimLimit(in.PackageQuota, 160),
		RateMultiplier:    nonNegativeFloatPtr(in.RateMultiplier),
		MinRechargeCents:  nonNegativePtr(in.MinRechargeCents),
		BonusPercent:      nonNegativeFloatPtr(in.BonusPercent),
		Confidence:        confidence,
		ObservedAt:        observedAt,
		RawPayload:        in.RawPayload,
	}, nil
}

func (s *Service) cacheEfficiencyFromInput(in CacheEfficiencyInput) (*adminplusdomain.CacheEfficiencySnapshot, error) {
	model := strings.TrimSpace(in.Model)
	if model == "" {
		return nil, badRequest("KANBAN_MODEL_REQUIRED", "model is required")
	}
	if in.SampleRequests < 0 || in.CacheReadTokens < 0 || in.CacheWriteTokens < 0 || in.InputTokens < 0 || in.OutputTokens < 0 || in.DuplicateInputTokens < 0 || in.EstimatedWasteCents < 0 {
		return nil, badRequest("KANBAN_CACHE_METRIC_INVALID", "cache metrics must be non-negative")
	}
	hitRatio := deriveCacheHitRatio(in)
	if hitRatio < 0 || hitRatio > 1 {
		return nil, badRequest("KANBAN_CACHE_HIT_RATIO_INVALID", "cache_hit_ratio must be between 0 and 1")
	}
	observedAt := s.now().UTC()
	if in.ObservedAt != nil {
		observedAt = in.ObservedAt.UTC()
	}
	status := normalizeCacheStatus(in.Status)
	if status == "" {
		status = statusFromCacheHitRatio(hitRatio)
	}
	return &adminplusdomain.CacheEfficiencySnapshot{
		SupplyType:            normalizeSupplyType(in.SupplyType),
		SupplierID:            positiveID(in.SupplierID),
		LocalSub2APIAccountID: positiveID(in.LocalSub2APIAccountID),
		Model:                 model,
		RoutingStrategy:       normalizeRoutingStrategy(in.RoutingStrategy),
		StickyScope:           normalizeStickyScope(in.StickyScope),
		SampleRequests:        in.SampleRequests,
		CacheReadTokens:       in.CacheReadTokens,
		CacheWriteTokens:      in.CacheWriteTokens,
		InputTokens:           in.InputTokens,
		OutputTokens:          in.OutputTokens,
		CacheHitRatio:         hitRatio,
		DuplicateInputTokens:  in.DuplicateInputTokens,
		EstimatedWasteCents:   in.EstimatedWasteCents,
		AvgTTFTMS:             nonNegativePtr(in.AvgTTFTMS),
		AvgTotalLatencyMS:     nonNegativePtr(in.AvgTotalLatencyMS),
		Status:                status,
		Notes:                 strings.TrimSpace(in.Notes),
		ObservedAt:            observedAt,
		RawPayload:            in.RawPayload,
	}, nil
}

func (s *Service) supplyQualityFromInput(in SupplyQualityInput) (*adminplusdomain.SupplyQualitySnapshot, error) {
	if !validRatio(in.AvailabilityRatio) || !validRatio(in.ErrorRatio) || !validRatio(in.CacheHitRatio) {
		return nil, badRequest("KANBAN_QUALITY_RATIO_INVALID", "quality ratios must be between 0 and 1")
	}
	if !validScore(in.PurityScore) || !validScore(in.UsageTrustScore) || !validScore(in.BalanceRiskScore) || !validScore(in.ConcurrencyScore) || !validScore(in.QualityScore) {
		return nil, badRequest("KANBAN_QUALITY_SCORE_INVALID", "quality scores must be between 0 and 100")
	}
	observedAt := s.now().UTC()
	if in.ObservedAt != nil {
		observedAt = in.ObservedAt.UTC()
	}
	qualityScore := in.QualityScore
	if qualityScore <= 0 {
		qualityScore = deriveQualityScore(in)
	}
	decision := normalizeQualityDecision(in.Decision)
	if decision == "" {
		decision = decisionFromQualityScore(qualityScore, in)
	}
	return &adminplusdomain.SupplyQualitySnapshot{
		SupplyType:            normalizeSupplyType(in.SupplyType),
		SupplierID:            positiveID(in.SupplierID),
		LocalSub2APIAccountID: positiveID(in.LocalSub2APIAccountID),
		Model:                 strings.TrimSpace(in.Model),
		AvailabilityRatio:     in.AvailabilityRatio,
		ErrorRatio:            in.ErrorRatio,
		AvgTTFTMS:             nonNegativePtr(in.AvgTTFTMS),
		AvgTotalLatencyMS:     nonNegativePtr(in.AvgTotalLatencyMS),
		CacheHitRatio:         in.CacheHitRatio,
		PurityScore:           in.PurityScore,
		UsageTrustScore:       in.UsageTrustScore,
		BalanceRiskScore:      in.BalanceRiskScore,
		ConcurrencyScore:      in.ConcurrencyScore,
		QualityScore:          qualityScore,
		Decision:              decision,
		Notes:                 strings.TrimSpace(in.Notes),
		ObservedAt:            observedAt,
		RawPayload:            in.RawPayload,
	}, nil
}

func (s *Service) acceptanceReportFromInput(in AcceptanceReportInput) (*adminplusdomain.AcceptanceReport, error) {
	observedAt := s.now().UTC()
	if in.ObservedAt != nil {
		observedAt = in.ObservedAt.UTC()
	}
	report := &adminplusdomain.AcceptanceReport{
		SupplyType:            normalizeSupplyType(in.SupplyType),
		SupplierID:            positiveID(in.SupplierID),
		LocalSub2APIAccountID: positiveID(in.LocalSub2APIAccountID),
		Model:                 strings.TrimSpace(in.Model),
		Status:                normalizeQualityDecision(in.Status),
		ConnectivityStatus:    normalizeCheckStatus(in.ConnectivityStatus),
		ModelListStatus:       normalizeCheckStatus(in.ModelListStatus),
		PurityStatus:          normalizeCheckStatus(in.PurityStatus),
		TrialCallStatus:       normalizeCheckStatus(in.TrialCallStatus),
		UsageMeteringStatus:   normalizeCheckStatus(in.UsageMeteringStatus),
		CacheAuditStatus:      normalizeCheckStatus(in.CacheAuditStatus),
		BalanceStatus:         normalizeCheckStatus(in.BalanceStatus),
		ConcurrencyStatus:     normalizeCheckStatus(in.ConcurrencyStatus),
		FailureReason:         strings.TrimSpace(in.FailureReason),
		Recommendation:        strings.TrimSpace(in.Recommendation),
		ReportPayload:         in.ReportPayload,
		ObservedAt:            observedAt,
	}
	if report.Status == "" {
		report.Status = decisionFromAcceptance(report)
	}
	if report.Recommendation == "" {
		report.Recommendation = recommendationFromAcceptance(report)
	}
	return report, nil
}

func (s *Service) recordMarketPriceEvents(ctx context.Context, snapshot *adminplusdomain.MarketPriceSnapshot) {
	if snapshot == nil {
		return
	}
	previous, err := s.repo.ListMarketPriceSnapshots(ctx, MarketPriceFilter{Model: snapshot.Model, SourceType: snapshot.SourceType, SupplierID: snapshot.SupplierID, SiteID: snapshot.SiteID, Limit: 5})
	if err != nil {
		return
	}
	var previousSnapshot *adminplusdomain.MarketPriceSnapshot
	for _, item := range previous {
		if item != nil && item.ID != snapshot.ID && item.PriceMicros > 0 && comparableMarketPriceSnapshot(item, snapshot) {
			previousSnapshot = item
			break
		}
	}
	if previousSnapshot != nil {
		s.recordPriceMovementEvent(ctx, snapshot, previousSnapshot)
	}
	modelMarket, err := s.repo.ListMarketPriceSnapshots(ctx, MarketPriceFilter{Model: snapshot.Model, Limit: 50})
	if err == nil {
		s.recordMarketAnomalyEvent(ctx, snapshot, modelMarket)
	}
}

func comparableMarketPriceSnapshot(a *adminplusdomain.MarketPriceSnapshot, b *adminplusdomain.MarketPriceSnapshot) bool {
	if a == nil || b == nil {
		return false
	}
	return a.BillingMode == b.BillingMode && a.PriceItem == b.PriceItem && a.Unit == b.Unit && a.Currency == b.Currency
}

func (s *Service) recordPriceMovementEvent(ctx context.Context, snapshot *adminplusdomain.MarketPriceSnapshot, previous *adminplusdomain.MarketPriceSnapshot) {
	if snapshot == nil || previous == nil || previous.PriceMicros <= 0 || snapshot.PriceMicros <= 0 {
		return
	}
	changePercent := (float64(snapshot.PriceMicros-previous.PriceMicros) / float64(previous.PriceMicros)) * 100
	if changePercent > -defaultPriceMovePercent && changePercent < defaultPriceMovePercent {
		return
	}
	eventType := "market_price_rise"
	severity := "info"
	title := "市场价格上涨"
	recommendation := "观察是否为单一来源调价，暂不自动调整线上售价。"
	if changePercent < 0 {
		eventType = "market_price_drop"
		severity = "warning"
		title = "市场价格下降"
		recommendation = "复核质量和缓存效率后再决定是否跟价。"
	}
	_, _ = s.repo.CreateKanbanEvent(ctx, &adminplusdomain.KanbanEvent{
		EventType:           eventType,
		Severity:            severity,
		Status:              "open",
		Model:               snapshot.Model,
		SourceType:          snapshot.SourceType,
		SourceID:            firstPositiveID(snapshot.SupplierID, snapshot.SiteID),
		RelatedSnapshotType: "market_price",
		RelatedSnapshotID:   snapshot.ID,
		Title:               title,
		Description:         strings.TrimSpace(snapshot.SourceName),
		Recommendation:      recommendation,
		OccurredAt:          snapshot.ObservedAt,
		Payload: map[string]any{
			"previous_snapshot_id":  previous.ID,
			"previous_price_micros": previous.PriceMicros,
			"current_price_micros":  snapshot.PriceMicros,
			"change_percent":        changePercent,
			"currency":              snapshot.Currency,
		},
	})
}

func (s *Service) recordMarketAnomalyEvent(ctx context.Context, snapshot *adminplusdomain.MarketPriceSnapshot, market []*adminplusdomain.MarketPriceSnapshot) {
	if snapshot == nil || snapshot.PriceMicros <= 0 || len(market) < 3 {
		return
	}
	prices := make([]int64, 0, len(market))
	for _, item := range market {
		if item != nil && item.PriceMicros > 0 && item.ID != snapshot.ID {
			prices = append(prices, item.PriceMicros)
		}
	}
	if len(prices) < 2 {
		return
	}
	sort.Slice(prices, func(i, j int) bool { return prices[i] < prices[j] })
	median := prices[len(prices)/2]
	if median <= 0 || float64(snapshot.PriceMicros) >= float64(median)*defaultAnomalyDiscountRatio {
		return
	}
	_, _ = s.repo.CreateKanbanEvent(ctx, &adminplusdomain.KanbanEvent{
		EventType:           "market_price_anomaly",
		Severity:            "warning",
		Status:              "open",
		Model:               snapshot.Model,
		SourceType:          snapshot.SourceType,
		SourceID:            firstPositiveID(snapshot.SupplierID, snapshot.SiteID),
		RelatedSnapshotType: "market_price",
		RelatedSnapshotID:   snapshot.ID,
		Title:               "异常低价",
		Description:         strings.TrimSpace(snapshot.SourceName),
		Recommendation:      "不要直接跟价；先检查模型纯度、限量套餐、充值门槛和缓存命中率。",
		OccurredAt:          snapshot.ObservedAt,
		Payload: map[string]any{
			"market_median_price_micros": median,
			"current_price_micros":       snapshot.PriceMicros,
			"currency":                   snapshot.Currency,
		},
	})
}

func (s *Service) recordCacheEfficiencyEvents(ctx context.Context, snapshot *adminplusdomain.CacheEfficiencySnapshot) {
	if snapshot == nil {
		return
	}
	if snapshot.Status != "bad" && snapshot.Status != "risky" && snapshot.CacheHitRatio >= defaultWatchingHitRatio {
		return
	}
	severity := "warning"
	if snapshot.Status == "bad" || snapshot.CacheHitRatio < defaultCacheRiskHitRatio {
		severity = "critical"
	}
	_, _ = s.repo.CreateKanbanEvent(ctx, &adminplusdomain.KanbanEvent{
		EventType:           "cache_efficiency_risk",
		Severity:            severity,
		Status:              "open",
		Model:               snapshot.Model,
		SourceType:          snapshot.SupplyType,
		SourceID:            firstPositiveID(snapshot.SupplierID, snapshot.LocalSub2APIAccountID),
		RelatedSnapshotType: "cache_efficiency",
		RelatedSnapshotID:   snapshot.ID,
		Title:               "缓存效率风险",
		Description:         snapshot.Notes,
		Recommendation:      "轮询号池需改为用户、项目或会话级 sticky routing；否则应把重复输入成本计入报价。",
		OccurredAt:          snapshot.ObservedAt,
		Payload: map[string]any{
			"cache_hit_ratio":        snapshot.CacheHitRatio,
			"routing_strategy":       snapshot.RoutingStrategy,
			"sticky_scope":           snapshot.StickyScope,
			"duplicate_input_tokens": snapshot.DuplicateInputTokens,
			"estimated_waste_cents":  snapshot.EstimatedWasteCents,
			"status":                 snapshot.Status,
		},
	})
}

func (s *Service) recordSupplyQualityEvents(ctx context.Context, snapshot *adminplusdomain.SupplyQualitySnapshot) {
	if snapshot == nil {
		return
	}
	if snapshot.Decision != "paused" && snapshot.Decision != "blocked" && snapshot.Decision != "low_priority" {
		return
	}
	severity := "warning"
	if snapshot.Decision == "blocked" || snapshot.QualityScore < 50 {
		severity = "critical"
	}
	_, _ = s.repo.CreateKanbanEvent(ctx, &adminplusdomain.KanbanEvent{
		EventType:           "supply_quality_risk",
		Severity:            severity,
		Status:              "open",
		Model:               snapshot.Model,
		SourceType:          snapshot.SupplyType,
		SourceID:            firstPositiveID(snapshot.SupplierID, snapshot.LocalSub2APIAccountID),
		RelatedSnapshotType: "supply_quality",
		RelatedSnapshotID:   snapshot.ID,
		Title:               "供应质量风险",
		Description:         snapshot.Notes,
		Recommendation:      "低质量供应源不得直接进入生产；需复核纯度、账单可信度、缓存效率和余额风险。",
		OccurredAt:          snapshot.ObservedAt,
		Payload: map[string]any{
			"quality_score":      snapshot.QualityScore,
			"decision":           snapshot.Decision,
			"availability_ratio": snapshot.AvailabilityRatio,
			"error_ratio":        snapshot.ErrorRatio,
			"cache_hit_ratio":    snapshot.CacheHitRatio,
			"purity_score":       snapshot.PurityScore,
			"usage_trust_score":  snapshot.UsageTrustScore,
			"balance_risk_score": snapshot.BalanceRiskScore,
			"concurrency_score":  snapshot.ConcurrencyScore,
		},
	})
}

func (s *Service) recordAcceptanceEvents(ctx context.Context, report *adminplusdomain.AcceptanceReport) {
	if report == nil {
		return
	}
	if report.Status != "blocked" && report.Status != "paused" && report.Status != "low_priority" {
		return
	}
	severity := "warning"
	if report.Status == "blocked" || hasFailedAcceptanceStep(report) {
		severity = "critical"
	}
	_, _ = s.repo.CreateKanbanEvent(ctx, &adminplusdomain.KanbanEvent{
		EventType:           "acceptance_risk",
		Severity:            severity,
		Status:              "open",
		Model:               report.Model,
		SourceType:          report.SupplyType,
		SourceID:            firstPositiveID(report.SupplierID, report.LocalSub2APIAccountID),
		RelatedSnapshotType: "acceptance_report",
		RelatedSnapshotID:   report.ID,
		Title:               "接入验收风险",
		Description:         report.FailureReason,
		Recommendation:      report.Recommendation,
		OccurredAt:          report.ObservedAt,
		Payload: map[string]any{
			"status":                report.Status,
			"connectivity_status":   report.ConnectivityStatus,
			"model_list_status":     report.ModelListStatus,
			"purity_status":         report.PurityStatus,
			"trial_call_status":     report.TrialCallStatus,
			"usage_metering_status": report.UsageMeteringStatus,
			"cache_audit_status":    report.CacheAuditStatus,
			"balance_status":        report.BalanceStatus,
			"concurrency_status":    report.ConcurrencyStatus,
		},
	})
}

func buildModelMargins(market []*adminplusdomain.MarketPriceSnapshot, cache []*adminplusdomain.CacheEfficiencySnapshot, quality []*adminplusdomain.SupplyQualitySnapshot, acceptance []*adminplusdomain.AcceptanceReport, costs []*SupplierRateCost, filter OverviewFilter) []adminplusdomain.KanbanModelMarginRow {
	marginPercent := filter.TargetMarginPercent
	if marginPercent <= 0 {
		marginPercent = defaultTargetMarginPercent
	}
	riskBufferPercent := filter.RiskBufferPercent
	if riskBufferPercent < 0 {
		riskBufferPercent = defaultRiskBufferPercent
	}
	marketByModel := map[string][]*adminplusdomain.MarketPriceSnapshot{}
	cacheByModel := map[string]*adminplusdomain.CacheEfficiencySnapshot{}
	qualityByModel := map[string]*adminplusdomain.SupplyQualitySnapshot{}
	acceptanceByModel := map[string]*adminplusdomain.AcceptanceReport{}
	costByModel := map[string]*SupplierRateCost{}
	modelSet := map[string]struct{}{}
	for _, item := range market {
		if item == nil || item.Model == "" {
			continue
		}
		modelSet[item.Model] = struct{}{}
		marketByModel[item.Model] = append(marketByModel[item.Model], item)
	}
	for _, item := range cache {
		if item == nil || item.Model == "" {
			continue
		}
		modelSet[item.Model] = struct{}{}
		current := cacheByModel[item.Model]
		if current == nil || item.ObservedAt.After(current.ObservedAt) || (item.ObservedAt.Equal(current.ObservedAt) && item.ID > current.ID) {
			cacheByModel[item.Model] = item
		}
	}
	for _, item := range costs {
		if item == nil || item.Model == "" {
			continue
		}
		modelSet[item.Model] = struct{}{}
		current := costByModel[item.Model]
		if current == nil || item.PriceMicros < current.PriceMicros || (item.PriceMicros == current.PriceMicros && item.CapturedAt.After(current.CapturedAt)) {
			costByModel[item.Model] = item
		}
	}
	for _, item := range quality {
		if item == nil || item.Model == "" {
			continue
		}
		modelSet[item.Model] = struct{}{}
		current := qualityByModel[item.Model]
		if current == nil || item.ObservedAt.After(current.ObservedAt) || (item.ObservedAt.Equal(current.ObservedAt) && item.ID > current.ID) {
			qualityByModel[item.Model] = item
		}
	}
	for _, item := range acceptance {
		if item == nil || item.Model == "" {
			continue
		}
		modelSet[item.Model] = struct{}{}
		current := acceptanceByModel[item.Model]
		if current == nil || item.ObservedAt.After(current.ObservedAt) || (item.ObservedAt.Equal(current.ObservedAt) && item.ID > current.ID) {
			acceptanceByModel[item.Model] = item
		}
	}
	models := make([]string, 0, len(modelSet))
	for model := range modelSet {
		models = append(models, model)
	}
	sort.Strings(models)
	rows := make([]adminplusdomain.KanbanModelMarginRow, 0, len(models))
	for _, model := range models {
		row := buildModelMarginRow(model, marketByModel[model], cacheByModel[model], qualityByModel[model], acceptanceByModel[model], costByModel[model], marginPercent, riskBufferPercent)
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].RiskLevel != rows[j].RiskLevel {
			return riskRank(rows[i].RiskLevel) > riskRank(rows[j].RiskLevel)
		}
		return rows[i].Model < rows[j].Model
	})
	return rows
}

func buildModelMarginRow(model string, market []*adminplusdomain.MarketPriceSnapshot, cache *adminplusdomain.CacheEfficiencySnapshot, quality *adminplusdomain.SupplyQualitySnapshot, acceptance *adminplusdomain.AcceptanceReport, cost *SupplierRateCost, marginPercent float64, riskBufferPercent float64) adminplusdomain.KanbanModelMarginRow {
	requiredMarkupPercent := marginPercent + riskBufferPercent
	requiredGrossMarginPercent := grossMarginFromMarkupPercent(requiredMarkupPercent)
	row := adminplusdomain.KanbanModelMarginRow{
		Model:                 model,
		Currency:              "USD",
		RequiredMarginPercent: requiredGrossMarginPercent,
		RiskLevel:             "unknown",
		Recommendation:        "数据不足，继续补充市场价、成本和缓存审计。",
	}
	if len(market) > 0 {
		prices := make([]int64, 0, len(market))
		for _, item := range market {
			prices = append(prices, item.PriceMicros)
			if item.ObservedAt.After(timeValue(row.LatestMarketObservedAt)) {
				t := item.ObservedAt
				row.LatestMarketObservedAt = &t
			}
		}
		sort.Slice(prices, func(i, j int) bool { return prices[i] < prices[j] })
		low := prices[0]
		median := prices[len(prices)/2]
		high := prices[len(prices)-1]
		row.MarketLowPriceMicros = &low
		row.MarketMedianPriceMicros = &median
		row.MarketHighPriceMicros = &high
		row.MarketSampleCount = len(prices)
		row.Currency = market[0].Currency
	}
	var adjustedCost *int64
	if cost != nil {
		costValue := cost.PriceMicros
		adjusted := costValue
		if cache != nil && cache.EstimatedWasteCents > 0 {
			adjusted += cache.EstimatedWasteCents * 10000
		}
		row.BestSupplierCostMicros = &costValue
		row.CacheAdjustedCostMicros = &adjusted
		row.Currency = cost.Currency
		t := cost.CapturedAt
		row.LatestSupplierCapturedAt = &t
		adjustedCost = &adjusted
	}
	if cache != nil {
		ratio := cache.CacheHitRatio
		row.CacheHitRatio = &ratio
		row.CacheStatus = cache.Status
		t := cache.ObservedAt
		row.LatestCacheObservedAt = &t
	}
	if quality != nil {
		score := quality.QualityScore
		row.QualityScore = &score
		row.QualityDecision = quality.Decision
	}
	if acceptance != nil {
		row.AcceptanceStatus = acceptance.Status
	}
	if adjustedCost != nil {
		suggested := int64(float64(*adjustedCost) * (1 + requiredMarkupPercent/100))
		if suggested < defaultSuggestedPriceMicros {
			suggested = defaultSuggestedPriceMicros
		}
		row.SuggestedPriceMicros = &suggested
		if row.MarketMedianPriceMicros != nil && *row.MarketMedianPriceMicros > 0 {
			margin := (float64(*row.MarketMedianPriceMicros-*adjustedCost) / float64(*row.MarketMedianPriceMicros)) * 100
			row.GrossMarginPercent = &margin
			marginGap := margin - requiredGrossMarginPercent
			row.MarginGapPercent = &marginGap
			suggestedVsMarket := (float64(suggested-*row.MarketMedianPriceMicros) / float64(*row.MarketMedianPriceMicros)) * 100
			row.SuggestedVsMarketPercent = &suggestedVsMarket
		}
	}
	row.RiskLevel, row.Recommendation = classifyModelMargin(row)
	return row
}

func classifyModelMargin(row adminplusdomain.KanbanModelMarginRow) (string, string) {
	if row.AcceptanceStatus == "blocked" || row.AcceptanceStatus == "paused" {
		return "high", "接入验收未通过；不得进入生产候选。"
	}
	if row.QualityDecision == "blocked" || row.QualityDecision == "paused" {
		return "high", "供应质量未达生产标准；接入前需修复纯度、账单可信度、余额、并发或可用性问题。"
	}
	if row.CacheStatus == "bad" || (row.CacheHitRatio != nil && *row.CacheHitRatio < defaultCacheRiskHitRatio) {
		return "high", "缓存命中率过低；轮询号池可能导致长上下文重复计费，接入生产前需要 sticky routing。"
	}
	if row.GrossMarginPercent != nil && *row.GrossMarginPercent < 0 {
		return "high", "市场中位价低于缓存调整后成本，不建议跟价。"
	}
	if row.MarginGapPercent != nil && *row.MarginGapPercent < 0 {
		return "medium", "市场中位价仍有正毛利，但低于目标毛利和风险缓冲；建议仅低权重观察或继续找更低成本供应。"
	}
	if row.SuggestedVsMarketPercent != nil && *row.SuggestedVsMarketPercent > 0 {
		return "medium", "达到目标毛利所需售价高于市场中位价；建议先复核质量和成本，不要直接跟价。"
	}
	if row.CacheStatus == "risky" || (row.CacheHitRatio != nil && *row.CacheHitRatio < defaultWatchingHitRatio) {
		return "medium", "缓存效率偏低，建议降低权重或仅观察。"
	}
	if row.QualityDecision == "low_priority" || (row.QualityScore != nil && *row.QualityScore < 70) {
		return "medium", "供应质量偏低，建议作为低优先级或继续观察。"
	}
	if row.MarketMedianPriceMicros == nil || row.CacheAdjustedCostMicros == nil {
		return "unknown", "数据不足，继续补充市场价、成本和缓存审计。"
	}
	return "low", "价格和缓存效率可接受，可作为生产候选继续观察。"
}

func grossMarginFromMarkupPercent(markupPercent float64) float64 {
	multiplier := 1 + markupPercent/100
	if multiplier <= 0 {
		return 0
	}
	return (markupPercent / 100) / multiplier * 100
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}
	return limit
}

func normalizeSourceType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "site_catalog", "site_discovery", "provider_page", "api":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "manual"
	}
}

func normalizeSourceTypeFilter(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return ""
	}
	switch v {
	case "manual", "site_catalog", "site_discovery", "provider_page", "api":
		return v
	default:
		return ""
	}
}

func normalizeSupplyType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "own_pool", "competitor", "custom":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "supplier"
	}
}

func normalizeSupplyTypeFilter(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return ""
	}
	switch v {
	case "supplier", "own_pool", "competitor", "custom":
		return v
	default:
		return ""
	}
}

func normalizeRoutingStrategy(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "fixed_account", "round_robin", "weighted_round_robin", "sticky", "least_loaded", "custom":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unknown"
	}
}

func normalizeStickyScope(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "user", "api_key", "project", "session", "organization", "custom":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "none"
	}
}

func normalizeCacheStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "healthy", "watching", "risky", "bad":
		return strings.ToLower(strings.TrimSpace(value))
	case "unknown":
		return "unknown"
	default:
		return ""
	}
}

func normalizeEventType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "market_price_drop", "market_price_rise", "market_price_anomaly", "market_model_added", "market_model_removed", "market_promotion", "cache_efficiency_risk", "supply_quality_risk", "acceptance_risk", "unprofitable_model", "pricing_recommendation":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeCheckStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pass", "warn", "fail":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unknown"
	}
}

func normalizeQualityDecision(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "production", "watching", "low_priority", "paused", "blocked":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeEventSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "info", "warning", "critical":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeEventStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "open", "acknowledged", "ignored":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func statusFromCacheHitRatio(ratio float64) string {
	switch {
	case ratio <= 0:
		return "unknown"
	case ratio < defaultCacheRiskHitRatio:
		return "bad"
	case ratio < defaultWatchingHitRatio:
		return "risky"
	case ratio < 0.65:
		return "watching"
	default:
		return "healthy"
	}
}

func deriveCacheHitRatio(in CacheEfficiencyInput) float64 {
	if in.CacheHitRatio != nil {
		return *in.CacheHitRatio
	}
	denominator := in.CacheReadTokens + in.CacheWriteTokens + in.InputTokens
	if denominator <= 0 {
		return 0
	}
	return float64(in.CacheReadTokens) / float64(denominator)
}

func deriveQualityScore(in SupplyQualityInput) float64 {
	availabilityScore := in.AvailabilityRatio * 100
	errorScore := (1 - in.ErrorRatio) * 100
	cacheScore := in.CacheHitRatio * 100
	balanceScore := 100 - in.BalanceRiskScore
	score := availabilityScore*0.20 + errorScore*0.15 + latencyScore(in.AvgTTFTMS, in.AvgTotalLatencyMS)*0.15 + cacheScore*0.15 + in.ConcurrencyScore*0.10 + in.PurityScore*0.15 + in.UsageTrustScore*0.05 + balanceScore*0.05
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func latencyScore(ttft *int64, total *int64) float64 {
	score := 100.0
	if ttft != nil {
		switch {
		case *ttft > 8000:
			score -= 45
		case *ttft > 4000:
			score -= 25
		case *ttft > 2000:
			score -= 10
		}
	}
	if total != nil {
		switch {
		case *total > 60000:
			score -= 45
		case *total > 30000:
			score -= 25
		case *total > 15000:
			score -= 10
		}
	}
	if score < 0 {
		return 0
	}
	return score
}

func decisionFromQualityScore(score float64, in SupplyQualityInput) string {
	switch {
	case in.PurityScore > 0 && in.PurityScore < 50:
		return "blocked"
	case in.UsageTrustScore > 0 && in.UsageTrustScore < 50:
		return "blocked"
	case score < 50 || in.AvailabilityRatio < 0.8 || in.ErrorRatio > 0.2:
		return "paused"
	case score < 70 || in.CacheHitRatio < defaultWatchingHitRatio:
		return "low_priority"
	case score < 85:
		return "watching"
	default:
		return "production"
	}
}

func decisionFromAcceptance(report *adminplusdomain.AcceptanceReport) string {
	if hasFailedAcceptanceStep(report) {
		return "blocked"
	}
	if hasWarnAcceptanceStep(report) || hasUnknownAcceptanceStep(report) {
		return "watching"
	}
	return "production"
}

func recommendationFromAcceptance(report *adminplusdomain.AcceptanceReport) string {
	if report == nil {
		return ""
	}
	if hasFailedAcceptanceStep(report) {
		return "验收失败，需修复失败检查项后重新验收。"
	}
	if hasWarnAcceptanceStep(report) {
		return "验收存在警告，建议先观察或低优先级接入。"
	}
	if hasUnknownAcceptanceStep(report) {
		return "验收数据不足，补齐未知检查项后再进入生产。"
	}
	return "验收通过，可作为生产候选继续观察。"
}

func hasFailedAcceptanceStep(report *adminplusdomain.AcceptanceReport) bool {
	if report == nil {
		return false
	}
	for _, status := range acceptanceStepStatuses(report) {
		if status == "fail" {
			return true
		}
	}
	return false
}

func hasWarnAcceptanceStep(report *adminplusdomain.AcceptanceReport) bool {
	if report == nil {
		return false
	}
	for _, status := range acceptanceStepStatuses(report) {
		if status == "warn" {
			return true
		}
	}
	return false
}

func hasUnknownAcceptanceStep(report *adminplusdomain.AcceptanceReport) bool {
	if report == nil {
		return false
	}
	for _, status := range acceptanceStepStatuses(report) {
		if status == "unknown" {
			return true
		}
	}
	return false
}

func acceptanceStepStatuses(report *adminplusdomain.AcceptanceReport) []string {
	return []string{
		report.ConnectivityStatus,
		report.ModelListStatus,
		report.PurityStatus,
		report.TrialCallStatus,
		report.UsageMeteringStatus,
		report.CacheAuditStatus,
		report.BalanceStatus,
		report.ConcurrencyStatus,
	}
}

func acceptanceStepKeys() []string {
	return []string{
		"connectivity_status",
		"model_list_status",
		"purity_status",
		"trial_call_status",
		"usage_metering_status",
		"cache_audit_status",
		"balance_status",
		"concurrency_status",
	}
}

func acceptanceStepStatus(report *adminplusdomain.AcceptanceReport, step string) string {
	if report == nil {
		return "unknown"
	}
	switch step {
	case "connectivity_status":
		return normalizeCheckStatus(report.ConnectivityStatus)
	case "model_list_status":
		return normalizeCheckStatus(report.ModelListStatus)
	case "purity_status":
		return normalizeCheckStatus(report.PurityStatus)
	case "trial_call_status":
		return normalizeCheckStatus(report.TrialCallStatus)
	case "usage_metering_status":
		return normalizeCheckStatus(report.UsageMeteringStatus)
	case "cache_audit_status":
		return normalizeCheckStatus(report.CacheAuditStatus)
	case "balance_status":
		return normalizeCheckStatus(report.BalanceStatus)
	case "concurrency_status":
		return normalizeCheckStatus(report.ConcurrencyStatus)
	default:
		return "unknown"
	}
}

func buildAcceptanceStepSummaries(reports []*adminplusdomain.AcceptanceReport) []adminplusdomain.AcceptanceStepSummary {
	summaries := make([]adminplusdomain.AcceptanceStepSummary, 0, len(acceptanceStepKeys()))
	for _, step := range acceptanceStepKeys() {
		summary := adminplusdomain.AcceptanceStepSummary{Step: step}
		for _, report := range reports {
			if report == nil {
				continue
			}
			summary.TotalCount++
			switch acceptanceStepStatus(report, step) {
			case "pass":
				summary.PassCount++
			case "warn":
				summary.WarnCount++
			case "fail":
				summary.FailCount++
			default:
				summary.UnknownCount++
			}
		}
		summary.RiskLevel = acceptanceStepRiskLevel(summary)
		summaries = append(summaries, summary)
	}
	return summaries
}

func acceptanceStepRiskLevel(summary adminplusdomain.AcceptanceStepSummary) string {
	switch {
	case summary.TotalCount == 0:
		return "unknown"
	case summary.FailCount > 0:
		return "high"
	case summary.WarnCount > 0 || summary.UnknownCount > 0:
		return "medium"
	default:
		return "low"
	}
}

func validRatio(value float64) bool {
	return value >= 0 && value <= 1
}

func validScore(value float64) bool {
	return value >= 0 && value <= 100
}

func normalizeCurrency(value string) string {
	v := strings.ToUpper(strings.TrimSpace(value))
	if len(v) != 3 {
		return "USD"
	}
	return v
}

func defaultLower(value string, fallback string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return fallback
	}
	return v
}

func trimLimit(value string, limit int) string {
	v := strings.TrimSpace(value)
	if len(v) > limit {
		return v[:limit]
	}
	return v
}

func positiveID(value int64) int64 {
	if value > 0 {
		return value
	}
	return 0
}

func nonNegativePtr(value *int64) *int64 {
	if value == nil || *value < 0 {
		return nil
	}
	v := *value
	return &v
}

func nonNegativeFloatPtr(value *float64) *float64 {
	if value == nil || *value < 0 {
		return nil
	}
	v := *value
	return &v
}

func firstMarketSnapshots(items []*adminplusdomain.MarketPriceSnapshot, limit int) []*adminplusdomain.MarketPriceSnapshot {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func firstCacheSnapshots(items []*adminplusdomain.CacheEfficiencySnapshot, limit int) []*adminplusdomain.CacheEfficiencySnapshot {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func firstQualitySnapshots(items []*adminplusdomain.SupplyQualitySnapshot, limit int) []*adminplusdomain.SupplyQualitySnapshot {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func firstAcceptanceReports(items []*adminplusdomain.AcceptanceReport, limit int) []*adminplusdomain.AcceptanceReport {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func firstKanbanEvents(items []*adminplusdomain.KanbanEvent, limit int) []*adminplusdomain.KanbanEvent {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func timeValue(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

func countOpenEvents(items []*adminplusdomain.KanbanEvent) int {
	count := 0
	for _, item := range items {
		if item != nil && item.Status == "open" {
			count++
		}
	}
	return count
}

func countCriticalEvents(items []*adminplusdomain.KanbanEvent) int {
	count := 0
	for _, item := range items {
		if item != nil && item.Severity == "critical" && item.Status == "open" {
			count++
		}
	}
	return count
}

func firstPositiveID(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func countRiskyCacheRows(rows []adminplusdomain.KanbanModelMarginRow) int {
	count := 0
	for _, row := range rows {
		if row.CacheStatus == "bad" || row.CacheStatus == "risky" || (row.CacheHitRatio != nil && *row.CacheHitRatio < defaultWatchingHitRatio) {
			count++
		}
	}
	return count
}

func countUnprofitableRows(rows []adminplusdomain.KanbanModelMarginRow) int {
	count := 0
	for _, row := range rows {
		if row.GrossMarginPercent != nil && *row.GrossMarginPercent < 0 {
			count++
		}
	}
	return count
}

func countRiskyQualityRows(rows []adminplusdomain.KanbanModelMarginRow) int {
	count := 0
	for _, row := range rows {
		if row.QualityDecision == "blocked" || row.QualityDecision == "paused" || row.QualityDecision == "low_priority" || (row.QualityScore != nil && *row.QualityScore < 70) {
			count++
		}
	}
	return count
}

func countBlockedAcceptanceReports(items []*adminplusdomain.AcceptanceReport) int {
	count := 0
	for _, item := range items {
		if item != nil && (item.Status == "blocked" || item.Status == "paused") {
			count++
		}
	}
	return count
}

func riskRank(value string) int {
	switch value {
	case "high":
		return 4
	case "medium":
		return 3
	case "low":
		return 2
	default:
		return 1
	}
}
