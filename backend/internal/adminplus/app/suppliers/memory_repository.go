package suppliers

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type MemoryRepository struct {
	mu            sync.RWMutex
	nextID        int64
	nextAccountID int64
	nextGroupID   int64
	suppliers     map[int64]*adminplusdomain.Supplier
	accounts      map[int64]*adminplusdomain.SupplierAccount
	groups        map[int64]*adminplusdomain.SupplierGroup
	localAccounts map[int64]*adminplusdomain.LocalSub2APIAccount
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		nextID:        1,
		nextAccountID: 1,
		nextGroupID:   1,
		suppliers:     make(map[int64]*adminplusdomain.Supplier),
		accounts:      make(map[int64]*adminplusdomain.SupplierAccount),
		groups:        make(map[int64]*adminplusdomain.SupplierGroup),
		localAccounts: map[int64]*adminplusdomain.LocalSub2APIAccount{
			1: {
				ID:             1,
				Name:           "Local OpenAI",
				Platform:       "openai",
				Type:           "apikey",
				Status:         "active",
				Schedulable:    true,
				Concurrency:    3,
				Priority:       50,
				RateMultiplier: 1,
			},
		},
	}
}

func (r *MemoryRepository) Create(_ context.Context, supplier *adminplusdomain.Supplier) (*adminplusdomain.Supplier, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cp := cloneSupplier(supplier)
	cp.ID = r.nextID
	r.nextID++
	r.suppliers[cp.ID] = cp
	return cloneSupplier(cp), nil
}

func (r *MemoryRepository) Get(_ context.Context, id int64) (*adminplusdomain.Supplier, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	supplier, ok := r.suppliers[id]
	if !ok {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found")
	}
	return cloneSupplier(supplier), nil
}

func (r *MemoryRepository) GetBrowserCredential(_ context.Context, id int64) (*adminplusdomain.SupplierBrowserCredential, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	supplier, ok := r.suppliers[id]
	if !ok {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found")
	}
	if !supplier.Credential.BrowserLoginEnabled {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_BROWSER_LOGIN_DISABLED", "supplier browser login is disabled")
	}
	return &adminplusdomain.SupplierBrowserCredential{
		SupplierID:   supplier.ID,
		SupplierName: supplier.Name,
		Kind:         supplier.Kind,
		Type:         supplier.Type,
		DashboardURL: supplier.DashboardURL,
		APIBaseURL:   supplier.APIBaseURL,
		Username:     supplier.BrowserLoginUsername,
		Password:     supplier.BrowserLoginPassword,
		Token:        supplier.BrowserLoginToken,
	}, nil
}

