package rates

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const defaultChangeThresholdPercent = 1.0

type RateEntryInput struct {
	Model       string
	BillingMode string
	PriceItem   string
	Unit        string
	Currency    string
	PriceMicros int64
	RawPayload  map[string]any
}

type RecordSnapshotInput struct {
	SupplierID       int64
	Source           string
	CapturedAt       *time.Time
	ThresholdPercent float64
	Entries          []RateEntryInput
}

type RecordSnapshotResult struct {
	Snapshots []*adminplusdomain.RateSnapshot    `json:"snapshots"`
	Events    []*adminplusdomain.RateChangeEvent `json:"events"`
}

type SyncFromSessionInput struct {
	SupplierID       int64
	ThresholdPercent float64
}

type SyncFromSessionResult struct {
	SupplierID int64                 `json:"supplier_id"`
	SystemType string                `json:"system_type"`
	Origin     string                `json:"origin"`
	APIBaseURL string                `json:"api_base_url"`
	SyncedAt   time.Time             `json:"synced_at"`
	Total      int                   `json:"total"`
	Snapshot   *RecordSnapshotResult `json:"snapshot"`
}

type SnapshotFilter struct {
	SupplierID int64
	Model      string
	Limit      int
}

type EventFilter struct {
	SupplierID int64
	Status     adminplusdomain.RateChangeStatus
	Limit      int
}

type Repository interface {
	CreateSnapshot(ctx context.Context, snapshot *adminplusdomain.RateSnapshot) (*adminplusdomain.RateSnapshot, error)
	FindLatestComparableSnapshot(ctx context.Context, snapshot *adminplusdomain.RateSnapshot) (*adminplusdomain.RateSnapshot, error)
	CreateChangeEvent(ctx context.Context, event *adminplusdomain.RateChangeEvent) (*adminplusdomain.RateChangeEvent, error)
	ListSnapshots(ctx context.Context, filter SnapshotFilter) ([]*adminplusdomain.RateSnapshot, error)
	ListChangeEvents(ctx context.Context, filter EventFilter) ([]*adminplusdomain.RateChangeEvent, error)
	UpdateChangeEventStatus(ctx context.Context, id int64, status adminplusdomain.RateChangeStatus) (*adminplusdomain.RateChangeEvent, error)
}

type Notifier interface {
	NotifyRateChange(ctx context.Context, event *adminplusdomain.RateChangeEvent, snapshot *adminplusdomain.RateSnapshot) error
}

type SessionReader interface {
	DecryptedProbeInput(ctx context.Context, supplierID int64) (ports.SessionProbeInput, error)
}

