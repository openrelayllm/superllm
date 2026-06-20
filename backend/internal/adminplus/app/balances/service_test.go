package balances

import (
	"context"
	"net/http"
	"testing"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServiceRecordSnapshotCreatesLowBalanceEvent(t *testing.T) {
	repo := newFakeBalanceRepository()
	notifier := &fakeBalanceNotifier{}
	svc := NewServiceWithNotifier(repo, notifier)
	capturedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	repo.snapshots = append(repo.snapshots, &adminplusdomain.BalanceSnapshot{
		ID:            1,
		SupplierID:    7,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusCandidate,
		BalanceCents:  3000,
		Currency:      "USD",
		CapturedAt:    capturedAt.Add(-time.Minute),
	})

	event, snapshot, err := svc.RecordSnapshot(context.Background(), RecordSnapshotInput{
		SupplierID:               7,
		RuntimeStatus:            adminplusdomain.SupplierRuntimeStatusCandidate,
		BalanceCents:             500,
		Currency:                 "usd",
		LowBalanceThresholdCents: 1000,
		CapturedAt:               &capturedAt,
	})

	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.NotNil(t, event)
	require.Equal(t, adminplusdomain.BalanceEventTypeLowBalance, event.Type)
	require.Equal(t, adminplusdomain.BalanceEventStatusOpen, event.Status)
	require.Equal(t, "USD", snapshot.Currency)
	require.True(t, snapshot.SwitchEligible)
	require.Len(t, notifier.events, 1)
	require.Equal(t, event.ID, notifier.events[0].ID)
}

func TestServiceRecordSnapshotIgnoresNotifierError(t *testing.T) {
	repo := newFakeBalanceRepository()
	notifier := &fakeBalanceNotifier{err: infraerrors.New(http.StatusBadGateway, "FEISHU_FAILED", "feishu failed")}
	svc := NewServiceWithNotifier(repo, notifier)
	capturedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	repo.snapshots = append(repo.snapshots, &adminplusdomain.BalanceSnapshot{
		ID:            1,
		SupplierID:    7,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusCandidate,
		BalanceCents:  3000,
		Currency:      "USD",
		CapturedAt:    capturedAt.Add(-time.Minute),
	})

	event, snapshot, err := svc.RecordSnapshot(context.Background(), RecordSnapshotInput{
		SupplierID:               7,
		RuntimeStatus:            adminplusdomain.SupplierRuntimeStatusCandidate,
		BalanceCents:             500,
		Currency:                 "USD",
		LowBalanceThresholdCents: 1000,
		CapturedAt:               &capturedAt,
	})

	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.NotNil(t, event)
	require.Len(t, notifier.events, 1)
}

func TestServiceRecordSnapshotCreatesDepletedAndRecoveredEvents(t *testing.T) {
	repo := newFakeBalanceRepository()
	svc := NewService(repo)
	capturedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	repo.snapshots = append(repo.snapshots, &adminplusdomain.BalanceSnapshot{
		ID:            1,
		SupplierID:    7,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusActive,
		BalanceCents:  1200,
		Currency:      "USD",
		CapturedAt:    capturedAt.Add(-time.Minute),
	})

	depleted, _, err := svc.RecordSnapshot(context.Background(), RecordSnapshotInput{
		SupplierID:    7,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusActive,
		BalanceCents:  0,
		Currency:      "USD",
		CapturedAt:    &capturedAt,
	})
	require.NoError(t, err)
	require.NotNil(t, depleted)
	require.Equal(t, adminplusdomain.BalanceEventTypeDepleted, depleted.Type)
	require.False(t, depleted.SwitchEligible)

	recoveredAt := capturedAt.Add(time.Minute)
	recovered, snapshot, err := svc.RecordSnapshot(context.Background(), RecordSnapshotInput{
		SupplierID:    7,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusActive,
		BalanceCents:  2000,
		Currency:      "USD",
		CapturedAt:    &recoveredAt,
	})
	require.NoError(t, err)
	require.NotNil(t, recovered)
	require.Equal(t, adminplusdomain.BalanceEventTypeRecovered, recovered.Type)
	require.True(t, snapshot.SwitchEligible)
}

func TestServiceRecordSnapshotDoesNotMakeMonitorOnlySwitchEligible(t *testing.T) {
	repo := newFakeBalanceRepository()
	svc := NewService(repo)

	event, snapshot, err := svc.RecordSnapshot(context.Background(), RecordSnapshotInput{
		SupplierID:    7,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		BalanceCents:  5000,
		Currency:      "USD",
	})

	require.NoError(t, err)
	require.Nil(t, event)
	require.NotNil(t, snapshot)
	require.False(t, snapshot.SwitchEligible)
}

func TestServiceRecordSnapshotValidatesInput(t *testing.T) {
	svc := NewService(newFakeBalanceRepository())

	_, _, err := svc.RecordSnapshot(context.Background(), RecordSnapshotInput{
		SupplierID:   0,
		BalanceCents: 1,
	})

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "BALANCE_SUPPLIER_ID_INVALID", infraerrors.Reason(err))
}

