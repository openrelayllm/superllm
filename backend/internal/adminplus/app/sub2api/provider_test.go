package sub2api

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestSub2APIReadonlyDatabaseURLUsesConfig(t *testing.T) {
	t.Setenv("SUB2API_READONLY_DATABASE_URL", "postgres://env")

	got := sub2APIReadonlyDatabaseURL(&config.Config{
		Sub2API: config.Sub2APIIntegrationConfig{
			ReadonlyDatabaseURL: "postgres://config",
		},
	})

	require.Equal(t, "postgres://config", got)
}

func TestSub2APIReadonlyRedisDBUsesConfig(t *testing.T) {
	t.Setenv("SUB2API_READONLY_REDIS_DB", "3")

	db, ok := sub2APIReadonlyRedisDB(&config.Config{
		Sub2API: config.Sub2APIIntegrationConfig{
			ReadonlyRedisDB: 1,
		},
	})

	require.True(t, ok)
	require.Equal(t, 1, db)
}

func TestSub2APIReadonlyRedisDBUnsetConfigDoesNotUseEnvFallback(t *testing.T) {
	t.Setenv("SUB2API_READONLY_REDIS_DB", "3")

	_, ok := sub2APIReadonlyRedisDB(&config.Config{
		Sub2API: config.Sub2APIIntegrationConfig{
			ReadonlyRedisDB: -1,
		},
	})

	require.False(t, ok)
}

func TestSub2APIReadonlyRedisDBTreatsZeroValueConfigAsUnset(t *testing.T) {
	_, ok := sub2APIReadonlyRedisDB(&config.Config{})

	require.False(t, ok)
}

func TestSub2APIReadonlyRedisDBUsesEnvWhenConfigMissing(t *testing.T) {
	t.Setenv("SUB2API_READONLY_REDIS_DB", "2")

	db, ok := sub2APIReadonlyRedisDB(nil)

	require.True(t, ok)
	require.Equal(t, 2, db)
}

func TestProvideRoutingPortUsesLocalWhenRemoteConfigMissing(t *testing.T) {
	local := &SQLRepository{}

	routing := ProvideRoutingPort(local, &config.Config{}, nil)

	require.Same(t, local, routing)
}

func TestProvideRoutingPortUsesRemoteWhenConfigured(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()
	local := &SQLRepository{}

	routing := ProvideRoutingPort(local, &config.Config{
		AdminPlus: config.AdminPlusConfig{
			Sub2APIAdminBaseURL: server.URL,
			Sub2APIAdminAPIKey:  "admin-secret",
		},
	}, server.Client())

	require.IsType(t, &RemoteAdminAPIRoutingPort{}, routing)
}

func TestProvideRoutingPortFailsClosedWhenRemoteConfigInvalid(t *testing.T) {
	local := &SQLRepository{}

	routing := ProvideRoutingPort(local, &config.Config{
		AdminPlus: config.AdminPlusConfig{
			Sub2APIAdminBaseURL: "ftp://sub2api.example",
			Sub2APIAdminAPIKey:  "admin-secret",
		},
	}, nil)

	require.IsType(t, &FailingRoutingPort{}, routing)
	_, err := routing.GetAccount(context.Background(), 42)
	require.Equal(t, "SUB2API_REMOTE_ROUTING_BASE_URL_INVALID", infraerrors.Reason(err))
}