type Service struct {
	repo     Repository
	notifier Notifier
	session  SessionReader
	reader   ports.SessionRateAdapter
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

func NewServiceWithDependencies(repo Repository, notifier Notifier, session SessionReader, reader ports.SessionRateAdapter) *Service {
	service := NewServiceWithNotifier(repo, notifier)
	service.session = session
	service.reader = reader
	return service
}

func (s *Service) RecordSnapshot(ctx context.Context, in RecordSnapshotInput) (*RecordSnapshotResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("rate service is not configured")
	}
	if in.SupplierID <= 0 {
		return nil, badRequest("RATE_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if len(in.Entries) == 0 {
		return nil, badRequest("RATE_ENTRIES_REQUIRED", "rate entries are required")
	}
	if len(in.Entries) > 500 {
		return nil, badRequest("RATE_ENTRIES_TOO_MANY", "rate entries must be 500 or less")
	}
	source := normalizeSource(in.Source)
	threshold := in.ThresholdPercent
	if threshold <= 0 {
		threshold = defaultChangeThresholdPercent
	}
	capturedAt := s.now().UTC()
	if in.CapturedAt != nil {
		capturedAt = in.CapturedAt.UTC()
	}

	result := &RecordSnapshotResult{
		Snapshots: make([]*adminplusdomain.RateSnapshot, 0, len(in.Entries)),
		Events:    make([]*adminplusdomain.RateChangeEvent, 0),
	}
	for _, entry := range in.Entries {
		snapshot, err := buildSnapshot(in.SupplierID, source, capturedAt, entry)
		if err != nil {
			return nil, err
		}
		previous, err := s.repo.FindLatestComparableSnapshot(ctx, snapshot)
		if err != nil {
			return nil, err
		}
		created, err := s.repo.CreateSnapshot(ctx, snapshot)
		if err != nil {
			return nil, err
		}
		result.Snapshots = append(result.Snapshots, created)

		event := buildChangeEvent(created, previous, threshold)
		if event == nil {
			continue
		}
		createdEvent, err := s.repo.CreateChangeEvent(ctx, event)
		if err != nil {
			return nil, err
		}
		result.Events = append(result.Events, createdEvent)
		s.notifyRateChange(ctx, createdEvent, created)
	}
	return result, nil
}

func (s *Service) SyncFromSession(ctx context.Context, in SyncFromSessionInput) (*SyncFromSessionResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("rate service is not configured")
	}
	if s.session == nil {
		return nil, internalError("supplier browser session service is not configured")
	}
	if s.reader == nil {
		return nil, internalError("supplier rate provider adapter is not configured")
	}
	if in.SupplierID <= 0 {
		return nil, badRequest("RATE_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	probeInput, err := s.session.DecryptedProbeInput(ctx, in.SupplierID)
	if err != nil {
		return nil, err
	}
	readResult, err := s.reader.ReadRates(ctx, probeInput)
	if err != nil {
		return nil, err
	}
	entries := make([]RateEntryInput, 0, len(readResult.Entries))
	for _, entry := range readResult.Entries {
		entries = append(entries, RateEntryInput{
			Model:       entry.Model,
			BillingMode: entry.BillingMode,
			PriceItem:   entry.PriceItem,
			Unit:        entry.Unit,
			Currency:    entry.Currency,
			PriceMicros: entry.PriceMicros,
			RawPayload:  entry.RawPayload,
		})
	}
	capturedAt := readResult.CapturedAt.UTC()
	if capturedAt.IsZero() {
		capturedAt = s.now().UTC()
	}
	snapshot, err := s.RecordSnapshot(ctx, RecordSnapshotInput{
		SupplierID:       in.SupplierID,
		Source:           "provider_session",
		CapturedAt:       &capturedAt,
		ThresholdPercent: in.ThresholdPercent,
		Entries:          entries,
	})
	if err != nil {
		return nil, err
	}
	return &SyncFromSessionResult{
		SupplierID: in.SupplierID,
		SystemType: readResult.SystemType,
		Origin:     readResult.Origin,
		APIBaseURL: readResult.APIBaseURL,
		SyncedAt:   capturedAt,
		Total:      len(snapshot.Snapshots),
		Snapshot:   snapshot,
	}, nil
}

func (s *Service) notifyRateChange(_ context.Context, _ *adminplusdomain.RateChangeEvent, _ *adminplusdomain.RateSnapshot) {
	// Rate events stay in SuperLLM history; Feishu push is intentionally balance-only.
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

func (n *FeishuNotifier) NotifyRateChange(ctx context.Context, event *adminplusdomain.RateChangeEvent, snapshot *adminplusdomain.RateSnapshot) error {
	if n == nil || n.service == nil || event == nil {
		return nil
	}
	return n.service.Dispatch(ctx, notifications.DispatchInput{
		Type:           "rate." + string(event.Direction),
		ID:             event.ID,
		SupplierID:     event.SupplierID,
		ThrottleKey:    fmt.Sprintf("supplier:%d:model:%s:price:%s:%s:%s:direction:%s", event.SupplierID, event.Model, event.BillingMode, event.PriceItem, event.Unit, event.Direction),
		ThrottleWindow: notifications.DefaultThrottleWindow,
		Text:           buildFeishuRateText(event, snapshot),
	})
}

func buildFeishuRateText(event *adminplusdomain.RateChangeEvent, snapshot *adminplusdomain.RateSnapshot) string {
	oldPrice := "-"
	if event.OldPriceMicros != nil {
		oldPrice = fmt.Sprintf("%d micros", *event.OldPriceMicros)
	}
	change := "-"
	if event.ChangePercent != nil {
		change = fmt.Sprintf("%.2f%%", *event.ChangePercent)
	}
	source := "-"
	capturedAt := event.CreatedAt
	if snapshot != nil {
		source = snapshot.Source
		capturedAt = snapshot.CapturedAt
	}
	return fmt.Sprintf(
		"【SuperLLM 费率通知】\n供应商ID：%d\n模型：%s\n方向：%s\n价格项：%s/%s/%s\n旧价格：%s\n新价格：%d micros\n变化：%s\n超过阈值：%t\n来源：%s\n时间：%s",
		event.SupplierID,
		event.Model,
		event.Direction,
		event.BillingMode,
		event.PriceItem,
		event.Unit,
		oldPrice,
		event.NewPriceMicros,
		change,
		event.ThresholdExceeded,
		source,
		capturedAt.Format(time.RFC3339),
	)
}

func (s *Service) ListSnapshots(ctx context.Context, filter SnapshotFilter) ([]*adminplusdomain.RateSnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("rate service is not configured")
	}
	if filter.SupplierID < 0 {
		return nil, badRequest("RATE_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	filter.Model = strings.TrimSpace(filter.Model)
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListSnapshots(ctx, filter)
}

func (s *Service) ListChangeEvents(ctx context.Context, filter EventFilter) ([]*adminplusdomain.RateChangeEvent, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("rate service is not configured")
	}
	if filter.SupplierID < 0 {
		return nil, badRequest("RATE_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, badRequest("RATE_EVENT_STATUS_INVALID", "invalid rate event status")
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListChangeEvents(ctx, filter)
}

func (s *Service) AcknowledgeChangeEvent(ctx context.Context, id int64) (*adminplusdomain.RateChangeEvent, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("rate service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("RATE_EVENT_ID_INVALID", "invalid rate event id")
	}
	return s.repo.UpdateChangeEventStatus(ctx, id, adminplusdomain.RateChangeStatusAcknowledged)
}

func buildSnapshot(supplierID int64, source string, capturedAt time.Time, entry RateEntryInput) (*adminplusdomain.RateSnapshot, error) {
	model := strings.TrimSpace(entry.Model)
	if model == "" {
		return nil, badRequest("RATE_MODEL_REQUIRED", "rate model is required")
	}
	billingMode := strings.ToLower(strings.TrimSpace(entry.BillingMode))
	if billingMode == "" {
		return nil, badRequest("RATE_BILLING_MODE_REQUIRED", "rate billing mode is required")
	}
	priceItem := strings.ToLower(strings.TrimSpace(entry.PriceItem))
	if priceItem == "" {
		return nil, badRequest("RATE_PRICE_ITEM_REQUIRED", "rate price item is required")
	}
	unit := strings.ToLower(strings.TrimSpace(entry.Unit))
	if unit == "" {
		return nil, badRequest("RATE_UNIT_REQUIRED", "rate unit is required")
	}
	if entry.PriceMicros < 0 {
		return nil, badRequest("RATE_PRICE_INVALID", "rate price must be non-negative")
	}
	return &adminplusdomain.RateSnapshot{
		SupplierID:  supplierID,
		Source:      source,
		Model:       model,
		BillingMode: billingMode,
		PriceItem:   priceItem,
		Unit:        unit,
		Currency:    normalizeCurrency(entry.Currency),
		PriceMicros: entry.PriceMicros,
		RawPayload:  entry.RawPayload,
		CapturedAt:  capturedAt,
	}, nil
}

func buildChangeEvent(current, previous *adminplusdomain.RateSnapshot, thresholdPercent float64) *adminplusdomain.RateChangeEvent {
	if current == nil {
		return nil
	}
	event := &adminplusdomain.RateChangeEvent{
		SupplierID:       current.SupplierID,
		SnapshotID:       current.ID,
		Model:            current.Model,
		BillingMode:      current.BillingMode,
		PriceItem:        current.PriceItem,
		Unit:             current.Unit,
		Currency:         current.Currency,
		NewPriceMicros:   current.PriceMicros,
		ThresholdPercent: thresholdPercent,
		Status:           adminplusdomain.RateChangeStatusOpen,
	}
	if previous == nil {
		event.Direction = adminplusdomain.RateChangeDirectionNew
		event.ThresholdExceeded = true
		return event
	}
	if previous.PriceMicros == current.PriceMicros {
		return nil
	}
	oldPrice := previous.PriceMicros
	event.OldPriceMicros = &oldPrice
	if current.PriceMicros > previous.PriceMicros {
		event.Direction = adminplusdomain.RateChangeDirectionIncrease
	} else {
		event.Direction = adminplusdomain.RateChangeDirectionDecrease
	}
	if previous.PriceMicros != 0 {
		changePercent := (float64(current.PriceMicros-previous.PriceMicros) / math.Abs(float64(previous.PriceMicros))) * 100
		event.ChangePercent = &changePercent
		event.ThresholdExceeded = math.Abs(changePercent) >= thresholdPercent
	} else {
		event.ThresholdExceeded = true
	}
	return event
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
