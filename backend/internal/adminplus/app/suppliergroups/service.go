package suppliergroups

import (
	"context"
	"math"
	"net/http"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const lowRateOpportunityThreshold = 0.06

type SyncResult struct {
	SupplierID int64                                       `json:"supplier_id"`
	SystemType string                                      `json:"system_type"`
	Origin     string                                      `json:"origin"`
	APIBaseURL string                                      `json:"api_base_url"`
	Groups     []*adminplusdomain.SupplierGroup            `json:"groups"`
	Events     []*adminplusdomain.SupplierGroupChangeEvent `json:"events,omitempty"`
	SyncedAt   time.Time                                   `json:"synced_at"`
	Total      int                                         `json:"total"`
}

type ListFilter struct {
	SupplierID int64
	Status     adminplusdomain.SupplierGroupStatus
	Query      string
	Limit      int
}

type EventFilter struct {
	SupplierID int64
	Direction  adminplusdomain.SupplierGroupChangeDirection
	LowRate    *bool
	Limit      int
}

type UpdateKeyCapacityInput struct {
	SupplierID      int64  `json:"supplier_id"`
	SupplierGroupID int64  `json:"supplier_group_id"`
	KeyLimitPolicy  string `json:"key_limit_policy"`
	KeyLimitValue   int    `json:"key_limit_value"`
}

type Repository interface {
	GetSupplierName(ctx context.Context, supplierID int64) (string, error)
	UpsertMany(ctx context.Context, supplierID int64, groups []*adminplusdomain.SupplierGroup, seenAt time.Time) ([]*adminplusdomain.SupplierGroup, error)
	List(ctx context.Context, filter ListFilter) ([]*adminplusdomain.SupplierGroup, error)
	UpdateKeyCapacity(ctx context.Context, in UpdateKeyCapacityInput) (*adminplusdomain.SupplierGroup, error)
	ListChangeEvents(ctx context.Context, filter EventFilter) ([]*adminplusdomain.SupplierGroupChangeEvent, error)
	CreateChangeEvents(ctx context.Context, events []*adminplusdomain.SupplierGroupChangeEvent) ([]*adminplusdomain.SupplierGroupChangeEvent, error)
}

type Notifier interface {
	NotifyGroupChange(ctx context.Context, event *adminplusdomain.SupplierGroupChangeEvent) error
}

type SessionReader interface {
	DecryptedProbeInput(ctx context.Context, supplierID int64) (ports.SessionProbeInput, error)
}

type Service struct {
	repo     Repository
	notifier Notifier
	session  SessionReader
	reader   ports.SessionGroupAdapter
	now      func() time.Time
}

func NewService(repo Repository, session SessionReader, reader ports.SessionGroupAdapter) *Service {
	return NewServiceWithNotifier(repo, nil, session, reader)
}

func NewServiceWithNotifier(repo Repository, notifier Notifier, session SessionReader, reader ports.SessionGroupAdapter) *Service {
	return &Service{
		repo:     repo,
		notifier: notifier,
		session:  session,
		reader:   reader,
		now:      time.Now,
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
	previous, err := s.repo.List(ctx, ListFilter{SupplierID: supplierID, Limit: 1000})
	if err != nil {
		return nil, err
	}
	saved, err := s.repo.UpsertMany(ctx, supplierID, groups, seenAt)
	if err != nil {
		return nil, err
	}
	events, err := s.recordChangeEvents(ctx, previous, saved, seenAt)
	if err != nil {
		return nil, err
	}
	s.notifyChangeEvents(ctx, events)
	return &SyncResult{
		SupplierID: supplierID,
		SystemType: result.SystemType,
		Origin:     result.Origin,
		APIBaseURL: result.APIBaseURL,
		Groups:     saved,
		Events:     events,
		SyncedAt:   seenAt,
		Total:      len(saved),
	}, nil
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]*adminplusdomain.SupplierGroup, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier group service is not configured")
	}
	if filter.SupplierID < 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, badRequest("SUPPLIER_GROUP_STATUS_INVALID", "invalid supplier group status")
	}
	filter.Query = normalizeQuery(filter.Query)
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.List(ctx, filter)
}

func (s *Service) ListChangeEvents(ctx context.Context, filter EventFilter) ([]*adminplusdomain.SupplierGroupChangeEvent, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier group service is not configured")
	}
	if filter.SupplierID <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if filter.Direction != "" && !filter.Direction.Valid() {
		return nil, badRequest("SUPPLIER_GROUP_EVENT_DIRECTION_INVALID", "invalid supplier group event direction")
	}
	filter.Limit = normalizeEventLimit(filter.Limit)
	return s.repo.ListChangeEvents(ctx, filter)
}

func (s *Service) UpdateKeyCapacity(ctx context.Context, in UpdateKeyCapacityInput) (*adminplusdomain.SupplierGroup, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier group service is not configured")
	}
	if in.SupplierID <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if in.SupplierGroupID <= 0 {
		return nil, badRequest("SUPPLIER_GROUP_ID_INVALID", "invalid supplier group id")
	}
	policy := normalizeGroupKeyLimitPolicy(in.KeyLimitPolicy)
	value := in.KeyLimitValue
	if policy == adminplusdomain.SupplierGroupKeyLimitPolicyLimited {
		if value <= 0 {
			return nil, badRequest("SUPPLIER_GROUP_KEY_LIMIT_VALUE_INVALID", "limited group key capacity requires a positive limit")
		}
	} else {
		value = 0
	}
	return s.repo.UpdateKeyCapacity(ctx, UpdateKeyCapacityInput{
		SupplierID:      in.SupplierID,
		SupplierGroupID: in.SupplierGroupID,
		KeyLimitPolicy:  policy,
		KeyLimitValue:   value,
	})
}

