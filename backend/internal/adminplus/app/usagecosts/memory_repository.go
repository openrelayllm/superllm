package usagecosts

import (
	"context"
	"sort"
	"sync"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

type MemoryRepository struct {
	mu     sync.Mutex
	nextID int64
	lines  []*adminplusdomain.SupplierUsageCostLine
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{nextID: 1}
}

func (r *MemoryRepository) CreateUsageCostLine(_ context.Context, line *adminplusdomain.SupplierUsageCostLine) (*adminplusdomain.SupplierUsageCostLine, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cp := cloneMemoryUsageCostLine(line)
	cp.ID = r.nextID
	r.nextID++
	r.lines = append(r.lines, cp)
	return cloneMemoryUsageCostLine(cp), nil
}

func (r *MemoryRepository) ListUsageCostLines(_ context.Context, filter UsageCostLineFilter) ([]*adminplusdomain.SupplierUsageCostLine, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]*adminplusdomain.SupplierUsageCostLine, 0, len(r.lines))
	for _, line := range r.lines {
		if filter.SupplierID > 0 && line.SupplierID != filter.SupplierID {
			continue
		}
		items = append(items, cloneMemoryUsageCostLine(line))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].StartedAt.Equal(items[j].StartedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].StartedAt.After(items[j].StartedAt)
	})
	if filter.Limit > 0 && len(items) > filter.Limit {
		items = items[:filter.Limit]
	}
	return items, nil
}

func cloneMemoryUsageCostLine(in *adminplusdomain.SupplierUsageCostLine) *adminplusdomain.SupplierUsageCostLine {
	if in == nil {
		return nil
	}
	out := *in
	if in.EndedAt != nil {
		t := *in.EndedAt
		out.EndedAt = &t
	}
	if in.RawPayload != nil {
		out.RawPayload = make(map[string]any, len(in.RawPayload))
		for k, v := range in.RawPayload {
			out.RawPayload[k] = v
		}
	}
	return &out
}
