package suppliers

import (
	"context"
	"testing"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func TestServiceCreateSupplierPreservesBrowserLoginSecrets(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:                 "Relay",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		DashboardURL:         "https://relay.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: " ops@example.com ",
		BrowserLoginPassword: " secret-with-spaces ",
		BrowserLoginToken:    " token-with-spaces ",
	})

	require.NoError(t, err)
	require.Equal(t, "ops@example.com", supplier.BrowserLoginUsername)
	require.Equal(t, " secret-with-spaces ", supplier.BrowserLoginPassword)
	require.Equal(t, " token-with-spaces ", supplier.BrowserLoginToken)
	require.True(t, supplier.Credential.BrowserLoginPasswordConfigured)
	require.True(t, supplier.Credential.BrowserLoginTokenConfigured)

	credential, err := svc.GetBrowserCredential(context.Background(), supplier.ID)
	require.NoError(t, err)
	require.Equal(t, " secret-with-spaces ", credential.Password)
	require.Equal(t, " token-with-spaces ", credential.Token)
}

func TestServiceUpdateSupplierKeepsBrowserCredentialWhenSecretsOmitted(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:                 "Relay",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		DashboardURL:         "https://relay.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: "ops@example.com",
		BrowserLoginPassword: "secret",
		BrowserLoginToken:    "token",
	})
	require.NoError(t, err)

	updated, err := svc.Update(context.Background(), supplier.ID, UpdateSupplierInput{
		Name:                "Relay Updated",
		Kind:                adminplusdomain.SupplierKindRelay,
		Type:                adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:       adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:        adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:        "https://relay.example.com",
		BrowserLoginEnabled: true,
	})
	require.NoError(t, err)
	require.Equal(t, "Relay Updated", updated.Name)
	require.True(t, updated.Credential.BrowserLoginUsernameConfigured)
	require.True(t, updated.Credential.BrowserLoginPasswordConfigured)
	require.True(t, updated.Credential.BrowserLoginTokenConfigured)
	require.Equal(t, "op***@example.com", updated.Credential.MaskedBrowserLoginUsername)

	credential, err := svc.GetBrowserCredential(context.Background(), supplier.ID)
	require.NoError(t, err)
	require.Equal(t, "ops@example.com", credential.Username)
	require.Equal(t, "secret", credential.Password)
	require.Equal(t, "token", credential.Token)
}

func TestServiceUpdateSupplierPreservesBrowserLoginSecrets(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:                 "Relay",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		DashboardURL:         "https://relay.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: "ops@example.com",
		BrowserLoginPassword: "secret",
		BrowserLoginToken:    "token",
	})
	require.NoError(t, err)

	updated, err := svc.Update(context.Background(), supplier.ID, UpdateSupplierInput{
		Name:                 "Relay Updated",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:        adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:         adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:         "https://relay.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: " ops2@example.com ",
		BrowserLoginPassword: " changed-secret ",
		BrowserLoginToken:    " changed-token ",
	})

	require.NoError(t, err)
	require.Equal(t, "ops2@example.com", updated.BrowserLoginUsername)
	require.Equal(t, " changed-secret ", updated.BrowserLoginPassword)
	require.Equal(t, " changed-token ", updated.BrowserLoginToken)

	credential, err := svc.GetBrowserCredential(context.Background(), supplier.ID)
	require.NoError(t, err)
	require.Equal(t, "ops2@example.com", credential.Username)
	require.Equal(t, " changed-secret ", credential.Password)
	require.Equal(t, " changed-token ", credential.Token)
}
