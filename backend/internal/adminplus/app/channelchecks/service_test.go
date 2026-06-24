package channelchecks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	healthapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/health"
	supplierkeysapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/supplierkeys"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func TestServiceCheckRecommendsLowestAvailableChannel(t *testing.T) {
	repo := newFakeChannelCheckRepository()
	repo.candidates = []*Candidate{
		fakeCandidate(7, 101, 2.0, 201, true),
		fakeCandidate(7, 102, 0.7, 202, true),
	}
	receivedModels := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/responses", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		model, _ := body["model"].(string)
		receivedModels = append(receivedModels, model)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n"))
	}))
	t.Cleanup(server.Close)

	healthRepo := &fakeChannelHealthRepository{baseURL: server.URL}
	svc := NewService(repo, nil, nil, healthapp.NewService(healthRepo))
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	result, err := svc.Check(context.Background(), CheckInput{SupplierID: 7, CandidateLimit: 2})

	require.NoError(t, err)
	require.Len(t, result.Items, 2)
	require.NotNil(t, result.Best)
	require.Equal(t, int64(102), result.Best.SupplierGroupID)
	require.Equal(t, adminplusdomain.SupplierChannelProbeStatusAvailable, result.Best.ProbeStatus)
	require.True(t, result.Best.Recommended)
	require.Equal(t, DefaultProbeModel, receivedModels[0])
}

func TestServiceCheckAutoPausesFailedChannel(t *testing.T) {
	repo := newFakeChannelCheckRepository()
	repo.candidates = []*Candidate{fakeCandidate(7, 101, 0.7, 201, true)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream failed", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	healthRepo := &fakeChannelHealthRepository{baseURL: server.URL}
	svc := NewService(repo, nil, nil, healthapp.NewService(healthRepo))
	svc.now = func() time.Time { return time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC) }

	result, err := svc.Check(context.Background(), CheckInput{
		SupplierID:         7,
		SupplierGroupID:    101,
		AutoPauseOnFailure: true,
	})

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Nil(t, result.Best)
	require.Equal(t, adminplusdomain.SupplierChannelProbeStatusRequestError, result.Items[0].ProbeStatus)
	require.False(t, result.Items[0].Recommended)
	require.False(t, result.Items[0].LocalAccountSchedulable)
	require.Equal(t, []int64{201}, repo.pausedAccountIDs)
}

func TestServiceCheckUsesRequestedProbeModel(t *testing.T) {
	repo := newFakeChannelCheckRepository()
	repo.candidates = []*Candidate{fakeCandidate(7, 101, 0.7, 201, true)}
	var receivedModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		receivedModel, _ = body["model"].(string)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n"))
	}))
	t.Cleanup(server.Close)

	healthRepo := &fakeChannelHealthRepository{baseURL: server.URL}
	svc := NewService(repo, nil, nil, healthapp.NewService(healthRepo))

	result, err := svc.Check(context.Background(), CheckInput{
		SupplierID:      7,
		SupplierGroupID: 101,
		ProbeModel:      "gpt-5.5",
	})

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, "gpt-5.5", receivedModel)
	require.Equal(t, "gpt-5.5", result.Items[0].ProbeModel)
}

func TestServiceSetSchedulingEnsuresMissingLocalGroupBinding(t *testing.T) {
	repo := newFakeChannelCheckRepository()
	repo.candidates = []*Candidate{fakeCandidate(7, 101, 0.7, 201, false)}
	repo.candidates[0].LocalAccountGroupIDs = nil

	ensurer := &fakeLocalBindingEnsurer{
		after: func() {
			repo.candidates[0].LocalAccountGroupIDs = []int64{301}
		},
	}
	svc := NewServiceWithBindingEnsurer(repo, nil, nil, nil, ensurer)
	svc.now = func() time.Time { return time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC) }

	snapshot, err := svc.SetScheduling(context.Background(), 7, 101, true)

	require.NoError(t, err)
	require.True(t, snapshot.LocalAccountSchedulable)
	require.Equal(t, int64(201), snapshot.LocalSub2APIAccountID)
	require.True(t, repo.candidates[0].LocalAccountSchedulable)
	require.Len(t, ensurer.calls, 1)
	require.Equal(t, int64(7), ensurer.calls[0].SupplierID)
	require.Equal(t, int64(101), ensurer.calls[0].SupplierGroupID)
}

