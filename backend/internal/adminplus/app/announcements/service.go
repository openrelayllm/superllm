package announcements

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type RecordAnnouncementInput struct {
	SupplierID       int64
	Source           string
	Type             adminplusdomain.AnnouncementType
	Title            string
	Description      string
	Currency         string
	MinRechargeCents int64
	BonusPercent     *float64
	DiscountPercent  *float64
	RuntimeStatus    adminplusdomain.SupplierRuntimeStatus
	BalanceCents     int64
	StartsAt         *time.Time
	EndsAt           *time.Time
	CapturedAt       *time.Time
	RawPayload       map[string]any
}

type EventFilter struct {
	SupplierID     int64
	Status         adminplusdomain.AnnouncementStatus
	Recommendation adminplusdomain.AnnouncementRecommendation
	Limit          int
}

type SyncFromSessionInput struct {
	SupplierID int64
}

type SyncFromSessionResult struct {
	SupplierID int64                                `json:"supplier_id"`
	SystemType string                               `json:"system_type"`
	Origin     string                               `json:"origin"`
	APIBaseURL string                               `json:"api_base_url"`
	SyncedAt   time.Time                            `json:"synced_at"`
	Total      int                                  `json:"total"`
	Events     []*adminplusdomain.AnnouncementEvent `json:"events"`
}

type Repository interface {
	CreateEvent(ctx context.Context, event *adminplusdomain.AnnouncementEvent) (*adminplusdomain.AnnouncementEvent, error)
	ListEvents(ctx context.Context, filter EventFilter) ([]*adminplusdomain.AnnouncementEvent, error)
	UpdateEventStatus(ctx context.Context, id int64, status adminplusdomain.AnnouncementStatus) (*adminplusdomain.AnnouncementEvent, error)
}

type Notifier interface {
	NotifyAnnouncement(ctx context.Context, event *adminplusdomain.AnnouncementEvent) error
}

type SessionReader interface {
	DecryptedProbeInput(ctx context.Context, supplierID int64) (ports.SessionProbeInput, error)
}

type Service struct {
	repo     Repository
	notifier Notifier
	session  SessionReader
	reader   ports.SessionAnnouncementAdapter
	now      func() time.Time
}

func NewService(repo Repository) *Service {
	return NewServiceWithNotifier(repo, nil)
}

func NewServiceWithNotifier(repo Repository, notifier Notifier) *Service {
	return &Service{
		repo:     repo,
		notifier: notifier,
		now:      time.Now,
	}
}

func NewServiceWithDependencies(repo Repository, notifier Notifier, session SessionReader, reader ports.SessionAnnouncementAdapter) *Service {
	service := NewServiceWithNotifier(repo, notifier)
	service.session = session
	service.reader = reader
	return service
}

func (s *Service) RecordAnnouncement(ctx context.Context, in RecordAnnouncementInput) (*adminplusdomain.AnnouncementEvent, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("announcement service is not configured")
	}
	event, err := s.buildEvent(in)
	if err != nil {
		return nil, err
	}
	created, err := s.repo.CreateEvent(ctx, event)
	if err != nil {
		return nil, err
	}
	s.notifyAnnouncement(ctx, created)
	return created, nil
}

func (s *Service) notifyAnnouncement(ctx context.Context, event *adminplusdomain.AnnouncementEvent) {
	if s == nil || s.notifier == nil || event == nil {
		return
	}
	if err := s.notifier.NotifyAnnouncement(ctx, event); err != nil {
		slog.Warn("admin plus announcement notification failed", "supplier_id", event.SupplierID, "event_id", event.ID, "type", event.Type, "err", err)
	}
}

