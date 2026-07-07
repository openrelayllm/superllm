package suppliers

import (
	"context"
	"testing"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServiceUpdateStatusRejectsNoBalanceCandidate(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name: "No Balance Supplier",
		Kind: adminplusdomain.SupplierKindRelay,
		Type: adminplusdomain.SupplierTypeSub2API,
	})
	require.NoError(t, err)

	_, err = svc.UpdateStatus(context.Background(), supplier.ID, UpdateSupplierStatusInput{
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusCandidate,
		HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
	})

	require.Error(t, err)
	require.Equal(t, "SUPPLIER_BALANCE_REQUIRED_FOR_CANDIDATE", infraerrors.Reason(err))
}
