package billing

import (
	"context"
	"net/http"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type ImportBillLineInput struct {
	SupplierID        int64
	Source            string
	ExternalBillID    string
	ExternalRequestID string
	Model             string
	Currency          string
	CostCents         int64
	InputTokens       int64
	OutputTokens      int64
	StartedAt         time.Time
	EndedAt           *time.Time
	RawPayload        map[string]any
}

type BillLineFilter struct {
	SupplierID int64
	Limit      int
}

type Repository interface {
	CreateBillLine(ctx context.Context, line *adminplusdomain.SupplierBillLine) (*adminplusdomain.SupplierBillLine, error)
	ListBillLines(ctx context.Context, filter BillLineFilter) ([]*adminplusdomain.SupplierBillLine, error)
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  time.Now,
	}
}

func (s *Service) ImportBillLines(ctx context.Context, lines []ImportBillLineInput) ([]*adminplusdomain.SupplierBillLine, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("billing service is not configured")
	}
	if len(lines) == 0 {
		return nil, badRequest("BILLING_LINES_REQUIRED", "billing lines are required")
	}
	if len(lines) > 1000 {
		return nil, badRequest("BILLING_LINES_TOO_MANY", "billing lines must be 1000 or less")
	}
	created := make([]*adminplusdomain.SupplierBillLine, 0, len(lines))
	for _, input := range lines {
		line, err := s.buildLine(input)
		if err != nil {
			return nil, err
		}
		stored, err := s.repo.CreateBillLine(ctx, line)
		if err != nil {
			return nil, err
		}
		created = append(created, stored)
	}
	return created, nil
}

func (s *Service) ListBillLines(ctx context.Context, filter BillLineFilter) ([]*adminplusdomain.SupplierBillLine, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("billing service is not configured")
	}
	if filter.SupplierID < 0 {
		return nil, badRequest("BILLING_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListBillLines(ctx, filter)
}

func (s *Service) buildLine(in ImportBillLineInput) (*adminplusdomain.SupplierBillLine, error) {
	if in.SupplierID <= 0 {
		return nil, badRequest("BILLING_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	model := strings.TrimSpace(in.Model)
	if model == "" {
		return nil, badRequest("BILLING_MODEL_REQUIRED", "billing model is required")
	}
	if in.CostCents < 0 {
		return nil, badRequest("BILLING_COST_INVALID", "billing cost must be non-negative")
	}
	if in.InputTokens < 0 || in.OutputTokens < 0 {
		return nil, badRequest("BILLING_TOKENS_INVALID", "billing tokens must be non-negative")
	}
	if in.StartedAt.IsZero() {
		return nil, badRequest("BILLING_STARTED_AT_REQUIRED", "billing started_at is required")
	}
	if in.EndedAt != nil && in.EndedAt.Before(in.StartedAt) {
		return nil, badRequest("BILLING_TIME_RANGE_INVALID", "billing ended_at must be after started_at")
	}
	return &adminplusdomain.SupplierBillLine{
		SupplierID:        in.SupplierID,
		Source:            normalizeSource(in.Source),
		ExternalBillID:    trimLimit(in.ExternalBillID, 120),
		ExternalRequestID: trimLimit(in.ExternalRequestID, 160),
		Model:             model,
		Currency:          normalizeCurrency(in.Currency),
		CostCents:         in.CostCents,
		InputTokens:       in.InputTokens,
		OutputTokens:      in.OutputTokens,
		StartedAt:         in.StartedAt.UTC(),
		EndedAt:           cloneTime(in.EndedAt),
		RawPayload:        in.RawPayload,
		CreatedAt:         s.now().UTC(),
	}, nil
}

func normalizeSource(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return "manual"
	}
	if len(v) > 60 {
		return v[:60]
	}
	return v
}

func normalizeCurrency(value string) string {
	v := strings.ToUpper(strings.TrimSpace(value))
	if len(v) != 3 {
		return "USD"
	}
	return v
}

func trimLimit(value string, limit int) string {
	v := strings.TrimSpace(value)
	if len(v) <= limit {
		return v
	}
	return v[:limit]
}

func cloneTime(in *time.Time) *time.Time {
	if in == nil {
		return nil
	}
	v := in.UTC()
	return &v
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func badRequest(reason string, message string) error {
	return infraerrors.New(http.StatusBadRequest, reason, message)
}

func internalError(message string) error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", message)
}
