package sitecatalog

import (
	"context"
	"net/url"
	"regexp"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

const defaultListLimit = 1000

type Repository interface {
	ListSites(ctx context.Context, filter SiteFilter) ([]*adminplusdomain.SiteCatalogSite, error)
	GetSite(ctx context.Context, id int64) (*adminplusdomain.SiteCatalogSite, error)
	CreateSite(ctx context.Context, site *adminplusdomain.SiteCatalogSite) (*adminplusdomain.SiteCatalogSite, error)
	DeleteSite(ctx context.Context, id int64) error
	BulkPublishSites(ctx context.Context, input BulkPublishSitesInput, publishedAt time.Time) (int64, error)
	AddDiscoveryCandidate(ctx context.Context, candidate *adminplusdomain.SiteDiscoveryItem, input AddDiscoveryCandidateInput) (*adminplusdomain.SiteCatalogSite, error)
	ListCategories(ctx context.Context) ([]*adminplusdomain.SiteCatalogCategory, error)
	ListTags(ctx context.Context) ([]*adminplusdomain.SiteCatalogTag, error)
	SlugExists(ctx context.Context, slug string) (bool, error)
}

type Service struct {
	repo Repository
	now  func() time.Time
}

type SiteFilter struct {
	Query    string
	Status   adminplusdomain.SiteCatalogStatus
	SiteKind adminplusdomain.SiteCatalogKind
	Provider adminplusdomain.SupplierType
	Limit    int
}

type SiteLinkInput struct {
	LinkType  adminplusdomain.SiteCatalogLinkType `json:"link_type"`
	URL       string                              `json:"url"`
	Label     string                              `json:"label"`
	IsPrimary bool                                `json:"is_primary"`
}

type CreateSiteInput struct {
	Slug                 string                                         `json:"slug"`
	CanonicalHost        string                                         `json:"canonical_host"`
	Name                 string                                         `json:"name"`
	ShortName            string                                         `json:"short_name"`
	Summary              string                                         `json:"summary"`
	Description          string                                         `json:"description"`
	ProviderType         adminplusdomain.SupplierType                   `json:"provider_type"`
	SiteKind             adminplusdomain.SiteCatalogKind                `json:"site_kind"`
	Status               adminplusdomain.SiteCatalogStatus              `json:"status"`
	Visibility           adminplusdomain.SiteCatalogVisibility          `json:"visibility"`
	RecommendationLevel  adminplusdomain.SiteCatalogRecommendationLevel `json:"recommendation_level"`
	RecommendationReason string                                         `json:"recommendation_reason"`
	RiskLevel            adminplusdomain.SiteCatalogRiskLevel           `json:"risk_level"`
	LogoURL              string                                         `json:"logo_url"`
	ScreenshotURL        string                                         `json:"screenshot_url"`
	PrimaryLanguage      string                                         `json:"primary_language"`
	CountryOrRegion      string                                         `json:"country_or_region"`
	SupplierID           int64                                          `json:"supplier_id"`
	Metadata             map[string]any                                 `json:"metadata"`
	Links                []SiteLinkInput                                `json:"links"`
	CategoryIDs          []int64                                        `json:"category_ids"`
	TagIDs               []int64                                        `json:"tag_ids"`
}

type AddDiscoveryCandidateInput struct {
	SiteID               int64                                          `json:"site_id"`
	Slug                 string                                         `json:"slug"`
	Name                 string                                         `json:"name"`
	Summary              string                                         `json:"summary"`
	Description          string                                         `json:"description"`
	SiteKind             adminplusdomain.SiteCatalogKind                `json:"site_kind"`
	Status               adminplusdomain.SiteCatalogStatus              `json:"status"`
	Visibility           adminplusdomain.SiteCatalogVisibility          `json:"visibility"`
	RecommendationLevel  adminplusdomain.SiteCatalogRecommendationLevel `json:"recommendation_level"`
	RecommendationReason string                                         `json:"recommendation_reason"`
	RiskLevel            adminplusdomain.SiteCatalogRiskLevel           `json:"risk_level"`
	CategoryIDs          []int64                                        `json:"category_ids"`
	TagIDs               []int64                                        `json:"tag_ids"`
	Links                []SiteLinkInput                                `json:"links"`
}

type BulkAddDiscoveryCandidatesInput struct {
	SiteKind             adminplusdomain.SiteCatalogKind                `json:"site_kind"`
	Status               adminplusdomain.SiteCatalogStatus              `json:"status"`
	Visibility           adminplusdomain.SiteCatalogVisibility          `json:"visibility"`
	RecommendationLevel  adminplusdomain.SiteCatalogRecommendationLevel `json:"recommendation_level"`
	RecommendationReason string                                         `json:"recommendation_reason"`
	RiskLevel            adminplusdomain.SiteCatalogRiskLevel           `json:"risk_level"`
	CategoryIDs          []int64                                        `json:"category_ids"`
	TagIDs               []int64                                        `json:"tag_ids"`
	IncludeUnsupported   bool                                           `json:"include_unsupported"`
}

type BulkAddDiscoveryCandidatesResult struct {
	Total   int                                `json:"total"`
	Created int                                `json:"created"`
	Skipped int                                `json:"skipped"`
	Failed  int                                `json:"failed"`
	Sites   []*adminplusdomain.SiteCatalogSite `json:"sites,omitempty"`
	Errors  []BulkAddDiscoveryCandidateError   `json:"errors,omitempty"`
}

type BulkAddDiscoveryCandidateError struct {
	DiscoveryID int64  `json:"discovery_id"`
	Name        string `json:"name"`
	Error       string `json:"error"`
}

type BulkPublishSitesInput struct {
	IDs      []int64                           `json:"ids"`
	Query    string                            `json:"q"`
	Status   adminplusdomain.SiteCatalogStatus `json:"status"`
	SiteKind adminplusdomain.SiteCatalogKind   `json:"site_kind"`
	Provider adminplusdomain.SupplierType      `json:"provider_type"`
}

type BulkPublishSitesResult struct {
	Total   int   `json:"total"`
	Updated int64 `json:"updated"`
	Skipped int   `json:"skipped"`
}

type BulkAddDiscoveryCandidateProgressEvent struct {
	Type    string                            `json:"type"`
	Level   string                            `json:"level,omitempty"`
	Message string                            `json:"message"`
	Current int                               `json:"current,omitempty"`
	Total   int                               `json:"total,omitempty"`
	Item    *adminplusdomain.SiteCatalogSite  `json:"item,omitempty"`
	Result  *BulkAddDiscoveryCandidatesResult `json:"result,omitempty"`
}

type BulkAddDiscoveryCandidateEmitter func(BulkAddDiscoveryCandidateProgressEvent)

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

func (s *Service) ListSites(ctx context.Context, filter SiteFilter) ([]*adminplusdomain.SiteCatalogSite, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("site catalog service is not configured")
	}
	if filter.Limit <= 0 {
		filter.Limit = defaultListLimit
	}
	filter.Query = strings.TrimSpace(filter.Query)
	return s.repo.ListSites(ctx, filter)
}

