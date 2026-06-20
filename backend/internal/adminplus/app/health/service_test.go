package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServiceRecordSampleCreatesLatencyAndErrorEvents(t *testing.T) {
	repo := newFakeHealthRepository()
	svc := NewService(repo)

	result, err := svc.RecordSample(context.Background(), RecordSampleInput{
		SupplierID:              7,
		Model:                   "gpt-4o-mini",
		FirstTokenLatencyMS:     4500,
		TotalLatencyMS:          35000,
		StatusCode:              502,
		FirstTokenThresholdMS:   3000,
		TotalLatencyThresholdMS: 30000,
	})

	require.NoError(t, err)
	require.NotNil(t, result.Sample)
	require.Len(t, result.Events, 3)
	require.Equal(t, adminplusdomain.HealthEventTypeSlowFirstToken, result.Events[0].Type)
	require.Equal(t, adminplusdomain.HealthEventTypeSlowTotal, result.Events[1].Type)
	require.Equal(t, adminplusdomain.HealthEventTypeRequestError, result.Events[2].Type)
}

func TestServiceRecordSampleCreatesConcurrencyFullEvent(t *testing.T) {
	repo := newFakeHealthRepository()
	svc := NewService(repo)
	limit := 10
	available := 0

	result, err := svc.RecordSample(context.Background(), RecordSampleInput{
		SupplierID:                   7,
		Model:                        "claude-sonnet-4",
		StatusCode:                   200,
		ObservedConcurrency:          9,
		AvailableConcurrency:         &available,
		ConcurrencyLimit:             &limit,
		ConcurrencySaturationPercent: 90,
	})

	require.NoError(t, err)
	require.Len(t, result.Events, 1)
	require.Equal(t, adminplusdomain.HealthEventTypeConcurrencyFull, result.Events[0].Type)
	require.Equal(t, int64(9), result.Events[0].ObservedValue)
	require.Equal(t, int64(10), result.Events[0].ThresholdValue)
}

func TestServiceRecordSampleKeepsHealthySampleQuiet(t *testing.T) {
	repo := newFakeHealthRepository()
	svc := NewService(repo)
	limit := 10

	result, err := svc.RecordSample(context.Background(), RecordSampleInput{
		SupplierID:              7,
		Model:                   "gemini-2.5-pro",
		FirstTokenLatencyMS:     800,
		TotalLatencyMS:          4000,
		StatusCode:              200,
		ObservedConcurrency:     2,
		ConcurrencyLimit:        &limit,
		FirstTokenThresholdMS:   3000,
		TotalLatencyThresholdMS: 30000,
	})

	require.NoError(t, err)
	require.NotNil(t, result.Sample)
	require.Empty(t, result.Events)
}

func TestServiceRecordSampleValidatesInput(t *testing.T) {
	svc := NewService(newFakeHealthRepository())

	_, err := svc.RecordSample(context.Background(), RecordSampleInput{
		SupplierID:          7,
		Model:               "gpt-4o-mini",
		FirstTokenLatencyMS: -1,
	})

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "HEALTH_LATENCY_INVALID", infraerrors.Reason(err))
}

func TestServiceProbeOpenAIResponsesRecordsGPT55Sample(t *testing.T) {
	repo := newFakeHealthRepository()
	var received struct {
		Path  string
		Model string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Path = r.URL.Path
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		received.Model, _ = body["model"].(string)
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n"))
	}))
	t.Cleanup(server.Close)
	repo.probeTarget = &ProbeTarget{
		SupplierID:              7,
		SupplierAPIBaseURL:      server.URL,
		SupplierAccountID:       3,
		LocalAccountID:          11,
		LocalAccountName:        "OpenAI upstream",
		LocalAccountPlatform:    "openai",
		LocalAccountType:        "apikey",
		LocalAccountStatus:      "active",
		LocalAccountConcurrency: 8,
		APIKey:                  "sk-test",
	}
	svc := NewService(repo)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	tick := 0
	svc.now = func() time.Time {
		tick++
		return now.Add(time.Duration(tick*100) * time.Millisecond)
	}

	result, err := svc.ProbeOpenAIResponses(context.Background(), ProbeInput{
		SupplierID:        7,
		SupplierAccountID: 3,
	})

	require.NoError(t, err)
	require.Equal(t, "/v1/responses", received.Path)
	require.Equal(t, "gpt-5.5", received.Model)
	require.NotNil(t, result.Sample)
	require.Equal(t, "responses_probe", result.Sample.Source)
	require.Equal(t, "gpt-5.5", result.Sample.Model)
	require.Equal(t, 200, result.Sample.StatusCode)
	require.Zero(t, result.Sample.ObservedConcurrency)
	require.NotNil(t, result.Sample.AvailableConcurrency)
	require.Equal(t, 8, *result.Sample.AvailableConcurrency)
	require.NotNil(t, result.Sample.ConcurrencyLimit)
	require.Equal(t, 8, *result.Sample.ConcurrencyLimit)
	require.Empty(t, result.Events)
}