func (s *Service) SyncFromSession(ctx context.Context, in SyncFromSessionInput) (*SyncFromSessionResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("announcement service is not configured")
	}
	if s.session == nil {
		return nil, internalError("supplier browser session service is not configured")
	}
	if s.reader == nil {
		return nil, internalError("supplier announcement provider adapter is not configured")
	}
	if in.SupplierID <= 0 {
		return nil, badRequest("ANNOUNCEMENT_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	probeInput, err := s.session.DecryptedProbeInput(ctx, in.SupplierID)
	if err != nil {
		return nil, err
	}
	readResult, err := s.reader.ReadAnnouncements(ctx, probeInput)
	if err != nil {
		return nil, err
	}
	capturedAt := readResult.CapturedAt.UTC()
	if capturedAt.IsZero() {
		capturedAt = s.now().UTC()
	}
	events := make([]*adminplusdomain.AnnouncementEvent, 0, len(readResult.Announcements))
	for _, item := range readResult.Announcements {
		event, err := s.RecordAnnouncement(ctx, RecordAnnouncementInput{
			SupplierID:       in.SupplierID,
			Source:           "provider_session",
			Type:             item.Type,
			Title:            item.Title,
			Description:      item.Description,
			Currency:         item.Currency,
			MinRechargeCents: item.MinRechargeCents,
			BonusPercent:     item.BonusPercent,
			DiscountPercent:  item.DiscountPercent,
			RuntimeStatus:    item.RuntimeStatus,
			BalanceCents:     item.BalanceCents,
			StartsAt:         item.StartsAt,
			EndsAt:           item.EndsAt,
			CapturedAt:       &capturedAt,
			RawPayload:       item.RawPayload,
		})
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return &SyncFromSessionResult{
		SupplierID: in.SupplierID,
		SystemType: readResult.SystemType,
		Origin:     readResult.Origin,
		APIBaseURL: readResult.APIBaseURL,
		SyncedAt:   capturedAt,
		Total:      len(events),
		Events:     events,
	}, nil
}

type FeishuNotifier struct {
	service *notifications.Service
}

func NewFeishuNotifierFromEnv(repo notifications.Repository) *FeishuNotifier {
	if repo == nil {
		return nil
	}
	return &FeishuNotifier{service: notifications.NewService(repo)}
}

func NewFeishuNotifier(service *notifications.Service) *FeishuNotifier {
	if service == nil {
		return nil
	}
	return &FeishuNotifier{service: service}
}

func (n *FeishuNotifier) NotifyAnnouncement(ctx context.Context, event *adminplusdomain.AnnouncementEvent) error {
	if n == nil || n.service == nil || event == nil {
		return nil
	}
	return n.service.Dispatch(ctx, notifications.DispatchInput{
		Type:           "announcement." + string(event.Type),
		ID:             event.ID,
		SupplierID:     event.SupplierID,
		ThrottleKey:    fmt.Sprintf("supplier:%d:type:%s:title:%s", event.SupplierID, event.Type, event.Title),
		ThrottleWindow: 6 * time.Hour,
		Text:           buildFeishuAnnouncementText(event),
	})
}

func buildFeishuAnnouncementText(event *adminplusdomain.AnnouncementEvent) string {
	bonus := "-"
	if event.BonusPercent != nil {
		bonus = fmt.Sprintf("%.2f%%", *event.BonusPercent)
	}
	discount := "-"
	if event.DiscountPercent != nil {
		discount = fmt.Sprintf("%.2f%%", *event.DiscountPercent)
	}
	return fmt.Sprintf(
		"【Sub2API Admin Plus 公告监控】\n供应商ID：%d\n标题：%s\n分类：%s\n建议：%s\n最低充值：%s\n赠送比例：%s\n折扣比例：%s\n当前余额：%s\n可切换：%t\n来源：%s\n时间：%s",
		event.SupplierID,
		event.Title,
		event.Type,
		event.Recommendation,
		formatCents(event.MinRechargeCents, event.Currency),
		bonus,
		discount,
		formatCents(event.BalanceCents, event.Currency),
		event.SwitchEligible,
		event.Source,
		event.CapturedAt.Format(time.RFC3339),
	)
}

func formatCents(cents int64, currency string) string {
	return fmt.Sprintf("%.2f %s", float64(cents)/100, strings.ToUpper(strings.TrimSpace(currency)))
}

func (s *Service) ListEvents(ctx context.Context, filter EventFilter) ([]*adminplusdomain.AnnouncementEvent, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("announcement service is not configured")
	}
	if filter.SupplierID < 0 {
		return nil, badRequest("ANNOUNCEMENT_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, badRequest("ANNOUNCEMENT_STATUS_INVALID", "invalid announcement status")
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListEvents(ctx, filter)
}

func (s *Service) AcknowledgeEvent(ctx context.Context, id int64) (*adminplusdomain.AnnouncementEvent, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("announcement service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("ANNOUNCEMENT_EVENT_ID_INVALID", "invalid announcement event id")
	}
	return s.repo.UpdateEventStatus(ctx, id, adminplusdomain.AnnouncementStatusAcknowledged)
}

func (s *Service) buildEvent(in RecordAnnouncementInput) (*adminplusdomain.AnnouncementEvent, error) {
	if in.SupplierID <= 0 {
		return nil, badRequest("ANNOUNCEMENT_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if !in.Type.Valid() {
		return nil, badRequest("ANNOUNCEMENT_TYPE_INVALID", "invalid announcement type")
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, badRequest("ANNOUNCEMENT_TITLE_REQUIRED", "announcement title is required")
	}
	if len(title) > 160 {
		return nil, badRequest("ANNOUNCEMENT_TITLE_TOO_LONG", "announcement title must be 160 characters or less")
	}
	if in.MinRechargeCents < 0 {
		return nil, badRequest("ANNOUNCEMENT_MIN_RECHARGE_INVALID", "minimum recharge must be non-negative")
	}
	if in.BalanceCents < 0 {
		return nil, badRequest("ANNOUNCEMENT_BALANCE_INVALID", "balance must be non-negative")
	}
	if err := validatePercent("ANNOUNCEMENT_BONUS_PERCENT_INVALID", in.BonusPercent); err != nil {
		return nil, err
	}
	if err := validatePercent("ANNOUNCEMENT_DISCOUNT_PERCENT_INVALID", in.DiscountPercent); err != nil {
		return nil, err
	}
	if in.StartsAt != nil && in.EndsAt != nil && in.EndsAt.Before(*in.StartsAt) {
		return nil, badRequest("ANNOUNCEMENT_TIME_RANGE_INVALID", "announcement end time must be after start time")
	}
	runtimeStatus := in.RuntimeStatus
	if runtimeStatus == "" {
		runtimeStatus = adminplusdomain.SupplierRuntimeStatusMonitorOnly
	}
	if !runtimeStatus.Valid() {
		return nil, badRequest("ANNOUNCEMENT_RUNTIME_STATUS_INVALID", "invalid supplier runtime status")
	}
	capturedAt := s.now().UTC()
	if in.CapturedAt != nil {
		capturedAt = in.CapturedAt.UTC()
	}
	switchEligible := in.Type.IsCostAnnouncement() && adminplusdomain.CanUseSupplierForSwitching(runtimeStatus, in.BalanceCents)
	return &adminplusdomain.AnnouncementEvent{
		SupplierID:       in.SupplierID,
		Source:           normalizeSource(in.Source),
		Type:             in.Type,
		Title:            title,
		Description:      trimLimit(in.Description, 2000),
		Currency:         normalizeCurrency(in.Currency),
		MinRechargeCents: in.MinRechargeCents,
		BonusPercent:     cloneFloat64(in.BonusPercent),
		DiscountPercent:  cloneFloat64(in.DiscountPercent),
		RuntimeStatus:    runtimeStatus,
		BalanceCents:     in.BalanceCents,
		SwitchEligible:   switchEligible,
		Recommendation:   resolveRecommendation(in.Type, runtimeStatus, in.BalanceCents, switchEligible),
		Status:           adminplusdomain.AnnouncementStatusOpen,
		StartsAt:         cloneTime(in.StartsAt),
		EndsAt:           cloneTime(in.EndsAt),
		CapturedAt:       capturedAt,
		RawPayload:       in.RawPayload,
	}, nil
}

func resolveRecommendation(eventType adminplusdomain.AnnouncementType, status adminplusdomain.SupplierRuntimeStatus, balanceCents int64, switchEligible bool) adminplusdomain.AnnouncementRecommendation {
	if status == adminplusdomain.SupplierRuntimeStatusDisabled || !eventType.IsCostAnnouncement() {
		return adminplusdomain.AnnouncementRecommendationInformational
	}
	if switchEligible {
		return adminplusdomain.AnnouncementRecommendationSwitchCandidate
	}
	if balanceCents <= 0 {
		return adminplusdomain.AnnouncementRecommendationRechargeToUnlock
	}
	return adminplusdomain.AnnouncementRecommendationMonitorOnly
}

func validatePercent(reason string, value *float64) error {
	if value == nil {
		return nil
	}
	if *value < 0 || *value > 100 {
		return badRequest(reason, "announcement percent must be between 0 and 100")
	}
	return nil
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

func cloneFloat64(in *float64) *float64 {
	if in == nil {
		return nil
	}
	v := *in
	return &v
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
