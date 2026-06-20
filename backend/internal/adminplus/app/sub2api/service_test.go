package sub2api

import (
	"context"
	"testing"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

type stubUsageRepository struct {
	filter UsageFilter
}

type stubRuntimeRepository struct {
	filter RuntimeFilter
}

func (r *stubUsageRepository) ListLocalUsageLines(_ context.Context, filter UsageFilter) ([]*adminplusdomain.LocalUsageLine, error) {
	r.filter = filter
	return []*adminplusdomain.LocalUsageLine{}, nil
}

func (r *stubUsageRepository) ListLocalUsageSummaries(_ context.Context, filter UsageFilter) ([]*adminplusdomain.LocalUsageSummary, error) {
	r.filter = filter
	return []*adminplusdomain.LocalUsageSummary{}, nil
}

func (r *stubUsageRepository) ListLocalAccountUsageSummaries(_ context.Context, filter UsageFilter) ([]*adminplusdomain.LocalAccountUsageSummary, error) {
	r.filter = filter
	return []*adminplusdomain.LocalAccountUsageSummary{}, nil
}

func (r *stubRuntimeRepository) ListAccountRuntime(_ context.Context, filter RuntimeFilter) ([]*adminplusdomain.LocalAccountRuntime, error) {
	r.filter = filter
	return []*adminplusdomain.LocalAccountRuntime{}, nil
}

func TestServiceListLocalUsageLinesDefaultsToLast24Hours(t *testing.T) {
	repo := &stubUsageRepository{}
	svc := NewService(repo, &stubRuntimeRepository{})
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	_, err := svc.ListLocalUsageLines(context.Background(), UsageFilter{})

	require.NoError(t, err)
	require.Equal(t, now.Add(-24*time.Hour), repo.filter.From)
	require.Equal(t, now, repo.filter.To)
	require.Equal(t, 200, repo.filter.Limit)
}

func TestServiceRejectsTooLargeUsageRange(t *testing.T) {
	svc := NewService(&stubUsageRepository{}, &stubRuntimeRepository{})
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(32 * 24 * time.Hour)

	_, err := svc.ListLocalUsageSummaries(context.Background(), UsageFilter{From: from, To: to})

	require.Error(t, err)
	require.Contains(t, err.Error(), "LOCAL_USAGE_TIME_RANGE_TOO_LARGE")
}

func TestServiceListAccountRuntimeRejectsNegativeAccountID(t *testing.T) {
	svc := NewService(&stubUsageRepository{}, &stubRuntimeRepository{})

	_, err := svc.ListAccountRuntime(context.Background(), RuntimeFilter{AccountID: -1})

	require.Error(t, err)
	require.Contains(t, err.Error(), "ACCOUNT_RUNTIME_ACCOUNT_ID_INVALID")
}

func TestServiceListAccountRuntimePassesFilter(t *testing.T) {
	repo := &stubRuntimeRepository{}
	svc := NewService(&stubUsageRepository{}, repo)

	_, err := svc.ListAccountRuntime(context.Background(), RuntimeFilter{AccountID: 7, Query: "prod", Limit: 20})

	require.NoError(t, err)
	require.Equal(t, int64(7), repo.filter.AccountID)
	require.Equal(t, "prod", repo.filter.Query)
	require.Equal(t, 20, repo.filter.Limit)
}