func TestServiceSetSchedulingKeepsConflictWhenEnsureCannotBindLocalGroup(t *testing.T) {
	repo := newFakeChannelCheckRepository()
	repo.candidates = []*Candidate{fakeCandidate(7, 101, 0.7, 201, false)}
	repo.candidates[0].LocalAccountGroupIDs = nil

	svc := NewServiceWithBindingEnsurer(repo, nil, nil, nil, &fakeLocalBindingEnsurer{})

	snapshot, err := svc.SetScheduling(context.Background(), 7, 101, true)

	require.Nil(t, snapshot)
	require.Error(t, err)
	require.Contains(t, err.Error(), "local Sub2API account is not bound to any group")
	require.False(t, repo.candidates[0].LocalAccountSchedulable)
}

func TestServiceListBestReturnsBestChannelPerProtocol(t *testing.T) {
	repo := newFakeChannelCheckRepository()
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	repo.snapshots = []*adminplusdomain.SupplierChannelCheckSnapshot{
		fakeSnapshot(1, 7, 101, "new_api", "gptpro", 0.8, true, now),
		fakeSnapshot(2, 7, 102, "new_api", "gpt-4o", 0.9, true, now.Add(time.Second)),
		fakeSnapshot(3, 7, 103, "new_api", "plus", 0.6, true, now.Add(2*time.Second)),
		fakeSnapshot(4, 7, 201, "new_api", "claude_kiro", 0.1, true, now.Add(3*time.Second)),
		fakeSnapshot(5, 7, 301, "new_api", "gemini-flash", 0.3, true, now.Add(4*time.Second)),
		fakeSnapshot(6, 8, 401, "new_api", "claude-sonnet", 0.4, true, now.Add(5*time.Second)),
	}
	svc := NewService(repo, nil, nil, nil)

	items, err := svc.ListBest(context.Background(), []int64{7})

	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, int64(103), items[0].SupplierGroupID)
	require.Equal(t, "openai", snapshotProtocolKey(items[0]))
	require.Equal(t, int64(201), items[1].SupplierGroupID)
	require.Equal(t, "claude", snapshotProtocolKey(items[1]))
	require.Equal(t, int64(301), items[2].SupplierGroupID)
	require.Equal(t, "gemini", snapshotProtocolKey(items[2]))
}

func TestServiceListBestPrefersAvailableOverLowerUntestedCandidate(t *testing.T) {
	repo := newFakeChannelCheckRepository()
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	repo.candidates = []*Candidate{
		fakeCandidate(7, 101, 0.04, 201, false),
		fakeCandidate(7, 102, 0.1, 202, true),
	}
	staleLowest := fakeSnapshot(1, 7, 101, "openai", "limited", 0.04, false, now)
	staleLowest.ProbeStatus = adminplusdomain.SupplierChannelProbeStatusNoLocalAccount
	staleLowest.ErrorClass = "local_account_missing"
	staleLowest.ErrorMessage = "local Sub2API account binding is missing"
	repo.snapshots = []*adminplusdomain.SupplierChannelCheckSnapshot{
		staleLowest,
		fakeSnapshot(2, 7, 102, "openai", "plus", 0.1, true, now.Add(time.Second)),
	}
	svc := NewService(repo, nil, nil, nil)
	svc.now = func() time.Time { return now.Add(2 * time.Second) }

	items, err := svc.ListBest(context.Background(), []int64{7})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(102), items[0].SupplierGroupID)
	require.Equal(t, 0.1, items[0].EffectiveRateMultiplier)
	require.Equal(t, adminplusdomain.SupplierChannelProbeStatusAvailable, items[0].ProbeStatus)
}

