package balances

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
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

func TestServiceRecordSnapshotConvertsLegacyNewAPIQuotaBalance(t *testing.T) {
	repo := newFakeBalanceRepository()
	svc := NewService(repo)

	event, snapshot, err := svc.RecordSnapshot(context.Background(), RecordSnapshotInput{
		SupplierID:               7,
		RuntimeStatus:            adminplusdomain.SupplierRuntimeStatusCandidate,
		BalanceCents:             892305600,
		Currency:                 "QTA",
		LowBalanceThresholdCents: 2000,
	})

	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.NotNil(t, event)
	require.Equal(t, int64(1785), snapshot.BalanceCents)
	require.Equal(t, "USD", snapshot.Currency)
	require.Equal(t, int64(1785), event.NewBalanceCents)
	require.Equal(t, "USD", event.Currency)
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

func TestServiceSyncFromSessionRecordsBalanceSnapshot(t *testing.T) {
	repo := newFakeBalanceRepository()
	probedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	balanceCents := int64(1234)
	session := &fakeBalanceSessionReader{input: ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     "https://relay.example.com",
		APIBaseURL: "https://relay.example.com/api/v1",
		Bundle:     map[string]any{"access_token": "browser-token"},
	}}
	reader := &fakeBalanceProbeAdapter{result: &ports.SessionProbeResult{
		SupplierID:      7,
		Status:          "ok",
		SystemType:      "sub2api",
		Origin:          "https://relay.example.com",
		APIBaseURL:      "https://relay.example.com/api/v1",
		BalanceCents:    &balanceCents,
		BalanceCurrency: "cny",
		Capabilities:    map[string]bool{"can_read_balance": true},
		ProbedAt:        probedAt,
	}}
	svc := NewServiceWithDependencies(repo, nil, session, reader)

	result, err := svc.SyncFromSession(context.Background(), SyncFromSessionInput{
		SupplierID:               7,
		LowBalanceThresholdCents: 2000,
	})

	require.NoError(t, err)
	require.Equal(t, int64(7), session.supplierID)
	require.Equal(t, "sub2api", result.SystemType)
	require.Equal(t, probedAt, result.SyncedAt)
	require.NotNil(t, result.Snapshot)
	require.NotNil(t, result.Event)
	require.Equal(t, int64(1234), result.Snapshot.BalanceCents)
	require.Equal(t, "USD", result.Snapshot.Currency)
	require.Equal(t, adminplusdomain.BalanceEventTypeLowBalance, result.Event.Type)
}

func TestServiceSyncFromSessionAllowsProbeWithoutBalance(t *testing.T) {
	probedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	session := &fakeBalanceSessionReader{}
	reader := &fakeBalanceProbeAdapter{result: &ports.SessionProbeResult{
		SupplierID:   7,
		Status:       "ok",
		SystemType:   "sub2api",
		Capabilities: map[string]bool{"can_read_balance": false},
		ProbedAt:     probedAt,
	}}
	svc := NewServiceWithDependencies(newFakeBalanceRepository(), nil, session, reader)

	result, err := svc.SyncFromSession(context.Background(), SyncFromSessionInput{SupplierID: 7})

	require.NoError(t, err)
	require.Equal(t, probedAt, result.SyncedAt)
	require.NotNil(t, result.Probe)
	require.Nil(t, result.Snapshot)
	require.Nil(t, result.Event)
}

func TestServiceSyncFromSessionFailureRecordsDiagnostics(t *testing.T) {
	writer := &balanceLogWriter{}
	readerErr := infraerrors.New(http.StatusBadGateway, "SUPPLIER_SESSION_BAD_STATUS", "supplier session endpoint returned non-success status").
		WithMetadata(map[string]string{
			"endpoint":     "https://supplier.example/api/v1/user/profile",
			"status_code":  "502",
			"content_type": "application/json",
			"body_type":    "json",
			"body_excerpt": `{"message":"bad gateway"}`,
		})
	svc := NewServiceWithDependencies(
		newFakeBalanceRepository(),
		nil,
		&fakeBalanceSessionReader{input: ports.SessionProbeInput{
			SupplierID: 7,
			Origin:     "https://supplier.example",
			APIBaseURL: "https://supplier.example/api/v1",
		}},
		&fakeBalanceProbeAdapter{err: readerErr},
	).WithDiagnostics(bizlogs.NewRecorder(writer))

	result, err := svc.SyncFromSession(context.Background(), SyncFromSessionInput{SupplierID: 7})

	require.Nil(t, result)
	require.Error(t, err)
	require.Len(t, writer.inputs, 1)
	input := writer.inputs[0]
	require.Equal(t, "admin_plus.balance", input.Component)
	var extra map[string]any
	require.NoError(t, json.Unmarshal([]byte(input.ExtraJSON), &extra))
	require.Equal(t, "probe_profile", extra["action"])
	require.Equal(t, "failed", extra["outcome"])
	require.Equal(t, "SUPPLIER_SESSION_BAD_STATUS", extra["reason"])
	require.Equal(t, "https://supplier.example/api/v1/user/profile", extra["endpoint"])
	require.Equal(t, float64(502), extra["status_code"])
}

