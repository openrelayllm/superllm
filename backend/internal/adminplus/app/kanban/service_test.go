package kanban

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	schedulerapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/scheduler"
	sitecatalogapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sitecatalog"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

func TestServiceOverviewClassifiesRoundRobinCacheRisk(t *testing.T) {
	now := time.Date(2026, 7, 4, 1, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		market: []*adminplusdomain.MarketPriceSnapshot{
			{ID: 1, Model: "gpt-test", Currency: "USD", PriceMicros: 2000000, ObservedAt: now.Add(-time.Hour)},
		},
		cache: []*adminplusdomain.CacheEfficiencySnapshot{
			{
				ID:                   2,
				Model:                "gpt-test",
				SupplyType:           "own_pool",
				RoutingStrategy:      "round_robin",
				StickyScope:          "none",
				CacheHitRatio:        0.12,
				EstimatedWasteCents:  30,
				Status:               "bad",
				ObservedAt:           now,
				DuplicateInputTokens: 10000,
			},
		},
		costs: []*SupplierRateCost{
			{SupplierID: 10, Model: "gpt-test", Currency: "USD", PriceMicros: 1200000, CapturedAt: now},
		},
	}
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	overview, err := svc.Overview(context.Background(), OverviewFilter{TargetMarginPercent: 25, RiskBufferPercent: 8})
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if overview.RiskyCacheModelCount != 1 {
		t.Fatalf("RiskyCacheModelCount = %d, want 1", overview.RiskyCacheModelCount)
	}
	if len(overview.ModelMargins) != 1 {
		t.Fatalf("ModelMargins len = %d, want 1", len(overview.ModelMargins))
	}
	row := overview.ModelMargins[0]
	if row.RiskLevel != "high" {
		t.Fatalf("RiskLevel = %q, want high", row.RiskLevel)
	}
	if row.CacheAdjustedCostMicros == nil || *row.CacheAdjustedCostMicros != 1500000 {
		t.Fatalf("CacheAdjustedCostMicros = %v, want 1500000", row.CacheAdjustedCostMicros)
	}
	if row.CacheHitRatio == nil || *row.CacheHitRatio != 0.12 {
		t.Fatalf("CacheHitRatio = %v, want 0.12", row.CacheHitRatio)
	}
	if row.SuggestedPriceMicros == nil || *row.SuggestedPriceMicros != 1995000 {
		t.Fatalf("SuggestedPriceMicros = %v, want 1995000", row.SuggestedPriceMicros)
	}
}

func TestServiceOverviewSummarizesAcceptanceSteps(t *testing.T) {
	now := time.Date(2026, 7, 4, 1, 15, 0, 0, time.UTC)
	repo := &fakeRepository{
		acceptance: []*adminplusdomain.AcceptanceReport{
			{
				ID:                  1,
				Model:               "gpt-acceptance",
				Status:              "production",
				ConnectivityStatus:  "pass",
				ModelListStatus:     "pass",
				PurityStatus:        "pass",
				TrialCallStatus:     "pass",
				UsageMeteringStatus: "pass",
				CacheAuditStatus:    "pass",
				BalanceStatus:       "pass",
				ConcurrencyStatus:   "pass",
				ObservedAt:          now,
			},
			{
				ID:                  2,
				Model:               "gpt-acceptance",
				Status:              "blocked",
				ConnectivityStatus:  "pass",
				ModelListStatus:     "warn",
				PurityStatus:        "fail",
				TrialCallStatus:     "unknown",
				UsageMeteringStatus: "pass",
				CacheAuditStatus:    "fail",
				BalanceStatus:       "warn",
				ConcurrencyStatus:   "unknown",
				ObservedAt:          now,
			},
		},
	}
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	overview, err := svc.Overview(context.Background(), OverviewFilter{})
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if len(overview.AcceptanceStepSummaries) != 8 {
		t.Fatalf("AcceptanceStepSummaries len = %d, want 8", len(overview.AcceptanceStepSummaries))
	}
	purity := findAcceptanceStepSummary(overview.AcceptanceStepSummaries, "purity_status")
	if purity == nil || purity.FailCount != 1 || purity.PassCount != 1 || purity.RiskLevel != "high" {
		t.Fatalf("purity summary = %#v, want 1 fail, 1 pass, high", purity)
	}
	modelList := findAcceptanceStepSummary(overview.AcceptanceStepSummaries, "model_list_status")
	if modelList == nil || modelList.WarnCount != 1 || modelList.RiskLevel != "medium" {
		t.Fatalf("model list summary = %#v, want warning medium", modelList)
	}
	trial := findAcceptanceStepSummary(overview.AcceptanceStepSummaries, "trial_call_status")
	if trial == nil || trial.UnknownCount != 1 || trial.RiskLevel != "medium" {
		t.Fatalf("trial call summary = %#v, want unknown medium", trial)
	}
	connectivity := findAcceptanceStepSummary(overview.AcceptanceStepSummaries, "connectivity_status")
	if connectivity == nil || connectivity.PassCount != 2 || connectivity.RiskLevel != "low" {
		t.Fatalf("connectivity summary = %#v, want all pass low", connectivity)
	}
}

