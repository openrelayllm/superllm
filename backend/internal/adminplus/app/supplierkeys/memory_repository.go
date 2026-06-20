package supplierkeys

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type MemoryRepository struct {
	mu         sync.Mutex
	nextKeyID  int64
	nextBindID int64
	suppliers  map[int64]*adminplusdomain.Supplier
	groups     map[int64]*adminplusdomain.SupplierGroup
	keys       map[int64]*adminplusdomain.SupplierKey
	bindings   map[int64]*adminplusdomain.SupplierAccount
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		nextKeyID:  1,
		nextBindID: 1,
		suppliers:  make(map[int64]*adminplusdomain.Supplier),
		groups:     make(map[int64]*adminplusdomain.SupplierGroup),
		keys:       make(map[int64]*adminplusdomain.SupplierKey),
		bindings:   make(map[int64]*adminplusdomain.SupplierAccount),
	}
}

func (r *MemoryRepository) PutSupplier(supplier *adminplusdomain.Supplier) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *supplier
	r.suppliers[cp.ID] = &cp
}

func (r *MemoryRepository) PutGroup(group *adminplusdomain.SupplierGroup) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := cloneGroup(group)
	r.groups[cp.ID] = cp
}

func (r *MemoryRepository) GetSupplier(_ context.Context, supplierID int64) (*adminplusdomain.Supplier, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	supplier, ok := r.suppliers[supplierID]
	if !ok {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_NOT_FOUND", "supplier not found")
	}
	cp := *supplier
	return &cp, nil
}

func (r *MemoryRepository) GetGroup(_ context.Context, supplierID int64, groupID int64) (*adminplusdomain.SupplierGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	group, ok := r.groups[groupID]
	if !ok || group.SupplierID != supplierID {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_GROUP_NOT_FOUND", "supplier group not found")
	}
	return cloneGroup(group), nil
}

func (r *MemoryRepository) GetKey(_ context.Context, supplierID int64, keyID int64) (*adminplusdomain.SupplierKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key, ok := r.keys[keyID]
	if !ok || key.SupplierID != supplierID {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_KEY_NOT_FOUND", "supplier key not found")
	}
	return cloneKey(key), nil
}

func (r *MemoryRepository) FindActiveByGroup(_ context.Context, supplierID int64, groupID int64) (*adminplusdomain.SupplierKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var latest *adminplusdomain.SupplierKey
	for _, key := range r.keys {
		if key.SupplierID != supplierID || key.SupplierGroupID != groupID {
			continue
		}
		if !isBlockingKeyStatus(key.Status) {
			continue
		}
		if latest == nil || key.ID > latest.ID {
			latest = key
		}
	}
	return cloneKey(latest), nil
}

func (r *MemoryRepository) CreateKey(_ context.Context, key *adminplusdomain.SupplierKey) (*adminplusdomain.SupplierKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.keys {
		if existing.SupplierID == key.SupplierID && existing.SupplierGroupID == key.SupplierGroupID && isBlockingKeyStatus(existing.Status) {
			return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_GROUP_KEY_ALREADY_BOUND", "supplier group already has a bound or provisioning key")
		}
	}
	cp := cloneKey(key)
	cp.ID = r.nextKeyID
	r.nextKeyID++
	r.keys[cp.ID] = cp
	return cloneKey(cp), nil
}

func (r *MemoryRepository) UpdateKeyAfterLocalBind(_ context.Context, keyID int64, localAccount *service.Account, status adminplusdomain.SupplierKeyStatus, errorCode string, errorMessage string) (*adminplusdomain.SupplierKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key, ok := r.keys[keyID]
	if !ok {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_KEY_NOT_FOUND", "supplier key not found")
	}
	if localAccount != nil {
		key.LocalSub2APIAccountID = localAccount.ID
		key.LocalAccountName = localAccount.Name
		key.LocalAccountPlatform = localAccount.Platform
		key.LocalAccountType = localAccount.Type
	}
	key.Status = status
	key.ErrorCode = errorCode
	key.ErrorMessage = errorMessage
	return cloneKey(key), nil
}

func isBlockingKeyStatus(status adminplusdomain.SupplierKeyStatus) bool {
	switch status {
	case adminplusdomain.SupplierKeyStatusProvisioning, adminplusdomain.SupplierKeyStatusBound, adminplusdomain.SupplierKeyStatusManualSecretRequired:
		return true
	default:
		return false
	}
}

func (r *MemoryRepository) CreateBinding(_ context.Context, account *adminplusdomain.SupplierAccount) (*adminplusdomain.SupplierAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.bindings {
		if existing.SupplierID == account.SupplierID && existing.LocalSub2APIAccountID == account.LocalSub2APIAccountID {
			return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_ACCOUNT_ALREADY_BOUND", "local Sub2API account is already bound to this supplier")
		}
		if account.SupplierKeyID > 0 && existing.SupplierID == account.SupplierID && existing.SupplierKeyID == account.SupplierKeyID {
			return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_KEY_ALREADY_BOUND", "supplier key is already bound")
		}
	}
	cp := *account
	cp.ID = r.nextBindID
	r.nextBindID++
	r.bindings[cp.ID] = &cp
	out := cp
	return &out, nil
}

func (r *MemoryRepository) List(_ context.Context, filter ListFilter) ([]*adminplusdomain.SupplierKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	items := make([]*adminplusdomain.SupplierKey, 0)
	for _, key := range r.keys {
		if key.SupplierID != filter.SupplierID {
			continue
		}
		if filter.Status != "" && key.Status != filter.Status {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(key.Name + " " + key.ExternalKeyID + " " + key.ExternalGroupID + " " + key.KeyLast4)
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		items = append(items, cloneKey(key))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	if filter.Limit > 0 && len(items) > filter.Limit {
		items = items[:filter.Limit]
	}
	return items, nil
}

func cloneGroup(in *adminplusdomain.SupplierGroup) *adminplusdomain.SupplierGroup {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneKey(in *adminplusdomain.SupplierKey) *adminplusdomain.SupplierKey {
	if in == nil {
		return nil
	}
	out := *in
	out.ProvisionRequest = cloneMap(in.ProvisionRequest)
	out.ProvisionResponse = cloneMap(in.ProvisionResponse)
	return &out
}
