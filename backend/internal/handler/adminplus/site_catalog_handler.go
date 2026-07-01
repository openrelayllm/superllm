package adminplus

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"

	sitecatalogapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sitecatalog"
	sitediscoveryapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sitediscovery"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type SiteCatalogHandler struct {
	service   *sitecatalogapp.Service
	discovery *sitediscoveryapp.Service
}

func NewSiteCatalogHandler(service *sitecatalogapp.Service, discovery *sitediscoveryapp.Service) *SiteCatalogHandler {
	return &SiteCatalogHandler{service: service, discovery: discovery}
}

type siteCatalogLinkRequest struct {
	LinkType  string `json:"link_type"`
	URL       string `json:"url"`
	Label     string `json:"label"`
	IsPrimary bool   `json:"is_primary"`
}

type createSiteCatalogSiteRequest struct {
	Slug                 string                   `json:"slug"`
	CanonicalHost        string                   `json:"canonical_host"`
	Name                 string                   `json:"name"`
	ShortName            string                   `json:"short_name"`
	Summary              string                   `json:"summary"`
	Description          string                   `json:"description"`
	ProviderType         string                   `json:"provider_type"`
	SiteKind             string                   `json:"site_kind"`
	Status               string                   `json:"status"`
	Visibility           string                   `json:"visibility"`
	RecommendationLevel  string                   `json:"recommendation_level"`
	RecommendationReason string                   `json:"recommendation_reason"`
	RiskLevel            string                   `json:"risk_level"`
	LogoURL              string                   `json:"logo_url"`
	ScreenshotURL        string                   `json:"screenshot_url"`
	PrimaryLanguage      string                   `json:"primary_language"`
	CountryOrRegion      string                   `json:"country_or_region"`
	SupplierID           int64                    `json:"supplier_id"`
	Metadata             map[string]any           `json:"metadata"`
	Links                []siteCatalogLinkRequest `json:"links"`
	CategoryIDs          []int64                  `json:"category_ids"`
	TagIDs               []int64                  `json:"tag_ids"`
}

type addDiscoveryCandidateRequest struct {
	SiteID               int64                    `json:"site_id"`
	Slug                 string                   `json:"slug"`
	Name                 string                   `json:"name"`
	Summary              string                   `json:"summary"`
	Description          string                   `json:"description"`
	SiteKind             string                   `json:"site_kind"`
	Status               string                   `json:"status"`
	Visibility           string                   `json:"visibility"`
	RecommendationLevel  string                   `json:"recommendation_level"`
	RecommendationReason string                   `json:"recommendation_reason"`
	RiskLevel            string                   `json:"risk_level"`
	CategoryIDs          []int64                  `json:"category_ids"`
	TagIDs               []int64                  `json:"tag_ids"`
	Links                []siteCatalogLinkRequest `json:"links"`
}

type bulkAddDiscoveryCandidatesRequest struct {
	Query                string  `json:"q"`
	ProviderType         string  `json:"provider_type"`
	ClassificationStatus string  `json:"classification_status"`
	ImportStatus         string  `json:"import_status"`
	RegistrationStatus   string  `json:"registration_status"`
	ProcessedStatus      string  `json:"processed_status"`
	OnlySupported        *bool   `json:"only_supported"`
	Limit                int     `json:"limit"`
	SiteKind             string  `json:"site_kind"`
	Status               string  `json:"status"`
	Visibility           string  `json:"visibility"`
	RecommendationLevel  string  `json:"recommendation_level"`
	RecommendationReason string  `json:"recommendation_reason"`
	RiskLevel            string  `json:"risk_level"`
	CategoryIDs          []int64 `json:"category_ids"`
	TagIDs               []int64 `json:"tag_ids"`
}

type bulkPublishSiteCatalogSitesRequest struct {
	IDs          []int64 `json:"ids"`
	Query        string  `json:"q"`
	Status       string  `json:"status"`
	SiteKind     string  `json:"site_kind"`
	ProviderType string  `json:"provider_type"`
}

