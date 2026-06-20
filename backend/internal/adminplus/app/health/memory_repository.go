package health

import (
	"context"
	"net/http"
	"sort"
	"sync"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type MemoryRepository struct {
	mu           sync.Mutex
	nextSampleID int64
	nextEventID  int64
	samples      []*adminplusdomain.HealthSample
	events       []*adminplusdomain.HealthEvent
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		nextSampleID: 1,
		nextEventID:  1,
	}
}

func (r *MemoryRepository) CreateSample(_ context.Context, sample *adminplusdomain.HealthSample) (*adminplusdomain.HealthSample, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cp := cloneMemoryHealthSample(sample)
	cp.ID = r.nextSampleID
	r.nextSampleID++
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = cp.CapturedAt
	}
	r.samples = append(r.samples, cp)
	return cloneMemoryHealthSample(cp), nil
}

func (r *MemoryRepository) CreateEvent(_ context.Context, event *adminplusdomain.HealthEvent) (*adminplusdomain.HealthEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cp := cloneMemoryHealthEvent(event)
	cp.ID = r.nextEventID
	r.nextEventID++
	r.events = append(r.events, cp)
	return cloneMemoryHealthEvent(cp), nil
}

func (r *MemoryRepository) ListSamples(_ context.Context, filter SampleFilter) ([]*adminplusdomain.HealthSample, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]*adminplusdomain.HealthSample, 0, len(r.samples))
	for _, item := range r.samples {
		if filter.SupplierID > 0 && item.SupplierID != filter.SupplierID {
			continue
		}
		if filter.Model != "" && item.Model != filter.Model {
			continue
		}
		items = append(items, cloneMemoryHealthSample(item))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CapturedAt.Equal(items[j].CapturedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CapturedAt.After(items[j].CapturedAt)
	})
	if filter.Limit > 0 && len(items) > filter.Limit {
		items = items[:filter.Limit]
	}
	return items, nil
}

func (r *MemoryRepository) ListEvents(_ context.Context, filter EventFilter) ([]*adminplusdomain.HealthEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]*adminplusdomain.HealthEvent, 0, len(r.events))
	for _, item := range r.events {
		if filter.SupplierID > 0 && item.SupplierID != filter.SupplierID {
			continue
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		if filter.Type != "" && item.Type != filter.Type {
			continue
		}
		items = append(items, cloneMemoryHealthEvent(item))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if filter.Limit > 0 && len(items) > filter.Limit {
		items = items[:filter.Limit]
	}
	return items, nil
}

func (r *MemoryRepository) UpdateEventStatus(_ context.Context, id int64, status adminplusdomain.HealthEventStatus) (*adminplusdomain.HealthEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, event := range r.events {
		if event.ID == id {
			event.Status = status
			return cloneMemoryHealthEvent(event), nil
		}
	}
	return nil, infraerrors.New(http.StatusNotFound, "HEALTH_EVENT_NOT_FOUND", "health event not found")
}

func (r *MemoryRepository) GetProbeTarget(_ context.Context, supplierID int64, supplierAccountID int64) (*ProbeTarget, error) {
	if supplierID <= 0 {
		return nil, infraerrors.New(http.StatusNotFound, "HEALTH_PROBE_TARGET_NOT_FOUND", "health probe target not found")
	}
	if supplierAccountID <= 0 {
		supplierAccountID = 1
	}
	return &ProbeTarget{
		SupplierID:              supplierID,
		SupplierName:            "Memory Supplier",
		SupplierAPIBaseURL:      "https://api.openai.com",
		SupplierAccountID:       supplierAccountID,
		LocalAccountID:          1,
		LocalAccountName:        "Memory OpenAI",
		LocalAccountPlatform:    "openai",
		LocalAccountType:        "apikey",
		LocalAccountStatus:      "active",
		LocalAccountSchedulable: true,
		LocalAccountConcurrency: 3,
		APIKey:                  "sk-memory-test",
	}, nil
}

func cloneMemoryHealthSample(in *adminplusdomain.HealthSample) *adminplusdomain.HealthSample {
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

func cloneMemoryHealthEvent(in *adminplusdomain.HealthEvent) *adminplusdomain.HealthEvent {
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