type fakeBalanceRepository struct {
	nextSnapshotID int64
	nextEventID    int64
	snapshots      []*adminplusdomain.BalanceSnapshot
	events         []*adminplusdomain.BalanceEvent
}

type fakeBalanceNotifier struct {
	err    error
	events []*adminplusdomain.BalanceEvent
}

func (n *fakeBalanceNotifier) NotifyBalanceEvent(_ context.Context, event *adminplusdomain.BalanceEvent, _ *adminplusdomain.BalanceSnapshot) error {
	n.events = append(n.events, cloneBalanceEvent(event))
	return n.err
}

func newFakeBalanceRepository() *fakeBalanceRepository {
	return &fakeBalanceRepository{
		nextSnapshotID: 1,
		nextEventID:    1,
	}
}

func (r *fakeBalanceRepository) CreateSnapshot(_ context.Context, snapshot *adminplusdomain.BalanceSnapshot) (*adminplusdomain.BalanceSnapshot, error) {
	cp := cloneBalanceSnapshot(snapshot)
	cp.ID = r.nextSnapshotID
	r.nextSnapshotID++
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = cp.CapturedAt
	}
	r.snapshots = append(r.snapshots, cp)
	return cloneBalanceSnapshot(cp), nil
}

func (r *fakeBalanceRepository) FindLatestSnapshot(_ context.Context, supplierID int64, currency string, capturedAt time.Time) (*adminplusdomain.BalanceSnapshot, error) {
	var latest *adminplusdomain.BalanceSnapshot
	for _, item := range r.snapshots {
		if item.SupplierID != supplierID || item.Currency != currency {
			continue
		}
		if item.CapturedAt.After(capturedAt) {
			continue
		}
		if latest == nil || item.CapturedAt.After(latest.CapturedAt) || (item.CapturedAt.Equal(latest.CapturedAt) && item.ID > latest.ID) {
			latest = item
		}
	}
	return cloneBalanceSnapshot(latest), nil
}

func (r *fakeBalanceRepository) CreateEvent(_ context.Context, event *adminplusdomain.BalanceEvent) (*adminplusdomain.BalanceEvent, error) {
	cp := cloneBalanceEvent(event)
	cp.ID = r.nextEventID
	r.nextEventID++
	r.events = append(r.events, cp)
	return cloneBalanceEvent(cp), nil
}

func (r *fakeBalanceRepository) ListSnapshots(_ context.Context, _ SnapshotFilter) ([]*adminplusdomain.BalanceSnapshot, error) {
	items := make([]*adminplusdomain.BalanceSnapshot, 0, len(r.snapshots))
	for _, item := range r.snapshots {
		items = append(items, cloneBalanceSnapshot(item))
	}
	return items, nil
}

func (r *fakeBalanceRepository) ListEvents(_ context.Context, _ EventFilter) ([]*adminplusdomain.BalanceEvent, error) {
	items := make([]*adminplusdomain.BalanceEvent, 0, len(r.events))
	for _, item := range r.events {
		items = append(items, cloneBalanceEvent(item))
	}
	return items, nil
}

func (r *fakeBalanceRepository) UpdateEventStatus(_ context.Context, id int64, status adminplusdomain.BalanceEventStatus) (*adminplusdomain.BalanceEvent, error) {
	for _, event := range r.events {
		if event.ID == id {
			event.Status = status
			return cloneBalanceEvent(event), nil
		}
	}
	return nil, nil
}

func cloneBalanceSnapshot(in *adminplusdomain.BalanceSnapshot) *adminplusdomain.BalanceSnapshot {
	if in == nil {
		return nil
	}
	out := *in
	if in.RawPayload != nil {
		out.RawPayload = make(map[string]any, len(in.RawPayload))
		for k, v := range in.RawPayload {
			out.RawPayload[k] = v
		}
	}
	return &out
}

func cloneBalanceEvent(in *adminplusdomain.BalanceEvent) *adminplusdomain.BalanceEvent {
	if in == nil {
		return nil
	}
	out := *in
	if in.OldBalanceCents != nil {
		v := *in.OldBalanceCents
		out.OldBalanceCents = &v
	}
	if in.AcknowledgedAt != nil {
		t := *in.AcknowledgedAt
		out.AcknowledgedAt = &t
	}
	return &out
}
