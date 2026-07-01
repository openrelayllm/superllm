package adminplus

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	sitecatalogapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sitecatalog"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicProxyAIListSitesProjectsOnlyPublicPublishedSites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC)
	handler := NewPublicProxyAIHandler(sitecatalogapp.NewService(&fakePublicProxyAIRepo{
		sites: []*adminplusdomain.SiteCatalogSite{
			publicProxyAITestSite(1, "good", adminplusdomain.SiteCatalogVisibilityPublic, adminplusdomain.SiteCatalogStatusPublished, adminplusdomain.SiteCatalogQualityComplete),
			publicProxyAITestSite(2, "draft", adminplusdomain.SiteCatalogVisibilityPublic, adminplusdomain.SiteCatalogStatusDraft, adminplusdomain.SiteCatalogQualityComplete),
			publicProxyAITestSite(3, "private", adminplusdomain.SiteCatalogVisibilityPrivate, adminplusdomain.SiteCatalogStatusPublished, adminplusdomain.SiteCatalogQualityComplete),
			publicProxyAITestSite(4, "duplicate", adminplusdomain.SiteCatalogVisibilityPublic, adminplusdomain.SiteCatalogStatusPublished, adminplusdomain.SiteCatalogQualityDuplicate),
		},
	}), nil)
	handler.now = func() time.Time { return now }

	router := gin.New()
	router.GET("/api/v1/public/proxyai/sites", handler.ListSites)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/proxyai/sites?sort=rate_asc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), "supplier_id")
	require.NotContains(t, w.Body.String(), `"id":`)
	require.NotContains(t, w.Body.String(), `"dashboard"`)
	require.NotContains(t, w.Body.String(), "/dashboard")

	var envelope struct {
		Data struct {
			Items []struct {
				Slug  string `json:"slug"`
				Links []struct {
					LinkType string `json:"link_type"`
				} `json:"links"`
				Metrics struct {
					LowestRate    *float64 `json:"lowest_rate"`
					Available     *bool    `json:"available"`
					FirstTokenMS  *int     `json:"first_token_ms"`
					Uptime24H     *float64 `json:"uptime_24h"`
					AvgResponseMS *int     `json:"avg_response_ms"`
				} `json:"metrics"`
			} `json:"items"`
			Total       int       `json:"total"`
			GeneratedAt time.Time `json:"generated_at"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, 1, envelope.Data.Total)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "good", envelope.Data.Items[0].Slug)
	require.NotNil(t, envelope.Data.Items[0].Metrics.LowestRate)
	require.Equal(t, 0.04, *envelope.Data.Items[0].Metrics.LowestRate)
	require.NotNil(t, envelope.Data.Items[0].Metrics.Available)
	require.True(t, *envelope.Data.Items[0].Metrics.Available)
	require.NotNil(t, envelope.Data.Items[0].Metrics.Uptime24H)
	require.InDelta(t, 99.27, *envelope.Data.Items[0].Metrics.Uptime24H, 0.001)
	require.NotNil(t, envelope.Data.Items[0].Metrics.FirstTokenMS)
	require.Equal(t, 980, *envelope.Data.Items[0].Metrics.FirstTokenMS)
	require.Len(t, envelope.Data.Items[0].Links, 1)
	require.Equal(t, string(adminplusdomain.SiteCatalogLinkRegister), envelope.Data.Items[0].Links[0].LinkType)
	require.Equal(t, now, envelope.Data.GeneratedAt)
}

func TestPublicProxyAIGetSiteDoesNotRevealPrivateSite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewPublicProxyAIHandler(sitecatalogapp.NewService(&fakePublicProxyAIRepo{
		sites: []*adminplusdomain.SiteCatalogSite{
			publicProxyAITestSite(1, "private", adminplusdomain.SiteCatalogVisibilityPrivate, adminplusdomain.SiteCatalogStatusPublished, adminplusdomain.SiteCatalogQualityComplete),
		},
	}), nil)

	router := gin.New()
	router.GET("/api/v1/public/proxyai/sites/:slug", handler.GetSite)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/proxyai/sites/private", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "site not found")
}

func TestPublicProxyAISummaryCountsBeyondThousandSites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewPublicProxyAIHandler(sitecatalogapp.NewService(&fakePublicProxyAIRepo{
		sites: buildPublicProxyAIList(1001),
	}), nil)

	router := gin.New()
	router.GET("/api/v1/public/proxyai/summary", handler.Summary)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/proxyai/summary", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var envelope struct {
		Data struct {
			SiteCount int `json:"site_count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, 1001, envelope.Data.SiteCount)
}

func TestPublicProxyAIListSitesCountsBeyondThousandSites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewPublicProxyAIHandler(sitecatalogapp.NewService(&fakePublicProxyAIRepo{
		sites: buildPublicProxyAIList(1001),
	}), nil)

	router := gin.New()
	router.GET("/api/v1/public/proxyai/sites", handler.ListSites)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/proxyai/sites?page_size=1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var envelope struct {
		Data struct {
			Total int `json:"total"`
			Items []struct {
				Slug string `json:"slug"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, 1001, envelope.Data.Total)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "site-0", envelope.Data.Items[0].Slug)
}

type fakePublicProxyAIRepo struct {
	sites []*adminplusdomain.SiteCatalogSite
}

func (r *fakePublicProxyAIRepo) ListSites(_ context.Context, filter sitecatalogapp.SiteFilter) ([]*adminplusdomain.SiteCatalogSite, error) {
	out := make([]*adminplusdomain.SiteCatalogSite, 0, len(r.sites))
	for _, site := range r.sites {
		if site == nil {
			continue
		}
		if filter.Status != "" && site.Status != filter.Status {
			continue
		}
		out = append(out, site)
	}
	return out, nil
}

func (r *fakePublicProxyAIRepo) GetSite(context.Context, int64) (*adminplusdomain.SiteCatalogSite, error) {
	return nil, errors.New("not implemented")
}

func (r *fakePublicProxyAIRepo) CreateSite(context.Context, *adminplusdomain.SiteCatalogSite) (*adminplusdomain.SiteCatalogSite, error) {
	return nil, errors.New("not implemented")
}

func (r *fakePublicProxyAIRepo) DeleteSite(context.Context, int64) error {
	return errors.New("not implemented")
}

func (r *fakePublicProxyAIRepo) BulkPublishSites(context.Context, sitecatalogapp.BulkPublishSitesInput, time.Time) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *fakePublicProxyAIRepo) AddDiscoveryCandidate(context.Context, *adminplusdomain.SiteDiscoveryItem, sitecatalogapp.AddDiscoveryCandidateInput) (*adminplusdomain.SiteCatalogSite, error) {
	return nil, errors.New("not implemented")
}

func (r *fakePublicProxyAIRepo) ListCategories(context.Context) ([]*adminplusdomain.SiteCatalogCategory, error) {
	return nil, errors.New("not implemented")
}

func (r *fakePublicProxyAIRepo) ListTags(context.Context) ([]*adminplusdomain.SiteCatalogTag, error) {
	return nil, errors.New("not implemented")
}

func (r *fakePublicProxyAIRepo) SlugExists(context.Context, string) (bool, error) {
	return false, errors.New("not implemented")
}

func publicProxyAITestSite(id int64, slug string, visibility adminplusdomain.SiteCatalogVisibility, status adminplusdomain.SiteCatalogStatus, quality adminplusdomain.SiteCatalogQualityStatus) *adminplusdomain.SiteCatalogSite {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	return &adminplusdomain.SiteCatalogSite{
		ID:                  id,
		Slug:                slug,
		CanonicalHost:       slug + ".example.com",
		Name:                slug,
		ProviderType:        adminplusdomain.SupplierTypeSub2API,
		SiteKind:            adminplusdomain.SiteCatalogKindAPIRelay,
		Status:              status,
		Visibility:          visibility,
		QualityStatus:       quality,
		RecommendationLevel: adminplusdomain.SiteCatalogRecommendationNormal,
		RiskLevel:           adminplusdomain.SiteCatalogRiskLow,
		SupplierID:          999,
		Metadata: map[string]any{
			"monitor_average_response_ms": 820,
			"first_token_ms":              980,
		},
		Links: []*adminplusdomain.SiteCatalogLink{
			{
				ID:        100,
				SiteID:    id,
				LinkType:  adminplusdomain.SiteCatalogLinkRegister,
				URL:       "https://" + slug + ".example.com/register",
				Label:     "注册",
				IsPrimary: true,
				Status:    adminplusdomain.SiteCatalogLinkOK,
			},
			{
				ID:        101,
				SiteID:    id,
				LinkType:  adminplusdomain.SiteCatalogLinkDashboard,
				URL:       "https://" + slug + ".example.com/dashboard",
				Label:     "控制台",
				IsPrimary: false,
				Status:    adminplusdomain.SiteCatalogLinkOK,
			},
		},
		Categories: []*adminplusdomain.SiteCatalogCategory{
			{ID: 10, Slug: "api-relay", Name: "API 中转", Enabled: true},
		},
		Tags: []*adminplusdomain.SiteCatalogTag{
			{ID: 20, Slug: "openai", Name: "OpenAI", TagType: "model_family", Enabled: true},
		},
		Sources: []*adminplusdomain.SiteCatalogSource{
			{
				ID:         30,
				SiteID:     id,
				LastSeenAt: now,
				ObservedPayload: map[string]any{
					"available_count":   1,
					"latest_checked_at": "2026-06-27T08:30:00Z",
					"min_rate":          0.04,
					"plan_types":        []any{"GPT 混合(对接)", "GPT Pro"},
					"success_rate_24h":  0.9927,
				},
			},
		},
		PublishedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func buildPublicProxyAIList(count int) []*adminplusdomain.SiteCatalogSite {
	items := make([]*adminplusdomain.SiteCatalogSite, 0, count)
	for i := 0; i < count; i++ {
		slug := "site-" + strconv.Itoa(i)
		items = append(items, publicProxyAITestSite(int64(i+1), slug, adminplusdomain.SiteCatalogVisibilityPublic, adminplusdomain.SiteCatalogStatusPublished, adminplusdomain.SiteCatalogQualityComplete))
	}
	return items
}
