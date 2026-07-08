package suppliergroups

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type MemoryRepository struct {
	mu          sync.Mutex
	nextID      int64
	nextEventID int64
	items       map[int64]*adminplusdomain.SupplierGroup
	events      map[int64]*adminplusdomain.SupplierGroupChangeEvent
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		nextID:      1,
		nextEventID: 1,
		items:       make(map[int64]*adminplusdomain.SupplierGroup),
		events:      make(map[int64]*adminplusdomain.SupplierGroupChangeEvent),
	}
}

func (r *MemoryRepository) GetSupplierName(_ context.Context, supplierID int64) (string, error) {
	return "supplier-" + strconv.FormatInt(supplierID, 10), nil
}

func (r *MemoryRepository) UpsertMany(_ context.Context, supplierID int64, groups []*adminplusdomain.SupplierGroup, seenAt time.Time) ([]*adminplusdomain.SupplierGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	seen := make(map[string]struct{}, len(groups))
	out := make([]*adminplusdomain.SupplierGroup, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		seen[group.ExternalGroupID] = struct{}{}
		existingID := int64(0)
		for id, existing := range r.items {
			if existing.SupplierID == supplierID && existing.ExternalGroupID == group.ExternalGroupID {
				existingID = id
				break
			}
		}
		cp := cloneSupplierGroup(group)
		if existingID > 0 {
			existing := r.items[existingID]
			cp.ID = existingID
			cp.CreatedAt = existing.CreatedAt
			cp.KeyLimitPolicy = existing.KeyLimitPolicy
			cp.KeyLimitValue = existing.KeyLimitValue
		} else {
			cp.ID = r.nextID
			r.nextID++
			cp.KeyLimitPolicy = adminplusdomain.SupplierGroupKeyLimitPolicyInherit
			cp.KeyLimitValue = 0
		}
		cp.KeyCapacityStatus = groupKeyCapacityStatus(cp.KeyLimitPolicy, cp.KeyLimitValue, cp.ActiveKeyCount)
		r.items[cp.ID] = cp
		out = append(out, cloneSupplierGroup(cp))
	}
	for _, existing := range r.items {
		if existing.SupplierID != supplierID {
			continue
		}
		if _, ok := seen[existing.ExternalGroupID]; !ok {
			existing.Status = adminplusdomain.SupplierGroupStatusMissing
			existing.UpdatedAt = seenAt
		}
	}
	return out, nil
}

func (r *MemoryRepository) List(_ context.Context, filter ListFilter) ([]*adminplusdomain.SupplierGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]*adminplusdomain.SupplierGroup, 0)
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	for _, group := range r.items {
		if filter.SupplierID > 0 && group.SupplierID != filter.SupplierID {
			continue
		}
		if filter.Status != "" && group.Status != filter.Status {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(group.Name + " " + group.Description + " " + group.ProviderFamily + " " + group.ExternalGroupID + " " + group.OfficialName + " " + group.ModelFamily + " " + group.ModelSpec + " " + group.StandardKeyName)
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		items = append(items, cloneSupplierGroup(group))
	}
	sortSupplierGroups(items)
	if filter.Limit > 0 && len(items) > filter.Limit {
		items = items[:filter.Limit]
	}
	return items, nil
}

func (r *MemoryRepository) UpdateKeyCapacity(_ context.Context, in UpdateKeyCapacityInput) (*adminplusdomain.SupplierGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	group, ok := r.items[in.SupplierGroupID]
	if !ok || group.SupplierID != in.SupplierID {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_GROUP_NOT_FOUND", "supplier group not found")
	}
	group.KeyLimitPolicy = normalizeGroupKeyLimitPolicy(in.KeyLimitPolicy)
	group.KeyLimitValue = in.KeyLimitValue
	group.KeyCapacityStatus = groupKeyCapacityStatus(group.KeyLimitPolicy, group.KeyLimitValue, group.ActiveKeyCount)
	group.UpdatedAt = time.Now().UTC()
	return cloneSupplierGroup(group), nil
}

func (r *MemoryRepository) CreateChangeEvents(_ context.Context, events []*adminplusdomain.SupplierGroupChangeEvent) ([]*adminplusdomain.SupplierGroupChangeEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]*adminplusdomain.SupplierGroupChangeEvent, 0, len(events))
	for _, event := range events {
		if event == nil {
			continue
		}
		cp := cloneSupplierGroupChangeEvent(event)
		cp.ID = r.nextEventID
		r.nextEventID++
		r.events[cp.ID] = cp
		out = append(out, cloneSupplierGroupChangeEvent(cp))
	}
	return out, nil
}

func (r *MemoryRepository) ListChangeEvents(_ context.Context, filter EventFilter) ([]*adminplusdomain.SupplierGroupChangeEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]*adminplusdomain.SupplierGroupChangeEvent, 0)
	for _, event := range r.events {
		if event.SupplierID != filter.SupplierID {
			continue
		}
		if filter.Direction != "" && event.Direction != filter.Direction {
			continue
		}
		if filter.LowRate != nil && event.LowRate != *filter.LowRate {
			continue
		}
		items = append(items, cloneSupplierGroupChangeEvent(event))
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

func sortSupplierGroups(items []*adminplusdomain.SupplierGroup) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].LastSeenAt.Equal(items[j].LastSeenAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].LastSeenAt.After(items[j].LastSeenAt)
	})
}

func cloneSupplierGroup(in *adminplusdomain.SupplierGroup) *adminplusdomain.SupplierGroup {
	if in == nil {
		return nil
	}
	out := *in
	out.KeyLimitPolicy = normalizeGroupKeyLimitPolicy(out.KeyLimitPolicy)
	out.KeyCapacityStatus = groupKeyCapacityStatus(out.KeyLimitPolicy, out.KeyLimitValue, out.ActiveKeyCount)
	if in.UserRateMultiplier != nil {
		value := *in.UserRateMultiplier
		out.UserRateMultiplier = &value
	}
	if in.RPMLimit != nil {
		value := *in.RPMLimit
		out.RPMLimit = &value
	}
	if in.DailyLimitUSD != nil {
		value := *in.DailyLimitUSD
		out.DailyLimitUSD = &value
	}
	if in.WeeklyLimitUSD != nil {
		value := *in.WeeklyLimitUSD
		out.WeeklyLimitUSD = &value
	}
	if in.MonthlyLimitUSD != nil {
		value := *in.MonthlyLimitUSD
		out.MonthlyLimitUSD = &value
	}
	if in.RawPayload != nil {
		out.RawPayload = make(map[string]any, len(in.RawPayload))
		for key, value := range in.RawPayload {
			out.RawPayload[key] = value
		}
	}
	return &out
}

func cloneSupplierGroupChangeEvent(in *adminplusdomain.SupplierGroupChangeEvent) *adminplusdomain.SupplierGroupChangeEvent {
	if in == nil {
		return nil
	}
	out := *in
	if in.OldEffectiveRateMultiplier != nil {
		value := *in.OldEffectiveRateMultiplier
		out.OldEffectiveRateMultiplier = &value
	}
	if in.ChangePercent != nil {
		value := *in.ChangePercent
		out.ChangePercent = &value
	}
	return &out
}