func TestServiceGetCurrentUsesFreshCache(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	cache := &fakeBalanceCache{current: &CurrentBalance{
		SupplierID:     7,
		RuntimeStatus:  adminplusdomain.SupplierRuntimeStatusCandidate,
		BalanceCents:   9000,
		Currency:       "USD",
		SwitchEligible: true,
		Source:         "provider_session",
		CapturedAt:     now.Add(-time.Minute),
		RefreshAfter:   now.Add(time.Minute),
		ExpiresAt:      now.Add(10 * time.Minute),
	}}
	reader := &fakeBalanceProbeAdapter{}
	svc := NewServiceWithCurrentCache(newFakeBalanceRepository(), nil, &fakeBalanceSessionReader{}, reader, cache)
	svc.now = func() time.Time { return now }

	current, err := svc.GetCurrent(context.Background(), CurrentBalanceInput{SupplierID: 7})

	require.NoError(t, err)
	require.Equal(t, int64(9000), current.BalanceCents)
	require.False(t, current.Stale)
	require.False(t, current.Expired)
	require.False(t, current.Fallback)
	require.Equal(t, 0, reader.calls)
	require.Equal(t, 1, cache.getCalls)
}

func TestServiceGetCurrentConvertsLegacyNewAPIQuotaCache(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	cache := &fakeBalanceCache{current: &CurrentBalance{
		SupplierID:     7,
		RuntimeStatus:  adminplusdomain.SupplierRuntimeStatusCandidate,
		BalanceCents:   892305600,
		Currency:       "QTA",
		SwitchEligible: true,
		Source:         "provider_session",
		CapturedAt:     now.Add(-time.Minute),
		RefreshAfter:   now.Add(time.Minute),
		ExpiresAt:      now.Add(10 * time.Minute),
	}}
	reader := &fakeBalanceProbeAdapter{}
	svc := NewServiceWithCurrentCache(newFakeBalanceRepository(), nil, &fakeBalanceSessionReader{}, reader, cache)
	svc.now = func() time.Time { return now }

	current, err := svc.GetCurrent(context.Background(), CurrentBalanceInput{SupplierID: 7})

	require.NoError(t, err)
	require.Equal(t, int64(1785), current.BalanceCents)
	require.Equal(t, "USD", current.Currency)
	require.False(t, current.Stale)
	require.False(t, current.Expired)
	require.Equal(t, 0, reader.calls)
}

func TestServiceListSnapshotsConvertsLegacyNewAPIQuotaBalance(t *testing.T) {
	repo := newFakeBalanceRepository()
	repo.snapshots = append(repo.snapshots, &adminplusdomain.BalanceSnapshot{
		ID:           1,
		SupplierID:   7,
		BalanceCents: 892305600,
		Currency:     "QTA",
		CapturedAt:   time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC),
	})
	svc := NewService(repo)

	items, err := svc.ListSnapshots(context.Background(), SnapshotFilter{SupplierID: 7})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(1785), items[0].BalanceCents)
	require.Equal(t, "USD", items[0].Currency)
}

func TestServiceListEventsConvertsLegacyNewAPIQuotaBalance(t *testing.T) {
	oldBalance := int64(900000000)
	repo := newFakeBalanceRepository()
	repo.events = append(repo.events, &adminplusdomain.BalanceEvent{
		ID:              1,
		SupplierID:      7,
		OldBalanceCents: &oldBalance,
		NewBalanceCents: 892305600,
		Currency:        "QTA",
		Status:          adminplusdomain.BalanceEventStatusOpen,
	})
	svc := NewService(repo)

	items, err := svc.ListEvents(context.Background(), EventFilter{SupplierID: 7})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.NotNil(t, items[0].OldBalanceCents)
	require.Equal(t, int64(1800), *items[0].OldBalanceCents)
	require.Equal(t, int64(1785), items[0].NewBalanceCents)
	require.Equal(t, "USD", items[0].Currency)
}

