package sitecatalog

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

func TestBulkPublishSitesDeduplicatesIDsAndReportsSkipped(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	repo := &bulkPublishSitesRepo{updated: 1}
	service := NewService(repo)
	service.now = func() time.Time { return now }

	result, err := service.BulkPublishSites(context.Background(), BulkPublishSitesInput{IDs: []int64{1, 2, 2, 0, -1}})
	if err != nil {
		t.Fatalf("BulkPublishSites returned error: %v", err)
	}
	if !reflect.DeepEqual(repo.input.IDs, []int64{1, 2}) {
		t.Fatalf("expected deduplicated ids [1 2], got %#v", repo.input.IDs)
	}
	if !repo.publishedAt.Equal(now) {
		t.Fatalf("expected published time %s, got %s", now, repo.publishedAt)
	}
	if result.Total != 1 || result.Updated != 1 || result.Skipped != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestBulkPublishSitesPublishesByFilterWithoutIDs(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	repo := &bulkPublishSitesRepo{updated: 3}
	service := NewService(repo)
	service.now = func() time.Time { return now }

	result, err := service.BulkPublishSites(context.Background(), BulkPublishSitesInput{
		Query:  " site ",
		Status: adminplusdomain.SiteCatalogStatusDraft,
	})
	if err != nil {
		t.Fatalf("BulkPublishSites returned error: %v", err)
	}
	if repo.input.Query != "site" || repo.input.Status != adminplusdomain.SiteCatalogStatusDraft {
		t.Fatalf("unexpected input: %#v", repo.input)
	}
	if result.Total != 3 || result.Updated != 3 || result.Skipped != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestListSitesPreservesExplicitLimitAboveDefault(t *testing.T) {
	repo := &listSitesCaptureRepo{}
	service := NewService(repo)

	_, err := service.ListSites(context.Background(), SiteFilter{
		Limit: defaultListLimit + 1,
	})
	if err != nil {
		t.Fatalf("ListSites returned error: %v", err)
	}
	if repo.limit != defaultListLimit+1 {
		t.Fatalf("expected limit %d, got %d", defaultListLimit+1, repo.limit)
	}
}

func TestListSitesUsesDefaultLimitWhenUnset(t *testing.T) {
	repo := &listSitesCaptureRepo{}
	service := NewService(repo)

	_, err := service.ListSites(context.Background(), SiteFilter{})
	if err != nil {
		t.Fatalf("ListSites returned error: %v", err)
	}
	if repo.limit != defaultListLimit {
		t.Fatalf("expected default limit %d, got %d", defaultListLimit, repo.limit)
	}
}

func TestDeleteSiteDelegatesToRepository(t *testing.T) {
	repo := &deleteSiteCaptureRepo{}
	service := NewService(repo)

	if err := service.DeleteSite(context.Background(), 42); err != nil {
		t.Fatalf("DeleteSite returned error: %v", err)
	}
	if repo.deletedID != 42 {
		t.Fatalf("expected deleted id 42, got %d", repo.deletedID)
	}
}

func TestDeleteSiteRejectsInvalidID(t *testing.T) {
	repo := &deleteSiteCaptureRepo{}
	service := NewService(repo)

	if err := service.DeleteSite(context.Background(), 0); err == nil {
		t.Fatal("expected error for invalid id")
	}
	if repo.deletedID != 0 {
		t.Fatalf("expected repo not called, got %d", repo.deletedID)
	}
}

func TestBulkAddDiscoveryCandidatesCanIncludeUnsupported(t *testing.T) {
	repo := &bulkPublishSitesRepo{}
	service := NewService(repo)

	result, err := service.BulkAddDiscoveryCandidates(context.Background(), []*adminplusdomain.SiteDiscoveryItem{
		{
			ID:                   9,
			Name:                 "Unknown Site",
			Host:                 "unknown.example.com",
			ClassificationStatus: adminplusdomain.SiteDiscoveryClassificationUnknown,
		},
	}, BulkAddDiscoveryCandidatesInput{IncludeUnsupported: true}, nil)
	if err != nil {
		t.Fatalf("BulkAddDiscoveryCandidates returned error: %v", err)
	}
	if result.Created != 1 || result.Skipped != 0 || len(repo.addedIDs) != 1 || repo.addedIDs[0] != 9 {
		t.Fatalf("unexpected result=%#v added=%#v", result, repo.addedIDs)
	}
}

func TestBulkAddDiscoveryCandidatesSkipsUnsupportedByDefault(t *testing.T) {
	repo := &bulkPublishSitesRepo{}
	service := NewService(repo)

	result, err := service.BulkAddDiscoveryCandidates(context.Background(), []*adminplusdomain.SiteDiscoveryItem{
		{
			ID:                   9,
			Name:                 "Unknown Site",
			Host:                 "unknown.example.com",
			ClassificationStatus: adminplusdomain.SiteDiscoveryClassificationUnknown,
		},
	}, BulkAddDiscoveryCandidatesInput{}, nil)
	if err != nil {
		t.Fatalf("BulkAddDiscoveryCandidates returned error: %v", err)
	}
	if result.Created != 0 || result.Skipped != 1 || len(repo.addedIDs) != 0 {
		t.Fatalf("unexpected result=%#v added=%#v", result, repo.addedIDs)
	}
}

type bulkPublishSitesRepo struct {
	input       BulkPublishSitesInput
	publishedAt time.Time
	updated     int64
	addedIDs    []int64
}

type listSitesCaptureRepo struct {
	limit int
}

type deleteSiteCaptureRepo struct {
	deletedID int64
}

func (r *bulkPublishSitesRepo) ListSites(context.Context, SiteFilter) ([]*adminplusdomain.SiteCatalogSite, error) {
	return nil, errors.New("not implemented")
}

func (r *listSitesCaptureRepo) ListSites(_ context.Context, filter SiteFilter) ([]*adminplusdomain.SiteCatalogSite, error) {
	r.limit = filter.Limit
	return []*adminplusdomain.SiteCatalogSite{}, nil
}

func (r *bulkPublishSitesRepo) GetSite(context.Context, int64) (*adminplusdomain.SiteCatalogSite, error) {
	return nil, errors.New("not implemented")
}

func (r *listSitesCaptureRepo) GetSite(context.Context, int64) (*adminplusdomain.SiteCatalogSite, error) {
	return nil, errors.New("not implemented")
}

func (r *bulkPublishSitesRepo) CreateSite(context.Context, *adminplusdomain.SiteCatalogSite) (*adminplusdomain.SiteCatalogSite, error) {
	return nil, errors.New("not implemented")
}

func (r *listSitesCaptureRepo) CreateSite(context.Context, *adminplusdomain.SiteCatalogSite) (*adminplusdomain.SiteCatalogSite, error) {
	return nil, errors.New("not implemented")
}

func (r *bulkPublishSitesRepo) DeleteSite(context.Context, int64) error {
	return errors.New("not implemented")
}

func (r *listSitesCaptureRepo) DeleteSite(context.Context, int64) error {
	return errors.New("not implemented")
}

func (r *deleteSiteCaptureRepo) CreateSite(context.Context, *adminplusdomain.SiteCatalogSite) (*adminplusdomain.SiteCatalogSite, error) {
	return nil, errors.New("not implemented")
}

func (r *bulkPublishSitesRepo) BulkPublishSites(_ context.Context, input BulkPublishSitesInput, publishedAt time.Time) (int64, error) {
	r.input = input
	r.input.IDs = append([]int64(nil), input.IDs...)
	r.publishedAt = publishedAt
	return r.updated, nil
}

func (r *listSitesCaptureRepo) BulkPublishSites(context.Context, BulkPublishSitesInput, time.Time) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *deleteSiteCaptureRepo) BulkPublishSites(context.Context, BulkPublishSitesInput, time.Time) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *bulkPublishSitesRepo) AddDiscoveryCandidate(_ context.Context, candidate *adminplusdomain.SiteDiscoveryItem, _ AddDiscoveryCandidateInput) (*adminplusdomain.SiteCatalogSite, error) {
	r.addedIDs = append(r.addedIDs, candidate.ID)
	return &adminplusdomain.SiteCatalogSite{
		ID:            candidate.ID,
		Slug:          candidate.Host,
		CanonicalHost: candidate.Host,
		Name:          candidate.Name,
	}, nil
}

