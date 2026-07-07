package suppliers

import (
	"context"
	"testing"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServiceCreateSupplierAccountBindsLocalAccount(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:          "Relay",
		Kind:          adminplusdomain.SupplierKindRelay,
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusCandidate,
		BalanceCents:  1000,
	})
	require.NoError(t, err)

	account, err := svc.CreateAccount(context.Background(), CreateSupplierAccountInput{
		SupplierID:                supplier.ID,
		LocalSub2APIAccountID:     1,
		SupplierAccountIdentifier: "supplier-user",
		SupplierAccountLabel:      "primary",
		BalanceCents:              1000,
		RuntimeStatus:             adminplusdomain.SupplierRuntimeStatusCandidate,
	})

	require.NoError(t, err)
	require.Equal(t, supplier.ID, account.SupplierID)
	require.Equal(t, int64(1), account.LocalSub2APIAccountID)
	require.Equal(t, "Local OpenAI", account.LocalAccountName)
	require.True(t, account.HasUsableBalance)
}

func TestServiceRejectsSwitchableAccountWhenParentIsMonitorOnly(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name: "Relay",
		Kind: adminplusdomain.SupplierKindRelay,
		Type: adminplusdomain.SupplierTypeSub2API,
	})
	require.NoError(t, err)

	_, err = svc.CreateAccount(context.Background(), CreateSupplierAccountInput{
		SupplierID:            supplier.ID,
		LocalSub2APIAccountID: 1,
		BalanceCents:          1000,
		RuntimeStatus:         adminplusdomain.SupplierRuntimeStatusCandidate,
	})

	require.Error(t, err)
	require.Equal(t, "SUPPLIER_PARENT_NOT_SWITCHABLE", infraerrors.Reason(err))
}

func TestServiceDeleteSupplierCascadesAccountsInRepository(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name: "Relay",
		Kind: adminplusdomain.SupplierKindRelay,
		Type: adminplusdomain.SupplierTypeSub2API,
	})
	require.NoError(t, err)
	_, err = svc.CreateAccount(context.Background(), CreateSupplierAccountInput{
		SupplierID:            supplier.ID,
		LocalSub2APIAccountID: 1,
		RuntimeStatus:         adminplusdomain.SupplierRuntimeStatusMonitorOnly,
	})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(context.Background(), supplier.ID))
	_, err = svc.Get(context.Background(), supplier.ID)
	require.Error(t, err)
	require.Equal(t, "SUPPLIER_NOT_FOUND", infraerrors.Reason(err))
}

func TestServiceUpdateSupplierAccount(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:          "Relay",
		Kind:          adminplusdomain.SupplierKindRelay,
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusCandidate,
		BalanceCents:  1000,
	})
	require.NoError(t, err)
	account, err := svc.CreateAccount(context.Background(), CreateSupplierAccountInput{
		SupplierID:            supplier.ID,
		LocalSub2APIAccountID: 1,
		RuntimeStatus:         adminplusdomain.SupplierRuntimeStatusMonitorOnly,
	})
	require.NoError(t, err)

	updated, err := svc.UpdateAccount(context.Background(), UpdateSupplierAccountInput{
		SupplierID:                supplier.ID,
		AccountID:                 account.ID,
		SupplierAccountIdentifier: "supplier-key-1",
		SupplierAccountLabel:      "primary",
		RateProfile:               "discount-a",
		ConfiguredConcurrency:     8,
		ObservedMaxConcurrency:    6,
		BalanceThresholdCents:     500,
		BalanceCents:              3000,
		BalanceCurrency:           "CNY",
		RuntimeStatus:             adminplusdomain.SupplierRuntimeStatusCandidate,
		HealthStatus:              adminplusdomain.SupplierHealthStatusNormal,
	})

	require.NoError(t, err)
	require.Equal(t, account.ID, updated.ID)
	require.Equal(t, int64(1), updated.LocalSub2APIAccountID)
	require.Equal(t, "Local OpenAI", updated.LocalAccountName)
	require.Equal(t, "supplier-key-1", updated.SupplierAccountIdentifier)
	require.Equal(t, "discount-a", updated.RateProfile)
	require.Equal(t, 8, updated.ConfiguredConcurrency)
	require.Equal(t, 6, updated.ObservedMaxConcurrency)
	require.True(t, updated.HasUsableBalance)
	require.Equal(t, adminplusdomain.SupplierRuntimeStatusCandidate, updated.RuntimeStatus)
}