func (s *Service) GetSite(ctx context.Context, id int64) (*adminplusdomain.SiteCatalogSite, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("site catalog service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("SITE_CATALOG_SITE_ID_INVALID", "invalid site id")
	}
	return s.repo.GetSite(ctx, id)
}

func (s *Service) CreateSite(ctx context.Context, in CreateSiteInput) (*adminplusdomain.SiteCatalogSite, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("site catalog service is not configured")
	}
	site, err := s.siteFromInput(ctx, in)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateSite(ctx, site)
}

func (s *Service) DeleteSite(ctx context.Context, id int64) error {
	if s == nil || s.repo == nil {
		return internalError("site catalog service is not configured")
	}
	if id <= 0 {
		return badRequest("SITE_CATALOG_SITE_ID_INVALID", "invalid site id")
	}
	return s.repo.DeleteSite(ctx, id)
}

func (s *Service) BulkPublishSites(ctx context.Context, in BulkPublishSitesInput) (*BulkPublishSitesResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("site catalog service is not configured")
	}
	in.IDs = uniquePositiveIDs(in.IDs)
	in.Query = strings.TrimSpace(in.Query)
	updated, err := s.repo.BulkPublishSites(ctx, in, s.now().UTC())
	if err != nil {
		return nil, err
	}
	return &BulkPublishSitesResult{
		Total:   int(updated),
		Updated: updated,
	}, nil
}