type fakeHealthRepository struct {
	nextSampleID int64
	nextEventID  int64
	samples      []*adminplusdomain.HealthSample
	events       []*adminplusdomain.HealthEvent
	probeTarget  *ProbeTarget
}

func newFakeHealthRepository() *fakeHealthRepository {
	return &fakeHealthRepository{
		nextSampleID: 1,
		nextEventID:  1,
	}
}

func (r *fakeHealthRepository) CreateSample(_ context.Context, sample *adminplusdomain.HealthSample) (*adminplusdomain.HealthSample, error) {
	cp := cloneHealthSample(sample)
	cp.ID = r.nextSampleID
	r.nextSampleID++
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = cp.CapturedAt
	}
	r.samples = append(r.samples, cp)
	return cloneHealthSample(cp), nil
}

func (r *fakeHealthRepository) CreateEvent(_ context.Context, event *adminplusdomain.HealthEvent) (*adminplusdomain.HealthEvent, error) {
	cp := cloneHealthEvent(event)
	cp.ID = r.nextEventID
	r.nextEventID++
	r.events = append(r.events, cp)
	return cloneHealthEvent(cp), nil
}

func (r *fakeHealthRepository) ListSamples(_ context.Context, _ SampleFilter) ([]*adminplusdomain.HealthSample, error) {
	items := make([]*adminplusdomain.HealthSample, 0, len(r.samples))
	for _, item := range r.samples {
		items = append(items, cloneHealthSample(item))
	}
	return items, nil
}

func (r *fakeHealthRepository) ListEvents(_ context.Context, _ EventFilter) ([]*adminplusdomain.HealthEvent, error) {
	items := make([]*adminplusdomain.HealthEvent, 0, len(r.events))
	for _, item := range r.events {
		items = append(items, cloneHealthEvent(item))
	}
	return items, nil
}

func (r *fakeHealthRepository) UpdateEventStatus(_ context.Context, id int64, status adminplusdomain.HealthEventStatus) (*adminplusdomain.HealthEvent, error) {
	for _, event := range r.events {
		if event.ID == id {
			event.Status = status
			return cloneHealthEvent(event), nil
		}
	}
	return nil, nil
}

func (r *fakeHealthRepository) GetProbeTarget(_ context.Context, supplierID int64, supplierAccountID int64) (*ProbeTarget, error) {
	if r.probeTarget != nil {
		cp := *r.probeTarget
		return &cp, nil
	}
	return &ProbeTarget{
		SupplierID:              supplierID,
		SupplierAPIBaseURL:      "https://api.openai.com",
		SupplierAccountID:       supplierAccountID,
		LocalAccountID:          1,
		LocalAccountPlatform:    "openai",
		LocalAccountType:        "apikey",
		LocalAccountStatus:      "active",
		LocalAccountConcurrency: 3,
		APIKey:                  "sk-test",
	}, nil
}

func cloneHealthSample(in *adminplusdomain.HealthSample) *adminplusdomain.HealthSample {
	if in == nil {
		return nil
	}
	out := *in
	if in.AvailableConcurrency != nil {
		v := *in.AvailableConcurrency
		out.AvailableConcurrency = &v
	}
	if in.ConcurrencyLimit != nil {
		v := *in.ConcurrencyLimit
		out.ConcurrencyLimit = &v
	}
	if in.RawPayload != nil {
		out.RawPayload = make(map[string]any, len(in.RawPayload))
		for k, v := range in.RawPayload {
			out.RawPayload[k] = v
		}
	}
	return &out
}

func cloneHealthEvent(in *adminplusdomain.HealthEvent) *adminplusdomain.HealthEvent {
	if in == nil {
		return nil
	}
	out := *in
	if in.AcknowledgedAt != nil {
		t := *in.AcknowledgedAt
		out.AcknowledgedAt = &t
	}
	return &out
}