func TestServiceOverviewUsesUsageDerivedCacheRisk(t *testing.T) {
	now := time.Date(2026, 7, 4, 1, 30, 0, 0, time.UTC)
	repo := &fakeRepository{
		market: []*adminplusdomain.MarketPriceSnapshot{
			{ID: 1, Model: "gpt-derived-cache", Currency: "USD", PriceMicros: 2000000, ObservedAt: now.Add(-time.Hour)},
		},
		costs: []*SupplierRateCost{
			{SupplierID: 10, Model: "gpt-derived-cache", Currency: "USD", PriceMicros: 1200000, CapturedAt: now},
		},
		derived: &UsageDerivedSnapshots{
			Cache: []*adminplusdomain.CacheEfficiencySnapshot{
				{
					ID:                   -1,
					SupplyType:           "own_pool",
					Model:                "gpt-derived-cache",
					RoutingStrategy:      "round_robin",
					StickyScope:          "none",
					CacheHitRatio:        0.08,
					EstimatedWasteCents:  25,
					Status:               "bad",
					ObservedAt:           now,
					DuplicateInputTokens: 9000,
					RawPayload:           map[string]any{"derived": true, "source": "usage_logs"},
				},
			},
		},
	}
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	overview, err := svc.Overview(context.Background(), OverviewFilter{TargetMarginPercent: 25, RiskBufferPercent: 8})
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if overview.CacheSnapshotCount != 1 {
		t.Fatalf("CacheSnapshotCount = %d, want 1 derived snapshot", overview.CacheSnapshotCount)
	}
	if overview.RiskyCacheModelCount != 1 {
		t.Fatalf("RiskyCacheModelCount = %d, want 1", overview.RiskyCacheModelCount)
	}
	row := overview.ModelMargins[0]
	if row.RiskLevel != "high" {
		t.Fatalf("RiskLevel = %q, want high", row.RiskLevel)
	}
	if row.CacheHitRatio == nil || *row.CacheHitRatio != 0.08 {
		t.Fatalf("CacheHitRatio = %v, want 0.08", row.CacheHitRatio)
	}
	if len(overview.RecentCacheSnapshots) != 1 || overview.RecentCacheSnapshots[0].ID != -1 {
		t.Fatalf("RecentCacheSnapshots = %#v, want derived cache snapshot", overview.RecentCacheSnapshots)
	}
}

func TestGenerateAcceptanceReportUsesUsageDerivedEvidence(t *testing.T) {
	now := time.Date(2026, 7, 4, 2, 15, 0, 0, time.UTC)
	repo := &fakeRepository{
		derived: &UsageDerivedSnapshots{
			Cache: []*adminplusdomain.CacheEfficiencySnapshot{
				{
					ID:                    -1,
					SupplyType:            "supplier",
					SupplierID:            7,
					LocalSub2APIAccountID: 19,
					Model:                 "gpt-derived-acceptance",
					CacheHitRatio:         0.1,
					Status:                "bad",
					ObservedAt:            now,
					RawPayload:            map[string]any{"derived": true, "source": "usage_logs"},
				},
			},
			Quality: []*adminplusdomain.SupplyQualitySnapshot{
				{
					ID:                    -100001,
					SupplyType:            "supplier",
					SupplierID:            7,
					LocalSub2APIAccountID: 19,
					Model:                 "gpt-derived-acceptance",
					AvailabilityRatio:     0.76,
					ErrorRatio:            0.24,
					CacheHitRatio:         0.1,
					PurityScore:           90,
					UsageTrustScore:       76,
					BalanceRiskScore:      15,
					ConcurrencyScore:      80,
					QualityScore:          60,
					Decision:              "paused",
					ObservedAt:            now,
					RawPayload:            map[string]any{"derived": true, "source": "usage_logs", "error_count": int64(24)},
				},
			},
		},
	}
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	report, err := svc.GenerateAcceptanceReport(context.Background(), AcceptanceReportGenerateInput{
		SupplyType:            "supplier",
		SupplierID:            7,
		LocalSub2APIAccountID: 19,
		Model:                 "gpt-derived-acceptance",
	})
	if err != nil {
		t.Fatalf("GenerateAcceptanceReport() error = %v", err)
	}
	if report.Status != "blocked" {
		t.Fatalf("Status = %q, want blocked", report.Status)
	}
	if report.ConnectivityStatus != "fail" || report.CacheAuditStatus != "fail" {
		t.Fatalf("report statuses = %#v, want usage-derived connectivity/cache failures", report)
	}
	if report.ReportPayload["quality_snapshot_id"] != int64(-100001) || report.ReportPayload["cache_snapshot_id"] != int64(-1) {
		t.Fatalf("payload = %#v, want usage-derived snapshot ids", report.ReportPayload)
	}
	if len(repo.events) != 1 || repo.events[0].EventType != "acceptance_risk" {
		t.Fatalf("events = %#v, want acceptance_risk from derived evidence", repo.events)
	}
}

func TestRecordCacheEfficiencyCreatesRiskEvent(t *testing.T) {
	now := time.Date(2026, 7, 4, 3, 0, 0, 0, time.UTC)
	repo := &fakeRepository{}
	svc := NewService(repo)
	svc.now = func() time.Time { return now }
	ratio := 0.18

	snapshot, err := svc.RecordCacheEfficiency(context.Background(), CacheEfficiencyInput{
		SupplyType:           "own_pool",
		Model:                "gpt-cache",
		RoutingStrategy:      "round_robin",
		StickyScope:          "none",
		CacheHitRatio:        &ratio,
		DuplicateInputTokens: 12000,
		EstimatedWasteCents:  42,
		Notes:                "round robin",
	})
	if err != nil {
		t.Fatalf("RecordCacheEfficiency() error = %v", err)
	}
	if snapshot.Status != "bad" {
		t.Fatalf("snapshot status = %q, want bad", snapshot.Status)
	}
	if len(repo.events) != 1 {
		t.Fatalf("events len = %d, want 1", len(repo.events))
	}
	event := repo.events[0]
	if event.EventType != "cache_efficiency_risk" || event.Severity != "critical" {
		t.Fatalf("event = %#v, want critical cache_efficiency_risk", event)
	}
	if event.RelatedSnapshotID != snapshot.ID {
		t.Fatalf("RelatedSnapshotID = %d, want %d", event.RelatedSnapshotID, snapshot.ID)
	}
}

func TestListCacheEfficiencyEmptySupplyTypeDoesNotFilterToSupplier(t *testing.T) {
	now := time.Date(2026, 7, 4, 3, 30, 0, 0, time.UTC)
	repo := &fakeRepository{
		cache: []*adminplusdomain.CacheEfficiencySnapshot{
			{ID: 1, SupplyType: "supplier", Model: "gpt-cache-list", CacheHitRatio: 0.8, Status: "healthy", ObservedAt: now},
			{ID: 2, SupplyType: "own_pool", Model: "gpt-cache-list", CacheHitRatio: 0.2, Status: "bad", ObservedAt: now.Add(time.Minute)},
		},
	}
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	items, err := svc.ListCacheEfficiency(context.Background(), CacheEfficiencyFilter{Model: "gpt-cache-list"})
	if err != nil {
		t.Fatalf("ListCacheEfficiency() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len = %d, want supplier and own_pool", len(items))
	}
	if items[0].SupplyType != "own_pool" || items[1].SupplyType != "supplier" {
		t.Fatalf("items order/types = %#v, want all supply types sorted by observed_at", items)
	}
}

