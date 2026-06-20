package suppliers

import (
	"context"
	"net/http"
	"testing"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServiceCreateSupplierDefaultsToMonitorOnly(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:                 "Local OpenAI Pool",
		Kind:                 adminplusdomain.SupplierKindSourceAccount,
		Type:                 adminplusdomain.SupplierTypeOpenAI,
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: "ops@example.com",
		BrowserLoginPassword: "secret",
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), supplier.ID)
	require.Equal(t, adminplusdomain.SupplierRuntimeStatusMonitorOnly, supplier.RuntimeStatus)
	require.Equal(t, adminplusdomain.SupplierHealthStatusNormal, supplier.HealthStatus)
	require.True(t, supplier.Credential.BrowserLoginEnabled)
	require.True(t, supplier.Credential.BrowserLoginUsernameConfigured)
	require.True(t, supplier.Credential.BrowserLoginPasswordConfigured)
	require.Equal(t, "op***@example.com", supplier.Credential.MaskedBrowserLoginUsername)
	require.Equal(t, "USD", supplier.BalanceCurrency)
}

func TestServiceCreateCandidateRequiresPositiveBalance(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	_, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:          "Cheap Relay",
		Kind:          adminplusdomain.SupplierKindRelay,
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusCandidate,
	})

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "SUPPLIER_BALANCE_REQUIRED_FOR_CANDIDATE", infraerrors.Reason(err))
}

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

func TestServiceAllowsMonitorOnlySupplierWithoutBalance(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:          "Discount Watch",
		Kind:          adminplusdomain.SupplierKindRelay,
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusMonitorOnly,
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierRuntimeStatusMonitorOnly, supplier.RuntimeStatus)
	require.Zero(t, supplier.BalanceCents)
}

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

func TestServiceListFiltersSuppliers(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	_, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:          "Active Relay",
		Kind:          adminplusdomain.SupplierKindRelay,
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusActive,
		BalanceCents:  1000,
	})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), CreateSupplierInput{
		Name: "Source Account",
		Kind: adminplusdomain.SupplierKindSourceAccount,
		Type: adminplusdomain.SupplierTypeOpenAI,
	})
	require.NoError(t, err)

	items, err := svc.List(context.Background(), SupplierFilter{
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusActive,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "Active Relay", items[0].Name)
}