func (s *Service) AddDiscoveryCandidate(ctx context.Context, candidate *adminplusdomain.SiteDiscoveryItem, in AddDiscoveryCandidateInput) (*adminplusdomain.SiteCatalogSite, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("site catalog service is not configured")
	}
	if candidate == nil || candidate.ID <= 0 {
		return nil, badRequest("SITE_CATALOG_DISCOVERY_CANDIDATE_REQUIRED", "discovery candidate is required")
	}
	in.Name = firstNonEmpty(in.Name, candidate.Name, candidate.Host)
	in.Summary = firstNonEmpty(in.Summary, trimLimit(candidate.Description, 160))
	in.Description = firstNonEmpty(in.Description, candidate.Description)
	if in.SiteKind == "" {
		in.SiteKind = siteKindFromDiscoverySection(candidate.SourceSection)
	}
	if in.Status == "" {
		in.Status = adminplusdomain.SiteCatalogStatusDraft
	}
	if in.Visibility == "" {
		in.Visibility = adminplusdomain.SiteCatalogVisibilityPublic
	}
	if in.RecommendationLevel == "" {
		in.RecommendationLevel = adminplusdomain.SiteCatalogRecommendationNone
	}
	if in.RiskLevel == "" {
		in.RiskLevel = adminplusdomain.SiteCatalogRiskUnknown
	}
	if in.Slug == "" {
		slug, err := s.uniqueSlug(ctx, firstNonEmpty(candidate.Host, candidate.Name))
		if err != nil {
			return nil, err
		}
		in.Slug = slug
	}
	if len(in.Links) == 0 {
		in.Links = discoveryCandidateLinks(candidate)
	}
	return s.repo.AddDiscoveryCandidate(ctx, candidate, in)
}

func (s *Service) BulkAddDiscoveryCandidates(ctx context.Context, candidates []*adminplusdomain.SiteDiscoveryItem, in BulkAddDiscoveryCandidatesInput, emit BulkAddDiscoveryCandidateEmitter) (*BulkAddDiscoveryCandidatesResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("site catalog service is not configured")
	}
	result := &BulkAddDiscoveryCandidatesResult{
		Total: len(candidates),
		Sites: make([]*adminplusdomain.SiteCatalogSite, 0, len(candidates)),
	}
	emitBulkAddProgress(emit, BulkAddDiscoveryCandidateProgressEvent{
		Type:    "started",
		Level:   "info",
		Message: "批量加入目录已开始，共 " + stringFromInt64(int64(len(candidates))) + " 个候选",
		Total:   len(candidates),
	})
	for i, candidate := range candidates {
		current := i + 1
		if candidate == nil || candidate.ID <= 0 {
			result.Skipped++
			emitBulkAddProgress(emit, BulkAddDiscoveryCandidateProgressEvent{Type: "item_skipped", Level: "warning", Message: "候选为空，已跳过", Current: current, Total: len(candidates)})
			continue
		}
		name := firstNonEmpty(candidate.Name, candidate.Host, candidate.RegisterURL)
		if candidate.CatalogSiteID > 0 || candidate.ProcessStatus == adminplusdomain.SiteDiscoveryProcessAddedToCatalog {
			result.Skipped++
			emitBulkAddProgress(emit, BulkAddDiscoveryCandidateProgressEvent{Type: "item_skipped", Level: "warning", Message: "已在目录中，跳过：" + name, Current: current, Total: len(candidates)})
			continue
		}
		if !in.IncludeUnsupported && (candidate.ClassificationStatus != adminplusdomain.SiteDiscoveryClassificationSupported || (candidate.ProviderType != adminplusdomain.SupplierTypeNewAPI && candidate.ProviderType != adminplusdomain.SupplierTypeSub2API)) {
			result.Skipped++
			emitBulkAddProgress(emit, BulkAddDiscoveryCandidateProgressEvent{Type: "item_skipped", Level: "warning", Message: "未识别为支持类型，跳过：" + name, Current: current, Total: len(candidates)})
			continue
		}
		site, err := s.AddDiscoveryCandidate(ctx, candidate, AddDiscoveryCandidateInput{
			SiteKind:             in.SiteKind,
			Status:               in.Status,
			Visibility:           in.Visibility,
			RecommendationLevel:  in.RecommendationLevel,
			RecommendationReason: in.RecommendationReason,
			RiskLevel:            in.RiskLevel,
			CategoryIDs:          in.CategoryIDs,
			TagIDs:               in.TagIDs,
		})
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BulkAddDiscoveryCandidateError{DiscoveryID: candidate.ID, Name: name, Error: err.Error()})
			emitBulkAddProgress(emit, BulkAddDiscoveryCandidateProgressEvent{Type: "item_failed", Level: "error", Message: "加入失败：" + name + " - " + err.Error(), Current: current, Total: len(candidates)})
			continue
		}
		result.Created++
		result.Sites = append(result.Sites, site)
		emitBulkAddProgress(emit, BulkAddDiscoveryCandidateProgressEvent{Type: "item_success", Level: "success", Message: "已加入目录：" + name, Current: current, Total: len(candidates), Item: site})
	}
	level := "success"
	if result.Failed > 0 {
		level = "warning"
	}
	emitBulkAddProgress(emit, BulkAddDiscoveryCandidateProgressEvent{
		Type:    "completed",
		Level:   level,
		Message: "批量加入完成：新增 " + stringFromInt64(int64(result.Created)) + "，跳过 " + stringFromInt64(int64(result.Skipped)) + "，失败 " + stringFromInt64(int64(result.Failed)),
		Current: result.Total,
		Total:   result.Total,
		Result:  result,
	})
	return result, nil
}

