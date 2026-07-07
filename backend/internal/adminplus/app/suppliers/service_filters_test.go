package suppliers

import (
	"context"
	"testing"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServiceListFiltersByCapabilityStatus(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	_, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:            "Ready Source",
		Kind:            adminplusdomain.SupplierKindSourceAccount,
		Type:            adminplusdomain.SupplierTypeOpenAI,
		PostgresReadDSN: "postgres://readonly",
	})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), CreateSupplierInput{
		Name: "New API Relay",
		Kind: adminplusdomain.SupplierKindRelay,
		Type: adminplusdomain.SupplierTypeNewAPI,
	})
	require.NoError(t, err)

	planned, err := svc.List(context.Background(), SupplierFilter{
		CapabilityStatus: adminplusdomain.SupplierCapabilityStatusPlanned,
	})
	require.NoError(t, err)
	require.Len(t, planned, 1)
	require.Equal(t, "New API Relay", planned[0].Name)

	available, err := svc.List(context.Background(), SupplierFilter{
		CapabilityStatus: adminplusdomain.SupplierCapabilityStatusAvailable,
	})
	require.NoError(t, err)
	require.Len(t, available, 1)
	require.Equal(t, "Ready Source", available[0].Name)
}

func TestServiceListRejectsInvalidCapabilityStatus(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	_, err := svc.List(context.Background(), SupplierFilter{
		CapabilityStatus: adminplusdomain.SupplierCapabilityStatus("unknown"),
	})

	require.Error(t, err)
	require.Equal(t, "SUPPLIER_CAPABILITY_STATUS_INVALID", infraerrors.Reason(err))
}

func TestServiceListFiltersByIntegrationProtocol(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	_, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:       "DeepSeek OpenAI",
		Kind:       adminplusdomain.SupplierKindSourceAccount,
		Type:       adminplusdomain.SupplierTypeOpenAI,
		APIBaseURL: "https://api.deepseek.com/v1",
	})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), CreateSupplierInput{
		Name:       "DeepSeek Claude",
		Kind:       adminplusdomain.SupplierKindSourceAccount,
		Type:       adminplusdomain.SupplierTypeAnthropic,
		APIBaseURL: "https://api.deepseek.com/anthropic",
	})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), CreateSupplierInput{
		Name:       "Unrecognized Relay",
		Kind:       adminplusdomain.SupplierKindRelay,
		Type:       adminplusdomain.SupplierTypeSub2API,
		APIBaseURL: "https://relay.example.com",
	})
	require.NoError(t, err)

	openai, err := svc.List(context.Background(), SupplierFilter{
		IntegrationProtocol: "openai",
	})
	require.NoError(t, err)
	require.Len(t, openai, 1)
	require.Equal(t, "DeepSeek OpenAI", openai[0].Name)

	claude, err := svc.List(context.Background(), SupplierFilter{
		IntegrationProtocol: " CLAUDE ",
	})
	require.NoError(t, err)
	require.Len(t, claude, 1)
	require.Equal(t, "DeepSeek Claude", claude[0].Name)
}

func TestServiceListRejectsInvalidIntegrationProtocol(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	_, err := svc.List(context.Background(), SupplierFilter{
		IntegrationProtocol: "codex",
	})

	require.Error(t, err)
	require.Equal(t, "SUPPLIER_INTEGRATION_PROTOCOL_INVALID", infraerrors.Reason(err))
}

func TestServiceListFiltersByPlatformHintAndFamily(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	_, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:         "DoneHub Relay",
		Kind:         adminplusdomain.SupplierKindRelay,
		Type:         adminplusdomain.SupplierTypeNewAPI,
		DashboardURL: "https://donehub.example.com",
	})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), CreateSupplierInput{
		Name:       "DeepSeek Source",
		Kind:       adminplusdomain.SupplierKindSourceAccount,
		Type:       adminplusdomain.SupplierTypeOpenAI,
		APIBaseURL: "https://api.deepseek.com/v1",
	})
	require.NoError(t, err)

	donehub, err := svc.List(context.Background(), SupplierFilter{
		PlatformHint: " DONE-HUB ",
	})
	require.NoError(t, err)
	require.Len(t, donehub, 1)
	require.Equal(t, "DoneHub Relay", donehub[0].Name)

	apiProviders, err := svc.List(context.Background(), SupplierFilter{
		PlatformFamily: "api_provider",
	})
	require.NoError(t, err)
	require.Len(t, apiProviders, 1)
	require.Equal(t, "DeepSeek Source", apiProviders[0].Name)
}

func TestServiceListRejectsInvalidPlatformFamily(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	_, err := svc.List(context.Background(), SupplierFilter{
		PlatformFamily: "unknown",
	})

	require.Error(t, err)
	require.Equal(t, "SUPPLIER_PLATFORM_FAMILY_INVALID", infraerrors.Reason(err))
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

func TestServiceListConvertsLegacyNewAPIQuotaBalance(t *testing.T) {
	repo := NewMemoryRepository()
	_, err := repo.Create(context.Background(), &adminplusdomain.Supplier{
		Name:            "Codex APIs",
		Kind:            adminplusdomain.SupplierKindRelay,
		Type:            adminplusdomain.SupplierTypeNewAPI,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusCandidate,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		BalanceCents:    892305600,
		BalanceCurrency: "QTA",
	})
	require.NoError(t, err)
	svc := NewService(repo)

	items, err := svc.List(context.Background(), SupplierFilter{})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(1785), items[0].BalanceCents)
	require.Equal(t, "USD", items[0].BalanceCurrency)
}
