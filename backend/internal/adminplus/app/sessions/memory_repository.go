package sessions

import (
	"context"
	"net/http"
	"sync"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type MemoryRepository struct {
	mu       sync.Mutex
	sessions map[int64]*adminplusdomain.SupplierBrowserSession
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{sessions: make(map[int64]*adminplusdomain.SupplierBrowserSession)}
}

func (r *MemoryRepository) Upsert(_ context.Context, session *adminplusdomain.SupplierBrowserSession) (*adminplusdomain.SupplierBrowserSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing := r.sessions[session.SupplierID]
	cp := cloneSession(session)
	if existing != nil {
		cp.CreatedAt = existing.CreatedAt
	}
	r.sessions[cp.SupplierID] = cp
	return cloneSession(cp), nil
}

func (r *MemoryRepository) Get(_ context.Context, supplierID int64) (*adminplusdomain.SupplierBrowserSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session := r.sessions[supplierID]
	if session == nil {
		return nil, infraerrors.New(http.StatusNotFound, "SUPPLIER_SESSION_NOT_FOUND", "supplier browser session not found")
	}
	return cloneSession(session), nil
}

func cloneSession(in *adminplusdomain.SupplierBrowserSession) *adminplusdomain.SupplierBrowserSession {
	if in == nil {
		return nil
	}
	out := *in
	if in.SessionSummary != nil {
		out.SessionSummary = make(map[string]any, len(in.SessionSummary))
		for k, v := range in.SessionSummary {
			out.SessionSummary[k] = v
		}
	}
	if in.ExpiresAt != nil {
		t := *in.ExpiresAt
		out.ExpiresAt = &t
	}
	return &out
}
