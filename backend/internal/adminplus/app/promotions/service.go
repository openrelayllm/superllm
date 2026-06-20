package promotions

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type RecordPromotionInput struct {
	SupplierID       int64
	Source           string
	Type             adminplusdomain.PromotionType
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
	Status         adminplusdomain.PromotionStatus
	Recommendation adminplusdomain.PromotionRecommendation
	Limit          int
}

type Repository interface {
	CreateEvent(ctx context.Context, event *adminplusdomain.PromotionEvent) (*adminplusdomain.PromotionEvent, error)
	ListEvents(ctx context.Context, filter EventFilter) ([]*adminplusdomain.PromotionEvent, error)
	UpdateEventStatus(ctx context.Context, id int64, status adminplusdomain.PromotionStatus) (*adminplusdomain.PromotionEvent, error)
}

type Notifier interface {
	NotifyPromotion(ctx context.Context, event *adminplusdomain.PromotionEvent) error
}

type Service struct {
	repo     Repository
	notifier Notifier
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

func (s *Service) RecordPromotion(ctx context.Context, in RecordPromotionInput) (*adminplusdomain.PromotionEvent, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("promotion service is not configured")
	}
	event, err := s.buildEvent(in)
	if err != nil {
		return nil, err
	}
	created, err := s.repo.CreateEvent(ctx, event)
	if err != nil {
		return nil, err
	}
	s.notifyPromotion(ctx, created)
	return created, nil
}

func (s *Service) notifyPromotion(ctx context.Context, event *adminplusdomain.PromotionEvent) {
	if s == nil || s.notifier == nil || event == nil {
		return
	}
	if err := s.notifier.NotifyPromotion(ctx, event); err != nil {
		slog.Warn("admin plus promotion notification failed", "supplier_id", event.SupplierID, "event_id", event.ID, "type", event.Type, "err", err)
	}
}

type FeishuNotifier struct {
	sender *notifications.Feishu
}

func NewFeishuNotifierFromEnv(repo notifications.Repository) *FeishuNotifier {
	sender := notifications.NewFeishuFromEnv(repo)
	if sender == nil {
		return nil
	}
	return &FeishuNotifier{sender: sender}
}

func (n *FeishuNotifier) NotifyPromotion(ctx context.Context, event *adminplusdomain.PromotionEvent) error {
	if n == nil || n.sender == nil || event == nil {
		return nil
	}
	return n.sender.SendEvent(ctx, notifications.Event{
		Type:           "promotion." + string(event.Type),
		ID:             event.ID,
		SupplierID:     event.SupplierID,
		ThrottleKey:    fmt.Sprintf("supplier:%d:type:%s:title:%s", event.SupplierID, event.Type, event.Title),
		ThrottleWindow: 6 * time.Hour,
		Text:           buildFeishuPromotionText(event),
	})
}

func buildFeishuPromotionText(event *adminplusdomain.PromotionEvent) string {
	bonus := "-"
	if event.BonusPercent != nil {
		bonus = fmt.Sprintf("%.2f%%", *event.BonusPercent)
	}
	discount := "-"
	if event.DiscountPercent != nil {
		discount = fmt.Sprintf("%.2f%%", *event.DiscountPercent)
	}
	return fmt.Sprintf(
		"【Sub2API Admin Plus 优惠通知】\n供应商ID：%d\n标题：%s\n类型：%s\n建议：%s\n最低充值：%s\n赠送比例：%s\n折扣比例：%s\n当前余额：%s\n可切换：%t\n来源：%s\n时间：%s",
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

func (s *Service) ListEvents(ctx context.Context, filter EventFilter) ([]*adminplusdomain.PromotionEvent, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("promotion service is not configured")
	}
	if filter.SupplierID < 0 {
		return nil, badRequest("PROMOTION_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, badRequest("PROMOTION_STATUS_INVALID", "invalid promotion status")
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListEvents(ctx, filter)
}

func (s *Service) AcknowledgeEvent(ctx context.Context, id int64) (*adminplusdomain.PromotionEvent, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("promotion service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("PROMOTION_EVENT_ID_INVALID", "invalid promotion event id")
	}
	return s.repo.UpdateEventStatus(ctx, id, adminplusdomain.PromotionStatusAcknowledged)
}

func (s *Service) buildEvent(in RecordPromotionInput) (*adminplusdomain.PromotionEvent, error) {
	if in.SupplierID <= 0 {
		return nil, badRequest("PROMOTION_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if !in.Type.Valid() {
		return nil, badRequest("PROMOTION_TYPE_INVALID", "invalid promotion type")
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, badRequest("PROMOTION_TITLE_REQUIRED", "promotion title is required")
	}
	if len(title) > 160 {
		return nil, badRequest("PROMOTION_TITLE_TOO_LONG", "promotion title must be 160 characters or less")
	}
	if in.MinRechargeCents < 0 {
		return nil, badRequest("PROMOTION_MIN_RECHARGE_INVALID", "minimum recharge must be non-negative")
	}
	if in.BalanceCents < 0 {
		return nil, badRequest("PROMOTION_BALANCE_INVALID", "balance must be non-negative")
	}
	if err := validatePercent("PROMOTION_BONUS_PERCENT_INVALID", in.BonusPercent); err != nil {
		return nil, err
	}
	if err := validatePercent("PROMOTION_DISCOUNT_PERCENT_INVALID", in.DiscountPercent); err != nil {
		return nil, err
	}
	if in.StartsAt != nil && in.EndsAt != nil && in.EndsAt.Before(*in.StartsAt) {
		return nil, badRequest("PROMOTION_TIME_RANGE_INVALID", "promotion end time must be after start time")
	}
	runtimeStatus := in.RuntimeStatus
	if runtimeStatus == "" {
		runtimeStatus = adminplusdomain.SupplierRuntimeStatusMonitorOnly
	}
	if !runtimeStatus.Valid() {
		return nil, badRequest("PROMOTION_RUNTIME_STATUS_INVALID", "invalid supplier runtime status")
	}
	capturedAt := s.now().UTC()
	if in.CapturedAt != nil {
		capturedAt = in.CapturedAt.UTC()
	}
	switchEligible := adminplusdomain.CanUseSupplierForSwitching(runtimeStatus, in.BalanceCents)
	return &adminplusdomain.PromotionEvent{
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
		Recommendation:   resolveRecommendation(runtimeStatus, in.BalanceCents, switchEligible),
		Status:           adminplusdomain.PromotionStatusOpen,
		StartsAt:         cloneTime(in.StartsAt),
		EndsAt:           cloneTime(in.EndsAt),
		CapturedAt:       capturedAt,
		RawPayload:       in.RawPayload,
	}, nil
}

func resolveRecommendation(status adminplusdomain.SupplierRuntimeStatus, balanceCents int64, switchEligible bool) adminplusdomain.PromotionRecommendation {
	if status == adminplusdomain.SupplierRuntimeStatusDisabled {
		return adminplusdomain.PromotionRecommendationInformational
	}
	if switchEligible {
		return adminplusdomain.PromotionRecommendationSwitchCandidate
	}
	if balanceCents <= 0 {
		return adminplusdomain.PromotionRecommendationRechargeToUnlock
	}
	return adminplusdomain.PromotionRecommendationMonitorOnly
}

func validatePercent(reason string, value *float64) error {
	if value == nil {
		return nil
	}
	if *value < 0 || *value > 100 {
		return badRequest(reason, "promotion percent must be between 0 and 100")
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