func TestServiceListBestReturnsLowestCurrentCandidateWhenNoAvailable(t *testing.T) {
	repo := newFakeChannelCheckRepository()
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	repo.candidates = []*Candidate{
		fakeCandidate(7, 101, 0.04, 201, false),
		fakeCandidate(7, 102, 0.1, 202, true),
	}
	failedLowest := fakeSnapshot(1, 7, 101, "openai", "limited", 0.04, false, now)
	failedLowest.ProbeStatus = adminplusdomain.SupplierChannelProbeStatusRequestError
	failedLowest.ErrorClass = "request_error"
	failedLowest.ErrorMessage = "upstream request failed"
	untestedHigher := fakeSnapshot(2, 7, 102, "openai", "plus", 0.1, false, now.Add(time.Second))
	untestedHigher.ProbeStatus = adminplusdomain.SupplierChannelProbeStatusUntested
	repo.snapshots = []*adminplusdomain.SupplierChannelCheckSnapshot{failedLowest, untestedHigher}
	svc := NewService(repo, nil, nil, nil)
	svc.now = func() time.Time { return now.Add(2 * time.Second) }

	items, err := svc.ListBest(context.Background(), []int64{7})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(101), items[0].SupplierGroupID)
	require.Equal(t, 0.04, items[0].EffectiveRateMultiplier)
	require.Equal(t, adminplusdomain.SupplierChannelProbeStatusRequestError, items[0].ProbeStatus)
}

type fakeChannelCheckRepository struct {
	nextID           int64
	candidates       []*Candidate
	snapshots        []*adminplusdomain.SupplierChannelCheckSnapshot
	pausedAccountIDs []int64
}

func newFakeChannelCheckRepository() *fakeChannelCheckRepository {
	return &fakeChannelCheckRepository{nextID: 1}
}

func (r *fakeChannelCheckRepository) ListCandidates(_ context.Context, supplierID int64) ([]*Candidate, error) {
	out := make([]*Candidate, 0, len(r.candidates))
	for _, candidate := range r.candidates {
		if candidate.SupplierID == supplierID {
			cp := *candidate
			cp.LocalAccountGroupIDs = append([]int64(nil), candidate.LocalAccountGroupIDs...)
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *fakeChannelCheckRepository) CreateSnapshot(_ context.Context, snapshot *adminplusdomain.SupplierChannelCheckSnapshot) (*adminplusdomain.SupplierChannelCheckSnapshot, error) {
	cp := *snapshot
	cp.ID = r.nextID
	r.nextID++
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = cp.CapturedAt
	}
	r.snapshots = append(r.snapshots, cloneChannelSnapshot(&cp))
	return cloneChannelSnapshot(&cp), nil
}

func (r *fakeChannelCheckRepository) ListLatestSnapshots(_ context.Context, supplierID int64, limit int) ([]*adminplusdomain.SupplierChannelCheckSnapshot, error) {
	items := make([]*adminplusdomain.SupplierChannelCheckSnapshot, 0)
	for _, item := range r.snapshots {
		if item.SupplierID == supplierID {
			items = append(items, cloneChannelSnapshot(item))
		}
	}
	sortSnapshotsByNewest(items)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *fakeChannelCheckRepository) ListLatestSnapshotsBySupplierIDs(_ context.Context, supplierIDs []int64) ([]*adminplusdomain.SupplierChannelCheckSnapshot, error) {
	seen := make(map[int64]struct{}, len(supplierIDs))
	for _, id := range supplierIDs {
		seen[id] = struct{}{}
	}
	items := make([]*adminplusdomain.SupplierChannelCheckSnapshot, 0)
	for _, item := range r.snapshots {
		if _, ok := seen[item.SupplierID]; ok {
			items = append(items, cloneChannelSnapshot(item))
		}
	}
	sortSnapshotsByNewest(items)
	return items, nil
}

func (r *fakeChannelCheckRepository) SetLocalAccountSchedulable(_ context.Context, localAccountID int64, schedulable bool) error {
	for _, candidate := range r.candidates {
		if candidate.LocalSub2APIAccountID == localAccountID {
			candidate.LocalAccountSchedulable = schedulable
			break
		}
	}
	if !schedulable {
		r.pausedAccountIDs = append(r.pausedAccountIDs, localAccountID)
	}
	return nil
}

type fakeChannelHealthRepository struct {
	baseURL      string
	nextSampleID int64
}

func (r *fakeChannelHealthRepository) CreateSample(_ context.Context, sample *adminplusdomain.HealthSample) (*adminplusdomain.HealthSample, error) {
	cp := *sample
	r.nextSampleID++
	cp.ID = r.nextSampleID
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = cp.CapturedAt
	}
	return &cp, nil
}