func emitBulkAddProgress(emit BulkAddDiscoveryCandidateEmitter, event BulkAddDiscoveryCandidateProgressEvent) {
	if emit == nil {
		return
	}
	emit(event)
}

func (s *Service) ListCategories(ctx context.Context) ([]*adminplusdomain.SiteCatalogCategory, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("site catalog service is not configured")
	}
	return s.repo.ListCategories(ctx)
}

func (s *Service) ListTags(ctx context.Context) ([]*adminplusdomain.SiteCatalogTag, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("site catalog service is not configured")
	}
	return s.repo.ListTags(ctx)
}

func (s *Service) siteFromInput(ctx context.Context, in CreateSiteInput) (*adminplusdomain.SiteCatalogSite, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, badRequest("SITE_CATALOG_NAME_REQUIRED", "site name is required")
	}
	if in.SiteKind == "" {
		in.SiteKind = adminplusdomain.SiteCatalogKindAPIRelay
	}
	if in.Status == "" {
		in.Status = adminplusdomain.SiteCatalogStatusDraft
	}
	if in.Visibility == "" {
		in.Visibility = adminplusdomain.SiteCatalogVisibilityPublic
	}
	if in.RecommendationLevel == "" {
		in.RecommendationLevel = adminplusdomain.SiteCatalogRecommendationNone
	}
	if in.RiskLevel == "" {
		in.RiskLevel = adminplusdomain.SiteCatalogRiskUnknown
	}
	if in.Slug == "" {
		slug, err := s.uniqueSlug(ctx, firstNonEmpty(in.CanonicalHost, in.Name))
		if err != nil {
			return nil, err
		}
		in.Slug = slug
	} else {
		in.Slug = slugify(in.Slug)
	}
	if in.CanonicalHost == "" {
		in.CanonicalHost = canonicalHostFromLinks(in.Links)
	}
	now := s.now().UTC()
	links := make([]*adminplusdomain.SiteCatalogLink, 0, len(in.Links))
	for _, link := range in.Links {
		if normalized := siteCatalogLinkFromInput(link, now); normalized != nil {
			links = append(links, normalized)
		}
	}
	return &adminplusdomain.SiteCatalogSite{
		Slug:                 in.Slug,
		CanonicalHost:        strings.TrimSpace(in.CanonicalHost),
		Name:                 in.Name,
		ShortName:            strings.TrimSpace(in.ShortName),
		Summary:              strings.TrimSpace(in.Summary),
		Description:          strings.TrimSpace(in.Description),
		ProviderType:         in.ProviderType,
		SiteKind:             in.SiteKind,
		Status:               in.Status,
		Visibility:           in.Visibility,
		QualityStatus:        adminplusdomain.SiteCatalogQualityNeedsReview,
		RecommendationLevel:  in.RecommendationLevel,
		RecommendationReason: strings.TrimSpace(in.RecommendationReason),
		RiskLevel:            in.RiskLevel,
		LogoURL:              strings.TrimSpace(in.LogoURL),
		ScreenshotURL:        strings.TrimSpace(in.ScreenshotURL),
		PrimaryLanguage:      strings.TrimSpace(in.PrimaryLanguage),
		CountryOrRegion:      strings.TrimSpace(in.CountryOrRegion),
		SupplierID:           in.SupplierID,
		Metadata:             in.Metadata,
		Links:                links,
		Categories:           categoriesFromIDs(in.CategoryIDs),
		Tags:                 tagsFromIDs(in.TagIDs),
		CreatedAt:            now,
		UpdatedAt:            now,
	}, nil
}

