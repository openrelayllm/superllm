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

func TestSiteCatalogListSitesCountsBeyondThousandSites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewSiteCatalogHandler(sitecatalogapp.NewService(&fakeSiteCatalogRepo{
		sites: buildSiteCatalogList(1001),
	}), nil)

	router := gin.New()
	router.GET("/api/v1/admin-plus/site-catalog/sites", handler.ListSites)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin-plus/site-catalog/sites?page_size=1", nil)
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

func TestSiteCatalogDeleteSiteReturnsDeletedFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeSiteCatalogRepo{
		sites: buildSiteCatalogList(2),
	}
	handler := NewSiteCatalogHandler(sitecatalogapp.NewService(repo), nil)

	router := gin.New()
	router.DELETE("/api/v1/admin-plus/site-catalog/sites/:id", handler.DeleteSite)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin-plus/site-catalog/sites/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"deleted":true`)
	require.Len(t, repo.deletedIDs, 1)
	require.EqualValues(t, 1, repo.deletedIDs[0])
}

type fakeSiteCatalogRepo struct {
	sites      []*adminplusdomain.SiteCatalogSite
	deletedIDs []int64
}

func (r *fakeSiteCatalogRepo) ListSites(_ context.Context, filter sitecatalogapp.SiteFilter) ([]*adminplusdomain.SiteCatalogSite, error) {
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

func (r *fakeSiteCatalogRepo) GetSite(context.Context, int64) (*adminplusdomain.SiteCatalogSite, error) {
	return nil, errors.New("not implemented")
}

func (r *fakeSiteCatalogRepo) CreateSite(context.Context, *adminplusdomain.SiteCatalogSite) (*adminplusdomain.SiteCatalogSite, error) {
	return nil, errors.New("not implemented")
}

func (r *fakeSiteCatalogRepo) DeleteSite(_ context.Context, id int64) error {
	r.deletedIDs = append(r.deletedIDs, id)
	for i, site := range r.sites {
		if site != nil && site.ID == id {
			r.sites = append(r.sites[:i], r.sites[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *fakeSiteCatalogRepo) BulkPublishSites(context.Context, sitecatalogapp.BulkPublishSitesInput, time.Time) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *fakeSiteCatalogRepo) AddDiscoveryCandidate(context.Context, *adminplusdomain.SiteDiscoveryItem, sitecatalogapp.AddDiscoveryCandidateInput) (*adminplusdomain.SiteCatalogSite, error) {
	return nil, errors.New("not implemented")
}

func (r *fakeSiteCatalogRepo) ListCategories(context.Context) ([]*adminplusdomain.SiteCatalogCategory, error) {
	return nil, errors.New("not implemented")
}

func (r *fakeSiteCatalogRepo) ListTags(context.Context) ([]*adminplusdomain.SiteCatalogTag, error) {
	return nil, errors.New("not implemented")
}

func (r *fakeSiteCatalogRepo) SlugExists(context.Context, string) (bool, error) {
	return false, errors.New("not implemented")
}

func buildSiteCatalogList(total int) []*adminplusdomain.SiteCatalogSite {
	items := make([]*adminplusdomain.SiteCatalogSite, 0, total)
	for i := 0; i < total; i++ {
		items = append(items, &adminplusdomain.SiteCatalogSite{
			ID:            int64(i + 1),
			Slug:          "site-" + strconv.Itoa(i),
			CanonicalHost: "site-" + strconv.Itoa(i) + ".example.com",
			Name:          "Site " + strconv.Itoa(i),
			Status:        adminplusdomain.SiteCatalogStatusPublished,
			Visibility:    adminplusdomain.SiteCatalogVisibilityPublic,
		})
	}
	return items
}
