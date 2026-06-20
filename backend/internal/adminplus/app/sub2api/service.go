package sub2api

import (
	"context"
	"net/http"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type UsageFilter struct {
	AccountID int64
	Model     string
	From      time.Time
	To        time.Time
	Limit     int
}

type Repository interface {
	ListLocalUsageLines(ctx context.Context, filter UsageFilter) ([]*adminplusdomain.LocalUsageLine, error)
	ListLocalUsageSummaries(ctx context.Context, filter UsageFilter) ([]*adminplusdomain.LocalUsageSummary, error)
	ListLocalAccountUsageSummaries(ctx context.Context, filter UsageFilter) ([]*adminplusdomain.LocalAccountUsageSummary, error)
}

type RuntimeReader interface {
	ListAccountRuntime(ctx context.Context, filter RuntimeFilter) ([]*adminplusdomain.LocalAccountRuntime, error)
}

type Service struct {
	repo        Repository
	runtimeRepo RuntimeReader
	now         func() time.Time
}

func NewService(repo Repository, runtimeRepo RuntimeReader) *Service {
	return &Service{
		repo:        repo,
		runtimeRepo: runtimeRepo,
		now:         time.Now,
	}
}

func (s *Service) ListLocalUsageLines(ctx context.Context, filter UsageFilter) ([]*adminplusdomain.LocalUsageLine, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("sub2api service is not configured")
	}
	normalized, err := s.normalizeFilter(filter)
	if err != nil {
		return nil, err
	}
	return s.repo.ListLocalUsageLines(ctx, normalized)
}

func (s *Service) ListLocalUsageSummaries(ctx context.Context, filter UsageFilter) ([]*adminplusdomain.LocalUsageSummary, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("sub2api service is not configured")
	}
	normalized, err := s.normalizeFilter(filter)
	if err != nil {
		return nil, err
	}
	return s.repo.ListLocalUsageSummaries(ctx, normalized)
}

func (s *Service) ListLocalAccountUsageSummaries(ctx context.Context, filter UsageFilter) ([]*adminplusdomain.LocalAccountUsageSummary, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("sub2api service is not configured")
	}
	normalized, err := s.normalizeFilter(filter)
	if err != nil {
		return nil, err
	}
	return s.repo.ListLocalAccountUsageSummaries(ctx, normalized)
}

func (s *Service) ListAccountRuntime(ctx context.Context, filter RuntimeFilter) ([]*adminplusdomain.LocalAccountRuntime, error) {
	if s == nil || s.runtimeRepo == nil {
		return nil, internalError("sub2api runtime reader is not configured")
	}
	if filter.AccountID < 0 {
		return nil, badRequest("ACCOUNT_RUNTIME_ACCOUNT_ID_INVALID", "invalid account id")
	}
	return s.runtimeRepo.ListAccountRuntime(ctx, filter)
}

func (s *Service) normalizeFilter(filter UsageFilter) (UsageFilter, error) {
	if filter.AccountID < 0 {
		return UsageFilter{}, badRequest("LOCAL_USAGE_ACCOUNT_ID_INVALID", "invalid account id")
	}
	if s == nil {
		return UsageFilter{}, internalError("sub2api service is not configured")
	}
	to := filter.To.UTC()
	if to.IsZero() {
		to = s.now().UTC()
	}
	from := filter.From.UTC()
	if from.IsZero() {
		from = to.Add(-24 * time.Hour)
	}
	if !from.Before(to) {
		return UsageFilter{}, badRequest("LOCAL_USAGE_TIME_RANGE_INVALID", "from must be before to")
	}
	if to.Sub(from) > 31*24*time.Hour {
		return UsageFilter{}, badRequest("LOCAL_USAGE_TIME_RANGE_TOO_LARGE", "time range must be 31 days or less")
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	return UsageFilter{
		AccountID: filter.AccountID,
		Model:     strings.TrimSpace(filter.Model),
		From:      from,
		To:        to,
		Limit:     limit,
	}, nil
}

func badRequest(reason string, message string) error {
	return infraerrors.New(http.StatusBadRequest, reason, message)
}

func internalError(message string) error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", message)
}