func (h *SiteCatalogHandler) ListSites(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListSites(c.Request.Context(), sitecatalogapp.SiteFilter{
		Query:    c.Query("q"),
		Status:   adminplusdomain.SiteCatalogStatus(strings.TrimSpace(c.Query("status"))),
		SiteKind: adminplusdomain.SiteCatalogKind(strings.TrimSpace(c.Query("site_kind"))),
		Provider: normalizeDiscoveryProviderType(c.Query("provider_type")),
		Limit:    math.MaxInt,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *SiteCatalogHandler) GetSite(c *gin.Context) {
	id, ok := parseSiteCatalogID(c)
	if !ok {
		return
	}
	item, err := h.service.GetSite(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, item)
}

func (h *SiteCatalogHandler) DeleteSite(c *gin.Context) {
	id, ok := parseSiteCatalogID(c)
	if !ok {
		return
	}
	if response.ErrorFrom(c, h.service.DeleteSite(c.Request.Context(), id)) {
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *SiteCatalogHandler) CreateSite(c *gin.Context) {
	var req createSiteCatalogSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	item, err := h.service.CreateSite(c.Request.Context(), sitecatalogapp.CreateSiteInput{
		Slug:                 req.Slug,
		CanonicalHost:        req.CanonicalHost,
		Name:                 req.Name,
		ShortName:            req.ShortName,
		Summary:              req.Summary,
		Description:          req.Description,
		ProviderType:         normalizeDiscoveryProviderType(req.ProviderType),
		SiteKind:             adminplusdomain.SiteCatalogKind(strings.TrimSpace(req.SiteKind)),
		Status:               adminplusdomain.SiteCatalogStatus(strings.TrimSpace(req.Status)),
		Visibility:           adminplusdomain.SiteCatalogVisibility(strings.TrimSpace(req.Visibility)),
		RecommendationLevel:  adminplusdomain.SiteCatalogRecommendationLevel(strings.TrimSpace(req.RecommendationLevel)),
		RecommendationReason: req.RecommendationReason,
		RiskLevel:            adminplusdomain.SiteCatalogRiskLevel(strings.TrimSpace(req.RiskLevel)),
		LogoURL:              req.LogoURL,
		ScreenshotURL:        req.ScreenshotURL,
		PrimaryLanguage:      req.PrimaryLanguage,
		CountryOrRegion:      req.CountryOrRegion,
		SupplierID:           req.SupplierID,
		Metadata:             req.Metadata,
		Links:                siteCatalogLinksFromRequest(req.Links),
		CategoryIDs:          req.CategoryIDs,
		TagIDs:               req.TagIDs,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, item)
}

func (h *SiteCatalogHandler) BulkPublishSites(c *gin.Context) {
	var req bulkPublishSiteCatalogSitesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	result, err := h.service.BulkPublishSites(c.Request.Context(), sitecatalogapp.BulkPublishSitesInput{
		IDs:      req.IDs,
		Query:    req.Query,
		Status:   adminplusdomain.SiteCatalogStatus(strings.TrimSpace(req.Status)),
		SiteKind: adminplusdomain.SiteCatalogKind(strings.TrimSpace(req.SiteKind)),
		Provider: normalizeDiscoveryProviderType(req.ProviderType),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *SiteCatalogHandler) AddDiscoveryCandidate(c *gin.Context) {
	id, ok := parseSiteDiscoveryItemID(c)
	if !ok {
		return
	}
	var req addDiscoveryCandidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	candidate, err := h.discovery.GetItem(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	item, err := h.service.AddDiscoveryCandidate(c.Request.Context(), candidate, sitecatalogapp.AddDiscoveryCandidateInput{
		SiteID:               req.SiteID,
		Slug:                 req.Slug,
		Name:                 req.Name,
		Summary:              req.Summary,
		Description:          req.Description,
		SiteKind:             adminplusdomain.SiteCatalogKind(strings.TrimSpace(req.SiteKind)),
		Status:               adminplusdomain.SiteCatalogStatus(strings.TrimSpace(req.Status)),
		Visibility:           adminplusdomain.SiteCatalogVisibility(strings.TrimSpace(req.Visibility)),
		RecommendationLevel:  adminplusdomain.SiteCatalogRecommendationLevel(strings.TrimSpace(req.RecommendationLevel)),
		RecommendationReason: req.RecommendationReason,
		RiskLevel:            adminplusdomain.SiteCatalogRiskLevel(strings.TrimSpace(req.RiskLevel)),
		CategoryIDs:          req.CategoryIDs,
		TagIDs:               req.TagIDs,
		Links:                siteCatalogLinksFromRequest(req.Links),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, item)
}

func (h *SiteCatalogHandler) BulkAddDiscoveryCandidatesStream(c *gin.Context) {
	var req bulkAddDiscoveryCandidatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Error(c, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	processedStatus := strings.TrimSpace(req.ProcessedStatus)
	if processedStatus == "" {
		processedStatus = "unprocessed"
	}
	classificationStatus := adminplusdomain.SiteDiscoveryClassificationStatus(strings.TrimSpace(req.ClassificationStatus))
	if boolDefault(req.OnlySupported, true) && classificationStatus == "" {
		classificationStatus = adminplusdomain.SiteDiscoveryClassificationSupported
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 1000
	}
	candidates, err := h.discovery.ListItems(c.Request.Context(), sitediscoveryapp.ListFilter{
		Query:                req.Query,
		ProviderType:         normalizeDiscoveryProviderType(req.ProviderType),
		ClassificationStatus: classificationStatus,
		ImportStatus:         adminplusdomain.SiteDiscoveryImportStatus(strings.TrimSpace(req.ImportStatus)),
		RegistrationStatus:   adminplusdomain.SupplierRegistrationStatus(strings.TrimSpace(req.RegistrationStatus)),
		ProcessedStatus:      processedStatus,
		Limit:                limit,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusCreated)
	encoder := json.NewEncoder(c.Writer)
	emit := func(event sitecatalogapp.BulkAddDiscoveryCandidateProgressEvent) {
		_ = encoder.Encode(event)
		flusher.Flush()
	}
	_, err = h.service.BulkAddDiscoveryCandidates(c.Request.Context(), candidates, sitecatalogapp.BulkAddDiscoveryCandidatesInput{
		SiteKind:             adminplusdomain.SiteCatalogKind(strings.TrimSpace(req.SiteKind)),
		Status:               adminplusdomain.SiteCatalogStatus(strings.TrimSpace(req.Status)),
		Visibility:           adminplusdomain.SiteCatalogVisibility(strings.TrimSpace(req.Visibility)),
		RecommendationLevel:  adminplusdomain.SiteCatalogRecommendationLevel(strings.TrimSpace(req.RecommendationLevel)),
		RecommendationReason: req.RecommendationReason,
		RiskLevel:            adminplusdomain.SiteCatalogRiskLevel(strings.TrimSpace(req.RiskLevel)),
		CategoryIDs:          req.CategoryIDs,
		TagIDs:               req.TagIDs,
		IncludeUnsupported:   !boolDefault(req.OnlySupported, true),
	}, emit)
	if err != nil {
		emit(sitecatalogapp.BulkAddDiscoveryCandidateProgressEvent{
			Type:    "failed",
			Level:   "error",
			Message: err.Error(),
		})
	}
}

func (h *SiteCatalogHandler) ListCategories(c *gin.Context) {
	items, err := h.service.ListCategories(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *SiteCatalogHandler) ListTags(c *gin.Context) {
	items, err := h.service.ListTags(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"items": items})
}

func parseSiteCatalogID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid site catalog id")
		return 0, false
	}
	return id, true
}

func siteCatalogLinksFromRequest(items []siteCatalogLinkRequest) []sitecatalogapp.SiteLinkInput {
	out := make([]sitecatalogapp.SiteLinkInput, 0, len(items))
	for _, item := range items {
		out = append(out, sitecatalogapp.SiteLinkInput{
			LinkType:  adminplusdomain.SiteCatalogLinkType(strings.TrimSpace(item.LinkType)),
			URL:       item.URL,
			Label:     item.Label,
			IsPrimary: item.IsPrimary,
		})
	}
	return out
}