func (r *MemoryRepository) List(_ context.Context, filter SupplierFilter) ([]*adminplusdomain.Supplier, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]*adminplusdomain.Supplier, 0, len(r.suppliers))
	for _, supplier := range r.suppliers {
		if !matchesFilter(supplier, filter) {
			continue
		}
		items = append(items, cloneSupplier(supplier))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (r *MemoryRepository) Update(_ context.Context, supplier *adminplusdomain.Supplier) (*adminplusdomain.Supplier, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.suppliers[supplier.ID]
	if !ok {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found")
	}
	cp := cloneSupplier(supplier)
	cp.ID = existing.ID
	cp.CreatedAt = existing.CreatedAt
	if cp.BrowserLoginUsername == "" {
		cp.BrowserLoginUsername = existing.BrowserLoginUsername
	}
	if cp.BrowserLoginPassword == "" {
		cp.BrowserLoginPassword = existing.BrowserLoginPassword
	}
	if cp.BrowserLoginToken == "" {
		cp.BrowserLoginToken = existing.BrowserLoginToken
	}
	r.suppliers[cp.ID] = cp
	return cloneSupplier(cp), nil
}

func (r *MemoryRepository) UpdateStatus(_ context.Context, id int64, runtimeStatus adminplusdomain.SupplierRuntimeStatus, healthStatus adminplusdomain.SupplierHealthStatus) (*adminplusdomain.Supplier, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	supplier, ok := r.suppliers[id]
	if !ok {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found")
	}
	supplier.RuntimeStatus = runtimeStatus
	supplier.HealthStatus = healthStatus
	supplier.UpdatedAt = time.Now().UTC()
	return cloneSupplier(supplier), nil
}

func (r *MemoryRepository) Delete(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.suppliers[id]; !ok {
		return infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found")
	}
	delete(r.suppliers, id)
	for accountID, account := range r.accounts {
		if account.SupplierID == id {
			delete(r.accounts, accountID)
		}
	}
	for groupID, group := range r.groups {
		if group.SupplierID == id {
			delete(r.groups, groupID)
		}
	}
	return nil
}

func (r *MemoryRepository) ListAccounts(_ context.Context, supplierID int64) ([]*adminplusdomain.SupplierAccount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]*adminplusdomain.SupplierAccount, 0)
	for _, account := range r.accounts {
		if account.SupplierID == supplierID {
			items = append(items, cloneSupplierAccount(account))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (r *MemoryRepository) CreateAccount(_ context.Context, account *adminplusdomain.SupplierAccount) (*adminplusdomain.SupplierAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.suppliers[account.SupplierID]; !ok {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found")
	}
	local, ok := r.localAccounts[account.LocalSub2APIAccountID]
	if !ok {
		return nil, infraerrors.New(http.StatusNotFound, "LOCAL_ACCOUNT_NOT_FOUND", "local Sub2API account not found")
	}
	for _, existing := range r.accounts {
		if existing.SupplierID == account.SupplierID && existing.LocalSub2APIAccountID == account.LocalSub2APIAccountID {
			return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_ACCOUNT_ALREADY_BOUND", "local Sub2API account is already bound to this supplier")
		}
	}
	cp := cloneSupplierAccount(account)
	cp.ID = r.nextAccountID
	r.nextAccountID++
	cp.LocalAccountName = local.Name
	cp.LocalAccountPlatform = local.Platform
	cp.LocalAccountType = local.Type
	r.accounts[cp.ID] = cp
	return cloneSupplierAccount(cp), nil
}

func (r *MemoryRepository) UpdateAccount(_ context.Context, account *adminplusdomain.SupplierAccount) (*adminplusdomain.SupplierAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.accounts[account.ID]
	if !ok || existing.SupplierID != account.SupplierID {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_ACCOUNT_NOT_FOUND", "supplier account binding not found")
	}
	cp := cloneSupplierAccount(account)
	cp.LocalSub2APIAccountID = existing.LocalSub2APIAccountID
	cp.LocalAccountName = existing.LocalAccountName
	cp.LocalAccountPlatform = existing.LocalAccountPlatform
	cp.LocalAccountType = existing.LocalAccountType
	cp.CreatedAt = existing.CreatedAt
	r.accounts[cp.ID] = cp
	return cloneSupplierAccount(cp), nil
}

func (r *MemoryRepository) DeleteAccount(_ context.Context, supplierID int64, accountID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	account, ok := r.accounts[accountID]
	if !ok || account.SupplierID != supplierID {
		return infraerrors.New(http.StatusNotFound, "SUPPLIER_ACCOUNT_NOT_FOUND", "supplier account binding not found")
	}
	delete(r.accounts, accountID)
	return nil
}

func (r *MemoryRepository) ListLocalAccounts(_ context.Context, query string, limit int) ([]*adminplusdomain.LocalSub2APIAccount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	q := strings.ToLower(strings.TrimSpace(query))
	items := make([]*adminplusdomain.LocalSub2APIAccount, 0, len(r.localAccounts))
	for _, account := range r.localAccounts {
		haystack := strings.ToLower(account.Name + " " + account.Platform + " " + account.Type)
		if q != "" && !strings.Contains(haystack, q) {
			continue
		}
		cp := *account
		items = append(items, &cp)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *MemoryRepository) UpsertGroups(_ context.Context, groups []*adminplusdomain.SupplierGroup) ([]*adminplusdomain.SupplierGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]*adminplusdomain.SupplierGroup, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		if _, ok := r.suppliers[group.SupplierID]; !ok {
			return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found")
		}
		var existingID int64
		for id, existing := range r.groups {
			if existing.SupplierID == group.SupplierID && existing.ExternalGroupID == group.ExternalGroupID {
				existingID = id
				break
			}
		}
		cp := cloneSupplierGroup(group)
		if existingID == 0 {
			cp.ID = r.nextGroupID
			r.nextGroupID++
			if cp.CreatedAt.IsZero() {
				cp.CreatedAt = cp.UpdatedAt
			}
		} else {
			existing := r.groups[existingID]
			cp.ID = existing.ID
			cp.CreatedAt = existing.CreatedAt
		}
		r.groups[cp.ID] = cp
		items = append(items, cloneSupplierGroup(cp))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (r *MemoryRepository) ListGroups(_ context.Context, supplierID int64, status adminplusdomain.SupplierGroupStatus) ([]*adminplusdomain.SupplierGroup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.suppliers[supplierID]; !ok {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found")
	}
	items := make([]*adminplusdomain.SupplierGroup, 0)
	for _, group := range r.groups {
		if group.SupplierID != supplierID {
			continue
		}
		if status != "" && group.Status != status {
			continue
		}
		items = append(items, cloneSupplierGroup(group))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Status == items[j].Status {
			return items[i].ID < items[j].ID
		}
		return items[i].Status < items[j].Status
	})
	return items, nil
}

func (r *MemoryRepository) MarkMissingGroups(_ context.Context, supplierID int64, seenExternalGroupIDs []string, missingAt time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.suppliers[supplierID]; !ok {
		return 0, infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found")
	}
	seen := make(map[string]struct{}, len(seenExternalGroupIDs))
	for _, id := range seenExternalGroupIDs {
		seen[strings.TrimSpace(id)] = struct{}{}
	}
	count := 0
	for _, group := range r.groups {
		if group.SupplierID != supplierID {
			continue
		}
		if _, ok := seen[group.ExternalGroupID]; ok {
			continue
		}
		if group.Status == adminplusdomain.SupplierGroupStatusMissing {
			continue
		}
		group.Status = adminplusdomain.SupplierGroupStatusMissing
		group.UpdatedAt = missingAt
		count++
	}
	return count, nil
}

func matchesFilter(supplier *adminplusdomain.Supplier, filter SupplierFilter) bool {
	if filter.Kind != "" && supplier.Kind != filter.Kind {
		return false
	}
	if filter.Type != "" && supplier.Type != filter.Type {
		return false
	}
	if filter.RuntimeStatus != "" && supplier.RuntimeStatus != filter.RuntimeStatus {
		return false
	}
	if filter.HealthStatus != "" && supplier.HealthStatus != filter.HealthStatus {
		return false
	}
	if filter.Query != "" {
		haystack := strings.ToLower(supplier.Name + " " + supplier.Contact + " " + supplier.Notes)
		if !strings.Contains(haystack, filter.Query) {
			return false
		}
	}
	return true
}

func cloneSupplier(in *adminplusdomain.Supplier) *adminplusdomain.Supplier {
	if in == nil {
		return nil
	}
	out := *in
	if in.BalanceUpdatedAt != nil {
		t := *in.BalanceUpdatedAt
		out.BalanceUpdatedAt = &t
	}
	return &out
}

func cloneSupplierAccount(in *adminplusdomain.SupplierAccount) *adminplusdomain.SupplierAccount {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneSupplierGroup(in *adminplusdomain.SupplierGroup) *adminplusdomain.SupplierGroup {
	if in == nil {
		return nil
	}
	out := *in
	if len(in.RawPayload) > 0 {
		out.RawPayload = make(map[string]any, len(in.RawPayload))
		for key, value := range in.RawPayload {
			out.RawPayload[key] = value
		}
	} else {
		out.RawPayload = map[string]any{}
	}
	return &out
}
