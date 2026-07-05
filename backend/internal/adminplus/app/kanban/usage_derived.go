package kanban

import (
	"context"
	"sort"
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

func usageDerivedFilterFromCache(filter CacheEfficiencyFilter) UsageDerivedFilter {
	return UsageDerivedFilter{
		Model:                 filter.Model,
		SupplyType:            filter.SupplyType,
		SupplierID:            filter.SupplierID,
		LocalSub2APIAccountID: filter.LocalSub2APIAccountID,
		Limit:                 filter.Limit,
	}
}

func usageDerivedFilterFromQuality(filter SupplyQualityFilter) UsageDerivedFilter {
	return UsageDerivedFilter{
		Model:                 filter.Model,
		SupplyType:            filter.SupplyType,
		SupplierID:            filter.SupplierID,
		LocalSub2APIAccountID: filter.LocalSub2APIAccountID,
		Limit:                 filter.Limit,
	}
}

func (s *Service) usageDerivedSnapshots(ctx context.Context, filter UsageDerivedFilter) (*UsageDerivedSnapshots, error) {
	if filter.SupplyType == "competitor" || filter.SupplyType == "custom" {
		return &UsageDerivedSnapshots{}, nil
	}
	now := s.now().UTC()
	if filter.Until.IsZero() {
		filter.Until = now
	}
	if filter.Since.IsZero() {
		filter.Since = filter.Until.Add(-defaultUsageDerivedWindow)
	}
	if !filter.Since.Before(filter.Until) {
		filter.Since = filter.Until.Add(-defaultUsageDerivedWindow)
	}
	filter.Model = strings.TrimSpace(filter.Model)
	filter.SupplyType = normalizeSupplyTypeFilter(filter.SupplyType)
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListUsageDerivedSnapshots(ctx, filter)
}

func sortCacheSnapshots(items []*adminplusdomain.CacheEfficiencySnapshot) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i] == nil {
			return false
		}
		if items[j] == nil {
			return true
		}
		if !items[i].ObservedAt.Equal(items[j].ObservedAt) {
			return items[i].ObservedAt.After(items[j].ObservedAt)
		}
		return items[i].ID > items[j].ID
	})
}

func sortQualitySnapshots(items []*adminplusdomain.SupplyQualitySnapshot) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i] == nil {
			return false
		}
		if items[j] == nil {
			return true
		}
		if !items[i].ObservedAt.Equal(items[j].ObservedAt) {
			return items[i].ObservedAt.After(items[j].ObservedAt)
		}
		return items[i].ID > items[j].ID
	})
}