func (s *Service) recordChangeEvents(ctx context.Context, previous []*adminplusdomain.SupplierGroup, current []*adminplusdomain.SupplierGroup, createdAt time.Time) ([]*adminplusdomain.SupplierGroupChangeEvent, error) {
	events := buildChangeEvents(previous, current, createdAt)
	if len(events) == 0 {
		return nil, nil
	}
	return s.repo.CreateChangeEvents(ctx, events)
}

func (s *Service) notifyChangeEvents(ctx context.Context, events []*adminplusdomain.SupplierGroupChangeEvent) {
	if s == nil || s.notifier == nil {
		return
	}
	for _, event := range events {
		if event == nil {
			continue
		}
		_ = s.notifier.NotifyGroupChange(ctx, event)
	}
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

func buildChangeEvents(previous []*adminplusdomain.SupplierGroup, current []*adminplusdomain.SupplierGroup, createdAt time.Time) []*adminplusdomain.SupplierGroupChangeEvent {
	previousByExternalID := make(map[string]*adminplusdomain.SupplierGroup, len(previous))
	for _, group := range previous {
		if group == nil {
			continue
		}
		previousByExternalID[group.ExternalGroupID] = group
	}
	events := make([]*adminplusdomain.SupplierGroupChangeEvent, 0)
	for _, group := range current {
		if group == nil || group.ID <= 0 {
			continue
		}
		previousGroup := previousByExternalID[group.ExternalGroupID]
		if previousGroup == nil || previousGroup.Status == adminplusdomain.SupplierGroupStatusMissing {
			events = append(events, newChangeEvent(group, nil, adminplusdomain.SupplierGroupChangeDirectionNew, createdAt))
			continue
		}
		oldRate := effectiveRate(previousGroup)
		newRate := effectiveRate(group)
		if oldRate <= 0 || newRate <= 0 || math.Abs(newRate-oldRate) < 0.000001 {
			continue
		}
		direction := adminplusdomain.SupplierGroupChangeDirectionDecrease
		if newRate > oldRate {
			direction = adminplusdomain.SupplierGroupChangeDirectionIncrease
		}
		events = append(events, newChangeEvent(group, &oldRate, direction, createdAt))
	}
	return events
}

func newChangeEvent(group *adminplusdomain.SupplierGroup, oldRate *float64, direction adminplusdomain.SupplierGroupChangeDirection, createdAt time.Time) *adminplusdomain.SupplierGroupChangeEvent {
	newRate := effectiveRate(group)
	var changePercent *float64
	if oldRate != nil && *oldRate > 0 {
		value := (newRate - *oldRate) / *oldRate * 100
		changePercent = &value
	}
	return &adminplusdomain.SupplierGroupChangeEvent{
		SupplierID:                 group.SupplierID,
		SupplierGroupID:            group.ID,
		ExternalGroupID:            group.ExternalGroupID,
		GroupName:                  group.Name,
		ProviderFamily:             group.ProviderFamily,
		Direction:                  direction,
		OldEffectiveRateMultiplier: oldRate,
		NewEffectiveRateMultiplier: newRate,
		ChangePercent:              changePercent,
		LowRate:                    isOpenAIGroup(group) && newRate > 0 && newRate < lowRateOpportunityThreshold,
		CreatedAt:                  createdAt,
	}
}

func isOpenAIGroup(group *adminplusdomain.SupplierGroup) bool {
	if group == nil {
		return false
	}
	haystack := strings.ToLower(strings.Join([]string{
		group.ProviderFamily,
		group.Name,
		group.Description,
		group.OfficialName,
		group.ModelFamily,
		group.ModelSpec,
	}, " "))
	return strings.Contains(haystack, "openai") || strings.Contains(haystack, "gpt")
}

func effectiveRate(group *adminplusdomain.SupplierGroup) float64 {
	if group == nil {
		return 0
	}
	if group.EffectiveRateMultiplier > 0 {
		return group.EffectiveRateMultiplier
	}
	if group.UserRateMultiplier != nil && *group.UserRateMultiplier > 0 {
		return *group.UserRateMultiplier
	}
	if group.RateMultiplier > 0 {
		return group.RateMultiplier
	}
	return 0
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

func normalizeEventLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func normalizeGroupKeyLimitPolicy(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case adminplusdomain.SupplierGroupKeyLimitPolicyUnknown:
		return adminplusdomain.SupplierGroupKeyLimitPolicyUnknown
	case adminplusdomain.SupplierGroupKeyLimitPolicyUnlimited:
		return adminplusdomain.SupplierGroupKeyLimitPolicyUnlimited
	case adminplusdomain.SupplierGroupKeyLimitPolicyLimited:
		return adminplusdomain.SupplierGroupKeyLimitPolicyLimited
	case adminplusdomain.SupplierGroupKeyLimitPolicyUnsupported:
		return adminplusdomain.SupplierGroupKeyLimitPolicyUnsupported
	default:
		return adminplusdomain.SupplierGroupKeyLimitPolicyInherit
	}
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