func (r *fakeChannelHealthRepository) CreateEvent(_ context.Context, event *adminplusdomain.HealthEvent) (*adminplusdomain.HealthEvent, error) {
	cp := *event
	cp.ID = 1
	return &cp, nil
}

func (r *fakeChannelHealthRepository) ListSamples(_ context.Context, _ healthapp.SampleFilter) ([]*adminplusdomain.HealthSample, error) {
	return nil, nil
}

func (r *fakeChannelHealthRepository) ListEvents(_ context.Context, _ healthapp.EventFilter) ([]*adminplusdomain.HealthEvent, error) {
	return nil, nil
}

func (r *fakeChannelHealthRepository) UpdateEventStatus(_ context.Context, _ int64, _ adminplusdomain.HealthEventStatus) (*adminplusdomain.HealthEvent, error) {
	return nil, nil
}

func (r *fakeChannelHealthRepository) GetProbeTarget(_ context.Context, supplierID int64, supplierAccountID int64) (*healthapp.ProbeTarget, error) {
	return &healthapp.ProbeTarget{
		SupplierID:              supplierID,
		SupplierAPIBaseURL:      r.baseURL,
		SupplierAccountID:       supplierAccountID,
		LocalAccountID:          201,
		LocalAccountName:        "test-openai",
		LocalAccountPlatform:    "openai",
		LocalAccountType:        "apikey",
		LocalAccountStatus:      "active",
		LocalAccountSchedulable: true,
		LocalAccountConcurrency: 3,
		APIKey:                  "sk-test",
	}, nil
}

type fakeLocalBindingEnsurer struct {
	calls []supplierkeysapp.EnsureGroupInput
	after func()
}

func (e *fakeLocalBindingEnsurer) EnsureGroup(_ context.Context, in supplierkeysapp.EnsureGroupInput) (*supplierkeysapp.EnsureAllResultItem, error) {
	e.calls = append(e.calls, in)
	if e.after != nil {
		e.after()
	}
	return &supplierkeysapp.EnsureAllResultItem{
		SupplierGroupID: in.SupplierGroupID,
		Action:          "skipped",
	}, nil
}

func fakeCandidate(supplierID int64, groupID int64, rate float64, localAccountID int64, schedulable bool) *Candidate {
	return &Candidate{
		SupplierID:              supplierID,
		SupplierName:            "supplier",
		SupplierType:            adminplusdomain.SupplierTypeSub2API,
		SupplierRuntimeStatus:   adminplusdomain.SupplierRuntimeStatusCandidate,
		SupplierHealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		SupplierGroupID:         groupID,
		ExternalGroupID:         "group",
		GroupName:               "group",
		ProviderFamily:          "openai",
		EffectiveRateMultiplier: rate,
		SupplierKeyID:           groupID + 1000,
		SupplierAccountID:       groupID + 2000,
		LocalSub2APIAccountID:   localAccountID,
		LocalAccountName:        "local",
		LocalAccountPlatform:    "openai",
		LocalAccountType:        "apikey",
		LocalAccountStatus:      "active",
		LocalAccountSchedulable: schedulable,
		LocalAccountGroupIDs:    []int64{1},
	}
}

func fakeSnapshot(id int64, supplierID int64, groupID int64, provider string, groupName string, rate float64, recommended bool, capturedAt time.Time) *adminplusdomain.SupplierChannelCheckSnapshot {
	return &adminplusdomain.SupplierChannelCheckSnapshot{
		ID:                      id,
		SupplierID:              supplierID,
		SupplierGroupID:         groupID,
		ProviderFamily:          provider,
		GroupName:               groupName,
		ProbeStatus:             adminplusdomain.SupplierChannelProbeStatusAvailable,
		Recommended:             recommended,
		EffectiveRateMultiplier: rate,
		FirstTokenMS:            1000,
		DurationMS:              1500,
		CapturedAt:              capturedAt,
		CreatedAt:               capturedAt,
	}
}

func cloneChannelSnapshot(in *adminplusdomain.SupplierChannelCheckSnapshot) *adminplusdomain.SupplierChannelCheckSnapshot {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func sortSnapshotsByNewest(items []*adminplusdomain.SupplierChannelCheckSnapshot) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CapturedAt.Equal(items[j].CapturedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CapturedAt.After(items[j].CapturedAt)
	})
}