func TestServiceGetCurrentRefreshesStaleCache(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	balanceCents := int64(1234)
	cache := &fakeBalanceCache{current: &CurrentBalance{
		SupplierID:     7,
		RuntimeStatus:  adminplusdomain.SupplierRuntimeStatusCandidate,
		BalanceCents:   9000,
		Currency:       "USD",
		SwitchEligible: true,
		Source:         "provider_session",
		CapturedAt:     now.Add(-7 * time.Minute),
		RefreshAfter:   now.Add(-time.Minute),
		ExpiresAt:      now.Add(8 * time.Minute),
	}}
	session := &fakeBalanceSessionReader{input: ports.SessionProbeInput{SupplierID: 7}}
	reader := &fakeBalanceProbeAdapter{result: &ports.SessionProbeResult{
		SupplierID:      7,
		Status:          "ok",
		SystemType:      "sub2api",
		BalanceCents:    &balanceCents,
		BalanceCurrency: "usd",
		ProbedAt:        now,
	}}
	svc := NewServiceWithCurrentCache(newFakeBalanceRepository(), nil, session, reader, cache)
	svc.now = func() time.Time { return now }

	current, err := svc.GetCurrent(context.Background(), CurrentBalanceInput{SupplierID: 7})

	require.NoError(t, err)
	require.Equal(t, int64(1234), current.BalanceCents)
	require.Equal(t, "USD", current.Currency)
	require.False(t, current.Stale)
	require.False(t, current.Expired)
	require.Equal(t, 1, reader.calls)
	require.Equal(t, 1, cache.setCalls)
	require.Equal(t, int64(1234), cache.setCurrent.BalanceCents)
}

func TestServiceGetCurrentFallsBackToZeroWhenCacheMissAndRefreshFails(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	cache := &fakeBalanceCache{err: ErrCurrentBalanceCacheMiss}
	reader := &fakeBalanceProbeAdapter{err: infraerrors.New(http.StatusBadGateway, "SUPPLIER_UNAVAILABLE", "supplier unavailable")}
	svc := NewServiceWithCurrentCache(newFakeBalanceRepository(), nil, &fakeBalanceSessionReader{}, reader, cache)
	svc.now = func() time.Time { return now }

	current, err := svc.GetCurrent(context.Background(), CurrentBalanceInput{SupplierID: 7})

	require.NoError(t, err)
	require.Equal(t, int64(0), current.BalanceCents)
	require.True(t, current.Fallback)
	require.True(t, current.Stale)
	require.True(t, current.Expired)
	require.Equal(t, "SUPPLIER_UNAVAILABLE", current.RefreshErrorReason)
	require.Equal(t, 1, reader.calls)
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

type fakeBalanceSessionReader struct {
	input      ports.SessionProbeInput
	supplierID int64
}

func (r *fakeBalanceSessionReader) DecryptedProbeInput(_ context.Context, supplierID int64) (ports.SessionProbeInput, error) {
	r.supplierID = supplierID
	return r.input, nil
}

type fakeBalanceProbeAdapter struct {
	result *ports.SessionProbeResult
	err    error
	calls  int
}

func (a *fakeBalanceProbeAdapter) ProbeSub2APIUserProfile(_ context.Context, _ ports.SessionProbeInput) (*ports.SessionProbeResult, error) {
	a.calls++
	if a.err != nil {
		return nil, a.err
	}
	return a.result, nil
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

type fakeBalanceCache struct {
	current    *CurrentBalance
	err        error
	getCalls   int
	setCalls   int
	setCurrent *CurrentBalance
	setTTL     time.Duration
}

func (c *fakeBalanceCache) GetCurrent(_ context.Context, _ int64) (*CurrentBalance, error) {
	c.getCalls++
	if c.err != nil {
		return nil, c.err
	}
	return cloneCurrentBalance(c.current), nil
}

func (c *fakeBalanceCache) SetCurrent(_ context.Context, current *CurrentBalance, ttl time.Duration) error {
	c.setCalls++
	c.setCurrent = cloneCurrentBalance(current)
	c.setTTL = ttl
	return nil
}

func cloneCurrentBalance(in *CurrentBalance) *CurrentBalance {
	if in == nil {
		return nil
	}
	out := *in
	return &out
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

type balanceLogWriter struct {
	inputs []*service.OpsInsertSystemLogInput
}

func (w *balanceLogWriter) BatchInsertSystemLogs(_ context.Context, inputs []*service.OpsInsertSystemLogInput) (int64, error) {
	w.inputs = append(w.inputs, inputs...)
	return int64(len(inputs)), nil
}