func (r *listSitesCaptureRepo) AddDiscoveryCandidate(context.Context, *adminplusdomain.SiteDiscoveryItem, AddDiscoveryCandidateInput) (*adminplusdomain.SiteCatalogSite, error) {
	return nil, errors.New("not implemented")
}

func (r *deleteSiteCaptureRepo) AddDiscoveryCandidate(context.Context, *adminplusdomain.SiteDiscoveryItem, AddDiscoveryCandidateInput) (*adminplusdomain.SiteCatalogSite, error) {
	return nil, errors.New("not implemented")
}

func (r *bulkPublishSitesRepo) ListCategories(context.Context) ([]*adminplusdomain.SiteCatalogCategory, error) {
	return nil, errors.New("not implemented")
}

func (r *listSitesCaptureRepo) ListCategories(context.Context) ([]*adminplusdomain.SiteCatalogCategory, error) {
	return nil, errors.New("not implemented")
}

func (r *deleteSiteCaptureRepo) ListCategories(context.Context) ([]*adminplusdomain.SiteCatalogCategory, error) {
	return nil, errors.New("not implemented")
}

func (r *bulkPublishSitesRepo) ListTags(context.Context) ([]*adminplusdomain.SiteCatalogTag, error) {
	return nil, errors.New("not implemented")
}

func (r *listSitesCaptureRepo) ListTags(context.Context) ([]*adminplusdomain.SiteCatalogTag, error) {
	return nil, errors.New("not implemented")
}

func (r *deleteSiteCaptureRepo) ListTags(context.Context) ([]*adminplusdomain.SiteCatalogTag, error) {
	return nil, errors.New("not implemented")
}

func (r *bulkPublishSitesRepo) SlugExists(context.Context, string) (bool, error) {
	return false, nil
}

func (r *listSitesCaptureRepo) SlugExists(context.Context, string) (bool, error) {
	return false, nil
}

func (r *deleteSiteCaptureRepo) SlugExists(context.Context, string) (bool, error) {
	return false, nil
}

func (r *deleteSiteCaptureRepo) ListSites(context.Context, SiteFilter) ([]*adminplusdomain.SiteCatalogSite, error) {
	return nil, errors.New("not implemented")
}

func (r *deleteSiteCaptureRepo) GetSite(context.Context, int64) (*adminplusdomain.SiteCatalogSite, error) {
	return nil, errors.New("not implemented")
}

func (r *deleteSiteCaptureRepo) DeleteSite(_ context.Context, id int64) error {
	r.deletedID = id
	return nil
}
