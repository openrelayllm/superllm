package supplierkeys

import (
	"context"
	"net/http/httptest"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type noopAdminGateway struct {
	service.AdminService
}

func TestUseSub2APIGatewayDefaultsToFailingGatewayWhenRemoteConfigMissing(t *testing.T) {
	t.Setenv(sub2APIAdminBaseURLEnv, "")
	t.Setenv(sub2APIAdminAPIKeyEnv, "")
	t.Setenv(sub2APIEmbeddedGatewayFallbackEnv, "")

	gateway := UseSub2APIGateway(&noopAdminGateway{}, nil)

	require.IsType(t, &FailingSub2APIGateway{}, gateway)
	_, err := gateway.GetAllGroupsIncludingInactive(context.Background())
	require.Equal(t, "SUB2API_GATEWAY_CONFIG_REQUIRED", infraerrors.Reason(err))
}

func TestUseSub2APIGatewayAllowsEmbeddedFallbackWhenExplicitlyEnabled(t *testing.T) {
	t.Setenv(sub2APIAdminBaseURLEnv, "")
	t.Setenv(sub2APIAdminAPIKeyEnv, "")
	t.Setenv(sub2APIEmbeddedGatewayFallbackEnv, "true")
	admin := &noopAdminGateway{}

	gateway := UseSub2APIGateway(admin, nil)

	require.Same(t, admin, gateway)
}

func TestUseSub2APIGatewayReturnsFailingGatewayForInvalidRemoteConfig(t *testing.T) {
	t.Setenv(sub2APIAdminBaseURLEnv, "ftp://sub2api.example")
	t.Setenv(sub2APIAdminAPIKeyEnv, "admin-secret")
	t.Setenv(sub2APIEmbeddedGatewayFallbackEnv, "")

	gateway := UseSub2APIGateway(&noopAdminGateway{}, nil)

	require.IsType(t, &FailingSub2APIGateway{}, gateway)
	_, err := gateway.GetAllGroupsIncludingInactive(context.Background())
	require.Equal(t, "SUB2API_GATEWAY_BASE_URL_INVALID", infraerrors.Reason(err))
}

func TestUseSub2APIGatewayUsesHTTPGatewayWhenConfigured(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()
	t.Setenv(sub2APIAdminBaseURLEnv, server.URL)
	t.Setenv(sub2APIAdminAPIKeyEnv, "admin-secret")
	t.Setenv(sub2APIEmbeddedGatewayFallbackEnv, "")

	gateway := UseSub2APIGateway(&noopAdminGateway{}, server.Client())

	require.IsType(t, &Sub2APIHTTPGateway{}, gateway)
}

func TestShouldAllowEmbeddedSub2APIGatewayFallbackFromEnv(t *testing.T) {
	t.Setenv(sub2APIEmbeddedGatewayFallbackEnv, "")
	require.False(t, ShouldAllowEmbeddedSub2APIGatewayFallbackFromEnv())
	t.Setenv(sub2APIEmbeddedGatewayFallbackEnv, "false")
	require.False(t, ShouldAllowEmbeddedSub2APIGatewayFallbackFromEnv())
	t.Setenv(sub2APIEmbeddedGatewayFallbackEnv, "true")
	require.True(t, ShouldAllowEmbeddedSub2APIGatewayFallbackFromEnv())
	t.Setenv(sub2APIEmbeddedGatewayFallbackEnv, "1")
	require.True(t, ShouldAllowEmbeddedSub2APIGatewayFallbackFromEnv())
}
