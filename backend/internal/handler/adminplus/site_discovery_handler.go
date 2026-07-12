package adminplus

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	sitediscoveryapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sitediscovery"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type SiteDiscoveryHandler struct {
	service *sitediscoveryapp.Service
}

func NewSiteDiscoveryHandler(service *sitediscoveryapp.Service) *SiteDiscoveryHandler {
	return &SiteDiscoveryHandler{service: service}
}

type updateSiteDiscoverySettingsRequest struct {
	RegistrationEmail   string  `json:"registration_email"`
	RegistrationEnabled bool    `json:"registration_enabled"`
	LowRateThreshold    float64 `json:"low_rate_threshold"`
}

type runSiteDiscoveryRequest struct {
	SourceURL       string `json:"source_url"`
	ProbeInterfaces *bool  `json:"probe_interfaces"`
	ProbeSites      bool   `json:"probe_sites"`
	Limit           int    `json:"limit"`
}

type classifySiteDiscoveryRequest struct {
	Query                string `json:"q"`
	ProviderType         string `json:"provider_type"`
	ClassificationStatus string `json:"classification_status"`
	ImportStatus         string `json:"import_status"`
	RegistrationStatus   string `json:"registration_status"`
	ProcessedStatus      string `json:"processed_status"`
	ProbeInterfaces      *bool  `json:"probe_interfaces"`
	ProbeSites           bool   `json:"probe_sites"`
	Limit                int    `json:"limit"`
}

func (h *SiteDiscoveryHandler) GetSettings(c *gin.Context) {
	settings, err := h.service.GetSettings(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, settings)
}

