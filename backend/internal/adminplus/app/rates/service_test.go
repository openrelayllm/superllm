package rates

import (
	"context"
	"net/http"
	"testing"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServiceRecordSnapshotCreatesNewEventForFirstRate(t *testing.T) {
	repo := newFakeRateRepository()
	notifier := &fakeRateNotifier{}
	svc := NewServiceWithNotifier(repo, notifier)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	result, err := svc.RecordSnapshot(context.Background(), RecordSnapshotInput{
		SupplierID: 1,
		Source:     "manual",
		Entries: []RateEntryInput{
			{
				Model:       "gpt-4o-mini",
				BillingMode: "token",
				PriceItem:   "input",
				Unit:        "1m_tokens",
				Currency:    "usd",
				PriceMicros: 150000,
			},
		},
	})

	require.NoError(t, err)
	require.Len(t, result.Snapshots, 1)
	require.Len(t, result.Events, 1)
	require.Equal(t, adminplusdomain.RateChangeDirectionNew, result.Events[0].Direction)
	require.True(t, result.Events[0].ThresholdExceeded)
	require.Equal(t, adminplusdomain.RateChangeStatusOpen, result.Events[0].Status)
	require.Equal(t, "USD", result.Snapshots[0].Currency)
	require.Len(t, notifier.events, 1)
	require.Equal(t, result.Events[0].ID, notifier.events[0].ID)
}

func TestServiceRecordSnapshotSkipsEventWhenPriceIsUnchanged(t *testing.T) {
	repo := newFakeRateRepository()
	svc := NewService(repo)
	capturedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	repo.snapshots = append(repo.snapshots, &adminplusdomain.RateSnapshot{
		ID:          1,
		SupplierID:  1,
		Model:       "gpt-4o-mini",
		BillingMode: "token",
		PriceItem:   "input",
		Unit:        "1m_tokens",
		Currency:    "USD",
		PriceMicros: 150000,
		CapturedAt:  capturedAt.Add(-time.Minute),
		CreatedAt:   capturedAt.Add(-time.Minute),
	})

	result, err := svc.RecordSnapshot(context.Background(), RecordSnapshotInput{
		SupplierID: 1,
		CapturedAt: &capturedAt,
		Entries: []RateEntryInput{
			{
				Model:       "gpt-4o-mini",
				BillingMode: "token",
				PriceItem:   "input",
				Unit:        "1m_tokens",
				Currency:    "USD",
				PriceMicros: 150000,
			},
		},
	})

	require.NoError(t, err)
	require.Len(t, result.Snapshots, 1)
	require.Empty(t, result.Events)
}

func TestServiceRecordSnapshotCalculatesIncreaseAndThreshold(t *testing.T) {
	repo := newFakeRateRepository()
	svc := NewService(repo)
	capturedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	repo.snapshots = append(repo.snapshots, &adminplusdomain.RateSnapshot{
		ID:          1,
		SupplierID:  1,
		Model:       "gpt-4o-mini",
		BillingMode: "token",
		PriceItem:   "input",
		Unit:        "1m_tokens",
		Currency:    "USD",
		PriceMicros: 100000,
		CapturedAt:  capturedAt.Add(-time.Minute),
		CreatedAt:   capturedAt.Add(-time.Minute),
	})

	result, err := svc.RecordSnapshot(context.Background(), RecordSnapshotInput{
		SupplierID:       1,
		CapturedAt:       &capturedAt,
		ThresholdPercent: 10,
		Entries: []RateEntryInput{
			{
				Model:       "gpt-4o-mini",
				BillingMode: "token",
				PriceItem:   "input",
				Unit:        "1m_tokens",
				Currency:    "USD",
				PriceMicros: 125000,
			},
		},
	})

	require.NoError(t, err)
	require.Len(t, result.Events, 1)
	event := result.Events[0]
	require.Equal(t, adminplusdomain.RateChangeDirectionIncrease, event.Direction)
	require.NotNil(t, event.OldPriceMicros)
	require.Equal(t, int64(100000), *event.OldPriceMicros)
	require.NotNil(t, event.ChangePercent)
	require.InDelta(t, 25.0, *event.ChangePercent, 0.001)
	require.True(t, event.ThresholdExceeded)
}

func TestServiceRecordSnapshotMarksBelowThresholdEvent(t *testing.T) {
	repo := newFakeRateRepository()
	svc := NewService(repo)
	capturedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	repo.snapshots = append(repo.snapshots, &adminplusdomain.RateSnapshot{
		ID:          1,
		SupplierID:  1,
		Model:       "gpt-4o-mini",
		BillingMode: "token",
		PriceItem:   "output",
		Unit:        "1m_tokens",
		Currency:    "USD",
		PriceMicros: 100000,
		CapturedAt:  capturedAt.Add(-time.Minute),
		CreatedAt:   capturedAt.Add(-time.Minute),
	})

	result, err := svc.RecordSnapshot(context.Background(), RecordSnapshotInput{
		SupplierID:       1,
		CapturedAt:       &capturedAt,
		ThresholdPercent: 10,
		Entries: []RateEntryInput{
			{
				Model:       "gpt-4o-mini",
				BillingMode: "token",
				PriceItem:   "output",
				Unit:        "1m_tokens",
				Currency:    "USD",
				PriceMicros: 95000,
			},
		},
	})

	require.NoError(t, err)
	require.Len(t, result.Events, 1)
	event := result.Events[0]
	require.Equal(t, adminplusdomain.RateChangeDirectionDecrease, event.Direction)
	require.NotNil(t, event.ChangePercent)
	require.InDelta(t, -5.0, *event.ChangePercent, 0.001)
	require.False(t, event.ThresholdExceeded)
}

func TestServiceRecordSnapshotValidatesInput(t *testing.T) {
	svc := NewService(newFakeRateRepository())

	_, err := svc.RecordSnapshot(context.Background(), RecordSnapshotInput{
		SupplierID: 0,
		Entries: []RateEntryInput{
			{Model: "gpt-4o-mini", BillingMode: "token", PriceItem: "input", Unit: "1m_tokens", PriceMicros: 1},
		},
	})

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "RATE_SUPPLIER_ID_INVALID", infraerrors.Reason(err))
}