func (s *Service) uniqueSlug(ctx context.Context, seed string) (string, error) {
	base := slugify(seed)
	if base == "" {
		base = "site"
	}
	for i := 0; i < 100; i++ {
		candidate := base
		if i > 0 {
			candidate = base + "-" + stringFromInt64(int64(i+1))
		}
		exists, err := s.repo.SlugExists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", internalError("failed to allocate site slug")
}

func discoveryCandidateLinks(candidate *adminplusdomain.SiteDiscoveryItem) []SiteLinkInput {
	links := make([]SiteLinkInput, 0, 3)
	if candidate.RegisterURL != "" {
		links = append(links, SiteLinkInput{LinkType: adminplusdomain.SiteCatalogLinkRegister, URL: candidate.RegisterURL, Label: "注册", IsPrimary: true})
	}
	if candidate.DashboardURL != "" && candidate.DashboardURL != candidate.RegisterURL {
		links = append(links, SiteLinkInput{LinkType: adminplusdomain.SiteCatalogLinkDashboard, URL: candidate.DashboardURL, Label: "控制台", IsPrimary: len(links) == 0})
	}
	if candidate.APIBaseURL != "" && candidate.APIBaseURL != candidate.DashboardURL {
		links = append(links, SiteLinkInput{LinkType: adminplusdomain.SiteCatalogLinkAPIBase, URL: candidate.APIBaseURL, Label: "API Base"})
	}
	return links
}

func siteCatalogLinkFromInput(in SiteLinkInput, now time.Time) *adminplusdomain.SiteCatalogLink {
	rawURL := strings.TrimSpace(in.URL)
	if rawURL == "" {
		return nil
	}
	linkType := in.LinkType
	if linkType == "" {
		linkType = adminplusdomain.SiteCatalogLinkHomepage
	}
	return &adminplusdomain.SiteCatalogLink{
		LinkType:  linkType,
		URL:       rawURL,
		Label:     strings.TrimSpace(in.Label),
		IsPrimary: in.IsPrimary,
		Status:    adminplusdomain.SiteCatalogLinkUnknown,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func canonicalHostFromLinks(links []SiteLinkInput) string {
	for _, link := range links {
		if host := canonicalHost(link.URL); host != "" {
			return host
		}
	}
	return ""
}

func canonicalHost(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	host := strings.ToLower(parsed.Host)
	host = strings.TrimPrefix(host, "www.")
	return host
}

func siteKindFromDiscoverySection(section string) adminplusdomain.SiteCatalogKind {
	switch strings.ToLower(strings.TrimSpace(section)) {
	case "official":
		return adminplusdomain.SiteCatalogKindOfficial
	case "opensource":
		return adminplusdomain.SiteCatalogKindTool
	case "clients":
		return adminplusdomain.SiteCatalogKindClient
	case "benchmarks":
		return adminplusdomain.SiteCatalogKindBenchmark
	default:
		return adminplusdomain.SiteCatalogKindAPIRelay
	}
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if host := canonicalHost(value); host != "" {
		value = host
	}
	value = strings.TrimPrefix(value, "www.")
	value = slugPattern.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if len(value) > 80 {
		value = strings.Trim(value[:80], "-")
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if text := strings.TrimSpace(value); text != "" {
			return text
		}
	}
	return ""
}

func trimLimit(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func stringFromInt64(value int64) string {
	if value == 0 {
		return "0"
	}
	var out []byte
	for value > 0 {
		out = append([]byte{byte('0' + value%10)}, out...)
		value /= 10
	}
	return string(out)
}

func categoriesFromIDs(ids []int64) []*adminplusdomain.SiteCatalogCategory {
	categories := make([]*adminplusdomain.SiteCatalogCategory, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			categories = append(categories, &adminplusdomain.SiteCatalogCategory{ID: id})
		}
	}
	return categories
}

func tagsFromIDs(ids []int64) []*adminplusdomain.SiteCatalogTag {
	tags := make([]*adminplusdomain.SiteCatalogTag, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			tags = append(tags, &adminplusdomain.SiteCatalogTag{ID: id})
		}
	}
	return tags
}

func uniquePositiveIDs(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
