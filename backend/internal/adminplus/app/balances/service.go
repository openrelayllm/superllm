package balances

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type RecordSnapshotInput struct {
	SupplierID               int64
	Source                   string
	RuntimeStatus            adminplusdomain.SupplierRuntimeStatus
	BalanceCents             int64
	Currency                 string
	LowBalanceThresholdCents int64
	RawPayload               map[string]any
	CapturedAt               *time.Time
}

type SnapshotFilter struct {
	SupplierID int64
	Limit      int
}

type EventFilter struct {
	SupplierID int64
	Status     adminplusdomain.BalanceEventStatus
	Limit      int
}

type Repository interface {
	CreateSnapshot(ctx context.Context, snapshot *adminplusdomain.BalanceSnapshot) (*adminplusdomain.BalanceSnapshot, error)
	FindLatestSnapshot(ctx context.Context, supplierID int64, currency string, capturedAt time.Time) (*adminplusdomain.BalanceSnapshot, error)
	CreateEvent(ctx context.Context, event *adminplusdomain.BalanceEvent) (*adminplusdomain.BalanceEvent, error)
	ListSnapshots(ctx context.Context, filter SnapshotFilter) ([]*adminplusdomain.BalanceSnapshot, error)
	ListEvents(ctx context.Context, filter EventFilter) ([]*adminplusdomain.BalanceEvent, error)
	UpdateEventStatus(ctx context.Context, id int64, status adminplusdomain.BalanceEventStatus) (*adminplusdomain.BalanceEvent, error)
}

type Notifier interface {
	NotifyBalanceEvent(ctx context.Context, event *adminplusdomain.BalanceEvent, snapshot *adminplusdomain.BalanceSnapshot) error
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

func (s *Service) RecordSnapshot(ctx context.Context, in RecordSnapshotInput) (*adminplusdomain.BalanceEvent, *adminplusdomain.BalanceSnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, nil, internalError("balance service is not configured")
	}
	snapshot, err := s.buildSnapshot(in)
	if err != nil {
		return nil, nil, err
	}
	previous, err := s.repo.FindLatestSnapshot(ctx, snapshot.SupplierID, snapshot.Currency, snapshot.CapturedAt)
	if err != nil {
		return nil, nil, err
	}
	created, err := s.repo.CreateSnapshot(ctx, snapshot)
	if err != nil {
		return nil, nil, err
	}
	event := buildBalanceEvent(created, previous, in.LowBalanceThresholdCents)
	if event == nil {
		return nil, created, nil
	}
	createdEvent, err := s.repo.CreateEvent(ctx, event)
	if err != nil {
		return nil, nil, err
	}
	s.notifyBalanceEvent(ctx, createdEvent, created)
	return createdEvent, created, nil
}

func (s *Service) notifyBalanceEvent(ctx context.Context, event *adminplusdomain.BalanceEvent, snapshot *adminplusdomain.BalanceSnapshot) {
	if s == nil || s.notifier == nil || event == nil {
		return
	}
	if err := s.notifier.NotifyBalanceEvent(ctx, event, snapshot); err != nil {
		slog.Warn("admin plus balance notification failed", "supplier_id", event.SupplierID, "event_id", event.ID, "type", event.Type, "err", err)
	}
}

func (s *Service) ListSnapshots(ctx context.Context, filter SnapshotFilter) ([]*adminplusdomain.BalanceSnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("balance service is not configured")
	}
	if filter.SupplierID < 0 {
		return nil, badRequest("BALANCE_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListSnapshots(ctx, filter)
}

func (s *Service) ListEvents(ctx context.Context, filter EventFilter) ([]*adminplusdomain.BalanceEvent, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("balance service is not configured")
	}
	if filter.SupplierID < 0 {
		return nil, badRequest("BALANCE_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, badRequest("BALANCE_EVENT_STATUS_INVALID", "invalid balance event status")
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListEvents(ctx, filter)
}

func (s *Service) AcknowledgeEvent(ctx context.Context, id int64) (*adminplusdomain.BalanceEvent, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("balance service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("BALANCE_EVENT_ID_INVALID", "invalid balance event id")
	}
	return s.repo.UpdateEventStatus(ctx, id, adminplusdomain.BalanceEventStatusAcknowledged)
}

func (s *Service) buildSnapshot(in RecordSnapshotInput) (*adminplusdomain.BalanceSnapshot, error) {
	if in.SupplierID <= 0 {
		return nil, badRequest("BALANCE_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if in.BalanceCents < 0 {
		return nil, badRequest("BALANCE_AMOUNT_INVALID", "balance must be non-negative")
	}
	if in.LowBalanceThresholdCents < 0 {
		return nil, badRequest("BALANCE_THRESHOLD_INVALID", "low balance threshold must be non-negative")
	}
	runtimeStatus := in.RuntimeStatus
	if runtimeStatus == "" {
		runtimeStatus = adminplusdomain.SupplierRuntimeStatusMonitorOnly
	}
	if !runtimeStatus.Valid() {
		return nil, badRequest("BALANCE_RUNTIME_STATUS_INVALID", "invalid supplier runtime status")
	}
	capturedAt := s.now().UTC()
	if in.CapturedAt != nil {
		capturedAt = in.CapturedAt.UTC()
	}
	return &adminplusdomain.BalanceSnapshot{
		SupplierID:     in.SupplierID,
		Source:         normalizeSource(in.Source),
		RuntimeStatus:  runtimeStatus,
		BalanceCents:   in.BalanceCents,
		Currency:       normalizeCurrency(in.Currency),
		SwitchEligible: adminplusdomain.CanUseSupplierForSwitching(runtimeStatus, in.BalanceCents),
		RawPayload:     in.RawPayload,
		CapturedAt:     capturedAt,
	}, nil
}

func buildBalanceEvent(current, previous *adminplusdomain.BalanceSnapshot, thresholdCents int64) *adminplusdomain.BalanceEvent {
	if current == nil {
		return nil
	}
	event := &adminplusdomain.BalanceEvent{
		SupplierID:               current.SupplierID,
		SnapshotID:               current.ID,
		RuntimeStatus:            current.RuntimeStatus,
		NewBalanceCents:          current.BalanceCents,
		LowBalanceThresholdCents: thresholdCents,
		Currency:                 current.Currency,
		SwitchEligible:           current.SwitchEligible,
		Status:                   adminplusdomain.BalanceEventStatusOpen,
	}
	if previous != nil {
		oldBalance := previous.BalanceCents
		event.OldBalanceCents = &oldBalance
	}

	if current.BalanceCents == 0 {
		if previous == nil || previous.BalanceCents > 0 {
			event.Type = adminplusdomain.BalanceEventTypeDepleted
			return event
		}
		return nil
	}
	if previous != nil && previous.BalanceCents == 0 {
		event.Type = adminplusdomain.BalanceEventTypeRecovered
		return event
	}
	if thresholdCents > 0 && current.BalanceCents <= thresholdCents {
		if previous == nil || previous.BalanceCents > thresholdCents {
			event.Type = adminplusdomain.BalanceEventTypeLowBalance
			return event
		}
	}
	if previous != nil && thresholdCents > 0 && previous.BalanceCents <= thresholdCents && current.BalanceCents > thresholdCents {
		event.Type = adminplusdomain.BalanceEventTypeRecovered
		return event
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