type fakeRateRepository struct {
	nextSnapshotID int64
	nextEventID    int64
	snapshots      []*adminplusdomain.RateSnapshot
	events         []*adminplusdomain.RateChangeEvent
}

type fakeRateNotifier struct {
	events []*adminplusdomain.RateChangeEvent
}

func (n *fakeRateNotifier) NotifyRateChange(_ context.Context, event *adminplusdomain.RateChangeEvent, _ *adminplusdomain.RateSnapshot) error {
	n.events = append(n.events, cloneRateChangeEvent(event))
	return nil
}

func newFakeRateRepository() *fakeRateRepository {
	return &fakeRateRepository{
		nextSnapshotID: 1,
		nextEventID:    1,
	}
}

func (r *fakeRateRepository) CreateSnapshot(_ context.Context, snapshot *adminplusdomain.RateSnapshot) (*adminplusdomain.RateSnapshot, error) {
	cp := cloneRateSnapshot(snapshot)
	cp.ID = r.nextSnapshotID
	r.nextSnapshotID++
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = cp.CapturedAt
	}
	r.snapshots = append(r.snapshots, cp)
	return cloneRateSnapshot(cp), nil
}

func (r *fakeRateRepository) FindLatestComparableSnapshot(_ context.Context, snapshot *adminplusdomain.RateSnapshot) (*adminplusdomain.RateSnapshot, error) {
	var latest *adminplusdomain.RateSnapshot
	for _, item := range r.snapshots {
		if !sameRateDimension(item, snapshot) {
			continue
		}
		if item.CapturedAt.After(snapshot.CapturedAt) {
			continue
		}
		if latest == nil || item.CapturedAt.After(latest.CapturedAt) || (item.CapturedAt.Equal(latest.CapturedAt) && item.ID > latest.ID) {
			latest = item
		}
	}
	return cloneRateSnapshot(latest), nil
}

func (r *fakeRateRepository) CreateChangeEvent(_ context.Context, event *adminplusdomain.RateChangeEvent) (*adminplusdomain.RateChangeEvent, error) {
	cp := cloneRateChangeEvent(event)
	cp.ID = r.nextEventID
	r.nextEventID++
	r.events = append(r.events, cp)
	return cloneRateChangeEvent(cp), nil
}

func (r *fakeRateRepository) ListSnapshots(_ context.Context, _ SnapshotFilter) ([]*adminplusdomain.RateSnapshot, error) {
	items := make([]*adminplusdomain.RateSnapshot, 0, len(r.snapshots))
	for _, item := range r.snapshots {
		items = append(items, cloneRateSnapshot(item))
	}
	return items, nil
}

func (r *fakeRateRepository) ListChangeEvents(_ context.Context, _ EventFilter) ([]*adminplusdomain.RateChangeEvent, error) {
	items := make([]*adminplusdomain.RateChangeEvent, 0, len(r.events))
	for _, item := range r.events {
		items = append(items, cloneRateChangeEvent(item))
	}
	return items, nil
}

func (r *fakeRateRepository) UpdateChangeEventStatus(_ context.Context, id int64, status adminplusdomain.RateChangeStatus) (*adminplusdomain.RateChangeEvent, error) {
	for _, event := range r.events {
		if event.ID == id {
			event.Status = status
			return cloneRateChangeEvent(event), nil
		}
	}
	return nil, nil
}

func sameRateDimension(a, b *adminplusdomain.RateSnapshot) bool {
	return a.SupplierID == b.SupplierID &&
		a.Model == b.Model &&
		a.BillingMode == b.BillingMode &&
		a.PriceItem == b.PriceItem &&
		a.Unit == b.Unit &&
		a.Currency == b.Currency
}

func cloneRateSnapshot(in *adminplusdomain.RateSnapshot) *adminplusdomain.RateSnapshot {
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

func cloneRateChangeEvent(in *adminplusdomain.RateChangeEvent) *adminplusdomain.RateChangeEvent {
	if in == nil {
		return nil
	}
	out := *in
	if in.OldPriceMicros != nil {
		v := *in.OldPriceMicros
		out.OldPriceMicros = &v
	}
	if in.ChangePercent != nil {
		v := *in.ChangePercent
		out.ChangePercent = &v
	}
	if in.AcknowledgedAt != nil {
		t := *in.AcknowledgedAt
		out.AcknowledgedAt = &t
	}
	return &out
}
