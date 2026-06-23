package suppliergroups

import (
	"context"
	"net/http"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type SyncResult struct {
	SupplierID int64                            `json:"supplier_id"`
	SystemType string                           `json:"system_type"`
	Origin     string                           `json:"origin"`
	APIBaseURL string                           `json:"api_base_url"`
	Groups     []*adminplusdomain.SupplierGroup `json:"groups"`
	SyncedAt   time.Time                        `json:"synced_at"`
	Total      int                              `json:"total"`
}

type ListFilter struct {
	SupplierID int64
	Status     adminplusdomain.SupplierGroupStatus
	Query      string
	Limit      int
}

type Repository interface {
	GetSupplierName(ctx context.Context, supplierID int64) (string, error)
	UpsertMany(ctx context.Context, supplierID int64, groups []*adminplusdomain.SupplierGroup, seenAt time.Time) ([]*adminplusdomain.SupplierGroup, error)
	List(ctx context.Context, filter ListFilter) ([]*adminplusdomain.SupplierGroup, error)
}

type SessionReader interface {
	DecryptedProbeInput(ctx context.Context, supplierID int64) (ports.SessionProbeInput, error)
}

type Service struct {
	repo    Repository
	session SessionReader
	reader  ports.SessionGroupAdapter
	now     func() time.Time
}

func NewService(repo Repository, session SessionReader, reader ports.SessionGroupAdapter) *Service {
	return &Service{
		repo:    repo,
		session: session,
		reader:  reader,
		now:     time.Now,
	}
}

func (s *Service) Sync(ctx context.Context, supplierID int64) (*SyncResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier group service is not configured")
	}
	if s.session == nil {
		return nil, internalError("supplier browser session service is not configured")
	}
	if s.reader == nil {
		return nil, internalError("supplier group provider adapter is not configured")
	}
	if supplierID <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	input, err := s.session.DecryptedProbeInput(ctx, supplierID)
	if err != nil {
		return nil, err
	}
	result, err := s.reader.ReadGroups(ctx, input)
	if err != nil {
		return nil, err
	}
	supplierName, err := s.repo.GetSupplierName(ctx, supplierID)
	if err != nil {
		return nil, err
	}
	seenAt := result.CapturedAt.UTC()
	if seenAt.IsZero() {
		seenAt = s.now().UTC()
	}
	groups := make([]*adminplusdomain.SupplierGroup, 0, len(result.Groups))
	for _, providerGroup := range result.Groups {
		group := buildSupplierGroup(supplierID, supplierName, providerGroup, seenAt)
		if group == nil {
			continue
		}
		groups = append(groups, group)
	}
	saved, err := s.repo.UpsertMany(ctx, supplierID, groups, seenAt)
	if err != nil {
		return nil, err
	}
	return &SyncResult{
		SupplierID: supplierID,
		SystemType: result.SystemType,
		Origin:     result.Origin,
		APIBaseURL: result.APIBaseURL,
		Groups:     saved,
		SyncedAt:   seenAt,
		Total:      len(saved),
	}, nil
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]*adminplusdomain.SupplierGroup, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier group service is not configured")
	}
	if filter.SupplierID <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, badRequest("SUPPLIER_GROUP_STATUS_INVALID", "invalid supplier group status")
	}
	filter.Query = normalizeQuery(filter.Query)
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.List(ctx, filter)
}

func buildSupplierGroup(supplierID int64, supplierName string, in *ports.ProviderGroup, seenAt time.Time) *adminplusdomain.SupplierGroup {
	if in == nil {
		return nil
	}
	externalID := trimLimit(in.ExternalGroupID, 120)
	name := trimLimit(in.Name, 160)
	if externalID == "" || name == "" {
		return nil
	}
	status := adminplusdomain.NormalizeSupplierGroupStatus(in.Status)
	if status == "" {
		status = adminplusdomain.SupplierGroupStatusActive
	}
	if !status.Valid() {
		status = adminplusdomain.SupplierGroupStatusDisabled
	}
	rateMultiplier := in.RateMultiplier
	if rateMultiplier <= 0 {
		rateMultiplier = 1
	}
	effectiveRateMultiplier := in.EffectiveRateMultiplier
	if effectiveRateMultiplier <= 0 {
		effectiveRateMultiplier = rateMultiplier
	}
	group := &adminplusdomain.SupplierGroup{
		SupplierID:              supplierID,
		ExternalGroupID:         externalID,
		Name:                    name,
		Description:             trimLimit(in.Description, 500),
		ProviderFamily:          normalizeProviderFamily(in.ProviderFamily),
		RateMultiplier:          rateMultiplier,
		UserRateMultiplier:      in.UserRateMultiplier,
		EffectiveRateMultiplier: effectiveRateMultiplier,
		RPMLimit:                in.RPMLimit,
		DailyLimitUSD:           in.DailyLimitUSD,
		WeeklyLimitUSD:          in.WeeklyLimitUSD,
		MonthlyLimitUSD:         in.MonthlyLimitUSD,
		AllowImageGeneration:    in.AllowImageGeneration,
		IsPrivate:               in.IsPrivate,
		Status:                  status,
		RawPayload:              cloneMap(in.RawPayload),
		LastSeenAt:              seenAt,
		CreatedAt:               seenAt,
		UpdatedAt:               seenAt,
	}
	adminplusdomain.ApplySupplierGroupNaming(group, supplierName, seenAt)
	return group
}

func normalizeProviderFamily(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return "mixed"
	}
	return trimLimit(v, 60)
}

func normalizeQuery(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	return trimLimit(v, 120)
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

func trimLimit(value string, limit int) string {
	v := strings.TrimSpace(value)
	if len(v) <= limit {
		return v
	}
	return v[:limit]
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func badRequest(reason string, message string) error {
	return infraerrors.New(http.StatusBadRequest, reason, message)
}

func internalError(message string) error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", message)
}