func (h *SiteDiscoveryHandler) UpdateSettings(c *gin.Context) {
	var req updateSiteDiscoverySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	settings, err := h.service.UpdateSettings(c.Request.Context(), adminplusdomain.SiteDiscoverySettings{
		RegistrationEmail:   req.RegistrationEmail,
		RegistrationEnabled: req.RegistrationEnabled,
		LowRateThreshold:    req.LowRateThreshold,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, settings)
}

func (h *SiteDiscoveryHandler) Run(c *gin.Context) {
	var req runSiteDiscoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	result, err := h.service.Run(c.Request.Context(), sitediscoveryapp.RunInput{
		SourceURL:       req.SourceURL,
		ProbeInterfaces: boolDefault(req.ProbeInterfaces, true),
		ProbeSites:      req.ProbeSites,
		Limit:           req.Limit,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, result)
}

func (h *SiteDiscoveryHandler) RunStream(c *gin.Context) {
	var req runSiteDiscoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Error(c, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusCreated)
	encoder := json.NewEncoder(c.Writer)
	emit := func(event sitediscoveryapp.RunProgressEvent) {
		_ = encoder.Encode(event)
		flusher.Flush()
	}
	_, err := h.service.RunWithProgress(c.Request.Context(), sitediscoveryapp.RunInput{
		SourceURL:       req.SourceURL,
		ProbeInterfaces: boolDefault(req.ProbeInterfaces, true),
		ProbeSites:      req.ProbeSites,
		Limit:           req.Limit,
	}, emit)
	if err != nil {
		emit(sitediscoveryapp.RunProgressEvent{
			Type:    "failed",
			Level:   "error",
			Message: err.Error(),
		})
	}
}

func (h *SiteDiscoveryHandler) ClassifyStream(c *gin.Context) {
	var req classifySiteDiscoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Error(c, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusCreated)
	encoder := json.NewEncoder(c.Writer)
	emit := func(event sitediscoveryapp.RunProgressEvent) {
		_ = encoder.Encode(event)
		flusher.Flush()
	}
	_, err := h.service.ClassifyWithProgress(c.Request.Context(), sitediscoveryapp.ClassifyInput{
		Query:                req.Query,
		ProviderType:         normalizeDiscoveryProviderType(req.ProviderType),
		ClassificationStatus: adminplusdomain.SiteDiscoveryClassificationStatus(strings.TrimSpace(req.ClassificationStatus)),
		ImportStatus:         adminplusdomain.SiteDiscoveryImportStatus(strings.TrimSpace(req.ImportStatus)),
		RegistrationStatus:   adminplusdomain.SupplierRegistrationStatus(strings.TrimSpace(req.RegistrationStatus)),
		ProcessedStatus:      strings.TrimSpace(req.ProcessedStatus),
		ProbeInterfaces:      boolDefault(req.ProbeInterfaces, true),
		ProbeSites:           req.ProbeSites,
		Limit:                req.Limit,
	}, emit)
	if err != nil {
		emit(sitediscoveryapp.RunProgressEvent{
			Type:    "failed",
			Level:   "error",
			Message: err.Error(),
		})
	}
}

func (h *SiteDiscoveryHandler) ListItems(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListItems(c.Request.Context(), sitediscoveryapp.ListFilter{
		Query:                c.Query("q"),
		ProviderType:         normalizeDiscoveryProviderType(c.Query("provider_type")),
		ClassificationStatus: adminplusdomain.SiteDiscoveryClassificationStatus(strings.TrimSpace(c.Query("classification_status"))),
		ImportStatus:         adminplusdomain.SiteDiscoveryImportStatus(strings.TrimSpace(c.Query("import_status"))),
		RegistrationStatus:   adminplusdomain.SupplierRegistrationStatus(strings.TrimSpace(c.Query("registration_status"))),
		ProcessedStatus:      strings.TrimSpace(c.Query("processed_status")),
		Limit:                fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *SiteDiscoveryHandler) ImportItem(c *gin.Context) {
	id, ok := parseSiteDiscoveryItemID(c)
	if !ok {
		return
	}
	item, err := h.service.ImportItem(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, item)
}

func (h *SiteDiscoveryHandler) RegisterItem(c *gin.Context) {
	id, ok := parseSiteDiscoveryItemID(c)
	if !ok {
		return
	}
	credential, task, err := h.service.RegisterItemWithOptions(c.Request.Context(), sitediscoveryapp.RegisterItemInput{
		ItemID: id,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, gin.H{
		"credential": credential,
		"task":       task,
	})
}

func (h *SiteDiscoveryHandler) ListRegistrationTasks(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListRegistrationTasks(c.Request.Context(), sitediscoveryapp.ListFilter{
		Query:              c.Query("q"),
		ProviderType:       normalizeDiscoveryProviderType(c.Query("provider_type")),
		RegistrationStatus: adminplusdomain.SupplierRegistrationStatus(strings.TrimSpace(c.Query("registration_status"))),
		Limit:              fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *SiteDiscoveryHandler) RerunRegistration(c *gin.Context) {
	id, ok := parseRegistrationWorkflowID(c)
	if !ok {
		return
	}
	credential, task, err := h.service.RerunRegistrationWithOptions(c.Request.Context(), sitediscoveryapp.RerunRegistrationInput{
		RegistrationID: id,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, gin.H{
		"credential": credential,
		"task":       task,
	})
}

func (h *SiteDiscoveryHandler) ListRegistrationLogs(c *gin.Context) {
	id, ok := parseRegistrationWorkflowID(c)
	if !ok {
		return
	}
	limit := parsePositiveIntQuery(c, "limit", 50)
	result, err := h.service.ListRegistrationLogs(c.Request.Context(), id, limit)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *SiteDiscoveryHandler) Recommendations(c *gin.Context) {
	limit := parsePositiveIntQuery(c, "limit", 100)
	items, err := h.service.ListRecommendations(c.Request.Context(), limit)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *SiteDiscoveryHandler) GetRegistrationCredential(c *gin.Context) {
	id, ok := parseExtensionTaskID(c)
	if !ok {
		return
	}
	var req extensionTaskLeaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	credential, err := h.service.GetTaskRegistrationCredential(c.Request.Context(), id, req.DeviceID, req.LeaseToken)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, credential)
}

func parseSiteDiscoveryItemID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid site discovery item id")
		return 0, false
	}
	return id, true
}

func parseRegistrationWorkflowID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid registration id")
		return 0, false
	}
	return id, true
}

func normalizeDiscoveryProviderType(value string) adminplusdomain.SupplierType {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return ""
	case "newapi", "new-api", "new_api":
		return adminplusdomain.SupplierTypeNewAPI
	case "sub2api", "sub2-api", "sub2_api":
		return adminplusdomain.SupplierTypeSub2API
	default:
		return adminplusdomain.NormalizeSupplierType(value)
	}
}