func TestListSupplyQualityEmptySupplyTypeDoesNotFilterToSupplier(t *testing.T) {
	now := time.Date(2026, 7, 4, 3, 45, 0, 0, time.UTC)
	repo := &fakeRepository{
		quality: []*adminplusdomain.SupplyQualitySnapshot{
			{ID: 1, SupplyType: "supplier", Model: "gpt-quality-list", AvailabilityRatio: 0.99, QualityScore: 90, Decision: "production", ObservedAt: now},
			{ID: 2, SupplyType: "own_pool", Model: "gpt-quality-list", AvailabilityRatio: 0.8, QualityScore: 60, Decision: "paused", ObservedAt: now.Add(time.Minute)},
		},
	}
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	items, err := svc.ListSupplyQuality(context.Background(), SupplyQualityFilter{Model: "gpt-quality-list"})
	if err != nil {
		t.Fatalf("ListSupplyQuality() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len = %d, want supplier and own_pool", len(items))
	}
	if items[0].SupplyType != "own_pool" || items[1].SupplyType != "supplier" {
		t.Fatalf("items order/types = %#v, want all supply types sorted by observed_at", items)
	}
}

func TestUsageDerivedQualityFromErrorOnlyRow(t *testing.T) {
	now := time.Date(2026, 7, 4, 3, 50, 0, 0, time.UTC)
	row := usageDerivedRow{
		supplyType:   "supplier",
		supplierID:   7,
		accountID:    19,
		model:        "gpt-error-only",
		accountCount: 1,
		observedAt:   now,
		errorCount:   3,
	}

	snapshot := qualitySnapshotFromUsageDerivedRow(row, -100001)

	if snapshot.AvailabilityRatio != 0 {
		t.Fatalf("AvailabilityRatio = %v, want 0 for error-only evidence", snapshot.AvailabilityRatio)
	}
	if snapshot.ErrorRatio != 1 {
		t.Fatalf("ErrorRatio = %v, want 1 for error-only evidence", snapshot.ErrorRatio)
	}
	if snapshot.Decision != "paused" {
		t.Fatalf("Decision = %q, want paused", snapshot.Decision)
	}
	if snapshot.RawPayload["derived"] != true || snapshot.RawPayload["error_count"] != int64(3) {
		t.Fatalf("RawPayload = %#v, want derived error evidence", snapshot.RawPayload)
	}
}

func TestRecordMarketPriceCreatesDropEvent(t *testing.T) {
	now := time.Date(2026, 7, 4, 4, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		market: []*adminplusdomain.MarketPriceSnapshot{
			{ID: 10, Model: "gpt-price", SourceType: "manual", SourceName: "competitor", BillingMode: "tokens", PriceItem: "blended", Unit: "1m_tokens", Currency: "USD", PriceMicros: 2000000, ObservedAt: now.Add(-time.Hour)},
		},
	}
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	snapshot, err := svc.RecordMarketPrice(context.Background(), MarketPriceInput{
		SourceType:  "manual",
		SourceName:  "competitor",
		Model:       "gpt-price",
		Currency:    "USD",
		PriceMicros: 1500000,
	})
	if err != nil {
		t.Fatalf("RecordMarketPrice() error = %v", err)
	}
	if snapshot.ID == 0 {
		t.Fatalf("snapshot ID was not assigned")
	}
	if len(repo.events) != 1 {
		t.Fatalf("events len = %d, want 1", len(repo.events))
	}
	event := repo.events[0]
	if event.EventType != "market_price_drop" || event.Severity != "warning" {
		t.Fatalf("event = %#v, want warning market_price_drop", event)
	}
}

