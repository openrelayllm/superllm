package adminplus

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	sitecatalogapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sitecatalog"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	defaultPublicProxyAIPageSize = 50
	maxPublicProxyAIPageSize     = 100
)

type PublicProxyAIHandler struct {
	service        *sitecatalogapp.Service
	settingService *service.SettingService
	now            func() time.Time
}

func NewPublicProxyAIHandler(service *sitecatalogapp.Service, settingService *service.SettingService) *PublicProxyAIHandler {
	return &PublicProxyAIHandler{service: service, settingService: settingService, now: time.Now}
}

type publicProxyAISummary struct {
	SiteCount      int        `json:"site_count"`
	AvailableCount int        `json:"available_count"`
	LowestRate     *float64   `json:"lowest_rate"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
	GeneratedAt    time.Time  `json:"generated_at"`
}

type publicProxyAISite struct {
	Slug                 string                                         `json:"slug"`
	CanonicalHost        string                                         `json:"canonical_host"`
	Name                 string                                         `json:"name"`
	ShortName            string                                         `json:"short_name,omitempty"`
	Summary              string                                         `json:"summary,omitempty"`
	Description          string                                         `json:"description,omitempty"`
	ProviderType         adminplusdomain.SupplierType                   `json:"provider_type,omitempty"`
	SiteKind             adminplusdomain.SiteCatalogKind                `json:"site_kind"`
	RecommendationLevel  adminplusdomain.SiteCatalogRecommendationLevel `json:"recommendation_level"`
	RecommendationReason string                                         `json:"recommendation_reason,omitempty"`
	RiskLevel            adminplusdomain.SiteCatalogRiskLevel           `json:"risk_level"`
	LogoURL              string                                         `json:"logo_url,omitempty"`
	ScreenshotURL        string                                         `json:"screenshot_url,omitempty"`
	PrimaryLanguage      string                                         `json:"primary_language,omitempty"`
	CountryOrRegion      string                                         `json:"country_or_region,omitempty"`
	Categories           []publicProxyAICategory                        `json:"categories,omitempty"`
	Tags                 []publicProxyAITag                             `json:"tags,omitempty"`
	Links                []publicProxyAILink                            `json:"links,omitempty"`
	Metrics              publicProxyAIMetrics                           `json:"metrics"`
	DataNotes            []string                                       `json:"data_notes,omitempty"`
	PublishedAt          *time.Time                                     `json:"published_at,omitempty"`
	UpdatedAt            time.Time                                      `json:"updated_at"`
}

type publicProxyAICategory struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type publicProxyAITag struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	TagType string `json:"tag_type"`
	Color   string `json:"color,omitempty"`
}

type publicProxyAILink struct {
	LinkType  adminplusdomain.SiteCatalogLinkType   `json:"link_type"`
	URL       string                                `json:"url"`
	Label     string                                `json:"label,omitempty"`
	IsPrimary bool                                  `json:"is_primary"`
	Status    adminplusdomain.SiteCatalogLinkStatus `json:"status"`
}

type publicProxyAIMetrics struct {
	LowestRate       *float64   `json:"lowest_rate,omitempty"`
	RateGroupName    string     `json:"rate_group_name,omitempty"`
	RateUpdatedAt    *time.Time `json:"rate_updated_at,omitempty"`
	Available        *bool      `json:"available,omitempty"`
	Uptime24H        *float64   `json:"uptime_24h,omitempty"`
	AvgResponseMS    *int       `json:"avg_response_ms,omitempty"`
	LatestResponseMS *int       `json:"latest_response_ms,omitempty"`
	FirstTokenMS     *int       `json:"first_token_ms,omitempty"`
	CheckedAt        *time.Time `json:"checked_at,omitempty"`
}

type publicProxyAIPage struct {
	Items       []*publicProxyAISite `json:"items"`
	Total       int                  `json:"total"`
	Page        int                  `json:"page"`
	PageSize    int                  `json:"page_size"`
	Pages       int                  `json:"pages"`
	GeneratedAt time.Time            `json:"generated_at"`
}

type publicProxyAIRuntimeConfig struct {
	TurnstileSiteKey           string `json:"turnstile_site_key"`
	WebPurityRequiresTurnstile bool   `json:"web_purity_requires_turnstile"`
}

func (h *PublicProxyAIHandler) RuntimeConfig(c *gin.Context) {
	if h == nil || h.settingService == nil {
		response.Success(c, publicProxyAIRuntimeConfig{})
		return
	}
	config, err := h.settingService.GetProxyAIPurityTurnstileConfig(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, publicProxyAIRuntimeConfig{
		TurnstileSiteKey:           config.SiteKey,
		WebPurityRequiresTurnstile: config.Enabled,
	})
}

func (h *PublicProxyAIHandler) Summary(c *gin.Context) {
	sites, ok := h.publicSites(c)
	if !ok {
		return
	}
	summary := publicProxyAISummary{SiteCount: len(sites), GeneratedAt: h.generatedAt()}
	for _, site := range sites {
		if site == nil {
			continue
		}
		publicSite := projectPublicProxyAISite(site, false)
		if publicSite.Metrics.Available != nil && *publicSite.Metrics.Available {
			summary.AvailableCount++
		}
		if publicSite.Metrics.LowestRate != nil && (summary.LowestRate == nil || *publicSite.Metrics.LowestRate < *summary.LowestRate) {
			value := *publicSite.Metrics.LowestRate
			summary.LowestRate = &value
		}
		if summary.UpdatedAt == nil || site.UpdatedAt.After(*summary.UpdatedAt) {
			updated := site.UpdatedAt
			summary.UpdatedAt = &updated
		}
	}
	response.Success(c, summary)
}

func (h *PublicProxyAIHandler) ListSites(c *gin.Context) {
	sites, ok := h.publicSites(c)
	if !ok {
		return
	}
	items := make([]*publicProxyAISite, 0, len(sites))
	for _, site := range sites {
		if site == nil || !matchesPublicProxyAIFilters(c, site) {
			continue
		}
		items = append(items, projectPublicProxyAISite(site, false))
	}
	sortPublicProxyAISites(items, strings.TrimSpace(c.Query("sort")))
	page, pageSize := publicProxyAIPagination(c)
	total := len(items)
	response.Success(c, publicProxyAIPage{
		Items:       paginatePublicProxyAISites(items, page, pageSize),
		Total:       total,
		Page:        page,
		PageSize:    pageSize,
		Pages:       pageCount(total, pageSize),
		GeneratedAt: h.generatedAt(),
	})
}

func (h *PublicProxyAIHandler) GetSite(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		response.Error(c, http.StatusBadRequest, "site slug is required")
		return
	}
	sites, ok := h.publicSites(c)
	if !ok {
		return
	}
	for _, site := range sites {
		if site != nil && site.Slug == slug {
			response.Success(c, projectPublicProxyAISite(site, true))
			return
		}
	}
	response.Error(c, http.StatusNotFound, "site not found")
}

func (h *PublicProxyAIHandler) publicSites(c *gin.Context) ([]*adminplusdomain.SiteCatalogSite, bool) {
	if h == nil || h.service == nil {
		response.Error(c, http.StatusInternalServerError, "public proxyai service is not configured")
		return nil, false
	}
	items, err := h.service.ListSites(c.Request.Context(), sitecatalogapp.SiteFilter{
		Query:    c.Query("q"),
		Status:   adminplusdomain.SiteCatalogStatusPublished,
		SiteKind: adminplusdomain.SiteCatalogKind(strings.TrimSpace(c.Query("site_kind"))),
		Provider: normalizeDiscoveryProviderType(c.Query("provider_type")),
		Limit:    1000,
	})
	if response.ErrorFrom(c, err) {
		return nil, false
	}
	public := make([]*adminplusdomain.SiteCatalogSite, 0, len(items))
	for _, item := range items {
		if isPublicProxyAISite(item) {
			public = append(public, item)
		}
	}
	return public, true
}

func (h *PublicProxyAIHandler) generatedAt() time.Time {
	if h == nil || h.now == nil {
		return time.Now().UTC()
	}
	return h.now().UTC()
}

func isPublicProxyAISite(site *adminplusdomain.SiteCatalogSite) bool {
	if site == nil {
		return false
	}
	return site.Status == adminplusdomain.SiteCatalogStatusPublished &&
		site.Visibility == adminplusdomain.SiteCatalogVisibilityPublic &&
		site.QualityStatus != adminplusdomain.SiteCatalogQualityDuplicate
}

func matchesPublicProxyAIFilters(c *gin.Context, site *adminplusdomain.SiteCatalogSite) bool {
	recommendation := strings.TrimSpace(c.Query("recommendation"))
	if recommendation != "" && string(site.RecommendationLevel) != recommendation {
		return false
	}
	risk := strings.TrimSpace(c.Query("risk"))
	if risk != "" && string(site.RiskLevel) != risk {
		return false
	}
	return true
}

func projectPublicProxyAISite(site *adminplusdomain.SiteCatalogSite, includeDetail bool) *publicProxyAISite {
	out := &publicProxyAISite{
		Slug:                 site.Slug,
		CanonicalHost:        site.CanonicalHost,
		Name:                 site.Name,
		ShortName:            site.ShortName,
		Summary:              site.Summary,
		ProviderType:         site.ProviderType,
		SiteKind:             site.SiteKind,
		RecommendationLevel:  site.RecommendationLevel,
		RecommendationReason: site.RecommendationReason,
		RiskLevel:            site.RiskLevel,
		LogoURL:              site.LogoURL,
		PrimaryLanguage:      site.PrimaryLanguage,
		CountryOrRegion:      site.CountryOrRegion,
		Categories:           projectPublicProxyAICategories(site.Categories),
		Tags:                 projectPublicProxyAITags(site.Tags),
		Links:                projectPublicProxyAILinks(site.Links),
		Metrics:              publicProxyAIMetricsFromMetadata(publicProxyAIMergedMetadata(site)),
		PublishedAt:          site.PublishedAt,
		UpdatedAt:            site.UpdatedAt,
	}
	if includeDetail {
		out.Description = site.Description
		out.ScreenshotURL = site.ScreenshotURL
		out.DataNotes = publicProxyAIDataNotes(out.Metrics)
	}
	return out
}

func projectPublicProxyAICategories(categories []*adminplusdomain.SiteCatalogCategory) []publicProxyAICategory {
	out := make([]publicProxyAICategory, 0, len(categories))
	for _, category := range categories {
		if category == nil || !category.Enabled {
			continue
		}
		out = append(out, publicProxyAICategory{Slug: category.Slug, Name: category.Name})
	}
	return out
}

func projectPublicProxyAITags(tags []*adminplusdomain.SiteCatalogTag) []publicProxyAITag {
	out := make([]publicProxyAITag, 0, len(tags))
	for _, tag := range tags {
		if tag == nil || !tag.Enabled {
			continue
		}
		out = append(out, publicProxyAITag{Slug: tag.Slug, Name: tag.Name, TagType: tag.TagType, Color: tag.Color})
	}
	return out
}

func projectPublicProxyAILinks(links []*adminplusdomain.SiteCatalogLink) []publicProxyAILink {
	out := make([]publicProxyAILink, 0, len(links))
	for _, link := range links {
		if link == nil || strings.TrimSpace(link.URL) == "" || !publicProxyAILinkTypeAllowed(link.LinkType) {
			continue
		}
		out = append(out, publicProxyAILink{
			LinkType:  link.LinkType,
			URL:       link.URL,
			Label:     link.Label,
			IsPrimary: link.IsPrimary,
			Status:    link.Status,
		})
	}
	return out
}

func publicProxyAILinkTypeAllowed(linkType adminplusdomain.SiteCatalogLinkType) bool {
	switch linkType {
	case adminplusdomain.SiteCatalogLinkHomepage,
		adminplusdomain.SiteCatalogLinkRegister,
		adminplusdomain.SiteCatalogLinkAPIBase,
		adminplusdomain.SiteCatalogLinkRecharge,
		adminplusdomain.SiteCatalogLinkDocs,
		adminplusdomain.SiteCatalogLinkContact:
		return true
	default:
		return false
	}
}

func publicProxyAIMergedMetadata(site *adminplusdomain.SiteCatalogSite) map[string]any {
	if site == nil {
		return nil
	}
	out := make(map[string]any, len(site.Metadata))
	for key, value := range site.Metadata {
		out[key] = value
	}
	for i := len(site.Sources) - 1; i >= 0; i-- {
		source := site.Sources[i]
		if source == nil {
			continue
		}
		for key, value := range source.ObservedPayload {
			out[key] = value
		}
	}
	return out
}

func publicProxyAIMetricsFromMetadata(metadata map[string]any) publicProxyAIMetrics {
	uptime := metadataFloat(metadata, "success_rate_24h", "uptime_24h", "monitor_uptime_percent")
	return publicProxyAIMetrics{
		LowestRate:       metadataFloat(metadata, "min_rate", "lowest_rate", "rate", "price_multiplier"),
		RateGroupName:    metadataString(metadata, "plan_types", "rate_group_name", "group_name", "plan_type"),
		RateUpdatedAt:    metadataTime(metadata, "latest_checked_at", "rate_updated_at"),
		Available:        publicProxyAIAvailableFromMetadata(metadata),
		Uptime24H:        publicProxyAINormalizedPercent(uptime),
		AvgResponseMS:    metadataInt(metadata, "avg_response_ms", "monitor_average_response_ms", "monitor_avg_response_ms"),
		LatestResponseMS: metadataInt(metadata, "latest_response_ms", "monitor_latest_response_ms"),
		FirstTokenMS:     metadataInt(metadata, "first_token_ms", "firstTokenMs", "first_token_latency_ms", "firstTokenLatencyMs"),
		CheckedAt:        metadataTime(metadata, "checked_at", "latest_checked_at", "monitor_checked_at"),
	}
}

func publicProxyAIAvailableFromMetadata(metadata map[string]any) *bool {
	if availableCount := metadataFloat(metadata, "available_count"); availableCount != nil {
		next := *availableCount > 0
		return &next
	}
	return metadataBool(metadata, "available", "monitor_available")
}

func publicProxyAINormalizedPercent(value *float64) *float64 {
	if value == nil {
		return nil
	}
	next := *value
	if next >= 0 && next <= 1 {
		next *= 100
	}
	return &next
}

func metadataString(metadata map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case []string:
			if len(typed) > 0 {
				return strings.Join(typed, "、")
			}
		case []any:
			parts := make([]string, 0, len(typed))
			for _, item := range typed {
				if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
					parts = append(parts, strings.TrimSpace(text))
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "、")
			}
		}
	}
	return ""
}

func metadataFloat(metadata map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return &typed
		case float32:
			next := float64(typed)
			return &next
		case int:
			next := float64(typed)
			return &next
		case int64:
			next := float64(typed)
			return &next
		case string:
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
				return &parsed
			}
		}
	}
	return nil
}

func metadataInt(metadata map[string]any, keys ...string) *int {
	for _, key := range keys {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case int:
			return &typed
		case int64:
			next := int(typed)
			return &next
		case float64:
			next := int(typed)
			return &next
		case float32:
			next := int(typed)
			return &next
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
				return &parsed
			}
		}
	}
	return nil
}

func metadataBool(metadata map[string]any, keys ...string) *bool {
	for _, key := range keys {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return &typed
		case string:
			normalized := strings.ToLower(strings.TrimSpace(typed))
			if normalized == "true" || normalized == "1" || normalized == "yes" {
				next := true
				return &next
			}
			if normalized == "false" || normalized == "0" || normalized == "no" {
				next := false
				return &next
			}
		}
	}
	return nil
}

func metadataTime(metadata map[string]any, keys ...string) *time.Time {
	for _, key := range keys {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case time.Time:
			next := typed.UTC()
			return &next
		case string:
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
				if parsed, err := time.Parse(layout, strings.TrimSpace(typed)); err == nil {
					next := parsed.UTC()
					return &next
				}
			}
		}
	}
	return nil
}

func publicProxyAIDataNotes(metrics publicProxyAIMetrics) []string {
	notes := []string{"倍率可能由人工维护", "监测数据来自系统采样"}
	if metrics.LowestRate == nil {
		notes = append(notes, "暂无公开倍率")
	}
	if metrics.Available == nil {
		notes = append(notes, "暂无公开监测")
	}
	return notes
}

func publicProxyAIPagination(c *gin.Context) (int, int) {
	page := parsePublicProxyAIPositiveInt(c.Query("page"), 1)
	pageSize := parsePublicProxyAIPositiveInt(c.Query("page_size"), defaultPublicProxyAIPageSize)
	if pageSize > maxPublicProxyAIPageSize {
		pageSize = maxPublicProxyAIPageSize
	}
	return page, pageSize
}

func parsePublicProxyAIPositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func paginatePublicProxyAISites(items []*publicProxyAISite, page int, pageSize int) []*publicProxyAISite {
	offset := (page - 1) * pageSize
	if offset >= len(items) {
		return []*publicProxyAISite{}
	}
	end := offset + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

func pageCount(total int, pageSize int) int {
	if total <= 0 || pageSize <= 0 {
		return 0
	}
	pages := total / pageSize
	if total%pageSize != 0 {
		pages++
	}
	return pages
}

func sortPublicProxyAISites(items []*publicProxyAISite, sortKey string) {
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		switch sortKey {
		case "rate_asc":
			return optionalFloatLess(left.Metrics.LowestRate, right.Metrics.LowestRate)
		case "availability_desc":
			return optionalFloatGreater(left.Metrics.Uptime24H, right.Metrics.Uptime24H)
		case "first_token_asc":
			return optionalIntLess(left.Metrics.FirstTokenMS, right.Metrics.FirstTokenMS)
		case "updated_desc":
			return left.UpdatedAt.After(right.UpdatedAt)
		default:
			return publicProxyAIDefaultLess(left, right)
		}
	})
}

func publicProxyAIDefaultLess(left *publicProxyAISite, right *publicProxyAISite) bool {
	if left.RecommendationLevel != right.RecommendationLevel {
		return recommendationRank(left.RecommendationLevel) > recommendationRank(right.RecommendationLevel)
	}
	leftAvailable := left.Metrics.Available != nil && *left.Metrics.Available
	rightAvailable := right.Metrics.Available != nil && *right.Metrics.Available
	if leftAvailable != rightAvailable {
		return leftAvailable
	}
	if left.Metrics.LowestRate != nil || right.Metrics.LowestRate != nil {
		return optionalFloatLess(left.Metrics.LowestRate, right.Metrics.LowestRate)
	}
	return left.UpdatedAt.After(right.UpdatedAt)
}

func recommendationRank(level adminplusdomain.SiteCatalogRecommendationLevel) int {
	switch level {
	case adminplusdomain.SiteCatalogRecommendationFeatured:
		return 3
	case adminplusdomain.SiteCatalogRecommendationNormal:
		return 2
	case adminplusdomain.SiteCatalogRecommendationNone:
		return 1
	default:
		return 0
	}
}

func optionalFloatLess(left *float64, right *float64) bool {
	if left == nil {
		return false
	}
	if right == nil {
		return true
	}
	return *left < *right
}

func optionalFloatGreater(left *float64, right *float64) bool {
	if left == nil {
		return false
	}
	if right == nil {
		return true
	}
	return *left > *right
}

func optionalIntLess(left *int, right *int) bool {
	if left == nil {
		return false
	}
	if right == nil {
		return true
	}
	return *left < *right
}
