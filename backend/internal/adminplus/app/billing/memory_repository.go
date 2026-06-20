package billing

import (
	"context"
	"sort"
	"sync"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

type MemoryRepository struct {
	mu     sync.Mutex
	nextID int64
	lines  []*adminplusdomain.SupplierBillLine
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{nextID: 1}
}

func (r *MemoryRepository) CreateBillLine(_ context.Context, line *adminplusdomain.SupplierBillLine) (*adminplusdomain.SupplierBillLine, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cp := cloneMemoryBillLine(line)
	cp.ID = r.nextID
	r.nextID++
	r.lines = append(r.lines, cp)
	return cloneMemoryBillLine(cp), nil
}

func (r *MemoryRepository) ListBillLines(_ context.Context, filter BillLineFilter) ([]*adminplusdomain.SupplierBillLine, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]*adminplusdomain.SupplierBillLine, 0, len(r.lines))
	for _, line := range r.lines {
		if filter.SupplierID > 0 && line.SupplierID != filter.SupplierID {
			continue
		}
		items = append(items, cloneMemoryBillLine(line))
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

func cloneMemoryBillLine(in *adminplusdomain.SupplierBillLine) *adminplusdomain.SupplierBillLine {
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