func TestParseMarketPricesCreatesSnapshotsWithoutCrossItemPriceEvent(t *testing.T) {
	now := time.Date(2026, 7, 4, 4, 30, 0, 0, time.UTC)
	repo := &fakeRepository{}
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	result, err := svc.ParseMarketPrices(context.Background(), MarketPriceParseInput{
		SourceType: "provider_page",
		SourceName: "competitor",
		SourceURL:  "https://example.com/pricing",
		Text:       "gpt-4o-mini input $0.15 / 1M tokens\ngpt-4o-mini output $0.60 / 1M tokens",
	})

	if err != nil {
		t.Fatalf("ParseMarketPrices() error = %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("Total = %d, want 2", result.Total)
	}
	if result.Items[0].Model != "gpt-4o-mini" || result.Items[0].PriceItem != "input" || result.Items[0].PriceMicros != 150000 {
		t.Fatalf("first parsed item = %#v, want gpt-4o-mini input 150000", result.Items[0])
	}
	if result.Items[1].PriceItem != "output" || result.Items[1].PriceMicros != 600000 {
		t.Fatalf("second parsed item = %#v, want output 600000", result.Items[1])
	}
	if len(repo.events) != 0 {
		t.Fatalf("events len = %d, want 0", len(repo.events))
	}
}

func TestParseMarketPricesCapturesPackageRechargeAndMultiplier(t *testing.T) {
	now := time.Date(2026, 7, 4, 4, 45, 0, 0, time.UTC)
	repo := &fakeRepository{}
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	result, err := svc.ParseMarketPrices(context.Background(), MarketPriceParseInput{
		SourceType: "provider_page",
		SourceName: "competitor",
		SourceURL:  "https://example.com/pricing",
		Text:       "gpt-4o-mini input $0.15 / 1M tokens package $10 100M tokens min recharge $20 bonus 15% rate 0.8x",
	})

	if err != nil {
		t.Fatalf("ParseMarketPrices() error = %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("Total = %d, want 1", result.Total)
	}
	item := result.Items[0]
	if item.Model != "gpt-4o-mini" || item.PriceItem != "input" || item.PriceMicros != 150000 {
		t.Fatalf("parsed item = %#v, want gpt-4o-mini input 150000", item)
	}
	if item.PackageLabel != "package" {
		t.Fatalf("PackageLabel = %q, want package", item.PackageLabel)
	}
	if item.PackagePriceCents == nil || *item.PackagePriceCents != 1000 {
		t.Fatalf("PackagePriceCents = %v, want 1000", item.PackagePriceCents)
	}
	if item.PackageQuota == "" {
		t.Fatalf("PackageQuota is empty")
	}
	if item.MinRechargeCents == nil || *item.MinRechargeCents != 2000 {
		t.Fatalf("MinRechargeCents = %v, want 2000", item.MinRechargeCents)
	}
	if item.BonusPercent == nil || *item.BonusPercent != 15 {
		t.Fatalf("BonusPercent = %v, want 15", item.BonusPercent)
	}
	if item.RateMultiplier == nil || *item.RateMultiplier != 0.8 {
		t.Fatalf("RateMultiplier = %v, want 0.8", item.RateMultiplier)
	}
}

func TestParseMarketPricesRecordsMarketTextEvents(t *testing.T) {
	now := time.Date(2026, 7, 4, 4, 50, 0, 0, time.UTC)
	repo := &fakeRepository{}
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	_, err := svc.ParseMarketPrices(context.Background(), MarketPriceParseInput{
		SourceType: "provider_page",
		SourceName: "competitor",
		SourceURL:  "https://example.com/pricing",
		Text: strings.Join([]string{
			"gpt-4o-mini input $0.15 / 1M tokens",
			"新增模型 gpt-5 已上线",
			"claude-2 下架",
			"限时活动 gpt-4o-mini bonus 20%",
		}, "\n"),
	})

	if err != nil {
		t.Fatalf("ParseMarketPrices() error = %v", err)
	}
	if len(repo.events) != 3 {
		t.Fatalf("events len = %d, want 3", len(repo.events))
	}
	eventTypes := map[string]bool{}
	for _, event := range repo.events {
		eventTypes[event.EventType] = true
	}
	for _, eventType := range []string{"market_model_added", "market_model_removed", "market_promotion"} {
		if !eventTypes[eventType] {
			t.Fatalf("eventTypes = %#v, missing %s", eventTypes, eventType)
		}
	}
}

func TestImportMarketPricesFromURLFetchesHTMLAndParsesSnapshots(t *testing.T) {
	now := time.Date(2026, 7, 4, 4, 55, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Fatalf("missing user agent")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><script>ignored()</script><main><p>gpt-import input $0.20 / 1M tokens</p><p>gpt-import output $0.80 / 1M tokens</p></main></body></html>`))
	}))
	defer server.Close()
	repo := &fakeRepository{}
	svc := NewService(repo).WithMarketPriceHTTPClient(server.Client())
	svc.now = func() time.Time { return now }

	result, err := svc.ImportMarketPricesFromURL(context.Background(), MarketPriceImportURLInput{
		SourceType:      "provider_page",
		SourceName:      "competitor",
		SourceURL:       server.URL + "/pricing#plans",
		DefaultCurrency: "USD",
		Confidence:      0.8,
	})

	if err != nil {
		t.Fatalf("ImportMarketPricesFromURL() error = %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("Total = %d, want 2", result.Total)
	}
	if result.SourceURL != server.URL+"/pricing" {
		t.Fatalf("SourceURL = %q, want normalized url", result.SourceURL)
	}
	if result.TextLength == 0 {
		t.Fatalf("TextLength = 0, want extracted page text length")
	}
	if repo.market[0].Model != "gpt-import" || repo.market[0].PriceItem != "input" || repo.market[0].PriceMicros != 200000 {
		t.Fatalf("first market snapshot = %#v, want gpt-import input 200000", repo.market[0])
	}
	if repo.market[1].PriceItem != "output" || repo.market[1].PriceMicros != 800000 {
		t.Fatalf("second market snapshot = %#v, want output 800000", repo.market[1])
	}
}

func TestImportMarketPricesFromURLRejectsUnsupportedScheme(t *testing.T) {
	svc := NewService(&fakeRepository{})

	_, err := svc.ImportMarketPricesFromURL(context.Background(), MarketPriceImportURLInput{
		SourceURL: "file:///tmp/pricing.html",
	})

	if err == nil {
		t.Fatalf("ImportMarketPricesFromURL() error = nil, want unsupported scheme")
	}
}

func TestDiscoverMarketPriceSourcesFromSiteCatalog(t *testing.T) {
	repo := &fakeRepository{}
	reader := &fakeSiteCatalogReader{
		sites: []*adminplusdomain.SiteCatalogSite{
			{
				ID:            42,
				Slug:          "competitor",
				CanonicalHost: "example.com",
				Name:          "Competitor",
				SupplierID:    7,
				Links: []*adminplusdomain.SiteCatalogLink{
					{LinkType: adminplusdomain.SiteCatalogLinkHomepage, URL: "https://example.com", Label: "首页", IsPrimary: true},
					{LinkType: adminplusdomain.SiteCatalogLinkDocs, URL: "https://example.com/docs/pricing", Label: "模型价格"},
					{LinkType: adminplusdomain.SiteCatalogLinkDashboard, URL: "https://example.com/dashboard", Label: "控制台"},
				},
			},
		},
	}
	svc := NewServiceWithDependencies(repo, reader)

	result, err := svc.DiscoverMarketPriceSources(context.Background(), PriceSourceDiscoveryInput{Limit: 10})
	if err != nil {
		t.Fatalf("DiscoverMarketPriceSources() error = %v", err)
	}
	if result.Total == 0 {
		t.Fatalf("Total = 0, want candidates")
	}
	first := result.Items[0]
	if first.SiteID != 42 || first.SupplierID != 7 || first.SourceType != "site_catalog" {
		t.Fatalf("first candidate = %#v, want site catalog candidate", first)
	}
	if first.SourceURL != "https://example.com/docs/pricing" {
		t.Fatalf("SourceURL = %q, want pricing docs link", first.SourceURL)
	}
}

func TestRecordSupplyQualityCreatesRiskEvent(t *testing.T) {
	now := time.Date(2026, 7, 4, 5, 0, 0, 0, time.UTC)
	repo := &fakeRepository{}
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	snapshot, err := svc.RecordSupplyQuality(context.Background(), SupplyQualityInput{
		SupplyType:        "supplier",
		Model:             "gpt-quality",
		AvailabilityRatio: 0.72,
		ErrorRatio:        0.25,
		CacheHitRatio:     0.2,
		PurityScore:       80,
		UsageTrustScore:   90,
		BalanceRiskScore:  10,
		ConcurrencyScore:  70,
	})
	if err != nil {
		t.Fatalf("RecordSupplyQuality() error = %v", err)
	}
	if snapshot.Decision != "paused" {
		t.Fatalf("Decision = %q, want paused", snapshot.Decision)
	}
	if len(repo.events) != 1 {
		t.Fatalf("events len = %d, want 1", len(repo.events))
	}
	event := repo.events[0]
	if event.EventType != "supply_quality_risk" || event.Severity != "warning" {
		t.Fatalf("event = %#v, want warning supply_quality_risk", event)
	}
}

func TestRecordAcceptanceReportCreatesRiskEvent(t *testing.T) {
	now := time.Date(2026, 7, 4, 6, 0, 0, 0, time.UTC)
	repo := &fakeRepository{}
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	report, err := svc.RecordAcceptanceReport(context.Background(), AcceptanceReportInput{
		SupplyType:          "supplier",
		SupplierID:          12,
		Model:               "gpt-acceptance",
		ConnectivityStatus:  "pass",
		ModelListStatus:     "pass",
		PurityStatus:        "pass",
		TrialCallStatus:     "pass",
		UsageMeteringStatus: "pass",
		CacheAuditStatus:    "fail",
		BalanceStatus:       "pass",
		ConcurrencyStatus:   "pass",
		FailureReason:       "round robin cache audit failed",
	})
	if err != nil {
		t.Fatalf("RecordAcceptanceReport() error = %v", err)
	}
	if report.Status != "blocked" {
		t.Fatalf("Status = %q, want blocked", report.Status)
	}
	if len(repo.events) != 1 {
		t.Fatalf("events len = %d, want 1", len(repo.events))
	}
	event := repo.events[0]
	if event.EventType != "acceptance_risk" || event.Severity != "critical" {
		t.Fatalf("event = %#v, want critical acceptance_risk", event)
	}
	if event.RelatedSnapshotType != "acceptance_report" || event.RelatedSnapshotID != report.ID {
		t.Fatalf("event related = %s/%d, want acceptance_report/%d", event.RelatedSnapshotType, event.RelatedSnapshotID, report.ID)
	}
}

func TestGenerateAcceptanceReportUsesQualityAndCacheEvidence(t *testing.T) {
	now := time.Date(2026, 7, 4, 6, 30, 0, 0, time.UTC)
	repo := &fakeRepository{
		quality: []*adminplusdomain.SupplyQualitySnapshot{
			{
				ID:                11,
				SupplyType:        "supplier",
				SupplierID:        7,
				Model:             "gpt-acceptance-auto",
				AvailabilityRatio: 0.72,
				ErrorRatio:        0.21,
				PurityScore:       40,
				UsageTrustScore:   90,
				BalanceRiskScore:  20,
				ConcurrencyScore:  85,
				ObservedAt:        now.Add(-time.Minute),
			},
		},
		cache: []*adminplusdomain.CacheEfficiencySnapshot{
			{
				ID:            12,
				SupplyType:    "supplier",
				SupplierID:    7,
				Model:         "gpt-acceptance-auto",
				CacheHitRatio: 0.12,
				Status:        "bad",
				ObservedAt:    now,
			},
		},
	}
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	report, err := svc.GenerateAcceptanceReport(context.Background(), AcceptanceReportGenerateInput{
		SupplyType: "supplier",
		SupplierID: 7,
		Model:      "gpt-acceptance-auto",
	})

	if err != nil {
		t.Fatalf("GenerateAcceptanceReport() error = %v", err)
	}
	if report.Status != "blocked" {
		t.Fatalf("Status = %q, want blocked", report.Status)
	}
	if report.ConnectivityStatus != "fail" || report.PurityStatus != "fail" || report.CacheAuditStatus != "fail" {
		t.Fatalf("report statuses = %#v, want failed connectivity/purity/cache", report)
	}
	if report.ReportPayload["quality_snapshot_id"] != int64(11) || report.ReportPayload["cache_snapshot_id"] != int64(12) {
		t.Fatalf("payload = %#v, want evidence snapshot ids", report.ReportPayload)
	}
	if len(repo.events) != 1 || repo.events[0].EventType != "acceptance_risk" {
		t.Fatalf("events = %#v, want acceptance_risk", repo.events)
	}
}

func TestGenerateAcceptanceReportCanEnqueueEvidenceTasks(t *testing.T) {
	now := time.Date(2026, 7, 4, 6, 45, 0, 0, time.UTC)
	repo := &fakeRepository{}
	scheduler := &fakeAcceptanceEvidenceScheduler{
		run: &adminplusdomain.SchedulerRunSummary{
			ID:          "kanban-acceptance-1",
			Status:      "queued",
			TaskType:    "multiple",
			TotalSteps:  5,
			RequestedAt: now,
		},
	}
	svc := NewServiceWithAllDependencies(repo, nil, scheduler)
	svc.now = func() time.Time { return now }

	report, err := svc.GenerateAcceptanceReport(context.Background(), AcceptanceReportGenerateInput{
		SupplyType:            "supplier",
		SupplierID:            7,
		LocalSub2APIAccountID: 19,
		Model:                 "gpt-acceptance-scheduled",
		EnqueueEvidenceTasks:  true,
	})

	if err != nil {
		t.Fatalf("GenerateAcceptanceReport() error = %v", err)
	}
	if len(scheduler.inputs) != 1 {
		t.Fatalf("scheduler inputs len = %d, want 1", len(scheduler.inputs))
	}
	if scheduler.inputs[0].SupplierID != 7 {
		t.Fatalf("scheduled supplier id = %d, want 7", scheduler.inputs[0].SupplierID)
	}
	wantTaskTypes := map[adminplusdomain.ExtensionTaskType]bool{
		adminplusdomain.ExtensionTaskTypeFetchHealth:     true,
		adminplusdomain.ExtensionTaskTypeCheckChannels:   true,
		adminplusdomain.ExtensionTaskTypeRunPurityCheck:  true,
		adminplusdomain.ExtensionTaskTypeFetchUsageCosts: true,
		adminplusdomain.ExtensionTaskTypeFetchBalance:    true,
	}
	for _, taskType := range scheduler.inputs[0].TaskTypes {
		delete(wantTaskTypes, taskType)
	}
	if len(wantTaskTypes) != 0 {
		t.Fatalf("scheduled task types missing = %#v", wantTaskTypes)
	}
	if scheduler.inputs[0].Request["model"] != "gpt-acceptance-scheduled" {
		t.Fatalf("scheduled request model = %#v, want gpt-acceptance-scheduled", scheduler.inputs[0].Request["model"])
	}
	if scheduler.inputs[0].Request["supply_type"] != "supplier" {
		t.Fatalf("scheduled request supply_type = %#v, want supplier", scheduler.inputs[0].Request["supply_type"])
	}
	if scheduler.inputs[0].Request["local_sub2api_account_id"] != int64(19) {
		t.Fatalf("scheduled request account id = %#v, want 19", scheduler.inputs[0].Request["local_sub2api_account_id"])
	}
	if report.ReportPayload["evidence_scheduler_run_id"] != "kanban-acceptance-1" {
		t.Fatalf("payload = %#v, want evidence scheduler run id", report.ReportPayload)
	}
}

func TestAcceptanceEvidenceRunObserverRecordsQualityCacheAndReport(t *testing.T) {
	now := time.Date(2026, 7, 4, 7, 0, 0, 0, time.UTC)
	finishedAt := now.Add(time.Minute)
	repo := &fakeRepository{}
	scheduler := &fakeAcceptanceEvidenceScheduler{
		detail: &adminplusdomain.SchedulerRunDetail{
			Run: adminplusdomain.SchedulerRunSummary{
				ID:          "kanban_acceptance-1",
				TriggerType: "kanban_acceptance",
				TaskType:    "mixed",
				Status:      "succeeded",
				RequestedAt: now,
				FinishedAt:  &finishedAt,
				TotalSteps:  5,
				RequestSnapshot: map[string]any{
					"model":                    "gpt-acceptance-run",
					"local_sub2api_account_id": int64(19),
				},
			},
			Steps: []adminplusdomain.SchedulerStepRecord{
				{ID: 1, RunID: "kanban_acceptance-1", SupplierID: 7, TaskType: adminplusdomain.ExtensionTaskTypeFetchHealth, Status: "succeeded"},
				{ID: 2, RunID: "kanban_acceptance-1", SupplierID: 7, TaskType: adminplusdomain.ExtensionTaskTypeCheckChannels, Status: "succeeded"},
				{
					ID:         3,
					RunID:      "kanban_acceptance-1",
					SupplierID: 7,
					TaskType:   adminplusdomain.ExtensionTaskTypeRunPurityCheck,
					Status:     "succeeded",
					ResultSnapshot: map[string]any{
						"report_id":                "purity-report-1",
						"model":                    "gpt-acceptance-run",
						"score":                    float64(92),
						"input_tokens":             int64(1000),
						"cached_tokens":            int64(100),
						"output_tokens":            int64(200),
						"token_audit_status":       "pass",
						"local_sub2api_account_id": int64(19),
					},
				},
				{ID: 4, RunID: "kanban_acceptance-1", SupplierID: 7, TaskType: adminplusdomain.ExtensionTaskTypeFetchUsageCosts, Status: "succeeded"},
				{ID: 5, RunID: "kanban_acceptance-1", SupplierID: 7, TaskType: adminplusdomain.ExtensionTaskTypeFetchBalance, Status: "succeeded"},
			},
		},
	}
	svc := NewServiceWithAllDependencies(repo, nil, scheduler)
	if scheduler.observer != svc {
		t.Fatalf("scheduler observer was not registered")
	}

	if err := scheduler.observer.OnSchedulerRunStatusRefreshed(context.Background(), "kanban_acceptance-1"); err != nil {
		t.Fatalf("OnSchedulerRunStatusRefreshed() error = %v", err)
	}
	if len(repo.quality) != 1 {
		t.Fatalf("quality snapshots len = %d, want 1", len(repo.quality))
	}
	if repo.quality[0].SupplierID != 7 || repo.quality[0].LocalSub2APIAccountID != 19 || repo.quality[0].Model != "gpt-acceptance-run" {
		t.Fatalf("quality target = %#v, want supplier/account/model from run", repo.quality[0])
	}
	if len(repo.cache) != 1 {
		t.Fatalf("cache snapshots len = %d, want 1", len(repo.cache))
	}
	if repo.cache[0].CacheHitRatio != 0.1 || repo.cache[0].Status != "bad" {
		t.Fatalf("cache snapshot = %#v, want 0.1 bad", repo.cache[0])
	}
	if len(repo.acceptance) != 1 {
		t.Fatalf("acceptance reports len = %d, want 1", len(repo.acceptance))
	}
	report := repo.acceptance[0]
	if report.PurityStatus != "pass" || report.UsageMeteringStatus != "pass" || report.CacheAuditStatus != "fail" {
		t.Fatalf("report statuses = %#v, want purity/usage pass and cache fail", report)
	}
	if report.Status != "blocked" {
		t.Fatalf("report status = %q, want blocked", report.Status)
	}
	if report.ReportPayload["evidence_scheduler_run_id"] != "kanban_acceptance-1" || report.ReportPayload["quality_snapshot_id"] != int64(1) || report.ReportPayload["cache_snapshot_id"] != int64(1) {
		t.Fatalf("report payload = %#v, want scheduler and snapshot evidence ids", report.ReportPayload)
	}
}

func TestServiceOverviewClassifiesUnprofitableMarketPrice(t *testing.T) {
	now := time.Date(2026, 7, 4, 2, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		market: []*adminplusdomain.MarketPriceSnapshot{
			{ID: 1, Model: "loss-model", Currency: "USD", PriceMicros: 1000000, ObservedAt: now},
		},
		cache: []*adminplusdomain.CacheEfficiencySnapshot{
			{ID: 2, Model: "loss-model", SupplyType: "supplier", CacheHitRatio: 0.8, Status: "healthy", ObservedAt: now},
		},
		costs: []*SupplierRateCost{
			{SupplierID: 11, Model: "loss-model", Currency: "USD", PriceMicros: 1500000, CapturedAt: now},
		},
	}
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	overview, err := svc.Overview(context.Background(), OverviewFilter{})
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if overview.UnprofitableModelCount != 1 {
		t.Fatalf("UnprofitableModelCount = %d, want 1", overview.UnprofitableModelCount)
	}
	row := overview.ModelMargins[0]
	if row.RiskLevel != "high" {
		t.Fatalf("RiskLevel = %q, want high", row.RiskLevel)
	}
	if row.GrossMarginPercent == nil || *row.GrossMarginPercent != -50 {
		t.Fatalf("GrossMarginPercent = %v, want -50", row.GrossMarginPercent)
	}
}

func TestServiceOverviewShowsMarginGapAndMarketPricePressure(t *testing.T) {
	now := time.Date(2026, 7, 4, 2, 30, 0, 0, time.UTC)
	repo := &fakeRepository{
		market: []*adminplusdomain.MarketPriceSnapshot{
			{ID: 1, Model: "thin-margin-model", Currency: "USD", PriceMicros: 2000000, ObservedAt: now},
		},
		cache: []*adminplusdomain.CacheEfficiencySnapshot{
			{ID: 2, Model: "thin-margin-model", SupplyType: "supplier", CacheHitRatio: 0.8, Status: "healthy", ObservedAt: now},
		},
		costs: []*SupplierRateCost{
			{SupplierID: 11, Model: "thin-margin-model", Currency: "USD", PriceMicros: 1800000, CapturedAt: now},
		},
	}
	svc := NewService(repo)
	svc.now = func() time.Time { return now }

	overview, err := svc.Overview(context.Background(), OverviewFilter{TargetMarginPercent: 25, RiskBufferPercent: 8})
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	row := overview.ModelMargins[0]
	if row.RiskLevel != "medium" {
		t.Fatalf("RiskLevel = %q, want medium", row.RiskLevel)
	}
	if row.GrossMarginPercent == nil || *row.GrossMarginPercent != 10 {
		t.Fatalf("GrossMarginPercent = %v, want 10", row.GrossMarginPercent)
	}
	if row.MarginGapPercent == nil || *row.MarginGapPercent > -14 || *row.MarginGapPercent < -16 {
		t.Fatalf("MarginGapPercent = %v, want about -14.8", row.MarginGapPercent)
	}
	if row.SuggestedVsMarketPercent == nil || *row.SuggestedVsMarketPercent < 19 || *row.SuggestedVsMarketPercent > 20 {
		t.Fatalf("SuggestedVsMarketPercent = %v, want about 19.7", row.SuggestedVsMarketPercent)
	}
}

type fakeRepository struct {
	market     []*adminplusdomain.MarketPriceSnapshot
	cache      []*adminplusdomain.CacheEfficiencySnapshot
	quality    []*adminplusdomain.SupplyQualitySnapshot
	acceptance []*adminplusdomain.AcceptanceReport
	costs      []*SupplierRateCost
	events     []*adminplusdomain.KanbanEvent
	derived    *UsageDerivedSnapshots
}

type fakeSiteCatalogReader struct {
	sites []*adminplusdomain.SiteCatalogSite
}

type fakeAcceptanceEvidenceScheduler struct {
	run      *adminplusdomain.SchedulerRunSummary
	detail   *adminplusdomain.SchedulerRunDetail
	observer schedulerapp.RunStatusObserver
	inputs   []schedulerapp.RunInput
}

func (s *fakeAcceptanceEvidenceScheduler) EnqueueRun(_ context.Context, in schedulerapp.RunInput) (*adminplusdomain.SchedulerRunSummary, error) {
	s.inputs = append(s.inputs, in)
	return s.run, nil
}

func (s *fakeAcceptanceEvidenceScheduler) GetRunDetail(_ context.Context, _ string) (*adminplusdomain.SchedulerRunDetail, error) {
	return s.detail, nil
}

func (s *fakeAcceptanceEvidenceScheduler) WithRunStatusObserver(observer schedulerapp.RunStatusObserver) *schedulerapp.Service {
	s.observer = observer
	return nil
}

func (r *fakeSiteCatalogReader) ListSites(_ context.Context, filter sitecatalogapp.SiteFilter) ([]*adminplusdomain.SiteCatalogSite, error) {
	out := make([]*adminplusdomain.SiteCatalogSite, 0, len(r.sites))
	for _, site := range r.sites {
		if site == nil {
			continue
		}
		if filter.Query != "" && !strings.Contains(site.Name, filter.Query) && !strings.Contains(site.CanonicalHost, filter.Query) && !strings.Contains(site.Slug, filter.Query) {
			continue
		}
		out = append(out, site)
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (r *fakeRepository) CreateMarketPriceSnapshot(_ context.Context, snapshot *adminplusdomain.MarketPriceSnapshot) (*adminplusdomain.MarketPriceSnapshot, error) {
	if snapshot.ID == 0 {
		snapshot.ID = int64(len(r.market) + 1)
	}
	r.market = append(r.market, snapshot)
	return snapshot, nil
}

func (r *fakeRepository) ListMarketPriceSnapshots(_ context.Context, filter MarketPriceFilter) ([]*adminplusdomain.MarketPriceSnapshot, error) {
	return filterMarketSnapshots(r.market, filter), nil
}

func (r *fakeRepository) CreateCacheEfficiencySnapshot(_ context.Context, snapshot *adminplusdomain.CacheEfficiencySnapshot) (*adminplusdomain.CacheEfficiencySnapshot, error) {
	if snapshot.ID == 0 {
		snapshot.ID = int64(len(r.cache) + 1)
	}
	r.cache = append(r.cache, snapshot)
	return snapshot, nil
}

func (r *fakeRepository) ListCacheEfficiencySnapshots(_ context.Context, filter CacheEfficiencyFilter) ([]*adminplusdomain.CacheEfficiencySnapshot, error) {
	return filterCacheSnapshots(r.cache, filter), nil
}

func (r *fakeRepository) CreateSupplyQualitySnapshot(_ context.Context, snapshot *adminplusdomain.SupplyQualitySnapshot) (*adminplusdomain.SupplyQualitySnapshot, error) {
	if snapshot.ID == 0 {
		snapshot.ID = int64(len(r.quality) + 1)
	}
	r.quality = append(r.quality, snapshot)
	return snapshot, nil
}

func (r *fakeRepository) ListSupplyQualitySnapshots(_ context.Context, filter SupplyQualityFilter) ([]*adminplusdomain.SupplyQualitySnapshot, error) {
	return filterQualitySnapshots(r.quality, filter), nil
}

func (r *fakeRepository) CreateAcceptanceReport(_ context.Context, report *adminplusdomain.AcceptanceReport) (*adminplusdomain.AcceptanceReport, error) {
	if report.ID == 0 {
		report.ID = int64(len(r.acceptance) + 1)
	}
	r.acceptance = append(r.acceptance, report)
	return report, nil
}

func (r *fakeRepository) ListAcceptanceReports(_ context.Context, filter AcceptanceReportFilter) ([]*adminplusdomain.AcceptanceReport, error) {
	return filterAcceptanceReports(r.acceptance, filter.Model), nil
}

func (r *fakeRepository) CreateKanbanEvent(_ context.Context, event *adminplusdomain.KanbanEvent) (*adminplusdomain.KanbanEvent, error) {
	if event.ID == 0 {
		event.ID = int64(len(r.events) + 1)
	}
	r.events = append(r.events, event)
	return event, nil
}

func (r *fakeRepository) ListKanbanEvents(_ context.Context, filter KanbanEventFilter) ([]*adminplusdomain.KanbanEvent, error) {
	out := make([]*adminplusdomain.KanbanEvent, 0, len(r.events))
	for _, item := range r.events {
		if item == nil {
			continue
		}
		if filter.Model != "" && item.Model != filter.Model {
			continue
		}
		if filter.EventType != "" && item.EventType != filter.EventType {
			continue
		}
		if filter.Severity != "" && item.Severity != filter.Severity {
			continue
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *fakeRepository) UpdateKanbanEventStatus(_ context.Context, id int64, status string) (*adminplusdomain.KanbanEvent, error) {
	for _, item := range r.events {
		if item != nil && item.ID == id {
			item.Status = status
			return item, nil
		}
	}
	return nil, nil
}

func (r *fakeRepository) ListSupplierRateCosts(_ context.Context, model string, _ int) ([]*SupplierRateCost, error) {
	out := make([]*SupplierRateCost, 0, len(r.costs))
	for _, item := range r.costs {
		if item != nil && (model == "" || item.Model == model) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (r *fakeRepository) ListUsageDerivedSnapshots(_ context.Context, filter UsageDerivedFilter) (*UsageDerivedSnapshots, error) {
	if r.derived == nil {
		return &UsageDerivedSnapshots{}, nil
	}
	out := &UsageDerivedSnapshots{
		Cache:   make([]*adminplusdomain.CacheEfficiencySnapshot, 0, len(r.derived.Cache)),
		Quality: make([]*adminplusdomain.SupplyQualitySnapshot, 0, len(r.derived.Quality)),
	}
	for _, item := range r.derived.Cache {
		if item == nil {
			continue
		}
		if filter.Model != "" && item.Model != filter.Model {
			continue
		}
		if filter.SupplyType != "" && item.SupplyType != filter.SupplyType {
			continue
		}
		if filter.SupplierID > 0 && item.SupplierID != filter.SupplierID {
			continue
		}
		if filter.LocalSub2APIAccountID > 0 && item.LocalSub2APIAccountID != filter.LocalSub2APIAccountID {
			continue
		}
		out.Cache = append(out.Cache, item)
	}
	for _, item := range r.derived.Quality {
		if item == nil {
			continue
		}
		if filter.Model != "" && item.Model != filter.Model {
			continue
		}
		if filter.SupplyType != "" && item.SupplyType != filter.SupplyType {
			continue
		}
		if filter.SupplierID > 0 && item.SupplierID != filter.SupplierID {
			continue
		}
		if filter.LocalSub2APIAccountID > 0 && item.LocalSub2APIAccountID != filter.LocalSub2APIAccountID {
			continue
		}
		out.Quality = append(out.Quality, item)
	}
	return out, nil
}

func filterMarketSnapshots(items []*adminplusdomain.MarketPriceSnapshot, filter MarketPriceFilter) []*adminplusdomain.MarketPriceSnapshot {
	out := make([]*adminplusdomain.MarketPriceSnapshot, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if filter.Model != "" && item.Model != filter.Model {
			continue
		}
		if filter.SourceType != "" && item.SourceType != filter.SourceType {
			continue
		}
		if filter.SiteID > 0 && item.SiteID != filter.SiteID {
			continue
		}
		if filter.SupplierID > 0 && item.SupplierID != filter.SupplierID {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterCacheSnapshots(items []*adminplusdomain.CacheEfficiencySnapshot, filter CacheEfficiencyFilter) []*adminplusdomain.CacheEfficiencySnapshot {
	out := make([]*adminplusdomain.CacheEfficiencySnapshot, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if filter.Model != "" && item.Model != filter.Model {
			continue
		}
		if filter.SupplyType != "" && item.SupplyType != filter.SupplyType {
			continue
		}
		if filter.SupplierID > 0 && item.SupplierID != filter.SupplierID {
			continue
		}
		if filter.LocalSub2APIAccountID > 0 && item.LocalSub2APIAccountID != filter.LocalSub2APIAccountID {
			continue
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterQualitySnapshots(items []*adminplusdomain.SupplyQualitySnapshot, filter SupplyQualityFilter) []*adminplusdomain.SupplyQualitySnapshot {
	out := make([]*adminplusdomain.SupplyQualitySnapshot, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if filter.Model != "" && item.Model != filter.Model {
			continue
		}
		if filter.SupplyType != "" && item.SupplyType != filter.SupplyType {
			continue
		}
		if filter.SupplierID > 0 && item.SupplierID != filter.SupplierID {
			continue
		}
		if filter.LocalSub2APIAccountID > 0 && item.LocalSub2APIAccountID != filter.LocalSub2APIAccountID {
			continue
		}
		if filter.Decision != "" && item.Decision != filter.Decision {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterAcceptanceReports(items []*adminplusdomain.AcceptanceReport, model string) []*adminplusdomain.AcceptanceReport {
	out := make([]*adminplusdomain.AcceptanceReport, 0, len(items))
	for _, item := range items {
		if item != nil && (model == "" || item.Model == model) {
			out = append(out, item)
		}
	}
	return out
}

func findAcceptanceStepSummary(items []adminplusdomain.AcceptanceStepSummary, step string) *adminplusdomain.AcceptanceStepSummary {
	for i := range items {
		if items[i].Step == step {
			return &items[i]
		}
	}
	return nil
}
